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

	// Ensure this path matches your go.mod module name
	"stdiscm_p4/backend/internal/auth"
	pb "stdiscm_p4/backend/internal/pb/auth"
	"stdiscm_p4/backend/internal/shared"
)

func main() {
	// Load environment variables
	if err := shared.LoadEnv(".env"); err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	// 1. Load Configuration (validates JWT_SECRET is present)
	cfg, err := shared.LoadServiceConfig("auth-service")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// 2. Connect to MongoDB
	client, db, err := shared.ConnectMongoDB(&cfg.MongoDB)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	// 3. Create gRPC Server
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(cfg.GRPC.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(cfg.GRPC.MaxSendMsgSize),
	)

	// 4. Initialize Auth Service
	// We pass the full config to access Security settings (JWT Secret, BCrypt cost)
	authService := auth.NewAuthService(db, cfg)
	pb.RegisterAuthServiceServer(grpcServer, authService)

	// 5. Register Health Server
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("auth.AuthService", grpc_health_v1.HealthCheckResponse_SERVING)

	// 6. Register Reflection
	reflection.Register(grpcServer)

	// 7. Start Listening
	listener, err := net.Listen("tcp", ":"+cfg.ServicePort)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", cfg.ServicePort, err)
	}

	// 8. Graceful Shutdown
	go func() {
		log.Printf("Auth Service is listening on port %s", cfg.ServicePort)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Auth Service...")
	healthServer.SetServingStatus("auth.AuthService", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	grpcServer.GracefulStop()

	if err := shared.DisconnectMongoDB(client); err != nil {
		log.Printf("Error disconnecting from MongoDB: %v", err)
	}
	log.Println("Auth Service stopped")
}
