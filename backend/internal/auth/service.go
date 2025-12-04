package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "stdiscm_p4/backend/internal/pb/auth"
	"stdiscm_p4/backend/internal/shared"
)

// AuthService implements the gRPC AuthService
type AuthService struct {
	pb.UnimplementedAuthServiceServer
	db          *mongo.Database
	config      *shared.ServiceConfig
	usersCol    *mongo.Collection
	sessionsCol *mongo.Collection
}

// CustomClaims for JWT
type CustomClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// NewAuthService creates a new AuthService instance
func NewAuthService(db *mongo.Database, config *shared.ServiceConfig) *AuthService {
	return &AuthService{
		db:          db,
		config:      config,
		usersCol:    db.Collection("users"),
		sessionsCol: db.Collection("sessions"),
	}
}

// Login authenticates a user and returns a JWT
func (s *AuthService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	if req.Identifier == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "identifier and password are required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 1. Find User (by Email OR Student ID/Faculty ID)
	var user shared.User
	filter := bson.M{
		"$or": []bson.M{
			{"email": req.Identifier},
			{"student_id": req.Identifier},
			{"faculty_id": req.Identifier},
		},
	}

	err := s.usersCol.FindOne(queryCtx, filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}
		return nil, status.Error(codes.Internal, "database error")
	}

	// 2. Check Password (BCrypt)
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	if !user.IsActive {
		return nil, status.Error(codes.PermissionDenied, "account is inactive")
	}

	// 3. Generate JWT using Shared Config
	tokenString, expiresAt, err := s.generateToken(user.ID, user.Role)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate token")
	}

	// 4. Create Session in DB (allows for server-side logout/revocation)
	session := shared.Session{
		ID:        shared.GenerateID("sess"),
		UserID:    user.ID,
		Token:     tokenString,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}

	if _, err := s.sessionsCol.InsertOne(queryCtx, session); err != nil {
		return nil, status.Error(codes.Internal, "failed to create session")
	}

	// 5. Convert to Proto User
	protoUser := s.userToProto(&user)

	return &pb.LoginResponse{
		Success: true,
		Token:   tokenString,
		User:    protoUser,
		Message: "login successful",
	}, nil
}

// Logout invalidates the user's session
func (s *AuthService) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	if req.Token == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Remove session from DB
	// Changed to DeleteMany to ensure idempotency and handle potential duplicate tokens from rapid testing
	result, err := s.sessionsCol.DeleteMany(queryCtx, bson.M{"token": req.Token})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to logout")
	}

	if result.DeletedCount == 0 {
		// Even if not found, we treat logout as successful from client perspective (idempotent operation)
		return &pb.LogoutResponse{Success: true, Message: "session already expired or invalid"}, nil
	}

	return &pb.LogoutResponse{Success: true, Message: "logout successful"}, nil
}

// ValidateToken checks if a token is valid and active
func (s *AuthService) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	if req.Token == "" {
		return &pb.ValidateTokenResponse{Valid: false, Message: "token missing"}, nil
	}

	// 1. Parse and Verify Signature locally
	token, claims, err := s.parseToken(req.Token)
	if err != nil || !token.Valid {
		return &pb.ValidateTokenResponse{Valid: false, Message: "invalid token signature"}, nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 2. Check Database for Active Session (Revocation Check)
	count, err := s.sessionsCol.CountDocuments(queryCtx, bson.M{"token": req.Token})
	if err != nil || count == 0 {
		return &pb.ValidateTokenResponse{Valid: false, Message: "session expired or revoked"}, nil
	}

	// 3. Fetch User Details
	var user shared.User
	err = s.usersCol.FindOne(queryCtx, bson.M{"_id": claims.UserID}).Decode(&user)
	if err != nil {
		return &pb.ValidateTokenResponse{Valid: false, Message: "user not found"}, nil
	}

	if !user.IsActive {
		return &pb.ValidateTokenResponse{Valid: false, Message: "account inactive"}, nil
	}

	return &pb.ValidateTokenResponse{
		Valid: true,
		User:  s.userToProto(&user),
	}, nil
}

// ChangePassword updates the user's password
func (s *AuthService) ChangePassword(ctx context.Context, req *pb.ChangePasswordRequest) (*pb.ChangePasswordResponse, error) {
	if req.UserId == "" || req.OldPassword == "" || req.NewPassword == "" {
		return nil, status.Error(codes.InvalidArgument, "all fields required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// 1. Fetch User
	var user shared.User
	err := s.usersCol.FindOne(queryCtx, bson.M{"_id": req.UserId}).Decode(&user)
	if err != nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	// 2. Verify Old Password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		return &pb.ChangePasswordResponse{Success: false, Message: "incorrect old password"}, nil
	}

	// 3. Hash New Password using Shared Config Cost
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), s.config.Security.BCryptCost)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to process password")
	}

	// 4. Update DB
	_, err = s.usersCol.UpdateOne(queryCtx, bson.M{"_id": req.UserId}, bson.M{
		"$set": bson.M{
			"password_hash": string(newHash),
			"updated_at":    primitive.NewDateTimeFromTime(time.Now()),
		},
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to update password")
	}

	// 5. Invalidate existing sessions (Force logout)
	_, _ = s.sessionsCol.DeleteMany(queryCtx, bson.M{"user_id": req.UserId})

	return &pb.ChangePasswordResponse{Success: true, Message: "password changed successfully"}, nil
}

// ============================================================================
// Internal Helpers
// ============================================================================

// generateToken creates a signed JWT using Shared Config
func (s *AuthService) generateToken(userID, role string) (string, time.Time, error) {
	expirationTime := time.Now().Add(time.Duration(s.config.Security.JWTExpirationHours) * time.Hour)

	claims := CustomClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			// Add unique ID (jti) to claims to ensure tokens are unique even if generated at the exact same timestamp
			ID:        shared.GenerateID("jti"),
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "college-enrollment-system",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.config.Security.JWTSecret))

	return tokenString, expirationTime, err
}

// parseToken validates the JWT signature and extracts claims
func (s *AuthService) parseToken(tokenString string) (*jwt.Token, *CustomClaims, error) {
	claims := &CustomClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		// Use secret from Shared Config
		return []byte(s.config.Security.JWTSecret), nil
	})

	return token, claims, err
}

// userToProto maps the shared MongoDB model to the Protobuf message
func (s *AuthService) userToProto(u *shared.User) *pb.User {
	return &pb.User{
		Id:         u.ID,
		Email:      u.Email,
		Role:       u.Role,
		Name:       u.Name,
		CreatedAt:  timestamppb.New(u.CreatedAt),
		StudentId:  u.StudentID,
		FacultyId:  u.FacultyID,
		Department: u.Department,
		Major:      u.Major,
		YearLevel:  u.YearLevel,
		IsActive:   u.IsActive,
	}
}
