# Lithic Gio gallery

This module renders the reusable Go library without application-specific code.
It has four pages: `foundation`, `controls`, `lists`, and `overlays`.

From this directory, open any page in a window:

```sh
go run . --page foundation --mode follow
go run . --page controls --mode dark
go run . --page lists --mode light
go run . --page overlays --mode follow
```

Render a complete page to a headless PNG:

```sh
go run . --page foundation --mode dark --png foundation-dark.png
go run . --page controls --mode dark --png controls-dark.png
go run . --page lists --mode dark --png lists-dark.png
go run . --page overlays --mode dark --png overlays-dark.png
```

The gallery command from the repository root is:

```sh
cd docs/lithic/examples/gio
go run . --page <page> --mode <mode> [--png <file>]
```

`--page` defaults to `foundation` and `--mode` defaults to `follow`. Omit
`--png` to open a window. PNG output is 1440 dp wide at scale 1 and grows to
the complete page height.

Each page uses the live palette roles, generated tokens, type styles, cuts, and
interaction recipes. Interactive components set `Preview kit.State` so the
requested final state is present on the first captured frame. Overlay previews
draw in place. The live components use the normal deferred overlay path.
