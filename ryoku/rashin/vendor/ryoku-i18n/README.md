# ryoku/i18n

One catalog of English source strings and their translations, read by every
Ryoku surface: the shell, the Hub, the apps, the wallpaper picker, the greeter,
both installers and the `ryoku` CLI.

The developer-facing guide is `docs/i18n.md`. This file is the map of the
directory.

|Path|What it is|
|---|---|
|`langs.json`|The one language table. Add a language here and nowhere else.|
|`catalog/<code>.json`|Generated catalog, one language. Never hand-edited.|
|`catalog/overrides/<code>.json`|Human fixes. Beat the generator, survive every pass.|
|`catalog/catalog.go`|The catalogs as an `embed.FS`, for a binary with no catalog on disk.|
|`i18n.go`|The Go runtime: `T`, `Tf`, `Use`, language resolution, catalog loading.|
|`langs.go`|`langs.json` compiled in: `Languages()`, `Find()`, `IsRTL()`.|
|`tools/sync.py`|extract / sync / check / cost / llm / langs. Ships as `/usr/bin/ryoku-i18n`.|
|`tools/wrap.py`|AST wrapper: finds displayed literals in QML and wraps them in `I18n.tr`.|
|`tools/install.sh`|Puts the catalog where a dev checkout looks for it.|

The tools live in `tools/` and not here because this directory is a Go module
(`ryoku-i18n`), and `go mod vendor` in a consumer copies every file beside the
package's sources. Keeping the module root to Go and `langs.json` keeps a
vendored copy down to three small files.

The other three readers of this catalog live with the code that uses them:

- `ryoku/ui/Singletons/I18n.qml` - QML, live-reloading, layered with the user overlay
- `installation/backend/lib/i18n.sh` - the installer's shell, one jq pass at start
- `.github/workflows/i18n.yml` - the job that keeps the catalogs current
