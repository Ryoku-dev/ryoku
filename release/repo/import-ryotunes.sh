#!/usr/bin/env bash
# Import one independently released Ryoku app package, verify its release
# checksum and pacman identity, and preserve the official package bytes.
#
# RYOKU_APP selects ryotunes (default) or ryoport. Ryotunes carries epoch 1;
# Ryoport has no epoch. Asset filenames are epochless for both.
#
# usage:
#   RYOKU_APP=ryoport import-ryotunes.sh <OUT_DIR>
#
# env:
#   RYOKU_APP                 ryotunes (default) or ryoport
#   RYOKU_REPO_KEY            optional signing key
#   RYOKU_GITHUB_API          GitHub API base
#   RYOKU_GITHUB_DL           GitHub download base
#   GH_TOKEN / GITHUB_TOKEN   optional API token
#   RYOKU_IMPORT_RESOLVE_ONLY print the pacman version without downloading
set -euo pipefail

APP=${RYOKU_APP:-ryotunes}
case "$APP" in
  ryotunes) REPO=${RYOKU_RYOTUNES_REPO:-ryoku-dev/ryotunes}; WANT_EPOCH=1 ;;
  ryoport) REPO=${RYOKU_RYOPORT_REPO:-ryoku-dev/ryoport}; WANT_EPOCH= ;;
  *) printf 'import-app: error: unsupported app %s\n' "$APP" >&2; exit 1 ;;
esac
API=${RYOKU_GITHUB_API:-https://api.github.com}
DL=${RYOKU_GITHUB_DL:-https://github.com}
WANT_ARCH=x86_64

log() { printf '\033[1;35m::\033[0m %s\n' "$*" >&2; }
die() { printf 'import-app: error: %s\n' "$*" >&2; exit 1; }

RESOLVE_ONLY=${RYOKU_IMPORT_RESOLVE_ONLY:-}
OUT_DIR=${1:-}
[[ -n $OUT_DIR || -n $RESOLVE_ONLY ]] || die "usage: RYOKU_APP=<ryotunes|ryoport> import-ryotunes.sh <OUT_DIR>"

for tool in curl jq bsdtar sha256sum; do
  command -v "$tool" >/dev/null 2>&1 || die "$tool not found (need curl, jq, bsdtar, sha256sum)"
done
[[ -z ${RYOKU_REPO_KEY:-} ]] || command -v gpg >/dev/null 2>&1 || die "gpg not found but RYOKU_REPO_KEY is set"


# curl for the API: authenticated when a token is present (higher rate limit),
# never required for the public asset download below.
api_get() {
  local url=$1
  local -a auth=()
  local tok=${GH_TOKEN:-${GITHUB_TOKEN:-}}
  [[ -n $tok ]] && auth=(-H "Authorization: Bearer $tok")
  curl -fsSL --retry 3 -H 'Accept: application/vnd.github+json' \
    -H 'User-Agent: ryoku-repo-import' "${auth[@]}" "$url"
}

# 1. resolve the newest published stable release. A draft/prerelease is never
#    returned by releases/latest; the tag must be a clean vX.Y.Z.
release_json=$(api_get "$API/repos/$REPO/releases/latest") \
  || die "could not fetch the latest $REPO release"
tag=$(jq -r '.tag_name // empty' <<<"$release_json")
[[ $tag =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "latest $REPO tag '$tag' is not a vX.Y.Z stable tag"
if [[ $APP == ryoport && ! $tag =~ ^v1\.[0-9]+\.[0-9]$ ]]; then
  die "latest Ryoport tag '$tag' is not v1.<minor>.<digit>"
fi
pkgver=${tag#v}

# 2. locate THE single package asset for this version. The name is matched
#    strictly (dots escaped so nothing but the real version can match) and the
#    pkgrel must be a positive integer with no leading zero; more than one match,
#    or none, is treated as untrustworthy rather than "pick one".
pv_re=${pkgver//./\\.}
pkgrel_re='[1-9][0-9]*'
[[ $APP == ryoport ]] && pkgrel_re=1
mapfile -t matches < <(jq -r '.assets[].name' <<<"$release_json" \
  | grep -E "^${APP}-${pv_re}-${pkgrel_re}-${WANT_ARCH}\.pkg\.tar\.zst$" || true)
(( ${#matches[@]} == 1 )) \
  || die "release $tag offers ${#matches[@]} $APP ${WANT_ARCH} package assets (want exactly 1)"
asset=${matches[0]}
rest=${asset#"$APP"-"${pkgver}"-}
pkgrel=${rest%-"${WANT_ARCH}".pkg.tar.zst}
version="${pkgver}-${pkgrel}"
[[ -z $WANT_EPOCH ]] || version="${WANT_EPOCH}:${version}"

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
#    install to bytes off another host (parity with internal/apprelease).
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

# 5. verify the package identity from its own .PKGINFO.
pkginfo=$(bsdtar -xOf "$dir/$asset" .PKGINFO 2>/dev/null) || die "cannot read .PKGINFO from $asset"
field() { awk -F ' = ' -v key="$1" '$1 == key {print $2; exit}' <<<"$pkginfo"; }
meta_name=$(field pkgname)
meta_arch=$(field arch)
meta_ver=$(field pkgver)
[[ $meta_name == "$APP" ]]        || die "$asset identifies as '$meta_name', not $APP"
[[ $meta_arch == "$WANT_ARCH" ]] || die "$asset architecture '$meta_arch', want $WANT_ARCH"
[[ $meta_ver == "$version" ]]    || die ".PKGINFO version '$meta_ver' is not the expected '$version'"

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
