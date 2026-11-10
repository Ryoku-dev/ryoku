# Hyprland in Lua

Ryoku authors Hyprland config in Lua, not a hand-written `hyprland.conf`. The
repository source is `ryoku/hyprland/`. On a running machine,
`~/.config/hypr/hyprland.lua` is the entry point. It loads small modules from
`~/.config/hypr/modules/`, one concern per file:

- `binds.lua` for shortcuts
- `window_rules.lua` for window matching and behavior
- `animations.lua` for motion
- `decoration.lua` for gaps, borders, rounding, blur, and shadows
- `input.lua` and `touchpad.lua` for devices
- `autostart.lua` for session startup

These files are Ryoku's shipped base. Reading them is useful. Editing them in
place is not: an update lays them down again.

## Find the safe edit path

Ask before touching a live file:

```bash
ryoku wm config hyprland
ryoku owner ~/.config/hypr/user.lua
```

`~/.config/hypr/user.lua` is seeded once, loaded last, and then belongs to you.
It wins over the shipped modules and generated Hub settings. Use it only when
Ryoku Hub cannot express the setting.

Generated `settings.lua` and `rebinds.lua` belong to the provider and Hub. Do not
edit them. Hardware files such as `monitors.lua`, `gpu.lua`, and `keyboard.lua`
are seeds or helper output; follow `ryoku owner` for each one.

## Add a keybind

Ryoku's modules use the `hl` Lua API. This binds Super+Shift+T to Kitty:

```lua
hl.bind("SUPER + SHIFT + T", hl.dsp.exec_cmd("kitty"))
```

Put that line in `~/.config/hypr/user.lua`. Prefer the Keybinds page in Ryoku Hub
for ordinary remaps; the raw file is the escape hatch for an action the editor
does not offer.

## Add a window rule

This makes mpv open floating:

```lua
hl.window_rule({
    name = "float-mpv",
    match = { class = "mpv" },
    float = true,
})
```

The `name` makes the rule readable, `match.class` selects the app, and `float`
is the behavior. Put it in the same user file. Look at the shipped
`modules/window_rules.lua` for nearby `hl.window_rule` shapes, then keep your
addition separate.

## Validate, reload, verify

Compile-check the Lua without running it, then reload through the neutral
provider seam:

```bash
luac -p ~/.config/hypr/user.lua
ryoku wm act config.reload
ryoku wm status
```

Now press the new chord and open mpv. If the window does not match, inspect its
live identity with `ryoku wm state` instead of guessing another class string.
A clean parse proves syntax; seeing the shortcut and rule work proves behavior.

A whole-file fork belongs under the mirrored overlay, for example
`~/.config/ryoku/user_edits/hypr/modules/binds.lua`. That is a last resort: the
fork wins after updates, but upstream changes to that module no longer reach it.
Return it to the shipped version with:

```bash
ryoku reset hypr/modules/binds.lua
```

Back to the [desktop tour](desktop.md), or compare the same model in [niri
KDL](niri-kdl.md).
