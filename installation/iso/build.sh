#!/usr/bin/env bash
# build the Ryoku live ISO.
#
# stages a throwaway copy of this archiso profile, bakes the prebuilt
# installer (TUI + backend) + the repo payload into its airootfs, then hands
# the staged copy to mkarchiso. the committed profile under installation/iso
# is never mutated: every generated artifact lands in the staging tree.
#
# usage:
#   ./build.sh                build the ISO (mkarchiso; root + archiso)
#   ./build.sh --stage-only   stage the profile, stop before mkarchiso
#
# env:
#   RYOKU_ISO_OUT     ISO output dir   (./out)
#   RYOKU_ISO_WORK    mkarchiso work   (./work)
#   RYOKU_ISO_STAGE   staging tree     (./staging)
#   RYOKU_ISO_REPRO   1 = pin packages to the commit-dated Arch archive (off)
#   RYOKU_ISO_WORKTREE 1 = bake the working tree (uncommitted and untracked,
#                     non-ignored files) instead of HEAD, for a local test ISO
set -euo pipefail

PROFILE_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)   # installation/iso
INSTALL_DIR=$(cd "$PROFILE_DIR/.." && pwd)                  # installation
REPO_ROOT=$(cd "$INSTALL_DIR/.." && pwd)                    # repo root
TUI_DIR=$INSTALL_DIR/tui
BACKEND_DIR=$INSTALL_DIR/backend

OUT_DIR=${RYOKU_ISO_OUT:-$PROFILE_DIR/out}
WORK_DIR=${RYOKU_ISO_WORK:-$PROFILE_DIR/work}
STAGE_DIR=${RYOKU_ISO_STAGE:-$PROFILE_DIR/staging}
PROFILE_STAGE=$STAGE_DIR/profile

# reproducibility anchor. pin every timestamp-bearing step (mkarchiso, tar,
# gzip, squashfs, and profiledef.sh's iso_label / iso_version) to the commit's
# committer date instead of wall-clock build time, so the same commit builds to
# the same bytes. an already-exported value (e.g. CI) survives only when there
# is no git history to read; otherwise the commit wins.
if _commit_epoch=$(git -C "$REPO_ROOT" log -1 --pretty=%ct 2>/dev/null) && [[ -n $_commit_epoch ]]; then
  SOURCE_DATE_EPOCH=$_commit_epoch
fi
SOURCE_DATE_EPOCH=${SOURCE_DATE_EPOCH:-$(date +%s)}
export SOURCE_DATE_EPOCH

# payload provenance, stamped into /usr/share/ryoku/.payload below and sed'd
# into the live motd. lets the target's deploy step warn when a long-lived
# ISO's baked payload has drifted from the live [ryoku] repo's package version.
PAYLOAD_COMMIT=$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)
[[ ${RYOKU_ISO_WORKTREE:-0} != 1 ]] || PAYLOAD_COMMIT+="+worktree"
PAYLOAD_DATE=$(git -C "$REPO_ROOT" log -1 --pretty=%cI 2>/dev/null || date -Iseconds)
PAYLOAD_VERSION=$(tr -d '[:space:]' <"$REPO_ROOT/VERSION" 2>/dev/null || echo unknown)
PAYLOAD_NAME=$(tr -d '[:space:]' <"$REPO_ROOT/CODENAME" 2>/dev/null || echo unknown)

STAGE_ONLY=0
[[ ${1:-} == --stage-only ]] && STAGE_ONLY=1

# variant selector: plain (stock Arch) or cachyos (the full CachyOS layer baked
# into the offline closure). baked into the airootfs as a marker the installer
# reads (ryoku-install), and passed to offline-repo.sh so the closure carries
# the cachy packages. one flag, gated reads: the two ISOs share every path but
# this switch.
VARIANT=${RYOKU_VARIANT:-plain}

log() { printf '\033[1;35m::\033[0m %s\n' "$*"; }
die() { printf 'build.sh: error: %s\n' "$*" >&2; exit 1; }
case $VARIANT in plain | cachyos) ;; *) die "RYOKU_VARIANT must be plain or cachyos, got: $VARIANT" ;; esac

# bake the repo from tracked files only (git archive at HEAD), so gitignored
# dev cruft (editor / AI-tooling configs, build dirs, ISOs) never ships.
# RYOKU_ISO_WORKTREE=1 archives a tree of the working copy instead, written
# through a throwaway index so no commit is made and the real index is untouched.
stage_repo() {
  local src=$1 dst=$2 tree=HEAD idx
  mkdir -p "$dst"
  if [[ ${RYOKU_ISO_WORKTREE:-0} == 1 ]]; then
    idx=$(mktemp)
    GIT_INDEX_FILE=$idx git -C "$src" read-tree HEAD
    GIT_INDEX_FILE=$idx git -C "$src" add -A
    tree=$(GIT_INDEX_FILE=$idx git -C "$src" write-tree)
    rm -f "$idx"
    log "RYOKU_ISO_WORKTREE=1: baking the working tree ($tree) instead of HEAD"
  fi
  git -C "$src" archive --format=tar "$tree" | tar -C "$dst" -xf -
}

# populate a local [ryoku] repo the staged pacman.conf's file:// token points at,
# so mkarchiso can install ryoku-cursors (the Bibata cursor theme the live kiosk
# needs) into the airootfs. source precedence keeps dev/test ISOs off public
# infra and fails closed:
#   1. RYOKU_ISO_LOCAL_REPO   an explicit build-repo.sh out/ tree
#   2. release/repo/out/x86_64 a repo already built in this checkout
#   3. repo.ryoku.dev          fetched; fails actionably if unreachable or if
#                              ryoku-cursors is not published yet
# publish-repo (on a main push) therefore precedes build-iso (on a tag): the
# package must already be in the repo when a tagged ISO builds.
stage_ryoku_repo() {
  local arch=x86_64
  # outside PROFILE_STAGE on purpose: it must not land in the profile mkarchiso
  # ships, and iso-stage-check diffs only the profile tree.
  local dst=$STAGE_DIR/ryoku-repo/$arch
  rm -rf "$dst"; mkdir -p "$dst"

  local src=""
  if [[ -n ${RYOKU_ISO_LOCAL_REPO:-} ]]; then
    src=$RYOKU_ISO_LOCAL_REPO/$arch
    [[ -f $src/ryoku.db ]] || src=$RYOKU_ISO_LOCAL_REPO   # accept a tree with or without the arch subdir
    [[ -f $src/ryoku.db ]] || die "RYOKU_ISO_LOCAL_REPO=$RYOKU_ISO_LOCAL_REPO has no ryoku.db (expected a release/repo/build-repo.sh out/ tree)"
  elif [[ -f $REPO_ROOT/release/repo/out/$arch/ryoku.db ]]; then
    src=$REPO_ROOT/release/repo/out/$arch
  fi

  if [[ -n $src ]]; then
    log "Staging [ryoku] from local repo $src"
    cp -a "$src"/. "$dst/"
    compgen -G "$dst/ryoku-cursors-*.pkg.tar.zst" >/dev/null \
      || die "local [ryoku] repo $src has no ryoku-cursors package; rebuild it (release/repo/build-repo.sh) or repoint RYOKU_ISO_LOCAL_REPO"
  else
    local base=${RYOKU_ISO_REPO_URL:-https://repo.ryoku.dev/stable/$arch}
    base=${base%/}
    log "Fetching [ryoku] db + ryoku-cursors from $base"
    curl -fsSL --retry 2 --max-time 30 -o "$dst/ryoku.db" "$base/ryoku.db" \
      || die "cannot reach the [ryoku] repo at $base -- publish the repo first, or set RYOKU_ISO_LOCAL_REPO to a build-repo.sh out/ tree"
    local file
    file=$(bsdtar -xOf "$dst/ryoku.db" 2>/dev/null | awk '/^%FILENAME%/{getline; print}' | grep -E '^ryoku-cursors-' | head -n1 || true)
    [[ -n $file ]] || die "ryoku-cursors is not published in $base yet -- publish the repo first, or set RYOKU_ISO_LOCAL_REPO to a build-repo.sh out/ tree"
    curl -fsSL --retry 2 --max-time 120 -o "$dst/$file" "$base/$file" || die "cannot fetch $file from $base"
    curl -fsSL --retry 2 --max-time 30 -o "$dst/$file.sig" "$base/$file.sig" 2>/dev/null || true
  fi

  # only the Server line carries the file:// token; the comment's bare
  # @RYOKU_ISO_REPO@ mention is left intact (and stays reproducible).
  sed -i "s|file://@RYOKU_ISO_REPO@|file://$dst|" "$PROFILE_STAGE/pacman.conf"
  grep -q "^Server = file://$dst\$" "$PROFILE_STAGE/pacman.conf" \
    || die "failed to wire the [ryoku] repo path into the staged pacman.conf"
  log "[ryoku] staged at $dst"
}

# 0. preflight.
[[ -f $TUI_DIR/go.mod ]]         || die "installer TUI not found at $TUI_DIR"
[[ -f $BACKEND_DIR/ryoku-install ]] || die "backend not found at $BACKEND_DIR/ryoku-install"
[[ -d $BACKEND_DIR/lib ]]        || die "backend lib not found at $BACKEND_DIR/lib"
[[ -f $REPO_ROOT/installation/installerplan/choices.tsv ]] || die "installer choice manifest not found"

# 1. fresh profile copy. profile components only, never build dirs.
log "Staging profile -> $PROFILE_STAGE"
rm -rf "$PROFILE_STAGE"
mkdir -p "$PROFILE_STAGE"
for item in profiledef.sh packages.x86_64 pacman.conf airootfs efiboot syslinux grub; do
  cp -a "$PROFILE_DIR/$item" "$PROFILE_STAGE/"
done
AIROOTFS=$PROFILE_STAGE/airootfs

# Keep the committed profile generic. Edition-specific package, boot, and
# branding changes are made only in this throwaway copy.
# shellcheck source=stage-variant.sh
source "$PROFILE_DIR/stage-variant.sh"
ryoku_stage_variant "$VARIANT" "$PROFILE_STAGE"

# reproducible package set (opt-in). the default build pulls whatever the live
# mirrors currently serve, so two builds weeks apart differ by upstream package
# churn. RYOKU_ISO_REPRO=1 repoints the STAGED pacman.conf's [core]/[extra] at
# the Arch Linux Archive snapshot dated from the commit, freezing the exact
# package versions baked into the image. reproducible here means frozen, not
# latest: turn it on only to reproduce a specific historical ISO.
if [[ ${RYOKU_ISO_REPRO:-0} == 1 ]]; then
  ala_date=$(date -u --date="@$SOURCE_DATE_EPOCH" +%Y/%m/%d)
  log "RYOKU_ISO_REPRO=1: pinning [core]/[extra] to archive.archlinux.org/$ala_date"
  sed -i "s|^Include = /etc/pacman.d/mirrorlist|Server = https://archive.archlinux.org/repos/$ala_date/\$repo/os/\$arch|" \
    "$PROFILE_STAGE/pacman.conf"
fi

# 1b. wire the [ryoku] repo so the staged pacman.conf's file:// token resolves
#     and mkarchiso can install ryoku-cursors into the live set.
stage_ryoku_repo

# 2. build the installer TUI from source. live env has no Go toolchain;
#    the ISO carries the prebuilt binary.
command -v go >/dev/null 2>&1 || die "go is required to build the TUI (pacman -S go)"
log "Building ryoku-tui from $TUI_DIR"
install -d "$AIROOTFS/usr/local/bin"
( cd "$TUI_DIR" && CGO_ENABLED=0 go build -trimpath -ldflags '-s -w -buildid=' -o "$AIROOTFS/usr/local/bin/ryoku-tui" . )

# 3. bake the backend + its lib under /usr/local/lib/ryoku/backend.
#    /usr/local/bin/ryoku-install (overlay) execs the real script; the script
#    finds its lib next to itself via realpath, so they stay together.
log "Installing backend -> /usr/local/lib/ryoku/backend"
install -d "$AIROOTFS/usr/local/lib/ryoku/backend"
install -m0755 "$BACKEND_DIR/ryoku-install" "$AIROOTFS/usr/local/lib/ryoku/backend/ryoku-install"
cp -a "$BACKEND_DIR/lib" "$AIROOTFS/usr/local/lib/ryoku/backend/lib"

# 3b. the translation catalog at the path the backend's i18n.sh and the CLI's
#     Go runtime both look for. The TUI compiles its own copy in (it must work
#     with nothing mounted), but the shell libs read it from here.
log "Installing translation catalog -> /usr/share/ryoku/i18n"
install -d "$AIROOTFS/usr/share/ryoku/i18n"
install -m0644 "$REPO_ROOT"/ryoku/i18n/catalog/*.json "$AIROOTFS/usr/share/ryoku/i18n/"
install -m0644 "$REPO_ROOT/ryoku/i18n/langs.json" "$AIROOTFS/usr/share/ryoku/i18n/langs.json"

# 4. bake the repo payload at /usr/share/ryoku (RYOKU_REPO).
log "Baking repo payload -> /usr/share/ryoku"
stage_repo "$REPO_ROOT" "$AIROOTFS/usr/share/ryoku"
install -d "$AIROOTFS/usr/share/ryoku/installation/installerplan"
install -m0644 "$REPO_ROOT/installation/installerplan/choices.tsv" \
  "$AIROOTFS/usr/share/ryoku/installation/installerplan/choices.tsv"

# provenance stamp. records the exact commit + version baked into this payload
# so the target's deploy step can flag drift from the live [ryoku] repo. keep
# the format greppable (key=value); iso-stage-check.sh strips it before diffing.
cat >"$AIROOTFS/usr/share/ryoku/.payload" <<EOF
commit=$PAYLOAD_COMMIT
date=$PAYLOAD_DATE
version=$PAYLOAD_VERSION
name=$PAYLOAD_NAME
EOF

# variant marker: the installer (ryoku-install) reads this to decide whether to
# pull the CachyOS layer, boot linux-cachyos, and wire the CachyOS repos.
printf '%s\n' "$VARIANT" >"$AIROOTFS/usr/share/ryoku/variant"

# fill the motd placeholders on the STAGED copy only (the committed motd keeps
# the @...@ tokens), so the live shell greets with the baked version + commit.
sed -i \
  -e "s|@RYOKU_NAME@|$PAYLOAD_NAME|g" \
  -e "s|@RYOKU_VERSION@|$PAYLOAD_VERSION|g" \
  -e "s|@RYOKU_COMMIT@|${PAYLOAD_COMMIT:0:12}|g" \
  "$AIROOTFS/etc/motd"

# 4b. ryoku-shell daemon (Go). same as the TUI: neither the ISO nor the
#     target has a Go toolchain, so it ships prebuilt inside the payload for
#     the deploy step.
log "Building ryoku-shell from $REPO_ROOT/ryoku/shell/ipc"
install -d "$AIROOTFS/usr/share/ryoku/ryoku/shell/ipc"
( cd "$REPO_ROOT/ryoku/shell/ipc" && CGO_ENABLED=0 go build -trimpath -ldflags '-s -w -buildid=' -o "$AIROOTFS/usr/share/ryoku/ryoku/shell/ipc/ryoku-shell" . )

# 4d. ryoku-hub backend (Go), same prebuilt model as ryoku-shell.
log "Building ryoku-hub from $REPO_ROOT/ryoku/hub/backend"
install -d "$AIROOTFS/usr/share/ryoku/ryoku/hub/backend"
( cd "$REPO_ROOT/ryoku/hub/backend" && CGO_ENABLED=0 go build -trimpath -ldflags '-s -w -buildid=' -o "$AIROOTFS/usr/share/ryoku/ryoku/hub/backend/ryoku-hub" . )

# 4c. prebuild the Ryoku QML plugins into the payload, the target has no build toolchain
command -v cmake >/dev/null 2>&1 || die "cmake is required to build the Ryoku QML plugins (pacman -S cmake ninja qt6-shadertools qt6-quick3d)"
command -v ninja >/dev/null 2>&1 || die "ninja is required to build the Ryoku QML plugins (pacman -S cmake ninja)"
log "Building the Ryoku QML plugins from $REPO_ROOT/ryoku/shell/plugin"
RYOKU_BLOBS_BUILD="$STAGE_DIR/blobs-build" \
  "$REPO_ROOT/ryoku/shell/plugin/build.sh" \
  "$AIROOTFS/usr/share/ryoku/ryoku/shell/plugin/dist"

# 4e. bake the full offline package closure into a [offline] file:// repo, so the
#     installer pacstraps the whole system (base + every hardware profile +
#     CachyOS + the desktop set) with NO network (installation/backend/lib/
#     offline.sh). the download is cached under offline-cache/ (gitignored) and
#     reused across builds; RYOKU_OFFLINE_SKIP=1 builds a networked ISO instead.
if [[ ${RYOKU_OFFLINE_SKIP:-0} != 1 ]]; then
  log "Baking offline package closure ($VARIANT) -> /usr/share/ryoku/offline/repo"
  # the same [ryoku] source the live ISO's own stanza uses (RYOKU_ISO_REPO_URL,
  # a frozen releases/<tag>/ directory for a release ISO), so the offline
  # closure and the live media agree on which release this ISO is.
  RYOKU_OFFLINE_CACHE=${RYOKU_OFFLINE_CACHE:-$PROFILE_DIR/offline-cache-$VARIANT} \
  RYOKU_VARIANT="$VARIANT" \
    "$PROFILE_DIR/offline-repo.sh" "$REPO_ROOT" "$AIROOTFS/usr/share/ryoku/offline/repo" \
      "${RYOKU_ISO_REPO_URL:-https://repo.ryoku.dev/stable/x86_64}"
else
  log "RYOKU_OFFLINE_SKIP=1: skipping the offline repo bake (networked ISO)"
fi

# 5. keep the staged launchers executable. profiledef file_permissions also
#    sets these at build time, but this keeps the staged tree consistent now.
chmod 0755 \
  "$AIROOTFS/usr/local/bin/ryoku-installer-session" \
  "$AIROOTFS/usr/local/bin/ryoku-install" \
  "$AIROOTFS/usr/local/bin/ryoku-tui" \
  "$AIROOTFS/usr/local/lib/ryoku/backend/ryoku-install"

log "Profile staged at $PROFILE_STAGE"

if [[ $STAGE_ONLY -eq 1 ]]; then
  log "--stage-only: skipping mkarchiso"
  exit 0
fi


# 6. assemble. mkarchiso needs root + the archiso package.
if ! command -v mkarchiso >/dev/null 2>&1; then
  cat >&2 <<EOF

build.sh: mkarchiso not found. Install the 'archiso' package, then build the
staged profile as root:

  sudo --preserve-env=SOURCE_DATE_EPOCH mkarchiso -v -w "$WORK_DIR" -o "$OUT_DIR" "$PROFILE_STAGE"

or, equivalently, from inside the staged profile:

  cd "$PROFILE_STAGE" && sudo --preserve-env=SOURCE_DATE_EPOCH mkarchiso -v -w work -o out .

EOF
  exit 1
fi
if [[ $VARIANT == cachyos ]]; then
  # CachyOS's own buildiso requires host trust too. A local CachyOS build
  # therefore populates the build host keyring before mkarchiso resolves it.
  log "Populating CachyOS signing trust on the build host"
  ryoku_trust_cachyos_build_host
fi

# mkarchiso reuses a populated work dir: with a completed build already there it
# skips the airootfs rebuild and re-emits the OLD image, silently shipping an ISO
# without the edits just made (a stale-image trap on every rebuild). Start clean.
# a prior `sudo mkarchiso` leaves the tree root-owned, so match that privilege.
if [[ $EUID -eq 0 ]]; then rm -rf "$WORK_DIR"; else sudo rm -rf "$WORK_DIR"; fi

log "Running mkarchiso (requires root)"
install -d "$OUT_DIR"
# default sudoers env_reset drops SOURCE_DATE_EPOCH, so a non-root local build
# would silently lose the reproducibility anchor; --preserve-env carries it in.
if [[ $EUID -eq 0 ]]; then
  mkarchiso -v -w "$WORK_DIR" -o "$OUT_DIR" "$PROFILE_STAGE"
else
  sudo --preserve-env=SOURCE_DATE_EPOCH mkarchiso -v -w "$WORK_DIR" -o "$OUT_DIR" "$PROFILE_STAGE"
fi

log "ISO written to $OUT_DIR"

# checksums next to the ISO for verification. deterministic for a fixed commit,
# since the ISO is (see README.md, "Reproducibility"). a missing *.iso here is a
# real mkarchiso failure, so let the glob stay literal and fail loudly.
log "Writing SHA256SUMS"
( cd "$OUT_DIR" && sha256sum -- *.iso >SHA256SUMS )
