# bin/art -- baking Ryoku decor art

`Decor` and `Placard` render a shared art set from `ryodecors/` under the
configured Pictures folder (seeded from `ryoku/assets/ryodecors`; resolved by
`ryoku/ui/Singletons/Ryodecors.qml`). Every file there is reduced to Ryoku
**bone** (`#e8d8c9`) on a transparent ground, so it composites on any surface
and reads as one set. These scripts do the reducing.

## The two treatments

A source is baked one of two ways, chosen by what it is, not by taste:

- **Dither** -- `ryodither` -- an ordered Bayer stipple carries the tone. Right
  for photographs, sculpture, and motion loops, where the eye reads tone through
  grain. It is the set's default (the sculpture stills and gif loops use it).
- **Smooth** -- `ryoduo` -- the tone maps straight onto bone through the alpha,
  no stipple. Right for fine line-art (a patent drawing, an engraving, a
  dimensioned blueprint), which a dither would break into noise.

Rule of thumb: if a dithered bake looks noisy, bake it smooth; if a smooth bake
looks flat, dither it.

## The tools

| Tool | Makes |
|---|---|
| `ryodither <src>` | dithered decor, bone-on-transparent, Bayer. Stills -> PNG, gifs per-frame. |
| `ryoduo <src>` | smooth decor, bone-on-transparent. Same I/O. |
| `ryowave --out f.gif` | draws the dictation audio-wave loop as a grayscale gif source, to be dithered. |
| `ryorender --out f.gif` | draws the GPU render loop, a shaded 3D object turning, as a grayscale gif source, to be dithered. `--shape` picks the object; output is deterministic. |
| `ryobounce --out f.gif` | draws the animation section's bouncing-ball loop, gravity easing with squash, as a grayscale gif source, to be dithered. Deterministic. |
| `ryocompass --out f.gif` | draws the Rashin compass loop, a needle sweeping a degree ring and eight-point rose, as a grayscale gif source, to be baked smooth. Deterministic. `--frames 1` draws the still rose. |
| `ryoneedle --out f.png` | draws "The Needle": a needle rising over a radiant north star, as a grayscale still for the Rashin poster. Deterministic. |
| `regrade-svg.py` | maps a recraft `line_art` SVG onto the ink ramp. |
| `qmlrecord.py` | the shared harness for previews made of real frames: a private headless weston, a QML scene stepped one display frame at a time (deterministic), true-alpha grabs and a looping WebP. A module, not a command. |
| `osd-previews` | records Ryoku's own pop-up in each design, dark and light, into `ryoku/hub/quickshell/art/osd/` for Settings, on `qmlrecord.py`. Re-run after changing a look; `--kind mic\|brightness\|keyboard` records the others for review. |

Both bakers default their output into `ryoku/assets/ryodecors`, so a new decor
ships everywhere at once: the installer seeds it, the `ryoku-desktop` package
carries it to `/usr/share/ryoku/ryodecors`, and `ryoku doctor` lays it into
`ryodecors/` under each user's configured Pictures folder. Reference it by bare
filename in a `Decor`/`Placard` `art:`. `--invert` inks dark-on-light sources (a
scanned patent); `--out /tmp` bakes elsewhere to preview first. `ryodither
--help` / `ryoduo --help` list the flags and public-domain source sites
(Wikimedia, the Met, NASA, and friends).

## How the current specimens were baked

| Decor | Source | Treatment |
|---|---|---|
| `katana.png` | an ink drawing of a katana | `ryoduo` (line-art, smooth) |
| `camera.png` | a camera dimensions blueprint | `ryoduo` (line-art, smooth) |
| `mic.png` | US 2,113,219, the RCA ribbon-mic patent (1938, public domain) | `ryoduo --invert` (dark-on-light line-art) |
| `wave.gif` | drawn by `ryowave` (no public-domain still of a live wave exists) | `ryowave` -> `ryodither` (motion loop) |
| `render.gif` | the torus knot, drawn by `ryorender` (the GPU section's default render) | `ryorender` -> `ryodither` (motion loop) |
| `torus.gif` `sphere.gif` `cube.gif` `spring.gif` | `ryorender --shape`, the render loops in the shuffle gallery | `ryorender` -> `ryodither` (motion loop) |
| `bounce.gif` | drawn by `ryobounce` (a bouncing ball, the plainest read on easing) | `ryobounce` -> `ryodither` (motion loop) |
| `compass.gif` | drawn by `ryocompass` (the Rashin 羅針 compass) | `ryocompass` -> `ryoduo` (thin line-art loop, smooth) |
| `needle.png` | drawn by `ryoneedle` (the Rashin poster, "The Needle") | `ryoneedle` -> `ryoduo` (line-art, smooth) |

The sculpture stills and the other gif loops are public-domain art baked with
`ryodither`.

## Making a new one

```sh
# a line-art specimen (patent, blueprint, engraving)
ryoduo source.png --name thing            # add --invert if it is dark-on-light
# a photograph, sculpture, or motion loop
ryodither source.jpg --name thing
# the audio wave (or re-tune it)
ryowave --out /tmp/w.gif && ryodither /tmp/w.gif --name wave
# the GPU 3D render (--shape knot/torus/sphere/cube/spring, or --list; same flags, same gif)
ryorender --out /tmp/r.gif && ryodither /tmp/r.gif --name render
# the bouncing-ball easing loop
ryobounce --out /tmp/b.gif && ryodither /tmp/b.gif --name bounce
# the Rashin compass (a needle sweeping a degree ring and rose)
ryocompass --out /tmp/c.gif && ryoduo /tmp/c.gif --name compass
# the Rashin poster art (a needle over a radiant north star)
ryoneedle --out /tmp/n.png && ryoduo /tmp/n.png --name needle
```

Then reference `thing.png` / `thing.gif` by bare name in a `Decor` or `Placard`,
and add it to `Decor.qml`'s `defaultArt` if it should join the shuffle gallery.
