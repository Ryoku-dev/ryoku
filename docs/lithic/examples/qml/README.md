# QML gallery

Run a page from this directory:

```sh
QT_FORCE_STDERR_LOGGING=1 \
  LITHIC_PAGE=<page> \
  LITHIC_MODE=<mode> \
  qml6 -I ../../library/qml Gallery.qml
```

`LITHIC_MODE` accepts `follow`, `dark`, or `light`. Set `QT_FORCE_STDERR_LOGGING=1` on every plain QML run so Qt writes diagnostics to stderr instead of sending them only to the journal.

## Pages

- `foundation` shows palette roles, type, primitives, recipes, and motion.
- `controls` shows plaque tiers, fields on a pane and inside a slab, selectors including `Segmented`, slab rows, selection, a checked menu item, and disclosure.
- `lists` shows list rows, containers, state, and progress.
- `overlays` shows popovers, dialogs, navigation, charts, and the application frame.

For example:

```sh
QT_FORCE_STDERR_LOGGING=1 \
  LITHIC_PAGE=controls \
  LITHIC_MODE=dark \
  qml6 -I ../../library/qml Gallery.qml
```

## Headless capture

Add the offscreen platform, file-XHR permission, and capture path. The gallery waits for palette resolution and layout, writes the complete page, then exits:

```sh
QT_QPA_PLATFORM=offscreen \
  QT_FORCE_STDERR_LOGGING=1 \
  QML_XHR_ALLOW_FILE_READ=1 \
  LITHIC_PAGE=<page> \
  LITHIC_MODE=<mode> \
  LITHIC_CAPTURE_PATH=<png> \
  qml6 -I ../../library/qml Gallery.qml
```

Set `LITHIC_PALETTE_JSON=/path/to/colors.json` when a deterministic palette should replace the live palette. The file may contain Material roles directly or wrap them in `themePalette`.

The gallery is 1440 dp wide, at least 900 dp high, and grows to the loaded page's complete `implicitHeight`.

## Adding a page

A page is an `Item` in `pages/` with an accurate `implicitHeight`. It receives its width from the gallery Loader. Title each section with the exact component name and label every state in plain sentence case. Pages must be static at rest, must not poll, and must not write warnings or diagnostics.

The reusable API and Quickshell palette-watching pattern are documented in [`../../library/qml/README.md`](../../library/qml/README.md).
