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
each stack paints:

| Case | Files | What it proves |
|---|---|---|
| `wallpaper-dark`, `wallpaper-light` | `colors.json` from `testdata/` | The wallpaper mapping, toning and scrim in follow, dark and light. |
| `scheme` | `shell.json` with `themePalette` | A named scheme wins over the wallpaper. |
| `signature` | none | The signature, with this host's edition accent. |
| `unfollowed` | `theme.json` with `followWallpaper` off | Follow mode ignores the wallpaper. Go and QML only: a browser receives the already resolved palette. |

Go is the reference. A role may differ by one step per channel, because Qt keeps colours
as floats. The web resolver also exports `jasperInk`, which CSS cannot compute; it is not
compared.

The fixtures come from fixed Matugen 4.2 seeds (`#2f5f8a` dark, `#2f7a64` light,
`#8a3f6b` scheme), so the cases do not depend on anyone's wallpaper.

`go run ./cmd/lithic-roles --mode dark` in `library/go` prints the roles resolved on the
current machine, which is useful when an app looks wrong.
