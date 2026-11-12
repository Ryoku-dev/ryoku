# Ryogami

Ryogami owns wallpaper selection, restore, live playback, and the wallpaper
frames consumed by the shell. The picker and Stage Editor send requests to its
Unix socket; they never write wallpaper state themselves.

## Targets

A wallpaper can target every display, one display, or one workspace. The picker
exposes **All**, **This monitor**, and **This workspace**. The workspace choice
shows the active workspace name for the monitor where the picker opened. With
**Apply only on the picker monitor** on, a plain pick and its hover preview
both stay on the picker's monitor.

Random rotation keeps per-display choices: once a display has its own
wallpaper, each display gets its own pick on every tick.

`ryogami wallpaper set PATH` without `--screen` follows the picker monitor
setting: on a desk with more than one display it sets the focused display
only. `--screen '*'` always sets every display.

The effective wallpaper on each display is resolved in this order:

1. the active workspace assignment;
2. that display's assignment;
3. the all-display `*` assignment;
4. the shipped default wallpaper.

Workspace assignments live in
`~/.cache/ryogami/workspaces.json`, beside `outputs.json`. The file is
versioned and daemon-owned. An entry records the window-manager provider's
stable workspace id plus its last name and output. The id is authoritative. If
a dynamic workspace is recreated with a new id, Ryogami uses the matching name
as a fallback and moves the assignment to the new id. Consumers only provide
neutral workspace frames from the window-manager seam.

`wall.apply` accepts `target: "workspace"` and a workspace object. The explicit
`wall.assign` and `wall.unassign` methods use the same target. The command-line
forms are:

```text
ryogami wallpaper set --workspace [--screen OUTPUT] PATH
ryogami wallpaper assign [--screen OUTPUT] [PATH]
ryogami wallpaper unassign [--screen OUTPUT]
```

`assign` without a path pins the wallpaper currently inherited by that
workspace. This is what the Stage Editor's **Use for this workspace** action
uses.

## Switching and restore

Workspace activation is handled per output. The first workspace frame after
startup restores with a cut. Rapid workspace cycling cuts intermediate choices,
then performs one reveal after the active workspace settles. Static images,
videos, and Wallpaper Engine scenes all use the same precedence. Videos publish
their poster first and keep at most one player on each output; changing one
output does not restart another output's player. A newly connected output first
restores its display fallback, then its active workspace assignment.

Stage scenes remain keyed by wallpaper path. Assigning the same image to more
than one workspace shares its Depth, Parallax, and layer scene.

## Preview

The picker previews a still through `wall.preview` before committing it. A
preview paints on the target outputs without recording anything: no
assignment changes, no apply count moves, no applied event fires, and the
palette stays with the committed wallpaper. The frame entries a preview
publishes carry a `preview` flag, so the shell and its readers can tell the
paint apart from a commit. Previewing another still replaces the preview,
and applying a wallpaper supersedes it. An apply that covers fewer displays
than the preview puts the committed wallpaper back on the rest. Closing the
picker clears the preview (`wall.preview.clear`, or `ryogami wallpaper
preview-clear`) and repaints each covered output's committed wallpaper. Only
stills preview; videos and Wallpaper Engine scenes apply directly.

## Image optimisation

New PNG, JPEG and GIF wallpapers are re-encoded to WebP when they land in the
library (**Settings > Wallpaper > Library > Images and recovery > Auto-optimise new images**,
on by default). The original goes to Ryogami's trash for the configured number
of days. When the file on screen gets replaced, Ryogami moves its own state to
the WebP copy: display and workspace assignments, library entries, playlists
and the recorded wallpaper that rice saves read.

## Downloading wallpapers

The Download browser (Ctrl + D) searches Wallhaven, Steam Workshop, Unsplash,
Pexels, YouTube, Bing, MoeWalls, MotionBGs, Ryostore and GitHub repos. Every
result card and the preview pane carry **Apply** and **Save**. Save downloads
into the library. Apply downloads when needed and sets the result at once,
stills and live wallpapers alike: a YouTube result applies its clip window and a
Workshop item applies as a scene. Enter applies the focused card. A source's
"Search only when I press Apply" setting is about filters only: changes wait for
the drawer's Apply button or Enter instead of searching on every change.

Workshop downloads go through the Steam client. Ryoku does not ship the
Steamworks helper, because it would mean redistributing Valve's library.

Ryogami looks for Wallpaper Engine in every library Steam lists in
`libraryfolders.vdf`, so an install on a second drive or under Flatpak Steam
shows its Workshop scenes, and scenes render with that install's `assets`
folder. **Workshop directory** and **WE assets directory** in Settings > Wallpaper > Sources
override both. Scenes already on disk show up in the Workshop tab as soon as
the first scan after a fresh install finishes.

Scenes that follow the cursor (camera parallax, mouse-driven effects) react
while the pointer is over the desktop, the same way Stage's Parallax does. The
desktop surface sits above the scene, so the shell reports hover positions and
ryogami passes them to the scene renderer, at most 60 times a second and only
while a scene is on that monitor. This needs skwd-paper beta.27 or newer.

Searching never holds up the picker. The daemon answers slow requests
(provider and Workshop searches, semantic queries, colour previews) on their own
workers, a newer search from the same picker cancels the older one, a search
gives up after 20 seconds, and download progress is sent at most eight times a
second.

## Settings

The picker's settings have five tabs: **Wallpaper** (Library, Sources,
Playback and displays, Transitions, Automation), **Colours** (Source, Scheme,
Apps and outputs, Palette bridge, Desktop accents, Designer), **Lighting**,
**Picker** (Picker, Filter and search, Layout and position, Motion, Picker
keys) and **System** (Performance, Integrations). The interface language is
Ryoku Settings' (Time and language); the System tab links there.

```text
ryogami wallpaper settings [TAB] [SECTION] [CONTROL]
ryogami wallpaper collection wallpapers|themes|rices|workshop
ryogami settings get KEY
ryogami settings set KEY VALUE
ryogami settings reset KEY
```

`settings` opens the folio on a tab, section and control by key, for example
`ryogami wallpaper settings colours scheme theme.mode`; `-` skips a slot, and a
control alone finds its own section. The tab keys from before the regroup
(`theme`, `playback`, `performance`, `language` and the rest) still land on the
section their settings moved to. `collection` opens the picker on one
collection. `ryogami settings` reads or writes one setting through the daemon:
the value is JSON when it parses and plain text otherwise. The daemon is the
one writer of `matugen.json` (scheme, light or dark, contrast, leading colour,
the app roster) and of the theme keys in `theme.json` (`themeApps`,
`gtkTheme`, `gnomeAccent`), so other tools change them through this verb.

## Lighting

Settings > Lighting is the control for RGB hardware. OpenRGB covers USB
keyboards, mice and headsets plus SMBus memory and motherboards, `asusd` covers
ASUS laptop keyboards, and Ryoku covers QMK/VIA boards through `qmk_hid` or
VialRGB raw HID. Nothing is scanned or written until **Let Ryoku control
lighting** is on, and a device is touched only after it is adopted. **Follow
wallpaper & theme** adopts a device in a mode that can show a colour (its Static
mode, else a per-LED mode) and paints the current Matugen accent. Every later
palette change, from a new wallpaper or a fixed theme, repaints it. The picker
talks to the daemon, the daemon runs `ryoku-hub lighting`, and that command is
the only writer of `~/.config/ryoku/lighting.json`. QMK and Vial lighting is
re-applied when a board is plugged back in.

Arch, CachyOS and Void ship OpenRGB 1.0 (SDK protocol 6); Fedora ships
1.0~rc2 (protocol 5). The client speaks both and addresses devices by
controller ID on protocol 6, so a write lands on the intended device after a
hotplug. The OpenRGB server starts on demand through `ryoku-host transient`: a
systemd user unit, or a runit service on Void.

## Video codecs per edition

Fedora's `ffmpeg-free` has no x264 or x265. Ryogami encodes live wallpaper
clips and previews with OpenH264 there (the `openh264` package from Fedora's
Cisco repository), refuses HEVC sources it cannot decode with a clear message,
and disables the video optimiser with the reason shown beside it. Arch, CachyOS
and Void keep x264 and x265.
