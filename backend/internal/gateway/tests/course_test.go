package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	pb_admin "stdiscm_p4/backend/internal/pb/admin"
	pb_auth "stdiscm_p4/backend/internal/pb/auth"
)

func TestGateway_Course(t *testing.T) {
	env := setupGatewayTestEnv(t)
	ctx := context.Background()

	// Setup: Create a course via Admin Service directly
	// Note: Removed IsOpen: true because it is not in CreateCourseRequest
	cResp, err := env.AdminClient.CreateCourse(ctx, &pb_admin.CreateCourseRequest{
		Code:     "REST-101",
		Title:    "Rest API Testing",
		Units:    3,
		Semester: "TestSem",
		Capacity: 50,                // FIX: Added capacity (Required: 5-100)
		Schedule: "MWF 10:00-11:00", // Added schedule for completeness
	})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	// FIX: Explicitly check success to catch validation errors (like invalid capacity)
	if !cResp.Success {
		t.Fatalf("Setup CreateCourse failed: %s", cResp.Message)
	}
	testCourseID := cResp.CourseId

	// Setup: Open the course explicitly (required for availability tests)
	_, err = env.AdminClient.UpdateCourse(ctx, &pb_admin.UpdateCourseRequest{
		CourseId: testCourseID,
		IsOpen:   true,
	})
	if err != nil {
		t.Fatalf("Failed to open course: %v", err)
	}

	// Setup: Create Student and get token for protected routes
	uResp, _ := env.AdminClient.CreateUser(ctx, &pb_admin.CreateUserRequest{
		Email: "course_student@test.com", Role: "student", Name: "Course Student", StudentId: "CS001",
	})
	lResp, _ := env.AuthClient.Login(ctx, &pb_auth.LoginRequest{
		Identifier: "course_student@test.com", Password: uResp.InitialPassword,
	})
	studentToken := lResp.Token

	// --- Test 1: List Courses (Public) (GET /api/courses) ---
	t.Run("List Courses Public", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/courses?semester=TestSem", nil)
		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", rr.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &resp)

		// Verify data structure
		courses, ok := resp["courses"].([]interface{})
		if !ok || len(courses) == 0 {
			t.Error("Expected courses list in response")
		}
	})

	// --- Test 2: Get Course (Public) (GET /api/courses/:id) ---
	t.Run("Get Course Public", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/courses/"+testCourseID, nil)
		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", rr.Code)
		}
		var resp map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &resp)
		if _, ok := resp["course"]; !ok {
			t.Error("Response missing 'course' object")
		}
	})

	// --- Test 3: Get Course Availability (Public) (GET /api/courses/:id/availability) ---
	t.Run("Get Availability", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/courses/"+testCourseID+"/availability", nil)
		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", rr.Code)
		}
	})

	// --- Test 4: Check Prerequisites (Protected) (GET /api/courses/:id/prerequisites) ---
	t.Run("Check Prerequisites", func(t *testing.T) {
		// Requires student token
		req, _ := http.NewRequest("GET", "/api/courses/"+testCourseID+"/prerequisites?student_id=CS001", nil)
		req.Header.Set("Authorization", "Bearer "+studentToken)

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
		}
	})
}
