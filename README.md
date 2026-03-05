# Tempest ⚡

**Modern BitTorrent Tracker Simulation Tool**

A next-generation, open-source tool for simulating BitTorrent announce requests. Built for educational purposes, tracker testing, and protocol experimentation. Replaces legacy tools like RatioMaster and JOAL with a clean, modern architecture.

![Go](https://img.shields.io/badge/Go-1.19+-00ADD8?logo=go&logoColor=white)
![React](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=white)
![SQLite](https://img.shields.io/badge/SQLite-WAL-003B57?logo=sqlite&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-green)
![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker&logoColor=white)

---

## Table of Contents

- [Features](#-features)
- [Screenshots](#-screenshots)
- [Quick Start](#-quick-start)
- [Installation](#-installation)
- [Configuration](#-configuration)
- [Usage Guide](#-usage-guide)
- [API Reference](#-api-reference)
- [Client Profiles](#-client-profiles)
- [Architecture](#-architecture)
- [Project Structure](#-project-structure)
- [Security](#-security)
- [Docker](#-docker)
- [Development](#-development)
- [FAQ](#-faq)
- [Disclaimer](#%EF%B8%8F-disclaimer)
- [License](#-license)

---

## ⚡ Features

### Core
- **Multi-Torrent Sessions** — Simulate hundreds of concurrent torrent sessions
- **Client Emulation** — 5 built-in profiles (qBittorrent, Transmission, Deluge, Vuze) with accurate peer IDs, user agents, and query parameter ordering
- **Realistic Simulation** — Randomized speeds, intervals, and transfer amounts with configurable variance
- **Modern Web UI** — Dark-themed dashboard with real-time stats, session management, drag-and-drop uploads, and live logs
- **Single Binary** — One Go binary serves both the REST API and the React frontend
- **SQLite Storage** — Zero-config embedded database with WAL mode, no external DB needed

### Advanced
- **Speed Variance** — Configure ±absolute bytes/s randomization per session (e.g., ±10 MB/s)
- **Stop-at-Ratio** — Automatically stop sessions when a target upload/download ratio is reached
- **Upload/Download Limits** — Set maximum total bytes for upload and download per session
- **Network Interface Binding** — Route tracker announces through a specific network interface (VPN support)
- **Live Log Streaming** — SSE-based real-time log feed
- **Docker Support** — Multi-stage Dockerfile and docker-compose included

---

## 📸 Screenshots

The web UI features a dark-themed dashboard with:
- **Stats Bar** — Total torrents, active sessions, uploaded, and downloaded
- **Session List** — Table view with status, client profile, speeds, ratio, variance, limits, seeders/leechers, and per-session controls
- **Torrent Manager** — Drag-and-drop `.torrent` file upload with deduplication
- **Log Viewer** — Color-coded, reverse-chronological event log
- **Session Creator** — Full configuration modal with speed, variance, ratio, limits, and network interface selection

---

## 🚀 Quick Start

### Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.19+ | Backend compilation |
| Node.js | 18+ | Frontend build |
| npm | 9+ | Frontend dependencies |
| GCC | Any | CGO for SQLite driver |

### Build & Run

```bash
# Clone the repository
git clone https://github.com/tempest-bt/tempest.git
cd tempest

# Build everything (frontend + backend)
make all

# Run
make run
```

Open **http://127.0.0.1:8377** in your browser.

### Using Docker

```bash
docker-compose up -d
```

---

## 📦 Installation

### Option 1: Install Script (Recommended)

The install script auto-detects your init system and sets up Tempest as a service.

```bash
# As root — installs systemd service (Linux) or launchd (macOS)
sudo ./install.sh

# Without root — standalone mode with start/stop scripts
./install.sh --standalone

# Uninstall
sudo ./install.sh --uninstall
```

The install script:
- Builds the frontend and backend from source
- Creates a dedicated `tempest` system user
- Installs the binary to `/opt/tempest`
- Creates a config at `/etc/tempest/config.json`
- Stores data in `/var/lib/tempest`
- Logs to `/var/log/tempest`
- Sets up a systemd unit with security hardening (NoNewPrivileges, ProtectSystem, etc.)

**Environment variables** for customization:

| Variable | Default | Description |
|----------|---------|-------------|
| `TEMPEST_PORT` | `8377` | Listen port |
| `TEMPEST_HOST` | `127.0.0.1` | Bind address |
| `TEMPEST_INSTALL_DIR` | `/opt/tempest` | Binary location |
| `TEMPEST_DATA_DIR` | `/var/lib/tempest` | Database location |
| `TEMPEST_LOG_DIR` | `/var/log/tempest` | Log files |
| `TEMPEST_CONFIG_DIR` | `/etc/tempest` | Config location |
| `TEMPEST_USER` | `tempest` | System user |

### Option 2: Manual Build

```bash
# Build frontend
cd web && npm install && npm run build && cd ..

# Build backend
mkdir -p build
CGO_ENABLED=1 go build -ldflags "-s -w" -o build/tempest ./cmd/tempest

# Run (from project root — frontend is served from web/dist/)
./build/tempest
```

### Option 3: Docker

```bash
# Build image
docker build -t tempest:latest .

# Run container
docker run -d \
  --name tempest \
  -p 8377:8377 \
  -v tempest-data:/app/data \
  tempest:latest
```

---

## 🔧 Configuration

Create a `config.json` file or pass `--config path/to/config.json`:

```json
{
  "server": {
    "host": "127.0.0.1",
    "port": 8377
  },
  "database": {
    "path": "tempest.db"
  },
  "engine": {
    "max_concurrent_sessions": 500,
    "default_announce_port": 6881,
    "enable_randomization": true
  }
}
```

| Field | Description | Default |
|-------|-------------|---------|
| `server.host` | Bind address. Use `0.0.0.0` to expose to LAN | `127.0.0.1` |
| `server.port` | HTTP server port | `8377` |
| `database.path` | SQLite database file path | `tempest.db` |
| `engine.max_concurrent_sessions` | Max simultaneous sessions | `500` |
| `engine.default_announce_port` | Default port announced to trackers | `6881` |
| `engine.enable_randomization` | Enable speed/interval randomization | `true` |

> **Note:** If no config file is provided, defaults are used. The database is created automatically.

---

## 📖 Usage Guide

### 1. Add Torrents

- Go to the **Torrents** tab
- Drag and drop `.torrent` files onto the upload zone, or click to browse
- Torrents are parsed and stored in the database (deduplicated by info hash)

### 2. Create a Session

- Go to the **Sessions** tab
- Click **+ New Session**
- Configure:

| Setting | Description |
|---------|-------------|
| **Torrent** | Select from uploaded torrents |
| **Client Profile** | BitTorrent client to emulate |
| **Upload Speed** | Base upload speed in bytes/s (e.g., `1048576` = 1 MB/s) |
| **Download Speed** | Base download speed in bytes/s (0 = seed only) |
| **Speed Variance** | ± randomization in bytes/s (e.g., `524288` = ±512 KB/s) |
| **Target Ratio** | Desired upload/download ratio |
| **Stop at Ratio** | Auto-stop when ratio is reached |
| **Max Upload** | Total upload cap in bytes (0 = unlimited) |
| **Max Download** | Total download cap in bytes (0 = unlimited) |
| **Network Interface** | Bind to specific NIC for VPN routing |

### 3. Start/Stop Sessions

- Click the **▶ Start** button to begin announcing to the tracker
- The session will simulate data transfer between announce intervals
- Click **■ Stop** to send a `stopped` event to the tracker
- Sessions auto-stop when configured limits or ratios are reached (status: `completed`)

### 4. Monitor

- **Stats Bar** shows global totals
- **Session List** shows per-session upload/download/ratio/speeds
- **Logs** tab shows announce events, tracker responses, and errors

---

## 📡 API Reference

All endpoints return JSON. Base URL: `http://127.0.0.1:8377`

### Torrents

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/torrents` | List all torrents |
| `POST` | `/api/torrents` | Upload `.torrent` file (multipart form, field: `torrent`) |
| `GET` | `/api/torrents/:id` | Get torrent details |
| `DELETE` | `/api/torrents/:id` | Delete a torrent |

### Sessions

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/sessions` | List all sessions (with torrent info) |
| `POST` | `/api/sessions` | Create new session |
| `GET` | `/api/sessions/:id` | Get session details |
| `PUT` | `/api/sessions/:id` | Update session config |
| `POST` | `/api/sessions/:id/start` | Start session (begins announcing) |
| `POST` | `/api/sessions/:id/stop` | Stop session (sends stopped event) |
| `DELETE` | `/api/sessions/:id` | Delete session (stops if running) |

#### Create Session Request Body

```json
{
  "torrent_id": "1234567890",
  "client_profile": "qbittorrent-4.6.2",
  "upload_speed": 1048576,
  "download_speed": 0,
  "speed_variance": 524288,
  "target_ratio": 2.0,
  "stop_at_ratio": true,
  "max_upload": 10737418240,
  "max_download": 0,
  "network_interface": "tun0",
  "port": 6881
}
```

#### Update Session Request Body

All fields are optional (only provided fields are updated):

```json
{
  "upload_speed": 2097152,
  "speed_variance": 1048576,
  "stop_at_ratio": false
}
```

### Stats & Logs

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/stats` | Global statistics (torrents, sessions, totals) |
| `GET` | `/api/logs?limit=100` | Recent log entries |
| `GET` | `/api/ws/logs` | SSE stream for live logs (`text/event-stream`) |

### System

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/profiles` | List available client profiles |
| `GET` | `/api/interfaces` | List network interfaces (name, IPs, flags) |

---

## 🎭 Client Profiles

Tempest emulates real BitTorrent clients by matching their:
- **Peer ID format** (prefix + random suffix, 20 bytes)
- **User-Agent header**
- **Query parameter ordering** in announce URLs
- **Key format** (hex, uppercase/lowercase)

| Profile ID | Client | Peer ID | User-Agent |
|------------|--------|---------|------------|
| `qbittorrent-4.6.2` | qBittorrent 4.6.2 | `-qB4620-xxxxxxxxxxxx` | `qBittorrent/4.6.2` |
| `qbittorrent-5.0.0` | qBittorrent 5.0.0 | `-qB5000-xxxxxxxxxxxx` | `qBittorrent/5.0.0` |
| `transmission-4.0.0` | Transmission 4.0.0 | `-TR4000-xxxxxxxxxxxx` | `Transmission/4.0.0` |
| `deluge-2.1.1` | Deluge 2.1.1 | `-DE211s-xxxxxxxxxxxx` | `Deluge/2.1.1 libtorrent/2.0.9.0` |
| `vuze-5.7.7` | Vuze 5.7.7 | `-AZ5770-xxxxxxxxxxxx` | `Azureus 5.7.7.0` |

---

## 🏗️ Architecture

```
┌──────────────────────────────────────────────────┐
│              React Frontend (Vite)                │
│   Dashboard │ Sessions │ Torrents │ Logs │ Upload │
└──────────────────────┬───────────────────────────┘
                       │ REST API (JSON)
┌──────────────────────┴───────────────────────────┐
│             Go Backend (net/http stdlib)          │
├──────────────────────────────────────────────────┤
│  API Layer         │ Routes, Middleware, SSE      │
├──────────────────────────────────────────────────┤
│  Simulation Engine │ Session Runner, Manager      │
│                    │ Concurrent goroutines        │
├──────────────────────────────────────────────────┤
│  Protocol Layer    │ HTTP Announce, Bencode       │
│                    │ Torrent Parser, Tracker      │
├──────────────────────────────────────────────────┤
│  Client Emulation  │ 5 Profiles, Randomizer      │
│                    │ PeerID/Key Generation        │
├──────────────────────────────────────────────────┤
│  Storage Layer     │ SQLite (WAL), Models, CRUD   │
└──────────────────────────────────────────────────┘
```

### How It Works

1. **Upload** a `.torrent` file → parsed and stored in SQLite
2. **Create a session** → select torrent, client profile, speeds, limits
3. **Start the session** → spawns a goroutine that:
   - Sends a `started` announce to the tracker
   - Receives tracker response (interval, peers, seeders/leechers)
   - Simulates data transfer between announcements (with randomized speed + variance)
   - Sends periodic announces with updated stats
   - Sends `completed` event when download finishes (once)
   - Auto-stops if ratio/limits are reached
   - Sends `stopped` event on stop

### Concurrency Model

Each session runs in its own goroutine with:
- A `stopCh` channel for clean shutdown
- Mutex-protected state
- Non-blocking log pub/sub
- Independent HTTP client (optionally bound to a specific network interface)

---

## 📁 Project Structure

```
tempest/
├── cmd/tempest/
│   └── main.go                    # Entry point, config, graceful shutdown
├── internal/
│   ├── api/
│   │   ├── server.go              # HTTP server, route setup, static files
│   │   ├── middleware.go          # CORS, logging, security headers
│   │   └── handler/
│   │       ├── torrent.go         # Torrent CRUD + file upload
│   │       ├── session.go         # Session CRUD + start/stop
│   │       ├── stats.go           # Global stats, logs, SSE streaming
│   │       ├── profile.go         # Client profile listing
│   │       └── network.go         # Network interface enumeration
│   ├── engine/
│   │   ├── session.go             # Session runner (goroutine-based)
│   │   └── manager.go             # Session lifecycle, log ring buffer
│   ├── protocol/
│   │   ├── announce.go            # HTTP tracker client, response parsing
│   │   ├── torrent.go             # .torrent file parser
│   │   └── bencode/
│   │       ├── decode.go          # Bencode decoder (with size limits)
│   │       └── encode.go          # Bencode encoder
│   ├── client/
│   │   ├── profile.go             # 5 client profiles, PeerID/Key gen
│   │   └── randomizer.go          # Speed/interval randomization
│   ├── storage/
│   │   ├── database.go            # SQLite CRUD, migrations
│   │   └── models.go              # Data structures
│   └── config/
│       └── config.go              # JSON config with defaults
├── web/                           # React frontend (Vite + TypeScript)
│   ├── src/
│   │   ├── App.tsx                # Main app, routing, styles
│   │   ├── components/
│   │   │   ├── StatsBar.tsx       # Dashboard stats
│   │   │   ├── SessionList.tsx    # Session table
│   │   │   ├── TorrentUpload.tsx  # Drag & drop upload
│   │   │   ├── LogViewer.tsx      # Log display
│   │   │   └── CreateSessionModal.tsx  # Session config form
│   │   ├── api/client.ts          # Typed API client
│   │   └── types/index.ts         # TypeScript interfaces
│   ├── package.json
│   ├── vite.config.ts
│   └── tsconfig.json
├── install.sh                     # Install script (systemd/launchd/standalone)
├── Makefile                       # Build targets
├── Dockerfile                     # Multi-stage Docker build
├── docker-compose.yml             # Docker Compose config
├── .gitignore
├── .editorconfig
├── LICENSE                        # MIT
└── README.md
```

---

## 🔒 Security

### Default Security Posture

- **Binds to localhost** (`127.0.0.1`) by default — not exposed to the network
- **Security headers** — `X-Content-Type-Options`, `X-Frame-Options`, `X-XSS-Protection`, `Referrer-Policy`
- **Error sanitization** — Internal errors are logged server-side, not exposed to API clients
- **Bencode limits** — String length capped at 64 MB, integer length capped at 32 digits (prevents memory bombs)
- **Upload limit** — Torrent file uploads capped at 10 MB
- **Path sanitization** — Static file serving uses `filepath.Clean` to prevent traversal
- **Systemd hardening** — Install script sets `NoNewPrivileges`, `ProtectSystem=strict`, `ProtectHome`, `PrivateTmp`

### Recommendations for Production Use

> **Tempest is designed for local use.** If you need to expose it:

1. **Use a reverse proxy** (nginx/Caddy) with TLS
2. **Add authentication** — Tempest has no built-in auth; put it behind HTTP Basic Auth or an auth proxy
3. **Firewall** — Only allow trusted IPs to access port 8377
4. **Don't expose to the internet** — This is a local tool, not a web service

### Known Limitations

- No built-in authentication or authorization
- No HTTPS (use a reverse proxy)
- CORS allows the request origin (for development convenience)
- Network interface enumeration is available without auth

---

## 🐳 Docker

### docker-compose

```yaml
version: '3.8'
services:
  tempest:
    build: .
    container_name: tempest
    ports:
      - "8377:8377"
    volumes:
      - tempest-data:/app/data
    restart: unless-stopped
volumes:
  tempest-data:
```

```bash
# Start
docker-compose up -d

# Logs
docker-compose logs -f

# Stop
docker-compose down
```

### Manual Docker Build

```bash
docker build -t tempest:latest .
docker run -d --name tempest -p 8377:8377 tempest:latest
```

---

## 🛠️ Development

### Dev Mode (Hot Reload)

```bash
# Terminal 1: Backend
make dev-backend

# Terminal 2: Frontend (proxies API to :8377)
make dev-frontend
```

Frontend dev server runs on `http://localhost:5173` with hot reload.
API requests are proxied to the Go backend on `:8377`.

### Build Commands

```bash
make all            # Build frontend + backend
make backend        # Build Go binary only
make frontend       # Build React frontend only
make run            # Build & run
make clean          # Remove build artifacts
make docker         # Build Docker image
make test           # Run Go tests
make help           # Show all commands
```

### Adding a Client Profile

Edit `internal/client/profile.go` and add to the `profiles` map:

```go
"myclient-1.0.0": {
    Name:           "MyClient",
    Version:        "1.0.0",
    PeerIDPrefix:   "-MC1000-",
    UserAgent:      "MyClient/1.0.0",
    KeyLength:      8,
    KeyUpperCase:   false,
    NumWantDefault:  200,
    QueryOrder:     []string{"info_hash", "peer_id", "port", ...},
    SupportsCompact: true,
},
```

---

## ❓ FAQ

**Q: Does this actually download/upload files?**
No. Tempest only communicates with BitTorrent trackers via HTTP announce requests. It does not connect to peers, and no file data is transferred.

**Q: Is this legal?**
Tempest is designed for educational and testing purposes. Using it to manipulate tracker statistics without authorization may violate terms of service. Use responsibly.

**Q: Can I use this with UDP trackers?**
Not yet. Only HTTP/HTTPS trackers are currently supported. UDP tracker support is planned.

**Q: How many sessions can I run?**
The default limit is 500 concurrent sessions. Each session uses one goroutine and minimal memory. The binary itself is ~8 MB.

**Q: Where is the database stored?**
By default, `tempest.db` is created in the working directory. Configure via `config.json` or the install script.

**Q: Do I need to delete the database when upgrading?**
If the database schema changes between versions, yes. There is no automated migration system yet. Back up your data and delete `tempest.db*` files.

---

## ⚠️ Disclaimer

This tool is designed **exclusively for educational and testing purposes**:

- Testing tracker implementations
- Learning about the BitTorrent protocol
- Protocol experimentation in controlled environments

**Do not use this tool to violate any terms of service, laws, or regulations.** The authors are not responsible for misuse.

---

## 📄 License

MIT License — see [LICENSE](LICENSE) for details.

---

**Made with ⚡ by the Tempest community**
