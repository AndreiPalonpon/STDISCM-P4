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

	pb "stdiscm_p4/backend/pb/grade"
	"stdiscm_p4/backend/shared"
)

func main() {
	// 1. Load Configuration using Shared Package
	// Ensure your .env uses keys: SERVICE_PORT, MONGO_URI, MONGO_DB_NAME
	cfg, err := shared.LoadServiceConfig(".env")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// 2. Connect to MongoDB using Shared Package
	client, db, err := shared.ConnectMongoDB(&cfg.MongoDB)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	// 3. Create gRPC Server with config
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(cfg.GRPC.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(cfg.GRPC.MaxSendMsgSize),
	)

	// 4. Initialize and Register Grade Service
	gradeService := NewGradeService(db)
	pb.RegisterGradeServiceServer(grpcServer, gradeService)

	// 5. Register Health Check
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("grade.GradeService", grpc_health_v1.HealthCheckResponse_SERVING)

	// 6. Register Reflection
	reflection.Register(grpcServer)

	// 7. Start Listening
	listener, err := net.Listen("tcp", ":"+cfg.ServicePort)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", cfg.ServicePort, err)
	}

	// 8. Graceful Shutdown Handling
	go func() {
		log.Printf("Grade Service is listening on port %s", cfg.ServicePort)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Grade Service...")

	healthServer.SetServingStatus("grade.GradeService", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	grpcServer.GracefulStop()

	// Use shared disconnect helper
	if err := shared.DisconnectMongoDB(client); err != nil {
		log.Printf("Error disconnecting from MongoDB: %v", err)
	}

	log.Println("Grade Service stopped")
}
