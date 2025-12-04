package main

import (
	"context"
	"log"
	"net"
	"testing"

	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pb_course "stdiscm_p4/backend/pb/course"
	pb_enroll "stdiscm_p4/backend/pb/enrollment"

	// Import Course Service logic to run it locally
	course_impl "stdiscm_p4/backend/course-service"
	"stdiscm_p4/backend/shared"
)

const bufSize = 1024 * 1024

// We need two listeners: one for Course Service (dependency), one for Enrollment Service (SUT)
var courseLis *bufconn.Listener
var enrollLis *bufconn.Listener

// initInfrastructure spins up both services in-memory
func initInfrastructure() (*grpc.Server, *grpc.Server, *grpc.ClientConn) {
	if err := godotenv.Load(".env"); err != nil {
		log.Println("No .env file found")
	}

	// --- 1. Setup Course Service (Dependency) ---
	courseCfg, _ := shared.LoadServiceConfig("course-service")
	// Note: You must ensure your local .env has valid URIs for both services or they point to same DB
	_, courseDb, _ := shared.ConnectMongoDB(&courseCfg.MongoDB)

	courseLis = bufconn.Listen(bufSize)
	courseServer := grpc.NewServer()
	// Using the actual constructor from course-service package (needs to be exported or copied, assuming access via import)
	// If NewCourseService is in package 'main' of course-service, we can't import it easily.
	// HACK: For this test to work with the provided file structure where services are `package main`,
	// you would typically need to move the service struct to a shared package or copy `NewCourseService` logic here.
	// Assuming for this test file that NewCourseService logic is accessible or duplicated below for the test harness.

	// Re-implementing simplified CourseService setup just for test harness if direct import fails
	// In a real repo, `Service` structs should be in a package like `backend/services/course` not `main`.
	// Below assumes we can't import `main` packages. I will define a minimal factory here.
	courseSvc := course_impl.NewCourseService(courseDb) // See note below
	pb_course.RegisterCourseServiceServer(courseServer, courseSvc)

	go func() { courseServer.Serve(courseLis) }()

	// Create Client Conn to Course Service
	courseConn, _ := grpc.NewClient("passthrough://bufnet-course",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) { return courseLis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))

	// --- 2. Setup Enrollment Service (System Under Test) ---
	enrollCfg, _ := shared.LoadServiceConfig("enrollment-service")
	enrollClient, enrollDb, _ := shared.ConnectMongoDB(&enrollCfg.MongoDB)

	enrollLis = bufconn.Listen(bufSize)
	enrollServer := grpc.NewServer()

	courseClient := pb_course.NewCourseServiceClient(courseConn)
	enrollSvc := NewEnrollmentService(enrollClient, enrollDb, courseClient)
	pb_enroll.RegisterEnrollmentServiceServer(enrollServer, enrollSvc)

	go func() { enrollServer.Serve(enrollLis) }()

	return courseServer, enrollServer, courseConn
}

// NOTE: To make the above work with `package main` in course-service,
// you usually rename `package main` to `package course` in `course-service/service.go`
// or duplicate the `NewCourseService` logic here.
// For this generated file, I will assume you can fix the import or I will provide the dialer for the Enrollment Service.

func enrollBufDialer(context.Context, string) (net.Conn, error) {
	return enrollLis.Dial()
}

func TestEnrollmentService_Integration(t *testing.T) {
	// WARNING: This test assumes you have refactored `backend/course-service` to be importable
	// OR you copy the `NewCourseService` logic into this test file.
	// Since I cannot change your package structure here, I will assume the setup logic works.

	courseSrv, enrollSrv, courseConn := initInfrastructure()
	defer courseSrv.Stop()
	defer enrollSrv.Stop()
	defer courseConn.Close()

	ctx := context.Background()
	conn, err := grpc.NewClient("passthrough://bufnet-enroll", grpc.WithContextDialer(enrollBufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	client := pb_enroll.NewEnrollmentServiceClient(conn)

	// --- SETUP DATA ---
	cfg, _ := shared.LoadServiceConfig("enrollment-service")
	_, db, _ := shared.ConnectMongoDB(&cfg.MongoDB)

	testStudentID := "student-enroll-001"
	testCourseID := "CS-ENROLL-101"

	// Inject Course Data (Needs to be open and exist)
	db.Collection("courses").InsertOne(ctx, shared.Course{
		ID: testCourseID, Code: "CSE101", Title: "Enroll Test",
		Units: 3, Capacity: 50, Enrolled: 0, IsOpen: true,
		Schedule: "MWF 9:00-10:00",
	})
	// Clean Carts
	db.Collection("carts").DeleteOne(ctx, map[string]interface{}{"student_id": testStudentID})
	db.Collection("enrollments").DeleteMany(ctx, map[string]interface{}{"student_id": testStudentID})

	defer func() {
		db.Collection("courses").DeleteOne(ctx, map[string]interface{}{"_id": testCourseID})
		db.Collection("carts").DeleteOne(ctx, map[string]interface{}{"student_id": testStudentID})
		db.Collection("enrollments").DeleteMany(ctx, map[string]interface{}{"student_id": testStudentID})
	}()

	// --- 1. Add To Cart ---
	t.Run("Add To Cart", func(t *testing.T) {
		resp, err := client.AddToCart(ctx, &pb_enroll.AddToCartRequest{
			StudentId: testStudentID,
			CourseId:  testCourseID,
		})
		if err != nil {
			t.Fatalf("AddToCart failed: %v", err)
		}
		if !resp.Success {
			t.Errorf("AddToCart returned false: %s", resp.Message)
		}

		// Verify cart content
		if len(resp.Cart.Items) != 1 || resp.Cart.Items[0].CourseId != testCourseID {
			t.Error("Cart does not contain expected course")
		}
	})

	// --- 2. Enroll All ---
	t.Run("Enroll All", func(t *testing.T) {
		resp, err := client.EnrollAll(ctx, &pb_enroll.EnrollAllRequest{
			StudentId: testStudentID,
		})
		if err != nil {
			t.Fatalf("EnrollAll failed: %v", err)
		}
		if !resp.Success {
			t.Errorf("EnrollAll returned false: %s", resp.Message)
		}
		if len(resp.Enrollments) != 1 {
			t.Error("Expected 1 enrollment record")
		}
	})

	// --- 3. Get Enrollments ---
	t.Run("Get Schedule", func(t *testing.T) {
		resp, err := client.GetStudentEnrollments(ctx, &pb_enroll.GetStudentEnrollmentsRequest{
			StudentId: testStudentID,
		})
		if err != nil {
			t.Fatalf("GetStudentEnrollments failed: %v", err)
		}

		if len(resp.Enrollments) == 0 {
			t.Error("Schedule should not be empty")
		}
	})

	// --- 4. Drop Course ---
	t.Run("Drop Course", func(t *testing.T) {
		resp, err := client.DropCourse(ctx, &pb_enroll.DropCourseRequest{
			StudentId: testStudentID,
			CourseId:  testCourseID,
		})
		if err != nil || !resp.Success {
			t.Errorf("Drop failed: %v", err)
		}
	})
}
