# The frame

The frame is the rounded monitor border and the shared material behind Ryoku's
edge-bound shell surfaces. Each monitor creates one `BlobGroup` in
`ryoku/shell/quickshell/shell/modules/bar/Frame.qml`. The group contains the frame cut-out,
all enabled `FrameRail`s, and the one monitor-local `FrameMenuManager`.

A frame bar is part of that scene; it is not a floating island or a separate
popup host. The screen outside enabled rails and active surface rectangles
remains transparent to input.

## Geometry

`FrameRail.qml` turns the normalized `frameBars` configuration into one rail per
edge. A rail reports its occupied strip to the monitor's exclusive-zone reserve
and lays out axis-compatible widgets in three zones:

| Edge | Zones |
|---|---|
| top / bottom | start, center, end |
| left / right | top, center, bottom |

The `FrameBars` singleton is the only runtime normalizer. It preserves valid
frame-bar choices, rejects incompatible widget placements, and supplies the
safe default profile when data is missing or malformed.

`BlobRect` is a rounded rectangle. Its QML shape controls are `radius`,
`topLeftRadius`, `topRightRadius`, `bottomLeftRadius`, `bottomRightRadius`,
`deformScale`, and `taper`. Positive taper narrows the bottom edge; negative
taper narrows the top.

`BlobWave` adds an audio-driven bottom edge. A `BlobGroup` takes up to four
`BlobWave`s. Its QML properties are `levels`, `points` (2 to 64), `amplitude`,
`gain`, `mirror`, `edgeInset`, `waveStiffness`, `waveDamping`, `stepped`,
`blockPitch`, and `moving`. With `stepped` set, a wide band becomes a row of
square blocks spaced `blockPitch` apart, filling with up to `points` blocks, and
kept `edgeInset` clear of the ends. A group's `waveColor` traces the lifted edge
in ink; it is transparent by default. The edge is flat and stops repainting when
the levels are silent.

## Surface ownership

`FrameMenuManager.qml` owns active menu and surface state for a single monitor.
It resolves a finite menu/surface catalogue before opening anything and tracks
the request's anchor and identity. This gives the manager one place to enforce:

- replacement at a shared anchor;
- close only for the currently active request;
- Escape, backdrop, focus-loss, and fullscreen closure;
- owner, body, and backdrop input-mask regions; and
- monitor-local geometry after hotplug or scale changes.

`FrameMenu.qml` draws catalogue-backed menus. `FrameSurface.qml` is the single
Ryoku-owned surface host: it rides the shared `Popout` and mounts a body by
`kind`, so credential prompts, voice, capture, and small rail cards all share
one component. The menu manager, rather than a widget or IPC client, owns
their lifecycle. The two sidebars (Super+Escape, Super+S) are not frame
surfaces; they are screen-edge overlays of their own subsystem, see
`docs/sidebars.md`.

## Input and focus

The frame overlay uses explicit mask regions. Closed surfaces expose no body
mask. Opening a surface adds its trigger and body rectangles without capturing
the rest of the desktop. Closing a surface returns focus to the previous active
client when the manager's request still identifies that same surface.

Fullscreen and backdrop paths use the same close operation as an explicit
command. A stale delayed close, stale keyring prompt, or plugin self-dismissal
cannot close a replacement surface.

## Extending the frame

A new frame surface is a contract change. Keep all of the following in one
change:

1. Register a finite identifier, anchor, and monitor-local geometry.
2. Provide a content component with explicit dimensions and open-state gating.
3. Route command and widget requests through `FrameMenuManager`.
4. Add body and owner mask regions and verify focus/close behavior.
5. Expose only supported configuration through Ryoku Settings > Desktop > Bar.
6. Document the interaction and test normal, replacement, and stale-close
   cases.

Do not restore a second popup scene or create an unbounded dynamic component
loader. One monitor has one frame scene and one manager.
