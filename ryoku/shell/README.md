# ryoku/shell/

The Ryoku desktop shell: the bar, panels, launcher, and screenshot tool that run
on top of the Hyprland config in `ryoku/hyprland`. It ships in the `ryoku-desktop`
package as the base config under `/usr/share/ryoku/config`, which
`ryoku materialize` copies into `~/.config`.

## Layout

- `ipc/` The control plane: one Go program, `ryoku-shell`. As `ryoku-shell daemon`
  it supervises the Quickshell components, starts the clipboard helpers (the
  selection watcher and the selection keeper) and the wallpaper helpers, and
  listens on a single Unix socket. As `ryoku-shell <command>` it is a
  thin client that forwards a command to that socket; Hyprland keybinds use it.
- `quickshell/` The hand-written QML UI: `pill` (the four-edge frame bars,
  screen frame, bounded menu manager, power menu, and preserved frame
  surfaces), `launcher`, `overview`, `ryoshot`, `visualizer`,
  `welcome`, and `widgets` (the wallpaper clock plus enabled third-party
  widgets). These render the shell; they hold no daemon logic.
  `pill/Singletons/Config` and `visualizer/Singletons/Config` watch
  `~/.config/ryoku/shell.json` and `~/.config/ryoku/visualizer.json`, so Ryoku
  Settings retunes their live appearance without a reload. The shell store owns
  frame-bar, type, frame-surface, and weather-data settings.
  `widgets/Singletons/Config` watches `~/.config/ryoku/widgets.json` for the
  clock design, size, shape and placement.
- `plugin/` `Ryoku.Blobs`, the packaged C++/QML SDF module used by the frame and
  active edge surfaces. `build.sh` builds it onto a QML import path. See
  `docs/frame.md`.
- `matugen/` Palette generation from the current wallpaper (the kitty palette, the
  Hyprland colors, and the shell visualiser palette at `~/.cache/ryoku/colors.json`).
- `qt6ct/` The Qt platform theme config (`qt6ct.conf`): the icon theme
  (`Papirus-Dark`) and Fusion style for Qt apps. GTK apps are themed by the
  Hyprland autostart (`gsettings color-scheme`), not a shipped file.
- `systemd/` The user session target.

The Hyprland config that hosts this shell lives at `ryoku/hyprland`; its
`scripts/` holds the clipboard and wallpaper thumbnailers the UI calls directly.
Its autostart also runs `ryoku-idle start`, which enables dim/lock/display-off/
suspend timeouts only on detected laptops.

## The IPC

Everything that controls the shell goes through `ryoku-shell`, so there is one
socket and one place that knows how to talk to the components:

| Command | Effect |
|---|---|
| `ryoku-shell daemon` | supervise the persistent components, clipboard history and wallpaper workers, then serve the socket |
| `launcher`, `power` | toggle the launcher or power surface on the active monitor |
| `bar <id>` | open a finite frame-bar menu or surface on the active monitor |
| `overview` | open the workspace overview on the active monitor |
| `lock` | lock the screen with qylock |
| `wallpaper [next\|init\|set <path>]` | change the wallpaper and retheme |
| `voice` | toggle Voxtype transcription and its live mic surface |
| `visualizer`, `visualizer-overlay` | toggle the desktop audio visualizer or its overlay mode |
| `reload`, `status`, `ping`, `quit` | manage the daemon |

The daemon resolves the active monitor itself, so the client and the keybinds stay
dumb. Build it with `go build` in `ipc/`; the binary belongs on `PATH` as
`ryoku-shell`.

## Dependencies

Beyond Hyprland, quickshell, `go` (to build `ryoku-shell`), and cmake + ninja +
qt6-shadertools (to build the `Ryoku.Blobs` plugin), the shell calls at
LED color), `wl-clipboard` (clipboard history and capture copy), `wl-clip-persist`
(keeps the selection when the app that copied closes), `imagemagick`
(wallpaper thumbnails), `hyprpicker`, `hypridle` and `brightnessctl` (laptop
idle/dim), `upower` (battery state), `wireplumber` (`wpctl`), `pipewire-pulse`
(`pactl` voice-call state and mic source), `cava` (music, mic, and desktop visualizers), `playerctl` (media keys),
`jq`, `glib2` (`gio`), `curl` (weather and LocalSend), and `python`/`openssl`/
`libnotify`/`xdg-utils` (the LocalSend file stash and opening stashed files).
The frame-surface tools use `grim`/`slurp`, `hyprpicker`, `curl`/`jq`, `mpv`,
`tesseract`, `zbar`, `gpu-screen-recorder`/`wf-recorder`, and `hyprsunset`.
The ``Super+` `` voice dictation drives `voxtype` (optional, from `voxtype-bin`)
for the transcription and `wtype` to type it into the focused app; pick the
engine and model in Ryoku Settings' Dictation page.
The keybinds open `kitty` (terminal) and `nautilus` (files). Fonts: JetBrains
Mono Nerd and Noto; cursor: Bibata. The lock is qylock, from `ryoku/`.

## Develop it live

Run the shell straight from this checkout on a running Hyprland session, no
install required:

    ryoku/shell/dev-run.sh       # build ryoku-shell, then run it with RYOKU_SHELL_DIR set
    ryoku/hyprland/dev-binds.sh on  # optional: bind the shell keys for this session
    ryoku/shell/dev-stop.sh      # stop it (restore your keys with: ryoku wm act config.reload)
    ryoku deploy                # build + materialize this checkout into ~/.config, then reload

The daemon launches each component with `qs -p`, so your own `~/.config` is never
touched, and quickshell hot-reloads QML edits, so changes show as you save.
On an installed system, `ryoku update` is the real system update: a snapper
pre-snapshot, `pacman -Syu` plus the AUR, a config materialize, a shell reload,
then a post-snapshot. `ryoku deploy` is the dev-only path that builds the Go
binaries and the plugin and materializes from a checkout. Both leave user files
(`hypr/user.lua`, `fish/user.fish`) untouched.

## Install

This tree ships in the `[ryoku]` packages: `ryoku-shell` builds the daemon to
`/usr/bin`, `ryoku-blobs` installs the `Ryoku.Blobs` plugin onto the QML import
path (`ryoku-shell` points `QML2_IMPORT_PATH` there for the components it
supervises), and `ryoku-desktop` lays the QML and configs under
`/usr/share/ryoku/config` for `ryoku materialize` to copy into `~/.config`. The
lock screen is qylock, shipped by `ryoku/lockscreen`; the shell does not replace it.
