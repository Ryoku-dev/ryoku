# dolphin

The KDE file manager, when it is the chosen one. Dolphin keeps its own settings,
so the only thing shipped here is the wallpaper action on its right-click menu.

## Set as Wallpaper

Plasma's own "Set as Wallpaper" action (`wallpaperfileitemaction`, from
`plasma-workspace`) asks the Plasma shell over D-Bus. On a Ryoku session there
is no Plasma shell to answer, so it fails with "The name is not activatable".

- `ryoku-set-wallpaper.desktop` is a KIO service menu that runs
  `ryogami wallpaper import` on the picture instead: it copies the picture into
  the wallpaper library, so it shows up in the picker, then sets it.
- `kservicemenurc` hides Plasma's action, so the menu shows one wallpaper entry
  that works.

`ryoku-desktop` installs the service menu to `/usr/share/kio/servicemenus/` and
the menu config to `/etc/xdg/kservicemenurc`, a system default that a user's
own `~/.config/kservicemenurc` still overrides. Dolphin reads both at startup
(`dolphin -q` to restart a running one).
