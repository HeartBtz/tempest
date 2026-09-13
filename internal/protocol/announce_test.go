package protocol

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/HeartBtz/tempest/internal/client"
	"github.com/HeartBtz/tempest/internal/config"
)

func TestValidateTrackerDestinationBlocksLocalNetworks(t *testing.T) {
	blocked := map[string]string{
		"IPv4 unspecified":    "0.0.0.0",
		"IPv4 private":        "10.0.0.1",
		"IPv4 CGNAT":          "100.64.0.1",
		"IPv4 loopback":       "127.0.0.1",
		"IPv4 link-local":     "169.254.169.254",
		"IPv4 documentation":  "192.0.2.1",
		"IPv4 benchmarking":   "198.18.0.1",
		"IPv4 multicast":      "224.0.0.1",
		"IPv4 reserved":       "240.0.0.1",
		"IPv6 unspecified":    "::",
		"IPv6 loopback":       "::1",
		"IPv6 discard-only":   "100::1",
		"IPv6 documentation":  "2001:db8::1",
		"IPv6 6to4":           "2002::1",
		"IPv6 documentation2": "3fff::1",
		"IPv6 private":        "fd00::1",
		"IPv6 link-local":     "fe80::1",
		"IPv6 reserved":       "fec0::1",
		"IPv6 multicast":      "ff02::1",
	}
	for name, address := range blocked {
		t.Run(name, func(t *testing.T) {
			ip := netip.MustParseAddr(address)
			if !isRestrictedDestination(ip) {
				t.Fatalf("expected %s to be classified as restricted", ip)
			}

			u := &url.URL{Scheme: "http", Host: net.JoinHostPort(ip.String(), "80"), Path: "/announce"}
			if err := validateTrackerDestination(context.Background(), u, false); err == nil {
				t.Fatalf("expected initial destination %s to be blocked", u)
			}
		})
	}

	for _, address := range []string{
		"8.8.8.8",
		"2606:4700:4700::1111",
	} {
		if isRestrictedDestination(netip.MustParseAddr(address)) {
			t.Errorf("expected global address %s to be allowed", address)
		}
	}
}

func TestValidateTrackerDestinationPrivateOptIn(t *testing.T) {
	u, _ := url.Parse("http://127.0.0.1/announce")
	if err := validateTrackerDestination(context.Background(), u, true); err != nil {
		t.Fatalf("private opt-in rejected destination: %v", err)
	}
}

func TestInitialTrackerValidationContextIsBounded(t *testing.T) {
	ctx, cancel := initialTrackerValidationContext()
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("initial tracker validation context has no deadline")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > trackerRequestTimeout {
		t.Fatalf("initial tracker validation deadline remaining = %v, want within (0, %v]", remaining, trackerRequestTimeout)
	}
}

func TestRedirectRevalidatesDestination(t *testing.T) {
	cfg := config.Default()
	config.Set(cfg)
	t.Cleanup(func() { config.Set(config.Default()) })
	tc := NewTrackerClient()
	for _, destination := range []string{
		"http://100.64.0.1/redirected",
		"http://192.0.2.1/redirected",
		"http://[2001:db8::1]/redirected",
	} {
		req := httptest.NewRequest(http.MethodGet, destination, nil)
		if err := tc.httpClient.CheckRedirect(req, nil); err == nil || !strings.Contains(err.Error(), "blocked by SSRF policy") {
			t.Fatalf("redirect validation error for %s = %v", destination, err)
		}
	}
}

func TestTrackerClientMissingInterfaceFailsClosed(t *testing.T) {
	tc := NewTrackerClientWithInterface("tempest-interface-that-does-not-exist")
	_, err := tc.Announce(AnnounceRequest{TrackerURL: "http://8.8.8.8/announce"})
	if err == nil || !strings.Contains(err.Error(), "is unavailable") {
		t.Fatalf("missing interface error = %v", err)
	}
}

func TestCompatibleDialAddressesRequiresMatchingFamily(t *testing.T) {
	local := []netip.Addr{netip.MustParseAddr("192.168.1.2")}
	remote := []netip.Addr{netip.MustParseAddr("2606:4700:4700::1111")}
	if _, _, ok := compatibleDialAddresses(local, remote); ok {
		t.Fatal("IPv4-only interface unexpectedly matched IPv6 destination")
	}

	remote = append(remote, netip.MustParseAddr("8.8.8.8"))
	localIP, remoteIP, ok := compatibleDialAddresses(local, remote)
	if !ok || localIP.String() != "192.168.1.2" || remoteIP.String() != "8.8.8.8" {
		t.Fatalf("compatible addresses = %s, %s, %v", localIP, remoteIP, ok)
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

func TestParseResponseClampsTrackerIntervals(t *testing.T) {
	tc := NewTrackerClient()
	oversized := int64(^uint64(0) >> 1)
	response := fmt.Sprintf("d8:intervali%de12:min intervali-1ee", oversized)
	parsed, err := tc.parseResponse([]byte(response))
	if err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if want := safeTrackerInterval(oversized); parsed.Interval != want {
		t.Fatalf("interval = %d, want %d", parsed.Interval, want)
	}
	if parsed.MinInterval != 0 {
		t.Fatalf("minimum interval = %d, want 0", parsed.MinInterval)
	}
	if duration := time.Duration(parsed.Interval) * time.Second; duration < 0 || duration/time.Second != time.Duration(parsed.Interval) {
		t.Fatalf("clamped interval overflowed duration: %v", duration)
	}
}
