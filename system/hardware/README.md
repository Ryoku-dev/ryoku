# system/hardware/

Drivers and hardware setup. The job here is simple to state: use the best GPU,
make the screen look right, install the right driver for whatever vendor is
in the machine, and do not waste power doing it.

## What's here

- `gpu/` Picks the strongest GPU and pins it as Hyprland's main renderer.
  - `ryoku-gpu` The command. `detect` lists every GPU strongest first and names
    the chosen primary; `persist` writes the Hyprland pin; `install-udev`
    installs the stable device names; `status` shows the current state;
    `disable` clears the pin.
  - `ryoku-gpu-detect` The detection helper the command sources. It reads the
    GPUs from the kernel and ranks them. Kept separate so it is easy to test.
  - `ryoku-gpu-mux` The hardware display MUX on laptops that have one, read
    through the kernel's `firmware-attributes` class (falling back to the
    deprecated per-platform sysfs). `status` reports the mode and whether the
    panel is wired to the discrete GPU; `set hybrid|discrete` switches it, which
    takes a reboot because the firmware re-routes the panel at POST. This is a
    different and lower layer than the `ryoku-gpu` pin: in discrete mode the
    panel hangs off the dGPU, so the dGPU can never runtime-suspend and no
    `AQ_DRM_DEVICES` value can change that. `ryoku doctor` reports the idle cost
    when it sees the condition. See `docs/power.md`.
  - `ryoku-gpu-lib32` Installs the 32-bit (lib32) GPU drivers matching the
    detected hardware, so 32-bit and Proton/DXVK games render on the GPU rather
    than in software. Needs `[multilib]`; the Gaming bundle enables it, then runs
    this. Reuses `ryoku-gpu-detect` to pick the right per-vendor Vulkan ICD on a
    `lib32-mesa` + `lib32-vulkan-icd-loader` baseline.
  - `90-ryoku-gpu.rules` A udev rule that gives every GPU a stable, predictable
    name under `/dev/dri` so the pin keeps working across reboots.
- `display/` Backlight and output policy. The scaling tool itself moved to the
  compositor payload (`ryoku/hyprland/scripts/ryoku-monitor`): it speaks the
  compositor's output config, so each window manager ships its own.
- `power/`
  - `ryoku-hw-laptop` Classifies the host as laptop or desktop from DMI chassis
    type, battery presence, and lid switches. It is shared by GPU and idle policy.
  - `ryoku-idle` Renders `~/.config/ryoku/hypridle.conf` from the idle policy in
    `power.json` and runs `hypridle` for Ryoku's dim, lock, screen-off and suspend
    timers; the timers are hypridle's whole job. Screen-off goes through
    `ryoku wm act output.power`, and suspend calls `ryoku-shell suspend`, so
    neither path names a compositor. `apply` re-renders and restarts a running
    hypridle after a change. The fail-closed lock transaction, final login1
    handshake, output wake and lighting restore belong to the always-on shell
    daemon, even when idle timers are disabled.
  - `ryoku-power` Owns the CPU and power knobs the Hub's Machine page drives.
    `capabilities --json` reports what this machine actually exposes;
    `profile get|set <profile> <key> <value>` stores a per-profile definition
    (governor, EPP, `maxFreqPct`, `platformProfile`) in `~/.config/ryoku/power.json`;
    `apply-profile` writes one to sysfs; `charge-limit` caps the battery charge
    ceiling (the biggest lever on cell lifetime; the kernel reports no value at all
    until something writes one) and `aspm` sets the PCIe link policy. `idle get|set`
    stores the dim/lock/screen-off/suspend policy that `ryoku-idle` renders. `apply` is
    the idempotent pass that converges the globals plus the active profile, and
    exits quietly on a desktop or a machine without the knobs.

    It does not fight ppd: ppd still owns which profile is active, and a stored
    definition is re-applied on top after each switch. CPU boost and the PPT/TDP
    limits are deliberately absent, both measured as firmware placebos here. Three
    ordering facts are load-bearing and commented in the script: the platform
    profile is written first (a quiet profile clamps the dynamic
    `cpuinfo_max_freq`), the governor pass finishes before the EPP pass (a governor
    write resets EPP), and the whole apply is a write-verify-retry under a lock
    (ppd writes asynchronously and a single write loses the race). See
    `docs/power.md`.
  - `47-ryoku-power.rules` A polkit rule that lets the active wheel user run
    exactly `ryoku-power` without a password, so the login `apply` and each profile
    switch never throw a prompt (the sysfs writes are root-owned and do not survive
    a reboot). Every argument is a closed, validated set -- charge limit 50-100,
    ASPM policy and governor from the kernel's own lists, `maxFreqPct` 20-100 --
    so the passwordless grant stays safe. The rule pins `/usr/bin/ryoku-power`
    on purpose: granting it to a user-writable path would be a privilege hole.
  - `ryoku-clamshell` macOS-style clamshell for laptops, and the sole owner of
    lid policy for the active login1 session. Its daemon holds
    `handle-lid-switch` only while active, follows session activity, and
    reacquires after a login1 restart. A close on AC with an external display
    remains awake; every other close uses `ryoku-shell suspend`, and a docked
    machine requests the same transaction if power or the external display
    disappears while its lid is shut. ACPI is the live physical-state source;
    UPower seeds an already-closed daemon startup where ACPI exposes no lid, but
    never vetoes the ordered compositor edge. Hyprland serializes panel handoff
    and restores only outputs the helper disabled. niri sends both native close
    and open events through `policy` and retains its topology.
  - `logind-ryoku-lid.conf` The logind drop-in (installed to
    `/etc/systemd/logind.conf.d/10-ryoku-lid.conf`). It suspends on an undocked
    lid close when no Ryoku session owns the switch, ignores a docked close, and
    gives every shell daemon a 15-second delay budget. The package ships the
    drop-in and a checkout deploy seeds it; an unowned copy is adopted on the
    next update, per `docs/updates.md`.
  - `ryoku-power-cutover` Is the session-lifecycle and package-adoption boundary
    for the power policy. It selects one confirmed active Ryoku session per
    user, binds `ryoku-session.target` to login1 activity and logout, and
    recovers its watcher without a finite restart burst. Login, deploy, update,
    and paired libalpm hooks use it to reload compositor power bindings and
    replace clamshell, idle, qylock, shell and wallpaper owners under a durable
    sleep block. Doctor uses the same qylock generation guard to stage a repair
    for the next managed shell activation; it does not claim a live lifecycle
    cutover. Greeter, lock-screen, stale lingering managers and unrelated
    desktops are excluded. Each live replacement must report `ryoku-shell
    sleep-ready`; a failed or partial cutover remains blocked until a successful
    retry or reboot.
- `audio/`
  - `ryoku-mic` Caps the default microphone at its Base Volume (the level the
    device reports as 0 dB hardware gain, no amplification) so a codec that runs
    capture far hotter than unity does not clip speech into distortion. A mic
    already at or below unity is left alone. Launched from Hyprland autostart for
    Voxtype dictation and the pill voice visualizer.
  - `ryoku-volume` Steps the default sink for the `XF86AudioRaiseVolume` /
    `XF86AudioLowerVolume` keys, snapping to a five-point grid and honouring the
    volume panel's BOOST toggle (`qsbar.audioBoost` in `shell.json`): off caps at
    100%, on at 150%. The keys go through it rather than calling `wpctl` inline so
    the stepping lives in one place.
- `network/`
  - `ryoku-wifi-powersave` Disables, then restores, 802.11 power-save on every
    WiFi device for the shell's Game Mode, via `iw`, so the radio stays fully awake
    for lower, steadier latency. It saves each device's prior state and reverts it;
    no reconnect and no throughput cap. Runs as root through pkexec.
  - `49-ryoku-wifi-powersave.rules` A polkit rule that lets the active wheel user
    run exactly that helper without a password, so the Game Mode toggle stays one click.
  - `ryoku-wifi-regdom` Pins the Wi-Fi regulatory domain (the country) so 5 GHz is
    usable: on world domain `00` the kernel disables or no-IRs most 5 GHz channels,
    so a dual-band SSID is only seen on 2.4 GHz. `set <CC>` persists the country in
    the wireless-regdb conf (re-applied at boot by its udev rule) and iwd's
    `Country` hint and applies it now; `apply` is the idempotent install/upgrade/
    boot pass that also seeds iwd's 5 GHz rank nudge; `get`/`status` report. Runs
    as root through pkexec.
  - `48-ryoku-wifi-regdom.rules` A polkit rule that lets the active wheel user run
    exactly that helper without a password, so pinning the country stays one click.
- `drivers/` One install script per vendor: `nvidia.sh`, `intel.sh`, `amd.sh`,
  and `vulkan.sh`. Each one checks whether its hardware is present and installs
  only what that hardware needs.

## How the strongest GPU is chosen

Many machines have two GPUs: a fast discrete card (NVIDIA or AMD) next to the
slower one built into the CPU. If the desktop renders on the slow one it feels
sluggish even on a fast screen. `ryoku-gpu` ranks the GPUs (an external GPU beats
a discrete card, which beats an integrated one) and makes the strongest one
Hyprland's primary renderer through `AQ_DRM_DEVICES`. Every GPU stays in the
list, so a monitor plugged into a different GPU still works.

On a laptop the integrated GPU stays primary by default, because that is easier
on the battery and is what Hyprland itself recommends. An external GPU is always
preferred (you plugged it in on purpose). To force the discrete GPU on a laptop,
run `RYOKU_GPU_FORCE=1 ryoku-gpu persist`.

## How display scaling works

`ryoku-monitor` measures each monitor's pixel density (resolution against its
physical size) and picks a scale from a small set of steps, from 1x for normal
screens up to 2x for very dense panels. Nothing is hardcoded per model, so a new
monitor is handled sensibly the first time it is plugged in. GTK and older apps
get a matching `GDK_SCALE` so they stay crisp too.

## Idle policy

`ryoku-idle` renders `~/.config/ryoku/hypridle.conf` from the `idle` section of
`power.json` and starts `hypridle`, an `ext-idle-notify` client every supported
compositor serves. Both session entries go through the guarded
`ryoku-power-cutover session-start` lifecycle; it starts idle timers only after
the foreground shell and compositor bindings are ready. By default timers run
on laptops only; `idle.onDesktops` opts a desktop in, and `idle.enabled` false
turns them off. Each stage has a battery-aggressive and an AC-relaxed timeout
(shipped defaults: dim 2/5 min, lock 5/10, screen off 5.5/11, suspend 15/30),
and a stage set to 0 is dropped. The screen-off stage reaches the display
through `ryoku wm act output.power`, so the config names no compositor.
Edit the timeouts from the Hub's Machine page or with `ryoku-power idle set`; a
change runs `ryoku-idle apply`. The shell's Keep Awake toggle uses Wayland idle
inhibition, so hypridle stays paused while that toggle is on.

Idle timers are all hypridle owns. The suspend path itself is the shell daemon's.
The daemon bound to the foreground login1 session holds the delay inhibitor and
the long-lived hard sleep block. Before ownership moves, it synchronously locks
the outgoing session and proves that session's live qylock generation. Other
online sessions are locked directly with their own compositor environment and
kept under login1 observers; they never need a second user-daemon delay FD.
Only the foreground session may accept `ryoku-shell suspend` or unlock its own
proof.

The authorized suspend transaction secures qylock before releasing the active
block, then releases the one delay FD only after compositor coverage. On resume
the active session immediately wakes outputs across the retrain window with a
deadline on every provider attempt and restores lighting. Lost login1 signal or
owner connections reconnect in-process without dropping still-valid hard
protection. This remains active when idle timers are disabled.
`ryoku-clamshell` owns lid events only for the foreground session; logind
supplies the fallback when no session owner is present.

## How mic normalization works

Some laptop codecs let the analog capture gain reach its maximum (often +30 dB)
at a 100% source volume, which clips every word into broken audio. `ryoku-mic`
reads the default source's Base Volume, the device's own 0 dB hardware-gain
point, and lowers the source to it when it is running hotter. Nothing is
hardcoded per model: a mic that is already at or below unity is untouched, so a
well-behaved codec is a no-op.

## Per-vendor drivers

- NVIDIA: the open kernel modules on recent cards (Turing and newer), the
  proprietary ones on older cards, plus the userspace and video-acceleration
  bits.
- Intel: the modern media driver, the video runtime, and the Vulkan driver.
- AMD: the open Mesa stack and its Vulkan driver. No extra blob is needed.
- Vulkan: the vendor-neutral loader that every Vulkan app talks to.

The driver scripts are safe to run more than once (already-installed packages
are skipped), and they do nothing when their hardware is not present. Set
`RYOKU_DRYRUN=1` to print what would be installed without changing anything.

## How the installer uses this

The install backend runs the driver scripts for the detected hardware, installs
the GPU udev rule, and writes the first GPU pin and monitor scale so the very
first login already renders on the right GPU at the right size.

## Tools assumed present

`lspci` (GPU model names and the NVIDIA generation check) and `udevadm` (loading
the GPU rule) are expected on the target. `nvidia-smi` is optional and only fills
in the NVIDIA VRAM figure. `hyprctl` and `jq` are needed for live display
changes. `pacman` does the installing. `pactl` (PipeWire-Pulse) reads and sets
the microphone base volume for `ryoku-mic`. `iw` toggles WiFi power-save for
`ryoku-wifi-powersave` and sets the regulatory domain for `ryoku-wifi-regdom`.
