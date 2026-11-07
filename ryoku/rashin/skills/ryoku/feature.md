# A feature that does not exist yet

The user asks for something the desktop does not do: "add a crypto price applet
to the bar", "I want a clipboard history widget", "put a rainfall gauge on
the desktop". The mistake is to start writing code. The right order is a ladder:
check the store, check the machine, check the built-ins, and only then build.
Each rung is one command, and each answer changes what you do next.

## The ladder, in order

### 1. Does Ryostore already ship it?

```bash
ryostore catalog            # every category with counts and installedCount
```

The catalog is one JSON document: categories (plugins, subshells, bar styles,
rices, themes, lockscreens, widgets), each with an `installedCount`. Search its
plugin and widget categories for the thing asked for, and the subshells and bar
styles when the ask is a whole bar or shell look
(`ryostore catalog --category subshells`, `--category barstyles`). If it exists:

```bash
ryostore install plugins <id>    # or the Hub: Add-ons page (Super+comma > Add-ons)
ryostore install barstyles <id>  # a bar style; subshells the same way
```

Say where it landed (the bar, the desktop, QS Bar Settings > Community) and
stop. A subshell or bar style still needs selecting: Ryoku Hub > Bar Studio, or
`ryoku-shell barstyle <id>`. Never rebuild what the store delivers.

### 2. Is it already on this machine, just hidden?

```bash
ryoku plugin list                    # installed plugins and their host and enabled state
ryoku-shell ipc call qsbar catalog   # QS Bar's widgets, builtin or plugin, with settings
ryoku-shell ipc call qsbar list      # QS Bar's live layout, per section, with shown state
```

The two `qsbar` calls answer only while QS Bar is the active bar style; under
another item, check its own settings window (Orpheus: the Layout page lists its
optional modules). A QS Bar widget that exists but is hidden needs
`ryoku-shell ipc call qsbar show <id>`, not code. A plugin installed but not
placed needs `ryoku plugin` or QS Bar Settings. Check before you build; half of
"missing" features are switched off.

### 3. Can the built-ins compose it?

The active bar's built-in set covers system state (on QS Bar: cpu, memory,
volume, battery, clock, workspaces, launcher, status; Orpheus has its own
module list, tray included). Some asks decompose: a "system monitor applet" is three
built-ins plus a panel. Read `bar.md` for the shell items and `plugins.md` for
what a plugin can add on top.

### 4. Build it, as a plugin, the Ryoku way

Nothing exists. Now you write code, and you write it as a shell plugin, because
that is the only supported way to add a widget: the receipt system owns
placement, the validator owns safety, and Ryostore owns distribution. Follow
`plugins.md` end to end; the short version:

```bash
ryoku plugin new <id> --bar --name "..." --author "Name <mail>"
# edit service/Main.qml, content/Widget.qml, content/Panel.qml, manifest.json
ryoku plugin validate <dir>
ryoku plugin add <dir> --bar --yes
```

Author it under `~/Documents/ryoku-plugins/<id>/` (R1). Never write into
`~/.local/share/ryoku/plugins/` or `~/.config/quickshell/`.

### 5. Learn the host's own patterns first (prowl)

Before you invent a settings row or a poll timer, see how the shipped widgets
and installed plugins do it. The vault's source mirror is indexed for exactly
this question:

```bash
cd ~/.local/share/ryoku/rashin/source
prowl search "poll interval bar widget"
prowl find <name> && prowl def <id>
prowl references <id>
```

On a dev box with the checkout indexed (`prowl init` in the repo), run the same
queries there for the real sources. Structural answers come from the index in
one call; grep only for exact strings.

## What the ladder never does

- It never edits a shipped file to add a feature (the overlay is for user
  choices, not for new code; new code is a plugin).
- It never installs from a random git URL without reading the manifest first
  (`ryoku plugin add` validates, but you still read what you are adding).
- It never runs `ryoku plugin share` unless the user asks to publish.
- It never claims a feature is impossible before running steps 1 and 2.

## When the ask is bigger than a widget

Some asks are not widgets: a new keybind (Hub > Keybinds), a Hyprland behavior
(`ryoku-hub hypr get`, or the override files in `SKILL.md`), a compositor
plugin (`.so`; `ryoku-hub desktop plugins`, see SKILL.md step 4). The ladder
applies to shell features; route system-level asks through the decision
framework in `SKILL.md`.
