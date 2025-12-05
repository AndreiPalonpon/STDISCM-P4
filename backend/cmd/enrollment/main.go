// ============================================================================
// backend/enrollment-service/main.go
// Entry point for the Enrollment Service
// ============================================================================

package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"stdiscm_p4/backend/internal/enrollment"
	pb_course "stdiscm_p4/backend/internal/pb/course"
	pb "stdiscm_p4/backend/internal/pb/enrollment"
	"stdiscm_p4/backend/internal/shared"
)

func main() {
	// Load environment variables
	if err := shared.LoadEnv(".env"); err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	// Load service configuration
	config, err := shared.LoadServiceConfig("enrollment-service")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Override with service-specific env vars
	config.ServicePort = shared.GetEnv("ENROLLMENT_SERVICE_PORT", shared.DefaultEnrollmentServicePort)
	config.MongoDB.URI = shared.GetEnv("ENROLLMENT_MONGO_URI", config.MongoDB.URI)

	// Validate configuration
	if err := shared.ValidateServiceConfig(config); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Connect to MongoDB
	mongoClient, db, err := shared.ConnectMongoDB(&config.MongoDB)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer func() {
		if err := shared.DisconnectMongoDB(mongoClient); err != nil {
			log.Printf("Error disconnecting from MongoDB: %v", err)
		}
	}()

	// ========================================================================
	// Initialize Client Connection to Course Service
	// Required for checking prerequisites and course details
	// ========================================================================
	courseServiceAddr := shared.GetEnv("COURSE_SERVICE_ADDR", "localhost:50052")
	courseConn, err := grpc.NewClient(
		courseServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect to Course Service: %v", err)
	}
	defer courseConn.Close()
	courseClient := pb_course.NewCourseServiceClient(courseConn)

	// Create gRPC server
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(config.GRPC.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(config.GRPC.MaxSendMsgSize),
	)

	// Initialize and register Enrollment Service
	// We pass mongoClient for transactions, db for collections, and courseClient for inter-service calls
	enrollmentService := enrollment.NewEnrollmentService(mongoClient, db, courseClient)
	pb.RegisterEnrollmentServiceServer(grpcServer, enrollmentService)

	// Register health check service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("enrollment.EnrollmentService", grpc_health_v1.HealthCheckResponse_SERVING)

	// Register reflection service
	reflection.Register(grpcServer)

	// Start listening
	listener, err := net.Listen("tcp", ":"+config.ServicePort)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", config.ServicePort, err)
	}

	// Graceful shutdown handling
	go func() {
		log.Printf("Enrollment Service is listening on port %s", config.ServicePort)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Enrollment Service...")
	healthServer.SetServingStatus("enrollment.EnrollmentService", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	grpcServer.GracefulStop()
	log.Println("Enrollment Service stopped")
}
