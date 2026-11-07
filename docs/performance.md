# Performance

What the Ryoku desktop costs while it runs, how that was measured, and what the
October 2026 pass changed. The numbers are from one machine; the method is the
part to reuse.

The pass predates Ryoku 2.0, which moved every bar style and subshell except
Orpheus and Kairos into the catalogue repo (see `docs/subshells.md`). The rows
for QS Bar, Python, Nomarchy, Shima and Sumi measure their in-tree versions.

## How it was measured

**The machine.** Ryzen 7 7940HS with an RTX 4060 in MUX discrete mode (the
panel is driven by the NVIDIA GPU), one 2560×1600 panel at 165 Hz and scale
1.333, niri, NVIDIA 615.78.08, the Performance power profile for every run. The
user's real desktop was measured, not a fresh install: Orpheus with the Python
dock, the line visualizer at the top edge, iRiS and ii desktop widgets, a still
wallpaper and a paired phone.

**The noise.** Everything unrelated was stopped first (Vesktop, a Discord bot,
both Ollama services, a local LLM container). The terminal multiplexer and the
coding agent that drove the runs are excluded from every sum.

**The tool.** `bin/ryoku-dev-perf-baseline` samples every process the desktop
owns for one window (60 s here) and records, per process and per group:

- proportional memory (PSS, never summed RSS: shared Qt and driver pages would
  count once per process);
- CPU per thread and wakeups (context switches per second);
- GPU time and VRAM from `nvidia-smi pmon` and the DRM fdinfo counters;
- package and core power (RAPL), the amdgpu PPT sensor and NVIDIA board power;
- the active power profile, because Power Saver changes clocks and the shell's
  own effects;
- with `--spawns`, every process launch per minute and who started it, from the
  kernel's exec tracepoint;
- with `--frames`, frames per shell window from Qt's render loop;
- with `--surfaces`, commits and presented frames per layer surface by
  namespace, from `WAYLAND_DEBUG`;
- with `--startup`, the time from a shell restart to its configuration loading.

It asks the compositor nothing, so it reads the same under every compositor.
The debug modes restart the shell with a log that costs CPU itself: take their
frame counts, not their CPU.

**The scenarios.** Each ran after a shell restart and a 90 to 120 s settle, on an
empty workspace:

- *idle*: nothing playing;
- *music*: mpv playing into a muted sink, so the analysers, the Orpheus drops
  and the visualizer all work as they do with sound;
- *visualizer off*: idle with the line visualizer disabled;
- *sweep*: every bar style then in the tree, at idle;
- *startup*: three shell restarts.

**The base desktop** is everything Ryoku ships and runs for everyone: the shell
UI (`qs`), the shell daemon, its helpers (cava, pw-record), the window-manager
provider, the wallpaper daemon, the compositor, session daemons and Ryoku's
system units. Optional features are reported apart: phone integration,
dictation, Rashin and Prowl, Cobalt.

## Results

Measured on 2026-10-22. CPU is a percentage of one core, averaged over the 60 s
window. "Base" is the whole base desktop; `qs` is the shell process alone, the
part that draws. Before is unstable-dev at `da60f0d30`. The idle rows after come
from the first full run of this branch, the music rows from the second, which
also carries the C++ spectrum and the shared Orpheus clock; the second run's
idle sweep is left out because the terminal driving it was busy, which shows up
in niri and in the power readings.

| scenario | base CPU | `qs` CPU | `qs` wakeups/s | niri CPU | launches/min | package W | NVIDIA W |
|---|---:|---:|---:|---:|---:|---:|---:|
| Orpheus, idle | 6.17 → 4.12 | 2.85 → 3.13 | 123 → 83 | 1.13 → 0.63 | 5,796 → 13 | 15.8 → 12.7 | 4.36 → 4.98 |
| Orpheus, music | 81.0 → 60.4 | 63.6 → 47.6 | 5,978 → 3,778 | 11.7 → 10.4 | 5,954 → 16 | 26.4 → 23.9 | 16.0 → 10.6 |
| Orpheus, visualizer off | 6.22 → 2.77 | 2.97 → 1.80 | 118 → 33 | 1.12 → 0.43 | 5,311 → 13 | 15.3 → 12.4 | 4.42 → 4.85 |
| Kairos, idle | 4.57 → 2.25 | 2.22 → 1.53 | 72 → 27 | 0.25 → 0.23 | 5,411 → 13 | 15.0 → 12.5 | 3.65 → 4.89 |
| Kairos, music | 60.0 → 38.2 | 44.9 → 28.5 | 4,388 → 1,897 | 10.2 → 7.3 | 5,446 → 16 | 24.6 → 19.1 | 11.4 → 10.8 |

Per-process CPU misses processes that start and exit inside the window, which
is where most of the idle cost was. The whole machine, every process and the
kernel included, was busy for 41.6% → 27.2% of a core with Orpheus at idle,
119% → 89% with music, and 35.7% → 24.6% and 90.6% → 63.4% for Kairos. With
Orpheus the shell's video memory went from 507 to 483 MB at idle and from 562 to
488 MB with music; with Kairos at idle and the other styles it rose by 28 to 60
MB, which this data does not explain. During the first before runs a model that
had just been stopped still held 5.2 GB of GPU memory in an idle `llama-server`;
it used no CPU.

**Frames.** On a 165 Hz panel the shell's windows presented 5.3 frames a second
at idle before and 1.8 after. With music, 404 before (two full-screen windows
at the panel's rate) and 273 after; per surface, the Orpheus bar (now 84 px
tall) presents about 150, the desktop with its iRiS visualizer about 65 and the
line visualizer about 55.

**Startup.** From a shell restart to its configuration loaded: 3.93 s before,
3.27 to 3.31 s after.

**Memory.** Measured back to back on the same base (`8a17af10c`), the shell
process holds 597 to 620 MB PSS on that base and 593 to 597 MB on this branch.
Two restarts of the same code differ by up to about 25 MB, so only one change is
clearly measurable on its own: a still wallpaper no longer loads the video
stack, 655 MB with it and 593 MB without on this branch. The wallpaper picker
held another 116 to 131 MB while hidden; it now frees that a minute after it
closes (see below).

**The bar styles at idle**, same conditions, before → after:

| style | base CPU | `qs` CPU | `qs` wakeups/s |
|---|---:|---:|---:|
| Orpheus | 6.17 → 4.12 | 2.85 → 3.13 | 123 → 83 |
| Kairos | 4.57 → 2.25 | 2.22 → 1.53 | 72 → 27 |
| QS Bar | 15.7 → 12.7 | 11.3 → 10.7 | 426 → 245 |
| Python | 17.3 → 2.42 | 12.9 → 1.62 | 711 → 28 |
| Nomarchy | 4.84 → 2.24 | 2.56 → 1.49 | 58 → 24 |
| Shima | 4.62 → 3.04 | 2.22 → 2.09 | 59 → 35 |
| Sumi | 4.49 → 2.24 | 2.11 → 1.60 | 56 → 24 |

Two readings did not improve. Orpheus's own idle CPU was within noise of before
(its springs kept re-settling, which the pass below found and fixed), and the
NVIDIA board drew about half a watt more at idle in the after runs while no
Ryoku process used measurably more GPU time; the GPU ran 5 °C warmer, and this
data does not separate the two.

## At rest

A second pass, also on 2026-10-22, went after the desktop with nothing moving:
the empty workspace, no music, no window open, Orpheus, the Performance
profile, the same noise stopped, a 60 s sample after a 150 s settle, and a
separate `--surfaces` sample for frames. Before is this branch ahead of that
pass.

| process | CPU | wakeups/s |
|---|---:|---:|
| shell (`qs`) | 5.07 → 0.37 | 172 → 12 |
| niri | 0.93 → 0.32 | 49 → 15 |
| shell daemon | 0.20 → 0.00 | 125 → 35 |
| Ryogami | 0.03 → 0.00 | 6 → 0 |

The whole machine went from 37.1% to 23.2% of a core busy, the shell's windows
from 1.8 presented frames a second to 0.8 (what is left is Orpheus's network
reading changing about once a second), and the NVIDIA board from 5.9% to 0.9%
utilisation. The shell holds 582 MB PSS against 624 MB, and the hidden
wallpaper picker's 114 MB is gone a minute after it closes.

What it was:

- **Orpheus's springs never settled.** Every slot springs to its content
  width, and background traffic moved the network rate across a digit every
  few seconds, so the liquid body re-sprang for about a second each time: 7,500
  item updates and 400 sync rounds in 30 s behind about one presented frame a
  second. Readings now hold their widest usual width.
- **Hidden surfaces drew every second.** niri sends a covered surface one
  frame callback a second, and Qt's 100 ms callback timeout turned each into an
  unexpose and re-expose that rebuilt the full-screen swapchain and drew the
  same picture. The overview backdrop did this all the time; the bar, desktop
  and frame did it under a fullscreen window, a locked screen or blank displays
  once they had drawn while covered. The daemon now starts shell processes with
  that timeout off. Quickshell's `updatesEnabled` was tried first and dropped:
  Qt ignores expose events while updates are off, so switching them back on
  can leave a surface frozen on its old frame.
- **Readouts sampled for nobody.** System stats now pause while the session is
  locked or the displays are off.
- **Four windows reserved nothing.** The frame's edge reservations exist for
  Sumi only, so every other style maps four windows fewer.
- **The daemons polled.** The shell daemon checked shell.json every 500 ms,
  read the shell's memory every second, scanned `/proc` for night light every
  30 s and for the lock guard, and re-read all of NetworkManager, about 2,000
  system calls, each time the Wi-Fi signal drifted, every six seconds. Ryogami
  signed the Steam workshop every three seconds and checked its config every
  two. The lid daemon woke every second and the login-screen guard every five.
  Each now waits on inotify, pidfds, netlink or D-Bus signals, with a slow
  fallback only where a source is missing.

The login-screen guard on the measured machine was still the packaged one, so
its 12 launches a minute are in the after numbers; the new guard removes them.

## Where the cost was

Most of it was not drawing. It was processes started on timers, analysers and
captures nobody looked at, and windows redrawn whole for a small change.

**Process launches.** An idle desktop started about 5,800 processes a minute.
Almost all of it was the login-screen guard, which walked `/proc` every five
seconds and forked `tr` for every entry. The lid daemon ran `awk` twice a second,
the shell daemon ran `pgrep`, `ryoku-host` and `systemctl` every five seconds,
and the shell itself ran `nvidia-smi`, `sh`, `adb`, `awk` and `gsr-cli` on
timers. Each of these now reads the kernel directly or waits for an event: the
guard and the lid daemon read `/proc` with shell builtins, the daemon scans
`/proc` in-process and asks the user manager over D-Bus, system stats come from
one native sampler, and the phone module follows KDE Connect and network events
instead of polling `adb`.

**Audio.** Music ran three cava processes, a Python waveform capture and
`pw-record`, and the Python capture and `pw-record` kept running in silence.
Now one cava feed read in C++ (`CavaFeed`) serves every bar style, the desktop
music widgets and Shima's iNiR services; the line visualizer keeps its own feed (64
bands at 60 fps, which its look needs) and reads its waveform through a C++
capture (`WaveCapture`) instead of Python. With music that is two cava processes
and `pw-record`; at idle none of them runs, and the visualizer rests instead of
ticking.

**Whole-window redraws.** Orpheus kept a full-screen window at all times, so
every frame of its drops re-rendered and recomposited the whole output, at the
panel's refresh rate while music played. It now rests at bar height and grows
only for a card, its settings or a morph, and its drops request at most about 60
frames a second. The spinning cover and the seek squiggle step on one 30 Hz
clock instead of running animations at 165 Hz. The desktop re-rendered the full
screen every second for its clock; minute faces now read a minute clock. The
desktop visualizer widget was a JavaScript Canvas that cost about three
full-window frames per analyser update; as a C++ painter it costs two (see the
layer note below), and none of the drawing runs in JavaScript.

**Idle redraws in the faces.** Python faces restarted an 800 ms ease for
sub-percent sensor jitter, the battery wave ran forever, material clocks woke
every second without a seconds hand, and QS Bar's silent drift ran at 20 fps.
Each now changes only when what it shows changes.

**Docks.** Docks recomputed on every window title change and kept drawing while
hidden; they ignore title-only updates now, the Python dock unmaps behind an
edge strip, and the ii dock gates its magnification, previews and widgets on
being shown.

## What moved to C++

The QML stays presentation; the hot loops moved into the `Ryoku.Blobs` plugin
(`ryoku/shell/plugin/`), each with a single owner and consumer counting:

- `CavaFeed` reads cava's binary frames, with the peak follower and stale
  flattening the QML parser had;
- `WaveCapture` turns `pw-record`'s PCM into the scope's waveform;
- `SystemMonitor` serves CPU, memory, temperatures, GPU and network from the
  kernel on one worker, replacing QML `/proc` parsing and `nvidia-smi` calls;
- `CavaSpectrumPainter` is the iNiR spectrum's QPainter drawing as a painted
  item that repaints once per change;
- `BlobWave` caps its wave frames and stops once settled.

## Next to other Quickshell shells

A source study of Noctalia, Caelestia, DankMaterialShell, iNiR, end-4,
Serpantinum and Omarchy Desktop found the same lesson in each: the cost is
surface and service lifetime, not QML itself. The shells that idle well map
about two surfaces per output, start an analyser or sampler only for a visible
consumer, read the kernel natively and schedule frames on demand. Noctalia
(now a native C++ shell) runs PipeWire capture and FFT in process and goes
dormant after a second of silence; Caelestia embeds libcava in its plugin; DMS
runs six cava bars at 25 fps only while a visible widget plays. This pass moved
Ryoku onto those patterns where it was behind: one consumer-owned analyser,
native stats, no timers where an event exists, and windows sized to what they
draw.

Public resource reports for those shells (for example
[DMS #631](https://github.com/AvengeMedia/DankMaterialShell/issues/631), 7%
Quickshell plus 5% niri at idle, most of it cava, and
[Caelestia #1603](https://github.com/caelestia-dots/shell/issues/1603), about 4%
CPU for a continuously repainting background) are bug reports on other machines,
not baselines to rank against.

## What still costs

In the order to take them:

1. **Layered content presents twice.** Content that animates inside
   `layer.enabled` measured two presented frames per change: the layer's
   texture is refreshed in one frame and shown in the next. Orpheus applies its
   default 0.94 opacity to the composed body through a layer, so with music its
   bar presents about 150 frames a second, against about 88 with the layer off.
   The desktop's iRiS visualizer shows the same ratio, about 65 frames a second
   for a 30 Hz analyser. Folding the opacity
   into the blob shader, each pixel drawn by exactly one shape, would remove
   the layer without changing the look.
2. **The desktop visualizer redraws the whole desktop.** Its widget lives in
   the full-screen desktop surface, so each of its frames re-renders the
   output (about 65 a second with music). iNiR's own background lets such
   content move into a small surface of its own while nothing covers it
   (`LiveLayer`); the Ryoku desktop does not host that yet, and a scaled widget
   cannot use it.
3. **The line visualizer** draws at its configured 60 fps while music plays;
   30 halves it.
4. **At rest, a live reading still redraws its window.** Orpheus's network
   rate changes about once a second at idle and redraws the bar each time. The
   shell daemon still wakes about 35 times a second, nearly all of it Go's own
   scheduler: its work at rest is a memory sample every few seconds and a
   battery read when UPower reports a change. The recorder checks `gsr-cli
   status` once a minute as a stale-state watchdog.
5. **Memory per window.** The shell holds about 580 MB at rest, and each mapped
   window has its own render thread, GL context and swapchain on top of the QML
   engine. Fewer, smaller surfaces is the lever, not QML size.
6. **The discrete GPU drives the panel** in MUX discrete mode, so the RTX 4060
   idles at 4 to 5 W whatever the shell does; Hybrid mode is the saving there.
7. **Startup** is 3.3 s to the configuration loaded; precompiling the QML
   (`docs/roadmap.md`, phase 1) is the next step.

Optional features carry their own cost and are off unless chosen: phone
integration (the KDE Connect monitor), dictation, Rashin and Prowl, Cobalt. QS
Bar stays the heaviest style: its ambient stream keeps it near 11% of a core
and a quarter of the GPU at idle on Performance.

## Measuring it yourself

```sh
ryoku-dev-perf-baseline --label idle --json idle.json            # 60 s, as the desktop is now
ryoku-dev-perf-baseline --restart --settle 120 --spawns          # who launches what
ryoku-dev-perf-baseline --surfaces --window 15 --settle 30       # which surface draws
ryoku-dev-perf-baseline --startup                                # restart to config loaded
ryoku-dev-perf-baseline --compare before.json after.json
```

Set the scenario up first (bar style, music, open surfaces), keep the power
profile the same between runs (`--compare` warns when it differs), and stop
anything heavy the desktop does not own.
