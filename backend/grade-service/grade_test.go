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

	pb "stdiscm_p4/backend/pb/grade"
	"stdiscm_p4/backend/shared"
)

const bufSize = 1024 * 1024

var lis *bufconn.Listener

func initServer() *grpc.Server {
	if err := godotenv.Load(".env"); err != nil {
		log.Println("No .env file found, using defaults")
	}
	cfg, _ := shared.LoadServiceConfig("grade-service")
	// Use the shared connector
	_, db, _ := shared.ConnectMongoDB(&cfg.MongoDB)

	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()

	gradeService := NewGradeService(db)
	pb.RegisterGradeServiceServer(s, gradeService)

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited: %v", err)
		}
	}()
	return s
}

func bufDialer(context.Context, string) (net.Conn, error) { return lis.Dial() }

func TestGradeService_Integration(t *testing.T) {
	server := initServer()
	defer server.Stop()

	ctx := context.Background()
	conn, err := grpc.NewClient("passthrough://bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewGradeServiceClient(conn)

	// --- SETUP DATA ---
	cfg, _ := shared.LoadServiceConfig("grade-service")
	_, db, _ := shared.ConnectMongoDB(&cfg.MongoDB)

	// Test Data Constants
	testCourseID := "CS-GRADE-TEST-101"
	testStudentID1 := "student-grade-001"
	testStudentID2 := "student-grade-002"
	testFacultyID := "faculty-grade-001"
	enrollmentID1 := "ENR-TEST-001"
	enrollmentID2 := "ENR-TEST-002"

	// Cleanup Helper
	cleanup := func() {
		db.Collection("courses").DeleteOne(ctx, bson.M{"_id": testCourseID})
		db.Collection("users").DeleteMany(ctx, bson.M{"_id": bson.M{"$in": []string{testFacultyID, testStudentID1, testStudentID2}}})
		db.Collection("enrollments").DeleteMany(ctx, bson.M{"_id": bson.M{"$in": []string{enrollmentID1, enrollmentID2}}})
		db.Collection("grades").DeleteMany(ctx, bson.M{"course_id": testCourseID})
	}

	cleanup()
	defer cleanup()

	// 1. Insert Dependencies (Users, Course, Enrollments) directly into DB
	// We need these because GradeService validates existence of these entities
	_, err = db.Collection("courses").InsertOne(ctx, shared.Course{
		ID: testCourseID, Code: "CSG101", Title: "Grade Integration Test",
		FacultyID: testFacultyID, Units: 3, Semester: "TestSem",
	})
	if err != nil {
		t.Fatalf("Setup failed (course): %v", err)
	}

	_, err = db.Collection("users").InsertMany(ctx, []interface{}{
		shared.User{ID: testFacultyID, Role: "faculty", Name: "Prof Test", IsActive: true},
		shared.User{ID: testStudentID1, Role: "student", Name: "Student One", IsActive: true},
		shared.User{ID: testStudentID2, Role: "student", Name: "Student Two", IsActive: true},
	})
	if err != nil {
		t.Fatalf("Setup failed (users): %v", err)
	}

	_, err = db.Collection("enrollments").InsertMany(ctx, []interface{}{
		shared.Enrollment{ID: enrollmentID1, StudentID: testStudentID1, CourseID: testCourseID, Status: "enrolled"},
		shared.Enrollment{ID: enrollmentID2, StudentID: testStudentID2, CourseID: testCourseID, Status: "enrolled"},
	})
	if err != nil {
		t.Fatalf("Setup failed (enrollments): %v", err)
	}

	// ========================================================================
	// Test 1: Upload Grades (Streaming RPC)
	// ========================================================================
	t.Run("Upload Grades", func(t *testing.T) {
		stream, err := client.UploadGrades(ctx)
		if err != nil {
			t.Fatalf("Failed to open stream: %v", err)
		}

		// A. Send Metadata (First Message)
		err = stream.Send(&pb.UploadGradeEntryRequest{
			Payload: &pb.UploadGradeEntryRequest_Metadata{
				Metadata: &pb.UploadMetadata{CourseId: testCourseID, FacultyId: testFacultyID},
			},
		})
		if err != nil {
			t.Fatalf("Failed to send metadata: %v", err)
		}

		// B. Send Entry 1 (Student 1 -> A)
		err = stream.Send(&pb.UploadGradeEntryRequest{
			Payload: &pb.UploadGradeEntryRequest_Entry{
				Entry: &pb.GradeEntry{StudentId: testStudentID1, Grade: "A"},
			},
			IsLast: false,
		})
		if err != nil {
			t.Fatalf("Failed to send entry 1: %v", err)
		}

		// C. Send Entry 2 (Student 2 -> B)
		err = stream.Send(&pb.UploadGradeEntryRequest{
			Payload: &pb.UploadGradeEntryRequest_Entry{
				Entry: &pb.GradeEntry{StudentId: testStudentID2, Grade: "B"},
			},
			IsLast: true, // End of stream marker logic if implemented, or just close stream
		})
		if err != nil {
			t.Fatalf("Failed to send entry 2: %v", err)
		}

		// D. Close and Recv
		resp, err := stream.CloseAndRecv()
		if err != nil {
			t.Fatalf("CloseAndRecv failed: %v", err)
		}

		if !resp.Success {
			t.Errorf("Upload failed: %v", resp.Errors)
		}
		if resp.TotalProcessed != 2 || resp.Successful != 2 {
			t.Errorf("Expected 2 successful uploads, got %d", resp.Successful)
		}
	})

	// ========================================================================
	// Test 2: Get Course Grades (Faculty View - Before Publish)
	// ========================================================================
	t.Run("Get Course Grades (Unpublished)", func(t *testing.T) {
		resp, err := client.GetCourseGrades(ctx, &pb.GetCourseGradesRequest{
			CourseId:  testCourseID,
			FacultyId: testFacultyID,
		})
		if err != nil {
			t.Fatalf("GetCourseGrades failed: %v", err)
		}

		if len(resp.Grades) != 2 {
			t.Errorf("Expected 2 grades, got %d", len(resp.Grades))
		}
		if resp.AllPublished {
			t.Error("Grades should NOT be published yet")
		}
	})

	// ========================================================================
	// Test 3: Get Student Grades (Student View - Before Publish)
	// ========================================================================
	t.Run("Get Student Grades (Hidden)", func(t *testing.T) {
		// Should return empty list because not published
		resp, err := client.GetStudentGrades(ctx, &pb.GetStudentGradesRequest{
			StudentId: testStudentID1,
		})
		if err != nil {
			t.Fatalf("GetStudentGrades failed: %v", err)
		}
		if len(resp.Grades) != 0 {
			t.Error("Student should not see unpublished grades")
		}
	})

	// ========================================================================
	// Test 4: Publish Grades
	// ========================================================================
	t.Run("Publish Grades", func(t *testing.T) {
		resp, err := client.PublishGrades(ctx, &pb.PublishGradesRequest{
			CourseId:  testCourseID,
			FacultyId: testFacultyID,
		})
		if err != nil {
			t.Fatalf("PublishGrades failed: %v", err)
		}
		if !resp.Success {
			t.Errorf("PublishGrades returned success=false: %s", resp.Message)
		}
		if resp.GradesPublished != 2 {
			t.Errorf("Expected 2 grades published, got %d", resp.GradesPublished)
		}
	})

	// ========================================================================
	// Test 5: Get Student Grades & GPA (After Publish)
	// ========================================================================
	t.Run("Get Student Grades & GPA (Visible)", func(t *testing.T) {
		resp, err := client.GetStudentGrades(ctx, &pb.GetStudentGradesRequest{
			StudentId: testStudentID1,
		})
		if err != nil {
			t.Fatalf("GetStudentGrades failed: %v", err)
		}

		if len(resp.Grades) != 1 {
			t.Fatalf("Expected 1 grade, got %d", len(resp.Grades))
		}
		if resp.Grades[0].Grade != "A" {
			t.Errorf("Expected Grade A, got %s", resp.Grades[0].Grade)
		}

		// Verify embedded GPA calculation
		// Student 1 has 3 units of 'A' (4.0). GPA should be 4.0
		if resp.GpaInfo.Cgpa != 4.0 {
			t.Errorf("Expected CGPA 4.0, got %f", resp.GpaInfo.Cgpa)
		}
	})

	// ========================================================================
	// Test 6: Calculate GPA (Direct Call)
	// ========================================================================
	t.Run("Calculate GPA", func(t *testing.T) {
		// Student 2 has 3 units of 'B' (3.0). GPA should be 3.0
		resp, err := client.CalculateGPA(ctx, &pb.CalculateGPARequest{
			StudentId: testStudentID2,
		})
		if err != nil {
			t.Fatalf("CalculateGPA failed: %v", err)
		}
		if !resp.Success {
			t.Error("CalculateGPA returned success=false")
		}
		if resp.GpaInfo.TermGpa != 3.0 {
			t.Errorf("Expected Term GPA 3.0 for Student 2, got %f", resp.GpaInfo.TermGpa)
		}
	})

	// ========================================================================
	// Test 7: Get Class Roster
	// ========================================================================
	t.Run("Get Class Roster", func(t *testing.T) {
		resp, err := client.GetClassRoster(ctx, &pb.GetClassRosterRequest{
			CourseId: testCourseID,
		})
		if err != nil {
			t.Fatalf("GetClassRoster failed: %v", err)
		}

		if resp.TotalStudents != 2 {
			t.Errorf("Expected 2 students in roster, got %d", resp.TotalStudents)
		}

		// Verify we can see the grades in the roster (since they are uploaded)
		foundA := false
		foundB := false
		for _, s := range resp.Students {
			if s.StudentId == testStudentID1 && s.Grade == "A" {
				foundA = true
			}
			if s.StudentId == testStudentID2 && s.Grade == "B" {
				foundB = true
			}
		}
		if !foundA || !foundB {
			t.Error("Roster did not contain expected students with grades")
		}
	})
}
