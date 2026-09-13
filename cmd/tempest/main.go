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

	"github.com/HeartBtz/tempest/internal/api"
	"github.com/HeartBtz/tempest/internal/config"
	"github.com/HeartBtz/tempest/internal/engine"
	"github.com/HeartBtz/tempest/internal/storage"
)

var version = "0.1.0"

func main() {
	configPath := flag.String("config", "", "Path to config file")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Tempest ⚡ v%s\n", version)
		os.Exit(0)
	}

	// Load .env file (if present) before reading config so env vars are available.
	// Looks for .env in the current working directory.
	if err := config.LoadDotEnv(".env"); err != nil {
		log.Printf("Warning: failed to load .env: %v", err)
	}

	fmt.Println(`
  ████████╗███████╗███╗   ███╗██████╗ ███████╗███████╗████████╗
  ╚══██╔══╝██╔════╝████╗ ████║██╔══██╗██╔════╝██╔════╝╚══██╔══╝
     ██║   █████╗  ██╔████╔██║██████╔╝█████╗  ███████╗   ██║
     ██║   ██╔══╝  ██║╚██╔╝██║██╔═══╝ ██╔══╝  ╚════██║   ██║
     ██║   ███████╗██║ ╚═╝ ██║██║     ███████╗███████║   ██║
     ╚═╝   ╚══════╝╚═╝     ╚═╝╚═╝     ╚══════╝╚══════╝   ╚═╝
                  ⚡ BitTorrent Announce Testing Dashboard`)
	fmt.Printf("                        v%s\n\n", version)

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	config.Set(cfg)

	// Initialize database
	db, err := storage.NewDatabase(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize engine
	manager := engine.NewManager(db)

	// Initialize API server
	server := api.NewServer(cfg, db, manager)

	// Create http.Server for graceful shutdown
	httpServer := server.HTTPServer()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("\n⚡ Shutting down Tempest...")
		manager.StopAll()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("HTTP shutdown failed: %v", err)
		}
	}()

	// Start server
	if err := server.Start(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}
