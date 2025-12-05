package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	pb_admin "stdiscm_p4/backend/internal/pb/admin"
)

func TestGateway_Auth(t *testing.T) {
	env := setupGatewayTestEnv(t)
	ctx := context.Background()

	// Setup: Create user via Admin Service directly
	uResp, _ := env.AdminClient.CreateUser(ctx, &pb_admin.CreateUserRequest{
		Email: "auth_user@test.com", Role: "student", Name: "Auth User", StudentId: "AUTH001",
	})
	userPass := uResp.InitialPassword

	// --- Test 1: Login (POST /api/auth/login) -> gRPC Login ---
	var authToken string
	t.Run("Login Success", func(t *testing.T) {
		body := map[string]string{
			"identifier": "auth_user@test.com",
			"password":   userPass,
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &resp)
		if token, ok := resp["token"].(string); ok {
			authToken = token
		} else {
			t.Error("Token missing in response")
		}
	})

	// --- Test 2: Validate Token (GET /api/auth/validate) -> gRPC ValidateToken ---
	t.Run("Validate Token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/auth/validate", nil)
		req.Header.Set("Authorization", "Bearer "+authToken)

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", rr.Code)
		}
	})

	// --- Test 3: Change Password (POST /api/auth/change-password) -> gRPC ChangePassword ---
	t.Run("Change Password", func(t *testing.T) {
		body := map[string]string{
			"old_password": userPass,
			"new_password": "newSecretPassword123",
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/auth/change-password", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+authToken)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
		}
	})

	// --- Test 4: Logout (POST /api/auth/logout) -> gRPC Logout ---
	t.Run("Logout", func(t *testing.T) {
		// Login again to get a fresh token if needed, or reuse existing if valid.
		// We reuse authToken since it should still be valid.
		req, _ := http.NewRequest("POST", "/api/auth/logout", nil)
		req.Header.Set("Authorization", "Bearer "+authToken)

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
		}
	})
}
