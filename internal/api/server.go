package api

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/HeartBtz/tempest/internal/api/handler"
	"github.com/HeartBtz/tempest/internal/config"
	"github.com/HeartBtz/tempest/internal/engine"
	"github.com/HeartBtz/tempest/internal/storage"
)

type Server struct {
	mux        *http.ServeMux
	httpServer *http.Server
}

func NewServer(cfg *config.Config, db *storage.Database, manager *engine.Manager) *Server {
	if reconciled, err := db.ReconcileRunningSessions(); err != nil {
		log.Printf("Failed to reconcile interrupted sessions: %v", err)
	} else if reconciled > 0 {
		log.Printf("Reconciled %d interrupted running session(s)", reconciled)
	}
	s := &Server{
		mux: http.NewServeMux(),
	}
	s.setupRoutes(db, manager)
	s.httpServer = &http.Server{
		Addr:              listenAddress(cfg.Server.Host, cfg.Server.Port),
		Handler:           securityHeaders(logMiddleware(requestOriginPolicy(cfg, s.mux))),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return s
}

func listenAddress(host string, port int) string {
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		if unbracketed := strings.TrimSuffix(strings.TrimPrefix(host, "["), "]"); net.ParseIP(unbracketed) != nil {
			host = unbracketed
		}
	}
	return net.JoinHostPort(host, fmt.Sprintf("%d", port))
}

func (s *Server) setupRoutes(db *storage.Database, manager *engine.Manager) {
	s.mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})

	th := handler.NewTorrentHandler(db, manager)
	sh := handler.NewSessionHandler(db, manager)
	sth := handler.NewStatsHandler(db, manager)
	ph := handler.NewProfileHandler()
	nh := handler.NewNetworkHandler()
	seth := handler.NewSettingsHandler(db, manager)
	ch := handler.NewCategoryHandler(db, manager)

	// API routes
	s.mux.HandleFunc("/api/torrents", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			th.List(w, r)
		case http.MethodPost:
			th.Create(w, r)
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodPost)
		}
	})

	s.mux.HandleFunc("/api/torrents/", func(w http.ResponseWriter, r *http.Request) {
		if !isSingleIDPath(r.URL.Path, "/api/torrents/") {
			writeAPIError(w, http.StatusNotFound, "Route not found")
			return
		}
		switch r.Method {
		case http.MethodGet:
			th.Get(w, r)
		case http.MethodDelete:
			th.Delete(w, r)
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodDelete)
		}
	})

	s.mux.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			sh.List(w, r)
		case http.MethodPost:
			sh.Create(w, r)
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodPost)
		}
	})

	s.mux.HandleFunc("/api/sessions/", func(w http.ResponseWriter, r *http.Request) {
		sh.HandleSession(w, r)
	})

	s.mux.HandleFunc("/api/categories", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			ch.List(w, r)
		case http.MethodPost:
			ch.Create(w, r)
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodPost)
		}
	})

	s.mux.HandleFunc("/api/categories/unassign", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			ch.Unassign(w, r)
		} else {
			methodNotAllowed(w, http.MethodPut)
		}
	})

	s.mux.HandleFunc("/api/categories/", func(w http.ResponseWriter, r *http.Request) {
		ch.HandleOne(w, r)
	})

	s.mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		sth.GetGlobal(w, r)
	})

	s.mux.HandleFunc("/api/profiles", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		ph.List(w, r)
	})

	s.mux.HandleFunc("/api/interfaces", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		nh.ListInterfaces(w, r)
	})

	s.mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			seth.Get(w, r)
		case http.MethodPut:
			seth.Update(w, r)
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodPut)
		}
	})

	s.mux.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		sth.GetLogs(w, r)
	})

	s.mux.HandleFunc("/api/ws/logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		sth.StreamLogs(w, r)
	})

	s.mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		writeAPIError(w, http.StatusNotFound, "Route not found")
	})
	s.mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeAPIError(w, http.StatusNotFound, "Route not found")
	})

	// Serve frontend static files
	distDir := resolveDistDir()
	log.Printf("Serving frontend from: %s", distDir)
	fs := http.FileServer(http.Dir(distDir))
	indexPath := filepath.Join(distDir, "index.html")

	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			methodNotAllowed(w, http.MethodGet, http.MethodHead)
			return
		}
		clean := filepath.Clean(r.URL.Path)
		r.URL.Path = clean

		// Serve index.html for root
		if clean == "/" || clean == "/index.html" {
			http.ServeFile(w, r, indexPath)
			return
		}

		// Try to serve the actual file; if it doesn't exist, serve index.html (SPA fallback)
		filePath := filepath.Join(distDir, filepath.FromSlash(clean))
		if _, err := os.Stat(filePath); err == nil {
			fs.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html for any unmatched route
		http.ServeFile(w, r, indexPath)
	})
}

func (s *Server) HTTPServer() *http.Server {
	return s.httpServer
}

func (s *Server) Start() error {
	log.Printf("⚡ Tempest server starting on http://%s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

func isSingleIDPath(path, prefix string) bool {
	id := strings.TrimPrefix(path, prefix)
	return id != "" && !strings.Contains(id, "/")
}

func methodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	writeAPIError(w, http.StatusMethodNotAllowed, "Method not allowed")
}

func writeAPIError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `{"error":%q}`+"\n", message)
}

// resolveDistDir finds the frontend dist directory.
// It checks in order: relative to executable, then relative to working directory.
func resolveDistDir() string {
	candidates := []string{}

	// 1. Relative to executable (handles installed / production layout)
	if exe, err := os.Executable(); err == nil {
		realExe, err := filepath.EvalSymlinks(exe)
		if err == nil {
			exe = realExe
		}
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "web", "dist"),
			filepath.Join(exeDir, "..", "web", "dist"),
		)
	}

	// 2. Relative to working directory (handles go run / dev mode)
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, "web", "dist"))
	}

	for _, dir := range candidates {
		indexFile := filepath.Join(dir, "index.html")
		if info, err := os.Stat(indexFile); err == nil && !info.IsDir() {
			abs, _ := filepath.Abs(dir)
			if abs != "" {
				return abs
			}
			return dir
		}
	}

	// Fallback — will 404 but at least log a helpful message
	log.Printf("WARNING: could not find web/dist/index.html in any of: %s", strings.Join(candidates, ", "))
	return "web/dist"
}
