#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
installer="${repo_root}/install.sh"
# These patterns intentionally match unexpanded shell variables in install.sh.
# shellcheck disable=SC2016
readonly expected_read_write_paths='ReadWritePaths=${DATA_DIR} ${LOG_DIR}'
# shellcheck disable=SC2016
readonly expected_root_owner='chown -R root:root "$STAGED_INSTALL_DIR"'
# shellcheck disable=SC2016
readonly expected_read_only='chmod -R go-w "$STAGED_INSTALL_DIR"'
# shellcheck disable=SC2016
readonly expected_traverse='chmod 755 "$STAGED_INSTALL_DIR"'
# shellcheck disable=SC2016
readonly expected_backup='mv -T -- "$INSTALL_DIR" "$BACKUP_INSTALL_DIR"'
# shellcheck disable=SC2016
readonly expected_effective_probe='env -i "$helper" "${args[@]}"'
# shellcheck disable=SC2016
readonly expected_enabled_state='WAS_ENABLE_STATE="$(systemctl is-enabled "$unit_name" 2>/dev/null || true)"'
# shellcheck disable=SC2016
readonly expected_failed_deployment='mv -T -- "$INSTALL_DIR" "$failed_install_dir"'
# shellcheck disable=SC2016
readonly forbidden_install_delete='rm -rf "$INSTALL_DIR"'

fail() {
	printf 'installer static check failed: %s\n' "$1" >&2
	exit 1
}

bash -n "$installer"
if command -v shellcheck >/dev/null 2>&1; then
	shellcheck "$installer" "$0"
fi

grep -Fq "$expected_read_write_paths" "$installer" || fail "writable paths are not restricted to data and logs"
if grep -Eq '^ReadWritePaths=.*INSTALL_DIR' "$installer"; then
	fail "installation directory is writable by the service"
fi
grep -Fq "$expected_root_owner" "$installer" || fail "staged deployment is not root-owned"
grep -Fq "$expected_read_only" "$installer" || fail "service write bits are not removed from the deployment"
grep -Fq "$expected_traverse" "$installer" || fail "service cannot traverse the staged deployment"
grep -Fq "$expected_backup" "$installer" || fail "deployment backup is missing"
grep -Fq 'rollback_installation' "$installer" || fail "automatic rollback is missing"
grep -Fq 'wait_for_systemd_health' "$installer" || fail "post-restart health probe is missing"
grep -Fq "$expected_effective_probe" "$installer" || fail "health URL is not resolved from a clean effective service environment"
grep -Fq "$expected_enabled_state" "$installer" || fail "previous systemd enablement is not captured"
grep -Fq 'restore_systemd_state' "$installer" || fail "previous active/enabled state is not restored"
grep -Fq "wait_for_systemd_health \"\${SCRIPT_DIR}/build/tempest\"" "$installer" || fail "restored active service health is not verified"
grep -Fq "Existing \${unit_name} runs as" "$installer" || fail "existing service identity mismatch is not rejected"
grep -Fq "validate_existing_path \"\$DATA_DIR\"" "$installer" || fail "existing writable-directory ownership is not preflighted"
grep -Fq "runuser -u \"\$APP_USER\" -- test" "$installer" || fail "existing paths are not access-tested as the service user"
[[ "$(grep -Fc "wait_for_systemd_health \"\${SCRIPT_DIR}/build/tempest\"" "$installer")" -ge 2 ]] || fail "both updated and restored active services must be health-checked"
grep -Fq "trap 'handle_install_error \$?' ERR" "$installer" || fail "ERR trap is missing"
grep -Fq "trap 'handle_install_signal INT 130' INT" "$installer" || fail "INT trap is missing"
grep -Fq "trap 'handle_install_signal TERM 143' TERM" "$installer" || fail "TERM trap is missing"
grep -Fq "trap 'handle_install_exit \$?' EXIT" "$installer" || fail "EXIT trap is missing"
grep -Fq 'INSTALL_PHASE=10' "$installer" || fail "deployment activation phase is not recorded"
grep -Fq 'INSTALL_PHASE=30' "$installer" || fail "environment activation phase is not recorded"
grep -Fq 'INSTALL_PHASE=40' "$installer" || fail "unit activation phase is not recorded"
grep -Fq 'INSTALL_PHASE=50' "$installer" || fail "service-state activation phase is not recorded"
grep -Fq 'INSTALL_TRANSACTION_VERIFIED=true' "$installer" || fail "verified transaction state is not recorded"
grep -Fq "$expected_failed_deployment" "$installer" || fail "failed deployment is not moved to installer-owned cleanup space"
if grep -Fq "$forbidden_install_delete" "$installer"; then
	fail "rollback recursively deletes the active installation path"
fi

trap_line="$(grep -nF "trap 'handle_install_exit \$?' EXIT" "$installer" | cut -d: -f1)"
activation_line="$(grep -nF 'INSTALL_PHASE=10' "$installer" | cut -d: -f1)"
verified_line="$(grep -nF 'INSTALL_TRANSACTION_VERIFIED=true' "$installer" | cut -d: -f1)"
disarm_line="$(grep -nF 'trap - ERR INT TERM EXIT' "$installer" | tail -n 1 | cut -d: -f1)"
[[ "$trap_line" -lt "$activation_line" ]] || fail "transaction traps are armed after deployment activation begins"
[[ "$activation_line" -lt "$verified_line" ]] || fail "transaction is marked verified before deployment activation"
[[ "$verified_line" -lt "$disarm_line" ]] || fail "transaction traps are disarmed before verified success"
for writable_path in "\$DATA_DIR" "\$LOG_DIR" "\$CONFIG_DIR"; do
	if grep -F 'chown -R' "$installer" | grep -Fq "$writable_path"; then
		fail "installer recursively changes existing writable-directory ownership"
	fi
done
if grep -Eq 'install_launchd|LaunchDaemons|launchctl (load|bootstrap)' "$installer"; then
	fail "unsupported launchd installation code is present"
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT
go build -o "$tmp_dir/tempest" ./cmd/tempest
cat >"$tmp_dir/config.json" <<'JSON'
{"server":{"host":"0.0.0.0","port":9101}}
JSON
cat >"$tmp_dir/tempest.env" <<'ENV'
TEMPEST_HOST=::
TEMPEST_PORT=9102
ENV

actual="$(cd "$tmp_dir" && env -i ./tempest --config config.json --health-url)"
[[ "$actual" == 'http://127.0.0.1:9101/health' ]] || fail "IPv4 wildcard health URL was not normalized: $actual"
actual="$(cd "$tmp_dir" && env -i ./tempest --config config.json --env-file tempest.env --health-url)"
[[ "$actual" == 'http://[::1]:9102/health' ]] || fail "environment override or IPv6 wildcard health URL failed: $actual"
TEMPEST_HOST='[::]' TEMPEST_PORT=9102 "$tmp_dir/tempest" --config "$tmp_dir/config.json" --health-url >"$tmp_dir/health-url"
actual="$(<"$tmp_dir/health-url")"
[[ "$actual" == 'http://[::1]:9102/health' ]] || fail "bracketed IPv6 wildcard health URL failed: $actual"
TEMPEST_HOST='2001:db8::1' TEMPEST_PORT=9103 "$tmp_dir/tempest" --config "$tmp_dir/config.json" --health-url >"$tmp_dir/health-url"
actual="$(<"$tmp_dir/health-url")"
[[ "$actual" == 'http://[2001:db8::1]:9103/health' ]] || fail "IPv6 health URL was not bracketed: $actual"

printf 'installer static checks: ok\n'
