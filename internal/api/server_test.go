package api

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestNewServerBuildsIPv6ListenAddress(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Host = "[::]"
	cfg.Server.Port = 0
	cfg.Database.Path = t.TempDir() + "/tempest.db"
	config.Set(cfg)
	db, err := storage.NewDatabase(cfg.Database.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := NewServer(cfg, db, engine.NewManager(db))
	if server.HTTPServer().Addr != "[::]:0" {
		t.Fatalf("listen address = %q, want [::]:0", server.HTTPServer().Addr)
	}
	listener, err := net.Listen("tcp", server.HTTPServer().Addr)
	if err != nil {
		t.Skipf("IPv6 wildcard listener unavailable: %v", err)
	}
	defer listener.Close()
	if host, _, err := net.SplitHostPort(listener.Addr().String()); err != nil || net.ParseIP(host) == nil {
		t.Fatalf("listener address = %q, split error = %v", listener.Addr(), err)
	}
}

func TestNewServerReconcilesInterruptedSessions(t *testing.T) {
	cfg := config.Default()
	cfg.Database.Path = t.TempDir() + "/tempest.db"
	config.Set(cfg)
	db, err := storage.NewDatabase(cfg.Database.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now()
	if err := db.CreateTorrent(&storage.Torrent{ID: "torrent", Name: "test", InfoHash: "hash", Trackers: "[]", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateSession(&storage.Session{ID: "session", TorrentID: "torrent", Status: "running", UploadSpeed: 10, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	_ = NewServer(cfg, db, engine.NewManager(db))
	session, err := db.GetSession("session")
	if err != nil {
		t.Fatal(err)
	}
	if session.Status != "stopped" || session.UploadSpeed != 0 {
		t.Fatalf("interrupted session not reconciled: %+v", session)
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
			req.Host = "localhost:8377"
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

func TestOriginPolicyForAPIWritesAndSSE(t *testing.T) {
	cfg := config.Default()
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := requestOriginPolicy(cfg, next)
	tests := []struct {
		name   string
		method string
		path   string
		host   string
		origin string
		status int
	}{
		{"same origin", http.MethodPut, "/api/settings", "127.0.0.1:8377", "http://127.0.0.1:8377", http.StatusNoContent},
		{"cross port", http.MethodPost, "/api/torrents", "localhost:8377", "http://localhost:3000", http.StatusForbidden},
		{"loopback aliases differ", http.MethodGet, "/api/ws/logs", "127.0.0.1:8377", "http://localhost:8377", http.StatusForbidden},
		{"cross scheme", http.MethodPost, "/api/torrents", "localhost:8377", "https://localhost:8377", http.StatusForbidden},
		{"bad host", http.MethodDelete, "/api/sessions/id", "attacker.example", "", http.StatusForbidden},
		{"bad origin", http.MethodPost, "/api/sessions", "localhost:8377", "https://attacker.example", http.StatusForbidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != tc.status {
				t.Fatalf("status = %d, want %d; body=%s", rr.Code, tc.status, rr.Body.String())
			}
		})
	}
}

func TestOriginPolicyAllowsExplicitConfiguredProxyHost(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Host = "0.0.0.0"
	cfg.Security.AllowedHosts = []string{"tempest.example"}
	cfg.Security.AllowedOrigins = []string{"https://tempest.example"}
	req := httptest.NewRequest(http.MethodPost, "/api/sessions", nil)
	req.Host = "tempest.example"
	req.Header.Set("Origin", "https://tempest.example")
	req.Header.Set("X-Forwarded-Proto", "https")
	rr := httptest.NewRecorder()
	requestOriginPolicy(cfg, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d; body=%s", rr.Code, rr.Body.String())
	}
}

func TestOriginPolicyKeepsHostAllowlistIndependentFromAllowedOrigin(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Host = "0.0.0.0"
	cfg.Security.AllowedOrigins = []string{"https://tempest.example"}
	req := httptest.NewRequest(http.MethodPost, "/api/sessions", nil)
	req.Host = "tempest.example"
	req.Header.Set("Origin", "https://tempest.example")
	rr := httptest.NewRecorder()
	requestOriginPolicy(cfg, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rr.Code, rr.Body.String())
	}
}

func TestOriginPolicyDoesNotTrustForwardingHeaders(t *testing.T) {
	cfg := config.Default()
	req := httptest.NewRequest(http.MethodPost, "/api/sessions", nil)
	req.Host = "localhost:8377"
	req.Header.Set("Origin", "https://localhost:8377")
	req.Header.Set("Forwarded", "proto=https")
	req.Header.Set("X-Forwarded-Proto", "https")
	rr := httptest.NewRecorder()
	requestOriginPolicy(cfg, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rr.Code, rr.Body.String())
	}
}

func TestOriginPolicyRejectsConfiguredOriginWithPath(t *testing.T) {
	cfg := config.Default()
	cfg.Security.AllowedHosts = []string{"tempest.example"}
	cfg.Security.AllowedOrigins = []string{"https://tempest.example/path"}
	req := httptest.NewRequest(http.MethodPost, "/api/sessions", nil)
	req.Host = "tempest.example"
	req.Header.Set("Origin", "https://tempest.example")
	rr := httptest.NewRecorder()
	requestOriginPolicy(cfg, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rr.Code, rr.Body.String())
	}
}

func TestOriginPolicyWildcardBindOnlyAddsLoopbackHosts(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Host = "0.0.0.0"
	handler := requestOriginPolicy(cfg, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, tc := range []struct {
		host   string
		status int
	}{
		{"localhost:8377", http.StatusNoContent},
		{"attacker.example", http.StatusForbidden},
	} {
		req := httptest.NewRequest(http.MethodPut, "/api/settings", nil)
		req.Host = tc.host
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != tc.status {
			t.Fatalf("host %q: status = %d, want %d", tc.host, rr.Code, tc.status)
		}
	}
}

func TestOriginPolicyRejectsDisallowedHostOnReads(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Host = "0.0.0.0"
	req := httptest.NewRequest(http.MethodGet, "/api/torrents", nil)
	req.Host = "rebound.example"
	rr := httptest.NewRecorder()
	requestOriginPolicy(cfg, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rr.Code, rr.Body.String())
	}
}

func TestOriginPolicyWildcardBindAllowsExplicitHosts(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Host = "0.0.0.0"
	cfg.Security.AllowedHosts = []string{"tempest.example"}
	handler := requestOriginPolicy(cfg, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPut, "/api/settings", nil)
	req.Host = "tempest.example:8377"
	req.Header.Set("Origin", "http://tempest.example:8377")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rr.Code, rr.Body.String())
	}
}

func TestOriginPolicyRejectsMalformedAuthorities(t *testing.T) {
	cfg := config.Default()
	cfg.Security.AllowedHosts = []string{"valid.example", "https://invalid.example", "0.0.0.0"}
	handler := requestOriginPolicy(cfg, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	tests := []struct {
		name   string
		host   string
		origin string
	}{
		{"host user info", "user@localhost:8377", ""},
		{"host bad port", "localhost:99999", ""},
		{"host leading-zero port", "localhost:08377", ""},
		{"host URL", "http://localhost:8377", ""},
		{"origin path", "localhost:8377", "http://localhost:8377/path"},
		{"origin credentials", "localhost:8377", "http://user@localhost:8377"},
		{"origin malformed port", "localhost:8377", "http://localhost:bad"},
		{"origin list", "localhost:8377", "http://localhost:8377, http://localhost:8377"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/api/settings", nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403; body=%s", rr.Code, rr.Body.String())
			}
		})
	}
}

type flushRecorder struct {
	header  http.Header
	status  int
	body    strings.Builder
	flushed bool
}

func (w *flushRecorder) Header() http.Header    { return w.header }
func (w *flushRecorder) WriteHeader(status int) { w.status = status }
func (w *flushRecorder) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return io.WriteString(&w.body, string(p))
}
func (w *flushRecorder) Flush() { w.flushed = true }

func TestLoggingMiddlewarePreservesSSEFlushing(t *testing.T) {
	cfg := config.Default()
	stream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: test\n\n"))
		flusher.Flush()
	})
	w := &flushRecorder{header: make(http.Header)}
	req := httptest.NewRequest(http.MethodGet, "/api/ws/logs", nil)
	req.Host = "localhost:8377"
	securityHeaders(logMiddleware(requestOriginPolicy(cfg, stream))).ServeHTTP(w, req)
	if w.status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.status, w.body.String())
	}
	if !w.flushed {
		t.Fatal("SSE response was not flushed")
	}
}

func TestSecurityHeadersAllowCurrentInlineStyles(t *testing.T) {
	s := newTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Host = "localhost:8377"
	s.HTTPServer().Handler.ServeHTTP(rr, req)
	csp := rr.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self'") || !strings.Contains(csp, "style-src 'self' 'unsafe-inline'") {
		t.Fatalf("unexpected CSP: %q", csp)
	}
}
