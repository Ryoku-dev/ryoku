# Umbriel Roadmap

This roadmap covers Ryoku support for the Umbriel compositor. It tracks the
seam provider, the compositor config payload, the keybind set and the settings
that reach the compositor. Each phase states the change and what done means.
A phase closes with a working live session and a passing gate, not with a file
that merely parses.

Base: `0e2466da9` (hyperion, v1.1.1). Compositor: Umbriel 0.1.0
(`fd0a6e9a2335`).

## Where we stand

The desktop boots on Umbriel with the bar, dock, wallpaper, launcher, game bar
and native overview. The provider speaks Umbriel's JSON-lines IPC, the config
tree follows the same shipped and seeded split as niri, and the user's niri
keybind table is the reference for the scrolling layout.

Claimed capabilities: workspaces, focus history, window geometry, window
float, output power, monitor config, session exit, native overview, overview
state, touchpad toggle. Adaptive sync is deliberately not claimed: Umbriel
reports `adaptive_sync: null` for every output here, so the row is hidden
rather than offered as a control that does nothing.

Implemented:

- provider verbs for caps, state, watch, act, apply, outputs, binds, defaults,
  session, schema and environment
- 108 official bind legend rows and 134 unique chords in the config payload,
  following the niri scrolling layout on Ryoku; a chord-by-chord audit against
  the legend fixed the pin, display-cycle and wheel-workspace bindings
- display arrangement and each-display settings, written to `monitors.toml`
- native overview through the compositor, with live window previews
- floating defaults at the designed sizes for Ryoku's own apps: Welcome,
  Settings, Rashin, phone tools, update log, Ryostore, Ryoport, Ryoport SSH and
  the console/VNC clients; Ryotunes is left to choose its own floating state,
  as it is on niri, and its keybind passes the designed `-size`
- touchpad enable and disable through a watched optional include
- Mouse, Touchpad, Pointer and Keyboard settings, written from the neutral store
  into `settings.toml` by `apply_input.go`. The Keyboard rows cover layout,
  style, second layout and switch key, repeat rate and delay, and Num Lock.
  Umbriel's native multi-layout and XKB option support lets the provider claim
  `keyboardLayoutSwitch`, so the second-layout rows are shown and work live.
- provider input rows and the neutral focus row, written by `apply_wm.go` and
  declared in `schema.json`

The shell, Hub, CLI and wallpaper daemon reach Umbriel only through the seam.
No compositor name appears outside `ryoku/wm/` and the Umbriel payload.

## Phase 1: keybind and input parity

**Change:** make every official bind either active on Umbriel or reported with
a reason. Binds come from `ryoku/wm/binds.go` and the user's niri table. Input
settings from the neutral store map to Umbriel `[input.*]` keys. Rebinds land in
the generated `rebinds.toml` so a change in Settings reaches the compositor.

**Done means:**

- the Hub and Super+K show all 108 rows with accurate chords
- every row is bound or carries a human reason why Umbriel cannot honour it
- a rebind in Settings changes the compositor binding after apply
- the input settings page round-trips a change through the live session
- the `msg` action dialect has a pinning test, because a wrong action name
  fails silently on a live compositor

**Where it is:** the legend, chord layout, 15 Mouse, Touchpad and Pointer rows,
the keyboard rows, the 7 provider input rows and the neutral focus row are done,
and the chord audit found no conflicts. The rebind writer is done: the provider owns the
shipped bind template, and `apply` renders every effective chord into
`rebinds.toml`, which is the only keybind table Umbriel reads. Rebinds, disabled
chords and workspace families reach the compositor; a rebind onto an occupied
chord is reported unhonoured instead of binding the key twice.

## Phase 2: compositor settings and capabilities

**Change:** map the neutral desktop settings that Umbriel can express into its
TOML config. Add a capability only when a writer or action honours it. Leave a
setting off with a reason instead of showing a control that does nothing.

**Done means:**

- settings changed in the Hub survive a reload and a logout
- `apply` reports every unhonoured key with a full sentence
- `umbriel config validate` passes after every apply
- the capability list matches what the provider actually writes

**Where it is:** display arrangement, native overview, touchpad toggle and the
Mouse, Touchpad and Pointer rows are done. The refresh rate is converted from
Umbriel's millihertz to hertz with the fraction preserved. Values the config
cannot express are dropped and reported unhonoured, so the generated files
validate after every apply.

## Phase 3: packaging and delivery

**Change:** ship the provider, config tree, portal routing and session drop-ins
as a variant package. Register the variant with the installer, updater and
doctor so an installed machine receives it and a switch can be undone.

**Done means:**

- `ryoku-desktop-umbriel` provides the compositor virtual and depends on the
  compositor, its portal backend, Xwayland and the night light backend
- every config file in the payload reaches users through materialize or a
  package
- the compositor choice appears in the installer and in the switch flow
- `ryoku-dev-verify-wm-isolation` and `ryoku-dev-verify-delivery` pass

**Where it is:** the package skeleton, payload, portal conf and drop-ins exist.
Registration across installers and distribution payloads remains, and
`ryoku materialize` has no Umbriel branch, so a fresh package install does not
lay down the config tree before the compositor starts.

## Phase 4: validation and release

**Change:** run the repository gates on the finished change and verify the
desktop by hand on a fresh Umbriel session. Record the result in the dated
journal before committing.

**Done means:**

- `go build`, `go vet` and `go test` pass in every touched module
- Lua, shell and QML checks pass where files were touched
- a fresh session shows the full desktop and the settings round-trip
- the commit uses `[ryoku] wm: ...`, stays under 72 characters and carries a
  plain-language release note
- the change is tested by hand before it is committed

## Known issues

| Issue | Status |
|---|---|
| Displays showed 60010 Hz instead of 60 | fixed and verified live |
| Adaptive sync offered on an output that cannot use it | removed, verified live |
| Invalid values written into the generated TOML | fixed, live matrix passed |
| Wrong-typed values broke the whole generated config | fixed, adversarial store verified |
| Provider IPC calls could hang on a silent compositor | fixed with a read deadline |
| Keybind chords disagreed with the legend | fixed, chord audit clean |
| Rebind collisions broke the generated keybind table | fixed, regression tests added |
| Shipped rebinds.toml seed could never be laid | fixed, now generated like niri |
| Hub/Settings sometimes opens slowly | reported repeatedly, not yet measured |
| `RYOKU_WM=niri` returns after a compositor switch and breaks the desktop | fixed and verified live after a switch |

The refresh, adaptive sync, TOML, wrong-type, IPC, keybind, rebind-collision
and rebind-seed rows are faults in the port and are addressed here. The slow Hub is an open observation that
may belong to the shared Hub rather than this port. The `RYOKU_WM` row is a
session-environment fix in a separate change. Issues found during testing that
belong to other parts of Ryoku are in `~/bug-reports/` outside this project.

### `RYOKU_WM=niri` returns after login

Observed after a logout and login on 2026-11-20. The session is Umbriel
(`loginctl` reports `Desktop=Umbriel`, started by `start-umbriel`), but the
systemd user manager had `RYOKU_WM=niri`. Every Hub and shell process then
forced the niri provider, which has no live socket. Three symptoms followed:
the Displays page said it could not read displays, the window-manager row said
"ready to use", and touchpad and workspace actions did nothing. A settings save
during that state went to niri, so Umbriel kept stale input values.

No persisted source was found in shell profiles, environment files or systemd
units. It appears to be runtime state in the user manager.

**Re-test:** plain logout and login did not reproduce. Logout to niri and back
to Umbriel did. Switching compositors is the trigger.

**Root cause:** `start-umbriel` (external package) imports the login
environment into the user manager but does not clear `RYOKU_WM` from a previous
compositor. `niri-session` has the same gap. The stale tag survives a switch.

**Fix:** two systemd drop-ins clear `RYOKU_WM` before the compositor starts,
one for Umbriel and one for niri, in `ryoku/shell/systemd/user/`. Deployed to
`~/.config/systemd/user/`. An earlier change to `ryoku-power-cutover` was based
on an incomplete diagnosis and can be reverted.

**Verification:** live logout to niri and back to Umbriel produced a new
session with `Desktop=Umbriel`, no `RYOKU_WM` in the user manager, and
`ryoku wm status` reporting Umbriel from `UMBRIEL_SOCKET`. Fix confirmed end to
end. Full report: `~/bug-reports/2026-11-20-umbriel-wm-env-override.md`.

An issue closes when the behaviour is fixed and verified on a live session,
not when the report is written.

## Validation so far

Done on the live Umbriel session, not just by parsing files:

- five input store variants and seven display layouts applied and validated,
  including empty, boundary, disabled, malformed, quoted and duplicate cases
- `output.enable on/off`, `output.cycle`, profile save/load/remove and malformed
  action arguments
- a display scale change through `ryoku-hub outputs apply` observed live, then
  reverted from backup
- the Displays page renders five sections and reflects VRR and refresh changes
- `go test -race` on the provider; all repo gates and vet pass
- the own-app float rules and the Ryotunes keybind were checked through real
  launches and `umbriel config validate`
- a chord-by-chord audit of all 108 legend rows, which found and fixed three
  wrong bindings
- rebinds end to end: a plain rebind moved the chord, a family rebind moved all
  ten workspace keys, a disabled chord disappeared, and an unexpressible chord
  was reported unhonoured; config validated after each
- rebind edge cases: two rebinds onto one target emitted it once with an
  unhonoured row, and a disabled rebind target was not resurrected
- watch events, act verb argument errors, output rotation, resolution and
  position changes, and `go vet` and QML lint on the touched files
- settings surviving logout: `ryoku materialize` left the generated files
  byte-identical, and the user confirmed a real logout/login kept mouse and
  touchpad changes
- the compositor-switch `RYOKU_WM` bug reproduced, was fixed with session
  drop-ins, and was verified through a logout to niri and back
- the Keyboard page renders all layout and typing rows from the live store;
  `umbriel keyboard-layouts` lists both layouts and `keyboard-layout-next`

Still to check before a PR: the packaging and delivery registrations in Phase 3.
The compositor-switch fix is a separate change from the port.

## Untracked artifacts in this checkout

These are not part of the port and were not added to it. They predate this work
and are left in place so nothing is deleted without the owner's say-so:

- `CMakeFiles/CMakeSystem.cmake`, a CMake build file
- `ryoku/wm/wayfire/ryoku-wm-wayfire`, a stale binary from the removed Wayfire
  provider
- `docs/.obsidian/`, Obsidian editor configuration

They should be removed or ignored before the checkout is handed over, but they
do not affect the port or its PR.
