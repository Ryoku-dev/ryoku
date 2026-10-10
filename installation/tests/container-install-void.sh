#!/usr/bin/env bash
set -euo pipefail

ROOT=${RYOKU_PATH:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}
REPO=${RYOKU_XBPS_REPO:-$ROOT/void/packages/repo/out/x86_64}
KEYRING=${RYOKU_XBPS_KEYRING_DIR:-$ROOT/void/packages/srcpkgs/ryoku-keyring/files}
TESTUSER=${RYOKU_XBPS_TEST_USER:-ryokutest}

log() { printf '\033[1;35m::\033[0m %s\n' "$*"; }
die() { printf 'container-install-void: error: %s\n' "$*" >&2; exit 1; }

[[ $EUID -eq 0 ]] || die "must run as root in a clean Void container"
[[ -s $REPO/x86_64-repodata ]] || die "repository index not found: $REPO/x86_64-repodata"
shopt -s nullglob
plists=("$KEYRING"/*.plist)
((${#plists[@]})) || die "no build signing key plist found in $KEYRING"

log "Seeding the repository signing key"
install -d -m755 /var/db/xbps/keys /etc/xbps.d
for plist in "${plists[@]}"; do install -m644 "$plist" /var/db/xbps/keys/; done
printf 'repository=%s\n' "$REPO" > /etc/xbps.d/20-ryoku.conf

# A mutable base image may need xbps itself refreshed before it can read the
# current official indexes.
xbps-install -Syu xbps || xbps-install -yu xbps
log "Installing ryoku-desktop from the freshly built repository"
# The base must pull its only compositor variant through the virtual.
xbps-install -Sy -y ryoku-desktop

xbps-query -p pkgver ryoku-desktop-niri >/dev/null \
	|| die "ryoku-desktop-niri is not registered as installed"
[[ -x /usr/bin/ryoku ]] || die "ryoku CLI was not installed"
[[ -x /usr/bin/ryoku-wm-niri ]] || die "niri provider was not installed"
[[ -s /usr/share/ryoku/config/niri/config.kdl ]] || die "niri config source was not installed"
[[ -s /usr/share/xbps.d/20-ryoku.conf ]] || die "ryoku-keyring did not install its repository config"

id "$TESTUSER" >/dev/null 2>&1 || useradd -m "$TESTUSER"
home=/home/$TESTUSER
log "Materializing the packaged config as $TESTUSER"
su -s /bin/sh -c "HOME='$home' USER='$TESTUSER' LOGNAME='$TESTUSER' /usr/bin/ryoku materialize" "$TESTUSER"

cfg=$home/.config
missing=()
for path in \
	quickshell/shell/shell.qml \
	niri/config.kdl \
	niri/settings.kdl \
	niri/rebinds.kdl \
	fish/config.fish \
	kitty/kitty.conf \
	starship.toml; do
	[[ -s $cfg/$path ]] || missing+=("$cfg/$path")
done
[[ -n $(find "$cfg/quickshell/hub" -type f -print -quit 2>/dev/null) ]] \
	|| missing+=("$cfg/quickshell/hub/ (empty or absent)")

# Derive the complete common-surface module set from the checkout so adding an
# import without packaging its module fails this install gate.
mapfile -t modules < <(
	grep -rhoE 'import Ryoku\.[A-Za-z0-9]+' \
		"$ROOT/ryoku/shell" "$ROOT/ryoku/hub" "$ROOT/ryoku/apps" "$ROOT/ryoku/ui" 2>/dev/null \
		| awk '{print $2}' | LC_ALL=C sort -u
)
for module in "${modules[@]}"; do
	qmldir=/usr/lib/qt6/qml/Ryoku/${module#Ryoku.}/qmldir
	[[ -s $qmldir ]] || missing+=("$qmldir (imported as $module)")
done

if ((${#missing[@]})); then
	printf '%s\n' 'container-install-void: packaged install is incomplete:' >&2
	printf '  - %s\n' "${missing[@]}" >&2
	exit 1
fi

log "container-install-void: packaged niri desktop materialized successfully"
