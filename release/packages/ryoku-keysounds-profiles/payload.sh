#!/usr/bin/env bash
set -euo pipefail

: "${RYOKU_SRC:?set RYOKU_SRC to the checkout root}"
: "${RYOKU_PKGVER:?set RYOKU_PKGVER to the package version}"
: "${RYOKU_BUILD_DIR:?set RYOKU_BUILD_DIR to the build directory}"

_repo=$RYOKU_SRC
_keysounds="$_repo/ryoku/keysounds"

# mechvibes.bundle holds the eleven packs profiles.txt names plus the MIT
# licence, as one commit cut from upstream 326252a13e7bef4f1c35d08ef0189b5af6f8ba02
# (src/audio). Upstream rewrote its history for Mechvibes 3 on 2026-10-31 and
# that commit no longer exists there, so every lane builds from the bundle. To
# refresh: copy the packs and LICENSE into a fresh repo, commit, then
# `git bundle create mechvibes.bundle main` and update the commit and the
# PKGBUILD sha256sums.
MECHVIBES_BUNDLE="$_repo/release/packages/ryoku-keysounds-profiles/mechvibes.bundle"
MECHVIBES_COMMIT=8d18b40792c7ce859a898ce0e840a0c01112888c

stage_prebuilt() {
  [[ -n ${RYOKU_PREBUILT_DIR:-} ]] || return 1
  mkdir -p "$RYOKU_BUILD_DIR"
  cp -a "$RYOKU_PREBUILT_DIR/." "$RYOKU_BUILD_DIR/"
}

# The real switch recordings, cut from the MIT-licensed Mechvibes packs named in
# profiles.txt by ryoku-keysounds-import. The recipe is the [ryoku] build's, and
# Void's and Fedora's too, so it lives here once. MECHVIBES names a checkout;
# unset, one is cloned from the vendored bundle (Arch hands in its makepkg source).
sounds() {
  local mech=${MECHVIBES:-$RYOKU_BUILD_DIR/mechvibes}
  if [[ ! -d $mech/src/audio ]]; then
    git clone -q "$MECHVIBES_BUNDLE" "$mech"
    git -C "$mech" checkout -q "$MECHVIBES_COMMIT"
  fi
  rm -rf "$RYOKU_BUILD_DIR/sounds"
  local name pack
  grep -vE '^[[:space:]]*(#|$)' "$_keysounds/profiles.txt" | while read -r name pack; do
    "$_keysounds/ryoku-keysounds-import" "$mech/src/audio/$pack" --name "$name" --to "$RYOKU_BUILD_DIR/sounds" || exit 1
  done
  cp "$mech/LICENSE" "$RYOKU_BUILD_DIR/sounds/LICENSE.mechvibes"
}

install_payload() {
  install -d "$pkgdir/usr/share/ryoku/keysounds"
  cp -a "$RYOKU_BUILD_DIR/sounds/." "$pkgdir/usr/share/ryoku/keysounds/"
  chmod -R u=rwX,go=rX "$pkgdir/usr/share/ryoku/keysounds"
  # The importer ships beside the samples so both compositors share one copy.
  install -Dm755 "$_keysounds/ryoku-keysounds-import" "$pkgdir/usr/bin/ryoku-keysounds-import"
  install -Dm644 "$RYOKU_BUILD_DIR/sounds/LICENSE.mechvibes" \
    "$pkgdir/usr/share/licenses/ryoku-keysounds-profiles/LICENSE.mechvibes"
}

case "${1:-}" in
build)
  stage_prebuilt && exit 0
  mkdir -p "$RYOKU_BUILD_DIR"
  sounds
  ;;
install)
  pkgdir=${2:?install requires a destination root}
  install_payload
  ;;
*)
  printf 'usage: %s <build|install DESTDIR>\n' "$0" >&2
  exit 2
  ;;
esac
