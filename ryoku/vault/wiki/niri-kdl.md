# niri in KDL

Ryoku authors niri config in KDL. The repository source is `ryoku/niri/`; the
live entry is `~/.config/niri/config.kdl`. That entry includes the rest in a
deliberate order:

1. session bootstrap files
2. machine seeds for keyboard, GPU, and monitors
3. generated `settings.kdl` and `rebinds.kdl`
4. your `user.kdl`, last

The last include wins when a value is set twice. Every included file must exist,
because niri treats a missing include as a hard config error. Ryoku therefore
ships the seeds and always generates both settings files, even when one would be
empty.

## Find the safe edit path

Ask the provider and ownership map:

```bash
ryoku wm config niri
ryoku owner ~/.config/niri/user.kdl
```

`~/.config/niri/user.kdl` is seeded once and then yours. It is the last file niri
reads, so it is the safe place for raw KDL the Hub cannot express. Do not edit
`config.kdl`, `settings.kdl`, or `rebinds.kdl`. The provider regenerates the last
two from the neutral settings store.

niri has no unbind, and a duplicate chord in one bind block is a hard error.
Ryoku resolves defaults, remaps, and removals into one complete generated
`rebinds.kdl`. Use Ryoku Hub for normal key changes; use `user.kdl` only for a
careful raw override or addition.

## Add a keybind

This opens Kitty with Super+Shift+T. niri spells the logo key `Mod` in its KDL:

```kdl
binds {
    Mod+Shift+T { spawn "kitty"; }
}
```

Place the block in `~/.config/niri/user.kdl`. A chord already supplied by the
generated include is replaced by the later user definition, but keep one winner
per chord in your own block.

## Add a window rule

This makes mpv open floating:

```kdl
window-rule {
    match app-id="mpv"
    open-floating true
}
```

`app-id` is the Wayland application identity. The provider uses the same niri
shape when it translates neutral Hub rules: a `window-rule` block, a `match`, and
property lines.

## Validate and verify

niri watches its config file and reloads valid saves itself. Validate the whole
include tree, then check the live provider:

```bash
niri validate -c ~/.config/niri/config.kdl
ryoku wm status
```

Press the new chord and open mpv. If the rule misses, use `ryoku wm state` to
read the live `appId` and title. Correct the matcher in `user.kdl`, validate
again, and observe the actual window. Do not force a generic reload action:
niri's provider reports that its config is file-only and watched.

Back to the [desktop tour](desktop.md), or see how the same jobs are expressed
in [Hyprland Lua](hyprland-lua.md).
