# Changelog: system/hardware/

## Unreleased

### Added
- `display/ryoku-monitor`: **a hand-entered resolution is forced, not ignored.**
  When a layout carries a `WxH@rate` mode the panel does not advertise (the Hub's
  new Displays "Custom…" entry), `apply`/`save` generate a CVT reduced-blanking
  modeline (`cvt`, from the already-present libxcvt) and use that, which Hyprland
  accepts as a forced timing; advertised modes pass through unchanged. Covered by
  `tests/monitor-custom-mode.sh`.

### Fixed
- `power/ryoku-power-cutover`: **a killed generation guard releases its
  locks.** The hold keeps the launch and generation flocks exclusive while a
  cutover swaps lockscreen generations, and its keep-alive coprocess inherited
  both descriptors; a holder killed mid-swap left an orphaned sleep pinning
  the locks, and every later cutover waited on the guard forever. The
  keep-alive now starts without the lock fds, so the flocks belong to the
  holder alone and die with it (`tests/power-cutover.sh` covers the kill and
  the reacquire).
- `power/ryoku-clamshell`: **every lid close has one fail-closed owner.** The
  daemon holds `handle-lid-switch` only while its login1 session is active and
  reacquires it after activity changes or a login1 restart. Hyprland's close
  and open binds and niri's native `lid-close`/`lid-open` events run the same
  policy: verified live docked mode (AC plus an external display) remains
  awake; every other close calls `ryoku-shell suspend`, which cannot proceed
  without compositor-secure qylock. ACPI supplies physical state when present;
  UPower seeds an already-closed startup otherwise, but its asynchronous value
  cannot veto an ordered compositor close edge. Losing the dock while already
  closed uses the same transaction, and a close rejected by an update guard
  retries until the matching open edge. Hyprland serializes its panel handoff
  and restores only connectors this helper disabled; niri keeps native
  topology. Package hooks, login and checkout deploy share a guarded
  session-lifecycle handoff. It selects the confirmed foreground login1
  session, secures every other online same-user session with its own qylock and
  observer, then replaces shell, idle, lid and wallpaper owners. The watchers
  and rebind transients have no finite restart burst. Failed adoption stays
  blocked until retry or reboot.
- `power/logind-ryoku-lid.conf`: **login1 supplies the safe lid fallback and the
  final bounded lock handshake.** Undocked lid closes suspend when no active
  session inhibitor owns them, docked closes remain ignored, and
  `InhibitDelayMaxSec` gives the foreground shell time to finish its one delay
  handshake. Inactive sessions are secured before ownership moves rather than
  claiming duplicate per-user daemon delay FDs.
- `display/ryoku-monitor`: **an active monitor is no longer treated as disabled
  on Hyprland builds that mislabel it.** hyprland-git reports `"disabled": true`
  for a plainly active output (focused, DPMS on, a real mode, an active
  workspace); Ryoku trusted the flag, so every `select(.disabled | not)` dropped
  the live panel -- no scale, wrong `GDK_SCALE` -- and `write_monitors_conf`
  persisted `disabled = true` into `monitors.lua`, disabling it for real on the
  next login (the Hub also showed it disabled, and workspaces on it broke).
  `monitors_json` now derives disabled from the mode (a genuinely off output has
  no resolution, `0x0`), which is stable across Hyprland versions and keeps a
  DPMS-asleep panel enabled.
- `display/ryoku-monitor`: **display settings survive a reboot and a power-cycled
  TV; an HDMI output no longer reverts as if freshly connected.** A saved layout
  was recalled only when the connected identity set matched the saved one
  exactly, so an LG TV that reports a different (or empty) serial between boots,
  or is still asleep when `autoscale` runs at login, invalidated the whole layout
  and dropped it to DPI scaling; `monitors.lua` was then regenerated from the
  live outputs alone, dropping the missing display's stanza entirely. Matching is
  now per-display and identity-first: make/model/serial, then make/model when a
  serial drifts, with an EDID-less display keyed on its connector. A saved
  display absent at that instant keeps its stanza, and a connector rename remaps
  it; booting the same disk on another machine still falls back to DPI rather
  than draping one panel's settings over a different panel on the same port. The
  generated `monitors.lua` header now points hand edits at
  `~/.config/hypr/monitors_user.lua`, which is loaded after it and never
  rewritten. Covered by `tests/monitor-persist.sh` (#152).
- `bluetooth/ryoku-bluetooth-reset.service`: **a Bluetooth audio device that
  connected then dropped a second later now stays connected.** BlueZ 5.83+
  actively disconnects a device whose authentication is retried mid-stream (an
  upstream regression, bluez#1545); the reliable remedy is restarting the
  bluetooth service once the session audio stack (WirePlumber, which owns BlueZ's
  A2DP endpoints) is up. A new user-session unit does exactly that, gated on a
  Bluetooth radio being present and ordered after WirePlumber, with a scoped
  polkit rule (`54-ryoku-bluetooth-a2dp.rules`) so it needs no password.
  Restarting before anything is connected clears the race with no audible drop;
  if polkit ever denies it the fix simply no-ops, never a regression (#63).
- `display/ryoku-hw-backlight`: **brightness keys work on multi-backlight
  laptops instead of driving a dead device.** A laptop can register several
  `/sys/class/backlight` devices -- one per backlight-capable DRM connector, plus
  a phantom for the discrete GPU (e.g. `amdgpu_bl0` for a disconnected connector
  beside `amdgpu_bl2` for the real panel, or `nvidia_0` next to `amdgpu_bl*`). The
  old fixed name-priority list (which omarchy and end-4/brightnessctl also use)
  grabbed the first `amdgpu_bl*` or a device that controls nothing, so the keys
  went dead. It now selects by ground truth -- the backlight whose DRM connector
  is a CONNECTED internal panel (eDP/LVDS/DSI), resolving the discrete-GPU case
  through its PCI node -- and falls back to the name list only when no connector
  mapping exists. Covered by `tests/backlight-pick.sh`.
- `audio/99-ryoku-audio-powersave.conf`: **the headphone hiss/whine that only
  sounds while audio plays is gone.** The kernel default (`snd_hda_intel
  power_save=10`) powers the codec and its headphone amp down ~10s after a stream
  stops and back up on playback; on many codecs the amp's noise floor is a faint
  high-frequency whine audible only while it is energised -- the "mosquito" /
  tinnitus hiss on headphones, gone at idle. A new `/usr/lib/modprobe.d` drop-in
  sets `power_save=0 power_save_controller=N` so the codec stays steady (nothing
  re-writes it at runtime: power is power-profiles-daemon, not TLP). Shipped in
  `ryoku-desktop` so it reaches everyone on update; no reboot needed to test
  (`echo 0 | sudo tee /sys/module/snd_hda_intel/parameters/power_save`). Costs
  ~0.3-0.5 W idle on laptops; override in `/etc/modprobe.d` to keep the saving on
  hardware that does not hiss.
- `input/72-ryoku-keyboard-uaccess.rules`: **the recorder's "show keyboard"
  keycast works for every user, not just the install account.** The daemon reads
  keyboard evdev to draw the keypress overlay, but keyboards are `root:input` and
  the installer only puts freshly-created accounts in the `input` group -- so
  upgraders and pre-existing users hit "Cannot read keyboard input: permission
  denied." A new udev rule grants the active-seat user a `uaccess` ACL on
  keyboards (the approach the references recommend over the broad `input` group),
  shipped in `ryoku-desktop` so it reaches everyone on update, no reboot.
- `display/ryoku-hw-backlight-fix`: **stops breaking brightness on Intel+NVIDIA
  laptops (#54).** The AMD-only `acpi_backlight=native` quirk gated on
  `nvidia_wmi_ec_backlight` present and no `amdgpu_bl*` -- but that NVIDIA WMI EC
  backlight also registers on Intel+NVIDIA machines with no AMD GPU, so the quirk
  fired there and fought the correct `i915.enable_dpcd_backlight=1` fix. It now
  requires a real AMD GPU (PCI vendor `0x1002` on a DRM card) before anything
  else, so it only touches the ASUS AMD+NVIDIA panels it was meant for.
- `display/ryoku-monitor`: **GTK apps stay crisp on fractional monitors, and the
  scale stepper no longer offers sub-1.** `GDK_SCALE` now rounds to the nearest
  whole scale when the monitors agree (a 1.5x panel gets 2, so GTK renders at 2x
  and the compositor downscales, crisp) instead of falling to 1 and upscaling
  (blurry). The Hub's per-resolution scale ladder now stops at 1x: below 1,
  GTK/Qt/XWayland apps render in a sub-region with empty margin, so it was never
  a usable choice.
- `power/ryoku-power`: **the CPU frequency slider stops inventing a cap.**
  `capabilities` reported `maxFreqPct` as `scaling_max_freq` over the boost
  ceiling, and on `amd-pstate-epp` that is not a cap: `scaling_max_freq` rests at
  the nominal clock while boost is delivered through the CPPC perf request. So an
  entirely unconstrained machine reported **76%** and the Machine page drew the
  slider three quarters down, which is exactly the display that invites the one
  interaction that makes it true. Once a `maxFreqPct` is stored,
  `write_cpu_profile` pins `scaling_max_freq` on every core at every login and
  every profile switch, in a retry loop built to outlast ppd, and the slider floor
  is 20% (about 1.05 GHz here). An unset knob now reports 100, because uncapped is
  the truth on a machine nobody has capped; a cap set outside Ryoku is not guessed
  at, since there is no way to tell one from the driver idling at nominal.
- `power/ryoku-power`: **a defined knob can be undone.** `charge-limit` has always
  had a `clear`; profiles did not. `profile set <p> <key> ""` is refused by the
  validators and nothing else removed a key, so a definition could only be undone
  by editing `power.json` by hand, which is the thing shipped config is not
  supposed to need. `profile clear <p> [<key>]` un-defines one knob or the whole
  profile. `tests/power.sh` had been asserting the 76, so it was defending the
  bug; it now asserts an unconfigured machine reads 100.

### Added
- `bluetooth/99-ryoku-bt-autosuspend.conf`: **Bluetooth headphones stop dropping
  seconds after they connect.** `btusb` autosuspends the controller in an audio
  stream's idle gaps, and on combo Wi-Fi+BT chips (Intel AX2xx, Realtek RTL8761,
  MediaTek MT7921) the resume races the link. A `modprobe.d` drop-in sets
  `options btusb enable_autosuspend=0`, shipped by `ryoku-desktop`; reboot or
  `modprobe -r btusb && modprobe btusb` to apply. `tests/controllers.sh` pins it.
- `power/ryoku-game-tune` and `power/53-ryoku-game-tune.rules`: the system-level
  half of Game Mode. Deep CPU idle states off (C3 costs 350 us to leave on this
  hardware, C2 costs 18, so the choice is made by exit cost rather than by state
  name and the cheap ones keep working), the NMI watchdog off, split/bus-lock
  mitigation off, proactive compaction off, shorter page-lock waits, and
  swappiness down because swap here is a disk-backed file and eviction mid-frame
  costs a disk round trip. Every prior value is recorded before it is changed and
  replayed on restore, in `/run` so a reboot clears the record and the knobs
  together. A knob the running kernel does not expose is skipped, which is what
  lets one toggle serve both the Arch and CachyOS kernels.
  It deliberately does not touch CPU boost, `throttle_thermal_policy` or the
  PPT/TDP knobs: those were measured on this hardware and do nothing (writes are
  accepted, the package still pins the same wattage and the same 95 C), and a
  gaming toggle that flips placebos is worse than one that admits the ceiling.
  It also skips what a stock Arch install already gets right rather than
  rewriting it for show: the NVMe scheduler is already `none`, transparent huge
  pages already `always`, and `vm.max_map_count` already raised by `filesystem`.
- **A run with redirected paths refuses to escalate.** `pkexec` strips the
  environment, so escalating from a run whose paths were redirected would drop
  the redirection and apply to the real `/proc` and `/sys` instead. Found the
  hard way: an early version did exactly that during a test and tuned the live
  machine. The save/restore was exact so it came back clean, but the widened
  scope was the bug, and it is now a test.
- `tests/controllers.sh` and a `Controllers` workflow pin controller and
  Bluetooth-adapter support, which is almost entirely a delivery problem: every
  failure it guards is invisible on the build host and shows up only as "my
  controller does not work" on a real machine. It catches the ERTM option
  flipping back, a driver drifting into `aur.packages` where `ryoku update` can
  never reach it, a `base.packages` entry with no `[ryoku]` recipe for pacman to
  resolve, `meson` leaving the build toolchain, and `xone` arriving by default.
  Each of those five was introduced deliberately and confirmed to fail the test
  before the test was committed.
- `input/99-ryoku-controller.conf`: Xbox pads pair over Bluetooth instead of
  refusing to. Their L2CAP handshake does not survive the kernel's Enhanced
  Retransmission Mode, so with `disable_ertm` at its default `N` they either
  never finish pairing or connect and drop within seconds, which reads as a
  broken controller and is the most common "my Xbox pad will not connect on
  Linux" report. Shipped as a `modprobe.d` drop-in so it applies on first module
  load and survives a kernel change. Checked against xpadneo 0.10.4, which does
  not set this itself: its own `modprobe.d` file is only HID aliases binding
  Xbox VID/PIDs to `hid_xpadneo`, so the driver and this option are
  complementary rather than redundant.
- `audio/ryoku-bt-audio` remembers a Bluetooth device's A2DP codec and puts it
  back on every reconnect. WirePlumber persists a device's profile (headset vs
  A2DP) but not its codec, so a hand-picked codec was lost every single time the
  device came back, which is the part users otherwise redo forever. `set` refuses
  a codec the device never offered, because remembering one would mean retrying a
  doomed switch on each reconnect, and a device already on the wanted codec is
  left alone, because the switch renegotiates the A2DP link and audibly drops
  audio. No root: codec switching is a session operation through pipewire-pulse.
  Login autostart runs `watch`, which is inert for anyone who never picked one.

- `input/ryoku-hw-asus-aura` identifies laptops with a supported ASUS Aura
  keyboard from DMI product families or the `asus-nb-wmi` platform driver. The
  same quiet exit-status probe is used by both installers and `ryoku doctor`, so
  hardware selection has one definition.

- `power/ryoku-power`: the CPU side of the power profiles is now the user's to
  define. `capabilities --json` reports what the machine actually exposes
  (governor, EPP, a frequency ceiling as a percent, the ACPI platform profile),
  `profile get|set` stores a per-profile definition in `power.json`, and
  `apply-profile` writes it to sysfs. `apply` gained the CPU half so a stored
  definition also lands at login, not only on the next profile switch.
  Deliberately absent: CPU boost and the PPT/TDP limits, both measured as
  firmware placebos on this hardware.

  Three ordering and timing facts the implementation had to learn the hard way,
  each verified on real hardware. The platform profile is written first because
  `cpuinfo_max_freq` is dynamic on amd-pstate and a quiet profile clamps it, so a
  frequency written before the envelope widens is silently truncated. The governor
  pass then completes for every cpu before the EPP pass starts, because writing
  `scaling_governor` resets EPP to that governor's default. And the whole apply is
  a write-verify-retry loop under a lock, because ppd writes its own values
  asynchronously after the profile-changed signal and a single write loses that
  race. EPP options come from a governor-independent list: amd-pstate narrows
  `energy_performance_available_preferences` to just `performance` while the
  performance governor is in force, which would otherwise make a definition
  un-editable depending on the machine's current state.
- `power/ryoku-power`: the two laptop power levers that are global rather than
  per-profile -- battery charge limit and PCIe ASPM policy. Neither belongs to
  power-profiles-daemon, and on this hardware CPU boost and the PPT/TDP knobs are
  firmware-governed placebos (measured: writes accepted, but the package still
  pins the same wattage and 95 C), so they are not offered at all. The CPU knobs
  ppd does set are re-applied over it by the profile programmer above, not
  duplicated here. `charge-limit get|set <50-100>|clear` caps
  `power_supply/*/charge_control_end_threshold` so the pack is not held at 100%
  (the attribute reads ENODATA -- "No data available" -- until the first write,
  which `get` reports as `unset`); `aspm get|set <policy>` reads/writes
  `/sys/module/pcie_aspm/parameters/policy`, unwrapping the kernel's `[selected]`
  format and validating against the offered list. The desired state lives in
  `~/.config/ryoku/power.json` (`chargeLimit`, `aspm`) so a user edit survives
  updates; `apply` is the idempotent convergent entry point (laptop-only, like
  ryoku-idle; a desktop or knob-less box exits 0), and a missing file or key
  means "leave the hardware alone", never a forced default. A root-owned sysfs
  write it cannot do directly re-execs the whole command once under pkexec,
  authorized for the active wheel user by `47-ryoku-power.rules`; the
  `ryoku-desktop` PKGBUILD installs the helper via the hardware glob and the
  rule under `polkit-1/rules.d`.
- `power/47-ryoku-power.rules`: authorize `ryoku-power` for the active wheel
  user without a password, so the charge ceiling `apply` re-applies at login
  never blocks on a prompt. The threshold is a plain sysfs value that does not
  survive a reboot, so Hyprland autostart runs `ryoku-power apply` on every
  login; through an interactive `sudo` that would hang or fail with no terminal,
  and the stored `charge_control_end_threshold` would lapse silently. Unlike the
  WiFi power-save helper this one takes an argument, but the grant stays safe
  because it is a closed, validated set -- the charge limit is range-checked to
  50-100 and the ASPM policy must be one the kernel currently offers -- so
  nothing arbitrary reaches the privileged write. Installed to
  `/usr/share/polkit-1/rules.d` by `ryoku-desktop`, matching the network rules.
- `network/ryoku-wifi-regdom` + `network/48-ryoku-wifi-regdom.rules`: pin the
  Wi-Fi regulatory domain so 5 GHz works. A non-self-managed wiphy boots on world
  domain `00`, where most 5 GHz channels are disabled or no-IR, so a dual-band
  SSID is only ever seen on its 2.4 GHz BSS -- the "can only connect to 2.4 GHz"
  bug. Nothing set a country, so this helper does: `set <CC>` writes the single
  active `WIRELESS_REGDOM` line in the wireless-regdb backup file (re-applied at
  boot by its shipped udev rule), sets iwd's `[General] Country`, and runs `iw reg
  set` now; `apply` (idempotent, install/upgrade/boot) seeds iwd's `[Rank]
  BandModifier5GHz` when absent and re-asserts the country when the live domain
  drifted, never inventing one; `get`/`status` report for support. A polkit rule
  authorizes exactly this program for the active wheel user without a password (it
  takes only a validated two-letter code), matching the other network helpers. The
  `ryoku-desktop` PKGBUILD installs the helper via the hardware glob and the rule
  under `polkit-1/rules.d`; the `.install` runs `apply` on install + upgrade, and
  the installer seeds the country from geolocation or the locale.
- `audio/70-ryoku-maono.rules`: reach the control features on Maono USB mics.
  The PD400X and its siblings put gain, EQ, compressor, limiter and the monitor
  mix behind a vendor-defined HID page (usage page `0xFF01`) instead of the audio
  interface, and the kernel leaves both `/dev/hidraw*` and the raw USB node
  root-owned, so a control app running as the user cannot open either one. The
  rule grants the active-session user access (`uaccess`, no group setup), the
  same pattern as `60-ryoku-i2c.rules`. Shipped to `/usr/lib/udev` by
  `ryoku-desktop`. The four standard USB-audio controls (mic gain and mute,
  headphone level and mute) already worked without it.
- `leds/ryoku-leds`: `RYOKU_LEDS_DISABLE=1` turns the OpenRGB accent sync off.
  `ryoku-leds apply` runs from Hyprland autostart and again from the shell daemon
  on every wallpaper change, and there was no off switch short of forking the
  autostart module; the gate makes `apply` a no-op. Set `env = RYOKU_LEDS_DISABLE, 1`
  in Hyprland (Ryoku Settings > Environment, or `hypr/user.lua`) to keep LEDs dark
  across updates; `color`/`status` still report for debugging.
- `drivers/ryoku-nvidia-guard` + `drivers/nvidia.sh`: end the SDDM login loop on
  NVIDIA. nvidia.sh blacklists nouveau and forces DRM modeset while an nvidia
  module exists, but the `-dkms` branches (a custom kernel, or a pre-Turing/Kepler
  card on 580xx/470xx-dkms) rebuild per kernel; a failed rebuild left nouveau
  blacklisted with no nvidia module, so no driver bound the card -- the greeter
  drew on simpledrm but Hyprland could not, and SDDM looped the login. The new
  guard runs from nvidia.sh's pacman hook, which now also fires on kernel updates
  (a `usr/lib/modules/*/vmlinuz` trigger + `NeedsTargets`): when it finds that
  state it restores nouveau and rebuilds the initramfs, otherwise it just keeps
  the image in step with a driver update, and it no-ops on a non-NVIDIA box or
  mid-install. `ryoku doctor` writes the same hook onto boxes installed before it
  existed. Ships to `/usr/bin` via ryoku-desktop; covered by
  `tests/nvidia-guard.sh`.
- Battery-aware idle: `power/ryoku-idle` gains `on-battery`/`on-ac` (exit-status
  guards read the `/sys/class/power_supply` mains state), and the generated idle
  config pairs a battery-aggressive listener with an AC-relaxed one at each stage,
  gated by those guards. On battery the backlight dims at 2 min, the session
  locks at 5, the screen (DPMS) turns off at 5.5 and the machine suspends at 15;
  on AC the prior 5/10/11/30 hold. An unknown or absent mains reads as AC, so a
  desktop or an unreadable laptop keeps the relaxed policy.
- `audio/ryoku-restart-audio`: recover sound when it does not come back. Restarts
  the PipeWire stack (wireplumber, pipewire, pipewire-pulse) and resets a stuck
  USB audio device. Bound to Super+Shift+A. Ported from omarchy.
- `drivers/intel.sh` now installs `sof-firmware`: recent Intel laptops route audio
  through a DSP that stays silent without it.
- `bluetooth/ryoku-bluetooth-tune`: BlueZ pairing/reconnect tuning. bluez owns
  `/etc/bluetooth/main.conf` and has no drop-in dir, so this sets the keys in
  place: `Experimental` (device battery + newer profiles), `JustWorksRepairing`
  (re-pair after suspend), `FastConnectable`, `AutoEnable`. Run by the
  `ryoku-desktop` `.install` on install + upgrade.
- `display/ryoku-hw-backlight` + `ryoku-hw-backlight-fix`: brightness on ASUS
  AMD+NVIDIA laptops. The panel hangs off the AMD iGPU but the kernel registers
  only `nvidia_wmi_ec_backlight` and hides `amdgpu_bl0`, so brightness would not
  lower. The fix adds `acpi_backlight=native` (a hardware-gated limine drop-in) to
  reveal `amdgpu_bl0`, and `ryoku-cmd-brightness` now pins the real device with
  `brightnessctl -d`. `90-ryoku-backlight.rules` grants the `video` group write
  access. Ref: ArchWiki/Backlight, basecamp/omarchy#5067.
- `input/99-ryoku-uinput.conf`: load `uinput` at boot for Steam Input and the
  userspace game-controller drivers.
- `drivers/nvidia.sh`: detect NVIDIA by scanning `/sys/bus/pci` (vendor `0x10de`,
  display class) as well as `lspci`, so a card is found even when `pciutils` is
  absent in the install chroot (the reason nvidia drivers were silently skipped);
  also installs `lib32-nvidia-utils` when multilib is enabled.
- `network/50-ryoku-dns.rules`: the network panel's DNS switch applies without a
  password. `ryoku-dns` runs as root through pkexec but shipped no polkit grant,
  so the DNS buttons silently did nothing; a rule now authorizes any `ryoku-dns`
  helper (basename match, so a dev checkout's non-`/usr/bin` path also works) for
  the active wheel user, matching the WiFi power-save helper. Installed to
  `/usr/share/polkit-1/rules.d` by `ryoku-desktop`.
- `network/ryoku-dns`: persistent system-wide DNS provider switching for
  DHCP, Cloudflare, Google, and validated custom IPv4/IPv6 servers. The helper
  writes one NetworkManager global-DNS drop-in, reloads the active resolver
  state, and removes only its own drop-in when returning to DHCP; it is
  privilege-separated behind polkit and ships through `ryoku-desktop`.
- `ddc/`: external-monitor brightness over DDC/CI. `ryoku-i2c.conf` loads the
  `i2c-dev` module (`/etc/modules-load.d/`) so `ddcutil` can open `/dev/i2c-*`, and
  `60-ryoku-i2c.rules` grants the active-session user access (`uaccess`, no group
  setup). Drives the pill DISPLAY faders and the new `XF86MonBrightness` keys
  (`ryoku-cmd-brightness`). Shipped to `/etc` + `/usr/lib/udev` by `ryoku-desktop`.

### Removed
- `leds/ryoku-leds`: the old accent-to-all-devices helper is gone, along with
  the RYOKU_LEDS_* environment knobs (RYOKU_LEDS_DISABLE, RYOKU_LEDS_COLOR,
  RYOKU_LEDS_PALETTE, RYOKU_LEDS_FALLBACK, RYOKU_LEDS_MODE, RYOKU_LEDS_TIMEOUT).
  It applied the wallpaper palette accent to every OpenRGB device on every
  wallpaper change with no per-device opt-in, the behaviour users blame OpenRGB
  for. Use Settings > Appearance > Lighting and `ryoku-hub lighting` instead,
  which is opt-in per device and persists settings in `~/.config/ryoku/lighting.json`.

### Security
- `display/ryoku-monitor`: `apply_specs` now renders monitor string fields
  (`output`, `mode`, `position`, `mirror`) through jq's `@json`, which emits a
  properly escaped quoted literal, before interpolating them into the
  `hl.monitor({ ... })` Lua passed to `hyprctl eval`. The incoming layout JSON
  (from a saved profile or the Hub) could otherwise carry a value with an
  embedded `"` that broke out of the Lua string and injected code into the
  eval. Connector names from the kernel can't contain quotes, so the live path
  was not reachable, but user-supplied profile JSON is; the escape closes it
  with no behaviour change (empty/absent fields still render as `""`).

### Added
- `power/ryoku-clamshell`: macOS-style clamshell (closed-lid) mode for laptops.
  A laptop-only daemon (autostarted from Hyprland, like `ryoku-idle`) holds a
  systemd `handle-lid-switch` inhibitor while the machine is on AC power AND an
  external display is connected, so closing the lid keeps the session running on
  the external instead of suspending; it drops the inhibitor (and suspends if the
  lid is already shut) the moment either condition is lost. The `lid` subcommand,
  driven by the Hyprland lid-switch bind (`hypr/modules/lid.lua`), blanks the
  internal panel on close when an external is present and restores the layout on
  open. Event-driven via `udevadm monitor` (power_supply + drm), no polling.
- `power/logind-ryoku-lid.conf`: a logind drop-in (shipped by `ryoku-desktop` to
  `/etc/systemd/logind.conf.d/10-ryoku-lid.conf`) that sets `HandleLidSwitch`,
  `HandleLidSwitchExternalPower`, and `HandleLidSwitchDocked` all to `suspend`, so
  logind suspends on lid close in every case and `ryoku-clamshell` is the sole
  thing that keeps a closed lid awake -- power AND an external display, matching
  macOS (the default `docked=ignore` would keep it awake on battery too).
- `display/ryoku-monitor`: the Settings paths carry per-output colour management.
  `list` reports each monitor's `cm` (from Hyprland's `colorManagementPreset`,
  normalised to srgb/wide/hdr) and its SDR brightness; `apply`/`save`/`load` write
  `cm` with the bit depth it implies (sRGB -> 8-bit, Wide/HDR -> 10-bit) and, in
  HDR, `sdrbrightness`, into the `hl.monitor({ ... })` calls and the persisted
  layout. The colour spec is written on every enabled monitor rather than omitted
  at its default, so switching a display out of HDR live actually clears the
  10-bit / raised-brightness state instead of leaving it stuck. Covered by
  `tests/monitor-profiles.sh`.
- `gpu/ryoku-gpu-lib32`: installs the 32-bit (lib32) GPU userspace for the
  detected hardware, so 32-bit and Proton/DXVK games render on the real GPU
  instead of falling back to software. The base install and the 64-bit driver
  scripts are multilib-free; this runs after `[multilib]` is enabled (the Gaming
  bundle orders its `requires` as multilib, then gpu-lib32) and, reusing
  `ryoku-gpu-detect`, maps each GPU's loaded DRM driver to its Vulkan ICD
  (`amdgpu`/`radeon` -> `lib32-vulkan-radeon`, `i915`/`xe` ->
  `lib32-vulkan-intel`, `nvidia` -> `lib32-nvidia-utils`, `nouveau` ->
  `lib32-vulkan-nouveau`) on a `lib32-mesa` + `lib32-vulkan-icd-loader`
  baseline. A hybrid box gets both ICDs, Mesa once. Idempotent (pacman
  `--needed`), `RYOKU_DRYRUN=1` prints the plan. Shipped to `/usr/bin` by the
  `ryoku-desktop` hardware glob (and to `~/.local/bin` by the dev deploy);
  covered by `tests/gpu-lib32.sh`.
- `network/ryoku-wifi-powersave` + `network/49-ryoku-wifi-powersave.rules`: a
  privileged helper that disables, and later restores, 802.11 power-save on every
  WiFi device for Game Mode, via `iw` with no reconnect and no throughput cap,
  saving each device's prior state. A polkit rule authorizes exactly this program
  for the active wheel user without a password, so the deck toggle stays one click.
  The `ryoku-desktop` PKGBUILD installs the helper to `/usr/bin` and the rule under
  `/usr/share/polkit-1/rules.d`. Covered by `tests/wifi-powersave.sh`.
- `display/ryoku-monitor`: a `settle` subcommand re-asserts each output's intended
  mode so a display recovers in place when a cold-boot or post-upgrade link comes
  up advertising only a fallback resolution (e.g. 800x600) that Hyprland's
  `highrr`/`preferred` then pins. It generalises the old refresh-only settle to
  resolution too, reads intent from `monitors.lua` (so an explicit Ryoku Settings
  pick is restored, never overridden, and `monitors_user.lua` pins are skipped),
  and powers both `ryoku doctor` and the login/hotplug/Settings `autoscale` path.
  `settle --check` reports drift (exit 1) without changing anything.
- `gpu/ryoku-gpu`: a `detect --json` machine-readable GPU list and a `mode
  hybrid|performance|passthrough` switch (passthrough pins the iGPU alone, freeing
  the dGPU for a VM). Both feed the new Ryoku Settings -> GPU page and its
  Looking-Glass passthrough VM.
- `display/ryoku-monitor`: honours a hand-written `~/.config/hypr/monitors_user.lua`.
  Any output pinned there is left out of the generated `monitors.lua` and skipped
  by `autoscale` (scale and position), so a manually forced panel (a wrong/fake
  EDID that needs a custom mode or modeline) is never fought by auto-detection.
- `display/ryoku-monitor`: a GUI/profile surface for Ryoku Settings. `list` prints
  the connected monitors (identity, modes, layout) as JSON; `apply JSON` applies an
  explicit layout live and persists it with the chosen modes (not highrr); `save
  NAME JSON`/`load NAME`/`profiles`/`rm NAME` manage named layout profiles under
  `~/.config/ryoku/monitors/`. Profiles match on monitor hardware identity
  (make|model|serial), so they survive connector renames, and `autoscale` applies
  a matching profile at login/hotplug, falling back to DPI scaling when none fits
  (`--no-profile` forces DPI). Fixture mode (`RYOKU_MONITOR_JSON`) now skips the
  hyprctl requirement so the path is testable without a live compositor.
- `display/ryoku-monitor`: `mirror`, `extend`, and `toggle` subcommands to
  duplicate displays or lay them side by side (driven by `Super + P`). Live
  changes now go through `hyprctl eval` (the `hl.monitor` API), since the Lua
  config manager rejects `hyprctl keyword`; this also makes `autoscale` apply
  scaling live, including on hotplug, instead of only on the next login.
- `gpu/ryoku-gpu`: ranks every DRM GPU (eGPU over discrete over integrated, then
  by VRAM) and pins the strongest as Hyprland's primary renderer. Subcommands
  `detect`, `order`, `persist`, `disable`, `install-udev`, `status`. Pins the
  discrete GPU on desktops and external GPUs anywhere; keeps the iGPU primary on
  laptops for battery (override with `RYOKU_GPU_FORCE=1`). Writes a Lua drop-in
  (`~/.config/hypr/gpu.lua`) using the `hl` API.
- `gpu/ryoku-gpu-detect`: sourced detection helpers (GPU records, VRAM recovery,
  classification, laptop vs desktop). NVIDIA is always discrete; an AMD/Intel APU
  is integrated when its VRAM is a fully CPU-visible UMA carveout at or under
  8 GiB; a discrete card needs at least 2 GiB. Override seams make it testable
  against a synthesized `/sys` tree.
- `gpu/ryoku-gpu-mux`: report and switch the ASUS hardware GPU MUX -- which GPU
  the internal panel is physically wired to -- because that, not the
  `AQ_DRM_DEVICES` render pin, is what decides whether the discrete GPU can ever
  sleep. This class of ROG laptop ships the MUX in Discrete, so the only
  connected connector is on the dGPU and it can never runtime-suspend (measured
  ~10 W and 62 C sitting idle); Hybrid routes the panel to the iGPU so the dGPU
  can suspend. `status [--json]` reports capability, mode, interface, whether the
  panel is on the dGPU, the dGPU idle draw, and reboot-pending; `get`/`capable`
  are cheap silent predicates; `set <hybrid|discrete>` writes the firmware knob
  but a reboot is required (the firmware re-routes the panel at POST), so it is
  never done automatically and no reconciler ever flips it. Prefers the
  authoritative `firmware-attributes` (asus-armoury) knob over the deprecated
  asus-nb-wmi platform node, reuses `ryoku-gpu-detect` for GPU classification,
  and stays hermetic under the `RYOKU_MUX_ARMOURY_ROOT`/`RYOKU_MUX_LEGACY_ROOT`
  test seams. nvidia-smi is spawned only for `status` (measuring wakes the dGPU,
  the very cost this exposes). Shipped to `/usr/bin` by the `ryoku-desktop`
  hardware glob.
- `gpu/90-ryoku-gpu.rules`: udev rule creating boot-stable, colon-free
  `/dev/dri/ryoku-gpu-<pci-slot>` symlinks so `AQ_DRM_DEVICES` can reference GPUs
  by slot.
- `display/ryoku-monitor`: DPI-derived per-monitor scaling (buckets at 1x, 1.25,
  1.5, 1.75, 2x) with `GDK_SCALE` kept in step (integer only when every monitor
  agrees on a whole scale of 2 or more, else 1). `autoscale` applies live through
  `hyprctl` and writes `~/.config/hypr/monitors.lua`; `persist` saves the current
  layout. Catch-all monitor rule written last for hotplug.
- `power/ryoku-hw-laptop`: shared laptop/desktop detector using DMI chassis type,
  battery presence, and lid switches.
- `power/ryoku-idle`: laptop-only `hypridle` launcher for Ryoku's dim, lock,
  display-off, and suspend policy.
- `leds/ryoku-leds`: reads the current wallust Hyprland palette and applies the
  active accent to OpenRGB-compatible keyboards and attached lighting devices via
  generic OpenRGB mode/color controls. Missing or unsupported RGB hardware is
  non-fatal.
- `audio/ryoku-mic`: caps the default microphone at its Base Volume (0 dB
  hardware gain) so codecs that map a 100% source to maximum analog gain do not
  clip speech into distortion. Reads the level from the device and only lowers an
  over-amplified mic, never raising a quiet one. Launched from Hyprland autostart.
- `drivers/nvidia.sh`, `drivers/intel.sh`, `drivers/amd.sh`, `drivers/vulkan.sh`:
  per-vendor, hardware-gated, idempotent install scripts with a `RYOKU_DRYRUN=1`
  print mode. NVIDIA uses the open modules on Turing and newer and the
  proprietary modules otherwise.

### Fixed
- `display/ryoku-monitor`: connecting a second monitor no longer throws Hyprland
  errors or resets the display you already tuned. The hotplug catch-all brings an
  unknown display up at `preferred` (always valid on an untrained link) instead of
  `highrr` (which errored until the link resolved; autoscale + settle still raise
  it to highrr afterwards). And the autoscale DPI pass now skips displays already
  configured in Ryoku Settings (the applied layout), so plugging in a new screen
  DPI-scales only the new one and leaves the existing display's chosen scale
  alone (fixture-covered in `tests/monitor-profiles.sh`).
- `audio/ryoku-eq`: the equalizer no longer splits the volume, and toggling it
  never silences or jumps audio that was already playing. Node volumes multiply
  along `app -> ryoku.eq.sink -> ryoku.eq.out -> hardware`, so enabling the EQ
  (which makes `ryoku.eq.sink` the default) used to force that sink to 100% while
  the real level stayed on the hardware sink: a second, hidden volume the OSD,
  the mixer, and the volume keys could no longer reach. Enable now carries the
  current master level onto `ryoku.eq.sink` and pins the hardware leg at unmuted
  unity, so `@DEFAULT_AUDIO_SINK@` stays the one global volume every control
  reads and writes and the toggle never jumps the level; disable carries it back
  onto the hardware sink before repointing the default. Enable still pulls any
  stream on the old default across, and disable still moves every stream off
  `ryoku.eq.sink` before killing the filter chain (tearing the sink out from
  under a live stream made clients like mpv and browsers cork themselves, heard
  as audio that never came back). Full playing->enable->disable cycles stay
  audible under one continuous volume (verified live), and the crash-recovery
  self-heal is unchanged.
- `display/ryoku-monitor`: the Settings paths (`apply`, `save`, `load`) now
  snap every explicit scale to the nearest Hyprland-valid value for its mode (a
  1/120 multiple dividing width and height to whole logical pixels), the same
  rule `autoscale` already applied. A stale draft or an old profile carrying
  e.g. 1.5 for a 1280x720 mode was sent raw: Hyprland drew the "Invalid scale"
  overlay, picked its own value, and the invalid number was still written to
  monitors.lua and the applied layout, so the overlay came back at every login.
  `list` now also emits per-resolution `scaleLadders` (the valid scales between
  0.5x and 3x that keep at least a 640x360 logical desktop: a 720p panel tops
  out at 2x, and the odd 1366x768 offers exactly 0.5/0.67/1/2) for the Hub's
  scale stepper, and the shared snap searches 0.25x-6x so a deliberate sub-1x
  choice on a small panel survives instead of being forced up. Covered by
  `tests/monitor-profiles.sh` (snap on apply/save/load, ladder contents).
- `drivers/nvidia.sh`: on the stock `linux` kernel install the PREBUILT
  `nvidia-open` (matched to the kernel, so there is no DKMS build to fail on a
  fresh kernel) instead of `nvidia-open-dkms`; custom kernels still use `-dkms` +
  headers. The mkinitcpio `MODULES` drop-in is written only when `modinfo` finds
  the module for an installed kernel, so a missing or failed build can no longer
  force a broken initramfs (the machine boots on the integrated GPU). Pre-Turing
  cards -- which the open module cannot drive, and whose proprietary packages are
  gone from the repos -- are skipped with a pointer to the AUR legacy driver
  instead of pulling a package that no longer exists.
- `display/ryoku-monitor`: write each output's refresh as `highrr` instead of the
  live rate, and settle the refresh after applying scale. A panel whose
  DisplayPort link first comes up at a low refresh (common on a discrete GPU at
  cold boot) no longer has that low rate captured back into the drop-in and locked
  in; every monitor now holds its highest refresh across reboots.
- `display/ryoku-monitor`: `autoscale` now snaps each DPI-derived scale to the
  nearest Hyprland-valid value for the panel (a 1/120 multiple dividing both
  width and height to whole pixels) before applying it. Hyprland rejects any
  other scale outright with an "Invalid scale" error overlay (1.5 on a 2560 panel
  is 1706.67px), so the raw DPI bucket spammed the screen with errors on many
  panels; now a 2560x1600 laptop gets 1.6, a 4K panel keeps 1.5, a 1080p stays 1x.
- `display/ryoku-monitor`: `autoscale` lays the displays in one flush,
  non-overlapping row, positioning every output from its real (accepted) logical
  width rather than the live or "auto" x. A freshly plugged display lands exactly
  beside the laptop instead of overlapping it, which a stale position could do
  once the scales differed (the overlap also tripped Hyprland's "layout set up
  incorrectly" overlay).
- `drivers/nvidia.sh`: also write the early-KMS modprobe option
  (`nvidia_drm modeset=1`) and the initramfs `MODULES`, which are mandatory for a
  working NVIDIA Wayland session. Detection-gated, so they apply whenever an
  NVIDIA GPU is present, not only on the amd-nvidia profile.
- `gpu/ryoku-gpu`: let shellcheck follow the sourced detect helper from any cwd.
- `power/ryoku-idle`: ignore defunct `hypridle` zombies when deciding whether the
  idle daemon is already running, so a dead first start cannot block a restart.
