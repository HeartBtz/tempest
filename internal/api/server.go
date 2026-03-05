package api

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"github.com/tempest-bt/tempest/internal/api/handler"
	"github.com/tempest-bt/tempest/internal/config"
	"github.com/tempest-bt/tempest/internal/engine"
	"github.com/tempest-bt/tempest/internal/storage"
)

type Server struct {
	cfg        *config.Config
	db         *storage.Database
	manager    *engine.Manager
	mux        *http.ServeMux
	httpServer *http.Server
}

func NewServer(cfg *config.Config, db *storage.Database, manager *engine.Manager) *Server {
	s := &Server{
		cfg:     cfg,
		db:      db,
		manager: manager,
		mux:     http.NewServeMux(),
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	th := handler.NewTorrentHandler(s.db)
	sh := handler.NewSessionHandler(s.db, s.manager)
	sth := handler.NewStatsHandler(s.db, s.manager)
	ph := handler.NewProfileHandler()
	nh := handler.NewNetworkHandler()
	seth := handler.NewSettingsHandler(s.db)

	// API routes
	s.mux.HandleFunc("/api/torrents", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			th.List(w, r)
		case http.MethodPost:
			th.Create(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	s.mux.HandleFunc("/api/torrents/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			th.Get(w, r)
		case http.MethodDelete:
			th.Delete(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	s.mux.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			sh.List(w, r)
		case http.MethodPost:
			sh.Create(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	s.mux.HandleFunc("/api/sessions/", func(w http.ResponseWriter, r *http.Request) {
		sh.HandleSession(w, r)
	})

	s.mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		sth.GetGlobal(w, r)
	})

	s.mux.HandleFunc("/api/profiles", func(w http.ResponseWriter, r *http.Request) {
		ph.List(w, r)
	})

	s.mux.HandleFunc("/api/interfaces", func(w http.ResponseWriter, r *http.Request) {
		nh.ListInterfaces(w, r)
	})

	s.mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			seth.Get(w, r)
		case http.MethodPut:
			seth.Update(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	s.mux.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
		sth.GetLogs(w, r)
	})

	s.mux.HandleFunc("/api/ws/logs", func(w http.ResponseWriter, r *http.Request) {
		sth.StreamLogs(w, r)
	})

	// Serve frontend static files (clean path to prevent traversal)
	fs := http.FileServer(http.Dir("web/dist"))
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Sanitize path to prevent traversal
		clean := filepath.Clean(r.URL.Path)
		r.URL.Path = clean

		if clean == "/" || clean == "/index.html" {
			http.ServeFile(w, r, "web/dist/index.html")
			return
		}
		fs.ServeHTTP(w, r)
	})
}

func (s *Server) HTTPServer() *http.Server {
	return s.httpServer
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	log.Printf("⚡ Tempest server starting on http://%s", addr)

	wrapped := securityHeaders(corsMiddleware(logMiddleware(s.mux)))

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: wrapped,
	}
	return s.httpServer.ListenAndServe()
}
