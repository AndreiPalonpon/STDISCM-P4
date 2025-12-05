package auth

import (
	"context"
	"log"
	"net"
	"testing"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pb "stdiscm_p4/backend/internal/pb/auth"
	"stdiscm_p4/backend/internal/shared"
)

const bufSize = 1024 * 1024

var lis *bufconn.Listener

// initServer sets up the real server using bufconn
func initServer() *grpc.Server {
	// 1. Load Config & DB
	if err := godotenv.Load("../../cmd/auth/.env"); err != nil {
		log.Println("No .env file found, using defaults")
	}
	cfg, _ := shared.LoadServiceConfig("auth-service")
	_, db, err := shared.ConnectMongoDB(&cfg.MongoDB)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}

	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()

	// 2. Initialize Real Service
	authService := NewAuthService(db, cfg)
	pb.RegisterAuthServiceServer(s, authService)

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()

	return s
}

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func TestAuthService_Integration(t *testing.T) {
	server := initServer()
	defer server.Stop()

	ctx := context.Background()
	conn, err := grpc.NewClient("passthrough://bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewAuthServiceClient(conn)

	// --- SETUP DATA ---
	// Manually inject a test user into MongoDB to test Login
	cfg, _ := shared.LoadServiceConfig("auth-service")
	_, db, _ := shared.ConnectMongoDB(&cfg.MongoDB)
	usersCol := db.Collection("users")

	testPassword := "secret123"
	hashedPwd, _ := bcrypt.GenerateFromPassword([]byte(testPassword), 10)
	testUserID := "test_auth_user_001"

	testUser := shared.User{
		ID:           testUserID,
		Email:        "test_auth@example.com",
		PasswordHash: string(hashedPwd),
		Role:         "student",
		Name:         "Integration Test User",
		IsActive:     true,
		StudentID:    "202400001",
	}

	// Clean up before and after
	usersCol.DeleteOne(ctx, map[string]interface{}{"_id": testUserID})
	defer usersCol.DeleteOne(ctx, map[string]interface{}{"_id": testUserID})

	_, err = usersCol.InsertOne(ctx, testUser)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}

	// --- 1. Test Login ---
	t.Run("Login Success", func(t *testing.T) {
		resp, err := client.Login(ctx, &pb.LoginRequest{
			Identifier: "test_auth@example.com",
			Password:   testPassword,
		})
		if err != nil {
			t.Fatalf("Login failed: %v", err)
		}
		if !resp.Success || resp.Token == "" {
			t.Errorf("Expected success and token, got: %v", resp)
		}
	})

	// --- 2. Test Login Failure ---
	t.Run("Login Invalid Password", func(t *testing.T) {
		_, err := client.Login(ctx, &pb.LoginRequest{
			Identifier: "test_auth@example.com",
			Password:   "wrongpassword",
		})
		if err == nil {
			t.Error("Expected error for wrong password, got nil")
		}
	})

	// --- 3. Test Validate Token ---
	var authToken string
	t.Run("Validate Token", func(t *testing.T) {
		// Login first to get token
		loginResp, _ := client.Login(ctx, &pb.LoginRequest{
			Identifier: "test_auth@example.com",
			Password:   testPassword,
		})
		authToken = loginResp.Token

		// Validate
		valResp, err := client.ValidateToken(ctx, &pb.ValidateTokenRequest{Token: authToken})
		if err != nil {
			t.Fatalf("ValidateToken failed: %v", err)
		}
		if !valResp.Valid || valResp.User.Email != "test_auth@example.com" {
			t.Errorf("Token invalid or wrong user returned")
		}
	})

	// --- 4. Test Change Password ---
	t.Run("Change Password", func(t *testing.T) {
		newPass := "new_secret_456"
		resp, err := client.ChangePassword(ctx, &pb.ChangePasswordRequest{
			UserId:      testUserID,
			OldPassword: testPassword,
			NewPassword: newPass,
		})
		if err != nil {
			t.Fatalf("ChangePassword failed: %v", err)
		}
		if !resp.Success {
			t.Error("ChangePassword returned success=false")
		}

		// Verify login with new password
		loginResp, err := client.Login(ctx, &pb.LoginRequest{
			Identifier: "test_auth@example.com",
			Password:   newPass,
		})
		if err != nil || !loginResp.Success {
			t.Error("Could not login with new password")
		}
	})

	// --- 5. Test Logout ---
	t.Run("Logout", func(t *testing.T) {
		// Login again to get a fresh token (previous sessions might be cleared by change password)
		loginResp, _ := client.Login(ctx, &pb.LoginRequest{
			Identifier: "test_auth@example.com",
			Password:   "new_secret_456",
		})

		logoutResp, err := client.Logout(ctx, &pb.LogoutRequest{Token: loginResp.Token})
		if err != nil {
			t.Fatalf("Logout failed: %v", err)
		}
		if !logoutResp.Success {
			t.Error("Logout returned success=false")
		}

		// Verify token is invalid
		valResp, _ := client.ValidateToken(ctx, &pb.ValidateTokenRequest{Token: loginResp.Token})
		if valResp.Valid {
			t.Error("Token should be invalid after logout")
		}
	})
}
