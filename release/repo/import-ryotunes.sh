#!/usr/bin/env bash
# import-ryotunes.sh: fetch the current official Ryotunes package from its own
# GitHub release channel, verify it, and drop the verified (optionally signed)
# bytes into an output directory. It is the single importer both the full repo
# build (release/repo/build-repo.sh) and the narrow scheduled refresh reuse, so
# the download / epoch / checksum rules live in exactly one place.
#
# Ryotunes is released on its own cadence as a prebuilt Arch package on
# ryoku-dev/ryotunes, NOT built in the [ryoku] repo. This importer consumes that
# official artifact instead of rebuilding a divergent app: the package's own
# .PKGINFO (its real depends, including quickshell, and its epoch=1 version)
# carries through untouched, and the [ryoku] repo only re-signs it with the
# release key so pacman's signature chain stays intact.
#
# The published contract (kept in lockstep with internal/ryotunesrelease, the
# in-client updater):
#   - the package asset name is EPOCHLESS: ryotunes-<pkgver>-<pkgrel>-x86_64.pkg.tar.zst
#   - the package's own .PKGINFO version carries epoch=1: 1:<pkgver>-<pkgrel>
#   - a sha256 sidecar asset sits beside it: <asset>.sha256 ("HEX␠␠filename")
# The epoch is what lets a current build outrank the retired, divergently
# high-versioned build the [ryoku] repo used to ship (2.5.1-1): under pacman
# ordering 1:x-y beats any epoch-0 version, so an old box moves forward.
#
# usage:
#   import-ryotunes.sh <OUT_DIR>
#       resolve the newest 1.x release, download + verify the package (and its
#       sha256), verify its .PKGINFO identity/epoch, place it in <OUT_DIR>, sign
#       it when RYOKU_REPO_KEY is set, and print the package path on the last
#       stdout line. All progress goes to stderr, so `pkg=$(import-ryotunes.sh
#       "$dir")` captures just the path.
#
# env:
#   RYOKU_REPO_KEY            gpg key id; when set, a detached <pkg>.sig is written
#   RYOKU_RYOTUNES_REPO       source repo slug     (default: ryoku-dev/ryotunes)
#   RYOKU_GITHUB_API          GitHub API base      (default: https://api.github.com)
#   RYOKU_GITHUB_DL           GitHub download base (default: https://github.com)
#   GH_TOKEN / GITHUB_TOKEN   optional; sent as a Bearer token to lift the
#                             unauthenticated API rate limit (asset downloads are
#                             public and never need it)
#   RYOKU_IMPORT_RESOLVE_ONLY when set, print the resolved pacman version
#                             (1:<pkgver>-<pkgrel>) and exit WITHOUT downloading,
#                             so a poll can decide whether a refresh is even
#                             needed before spending a download.
#
# exit 3 means the repo lists no Ryotunes 1.x release (see
# resolve-ryotunes-release.sh); nothing is imported.
set -euo pipefail

REPO=${RYOKU_RYOTUNES_REPO:-ryoku-dev/ryotunes}
API=${RYOKU_GITHUB_API:-https://api.github.com}
HERE=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
DL=${RYOKU_GITHUB_DL:-https://github.com}
WANT_ARCH=x86_64
WANT_EPOCH=1

log() { printf '\033[1;35m::\033[0m %s\n' "$*" >&2; }
die() { printf 'import-ryotunes.sh: error: %s\n' "$*" >&2; exit 1; }

RESOLVE_ONLY=${RYOKU_IMPORT_RESOLVE_ONLY:-}
OUT_DIR=${1:-}
[[ -n $OUT_DIR || -n $RESOLVE_ONLY ]] || die "usage: import-ryotunes.sh <OUT_DIR>"

for tool in curl jq bsdtar sha256sum; do
  command -v "$tool" >/dev/null 2>&1 || die "$tool not found (need curl, jq, bsdtar, sha256sum)"
done
[[ -z ${RYOKU_REPO_KEY:-} ]] || command -v gpg >/dev/null 2>&1 || die "gpg not found but RYOKU_REPO_KEY is set"

# 1. resolve the newest stable 1.x release. GitHub's "latest" can be a
#    Ryotunes 2 build, which never belongs in this Ryoku line's repo.
rc=0
release_json=$(RYOKU_RYOTUNES_REPO=$REPO RYOKU_GITHUB_API=$API \
  bash "$HERE/resolve-ryotunes-release.sh") || rc=$?
(( rc != 3 )) || { printf 'import-ryotunes.sh: %s lists no Ryotunes 1.x release\n' "$REPO" >&2; exit 3; }
(( rc == 0 )) || die "could not resolve the newest $REPO 1.x release"
tag=$(jq -r '.tag_name // empty' <<<"$release_json")
[[ $tag =~ ^v1\.[0-9]+\.[0-9]+$ ]] || die "$REPO tag '$tag' is not a v1.Y.Z stable tag"
pkgver=${tag#v}

# 2. locate THE single package asset for this version. The name is matched
#    strictly (dots escaped so nothing but the real version can match) and the
#    pkgrel must be a positive integer with no leading zero; more than one match,
#    or none, is treated as untrustworthy rather than "pick one".
pv_re=${pkgver//./\\.}
mapfile -t matches < <(jq -r '.assets[].name' <<<"$release_json" \
  | grep -E "^ryotunes-${pv_re}-[1-9][0-9]*-${WANT_ARCH}\.pkg\.tar\.zst$" || true)
(( ${#matches[@]} == 1 )) \
  || die "release $tag offers ${#matches[@]} ryotunes ${WANT_ARCH} package assets (want exactly 1)"
asset=${matches[0]}
rest=${asset#ryotunes-"${pkgver}"-}
pkgrel=${rest%-"${WANT_ARCH}".pkg.tar.zst}
version="${WANT_EPOCH}:${pkgver}-${pkgrel}"

# resolve-only: hand the caller the pacman version so a poll can compare it to
# what the mirror already serves and skip a needless refresh.
if [[ -n $RESOLVE_ONLY ]]; then
  printf '%s\n' "$version"
  exit 0
fi

sidecar="$asset.sha256"
jq -e --arg s "$sidecar" '.assets[] | select(.name == $s)' <<<"$release_json" >/dev/null \
  || die "release $tag is missing the checksum sidecar $sidecar"

# 3. download the package and its sidecar. The URL is REBUILT from the trusted
#    download base, repo, tag and asset name -- never the API's own
#    browser_download_url -- so a stubbed or hijacked API can never redirect the
#    install to bytes off another host (parity with internal/ryotunesrelease).
dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
base="$DL/$REPO/releases/download/$tag"
log "Importing $asset from $REPO release $tag"
curl -fsSL --retry 3 -o "$dir/$asset"   "$base/$asset"   || die "download of $asset failed"
curl -fsSL --retry 3 -o "$dir/$sidecar" "$base/$sidecar" || die "download of $sidecar failed"

# 4. verify the bytes against the release's checksum sidecar, bound to EXACTLY
#    this asset. The sidecar must be a single sha256sum line naming exactly
#    $asset; parse its one hash + filename, confirm the filename is our asset
#    (basename only, so an absolute or traversal path is refused), and compare
#    the hash against the package we actually downloaded. A missing, extra, or
#    mismatched entry is refused -- we never let `sha256sum -c` validate some
#    other file the sidecar happened to name and then accept unchecked bytes.
mapfile -t _sc < <(grep -vE '^[[:space:]]*$' "$dir/$sidecar")
(( ${#_sc[@]} == 1 )) || die "checksum sidecar for $asset must hold exactly one entry (found ${#_sc[@]})"
read -r want_hex want_name extra <<<"${_sc[0]}"
want_name=${want_name#\*}                          # strip sha256sum's binary-mode marker
[[ -n $want_name ]]                  || die "checksum sidecar has no filename field"
[[ $want_name == "$asset" && -z $extra ]] || die "checksum sidecar must name exactly $asset"
[[ $want_hex =~ ^[0-9a-fA-F]{64}$ ]] || die "malformed sha256 '$want_hex' in the sidecar"
got_hex=$(sha256sum "$dir/$asset" | awk '{print $1}')
[[ "${want_hex,,}" == "${got_hex,,}" ]] || die "sha256 mismatch on $asset (got $got_hex, want ${want_hex,,})"

# 5. verify the package is what we think it is, from its OWN .PKGINFO rather than
#    its filename: the name, the architecture, and the full epoch-carrying
#    version (1:<pkgver>-<pkgrel>). This is where the epoch=1 contract is
#    enforced -- a package whose real epoch is not 1 is refused.
pkginfo=$(bsdtar -xOf "$dir/$asset" .PKGINFO 2>/dev/null) || die "cannot read .PKGINFO from $asset"
field() { sed -n "s/^$1 = //p" <<<"$pkginfo" | head -n1; }
meta_name=$(field pkgname)
meta_arch=$(field arch)
meta_ver=$(field pkgver)
[[ $meta_name == ryotunes ]]     || die "$asset identifies as '$meta_name', not ryotunes"
[[ $meta_arch == "$WANT_ARCH" ]] || die "$asset architecture '$meta_arch', want $WANT_ARCH"
[[ $meta_ver == "$version" ]]    || die ".PKGINFO version '$meta_ver' is not the expected epoch=1 '$version'"

# 6. place the verified bytes in OUT_DIR and, when a key is given, sign them with
#    the release key. The exact file that was verified is the file published --
#    it is never re-fetched between verification and this move.
mkdir -p "$OUT_DIR"
OUT_DIR=$(cd "$OUT_DIR" && pwd) || die "cannot resolve OUT_DIR to an absolute path"
mv -f "$dir/$asset" "$OUT_DIR/$asset"
if [[ -n ${RYOKU_REPO_KEY:-} ]]; then
  gpg --batch --yes --detach-sign -u "$RYOKU_REPO_KEY" -o "$OUT_DIR/$asset.sig" "$OUT_DIR/$asset" \
    || die "could not sign $asset with $RYOKU_REPO_KEY"
  log "Imported and signed $asset ($version)"
else
  log "Imported $asset ($version) unsigned (RYOKU_REPO_KEY not set)"
fi

# the package path is the last line of stdout; everything else went to stderr.
printf '%s\n' "$OUT_DIR/$asset"
