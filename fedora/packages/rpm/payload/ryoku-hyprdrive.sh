# shellcheck shell=bash
: "${startdir:?}" "${srcdir:?}" "${pkgdir:?}"
source "$startdir/hyprland-plugin.sh"
_repo="$startdir/../../../.."
# fetched from its own repository's ryoku branch (fetch/ryoku-hyprdrive.sh)
_src="$_repo/.rpm-ryoku-hyprdrive/hyprdrive"

# The plugin, agent-run beside it, the look bridge and Lithic's faces: the same
# payload as the Arch package.
build() {
  rm -rf "$srcdir/hyprdrive"
  cp -a "$_src" "$srcdir/hyprdrive"
  SCROLLOVERVIEW_BUILD_VERSION="$(cut -c1-12 "$srcdir/hyprdrive/.ryoku-source-commit" 2>/dev/null || true)" \
    make -C "$srcdir/hyprdrive" hyprdrive.so look
}

package() {
  install -Dm755 "$srcdir/hyprdrive/hyprdrive.so" "$pkgdir/usr/lib/hyprland/plugins/hyprdrive.so"
  install_plugin_abi hyprdrive
  install -Dm755 "$srcdir/hyprdrive/bin/agent-run" "$pkgdir/usr/lib/hyprland/plugins/agent-run"
  install -Dm755 "$srcdir/hyprdrive/ryoku-hyprdrive-look" "$pkgdir/usr/bin/ryoku-hyprdrive-look"
  install -Dm644 -t "$pkgdir/usr/share/ryoku/hyprdrive/fonts" "$_repo"/docs/lithic/library/go/fonts/*.ttf
}
