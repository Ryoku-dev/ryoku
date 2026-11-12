# Ryoku Wiki

## Ryoku in ten lines

1. Ryoku is an Arch Linux desktop built as one coherent system.
2. It runs on Hyprland or niri; `ryoku wm status` tells you which is live.
3. Quickshell draws the bar, dock, launcher, desktop widgets, and overlays.
4. Ryoku Settings is the graphical home for system, shell, and compositor settings.
5. The `ryoku` command updates, checks, repairs, and rolls back the desktop.
6. Provider commands keep shared desktop code independent of either compositor.
7. Hyprland config is Lua, while niri config is KDL.
8. Small additions belong in validated Quickshell plugins before shell source.
9. Rashin is optional; the Ryoku vault and this wiki remain available without it.
10. Ryoku updates its own packages separately from the Arch system update.

## How to read this wiki

Start with [Linux basics](linux-basics.md) if Arch, services, or config paths are
new. Take the [desktop tour](desktop.md) next. Read [Hyprland in
Lua](hyprland-lua.md) or [niri in KDL](niri-kdl.md) for the compositor reported
by `ryoku wm status`. Continue with [Quickshell and
QML](quickshell-qml.md), [Go tools](go-tools.md), or [Rashin](rashin.md) when you
want to build or use those parts.

The one rule is simple: configure through Ryoku Settings or the owning command first.
When a file really needs an edit, run `ryoku owner <path>` and use the path it
prints. Your sparse overrides live under `~/.config/ryoku/user_edits/`, mirror
`~/.config`, and survive updates. Shipped and generated files are reference
material, not a place for personal changes.
