# Agent guide for the {{ID}} plugin

This folder is a Ryoku shell plugin. If you are an AI agent editing it, follow
every rule below. They are the same rules `ryoku plugin validate` enforces, so
work that breaks one will not install. This file is self-contained: you do not
need any other document to work here.

## The rules

R1  Place. A plugin is one folder named after its id. Author it under
`$(xdg-user-dir DOCUMENTS)/ryoku-plugins/<id>/` (fallback `~/ryoku-plugins/<id>/`);
`ryoku plugin new <id>` creates it there. Never write into
`~/.local/share/ryoku/plugins/` (the store's install root, receipt-owned) or
`~/.config/quickshell/` (the shipped shell).

R2  Shape. `manifest.json` at the root; `service/Main.qml` (logic, no UI);
`content/Widget.qml` (the one view); optional `content/Panel.qml` (a bar panel);
`README.md`; `LICENSE`; `assets/preview-widget.png` (a real capture); scripts
only under `bin/` with a shebang and the exec bit. Every extra file listed in
manifest `files`.

R3  Id. Lowercase `[a-z0-9][a-z0-9-]*`, unique, not a built-in widget id.

R4  Imports. Only `QtQuick*`, `Quickshell*`, `Ryoku.PluginKit`,
`Ryoku.PluginKit.Singletons`, and files inside the plugin folder. Never
`shell.*`, `Ryoku.Ui*` internals, or a relative import that climbs out of the
folder.

R5  Settings. Declared in `metadata.settings` (types the bar renders: toggle,
choice, multi, int, text); read through `pluginApi.pluginSettings` behind a
default; written only through `pluginApi.saveSetting(key, value)`. Never edit
`shell.json` or `plugins.json` directly.

R6  Commands. Every external program the plugin runs is either in its own `bin/`
or listed in `dependencies.commands`. No `sudo`, `doas`, `su`. A privileged
action is allowed only through `pkexec`, only on an explicit click, only when
listed in manifest `capabilities.privileged` (exact command strings) and
explained in README.

R7  Network. Every host the plugin talks to is listed in
`capabilities.network`. No `curl … | sh`, no downloading and running code.

R8  Files. Write only under `pluginApi.stateDir`
(`$XDG_STATE_HOME/ryoku/plugins/<id>`), `$XDG_CACHE_HOME/ryoku/plugins/<id>`, or
a temp dir. Never touch `~/.ssh`, `/etc`, shell rc files, or another plugin's
folder.

R9  Shell. No `sh -c` with a string built from settings or program output
(injection). Pass argv arrays.

R10 Secrets and binaries. No tokens, keys or credentials in the tree; no
compiled binaries (ELF/Mach-O/.so); scripts only.

R11 Honesty. `official` is never true for a community plugin. `author` is
`Name <mail>`. README says what runs, what it reads, what it writes, and every
privileged/network capability.

## The workflow

1. `ryoku plugin new <id> [--bar|--desktop|--popout|--sidebar]` scaffolds this
   folder (already done). This scaffold selected `{{HOST}}`.
2. Edit `service/Main.qml` for logic and `content/Widget.qml` for the view.
   Edit `content/Panel.qml` only for a bar plugin. Capture
   `assets/preview-widget.png`.
3. `ryoku plugin validate <dir>` and fix every blocking finding (warnings are
   advisory but read them).
4. `ryoku plugin add <dir> --yes`, then enable and place it under
   **Ryoku Settings > Add-ons**. For a bar plugin, `--bar` enables it on the
   current bar immediately.
5. `ryoku plugin share <id>` ONLY when the user asks to publish. Never share a
   plugin unless asked.

## What the host sets

Ryoku ships four hosts: `topbarGlyph`, `desktopWidget`, `framePopout`, and
`sidebarCard`. They all mount `content/Widget.qml`; only a bar plugin may add
`content/Panel.qml`.

The bar, desktop, and popout hosts set `pluginApi`, `s`, `active`, `density`, and
`widthBudget`. Read them; never assign them.

For `sidebarCard`, the widget root is an `Item` with
`pragma ComponentBehavior: Bound`. The host sets `s`, `open`, `reveal`,
`tabActive`, `width`, and `pluginApi`; the card reports `implicitHeight` and
declares `signal requestClose()`. Read host-set values and never assign them.
There is no `content/Sidebar.qml`. Ryoku draws the card plate and owns its
position, size, motion, and chrome.

`pluginApi` exposes:

- `mainInstance`: the live `service/Main.qml` instance every view shares.
- `pluginSettings`: the resolved settings object; read behind a default.
- `pluginDir`: this plugin's folder.
- `stateDir`: `$XDG_STATE_HOME/ryoku/plugins/<id>`; the only place to write.
- `saveSetting(key, value)`: persist one setting.
- `panelOpen` (bool), `openPanel()`, `closePanel()`, `togglePanel()`: the bar
  panel controls (present but inert off the bar). The sanctioned bar-widget
  click is `onClicked: pluginApi.togglePanel()`.
