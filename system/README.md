# system/

How an installed Ryoku machine is put together, separate from the desktop in
`ryoku/`.

## What's here

- `packages/` The package lists the installer uses: `base.packages` (core system,
  Hyprland, SDDM, audio, fonts, CLI tools), `hardware.packages` (GPU drivers and
  microcode per vendor), `aur.packages` (built after install), and `dev.packages`
  (optional toolchains).
- `boot/` The boot chain: Limine with Ryoku branding, the Plymouth splash, and the
  mkinitcpio hooks.
- `hardware/` Hardware setup. `gpu/` picks the most capable GPU and pins it for
  Hyprland, `display/` scales high-resolution screens, and `drivers/` installs the
  right packages per vendor. The GPU and monitor settings are written as Hyprland
  Lua drop-ins.
- `policy/` System authorization rules (polkit).
  `52-ryoku-timedate.rules` lets the active desktop user set the time zone
  without a password, while `dev.ryoku.users.policy` gives the typed
  `ryoku-host users` API retained administrator authentication and
  `dev.ryoku.apps.policy` does the same for `ryoku-host apps`.

## Networking and services

Networking is NetworkManager with the iwd backend. The only services Ryoku turns
on are SDDM and NetworkManager. Both come from `packages/` and are enabled by the
installer in `installation/backend`, so there is no separate config to keep here
yet.
