# What ships preinstalled

Ryoku is opinionated on purpose. A fresh install is not a bare Arch box you
spend a weekend furnishing; it boots straight into a finished desktop with the
apps, tools, and toolchains already in place. This page is the honest inventory:
every application, CLI, TUI, daemon, and dependency that lands on a machine by
default, and roughly how much disk it all takes.

If you just want the short version: it's about **90 packages** from the Arch
repos, **~20 first-party Ryoku packages** from the `[ryoku]` repo, a handful of
developer toolchains, a small AUR set built after install, and, on the CachyOS
image, the CachyOS system layer. All told, a fresh box lands at roughly
**13–15 GiB** on disk.

## How the default set is assembled

Nothing here is magic; it comes from four well-defined places, in this order.
The installer's window-manager, browser, and apps steps (see
`installation/tui/apps.go`) can remove entries from the default set: what you
drop is filtered out of every step below and recorded so `ryoku update` never
puts it back.

1. **`pacstrap`** lays down the base system from `system/packages/base.packages`,
   the developer toolchains from `dev.packages`, and the microcode for your CPU
   from `hardware.packages`. On the CachyOS image it also pacstraps
   `cachyos.packages`. This always happens, online or offline.
2. **The desktop** comes from the signed `[ryoku]` repo: the installer runs
   `pacman -S ryoku-keyring ryoku-desktop`, and the `ryoku-desktop` umbrella
   drags in every first-party component and its hard dependencies.
3. **The AUR set** (`aur.packages`) is built in the post-install step. It needs
   a network, so it's best-effort: the install still completes without it and
   the pieces arrive later.
4. **Hardware conditionals** add only the provider a machine needs. Broadcom
   Wi-Fi gets `broadcom-wl`; a supported ASUS Aura laptop keyboard gets
   `asusctl`. Both packages are available to the offline installer.

The lists live in `system/packages/` and the umbrella is
`release/packages/ryoku-desktop/PKGBUILD`. Those files are the source of truth;
this page is the readable tour.

## The Ryoku desktop itself

These are Ryoku's own packages and default applications, served from the
`[ryoku]` repo. Most build from this monorepo; pinned upstream apps such as
RyoManager follow the same signed update path. The `ryoku-desktop` umbrella
version-pins the desktop components so a partial upgrade never leaves the shell
QML running against a stale plugin.

| Package | What it is |
|---|---|
| `ryoku-shell` | The Quickshell desktop: bar, pill, launcher, overview, wallpaper, visualizer, ryoshot capture, and the lock screen |
| `ryoku-hub` | Ryoku Settings, the control center (Appearance, Desktop, Windows, Devices, Connections, Apps, Accounts, System, and Updates) |
| `ryoku` | The `ryoku` CLI: `update`, `doctor`, `materialize`, rollback, snapshots |
| `ryoku-rashin` | Optional Rashin package: read-only Ask on free models, external coding harnesses in Agent, project and system Index, the Lithic app and the bar client. It is opt in, removable, and uses zero resources while off or absent |
| `ryoku-blobs` | The shared `Ryoku.Blobs`, `Ryoku.Spatial3D`, `Ryoku.Bus` and `Ryoku.Text` QML plugins |
| `gpk` | GlazePKG, the RyokuArch package manager |
| `ryogami` | The wallpaper daemon the shell drives: static, live, and shader transitions |
| `ryomanager` | RyoManager, the system monitor and process manager. It is selected by default in the installer and can be removed without doctor putting it back |
| `ryostore` | RyoStore: the catalogue of rices, bundles, subshells and bar styles |
| `ryotunes` | Ryotunes, the music player. It is selected by default in the installer and remains optional |
| `ryoport` | Ryoport, the fleet dashboard for virtual machines, SSH servers, and remote consoles. It is selected by default in the installer and remains optional |
| `ryoku-hypr-plugins`, `hyprglass`, `imgborders`, `hypr-dynamic-cursors` | Hyprland compositor plugins Ryoku Settings can toggle |
| `hyprland-preview-share-picker` | The screen-share source chooser the portal calls by name |
| `ryoku-cursors`, `ryoku-cursor-material` | The Bibata cursor set (the default pointer) and the optional Material variant |
| `otf-space-grotesk` | Space Grotesk, the legacy UI sans |
| `ttf-geologica` | Geologica, the UI sans |
| `ryoku-keyring` | The signing keys for the `[ryoku]` repo |
| `limine-mkinitcpio-hook`, `limine-snapper-sync` | Boot-menu regeneration and the snapshots submenu |

`ryoshot` (the screenshot studio) and the launcher live inside `ryoku-shell`.

The desktop package also includes two ready-to-apply rices: **Default** and
**NOC**. Each carries its own wallpaper, preview, shell look, and Fastfetch
style.

## Core system and boot

The bones. Kernel, initramfs, the encrypted-Btrfs boot chain, and snapshots.

| Package | Role |
|---|---|
| `base`, `base-devel` | The core system and build tools |
| `linux`, `linux-firmware`, `linux-headers` | The Arch edition's stock kernel, firmware, and DKMS headers; it remains the fallback when Gaming with CachyOS is selected |
| `rust` | Shipped in the base set because several AUR dependencies are Rust programs; every box can always build them |
| `dkms`, `mkinitcpio` | Out-of-tree kernel modules; the initramfs generator |
| `sudo` | Privilege escalation |
| `btrfs-progs`, `cryptsetup`, `dosfstools`, `efibootmgr` | Btrfs tools, LUKS, the FAT ESP, EFI boot entries |
| `limine`, `plymouth` | The bootloader and the boot splash |
| `snapper`, `snap-pac` | Btrfs snapshots, taken automatically around every pacman transaction |

## Networking, Bluetooth, and audio

| Package | Role |
|---|---|
| `networkmanager`, `iwd`, `wpa_supplicant` | Networking and the two Wi-Fi backends (iwd by default; wpa_supplicant for WPA3 edge cases) |
| `wireless-regdb`, `iw` | The Wi-Fi regulatory database and `iw`; the installer sets the regulatory domain from geolocation or the locale and `ryoku-wifi-regdom` keeps it set, so 5 GHz channels are enabled. `iw` also toggles power-save for Game Mode |
| `bluez`, `bluez-utils` | The Bluetooth stack and `bluetoothctl` |
| `pipewire`, `pipewire-alsa`, `pipewire-pulse`, `pipewire-audio`, `wireplumber` | The PipeWire audio stack, including the Bluetooth A2DP codecs |
| `alsa-utils` | `alsamixer` and friends, so audio stays diagnosable by hand |
| `rtkit` | Realtime scheduling for the audio thread, so it doesn't crackle under load |

## Graphics and the desktop session

| Package | Role |
|---|---|
| `mesa`, `vulkan-icd-loader` | The GL and Vulkan base (vendor drivers are added per profile) |
| `hyprland`, `xorg-xwayland` | The Wayland compositor and X11 app support |
| `hyprpolkitagent` | The polkit authentication agent |
| `xdg-desktop-portal-hyprland`, `xdg-desktop-portal-gtk` | Screen-share and file-chooser portals |
| `xdg-user-dirs` | The standard home directories |
| `qt6-wayland`, `qt6ct`, `qt6-declarative`, `qt6-5compat`, `qt6-svg`, `qt6-multimedia`, `qt6-multimedia-ffmpeg` | The Qt6 runtime the shell is built on |
| `gst-plugins-base/good/bad/ugly`, `gst-libav` | GStreamer media codecs |
| `adwaita-icon-theme`, `gnome-themes-extra`, `papirus-icon-theme` | The dark GTK theme and the icon themes the shell and apps resolve against |
| `vimix-cursors` | A second cursor theme, on top of the default Bibata set |
| `flatpak` | The portable app channel alongside the Arch and `[ryoku]` repos |

## Login and lock

| Package | Role |
|---|---|
| `sddm` | The login greeter |
| `polkit`, `gnome-keyring` | Authorization and the secrets store |

## Wayland tools

The small daemons and utilities the desktop leans on every session.

| Package | Role |
|---|---|
| `matugen` | The Material-You palette engine that recolors everything from the wallpaper |
| `brightnessctl` | Backlight control |
| `hypridle` | Idle and lock management |
| `upower`, `power-profiles-daemon` | Battery status and power profiles |
| `fuzzel` | A fallback launcher |
| `grim`, `slurp` | Screenshot capture and region selection |
| `playerctl` | Media playback control over MPRIS |
| `wl-clipboard` | The Wayland clipboard |

## Everyday apps

| Package | Role |
|---|---|
| `firefox` / `chromium` / Zen (`zen-browser-bin`) | The web browser: the installer picks exactly one of the three, with Firefox recommended, or none at all |
| `kitty` | The default terminal |
| `mpv`, `mpv-mpris` | The media player, wired onto the players bus |
| `nautilus`, `nautilus-python` | The file manager and its "Install / Compress / Send with Ryoku" right-click actions |

Every application on this page is yours to remove. Ryoku installs an app once
(on the ISO, or on the first `ryoku update` after a release adds it) and records
that it did; delete it afterwards and no update reinstalls it. (`ryoku doctor`
keeps that ledger in `~/.local/state/ryoku/provisioned`; drop a name from the
file to be offered the app again.) The tools the desktop itself calls, like
`grim` or `matugen`, stay hard dependencies.

## Terminal, CLI, and TUI

The command line Ryoku hands you is already comfortable.

| Package | Kind | Role |
|---|---|---|
| `fish` | Shell | The recommended account shell |
| `bash` | Shell | Account-wide alternative with `blesh` highlighting and suggestions |
| `zsh` | Shell | Account-wide alternative with Ryoku Oh My Zsh |
| `zsh-autosuggestions`, `zsh-syntax-highlighting`, `zsh-history-substring-search` | Shell | Fish-like editing for Zsh |
| `starship` | Prompt | The shared prompt |
| `bash-completion` | CLI | Bash completions |
| `bat` | CLI | `cat` with syntax highlighting |
| `eza` | CLI | A modern `ls` |
| `fd` | CLI | A friendly `find` |
| `fzf` | CLI | Fuzzy finder |
| `ripgrep` | CLI | Fast search |
| `zoxide` | CLI | Smart `cd` |
| `fastfetch` | CLI | The system-info readout |
| `git`, `github-cli` | CLI | Version control and `gh` |
| `jq` | CLI | JSON on the command line |
| `btop` | TUI | The resource monitor |
| `lazygit` | TUI | A git UI |
| `neovim` | TUI | The editor |
| `yazi` | TUI | A file manager |
| `pciutils` | CLI | `lspci`, used to detect your GPU during install |

## Shell runtime helpers

Tools the desktop, the pill, and the capture stack shell out to by name.

| Package | Role |
|---|---|
| `quickshell` | The framework the whole shell UI runs on |
| `cava` | The audio visualizer |
| `hyprpicker` | The color picker |
| `imagemagick` | Image processing |
| `waifu2x-ncnn-vulkan` | The GPU AI upscaler behind ryoshot's HD export and ryowalls' Enhance |
| `openrgb` | Per-device keyboard, mouse, RAM and motherboard lighting that follows the wallpaper palette, configurable from the wallpaper picker (Super + W) > Settings > Lighting |
| `asusctl` | Native built-in Aura keyboard lighting on supported ASUS laptops; selected by hardware rather than installed everywhere |
| `ddcutil` | External-monitor brightness over DDC/CI (the pill's DISPLAY faders) |
| `ffmpeg`, `yt-dlp` | Media transcode and download; also the launcher's live-radio play path |
| `curl`, `libnotify`, `xdg-utils`, `desktop-file-utils`, `python` | The plumbing the stash, scripts, and helpers rely on |

## Screen and capture toolkit

Everything behind the pill's Super+D, Super+U, and voice tools.

| Package | Role |
|---|---|
| `tesseract`, `tesseract-data-eng` | OCR text grab |
| `zbar` | QR-code scanning |
| `gpu-screen-recorder` | GPU-only screen capture and encode, replay-capable, on every compositor |
| `hyprsunset` (Hyprland), `wlsunset` (niri) | The night-light color-temperature toggle: each variant ships its compositor's gamma backend |
| `wtype` | Types Rashin's screen-control text into the focused app |
| `libqalculate` | The launcher's calculator backend |
| `songrec` | The launcher's Recognize Music |

## Developer toolchains

Every Ryoku machine is dev-ready the moment it boots.

| Package | Role |
|---|---|
| `go` | The Go toolchain |
| `nodejs`, `npm` | Node and npm |
| `python`, `python-pip`, `python-pipx` | Python with pip and pipx |
| `mise` | A runtime version manager |

## Gaming and virtualization

| Package | Role |
|---|---|
| `gamescope` | A nested micro-compositor that runs a game isolated from the desktop |
| `gamemode` | The performance governor Steam invokes |
| `mangohud` | The FPS and frametime overlay |
| `qemu-desktop`, `edk2-ovmf`, `virglrenderer` | The base VM stack: QEMU, UEFI firmware, and host-GL acceleration |

The installer has one Gaming question after Rashin and before Apps & tools. It
defaults to No and follows the edition:

- **Arch, Gaming with CachyOS:** requires a network and x86-64-v3 CPU. It adds
  the CachyOS kernel and headers, tuning, Ananicy rules, sched-ext schedulers,
  Steam, Proton-CachyOS SLR, Wine-CachyOS, `umu-launcher`, `winetricks`, and
  `protontricks`. The CachyOS kernel becomes the default, with Arch as the
  fallback when its NVIDIA module is usable. The same set is available later as
  RyoStore's **CachyOS Gaming** bundle.
- **CachyOS, Gaming:** keeps `steam` and `cachyos-gaming-meta` from the offline
  image. The edition already has the CachyOS kernel and tuning.
- **Void, Gaming:** keeps Steam and the 32-bit graphics baseline from the
  offline image. Its hardware lane adds the matching 32-bit Vulkan driver or
  NVIDIA libraries. Steam uses its own Proton.

The heavier passthrough VM pieces (`libvirt`, `looking-glass`, `swtpm`,
`quickemu`, and so on) are installed on demand from Ryoku Settings, not by
default.

## The CachyOS layer (CachyOS image only)

The CachyOS ISO boots a CachyOS live system and installs a CachyOS target with
the Ryoku desktop on top. The generic `[cachyos]` repository is above Arch's
repositories, CachyOS's branding script sets `/etc/os-release` during setup, and
`ryoku-base-os` and Ryoku Settings report CachyOS from the first boot.

| Package | Role |
|---|---|
| `linux-cachyos`, `linux-cachyos-headers` | The CachyOS kernel, booted by default, and its DKMS headers |
| `linux-cachyos-lts`, `linux-cachyos-lts-headers` | The CachyOS LTS fallback kernel and headers |
| `cachyos-settings`, `ananicy-cpp`, `cachyos-ananicy-rules`, `scx-scheds` | CachyOS tuning, process priorities, and sched-ext schedulers |
| `cachyos-keyring`, `cachyos-mirrorlist`, `cachyos-v3-mirrorlist`, `cachyos-v4-mirrorlist` | Repository trust and mirrors |
| `cachyos-rate-mirrors` | Ranks the Arch and CachyOS mirrors |
| `cachyos-hooks`, `chwd` | CachyOS's pacman hooks and hardware tool |
| `lsb-release` | Standard distribution identity reporting |
| `shelly`, `shelly-flatpak-backend` | CachyOS's graphical package manager, including its separately packaged Flatpak support |
| `cachyos-hello` | CachyOS Welcome in the launcher; its skel autostart is removed and blocked with `NoExtract`, so Ryoku Welcome remains the only first-login welcome |
| `cachyos-kernel-manager`, `cachyos-packageinstaller`, `btrfs-assistant` | Kernel, package, and snapshot tools |
| `yay` | The AUR helper, from `[cachyos]` in place of the AUR's `yay-bin` |
| `zlib-ng-compat`, `lib32-zlib-ng-compat` | CachyOS's zlib replacements, named up front so the first update has nothing to swap |

Ryoku deliberately leaves out `cachyos-fish-config`,
`cachyos-zsh-config`, `cachyos-wallpapers`, the CachyOS Plymouth themes,
`cachyos-snapper-support`, `cachy-update`, and `ufw` because Ryoku owns those
parts. It uses Shelly instead of `octopi`, already ships `yay` instead of
`paru`, and leaves GPU selection to Ryoku rather than running
`chwd --autoconfigure`.

Steam and Proton are not unconditional CachyOS packages. They come from the
Gaming question, which defaults to No.

## CPU microcode (per profile)

The installer picks these from `hardware.packages` based on your profile. GPU
drivers aren't listed here; separate scripts pick the generation-correct driver
for the card they detect.

| Profile | Package |
|---|---|
| `amd` | `amd-ucode` |
| `intel` | `intel-ucode` |
| `amd-nvidia` | `amd-ucode` + `intel-ucode` (the NVIDIA driver is chosen by a driver script) |
| `vm` | none needed |

## From the AUR (built after install)

These build in the post-install step. They need a network, so they're
best-effort: a fresh box comes up fine without them and they arrive on the next
pass. On the CachyOS image, `yay` and the Zen browser come from the CachyOS
repositories instead.

| Package | Role |
|---|---|
| `yay-bin` | The AUR helper, bootstrapped first so the rest can build |
| `localsend-bin` | AirDrop-style LAN file sharing, spoken by the file stash |
| `nvibrant-bin` | NVIDIA digital vibrance for the pill's saturation fader |
| `game-devices-udev` | udev rules and battery reporting for DualSense and Switch Pro pads |
| `xpadneo-dkms` | The Xbox One/Series wireless controller driver |
| `phinger-cursors`, `catppuccin-cursors-mocha`, `apple_cursor` | Extra cursor themes the cursor picker in Ryoku Settings > Appearance > Fonts & icons offers |
| `ttf-fraunces-variable` | Fraunces, the editorial serif in the brand type stack |

## Fonts

`inter-font`, `otf-space-grotesk`, `noto-fonts`, `noto-fonts-cjk`,
`noto-fonts-emoji`, `ttf-jetbrains-mono-nerd`, `ttf-firacode-nerd`,
`ttf-hack-nerd`, and `ttf-material-symbols-variable`. Between them they cover the
Ryoku UI text, Japanese labels and the 力 mark, emoji, terminal glyphs, and the
Material Symbols the bar and pill icons are drawn from. `ttf-fraunces-variable`
comes from the AUR set above.

## Storage footprint

The installer measures this so it can size the root partition, and the numbers
live in `installation/backend/lib/disk.sh` and `filesystem.sh`.

| Figure | Value |
|---|---|
| The `base + dev + desktop` package closure (installed size) | **~13–15 GiB** |
| Bytes physically written during the install | **~13 GiB** (before Btrfs zstd:1 compression, so a bit less on disk) |
| The root-partition floor the installer enforces | **20 GiB** |
| Minimum free space for an alongside (dual-boot) install | `2 + 20 + swap` GiB |
| Minimum target disk for a whole-disk install | **32 GiB** |

So a fresh Ryoku desktop is roughly **13–15 GiB** of real data. The 20 GiB root
floor is deliberately roomier than that: the extra headroom is for the AUR
builds, for Btrfs snapshots, and for the things that download on first launch
(any Flatpaks you add, a voice model, a game). The CachyOS image sits at the upper
end of the range because it carries a second kernel and CachyOS's desktop tools.
Steam, Wine, and Proton add to that only when Gaming is selected. The swapfile is
separate and off by default (`RYOKU_SWAP_GIB=0`).

These are the repo's own measured figures. If you want an exact byte count for a
specific profile and variant, resolve the full dependency closure against live
mirrors (`pacman -Sp --print-format '%s'` summed) on the target.
