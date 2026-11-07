---
name: ryoku
description: >
  Customize a Ryoku desktop: an Arch Linux system with a Hyprland or niri
  compositor and a Quickshell shell (the active subshell or bar style, the
  dock, widgets, the launcher, and the Hub). Use for end-user requests that
  touch the desktop or its config. Triggers: window-manager behavior, window
  rules, keybinds, monitors, gaps, borders, the bar, subshells, bar styles, the
  dock, bar widgets, plugins, themes, wallpaper, colours, night light, idle,
  lock screen, and user-facing ryoku commands (ryoku, ryoku-shell, ryoku-hub,
  ryogami, ryoku-rashin). Read the vault first; act through commands, not by
  editing shipped files.
---

# Ryoku

Ryoku is an Arch Linux desktop: a Hyprland or niri compositor (`ryoku wm
status` says which), a single Quickshell shell that draws the bar (through the
active subshell or bar style), the dock, the launcher, the popouts and the
widgets, and a set of Go command-line tools that
own the config. This skill is for changing a running Ryoku system on behalf of
its user. It is not for developing Ryoku itself (editing the source checkout,
writing migrations, cutting a release).

## When to use this skill

Use it whenever a request would change the desktop or read its state: the bar
and which subshell or bar style draws it, the dock, window-manager behaviour,
themes and wallpaper, keybinds, idle and lock, plugins, or any `~/.config` file
Ryoku owns. If you are
about to guess a path or edit a config file under `~/.config`, stop and use this
skill.

Do not use it to modify the Ryoku source tree, and never treat a shipped file as
a place to store a user's choice.

## Read the vault first

A maintained map of THIS machine lives in the Rashin vault at
`~/.local/share/ryoku/rashin/`. Read it before searching the filesystem or
guessing where anything lives:

- `AGENTS.md`: the entry contract and the vault's own rules.
- `desktop.md`: the map. Every subsystem, the config path that owns it, the
  binary that owns it, and how to reload it. Its generated "Bar and dock"
  section lists the installed subshells and bar styles (which one is active and
  how its settings open), QS Bar's widget catalogue while QS Bar is active, and
  the dock commands. Read this before touching the bar.
- `system.md`, `packages.md`, `user.md`, `habits.md`: hardware, packages, where
  this user diverges from the shipped defaults, and this user's directories and
  tool stack.

Ownership inside the vault: the generated maps (`AGENTS.md`, `desktop.md`,
`system.md`, `packages.md`, `ryoku-repo.md`, `user.md`, `habits.md`) are rewritten on
every reindex, so read them and never edit them. `memory/` and `journal/` are
yours: write durable notes and dated notes (`journal/YYYY-MM-DD.md`) there and
they survive. `user.md` lists the user's own choices; never revert one to a
shipped default without being asked.

When a user is new to Ryoku, cite the long-form wiki at
`~/.local/share/ryoku/rashin/wiki/`. It is shipped read-only; start at
`README.md`, then link the page that matches the question instead of assuming
Linux, QML, Go, or compositor knowledge.

Topic guides sit beside this file. Read the matching one first:

- [`gui.md`](gui.md): the GUI map. Every intent to its Ryoku Hub page, picker,
  Stage Editor section, or the active item's own settings, how to open it, and
  the command it wraps.
- [`bar.md`](bar.md): subshells, bar styles and the dock: what ships, what
  comes from Ryostore, how to switch (`ryoku-shell barstyle <id>`), QS Bar's
  `qsbar` IPC target, and `ryoku-shell dock`.
- [`plugins.md`](plugins.md): installing, listing, and removing shell plugins
  with `ryoku plugin`, and Ryostore.
- [`feature.md`](feature.md): the ladder for a feature the desktop does not
  have yet. Check the store, then the machine, then the built-ins; build a
  plugin only when all three come up empty.
- [`troubleshoot.md`](troubleshoot.md): the break/fix playbook. Gather the logs
  first (`ryoku-rashin logs <app>`), diagnose, fix through the owning command,
  verify, and know the rollback paths.
- [`wm.md`](wm.md): the window-manager seam, provider capabilities and actions,
  safe Hyprland Lua and niri KDL overrides, reload, and verification.
- [`build.md`](build.md): the feature ladder, shell plugin workflow, QML design
  rules, the neutral `Wm` singleton, and Go development in a checkout.

## Answer policy: GUI first

When a user asks HOW to change something, lead with the GUI path: the exact
keybind or the Ryoku Hub page. The Hub opens with Super+comma; deep-link one
page with `ryoku-shell hub open <section>` (the section key names the page, e.g.
`keybinds`, `gpu`, `lockscreen`). Give the command after, as the fallback or the
way to script the same change. When the user asks YOU to change it, act through
the command (the GUI is for the human), then say what you changed and where to
see or undo it in the GUI. Never hand back a bare shell command for a change that
has a page or a picker.

Two surfaces have no Hub page of their own. Wallpaper and theme are the shell
picker (Super+W, `ryogami wallpaper ui`); the dock is the Stage Editor
(Super+Shift+D, its Dock section). Which subshell or bar style draws the bar is
Ryoku Hub > Subshells and Bar Studio, and that item's own layout lives in its own
settings window (Orpheus: left-click its logo; QS Bar: QS Bar Settings), which
exists only while the item is active. See [`gui.md`](gui.md) for the full
intent-to-surface map.

## Safety rules

Ryoku separates the files it ships from the files you own, so an update can
refresh the base freely while your changes stand. Respect the split:

- **Ask `ryoku owner <path>` before you touch a file.** It classifies the path
  (a shipped `ryoku` file, a `generated` one, a tool `store`, a `seed`, a
  `user-override`, or the user's own) and prints where the change belongs. Never
  edit a path it calls `ryoku`, `generated`, or `store`; edit where it points.
- **Never edit a shipped file in place.** `/usr/share/ryoku/` (the packaged
  base) and the files Ryoku lays into `~/.config/quickshell/` are re-laid on
  every `ryoku update` (`ryoku materialize` clobbers every shipped file), so an
  edit there is lost on the next update. Reading them is safe and useful.
- **A user override goes to the overlay:** `~/.config/ryoku/user_edits/`, which
  mirrors `~/.config`. A file there wins at its mirrored path and survives every
  update. To change compositor config, use the dedicated last-loaded override
  (`hypr/user.lua` or `niri/user.kdl`) that `ryoku owner` names. A whole-file
  fork belongs at the mirrored path under `user_edits`; you own that file and
  stop receiving upstream changes to it. `ryoku reset <path>` drops an overlay
  file back to the base.
- **Prefer a command over a file edit.** The tool that owns a setting is its one
  writer; hand-editing its store drifts. Ryoku Settings' own state (bar, colours,
  launcher, device lighting) lives under `~/.config/ryoku/*.json`, written by
  their tools (the shell daemon, `ryoku-hub`, `ryogami`); do not hand-edit those
  JSON stores, drive them through the command or the GUI so one writer stays in
  charge.

## Command discovery

Ryoku's behaviour lives behind five command-line tools, all self-documenting.
Prefer a command to a file edit; read a command's `--help` before running it.

| Tool | Owns |
|---|---|
| `ryoku` | Updates, rollback, status, ownership, and the `wm` provider seam. See `docs/cli.md`. |
| `ryoku-shell` | The live shell: the active subshell or bar style (`barstyle`, `shells`), the dock, menus, popouts, and the `shell.json` settings store (the sole writer of `shell.json`). |
| `ryoku-hub` | Ryoku Settings and the window-manager settings applied through the active provider. |
| `ryogami` | Wallpapers and the colour palette (`ryogami wallpaper set|next|random`). |
| `ryoku-rashin` | The optional agent OS: Ask, Agent, Chat, the vault, harness wiring, `index`, and `terminal`. |

```bash
ryoku --help                              # the ryoku CLI surface
ryostore catalog --category subshells     # shipped and store subshells
ryostore catalog --category barstyles     # store bar styles
ryoku-shell dock list                     # the dock state
ryoku-shell ipc call qsbar catalog        # QS Bar's widgets, only while QS Bar is active
ryogami wallpaper --help
```

To find WHERE a setting is read (which QML file, which key), use `prowl`
inside the vault's read-only source mirror at
`~/.local/share/ryoku/rashin/source/`, which indexes the live `~/.config`:

```bash
cd ~/.local/share/ryoku/rashin/source && prowl search "barPosition"
cd ~/.local/share/ryoku/rashin/source && prowl find barShellStyle
```

The mirror is read-only and rebuilt on every reindex; never edit files in it,
edit the real path `desktop.md` names.

## Decision framework

When a request would change the system, in order:

1. **Is there a GUI for it?** A Ryoku Hub page (`ryoku-shell hub open <section>`,
   Super+comma), the wallpaper picker (Super+W), the Stage Editor
   (Super+Shift+D), or the active item's own settings. If a user asks how to do
   it, name the surface first; whether you or they drive it, the change still
   lands through the command that page wraps, so read on.
2. **Is there a command for it?** Use it. Switching the bar is
   `ryoku-shell barstyle <id>`, the dock is `ryoku-shell dock ...`, and QS Bar's
   layout is `ryoku-shell ipc call qsbar ...` while QS Bar is active (see
   `bar.md`); wallpaper has `ryogami wallpaper set`; updates have `ryoku update`.
3. **Is it a config edit with no command?** Ask `ryoku owner <path>`, then edit
   the override it names, never the shipped file. Use `wm.md` for the
   compositor-specific file, validation, reload, and proof.
4. **Is it a feature the desktop does not have?** Climb `feature.md`:
   `ryostore catalog` (does the store ship it, maybe as a subshell or bar
   style?), the active item's own widget list and `ryoku plugin list` (is it
   installed but hidden?), then build it as a plugin. A shell widget installs
   from git with
   `ryoku plugin add <url> --bar`, or from Ryostore; see `plugins.md`. Never
   run a plugin's code to install it. A Hyprland compositor plugin (title
   bars, cursor motion, key sounds, a `.so` the compositor loads) is managed
   by `ryoku-hub desktop plugins list|rebuild|add|remove` and Hub > Plugins;
   a "version mismatch" after an update means
   `ryoku-hub desktop plugins rebuild --stale`.
5. **Is it a theme or wallpaper?** Drive it through `ryogami` and `ryoku-hub`,
   which own the colour master; never write the palette or theme shadow by hand.
6. **Is it a package?** `ryoku update` for the whole system; pacman/yay for one
   package.
7. **Unsure a command exists?** Read the tool's `--help`, or `desktop.md`.

## Example requests

- "Switch to another bar" -> Ryoku Hub > Bar Studio
  (`ryoku-shell hub open bar-studio`), or install it with
  `ryostore install barstyles <id>` and run `ryoku-shell barstyle <id>`
- "Try a complete different shell" -> Ryoku Hub > Subshells
  (`ryoku-shell hub open subshells`), or `ryostore install subshells <id>` then
  `ryoku-shell barstyle <id>`
- "Open the bar settings" -> Bar Studio > OPEN SETTINGS for the active item, or
  its own click (Orpheus: left-click the 力 logo); scripted, the call desktop.md
  lists for it, e.g. `ryoku-shell ipc call orpheus settings ""`
- "Put the bar at the bottom" (QS Bar active) -> QS Bar Settings > Bar, or
  `ryoku-shell ipc call qsbar position bottom`
- "Move the clock to the right" (QS Bar active) -> QS Bar Settings > Layout, or
  `ryoku-shell ipc call qsbar move clock right ""`
- "Hide the GPU widget" (QS Bar active) -> QS Bar Settings > Widgets, or
  `ryoku-shell ipc call qsbar hide gpu`
- "Add a crypto price applet to the bar" -> climb `feature.md`:
  `ryostore catalog` (a store plugin may already do it), then the active item's
  widget list (a hidden built-in?), then `ryoku plugin new` and the plugin
  contract in `plugins.md`. Never start at "write a widget".
- "Turn the dock off" -> Stage Editor > Dock (Super+Shift+D), or
  `ryoku-shell dock hide`
- "Pin Firefox to the dock" -> Stage Editor > Dock, or
  `ryoku-shell dock pin firefox`
- "Change my wallpaper" -> press Super+W and pick one (the wallpaper picker);
  scripted: `ryogami wallpaper set <path>`
- "Remap a key" -> Ryoku Hub > Keybinds (Super+comma, `ryoku-shell hub open keybinds`)
- "Lock after ten minutes" -> Ryoku Hub > Graphics & Power > Idle
  (`ryoku-shell hub open gpu`); scripted, `ryoku-power idle set ac.lockSec 600`
  (and `battery.lockSec`), then `ryoku-idle apply` re-renders and restarts the
  idle daemon
- "Add a weather plugin from GitHub" -> `ryoku plugin add <git-url> --bar`
- "List my installed plugins" -> Ryoku Hub > Add-ons, or `ryoku plugin list`
- "Update the system" -> Ryoku Hub > Updates, or `ryoku update`
- "Roll back a bad update" -> `ryoku rollback`
