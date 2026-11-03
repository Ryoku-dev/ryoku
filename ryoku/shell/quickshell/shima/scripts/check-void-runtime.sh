#!/usr/bin/env bash
# Read-only Void runtime checks for a live Void session.
set -u

failures=0
check() {
  if "$@"; then
    printf 'PASS: %s\n' "$*"
  else
    printf 'FAIL: %s\n' "$*" >&2
    failures=$((failures + 1))
  fi
}

runtime_dir="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"
service_root="${XDG_CONFIG_HOME:-$HOME/.config}/service"
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
expected_branch="${INIR_EXPECTED_BRANCH:-feat/void-nonsystemd-runtime}"
expected_commit="${INIR_EXPECTED_COMMIT:-}"

check command -v loginctl
check command -v sv
check command -v dbus-update-activation-environment
check command -v busctl
check test -d "$runtime_dir"
check test -S "$runtime_dir/bus"
check loginctl --help >/dev/null 2>&1
loginctl_help="$(loginctl --help 2>&1)"
for power_verb in suspend hibernate poweroff reboot; do
  if grep -Eq "^[[:space:]]+$power_verb([[:space:]]|$)" <<< "$loginctl_help"; then
    printf 'PASS: loginctl %s available\n' "$power_verb"
  else
    printf 'FAIL: loginctl %s unavailable\n' "$power_verb" >&2
    failures=$((failures + 1))
  fi
done

actual_branch="$(git -C "$repo_root" branch --show-current 2>/dev/null || true)"
actual_commit="$(git -C "$repo_root" rev-parse HEAD 2>/dev/null || true)"
if [[ "$actual_branch" == "$expected_branch" ]]; then
  printf 'PASS: branch=%s\n' "$actual_branch"
else
  printf 'FAIL: branch=%s (expected %s)\n' "$actual_branch" "$expected_branch" >&2
  failures=$((failures + 1))
fi
if [[ -n "$expected_commit" ]]; then
  if [[ "$actual_commit" == "$expected_commit" ]]; then
    printf 'PASS: commit=%s\n' "$actual_commit"
  else
    printf 'FAIL: commit=%s (expected %s)\n' "$actual_commit" "$expected_commit" >&2
    failures=$((failures + 1))
  fi
else
  printf 'INFO: commit=%s (not pinned; set INIR_EXPECTED_COMMIT to pin it)\n' "$actual_commit"
fi
if [[ -n "$(git -C "$repo_root" status --porcelain 2>/dev/null)" ]]; then
  printf 'FAIL: checkout is dirty\n' >&2
  failures=$((failures + 1))
else
  printf 'PASS: checkout is clean\n'
fi

if [[ -S "$runtime_dir/systemd/private" ]] &&
   timeout 3s systemctl --user show-environment >/dev/null 2>&1; then
  printf 'INFO: usable systemd user manager detected; non-systemd branch is not active\n'
else
  printf 'PASS: usable systemd user manager predicate is false\n'
  check test -x "$service_root/inir/run"
  check sv status "$service_root/inir"
  if command -v xembedsniproxy >/dev/null 2>&1; then
    check test -x "$service_root/inir-xembedsniproxy/run"
    check sv status "$service_root/inir-xembedsniproxy"
  else
    printf 'INFO: xembedsniproxy is not installed; service remains dormant\n'
  fi
  check busctl --user list >/dev/null
fi

printf 'INFO: session=%s bus=%s runtime=%s\n' \
  "${XDG_SESSION_ID:-unknown}" \
  "${DBUS_SESSION_BUS_ADDRESS:-missing}" \
  "$runtime_dir"

if ((failures > 0)); then
  printf '%d Void runtime check(s) failed\n' "$failures" >&2
  exit 1
fi
printf 'All Void runtime checks passed\n'
