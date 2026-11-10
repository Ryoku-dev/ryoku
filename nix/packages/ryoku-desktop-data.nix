{ pkgs, src }:

let
  phonePython = pkgs.python3.withPackages (ps: [ ps.dbus-python ps.pygobject3 ps.pillow ]);
in
pkgs.stdenv.mkDerivation {
  pname = "ryoku-desktop-data";
  version = "unstable";

  inherit src;

  nativeBuildInputs = [
    pkgs.cmake
    pkgs.ninja
    pkgs.perl
    pkgs.pkg-config
  ];

  buildInputs = [
    pkgs.gtk3
    pkgs.webkitgtk_4_1
  ];

  dontConfigure = true;
  dontBuild = true;

  installPhase = ''
    runHook preInstall

    cfg="$out/share/ryoku/config"

    mkdir -p \
      "$cfg" \
      "$out/bin" \
      "$out/share/applications" \
      "$out/share/icons/hicolor/scalable/apps" \
      "$out/share/ryoku"

    # ── Hyprland ───────────────────────────────────────────────

    mkdir -p "$cfg/hypr"
    cp -a ryoku/hyprland/. "$cfg/hypr/"

    # Qt QML modules are exposed through the system profile on NixOS.
    substituteInPlace "$cfg/hypr/modules/env.lua" \
      --replace-fail \
        'os.getenv("HOME") .. "/.local/lib/qt6/qml"' \
        'os.getenv("HOME") .. "/.local/lib/qt6/qml:/run/current-system/sw/lib/qt-6/qml"'

    # ── Niri ──────────────────────────────────────────────────

    mkdir -p "$cfg/niri"
    cp -a ryoku/niri/. "$cfg/niri/"

    # NixOS owns graphical-session lifecycle through its declarative user
    # units. Route every compositor's upstream session handoff through the
    # Nix-native bootstrap instead of package-manager cutover machinery.
    substituteInPlace "$cfg/hypr/modules/autostart.lua" \
      --replace-fail \
        'ryoku-host session start xdg-desktop-portal-hyprland.service xdg-desktop-portal-gtk.service' \
        'ryoku-nix-session-start'

    substituteInPlace "$cfg/niri/autostart.kdl" \
      --replace-fail \
        'ryoku-host session start xdg-desktop-portal-gnome.service xdg-desktop-portal-gtk.service' \
        'ryoku-nix-session-start'


    # ── Shared translation catalog ─────────────────────────────
    #
    # Upstream installs this at /usr/share/ryoku/i18n. Nix keeps the
    # same layout inside the immutable desktop-data output and exposes
    # it through RYOKU_I18N_DIR.
    mkdir -p "$out/share/ryoku/i18n"

    install -m0644       ryoku/i18n/langs.json       "$out/share/ryoku/i18n/langs.json"

    install -m0644       ryoku/i18n/catalog/*.json       "$out/share/ryoku/i18n/"

    mkdir -p \
      "$out/share/ryoku/vault/wiki" \
      "$out/share/ryoku/vault/skills/ryoku"
    cp -a ryoku/vault/wiki/. "$out/share/ryoku/vault/wiki/"
    cp -a ryoku/vault/skills/ryoku/. "$out/share/ryoku/vault/skills/ryoku/"
    ryoku/vault/repo-index --packaged "$PWD" \
      "$out/share/ryoku/vault/ryoku-repo.md"

    # ── Main Quickshell shell ──────────────────────────────────

    mkdir -p "$cfg/quickshell"
    cp -a ryoku/shell/quickshell/. "$cfg/quickshell/"

    phone_dir="$cfg/quickshell/shell/modules/phone"
    # Not every phone file uses both forms, so a miss per file is expected; the
    # grep below still fails the build when a bare python3 is left anywhere.
    while IFS= read -r -d "" file; do
      substituteInPlace "$file" \
        --replace-quiet '"python3"' '"${phonePython}/bin/python3"' \
        --replace-quiet "python3 -c" "${phonePython}/bin/python3 -c"
    done < <(find "$phone_dir" -name '*.qml' -print0)
    if grep -rlE --include='*.qml' '"python3"|(^|[^/])python3 -c' "$phone_dir"; then
      echo "phone QML still calls a bare python3" >&2
      exit 1
    fi


    # ── Ryoku Settings / Hub ───────────────────────────────────

    mkdir -p "$cfg/quickshell/hub"
    cp -a ryoku/hub/quickshell/. "$cfg/quickshell/hub/"

    # ── First-party Ryoku apps ─────────────────────────────────
    #
    # Keep the same app contract as upstream: Quickshell apps ship their
    # config tree, while native apps with a CMakeLists.txt build into $out/bin.
    # Desktop entries, icons and helper scripts are shared by both shapes.

    for appdir in ryoku/apps/*/; do
      appname="$(basename "$appdir")"
      has_ui=0

      if [ -d "$appdir/quickshell" ]; then
        has_ui=1
        mkdir -p "$cfg/quickshell/$appname"
        cp -a "$appdir/quickshell/." "$cfg/quickshell/$appname/"
      fi

      if [ -f "$appdir/CMakeLists.txt" ]; then
        has_ui=1
        appbuild="$TMPDIR/ryoku-app-$appname"
        cmake \
          -S "$appdir" \
          -B "$appbuild" \
          -G Ninja \
          -DCMAKE_BUILD_TYPE=Release
        cmake --build "$appbuild"
        install -Dm755 \
          "$appbuild/$appname" \
          "$out/bin/$appname"
      fi

      if [ -d "$appdir/bin" ]; then
        for helper in "$appdir/bin/"*; do
          [ -f "$helper" ] || continue

          install -Dm755 \
            "$helper" \
            "$out/bin/$(basename "$helper")"
        done
      fi

      for desktop in "$appdir"*.desktop; do
        [ -f "$desktop" ] || continue

        install -Dm644 \
          "$desktop" \
          "$out/share/applications/$(basename "$desktop")"
      done

      if [ "$has_ui" -eq 1 ]; then
        icon="$appdir/quickshell/logo.svg"
        [ -f "$icon" ] || icon="$appdir/logo.svg"
        [ -f "$icon" ] || icon="ryoku/assets/brand/logo-mark.svg"

        install -Dm644 \
          "$icon" \
          "$out/share/icons/hicolor/scalable/apps/$appname.svg"
      fi
    done

    # ── Matugen / palette pipeline ─────────────────────────────

    mkdir -p "$cfg/matugen"
    cp -a ryoku/shell/matugen/. "$cfg/matugen/"

    # ── Qt / GTK appearance ────────────────────────────────────

    mkdir -p "$cfg/qt6ct"
    cp ryoku/shell/qt6ct/qt6ct.conf "$cfg/qt6ct/qt6ct.conf"

    mkdir -p "$cfg/gtk-3.0"
    cp ryoku/shell/gtk-3.0/settings.ini "$cfg/gtk-3.0/settings.ini"

    mkdir -p "$cfg/gtk-4.0"
    cp ryoku/shell/gtk-4.0/settings.ini "$cfg/gtk-4.0/settings.ini"

    # ── Application configs ────────────────────────────────────

    mkdir -p "$cfg/btop"
    cp ryoku/apps/btop/btop.conf "$cfg/btop/btop.conf"

    cp ryoku/apps/starship/starship.toml "$cfg/starship.toml"

    mkdir -p "$cfg/fastfetch"
    cp ryoku/apps/fastfetch/config.jsonc \
      "$cfg/fastfetch/config.jsonc"

    cp ryoku/assets/brand/logo-mark-v2.png \
      "$cfg/fastfetch/ryoku-nixos.png"

    mkdir -p "$cfg/kitty"
    cp -a ryoku/apps/kitty/. "$cfg/kitty/"

    # Ryoku's Fish configuration is part of the actual desktop package.
    mkdir -p "$cfg/fish/conf.d"
    cp ryoku/apps/fish/config.fish "$cfg/fish/config.fish"
    cp ryoku/apps/fish/conf.d/rashin.fish "$cfg/fish/conf.d/rashin.fish"

    mkdir -p "$cfg/wireplumber"
    cp -a ryoku/apps/wireplumber/. "$cfg/wireplumber/"

    mkdir -p "$cfg/yazi"
    cp ryoku/apps/yazi/yazi.toml "$cfg/yazi/yazi.toml"

    # The Vencord settings are a seed: materialize lays them only where the
    # client has not written its own yet. The theme's MIT license stays out of
    # the user's themes folder.
    for discord in vesktop equibop; do
      install -Dm644 ryoku/apps/vesktop/themes/ryoku-discord.theme.css \
        "$cfg/$discord/themes/ryoku-discord.theme.css"
      install -Dm644 ryoku/apps/vesktop/settings/settings.json \
        "$cfg/$discord/settings/settings.json"
    done
    install -Dm644 ryoku/apps/vesktop/themes/LICENSE \
      "$out/share/licenses/ryoku-desktop/LICENSE.ryoku-discord"

    mkdir -p "$cfg/nvim"
    cp ryoku/apps/nvim/init.lua "$cfg/nvim/init.lua"
    cp ryoku/apps/nvim/.ryoku-lazyvim "$cfg/nvim/.ryoku-lazyvim"
    cp -a ryoku/apps/nvim/lua "$cfg/nvim/"

    mkdir -p "$cfg/pip"
    cp ryoku/apps/pip/pip.conf "$cfg/pip/pip.conf"

    cp ryoku/apps/chromium-flags.conf \
      "$cfg/chromium-flags.conf"

    ${pkgs.lib.optionalString (builtins.hasAttr "hyprland-preview-share-picker" pkgs) ''
      mkdir -p "$cfg/hyprland-preview-share-picker"
      cp ryoku/apps/hyprland-preview-share-picker/config.yaml \
        "$cfg/hyprland-preview-share-picker/config.yaml"
    ''}

    ${pkgs.lib.optionalString (! builtins.hasAttr "hyprland-preview-share-picker" pkgs) ''
      # NixOS stable releases may not package the preview picker. In that case
      # remove the custom command so xdg-desktop-portal-hyprland falls back to
      # its bundled hyprland-share-picker instead of returning no selection.
      sed -Ei '/^[[:space:]]*custom_picker_binary[[:space:]]*=[[:space:]]*hyprland-preview-share-picker[[:space:]]*$/d' \
        "$cfg/hypr/xdph.conf"
    ''}

    # ── User systemd session units ─────────────────────────────
    #
    # NixOS owns Ryoku's user services and targets declaratively
    # through nix/modules/ryoku.nix. Do not materialize upstream
    # units into ~/.config/systemd/user: user-local units override
    # the NixOS-managed definitions and upstream ExecStart paths
    # target /usr/bin.
    #
    # Upstream unit behaviour is mirrored by the NixOS module.

    # ── Helper binaries used by binds / shell ──────────────────

    for helper in ryoku/hyprland/scripts/ryoku-*; do
      [ -f "$helper" ] || continue

      install -Dm755 \
        "$helper" \
        "$out/bin/$(basename "$helper")"
    done

    # Keep Awake is compositor-neutral upstream now, so it lives under
    # ryoku/shell/scripts rather than the Hyprland helper directory.
    install -Dm755 \
      ryoku/shell/scripts/ryoku-cmd-caffeine \
      "$out/bin/ryoku-cmd-caffeine"

    # NixOS coreutils is a multicall binary and dispatches by argv[0].
    # Upstream uses `exec -a ryoku-caffeine-inhibit sleep infinity` to give
    # the inhibitor child a friendly process name, but renaming argv[0] makes
    # Nix coreutils reject it as an unknown applet. The surrounding
    # systemd-inhibit process/unit already identifies and owns the inhibitor.
    substituteInPlace "$out/bin/ryoku-cmd-caffeine" \
      --replace-fail \
        "exec -a ryoku-caffeine-inhibit sleep infinity" \
        "exec sleep infinity"

    install -Dm755 \
      ryoku/shell/quickshell/plugins/ryoku-plugins-place \
      "$out/bin/ryoku-plugins-place"

    install -Dm755 \
      ryoku/apps/fastfetch/ryoku-fastfetch \
      "$out/bin/ryoku-fastfetch"


    # ── Ryotunes ───────────────────────────────────────────────
    #
    # Current Ryoku ships Ryotunes as a compiled application.
    # Musubi packages it independently instead of materializing
    # the retired browser-wrapper files from ryoku/apps.

    # ── Lockscreen ─────────────────────────────────────────────

    mkdir -p "$out/share/ryoku/lockscreen"

    cp -a \
      ryoku/lockscreen/qylock \
      "$out/share/ryoku/lockscreen/qylock"

    install -Dm755 \
      ryoku/lockscreen/install-qylock \
      "$out/share/ryoku/lockscreen/install-qylock"

    patchShebangs "$out/share/ryoku/lockscreen/install-qylock"

    # RYOKU NIXOS QYLOCK WRITABLE COPY
    #
    # install-qylock stages with `cp -a`. When its bundle comes from /nix/store,
    # that faithfully preserves the store's read-only directory modes. The
    # installer then creates themes_link inside the copied lockscreen and dies
    # with EACCES. Make only the user-owned copies writable after each copy.
    substituteInPlace "$out/share/ryoku/lockscreen/install-qylock" \
      --replace-fail \
        'user cp -a "$BUNDLE/quickshell-lockscreen" "$LOCK_DIR"' \
        'user cp -a "$BUNDLE/quickshell-lockscreen" "$LOCK_DIR"
user chmod -R u+w "$LOCK_DIR"' \
      --replace-fail \
        'user cp -a "$BUNDLE/themes/clockwork/orbital" "$STAGED_THEME_DIR/clockwork/orbital"' \
        'user cp -a "$BUNDLE/themes/clockwork/orbital" "$STAGED_THEME_DIR/clockwork/orbital"
user chmod -R u+w "$STAGED_THEME_DIR"' \
      --replace-fail \
        'user cp -a "$BUNDLE/themes/clockwork/orbital" "$QYLOCK_DIR/themes/clockwork/orbital"' \
        'user cp -a "$BUNDLE/themes/clockwork/orbital" "$QYLOCK_DIR/themes/clockwork/orbital"
user chmod -R u+w "$QYLOCK_DIR/themes/clockwork/orbital"'

    # ── Desktop integration ────────────────────────────────────

    install -Dm644 \
      ryoku/apps/nautilus/ryoku-stash-menu.py \
      "$out/share/ryoku/nautilus/ryoku-stash-menu.py"

    # ── Browser integration ────────────────────────────────────

    mkdir -p "$out/share/ryoku/browser"

    (
      cd ryoku/browser
      sh ./build.sh
    )

    cp -a ryoku/browser/. "$out/share/ryoku/browser/"

    # ── Wallpapers ─────────────────────────────────────────────

    mkdir -p "$out/share/ryoku/wallpapers"

    if [ -d ryoku/assets/wallpapers ]; then
      cp -a \
        ryoku/assets/wallpapers/. \
        "$out/share/ryoku/wallpapers/"
    fi

    # ── Ryoku decor artwork ────────────────────────────────────

    mkdir -p "$out/share/ryoku/ryodecors"

    cp -a \
      ryoku/assets/ryodecors/. \
      "$out/share/ryoku/ryodecors/"

    # ── Brand assets ───────────────────────────────────────────

    mkdir -p "$out/share/ryoku/brand"

    cp -a \
      ryoku/assets/brand/. \
      "$out/share/ryoku/brand/"

    # ── Packaged rices ─────────────────────────────────────────
    #
    # The Hub reads these through RYOKU_SYSTEM_RICES_DIR. Applying a rice
    # copies them into user state, so commands stay bare names and the
    # fastfetch tagline keeps the distro token.

    mkdir -p "$out/share/ryoku/rices"

    cp -a \
      ryoku/assets/rices/. \
      "$out/share/ryoku/rices/"

    for rice in "$out/share/ryoku/rices"/*/; do
      if [ -f "$rice/fastfetch.jsonc" ]; then
        substituteInPlace "$rice/fastfetch.jsonc" \
          --replace-quiet 'a hand-built Arch desktop' 'a hand-built {$RYOKU_BASE_OS} desktop'
      fi
      if [ -f "$rice/rice.json" ]; then
        substituteInPlace "$rice/rice.json" \
          --replace-quiet '"/usr/bin/kitty"' '"kitty"'
      fi
    done

    if grep -lw 'Arch' "$out/share/ryoku/rices"/*/fastfetch.jsonc; then
      echo "Ryoku Nix branding guard: a packaged fastfetch style still names Arch" >&2
      exit 1
    fi

    # ── Nix runtime path normalization ─────────────────────────
    #
    # Normalize executable paths in the packaged payload so shell and lockscreen
    # helpers use the generation-owned runtime tools available on NixOS.
    #
    # The qylock payload gets the same treatment. ryoku-qylock-materialize copies
    # it into the home for the in-session lock, and its helpers run outside the
    # shell service: the unlock helper's sleep guard is a transient unit started
    # with an absolute /usr/bin/sleep, so on NixOS it died at once and every
    # correct password was answered with a failed unlock.
    # Searches use -r, not -R: the qylock imports vendor QtGraphicalEffects as
    # symlinks into /usr/lib/qt6, which dangle here and are not ours to rewrite.
    runtime_roots=("$cfg/quickshell" "$out/share/ryoku/lockscreen")

    # Replace whole paths only: a substring match also hits paths an earlier
    # row already rewrote ('/bin/bash' inside '…-bash/bin/bash'). A JS escape
    # such as \n right before a path still counts as a boundary.
    rewrite_runtime_path() {
      from="$1"
      to="$2"
      while IFS= read -r -d "" file; do
        from="$from" to="$to" perl -pi -e \
          's#(?:(?<![A-Za-z0-9._+-])|(?<=\\[nrt]))\Q$ENV{from}\E(?![A-Za-z0-9._+-])#$ENV{to}#g' \
          "$file"
      done < <(grep -IlZr -- "$from" "''${runtime_roots[@]}" 2>/dev/null || true)
    }

    rewrite_runtime_path '/usr/bin/bash' '${pkgs.bash}/bin/bash'
    rewrite_runtime_path '/bin/bash' '${pkgs.bash}/bin/bash'
    rewrite_runtime_path '/bin/sh' '${pkgs.bash}/bin/sh'
    rewrite_runtime_path '/usr/bin/pkexec' 'pkexec'
    rewrite_runtime_path '/usr/bin/env' '${pkgs.coreutils}/bin/env'
    rewrite_runtime_path '/usr/bin/mkdir' '${pkgs.coreutils}/bin/mkdir'
    rewrite_runtime_path '/usr/bin/mv' '${pkgs.coreutils}/bin/mv'
    rewrite_runtime_path '/usr/bin/rm' '${pkgs.coreutils}/bin/rm'
    rewrite_runtime_path '/usr/bin/cp' '${pkgs.coreutils}/bin/cp'
    rewrite_runtime_path '/usr/bin/stat' '${pkgs.coreutils}/bin/stat'
    rewrite_runtime_path '/usr/bin/sleep' '${pkgs.coreutils}/bin/sleep'
    rewrite_runtime_path '/usr/bin/base64' '${pkgs.coreutils}/bin/base64'
    rewrite_runtime_path '/usr/bin/basename' '${pkgs.coreutils}/bin/basename'
    rewrite_runtime_path '/usr/bin/cat' '${pkgs.coreutils}/bin/cat'
    rewrite_runtime_path '/usr/bin/df' '${pkgs.coreutils}/bin/df'
    rewrite_runtime_path '/usr/bin/id' '${pkgs.coreutils}/bin/id'
    rewrite_runtime_path '/usr/bin/install' '${pkgs.coreutils}/bin/install'
    rewrite_runtime_path '/usr/bin/date' '${pkgs.coreutils}/bin/date'
    rewrite_runtime_path '/usr/bin/timeout' '${pkgs.coreutils}/bin/timeout'
    rewrite_runtime_path '/usr/bin/test' '${pkgs.coreutils}/bin/test'
    rewrite_runtime_path '/usr/bin/find' '${pkgs.findutils}/bin/find'
    rewrite_runtime_path '/usr/bin/grep' '${pkgs.gnugrep}/bin/grep'
    rewrite_runtime_path '/usr/bin/awk' '${pkgs.gawk}/bin/awk'
    rewrite_runtime_path '/usr/bin/getent' '${pkgs.getent}/bin/getent'
    rewrite_runtime_path '/usr/bin/curl' '${pkgs.curl}/bin/curl'
    rewrite_runtime_path '/usr/bin/python3' '${pkgs.python3}/bin/python3'
    rewrite_runtime_path '/usr/bin/notify-send' '${pkgs.libnotify}/bin/notify-send'
    rewrite_runtime_path '/usr/bin/busctl' '${pkgs.systemd}/bin/busctl'
    rewrite_runtime_path '/usr/bin/systemctl' '${pkgs.systemd}/bin/systemctl'
    rewrite_runtime_path '/usr/bin/file' '${pkgs.file}/bin/file'
    rewrite_runtime_path '/usr/bin/pgrep' '${pkgs.procps}/bin/pgrep'
    rewrite_runtime_path '/usr/bin/pkill' '${pkgs.procps}/bin/pkill'
    rewrite_runtime_path '/usr/bin/which' '${pkgs.which}/bin/which'
    rewrite_runtime_path '/usr/bin/lscpu' '${pkgs.util-linux}/bin/lscpu'
    rewrite_runtime_path '/usr/bin/wl-copy' '${pkgs.wl-clipboard}/bin/wl-copy'
    rewrite_runtime_path '/usr/bin/pw-play' '${pkgs.pipewire}/bin/pw-play'
    rewrite_runtime_path '/usr/bin/pactl' '${pkgs.pulseaudio}/bin/pactl'
    rewrite_runtime_path '/usr/bin/grim' '${pkgs.grim}/bin/grim'
    rewrite_runtime_path '/usr/bin/magick' '${pkgs.imagemagick}/bin/magick'
    rewrite_runtime_path '/usr/bin/hyprpicker' '${pkgs.hyprpicker}/bin/hyprpicker'
    rewrite_runtime_path '/usr/bin/xdg-settings' '${pkgs.xdg-utils}/bin/xdg-settings'

    # Optional/user-selected applications must remain optional, but still use PATH.
    rewrite_runtime_path '/usr/bin/flatpak' 'flatpak'
    rewrite_runtime_path '/usr/bin/easyeffects' 'easyeffects'
    rewrite_runtime_path '/usr/bin/dunstify' 'dunstify'
    rewrite_runtime_path '/usr/bin/firefox' 'firefox'
    rewrite_runtime_path '/usr/bin/code' 'code'
    rewrite_runtime_path '/usr/bin/nautilus' '${pkgs.nautilus}/bin/nautilus'
    rewrite_runtime_path '/usr/bin/kitty' '${pkgs.kitty}/bin/kitty'

    rewrite_runtime_path '/usr/share/X11/xkb/rules/base.lst' '${pkgs.xkeyboard_config}/share/X11/xkb/rules/base.lst'
    rewrite_runtime_path '/usr/share/zoneinfo' '${pkgs.tzdata}/share/zoneinfo'
    rewrite_runtime_path '/usr/share/applications' '/run/current-system/sw/share/applications'
    rewrite_runtime_path '/usr/share/icons' '/run/current-system/sw/share/icons'
    rewrite_runtime_path '/usr/share/sounds' '/run/current-system/sw/share/sounds'

    # A spliced path (…-bash/nix/store/…) never resolves. grep exits 2 on read
    # errors, which must not pass as nothing found.
    spliced_status=0
    grep -IrnE '(^|[^\\])[A-Za-z0-9._+-]/nix/store/' "''${runtime_roots[@]}" || spliced_status=$?
    if [ "$spliced_status" -ne 1 ]; then
      echo "Ryoku Nix runtime guard: store path spliced into another path (grep status $spliced_status)" >&2
      exit 1
    fi

    if grep -IrnE '/usr/bin/(bash|pkexec|env|mkdir|mv|rm|cp|stat|sleep|base64|basename|cat|df|id|install|date|timeout|test|find|grep|awk|getent|curl|python3|notify-send|busctl|systemctl|file|pgrep|pkill|which|lscpu|wl-copy|pw-play|pactl|grim|magick|hyprpicker|xdg-settings)' "''${runtime_roots[@]}"; then
      echo "Ryoku Nix runtime guard: critical FHS command path survived packaging" >&2
      exit 1
    fi

    chmod -R u=rwX,go=rX "$out/share"

    runHook postInstall
  '';
  meta = {
    description = "Ryoku desktop configuration, assets, applications and integration data";
    homepage = "https://github.com/Ryoku-dev/ryoku-arch";
    license = pkgs.lib.licenses.gpl3Only;
    platforms = [ "x86_64-linux" ];
  };

}
