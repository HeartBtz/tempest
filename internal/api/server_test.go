package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HeartBtz/tempest/internal/config"
	"github.com/HeartBtz/tempest/internal/engine"
	"github.com/HeartBtz/tempest/internal/storage"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := config.Default()
	cfg.Database.Path = t.TempDir() + "/tempest.db"
	config.Set(cfg)
	db, err := storage.NewDatabase(cfg.Database.Path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewServer(cfg, db, engine.NewManager(db))
}

func TestNewServerInitializesHTTPServer(t *testing.T) {
	if newTestServer(t).HTTPServer() == nil {
		t.Fatal("HTTPServer returned nil")
	}
}

func TestAPIRoutesEnforceMethodsAndJSONNotFound(t *testing.T) {
	s := newTestServer(t)
	tests := []struct {
		method string
		path   string
		status int
	}{
		{http.MethodPost, "/health", http.StatusMethodNotAllowed},
		{http.MethodDelete, "/api/stats", http.StatusMethodNotAllowed},
		{http.MethodGet, "/api/sessions/123/start", http.StatusMethodNotAllowed},
		{http.MethodPost, "/api/categories/123/assign", http.StatusMethodNotAllowed},
		{http.MethodGet, "/api/torrents/123/extra", http.StatusNotFound},
		{http.MethodGet, "/api/not-a-route", http.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rr := httptest.NewRecorder()
			s.HTTPServer().Handler.ServeHTTP(rr, req)
			if rr.Code != tc.status {
				t.Fatalf("status = %d, want %d; body=%s", rr.Code, tc.status, rr.Body.String())
			}
			if strings.HasPrefix(tc.path, "/api/") && !strings.HasPrefix(rr.Header().Get("Content-Type"), "application/json") {
				t.Fatalf("Content-Type = %q, want application/json", rr.Header().Get("Content-Type"))
			}
		})
	}
}

func TestSecurityHeadersAllowCurrentInlineStyles(t *testing.T) {
	s := newTestServer(t)
	rr := httptest.NewRecorder()
	s.HTTPServer().Handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/health", nil))
	csp := rr.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self'") || !strings.Contains(csp, "style-src 'self' 'unsafe-inline'") {
		t.Fatalf("unexpected CSP: %q", csp)
	}
}
