package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	// Proto imports
	"stdiscm_p4/backend/pb/course"
	"stdiscm_p4/backend/pb/enrollment"
)

const (
	port              = ":50053"
	mongoURI          = "mongodb://localhost:27017"
	courseServiceAddr = "localhost:50052" // Address of Course Service
)

func main() {
	log.Println("Starting Enrollment Service...")

	// 1. Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	// Ping DB
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("MongoDB not reachable: %v", err)
	}
	log.Println("Connected to MongoDB")

	// 2. Connect to Course Service (Client)
	// We need this to check prerequisites and course availability before enrolling
	conn, err := grpc.Dial(courseServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Course Service: %v", err)
	}
	defer conn.Close()
	courseClient := course.NewCourseServiceClient(conn)

	// 3. Setup gRPC Server
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()

	// Initialize the service logic
	enrollmentSvc := NewEnrollmentService(client, courseClient)
	enrollment.RegisterEnrollmentServiceServer(grpcServer, enrollmentSvc)

	// 4. Graceful Shutdown
	go func() {
		log.Printf("Enrollment Service listening on %s", port)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for kill signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down Enrollment Service...")
	grpcServer.GracefulStop()
}
