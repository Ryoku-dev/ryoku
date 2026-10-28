#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT=$(cd "$SCRIPT_DIR/../../.." && pwd)
ENGINE=${RYOKU_CONTAINER_ENGINE:-docker}
IMAGE=${RYOKU_XBPS_IMAGE:-ghcr.io/void-linux/void-glibc-full}
HOST_WORK=${RYOKU_XBPS_WORK:-${TMPDIR:-/tmp}/ryoku-xbps-work-$UID}
HOST_OUT=${RYOKU_XBPS_OUT:-$SCRIPT_DIR/out}

for command in git tar sha256sum curl jq; do
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

# Ryotunes ships on its own release train. Build its latest stable release, as
# the Arch importer and the Fedora fetch hook do; a pin in the template only
# ever fell behind. The token stays on the host: only the version and the
# release's own checksum enter the build container.
latest_ryotunes() {
	local api=${RYOKU_GITHUB_API:-https://api.github.com}
	local download=${RYOKU_GITHUB_DL:-https://github.com}
	local token=${GH_TOKEN:-${GITHUB_TOKEN:-}} release tag version sums sum file extra
	local -a auth=()
	[[ -z $token ]] || auth=(-H "Authorization: Bearer $token")
	release=$(curl -fsSL --retry 3 -H 'Accept: application/vnd.github+json' \
		"${auth[@]}" "$api/repos/Ryoku-dev/ryotunes/releases/latest") || return 1
	[[ $(jq -r '.draft' <<<"$release") == false && $(jq -r '.prerelease' <<<"$release") == false ]] \
		|| return 1
	tag=$(jq -r '.tag_name // empty' <<<"$release")
	[[ $tag =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || return 1
	version=${tag#v}
	sums=$(curl -fsSL --retry 3 \
		"$download/Ryoku-dev/ryotunes/releases/download/$tag/ryotunes-$version.tar.gz.sha256") || return 1
	[[ $(grep -c . <<<"$sums") == 1 ]] || return 1
	read -r sum file extra <<<"$sums"
	[[ $sum =~ ^[0-9a-fA-F]{64}$ && ${file#\*} == "ryotunes-$version.tar.gz" && -z ${extra:-} ]] \
		|| return 1
	printf '%s %s\n' "$version" "${sum,,}"
}
read -r ryotunes_version ryotunes_sha256 < <(latest_ryotunes) || {
	printf 'container-runner.sh: could not resolve the latest Ryoku-dev/ryotunes release\n' >&2
	exit 1
}

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
	-e "RYOKU_VOID_PACKAGES_REF=${RYOKU_VOID_PACKAGES_REF:-7023eba97f43b92c4d36e830677d27ab33e2d4ae}"
	-e "RYOKU_XBPS_PACKAGES=${RYOKU_XBPS_PACKAGES:-}"
	-e "RYOKU_XBPS_WORKTREE=${RYOKU_XBPS_WORKTREE:-0}"
	-e "RYOKU_XBPS_SOURCE_ARCHIVE=/run/ryoku-source.tar.gz"
	-e "RYOKU_XBPS_COMMIT=$commit"
	-e "RYOKU_RELEASE=$release"
	-e "RYOKU_CHANNEL=$channel"
	-e "RYOKU_NAME=$name"
	-e "RYOKU_PKGVER=$package_version"
	-e "RYOKU_XBPS_PRODUCTION=${RYOKU_XBPS_PRODUCTION:-0}"
	-e "XBPS_PASSPHRASE=${XBPS_PASSPHRASE:-}"
	-e "RYOKU_RYOTUNES_VERSION=$ryotunes_version"
	-e "RYOKU_RYOTUNES_SHA256=$ryotunes_sha256")

if [[ -n ${RYOKU_XBPS_KEY:-} ]]; then
	[[ -f $RYOKU_XBPS_KEY ]] || { printf 'container-runner.sh: key not found: %s\n' "$RYOKU_XBPS_KEY" >&2; exit 1; }
	run+=( -v "$(cd "$(dirname "$RYOKU_XBPS_KEY")" && pwd -P)/$(basename "$RYOKU_XBPS_KEY"):/run/ryoku-private.pem:ro" )
fi
if [[ -n ${RYOKU_XBPS_KEYRING_DIR:-} ]]; then
	[[ -d $RYOKU_XBPS_KEYRING_DIR ]] || { printf 'container-runner.sh: keyring directory not found: %s\n' "$RYOKU_XBPS_KEYRING_DIR" >&2; exit 1; }
	run+=( -v "$(cd "$RYOKU_XBPS_KEYRING_DIR" && pwd -P):/run/ryoku-keyring:ro" )
fi
if [[ -n ${RYOKU_XBPS_MIRROR:-} ]]; then
	[[ -d $RYOKU_XBPS_MIRROR ]] || { printf 'container-runner.sh: package mirror not found: %s\n' "$RYOKU_XBPS_MIRROR" >&2; exit 1; }
	run+=( -v "$(cd "$RYOKU_XBPS_MIRROR" && pwd -P):/run/ryoku-mirror:ro" )
	run+=( -e "RYOKU_XBPS_MIRROR=/run/ryoku-mirror" )
fi

# The official image needs bash installed before the build script takes over.
inner='
	xbps-install -y sudo git openssh zstd
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
		RYOKU_XBPS_MIRROR="${RYOKU_XBPS_MIRROR:-}" \
		RYOKU_XBPS_PRODUCTION="$RYOKU_XBPS_PRODUCTION" \
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
		RYOKU_RYOTUNES_VERSION="$RYOKU_RYOTUNES_VERSION" \
		RYOKU_RYOTUNES_SHA256="$RYOKU_RYOTUNES_SHA256" \
		/ryoku/void/packages/repo/build-repo.sh
'
run+=("$IMAGE" sh -c 'set -e
	xbps-install -Syu xbps || xbps-install -yu xbps
	xbps-install -Syu
	xbps-install -y bash
	exec bash -euo pipefail -c "$1"' ryoku-runner "$inner")

"${run[@]}"
