# system/packages/

The package lists that make up a Ryoku machine. One package per line; blank lines
and lines starting with `#` are ignored. The installer reads these at install
time (`RYOKU_REPO/system/packages/`), filtered through `RYOKU_DROP_PACKAGES`:
the apps checklist and the browser pick remove their deselections from the
transaction, so a removed app is never installed (and never put back by the
update manifest, which reads the same removal from the doctor's ledger).

## The lists

- `base.packages` The set every machine gets, installed by `pacstrap`. Core
  system, the boot chain, networking, audio, plain Hyprland, the SDDM greeter and
  qylock dependencies, laptop idle/battery support (`hypridle`, `upower`), OpenRGB
  for wallpaper-driven keyboard/LED color, the terminal apps (kitty, nautilus,
  chromium, mpv), the shell stack (fish, starship, fastfetch and friends), and
  fonts: Inter for Ryoku UI text, Noto Emoji, and Noto CJK so Japanese labels and
  the 力 mark render in browsers and Qt.
- `hardware.packages` Per-profile microcode and GPU drivers, grouped into
  `[amd]`, `[intel]`, `[nvidia]`, and `[vm]` sections. The installer picks the
  section(s) for the chosen `RYOKU_PROFILE` (`amd-nvidia` takes `[amd]` and
  `[nvidia]`) and adds them to the pacstrap set. Base `mesa` and the Vulkan
  loader live in `base.packages`, so these sections hold only the vendor extras.
- `aur.packages` Things that come from the AUR. These are not installed by
  pacstrap (the base system has no AUR helper yet); they are built later during
  post-install. Includes the Limine integration hooks and the Bibata cursors.
- `dev.packages` Developer toolchains (Go, Node/npm, Rust, Python/pip, mise),
  installed by `pacstrap` with the base set so every machine is dev-ready.

## Adding a package

Put it in the section it belongs to, keep it on its own line, and prefer the
official repos (`base.packages`) over the AUR when both have it. Hardware drivers
go under the right profile section in `hardware.packages`.
