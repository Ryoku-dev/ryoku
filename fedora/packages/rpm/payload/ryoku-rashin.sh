# Maintainer: Ryoku <releases@ryoku.dev>
#
# Ryoku Rashin backend (Go). The optional local agent OS daemon: it maintains a
# markdown knowledge vault, serves the embedded web dashboard on 127.0.0.1, and
# bridges the Hermes agent over ACP. Ships with the desktop but stays inert until
# the user enables it (optional means not running, not absent).
#
# Built from the in-repo source at ryoku/rashin/backend. go build reads the
# committed vendor/ tree (-mod=vendor), so the signed-repo CI builds offline.
# RPM metadata lives in ryoku-rashin.spec. Hermes itself is per-user opt-in;
# its prerequisites (uv, gcc, nodejs) and prowl-agent are Requires there so a
# setup never bootstraps a toolchain over the network.
startdir=${startdir:?stage-package.sh must set startdir}
srcdir=${srcdir:?stage-package.sh must set srcdir}
pkgdir=${pkgdir:?stage-package.sh must set pkgdir}
_repo="$startdir/../../../.."

build() {
  cd "$_repo/ryoku/rashin/backend" || exit
  CGO_ENABLED=0 go build -trimpath -mod=vendor -o "$srcdir/ryoku-rashin" .
  # Pre-index the monorepo: the installed target has no checkout, so the
  # vault's ryoku-repo.md ships as a snapshot generated from this exact tree.
  "$srcdir/ryoku-rashin" repo-index "$_repo" "$srcdir/ryoku-repo.md"
}

package() {
  install -Dm755 "$srcdir/ryoku-rashin" "$pkgdir/usr/bin/ryoku-rashin"
  # `rashin` is the terminal-lane command: the same binary under a second
  # name (busybox pattern); argv0 routes a bare argument to the terminal ask.
  ln -s ryoku-rashin "$pkgdir/usr/bin/rashin"
  install -Dm644 "$srcdir/ryoku-repo.md" "$pkgdir/usr/share/ryoku/rashin/ryoku-repo.md"
  # The `ryoku` agent skill: the source map, safety rules, the GUI map, and the
  # bar and plugin guides. `ryoku-rashin wire` symlinks this dir into every
  # agent's skills directory; the doctor's rashin reconciler re-wires it on update.
  for f in SKILL.md gui.md bar.md plugins.md; do
    install -Dm644 "$_repo/ryoku/rashin/skills/ryoku/$f" \
      "$pkgdir/usr/share/ryoku/skills/ryoku/$f"
  done
  # Systemd user unit: `ryoku-rashin enable` runs `systemctl --user enable
  # --now` on it; the daemon then starts at every login (or at boot with
  # lingering) instead of riding the Hyprland session.
  install -Dm644 "$_repo/ryoku/rashin/systemd/ryoku-rashin.service" \
    "$pkgdir/usr/lib/systemd/user/ryoku-rashin.service"
}
