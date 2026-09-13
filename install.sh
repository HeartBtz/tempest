#!/usr/bin/env bash
#
# Tempest ⚡ — Install Script
#
# This script builds and installs Tempest as a system service.
# Supports: systemd (Linux) or standalone mode.
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

WAS_ACTIVE=false
WAS_ENABLE_STATE="not-found"
STAGED_INSTALL_DIR=""
BACKUP_INSTALL_DIR=""
TRANSACTION_DIR=""
ENV_FILE=""
UNIT_FILE=""
UNIT_NEW=""
CONFIG_PENDING=false
INSTALL_PHASE=0
INSTALL_TRANSACTION_ACTIVE=false
INSTALL_TRANSACTION_VERIFIED=false
INSTALL_ROLLBACK_RUNNING=false

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
	[[ "$1" =~ ^[a-zA-Z0-9_.:-]+$ ]]
}

validate_name() {
	# Only allow alphanumeric + dash/underscore
	[[ "$1" =~ ^[a-zA-Z0-9_-]+$ ]]
}

validate_path() {
	[[ "$1" =~ ^/[a-zA-Z0-9_./-]+$ ]] &&
		[[ "$1" != "/" && "$1" != */ && "$1" != *"//"* ]] &&
		[[ "/$1/" != *"/../"* && "/$1/" != *"/./"* ]]
}

validate_config() {
	validate_name "$APP_NAME" || fatal "Invalid instance name: use letters, numbers, dashes, or underscores."
	validate_name "$APP_USER" || fatal "Invalid service user: use letters, numbers, dashes, or underscores."
	validate_port "$PORT" || fatal "Invalid HTTP port."
	validate_host "$HOST" || fatal "Invalid bind address."

	local path
	for path in "$INSTALL_DIR" "$DATA_DIR" "$LOG_DIR" "$CONFIG_DIR"; do
		validate_path "$path" || fatal "Install paths must be absolute and contain no whitespace or '..' segments."
	done

	case "${DATA_DIR}/" in
	"${INSTALL_DIR}/"*) fatal "Data directory must not be inside the installation directory." ;;
	esac
	case "${LOG_DIR}/" in
	"${INSTALL_DIR}/"*) fatal "Log directory must not be inside the installation directory." ;;
	esac
	case "${INSTALL_DIR}/" in
	"${DATA_DIR}/"* | "${LOG_DIR}/"*) fatal "Installation directory must not be inside a writable data or log directory." ;;
	esac
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

# Copy build artifacts to a target directory.
install_files() {
	local target="$1"
	install -m 0755 build/tempest "$target/tempest"
	mkdir -p "$target/web"
	rm -rf "$target/web/dist"
	cp -a web/dist "$target/web/"
}

stage_installation() {
	local parent
	parent="$(dirname "$INSTALL_DIR")"
	mkdir -p "$parent"
	STAGED_INSTALL_DIR="$(mktemp -d "${parent}/.${APP_NAME}.stage.XXXXXX")"

	if [[ -d "$INSTALL_DIR" ]]; then
		cp -a "$INSTALL_DIR/." "$STAGED_INSTALL_DIR/"
	fi
	install_files "$STAGED_INSTALL_DIR"
	chown -R root:root "$STAGED_INSTALL_DIR"
	chmod -R go-w "$STAGED_INSTALL_DIR"
	chmod 755 "$STAGED_INSTALL_DIR"
}

activate_staged_installation() {
	BACKUP_INSTALL_DIR=""
	if [[ -e "$INSTALL_DIR" ]]; then
		BACKUP_INSTALL_DIR="$(mktemp -d "${INSTALL_DIR}.backup.XXXXXX")"
		rmdir "$BACKUP_INSTALL_DIR"
		mv -T -- "$INSTALL_DIR" "$BACKUP_INSTALL_DIR"
	fi
	mv -T -- "$STAGED_INSTALL_DIR" "$INSTALL_DIR"
	STAGED_INSTALL_DIR=""
}

rollback_installation() {
	local failed_install_dir=""
	if [[ -e "$INSTALL_DIR" ]]; then
		failed_install_dir="$(mktemp -d "${INSTALL_DIR}.failed.XXXXXX")"
		rmdir "$failed_install_dir"
		mv -T -- "$INSTALL_DIR" "$failed_install_dir" || return 1
	fi
	if [[ -n "$BACKUP_INSTALL_DIR" && -e "$BACKUP_INSTALL_DIR" ]]; then
		if ! mv -T -- "$BACKUP_INSTALL_DIR" "$INSTALL_DIR"; then
			[[ -z "$failed_install_dir" ]] || mv -T -- "$failed_install_dir" "$INSTALL_DIR"
			return 1
		fi
	fi
	[[ -z "$failed_install_dir" ]] || rm -rf -- "$failed_install_dir"
}

discard_install_backup() {
	if [[ -n "$BACKUP_INSTALL_DIR" ]]; then
		rm -rf -- "$BACKUP_INSTALL_DIR"
	fi
}

cleanup_install_transaction() {
	[[ -z "$STAGED_INSTALL_DIR" ]] || rm -rf -- "$STAGED_INSTALL_DIR"
	[[ -z "$UNIT_NEW" ]] || rm -f -- "$UNIT_NEW"
	[[ -z "$TRANSACTION_DIR" ]] || rm -rf -- "$TRANSACTION_DIR"
}

restore_transaction_file() {
	local previous="$1" destination="$2"
	if [[ -f "$previous" ]]; then
		mv -f "$previous" "$destination"
	else
		rm -f -- "$destination"
	fi
}

rollback_systemd_transaction() {
	local rollback_ok=true
	[[ "$INSTALL_ROLLBACK_RUNNING" == false ]] || return 1
	INSTALL_ROLLBACK_RUNNING=true
	set +e

	error "Installation did not complete; restoring the previous systemd installation."
	if ((INSTALL_PHASE >= 50)); then
		systemctl stop "${APP_NAME}" || rollback_ok=false
	fi
	if ((INSTALL_PHASE >= 10)); then
		rollback_installation || rollback_ok=false
	fi
	if ((INSTALL_PHASE >= 20)) && [[ "$CONFIG_PENDING" == true ]]; then
		rm -f -- "$CONFIG_DIR/config.json" || rollback_ok=false
	fi
	if ((INSTALL_PHASE >= 30)); then
		restore_transaction_file "$TRANSACTION_DIR/tempest.env.previous" "$ENV_FILE" || rollback_ok=false
	fi
	if ((INSTALL_PHASE >= 40)); then
		restore_transaction_file "$TRANSACTION_DIR/service.previous" "$UNIT_FILE" || rollback_ok=false
	fi
	if ((INSTALL_PHASE >= 10)); then
		systemctl daemon-reload || rollback_ok=false
		restore_systemd_state || rollback_ok=false
	fi
	cleanup_install_transaction
	INSTALL_TRANSACTION_ACTIVE=false

	if [[ "$rollback_ok" == true ]]; then
		error "The previous deployment and service state were restored."
		return 0
	fi
	error "Rollback could not fully restore or verify the previous service state."
	return 1
}

handle_install_error() {
	local status="$1"
	error "Installer command failed with status ${status}."
	exit "$status"
}

handle_install_signal() {
	local signal="$1" status="$2"
	error "Installer interrupted by ${signal}."
	exit "$status"
}

handle_install_exit() {
	local status="$1"
	trap - ERR INT TERM EXIT
	if [[ "$INSTALL_TRANSACTION_ACTIVE" == true ]]; then
		if [[ "$INSTALL_TRANSACTION_VERIFIED" == true ]]; then
			discard_install_backup
			cleanup_install_transaction
		else
			[[ "$status" -ne 0 ]] || status=1
			rollback_systemd_transaction || status=1
		fi
	fi
	exit "$status"
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

	if ! command_exists curl; then
		missing+=("curl (for service health checks)")
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
		echo "  sudo apt install -y golang nodejs npm gcc libc6-dev curl"
		echo "  # Note: golang-go may be too old on older releases."
		echo "  # See https://go.dev/doc/install for the latest Go."
		echo ""
		echo "On RHEL/Fedora:"
		echo "  sudo dnf install -y golang nodejs npm gcc curl"
		echo ""
		echo "On macOS:"
		echo "  brew install go node gcc curl"
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

# Validate existing deployment metadata without changing it. Existing writable
# trees are never recursively chowned or chmodded by this installer.
validate_existing_path() {
	local path="$1" type="$2" owner="$3" required_mode="$4" forbidden_mode="$5"
	local actual_owner mode permissions
	[[ ! -L "$path" ]] || fatal "Refusing symbolic link at managed path: $path"
	if [[ "$type" == directory ]]; then
		[[ -d "$path" ]] || fatal "Managed path is not a directory: $path"
	else
		[[ -f "$path" ]] || fatal "Managed path is not a regular file: $path"
	fi
	actual_owner="$(stat -c '%U:%G' "$path")"
	[[ "$actual_owner" == "$owner" ]] || fatal "$path is owned by $actual_owner; expected $owner. Fix ownership before reinstalling."
	mode="$(stat -c '%a' "$path")"
	permissions=$((8#$mode))
	(((permissions & required_mode) == required_mode && (permissions & forbidden_mode) == 0)) ||
		fatal "$path mode $mode does not provide the required access without unsafe write permissions."
}

validate_service_access() {
	local path="$1" access
	shift
	for access in "$@"; do
		runuser -u "$APP_USER" -- test "-$access" "$path" ||
			fatal "Service user $APP_USER lacks required $access access to $path. No files were changed."
	done
}

preflight_systemd_install() {
	local unit_name="${APP_NAME}.service"
	local unit_file="/etc/systemd/system/${unit_name}"
	local existing_user existing_group load_state

	if id "$APP_USER" &>/dev/null && [[ "$(id -u "$APP_USER")" -eq 0 ]]; then
		fatal "The Tempest service user must be unprivileged."
	fi

	if ! load_state="$(systemctl show "$unit_name" --property=LoadState --value 2>/dev/null)"; then
		fatal "Unable to inspect existing systemd unit state. No files were changed."
	fi
	if [[ "$load_state" != "not-found" && -n "$load_state" ]]; then
		existing_user="$(systemctl show "$unit_name" --property=User --value)"
		existing_group="$(systemctl show "$unit_name" --property=Group --value)"
		[[ "$existing_user" == "$APP_USER" && "$existing_group" == "$APP_USER" ]] ||
			fatal "Existing ${unit_name} runs as ${existing_user:-root}:${existing_group:-default}; requested identity is ${APP_USER}:${APP_USER}. No files were changed."
		systemctl is-active --quiet "$unit_name" && WAS_ACTIVE=true
		WAS_ENABLE_STATE="$(systemctl is-enabled "$unit_name" 2>/dev/null || true)"
		case "$WAS_ENABLE_STATE" in
		enabled | enabled-runtime | disabled) ;;
		*) fatal "Existing ${unit_name} has unsupported enablement state '${WAS_ENABLE_STATE:-unknown}'. No files were changed." ;;
		esac
	fi

	[[ ! -e "$DATA_DIR" ]] || validate_existing_path "$DATA_DIR" directory "$APP_USER:$APP_USER" 0700 0022
	[[ ! -e "$LOG_DIR" ]] || validate_existing_path "$LOG_DIR" directory "$APP_USER:$APP_USER" 0700 0022
	[[ ! -e "$CONFIG_DIR" ]] || validate_existing_path "$CONFIG_DIR" directory "root:$APP_USER" 0750 0022
	[[ ! -e "$CONFIG_DIR/config.json" ]] || validate_existing_path "$CONFIG_DIR/config.json" file "root:$APP_USER" 0440 0133
	[[ ! -e "$CONFIG_DIR/tempest.env" ]] || validate_existing_path "$CONFIG_DIR/tempest.env" file "root:$APP_USER" 0440 0133
	[[ ! -e "$unit_file" ]] || validate_existing_path "$unit_file" file "root:root" 0444 0133

	if id "$APP_USER" &>/dev/null; then
		command_exists runuser || fatal "runuser is required to validate existing service-path access."
		[[ ! -e "$DATA_DIR" ]] || validate_service_access "$DATA_DIR" r w x
		[[ ! -e "$LOG_DIR" ]] || validate_service_access "$LOG_DIR" r w x
		[[ ! -e "$CONFIG_DIR" ]] || validate_service_access "$CONFIG_DIR" x
		[[ ! -e "$CONFIG_DIR/config.json" ]] || validate_service_access "$CONFIG_DIR/config.json" r
		[[ ! -e "$CONFIG_DIR/tempest.env" ]] || validate_service_access "$CONFIG_DIR/tempest.env" r
	fi
}

resolve_health_url() {
	local helper="$1" working_dir="$2" config_file="$3" env_file="$4"
	local args=(--config "$config_file" --health-url)
	[[ ! -f "$env_file" ]] || args+=(--env-file "$env_file")
	(cd "$working_dir" && env -i "$helper" "${args[@]}")
}

# --- Install (systemd) ---
install_systemd() {
	info "Installing with systemd..."

	# Create user
	if ! id "$APP_USER" &>/dev/null; then
		useradd --system --no-create-home --shell /usr/sbin/nologin "$APP_USER"
		ok "Created system user: $APP_USER"
	fi

	# New writable directories use fixed identities and modes. Existing paths
	# were validated during preflight and are deliberately left untouched.
	[[ -d "$DATA_DIR" ]] || install -d -o "$APP_USER" -g "$APP_USER" -m 0750 "$DATA_DIR"
	[[ -d "$LOG_DIR" ]] || install -d -o "$APP_USER" -g "$APP_USER" -m 0750 "$LOG_DIR"
	[[ -d "$CONFIG_DIR" ]] || install -d -o root -g "$APP_USER" -m 0750 "$CONFIG_DIR"

	local env_new config_new
	TRANSACTION_DIR="$(mktemp -d "${CONFIG_DIR}/.install-transaction.XXXXXX")"
	chmod 700 "$TRANSACTION_DIR"
	ENV_FILE="${CONFIG_DIR}/tempest.env"
	env_new="${TRANSACTION_DIR}/tempest.env.new"
	config_new="${TRANSACTION_DIR}/config.json.new"
	UNIT_FILE="/etc/systemd/system/${APP_NAME}.service"
	INSTALL_TRANSACTION_ACTIVE=true
	trap 'handle_install_error $?' ERR
	trap 'handle_install_signal INT 130' INT
	trap 'handle_install_signal TERM 143' TERM
	trap 'handle_install_exit $?' EXIT
	UNIT_NEW="$(mktemp "/etc/systemd/system/.${APP_NAME}.service.new.XXXXXX")"
	[[ -f "$ENV_FILE" ]] && cp -a "$ENV_FILE" "$TRANSACTION_DIR/tempest.env.previous"
	[[ -f "$UNIT_FILE" ]] && cp -a "$UNIT_FILE" "$TRANSACTION_DIR/service.previous"

	# Stage a default config only for a new instance.
	if [[ ! -f "$CONFIG_DIR/config.json" ]]; then
		CONFIG_PENDING=true
		cat >"$config_new" <<CONF
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
		chown root:"$APP_USER" "$config_new"
		chmod 640 "$config_new"
	else
		warn "Config already exists, skipping: $CONFIG_DIR/config.json"
	fi

	# Preserve operator-managed runtime overrides on upgrades.
	if [[ -f "$ENV_FILE" ]]; then
		cp -a "$ENV_FILE" "$env_new"
	else
		cat >"$env_new" <<ENV
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
	fi
	chown root:"$APP_USER" "$env_new"
	chmod 640 "$env_new"

	# Stage the systemd unit.
	# Note: \$TEMPEST_CONFIG is expanded at runtime by systemd from the EnvironmentFile,
	# not by this script. All other ${VAR} references are expanded by bash during install.
	cat >"$UNIT_NEW" <<UNIT
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
TimeoutStopSec=10
StandardOutput=append:${LOG_DIR}/tempest.log
StandardError=append:${LOG_DIR}/tempest-error.log

# Hardening
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ReadWritePaths=${DATA_DIR} ${LOG_DIR}
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_BIND_SERVICE

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
UNIT
	chmod 644 "$UNIT_NEW"

	stage_installation
	INSTALL_PHASE=10
	activate_staged_installation
	if [[ "$CONFIG_PENDING" == true ]]; then
		INSTALL_PHASE=20
		mv "$config_new" "$CONFIG_DIR/config.json"
	fi
	INSTALL_PHASE=30
	mv "$env_new" "$ENV_FILE"
	chown root:"$APP_USER" "$ENV_FILE"
	INSTALL_PHASE=40
	mv "$UNIT_NEW" "$UNIT_FILE"
	UNIT_NEW=""
	chown root:root "$UNIT_FILE"

	INSTALL_PHASE=50
	systemctl daemon-reload
	systemctl enable "${APP_NAME}"
	systemctl restart "${APP_NAME}"
	wait_for_systemd_health "${SCRIPT_DIR}/build/tempest"

	INSTALL_TRANSACTION_VERIFIED=true
	discard_install_backup
	BACKUP_INSTALL_DIR=""
	cleanup_install_transaction
	INSTALL_TRANSACTION_ACTIVE=false
	trap - ERR INT TERM EXIT

	ok "Systemd service installed and started"
	echo ""
	info "Useful commands:"
	echo "  systemctl status ${APP_NAME}     # Check status"
	echo "  systemctl restart ${APP_NAME}    # Restart"
	echo "  systemctl stop ${APP_NAME}       # Stop"
	echo "  journalctl -u ${APP_NAME} -f     # Follow logs"
	echo "  tail -f $LOG_DIR/${APP_NAME}.log # Application logs"
}

wait_for_systemd_health() {
	local helper="$1" health_url attempt
	health_url="$(resolve_health_url "$helper" "$INSTALL_DIR" "$CONFIG_DIR/config.json" "$CONFIG_DIR/tempest.env")" || return 1
	for ((attempt = 1; attempt <= 30; attempt++)); do
		if systemctl is-active --quiet "${APP_NAME}" && curl --fail --silent --max-time 2 "$health_url" >/dev/null; then
			return 0
		fi
		sleep 1
	done
	return 1
}

restore_systemd_state() {
	case "$WAS_ENABLE_STATE" in
	enabled)
		systemctl enable "${APP_NAME}" || return 1
		;;
	enabled-runtime)
		systemctl disable "${APP_NAME}" >/dev/null 2>&1 || true
		systemctl enable --runtime "${APP_NAME}" || return 1
		;;
	disabled | not-found)
		if systemctl is-enabled --quiet "${APP_NAME}"; then
			systemctl disable "${APP_NAME}" || return 1
		fi
		! systemctl is-enabled --quiet "${APP_NAME}" || return 1
		;;
	*) return 1 ;;
	esac
	if [[ "$WAS_ACTIVE" == true ]]; then
		systemctl restart "${APP_NAME}" && wait_for_systemd_health "${SCRIPT_DIR}/build/tempest"
	else
		if systemctl is-active --quiet "${APP_NAME}"; then
			systemctl stop "${APP_NAME}" || return 1
		fi
		if systemctl is-active --quiet "${APP_NAME}"; then
			return 1
		fi
		return 0
	fi
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
		rm -f "/etc/systemd/system/${APP_NAME}.service"
		systemctl daemon-reload
		ok "Systemd service removed"
	elif [[ "$init_system" == "launchd" ]]; then
		fatal "Managed launchd installations are not supported; no service was removed."
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
		validate_config
		uninstall
		exit 0
		;;
	--standalone | -s)
		validate_config
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
	validate_config
	check_root

	local init_system
	init_system=$(detect_init_system)
	if [[ "$init_system" == "systemd" ]]; then
		preflight_systemd_install
	elif [[ "$init_system" == "launchd" ]]; then
		fatal "Managed macOS installation is not supported. Use --standalone as an unprivileged user."
	fi

	check_prerequisites
	build_app

	case "$init_system" in
	systemd)
		install_systemd
		;;
	launchd)
		fatal "Managed macOS installation is not supported. Use --standalone as an unprivileged user."
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
