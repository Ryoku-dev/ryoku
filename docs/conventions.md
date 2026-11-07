# Conventions

How code and configuration are written in this repo. When in doubt, match the
file next to the one you are editing.

## Hyprland is Lua

`ryoku/hyprland/` is authored in Lua, never as a raw `hyprland.conf`.

- `hyprland.lua` is the entry point and `require`s each module by name.
- `modules/` holds one concern per file: keybinds in `binds.lua`, motion in
  `animations.lua`, window rules in `window_rules.lua`, session startup in
  `autostart.lua`, and so on. A new concern is a new module plus one `require`.
- Use the `hl` API the modules already use (`hl.exec_cmd`, `hl.on`, the keyword
  setters). Do not shell out to `hyprctl` from Lua when an `hl` call exists.
- `keyboard.lua`, `gpu.lua`, `monitors.lua` are seeds the hardware helpers
  manage. Keep them generic; never hardcode one machine's monitors or layout.

## QML is one component per file

`ryoku/shell/quickshell/` is the UI. Each surface (`pill`, `launcher`, `ryoshot`)
is its own directory, and each component is a single
`.qml` file. Compose components; do not merge unrelated UI into one file.

View and logic stay apart: QML renders and animates, `ryoku-shell` (Go, in
`ryoku/shell/ipc/`) decides. The UI asks the daemon over its socket; it does not
embed policy.

## Ryoku code never keys behaviour on a store item

Subshells and bar styles are items: the shipped Orpheus and Kairos under
`ryoku/shell/quickshell/shell/subshells/`, everything else in the catalogue repo.
No Ryoku file imports item code, and no behaviour is keyed on a store item's id.
The shell, the Hub and the daemon read what an item is, owns and provides from
`shells.json` (`Shells` in QML), so a new item needs no Ryoku change. Settings
namespaces in `Config.qml` hold data only, and data migrations are the one
exception. See `docs/subshells.md`.

## Launching a process goes through Spawn

Anything a surface starts that can end up running Quickshell (an app, a desktop
entry, a terminal, a `qs` call, or a shell line that runs one) goes through
`Spawn` in `Ryoku.Ui.Singletons`, never bare `Quickshell.execDetached` and never
`DesktopEntry.execute()`. A Quickshell instance that has crashed once keeps
`__QUICKSHELL_CRASH_INFO_FD` in its environment, naming the memfd of the config
it came back from, and Quickshell reads that variable before it parses any
argument: a child that inherits it relaunches the desktop instead of starting, so
opening an app from the launcher drew a second bar and a second dock. `Spawn.run`
unsets the handle; a declarative `Process` takes the same map as
`environment: Spawn.env`.

Each Go program that runs `qs` (`ryoku-shell`, `ryoku`, `ryostore`) clears the
same variables from its own environment at startup, in its module's
`crashenv.go`. Three programs, three modules, one rule: the process that spawns
`qs` owns the hygiene, so no call site has to remember it.

## An image never decodes at its own size

An `Image` that sets `sourceSize` from its `width` must get that width from
somewhere other than the image: anchors, an explicit size, or a layout slot
that keeps it while it loads. With no outside size, `width` is the decoded
width, so `sourceSize: width * devicePixelRatio` asks for a bigger image on
every load at a fractional scale. A dock scoreboard did that with two team
logos and took the shell to 40 GiB in a minute. Use `StyledImage` (ii dock and
stage), which keeps its last request while the image sizes itself and caps it
at the window; anything else follows the same rule.

The `ryoku-shell` daemon is the backstop (`ipc/memwatch.go`): a supervised `qs`
whose resident plus swapped memory passes a quarter of RAM (never less than
3 GiB) is killed and respawned with a growing delay, and the daemon logs why to
the journal.

## System logic is a named helper

Anything with real shell logic is a `ryoku-<thing>` script in
`system/hardware/.../`, shipped to `/usr/bin` by the `ryoku-desktop` package and
invoked by name from Lua autostart or keybinds: `ryoku-gpu`, `ryoku-monitor`,
`ryoku-hw-laptop`, `ryoku-idle`, `ryoku-mic`. Do not inline multi-step shell
into Lua, and do not copy a helper's logic into a second place; if two callers
need it, it is one shared helper (as `ryoku-hw-laptop` is).

## Native config formats are expected

The Lua rule covers the *Hyprland* config only. Every third-party tool keeps its
own native format under its own directory: `kitty.conf`,
`qt6ct.conf`, `yazi.toml`, `starship.toml`, `hypridle.conf`, `npmrc`, `pip.conf`.
That is correct, not a violation. Do not invent a Lua wrapper for a tool that
reads its own format.

## No duplication

Search before adding. There is one source for each config, script, value, and
fact. If a thing must exist in two places, extract it to one place and reference
it. Two copies will drift; treat a second copy as a bug.

## Comments

Comment the *why* when it is not obvious. Never narrate the *what*, never leave
commented-out code, never pad with filler. If a file reads as more explanation
than code, the code is too complicated; simplify it. The pre-commit hook rejects
filler comment lines.

## Naming

Helpers are `ryoku-<thing>`. Lua modules are named for their concern
(`binds.lua`). QML components are named for the component (`Pill.qml`). Names are
literal and lowercase-hyphenated for scripts.

## Commits and hooks

Run the hooks; never use `--no-verify`. Subjects are `[area] scope: imperative
summary`, where area is one of
`global | installation | system | ryoku | docs | test | tooling | release`
(shell changes use `[global]`). No em-dash, no authorship or attribution
trailer, no filler. Keep one logical change per commit. Details in
`docs/development.md`.
