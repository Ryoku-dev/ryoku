# Lithic terminal gallery

This gallery prints the terminal component pages as ANSI to standard output. It
does not start Bubble Tea, enter the alternate screen, or wait for input. It
has four pages: `foundation`, `controls`, `lists`, and `overlays`.

From this directory:

```sh
go run . --page foundation --width 120 --mode follow
go run . --page controls --width 120 --mode dark
go run . --page lists --width 120 --mode light
go run . --page overlays --width 120 --mode follow
```

The gallery command from the repository root is:

```sh
cd docs/lithic/examples/term
go run . --page <page> --width 120 --mode <mode> [--glyphs console|unicode]
```

`--page` defaults to `foundation`, `--width` defaults to 120, and `--mode`
defaults to `follow`. Leave `--glyphs` unset to use Unicode except when
`TERM=linux`.

Capture coloured output through a PTY. Lip Gloss downsamples colours when
standard output is not a terminal, so redirecting the command straight to a
file does not preserve the same palette. Redirect to a file only when the raw
ANSI stream is what you need to inspect.
