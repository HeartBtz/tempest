# Tempest ⚡

**A friendly dashboard for testing BitTorrent tracker announces**

Tempest is a self-hosted web application for simulating BitTorrent announce activity in a clean, modern interface. It helps you upload `.torrent` metadata, create simulated sessions, adjust global behavior, and watch announce traffic from a browser.

Tempest is best suited for **local labs, QA environments, protocol experiments, demos, and development work**.

![Go](https://img.shields.io/badge/Go-1.19+-00ADD8?logo=go&logoColor=white)
![React](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=white)
![SQLite](https://img.shields.io/badge/SQLite-Embedded-003B57?logo=sqlite&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-green)
![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker&logoColor=white)

---

## Why people like Tempest

- **Simple web UI** — upload torrents, create sessions, and monitor activity from one dashboard
- **Fast to install** — use the install script, Docker, or a manual build
- **Single-binary backend** — no external database or extra services required
- **Embedded SQLite storage** — easy local setup, easy backup
- **Live visibility** — watch stats, logs, seeders/leechers, and announce timing in real time
- **Global controls** — configure upload/download behavior once and share it across all running sessions
- **API included** — every major action is also available through `/api`

---

## What Tempest does

Tempest focuses on the **tracker side** of BitTorrent workflows.

It can:
- store and inspect `.torrent` metadata
- send HTTP/HTTPS announce requests to trackers
- simulate uploaded/downloaded counters between announce intervals
- emulate different BitTorrent client profiles
- show live logs and session statistics in the browser
- optionally bind traffic to a specific network interface on multi-homed systems

It does **not**:
- download torrent payloads
- connect to peers
- transfer file data
- require PostgreSQL, Redis, or any other external service

---

## Quick start

### Prerequisites

| Tool | Version |
|------|---------|
| Go | 1.19+ |
| Node.js | 18+ |
| npm | 9+ |
| gcc | Any recent version (required for SQLite/CGO) |

### Run from source

```bash
git clone https://github.com/tempest-bt/tempest.git
cd tempest
make all
make run
```

Then open:

**http://127.0.0.1:8377**

---

## Installation options

### 1) Install as a service

On Linux and macOS, the installer can build Tempest and register it as a managed service.

```bash
sudo ./install.sh
```

Useful defaults:
- **Binary**: `/opt/tempest`
- **Config**: `/etc/tempest/config.json`
- **Data**: `/var/lib/tempest/tempest.db`
- **Logs**: `/var/log/tempest/`
- **Web UI**: `http://127.0.0.1:8377`

Re-running the installer is safe: it preserves an existing config file and refreshes the built assets/binary.

### 2) Standalone install

If you do not want a system service:

```bash
./install.sh --standalone
```

### 3) Docker

```bash
docker compose up -d --build
```

### 4) Manual build

```bash
cd web && npm install && npm run build && cd ..
mkdir -p build
CGO_ENABLED=1 go build -ldflags "-s -w" -o build/tempest ./cmd/tempest
./build/tempest
```

---

## First run walkthrough

### 1. Upload one or more torrents
Open the **Torrents** tab and add `.torrent` files.

### 2. Open **Settings**
Choose the client profile and define the global upload/download behavior.

> Upload and download speeds are **shared across all running sessions**.
> If you configure `30 MB/s` of upload and start multiple sessions, Tempest distributes that total across them.

### 3. Create sessions
Go to **Sessions**, click **New Session**, and choose which torrent you want to simulate.

### 4. Start sessions
Use the **Start** button to begin announces.

### 5. Watch the dashboard
You can track:
- total uploaded/downloaded counters
- allocated speed per running session
- seeders/leechers returned by the tracker
- announce intervals
- recent logs and errors

---

## Configuration basics

Tempest can run with defaults, but you can also provide a config file:

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

### Common install-time environment variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `TEMPEST_PORT` | `8377` | HTTP port |
| `TEMPEST_HOST` | `127.0.0.1` | Bind address |
| `TEMPEST_INSTALL_DIR` | `/opt/tempest` | Install location |
| `TEMPEST_DATA_DIR` | `/var/lib/tempest` | Data directory |
| `TEMPEST_LOG_DIR` | `/var/log/tempest` | Log directory |
| `TEMPEST_CONFIG_DIR` | `/etc/tempest` | Config directory |
| `TEMPEST_USER` | `tempest` | Service account |

---

## REST API

Tempest exposes a small JSON API under `/api`.

Main endpoints:
- `/api/torrents`
- `/api/sessions`
- `/api/settings`
- `/api/stats`
- `/api/logs`
- `/api/profiles`
- `/api/interfaces`

This makes it easy to integrate Tempest into test pipelines or custom dashboards.

---

## Development

### Common commands

```bash
make all
make backend
make frontend
make run
make test
make docker
```

### Frontend / backend split in development

```bash
# Terminal 1
make dev-backend

# Terminal 2
cd web && npm run dev
```

---

## FAQ

### Does Tempest download files?
No. Tempest does not fetch torrent payload data.

### Does Tempest connect to peers?
No. Tempest focuses on tracker announces only.

### Which trackers are supported today?
HTTP and HTTPS trackers. UDP tracker support is not implemented yet.

### Is there a web interface?
Yes. The default UI is served from the same application on port `8377`.

### Can I run it as a background service?
Yes. The install script supports managed service installation, and Docker is also supported.

### Where is my data stored?
By default, Tempest uses SQLite.

---

## Responsible use

Use Tempest only with systems and services you are authorized to test.

---

## License

MIT — see [LICENSE](LICENSE).
