// cmd/vault/main.go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/askwhyharsh/lazytrader/internal/config"
	"github.com/askwhyharsh/lazytrader/internal/database"
	// "github.com/askwhyharsh/lazytrader/internal/executor"
	"github.com/askwhyharsh/lazytrader/internal/ingestion"
	"github.com/askwhyharsh/lazytrader/internal/server"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration
	cfg, err := config.Load("./config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	db, err := database.New(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize components
	ingestor := ingestion.New(cfg, db)
	// exec := executor.New(cfg, db)

	// Start ingestion service (event listener)
	go func() {
		if err := ingestor.Start(ctx); err != nil {
			log.Printf("Ingestion service error: %v", err)
		}
	}()

	// // Start execution engine
	// go func() {
	// 	if err := exec.Start(ctx); err != nil {
	// 		log.Printf("Execution engine error: %v", err)
	// 	}
	// }()

	// Start HTTP server
	srv := server.New(cfg, db)
	go func() {
		if err := srv.Start(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down gracefully...")
	cancel()
}