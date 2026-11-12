# Key sounds

A sound on every key press, in every app. Both Ryoku desktops have it: Hyprland
through its `keysounds` plugin, niri through a small engine (`ryoku-keysounds`)
that reads the keyboard directly.

It is not installed by default. Turn it on when you want it.

## Turn it on (niri)

1. Open **Ryoku Settings > Desktop > Sounds** (`ryoku-shell hub open sounds`).
2. Under **Key sounds**, press **Install engine**. The install runs in a terminal
   (it asks for your password) and the engine starts straight away, so keys sound
   before your next login.
3. The button becomes **INSTALLED**. From then on the session starts the engine
   on its own.

On Hyprland the feature is a plugin; its settings are in the same **Key sounds**
section, and Ryoku Settings > Windows > Window manager shows whether it is
running and rebuilds it.

## Settings

- **Play a sound on every key press** - switch it off to silence it without
  uninstalling.
- **Sound profile** - which switch you hear. Eleven ship, and any pack you import
  joins the list.
- **Volume** - how loud, relative to how the samples were recorded.
- **Key release sound** - a softer sample on the way up as well.

A change applies within a second of saving; no relogin.

## Add a profile

Any pack from [mechvibes.com](https://mechvibes.com) (a folder with
`config.json`) becomes a profile:

```
ryoku-keysounds-import ~/Downloads/MyPack
```

It lands under `~/.local/share/ryoku/keysounds/` and shows up in the profile
list. Needs `ffmpeg`. A profile of yours shadows a shipped one of the same name.

## Turn it off

The switch above silences it and keeps it installed. To remove the engine,
uninstall its package (`ryoku-keysounds-niri`) with your package manager.
