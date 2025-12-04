package main

import (
	"context"
	"log"
	"net"
	"testing"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pb "stdiscm_p4/backend/pb/admin"
	"stdiscm_p4/backend/shared"
)

const bufSize = 1024 * 1024

var lis *bufconn.Listener

func initServer() *grpc.Server {
	if err := godotenv.Load(".env"); err != nil {
		log.Println("No .env file found, using defaults")
	}
	cfg, _ := shared.LoadServiceConfig("admin-service")
	client, db, _ := shared.ConnectMongoDB(&cfg.MongoDB) // Need client for transactions

	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()

	adminService := NewAdminService(client, db, cfg)
	pb.RegisterAdminServiceServer(s, adminService)

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited: %v", err)
		}
	}()
	return s
}

func bufDialer(context.Context, string) (net.Conn, error) { return lis.Dial() }

func TestAdminService_Integration(t *testing.T) {
	server := initServer()
	defer server.Stop()

	ctx := context.Background()
	conn, err := grpc.NewClient("passthrough://bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewAdminServiceClient(conn)

	// --- SETUP & CLEANUP ---
	cfg, _ := shared.LoadServiceConfig("admin-service")
	_, db, _ := shared.ConnectMongoDB(&cfg.MongoDB)

	// Test Data Constants
	testCourseCode := "TEST-FULL-101"
	testStudentEmail := "admin_test_student@example.com"
	testFacultyEmail := "admin_test_faculty@example.com"
	testAdminID := "admin-integration-test"

	// Helper to clean DB
	cleanup := func() {
		db.Collection("courses").DeleteOne(ctx, bson.M{"code": testCourseCode})
		db.Collection("users").DeleteOne(ctx, bson.M{"email": testStudentEmail})
		db.Collection("users").DeleteOne(ctx, bson.M{"email": testFacultyEmail})
		db.Collection("enrollments").DeleteMany(ctx, bson.M{"student_id": bson.M{"$regex": "^STU-"}}) // Clean up override enrollments
		db.Collection("system_config").DeleteMany(ctx, bson.M{})
	}

	cleanup()
	defer cleanup()

	// IDs to be captured during tests for subsequent steps
	var createdCourseID string
	var createdStudentID string
	var createdFacultyID string

	// ========================================================================
	// 1. User Management Tests
	// ========================================================================
	t.Run("Create Student User", func(t *testing.T) {
		resp, err := client.CreateUser(ctx, &pb.CreateUserRequest{
			Email:     testStudentEmail,
			Role:      "student",
			Name:      "Integration Student",
			StudentId: "STU-001",
			Major:     "CS",
			YearLevel: 1,
		})
		if err != nil {
			t.Fatalf("CreateUser (Student) failed: %v", err)
		}
		if !resp.Success {
			t.Errorf("CreateUser returned false: %s", resp.Message)
		}
		createdStudentID = resp.UserId
	})

	t.Run("Create Faculty User", func(t *testing.T) {
		resp, err := client.CreateUser(ctx, &pb.CreateUserRequest{
			Email:      testFacultyEmail,
			Role:       "faculty",
			Name:       "Integration Prof",
			FacultyId:  "FAC-001",
			Department: "Science",
		})
		if err != nil {
			t.Fatalf("CreateUser (Faculty) failed: %v", err)
		}
		createdFacultyID = resp.UserId
	})

	t.Run("List Users", func(t *testing.T) {
		resp, err := client.ListUsers(ctx, &pb.ListUsersRequest{
			Role:       "student",
			ActiveOnly: true,
		})
		if err != nil {
			t.Fatalf("ListUsers failed: %v", err)
		}
		found := false
		for _, u := range resp.Users {
			if u.Email == testStudentEmail {
				found = true
				break
			}
		}
		if !found {
			t.Error("Created student not found in ListUsers")
		}
	})

	t.Run("Toggle User Status", func(t *testing.T) {
		// Deactivate
		resp, err := client.ToggleUserStatus(ctx, &pb.ToggleUserStatusRequest{
			UserId:   createdStudentID,
			Activate: false,
		})
		if err != nil || !resp.Success {
			t.Errorf("ToggleUserStatus (Deactivate) failed: %v", err)
		}

		// Verify Deactivation via ListUsers (ActiveOnly=true)
		listResp, _ := client.ListUsers(ctx, &pb.ListUsersRequest{ActiveOnly: true})
		for _, u := range listResp.Users {
			if u.Id == createdStudentID {
				t.Error("Deactivated user still appears in active list")
			}
		}

		// Reactivate for later tests
		client.ToggleUserStatus(ctx, &pb.ToggleUserStatusRequest{UserId: createdStudentID, Activate: true})
	})

	t.Run("Reset Password", func(t *testing.T) {
		resp, err := client.ResetPassword(ctx, &pb.ResetPasswordRequest{
			UserId: createdStudentID,
		})
		if err != nil || !resp.Success {
			t.Errorf("ResetPassword failed: %v", err)
		}
		if resp.NewPassword == "" {
			t.Error("New password not returned")
		}
	})

	// ========================================================================
	// 2. Course Management Tests
	// ========================================================================
	t.Run("Create Course", func(t *testing.T) {
		resp, err := client.CreateCourse(ctx, &pb.CreateCourseRequest{
			Code:        testCourseCode,
			Title:       "Admin Test Course",
			Description: "Testing Admin RPCs",
			Units:       3,
			Schedule:    "MWF 10:00-11:00",
			Room:        "WEB",
			Capacity:    40,
			Semester:    "TestSem",
		})
		if err != nil || !resp.Success {
			t.Fatalf("CreateCourse failed: %v", err)
		}
		createdCourseID = resp.CourseId
	})

	t.Run("Update Course", func(t *testing.T) {
		resp, err := client.UpdateCourse(ctx, &pb.UpdateCourseRequest{
			CourseId: createdCourseID,
			Title:    "Updated Test Course",
			Capacity: 50,
			IsOpen:   true,
		})
		if err != nil || !resp.Success {
			t.Fatalf("UpdateCourse failed: %v", err)
		}
		if resp.Course.Title != "Updated Test Course" || resp.Course.Capacity != 50 {
			t.Error("Course updates not reflected in response")
		}
	})

	t.Run("Assign Faculty", func(t *testing.T) {
		resp, err := client.AssignFaculty(ctx, &pb.AssignFacultyRequest{
			CourseId:  createdCourseID,
			FacultyId: createdFacultyID,
		})
		if err != nil || !resp.Success {
			t.Fatalf("AssignFaculty failed: %v", err)
		}
	})

	// ========================================================================
	// 3. System Configuration Tests
	// ========================================================================
	t.Run("Set Enrollment Period", func(t *testing.T) {
		resp, err := client.SetEnrollmentPeriod(ctx, &pb.SetEnrollmentPeriodRequest{
			StartDate: "2024-01-01T00:00:00Z",
			EndDate:   "2024-02-01T00:00:00Z",
		})
		if err != nil || !resp.Success {
			t.Errorf("SetEnrollmentPeriod failed: %v", err)
		}
	})

	t.Run("Toggle Enrollment", func(t *testing.T) {
		resp, err := client.ToggleEnrollment(ctx, &pb.ToggleEnrollmentRequest{
			Enable: true,
		})
		if err != nil || !resp.Success || !resp.EnrollmentOpen {
			t.Errorf("ToggleEnrollment failed: %v", err)
		}
	})

	t.Run("General Config CRUD", func(t *testing.T) {
		// Update
		upResp, err := client.UpdateSystemConfig(ctx, &pb.UpdateSystemConfigRequest{
			Key:     "maintenance_mode",
			Value:   "false",
			AdminId: testAdminID,
		})
		if err != nil || !upResp.Success {
			t.Errorf("UpdateSystemConfig failed")
		}

		// Get
		getResp, err := client.GetSystemConfig(ctx, &pb.GetSystemConfigRequest{Key: "maintenance_mode"})
		if err != nil || len(getResp.Configs) == 0 {
			t.Errorf("GetSystemConfig failed")
		} else if getResp.Configs[0].Value != "false" {
			t.Errorf("Config mismatch")
		}
	})

	// ========================================================================
	// 4. Overrides & Deletion
	// ========================================================================
	t.Run("Override Enrollment (Force Enroll)", func(t *testing.T) {
		// Ensure Course and User exist from previous steps
		resp, err := client.OverrideEnrollment(ctx, &pb.OverrideEnrollmentRequest{
			StudentId: createdStudentID,
			CourseId:  createdCourseID,
			Action:    "force_enroll",
			Reason:    "Integration Test Override",
			AdminId:   testAdminID,
		})
		if err != nil || !resp.Success {
			t.Fatalf("OverrideEnrollment (Enroll) failed: %v", err)
		}
	})

	t.Run("Override Enrollment (Force Drop)", func(t *testing.T) {
		resp, err := client.OverrideEnrollment(ctx, &pb.OverrideEnrollmentRequest{
			StudentId: createdStudentID,
			CourseId:  createdCourseID,
			Action:    "force_drop",
			Reason:    "Integration Test Drop",
			AdminId:   testAdminID,
		})
		if err != nil || !resp.Success {
			t.Fatalf("OverrideEnrollment (Drop) failed: %v", err)
		}
	})

	t.Run("Get System Stats", func(t *testing.T) {
		resp, err := client.GetSystemStats(ctx, &pb.GetSystemStatsRequest{})
		if err != nil {
			t.Fatalf("GetSystemStats failed: %v", err)
		}
		if resp.Stats.TotalCourses == 0 {
			t.Error("Stats mismatch, expected courses")
		}
	})

	// Run Delete last since it destroys the resource
	t.Run("Delete Course", func(t *testing.T) {
		resp, err := client.DeleteCourse(ctx, &pb.DeleteCourseRequest{
			CourseId: createdCourseID,
		})
		if err != nil || !resp.Success {
			t.Errorf("DeleteCourse failed: %v", err)
		}
	})
}
