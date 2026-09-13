package protocol

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/HeartBtz/tempest/internal/client"
	"github.com/HeartBtz/tempest/internal/config"
)

func TestValidateTrackerDestinationBlocksLocalNetworks(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1/announce",
		"http://10.0.0.1/announce",
		"http://169.254.169.254/latest/meta-data",
		"http://[::1]/announce",
	} {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateTrackerDestination(context.Background(), u, false); err == nil {
			t.Errorf("expected %s to be blocked", raw)
		}
	}
}

func TestValidateTrackerDestinationPrivateOptIn(t *testing.T) {
	u, _ := url.Parse("http://127.0.0.1/announce")
	if err := validateTrackerDestination(context.Background(), u, true); err != nil {
		t.Fatalf("private opt-in rejected destination: %v", err)
	}
}

func TestRedirectRevalidatesDestination(t *testing.T) {
	cfg := config.Default()
	config.Set(cfg)
	tc := NewTrackerClient()
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/redirected", nil)
	if err := tc.httpClient.CheckRedirect(req, nil); err == nil || !strings.Contains(err.Error(), "blocked by SSRF policy") {
		t.Fatalf("redirect validation error = %v", err)
	}
}

func TestTrackerResponseSizeLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, maxTrackerResponseSize+1))
	}))
	defer server.Close()

	cfg := config.Default()
	cfg.Security.AllowPrivateTrackerDestinations = true
	config.Set(cfg)
	t.Cleanup(func() { config.Set(config.Default()) })

	_, err := NewTrackerClient().Announce(AnnounceRequest{
		TrackerURL: server.URL + "/secret-passkey/announce?passkey=hidden",
		Profile:    client.DefaultProfile(),
	})
	if err == nil || !strings.Contains(err.Error(), "exceeds the 2 MiB limit") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRedactSensitiveText(t *testing.T) {
	input := "request http://tracker.example/secret-passkey/announce?passkey=abc123 failed; token=xyz"
	got := RedactSensitiveText(input)
	for _, secret := range []string{"secret-passkey", "abc123", "xyz"} {
		if strings.Contains(got, secret) {
			t.Fatalf("redacted text still contains %q: %s", secret, got)
		}
	}
}
