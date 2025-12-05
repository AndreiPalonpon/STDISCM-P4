package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"stdiscm_p4/backend/internal/gateway"
	"syscall"
	"time"
)

func main() {
	log.Println("INFO: Starting Gateway Service...")

	// 1. Initialize gRPC Clients
	// This connects to all 5 backend microservices
	serviceClients := gateway.NewServiceClients()
	defer serviceClients.Close()

	// 2. Setup Routes and Middleware
	router := gateway.SetupRoutes(serviceClients)

	// 3. Configure Server
	port := gateway.GetEnv("PORT", "8080")
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 4. Start Server in a Goroutine
	go func() {
		log.Printf("INFO: Gateway listening on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("FATAL: HTTP server error: %v", err)
		}
	}()

	// 5. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("INFO: Shutting down Gateway...")

	// Close any other resources if necessary
	log.Println("INFO: Gateway stopped.")
}
