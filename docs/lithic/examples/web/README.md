# Lithic web gallery

This folder exercises the dependency-free library in `../../library/web/`. It uses the aggregate stylesheet and module exactly as an application does.

## Serve it

Start the server from `docs/lithic/` so examples and library files share one origin:

```sh
cd docs/lithic
python -m http.server 4173 --bind 127.0.0.1
```

Open <http://127.0.0.1:4173/examples/web/>. Gallery pages are designed for a 1440 pixel wide capture and grow downward when their content exceeds 900 pixels.

## Pages

| Page | Coverage |
|---|---|
| [`index.html`](index.html) | Links all four galleries and preserves the current query parameters on each link. |
| [`foundation.html`](foundation.html) | Palette roles, type, cuts, spacing, Icon, CutRect, Notch, selection, Blade, Flake, Hatch, FocusRing, and interaction recipes. |
| [`controls.html`](controls.html) | Plaque, SplitPlaque, IconPlaque, TextField, Search, fields on a pane and in a slab, Toggle, Slider, Stepper, Segmented, Filters, Tabs, the selected row, Menu checked items, and Disclosure. |
| [`lists.html`](lists.html) | Tablet, RowList, GroupHeader, Skeleton, Empty, Pane, MineralField, Header, Slab, Section, Facts, Aside, DraftColumns, Claim, Marks, Sentence, BladeMeter, BladeProgress, Working, and BecomingName. |
| [`overlays.html`](overlays.html) | Popover, Menu, Picker, Dialog, Confirm, Tooltip, Placard, Rail, BrandMark, NewPlaque, SectionIndex, EngravedChart, and AppFrame. |

Each page links `../../library/web/lithic.css`, imports `../../library/web/lithic.js`, and calls `await Lithic.init(document)`.

## Query parameters

| Parameter | Values | Effect |
|---|---|---|
| `mode` | `dark`, `light`, or `follow` | Selects palette resolution mode. `dark` is the default; dark and light paint the signature and ignore `palette`. |
| `palette` | A relative or absolute JSON URL | In `follow`, fetches a raw Matugen palette, `{ "themePalette": { ... } }`, or `{ "roles": { ... }, "source": "..." }` descriptor. Without it, the signature is used. |
| `contrast` | `standard` or `high` | Selects the look's contrast. `standard` is the default; an unknown value is ignored. |
| `tint` | `off`, `low`, or `medium` | Selects the look's tint toward the palette. `off` is the default; an unknown value is ignored. |

Parameters can be combined:

```text
http://127.0.0.1:4173/examples/web/controls.html?mode=dark
http://127.0.0.1:4173/examples/web/lists.html?mode=follow&palette=colors.json
```

To preview this machine's current Ryoku palette, make a temporary local copy:

```sh
cp ~/.cache/ryoku/colors.json examples/web/colors.json
```

`examples/web/colors.json` is local runtime input. It is never committed. Remove it after the preview. A product exposes the palette through a narrow app or daemon endpoint instead of copying it.

The complete integration, JavaScript, CSS, class, property, event, and browser contracts are in [`../../library/web/README.md`](../../library/web/README.md).
