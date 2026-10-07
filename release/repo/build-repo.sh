#!/usr/bin/env bash
# build, sign, and assemble the [ryoku] pacman repo.
#
# walks release/packages/*/PKGBUILD, makepkg-builds each, signs with the ryoku
# release key, then `repo-add -s` to produce the signed db. layout matches the
# public mirror at https://repo.ryoku.dev/<arch>/: an <arch>/ subdir holding
# the *.pkg.tar.zst, their *.sig detached signatures, and ryoku.db /
# ryoku.db.sig (+ ryoku.files). publish workflow rclones that <arch>/ straight
# to R2.
#
# db is rebuilt from the package set every run: <arch>/ wiped, repacked from
# exactly the packages produced now. never `repo-add --new`. so: idempotent,
# and a removed package dir simply falls out of the db.
#
# build deps live on the build host only: Go and Qt build the Ryoku binaries,
# Rust builds the native tools, and asusctl additionally needs clang, llvm, and
# libusb. The compositor plugins need Hyprland, hyprcursor, pango, cairo, and
# pkgconf; gradle builds the vendored limine-entry-tool stack with GraalVM
# nativeCompile.
# makepkg runs --nodeps on purpose: runtime depends (and AUR depends) aren't
# needed to compile the artifacts and aren't resolvable here anyway.
#
# usage: ./build-repo.sh
#
# env overrides:
#   RYOKU_REPO_OUT      output dir            (default: ./out beside this script)
#   RYOKU_REPO_KEY      gpg key id to sign    (default: release key fingerprint)
#   RYOKU_REPO_NAME     repo db base name     (default: ryoku)
#   RYOKU_REPO_ARCH     target architecture   (default: x86_64)
#   RYOKU_PACKAGES_DIR  PKGBUILD parent dir   (default: <repo>/release/packages)
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)   # release/repo
RELEASE_DIR=$(cd "$SCRIPT_DIR/.." && pwd)                  # release

OUT_DIR=${RYOKU_REPO_OUT:-$SCRIPT_DIR/out}
KEY_ID=${RYOKU_REPO_KEY:-EB6D3C0F55A7B3CABA6B2838847B274F025DD6E3}
REPO_NAME=${RYOKU_REPO_NAME:-ryoku}
REPO_ARCH=${RYOKU_REPO_ARCH:-x86_64}
PACKAGES_DIR=${RYOKU_PACKAGES_DIR:-$RELEASE_DIR/packages}

ARCH_DIR=$OUT_DIR/$REPO_ARCH
DB_PATH=$ARCH_DIR/$REPO_NAME.db.tar.gz

log() { printf '\033[1;35m::\033[0m %s\n' "$*"; }
die() { printf 'build-repo.sh: error: %s\n' "$*" >&2; exit 1; }

# 0. preflight. makepkg refuses root; everything else just wants the pacman
#    tools and the release secret key in GNUPGHOME.
[[ $EUID -ne 0 ]] || die "makepkg refuses to run as root; run as a regular user"
command -v makepkg  >/dev/null 2>&1 || die "makepkg not found (pacman -S base-devel)"
command -v repo-add >/dev/null 2>&1 || die "repo-add not found (ships with pacman)"
command -v gpg      >/dev/null 2>&1 || die "gpg not found (pacman -S gnupg)"
[[ -d $PACKAGES_DIR ]] || die "packages dir not found: $PACKAGES_DIR"
gpg --list-secret-keys "$KEY_ID" >/dev/null 2>&1 \
  || die "release signing key $KEY_ID not in GNUPGHOME=${GNUPGHOME:-$HOME/.gnupg}"

shopt -s nullglob
pkgbuilds=("$PACKAGES_DIR"/*/PKGBUILD)
(( ${#pkgbuilds[@]} > 0 )) || die "no PKGBUILDs found under $PACKAGES_DIR"

# 1. fresh output tree. wiping the arch dir is what keeps the repo idempotent:
#    db gets rebuilt from exactly the packages produced this run, no stale
#    versions left to confuse repo-add.
log "Output dir -> $ARCH_DIR"
rm -rf "$ARCH_DIR"
mkdir -p "$ARCH_DIR"

# 2. build + sign every package straight into the arch dir. with --sign
#    makepkg writes the package and its detached .sig to PKGDEST. --nodeps
#    skips the (unresolvable, unneeded) runtime depends; --clean clears the
#    per-build $srcdir/$pkgdir so the checked-out source is left alone.
export PKGDEST=$ARCH_DIR
export PKGEXT='.pkg.tar.zst'

# per-build version for the monorepo packages: core semver + commit count +
# short sha (bin/ryoku-release-version --pkgver). every published build is
# then strictly newer + commit-identifiable in pacman terms, so `ryoku update`
# (pacman -Syu) actually sees an upgrade after each push and the Hub can show
# the commit. monorepo PKGBUILDs read RYOKU_PKGVER; ryoku-keyring and gpk
# keep their own versions (key-rotation date, upstream GlazePKG releases).
# overridable.
: "${RYOKU_PKGVER:=$("$RELEASE_DIR/../bin/ryoku-release-version" --pkgver)}"
export RYOKU_PKGVER
log "Monorepo package version -> $RYOKU_PKGVER"

# the named state this build is (publish-repo.yml sets both): a release tag on
# the stable channel, or the dev version on testing. ryoku-desktop writes them
# to /etc/ryoku-release so a box can say which release it runs; release.json
# beside the db says which one the channel serves. a dev box building by hand
# gets a local marker, never a name that looks like a published release.
: "${RYOKU_RELEASE:=local-$RYOKU_PKGVER}"
: "${RYOKU_CHANNEL:=local}"
# the line's name (CODENAME, see release/names.md): every release in a line
# carries it, and a box shows it next to the release it runs.
RYOKU_NAME=$(tr -d '[:space:]' < "$RELEASE_DIR/../CODENAME")
export RYOKU_RELEASE RYOKU_CHANNEL RYOKU_NAME
log "Release -> $RYOKU_NAME $RYOKU_RELEASE ($RYOKU_CHANNEL)"
# makepkg's VCS sources (imgborders clones from Codeberg) make a build only as
# reliable as that host, and a Codeberg 5xx has repeatedly aborted the whole
# publish. Retry a failed build with backoff so a transient fetch outage rides
# out; a real build error still surfaces once the attempts are spent.
build_pkg() {
  local pkgdir=$1 attempt=1 max=3 delay=15
  while true; do
    if ( cd "$pkgdir" \
           && makepkg --force --clean --nodeps --noconfirm --sign --key "$KEY_ID" ); then
      return 0
    fi
    (( attempt >= max )) && return 1
    log "build of $(basename "$pkgdir") failed (attempt $attempt/$max); retrying in ${delay}s"
    sleep "$delay"; delay=$(( delay * 2 )); attempt=$(( attempt + 1 ))
  done
}

# a published filename never changes bytes. makepkg is not reproducible, so
# a fixed-version package (gpk, ryoku-keyring) rebuilt here would overwrite
# its live file with new bytes and break every client holding the previous db
# ("Maximum file size exceeded", issue #21). so a package whose output names
# the mirror already serves is not built at all: its served bytes are copied
# in and re-signed. this is what makes a release a promotion of the testing
# build (same commit, same names, same bytes) instead of a rebuild, and spares
# a pinned external (ryotunes, prowl) its compile on every
# push. shipping a change to a fixed-version package means bumping pkgrel.
MIRROR=${RYOKU_REPO_MIRROR:-https://repo.ryoku.dev/stable/$REPO_ARCH}

# from the mirror URL on a dev box, or from a directory when CI pre-fetched
# the bucket (Cloudflare 403s datacenter runners on the public domain).
fetch_published() {
  case $MIRROR in
    http://* | https://*) curl -fsSL --retry 3 -o "$2" "$MIRROR/$1" 2>/dev/null ;;
    *) [[ -f $MIRROR/$1 ]] && cp -f "$MIRROR/$1" "$2" ;;
  esac
}

# adopt_published <pkgdir>: 0 when every package the PKGBUILD would produce is
# already on the mirror and has been copied in and signed; 1 when it must be
# built. a -debug companion is taken when present and not required: makepkg
# only emits one when the build leaves debug files behind.
adopt_published() {
  local pkgdir=$1 names=() f tmp main_missing=0
  mapfile -t names < <(cd "$pkgdir" && makepkg --packagelist 2>/dev/null | xargs -rn1 basename)
  (( ${#names[@]} > 0 )) || return 1
  tmp=$(mktemp -d)
  for f in "${names[@]}"; do
    if fetch_published "$f" "$tmp/$f" && bsdtar -tf "$tmp/$f" >/dev/null 2>&1; then
      continue
    fi
    rm -f "$tmp/$f"
    [[ $f == *-debug-* ]] || main_missing=1
  done
  if (( main_missing )); then
    rm -rf "$tmp"
    return 1
  fi
  for f in "$tmp"/*.pkg.tar.zst; do
    mv -f "$f" "$ARCH_DIR/$(basename "$f")"
    gpg --batch --yes --detach-sign -u "$KEY_ID" -o "$ARCH_DIR/$(basename "$f").sig" "$ARCH_DIR/$(basename "$f")"
  done
  rm -rf "$tmp"
  return 0
}

for pkgbuild in "${pkgbuilds[@]}"; do
  pkgdir=$(dirname "$pkgbuild")
  if adopt_published "$pkgdir"; then
    log "Adopted published bytes for $(basename "$pkgdir") (already on the mirror; filenames are immutable once live)"
    continue
  fi
  log "Building $(basename "$pkgdir")"
  build_pkg "$pkgdir" || die "makepkg failed for $(basename "$pkgdir") after retries"
done


# 3. a published filename never changes bytes (see adopt_published above):
#    a name that was built here anyway but which the mirror already serves
#    keeps the served bytes, re-signed. the normal path adopts before
#    building; this catches a name that appeared on the mirror meanwhile.
for pkg in "$ARCH_DIR"/*.pkg.tar.zst; do
  name=$(basename "$pkg")
  if ! fetch_published "$name" "$pkg.published"; then
    rm -f "$pkg.published"   # not on the mirror yet: this build introduces it
    continue
  fi
  if ! bsdtar -tf "$pkg.published" >/dev/null 2>&1; then
    log "Mirror copy of $name is unreadable; publishing this build over it"
    rm -f "$pkg.published"
    continue
  fi
  if cmp -s "$pkg" "$pkg.published"; then
    rm -f "$pkg.published"
    continue
  fi
  log "Adopting published bytes for $name (filenames are immutable once live)"
  mv -f "$pkg.published" "$pkg"
  gpg --batch --yes --detach-sign -u "$KEY_ID" -o "$pkg.sig" "$pkg"
done

# Import after mirror adoption: verified official bytes must not be replaced by
# a cached distro build. The shared importer verifies checksum/identity/epoch
# and re-signs the unchanged package; the scheduled refresh uses it too.
log "Importing ryotunes from its GitHub release channel"
RYOKU_REPO_KEY="$KEY_ID" "$SCRIPT_DIR/import-ryotunes.sh" "$ARCH_DIR" >/dev/null \
  || die "could not import the official ryotunes release into $ARCH_DIR"

# 4. collect built packages, confirm each is signed before indexing.
packages=("$ARCH_DIR"/*.pkg.tar.zst)
(( ${#packages[@]} > 0 )) || die "no packages were built into $ARCH_DIR"
for pkg in "${packages[@]}"; do
  [[ -e $pkg.sig ]] || die "missing signature for $(basename "$pkg")"
done

# 5. signed db from the actual package set. no --new: the dir was wiped above,
#    so repo-add always starts from empty and indexes exactly what's there. -s
#    signs the db with the release key.
log "Indexing ${#packages[@]} package(s) into $REPO_NAME.db"
repo-add -s -k "$KEY_ID" "$DB_PATH" "${packages[@]}"

# 6. repo-add leaves ryoku.db / ryoku.db.sig / ryoku.files as symlinks to the
#    versioned tarballs. object storage (R2/S3) has no symlinks and pacman
#    fetches the bare names, so materialize them as real files in place.
for link in "$REPO_NAME.db" "$REPO_NAME.db.sig" "$REPO_NAME.files" "$REPO_NAME.files.sig"; do
  target=$ARCH_DIR/$link
  [[ -L $target ]] || continue
  resolved=$(readlink -f "$target")
  rm -f "$target"
  cp "$resolved" "$target"
done

[[ -e $ARCH_DIR/$REPO_NAME.db ]]     || die "$REPO_NAME.db missing after repo-add"
[[ -e $ARCH_DIR/$REPO_NAME.db.sig ]] || die "$REPO_NAME.db.sig missing; signing failed"

# 7. release.json beside the db: what this directory serves. `ryoku status`
#    reads it from the channel to name the release a box would move to, and
#    publish-repo.yml copies its version into releases/index.json.
commit=$(git -C "$RELEASE_DIR/.." rev-parse HEAD 2>/dev/null || echo unknown)
printf '{"schema":1,"release":"%s","name":"%s","channel":"%s","version":"%s","commit":"%s","date":"%s"}\n' \
  "$RYOKU_RELEASE" "$RYOKU_NAME" "$RYOKU_CHANNEL" "$RYOKU_PKGVER" "$commit" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  > "$ARCH_DIR/release.json"

# 8. manifest.json beside release.json: every package this release is made of,
#    by lane, generated from this checkout (never hand-edited). A box's
#    `ryoku update` diff converges against it, which is what makes a package
#    added to a set reach every box on the next update, and `ryoku verify`
#    compare two boxes. The publish step's rclone sync carries it with the db.
log "Generating the control manifest"
manifest_src="$RELEASE_DIR/../ryoku/cli"
manifest_bin="$(mktemp -d)/ryoku-manifest"
( cd "$manifest_src" && go build -mod=vendor -o "$manifest_bin" ./cmd/ryoku-manifest ) \
  || die "could not build cmd/ryoku-manifest"
"$manifest_bin" -repo "$RELEASE_DIR/.." -release "$RYOKU_RELEASE" -version "$RYOKU_PKGVER" \
  -commit "$commit" -channel "$RYOKU_CHANNEL" -out "$ARCH_DIR/manifest.json" \
  || die "could not generate manifest.json"

log "Repo ready at $ARCH_DIR"
log "Serves as stable at https://repo.ryoku.dev/stable/$REPO_ARCH/, as testing under channels/testing/, or frozen under releases/<tag>/ (publish-repo.yml)"
