# Changelog: system/packages/

## Unreleased

### Added
- `base.packages`: **new installs choose one browser and one login-shell stack.**
  Firefox and Fish are recommended, with Chromium or Zen and Bash or Zsh
  available in the installers. The unchosen packages are filtered before
  installation and recorded so updates leave them out. `ryoku-oh-my-zsh` joins
  the Zsh stack in the signed package set, while shared prompt and CLI tools stay
  available with every shell.
- `base.packages`: **the screen recorder is now GPU Screen Recorder.** `wf-recorder`
  is replaced by `gpu-screen-recorder` (GPU-only capture and encode on both
  Hyprland and niri, behind `ryoku-cmd-record` / `ryoku-shell record`). It is also
  a hard depend of `ryoku-desktop`, so the ISO and `ryoku update` carry it either
  way.
- `aur.packages`: **fingerprint unlock at the lock and login screens.** The
  qylock lock and the SDDM greeter authenticate through a PAM stack that loads
  `pam_fprintd_grosshack.so` (scans the sensor while the password field is live),
  but the module was never packaged, so touch-to-unlock silently did nothing
  even though fprintd enroll/verify in Ryoku Settings worked. `pam-fprint-grosshack`
  now ships here for new installs; `ryoku doctor`'s fingerprint-module reconciler
  backfills existing boxes with a reader (this set is not revisited by
  `ryoku update`).
- `hardware.packages`: **Intel gets the iHD VA-API video-decode driver.**
  `intel-media-driver` joins the `[intel]` section so the live video wallpaper
  and media players hardware-decode on Gen8+ Intel instead of burning a CPU
  core in software. AMD's VA-API already ships in `mesa` and NVIDIA decodes via
  NVDEC through `nvidia-utils`, so only Intel needed a package. It bakes into
  the offline ISO closure.

- `base.packages`: **Bash and Zsh receive Fish-like editing when selected.**
  The installer keeps `blesh` with Bash and the Zsh plugins plus
  `ryoku-oh-my-zsh` with Zsh, rather than installing all three shell stacks on
  every machine.

- `base.packages`: **the Spicetify Marketplace store ships by default.**
  `spicetify-marketplace` (the store custom app -- the Marketplace icon in
  Spotify's sidebar for themes and extensions) joins `spotify-launcher` and
  `spicetify-cli`, shipped from `[ryoku]` (`release/packages/spicetify-marketplace`)
  so it is a plain pacman target on install, on `ryoku update`, and in the offline
  closure. `ryoku doctor` (`reconcileSpicetifyMarketplace`) copies it into the
  per-user spicetify CustomApps, enables it and applies it, so the store is set up
  out of the box instead of the fiddly manual install. Inert without Spotify.
- `base.packages`: **weston, to run the SDDM greeter on Wayland.** SDDM's default
  X11 greeter was orphaned when the Hyprland (Wayland) session started -- it
  lingered on a leftover Xorg and kept drawing power -- so the greeter now runs
  on Wayland (`DisplayServer=wayland`, `CompositorCommand=weston --shell=kiosk`)
  and SDDM tears it down cleanly at login. weston is the kiosk compositor that
  hosts the greeter as a Wayland client; without it a Wayland greeter cannot
  start. Official `extra` repo, and a `ryoku-desktop` depend so it also reaches
  existing boxes on `ryoku update`.
- `base.packages` / `aur.packages`: `game-devices-udev` moves from the AUR set to
  the base set, shipped from `[ryoku]`. It is not a driver: the kernel binds
  these pads already, but without its rules userspace cannot open their hidraw
  node, so a pad that works as a gamepad still reports no battery and cannot be
  configured by Steam or an emulator. 27 vendors, which is most of what people
  own once they stray from first-party pads: 8BitDo, PowerA, PDP, Nacon, Hori,
  Mad Catz, Razer, Thrustmaster, Valve. It was previously unreachable by `ryoku
  update`, exactly like `spicetify-cli` before it.
- `base.packages`: **Bluetooth adapter firmware is complete.** Arch split
  `linux-firmware` into per-vendor packages and the default set already pulls
  `-intel`, `-realtek`, `-mediatek`, `-atheros` and `-broadcom`, which covers
  essentially every built-in and USB adapter. The one hole is Broadcom's
  patchram blobs: `linux-firmware-broadcom` installs 118 files and zero `.hcd`,
  and Broadcom's USB parts are not self-initialising, so without the `.hcd`
  matching their USB id the adapter enumerates, logs `BCM: Patch
  brcm/BCM20702A1-....hcd not found`, and never comes up. That reads as dead
  hardware. `broadcom-bt-firmware` adds the 114 missing blobs; verified to share
  no file with `linux-firmware-broadcom`.
- `base.packages`: **wireless Xbox controllers work.** Most controllers already
  needed nothing here, which is why nothing was here: `hid_playstation`,
  `hid_sony` and `hid_nintendo` cover DualSense, DualShock and Switch Pro
  in-kernel, and `xpad` covers wired Xbox pads plus every XInput off-brand
  (Turtle Beach among them, claimed by vendor + interface signature since 6.4,
  so there is no per-brand driver to add). The gap was Bluetooth, which `xpad`
  does not speak at all: `xpadneo-dkms` fills it, and it now ships from
  `[ryoku]`.
- `base.packages`: ship `docker` and make Firefox available to the installer.
  Docker is what the stash "Cobalt engine" switch has always needed and never
  had: cobalt is distributed only as a container image, so on a clean install
  that switch could not work at all and said so with a dead end ("Install Docker
  to use cobalt"). Both are official repo packages, so `base.packages` is where
  they belong; they reach the ISO through the offline closure
  (`installation/iso/offline-repo.sh`) like every other repo package. The
  service is deliberately NOT enabled here: a machine that never opens that
  switch should not pay for a running dockerd, a `docker0` bridge and its
  iptables rules, so the setup wizard enables it on first use. No
  `docker-compose`: the cobalt lifecycle is a single `docker run`. Firefox is
  the recommended browser pick for new installs.
- `base.packages`: `spicetify-cli` moves out of `aur.packages` and ships from
  `[ryoku]` (`release/packages/spicetify-cli`), listed next to `spotify-launcher`
  because neither is useful for the Canvas backdrop without the other. The ISO
  already baked the AUR set and the build gate fails on a missing name, so an ISO
  install did get it; what it could not reach was an existing machine, because
  `ryoku update` is pacman and pacman never touches the AUR. A box that installed
  before this package set, or whose one-shot AUR build failed, had no spicetify
  and no route to one but a manual `yay -S`. As a signed `[ryoku]` package it is a
  plain pacman target: it lands on install, on update, and in the offline closure.

  `pnpm` joins the publish-repo build toolchain for it, and that is not
  decoration: `pnpm build:wrapper` generates `jsHelper/spicetifyWrapper.js`, the
  `Spicetify` global every extension is written against, and it is absent from the
  release tarball. A build that skipped it would look complete and silently break
  the Ryoku Canvas extension.
- `base.packages`: ship the Limine boot stack `limine-mkinitcpio-hook` (bundles
  `limine-entry-tool`) and `limine-snapper-sync` from `[ryoku]`, moved out of
  `aur.packages`. As AUR packages they were skipped on offline installs, so a
  fully offline box booted a frozen flat menu: no other-OS detection
  (`FIND_BOOTLOADERS`), no UKI tree, no boot-menu snapshots. Now they pacstrap
  with the base set, so `bootloader.sh`'s tool-managed path runs on every
  install. See `release/packages/limine-mkinitcpio-hook`.
- `base.packages` + `ryoku-desktop` hard depends: add the Bazzite-style gaming
  stack `gamescope`, `gamemode`, `mangohud`. gamescope runs a game in a nested
  micro-compositor isolated from the desktop, sidestepping the XWayland
  compositing that is the real source of in-game and Steam-overlay lag on Wayland
  (`gamescope -- %command%`); gamemode is the perf governor Steam invokes via
  `gamemoderun %command%`; mangohud is the FPS/frametime overlay. Hard depends so
  the ISO and `ryoku update` both carry them; lib32 variants ride opt-in multilib.
- `base.packages`: add `pciutils`. `lspci` is the GPU-detection path the driver
  scripts use during pacstrap; without it NVIDIA cards were silently missed.
- `base.packages`: add `alsa-utils` (`alsamixer`, `amixer`, `alsactl`) so a muted
  hardware codec can be inspected and unstuck by hand; omarchy ships them in its
  base set too.
- `base.packages`: add `spotify-launcher` so Spotify ships by default. It installs
  per-user (into `~/.local`, never root-owned `/opt`), so the music widget's
  Spotify Canvas backdrop wires up with no root chmod; the client itself downloads
  on first launch.
- `aur.packages`: add `game-devices-udev` (PS4/PS5/DualSense/DualShock and Switch
  Pro udev rules + battery) and `xpadneo-dkms` (Xbox One/Series Bluetooth pads).
- `base.packages`: add `linux-headers` and `dkms` (also hard `depends` of
  `ryoku-desktop`, so `ryoku update` reaches existing boxes). `xpadneo-dkms` (the
  Xbox One/Series Bluetooth driver above) is a DKMS module: without the kernel
  headers and the DKMS build system its module silently fails to build, so an Xbox
  controller was not detected at all until the user installed `linux-headers` +
  `dkms` by hand. `dkms`'s alpm hooks rebuild the module whenever headers land, so
  an existing box self-heals on the next update.
- `aur.packages`: add `spicetify-cli`, the Spotify client patcher the desktop
  music widget's Spotify Canvas backdrop needs. `ryoku doctor` drops the bundled
  `ryoku-canvas.js` extension into the spicetified client and applies it, pointing
  spicetify at the per-user `spotify-launcher` install.
- `base.packages`: add `gnome-themes-extra`, the standalone `Adwaita-dark` GTK
  theme the Hyprland autostart selects (`gsettings gtk-theme`). Without it that
  name has nothing on disk, so GTK3 apps -- notably the `xdg-desktop-portal-gtk`
  file chooser (the browser upload dialog) -- fell back to the built-in light
  Adwaita even in the dark session. It is also a hard `depends` of
  `ryoku-desktop`, which reaches existing boxes on `ryoku update` (an optdepend
  never installs on `-Syu`).
- `ryoku-extras-install`: installs and removes `nautilus-pack` guests, skips
  `optional`-tier items in a whole-bundle install (they install one at a time),
  and turns an aborted `interactive` fetch into a *deferred* state. Repo installs
  go through `pacman -Syu` (not `-S`), so a bundle can never trigger a partial
  upgrade (a stale library vs a freshly pulled one), and a bundle's `requires`
  (such as `multilib`) is ensured before its packages route.
- `base.packages`: add `ttf-firacode-nerd` and `ttf-hack-nerd`, two popular rice
  nerd fonts, so the shell's Global font picker has more that render on a fresh
  install (JetBrains Mono already ships). The picker also lists other popular
  families and shows whichever ones you install yourself.
- `base.packages`: add `waifu2x-ncnn-vulkan`, the GPU AI upscaler behind ryoshot
  Beautify's HD ×2 export and ryowalls Enhance, so every fresh install ships it.
  It is also a hard `depends` of `ryoku-desktop`, which reaches existing boxes on
  `ryoku update` (an optdepend never installs on `-Syu`).
- `base.packages`: add `ddcutil`, and `aur.packages`: add `nvibrant-bin`. The
  pill mixer's DISPLAY section drives external-monitor brightness through
  `ddcutil` (DDC/CI) and NVIDIA screen vibrance through `nvibrant`, both
  unguarded and declared nowhere, so on a packaged install those faders were
  silently dead. `ddcutil` ships from `extra`; `nvibrant-bin` is AUR (a no-op on
  non-NVIDIA GPUs). `tests/shell-tool-availability.sh` gained rows for both so
  CI catches the next such gap.
- `aur.packages`: add `mpvpaper`. ryowalls' Live tab plays video wallpapers
  through it (mpv on the background layer), but it was only an optdepend of
  `ryoku-desktop`, so a packaged install never pulled it and live wallpapers never
  worked for users. Listing it in the AUR set the installer installs fixes new
  boxes; `ryoku doctor` points existing ones at `ryoku-pkg-aur-add mpvpaper`.
- `base.packages`: add `wireless-regdb`, so the kernel can load `regulatory.db`.
  Without it the WiFi regulatory domain stays at world `00`, which caps TX power
  (weak uplink, TX rates collapse to the lowest MCS) and disables 6 GHz. The live
  ISO already shipped it, but the installed target set did not, so every install
  booted capped. Kernel-agnostic (shared `/usr/lib/firmware`), so stock `linux`
  and CachyOS kernels are fixed identically.
- `base.packages`: ship the Bluetooth stack, `bluez` + `bluez-utils`. The desktop
  has always had Bluetooth UI (Hub Connections > Bluetooth, the pill's link
  drill-in), but no install ever carried the daemon behind it: org.bluez never
  appeared on the bus, `Quickshell.Bluetooth.defaultAdapter` stayed null, and the
  adapter toggle no-opped silently. bluez-utils ships `bluetoothctl`, which the
  UI's pair-trust-connect flows shell out to. The installer enables
  `bluetooth.service` (see installation/backend).
- `base.packages`: add `nautilus-python`, which runs the Ryoku stash actions
  (Install, Compress, Send with LocalSend) in the Nautilus right-click menu.
- `base.packages`: ship the launcher's three missing tools so its features work
  on a fresh install: `libqalculate` (the calculator's qalc backend for units,
  currency, %, and functions), `mpv-mpris` (exposes the YouTube Music mpv stream
  over MPRIS, so the now-playing card and transport controls work), and `songrec`
  (the Recognize Music action). `tests/shell-tool-availability.sh` now gates all
  three, closing the "feature wired but tool not shipped" gap that let them ship
  broken.
- `base.packages`: add the windowed-VM stack (`qemu-desktop`, `edk2-ovmf`,
  `virglrenderer`) so a VM launches from Ryoku Settings > GPU > Machine out of the
  box. The GPU-passthrough extras (Looking Glass, kvmfr) stay AUR and on demand.
- Cursor themes: ship a curated set of modern XCursor themes the Ryoku Settings
  picker offers, beyond the Bibata family. `base.packages` adds `vimix-cursors`
  (official repo, flat modern); `aur.packages` adds `phinger-cursors` (clean
  rounded, light and dark), `volantes-cursors` (minimal, light and dark),
  `catppuccin-cursors-mocha` (pastel), and `apple_cursor` (macOS style; a source
  build, so best-effort). All install XCursor themes under `/usr/share/icons`, so
  they appear in the picker automatically.
- `base.packages`: add `iw`, used by `ryoku-wifi-powersave` to disable 802.11
  power-save for Game Mode (a low-latency win for competitive play).
- `base.packages`: the curated base set the installer pacstraps.
- `hardware.packages`: per-profile CPU microcode (`[amd]`, `[intel]`). GPU drivers
  come from `system/hardware/drivers/*.sh`, which the installer runs in the target.
- `aur.packages`: AUR add-ons (Limine hooks, Bibata cursors, AUR helper).
- `dev.packages`: the developer toolchains shipped with every machine (Go,
  Node/npm, Rust, Python/pip/pipx, mise).
- `base.packages`: the Ryoku shell runtime (`quickshell`, `awww`, `cliphist`,
  `hyprpicker`, `imagemagick`, `jq`) and the `yazi` file manager. `aur.packages`
  gains `wallust` (palette); `quickshell` moved from AUR to base (now official).
- `aur.packages`: add `localsend-bin` for the AirDrop-style LAN file sharing the
  pill's file stash speaks to. GlazePKG (`gpk`) now ships first-class from the
  `[ryoku]` repo as a `ryoku-desktop` dependency, so it is no longer in the AUR set.
- `base.packages`: add `hypridle` for laptop dim/lock/suspend timeouts and
  `upower` for the shell battery surface.
- `base.packages`: add `openrgb` for wallpaper-driven keyboard and lighting
  color control through `ryoku-leds`.
- `base.packages`: add `noto-fonts-cjk` and `inter-font` so Japanese Ryoku shell
  labels, the 力 brand mark, and the configured UI font render on fresh installs.
- `base.packages`: add `cava` for the pill's separated music visualizer island.
- `base.packages`: add `curl`, `python`, `libnotify`, and `xdg-utils` for the
  pill's file stash (LocalSend LAN discovery and send), weather (wttr.in), and
  opening stashed files with the default app.
- `base.packages`: add `ffmpeg` and `yt-dlp` for the stash's media compress and
  download actions, and `desktop-file-utils` so installing AppImages and tarballs
  refreshes the launcher's desktop database.
- `base.packages`: add `tesseract` and `tesseract-data-eng` for the pill's Super+D
  toolkit OCR (recognize text in a screen region to the clipboard).
- `base.packages`: add `zbar` for the Super+D toolkit QR scanner (decode a QR code
  in a screen region, copy it, and open URLs).
- `base.packages`: add `hyprsunset` for the Super+U utilities night-light toggle
  (a warm screen color temperature), driven by ryoku-cmd-nightlight.
- `base.packages`: add `gpu-screen-recorder` and `wf-recorder` for the pill's
  Super+U utilities Screen Recorder (gpu-screen-recorder, with a wf-recorder
  fallback on multi-GPU machines).
- `aur.packages`: add `handy-bin`, the offline speech-to-text app behind the
  pill's ``Super+` `` voice dictation. It provides `handy`, a normal desktop entry
  (so Handy shows in app search for configuring models), and pulls in
  `gtk-layer-shell`.
- `base.packages`: add `wtype` so Handy types the transcription into the focused
  app on Wayland.
- `base.packages`: add `snap-pac` for automatic pre/post Btrfs snapshots around
  pacman transactions, wired to the snapper `root` config by the installer.

### Changed
- `aur.packages`: drop `phonto` and `mpvpaper`. The shell's live (video)
  wallpaper backend is now `ryoku-livewall`, which ships in the `ryoku-shell`
  package from the `[ryoku]` repo, so no AUR video daemon is needed. Both old
  backends were client GL pipelines (300-700 MB RSS, mpvpaper also leaked per
  loop) that could not get under the 100 MB target, while livewall
  software-decodes a downscaled clip into `wl_shm` and holds ~40 MB on any GPU
  vendor. `base.packages`: drop `gst-plugin-va`, which was only phonto's VAAPI
  decode plugin; `gst-libav` stays for general GStreamer codec coverage, and
  `awww-git` (the image daemon) is unchanged.
- `aur.packages`: drop `wallust`. It moved to the `[ryoku]` repo as a hard
  `ryoku-desktop` dependency (its AUR package pins a checksum against Codeberg's
  auto-generated source archive, which Codeberg regenerates non-reproducibly, so
  the pin drifts and the build fails), so the AUR set no longer carries it.
- `aur.packages`: drop `awww-git`. The image wallpaper daemon `awww` moved to the
  `[ryoku]` repo as a hard `ryoku-desktop` dependency (`release/packages/awww`), so
  every install and `ryoku update` carries it. As an AUR package it was skipped on
  offline installs and best-effort on a failed build, so a fresh box could come up
  with no wallpaper daemon at all.
- `base.packages`: add `rust`, and `dev.packages`: drop it. The shell's wallpaper
  daemon (`awww`) and other AUR dependencies are Rust programs, so the toolchain
  belongs in the always-installed base set, not the optional dev toolchains; every
  fresh install can build them.

### Fixed
- `hardware.packages`: the `amd-nvidia` profile now pulls `[amd]` + `[intel]`
  microcode. It is offered for an AMD or Intel CPU with an NVIDIA GPU but only
  shipped `amd-ucode`, so an Intel+NVIDIA laptop got the wrong microcode and no
  `intel-ucode`; the mkinitcpio microcode hook keeps only the one matching the
  CPU present, so shipping both is correct on either.
- `base.packages`: ship `papirus-icon-theme`. The shipped Qt icon theme
  (`ryoku/shell/qt6ct/qt6ct.conf`) is `Papirus-Dark`, and `adwaita-icon-theme`
  alone left named freedesktop icons (e.g. `network-wired`, which the Avahi
  desktop entries use) unresolved, since Adwaita carries them only as
  `-symbolic`. With Papirus present the launcher's all-apps grid renders every
  entry's logo instead of a broken-image placeholder.
- `base.packages`: add the desktop session pieces a plain Hyprland needs to render
  and function: `xorg-xwayland`, `hyprpolkit-agent`, `qt6-wayland`, `qt6ct`,
  `xdg-desktop-portal-gtk`, and `adwaita-icon-theme`. Without them the installed
  desktop failed (no Xwayland binary, no polkit agent, unthemed Qt/GTK apps).
- `base.packages`: move the wallpaper daemon `awww` to `aur.packages` as
  `awww-git`. It is AUR-only (upstream renamed swww to awww), so listing it in the
  pacstrapped base set aborted the whole install with "target not found: awww".
- `aur.packages`: switch the cursor theme from the source `bibata-cursor-theme` to
  the prebuilt `bibata-cursor-theme-bin`. Both install the whole Bibata family
  (Modern and Original in Ice, Amber, Classic), but the source build needs
  `python-clickgen` and can fail, which left the Ryoku Settings cursor picker with
  only a fallback theme; the `-bin` package never compiles. The stale comment
  (it claimed XCURSOR_THEME=Bibata-Modern-Classic) now matches the real default,
  Bibata-Modern-Ice.
