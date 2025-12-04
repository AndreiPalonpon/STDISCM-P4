// ============================================================================
// backend/course-service/main.go
// Entry point for the Course Service (Refactored)
// ============================================================================

package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"stdiscm_p4/backend/internal/course"
	pb "stdiscm_p4/backend/internal/pb/course"
	"stdiscm_p4/backend/internal/shared"
)

func main() {
	// Load environment variables
	if err := shared.LoadEnv(".env"); err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	// Load service configuration
	config, err := shared.LoadServiceConfig("course-service")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Override with service-specific env vars if present
	config.ServicePort = shared.GetEnv("COURSE_SERVICE_PORT", shared.DefaultCourseServicePort)
	config.MongoDB.URI = shared.GetEnv("COURSE_MONGO_URI", config.MongoDB.URI)

	// Validate configuration
	if err := shared.ValidateServiceConfig(config); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Print configuration in development mode
	if shared.IsDevelopment(config) {
		shared.PrintConfig(config)
	}

	// Connect to MongoDB Atlas
	mongoClient, db, err := shared.ConnectMongoDB(&config.MongoDB)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer func() {
		if err := shared.DisconnectMongoDB(mongoClient); err != nil {
			log.Printf("Error disconnecting from MongoDB: %v", err)
		}
	}()

	// Create gRPC server with configuration
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(config.GRPC.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(config.GRPC.MaxSendMsgSize),
	)

	// Initialize and register Course Service
	courseService := course.NewCourseService(db)
	pb.RegisterCourseServiceServer(grpcServer, courseService)

	// Register health check service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("course.CourseService", grpc_health_v1.HealthCheckResponse_SERVING)

	// Register reflection service (useful for debugging with grpcurl)
	reflection.Register(grpcServer)

	// Start listening
	listener, err := net.Listen("tcp", ":"+config.ServicePort)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", config.ServicePort, err)
	}

	// Graceful shutdown handling
	go func() {
		log.Printf("Course Service is listening on port %s", config.ServicePort)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Course Service...")

	// Set health check to NOT_SERVING
	healthServer.SetServingStatus("course.CourseService", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	// Graceful shutdown
	grpcServer.GracefulStop()

	log.Println("Course Service stopped")
}
