package api

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/HeartBtz/tempest/internal/config"
)

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'none'; connect-src 'self'; frame-ancestors 'none'; form-action 'self'; img-src 'self' data:; object-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'")
		next.ServeHTTP(w, r)
	})
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		var wrapped http.ResponseWriter = rw
		if flusher, ok := w.(http.Flusher); ok {
			wrapped = &flushResponseWriter{responseWriter: rw, flusher: flusher}
		}
		next.ServeHTTP(wrapped, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.statusCode, time.Since(start))
	})
}

func requestOriginPolicy(cfg *config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestScheme := "http"
		if r.TLS != nil {
			requestScheme = "https"
		}
		requestAuthority, err := normalizeAuthority(r.Host, requestScheme)
		if err != nil || !allowedHostname(cfg, requestAuthority.host) {
			writeMiddlewareError(w, http.StatusForbidden, "Request host is not allowed")
			return
		}
		if !requiresOriginCheck(r) {
			next.ServeHTTP(w, r)
			return
		}

		origins := r.Header.Values("Origin")
		if len(origins) > 1 {
			writeMiddlewareError(w, http.StatusForbidden, "Request origin is not allowed")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			originURL, originAuthority, err := parseOrigin(origin)
			if err != nil {
				writeMiddlewareError(w, http.StatusForbidden, "Request origin is not allowed")
				return
			}
			if !allowedOrigin(cfg, originURL.Scheme, originAuthority) &&
				(originURL.Scheme != requestScheme || originAuthority != requestAuthority) {
				writeMiddlewareError(w, http.StatusForbidden, "Request origin is not allowed")
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func parseOrigin(origin string) (*url.URL, authority, error) {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Opaque != "" || parsed.User != nil || parsed.Host == "" ||
		parsed.Path != "" || parsed.RawPath != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, authority{}, fmt.Errorf("invalid origin")
	}
	normalized, err := normalizeAuthority(parsed.Host, parsed.Scheme)
	if err != nil {
		return nil, authority{}, err
	}
	return parsed, normalized, nil
}

func allowedOrigin(cfg *config.Config, scheme string, originAuthority authority) bool {
	if isUnspecifiedHost(originAuthority.host) {
		return false
	}
	for _, allowed := range cfg.Security.AllowedOrigins {
		parsed, allowedAuthority, err := parseOrigin(allowed)
		if err == nil && parsed.Scheme == scheme && allowedAuthority == originAuthority {
			return true
		}
	}
	return false
}

type authority struct {
	host string
	port string
}

func normalizeAuthority(value, scheme string) (authority, error) {
	host, port, err := splitAuthority(value)
	if err != nil {
		return authority{}, err
	}
	if port == "" {
		switch scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			return authority{}, fmt.Errorf("invalid scheme")
		}
	}
	return authority{host: host, port: port}, nil
}

func splitAuthority(value string) (string, string, error) {
	if value == "" || strings.Contains(value, "@") || strings.TrimSpace(value) != value {
		return "", "", fmt.Errorf("invalid authority")
	}
	host := value
	port := ""
	if net.ParseIP(value) == nil {
		if parsedHost, parsedPort, err := net.SplitHostPort(value); err == nil {
			host = parsedHost
			port = parsedPort
		} else if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
			host = strings.TrimSuffix(strings.TrimPrefix(value, "["), "]")
		} else if strings.Contains(value, ":") {
			return "", "", fmt.Errorf("invalid authority")
		}
	}
	if port != "" {
		portNumber, err := strconv.Atoi(port)
		if err != nil || portNumber < 1 || portNumber > 65535 || strconv.Itoa(portNumber) != port {
			return "", "", fmt.Errorf("invalid authority")
		}
	}
	host, err := normalizedHostname(host)
	if err != nil {
		return "", "", err
	}
	return host, port, nil
}

func requiresOriginCheck(r *http.Request) bool {
	if r.URL.Path == "/api/ws/logs" {
		return true
	}
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return strings.HasPrefix(r.URL.Path, "/api/")
	default:
		return false
	}
}

func normalizedHostname(authority string) (string, error) {
	if authority == "" {
		return "", fmt.Errorf("invalid authority")
	}
	host := strings.ToLower(strings.TrimSuffix(authority, "."))
	if host == "" {
		return "", fmt.Errorf("invalid authority")
	}
	if net.ParseIP(host) == nil {
		if len(host) > 253 || strings.ContainsAny(host, "/\\ ") {
			return "", fmt.Errorf("invalid authority")
		}
		for _, label := range strings.Split(host, ".") {
			if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return "", fmt.Errorf("invalid authority")
			}
			for _, char := range label {
				if char > 127 || char != '-' && (char < 'a' || char > 'z') && (char < '0' || char > '9') {
					return "", fmt.Errorf("invalid authority")
				}
			}
		}
	}
	return host, nil
}

func allowedHostname(cfg *config.Config, host string) bool {
	if isLoopbackHost(host) {
		return true
	}
	configured, err := normalizedHostname(cfg.Server.Host)
	if err == nil && !isUnspecifiedHost(configured) && host == configured {
		return true
	}
	for _, allowed := range cfg.Security.AllowedHosts {
		normalized, err := normalizedHostname(allowed)
		if err == nil && !isUnspecifiedHost(normalized) && host == normalized {
			return true
		}
	}
	return false
}

func isLoopbackHost(host string) bool {
	return host == "localhost" || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
}

func isUnspecifiedHost(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsUnspecified()
}

func writeMiddlewareError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `{"error":%q}`+"\n", message)
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

type flushResponseWriter struct {
	*responseWriter
	flusher http.Flusher
}

func (rw *flushResponseWriter) Flush() {
	rw.flusher.Flush()
}
