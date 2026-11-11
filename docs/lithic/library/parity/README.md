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
each stack paints for a mode and a look (contrast and tint, standard and off unless named):

| Case | Files | What it proves |
|---|---|---|
| `wallpaper-dark`, `wallpaper-light` | `colors.json` from `testdata/` | The wallpaper mapping, the tinted selection, toning and scrim in follow, dark and light; then follow with high contrast and medium tint (dark), and high contrast and low tint (light). |
| `scheme` | `shell.json` with `themePalette` | A named scheme wins over the wallpaper, in every mode and with low tint. |
| `signature` | none | The signature, with this host's edition accent, in every mode; high contrast on both signature tables, where tint does nothing. |
| `unfollowed` | `theme.json` with `followWallpaper` off | Follow mode ignores the wallpaper. Go and QML only: a browser receives the already resolved palette. Forced dark still reads the wallpaper, so dark with low tint also runs the web resolver: the signature neutrals take the palette's tint. |

Every case compares every role, including `slab`, `field` and `edge`.

Go is the reference. Every stack mixes whole bytes and rounds half up: Go and web per
channel as they always did, QML through byte helpers for the selection, the look and the
edge, so those roles match exactly. A role may still differ by one step per channel where
Qt keeps a toned colour as floats (gold ink and jasper). The web resolver also exports
`jasperInk`, which CSS cannot compute; it is not compared.

The fixtures come from fixed Matugen 4.2 seeds (`#2f5f8a` dark, `#2f7a64` light,
`#8a3f6b` scheme), so the cases do not depend on anyone's wallpaper.

`go run ./cmd/lithic-roles --mode dark --contrast high --tint low` in `library/go` prints
the roles resolved on the current machine, which is useful when an app looks wrong.
