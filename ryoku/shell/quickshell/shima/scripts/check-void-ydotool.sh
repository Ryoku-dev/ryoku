#!/usr/bin/env bash
# Void ydotool checks for a live Void session (ydotool provider).
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

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
expected_branch="${INIR_EXPECTED_BRANCH:-feat/void-ydotool-provider}"
expected_commit="${INIR_EXPECTED_COMMIT:-}"
actual_branch="$(git -C "$repo_root" branch --show-current 2>/dev/null || true)"
actual_commit="$(git -C "$repo_root" rev-parse HEAD 2>/dev/null || true)"

check test "$actual_branch" = "$expected_branch"
if [[ -n "$expected_commit" ]]; then
  check test "$actual_commit" = "$expected_commit"
else
  printf 'INFO: commit=%s (not pinned; set INIR_EXPECTED_COMMIT to pin it)\n' "$actual_commit"
fi
check test -z "$(git -C "$repo_root" status --porcelain 2>/dev/null)"

check command -v ydotool
check command -v ydotoold
check test "$(ydotoold --version 2>/dev/null)" = v1.0.4

deps="$repo_root/sdata/dist-void/install-deps.sh"
setups="$repo_root/sdata/subcmd-install/2.setups.sh"
functions="$repo_root/sdata/lib/functions.sh"
for needle in \
  'YDOTOOL_VERSION="1.0.4"' \
  'YDOTOOL_SOURCE_SHA256="ba075a43aa6ead51940e892ecffa4d0b8b40c241e4e2bc4bd9bd26b61fde23bd"' \
  'sha256sum -c -'; do
  if grep -Fq "$needle" "$deps"; then
    printf 'PASS: provider contains: %s\n' "$needle"
  else
    printf 'FAIL: provider missing: %s\n' "$needle" >&2
    failures=$((failures + 1))
  fi
done
check grep -Fq 'configure_void_ydotool_uinput' "$setups"
check grep -Fq 'KERNEL=="uinput", GROUP="input", MODE="0660"' "$functions"
check grep -Fq 'reconcile_ydotool_user_service' "$functions"

if id -nG "$(whoami)" | tr ' ' '\n' | grep -qx input; then
  printf 'PASS: user in input group\n'
else
  printf 'FAIL: user not in input group\n' >&2
  failures=$((failures + 1))
fi
check test -c /dev/uinput
check test -w /dev/uinput

service_dir="${XDG_CONFIG_HOME:-$HOME/.config}/service/ydotool"
socket_path="${YDOTOOL_SOCKET:-${XDG_RUNTIME_DIR:-/tmp}/.ydotool_socket}"
check test -x "$service_dir/run"
check grep -Fq '# Managed by iNiR.' "$service_dir/run"
check sv status "$service_dir"
check test -S "$socket_path"

# A Shift tap is harmless but traverses the real client, socket, daemon, and uinput path.
check ydotool key 42:1 42:0

if [[ "${INIR_VERIFY_IDEMPOTENCY:-false}" == true ]]; then
  before="$(mktemp)"
  after="$(mktemp)"
  provider_snapshot() {
    xbps-query -l | sort
    for path in \
      /usr/local/bin/ydotool \
      /usr/local/bin/ydotoold \
      /etc/modules-load.d/inir-ydotool.conf \
      /etc/udev/rules.d/80-inir-ydotool.rules \
      "$service_dir/run"; do
      [[ -f "$path" ]] && sha256sum "$path"
    done
  }
  provider_snapshot > "$before"
  check "$repo_root/setup" install -y --skip-sysupdate --skip-files
  # --skip-files avoids touching user configuration; reconcile only this provider.
  source "$repo_root/sdata/lib/functions.sh"
  check reconcile_ydotool_user_service "$(inir_supervisor)" true
  provider_snapshot > "$after"
  check cmp -s "$before" "$after"
  rm -f "$before" "$after"
else
  printf 'INFO: set INIR_VERIFY_IDEMPOTENCY=true to run the second-install package snapshot check\n'
fi

if ((failures > 0)); then
  printf '%d Void ydotool check(s) failed\n' "$failures" >&2
  exit 1
fi
printf 'All Void ydotool checks passed\n'
