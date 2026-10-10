# Maintainer: Ryoku <releases@ryoku.dev>
#
# Ryoku shell IPC daemon (Go). supervises the Quickshell desktop components,
# drives wallpaper + palette, serves the ryoku-shell control socket.
#
# Built from the in-repo source at ryoku/shell/ipc. stage-package.sh sets the
# directories below and the binary lands in $srcdir, so the source tree stays
# untouched. uv is a runtime dependency: it provisions the ryostage engine's
# Python 3.13 venv, because the system python rolls ahead of onnxruntime's wheels.
# RPM metadata lives in ryoku-shell.spec.
startdir=${startdir:?stage-package.sh must set startdir}
srcdir=${srcdir:?stage-package.sh must set srcdir}
pkgdir=${pkgdir:?stage-package.sh must set pkgdir}
_repo="$startdir/../../../.."

build() {
  cd "$_repo/ryoku/shell/ipc" || exit
  # -mod=vendor keeps the build hermetic. godbus is vendored, so the signed-repo
  # CI builds offline regardless of whatever GOFLAGS makepkg picked up.
  CGO_ENABLED=0 go build -trimpath -mod=vendor -o "$srcdir/ryoku-shell" .
  # ryoku-livewall: the C video-wallpaper daemon the shell drives. It software-
  # decodes a downscaled clip into wl_shm on a wlr background surface (no GL, so
  # ~40 MB RSS on any GPU vs mpv/mpvpaper's 300-700 MB). build.sh generates the
  # wayland protocol glue and compiles it.
  "$_repo/ryoku/shell/livewall/build.sh" "$srcdir/ryoku-livewall"
}

package() {
  install -Dm755 "$_repo/fedora/system/extras/ryoku-install-extra" "$pkgdir/usr/bin/ryoku-install-extra"
  install -Dm755 "$srcdir/ryoku-shell" "$pkgdir/usr/bin/ryoku-shell"
  install -Dm755 "$srcdir/ryoku-livewall" "$pkgdir/usr/bin/ryoku-livewall"
  # Every shell leaf script the bar, launcher, Hub, keybinds, recorder and the
  # daemon call by bare name (ryoku-app, ryoku-cmd-*, ryoku-sysinfo, the recorder
  # helpers, ...). They resolve on PATH with no compositor config tree, so a niri
  # box that ships no compositor scripts still gets every one of them. A single
  # glob, so a new script lands here with no packaging edit.
  local s
  for s in "$_repo"/ryoku/shell/scripts/ryoku-*; do
    [[ -f $s ]] || continue
    install -Dm755 "$s" "$pkgdir/usr/bin/${s##*/}"
  done
  # ryostage: the wallpaper engine's launcher. Not a ryoku-* name, so it installs
  # beside the glob.
  install -Dm755 "$_repo/ryoku/shell/scripts/ryostage" \
    "$pkgdir/usr/bin/ryostage"
  # The .sh helpers the shell drives by bare name: the Stash sidebar's cobalt
  # queue and its compress/install/download backends, the LocalSend LAN transfer,
  # the clipboard-thumbnail generator. Shell scripts, not compositor config, so
  # they ride the shell to PATH and resolve with no compositor config tree.
  for s in "$_repo"/ryoku/shell/scripts/*.sh; do
    [[ -f $s ]] || continue
    install -Dm755 "$s" "$pkgdir/usr/bin/${s##*/}"
  done
  install -d "$pkgdir/usr/share/ryoku/reload-cover"
  cp -a "$_repo/ryoku/shell/quickshell/reload-cover/." "$pkgdir/usr/share/ryoku/reload-cover/"
}
