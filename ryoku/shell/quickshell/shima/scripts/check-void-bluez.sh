#!/usr/bin/env bash
# Read-only Void BlueZ checks for a live Void session (BlueZ provider).
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
expected_branch="${INIR_EXPECTED_BRANCH:-feat/void-bluez-provider}"
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

for package in bluez blueman; do
  check xbps-query -p pkgver "$package"
done
check command -v bluetoothctl
check command -v blueman-manager

if xbps-query -p pkgver pipewire >/dev/null 2>&1; then
  check xbps-query -p pkgver libspa-bluetooth
else
  printf 'INFO: audio profile is not installed; skipping libspa-bluetooth check\n'
fi

setups="$repo_root/sdata/subcmd-install/2.setups.sh"
deps="$repo_root/sdata/dist-void/install-deps.sh"
for needle in \
  'ln -sfn /etc/sv/bluetoothd /var/service/bluetoothd' \
  'required_groups+=",bluetooth"'; do
  if grep -Fq "$needle" "$setups"; then
    printf 'PASS: setup contains: %s\n' "$needle"
  else
    printf 'FAIL: setup missing: %s\n' "$needle" >&2
    failures=$((failures + 1))
  fi
done
for package in bluez blueman libspa-bluetooth; do
  if grep -Eq "^[[:space:]]+${package}$" "$deps"; then
    printf 'PASS: Void profile contains: %s\n' "$package"
  else
    printf 'FAIL: Void profile missing: %s\n' "$package" >&2
    failures=$((failures + 1))
  fi
done

check test -L /var/service/bluetoothd
if sudo -n true >/dev/null 2>&1; then
  check sudo -n sv status /var/service/bluetoothd
  check sudo -n sv status /var/service/dbus
else
  printf 'INFO: privileged sv status skipped (sudo -n unavailable); D-Bus ownership remains authoritative\n'
fi

if busctl --system --no-pager --no-legend --acquired list 2>/dev/null \
    | grep -q '^org\.bluez[[:space:]]'; then
  printf 'PASS: org.bluez owns the system D-Bus name\n'
else
  printf 'FAIL: org.bluez is absent from the system D-Bus\n' >&2
  failures=$((failures + 1))
fi

if id -nG "$(whoami)" | tr ' ' '\n' | grep -qx bluetooth; then
  printf 'PASS: user in bluetooth group\n'
else
  printf 'FAIL: user not in bluetooth group\n' >&2
  failures=$((failures + 1))
fi

adapters="$(bluetoothctl list 2>/dev/null || true)"
if [[ -n "$adapters" ]]; then
  printf 'PASS: Bluetooth adapter available\n'
  check bluetoothctl show
  if command -v rfkill >/dev/null 2>&1; then
    check rfkill list bluetooth
  fi
else
  printf 'INFO: no Bluetooth adapter exposed; hardware operation checks skipped\n'
fi

if ((failures > 0)); then
  printf '%d Void BlueZ check(s) failed\n' "$failures" >&2
  exit 1
fi
printf 'All Void BlueZ checks passed\n'
