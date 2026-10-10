#!/usr/bin/env bash
set -euo pipefail

usage() {
	printf 'usage: %s REPO_ROOT DESTINATION\n' "${0##*/}" >&2
	exit 2
}

[[ $# -eq 2 ]] || usage
REPO_ROOT=$(cd "$1" && pwd -P)
DEST=$2
ARCH=${RYOKU_VOID_ISO_ARCH:-x86_64}
WORK=${RYOKU_VOID_ISO_OFFLINE_WORK:-${TMPDIR:-/tmp}/ryoku-void-offline-$UID}
VERIFY_ROOT=${RYOKU_VOID_ISO_VERIFY_ROOT:-$WORK}
KEYS_OUT=${RYOKU_VOID_ISO_KEYS_OUT:-$DEST/.keys}
RYOKU_REPO=${RYOKU_VOID_ISO_RYOKU_REPO:-}
VOID_MIRROR=${RYOKU_VOID_ISO_MIRROR:-https://repo-default.voidlinux.org/current}
RESOLVE=$REPO_ROOT/void/packages/resolve

log() { printf '\033[1;35m::\033[0m %s\n' "$*"; }
die() { printf 'offline-repo.sh: error: %s\n' "$*" >&2; exit 1; }

for command in xbps-install xbps-rindex awk du; do
	command -v "$command" >/dev/null 2>&1 || die "required command not found: $command"
done
[[ -x $RESOLVE ]] || die "package resolver not found: $RESOLVE"
[[ -n $RYOKU_REPO ]] \
	|| die "RYOKU_VOID_ISO_RYOKU_REPO must name the built Ryoku XBPS repository"

normalize_repo() {
	local source=$1
	if [[ -d $source ]]; then
		source=$(cd "$source" && pwd -P)
		if [[ -s $source/$ARCH-repodata ]]; then
			printf '%s\n' "$source"
		elif [[ -s $source/$ARCH/$ARCH-repodata ]]; then
			printf '%s\n' "$source/$ARCH"
		else
			die "local Ryoku repository has no $ARCH-repodata: $source"
		fi
	else
		printf '%s\n' "${source%/}"
	fi
}
RYOKU_REPO=$(normalize_repo "$RYOKU_REPO")

rm -rf "$WORK" "$DEST" "$KEYS_OUT"
mkdir -p "$WORK/cache" "$DEST" "$KEYS_OUT"

seed_keys() {
	local root=$1
	mkdir -p "$root/var/db/xbps/keys"
	if compgen -G '/var/db/xbps/keys/*.plist' >/dev/null; then
		cp -a /var/db/xbps/keys/*.plist "$root/var/db/xbps/keys/"
	fi
	if [[ -n ${RYOKU_VOID_ISO_KEYRING_DIR:-} ]]; then
		compgen -G "$RYOKU_VOID_ISO_KEYRING_DIR/*.plist" >/dev/null \
			|| die "no key plist found in RYOKU_VOID_ISO_KEYRING_DIR=$RYOKU_VOID_ISO_KEYRING_DIR"
		cp -a "$RYOKU_VOID_ISO_KEYRING_DIR"/*.plist "$root/var/db/xbps/keys/"
	fi
	if compgen -G "$KEYS_OUT/*.plist" >/dev/null; then
		cp -a "$KEYS_OUT"/*.plist "$root/var/db/xbps/keys/"
	fi
}

# Remote repositories need their trust key before the first index sync.
if [[ -d $RYOKU_REPO && -z ${RYOKU_VOID_ISO_KEYRING_DIR:-} ]]; then
	key_root=$WORK/key-root
	mkdir -p "$key_root/var/db/xbps/keys"
	if ! XBPS_ARCH=$ARCH xbps-install -r "$key_root" -i -R "$RYOKU_REPO" -Sy ryoku-keyring; then
		die "could not extract ryoku-keyring from $RYOKU_REPO; set RYOKU_VOID_ISO_KEYRING_DIR"
	fi
	compgen -G "$key_root/var/db/xbps/keys/*.plist" >/dev/null \
		|| die "ryoku-keyring installed no XBPS key plist"
	cp -a "$key_root"/var/db/xbps/keys/*.plist "$KEYS_OUT/"
elif [[ -n ${RYOKU_VOID_ISO_KEYRING_DIR:-} ]]; then
	cp -a "$RYOKU_VOID_ISO_KEYRING_DIR"/*.plist "$KEYS_OUT/"
else
	die "a remote Ryoku repository requires RYOKU_VOID_ISO_KEYRING_DIR"
fi

repositories=(
	-R "$VOID_MIRROR"
	-R "$VOID_MIRROR/nonfree"
	-R "$VOID_MIRROR/multilib"
	-R "$VOID_MIRROR/multilib/nonfree"
	-R "$RYOKU_REPO"
)

mapfile -t target_packages < <(
	"$RESOLVE" \
		--lane system \
		--lane desktop \
		--lane dev \
		--lane hardware:amd \
		--lane hardware:intel \
		--lane hardware:nvidia \
		--lane hardware:vm \
		--session
)
target_packages+=(
	base-system linux linux-headers dracut limine
	void-repo-nonfree void-repo-multilib void-repo-multilib-nonfree
)
mapfile -t target_packages < <(printf '%s\n' "${target_packages[@]}" | LC_ALL=C sort -u)

fetch_closure() {
	local name=$1
	shift
	local root=$WORK/root-$name
	rm -rf "$root"
	mkdir -p "$root"
	seed_keys "$root"
	log "Downloading $name closure ($# requested packages)"
	XBPS_ARCH=$ARCH xbps-install \
		-r "$root" -c "$WORK/cache" -i -S -D -y \
		"${repositories[@]}" "$@"
}

fetch_closure target "${target_packages[@]}"

# Fetch conflicting NVIDIA branches through separate roots into the shared cache.
nvidia_branches=(
	'nvidia nvidia-libs-32bit'
	'nvidia580 nvidia580-libs-32bit'
	'nvidia470 nvidia470-libs-32bit'
	'nvidia390 nvidia390-libs-32bit'
)
for branch in "${nvidia_branches[@]}"; do
	read -r -a packages <<< "$branch"
	fetch_closure "${packages[0]}" linux-headers "${packages[@]}"
done

shopt -s nullglob

link_or_copy_files() {
	local source
	for source in "$@"; do
		cp -al -- "$source" "$DEST/" 2>/dev/null \
			|| cp -a -- "$source" "$DEST/"
	done
}

cache_files=("$WORK/cache"/*.xbps "$WORK/cache"/*.xbps.sig2)
cache_archives=("$WORK/cache"/*.xbps)
((${#cache_archives[@]})) || die "XBPS downloaded no package archives"
link_or_copy_files "${cache_files[@]}"

# XBPS reads local repositories in place, so their packages never enter cache.
if [[ -d $RYOKU_REPO ]]; then
	local_files=("$RYOKU_REPO"/*.xbps "$RYOKU_REPO"/*.xbps.sig2)
	local_archives=("$RYOKU_REPO"/*.xbps)
	((${#local_archives[@]})) || die "local Ryoku repository has no package archives"
	link_or_copy_files "${local_files[@]}"
fi

packages=("$DEST"/*.xbps)
log "Indexing ${#packages[@]} package archives"
XBPS_TARGET_ARCH=$ARCH xbps-rindex --add "${packages[@]}"
XBPS_TARGET_ARCH=$ARCH xbps-rindex --remove-obsoletes "$DEST"
XBPS_TARGET_ARCH=$ARCH xbps-rindex --hashcheck --clean "$DEST"
[[ -s $DEST/$ARCH-repodata ]] || die "$ARCH-repodata was not created"
concrete_headers=("$DEST"/linux[0-9]*-headers-*.xbps)
((${#concrete_headers[@]})) \
	|| die "offline closure has linux-headers but no concrete kernel-series headers"

# XBPS checks root free space even for dry runs that write almost nothing.
verify_closure() {
	local name=$1
	shift
	local root=$VERIFY_ROOT/verify-$name
	mkdir -p "$root"
	seed_keys "$root"
	XBPS_ARCH=$ARCH xbps-install -r "$root" -i -R "$DEST" -Sny "$@" >/dev/null
}
verify_closure target "${target_packages[@]}"
for branch in "${nvidia_branches[@]}"; do
	read -r -a packages <<< "$branch"
	verify_closure "${packages[0]}" linux-headers "${packages[@]}"
done

size=$(du -sh "$DEST" | awk '{print $1}')
log "Offline repository ready at $DEST ($size)"
