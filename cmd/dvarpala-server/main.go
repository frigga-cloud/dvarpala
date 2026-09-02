package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dvarpala/internal/app"
	"dvarpala/internal/config"
)

func main() {
	var configPath = flag.String("config", "configs/environment.yaml", "Config file path")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize Dvarpala application
	dvarpala, err := app.NewDvarpala(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize Dvarpala: %v", err)
	}

	// Start server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: dvarpala.Router(),
	}

	// Work that runs alongside the HTTP server - currently closing tunnels
	// nobody ever signed in on. Stopped by the same signal that stops serving.
	background, stopBackground := context.WithCancel(context.Background())
	defer stopBackground()
	go dvarpala.Background(background)

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down server...")
		stopBackground()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	// Said last, where it will be read. Everything above this is setup detail
	// and, in debug mode, one line per registered route - so an operator who
	// restarts the service and looks at the end of the log sees the answer to
	// the question they restarted it for.
	for _, line := range dvarpala.SignInSummary() {
		log.Print(line)
	}

	log.Printf("Dvarpala server starting on port %d", cfg.Server.Port)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server failed to start: %v", err)
	}
}
