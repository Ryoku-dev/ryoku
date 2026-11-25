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
- The entry sets `NotShowIn=KDE;`, so a Plasma session on the same install keeps
  Plasma's own action. Nothing hides Plasma's action: a Ryoku session lists both,
  and its owner can switch Plasma's off in Dolphin > Configure > Context Menu.
  A machine-wide hide would also hit Plasma sessions, because KConfig reads
  `kservicemenurc` from `XDG_CONFIG_DIRS` and no Ryoku-only path sits on it yet.

`ryoku-desktop` installs the service menu to `/usr/share/kio/servicemenus/`.
Dolphin reads it at startup (`dolphin -q` to restart a running one).
