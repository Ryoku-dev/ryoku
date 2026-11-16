# Palette parity

Lithic resolves its roles three times: in Go (`go/palette`), in the browser
(`web/palette.js`) and in QML (`qml/Lithic/Lithic.qml`). A Ryoku desktop runs all three
at once: the shell in QML, Rashin's console in a browser, Ryoport in Gio. If one stack
tones a colour differently, the same machine shows two different jaspers. This check
keeps them identical.

```sh
docs/lithic/library/parity/check.sh
```

Run it after changing the palette groups in `tokens.json`, anything in `go/palette`,
`web/palette.js`, or the role resolution in `Lithic.qml`. It needs `go`, `node` (or set
`LITHIC_JS=bun`) and `qml6` (or set `LITHIC_QML`).

Each case writes real config and cache files into a temporary home, then compares what
each stack paints for a mode and a look (contrast and tint, standard and off unless named).
Ryoku's colours are the signature, so dark and light must paint it whatever the
files hold: in those modes each case also checks that Go painted exactly what the
`signature` case painted (`<case> <mode> is the signature`), and the browser receives the
palette with this host's edition, as a daemon route would serve it.

| Case | Files | What it proves |
|---|---|---|
| `signature` | none | The signature, with this host's edition accent, in every mode; high contrast on both signature tables, where tint does nothing. It runs first: the other cases compare against it. |
| `wallpaper-dark`, `wallpaper-light` | `colors.json` from `testdata/` | Dark and light are the signature while a wallpaper palette exists. Following, the wallpaper mapping, the tinted selection, toning and scrim; then follow with high contrast and medium tint (dark), and high contrast and low tint (light). |
| `scheme` | `shell.json` with `themePalette` | Dark and light ignore a named scheme. Following, it wins over the wallpaper, also with low tint. |
| `unfollowed` | `theme.json` with `followWallpaper` off, and a `colors.json` | Follow mode ignores the wallpaper and paints the signature (Go and QML only: a browser receives the already resolved palette). Dark and light are the signature too, and low tint changes nothing there. |
| the same cases in `material`, `adwaita` and `fluent` | as above, with `--design` (Go) and `LITHIC_DESIGN` (QML) | The round designs (`palette.Material`, `Lithic.qml` `_materialFrom`): following, they wear the wallpaper's or the scheme's Material roles as they are; on Ryoku's colours, and following with the wallpaper unfollowed, they paint the signature exactly. Material with high contrast and medium tint, Adwaita with high contrast and low tint, where only scrim and the minerals move. Go and QML only: the web library has no designs. |
| `material-partial` | `colors.json` from `testdata/material-partial.json` (surface, on-surface, primary, error, outline) | Following in Lithic, Material and Fluent: a role the palette lacks keeps the toned ladder's, the container among them. |

Every case compares every role, including `slab`, `field` and `edge`, and, between Go and
QML, the selection container (`container`, and `onContainer`, which QML calls
`containerInk`); the web resolver does not resolve the container.

Go is the reference. Every stack mixes whole bytes and rounds half up: Go and web per
channel as they always did, QML through byte helpers for the selection, the look and the
edge, so those roles match exactly. A role may still differ by one step per channel where
Qt keeps a toned colour as floats (gold ink and jasper). The web resolver also exports
`jasperInk`, which CSS cannot compute; it is not compared.

The fixtures come from fixed Matugen 4.2 seeds (`#2f5f8a` dark, `#2f7a64` light,
`#8a3f6b` scheme), so the cases do not depend on anyone's wallpaper.

`go run ./cmd/lithic-roles --mode dark --contrast high --tint low` in `library/go` prints
the roles resolved on the current machine, which is useful when an app looks wrong;
`--design material` prints what that design wears.
