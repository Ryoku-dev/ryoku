# Power, heat, and battery

What actually governs power draw and temperature on a Ryoku laptop, which knobs
move the needle, and which ones only look like they do. Written against measured
behaviour on a reference machine, not vendor marketing.

Reference machine for every number below: ASUS ROG Zephyrus G14 GA402XV, Ryzen 9
7940HS (Zen 4 mobile) plus an RTX 4060 Mobile, `amd-pstate-epp` in `active` mode,
`power-profiles-daemon` running, kernel 7.1. Numbers are specific to that
chassis; the *shape* of the conclusions holds for AMD laptops generally.

## Low-battery alerts

Open **Win+, → Graphics & Power → Battery** to enable alerts, set the warning
and critical percentages, enable either level independently, or send a test notification. Defaults are 25% and
10%. Both levels default to enabled; turn either one off for warning-only or
critical-only reminders. Warning accepts 5–50%; critical accepts 1–49% and must be below warning.
These settings are independent of hardware support for a charge ceiling.

While discharging, Ryoku sends “Battery low” at the warning level and “Battery
critical” at the critical level, with “Battery at N%. Connect your charger.”
Each level alerts once until AC power reconnects. Starting below the critical
level sends only the critical alert. Delivery failures can retry without
consuming that alert. The service runs regardless of the selected bar style
or whether its battery indicator is visible.

Alerts use the normal notification system and respect Do Not Disturb. The
native service replaces the Python bar's separate low-battery reminders while
preserving its full-charge notification. If you installed the Battery Alerts
community add-on, disable its alerts when enabling the native service to avoid
duplicate reminders.

## The layers, from the one that matters most

1. **Which GPU renders your desktop.** On a hybrid laptop this dominates
   everything else. A discrete GPU that cannot sleep costs about 10 W all day.
2. **The panel.** A high-refresh, high-resolution panel is a constant, and blur
   and fractional scaling multiply it.
3. **The battery charge ceiling.** Not a power draw at all, but the single
   biggest lever on how long the battery lasts in *years*.
4. **The CPU power profile.** Real, but far smaller than people assume, and
   almost irrelevant under sustained load. See below.

## What the power profiles actually do

`power-profiles-daemon` owns the CPU side. Ryoku does not duplicate it. On the
reference machine each profile writes exactly this:

| Profile | `platform_profile` | ASUS thermal policy | EPP | Governor | `scaling_max_freq` |
|---|---|---|---|---|---|
| Power Saver | `quiet` | silent | `power` | `powersave` | 4001 MHz |
| Balanced | `balanced` | default | `balance_performance` | `powersave` | 5263 MHz |
| Performance | `performance` | overboost | `performance` | `performance` | 5263 MHz |

That is the whole of it. Power Saver's real effect is the 4001 MHz cap (nominal
clock, no boost headroom) plus a quieter fan curve. It changes burst and
single-threaded behaviour, and it changes noise.

What it does **not** do is change the temperature the chip settles at under a
sustained all-core load. That is the next section, and it is the answer to
"the power options don't seem to do much".

## Making the profiles yours

Those values are ppd's, not yours. **Settings > Machine > CPU Power Profiles** lets
you define what each of the three profiles does, and Ryoku re-applies your
definition every time ppd switches:

|Knob|What it sets|
|---|---|
|Governor|`scaling_governor`|
|Energy preference|`energy_performance_preference`|
|Frequency ceiling|`scaling_max_freq`, as a percent of the boost ceiling|
|Fan / thermal policy|`/sys/firmware/acpi/platform_profile`|

The page edits a *definition*, not the live profile; it names which profile is
active so you can tell whether an edit is in effect now. Definitions live in
`~/.config/ryoku/power.json` under `profiles`, so they survive updates, and a
profile you never touch is left entirely to ppd.

From the terminal:

```sh
ryoku-power capabilities --json                        # what this machine exposes
ryoku-power profile set balanced epp balance_power
ryoku-power profile set balanced maxFreqPct 85
ryoku-power apply-profile balanced                     # or just switch profiles
```

Two hardware behaviours worth knowing, both measured here. The `performance`
governor **locks** the energy preference: while it is selected the kernel offers
only `performance`, so a definition that sets an energy preference without also
setting the governor cannot change it. And `cpuinfo_max_freq` is dynamic on
amd-pstate, so the frequency ceiling is stored as a percentage of the real boost
ceiling (`highest_perf / nominal_perf * nominal_freq`) rather than a raw kHz value
that would mean something different on other silicon.

## Why sustained load sits at 95 C, and why that is not a fault

Under a sustained all-core load this chip settles at 95 C regardless of what
software asks for. Measured, all-core busy loop, same load each run:

| Change | Result |
|---|---|
| `cpufreq/boost` 1 vs 0 | 95.1 C vs 95.0 C; 26.9 W vs 24.5 W |
| ASUS thermal policy silent / default / overboost | 95.0 C in all three; only fan speed moved, 2700 / 3200 / 5100 rpm |
| Package limits 35/65/80 W vs 15/25/35 W | 95.0 C and about 23 W both ways |

The package-limit writes were accepted and read back at the new values, and the
load was held for 150 s so a sustained limit had time to bind. Power did not
move. The firmware ignores them on this platform.

The conclusion is that the chip is **thermally** governed, not power governed. It
targets 95 C and rides it, and AMD documents Zen 4 as designed to operate there
continuously. `auto-cpufreq`, the usual reference for this kind of tuning,
[refuses to touch turbo at all when `amd_pstate` is in `active` mode](https://github.com/AdnanHodzic/auto-cpufreq)
for exactly this reason; the measurements above are why.

So: 95 C under a heavy sustained load is the hardware working as designed. The
only thing software changes there is how loud the fan is while it happens.

Idle temperature is a completely different story, and idle temperature is where a
laptop actually spends its life. See the next section.

## What actually saves battery

### The discrete GPU is the big one

On a laptop with a hardware MUX, the MUX decides whether the discrete GPU is
*able* to sleep. In discrete mode the internal panel is wired to the dGPU, so it
is pinned awake forever. On the reference machine, in discrete mode and with an
idle desktop:

- dGPU drawing 9.9 to 11.0 W and sitting at 62 C, doing nothing
- `power/runtime_suspended_time` of `0`: it had never once suspended since boot
- 26 % GPU utilisation while compositing an *empty* workspace

That parasitic heat is also why the CPU package reads 64 to 67 C at idle in Power
Saver. It is one chassis and one set of heatpipes; the dGPU heat-soaks the CPU.

Roughly 10 W of constant draw on a 60 Wh battery is on the order of half the
runtime. Nothing else on this page comes close.

Check your machine, and switch the MUX if it is costing you:

```sh
ryoku-gpu-mux status      # capability, current mode, is the panel on the dGPU
ryoku-gpu-mux set hybrid  # then reboot: the firmware re-routes the panel at POST
```

`ryoku doctor` reports this as *discrete GPU idle drain* when it detects the
condition. It never switches the MUX for you, because that re-routes your display
and needs a reboot.

**Why the MUX is the whole story.** NVIDIA's own driver documentation is explicit:
"the NVIDIA GPU will remain in an active state if it is driving a display". That
single sentence explains a `runtime_suspended_time` of `0` on a Discrete-mode
laptop. Runtime D3 is not broken and not disabled; it simply cannot engage while
the card drives the panel. Ask the driver directly:

```sh
cat /proc/driver/nvidia/gpus/<slot>/power
```

On a healthy Ada/Ampere laptop that reports `Runtime D3 status: Enabled
(fine-grained)`, which is the default on those notebooks. Fine-grained mode
tracks actual GPU *usage*, so merely having the device open does not keep the card
up: the documented blockers are driving a display and a running CUDA application.
A compositor holding `/dev/dri/cardN` is therefore not by itself a reason the card
stays awake.

**Blanking the screen is not a workaround.** NVIDIA's docs add that a
display-driving GPU can reach a low power state once the display is turned off,
but that caveat is gated on the X11 `HardDPMS` option and does not carry over to
Wayland. Measured here: 30 s with the panel blanked via `hyprctl dispatch dpms
off` left the card `active` with `runtime_suspended_time` still `0`. The
compositor keeps the CRTC configured, so the driver still counts the card as
driving a display. Only moving the panel off the dGPU releases it.

Supporting evidence that the D3 path itself is healthy on this hardware: the
dGPU's *audio* function (`...:00.1`) runtime-suspends normally and reports minutes
of suspended time, so ACPI and PCIe power management work on this device. Function
0 is held up by the display alone.

**Verify after the reboot** all the same, because "should" is not "did":

```sh
cat /sys/bus/pci/devices/<slot>/power/runtime_status   # want: suspended
```

If that still reads `active` with `runtime_suspended_time` at `0`, the same doctor
check fires again with a different message. It distinguishes the two real causes:
the driver reporting runtime D3 off (remedy is the
`NVreg_DynamicPowerManagement=0x02` module parameter) versus the card genuinely
being kept busy (remedy is finding the external display or CUDA process). Catching
that matters, because a silent "clean" report on a machine still paying the full
idle draw is the worst possible outcome.

The trade is real and yours to make: hybrid mode lets the dGPU sleep and costs
you the direct display path (and with it G-Sync, plus a few percent in games).
Discrete mode is the right answer for a machine that is always docked and
plugged in. Hybrid is the right answer for a laptop used as a laptop.

Note that `ryoku-gpu`, which pins Hyprland's render device via `AQ_DRM_DEVICES`,
is a *different* and lower layer. It cannot help here: in discrete mode the
integrated GPU has no connected connector at all, so pinning it would black-screen
the machine. The MUX has to change first.

### Cap the charge ceiling

Holding a lithium cell at 100 % is what wears it out. If the battery exposes a
charge limit, use it:

```sh
ryoku-power charge-limit get
ryoku-power charge-limit set 80
```

80 % is the usual sweet spot for a machine that lives on AC. The kernel returns
"No data available" for this attribute until something writes it once, so an
unset limit reads as `unset` rather than as `100`.

This costs you 20 % of runtime per charge and buys back a large multiple of that
in cell lifetime. Set it back to 100 before a trip.

The charge ceiling is a plain sysfs value and does not survive a reboot, so
`ryoku-power apply` runs at login to push a stored choice back.

### PCIe link power

`ryoku-power aspm set powersave` lets idle PCIe links drop into lower power
states. The write is verified to apply on this hardware and causes no
instability, but the saving was not separately measured, so treat it as a small
bonus rather than a headline. Set it back to `default` if you ever suspect a
device is misbehaving after a link-state change.

## The desktop's own footprint

What the shell costs at idle is Ryoku's to fix, not the laptop's. Measured on
the reference box (7940HS + 4060, checkout build) on 2026-09-04, before and
after the fixes that ship in Onogoro 0.57.8:

| Where | Before | After | What it was |
|---|---|---|---|
| `$XDG_RUNTIME_DIR` (tmpfs, so RAM) | 4.3 GB, 100% full | 3 MB | Quickshell keeps two unbounded logs per instance and never removes a dead instance; one warning storm wrote 4.3 GB and 113 dead instances kept theirs. The shell daemon now prunes dead instances and caps a live log at 32 MB. |
| Wallpaper picker, hidden | 410 MB | 164 MB | The picker's QML tree was built at daemon boot and kept for the session. It is built on the first Super+W and torn down after close; the process stays warm, a reopen takes 36 ms. |
| `bluetoothctl` processes | grew without bound | 0 | The bar widget polled `bluetoothctl` every few seconds even when hidden and with no adapter, and each poll orphaned the last. The widget reads BlueZ over D-Bus now and runs nothing in the background. |
| `makoctl mode` every 1.5 s | a failing spawn per tick | none | Polled on boxes without mako. Gone. |

What is left, in the order to take it:

1. The shell process at 700 MB to 1.2 GB. The overlays it keeps warm (launcher,
   control panel, overview) should follow the picker: build on show, drop on
   hide. Same pattern, same file per surface.
2. Thumbnails and previews without `sourceSize`: a 4K wallpaper decoded at full
   size for a 240 px card costs 60 MB where it should cost 1.
3. The bar's polling timers (CPU, GPU, thermal, storage) fire on separate
   clocks; one 5 s tick would halve the wakeups.
4. On battery, Perf.qml already freezes the visualizer and the pill; the video
   wallpaper decode rate and the blur behind overlays should follow the same
   `lowPower` flag.

How to measure before claiming a win: `for p in $(pgrep -f 'quickshell|^qs');
do grep VmRSS /proc/$p/status; done`, `df -h $XDG_RUNTIME_DIR`, `pgrep -c
bluetoothctl`, and `top -H -p $(pgrep -x qs)` for the thread that is awake.

## What Ryoku deliberately does not ship

These were implemented, measured, and rejected. They are recorded here so nobody
spends the afternoon again:

- **Dynamic turbo/boost gating** (the `auto-cpufreq` strategy). Measured as a
  placebo under `amd_pstate=active`: 95.1 C vs 95.0 C. `auto-cpufreq` declines to
  do it on this driver too.
- **CPU package power limits** (`ppt_pl1_spl` and friends). Writes are accepted
  and read back, and the firmware ignores them. A knob that reports success and
  changes nothing is worse than no knob.
- **A second CPU governor daemon.** ppd stays the one thing that decides which
  profile is active. Ryoku does set the governor, EPP, frequency ceiling and
  platform profile -- that is the profile programmer above -- but it does so by
  reacting to ppd's own profile-changed signal and writing on top, never by
  polling or by competing for ownership. A rival daemon would fight ppd on every
  switch and neither would win predictably.

If you are tempted to add one of these, measure first: hold a real all-core load
for at least two minutes, and read package power, not just clock speed.

## Checking your own machine

```sh
ryoku doctor --check                  # read-only; reports a dGPU pinned awake
ryoku-gpu-mux status                  # is the dGPU pinned awake by the MUX
ryoku-power status                    # charge ceiling and ASPM
powerprofilesctl get                  # active profile
cat /proc/driver/nvidia/gpus/*/power  # the driver's own runtime D3 verdict
cat /sys/bus/pci/devices/*/power/runtime_status   # did the dGPU actually sleep
cat /sys/class/hwmon/hwmon*/temp1_input   # package temperature, millidegrees
nvidia-smi --query-gpu=power.draw,temperature.gpu --format=csv  # dGPU idle cost
```

A healthy idle laptop has the dGPU suspended (or absent), a charge ceiling set if
it lives on AC, and a package temperature well under 60 C. If the dGPU reports
several watts while you are reading text, start at the MUX.
