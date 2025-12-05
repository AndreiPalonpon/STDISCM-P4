package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	pb_admin "stdiscm_p4/backend/internal/pb/admin"
	pb_auth "stdiscm_p4/backend/internal/pb/auth"
)

func TestGateway_Admin(t *testing.T) {
	env := setupGatewayTestEnv(t)
	ctx := context.Background()

	// --- Setup: Create Admin User & Get Token ---
	uResp, _ := env.AdminClient.CreateUser(ctx, &pb_admin.CreateUserRequest{
		Email: "admin@test.com", Role: "admin", Name: "Gateway Admin",
	})
	lResp, _ := env.AuthClient.Login(ctx, &pb_auth.LoginRequest{
		Identifier: "admin@test.com", Password: uResp.InitialPassword,
	})
	adminToken := lResp.Token
	adminID := lResp.User.Id

	// --- Test 1: Get System Stats (GET /api/admin/stats) ---
	t.Run("Get System Stats", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/admin/stats", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
		}
		// Basic body check
		var resp map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &resp)
		if _, ok := resp["stats"]; !ok {
			t.Error("Response missing 'stats' object")
		}
	})

	var createdCourseID string

	// --- Test 2: Create Course (POST /api/admin/courses) ---
	t.Run("Create Course", func(t *testing.T) {
		body := map[string]interface{}{
			"code":     "GATEWAY-101",
			"title":    "Gateway Testing",
			"units":    3,
			"semester": "TestSem",
			"capacity": 30,
			"schedule": "MWF 10:00-11:00",
			"room":     "VIRTUAL",
			// FIX: Removed invalid faculty_id "FAC-001" which caused service failure
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/admin/courses", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("Expected 201 Created, got %d. Body: %s", rr.Code, rr.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &resp)
		if id, ok := resp["course_id"].(string); ok {
			createdCourseID = id
		}
	})

	// --- Test 2b: Update Course (PUT /api/admin/courses/:id) ---
	t.Run("Update Course", func(t *testing.T) {
		body := map[string]interface{}{
			"title":    "Updated Gateway Testing",
			"capacity": 40,
			"is_open":  true,
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("PUT", "/api/admin/courses/"+createdCourseID, bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", rr.Code)
		}
	})

	// --- Test 2c: Assign Faculty (POST /api/admin/courses/:id/assign-faculty) ---
	t.Run("Assign Faculty", func(t *testing.T) {
		// Create Faculty first to assign
		facResp, _ := env.AdminClient.CreateUser(ctx, &pb_admin.CreateUserRequest{
			Email: "faculty@test.com", Role: "faculty", Name: "Test Faculty", FacultyId: "FAC-NEW",
		})

		body := map[string]string{"faculty_id": facResp.UserId}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/admin/courses/"+createdCourseID+"/assign-faculty", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", rr.Code)
		}
	})

	var createdUserID string

	// --- Test 3: Create User (POST /api/admin/users) ---
	t.Run("Create User", func(t *testing.T) {
		body := map[string]interface{}{
			"email":      "newuser@test.com",
			"role":       "student",
			"name":       "New User",
			"student_id": "99999",
			"major":      "CS",
			"year_level": 1,
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/admin/users", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("Expected 201 Created, got %d. Body: %s", rr.Code, rr.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &resp)
		if id, ok := resp["user_id"].(string); ok {
			createdUserID = id
		}
	})

	// --- Test 3b: List Users (GET /api/admin/users) ---
	t.Run("List Users", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/admin/users?role=student", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", rr.Code)
		}
	})

	// --- Test 3c: Reset Password (POST /api/admin/users/:id/reset-password) ---
	t.Run("Reset Password", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/admin/users/"+createdUserID+"/reset-password", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", rr.Code)
		}
	})

	// --- Test 3d: Toggle User Status (PATCH /api/admin/users/:id/status) ---
	t.Run("Toggle User Status", func(t *testing.T) {
		body := map[string]bool{"activate": false}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("PATCH", "/api/admin/users/"+createdUserID+"/status", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", rr.Code)
		}
	})

	// --- Test 4: Configure System (POST /api/admin/enrollment/toggle) ---
	t.Run("Toggle Enrollment", func(t *testing.T) {
		body := map[string]interface{}{"enable": true}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/admin/enrollment/toggle", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", rr.Code)
		}
	})

	// --- Test 4b: Set Enrollment Period (POST /api/admin/enrollment/period) ---
	t.Run("Set Enrollment Period", func(t *testing.T) {
		body := map[string]string{
			"start_date": "2024-01-01T00:00:00Z",
			"end_date":   "2024-02-01T00:00:00Z",
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/admin/enrollment/period", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", rr.Code)
		}
	})

	// --- Test 5: Overrides (POST /api/admin/override/enroll) ---
	t.Run("Override Enroll", func(t *testing.T) {
		// Re-activate user first since we deactivated them above
		env.AdminClient.ToggleUserStatus(ctx, &pb_admin.ToggleUserStatusRequest{UserId: createdUserID, Activate: true})

		body := map[string]string{
			"student_id": createdUserID,
			"course_id":  createdCourseID,
			"reason":     "Testing override",
			"admin_id":   adminID,
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/admin/override/enroll", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d. Msg: %s", rr.Code, rr.Body.String())
		}
	})

	// --- Test 5b: Override Drop (POST /api/admin/override/drop) ---
	t.Run("Override Drop", func(t *testing.T) {
		body := map[string]string{
			"student_id": createdUserID,
			"course_id":  createdCourseID,
			"reason":     "Testing override drop",
			"admin_id":   adminID,
		}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/admin/override/drop", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", rr.Code)
		}
	})

	// --- Test 6: System Config (PUT /api/admin/config/:key) ---
	t.Run("Update System Config", func(t *testing.T) {
		body := map[string]string{"value": "test-value"}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("PUT", "/api/admin/config/test_key", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", rr.Code)
		}
	})

	// --- Test 6b: Get System Config (GET /api/admin/config) ---
	t.Run("Get System Config", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/admin/config?key=test_key", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", rr.Code)
		}
	})

	// --- Cleanup: Delete Course ---
	t.Run("Delete Course", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/admin/courses/"+createdCourseID, nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", rr.Code)
		}
	})
}
