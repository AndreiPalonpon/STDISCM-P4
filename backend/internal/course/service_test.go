package course

import (
	"context"
	"log"
	"net"
	"testing"

	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pb "stdiscm_p4/backend/internal/pb/course"
	"stdiscm_p4/backend/internal/shared"
)

const bufSize = 1024 * 1024

var lis *bufconn.Listener

func initServer() *grpc.Server {
	if err := godotenv.Load("../../cmd/course/.env"); err != nil {
		log.Println("No .env file found")
	}
	cfg, _ := shared.LoadServiceConfig("course-service")
	_, db, _ := shared.ConnectMongoDB(&cfg.MongoDB)

	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()

	courseService := NewCourseService(db)
	pb.RegisterCourseServiceServer(s, courseService)

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited: %v", err)
		}
	}()
	return s
}

func bufDialer(context.Context, string) (net.Conn, error) { return lis.Dial() }

func TestCourseService_Integration(t *testing.T) {
	server := initServer()
	defer server.Stop()

	ctx := context.Background()
	conn, err := grpc.NewClient("passthrough://bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewCourseServiceClient(conn)

	// Inject Test Data
	cfg, _ := shared.LoadServiceConfig("course-service")
	_, db, _ := shared.ConnectMongoDB(&cfg.MongoDB)

	testCourseID := "CS-TEST-101"
	testCourse := shared.Course{
		ID:       testCourseID,
		Code:     "CS-TEST",
		Title:    "Test Course",
		Units:    3,
		Capacity: 30,
		Enrolled: 0,
		IsOpen:   true,
		Semester: "TestSem",
	}

	db.Collection("courses").DeleteOne(ctx, map[string]interface{}{"_id": testCourseID})
	defer db.Collection("courses").DeleteOne(ctx, map[string]interface{}{"_id": testCourseID})
	db.Collection("courses").InsertOne(ctx, testCourse)

	// --- 1. List Courses ---
	t.Run("List Courses", func(t *testing.T) {
		resp, err := client.ListCourses(ctx, &pb.ListCoursesRequest{
			Filters: &pb.CourseFilter{Semester: "TestSem"},
		})
		if err != nil {
			t.Fatalf("ListCourses failed: %v", err)
		}
		found := false
		for _, c := range resp.Courses {
			if c.Id == testCourseID {
				found = true
				break
			}
		}
		if !found {
			t.Error("Inserted test course not found in list")
		}
	})

	// --- 2. Get Course ---
	t.Run("Get Course", func(t *testing.T) {
		resp, err := client.GetCourse(ctx, &pb.GetCourseRequest{CourseId: testCourseID})
		if err != nil {
			t.Fatalf("GetCourse failed: %v", err)
		}
		if !resp.Success || resp.Course.Title != "Test Course" {
			t.Error("Failed to retrieve correct course details")
		}
	})

	// --- 3. Get Availability ---
	t.Run("Availability", func(t *testing.T) {
		resp, err := client.GetCourseAvailability(ctx, &pb.GetCourseAvailabilityRequest{CourseId: testCourseID})
		if err != nil {
			t.Fatalf("Availability check failed: %v", err)
		}
		if !resp.Available || resp.SeatsRemaining != 30 {
			t.Error("Incorrect availability calculation")
		}
	})

	// --- 4. Check Prerequisites (No prereqs case) ---
	t.Run("Check Prereqs", func(t *testing.T) {
		resp, err := client.CheckPrerequisites(ctx, &pb.CheckPrerequisitesRequest{
			StudentId: "some-student",
			CourseId:  testCourseID,
		})
		if err != nil {
			t.Fatalf("CheckPrerequisites failed: %v", err)
		}
		if !resp.AllMet {
			t.Error("Should meet prereqs for course with no prereqs")
		}
	})
}
