package protocol

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/HeartBtz/tempest/internal/client"
	"github.com/HeartBtz/tempest/internal/config"
	"github.com/HeartBtz/tempest/internal/protocol/bencode"
)

const (
	maxTrackerResponseSize          = 2 << 20
	trackerRequestTimeout           = 30 * time.Second
	maxTrackerIntervalSeconds int64 = (1<<63 - 1) / int64(time.Second)
)

var (
	trackerURLPattern  = regexp.MustCompile(`https?://[^\s]+`)
	passkeyPattern     = regexp.MustCompile(`(?i)(passkey|auth|token|key)=([^&\s]+)`)
	specialUsePrefixes = []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/8"),
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("100.64.0.0/10"),
		netip.MustParsePrefix("127.0.0.0/8"),
		netip.MustParsePrefix("169.254.0.0/16"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.0.0.0/24"),
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("192.31.196.0/24"),
		netip.MustParsePrefix("192.52.193.0/24"),
		netip.MustParsePrefix("192.88.99.0/24"),
		netip.MustParsePrefix("192.168.0.0/16"),
		netip.MustParsePrefix("192.175.48.0/24"),
		netip.MustParsePrefix("198.18.0.0/15"),
		netip.MustParsePrefix("198.51.100.0/24"),
		netip.MustParsePrefix("203.0.113.0/24"),
		netip.MustParsePrefix("224.0.0.0/4"),
		netip.MustParsePrefix("240.0.0.0/4"),
		netip.MustParsePrefix("64:ff9b::/96"),
		netip.MustParsePrefix("64:ff9b:1::/48"),
		netip.MustParsePrefix("100::/64"),
		netip.MustParsePrefix("2001::/23"),
		netip.MustParsePrefix("2001:db8::/32"),
		netip.MustParsePrefix("2002::/16"),
		netip.MustParsePrefix("2620:4f:8000::/48"),
		netip.MustParsePrefix("3fff::/20"),
		netip.MustParsePrefix("5f00::/16"),
		netip.MustParsePrefix("fc00::/7"),
		netip.MustParsePrefix("fe80::/10"),
		netip.MustParsePrefix("fec0::/10"),
		netip.MustParsePrefix("ff00::/8"),
	}
)

type AnnounceEvent string

const (
	EventStarted   AnnounceEvent = "started"
	EventStopped   AnnounceEvent = "stopped"
	EventCompleted AnnounceEvent = "completed"
	EventNone      AnnounceEvent = ""
)

type AnnounceRequest struct {
	TrackerURL string
	InfoHash   [20]byte
	PeerID     string
	Port       int
	Uploaded   int64
	Downloaded int64
	Left       int64
	Event      AnnounceEvent
	Compact    bool
	NumWant    int
	Key        string
	Profile    client.Profile
}

type AnnounceResponse struct {
	Interval       int    `json:"interval"`
	MinInterval    int    `json:"min_interval"`
	Complete       int    `json:"complete"`
	Incomplete     int    `json:"incomplete"`
	Peers          []Peer `json:"peers"`
	FailureReason  string `json:"failure_reason,omitempty"`
	WarningMessage string `json:"warning_message,omitempty"`
}

type Peer struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
	ID   string `json:"id,omitempty"`
}

type TrackerClient struct {
	httpClient   *http.Client
	allowPrivate bool
	setupErr     error
}

func NewTrackerClient() *TrackerClient {
	return newTrackerClient("")
}

func NewTrackerClientWithInterface(ifaceName string) *TrackerClient {
	return newTrackerClient(ifaceName)
}

func newTrackerClient(ifaceName string) *TrackerClient {
	baseDialer := net.Dialer{Timeout: trackerRequestTimeout}
	var localIPs []netip.Addr
	var setupErr error
	if ifaceName != "" {
		localIPs, setupErr = interfaceAddresses(ifaceName)
	}

	allowPrivate := config.Get().Security.AllowPrivateTrackerDestinations
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			if setupErr != nil {
				return nil, setupErr
			}
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, fmt.Errorf("invalid tracker address")
			}
			ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
			if err != nil {
				return nil, fmt.Errorf("could not resolve tracker host")
			}
			for _, ip := range ips {
				if !allowPrivate && isRestrictedDestination(ip) {
					return nil, fmt.Errorf("tracker destination is blocked by SSRF policy")
				}
			}
			if len(ips) == 0 {
				return nil, fmt.Errorf("tracker host resolved to no addresses")
			}

			remoteIP := ips[0]
			dialer := baseDialer
			if ifaceName != "" {
				localIP, selectedRemote, ok := compatibleDialAddresses(localIPs, ips)
				if !ok {
					return nil, fmt.Errorf("tracker interface %q has no usable address compatible with tracker destination", ifaceName)
				}
				remoteIP = selectedRemote
				dialer.LocalAddr = &net.TCPAddr{IP: net.IP(localIP.AsSlice()), Zone: localIP.Zone()}
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(remoteIP.String(), port))
		},
	}

	tc := &TrackerClient{allowPrivate: allowPrivate, setupErr: setupErr}
	tc.httpClient = &http.Client{
		Timeout:   trackerRequestTimeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			return validateTrackerDestination(req.Context(), req.URL, allowPrivate)
		},
	}
	return tc
}

func (tc *TrackerClient) Announce(req AnnounceRequest) (*AnnounceResponse, error) {
	if tc.setupErr != nil {
		return nil, tc.setupErr
	}
	trackerURL, err := url.Parse(req.TrackerURL)
	if err != nil || (trackerURL.Scheme != "http" && trackerURL.Scheme != "https") {
		return nil, fmt.Errorf("unsupported tracker protocol")
	}
	validationCtx, cancel := initialTrackerValidationContext()
	defer cancel()
	if err := validateTrackerDestination(validationCtx, trackerURL, tc.allowPrivate); err != nil {
		return nil, err
	}
	return tc.announceHTTP(req)
}

func initialTrackerValidationContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), trackerRequestTimeout)
}

func (tc *TrackerClient) announceHTTP(req AnnounceRequest) (*AnnounceResponse, error) {
	url := tc.buildAnnounceURL(req)

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("User-Agent", req.Profile.UserAgent)
	httpReq.Header.Set("Connection", "close")

	resp, err := tc.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("announce request failed: %s", RedactSensitiveText(err.Error()))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTrackerResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if len(body) > maxTrackerResponseSize {
		return nil, fmt.Errorf("tracker response exceeds the 2 MiB limit")
	}

	return tc.parseResponse(body)
}

func validateTrackerDestination(ctx context.Context, trackerURL *url.URL, allowPrivate bool) error {
	if trackerURL == nil || (trackerURL.Scheme != "http" && trackerURL.Scheme != "https") || trackerURL.Hostname() == "" {
		return fmt.Errorf("invalid tracker URL")
	}
	if allowPrivate {
		return nil
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", trackerURL.Hostname())
	if err != nil {
		return fmt.Errorf("could not resolve tracker host")
	}
	for _, ip := range ips {
		if isRestrictedDestination(ip) {
			return fmt.Errorf("tracker destination is blocked by SSRF policy")
		}
	}
	if len(ips) == 0 {
		return fmt.Errorf("tracker host resolved to no addresses")
	}
	return nil
}

func isRestrictedDestination(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || !ip.IsGlobalUnicast() {
		return true
	}
	// IsGlobalUnicast includes many IANA special-purpose ranges that are not
	// safe public tracker destinations, so reject the full registries explicitly.
	for _, prefix := range specialUsePrefixes {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

func interfaceAddresses(ifaceName string) ([]netip.Addr, error) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return nil, fmt.Errorf("tracker interface %q is unavailable: %w", ifaceName, err)
	}
	if iface.Flags&net.FlagUp == 0 {
		return nil, fmt.Errorf("tracker interface %q is not up", ifaceName)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, fmt.Errorf("list addresses for tracker interface %q: %w", ifaceName, err)
	}

	localIPs := make([]netip.Addr, 0, len(addrs))
	for _, addr := range addrs {
		var ip net.IP
		switch value := addr.(type) {
		case *net.IPNet:
			ip = value.IP
		case *net.IPAddr:
			ip = value.IP
		}
		parsed, ok := netip.AddrFromSlice(ip)
		if !ok || parsed.IsUnspecified() || parsed.IsMulticast() {
			continue
		}
		parsed = parsed.Unmap()
		if parsed.Is6() && parsed.IsLinkLocalUnicast() {
			parsed = parsed.WithZone(iface.Name)
		}
		localIPs = append(localIPs, parsed)
	}
	if len(localIPs) == 0 {
		return nil, fmt.Errorf("tracker interface %q has no usable IP address", ifaceName)
	}
	return localIPs, nil
}

func compatibleDialAddresses(localIPs, remoteIPs []netip.Addr) (netip.Addr, netip.Addr, bool) {
	for _, remoteIP := range remoteIPs {
		remoteIP = remoteIP.Unmap()
		for _, localIP := range localIPs {
			if localIP.Is4() == remoteIP.Is4() {
				return localIP, remoteIP, true
			}
		}
	}
	return netip.Addr{}, netip.Addr{}, false
}

func RedactTrackerURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return "[redacted tracker URL]"
	}
	return u.Scheme + "://" + u.Host + "/[redacted]"
}

func RedactSensitiveText(text string) string {
	text = trackerURLPattern.ReplaceAllStringFunc(text, RedactTrackerURL)
	return passkeyPattern.ReplaceAllString(text, "$1=[redacted]")
}

func (tc *TrackerClient) buildAnnounceURL(req AnnounceRequest) string {
	sep := "?"
	if strings.Contains(req.TrackerURL, "?") {
		sep = "&"
	}

	var sb strings.Builder
	sb.WriteString(req.TrackerURL)
	sb.WriteString(sep)

	infoHashEncoded := urlEncodeBytes(req.InfoHash[:])
	peerIDEncoded := client.PeerIDURLEncoded(req.PeerID)

	params := map[string]string{
		"info_hash":     infoHashEncoded,
		"peer_id":       peerIDEncoded,
		"port":          fmt.Sprintf("%d", req.Port),
		"uploaded":      fmt.Sprintf("%d", req.Uploaded),
		"downloaded":    fmt.Sprintf("%d", req.Downloaded),
		"left":          fmt.Sprintf("%d", req.Left),
		"key":           req.Key,
		"numwant":       fmt.Sprintf("%d", req.NumWant),
		"compact":       "1",
		"no_peer_id":    "1",
		"supportcrypto": "1",
		"corrupt":       "0",
		"redundant":     "0",
	}

	if req.Event != EventNone {
		params["event"] = string(req.Event)
	}

	if !req.Compact {
		params["compact"] = "0"
	}

	// Build query string in profile-specified order
	first := true
	for _, key := range req.Profile.QueryOrder {
		val, ok := params[key]
		if !ok {
			continue
		}
		if !first {
			sb.WriteString("&")
		}
		sb.WriteString(key)
		sb.WriteString("=")
		sb.WriteString(val)
		first = false
		delete(params, key)
	}

	// Append remaining params
	for key, val := range params {
		if !first {
			sb.WriteString("&")
		}
		sb.WriteString(key)
		sb.WriteString("=")
		sb.WriteString(val)
		first = false
	}

	return sb.String()
}

func (tc *TrackerClient) parseResponse(data []byte) (*AnnounceResponse, error) {
	decoded, err := bencode.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode tracker response: %w", err)
	}

	dict, ok := decoded.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("tracker response is not a dictionary")
	}

	resp := &AnnounceResponse{}

	if failure, ok := dict["failure reason"]; ok {
		if f, ok := failure.([]byte); ok {
			resp.FailureReason = string(f)
			return resp, nil
		}
	}

	if warning, ok := dict["warning message"]; ok {
		if w, ok := warning.([]byte); ok {
			resp.WarningMessage = string(w)
		}
	}

	if interval, ok := dict["interval"]; ok {
		if i, ok := interval.(int64); ok {
			resp.Interval = safeTrackerInterval(i)
		}
	}

	if minInterval, ok := dict["min interval"]; ok {
		if i, ok := minInterval.(int64); ok {
			resp.MinInterval = safeTrackerInterval(i)
		}
	}

	if complete, ok := dict["complete"]; ok {
		if c, ok := complete.(int64); ok {
			resp.Complete = int(c)
		}
	}

	if incomplete, ok := dict["incomplete"]; ok {
		if i, ok := incomplete.(int64); ok {
			resp.Incomplete = int(i)
		}
	}

	// Parse peers - compact format (6 bytes per peer: 4 IP + 2 port)
	if peers, ok := dict["peers"]; ok {
		switch p := peers.(type) {
		case []byte:
			resp.Peers = parseCompactPeers(p)
		case []interface{}:
			for _, peer := range p {
				if pd, ok := peer.(map[string]interface{}); ok {
					pr := Peer{}
					if ip, ok := pd["ip"].([]byte); ok {
						pr.IP = string(ip)
					}
					if port, ok := pd["port"].(int64); ok {
						pr.Port = int(port)
					}
					if id, ok := pd["peer id"].([]byte); ok {
						pr.ID = string(id)
					}
					resp.Peers = append(resp.Peers, pr)
				}
			}
		}
	}

	return resp, nil
}

func safeTrackerInterval(seconds int64) int {
	if seconds <= 0 {
		return 0
	}
	maxSeconds := maxTrackerIntervalSeconds
	if maxInt := int64(^uint(0) >> 1); maxSeconds > maxInt {
		maxSeconds = maxInt
	}
	if seconds > maxSeconds {
		seconds = maxSeconds
	}
	return int(seconds)
}

func parseCompactPeers(data []byte) []Peer {
	peers := make([]Peer, 0, len(data)/6)
	for i := 0; i+5 < len(data); i += 6 {
		ip := net.IPv4(data[i], data[i+1], data[i+2], data[i+3])
		port := int(data[i+4])<<8 | int(data[i+5])
		peers = append(peers, Peer{
			IP:   ip.String(),
			Port: port,
		})
	}
	return peers
}

func urlEncodeBytes(data []byte) string {
	var sb strings.Builder
	for _, b := range data {
		sb.WriteString(fmt.Sprintf("%%%02x", b))
	}
	return sb.String()
}
