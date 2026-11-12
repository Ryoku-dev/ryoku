# The Profile plate

The Profile plate on Ryoku Settings > System > About (`ryoku-shell hub open about`) is the system dossier: a full-bleed
plate carrying a dithered bone hero, your identity and epithets, live telemetry,
spec rows, a package wave, a palette strip, a barcode, a signal readout, the 顔
watermark, and a marginalia spine. It is the plate you screenshot to show off a
rice.

Out of the box it is fixed: the baked marble hero and the stock composition.
This guide covers making it yours without leaving the plate: an in-place edit
mode, your own hero image with a live bone dither, a plate PNG to share, and a
portable profile file to hand to someone else.

Everything is stored in `~/.config/ryoku/profile.json` (plus a copied hero image
under `~/.config/ryoku/profile/`). Both are created on demand. With no
`profile.json` present the plate renders the stock marble default, identical to a
fresh install, so you lose nothing by never touching it and you can always get
back to it with Reset.

## EDIT mode

The plate's top-right control is EDIT. Tap it and the plate flips into edit mode:
the composition stays exactly where it is (WYSIWYG) while edit chrome fades in
over it. Tap Done (or press Esc) to leave. Every change is live and saved the
moment you make it; there is no separate Save step.

### Showing and hiding blocks

The plate is a set of independent blocks. In edit mode each toggleable block
carries a small eye chip beside it; tap it to show or hide the block. A hidden
block stays visible but faint (ghosted) while editing, so you can bring it back,
and vanishes at rest. The blocks:

- **hero**, the dithered image (always on)
- **identity**, your name, host, time, and tagline (always on)
- **epithets**, the cycling word list and the kanji verse
- **live telemetry**, the CORE / GPU / MEMORY / NETWORK callouts pinned to the hero
- **spec rows**, resolution / compositor / uptime
- **package wave**
- **palette strip**
- **barcode** and edition
- **signal**, the LOAD readout and wave
- **顔 watermark**
- **marginalia** spine

Hero and identity are always on; the rest toggle freely.

### Editing text

The identity text is click-to-edit on the plate in edit mode. Click the name or
the tagline, type, click away (or press Enter); Esc cancels. Clearing a field
falls back to the live or default value, so blanking the name gives you the host
default again.

- **name**, an override for the identity name
- **tagline**

The decorative text (the marginalia spine, the 顔 watermark glyph, the cycling
epithet words, and the kanji verse) is set by hand in `profile.json` (see the
schema below) rather than inline, since those are the plate's fixed artwork.

### Presets

The edit toolbar has a Preset control that sets a whole sensible layout in one
tap:

- **Full**, today's plate with everything on. The default.
- **Minimal**, just the hero, identity, the spec rows, and the marginalia;
  everything else off.

A preset only sets which blocks are visible. Any toggle or text edit you make
afterward wins and persists, so you can start from Minimal and add back exactly
the blocks you want.

### Done and Reset

Done leaves edit mode. Reset restores the stock marble default (with a confirm,
since it clears your customization). Both live in the edit toolbar alongside the
import and export controls below.

## The hero image

The hero is the large dithered image on the plate. In edit mode the hero carries
an edit affordance (an overlay button) that opens the hero editor.

### Picking an image

The hero editor shows a gallery: the baked ryodecor set (the same
bone-on-transparent art the rest of the desktop uses, from the XDG Pictures
folder's `ryodecors/` directory) plus a **Your image** tile. Pick a gallery
piece to use it as-is, or use Your image to browse for a file or drop one onto
the tile.

### Framing

Framing works like the decor editor elsewhere in Ryoku Settings: the image fills the
hero box (cover), and you place it with a draggable focal point (drag the point
to say which part stays centered) and a zoom control. It is WYSIWYG against the
real hero box, so what you frame is what the plate shows.

### The live dither

The baked gallery art and the stock marble hero are already 1-bit bone, so they
render as-is. Your own image is full color, so the hero editor runs it through a
live bone dither: the same ordered (Bayer 4x4) dither `ryodither` bakes, applied
on the GPU as you frame it. It reduces the image to Ryoku bone (`#e8d8c9`) on a
transparent ground, so your photo reads in the set's 1-bit style instantly. Two
controls tune it:

- **Strength**, the dither scale.
- **Invert**, ink the dark tones instead of the light. Turn it on for a
  dark-on-light source (a dark subject on a pale background).

On a machine with no shader support the hero falls back to a monochrome
desaturate, so it still degrades to the bone look.

### Where it is stored

When you pick your own image it is copied into `~/.config/ryoku/profile/` (as
`hero.<ext>`), so it persists and travels with a shared profile. `profile.json`
records the name plus your framing and dither settings.

## Export image

Export image (in the edit toolbar) grabs the plate on its own, without the
Ryoku Settings window around it, and saves it as a PNG in `~/Pictures` (`ryoku-profile-<date>.png`)
at 2x. This is the file you share or post to show off the rice; it is only the
plate.

## Share and import a profile

A profile can be packed into one portable file so someone else can drop in your
exact plate.

- **Export profile** writes a single self-contained `.ryoprofile` file (default
  `~/ryoku-<name>.ryoprofile`). It holds all your settings and, if you set a
  custom hero, the hero image embedded inside it. One file, nothing else to send.
- **Import** (in the edit toolbar) opens a file picker for a `.ryoprofile`. Pick
  one and, after a confirm (it replaces your current profile), it unpacks the
  hero image and applies the settings in a single step. The plate updates live.

An invalid or corrupt `.ryoprofile` fails with a clear message and changes
nothing, so a bad file cannot leave you half-imported.

## Hand-editing profile.json

Everything edit mode writes lives in `~/.config/ryoku/profile.json`, and you can
edit it by hand. Every field is optional: an absent or empty field falls back to
its default (the live value or the stock plate), so a partial file is fine and an
absent file is exactly the stock marble plate.

```json
{
  "preset": "full",
  "blocks": {
    "epithets": true, "telemetry": true, "specs": true, "packages": true,
    "palette": true, "barcode": true, "signal": true, "watermark": true,
    "marginalia": true
  },
  "hero": {
    "kind": "default",
    "source": "hero.png",
    "focalX": 0.5, "focalY": 0.4, "zoom": 1.0,
    "dither": 1.0, "invert": false
  },
  "text": {
    "name": "", "tagline": "", "marginalia": "", "watermarkGlyph": "",
    "epithets": [], "verse": []
  },
  "vitals": ["core", "gpu", "mem", "net", "frac"],
  "specs": ["resolution", "compositor", "uptime"]
}
```

Notes:

- `hero.kind` is `default` (the baked marble), `gallery` (a ryodecor named by its
  file name in `source`), or `custom` (your copied image under
  `~/.config/ryoku/profile/`, dithered live).
- `focalX` / `focalY` are 0..1 (the focal point), `zoom` is the cover zoom,
  `dither` is the strength, and `invert` matches the editor toggle.
- `blocks` mirrors the eye chips; a missing block key reads as on.
- `text` holds overrides; an empty string or empty list keeps the live default.
- `vitals` and `specs` choose which telemetry callouts and spec rows show.

A missing or corrupt file is treated as all-defaults, so you cannot break the
plate by editing it: worst case you get the stock plate back.

## Power users: ryodither

The hero editor's live dither is convenient, but if you want to pre-bake art (or
batch a whole set) the same dither is available as a CLI, `bin/art/ryodither`. It
is the tool that bakes the shipped decor set, so anything you bake with it
matches the set exactly.

Bake your own art into the gallery so it shows up beside the shipped set:

```sh
ryodither yourart.jpg --out "$(xdg-user-dir PICTURES)/ryodecors"
```

That writes a bone-on-transparent dithered PNG into the `ryodecors/` directory
under your configured Pictures folder, where the hero editor's gallery picks it
up. Without `--out` it writes into the repo's `ryoku/assets/ryodecors` (the dev
path that ships the decor to everyone at once).

- `--invert` inks the dark tones instead of the light, for dark-on-light sources
  (a dark subject on a pale ground). It is the CLI form of the editor's Invert.
- A gif bakes per-frame, keeping its timing, loop, and transparency, so a motion
  loop stays a motion loop.
- `--name NAME` sets the output basename (single input only), and `--bone HEX`
  overrides the ink colour (default `#e8d8c9`, the Ryoku bone).

ryodither needs Pillow (`pip install pillow`).

It also lists CC0 and public-domain art worth baking:

- Wikimedia Commons (sculpture, Muybridge plates)
- The Public Domain Review (phenakistoscope and zoetrope loops)
- Internet Archive (films, Muybridge collections)
- The Met, Open Access (CC0 sculpture photography)
- Art Institute of Chicago (CC0 art)
- Smithsonian Open Access (CC0 art and objects)
- Rijksmuseum, Rijksstudio (CC0 art)
- Getty, Open Content (public-domain art)
- NASA image and video (earth, moon, space)

This is the same dither the hero editor applies live, so pre-baked art and a
live-dithered custom image read identically on the plate.
