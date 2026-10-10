#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT=$(cd "$SCRIPT_DIR/../../.." && pwd)
ENGINE=${RYOKU_CONTAINER_ENGINE:-docker}
IMAGE=${RYOKU_XBPS_IMAGE:-ghcr.io/void-linux/void-glibc-full}
HOST_WORK=${RYOKU_XBPS_WORK:-${TMPDIR:-/tmp}/ryoku-xbps-work-$UID}
HOST_OUT=${RYOKU_XBPS_OUT:-$SCRIPT_DIR/out}

for command in git tar sha256sum; do
	command -v "$command" >/dev/null 2>&1 || {
		printf 'container-runner.sh: required host command not found: %s\n' "$command" >&2
		exit 1
	}
done

raw_version=${RYOKU_PKGVER:-$("$ROOT/bin/ryoku-release-version" --pkgver)}
base_version=${raw_version//-/.}
base_version=${base_version//_/.}
[[ $base_version =~ ^[0-9][A-Za-z0-9.]*$ ]] || {
	printf 'container-runner.sh: invalid XBPS package version: %s\n' "$raw_version" >&2
	exit 1
}

source_tmp=$(mktemp -d "${TMPDIR:-/tmp}/ryoku-xbps-source.XXXXXX")
trap 'rm -rf "$source_tmp"' EXIT HUP INT TERM

write_worktree_archive() {
	local version=$1 output=$2
	(
		cd "$ROOT"
		git ls-files -co --exclude-standard -z \
			| while IFS= read -r -d '' path; do
				case $path in
					void/packages/repo/out/*|void/packages/repo/work/*) continue ;;
				esac
				printf '%s\0' "$path"
			done \
			| LC_ALL=C sort -z \
			| tar --create --gzip --file="$output" --null --no-recursion \
				--transform="flags=r;s,^,ryoku-$version/," --files-from=-
	)
}

package_version=$base_version
if [[ ${RYOKU_XBPS_WORKTREE:-0} == 1 ]]; then
	canonical_archive=$source_tmp/ryoku-worktree.tar.gz
	write_worktree_archive "$base_version" "$canonical_archive"
	source_hash=$(sha256sum "$canonical_archive" | awk '{print $1}')
	package_version=$base_version.wt${source_hash:0:10}
	source_archive=$source_tmp/ryoku-$package_version.tar.gz
	write_worktree_archive "$package_version" "$source_archive"
else
	source_archive=$source_tmp/ryoku-$package_version.tar.gz
	git -C "$ROOT" archive --format=tar.gz --prefix="ryoku-$package_version/" \
		-o "$source_archive" HEAD
fi

release=${RYOKU_RELEASE:-local-$package_version}
channel=${RYOKU_CHANNEL:-local}
name=${RYOKU_NAME:-$(tr -d '[:space:]' < "$ROOT/CODENAME")}
commit=$(git -C "$ROOT" rev-parse HEAD)

command -v "$ENGINE" >/dev/null 2>&1 || {
	printf 'container-runner.sh: container engine not found: %s\n' "$ENGINE" >&2
	exit 1
}
mkdir -p "$HOST_WORK" "$HOST_OUT"
HOST_WORK=$(cd "$HOST_WORK" && pwd -P)
HOST_OUT=$(cd "$HOST_OUT" && pwd -P)

run=("$ENGINE" run --rm --platform linux/amd64 --privileged
	-v /dev:/dev
	-v "$ROOT:/ryoku:ro"
	-v "$HOST_WORK:/work"
	-v "$HOST_OUT:/out"
	-v "$source_archive:/run/ryoku-source.tar.gz:ro"
	-e "RYOKU_VOID_PACKAGES_REF=${RYOKU_VOID_PACKAGES_REF:-deb0bc286bd5e1fbbf191802c3f578caf4fcdd94}"
	-e "RYOKU_XBPS_PACKAGES=${RYOKU_XBPS_PACKAGES:-}"
	-e "RYOKU_XBPS_WORKTREE=${RYOKU_XBPS_WORKTREE:-0}"
	-e "RYOKU_XBPS_SOURCE_ARCHIVE=/run/ryoku-source.tar.gz"
	-e "RYOKU_XBPS_COMMIT=$commit"
	-e "RYOKU_RELEASE=$release"
	-e "RYOKU_CHANNEL=$channel"
	-e "RYOKU_NAME=$name"
	-e "RYOKU_PKGVER=$package_version"
	-e "XBPS_PASSPHRASE=${XBPS_PASSPHRASE:-}")

if [[ -n ${RYOKU_XBPS_KEY:-} ]]; then
	[[ -f $RYOKU_XBPS_KEY ]] || { printf 'container-runner.sh: key not found: %s\n' "$RYOKU_XBPS_KEY" >&2; exit 1; }
	run+=( -v "$(cd "$(dirname "$RYOKU_XBPS_KEY")" && pwd -P)/$(basename "$RYOKU_XBPS_KEY"):/run/ryoku-private.pem:ro" )
fi
if [[ -n ${RYOKU_XBPS_KEYRING_DIR:-} ]]; then
	[[ -d $RYOKU_XBPS_KEYRING_DIR ]] || { printf 'container-runner.sh: keyring directory not found: %s\n' "$RYOKU_XBPS_KEYRING_DIR" >&2; exit 1; }
	run+=( -v "$(cd "$RYOKU_XBPS_KEYRING_DIR" && pwd -P):/run/ryoku-keyring:ro" )
fi

# The official image ships only sh, so bash is installed (with a current xbps)
# before the build script, which needs bash, takes over.
inner='
	xbps-install -y sudo git openssh
	getent group xbuilder >/dev/null || groupadd xbuilder
	id builder >/dev/null 2>&1 || useradd -m -G xbuilder builder
	chown -R builder:builder /work /out
	key=
	if [[ -r /run/ryoku-private.pem ]]; then
		cp /run/ryoku-private.pem /work/.ryoku-private.pem
		chown builder:builder /work/.ryoku-private.pem
		chmod 600 /work/.ryoku-private.pem
		key=/work/.ryoku-private.pem
		trap "rm -f /work/.ryoku-private.pem" EXIT
	fi
	keyring=
	if [[ -d /run/ryoku-keyring ]]; then
		rm -rf /work/.ryoku-keyring
		mkdir /work/.ryoku-keyring
		cp -a /run/ryoku-keyring/. /work/.ryoku-keyring/
		chown -R builder:builder /work/.ryoku-keyring
		keyring=/work/.ryoku-keyring
	fi
	sudo -Eu builder env \
		HOME=/home/builder \
		RYOKU_XBPS_WORK=/work \
		RYOKU_XBPS_OUT=/out \
		RYOKU_XBPS_KEY="$key" \
		RYOKU_XBPS_KEYRING_DIR="$keyring" \
		RYOKU_VOID_PACKAGES_REF="$RYOKU_VOID_PACKAGES_REF" \
		RYOKU_XBPS_PACKAGES="$RYOKU_XBPS_PACKAGES" \
		RYOKU_XBPS_WORKTREE="$RYOKU_XBPS_WORKTREE" \
		RYOKU_XBPS_SOURCE_ARCHIVE="$RYOKU_XBPS_SOURCE_ARCHIVE" \
		RYOKU_XBPS_COMMIT="$RYOKU_XBPS_COMMIT" \
		RYOKU_RELEASE="$RYOKU_RELEASE" \
		RYOKU_CHANNEL="$RYOKU_CHANNEL" \
		RYOKU_NAME="$RYOKU_NAME" \
		RYOKU_PKGVER="$RYOKU_PKGVER" \
		XBPS_PASSPHRASE="$XBPS_PASSPHRASE" \
		/ryoku/void/packages/repo/build-repo.sh
'
run+=("$IMAGE" sh -c 'set -e
	xbps-install -Syu xbps || xbps-install -yu xbps
	xbps-install -Syu
	xbps-install -y bash
	exec bash -euo pipefail -c "$1"' ryoku-runner "$inner")

"${run[@]}"
