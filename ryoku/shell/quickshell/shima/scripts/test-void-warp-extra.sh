#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

metadata="$tmp/Packages"
cat > "$metadata" <<'EOF'
Package: unrelated
Version: 1.0
Filename: pool/unrelated.deb
SHA256: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa

Package: cloudflare-warp
Version: 2099.4.3.2
Architecture: amd64
Filename: pool/bookworm/main/c/cloudflare-warp/cloudflare-warp_2099.4.3.2_amd64.deb
SHA256: bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
EOF

export INIR_WARP_PACKAGES_FILE="$metadata"
export XDG_STATE_HOME="$tmp/state"
export HOME="$tmp/home"
mkdir -p "$HOME"

# shellcheck source=/dev/null
source "$repo_root/sdata/lib/extras.sh"

# warp-svc cannot establish a tunnel without nftables.
prereq_log="$tmp/warp-prereqs"
xbps-query() { return 1; }
pkg_sudo() { printf '%s\n' "$*" > "$prereq_log"; }
tui_info() { :; }
ask=false
extras_void_warp_install_prereqs
grep -Eq '(^|[[:space:]])nftables($|[[:space:]])' "$prereq_log" || {
  printf 'FAIL: WARP provider does not install nftables required by warp-svc firewall setup\n' >&2
  exit 1
}
unset -f xbps-query pkg_sudo tui_info
unset ask

# Same-version refresh must still repair missing runtime prerequisites.
same_version_prereq_log="$tmp/warp-same-version-prereqs"
(
  OS_GROUP_ID=void
  XDG_STATE_HOME="$tmp/same-version-state"
  extras_void_warp_release_info() {
    printf '2026.7.1377.0\tfixture.deb\t%s\n' \
      '95d33c2b4fc42f21c204981c51470a6a679d618fb0b78ee64bdd0db142230c55'
  }
  extras_void_warp_installed_version() { printf '2026.7.1377.0\n'; }
  xbps-query() { return 1; }
  pkg_sudo() { printf '%s\n' "$*" > "$same_version_prereq_log"; }
  tui_info() { :; }
  log_success() { :; }
  configure_void_warp_service() { :; }
  ask=false
  extras_install_void_warp
)
grep -Eq '(^|[[:space:]])nftables($|[[:space:]])' "$same_version_prereq_log" || {
  printf 'FAIL: managed same-version WARP refresh skipped nftables prerequisite repair\n' >&2
  exit 1
}

# tar does not auto-detect compression for a payload streamed from `ar p`.
mkdir -p "$tmp/deb-src/usr/bin" "$tmp/deb-out" "$tmp/deb-build"
printf '#!/bin/sh\nprintf "fixture\\n"\n' > "$tmp/deb-srcwarp-cli"
printf '2.0\n' > "$tmp/deb-build/debian-binary"
tar -czf "$tmp/deb-build/control.tar.gz" --files-from /dev/null
tar -cJf "$tmp/deb-build/data.tar.xz" -C "$tmp/deb-src" .
( cd "$tmp/deb-build" && ar r "$tmp/fixture.deb" debian-binary control.tar.gz data.tar.xz >/dev/null )
extras_void_warp_extract_payload "$tmp/fixture.deb" "$tmp/deb-out"
[[ -f "$tmp/deb-outwarp-cli" ]] || {
  printf 'FAIL: WARP provider could not extract an xz-compressed Debian payload\n' >&2
  exit 1
}

expected=$'2099.4.3.2\tpool/bookworm/main/c/cloudflare-warp/cloudflare-warp_2099.4.3.2_amd64.deb\tbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'
actual="$(extras_void_warp_release_info)"
[[ "$actual" == "$expected" ]] || {
  printf 'FAIL: WARP metadata parser mismatch\nexpected: %s\nactual:   %s\n' "$expected" "$actual" >&2
  exit 1
}

# Metadata outages use the last verified in-tree artifact.
unset INIR_WARP_PACKAGES_FILE
curl() { return 1; }
fallback="$(extras_void_warp_release_info)"
[[ "$fallback" == "$INIR_WARP_FALLBACK_VERSION"$'\t'"$INIR_WARP_FALLBACK_FILENAME"$'\t'"$INIR_WARP_FALLBACK_SHA256" ]] || {
  printf 'FAIL: WARP offline fallback is not deterministic\n' >&2
  exit 1
}
unset -f curl

# A newer local install must never be replaced by older provider metadata.
export INIR_WARP_PACKAGES_FILE="$metadata"
mkdir -p "$tmp/bin"
cat > "$tmp/bin/warp-cli" <<'EOF'
#!/bin/sh
printf '%s\n' 'warp-cli 9999.1.0.0'
EOF
chmod +x "$tmp/bin/warp-cli"
old_path="$PATH"
export PATH="$tmp/bin:$PATH"
OS_GROUP_ID=void
log_warning() { :; }
log_success() { :; }
log_info() { :; }
tui_info() { :; }
xbps-query() { return 0; }
if ! extras_install_void_warp; then
  printf 'FAIL: WARP provider rejected a newer installed version\n' >&2
  exit 1
fi
state_file="$(extras_void_warp_state_file)"
[[ ! -e "$state_file" ]] || {
  printf 'FAIL: WARP provider claimed ownership metadata for an unverified newer local version\n' >&2
  exit 1
}

# musl must fail before provisioning.
cat > "$tmp/bin/ldd" <<'EOF'
#!/bin/sh
printf '%s\n' 'musl libc (x86_64)'
EOF
chmod +x "$tmp/bin/ldd"
rm -f "$state_file"
if extras_install_void_warp; then
  printf 'FAIL: WARP provider accepted a simulated Void musl host\n' >&2
  exit 1
fi
[[ ! -e "$state_file" ]] || { printf 'FAIL: musl rejection wrote provider state\n' >&2; exit 1; }

unset -f xbps-query
export PATH="$old_path"
printf 'Void WARP optional-extra checks passed\n'
