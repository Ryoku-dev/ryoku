# apps/

The applications Ryoku ships and their settings, one folder per app. Each folder
maps to a place under `~/.config` (except the small helper script noted below).

## What's here

- `kitty/` The terminal. JetBrains Mono Nerd Font, a beam cursor, and fish as the
  shell. `kitty.conf` includes `current-theme.conf`, which carries the Ryoku dark
  palette (background `#171717`, foreground `#CCD0CF`, accent `#F25623`).
- `fastfetch/` The branded system readout. `config.jsonc` owns its emblem,
  content and fixed colours. `ryoku-fastfetch` can merge the Matugen palette
  sibling into a cached effective config, leaving the user's file untouched.
- `fish/` The shell. The greeting is turned off so the login terminal stays
  clean, then it runs `ryoku-fastfetch` and wires up starship, zoxide, fzf, and a
  few eza listing aliases (each guarded so a missing tool is harmless).
- `starship/` The prompt. Five layouts share one active `starship.toml`: Pill,
  Minimal, Two line, Powerline and Lean. The Hub chooses the layout and whether
  its named palette stays fixed or follows the wallpaper.
- `nvim/` The editor (LazyVim seed) plus `ryoku-nvim.desktop`, which registers
  neovim as the default text handler.
- `yazi/` The terminal file manager (`yazi.toml`).
- `npm/` (`npmrc`) and `pip/` (`pip.conf`) keep each package manager writing under
  the home, so neither needs root.
- `nautilus/` The graphical file manager. Ships one `nautilus-python` extension,
  the Ryoku stash actions on the right-click menu (install, compress, LocalSend);
  see its README. No dconf settings to ship.
- `vesktop/` Discord, for Vesktop and Equibop (which keeps the same layout).
  `themes/ryoku-discord.theme.css` is Ron's Ryoku theme, laid beside Matugen's
  Midnight (`matugen.css`). `settings/settings.json` enables it and is seeded only
  into a profile the client has not written yet; `ryoku doctor` switches an
  older profile once, and copies the theme into the Flathub Vesktop's own
  folder, which its sandbox keeps apart from `~/.config` (see
  `../cli/internal/doctor/reconcile_discord_theme.go`).
- `mimeapps.list` The default-application map (text files route to neovim).

## GUI apps

These are full applications, not `~/.config` seeds. A Quickshell app ships its
`quickshell/` tree as `qs -c <name>`; a compiled app builds from a
`CMakeLists.txt` to `/usr/bin/<name>`.

The Rashin app (`rashin-app`) is not here: it builds from
`ryoku/rashin/cmd/rashin-app` and ships with `ryoku-rashin`, so a box without
Rashin has no Rashin window either.

The music app lives in [Ryotunes](https://github.com/ryoku-dev/ryotunes), with a
native Quickshell client and libmpv daemon. `release/repo/import-ryotunes.sh`
imports its checksummed release package unchanged and signs it for `[ryoku]`;
the distro does not rebuild a separate music app.

Ryoport lives in [its own repository](https://github.com/ryoku-dev/ryoport)
and ships as the optional `ryoport` package. Run `ryoport` or press
Super+Shift+V to open the fleet dashboard. The package also provides the
compatible `ryovm`, `ryossh`, `ryovm-mon` and `ryovm-fetch` commands.

A shell *surface* is a fourth thing and does not live here. `ryoshot` and
`welcome` launch the same single-instance way but ship inside the shell at
`ryoku/shell/quickshell/<name>/`, with no `.desktop` and no launcher entry.

- Wallpaper browsing, grading, sources and the theme surface now live in the
  shell's wallpaper picker (Super+W), not here; ryowalls was sunset.
- `ryostore/` The store: discover and install lockscreens, rices, subshells, bar styles,
  plugins and bundles. Engine: the `ryostore` Go backend.

## Single-instance launch

Quickshell applications here launch behind a lock:

```
flock -n -o /tmp/<name>.lock qs -c <name>
```

`-n` fails immediately while the lock is held, so a second launch dies instead of
opening a duplicate window. `-o` closes the descriptor before exec, so the lock
follows the app rather than the wrapper.

A keybind needs more than that. Because `flock` exits nonzero on a held lock,
pressing the key while the app already runs on another workspace does nothing
visible: the window stays where it first opened, and the key reads as dead.
`ryoku-summon` handles that case:

```
ryoku-summon <window-title> <launch command...>
ryoku-summon --app-id <app-id> <launch command...>
```

It moves a matching window to the active workspace and focuses it, and runs the
launch command only when nothing matches. Quickshell surfaces match their title;
native applications use their stable app ID. The script ships from
`ryoku/shell/scripts/` and lands on `PATH`.

## Adding one

`package()` in `release/packages/ryoku-desktop/PKGBUILD` walks `ryoku/apps/*/`,
so dropping a directory in ships it and there is nothing to register. The
directory name is the identity: it becomes the config name, the icon name, and
the binary name.

| Path                | Ships as                                                     |
| ------------------- | ------------------------------------------------------------ |
| `quickshell/`       | the package stages it under `/usr/share/ryoku/config/quickshell/<name>/` and `ryoku materialize` lays it at `~/.config/quickshell/<name>/`, where it runs as `qs -c <name>` |
| `quickshell/logo.svg` | the launcher icon, installed as `<name>.svg`                |
| `<name>.desktop`    | `/usr/share/applications/`, carrying the `flock` line         |
| `bin/*`             | one `/usr/bin` entry per file                                 |
| `<helper>/go.mod`   | a Go helper, built to the binary its `module` line names      |
| `CMakeLists.txt`    | a compiled Qt app at `/usr/bin/<name>`                        |

The loop only looks at a directory holding `quickshell/` or `CMakeLists.txt`.
Anything else is invisible to it and needs its own install lines.

Two things to know before testing by hand. `~/.config/quickshell` is wholly
Ryoku-owned: `ryoku materialize` converges it against the shipped tree and
deletes whatever the repo does not ship, so an app dropped there by hand
disappears on the next update. And a keybind belongs in
`ryoku/hyprland/modules/binds.lua`, while a window that should float wants a rule
in `window_rules.lua` (see `float-ryostore`).

## Install paths

| Folder          | Destination                               |
| --------------- | ----------------------------------------- |
| `kitty/`        | `~/.config/kitty/`                        |
| `fastfetch/`    | `~/.config/fastfetch/` (user config + generated palette); wrapper on `PATH` |
| `fish/`         | `~/.config/fish/config.fish`              |
| `starship/`     | active `~/.config/starship.toml`; catalogue under `~/.config/starship/` |
| `nvim/`         | `~/.config/nvim/`                         |
| `yazi/`         | `~/.config/yazi/`                         |
| `npm/`          | `~/.npmrc`                                 |
| `pip/`          | `~/.config/pip/pip.conf`                  |
| `vesktop/`      | `~/.config/vesktop/` and `~/.config/equibop/`: the theme under `themes/`, the Vencord settings seed at `settings/settings.json` |
| `mimeapps.list` | `~/.config/mimeapps.list`                 |

The `ryoku-fastfetch` wrapper must also land on `PATH` (for example
`~/.local/bin/ryoku-fastfetch`) so fish can call it on terminal start. It draws
the emblem at `~/.config/fastfetch/fastfetch-emblem.png`, laid there beside
`config.jsonc` by `ryoku materialize` (and the package that ships it).
