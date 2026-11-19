# Lithic Gio gallery

This module renders the reusable Go library without application-specific code.
It has five pages: `foundation`, `controls`, `lists`, `overlays`, and
`designs`.

From this directory, open any page in a window:

```sh
go run . --page foundation
go run . --page controls --mode dark
go run . --page lists --mode light --design material
go run . --page overlays --mode follow
go run . --page designs
```

Render a complete page to a headless PNG:

```sh
go run . --page foundation --mode dark --png foundation-dark.png
go run . --page controls --mode dark --png controls-dark.png
go run . --page lists --mode dark --design adwaita --png lists-adwaita.png
go run . --page overlays --mode dark --png overlays-dark.png
go run . --page designs --mode light --png designs-light.png
go run . --page designs --palette colors.json --png designs-wallpaper.png
```

The gallery command from the repository root is:

```sh
cd docs/lithic/examples/gio
go run . --page <page> --mode <mode> [--design <design>] [--palette <file>] [--png <file>] [--scale <n>]
```

`--page` defaults to `foundation` and `--mode` defaults to `dark`, the
signature; `follow` reads this machine's palette. `--design` is `lithic` (the
default), `material`, `adwaita` or `fluent`, and draws the whole page in that
design. `--palette` follows the Material roles in a JSON file (a
`colors.json`) instead of this machine's, so a render on the wallpaper's
colours is the same on every machine; the round designs then wear those roles
as they are. The gallery never reads the desktop's own style choice. Omit
`--png` to open a window. PNG output is 1440 dp wide and grows to the complete
page height; `--scale` sets its pixels per dp (1 by default, 1.75 for a
fractional display).

`designs` sets the same parts side by side in Lithic, Material, Adwaita and
Fluent, whatever `--design` says: each column is a strip of window in its own
design (the side plane, a list and a work pane, then a page pane) holding
slabs, tablets, a row list, the selection wash, tone marks, meters, the field
surface, keycaps, an icon plaque's mark and a slider. Use it to check a part in
all four designs at once, in dark, light and on a palette.

Each page uses the resolved palette roles, generated tokens, type styles,
cuts, and interaction recipes. Interactive components set `Preview kit.State`
so the requested final state is present on the first captured frame. Overlay
previews draw in place. The live components use the normal deferred overlay
path.
