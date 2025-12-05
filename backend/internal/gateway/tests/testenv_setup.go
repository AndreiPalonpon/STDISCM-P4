package tests

import (
	"context"
	"log"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	// Gateway Implementation
	"stdiscm_p4/backend/internal/gateway"

	// Backend Service Implementations
	admin_svc "stdiscm_p4/backend/internal/admin"
	auth_svc "stdiscm_p4/backend/internal/auth"
	course_svc "stdiscm_p4/backend/internal/course"
	enroll_svc "stdiscm_p4/backend/internal/enrollment"
	grade_svc "stdiscm_p4/backend/internal/grade"

	// Protobufs
	pb_admin "stdiscm_p4/backend/internal/pb/admin"
	pb_auth "stdiscm_p4/backend/internal/pb/auth"
	pb_course "stdiscm_p4/backend/internal/pb/course"
	pb_enroll "stdiscm_p4/backend/internal/pb/enrollment"
	pb_grade "stdiscm_p4/backend/internal/pb/grade"

	"stdiscm_p4/backend/internal/shared"
)

const bufSize = 1024 * 1024

// TestEnv holds all the running components for the test
type TestEnv struct {
	Router           http.Handler
	AuthClient       pb_auth.AuthServiceClient
	CourseClient     pb_course.CourseServiceClient
	EnrollmentClient pb_enroll.EnrollmentServiceClient
	GradeClient      pb_grade.GradeServiceClient
	AdminClient      pb_admin.AdminServiceClient
}

// setupGatewayTestEnv spins up the entire backend stack in-memory
func setupGatewayTestEnv(t *testing.T) *TestEnv {
	// Load Environment (adjust path relative to tests folder: backend/internal/gateway/tests -> backend/.env)
	if err := godotenv.Load(".env"); err != nil {
		log.Println("Note: No root .env file found, using defaults")
	}

	// --- 1. Prepare Shared MongoDB Connections ---
	mongoURI := shared.GetEnv("MONGO_URI", "mongodb://localhost:27017")
	dbName := "P4DB"

	cfg := &shared.MongoConfig{
		URI:            mongoURI,
		Database:       dbName,
		ConnectTimeout: 30 * time.Second,
		MaxPoolSize:    10,
		MinPoolSize:    1,
		MaxIdleTime:    60 * time.Second,
	}
	client, db, err := shared.ConnectMongoDB(cfg)
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	// Clean DB before starting
	db.Drop(context.Background())

	// Helper to create a bufconn server
	createService := func() (*grpc.Server, *bufconn.Listener) {
		lis := bufconn.Listen(bufSize)
		s := grpc.NewServer()
		return s, lis
	}

	// --- 2. Initialize Backend Services ---

	// Course Service (Initialize FIRST as Enrollment depends on it)
	sCourse, lCourse := createService()
	courseSvc := course_svc.NewCourseService(db)
	pb_course.RegisterCourseServiceServer(sCourse, courseSvc)
	go func() { sCourse.Serve(lCourse) }()

	// Auth Service
	sAuth, lAuth := createService()
	authCfg := &shared.ServiceConfig{Security: shared.SecurityConfig{JWTSecret: "test-secret", JWTExpirationHours: 1, BCryptCost: 4}}
	pb_auth.RegisterAuthServiceServer(sAuth, auth_svc.NewAuthService(db, authCfg))
	go func() { sAuth.Serve(lAuth) }()

	// Enrollment Service (Needs Course Client!)
	sEnroll, lEnroll := createService()
	// Create internal client connection to Course Service
	courseConnInternal, _ := grpc.NewClient("passthrough://bufnet", grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) { return lCourse.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	pb_enroll.RegisterEnrollmentServiceServer(sEnroll, enroll_svc.NewEnrollmentService(client, db, pb_course.NewCourseServiceClient(courseConnInternal)))
	go func() { sEnroll.Serve(lEnroll) }()

	// Grade Service
	sGrade, lGrade := createService()
	pb_grade.RegisterGradeServiceServer(sGrade, grade_svc.NewGradeService(db))
	go func() { sGrade.Serve(lGrade) }()

	// Admin Service
	sAdmin, lAdmin := createService()
	adminCfg := &shared.ServiceConfig{Security: shared.SecurityConfig{BCryptCost: 4}}
	pb_admin.RegisterAdminServiceServer(sAdmin, admin_svc.NewAdminService(client, db, adminCfg))
	go func() { sAdmin.Serve(lAdmin) }()

	// --- 3. Connect Gateway to Backends ---

	dialer := func(lis *bufconn.Listener) func(context.Context, string) (net.Conn, error) {
		return func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}
	}

	connAuth, _ := grpc.NewClient("passthrough://bufnet", grpc.WithContextDialer(dialer(lAuth)), grpc.WithTransportCredentials(insecure.NewCredentials()))
	connCourse, _ := grpc.NewClient("passthrough://bufnet", grpc.WithContextDialer(dialer(lCourse)), grpc.WithTransportCredentials(insecure.NewCredentials()))
	connEnroll, _ := grpc.NewClient("passthrough://bufnet", grpc.WithContextDialer(dialer(lEnroll)), grpc.WithTransportCredentials(insecure.NewCredentials()))
	connGrade, _ := grpc.NewClient("passthrough://bufnet", grpc.WithContextDialer(dialer(lGrade)), grpc.WithTransportCredentials(insecure.NewCredentials()))
	connAdmin, _ := grpc.NewClient("passthrough://bufnet", grpc.WithContextDialer(dialer(lAdmin)), grpc.WithTransportCredentials(insecure.NewCredentials()))

	serviceClients := &gateway.ServiceClients{
		AuthClient:       pb_auth.NewAuthServiceClient(connAuth),
		CourseClient:     pb_course.NewCourseServiceClient(connCourse),
		EnrollmentClient: pb_enroll.NewEnrollmentServiceClient(connEnroll),
		GradeClient:      pb_grade.NewGradeServiceClient(connGrade),
		AdminClient:      pb_admin.NewAdminServiceClient(connAdmin),
	}

	// --- 4. Initialize Gateway Router ---
	router := gateway.SetupRoutes(serviceClients)

	return &TestEnv{
		Router:           router,
		AuthClient:       serviceClients.AuthClient,
		AdminClient:      serviceClients.AdminClient,
		CourseClient:     serviceClients.CourseClient,
		EnrollmentClient: serviceClients.EnrollmentClient,
		GradeClient:      serviceClients.GradeClient,
	}
}
