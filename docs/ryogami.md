# Ryogami

Ryogami owns wallpaper selection, restore, live playback, and the wallpaper
frames consumed by the shell. The picker and Stage Editor send requests to its
Unix socket; they never write wallpaper state themselves.

## Targets

A wallpaper can target every display, one display, or one workspace. The picker
exposes **All**, **This monitor**, and **This workspace**. The workspace choice
shows the active workspace name for the monitor where the picker opened.

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
