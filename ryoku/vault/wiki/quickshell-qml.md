# Quickshell and QML

Quickshell is the QML runtime that draws Ryoku's shell. The safest first project
is a shell plugin: it has a small public API, validates before installation, and
cannot replace the shipped shell by accident.

## Scaffold a working widget

Start with a bar plugin:

```bash
ryoku plugin new hello-ryoku --bar --name "Hello Ryoku" --author "Name <mail>"
```

The command prints the authoring directory and next steps. By default it uses
your XDG Documents directory under `ryoku-plugins/`, with
`~/ryoku-plugins/` as the fallback. Author there, never under
`~/.local/share/ryoku/plugins/` or `~/.config/quickshell/`.

The scaffold already runs as a small counter:

- `manifest.json` describes the id, author, version, host, entry points,
  dependencies, capabilities, defaults, and settings.
- `service/Main.qml` owns persistent logic and state. It draws nothing.
- `content/Widget.qml` is the one view mounted on the bar.
- `content/Panel.qml` is the panel opened from the bar glyph.
- `AGENTS.md` records the plugin safety contract.
- `README.md`, `LICENSE`, and `assets/` carry the human explanation and preview.

Keep one component per QML file. If a view grows a reusable row, put that row in
its own file and add it to the manifest's `files` list.

## Follow Ryoku's visual language

Ryoku's own QML imports `Ryoku.Ui.Singletons` and reads `Tokens` for colour,
type, spacing, geometry, and motion. A plugin deliberately has a narrower API:
it imports `Ryoku.PluginKit` or `Ryoku.PluginKit.Singletons` and reads `Theme`
and `Motion`, which resolve the active Ryoku scheme.

Never hardcode a hex colour, font family, corner radius, or animation duration.
Use warm ink on paper, hairline borders instead of decorative shadows, inversion
for emphasis, and colour only when it carries data. Animate as a short response
to an action or state change, and stop timers or other live work while the host
marks the view inactive.

A simple themed label follows the scaffold's public API:

```qml
import QtQuick
import Ryoku.PluginKit.Singletons

Text {
    text: "Hello, Ryoku"
    color: Theme.bright
    font.family: Theme.font
}
```

The host owns the card, panel, placement, drag behavior, and outer motion. Your
view reports `implicitWidth` and `implicitHeight`, reads `density`,
`widthBudget`, `s`, `active`, and `pluginApi`, and never positions its own
window. Settings belong in `manifest.json`; read them through
`pluginApi.pluginSettings` behind defaults and save them with
`pluginApi.saveSetting`.

## Validate, run, and inspect it

Replace `<dir>` with the directory printed by the scaffold:

```bash
ryoku plugin validate <dir>
ryoku plugin add <dir> --bar --yes
ryoku plugin list
```

Validation checks the manifest and audits imports, commands, network hosts,
privilege, file writes, secrets, binaries, and symlinks. Installation uses
Ryostore's receipt transaction, enables the bar host, and updates
`plugins.json`. The shell watches that file, so no restart is needed.

Look at the real result, not only the source. Confirm the glyph on the bar, click
it to open `Panel.qml`, and open QS Bar Settings > Community to see its settings.
Test each manifest host and density you claim. Capture the real surface as
`assets/preview-widget.png` before sharing it.

For a local source edit loop, point `RYOSTORE_PLUGINS_DIR` at a directory that
contains plugin folders. Ryoku discovers those folders live. Run
`ryoku plugin validate <dir>` after each structural change. Publishing is a
separate choice: do not run `ryoku plugin share <id>` unless you intend to open
a public Ryostore contribution.

Next: [Go tools](go-tools.md) explains the daemons behind QML. Return to the
[wiki index](README.md) for compositor guides.
