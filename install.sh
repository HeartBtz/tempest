#!/usr/bin/env bash
#
# Tempest ⚡ — Install Script
#
# This script builds and installs Tempest as a system service.
# Supports: systemd (Linux), launchd (macOS), or standalone mode.
#
# Usage:
#   sudo ./install.sh              # Install with auto-detection
#   sudo ./install.sh --uninstall  # Remove installation
#   ./install.sh --standalone      # Run without service manager (no root needed)
#
set -euo pipefail

# --- Ensure we run from the script's directory ---
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# --- Configuration ---
# Derive the app name from the directory name (e.g. "tempest-dev", "tempest-prod")
# so multiple instances can coexist on the same host with isolated paths.
APP_NAME="${TEMPEST_APP_NAME:-$(basename "$SCRIPT_DIR")}"
APP_USER="${TEMPEST_USER:-${APP_NAME}}"
INSTALL_DIR="${TEMPEST_INSTALL_DIR:-/opt/${APP_NAME}}"
DATA_DIR="${TEMPEST_DATA_DIR:-/var/lib/${APP_NAME}}"
LOG_DIR="${TEMPEST_LOG_DIR:-/var/log/${APP_NAME}}"
CONFIG_DIR="${TEMPEST_CONFIG_DIR:-/etc/${APP_NAME}}"
PORT="${TEMPEST_PORT:-8377}"
HOST="${TEMPEST_HOST:-127.0.0.1}"

# --- Colors ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info() { echo -e "${BLUE}[INFO]${NC}  $*"; }
ok() { echo -e "${GREEN}[OK]${NC}    $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }
fatal() {
	error "$@"
	exit 1
}

# --- Helpers ---
command_exists() { command -v "$1" >/dev/null 2>&1; }

# Read a value from stdin, showing a prompt and default.
# Usage: prompt_value "Label" "default" [validator_fn]
# Prints the chosen value to stdout; logs go to stderr.
prompt_value() {
	local label="$1" default="$2" validator="${3:-}"
	local value
	while true; do
		read -rp "$(echo -e "  ${BLUE}${label}${NC} [${default}]: ")" value
		value="${value:-$default}"
		if [[ -z "$validator" ]] || $validator "$value"; then
			echo "$value"
			return
		fi
		echo -e "  ${RED}Invalid value.${NC} Try again." >&2
	done
}

validate_port() {
	local p="$1"
	[[ "$p" =~ ^[0-9]+$ ]] && ((p >= 1 && p <= 65535))
}

validate_host() {
	# Accept any non-empty string (hostname, IP, 0.0.0.0 …)
	[[ -n "$1" ]]
}

validate_name() {
	# Only allow alphanumeric + dash/underscore
	[[ "$1" =~ ^[a-zA-Z0-9_-]+$ ]]
}

# --- Interactive configuration prompt ---
# Called once at the start of a normal install.
# Values already set via environment variables are kept as-is.
prompt_interactive_config() {
	echo ""
	echo -e "${YELLOW}  ── Instance configuration ──────────────────────────────────${NC}"
	echo -e "  (Press Enter to accept the default shown in brackets)"
	echo ""

	# Instance / app name
	if [[ -z "${TEMPEST_APP_NAME:-}" ]]; then
		APP_NAME="$(prompt_value "Instance name" "$APP_NAME" validate_name)"
	else
		echo -e "  ${BLUE}Instance name${NC}: $APP_NAME  (from TEMPEST_APP_NAME)"
	fi

	# Port
	if [[ -z "${TEMPEST_PORT:-}" ]]; then
		PORT="$(prompt_value "HTTP port" "$PORT" validate_port)"
	else
		echo -e "  ${BLUE}HTTP port${NC}: $PORT  (from TEMPEST_PORT)"
	fi

	# Host / bind address
	if [[ -z "${TEMPEST_HOST:-}" ]]; then
		HOST="$(prompt_value "Bind address" "$HOST" validate_host)"
	else
		echo -e "  ${BLUE}Bind address${NC}: $HOST  (from TEMPEST_HOST)"
	fi

	# Recompute derived paths based on potentially updated APP_NAME
	INSTALL_DIR="${TEMPEST_INSTALL_DIR:-/opt/${APP_NAME}}"
	DATA_DIR="${TEMPEST_DATA_DIR:-/var/lib/${APP_NAME}}"
	LOG_DIR="${TEMPEST_LOG_DIR:-/var/log/${APP_NAME}}"
	CONFIG_DIR="${TEMPEST_CONFIG_DIR:-/etc/${APP_NAME}}"
	APP_USER="${TEMPEST_USER:-${APP_NAME}}"

	echo ""
	echo -e "${YELLOW}  ── Summary ─────────────────────────────────────────────────${NC}"
	echo -e "  Instance  : ${GREEN}${APP_NAME}${NC}"
	echo -e "  URL       : ${GREEN}http://${HOST}:${PORT}${NC}"
	echo -e "  Binary    : ${GREEN}${INSTALL_DIR}/tempest${NC}"
	echo -e "  Data      : ${GREEN}${DATA_DIR}${NC}"
	echo -e "  Config    : ${GREEN}${CONFIG_DIR}/config.json${NC}"
	echo -e "  Service   : ${GREEN}${APP_NAME}.service${NC}"
	echo ""

	local confirm
	read -rp "$(echo -e "  ${YELLOW}Proceed with installation? [Y/n]: ${NC}")" confirm
	confirm="${confirm:-Y}"
	if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
		echo "Installation cancelled."
		exit 0
	fi
	echo ""
}

# Copy build artifacts to target dir (skip if same directory)
install_files() {
	local target="$1"
	local src_real target_real
	src_real="$(cd "$SCRIPT_DIR" && pwd -P)"
	target_real="$(mkdir -p "$target" && cd "$target" && pwd -P)"

	# Always copy the binary from build/ to the target root
	cp build/tempest "$target/tempest"
	chmod 755 "$target/tempest"

	if [[ "$src_real" == "$target_real" ]]; then
		ok "Source and install dir are the same, skipping web copy"
		return
	fi

	mkdir -p "$target/web"
	rm -rf "$target/web/dist"
	cp -r web/dist "$target/web/"
}

check_root() {
	if [[ $EUID -ne 0 ]]; then
		fatal "This script must be run as root (use sudo). Or use --standalone mode."
	fi
}

detect_init_system() {
	if [[ -d /run/systemd/system ]]; then
		echo "systemd"
	elif [[ "$(uname)" == "Darwin" ]]; then
		echo "launchd"
	else
		echo "none"
	fi
}

# --- Banner ---
banner() {
	echo -e "${YELLOW}"
	cat <<'EOF'
  ████████╗███████╗███╗   ███╗██████╗ ███████╗███████╗████████╗
  ╚══██╔══╝██╔════╝████╗ ████║██╔══██╗██╔════╝██╔════╝╚══██╔══╝
     ██║   █████╗  ██╔████╔██║██████╔╝█████╗  ███████╗   ██║
     ██║   ██╔══╝  ██║╚██╔╝██║██╔═══╝ ██╔══╝  ╚════██║   ██║
     ██║   ███████╗██║ ╚═╝ ██║██║     ███████╗███████║   ██║
     ╚═╝   ╚══════╝╚═╝     ╚═╝╚═╝     ╚══════╝╚══════╝   ╚═╝
                  ⚡ Install Script
EOF
	echo -e "${NC}"
}

# --- Check Prerequisites ---
check_prerequisites() {
	info "Checking prerequisites..."

	local missing=()

	if ! command_exists go; then
		missing+=("go (1.24.x)")
	fi

	if ! command_exists node; then
		missing+=("node (24.x)")
	fi

	if ! command_exists npm; then
		missing+=("npm")
	fi

	if ! command_exists gcc; then
		missing+=("gcc (for CGO/SQLite)")
	fi

	if [[ ${#missing[@]} -gt 0 ]]; then
		error "Missing required tools:"
		for tool in "${missing[@]}"; do
			echo "  - $tool"
		done
		echo ""
		echo "Install them and run this script again."
		echo ""
		echo "On Debian/Ubuntu (Bookworm 12+):"
		echo "  sudo apt install -y golang nodejs npm gcc libc6-dev"
		echo "  # Note: golang-go may be too old on older releases."
		echo "  # See https://go.dev/doc/install for the latest Go."
		echo ""
		echo "On RHEL/Fedora:"
		echo "  sudo dnf install -y golang nodejs npm gcc"
		echo ""
		echo "On macOS:"
		echo "  brew install go node gcc"
		exit 1
	fi

	ok "All prerequisites met"
}

# --- Build ---
build_app() {
	info "Building frontend..."
	(cd web && npm ci --silent && npm run build --silent)
	ok "Frontend built"

	info "Building backend..."
	mkdir -p build
	CGO_ENABLED=1 go build -ldflags "-s -w" -o build/tempest ./cmd/tempest
	ok "Backend built ($(du -sh build/tempest | cut -f1))"
}

# --- Install (systemd) ---
install_systemd() {
	info "Installing with systemd..."

	# Create user
	if ! id "$APP_USER" &>/dev/null; then
		useradd --system --no-create-home --shell /usr/sbin/nologin "$APP_USER"
		ok "Created system user: $APP_USER"
	fi

	# Create directories
	mkdir -p "$INSTALL_DIR" "$DATA_DIR" "$LOG_DIR" "$CONFIG_DIR"

	# Copy binary and frontend
	install_files "$INSTALL_DIR"

	# Create config if not exists
	if [[ ! -f "$CONFIG_DIR/config.json" ]]; then
		cat >"$CONFIG_DIR/config.json" <<CONF
{
  "server": {
    "host": "${HOST}",
    "port": ${PORT}
  },
  "database": {
    "path": "${DATA_DIR}/tempest.db"
  },
  "engine": {
    "max_concurrent_sessions": 500,
    "default_announce_port": 6881,
    "enable_randomization": true
  }
}
CONF
		ok "Config created: $CONFIG_DIR/config.json"
	else
		warn "Config already exists, skipping: $CONFIG_DIR/config.json"
	fi

	# Create / refresh environment file (used by the systemd unit)
	cat >"$CONFIG_DIR/tempest.env" <<ENV
# Tempest — Environment configuration
# Generated by install.sh; edit this file to change paths or settings
# without touching the .service file directly.

TEMPEST_INSTALL_DIR=${INSTALL_DIR}
TEMPEST_DATA_DIR=${DATA_DIR}
TEMPEST_LOG_DIR=${LOG_DIR}
TEMPEST_CONFIG_DIR=${CONFIG_DIR}
TEMPEST_CONFIG=${CONFIG_DIR}/config.json
TEMPEST_USER=${APP_USER}
ENV
	ok "Environment file created: $CONFIG_DIR/tempest.env"

	# Set ownership
	chown -R "$APP_USER:$APP_USER" "$INSTALL_DIR" "$DATA_DIR" "$LOG_DIR"
	chown -R root:"$APP_USER" "$CONFIG_DIR"
	chmod 640 "$CONFIG_DIR/config.json" "$CONFIG_DIR/tempest.env"

	# Create systemd unit
	# Note: \$TEMPEST_CONFIG is expanded at runtime by systemd from the EnvironmentFile,
	# not by this script. All other ${VAR} references are expanded by bash during install.
	cat >/etc/systemd/system/${APP_NAME}.service <<UNIT
[Unit]
Description=Tempest ⚡ BitTorrent Announce Testing Dashboard (${APP_NAME})
After=network.target

[Service]
Type=simple
User=${APP_USER}
Group=${APP_USER}
EnvironmentFile=${CONFIG_DIR}/tempest.env
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/tempest --config \$TEMPEST_CONFIG
Restart=on-failure
RestartSec=5
StandardOutput=append:${LOG_DIR}/tempest.log
StandardError=append:${LOG_DIR}/tempest-error.log

# Hardening
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ReadWritePaths=${DATA_DIR} ${LOG_DIR} ${INSTALL_DIR}
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_BIND_SERVICE

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
UNIT

	# Enable and start (restart if already running)
	systemctl daemon-reload
	systemctl enable "${APP_NAME}"
	if systemctl is-active --quiet "${APP_NAME}"; then
		systemctl restart "${APP_NAME}"
	else
		systemctl start "${APP_NAME}"
	fi

	ok "Systemd service installed and started"
	echo ""
	info "Useful commands:"
	echo "  systemctl status ${APP_NAME}     # Check status"
	echo "  systemctl restart ${APP_NAME}    # Restart"
	echo "  systemctl stop ${APP_NAME}       # Stop"
	echo "  journalctl -u ${APP_NAME} -f     # Follow logs"
	echo "  tail -f $LOG_DIR/${APP_NAME}.log # Application logs"
}

# --- Install (launchd / macOS) ---
install_launchd() {
	info "Installing with launchd (macOS)..."

	mkdir -p "$INSTALL_DIR" "$DATA_DIR" "$LOG_DIR" "$CONFIG_DIR"

	install_files "$INSTALL_DIR"

	if [[ ! -f "$CONFIG_DIR/config.json" ]]; then
		cat >"$CONFIG_DIR/config.json" <<CONF
{
  "server": {
    "host": "${HOST}",
    "port": ${PORT}
  },
  "database": {
    "path": "${DATA_DIR}/tempest.db"
  },
  "engine": {
    "max_concurrent_sessions": 500,
    "default_announce_port": 6881,
    "enable_randomization": true
  }
}
CONF
	fi

	local plist="/Library/LaunchDaemons/fr.hbtz.tempest.plist"
	cat >"$plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>fr.hbtz.tempest</string>
    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/tempest</string>
        <string>--config</string>
        <string>${CONFIG_DIR}/config.json</string>
    </array>
    <key>WorkingDirectory</key>
    <string>${INSTALL_DIR}</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>${LOG_DIR}/tempest.log</string>
    <key>StandardErrorPath</key>
    <string>${LOG_DIR}/tempest-error.log</string>
</dict>
</plist>
PLIST

	launchctl unload "$plist" 2>/dev/null || true
	launchctl load "$plist"
	ok "launchd service installed and started"
	echo ""
	info "Useful commands:"
	echo "  sudo launchctl list | grep tempest  # Check status"
	echo "  sudo launchctl stop fr.hbtz.tempest   # Stop"
	echo "  sudo launchctl start fr.hbtz.tempest  # Start"
}

# --- Standalone mode (no root, no service manager) ---
install_standalone() {
	info "Setting up standalone mode..."

	local app_dir="${HOME}/.tempest"
	mkdir -p "$app_dir/data" "$app_dir/logs"

	install_files "$app_dir"

	if [[ ! -f "$app_dir/config.json" ]]; then
		cat >"$app_dir/config.json" <<CONF
{
  "server": {
    "host": "${HOST}",
    "port": ${PORT}
  },
  "database": {
    "path": "${app_dir}/data/tempest.db"
  },
  "engine": {
    "max_concurrent_sessions": 500,
    "default_announce_port": 6881,
    "enable_randomization": true
  }
}
CONF
	fi

	# Create start/stop scripts
	cat >"$app_dir/start.sh" <<'SCRIPT'
#!/usr/bin/env bash
DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$DIR"
nohup ./tempest --config config.json > logs/tempest.log 2>&1 &
echo $! > tempest.pid
echo "Tempest started (PID: $(cat tempest.pid))"
echo "Logs: $DIR/logs/tempest.log"
SCRIPT
	chmod +x "$app_dir/start.sh"

	cat >"$app_dir/stop.sh" <<'SCRIPT'
#!/usr/bin/env bash
DIR="$(cd "$(dirname "$0")" && pwd)"
if [[ -f "$DIR/tempest.pid" ]]; then
    PID=$(cat "$DIR/tempest.pid")
    if kill -0 "$PID" 2>/dev/null; then
        kill "$PID"
        echo "Tempest stopped (PID: $PID)"
    else
        echo "Tempest is not running (stale PID file)"
    fi
    rm -f "$DIR/tempest.pid"
else
    echo "No PID file found. Tempest may not be running."
fi
SCRIPT
	chmod +x "$app_dir/stop.sh"

	cat >"$app_dir/status.sh" <<'SCRIPT'
#!/usr/bin/env bash
DIR="$(cd "$(dirname "$0")" && pwd)"
if [[ -f "$DIR/tempest.pid" ]]; then
    PID=$(cat "$DIR/tempest.pid")
    if kill -0 "$PID" 2>/dev/null; then
        echo "Tempest is running (PID: $PID)"
        echo "URL: http://127.0.0.1:8377"
    else
        echo "Tempest is not running (stale PID file)"
    fi
else
    echo "Tempest is not running"
fi
SCRIPT
	chmod +x "$app_dir/status.sh"

	ok "Standalone installation complete: $app_dir"
	echo ""
	info "Usage:"
	echo "  $app_dir/start.sh     # Start Tempest"
	echo "  $app_dir/stop.sh      # Stop Tempest"
	echo "  $app_dir/status.sh    # Check status"
	echo "  Open http://${HOST}:${PORT} in your browser"
	echo ""
	info "To start now:"
	echo "  $app_dir/start.sh"
}

# --- Uninstall ---
uninstall() {
	check_root
	info "Uninstalling Tempest..."

	local init_system
	init_system=$(detect_init_system)

	if [[ "$init_system" == "systemd" ]]; then
		if systemctl is-active "${APP_NAME}" &>/dev/null; then
			systemctl stop "${APP_NAME}"
		fi
		systemctl disable "${APP_NAME}" 2>/dev/null || true
		rm -f /etc/systemd/system/${APP_NAME}.service
		systemctl daemon-reload
		ok "Systemd service removed"
	elif [[ "$init_system" == "launchd" ]]; then
		launchctl unload /Library/LaunchDaemons/fr.hbtz.tempest.plist 2>/dev/null || true
		rm -f /Library/LaunchDaemons/fr.hbtz.tempest.plist
		ok "launchd service removed"
	fi

	if id "$APP_USER" &>/dev/null; then
		userdel "$APP_USER" 2>/dev/null || true
		ok "User $APP_USER removed"
	fi

	echo ""
	warn "The following directories were NOT removed (may contain data):"
	echo "  $INSTALL_DIR"
	echo "  $DATA_DIR"
	echo "  $LOG_DIR"
	echo "  $CONFIG_DIR"
	echo ""
	echo "Remove them manually if no longer needed:"
	echo "  sudo rm -rf $INSTALL_DIR $DATA_DIR $LOG_DIR $CONFIG_DIR"
}

# --- Main ---
main() {
	banner

	case "${1:-}" in
	--uninstall | -u)
		uninstall
		exit 0
		;;
	--standalone | -s)
		check_prerequisites
		build_app
		install_standalone
		exit 0
		;;
	--help | -h)
		echo "Usage: $0 [OPTIONS]"
		echo ""
		echo "Options:"
		echo "  (none)          Interactive install as system service (requires root)"
		echo "  --standalone    Install without service manager (no root needed)"
		echo "  --uninstall     Remove Tempest service and user"
		echo "  --help          Show this help"
		echo ""
		echo "The script prompts for instance name, port, and bind address."
		echo "You can skip the prompts by setting environment variables:"
		echo ""
		echo "  TEMPEST_APP_NAME    Instance name (default: name of this directory)"
		echo "  TEMPEST_PORT        Port to listen on (default: 8377)"
		echo "  TEMPEST_HOST        Host to bind to (default: 127.0.0.1)"
		echo "  TEMPEST_INSTALL_DIR Installation directory (default: /opt/APP_NAME)"
		echo "  TEMPEST_DATA_DIR    Data directory (default: /var/lib/APP_NAME)"
		echo "  TEMPEST_LOG_DIR     Log directory (default: /var/log/APP_NAME)"
		echo "  TEMPEST_CONFIG_DIR  Config directory (default: /etc/APP_NAME)"
		echo "  TEMPEST_USER        System user (default: APP_NAME)"
		echo ""
		echo "Non-interactive example (CI / scripts):"
		echo "  sudo TEMPEST_PORT=8378 TEMPEST_APP_NAME=tempest-prod ./install.sh"
		exit 0
		;;
	esac

	prompt_interactive_config
	check_root
	check_prerequisites
	build_app

	local init_system
	init_system=$(detect_init_system)

	case "$init_system" in
	systemd)
		install_systemd
		;;
	launchd)
		install_launchd
		;;
	none)
		warn "No supported init system detected."
		warn "Falling back to standalone mode."
		install_standalone
		;;
	esac

	echo ""
	ok "Tempest ⚡ is installed and running!"
	echo -e "  Open ${GREEN}http://${HOST}:${PORT}${NC} in your browser"
}

main "$@"
