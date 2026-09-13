package client

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

type Profile struct {
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	PeerIDPrefix    string   `json:"peer_id_prefix"`
	UserAgent       string   `json:"user_agent"`
	KeyLength       int      `json:"key_length"`
	KeyUpperCase    bool     `json:"key_upper_case"`
	NumWantDefault  int      `json:"numwant_default"`
	QueryOrder      []string `json:"query_order"`
	SupportsCompact bool     `json:"supports_compact"`
}

var profiles = map[string]Profile{
	"qbittorrent-4.6.2": {
		Name:            "qBittorrent",
		Version:         "4.6.2",
		PeerIDPrefix:    "-qB4620-",
		UserAgent:       "qBittorrent/4.6.2",
		KeyLength:       8,
		KeyUpperCase:    false,
		NumWantDefault:  200,
		QueryOrder:      []string{"info_hash", "peer_id", "port", "uploaded", "downloaded", "left", "corrupt", "key", "event", "numwant", "compact", "no_peer_id", "supportcrypto", "redundant"},
		SupportsCompact: true,
	},
	"qbittorrent-5.0.0": {
		Name:            "qBittorrent",
		Version:         "5.0.0",
		PeerIDPrefix:    "-qB5000-",
		UserAgent:       "qBittorrent/5.0.0",
		KeyLength:       8,
		KeyUpperCase:    false,
		NumWantDefault:  200,
		QueryOrder:      []string{"info_hash", "peer_id", "port", "uploaded", "downloaded", "left", "corrupt", "key", "event", "numwant", "compact", "no_peer_id", "supportcrypto", "redundant"},
		SupportsCompact: true,
	},
	"transmission-4.0.0": {
		Name:            "Transmission",
		Version:         "4.0.0",
		PeerIDPrefix:    "-TR4000-",
		UserAgent:       "Transmission/4.0.0",
		KeyLength:       8,
		KeyUpperCase:    false,
		NumWantDefault:  80,
		QueryOrder:      []string{"info_hash", "peer_id", "port", "uploaded", "downloaded", "left", "numwant", "key", "compact", "supportcrypto", "event"},
		SupportsCompact: true,
	},
	"deluge-2.1.1": {
		Name:            "Deluge",
		Version:         "2.1.1",
		PeerIDPrefix:    "-DE211s-",
		UserAgent:       "Deluge/2.1.1 libtorrent/2.0.9.0",
		KeyLength:       8,
		KeyUpperCase:    true,
		NumWantDefault:  200,
		QueryOrder:      []string{"info_hash", "peer_id", "port", "uploaded", "downloaded", "left", "event", "key", "numwant", "compact"},
		SupportsCompact: true,
	},
	"vuze-5.7.7": {
		Name:            "Vuze",
		Version:         "5.7.7",
		PeerIDPrefix:    "-AZ5770-",
		UserAgent:       "Azureus 5.7.7.0",
		KeyLength:       8,
		KeyUpperCase:    false,
		NumWantDefault:  50,
		QueryOrder:      []string{"info_hash", "peer_id", "port", "uploaded", "downloaded", "left", "numwant", "key", "compact", "event"},
		SupportsCompact: true,
	},
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func GetProfile(name string) (Profile, bool) {
	p, ok := profiles[name]
	return p, ok
}

func ListProfiles() []Profile {
	result := make([]Profile, 0, len(profiles))
	for _, p := range profiles {
		result = append(result, p)
	}
	return result
}

func ListProfileNames() []string {
	result := make([]string, 0, len(profiles))
	for name := range profiles {
		result = append(result, name)
	}
	return result
}

func DefaultProfile() Profile {
	return profiles["qbittorrent-4.6.2"]
}

func (p Profile) GeneratePeerID() string {
	const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	suffix := make([]byte, 20-len(p.PeerIDPrefix))
	for i := range suffix {
		suffix[i] = charset[rand.Intn(len(charset))]
	}
	return p.PeerIDPrefix + string(suffix)
}

func (p Profile) GenerateKey() string {
	var charset string
	if p.KeyUpperCase {
		charset = "0123456789ABCDEF"
	} else {
		charset = "0123456789abcdef"
	}

	key := make([]byte, p.KeyLength)
	for i := range key {
		key[i] = charset[rand.Intn(len(charset))]
	}
	return string(key)
}

func PeerIDURLEncoded(peerID string) string {
	var sb strings.Builder
	for _, b := range []byte(peerID) {
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '-' || b == '_' || b == '.' || b == '~' {
			sb.WriteByte(b)
		} else {
			sb.WriteString(fmt.Sprintf("%%%02X", b))
		}
	}
	return sb.String()
}
