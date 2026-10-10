# What Ryoku is

Ryoku (力, "power") is a hand-built Linux distribution: a complete, opinionated
Wayland desktop plus the installer and system definition that reproduce it on
any machine. It ships Arch and CachyOS editions with Hyprland or niri, and an
x86_64 glibc Void edition with niri, runit, Turnstile, and elogind. The whole
thing lives in this repository; a live machine is only a deployment target.

## Philosophy

Ryoku is built on one belief: a desktop should be ready to live in the moment
it finishes installing. Not ready the way a blank page is ready, but the way a
good tool is. You pick it up, it already works, and it already looks like
someone cared.

Most Hyprland setups hand you freedom the way a hardware store does: here are
the parts, good luck. Want to move the clock, restyle the bar, change the accent
color? Open a config file, learn its syntax, edit it by hand, reload, and hope.
That is not freedom, it is homework, and it quietly tells every newcomer that
this world was not built for them.

Freedom should be simpler than that. You should be able to change your desktop
by using your desktop: open a panel, click a control, watch it happen. No
digging through files, no memorizing options, no editing code to do something as
ordinary as changing how your bar looks. The power is all still there. It is
just within reach.

So Ryoku is not a pile of overglorified basic dotfiles pretending to be the
future of Arch. It is a finished desktop with a front door: taste already
applied, and every choice that matters put behind a control you can actually
find. A beginner gets a real computer on day one. Everyone else gets to stop
babysitting a pile of configs and just use the thing.

Freedom of choice cuts the other way too. There is a strange trick going
around lately where a desktop built on Arch, an operating system whose whole
reason for being is choice and restraint, gets turned into a storefront: a
stack of big-tech web wrappers and somebody's paid, subscription apps bundled
in and sold to you as open-source freedom. That is not freedom, it is a sales
pitch with a monthly bill hiding inside your window manager. Ryoku ships what a
desktop genuinely needs and then gets out of the way: no services you never
asked for, no telemetry, no lock-in wearing an open-source badge. What you put
on top is your call, because that was always the point.

Power and beauty, in that order, and never one without the other, and never
locked behind a text editor.

## Goals

- **A cohesive Wayland desktop.** One look, one motion language, one control
  plane. The bar, panels, launcher, lock screen, and screenshot tool are parts of
  a single shell, not a pile of unrelated widgets.
- **Reproducible.** A fresh install reaches the same desktop the repo describes,
  from one source of truth.
- **Works on day one.** Sensible defaults that are actually usable immediately:
  the developer toolchains and their package managers work without root, theming
  follows the wallpaper, hardware (GPU, displays, laptop power) is detected and
  configured automatically.
- **Minimal and legible.** No cruft, no dead code, no duplicated config. Small,
  focused files you can read.
- **Opinionated by default, swappable by choice.** A fresh install is a
  deliberate set of choices, the stock Arch kernel among them. Where a
  power-user lever is genuinely worth it, Ryoku offers it as an explicit opt-in
  that leaves the default untouched: the Extras section can swap in the CachyOS
  kernel, for one, without changing what a fresh install is. See
  `docs/kernels.md`.

## How the parts fit

- **The shell** (`ryoku/shell/`) is the desktop UI. `quickshell/` is the QML
  front end (the `qsbar` top bar by default, with the built-in `sumi`
  monochrome rail as its alternative, plus the `launcher` and
  `ryoshot` screenshot tool). `ipc/ryoku-shell` is a single Go daemon that is the
  control plane: it supervises the UI components, owns the wallpaper, clipboard,
  and lock, and answers one socket. Keybinds and the UI talk to it; it decides.
- **The control CLI** (`ryoku/cli/`, the `ryoku` command) is the system front
  door: `ryoku update`, `status`, `materialize`, and the snapshot and rollback
  commands where the host supports them. It uses pacman on Arch and CachyOS,
  XBPS on Void, and the host's init seam for service work.
- **The compositor** is Hyprland (`ryoku/hyprland/`, configured in Lua) or
  niri (`ryoku/niri/`, KDL). Each is a provider under `ryoku/wm/`, configured
  in its own language, one concern per file. Its autostart brings up the shell
  and the hardware helpers.
- **Theming** is wallpaper-driven: `matugen` regenerates the palette from the
  current wallpaper, and the terminal and compositor colors follow it. With *Theme
  apps* on (the default), `matugen` fans that same palette into GTK / GUI apps
  (Files, editors, other libadwaita/GTK apps) too; off, they stay stock. Brand-
  fixed elements (the 力 logo, a few accents) stay constant.
- **The system** (`system/`) defines the boot chain, the hardware policy
  (GPU/driver/display/power helper scripts), and the package sets.
- **The installers** (`installation/` and `void/iso/`) share a Go TUI and use a
  host-specific backend to partition the disk, install the signed package set,
  and set up the boot chain. Each ISO carries the packages it needs, so an
  install needs no build toolchain.
- **Rashin** (`ryoku/rashin/`, optional and off by default) is the agent OS: a
  machine-generated knowledge vault, a local daemon with a web dashboard, and a
  one-click Hermes setup, so any coding agent starts with an exact map of the
  machine. Enabled from Ryoku Settings under Advanced. See `docs/rashin.md`.

## Working on it

Read `docs/structure.md` for where things live, `docs/conventions.md` for how to
write them, `docs/ui-ux.md` for the desktop's design and motion, and
`docs/development.md` for the deploy/test/commit loop. The cardinal rules in
`AGENTS.md` override anything that contradicts them.

## Clock format

Open Ryoku Settings with Super+comma, then Global > Language & Region.
The **24-hour time** switch applies to Ryoku's clocks, weather times,
notification timestamps and the built-in lockscreen. Off shows 12-hour time
with AM/PM; on shows 24-hour time. The default is 12-hour time.
The clock controls in bar settings and the clock's right-click shortcut change
this same preference. Third-party apps and downloaded lockscreen themes keep
their own formatting.

The shell daemon persists this existing setting as
`general.clock_format_24_h` in `shell.json`; `Ryoku.Ui.Singletons.TimeFormat`
watches it across processes. Legacy per-surface `clock12h` / `clock24h` values
no longer override the global preference.
