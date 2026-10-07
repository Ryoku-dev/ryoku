# Building things the Ryoku way

Do not begin with code. Read [`feature.md`](feature.md) and climb its ladder in
order:

1. Search Ryostore with `ryostore catalog`.
2. Check this machine with `ryoku-shell bar catalog`, `ryoku-shell bar list`, and
   `ryoku plugin list`.
3. See whether existing built-ins compose the result.
4. Only then build a shell plugin.

A hidden widget needs to be shown. A store item needs to be installed. Neither
needs a second implementation.

## Build a Quickshell plugin end to end

Scaffold a working plugin in the authoring tree, not in the installed plugin
root or the shipped shell:

```bash
ryoku plugin new hello-ryoku --bar --name "Hello Ryoku" --author "Name <mail>"
```

The default authoring root is
`$(xdg-user-dir DOCUMENTS)/ryoku-plugins/hello-ryoku/`, with
`~/ryoku-plugins/hello-ryoku/` as the fallback. The scaffold contains:

- `manifest.json`: identity, version, honest author and license, supported hosts,
  entry points, defaults, dependencies, capabilities, extra files, and the
  settings schema.
- `service/Main.qml`: persistent logic and state, with no UI.
- `content/Widget.qml`: the one view every selected host mounts.
- `content/Panel.qml`: the optional panel under a bar glyph.
- `README.md`, `LICENSE`, `AGENTS.md`, and an `assets/` directory for the real
  preview capture.

A plugin id is lowercase letters, digits, and dashes. Declare only hosts you
have tested: `topbarGlyph`, `desktopWidget`, `framePopout`, or `sidebarCard`.
Every external command belongs in the plugin's `bin/` or
`dependencies.commands`. Every network host and exact privileged command is
listed in the manifest. Extra QML and assets belong in `files`, or installation
will omit them.

Settings are manifest data, not a private UI. Declare `toggle`, `choice`,
`multi`, `int`, or `text` rows under `metadata.settings`. Read each value from
`pluginApi.pluginSettings` behind a default and write it only with
`pluginApi.saveSetting(key, value)`.

Validate, install, and look at the result:

```bash
ryoku plugin validate ~/Documents/ryoku-plugins/hello-ryoku
ryoku plugin add ~/Documents/ryoku-plugins/hello-ryoku --bar --yes
ryoku plugin list
```

`add` validates again, installs through Ryostore's receipt transaction, and
updates placement. The shell watches `plugins.json`, so an installed change
appears without a shell restart. Confirm the glyph on the bar, open its panel,
and inspect it under QS Bar Settings > Community. Test every host listed in the
manifest and every density it declares. Capture
`assets/preview-widget.png` from the real surface. Run `ryoku plugin share` only
when the user explicitly asks to publish.

For an edit loop against local plugin folders, point `RYOSTORE_PLUGINS_DIR` at
the folder containing them. For work on Ryoku's own QML, run
`ryoku/shell/dev-run.sh` from a checkout; it starts the checkout with QML hot
reload and does not copy changes back from `~/.config`.

## QML rules that matter

Ryoku's UI is paper and ink. Use restrained flat surfaces, warm ink, hairline
depth, generous spacing, and inversion for emphasis. Colour is data, not
ornament. Motion responds to an action or state change and stops when a surface
is hidden. Reuse the nearest component and shared primitive instead of inventing
a second visual language.

For Ryoku's own QML:

- Keep one component per `.qml` file and one concern per directory.
- Import `Ryoku.Ui.Singletons` and read `Tokens`. Never hardcode a hex colour,
  font, radius, spacing, or duration.
- Keep presentation in QML and validated policy or shared contracts in Go.
- Use the neutral `Wm` singleton from `Ryoku.Ui.Singletons` for outputs,
  workspaces, windows, focus, capabilities, and actions. Never import a
  compositor API or branch on a compositor name outside `ryoku/wm/`.
- Gate polling, effects, timers, and heavy rendering on the surface being open
  or visible.

A community plugin has a narrower public API. It imports
`Ryoku.PluginKit` or `Ryoku.PluginKit.Singletons`, then uses its `Theme` and
`Motion` values. Those resolve the active Ryoku scheme. A plugin must not import
`Ryoku.Ui` or shell internals directly. Report natural size through
`implicitWidth` and `implicitHeight`, read host-provided density and width, and
never draw outer chrome or position its own surface.

## Go tools and the seam

The Go command surfaces live in these checkout paths:

| Tool | Source |
|---|---|
| `ryoku` | `ryoku/cli/` |
| `ryoku-shell` | `ryoku/shell/ipc/` |
| `ryoku-wm-<name>` and the shared provider contract | `ryoku/wm/` |
| `ryoku-rashin` and `rashin` | `ryoku/rashin/backend/` |

Before opening files, ask Prowl's cited index:

```bash
prowl find <symbol>
prowl def <id>
prowl outline <path>
prowl references <id>
prowl impact <path>
```

Use `search` for a structural question, `find` for a named symbol, then `def`,
`outline`, or `references` to narrow the reading. Grep is for an exact literal
or regular expression after the location is known.

When adding a verb, find the existing dispatcher and the nearest command first.
Keep the dispatcher thin, put behavior beside its owning concern, update usage
and the user-facing reference, and add a behavior test in that Go module. A
window-manager behavior belongs in the shared contract or one provider, not as
a provider-name branch in a caller.

## Verify in a checkout

Ryoku development happens in a source checkout. Never modify
`/usr/share/ryoku/config`, `~/.config/quickshell/`, generated compositor files,
or an installed plugin and call that development. Those are delivery targets,
not source.

Use the focused checks for the files touched:

- Lua: `luac -p <file>`.
- Shell: `bash -n <file>`.
- QML: `qmllint <file>` when available, then load the real surface from the
  checkout and inspect it.
- Go: in every module touched, run `go build ./...`, `go vet ./...`, and
  `go test ./...`.
- Window-manager work: run `bin/ryoku-dev-verify-wm-isolation` and
  `bin/ryoku-dev-verify-delivery`, validate each provider's generated config,
  and exercise the behavior under every supported provider.

A parser, unit test, or build is not the last proof. Run the changed command or
surface, perform the user action, and observe the result. Update the matching
changelog and documentation with the same change. Delivery flows from the
checkout into packages and materialized config; it never flows backward from a
live machine.
