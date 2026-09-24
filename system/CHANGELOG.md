# Changelog: system/

## Unreleased

- **The GPU MUX knob is GUI-reachable without a terminal.** `ryoku-gpu-mux
  set` escalates through pkexec under a scoped polkit grant
  (`hardware/gpu/45-ryoku-gpu-mux.rules`, wheel, the one program), so the
  Hub's Machine page can flip display routing; the change still only takes
  effect at a reboot the user performs.


- **The base set no longer installs Spotify.** `spotify-launcher`,
  `spicetify-cli` and `spicetify-marketplace` are out of
  `system/packages/base.packages`; Ryotunes is the music app a fresh install
  gets. A Spotify a user installs themselves is untouched.

- **`ryoku update` no longer deadlocks on the Oh My Zsh swap.** CachyOS-era
  installs carry `cachyos-zsh-config`, which depends on `oh-my-zsh-git`;
  ryoku-oh-my-zsh previously conflicted with that package without providing
  it, so pacman offered the removal and then refused it ("removing
  oh-my-zsh-git breaks dependency"), failing every full upgrade. The package
  now version-provides, conflicts with and replaces both `oh-my-zsh` and
  `oh-my-zsh-git`, so `-Syu` swaps the AUR tree in one transaction. This same
  package contract serves existing systems, the script installer and the ISO
  package closure (`release/packages/ryoku-oh-my-zsh/PKGBUILD`).

- **CachyOS's Qt5 SDDM greeter now has its native Wayland platform plugin.**
  `qt6-wayland` cannot supply a QPA plugin to a Qt5 process; base installs and
  the `ryoku-desktop` dependency closure now include `qt5-wayland`, so the
  Wayland greeter starts on fresh ISO installs, script conversions, and
  existing systems after an update.

- **The render pin now covers laptops.** `ryoku-gpu` pinned the strongest GPU
  only on desktops, so a hybrid laptop composited, blurred and decoded video on
  its iGPU -- the same die as the CPU -- and the package cooked while a discrete
  GPU sat parked. The default policy now pins the discrete GPU everywhere; the
  graphics mode the user chose is stamped into gpu.lua
  (`-- ryoku-gpu-mode: hybrid|performance|passthrough`) and login-time
  `persist` honours it, so Hybrid stays an explicit opt-out for battery.
  `check-pin` grew a `missing-pin` verdict so the ryoku doctor writes the pin
  on machines the old policy left unpinned, and `mode performance` names the
  reboot-gated `ryoku-gpu-mux set discrete` step on MUX laptops
  (`system/hardware/gpu/ryoku-gpu`, `tests/gpu-pin-policy.sh`).

### Added
- `ttf-maple-mono-nf` (release/packages + base.packages): Maple Mono, Nerd Font
  variant, shipped from [ryoku] as the upstream prebuilt NF release so it
  pacstraps on install and updates with `ryoku update`. Offered as the monospace
  font in Hub > Global.
- `containers/ryoku-docker` and `containers/46-ryoku-docker.rules`: the one
  privileged door for container work, which is what makes the stash "Cobalt
  engine" switch a switch instead of a chore list. Verbs: `state` (read-only
  facts, never escalates), `provision` (enable `docker.service`, ensure the
  `docker` group, add the invoking user, wait for the socket), and
  `container-status` / `container-up` / `container-down` for the single
  `ryoku-cobalt` container.

  It escalates through pkexec rather than relying on the `docker` group, and that
  choice is the whole reason first-run setup needs no reboot: adding a user to a
  group does not affect the session they are already in, so a group-based design
  ends its first run at a reboot prompt. The group is still added, because a user
  who types `docker` wants it, but nothing here waits on it.

  SECURITY: the polkit grant is passwordless, so there is deliberately NO docker
  passthrough verb. Every container action is a fixed argument vector with a
  hardcoded image and container name, and the only caller-supplied value (a port)
  is range-checked to an integer 1024-65535 before anything escalates. A helper
  that forwarded caller flags to `docker run` would be root via `-v /:/host`.
  `tests/cobalt-setup.sh` asserts the refusals rather than the acceptances, and
  that each one happens before the privilege boundary.
- `policy/52-ryoku-timedate.rules`: a polkit rule that lets the active desktop
  user (in `wheel`) set the system time zone without a password, so the Hub's
  world-map time zone picker applies `org.freedesktop.timedate1.set-timezone`
  in one click. Installed to `/usr/share/polkit-1/rules.d/` by the ryoku-desktop
  package.

### Fixed
- `hardware/power/logind-ryoku-lid.conf`: raise `InhibitDelayMaxSec` to 15s.
  `hypridle` holds a `sleep` delay inhibitor while it runs `ryoku-shell lock`,
  and logind's 5s default expired first on a Quickshell lock that also had to
  wait out a display reconfigure (undocking as the lid shuts), so the machine
  suspended with the session unlocked. logind logged it and went ahead anyway:
  "Delay lock is active (hypridle) but inhibitor timeout is reached". The
  inhibitor is released the moment the lock is up, so a normal lid close still
  suspends immediately.
- `boot/limine/limine.conf`: ship `default_entry: 1` (the bootable flat
  placeholder) plus `remember_last_entry: yes`, not the bare `2`. Limine's
  numeric `default_entry` counts top-level entries, so once
  limine-mkinitcpio-hook makes entry 1 a directory, `2` lands on the `/EFI
  fallback` sibling, which chainloads Limine and loops the countdown forever.
  The installer finalize and the doctor reconciler repoint it at the kernel's
  entry path.
- `boot/limine/default.conf`: `MAX_SNAPSHOT_ENTRIES` now matches snapper's
  `NUMBER_LIMIT` (10). At 5 every limine-snapper-sync run warned about the
  snapshots it could not list, and the boot menu showed half the rollback
  depth the retention policy keeps.

### Added
- `extras/ryoku-extras-install`: `plugin` bundle items now install (fetched into
  `~/.local/share/ryoku/plugins` via `ryoku-hub extras plugin`) instead of being
  deferred, and report real present/absent state. Removal deletes the plugin's
  files while leaving the user's placement in `plugins.json`.
- `packages/`: base, hardware (per-vendor GPU drivers and microcode), aur, and dev
  package lists.
- `boot/`: Limine config with Ryoku branding, the Plymouth theme, and the
  mkinitcpio hooks.
- `hardware/`: `ryoku-gpu` (picks and pins the strongest GPU), `ryoku-monitor`
  (HiDPI autoscale), the GPU udev rule, and per-vendor driver scripts. GPU and
  monitor settings are written as Hyprland Lua drop-ins.
- `extras/`: the helpers behind the Hub's Extras section (`ryoku-extras-install`
  and the `ryoku-pkg-*` routing wrappers) that install, remove, and report the
  optional bundles from the `ryoku-extras` catalogue.

### Fixed
- `hardware/drivers/nvidia.sh`: keeps an already-installed NVIDIA module
  package instead of forcing a -dkms one next to it. CachyOS ships
  kernel-matched prebuilt modules (`linux-cachyos-nvidia-open`) that conflict
  with `nvidia-open-dkms`, so the old behaviour aborted the whole transaction
  under `--noconfirm`. The initramfs pacman hook also lost its
  `Depends=mkinitcpio` and now probes for limine-mkinitcpio, mkinitcpio, or
  dracut, and prebuilt module packages join its trigger list.
- `hardware/display/ryoku-monitor`: autoscale picked absurd scales inside
  virtual machines. A hypervisor fabricates the guest display's EDID, so the
  px/mm density math ran on fiction (a plausible fake physical size sails past
  the existing zero/absurd-DPI guards straight into the 1.25-2.0 buckets).
  Autoscale now pins the 1x bucket for every output of a VM guest
  (`systemd-detect-virt --vm`) and for `Virtual-*` connectors on bare metal;
  the host window does the real scaling anyway. `RYOKU_MONITOR_VM` overrides
  detection for tests, and `monitors_user.lua` pins still win as before.
- `hardware/gpu/ryoku-gpu-detect`: GPU detection could hang indefinitely. It
  reads NVIDIA VRAM from `nvidia-smi` (and model names from `lspci`), and a
  runtime-suspended or wedged GPU can make `nvidia-smi` block forever, stalling
  `ryoku-gpu detect` and every caller (including the Hub GPU page). The host
  probes now run once, in parallel, under a hard `timeout` (8s default, override
  with `RYOKU_GPU_PROBE_TIMEOUT`): a single pass covers every GPU, so the wait is
  one window instead of one per card and the budget can be generous without
  serialising. A probe that times out degrades to no VRAM/model rather than
  hanging; the GPU is still detected and classified.
