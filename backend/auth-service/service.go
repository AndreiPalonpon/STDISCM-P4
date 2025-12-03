package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "stdiscm_p4/backend/pb/auth"
)

// AuthService implements the gRPC AuthService
type AuthService struct {
	pb.UnimplementedAuthServiceServer
	db          *mongo.Database
	usersCol    *mongo.Collection
	sessionsCol *mongo.Collection
	jwtSecret   []byte
}

// JWTClaims represents the JWT token claims
type JWTClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// Session represents an active user session
type Session struct {
	ID        string             `bson:"_id"`
	UserID    string             `bson:"user_id"`
	Token     string             `bson:"token"`
	CreatedAt primitive.DateTime `bson:"created_at"`
	ExpiresAt primitive.DateTime `bson:"expires_at"`
	IsActive  bool               `bson:"is_active"`
}

// NewAuthService creates a new AuthService instance
func NewAuthService(db *mongo.Database, jwtSecret string) *AuthService {
	return &AuthService{
		db:          db,
		usersCol:    db.Collection("users"),
		sessionsCol: db.Collection("sessions"),
		jwtSecret:   []byte(jwtSecret),
	}
}

// Login authenticates a user and returns a JWT token
func (s *AuthService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	// Validate input
	if req == nil || req.Identifier == "" || req.Password == "" {
		return &pb.LoginResponse{
			Success: false,
			Token:   "",
			User:    nil,
			Message: "identifier and password are required",
		}, nil
	}

	// Trim whitespace from identifier
	identifier := strings.TrimSpace(req.Identifier)

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Find user by email or student_id
	filter := bson.M{
		"$or": []bson.M{
			{"email": identifier},
			{"student_id": identifier},
			{"faculty_id": identifier},
		},
		"is_active": true,
	}

	var userDoc bson.M
	err := s.usersCol.FindOne(queryCtx, filter).Decode(&userDoc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &pb.LoginResponse{
				Success: false,
				Token:   "",
				User:    nil,
				Message: "invalid credentials",
			}, nil
		}
		log.Printf("Error finding user during login: %v", err)
		return nil, status.Error(codes.Internal, "authentication failed")
	}

	// Verify password
	passwordHash, ok := userDoc["password_hash"].(string)
	if !ok {
		log.Printf("Invalid password_hash format for user: %v", userDoc["_id"])
		return nil, status.Error(codes.Internal, "authentication failed")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		return &pb.LoginResponse{
			Success: false,
			Token:   "",
			User:    nil,
			Message: "invalid credentials",
		}, nil
	}

	// Convert document to User
	user, err := s.documentToUser(userDoc)
	if err != nil {
		log.Printf("Error converting user document: %v", err)
		return nil, status.Error(codes.Internal, "authentication failed")
	}

	// Generate JWT token
	token, expiresAt, err := s.generateJWT(user.Id, user.Email, user.Role)
	if err != nil {
		log.Printf("Error generating JWT: %v", err)
		return nil, status.Error(codes.Internal, "failed to generate token")
	}

	// Create session record
	sessionID, err := s.createSession(queryCtx, user.Id, token, expiresAt)
	if err != nil {
		log.Printf("Warning: Failed to create session record: %v", err)
		// Continue anyway - session tracking is not critical for login
	} else {
		log.Printf("Session created: %s for user: %s", sessionID, user.Id)
	}

	log.Printf("User logged in successfully: %s (%s)", user.Email, user.Role)

	return &pb.LoginResponse{
		Success: true,
		Token:   token,
		User:    user,
		Message: "login successful",
	}, nil
}

// Logout invalidates a user session
func (s *AuthService) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	// Validate input
	if req == nil || req.Token == "" {
		return &pb.LogoutResponse{
			Success: false,
			Message: "token is required",
		}, nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Parse token to get user ID (without full validation)
	claims, err := s.parseJWTWithoutValidation(req.Token)
	if err != nil {
		log.Printf("Error parsing token during logout: %v", err)
		return &pb.LogoutResponse{
			Success: false,
			Message: "invalid token format",
		}, nil
	}

	// Invalidate all sessions for this token
	filter := bson.M{
		"token":     req.Token,
		"is_active": true,
	}

	update := bson.M{
		"$set": bson.M{
			"is_active": false,
		},
	}

	result, err := s.sessionsCol.UpdateMany(queryCtx, filter, update)
	if err != nil {
		log.Printf("Error invalidating session: %v", err)
		return nil, status.Error(codes.Internal, "logout failed")
	}

	if result.MatchedCount == 0 {
		log.Printf("No active session found for token (user may already be logged out)")
	}

	log.Printf("User logged out: %s", claims.UserID)

	return &pb.LogoutResponse{
		Success: true,
		Message: "logout successful",
	}, nil
}

// ValidateToken verifies a JWT token and returns user information
func (s *AuthService) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	// Validate input
	if req == nil || req.Token == "" {
		return &pb.ValidateTokenResponse{
			Valid:   false,
			User:    nil,
			Message: "token is required",
		}, nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Parse and validate JWT
	claims, err := s.validateJWT(req.Token)
	if err != nil {
		return &pb.ValidateTokenResponse{
			Valid:   false,
			User:    nil,
			Message: fmt.Sprintf("invalid token: %v", err),
		}, nil
	}

	// Check if session is still active
	var session Session
	err = s.sessionsCol.FindOne(queryCtx, bson.M{
		"token":     req.Token,
		"is_active": true,
	}).Decode(&session)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &pb.ValidateTokenResponse{
				Valid:   false,
				User:    nil,
				Message: "session expired or invalid",
			}, nil
		}
		log.Printf("Error checking session: %v", err)
		return nil, status.Error(codes.Internal, "validation failed")
	}

	// Fetch current user data
	var userDoc bson.M
	err = s.usersCol.FindOne(queryCtx, bson.M{
		"_id":       claims.UserID,
		"is_active": true,
	}).Decode(&userDoc)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &pb.ValidateTokenResponse{
				Valid:   false,
				User:    nil,
				Message: "user not found or inactive",
			}, nil
		}
		log.Printf("Error fetching user: %v", err)
		return nil, status.Error(codes.Internal, "validation failed")
	}

	// Convert to User message
	user, err := s.documentToUser(userDoc)
	if err != nil {
		log.Printf("Error converting user document: %v", err)
		return nil, status.Error(codes.Internal, "validation failed")
	}

	return &pb.ValidateTokenResponse{
		Valid:   true,
		User:    user,
		Message: "token valid",
	}, nil
}

// ChangePassword allows a user to change their password
func (s *AuthService) ChangePassword(ctx context.Context, req *pb.ChangePasswordRequest) (*pb.ChangePasswordResponse, error) {
	// Validate input
	if req == nil || req.UserId == "" || req.OldPassword == "" || req.NewPassword == "" {
		return &pb.ChangePasswordResponse{
			Success: false,
			Message: "user_id, old_password, and new_password are required",
		}, nil
	}

	// Validate new password strength
	if len(req.NewPassword) < 8 {
		return &pb.ChangePasswordResponse{
			Success: false,
			Message: "new password must be at least 8 characters long",
		}, nil
	}

	if req.OldPassword == req.NewPassword {
		return &pb.ChangePasswordResponse{
			Success: false,
			Message: "new password must be different from old password",
		}, nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Find user
	var userDoc bson.M
	err := s.usersCol.FindOne(queryCtx, bson.M{
		"_id":       req.UserId,
		"is_active": true,
	}).Decode(&userDoc)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &pb.ChangePasswordResponse{
				Success: false,
				Message: "user not found",
			}, nil
		}
		log.Printf("Error finding user for password change: %v", err)
		return nil, status.Error(codes.Internal, "password change failed")
	}

	// Verify old password
	passwordHash, ok := userDoc["password_hash"].(string)
	if !ok {
		log.Printf("Invalid password_hash format for user: %s", req.UserId)
		return nil, status.Error(codes.Internal, "password change failed")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.OldPassword)); err != nil {
		return &pb.ChangePasswordResponse{
			Success: false,
			Message: "incorrect old password",
		}, nil
	}

	// Hash new password
	newPasswordHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing new password: %v", err)
		return nil, status.Error(codes.Internal, "password change failed")
	}

	// Update password
	update := bson.M{
		"$set": bson.M{
			"password_hash": string(newPasswordHash),
			"updated_at":    primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	result, err := s.usersCol.UpdateOne(queryCtx, bson.M{"_id": req.UserId}, update)
	if err != nil {
		log.Printf("Error updating password: %v", err)
		return nil, status.Error(codes.Internal, "password change failed")
	}

	if result.MatchedCount == 0 {
		return &pb.ChangePasswordResponse{
			Success: false,
			Message: "user not found",
		}, nil
	}

	// Invalidate all existing sessions for this user (force re-login)
	_, err = s.sessionsCol.UpdateMany(queryCtx, bson.M{
		"user_id":   req.UserId,
		"is_active": true,
	}, bson.M{
		"$set": bson.M{"is_active": false},
	})

	if err != nil {
		log.Printf("Warning: Failed to invalidate sessions after password change: %v", err)
		// Continue anyway
	}

	log.Printf("Password changed successfully for user: %s", req.UserId)

	return &pb.ChangePasswordResponse{
		Success: true,
		Message: "password changed successfully, please login again",
	}, nil
}

// ============================================================================
// Helper Functions (Private to service.go)
// ============================================================================

// documentToUser converts a MongoDB document to a protobuf User message
func (s *AuthService) documentToUser(doc bson.M) (*pb.User, error) {
	user := &pb.User{}

	// Required fields
	if id, ok := doc["_id"].(string); ok {
		user.Id = id
	} else {
		return nil, fmt.Errorf("missing or invalid _id field")
	}

	if email, ok := doc["email"].(string); ok {
		user.Email = email
	} else {
		return nil, fmt.Errorf("missing or invalid email field")
	}

	if role, ok := doc["role"].(string); ok {
		user.Role = role
	} else {
		return nil, fmt.Errorf("missing or invalid role field")
	}

	if name, ok := doc["name"].(string); ok {
		user.Name = name
	} else {
		return nil, fmt.Errorf("missing or invalid name field")
	}

	// Optional fields
	if studentID, ok := doc["student_id"].(string); ok {
		user.StudentId = studentID
	}

	if facultyID, ok := doc["faculty_id"].(string); ok {
		user.FacultyId = facultyID
	}

	if department, ok := doc["department"].(string); ok {
		user.Department = department
	}

	if major, ok := doc["major"].(string); ok {
		user.Major = major
	}

	if yearLevel, ok := doc["year_level"].(int32); ok {
		user.YearLevel = yearLevel
	} else if yearLevel, ok := doc["year_level"].(int64); ok {
		user.YearLevel = int32(yearLevel)
	}

	if isActive, ok := doc["is_active"].(bool); ok {
		user.IsActive = isActive
	} else {
		user.IsActive = true // Default to true
	}

	// Timestamps
	if createdAt, ok := doc["created_at"].(primitive.DateTime); ok {
		user.CreatedAt = timestamppb.New(createdAt.Time())
	}

	return user, nil
}

// generateJWT creates a new JWT token for a user
func (s *AuthService) generateJWT(userID, email, role string) (string, time.Time, error) {
	expiresAt := time.Now().Add(24 * time.Hour) // Token valid for 24 hours

	claims := JWTClaims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "enrollment-system",
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, expiresAt, nil
}

// validateJWT verifies and parses a JWT token
func (s *AuthService) validateJWT(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}

// parseJWTWithoutValidation extracts claims without full validation (for logout)
func (s *AuthService) parseJWTWithoutValidation(tokenString string) (*JWTClaims, error) {
	token, _, err := jwt.NewParser().ParseUnverified(tokenString, &JWTClaims{})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token format")
}

// createSession stores a new session in the database
func (s *AuthService) createSession(ctx context.Context, userID, token string, expiresAt time.Time) (string, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		return "", err
	}

	session := Session{
		ID:        sessionID,
		UserID:    userID,
		Token:     token,
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		ExpiresAt: primitive.NewDateTimeFromTime(expiresAt),
		IsActive:  true,
	}

	_, err = s.sessionsCol.InsertOne(ctx, session)
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	return sessionID, nil
}

// generateSessionID creates a unique session identifier
func generateSessionID() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("failed to generate session ID: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
