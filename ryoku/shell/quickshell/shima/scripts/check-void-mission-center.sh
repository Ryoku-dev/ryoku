#!/usr/bin/env bash
# Void Mission Center checks for the Mission Center Flatpak provider in a live Void session.
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
expected_branch="${INIR_EXPECTED_BRANCH:-feat/void-pr50-missioncenter}"
expected_commit="${INIR_EXPECTED_COMMIT:-}"
actual_branch="$(git -C "$repo_root" branch --show-current 2>/dev/null || true)"
actual_commit="$(git -C "$repo_root" rev-parse HEAD 2>/dev/null || true)"

check test "$actual_branch" = "$expected_branch"
if [[ -n "$expected_commit" ]]; then
  check test "$actual_commit" = "$expected_commit"
else
  printf 'INFO: commit=%s (not pinned; set INIR_EXPECTED_COMMIT to pin it)\n' "$actual_commit"
fi
if [[ "${INIR_ALLOW_DIRTY:-false}" != true ]]; then
  check test -z "$(git -C "$repo_root" status --porcelain 2>/dev/null)"
fi

deps="$repo_root/sdata/dist-void/install-deps.sh"
for needle in \
  'install_void_missioncenter' \
  'io.missioncenter.MissionCenter' \
  'flatpak remote-add --if-not-exists --user flathub' \
  'exec flatpak run io.missioncenter.MissionCenter "$@"'; do
  if grep -Fq "$needle" "$deps"; then
    printf 'PASS: provider contains: %s\n' "$needle"
  else
    printf 'FAIL: provider missing: %s\n' "$needle" >&2
    failures=$((failures + 1))
  fi
done

check xbps-query -p pkgver flatpak
check command -v flatpak
check flatpak info --user io.missioncenter.MissionCenter

wrapper="${XDG_BIN_HOME:-$HOME/.local/bin}/missioncenter"
check test -x "$wrapper"
check grep -Fq 'exec flatpak run io.missioncenter.MissionCenter "$@"' "$wrapper"

if [[ "${INIR_VERIFY_IDEMPOTENCY:-false}" == true ]]; then
  before="$(mktemp)"
  after="$(mktemp)"
  provider_snapshot() {
    flatpak info --user --show-ref io.missioncenter.MissionCenter
    sha256sum "$wrapper"
  }
  provider_snapshot > "$before"
  check env ONLY_MISSING_DEPS=missioncenter \
    "$repo_root/setup" install -y --skip-sysupdate --skip-setups --skip-files
  provider_snapshot > "$after"
  check cmp -s "$before" "$after"
  rm -f "$before" "$after"
else
  printf 'INFO: set INIR_VERIFY_IDEMPOTENCY=true to run the second-install snapshot check\n'
fi

if ((failures > 0)); then
  printf '%d Void Mission Center check(s) failed\n' "$failures" >&2
  exit 1
fi
printf 'All Void Mission Center checks passed\n'
