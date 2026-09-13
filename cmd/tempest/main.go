package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/HeartBtz/tempest/internal/api"
	"github.com/HeartBtz/tempest/internal/config"
	"github.com/HeartBtz/tempest/internal/engine"
	"github.com/HeartBtz/tempest/internal/storage"
)

var version = "0.1.0"

const shutdownTimeout = 8 * time.Second

func main() {
	configPath := flag.String("config", "", "Path to config file")
	envPath := flag.String("env-file", "", "Path to environment file (for --health-url)")
	showHealthURL := flag.Bool("health-url", false, "Show the effective local health URL")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Tempest ⚡ v%s\n", version)
		os.Exit(0)
	}

	if *envPath != "" {
		if !*showHealthURL {
			log.Fatal("--env-file is only supported with --health-url")
		}
		if err := config.LoadDotEnv(*envPath); err != nil {
			log.Fatalf("Failed to load environment file: %v", err)
		}
	}

	// Explicit process or service environment values take precedence over .env.
	if err := config.LoadDotEnv(".env"); err != nil {
		log.Printf("Warning: failed to load .env: %v", err)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if *showHealthURL {
		fmt.Println(effectiveHealthURL(cfg.Server.Host, cfg.Server.Port))
		return
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
	defer signal.Stop(quit)
	shutdownStarted := make(chan struct{})
	shutdownDone := make(chan struct{})

	go func() {
		<-quit
		close(shutdownStarted)
		defer close(shutdownDone)
		log.Println("\n⚡ Shutting down Tempest...")

		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		managerDone := make(chan struct{})
		go func() {
			manager.StopAll()
			close(managerDone)
		}()

		httpDone := make(chan error, 1)
		go func() {
			httpDone <- httpServer.Shutdown(ctx)
		}()

		for managerDone != nil || httpDone != nil {
			select {
			case <-managerDone:
				managerDone = nil
			case err := <-httpDone:
				if err != nil && err != context.DeadlineExceeded {
					log.Printf("HTTP shutdown failed: %v", err)
				}
				httpDone = nil
			case <-ctx.Done():
				log.Printf("Graceful shutdown timed out after %s; forcing exit", shutdownTimeout)
				go httpServer.Close()
				return
			}
		}
	}()

	// Start server
	if err := server.Start(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}

	select {
	case <-shutdownStarted:
		<-shutdownDone
	default:
	}
}

func effectiveHealthURL(host string, port int) string {
	host = strings.TrimSpace(host)
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		if ip.To4() != nil {
			host = "127.0.0.1"
		} else {
			host = "::1"
		}
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(port)) + "/health"
}
