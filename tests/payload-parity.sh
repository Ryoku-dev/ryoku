#!/usr/bin/env bash
set -euo pipefail

root=${RYOKU_PATH:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

packages=(
  ryoku
  ryoku-shell
  ryoku-rashin
  ryoku-palette-bridge
  ryoku-hub
  ryoku-desktop
  ryoku-desktop-niri
  ryoku-desktop-hyprland
  ryoku-blobs
  ryoku-caelestia
  ryogami
  ryoku-keysounds-profiles
  ryoku-keysounds-niri
)

fail() {
  printf 'payload-parity: %s\n' "$*" >&2
  exit 1
}

fake_binary() {
  local path=$1 name=${1##*/}
  mkdir -p "${path%/*}"
  printf '#!/bin/sh\n# payload fixture: %s\nexit 0\n' "$name" > "$path"
  chmod 755 "$path"
}

prepare_prebuilt() {
  local pkg=$1 dir="$work/prebuilt/$1" appdir gomod helper
  mkdir -p "$dir"
  case "$pkg" in
    ryoku)
      fake_binary "$dir/ryoku"
      fake_binary "$dir/ryoku-host"
      ;;
    ryoku-shell)
      fake_binary "$dir/ryoku-shell"
      fake_binary "$dir/ryoku-livewall"
      ;;
    ryoku-rashin)
      fake_binary "$dir/ryoku-rashin"
      fake_binary "$dir/rashin-app"
      printf '# fixture repository index\n' > "$dir/ryoku-repo.md"
      ;;
    ryoku-palette-bridge)
      fake_binary "$dir/ryoku-palette-bridge"
      ;;
    ryoku-hub)
      fake_binary "$dir/ryoku-hub"
      ;;
    ryoku-desktop)
      shopt -s nullglob
      for appdir in "$root"/ryoku/apps/*/; do
        if [[ -f ${appdir}CMakeLists.txt ]]; then
          fake_binary "$dir/$(basename "$appdir")"
        fi
        for gomod in "${appdir}"*/go.mod; do
          helper=$(awk '/^module[[:space:]]+/ { print $2; exit }' "$gomod")
          [[ -n $helper ]] && fake_binary "$dir/$helper"
        done
      done
      shopt -u nullglob
      ;;
    ryoku-desktop-niri)
      fake_binary "$dir/ryoku-wm-niri"
      ;;
    ryoku-desktop-hyprland)
      fake_binary "$dir/ryoku-wm-hyprland"
      ;;
    ryoku-blobs)
      mkdir -p "$dir/qml/Ryoku/Blobs"
      printf 'module Ryoku.Blobs\n' > "$dir/qml/Ryoku/Blobs/qmldir"
      fake_binary "$dir/qml/Ryoku/Blobs/libryoku-blobs.so"
      ;;
    ryoku-caelestia)
      mkdir -p "$dir/qml/Caelestia/lib"
      printf 'module Caelestia\n' > "$dir/qml/Caelestia/qmldir"
      fake_binary "$dir/qml/Caelestia/lib/libcaelestia-core.so"
      mkdir -p "$dir/qml/M3Shapes"
      printf 'module M3Shapes\n' > "$dir/qml/M3Shapes/qmldir"
      fake_binary "$dir/qml/M3Shapes/libm3shapes.so"
      ;;
    ryogami)
      fake_binary "$dir/ryogami"
      fake_binary "$dir/ryogami-live"
      mkdir -p "$dir/qml/Ryoku/Ryogami"
      printf 'module Ryoku.Ryogami\n' > "$dir/qml/Ryoku/Ryogami/qmldir"
      fake_binary "$dir/qml/Ryoku/Ryogami/libryogami-picker.so"
      ;;
    ryoku-keysounds-profiles)
      mkdir -p "$dir/sounds/cherry-mx-brown"
      printf 'RIFF fixture\n' > "$dir/sounds/cherry-mx-brown/down-1.wav"
      printf 'MIT fixture\n' > "$dir/sounds/LICENSE.mechvibes"
      ;;
    ryoku-keysounds-niri)
      fake_binary "$dir/ryoku-keysounds"
      ;;
  esac
}

mkdir -p "$work/bin"
cat > "$work/bin/date" <<'EOF_DATE'
#!/bin/sh
printf '%s\n' 2026-10-11T00:00:00Z
EOF_DATE
cat > "$work/bin/pkg-config" <<'EOF_PKG_CONFIG'
#!/bin/sh
printf '%s\n' 6.9
EOF_PKG_CONFIG
chmod 755 "$work/bin/date" "$work/bin/pkg-config"
export PATH="$work/bin:$PATH"
export RYOKU_PKGVER=0.1.0
export RYOKU_RELEASE=0.1.0
export RYOKU_CHANNEL=testing
export RYOKU_NAME=Ryoku
export RYOKU_COMMIT=payload-parity

for pkg in "${packages[@]}"; do
  prepare_prebuilt "$pkg"
done

stage_payload() {
  local pkg=$1 init=$2 dest=$3 build
  build="$work/direct-build/$pkg-$init"
  rm -rf "$dest" "$build"
  mkdir -p "$dest" "$build"
  RYOKU_SRC="$root" \
  RYOKU_BUILD_DIR="$build" \
  RYOKU_PREBUILT_DIR="$work/prebuilt/$pkg" \
  RYOKU_PKGVER="$RYOKU_PKGVER" \
    "$root/release/packages/$pkg/payload.sh" build
  RYOKU_SRC="$root" \
  RYOKU_BUILD_DIR="$build" \
  RYOKU_PREBUILT_DIR="$work/prebuilt/$pkg" \
  RYOKU_PKGVER="$RYOKU_PKGVER" \
    "$root/release/packages/$pkg/payload.sh" install "$dest" "$init"
}

stage_arch() {
  local pkg=$1 dest=$2 build
  build="$work/arch-build/$pkg"
  rm -rf "$dest" "$build"
  mkdir -p "$dest" "$build"
  RYOKU_PREBUILT_DIR="$work/prebuilt/$pkg" bash -c '
    set -euo pipefail
    startdir=$1
    srcdir=$2
    pkgdir=$3
    source "$startdir/PKGBUILD"
    build
    package
  ' _ "$root/release/packages/$pkg" "$build" "$dest"
}

make_source_view() {
  local view=$1 entry
  mkdir -p "$view"
  for entry in CODENAME release ryoku system void fedora; do
    ln -s "$root/$entry" "$view/$entry"
  done
}

void_source="$work/void-source"
make_source_view "$void_source"

stage_void() {
  local pkg=$1 dest=$2 template build
  template="$root/void/packages/srcpkgs/$pkg/template"
  rm -rf "$dest"
  mkdir -p "$dest"
  if [[ ! -f $template ]]; then
    [[ $pkg == ryoku-desktop-hyprland ]] || fail "missing Void template for $pkg"
    # Void does not ship Hyprland, but the shared recipe must stay init-neutral.
    build="$work/void-build/$pkg"
    mkdir -p "$build"
    RYOKU_SRC="$root" RYOKU_BUILD_DIR="$build" RYOKU_PREBUILT_DIR="$work/prebuilt/$pkg" \
      RYOKU_PKGVER="$RYOKU_PKGVER" "$root/release/packages/$pkg/payload.sh" build
    RYOKU_SRC="$root" RYOKU_BUILD_DIR="$build" RYOKU_PREBUILT_DIR="$work/prebuilt/$pkg" \
      RYOKU_PKGVER="$RYOKU_PKGVER" "$root/release/packages/$pkg/payload.sh" install "$dest" runit
    return
  fi
  RYOKU_PREBUILT_DIR="$work/prebuilt/$pkg" bash -c '
    set -euo pipefail
    template=$1
    wrksrc=$2
    DESTDIR=$3
    FILESDIR=$4
    vinstall() {
      local src=$1 mode=$2 target=$3
      install -Dm"$mode" "$src" "$DESTDIR/$target/${src##*/}"
    }
    source "$template"
    wrksrc=$2
    DESTDIR=$3
    FILESDIR=$4
    do_build
    do_install
  ' _ "$template" "$void_source" "$dest" "$root/void/packages/srcpkgs/$pkg/files"
}

fedora_root="$work/fedora-root"
mkdir -p "$fedora_root/fedora/packages/rpm" "$fedora_root/fedora/packages"
for entry in CODENAME release ryoku system void; do
  ln -s "$root/$entry" "$fedora_root/$entry"
done
ln -s "$root/fedora/system" "$fedora_root/fedora/system"
ln -s "$root/fedora/packages/translations.tsv" "$fedora_root/fedora/packages/translations.tsv"
ln -s "$root/fedora/packages/rpm/payload" "$fedora_root/fedora/packages/rpm/payload"
ln -s "$root/fedora/packages/rpm/stage-package.sh" "$fedora_root/fedora/packages/rpm/stage-package.sh"
cat > "$fedora_root/.rpm-release" <<EOF_RELEASE
RELEASE=$RYOKU_RELEASE
NAME=$RYOKU_NAME
CHANNEL=$RYOKU_CHANNEL
VERSION=$RYOKU_PKGVER
COMMIT=$RYOKU_COMMIT
DATE=2026-10-11T00:00:00Z
EOF_RELEASE

stage_fedora() {
  local pkg=$1 dest=$2
  rm -rf "$dest" "$fedora_root/.rpm-build/$pkg"
  mkdir -p "$dest"
  RYOKU_PREBUILT_DIR="$work/prebuilt/$pkg" \
  RYOKU_PKGVER="$RYOKU_PKGVER" \
    "$fedora_root/fedora/packages/rpm/stage-package.sh" "$pkg" "$dest" /usr/lib64
}

stage_fedora_spec_install() {
  local pkg=$1 stage=$2 dest=$3 spec spec_work script
  spec="$root/fedora/packages/rpm/$pkg.spec"
  spec_work="$work/spec/$pkg"
  [[ -f $spec ]] || fail "missing Fedora spec for $pkg"
  rm -rf "$spec_work" "$dest"
  mkdir -p "$spec_work/stage" "$dest"
  cp -a "$stage/." "$spec_work/stage/"
  script=$(awk '
    /^%install[[:space:]]*$/ { active=1; next }
    active && /^%/ { exit }
    active { print }
  ' "$spec")
  [[ -n $script ]] || fail "$pkg spec has no install phase"
  # The replacement must leave $buildroot for the extracted RPM script to expand.
  # shellcheck disable=SC2016
  script=${script//'%{buildroot}'/'$buildroot'}
  printf '%s\n' "$script" > "$spec_work/install.sh"
  (cd "$spec_work" && buildroot="$dest" bash install.sh)
}

raw_manifest() {
  local tree=$1 out=$2 path full mode digest
  : > "$out"
  while IFS= read -r -d '' full; do
    path=${full#"$tree"/}
    mode=$(stat -c '%a' "$full")
    if [[ -L $full ]]; then
      digest=$(printf '%s' "$(readlink "$full")" | sha256sum | cut -d' ' -f1)
    else
      digest=$(sha256sum "$full" | cut -d' ' -f1)
    fi
    printf '%s\t%s\t%s\n' "$path" "$mode" "$digest" >> "$out"
  done < <(find "$tree" \( -type f -o -type l \) -print0 | sort -z)
}

adapter_entry() {
  local edition=$1 package=$2 source_path=$3
  normalized_path=$source_path
  normalized_filter=

  # This is the complete adapter allow-list. Entries are limited to init wiring,
  # package-manager hooks and tables, boot integration, Fedora's Qt libdir and
  # provider mime files, Void's XDG mime location, and Fedora's picker command.
  case "$edition:$source_path" in
    fedora:usr/lib64/qt6/*)
      normalized_path="usr/lib/qt6/${source_path#usr/lib64/qt6/}"
      ;;
    void:etc/xdg/mimeapps.list)
      normalized_path=usr/local/share/applications/mimeapps.list
      ;;
    void:etc/elogind/logind.conf.d/10-ryoku-lid.conf)
      normalized_path=etc/systemd/logind.conf.d/10-ryoku-lid.conf
      ;;
    *:usr/lib/systemd/*|*:usr/lib/tmpfiles.d/ryoku.conf|*:usr/share/ryoku/config/systemd/*|\
    *:usr/share/ryoku/palette-bridge/packaging/systemd/*|\
    *:usr/lib/environment.d/99-ryoku-session.conf|\
    *:usr/lib/ryoku/runit/*|*:etc/sv/*|*:etc/profile.d/ryoku-environment-d.sh|\
    *:etc/sudoers.d/ryoku-service-health)
      return 1
      ;;
    *:usr/share/libalpm/*|*:usr/share/ryoku/packages/void.tsv|*:usr/share/ryoku/packages/fedora.tsv|\
    fedora:etc/dnf/libdnf5-plugins/*|fedora:usr/libexec/ryoku/ryoku-dnf-snapshot)
      return 1
      ;;
    *:etc/boot/*|*:etc/kernel.d/*|*:usr/lib/initcpio/*|*:usr/share/ryoku/boot/*|\
    *:usr/bin/ryoku-boot-apply|*:usr/bin/ryoku-windows-entry|\
    fedora:etc/grub.d/15_ryoku_snapshots|fedora:usr/bin/ryoku-grub-menu|fedora:usr/bin/ryoku-plymouth-apply|fedora:usr/share/ryoku/grub/*|\
    fedora:usr/libexec/snapper/plugins/50-ryoku-boot-menu|fedora:usr/lib/dracut/modules.d/90ryoku-snapshot/*)
      return 1
      ;;
    *:usr/share/polkit-1/rules.d/54-ryoku-bluetooth-a2dp.rules|\
    *:usr/lib/udev/rules.d/60-ryoku-tpm-nvpcr.rules)
      return 1
      ;;
    fedora:usr/share/applications/*-mimeapps.list)
      return 1
      ;;
    fedora:usr/share/ryoku/config/hypr/xdph.conf)
      [[ $package == ryoku-desktop-hyprland ]] || return 1
      normalized_filter=fedora-picker
      ;;
  esac
}

normalized_manifest() {
  local edition=$1 package=$2 tree=$3 out=$4 source_path full mode digest target
  : > "$out"
  while IFS= read -r -d '' full; do
    source_path=${full#"$tree"/}
    adapter_entry "$edition" "$package" "$source_path" || continue
    mode=$(stat -c '%a' "$full")
    if [[ $normalized_filter == fedora-picker ]]; then
      digest=$(sed \
        's/custom_picker_binary = hyprland-share-picker/custom_picker_binary = hyprland-preview-share-picker/' \
        "$full" | sha256sum | cut -d' ' -f1)
    elif [[ -L $full ]]; then
      target=$(readlink "$full")
      if [[ $edition == fedora && $target == /usr/lib64/qt6/* ]]; then
        target="/usr/lib/qt6/${target#/usr/lib64/qt6/}"
      fi
      digest=$(printf '%s' "$target" | sha256sum | cut -d' ' -f1)
    else
      digest=$(sha256sum "$full" | cut -d' ' -f1)
    fi
    printf '%s\t%s\t%s\n' "$normalized_path" "$mode" "$digest" >> "$out"
  done < <(find "$tree" \( -type f -o -type l \) -print0 | sort -z)
  sort -o "$out" "$out"
}

compare_manifests() {
  local left=$1 right=$2 message=$3
  if ! diff -u "$left" "$right" > "$work/diff"; then
    cat "$work/diff" >&2
    fail "$message"
  fi
}

wait_for_jobs() {
  local label=$1 pid status=0
  shift
  for pid in "$@"; do
    wait "$pid" || status=1
  done
  ((status == 0)) || fail "$label"
}

mkdir -p "$work/direct" "$work/staged" "$work/manifests"
for pkg in "${packages[@]}"; do
  jobs=()
  stage_payload "$pkg" systemd "$work/direct/$pkg-systemd" & jobs+=("$!")
  stage_payload "$pkg" runit "$work/direct/$pkg-runit" & jobs+=("$!")
  stage_arch "$pkg" "$work/staged/$pkg-arch" & jobs+=("$!")
  stage_void "$pkg" "$work/staged/$pkg-void" & jobs+=("$!")
  stage_fedora "$pkg" "$work/staged/$pkg-fedora" & jobs+=("$!")
  wait_for_jobs "$pkg staging failed" "${jobs[@]}"
  stage_fedora_spec_install "$pkg" "$work/staged/$pkg-fedora" "$work/staged/$pkg-fedora-spec"

  if [[ $pkg == ryoku-desktop ]]; then
    [[ -f $work/staged/$pkg-arch/usr/share/libalpm/hooks/96-ryoku-polkit-reload.hook ]] \
      || fail "ryoku-desktop Arch package is missing the polkit reload hook"
    for edition in arch void fedora fedora-spec; do
      [[ -x $work/staged/$pkg-$edition/usr/bin/ryoku-polkit-reload ]] \
        || fail "ryoku-desktop $edition package is missing the polkit reload helper"
    done
  fi

  jobs=()
  raw_manifest "$work/direct/$pkg-systemd" "$work/manifests/$pkg-direct-systemd" & jobs+=("$!")
  raw_manifest "$work/direct/$pkg-runit" "$work/manifests/$pkg-direct-runit" & jobs+=("$!")
  raw_manifest "$work/staged/$pkg-arch" "$work/manifests/$pkg-arch-raw" & jobs+=("$!")
  raw_manifest "$work/staged/$pkg-void" "$work/manifests/$pkg-void-raw" & jobs+=("$!")
  raw_manifest "$work/staged/$pkg-fedora" "$work/manifests/$pkg-fedora-raw" & jobs+=("$!")
  raw_manifest "$work/staged/$pkg-fedora-spec" "$work/manifests/$pkg-fedora-spec-raw" & jobs+=("$!")
  wait_for_jobs "$pkg manifest generation failed" "${jobs[@]}"

  compare_manifests "$work/manifests/$pkg-direct-systemd" "$work/manifests/$pkg-arch-raw" \
    "$pkg PKGBUILD installs outside its shared payload"
  compare_manifests "$work/manifests/$pkg-direct-runit" "$work/manifests/$pkg-void-raw" \
    "$pkg Void template installs outside its shared payload"
  compare_manifests "$work/manifests/$pkg-fedora-raw" "$work/manifests/$pkg-fedora-spec-raw" \
    "$pkg Fedora spec installs outside stage-package.sh"

  jobs=()
  normalized_manifest arch "$pkg" "$work/staged/$pkg-arch" "$work/manifests/$pkg-arch" & jobs+=("$!")
  normalized_manifest void "$pkg" "$work/staged/$pkg-void" "$work/manifests/$pkg-void" & jobs+=("$!")
  normalized_manifest fedora "$pkg" "$work/staged/$pkg-fedora" "$work/manifests/$pkg-fedora" & jobs+=("$!")
  wait_for_jobs "$pkg normalized manifest generation failed" "${jobs[@]}"
  compare_manifests "$work/manifests/$pkg-arch" "$work/manifests/$pkg-void" \
    "$pkg differs between Arch and Void outside the adapter allow-list"
  compare_manifests "$work/manifests/$pkg-arch" "$work/manifests/$pkg-fedora" \
    "$pkg differs between Arch and Fedora outside the adapter allow-list"
  printf 'payload-parity: %s passed\n' "$pkg"
done
