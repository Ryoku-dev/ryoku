#!/usr/bin/env bash
set -euo pipefail

: "${RYOKU_SRC:?set RYOKU_SRC to the checkout root}"
: "${RYOKU_PKGVER:?set RYOKU_PKGVER to the package version}"

_repo=$RYOKU_SRC
pkgver=$RYOKU_PKGVER

install_runit_payload() {
  local init_root="$_repo/void/init"
  local src name

  install -Dm755 "$init_root/lib/wait-for" "$pkgdir/usr/lib/ryoku/runit/wait-for"
  install -Dm755 "$init_root/env/xdg-dirs" "$pkgdir/usr/lib/ryoku/runit/xdg-dirs"
  install -Dm755 "$init_root/session/session-start" "$pkgdir/usr/lib/ryoku/runit/session-start"
  install -Dm644 "$init_root/session-services" "$pkgdir/usr/lib/ryoku/runit/session-services"
  install -Dm644 "$init_root/env/environment-d" "$pkgdir/etc/profile.d/ryoku-environment-d.sh"

  install -Dm755 "$_repo/system/boot/limine/50-ryoku-limine" \
    "$pkgdir/etc/kernel.d/post-install/50-ryoku-limine"
  install -Dm755 "$_repo/system/boot/limine/50-ryoku-limine" \
    "$pkgdir/etc/kernel.d/post-remove/50-ryoku-limine"

  for src in "$init_root/system"/*; do
    [[ -d $src ]] || continue
    name=${src##*/}
    install -d "$pkgdir/etc/sv/$name"
    cp -a "$src/." "$pkgdir/etc/sv/$name/"
  done

  for src in "$init_root/user"/*; do
    [[ -d $src ]] || continue
    name=${src##*/}
    install -d "$pkgdir/usr/lib/ryoku/runit/user/$name"
    cp -a "$src/." "$pkgdir/usr/lib/ryoku/runit/user/$name/"
  done
}

package() {
  local cfg="$pkgdir/usr/share/ryoku/config"

  # /etc/ryoku-release: the named state this build is. build-repo.sh exports
  # RYOKU_RELEASE (a release tag on stable, the dev version on testing, a
  # local-* marker for a hand build) and RYOKU_CHANNEL; `ryoku version`,
  # `ryoku status`, the update island and the doctor read this file, and it is
  # pacman-owned so it always names the release actually installed.
  install -Dm644 /dev/stdin "$pkgdir/etc/ryoku-release" <<EOF
RELEASE=${RYOKU_RELEASE:-local-$pkgver}
NAME=$(tr -d '[:space:]' < "$_repo/CODENAME")
CHANNEL=${RYOKU_CHANNEL:-local}
VERSION=$pkgver
COMMIT=$(git -C "$_repo" rev-parse HEAD 2>/dev/null || echo unknown)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
EOF

  # --- base configs under /usr/share/ryoku/config, mirroring ~/.config -------
  # `ryoku materialize` copies this tree verbatim into the user's ~/.config.

  # Quickshell desktop components, Hub's config nested under hub/.
  install -d "$cfg/quickshell"
  cp -a "$_repo/ryoku/shell/quickshell/." "$cfg/quickshell/"
  install -d "$cfg/quickshell/hub"
  cp -a "$_repo/ryoku/hub/quickshell/." "$cfg/quickshell/hub/"

  # First-party GUI apps: a Quickshell app (ryovm, ryostore) ships its
  # quickshell/ tree as qs -c <name>; a compiled app (rashin-app) has a
  # CMakeLists.txt and builds to a /usr/bin/<name> binary. Either way
  # its bin/ helpers, .desktop and icon are installed. Dropping in an app dir
  # ships it; see ryoku/apps/README.md for the contract a new one has to meet.
  local appdir appname icon gomod helperdir helper b d has_qs has_cmake
  for appdir in "$_repo"/ryoku/apps/*/; do
    has_qs=0; [ -d "${appdir}quickshell" ] && has_qs=1
    has_cmake=0; [ -f "${appdir}CMakeLists.txt" ] && has_cmake=1
    [ "$has_qs" = 1 ] || [ "$has_cmake" = 1 ] || continue
    appname="$(basename "$appdir")"
    if [ "$has_qs" = 1 ]; then
      install -d "$cfg/quickshell/$appname"
      cp -a "${appdir}quickshell/." "$cfg/quickshell/$appname/"
    fi
    if [ "$has_cmake" = 1 ]; then
      # compiled Qt app: build to a binary named for the app dir.
      cmake -S "$appdir" -B "${appdir}build" -G Ninja -DCMAKE_BUILD_TYPE=Release
      cmake --build "${appdir}build"
      install -Dm755 "${appdir}build/$appname" "$pkgdir/usr/bin/$appname"
    fi
    if [ -d "${appdir}bin" ]; then
      for b in "${appdir}bin/"*; do
        [ -f "$b" ] && install -Dm755 "$b" "$pkgdir/usr/bin/$(basename "$b")"
      done
    fi
    # Go helper(s): a subdir with go.mod builds to a bin named for the module.
    for gomod in "${appdir}"*/go.mod; do
      [ -f "$gomod" ] || continue
      helperdir="$(dirname "$gomod")"
      helper="$(sed -n -E 's/^module[[:space:]]+//p' "$gomod" | head -1)"
      [ -n "$helper" ] || continue
      ( cd "$helperdir" && go build -o "$helper" . )
      install -Dm755 "$helperdir/$helper" "$pkgdir/usr/bin/$helper"
    done
    for d in "${appdir}"*.desktop; do
      [ -f "$d" ] && install -Dm644 "$d" "$pkgdir/usr/share/applications/$(basename "$d")"
    done
    icon="${appdir}quickshell/logo.svg"
    [ -f "$icon" ] || icon="${appdir}logo.svg"
    [ -f "$icon" ] || icon="$_repo/ryoku/assets/brand/logo-mark.svg"
    install -Dm644 "$icon" "$pkgdir/usr/share/icons/hicolor/scalable/apps/$appname.svg"
  done

  # Ryoku.PluginKit: the signature QML kit a shell plugin imports for its
  # content. pure QML, lives on the system import path next to Ryoku.Blobs so
  # plugins (loaded from outside the shell tree) can import it.
  install -d "$pkgdir/usr/lib/qt6/qml/Ryoku/PluginKit"
  cp -a "$_repo/ryoku/shell/quickshell/plugins/kit/." "$pkgdir/usr/lib/qt6/qml/Ryoku/PluginKit/"
  rm -f "$pkgdir/usr/lib/qt6/qml/Ryoku/PluginKit/install.sh"

  # Ryoku.Ui: the design system the shell, the Hub and the apps all import.
  # In /usr/lib/qt6/qml so Qt resolves it without an import path, whichever
  # way a surface was launched.
  install -d "$pkgdir/usr/lib/qt6/qml/Ryoku/Ui"
  cp -a "$_repo/ryoku/ui/." "$pkgdir/usr/lib/qt6/qml/Ryoku/Ui/"
  # install.sh is an authoring-time helper, not runtime QML, so it stays off
  # the Qt import path. The catalog it used to carry now lives once at
  # /usr/share/ryoku/i18n, which every runtime reads.
  rm -f "$pkgdir/usr/lib/qt6/qml/Ryoku/Ui/install.sh"
  rm -rf "$pkgdir/usr/lib/qt6/qml/Ryoku/Ui/__pycache__"
  chmod -R u=rwX,go=rX "$pkgdir/usr/lib/qt6/qml/Ryoku/PluginKit"

  # Ryoku.FrameBars: the shared frame-bar schema + menu/bar catalogs every
  # config root imports (shell/services/Config.qml, the launcher, widgets, and
  # plugin surfaces via PluginKit). Quickshell sandboxes a relative import to the
  # running config root, so this has to be an installed module, not a copy inside
  # one root. Omitting it loads no shell at all: qs fails the import and paints a
  # bare Hyprland desktop. Pure QML + JS like Ryoku.Ui/PluginKit; strip the
  # authoring-time install.sh and the JS unit test.
  install -d "$pkgdir/usr/lib/qt6/qml/Ryoku/FrameBars"
  cp -a "$_repo/ryoku/shell/framebars/." "$pkgdir/usr/lib/qt6/qml/Ryoku/FrameBars/"
  rm -f "$pkgdir/usr/lib/qt6/qml/Ryoku/FrameBars/install.sh" "$pkgdir"/usr/lib/qt6/qml/Ryoku/FrameBars/*.test.mjs

  # matugen templates (colors.json + kitty + Hyprland from a palette).
  install -d "$cfg/matugen"
  cp -a "$_repo/ryoku/shell/matugen/." "$cfg/matugen/"

  # browser extension (Ryoku Theme): source plus two assembled unpacked dirs the
  # browser loads from. Native-messaging host is `ryoku-shell browser-host`; the
  # per-user host manifests are laid by `ryoku doctor` (reconcileBrowserTheme).
  install -d "$pkgdir/usr/share/ryoku/browser"
  cp -a "$_repo/ryoku/browser/." "$pkgdir/usr/share/ryoku/browser/"
  rm -rf "$pkgdir/usr/share/ryoku/browser/dist"
  for eng in chromium firefox; do
    install -d "$pkgdir/usr/share/ryoku/browser/dist/$eng/src"
    cp "$_repo"/ryoku/browser/src/* "$pkgdir/usr/share/ryoku/browser/dist/$eng/src/"
    install -Dm644 "$_repo/ryoku/browser/manifest.$eng.json" \
      "$pkgdir/usr/share/ryoku/browser/dist/$eng/manifest.json"
  done

  # per-app configs
  install -Dm644 "$_repo/ryoku/apps/fish/config.fish"       "$cfg/fish/config.fish"
  install -Dm644 "$_repo/ryoku/apps/fish/conf.d/rashin.fish" "$cfg/fish/conf.d/rashin.fish"
  install -Dm644 "$_repo/ryoku/shell/qt6ct/qt6ct.conf"     "$cfg/qt6ct/qt6ct.conf"
  install -Dm644 "$_repo/ryoku/apps/terminal-shell/env.sh"  "$cfg/ryoku-terminal/env.sh"
  install -Dm644 "$_repo/ryoku/apps/bash/ryoku.bash"        "$cfg/bash/ryoku.bash"
  install -Dm644 "$_repo/ryoku/apps/bash/rashin.bash"       "$cfg/bash/rashin.bash"
  install -Dm644 "$_repo/ryoku/apps/zsh/ryoku.zsh"          "$cfg/zsh/ryoku.zsh"
  install -Dm644 "$_repo/ryoku/apps/zsh/rashin.zsh"         "$cfg/zsh/rashin.zsh"
  # GTK toolkit baseline for the xsettings-less Wayland session (see the header
  # of each file); materialized and pruned by `ryoku materialize` like qt6ct.conf.
  install -Dm644 "$_repo/ryoku/shell/gtk-3.0/settings.ini" "$cfg/gtk-3.0/settings.ini"
  install -Dm644 "$_repo/ryoku/shell/gtk-4.0/settings.ini" "$cfg/gtk-4.0/settings.ini"
  install -Dm644 "$_repo/ryoku/apps/btop/btop.conf"       "$cfg/btop/btop.conf"
  install -Dm644 "$_repo/ryoku/apps/starship/starship.toml" "$cfg/starship.toml"
  install -d "$cfg/starship/layouts"
  cp -a "$_repo/ryoku/apps/starship/layouts/." "$cfg/starship/layouts/"
  install -Dm644 "$_repo/ryoku/apps/starship/ryoku-colors.toml" \
    "$cfg/starship/ryoku-colors.toml"
  # session environment: Hyprland sets its own from env.lua, which never reaches
  # niri; the user manager reads this at login, so both compositors get it.
  install -Dm644 "$_repo/ryoku/shell/environment.d/ryoku-session.conf" \
    "$cfg/environment.d/ryoku-session.conf"
  install -Dm644 "$_repo/ryoku/apps/fastfetch/config.jsonc" "$cfg/fastfetch/config.jsonc"
  install -Dm644 "$_repo/ryoku/apps/fastfetch/ryoku-colors.json" \
    "$cfg/fastfetch/ryoku-colors.json"
  # the fastfetch logo the config above draws. ships beside config.jsonc so
  # `ryoku materialize` lays both together on every update; seeding it only as a
  # brand asset (installer-time) left updated machines pointing at a file they
  # never received, and fastfetch silently fell back to the Arch logo.
  install -Dm644 "$_repo/ryoku/assets/brand/fastfetch-emblem.png" \
    "$cfg/fastfetch/fastfetch-emblem.png"
  # wireplumber: Bluetooth codec and profile policy, materialized like every
  # other per-user app config so package installs and dev deploys converge.
  install -d "$cfg/wireplumber"
  cp -a "$_repo/ryoku/apps/wireplumber/." "$cfg/wireplumber/"
  install -Dm644 "$_repo/ryoku/apps/yazi/yazi.toml"         "$cfg/yazi/yazi.toml"
  install -d "$cfg/kitty"
  cp -a "$_repo/ryoku/apps/kitty/." "$cfg/kitty/"
  # ghostty: config is the user's (materialize seeds it once, never clobbers);
  # matugen owns ryoku-colors, which the config includes for the palette.
  install -d "$cfg/ghostty"
  cp -a "$_repo/ryoku/apps/ghostty/." "$cfg/ghostty/"

  # neovim (LazyVim seed): the config only, not the repo docs.
  install -Dm644 "$_repo/ryoku/apps/nvim/init.lua"       "$cfg/nvim/init.lua"
  install -Dm644 "$_repo/ryoku/apps/nvim/.ryoku-lazyvim" "$cfg/nvim/.ryoku-lazyvim"
  cp -a "$_repo/ryoku/apps/nvim/lua" "$cfg/nvim/"

  # pip: PEP 668 --user installs (materialized to ~/.config/pip/pip.conf).
  install -Dm644 "$_repo/ryoku/apps/pip/pip.conf" "$cfg/pip/pip.conf"
  # neovim as the default text/code editor: the handler ships system-wide and
  # the mimeapps map is materialized, so the mapping reaches installs on update.
  install -Dm644 "$_repo/ryoku/apps/nvim/ryoku-nvim.desktop" \
    "$pkgdir/usr/share/applications/ryoku-nvim.desktop"
  # Compress video / Install app: launcher-searchable tools that open a
  # multi-file picker and run the stash compress/install helpers on the pick.
  install -Dm644 "$_repo/ryoku/apps/tools/compress-video.desktop" \
    "$pkgdir/usr/share/applications/compress-video.desktop"
  install -Dm644 "$_repo/ryoku/apps/tools/install-app.desktop" \
    "$pkgdir/usr/share/applications/install-app.desktop"
  # Ryoku Hub: a launcher-searchable entry for the settings surface, so it shows
  # in the app launcher alongside its Super+comma keybind.
  install -Dm644 "$_repo/ryoku/hub/ryoku-hub.desktop" \
    "$pkgdir/usr/share/applications/ryoku-hub.desktop"
  install -Dm644 "$_repo/ryoku/assets/brand/logo.svg" \
    "$pkgdir/usr/share/icons/hicolor/scalable/apps/ryoku-hub.svg"
  # Default apps: the site layer, NOT ~/.config/mimeapps.list. That file is
  # where "Set as default" (GNOME, Firefox, xdg-mime) writes, and materializing
  # over it threw a user's choices away on every update. /usr/local/share is
  # first in the XDG mimeapps chain below the user's file, so these defaults
  # apply until the user picks something else, and their pick wins for good.
  # It is also the one applications layer no distro package owns: shipping to
  # /usr/share/applications/mimeapps.list collided with omarchy-settings on
  # Omarchy installs and aborted the whole transaction (#295).
  # Void packages may not own /usr/local, so there the defaults sit one layer
  # higher in the same chain, /etc/xdg, still below the user's own file.
  if [[ $RYOKU_INIT_MODE == systemd ]]; then
    install -Dm644 "$_repo/ryoku/apps/mimeapps.list" \
      "$pkgdir/usr/local/share/applications/mimeapps.list"
  else
    install -Dm644 "$_repo/ryoku/apps/mimeapps.list" "$pkgdir/etc/xdg/mimeapps.list"
  fi
  # chromium reads ~/.config/chromium-flags.conf and Google Chrome reads
  # ~/.config/chrome-flags.conf; lay the one source to both so each pins the GNOME
  # keyring password store and native Wayland (screen share works, and Chrome does
  # not sit on Xwayland where it renders pages blank on NVIDIA).
  install -Dm644 "$_repo/ryoku/apps/chromium-flags.conf" "$cfg/chromium-flags.conf"
  install -Dm644 "$_repo/ryoku/apps/chromium-flags.conf" "$cfg/chrome-flags.conf"

  # the qylock lockscreen bundle + its installer, so doctor can install or
  # repair the in-session lock on a box the ISO step missed (or that predates
  # it): a missing locker is a dead lock button AND suspend without a lock.
  mkdir -p "$pkgdir/usr/share/ryoku/lockscreen"
  cp -a "$_repo/ryoku/lockscreen/qylock" "$pkgdir/usr/share/ryoku/lockscreen/qylock"
  install -Dm755 "$_repo/ryoku/lockscreen/install-qylock" \
    "$pkgdir/usr/share/ryoku/lockscreen/install-qylock"
  install -Dm755 "$_repo/ryoku/lockscreen/ryoku-qylock-activate" \
    "$pkgdir/usr/bin/ryoku-qylock-activate"
  install -Dm755 "$_repo/ryoku/lockscreen/ryoku-qylock-lock" \
    "$pkgdir/usr/bin/ryoku-qylock-lock"
  install -Dm755 \
    "$_repo/ryoku/lockscreen/qylock/quickshell-lockscreen/ryoku-qylock-unlock-prepare" \
    "$pkgdir/usr/bin/ryoku-qylock-unlock-prepare"
  # the Wayland greeter compositor wrapper: weston --shell=kiosk at each
  # output's top mode, so the login screen matches a high-refresh session.
  install -Dm755 "$_repo/ryoku/lockscreen/sddm/ryoku-greeter" \
    "$pkgdir/usr/share/ryoku/lockscreen/ryoku-greeter"
  # the Wayland session wrapper: waits for the greeter (weston) to release the
  # GPU before the compositor probes KMS, so a hybrid-GPU laptop does not miss
  # its iGPU and fall back to a black headless dGPU (issue #174). Wired as SDDM's
  # [Wayland] SessionCommand by sddm/setup and doctor.
  install -Dm755 "$_repo/ryoku/lockscreen/sddm/ryoku-wayland-session" \
    "$pkgdir/usr/share/ryoku/lockscreen/ryoku-wayland-session"

  if [[ $RYOKU_INIT_MODE == systemd ]]; then
    # systemd user sources are materialized for the user manager on Arch.
    install -dm755 "$cfg/systemd/user"
    cp -a "$_repo/ryoku/shell/systemd/user/." "$cfg/systemd/user/"
  fi

  # AI-usage collectors + their timer: refresh the ~/.cache/{claude,codex,
  # opencode}-usage.json caches the qsbar AI pill reads. The collectors are
  # python3 stdlib-only (python is already a hard depend, for wavecap/pactl) and
  # each no-ops when its tool/creds/db are absent. Units live in the system user
  # generator dir so `systemctl --global enable` (the .install) can find them;
  # the ExecStart lines already point at /usr/bin, the packaged collector path.
  install -Dm755 "$_repo/ryoku/shell/bin/claude-usage"   "$pkgdir/usr/bin/claude-usage"
  install -Dm755 "$_repo/ryoku/shell/bin/codex-usage"    "$pkgdir/usr/bin/codex-usage"
  install -Dm755 "$_repo/ryoku/shell/bin/opencode-usage" "$pkgdir/usr/bin/opencode-usage"
  if [[ $RYOKU_INIT_MODE == systemd ]]; then
    install -Dm644 "$_repo/ryoku/shell/systemd/user/ryoku-ai-usage.service" \
      "$pkgdir/usr/lib/systemd/user/ryoku-ai-usage.service"
    install -Dm644 "$_repo/ryoku/shell/systemd/user/ryoku-ai-usage.timer" \
      "$pkgdir/usr/lib/systemd/user/ryoku-ai-usage.timer"
    # the config bootstrap: a package install lays nothing into ~/.config, so the
    # first login after it boots a bare compositor unless this runs first.
    install -Dm644 "$_repo/ryoku/shell/systemd/user/ryoku-bootstrap.service" \
      "$pkgdir/usr/lib/systemd/user/ryoku-bootstrap.service"

    # the same file for every user, so a session that never materialized still has it
    install -Dm644 "$_repo/ryoku/shell/environment.d/ryoku-session.conf" \
      "$pkgdir/usr/lib/environment.d/99-ryoku-session.conf"

    # Resolve localized XDG directories into the systemd user environment.
    install -Dm755 "$_repo/ryoku/shell/systemd/user-environment-generators/60-ryoku-xdg-dirs" \
      "$pkgdir/usr/lib/systemd/user-environment-generators/60-ryoku-xdg-dirs"
  fi

  # the decor art the Decor/Placard components render. shipped here so `ryoku
  # doctor` can seed it into ~/Pictures/ryodecors on a user box (beside
  # Wallpapers and livewalls) where a user can see and swap it; the installer
  # seeds a fresh install straight from the repo. the chmod below covers it.
  install -d "$pkgdir/usr/share/ryoku/ryodecors"
  cp -a "$_repo/ryoku/assets/ryodecors/." "$pkgdir/usr/share/ryoku/ryodecors/"

  # packaged rices are immutable source looks; the Hub copies one into the
  # user's rice library on first apply so later edits stay user-owned.
  install -d "$pkgdir/usr/share/ryoku/rices"
  cp -a "$_repo/ryoku/assets/rices/." "$pkgdir/usr/share/ryoku/rices/"

  # cp -a kept source mode bits; normalize so hypr/scripts stays executable
  # and the rest is world-readable data.
  chmod -R u=rwX,go=rX "$pkgdir/usr/share/ryoku"

  # --- helper scripts on PATH (/usr/bin) ------------------------------------
  # the hardware + container helpers the shell and autostart call by bare name.
  local s
  for s in "$_repo"/system/hardware/*/ryoku-*; do
    [[ -f $s && -x $s ]] || continue
    install -Dm755 "$s" "$pkgdir/usr/bin/${s##*/}"
  done
  for s in "$_repo"/system/extras/ryoku-*; do
    install -Dm755 "$s" "$pkgdir/usr/bin/${s##*/}"
  done
  # container runtime helper (ryoku-docker): the privileged door the stash
  # Cobalt setup wizard drives. Its own directory because it is not hardware.
  for s in "$_repo"/system/containers/ryoku-*; do
    install -Dm755 "$s" "$pkgdir/usr/bin/${s##*/}"
  done
  # the extras actuator (renamed from ryoku-extras-install); the ryoku-* glob
  # above no longer matches it, so install it by name.
  install -Dm755 "$_repo/system/extras/ryostore-install" "$pkgdir/usr/bin/ryostore-install"
  install -Dm755 "$_repo/ryoku/apps/fastfetch/ryoku-fastfetch" \
    "$pkgdir/usr/bin/ryoku-fastfetch"

  # plugin placement helper: desktop drag + Ryoku Settings call it to persist
  # a plugin's host/position into ~/.config/ryoku/plugins.json.
  install -Dm755 "$_repo/ryoku/shell/quickshell/plugins/ryoku-plugins-place" \
    "$pkgdir/usr/bin/ryoku-plugins-place"

  # The translation catalog, at the one path every runtime reads: the QML
  # singleton (ryoku/ui/Singletons/I18n.qml), the Go runtime the ryoku CLI and
  # the installers link (ryoku/i18n/i18n.go) and the installer's shell
  # (installation/backend/lib/i18n.sh). langs.json rides along because it is
  # the list the Hub's language picker draws from.
  install -d "$pkgdir/usr/share/ryoku/i18n"
  install -m0644 "$_repo"/ryoku/i18n/catalog/*.json "$pkgdir/usr/share/ryoku/i18n/"
  install -m0644 "$_repo/ryoku/i18n/langs.json" "$pkgdir/usr/share/ryoku/i18n/langs.json"

  # AI UI translation (Ryoku Settings > Global > Language > Generate with AI):
  # the Hub button runs `ryoku-i18n llm <lang>` and Hyprland autostart runs
  # `ryoku-i18n ensure`, which seeds ~/.config/ryoku/i18n-llm.json for the
  # user's API key and machine-translates the UI to their language.
  install -Dm755 "$_repo/ryoku/i18n/tools/sync.py" "$pkgdir/usr/bin/ryoku-i18n"

  # Nautilus stash actions: a nautilus-python extension that adds the deck's
  # install/compress/LocalSend actions to the file-manager right-click menu.
  # Loaded for every user from the system extensions dir, so it needs no
  # per-user materialize.
  install -Dm644 "$_repo/ryoku/apps/nautilus/ryoku-stash-menu.py" \
    "$pkgdir/usr/share/nautilus-python/extensions/ryoku-stash-menu.py"

  # GPU primary-renderer udev rule: boot-stable /dev/dri/ryoku-gpu-* symlinks
  # that ryoku-gpu pins via AQ_DRM_DEVICES. vendor location = active on boot,
  # so `ryoku-gpu install-udev` is never needed on an installed system.
  install -Dm644 "$_repo/system/hardware/gpu/90-ryoku-gpu.rules" \
    "$pkgdir/usr/lib/udev/rules.d/90-ryoku-gpu.rules"

  # Panel backlight writable by the video group without logind: ASUS AMD+NVIDIA
  # panels expose a root:root sysfs backlight with no seat ACL, so this rule
  # chgrp/chmods it for the video group ryoku-cmd-brightness runs as. The
  # ryoku-hw-backlight + ryoku-hw-backlight-fix helpers ride the hardware glob.
  install -Dm644 "$_repo/system/hardware/display/90-ryoku-backlight.rules" \
    "$pkgdir/usr/lib/udev/rules.d/90-ryoku-backlight.rules"

  # external-monitor brightness over DDC/CI (pill DISPLAY faders + the
  # XF86MonBrightness keys via ryoku-cmd-brightness): load i2c-dev so ddcutil
  # can open /dev/i2c-*, and grant the active-session user access (uaccess).
  install -Dm644 "$_repo/system/hardware/ddc/ryoku-i2c.conf" \
    "$pkgdir/etc/modules-load.d/ryoku-i2c.conf"
  install -Dm644 "$_repo/system/hardware/ddc/60-ryoku-i2c.rules" \
    "$pkgdir/usr/lib/udev/rules.d/60-ryoku-i2c.rules"

  if [[ $RYOKU_INIT_MODE == systemd ]]; then
    # No usable NvPCRs without a PCR-signed UKI, so keep systemd from starting
    # TPM units that cannot work.
    install -Dm644 "$_repo/system/hardware/tpm/60-ryoku-tpm-nvpcr.rules" \
      "$pkgdir/usr/lib/udev/rules.d/60-ryoku-tpm-nvpcr.rules"
  fi

  # Maono USB mics: their control features sit on a vendor HID page the kernel
  # leaves root-only, so uaccess is what lets a control app reach them at all.
  install -Dm644 "$_repo/system/hardware/audio/70-ryoku-maono.rules" \
    "$pkgdir/usr/lib/udev/rules.d/70-ryoku-maono.rules"

  # HDA codec power management off: the kernel default (power_save=10) cycles the
  # codec + headphone amp with playback, and on many codecs the amp's noise floor
  # is a faint whine audible only while it is powered -- the "mosquito"/tinnitus
  # hiss on headphones. Keeping it powered removes the whine (see the conf header).
  install -Dm644 "$_repo/system/hardware/audio/99-ryoku-audio-powersave.conf" \
    "$pkgdir/usr/lib/modprobe.d/99-ryoku-audio-powersave.conf"

  # Keyboard keycast: the shell's keypress overlay (shown while recording) reads
  # keyboard evdev via the daemon. Grant the active-seat user a uaccess ACL on
  # keyboards so it works without the broad input group (see the rule header).
  install -Dm644 "$_repo/system/hardware/input/72-ryoku-keyboard-uaccess.rules" \
    "$pkgdir/usr/lib/udev/rules.d/72-ryoku-keyboard-uaccess.rules"
  # QMK/VIA keyboard lighting: grant the seat user a uaccess ACL on a VIA board's
  # raw HID node so qmk_hid drives its RGB matrix without root (see rule header).
  install -Dm644 "$_repo/system/hardware/input/62-ryoku-qmk-hid.rules" \
    "$pkgdir/usr/lib/udev/rules.d/62-ryoku-qmk-hid.rules"

  # Bluetooth tuning: bluez owns /etc/bluetooth/main.conf and BlueZ has no
  # drop-in dir, so Ryoku cannot ship a copy here (pacman file conflict). The
  # ryoku-bluetooth-tune helper (installed by the hardware glob above) sets the
  # keys in place; the .install runs it on install + upgrade.

  # Game controllers: load uinput at boot so Steam Input and the userspace pad
  # drivers (xpadneo-dkms, game-devices-udev) can create their virtual gamepads.
  # The user must also be in the `input` group (installer).
  install -Dm644 "$_repo/system/hardware/input/99-ryoku-uinput.conf" \
    "$pkgdir/etc/modules-load.d/99-ryoku-uinput.conf"

  # Xbox pads pair over Bluetooth instead of refusing to: their L2CAP handshake
  # does not survive the kernel's Enhanced Retransmission Mode, which reads to a
  # user as a dead controller. modprobe.d rather than a kernel cmdline entry, so
  # it applies on first module load and survives a kernel change.
  install -Dm644 "$_repo/system/hardware/input/99-ryoku-controller.conf" \
    "$pkgdir/usr/lib/modprobe.d/99-ryoku-controller.conf"

  # Bluetooth audio drops seconds after connecting when btusb autosuspends the
  # controller in an audio stream's idle gaps. Disable it in modprobe.d so it
  # applies on first btusb load; separate module from the ERTM drop-in above.
  install -Dm644 "$_repo/system/hardware/bluetooth/99-ryoku-bt-autosuspend.conf" \
    "$pkgdir/usr/lib/modprobe.d/99-ryoku-bt-autosuspend.conf"

  if [[ $RYOKU_INIT_MODE == systemd ]]; then
    # The systemd reset service needs a matching manage-units policy grant.
    install -Dm644 "$_repo/system/hardware/bluetooth/ryoku-bluetooth-reset.service" \
      "$pkgdir/usr/lib/systemd/user/ryoku-bluetooth-reset.service"
    install -Dm644 "$_repo/system/hardware/bluetooth/54-ryoku-bluetooth-a2dp.rules" \
      "$pkgdir/usr/share/polkit-1/rules.d/54-ryoku-bluetooth-a2dp.rules"

    install -Dm644 "$_repo/ryoku/shell/systemd/user/ryoku-bt-agent.service" \
      "$pkgdir/usr/lib/systemd/user/ryoku-bt-agent.service"
  fi

  # Wi-Fi regulatory domain: the same one-click grant for ryoku-wifi-regdom
  # (installed by the hardware glob above), so the country can be pinned without a
  # password prompt; world domain 00 disables most 5 GHz channels.
  install -Dm644 "$_repo/system/hardware/network/48-ryoku-wifi-regdom.rules" \
    "$pkgdir/usr/share/polkit-1/rules.d/48-ryoku-wifi-regdom.rules"

  # Game Mode wifi power-save: a polkit rule authorizes the privileged helper
  # (ryoku-wifi-powersave, installed to /usr/bin by the hardware glob above)
  # without a password, so the deck toggle stays one click.
  install -Dm644 "$_repo/system/hardware/network/49-ryoku-wifi-powersave.rules" \
    "$pkgdir/usr/share/polkit-1/rules.d/49-ryoku-wifi-powersave.rules"

  # Full network isolation restores its drop table before NetworkManager starts
  # when the user left the switch armed.
  install -Dm644 "$_repo/system/hardware/network/55-ryoku-network-kill.rules" \
    "$pkgdir/usr/share/polkit-1/rules.d/55-ryoku-network-kill.rules"
  if [[ $RYOKU_INIT_MODE == systemd ]]; then
    install -Dm644 "$_repo/system/hardware/network/ryoku-network-kill-guard.service" \
      "$pkgdir/usr/lib/systemd/system/ryoku-network-kill-guard.service"
    install -Dm644 "$_repo/system/hardware/network/ryoku-network-kill-disconnect.service" \
      "$pkgdir/usr/lib/systemd/system/ryoku-network-kill-disconnect.service"
  fi

  # DNS provider switch: the same one-click grant for ryoku-dns (installed above),
  # so the network panel's DNS buttons apply without a password prompt.
  install -Dm644 "$_repo/system/hardware/network/50-ryoku-dns.rules" \
    "$pkgdir/usr/share/polkit-1/rules.d/50-ryoku-dns.rules"

  # Wi-Fi backend switch: the same one-click grant for ryoku-wifi-backend
  # (installed by the hardware glob above), so Settings -> Connections can flip
  # iwd <-> wpa_supplicant for WPA3 / Android-hotspot networks without a prompt.
  install -Dm644 "$_repo/system/hardware/network/51-ryoku-wifi-backend.rules" \
    "$pkgdir/usr/share/polkit-1/rules.d/51-ryoku-wifi-backend.rules"

  # Time zone: a polkit rule so the world-map picker in Settings > Global can
  # apply org.freedesktop.timedate1.set-timezone for the active user without a
  # password prompt (same one-click grant pattern as the network rules above).
  install -Dm644 "$_repo/system/policy/52-ryoku-timedate.rules" \
    "$pkgdir/usr/share/polkit-1/rules.d/52-ryoku-timedate.rules"

  # Container runtime: the same one-click grant for ryoku-docker (installed to
  # /usr/bin by the containers glob above), so the stash Cobalt setup wizard can
  # start docker.service and the cobalt container without a password prompt per
  # step. The helper exposes no docker passthrough, so the grant cannot be turned
  # into arbitrary root; see the rule's own comment.
  install -Dm644 "$_repo/system/containers/46-ryoku-docker.rules" \
    "$pkgdir/usr/share/polkit-1/rules.d/46-ryoku-docker.rules"

  # The sessionless lid policy belongs to logind. XBPS actions run the guarded
  # cutover that the ALPM hooks provide on Arch.
  if [[ $RYOKU_INIT_MODE == systemd ]]; then
    install -Dm644 "$_repo/system/hardware/power/logind-ryoku-lid.conf" \
      "$pkgdir/etc/systemd/logind.conf.d/10-ryoku-lid.conf"
    install -Dm644 "$_repo/system/hardware/power/94-ryoku-power-cutover-prepare.hook" \
      "$pkgdir/usr/share/libalpm/hooks/94-ryoku-power-cutover-prepare.hook"
    install -Dm644 "$_repo/system/hardware/power/95-ryoku-power-cutover.hook" \
      "$pkgdir/usr/share/libalpm/hooks/95-ryoku-power-cutover.hook"
  else
    install -Dm644 "$_repo/system/hardware/power/logind-ryoku-lid.conf" \
      "$pkgdir/etc/elogind/logind.conf.d/10-ryoku-lid.conf"
  fi

  # Battery/ASPM power knobs: a polkit rule authorizes ryoku-power (installed to
  # /usr/bin by the hardware glob above) for the active wheel user without a
  # password, so the charge ceiling that login autostart re-applies via `apply`
  # never throws a prompt; its argument is a range-checked value, so the grant
  # stays safe.
  install -Dm644 "$_repo/system/hardware/power/47-ryoku-power.rules" \
    "$pkgdir/usr/share/polkit-1/rules.d/47-ryoku-power.rules"

  # Hardware GPU MUX: the same one-click grant for ryoku-gpu-mux (installed by
  # the hardware glob above), so the Machine page can flip the display-routing
  # knob without a terminal. The helper accepts only hybrid|discrete and writes
  # a constant derived from that pair, so the grant stays safe; the change only
  # takes effect after a reboot the user performs.
  install -Dm644 "$_repo/system/hardware/gpu/45-ryoku-gpu-mux.rules" \
    "$pkgdir/usr/share/polkit-1/rules.d/45-ryoku-gpu-mux.rules"

  # Game Mode's system tuning: the same one-click grant for ryoku-game-tune
  # (installed by the hardware glob above), so the panel toggle applies the deep
  # idle, watchdog, lock-mitigation and VM knobs without a prompt. Its argument
  # set is three fixed verbs and every value written is a constant in the helper,
  # so the grant stays safe.
  install -Dm644 "$_repo/system/hardware/power/53-ryoku-game-tune.rules" \
    "$pkgdir/usr/share/polkit-1/rules.d/53-ryoku-game-tune.rules"

  if [[ $RYOKU_INIT_MODE == systemd ]]; then
    # Arch owns this Limine, mkinitcpio and Plymouth integration. Void uses
    # dracut and has no package hook directory matching these files.
    install -Dm755 "$_repo/system/boot/limine/ryoku-windows-entry" \
      "$pkgdir/usr/bin/ryoku-windows-entry"
    install -Dm755 "$_repo/system/boot/limine/45-ryoku-windows" \
      "$pkgdir/etc/boot/hooks/post.d/45-ryoku-windows"
    install -Dm644 "$_repo/system/boot/mkinitcpio/install/ryoku-gpu-trim" \
      "$pkgdir/usr/lib/initcpio/install/ryoku-gpu-trim"
    install -Dm644 "$_repo/system/boot/mkinitcpio/install/ryoku-console-keys" \
      "$pkgdir/usr/lib/initcpio/install/ryoku-console-keys"
    install -Dm644 "$_repo/system/boot/mkinitcpio/hooks/ryoku-console-keys" \
      "$pkgdir/usr/lib/initcpio/hooks/ryoku-console-keys"
    install -d "$pkgdir/usr/share/plymouth/themes/ryoku"
    cp -a "$_repo/system/boot/plymouth/ryoku/." "$pkgdir/usr/share/plymouth/themes/ryoku/"
    chmod -R u=rwX,go=rX "$pkgdir/usr/share/plymouth/themes/ryoku"
    install -Dm644 "$_repo/system/boot/limine/limine.conf" \
      "$pkgdir/usr/share/ryoku/boot/limine.conf"
    install -Dm644 "$_repo/system/boot/limine/default.conf" \
      "$pkgdir/usr/share/ryoku/boot/default.conf"
    install -Dm755 "$_repo/system/boot/ryoku-boot-apply" "$pkgdir/usr/bin/ryoku-boot-apply"
  else
    # The theme itself is init-neutral even though Void's dracut integration is
    # separate from Arch's mkinitcpio and Limine hooks.
    install -d "$pkgdir/usr/share/plymouth/themes/ryoku"
    cp -a "$_repo/system/boot/plymouth/ryoku/." "$pkgdir/usr/share/plymouth/themes/ryoku/"
    chmod -R u=rwX,go=rX "$pkgdir/usr/share/plymouth/themes/ryoku"
    install_runit_payload
  fi
}
case "${1:-}" in
  build)
    ;;
  install)
    pkgdir=${2:?install requires a destination root}
    RYOKU_INIT_MODE=${3:?install requires systemd or runit}
    case "$RYOKU_INIT_MODE" in
      systemd|runit) ;;
      *) printf 'unsupported init mode: %s\n' "$RYOKU_INIT_MODE" >&2; exit 2 ;;
    esac
    package
    ;;
  *)
    printf 'usage: %s <build|install DESTDIR systemd|runit>\n' "$0" >&2
    exit 2
    ;;
esac
