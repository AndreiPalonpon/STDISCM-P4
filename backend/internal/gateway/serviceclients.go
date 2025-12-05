package gateway

import (
	"context"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb_admin "stdiscm_p4/backend/internal/pb/admin"
	pb_auth "stdiscm_p4/backend/internal/pb/auth"
	pb_course "stdiscm_p4/backend/internal/pb/course"
	pb_enrollment "stdiscm_p4/backend/internal/pb/enrollment"
	pb_grade "stdiscm_p4/backend/internal/pb/grade"
)

// ServiceClients holds all gRPC clients for the backend services.
// This struct will be injected into your request handlers in main.go.
type ServiceClients struct {
	AuthClient       pb_auth.AuthServiceClient
	CourseClient     pb_course.CourseServiceClient
	EnrollmentClient pb_enrollment.EnrollmentServiceClient
	GradeClient      pb_grade.GradeServiceClient
	AdminClient      pb_admin.AdminServiceClient

	// Keep connections to close them later when the gateway shuts down
	conns []*grpc.ClientConn
}

// MustConnectGRPC establishes a connection to a gRPC server or panics.
// We use insecure credentials here as per the architecture (internal node communication).
func MustConnectGRPC(addr string) *grpc.ClientConn {
	log.Printf("INFO: Connecting to gRPC service at %s...", addr)

	// We use WithBlock() to ensure the connection is established before proceeding.
	// This makes the gateway fail fast if a backend service is down at startup.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)

	if err != nil {
		log.Fatalf("FATAL: Failed to connect to gRPC server at %s: %v", addr, err)
	}

	log.Printf("INFO: Successfully connected to %s", addr)
	return conn
}

// NewServiceClients initializes all gRPC clients.
// It reads addresses from environment variables or uses default local ports defined in the Architecture doc.
func NewServiceClients() *ServiceClients {
	// 1. Define Service Addresses (Env var or Default)
	authAddr := GetEnv("AUTH_SERVICE_ADDR", "localhost:50051")
	courseAddr := GetEnv("COURSE_SERVICE_ADDR", "localhost:50052")
	enrollmentAddr := GetEnv("ENROLLMENT_SERVICE_ADDR", "localhost:50053")
	gradeAddr := GetEnv("GRADE_SERVICE_ADDR", "localhost:50054")
	adminAddr := GetEnv("ADMIN_SERVICE_ADDR", "localhost:50055")

	// 2. Establish Connections
	authConn := MustConnectGRPC(authAddr)
	courseConn := MustConnectGRPC(courseAddr)
	enrollmentConn := MustConnectGRPC(enrollmentAddr)
	gradeConn := MustConnectGRPC(gradeAddr)
	adminConn := MustConnectGRPC(adminAddr)

	// 3. Create Clients and return the struct
	return &ServiceClients{
		AuthClient:       pb_auth.NewAuthServiceClient(authConn),
		CourseClient:     pb_course.NewCourseServiceClient(courseConn),
		EnrollmentClient: pb_enrollment.NewEnrollmentServiceClient(enrollmentConn),
		GradeClient:      pb_grade.NewGradeServiceClient(gradeConn),
		AdminClient:      pb_admin.NewAdminServiceClient(adminConn),
		conns:            []*grpc.ClientConn{authConn, courseConn, enrollmentConn, gradeConn, adminConn},
	}
}

// Close closes all underlying gRPC connections.
// Should be called via defer in main().
func (sc *ServiceClients) Close() {
	for _, conn := range sc.conns {
		if err := conn.Close(); err != nil {
			log.Printf("WARN: Error closing gRPC connection: %v", err)
		}
	}
}

// Helper to read env vars with a default fallback
func GetEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
