# keysounds

A Hyprland plugin that plays a keyboard sound on every key press. The
compositor sees every key before any application does, so one plugin covers
every window with no per-app hook and nothing grabbing your input.

Turn it on under Ryoku Settings > Desktop > Sounds > Key sounds. The package is
`ryoku-keysounds`; the plugin loads as `keysounds.so`. On niri, which has no
plugins, the same feature comes from the optional `ryoku-keysounds-niri` engine
under Ryoku Settings > Desktop > Sounds.

## Settings

All under `plugin:keysounds:` (Ryoku Settings writes them into `settings.lua` for you):

| Key | Type | Default | What it does |
|---|---|---|---|
| `enabled` | bool | `true` | play a sound on every key press |
| `profile` | string | `cherry-mx-brown` | which switch you hear (below) |
| `volume` | float 0..1 | `0.6` | playback volume; the samples are the 1.0 reference |
| `release` | bool | `true` | also play the key-up sample |

## Profiles

Every shipped profile is a recording of the real switch, cut from the
MIT-licensed [Mechvibes](https://github.com/hainguyents13/mechvibes) packs at
build time (`ryoku/keysounds/profiles.txt` maps each to its pack; the licence
ships beside the samples). The samples are their own package,
`ryoku-keysounds-profiles`, shared with niri's key-sounds daemon; this plugin
depends on it and reads the profiles from `/usr/share/ryoku/keysounds`. Named
for the switch, nothing more:

| Profile | Switch | Character |
|---|---|---|
| `cherry-mx-blue` | Cherry MX Blue | clicky, the classic mechanical click |
| `cherry-mx-brown` | Cherry MX Brown | tactile, quiet bump (the default) |
| `cherry-mx-black` | Cherry MX Black | linear, deep and firm |
| `cherry-mx-red` | Cherry MX Red, PBT caps | linear, light |
| `topre` | Topre Purple Hybrid | electro-capacitive thock |
| `creamy` | "Full Creamy Goodness" | lubed linear, smooth and creamy |
| `nk-cream` | NovelKeys Cream | POM linear, deep |
| `holy-panda` | Holy Panda | tactile, sharp round bump |
| `tealios` | Zeal Tealios | linear, glassy and soft |
| `crystal-purple` | Everglide Crystal Purple | tactile, bright |
| `oreo` | Everglide Oreo | linear, muted |

### Any Mechvibes pack

`ryoku-keysounds-import <pack-dir> [--name <profile>]` turns a pack from
[mechvibes.com](https://mechvibes.com) (a folder with `config.json`) into a
profile under `~/.local/share/ryoku/keysounds/`, slicing sprite packs and
copying per-key packs, release sounds included when the pack has them, every
sample normalised to one level. Pick the name as the profile and it plays on
the next key.

### Your own samples

A profile is a directory of short `.wav`, `.oga` or `.ogg` files named by role:

```
~/.local/share/ryoku/keysounds/<name>/
  down-1.wav down-2.wav ...     any key press (one is picked at random)
  up-1.wav ...                  key release (with `release` on)
  space-1.wav ...               optional: Space
  enter-1.wav ...               optional: Enter and keypad Enter
  backspace-1.wav ...           optional: Backspace
  space-up-1.wav ...            optional: the release of those three
```

A missing role falls back to `down` (or `up`). Lookup order:
`$XDG_DATA_HOME/ryoku/keysounds` (yours), `keysounds/` beside the loaded `.so`
(a local build), then `/usr/share/ryoku/keysounds` (the package), so a profile
of yours shadows a shipped one of the same name. A profile that no longer
exists falls back to `cherry-mx-brown` with a notice.

## How it plays

Samples go through libcanberra, the freedesktop event-sound library: the sound
server caches each file after its first play, so a press costs one small
request and overlapping presses mix there, not in the compositor. Nothing is
decoded or scheduled on Hyprland's thread.

## Building

`make keysounds.so` builds the plugin against the installed Hyprland headers.
`make all` also lays the profiles into `sounds/` for a local build: it clones
Mechvibes at the pinned commit (or uses `MECHVIBES=<checkout>`) and runs
`ryoku-keysounds-import` on each line of `ryoku/keysounds/profiles.txt`
(`IMPORTER=` overrides which copy). `hyprpm.toml` names the `.so` step, which is
what `ryoku-hub desktop plugins rebuild keysounds` runs (from a checkout, or
from the copy `ryoku-keysounds` lays under `/usr/share/ryoku/hypr-plugins`).
Depends on `hyprland` (headers), `libcanberra`, and `git` + `ffmpeg` to cut the
profiles.
