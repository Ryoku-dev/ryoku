# {{NAME}}

A Ryoku shell plugin (`{{ID}}`) scaffolded for the `{{HOST}}` host. The working
demo is a counter that ticks once a second; edit it into your own plugin.

## What it does

- **Service** (`service/Main.qml`): the logic, no UI. Holds the live state the
  view reads through `pluginApi.mainInstance`.
- **Widget** (`content/Widget.qml`): the one view every host mounts.
- **Panel** (`content/Panel.qml`): present only in a `--bar` scaffold; the bar
  opens it beneath the glyph.

Ryoku ships four hosts: `topbarGlyph`, `desktopWidget`, `framePopout`, and
`sidebarCard`. A sidebar card still uses `content/Widget.qml` plus
`service/Main.qml`; it does not have a separate sidebar entry point. Its root
`Item` exposes `s`, `open`, `reveal`, `tabActive`, `pluginApi`, and
`requestClose()`. Ryoku owns its `width`, and the item reports only its
`implicitHeight`.

## What it reads and writes

The demo reads nothing off the machine and writes nothing. When you add real
behaviour, keep to the rules in `AGENTS.md`: read settings through
`pluginApi.pluginSettings` behind a default, write them only through
`pluginApi.saveSetting(key, value)`, and write files only under
`pluginApi.stateDir`. Every external command belongs in `bin/` or in
`dependencies.commands`; every host you contact belongs in
`capabilities.network`; a privileged action runs only through `pkexec` listed in
`capabilities.privileged`.

## Settings

| key       | type   | default | description         |
| --------- | ------ | ------- | ------------------- |
| showCount | toggle | true    | Show the tick count |

## Preview

Capture a real screenshot of the widget and save it as
`assets/preview-widget.png`, then list it under `files` in `manifest.json`. The
store shows it in the catalogue.

## Build, check, install

```
ryoku plugin validate .
ryoku plugin add . --yes
```

Enable and place it under **Ryoku Settings > Add-ons**. A bar plugin can instead
use `ryoku plugin add . --bar --yes` to enable it on the current bar immediately.

## Author

{{AUTHOR}}: this plugin is community-made (`official` is false).
