# Shima upstream runtime on Arch

Shima keeps its existing `iris` barstyle identity. The original embedded iRiS
scene remains available and is the default. The complete iNiR 2.33.0 desktop
is bundled separately under `ryoku/shell/quickshell/shima`, from upstream
revision `48cdcecb0f6089cfbb7019054602eab28e154b3b`.

The independent runtime includes iRiS, Material ii and Waffle, the upstream
settings, desktop widgets, animations, Cava visualisers, Liquid Glass and
Ryoku's Matugen/theme and wallpaper integration. Copyright and licence
attributions are preserved in its LICENSE and NOTICE.

## Activation and rollback

The complete runtime is experimental on Arch and remains disabled by default.
A Niri session with `RYOKU_SHIMA_UPSTREAM=1` in the **ryoku-shell daemon's**
environment selects it when the existing `iris` barstyle is active. Setting
the flag to `0` or leaving it unset preserves the original embedded iRiS.
Other window managers retain the embedded implementation for now.

Ryoku's normal daemon starts and supervises the independent process.
A separate login session, iNiR service, manual upstream setup and a second
RyoStore entry are not required. The desktop supervisor requires a readiness
marker and cleanly stops the owned process before switching. If Shima crashes
or fails readiness, it restores the preceding Ryoku barstyle instead of
repeatedly launching a broken desktop.

To roll back, remove the opt-in flag from the daemon's environment and
restart the daemon using Ryoku's ordinary, session-safe management on that
machine. The previous `inir` files and original `iris/Scene.qml` have
not been replaced. Existing `ryoku/shell.json` settings are not erased.
New standalone configuration lives in `~/.config/ryoku/shima/config.json`.

The bundle was derived from the pinned NixOS runtime. Its Nix-store
interpreter shebangs have been converted to ordinary Linux shebangs,
including two Python helpers that originally assumed a managed venv.
The first-party launchers are ShellCheck-clean. Vendored upstream shell
scripts are not linted as first-party code because they contain sourced
variables and data-only script files; their portability is checked
independently.

The standard Arch deployment copies the entire Quickshell directory
and installs the `ryoku-shima` and `ryoku-shima-wallpaper` helpers with
the other Ryoku shell scripts. Do not run upstream's `inir install` command
or manage a separate `inir.service`.

## Safety and release gates

Run `python3 -m unittest discover -s tests -p test_shima_upstream_parity.py`
and the Shima-specific Go tests in `ryoku/shell/ipc`. Validate the QML
parser across bundled sources and use a disposable session to verify all
three panel families, settings, wallpaper bridge, IPC, switch-away,
forced-crash recovery, and no remaining Quickshell children.

Arch's Qt runtime and package universe must also be checked on an actual
Arch installation. Passing a nested test on a NixOS build host does not
establish Arch ABI compatibility. Keep this branch out of the release
until a clean Arch install passes these gates. SDDM and independent iNiR
login services are explicitly outside this integration.


## Launcher, overview and Ricelin controls

Ricelin's settings surface now includes a collapsible **Switch shell & bar**
entry. Users can choose iRiS, Material ii or Waffle, and switch the bar
between Classic, Islands, Scenic, Frame, M3 and Pill. Advanced bar settings
remain available in their original page.

Ryoku's `shell openSurface ... launcher` request now goes to a dedicated
launcher route rather than piggybacking on the overview command. Family
launchers keep their native behaviour: iRiS palette, Waffle Start and
Material ii's Pill launcher when Pill is selected. The default Material ii
overview remains its non-Pill launcher.

The default Niri overview uses the subscribed Ryogami wallpaper for the
correct output, with the existing iNiR still-image and multi-monitor
resolver as fallback. Video wallpapers are represented by an available
still frame rather than passed directly to Qt's Image renderer.

These changes passed a separate nested Niri test of all three launchers,
the default overview wallpaper, family switching, crash recovery and
process cleanup. A real Arch installation is still required before
releasing the upstream runtime by default.
