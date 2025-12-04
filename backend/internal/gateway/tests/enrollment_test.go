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

func TestGateway_Enrollment(t *testing.T) {
	env := setupGatewayTestEnv(t)
	ctx := context.Background()

	// 1. Create Student
	uResp, _ := env.AdminClient.CreateUser(ctx, &pb_admin.CreateUserRequest{
		Email: "student_enroll@test.com", Role: "student", Name: "Enroll Student", StudentId: "202100001",
	})

	// 2. Get Token
	lResp, _ := env.AuthClient.Login(ctx, &pb_auth.LoginRequest{
		Identifier: "student_enroll@test.com", Password: uResp.InitialPassword,
	})
	token := lResp.Token

	// 3. Create Course
	cResp, _ := env.AdminClient.CreateCourse(ctx, &pb_admin.CreateCourseRequest{
		Code: "ENR-101", Title: "Enrollment Test", Units: 3, Semester: "Sem1", Capacity: 50, Schedule: "MWF 10:00-11:00",
	})
	// Open the course
	env.AdminClient.UpdateCourse(ctx, &pb_admin.UpdateCourseRequest{
		CourseId: cResp.CourseId, IsOpen: true,
	})

	// --- Test 1: Add to Cart (POST /api/cart/add) ---
	t.Run("Add To Cart", func(t *testing.T) {
		body := map[string]string{"course_id": cResp.CourseId}
		jsonBody, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/cart/add", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d. Msg: %s", rr.Code, rr.Body.String())
		}
	})

	// --- Test 2: Get Cart (GET /api/cart) ---
	t.Run("Get Cart", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/cart", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", rr.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &resp)
		cart, ok := resp["cart"].(map[string]interface{})
		if !ok {
			t.Fatal("Response missing 'cart' object")
		}

		items, ok := cart["items"].([]interface{})
		if !ok || len(items) != 1 {
			t.Error("Expected 1 item in cart")
		}
	})

	// --- Test 3: Remove From Cart (DELETE /api/cart/remove/:course_id) ---
	t.Run("Remove From Cart", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/cart/remove/"+cResp.CourseId, nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", rr.Code)
		}

		// Verify empty
		reqGet, _ := http.NewRequest("GET", "/api/cart", nil)
		reqGet.Header.Set("Authorization", "Bearer "+token)
		rrGet := httptest.NewRecorder()
		env.Router.ServeHTTP(rrGet, reqGet)

		var resp map[string]interface{}
		json.Unmarshal(rrGet.Body.Bytes(), &resp)
		cart := resp["cart"].(map[string]interface{})

		// FIX: Safely check for items, avoiding panic if nil
		if itemsInterface := cart["items"]; itemsInterface != nil {
			items := itemsInterface.([]interface{})
			if len(items) != 0 {
				t.Error("Cart should be empty after remove")
			}
		}
	})

	// --- Test 4: Clear Cart (DELETE /api/cart/clear) ---
	t.Run("Clear Cart", func(t *testing.T) {
		// Add item back first
		body := map[string]string{"course_id": cResp.CourseId}
		jsonBody, _ := json.Marshal(body)
		reqAdd, _ := http.NewRequest("POST", "/api/cart/add", bytes.NewBuffer(jsonBody))
		reqAdd.Header.Set("Authorization", "Bearer "+token)
		reqAdd.Header.Set("Content-Type", "application/json")
		rrAdd := httptest.NewRecorder()
		env.Router.ServeHTTP(rrAdd, reqAdd)

		// Clear
		req, _ := http.NewRequest("DELETE", "/api/cart/clear", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", rr.Code)
		}
	})

	// --- Test 5: Enroll All (POST /api/enrollment/enroll-all) ---
	t.Run("Enroll All", func(t *testing.T) {
		// Add item back first to enroll
		body := map[string]string{"course_id": cResp.CourseId}
		jsonBody, _ := json.Marshal(body)
		reqAdd, _ := http.NewRequest("POST", "/api/cart/add", bytes.NewBuffer(jsonBody))
		reqAdd.Header.Set("Authorization", "Bearer "+token)
		reqAdd.Header.Set("Content-Type", "application/json")
		rrAdd := httptest.NewRecorder()
		env.Router.ServeHTTP(rrAdd, reqAdd)

		// Enroll
		req, _ := http.NewRequest("POST", "/api/enrollment/enroll-all", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d. Msg: %s", rr.Code, rr.Body.String())
		}
	})

	// --- Test 6: Get Schedule (GET /api/enrollment/schedule) ---
	t.Run("Get Schedule", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/enrollment/schedule?semester=Sem1", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", rr.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &resp)
		enrollments, ok := resp["enrollments"].([]interface{})
		if !ok || len(enrollments) != 1 {
			t.Error("Expected 1 enrolled course")
		}
	})

	// --- Test 7: Drop Course (POST /api/enrollment/drop) ---
	t.Run("Drop Course", func(t *testing.T) {
		body := map[string]string{"course_id": cResp.CourseId}
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", "/api/enrollment/drop", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		env.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d. Msg: %s", rr.Code, rr.Body.String())
		}
	})
}
