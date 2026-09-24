#!/usr/bin/env bash
# Deploy the Ryoku shell from this repo into the live config. One way: the repo
# is the source, the shell configs replace the matching ones under ~/.config,
# including the Hyprland config. Builds ryoku-shell and puts it on PATH.
#
#   deploy.sh              build + install, then apply live (provider config.reload).
#   deploy.sh --no-reload  build + install + stage the files, but DO NOT touch
#                          the running session. The new config takes effect on
#                          the next login. Useful so a live swap can't disrupt
#                          the current session.
#
# Hyprland auto-reloads its config on change. The hypr swap below builds the new
# config in a staging dir and renames it into place (near-atomic), so hyprland.lua
# is never missing mid-swap and emergency mode can't trip; auto-reload is paused
# too as a belt. Live mode reloads once at the end; staged mode leaves the swap
# for the next login.
set -euo pipefail

reload=1
[[ "${1:-}" == "--no-reload" ]] && reload=0

here="$(cd "$(dirname "$0")" && pwd)"
cfg="${XDG_CONFIG_HOME:-$HOME/.config}"
bindir="$HOME/.local/bin"
say() { printf '  %s\n' "$*"; }

# Lay the user's overrides over the freshly-deployed base: a regular file under
# ~/.config/ryoku/user_edits wins at the mirrored ~/.config path (a fork), the
# one-way overlay `ryoku materialize` also applies on an installed box. Symlinks
# (the store discovery pointers) and the overlay tree itself are skipped. Absent
# by default, so a box with no user edits sees no change.
overlay_user_edits() {
  local root="$cfg/ryoku/user_edits"
  [[ -d $root ]] || return 0
  local src rel dst n=0
  while IFS= read -r -d '' src; do
    rel="${src#"$root"/}"
    [[ $rel == ryoku/user_edits/* ]] && continue
    dst="$cfg/$rel"
    mkdir -p "${dst%/*}"
    cp -f "$src" "$dst"
    ((++n))
  done < <(find "$root" -type f -not -name '*.md' -print0)
  (( n > 0 )) && say "overlaid $n user edit(s)"
  return 0
}

# Quickshell links Qt's private API, so an AUR build (quickshell-git) stops
# loading after a Qt update. Restarting into that leaves a black screen with no
# way back, so check first and keep the live desktop instead.
check_renderer() {
  local out
  if out=$(qs --version 2>&1); then
    return 0
  fi
  say "ryoku-shell NOT restarted: quickshell cannot start"
  printf '%s\n' "$out" | sed 's/^/    /' >&2
  cat >&2 <<'EOF'
    Quickshell links Qt's private API, so a build made against another Qt will
    not load. The repo package is rebuilt with Qt; quickshell-git is not:
        sudo pacman -S quickshell
    Then run this deploy again. The desktop you have now was left running.
EOF
  return 1
}

restart_shell() {
  local shell=$bindir/ryoku-shell
  local log="${XDG_STATE_HOME:-$HOME/.local/state}/ryoku-shell.log"

  [[ -x $shell ]] || return 0
  check_renderer || return 0
  "$bindir/ryoku-reload-cover" begin >/dev/null 2>&1 || true
  systemctl --user stop ryoku-shell 2>/dev/null || true
  "$shell" quit >/dev/null 2>&1 || true
  for _ in {1..20}; do
    "$shell" ping >/dev/null 2>&1 || break
    sleep 0.1
  done

  # quit should stop the surfaces, but a crashed daemon orphans them and the
  # leftover qs keeps its single-instance lock, so the fresh pill cant come up and
  # the new daemon dies with it. clear any strays before i start again. the
  # pattern is anchored so a user's own longer config name never matches;
  # plugins/wallpaper are retired residents a stale daemon may still hold.
  for c in pill launcher visualizer widgets overview plugins wallpaper; do
    pkill -f "qs -c $c(\$| )" >/dev/null 2>&1 || true
  done
  # kill the video player too: a running livewall satisfies init's liveAlive
  # early return, so a freshly built binary would never take effect until the
  # next live switch. the restarted daemon relaunches it from state.
  pkill -x ryogami-live >/dev/null 2>&1 || true
  pkill -x ryoku-livewall >/dev/null 2>&1 || true  # pre-rename orphans
  sleep 0.2

  mkdir -p "$(dirname -- "$log")"
  # under systemd when the unit is installed, so the daemon stays supervised;
  # bare start otherwise (first deploy on a fresh checkout).
  if systemctl --user daemon-reload 2>/dev/null && systemctl --user restart ryoku-shell 2>/dev/null; then
    say "restarted ryoku-shell daemon (systemd unit)"
  else
    if command -v setsid >/dev/null 2>&1; then
      setsid "$shell" daemon >"$log" 2>&1 < /dev/null &
    else
      nohup "$shell" daemon >"$log" 2>&1 < /dev/null &
    fi
    say "restarted ryoku-shell daemon -> $log"
  fi
}

# Building the desktop from a checkout needs the Go toolchain (cmake/ninja and
# makepkg below self-gate; go is the one hard requirement). A packaged box that
# was switched to a checkout channel without it would otherwise die here with a
# bare "go: command not found"; name the problem and the fix.
if ! command -v go >/dev/null 2>&1; then
  printf '  the Go toolchain is required to build the desktop from a checkout, but go is not installed.\n' >&2
  printf '    install it:  sudo pacman -S --needed go\n' >&2
  printf '    (a packaged install updates through pacman and does not build from source; check ryoku status.)\n' >&2
  exit 1
fi

# Build the daemon/client and put it on PATH.
say "building ryoku-shell"
(cd "$here/ipc" && go build -o ryoku-shell .)
mkdir -p "$bindir"
install -m755 "$here/ipc/ryoku-shell" "$bindir/ryoku-shell"
say "installed $bindir/ryoku-shell"
# Build every window-manager provider the repo carries, not just the running
# one: a checkout must be able to deploy, then switch compositors and find the
# other provider already on PATH. deploy routes the config-swap pause and reload
# below through whichever one is live.
for p in "$here/../wm"/*/; do
  [[ -f "$p/main.go" ]] || continue
  name=${p%/}; name=${name##*/}
  say "building ryoku-wm-$name"
  (cd "$p" && go build -o "ryoku-wm-$name" .)
  install -m755 "$p/ryoku-wm-$name" "$bindir/ryoku-wm-$name"
  say "installed $bindir/ryoku-wm-$name"
done
# Every shell leaf script the bar, launcher, Hub, keybinds, recorder and the
# daemon call by bare name (ryoku-app, ryoku-cmd-*, ryoku-sysinfo, the recorder
# helpers, ...). They ride the shell to PATH with no compositor config tree, so
# a niri box that ships no compositor scripts still gets every one. One glob,
# mirroring the ryoku-shell package.
for s in "$here/scripts"/ryoku-*; do
  [[ -f $s ]] || continue
  install -m755 "$s" "$bindir/${s##*/}"
done
# ryostage: the wallpaper engine's launcher, not a ryoku-* name.
install -m755 "$here/scripts/ryostage" "$bindir/ryostage"
# The .sh helpers the shell drives by bare name: the Stash sidebar's cobalt queue
# and its compress/install/download backends, the LocalSend LAN transfer, the
# clipboard-thumbnail generator. Shell scripts, so they ride the shell to PATH.
for s in "$here/scripts"/*.sh; do
  [[ -f $s ]] || continue
  install -m755 "$s" "$bindir/${s##*/}"
done
# Depth and Parallax merged into ryostage; a checkout box that installed the old
# helpers keeps them on PATH forever otherwise (pacman drops them on packaged boxes).
rm -f "$bindir"/ryoku-{depth,parallax-engine}

# Build ryogami-live, the software-decode video-wallpaper daemon the shell drives
# for live wallpapers. Needs wayland-scanner + a C toolchain + ffmpeg/wayland dev
# libs (build-time only); skip cleanly when absent so a plain config deploy still
# works (it ships prebuilt on installs, and a missing daemon just leaves the clip's
# still frame as the wallpaper).
if command -v wayland-scanner >/dev/null 2>&1 && command -v cc >/dev/null 2>&1 &&
   "$here/livewall/build.sh" "$bindir/ryogami-live"; then
  say "installed $bindir/ryogami-live"
else
  say "skipped ryogami-live (toolchain or ffmpeg/wayland dev libs absent; live falls back to the still)"
fi

# Build ryogami, the Go wallpaper daemon (catalog, thumbs, applies, depth
# surface) the shell and the wall-ui picker drive over ryogami.sock. Same Go
# toolchain the rest of the desktop builds with, so no extra gate.
say "building ryogami"
(cd "$here/ryogami/daemon" && go build -o ryogami .)
install -m755 "$here/ryogami/daemon/ryogami" "$bindir/ryogami"
say "installed $bindir/ryogami"

# Stage the wall-ui, the vendored skwd-wall picker the daemon spawns through
# quickshell over ryogami.sock. Pure QML; the unit rewrite below points the
# daemon at this copy (the package resolver default is /usr/share/ryogami).
datadir="${XDG_DATA_HOME:-$HOME/.local/share}"
rm -rf "$datadir/ryogami/wall-ui"
mkdir -p "$datadir/ryogami/wall-ui"
cp -a "$here/ryogami/wall-ui/." "$datadir/ryogami/wall-ui/"
say "installed wall-ui -> $datadir/ryogami/wall-ui"
# Seed the picker's own config once; user edits persist across deploys. The
# empty object takes every built-in default (wallpapers in ~/Pictures/Wallpapers)
# and the marker skips the first-run onboarding on a box that already has walls.
if [[ ! -f "$cfg/ryogami-wall/config.json" ]]; then
  mkdir -p "$cfg/ryogami-wall"
  printf '{}\n' > "$cfg/ryogami-wall/config.json"
fi
[[ -e "$cfg/ryogami-wall/.bootstrapped" ]] || : > "$cfg/ryogami-wall/.bootstrapped"

# Build the Ryoku Hub backend (a separate Go binary; the hub's quickshell config
# shells out to it for the keybind legend and its TOML config).
say "building ryoku-hub"
(cd "$here/../hub/backend" && go build -o ryoku-hub .)
install -m755 "$here/../hub/backend/ryoku-hub" "$bindir/ryoku-hub"
say "installed $bindir/ryoku-hub"
# Build the Ryoku Rashin backend (the optional agent OS daemon; a separate Go
# binary that serves the dashboard and bridges the Hermes agent over ACP).
say "building ryoku-rashin"
(cd "$here/../rashin/backend" && go build -o ryoku-rashin .)
install -m755 "$here/../rashin/backend/ryoku-rashin" "$bindir/ryoku-rashin"
# `rashin` is the terminal-lane command: the same binary under a second name
# (busybox pattern), argv0 routes a bare argument to the terminal ask.
ln -sf ryoku-rashin "$bindir/rashin"
say "installed $bindir/ryoku-rashin (and the rashin command)"
# Pre-index the checkout for the Rashin vault: dev-machine equivalent of the
# snapshot the package ships to /usr/share/ryoku/rashin.
"$bindir/ryoku-rashin" repo-index "$here/../.." \
  "${XDG_STATE_HOME:-$HOME/.local/state}/ryoku/rashin-repo.md"
say "indexed ryoku repo for rashin"
# Rashin's systemd user unit: the dev deploy points ExecStart at ~/.local/bin
# (the package ships /usr/bin); reload so systemctl sees the fresh unit.
mkdir -p "$cfg/systemd/user"
sed "s|^ExecStart=.*|ExecStart=$bindir/ryoku-rashin serve --if-enabled|" \
  "$here/../rashin/systemd/ryoku-rashin.service" > "$cfg/systemd/user/ryoku-rashin.service"
systemctl --user daemon-reload 2>/dev/null || true
say "installed rashin systemd user unit"
# Rashin is on by default: bring it up at boot now unless the user opted out.
"$bindir/ryoku-rashin" ensure 2>/dev/null || true
say "building ryoku CLI"
(cd "$here/../cli" && go build -o ryoku .)
install -m755 "$here/../cli/ryoku" "$bindir/ryoku"
# every system helper the package ships to /usr/bin, by the same globs, so a new
# hardware or container helper reaches a checkout the moment it lands.
for s in "$here/../../system/hardware"/*/ryoku-* "$here/../../system/containers"/ryoku-*; do
  [[ -f $s && -x $s ]] || continue
  install -m755 "$s" "$bindir/${s##*/}"
done
for s in "$here/../../system/extras"/ryoku-*; do
  install -m755 "$s" "$bindir/${s##*/}"
done
# the extras actuator (renamed from ryoku-extras-install); the ryoku-* glob
# above no longer matches it, so install it by name.
install -m755 "$here/../../system/extras/ryostore-install" "$bindir/ryostore-install"
install -m755 "$here/quickshell/plugins/ryoku-plugins-place" "$bindir/ryoku-plugins-place"
# AI-usage collectors: refresh ~/.cache/{claude,codex,opencode}-usage.json for
# the qsbar AI pill, driven by the ryoku-ai-usage.timer installed below. The
# package ships them to /usr/bin; the dev loop puts the current copies on PATH.
for s in "$here/bin"/claude-usage "$here/bin"/codex-usage "$here/bin"/opencode-usage; do
  install -m755 "$s" "$bindir/${s##*/}"
done
say "installed the AI-usage collectors to $bindir"
say "installed Ryoku CLI and hardware helpers"

# Privileged network helpers + their polkit rules. A packaged install ships these
# to /usr/bin and /usr/share/polkit-1/rules.d; a dev box has neither, so pkexec
# has no rule to match and the qsbar DNS/wifi toggles silently fail. Install them
# here when sudo is available (skipped cleanly in a sudo-less/CI env), and skip
# each dest that already matches so a redeploy is a no-op.
if command -v sudo >/dev/null 2>&1; then
  netdir="$here/../../system/hardware/network"
  _priv_install() { # src dest mode
    cmp -s "$1" "$2" && return 0
    sudo install -Dm"$3" "$1" "$2" || true
  }
  _priv_install "$netdir/ryoku-dns" /usr/bin/ryoku-dns 755
  _priv_install "$netdir/50-ryoku-dns.rules" /usr/share/polkit-1/rules.d/50-ryoku-dns.rules 644
  _priv_install "$netdir/ryoku-wifi-powersave" /usr/bin/ryoku-wifi-powersave 755
  _priv_install "$netdir/49-ryoku-wifi-powersave.rules" /usr/share/polkit-1/rules.d/49-ryoku-wifi-powersave.rules 644
  _priv_install "$netdir/ryoku-network-kill" /usr/bin/ryoku-network-kill 755
  _priv_install "$netdir/55-ryoku-network-kill.rules" /usr/share/polkit-1/rules.d/55-ryoku-network-kill.rules 644
  _priv_install "$netdir/ryoku-network-kill-guard.service" /usr/lib/systemd/system/ryoku-network-kill-guard.service 644
  _priv_install "$netdir/ryoku-network-kill-disconnect.service" /usr/lib/systemd/system/ryoku-network-kill-disconnect.service 644
  # The Machine page flips the hardware GPU MUX through ryoku-gpu-mux (a
  # root-owned firmware knob); this grant lets the one-click path work on a dev
  # box too, mirroring the packaged rule.
  _priv_install "$here/../../system/hardware/gpu/45-ryoku-gpu-mux.rules" /usr/share/polkit-1/rules.d/45-ryoku-gpu-mux.rules 644
  sudo systemctl daemon-reload || true
  sudo systemctl enable --quiet ryoku-network-kill-guard.service ryoku-network-kill-disconnect.service || true
  say "installed privileged network helpers + polkit rules"
  # Boot look: lay the splash theme, Limine art and ryoku-boot-apply, then apply
  # them (set the splash, deploy the ESP wallpaper + globals, rebuild initramfs).
  bootsrc="$here/../../system/boot"
  sudo install -d /usr/share/plymouth/themes/ryoku
  sudo cp -a "$bootsrc/plymouth/ryoku/." /usr/share/plymouth/themes/ryoku/
  sudo install -Dm644 "$bootsrc/limine/limine.conf" /usr/share/ryoku/boot/limine.conf
  sudo install -Dm644 "$bootsrc/limine/default.conf" /usr/share/ryoku/boot/default.conf
  sudo install -Dm755 "$bootsrc/ryoku-boot-apply" /usr/bin/ryoku-boot-apply
  # the mkinitcpio install hook the HOOKS drop-in names: mkinitcpio aborts on a
  # hook it cannot find, and ryoku-boot-apply rebuilds the images right below.
  sudo install -Dm644 "$bootsrc/mkinitcpio/install/ryoku-gpu-trim" \
    /usr/lib/initcpio/install/ryoku-gpu-trim
  sudo ryoku-boot-apply || true
  say "installed and applied the boot splash + Limine theme"
fi

# Record the checkout this deploy came from and the commit it laid down, so the
# deployed `ryoku` binary (on PATH, far from the repo) can track the update
# channel in `ryoku status`: it compares this commit (what is now running)
# against origin/main. One way, like every step: the repo is the source.
repo_root="$(cd "$here/../.." && pwd)"
state_dir="${XDG_STATE_HOME:-$HOME/.local/state}/ryoku"
mkdir -p "$state_dir"
printf '%s\n' "$repo_root" > "$state_dir/repo"
git -C "$repo_root" rev-parse HEAD > "$state_dir/deployed" 2>/dev/null || rm -f "$state_dir/deployed"
say "recorded update-channel checkout -> $state_dir/repo"

# The `ryoku` agent skill resolves through the repo pointer just recorded:
# `ryoku-rashin wire` looks at RYOKU_RASHIN_SKILLS, then /usr/share/ryoku/skills
# (absent on a checkout), then <repo>/ryoku/rashin/skills via ~/.local/state/
# ryoku/repo, so it finds THIS checkout's skill dir with no separate symlink.
# Refresh the links now, but only when a Rashin vault already exists (the user
# opted in); never wire agents for a box that left Rashin off.
if [[ -d "$datadir/ryoku/rashin" && -x "$bindir/ryoku-rashin" ]]; then
  "$bindir/ryoku-rashin" wire >/dev/null 2>&1 || true
  say "refreshed rashin agent wiring (ryoku skill + vault pointers)"
fi

# Build the Ryoku.Blobs QML plugin (the frame's blob renderer) and install the
# module onto the user's QML import path. ryoku-shell points QML2_IMPORT_PATH
# there for the quickshell processes it supervises. Needs cmake + ninja +
# qt6-shadertools (build-time only); skip cleanly when the toolchain is absent so
# a plain config deploy still succeeds (the module ships prebuilt on installs).
#
# Stamped with the Qt it was built against, like the Hyprland plugins below: a
# module built against another Qt fails to load and takes the whole surface with
# it, so a Qt update has to force a rebuild.
qmldir="$HOME/.local/lib/qt6/qml"
qtver="$(pacman -Q qt6-base 2>/dev/null | awk '{print $2}')"
qtstamp="$qmldir/Ryoku/Blobs/.qt-version"
if command -v cmake >/dev/null 2>&1 && command -v ninja >/dev/null 2>&1; then
  if [ -n "$qtver" ] && [ "$(cat "$qtstamp" 2>/dev/null)" = "$qtver" ] \
     && [ -n "$(find "$qmldir/Ryoku/Blobs" -name '*.so' -print -quit 2>/dev/null)" ]; then
    say "Ryoku.Blobs already built against Qt $qtver"
  else
    say "building Ryoku.Blobs plugin"
    "$here/plugin/build.sh" "$qmldir"
    [ -n "$qtver" ] && printf '%s\n' "$qtver" > "$qtstamp"
    say "installed Ryoku.Blobs -> $qmldir/Ryoku/Blobs"
  fi
else
  say "skipping Ryoku.Blobs plugin (cmake/ninja not found)"
fi

# Build the optional Hyprland compositor plugins (dynamic-cursors, hyprbars,
# hyprfocus, hyprglass, imgborders, and this repo's keysounds) through the one
# builder the Hub's Plugins page and the doctor use, `ryoku-hub desktop plugins
# rebuild` (built above, which forwards to the provider): it clones each upstream into
# ~/.cache/ryoku/hypr-plugins-src, checks out the commit its hyprpm.toml pins
# for the installed Hyprland, runs the manifest's build steps against the
# installed headers, and lays the .so with its .abi receipt under the user
# plugin path the generated settings.lua loads from (no root, the way the QML
# modules above deploy). Plugins are ABI-locked to the compositor: `--stale`
# rebuilds only a plugin whose receipt no longer matches the headers (a
# Hyprland or aquamarine bump), one that is missing, or keysounds when its
# source changed, so a redeploy is quick. A plugin that fails to build is
# skipped, never fatal: its toggle degrades to off (settings.lua leaves the load
# out and the Plugins page says why). Packaged installs get these from [ryoku]
# as ryoku-desktop deps.
rm -f "$HOME/.local/lib/hyprland/plugins/.hyprland-version"   # the pre-receipt stamp; receipts carry the ABI now
if pkg-config --exists hyprland 2>/dev/null; then
  mkdir -p "$HOME/.cache/ryoku"
  say "building Hyprland compositor plugins that are missing or stale"
  if _out="$("$bindir/ryoku-hub" desktop plugins rebuild --stale --checkout "$here/../.." 2>"$HOME/.cache/ryoku/hypr-plugins-build.log")"; then
    say "  $(jq -r '"built: " + (.built|join(", ")|if .=="" then "none" else . end) + "  skipped: " + (.skipped|length|tostring) + "  failed: " + ((.failed|keys)|join(", ")|if .=="" then "none" else . end)' <<<"$_out")"
    say "  log: ~/.cache/ryoku/hypr-plugins-build.log"
  else
    say "  plugin build could not run (see ~/.cache/ryoku/hypr-plugins-build.log); toggles stay off"
  fi
else
  say "skipping Hyprland compositor plugins (Hyprland headers not found)"
fi

# Install the Ryoku.Ui QML module: the design system every surface imports --
# the shell's configs, the Hub and the first-party apps. Pure QML, a plain copy.
# Note the import path only reaches `qs` when the daemon launches it; a Hub
# started from a keybind needs QML_IMPORT_PATH from the session
# (hyprland/modules/env.lua). An installed system puts it in /usr/lib/qt6/qml
# instead, which Qt finds unaided.
say "installing Ryoku.Ui module"
"$here/../ui/install.sh" "$qmldir"

# Install the translation catalog + langs.json where every surface's I18n looks
# first on a dev box (~/.local/share/ryoku/i18n). Without this the shell and Hub
# fall back to an empty language table -- the Hub's language and regional-format
# pickers then show only Auto and the two English locales. A packaged system
# gets the same files at /usr/share/ryoku/i18n from the ryoku-desktop PKGBUILD.
say "installing Ryoku i18n catalog"
"$here/../i18n/tools/install.sh"

# Seed the decor art the Decor/Placard components render into ~/Pictures/ryodecors
# (beside Wallpapers and livewalls): the dev-loop equivalent of the installer seed
# and `ryoku doctor`. Missing-only, so a swapped or added file survives a redeploy.
decordir="$HOME/Pictures/ryodecors"
mkdir -p "$decordir"
for f in "$here/../assets/ryodecors"/*; do
  [[ -e $f ]] || continue
  [[ -e "$decordir/${f##*/}" ]] || cp -a "$f" "$decordir/"
done
say "seeded decor art -> $decordir"

# Install the Ryoku.PluginKit QML module (the signature kit a plugin imports for
# its content) onto the same import path. Pure QML, so a plain copy, no toolchain.
say "installing Ryoku.PluginKit module"
"$here/quickshell/plugins/kit/install.sh" "$qmldir"
say "installed Ryoku.PluginKit -> $qmldir/Ryoku/PluginKit"

# Install the Ryoku.FrameBars QML module (the shared frame-bar config schema and
# catalogs every config root and the Hub Bar Studio import). Pure QML + JS, a
# plain copy, no toolchain.
say "installing Ryoku.FrameBars module"
"$here/framebars/install.sh" "$qmldir"
say "installed Ryoku.FrameBars -> $qmldir/Ryoku/FrameBars"

# Install the Ryoku.Wm.Hyprland QML module (the provider's global-shortcut and
# focus-grab bridges the shell imports instead of Quickshell.Hyprland). Pure QML.
say "installing Ryoku.Wm.Hyprland module"
rm -rf "$qmldir/Ryoku/Wm/Hyprland"
mkdir -p "$qmldir/Ryoku/Wm/Hyprland"
cp -a "$here/../wm/hyprland/qml/." "$qmldir/Ryoku/Wm/Hyprland/"
say "installed Ryoku.Wm.Hyprland -> $qmldir/Ryoku/Wm/Hyprland"

# Quickshell components: a deployed daemon runs `qs -c <name>`, reading
# ~/.config/quickshell/<name>.
say "installing quickshell components -> $cfg/quickshell"
rm -rf "$cfg/quickshell"
mkdir -p "$cfg/quickshell"
cp -a "$here/quickshell/." "$cfg/quickshell/"

# xdg-desktop-portal: route each compositor's portals. hyprland owns its own
# ScreenCast/Screenshot; niri has no backend, so screen sharing rides gnome and
# only FileChooser is pinned to gtk (the gnome one hangs off a GNOME session).
# Both files land; the portal reads the one named for the running desktop.
install -Dm644 "$here/../hyprland/hyprland-portals.conf" "$cfg/xdg-desktop-portal/hyprland-portals.conf"
install -Dm644 "$here/../niri/niri-portals.conf" "$cfg/xdg-desktop-portal/niri-portals.conf"
# The single-instance shell ships as ryoku/shell/quickshell/shell and lands at
# $cfg/quickshell/shell via the copy above; the ryoku-shell daemon launches it as
# `qs -c shell`, the live desktop.

# Ryoku Hub's quickshell config (qs -c hub), kept beside the shell's components.
mkdir -p "$cfg/quickshell/hub"
cp -a "$here/../hub/quickshell/." "$cfg/quickshell/hub/"

# First-party GUI apps: each ryoku/apps/<name>/quickshell ships as qs -c <name>,
# launched from a keybind and a .desktop entry. Drop in a new app dir and it ships.
appshare="${XDG_DATA_HOME:-$HOME/.local/share}"
for appdir in "$here"/../apps/*/; do
  [[ -d "${appdir}quickshell" ]] || continue
  appname="$(basename "$appdir")"
  mkdir -p "$cfg/quickshell/$appname"
  cp -a "${appdir}quickshell/." "$cfg/quickshell/$appname/"
  for b in "${appdir}bin/"*; do [[ -f "$b" ]] && install -m755 "$b" "$bindir/$(basename "$b")"; done
  # an app may carry Go helper(s): a subdir with a go.mod builds to a bin named
  # for the module (ryovm/fetch -> ryovm-fetch). keeps "drop in an app dir" true.
  for gomod in "${appdir}"*/go.mod; do
    [[ -f "$gomod" ]] || continue
    helperdir="$(dirname "$gomod")"
    helper="$(sed -n -E 's/^module[[:space:]]+//p' "$gomod" | head -1)"
    [[ -n "$helper" ]] || continue
    say "building $helper"
    (cd "$helperdir" && go build -o "$helper" .) && install -m755 "$helperdir/$helper" "$bindir/$helper"
  done
  for d in "${appdir}"*.desktop; do [[ -f "$d" ]] && install -Dm644 "$d" "$appshare/applications/$(basename "$d")"; done
  icon="${appdir}quickshell/logo.svg"; [[ -f "$icon" ]] || icon="$here/../assets/brand/logo-mark.svg"
  install -Dm644 "$icon" "$appshare/icons/hicolor/scalable/apps/$appname.svg"
  say "installed app $appname -> $cfg/quickshell/$appname"
done

# Ryoku Hub (hub/): the surface is deployed above; ship a launcher entry so it
# shows in the app launcher search too, next to the Super+comma keybind.
install -Dm644 "$here/../hub/ryoku-hub.desktop" "$appshare/applications/ryoku-hub.desktop"
install -Dm644 "$here/../assets/brand/logo.svg" "$appshare/icons/hicolor/scalable/apps/ryoku-hub.svg"
say "installed ryoku-hub launcher entry"

# In-session lockscreen (qylock): deploy otherwise never lays it down, so the
# lock button and lock-on-sleep no-op. User-only half, mirroring ryoku doctor.
if [[ -x "$here/../lockscreen/install-qylock" ]]; then
  if RYOKU_QYLOCK_USER_ONLY=1 "$here/../lockscreen/install-qylock" >/dev/null 2>&1; then
    say "installed in-session lockscreen"
  else
    say "lockscreen install skipped"
  fi
fi

# Packaged externals on a checkout box. ryotunes (and every other package
# release/packages pins to an upstream commit) is a [ryoku] package users get
# from pacman; a dev box takes the same signed package from the channel its
# branch publishes to (unstable-dev -> testing, main -> stable) rather than
# spending minutes on a local makepkg that could differ from what ships. The
# release key is in the checkout (release/packages/ryoku-keyring), so trusting
# it needs no network; the stanza is added once and repointed when the tracked
# branch changes, and a [ryoku] that points somewhere Ryoku does not publish
# (a private mirror) is left alone. Skipped cleanly without sudo (CI).
# The Chromium wrapper this script once laid into ~/.local/bin is retired
# first so it can never shadow the app.
if [[ -f "$bindir/ryotunes" ]] && [[ "$(head -c 2 "$bindir/ryotunes" 2>/dev/null)" == '#!' ]] \
   && grep -q 'music.youtube.com' "$bindir/ryotunes"; then
  rm -f "$bindir/ryotunes" "$appshare/applications/ryotunes.desktop" \
    "$appshare/icons/hicolor/scalable/apps/ryotunes.svg"
  say "retired the ryotunes chromium wrapper"
fi
# a locally built copy from the interim makepkg path shadows the package on PATH
if [[ -x "$bindir/ryotunes" ]] && [[ -f "$HOME/.local/share/ryoku/ryotunes.commit" ]]; then
  rm -f "$bindir/ryotunes" "$appshare/applications/ryotunes.desktop" \
    "$HOME/.local/share/ryoku/ryotunes.commit" "$appshare"/icons/hicolor/*/apps/ryotunes.png
  say "retired the locally built ryotunes (the package takes over)"
fi
if command -v sudo >/dev/null 2>&1 && command -v pacman >/dev/null 2>&1; then
  _rkey=EB6D3C0F55A7B3CABA6B2838847B274F025DD6E3
  _rbase="https://repo.ryoku.dev/stable"
  case "${RYOKU_CHANNEL:-$(sed -n 's/^RYOKU_CHANNEL=//p' "$HOME/.config/environment.d/ryoku.conf" 2>/dev/null)}" in
    unstable-dev) _rserver="$_rbase/channels/testing/\$arch" ;;
    *)            _rserver="$_rbase/\$arch" ;;
  esac
  _rcur="$(awk '/^\[ryoku\]/{f=1;next} /^\[/{f=0} f && /^Server/{sub(/^Server *= */,""); print; exit}' /etc/pacman.conf)"
  if [[ -z "$_rcur" ]]; then
    say "adding the [ryoku] repo ($_rserver) so packaged externals install from it"
    sudo pacman-key --add "$repo_root/release/packages/ryoku-keyring/ryoku.gpg" >/dev/null 2>&1 || true
    sudo pacman-key --lsign-key "$_rkey" >/dev/null 2>&1 || true
    printf '\n[ryoku]\nSigLevel = Required\nServer = %s\n' "$_rserver" | sudo tee -a /etc/pacman.conf >/dev/null
  elif [[ "$_rcur" != "$_rserver" ]] && [[ "$_rcur" == "$_rbase"/* ]]; then
    say "repointing the [ryoku] repo at $_rserver"
    sudo sed -i "/^\[ryoku\]/,/^\[/ s|^Server *=.*|Server = $_rserver|" /etc/pacman.conf
  fi
  # -Syu, not -Sy + -S: a refreshed db with an un-upgraded system is the
  # partial-upgrade trap, and a packaged box upgrades on every update anyway.
  _plog="$HOME/.cache/ryoku/deploy-pacman.log"
  mkdir -p "$(dirname "$_plog")"
  # the redirect is the user's file, which is the intent (shellcheck SC2024 is
  # about root-owned targets); pacman's own output goes to the log for -v.
  # --overwrite the ryoku-owned paths the ISO installer and this script seed
  # unowned (privileged helpers, systemd units, polkit rules, the plymouth theme,
  # the boot configs); once ryoku-desktop packages them an unowned copy otherwise
  # aborts the whole -Syu with "exists in filesystem" and nothing upgrades.
  # Mirrors updater.ryokuOverwriteGlob / the doctor's ryokuSystemGlobs.
  _rovw='/usr/bin/ryoku-*,/usr/lib/systemd/system/ryoku-*,/usr/lib/initcpio/install/ryoku-*,/usr/share/polkit-1/rules.d/*ryoku*.rules,/usr/share/plymouth/themes/ryoku/*,/usr/share/ryoku/boot/*'
  _pac_ryotunes() { sudo pacman -Syu --needed --noconfirm --overwrite "$_rovw" ryotunes; }
  # shellcheck disable=SC2024
  if _pac_ryotunes >"$_plog" 2>&1; then
    say "ryotunes from [ryoku]: $(pacman -Q ryotunes 2>/dev/null | awk '{print $2}')"
  elif grep -q 'exists in filesystem' "$_plog"; then
    # a new package now claims files that exist unowned (an installer/deploy
    # stray for any package, not just ryoku): remove the ones no package owns and
    # retry once. A file another package owns is a real conflict, left in place.
    _strays=()
    while IFS= read -r _f; do
      [ -e "$_f" ] || continue
      pacman -Qo "$_f" >/dev/null 2>&1 || _strays+=("$_f")
    done < <(sed -n 's/.*: \(\/[^ ]*\) exists in filesystem.*/\1/p' "$_plog")
    # shellcheck disable=SC2024
    if [ "${#_strays[@]}" -gt 0 ] && sudo rm -f "${_strays[@]}" && _pac_ryotunes >>"$_plog" 2>&1; then
      say "ryotunes from [ryoku]: $(pacman -Q ryotunes 2>/dev/null | awk '{print $2}') (cleared ${#_strays[@]} unowned file(s))"
    else
      say "  ryotunes not installed from [ryoku] (file conflicts remain; see $_plog)"
    fi
  else
    say "  ryotunes not installed from [ryoku] (channel unreachable or not published yet); see $_plog"
  fi
else
  say "skipping packaged externals (sudo or pacman not available)"
fi

# Nautilus stash actions (a nautilus-python extension). Installs ship it system-wide
# from the ryoku-desktop package; the dev loop drops it in the user extensions dir.
install -Dm644 "$here/../apps/nautilus/ryoku-stash-menu.py" \
  "$appshare/nautilus-python/extensions/ryoku-stash-menu.py"
say "installed nautilus stash menu -> $appshare/nautilus-python/extensions"

# The compositor config comes from the ACTIVE provider's payload, so a checkout
# on niri deploys ryoku/niri exactly the way one on Hyprland deploys
# ryoku/hyprland. `ryoku wm config` is the single source for the config dir name
# and the seed list: a second copy of that table here would drift from
# ryoku/wm/detect.go, which is the one place allowed to know it.
wm_conf=$("$bindir/ryoku" wm config 2>/dev/null || true)
wm_name=$(jq -r '.name // empty' <<<"$wm_conf" 2>/dev/null)
wm_dir=$(jq -r '.dir // empty' <<<"$wm_conf" 2>/dev/null)
mapfile -t seeds < <(jq -r '.seeds[]? | sub("^[^/]+/"; "")' <<<"$wm_conf" 2>/dev/null)
wm_bin="$bindir/ryoku-wm-$wm_name"

# Only the LIVE compositor's leaf scripts (ryoku-monitor and friends) land here,
# its own payload the way the package ships it: a niri box gets none, a Hyprland
# box gets Hyprland's. Laying every provider's regardless of the live one is what
# let a niri checkout look fine while a packaged niri box had them all missing,
# so the other providers' copies from an earlier deploy are dropped as well.
for d in "$here/../wm"/*/; do
  other=${d%/}; other=${other##*/}
  [[ $other != "$wm_name" ]] || continue
  for s in "$here/../$other/scripts"/ryoku-*; do
    [[ -f $s ]] || continue
    rm -f "$bindir/${s##*/}"
  done
done
if [[ -n $wm_name && -d "$here/../$wm_name/scripts" ]]; then
  for s in "$here/../$wm_name/scripts"/ryoku-*; do
    [[ -f $s ]] || continue
    install -m755 "$s" "$bindir/${s##*/}"
  done
  say "installed the $wm_name leaf scripts to $bindir"
fi

# Liveness comes from the provider, not from the pause below: a compositor that
# watches its own config has no auto-reload to pause and would read as dead.
wm_live=0
if [[ -n $wm_name && -x $wm_bin ]] && "$wm_bin" state >/dev/null 2>&1; then
  wm_live=1
fi
# Pause auto-reload where the compositor has one, so the swap below never
# exposes a missing config mid-rename. Unsupported is fine: the swap is a
# rename, so a config-watching compositor never sees a partial tree.
if [[ -x $wm_bin ]]; then
  "$wm_bin" act config.autoreload off >/dev/null 2>&1 || true
fi

if [[ -z $wm_dir || ! -d "$here/../$wm_name" ]]; then
  say "no compositor payload for ${wm_name:-none}; skipped the config swap"
else
# The repo tree replaces the base, but the user's own files and the per-machine
# generated drop-ins must survive a redeploy, exactly the way a packaged
# `ryoku materialize` preserves every unshipped file (docs/updates.md). Two
# classes survive: (1) anything the repo tree does NOT ship (the hand-edit
# display file, the generated settings, and anything else the user dropped in)
# is user-owned and carried across untouched; (2) the seed drop-ins the repo
# ships a default for but the machine owns after first boot (the display and GPU
# pins the runtime rewrites, the keyboard and user files) keep their live copy
# over the shipped default. Shipped files stay Ryoku-owned: the repo copy wins,
# matching materialize clobbering them.
#
# Build the new config in a staging dir on the same filesystem, then rename it
# into place. A slow rm+cp of the config dir leaves a long window where the
# entry file is missing; anything that reloads then (a manual reload or a fresh
# login both bypass the autoreload pause) trips the compositor into its error
# path, and on niri a missing include is fatal. A rename swap closes that window.
rm -rf "$cfg/$wm_dir".staging.*
staging="$cfg/$wm_dir.staging.$$"
mkdir -p "$staging"
cp -a "$here/../$wm_name/." "$staging/"
# Carry the user's own files and the per-machine seeds across, mirroring
# materialize: any file the freshly-staged repo tree does not contain is
# user-owned and kept; the seeds keep their live copy over the shipped default.
if [[ -d $cfg/$wm_dir ]]; then
  while IFS= read -r -d '' f; do
    rel=${f#"$cfg/$wm_dir/"}
    [[ -e "$staging/$rel" ]] && continue   # shipped -> Ryoku-owned, repo copy wins
    mkdir -p "$staging/$(dirname "$rel")"
    cp -a "$f" "$staging/$rel"
    # -type l too: a user who symlinks a user-owned file from a dotfiles repo
    # owns it; -type f alone would drop the link and the redeploy would lose
    # their file. cp -a carries the symlink itself.
  done < <(find "$cfg/$wm_dir" \( -type f -o -type l \) -print0)
  for f in "${seeds[@]}"; do
    # -e follows the link and misses a dangling one (repo not mounted yet), so
    # test -L as well; without it a symlinked seed is replaced by the default.
    { [[ -e "$cfg/$wm_dir/$f" || -L "$cfg/$wm_dir/$f" ]]; } && cp -a "$cfg/$wm_dir/$f" "$staging/$f"
  done
fi
# cp -a carries the repo's older mtimes; bump the top level so an mtime-watching
# autoreload still registers the swapped-in config as new, whichever file the
# compositor treats as its entry.
touch "$staging"/* 2>/dev/null || true
if [[ -d $cfg/$wm_dir ]]; then
  bak="$cfg/$wm_dir.bak-$(date +%Y%m%d%H%M%S)"
  mv "$cfg/$wm_dir" "$bak"
  say "backed up existing $wm_dir -> $bak"
  # Keep the newest only: a deploy per session otherwise fills ~/.config with
  # trees, and an older backup recovers nothing the newest does not.
  shopt -s nullglob
  for old in "$cfg/$wm_dir".bak-*; do
    [[ $old == "$bak" ]] && continue
    rm -rf -- "$old"
  done
  shopt -u nullglob
fi
mv "$staging" "$cfg/$wm_dir"
fi

# A checkout box has every provider's binary on PATH, so lay every provider's
# payload too: without its config tree an inactive compositor cannot be logged
# into, and the greeter would offer a session that starts a bare desktop.
#
# Same ownership rule as the active swap above, so an inactive tree does not go
# stale as the repo gains files: shipped files are Ryoku-owned and the repo copy
# wins, while a seed or a hand-edited file that already exists is kept. Each
# provider then authors its own generated config, because a missing include is
# fatal on a compositor with no optional-include escape.
for d in "$here/../wm"/*/; do
  other=${d%/}; other=${other##*/}
  [[ $other == "$wm_name" ]] && continue
  [[ -d "$here/../$other" ]] || continue
  other_conf=$("$bindir/ryoku" wm config "$other" 2>/dev/null || true)
  other_dir=$(jq -r '.dir // empty' <<<"$other_conf" 2>/dev/null)
  [[ -n $other_dir ]] || continue
  mapfile -t other_seeds < <(jq -r '.seeds[]? | sub("^[^/]+/"; "")' <<<"$other_conf" 2>/dev/null)
  mkdir -p "$cfg/$other_dir"
  for f in "${other_seeds[@]}"; do
    # Keep a seed the machine already owns; cp below would overwrite it.
    { [[ -e "$cfg/$other_dir/$f" || -L "$cfg/$other_dir/$f" ]]; } && cp -a "$cfg/$other_dir/$f" "$cfg/$other_dir/$f.keep"
  done
  cp -a "$here/../$other/." "$cfg/$other_dir/"
  for f in "${other_seeds[@]}"; do
    [[ -e "$cfg/$other_dir/$f.keep" ]] && mv "$cfg/$other_dir/$f.keep" "$cfg/$other_dir/$f"
  done
  if [[ -x "$bindir/ryoku-wm-$other" ]]; then
    "$bindir/ryoku-wm-$other" apply "$cfg/ryoku/desktop.json" >/dev/null 2>&1 || true
  fi
  say "laid the $other config tree -> $cfg/$other_dir"
done

wireplumber_policy="$cfg/wireplumber/wireplumber.conf.d/51-ryoku-bluetooth.conf"
wireplumber_before=
[[ -f $wireplumber_policy ]] && wireplumber_before=$(<"$wireplumber_policy")

# Files the machine owns after first boot are seeded once and never re-laid,
# the same generatedSeed set `ryoku materialize` honours (ryoku/cli
# internal/updater/materialize.go): the Hub and the store rewrite
# fastfetch/config.jsonc in place (an imported logo lives in it), matugen owns
# kitty/current-theme.conf. Re-copying them on every deploy is what reset the
# fastfetch emblem on a dev box after each `ryoku update`.
seed_once() { [[ -e $2 ]] || cp -a "$1" "$2"; }

# Palette generation, per-app config, and the user session target.
mkdir -p "$cfg/matugen"; cp -a "$here/matugen/." "$cfg/matugen/"
cp -a "$here/../apps/fish/config.fish" "$cfg/fish/config.fish"
mkdir -p "$cfg/fish/conf.d"; cp -a "$here/../apps/fish/conf.d/." "$cfg/fish/conf.d/"
mkdir -p "$cfg/ryoku-terminal"; cp -a "$here/../apps/terminal-shell/." "$cfg/ryoku-terminal/"
mkdir -p "$cfg/bash"; cp -a "$here/../apps/bash/." "$cfg/bash/"
mkdir -p "$cfg/zsh"; cp -a "$here/../apps/zsh/." "$cfg/zsh/"
mkdir -p "$cfg/qt6ct"; cp -a "$here/qt6ct/qt6ct.conf" "$cfg/qt6ct/qt6ct.conf"
# GTK toolkit baseline for the xsettings-less session; the matugen hook renders
# gtk.css into these same dirs at runtime, so only settings.ini is copied here.
mkdir -p "$cfg/gtk-3.0"; cp -a "$here/gtk-3.0/settings.ini" "$cfg/gtk-3.0/settings.ini"
mkdir -p "$cfg/gtk-4.0"; cp -a "$here/gtk-4.0/settings.ini" "$cfg/gtk-4.0/settings.ini"
mkdir -p "$cfg/btop"; cp -a "$here/../apps/btop/btop.conf" "$cfg/btop/btop.conf"
mkdir -p "$cfg/fastfetch"
seed_once "$here/../apps/fastfetch/config.jsonc" "$cfg/fastfetch/config.jsonc"
install -m755 "$here/../apps/fastfetch/ryoku-fastfetch" "$bindir/ryoku-fastfetch"
mkdir -p "$cfg/kitty"
cp -a "$here/../apps/kitty/kitty.conf" "$cfg/kitty/kitty.conf"
seed_once "$here/../apps/kitty/current-theme.conf" "$cfg/kitty/current-theme.conf"
# ghostty: config is the user's (seeded once, editable); matugen owns ryoku-colors.
mkdir -p "$cfg/ghostty"
seed_once "$here/../apps/ghostty/config" "$cfg/ghostty/config"
seed_once "$here/../apps/ghostty/ryoku-colors" "$cfg/ghostty/ryoku-colors"
mkdir -p "$cfg/wireplumber"; cp -a "$here/../apps/wireplumber/." "$cfg/wireplumber/"
mkdir -p "$cfg/systemd/user"; cp -a "$here/systemd/user/." "$cfg/systemd/user/"
# session environment: read at the next login, so niri and its spawns carry what
# env.lua gives a Hyprland session
mkdir -p "$cfg/environment.d"; cp -a "$here/environment.d/." "$cfg/environment.d/"
# dev deploy runs the daemon from ~/.local/bin; the package ships /usr/bin.
sed -i -e "s|^ExecStart=.*|ExecStart=$bindir/ryoku-shell daemon|" \
  -e "s|^ExecStartPre=.*|ExecStartPre=-$bindir/ryoku-shell quit|" "$cfg/systemd/user/ryoku-shell.service"
# ryogami.service ships ExecStart=/usr/bin/ryogami (the package path); point the
# dev-deployed unit at ~/.local/bin, mirroring the ryoku-shell rewrite above,
# and at the staged wall-ui QML (the unit file is re-copied every deploy, so the
# injected line never stacks).
sed -i -e "s|^ExecStart=.*|ExecStart=$bindir/ryogami|" \
  -e "/^\[Service\]/a Environment=RYOGAMI_SHELL_QML=$datadir/ryogami/wall-ui/shell.qml" \
  "$cfg/systemd/user/ryogami.service"
systemctl --user daemon-reload 2>/dev/null || true
# daemon-reload only re-reads the unit; it never restarts a running service, so
# without this the freshly built ryogami binary sits on disk while the old
# daemon keeps running until the next logout ("ran ryoku update, nothing
# changed"). try-restart cycles it only when it is already up, so a pre-session
# install deploy does not start it early; the restart relaunches the resident
# wall-ui picker too.
systemctl --user try-restart ryogami.service 2>/dev/null || true
# ryoku-ai-usage.service ships three ExecStart=-/usr/bin/<collector> lines (the
# package path); rewrite them to ~/.local/bin so the dev-deployed collectors
# resolve, mirroring the ryoku-shell.service rewrite above.
sed -i "s|^ExecStart=-/usr/bin/|ExecStart=-$bindir/|" "$cfg/systemd/user/ryoku-ai-usage.service"
systemctl --user daemon-reload 2>/dev/null || true
systemctl --user enable --now ryoku-ai-usage.timer 2>/dev/null || true
# pip (PEP 668 --user): Ryoku-owned, so a dev box tracks it the way the package
# materializes it for an installed one.
mkdir -p "$cfg/pip"; cp -a "$here/../apps/pip/pip.conf" "$cfg/pip/pip.conf"
# Default apps go to the vendor layer the package uses, never to
# ~/.config/mimeapps.list: that file is the user's own ("Set as default" writes
# it) and a redeploy must not touch it. Needs root, so it is skipped cleanly in a
# sudo-less env, and cmp keeps a redeploy a no-op.
if command -v sudo >/dev/null 2>&1; then
  cmp -s "$here/../apps/mimeapps.list" /usr/share/applications/mimeapps.list ||
    sudo install -Dm644 "$here/../apps/mimeapps.list" /usr/share/applications/mimeapps.list || true
fi
# chromium reads ~/.config/chromium-flags.conf, Google Chrome reads chrome-flags.conf;
# lay the one source to both (GNOME keyring password store + native Wayland).
cp -a "$here/../apps/chromium-flags.conf" "$cfg/chromium-flags.conf"
cp -a "$here/../apps/chromium-flags.conf" "$cfg/chrome-flags.conf"
# the screen-share source chooser xdph launches (hypr/xdph.conf names it). Its
# stylesheet is matugen's, rendered to ~/.cache/ryoku/share-picker.css.
mkdir -p "$cfg/hyprland-preview-share-picker"
cp -a "$here/../apps/hyprland-preview-share-picker/config.yaml" \
  "$cfg/hyprland-preview-share-picker/config.yaml"
# Refresh the icon cache only when the theme has an index.theme; the user-overlay
# hicolor dir usually has none, and gtk-update-icon-cache -f on an index-less dir
# writes an EMPTY cache that Qt then trusts, hiding every icon in it. With no
# cache, Qt/GTK scan the dir directly (correct), so drop any stale one instead.
_iconroot="${XDG_DATA_HOME:-$HOME/.local/share}/icons/hicolor"
if [[ -f "$_iconroot/index.theme" ]] && command -v gtk-update-icon-cache >/dev/null 2>&1; then
  gtk-update-icon-cache -qtf "$_iconroot" 2>/dev/null || true
else
  rm -f "$_iconroot/icon-theme.cache" 2>/dev/null || true
fi
command -v systemctl >/dev/null 2>&1 && systemctl --user daemon-reload 2>/dev/null || true

# Re-emit settings.lua from hypr.json through the freshly built ryoku-hub, so a
# genLua change (like the compositor-plugin load path above) reaches an existing
# box on `ryoku update` with no manual Hub save. Derived from hypr.json (the
# editable truth), so idempotent; guarded, since a box may have no overrides yet.
# Runs before overlay_user_edits so a user_edits/hypr/settings.lua still wins.
"$bindir/ryoku-hub" desktop get >/dev/null 2>&1 || true

# User overrides win over the base just laid, for hypr and every other surface.
overlay_user_edits
wireplumber_after=
[[ -f $wireplumber_policy ]] && wireplumber_after=$(<"$wireplumber_policy")
if (( reload )) && [[ $wireplumber_before != "$wireplumber_after" ]]; then
  if systemctl --user try-restart wireplumber.service 2>/dev/null; then
    say "restarted WirePlumber for updated Bluetooth audio policy"
  fi
fi


if (( wm_live && reload )); then
  # One clean reload (which also restores auto-reload), then restart the shell
  # daemon so a changed binary and changed QML both take effect.
  "$bindir/ryoku-wm-hyprland" act config.reload >/dev/null 2>&1 || true
  restart_shell
  say "deployed and reloaded the compositor."
else
  # Staged: leave auto-reload paused so the running session keeps its current
  # config until the next login, which loads the new one and fires the autostart.
  say "staged. log out and back in to activate (autostart launches the daemon)."
fi
