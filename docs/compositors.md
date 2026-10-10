# Compositors

Ryoku is one desktop that runs on more than one window manager. The shell, the
Hub, the CLI and the lockscreen are the same code on every compositor; only a
provider underneath differs.

Today there are two: Hyprland and niri. A third is a new provider and a new
package, and nothing else.

This page is the contract in table form. The same job as a walkthrough, with the
traps that cost time on the second compositor, is in
`docs/adding-a-window-manager.md`.

## The seam

`ryoku/wm/` is the contract. Nothing outside it may name a compositor:
`bin/ryoku-dev-verify-wm-isolation` fails the build on `hyprctl`, an instance
signature, or an `isNiri`-style test anywhere else. Consumers ask what the
compositor *can do*, never which one it is.

A provider is one binary, `ryoku-wm-<name>`, shipped by
`ryoku-desktop-<name>`. It implements eleven verbs:

|Verb|Answers|
|---|---|
|`caps`|the capability manifest, workspace model, foreign shell interfaces, owned files, and package set|
|`state`|one snapshot: outputs, workspaces, windows, keyboard|
|`watch`|the same as a stream of frames, one JSON object per line|
|`act <id>`|one neutral action (`window.close`, `workspace.focus`, ...)|
|`apply <store>`|write the compositor's config from the neutral settings store|
|`defaults`|the provider's baseline subtree of that store|
|`schema`|the settings rows only this compositor has, for the Hub to render|
|`binds <store>`|the keybinds only this compositor has, resolved against the store, for the cheatsheet's compositor section|
|`outputs <layout>`|apply a display layout|
|`session`|the `wayland-session` desktop entry|
|`environment <pid>`|export the provider's opaque handle from one verified session process|

`ryoku wm compat <plugin-dir>` is the neutral plugin gate. It scans QML,
JavaScript, shell, Python and executable files without running them, then
compares any foreign shell interfaces it finds with `caps.foreignApis`. A
completed scan always exits zero and prints `{ok, requires, reason}` as JSON,
including `ok: false` when the active provider cannot serve every requirement.
An incomplete scan, a path error or a provider error exits non-zero, so plugin
hosts fail closed. The scan rejects symlinks that leave the plugin directory
and bounds per-file bytes, tree entries and total source bytes to keep enable
and load paths fast.

The Hyprland provider serves `quickshell-hyprland`, `hyprctl` and
`hyprland-ipc`. The niri provider serves no foreign shell APIs. A plugin that
imports `Quickshell.Hyprland`, calls `hyprctl`, or reaches Hyprland IPC through
`HYPRLAND_INSTANCE_SIGNATURE` or `.socket2.sock` therefore loads normally on
Hyprland and is refused before enable or load on niri, with the missing APIs in
`reason`.

`act window.place <id> <x> <y> <width> <height> <output>` is the neutral
floating-window placement contract. The id is copied unchanged from `state`;
coordinates are output-relative logical pixels. A provider makes the window
floating, moves it to the named output, sizes it and positions it. Missing
windows, outputs and invalid dimensions are errors rather than a fallback to the
focused window.

`apply --preview` writes nothing and reports what it could not honour. That
report is what `ryoku wm use` and the Hub show before a switch, so the cost of
moving is known in advance rather than discovered afterwards. The Hub also
renders it as the "what this compositor cannot do" list, so a setting that is
hidden reads as explained rather than missing.

`schema` is what keeps the Hub free of compositor vocabulary. A provider
declares its own exclusive rows and the Hub renders them through the one
renderer every settings page uses, so two compositors cannot drift apart
visually and adding a third needs no Hub edit. The `ctl` field is the shared
control vocabulary; a row whose control needs a bespoke editor names the page
that draws it.

`binds` is `schema`'s keybind twin: the provider declares the chords only it
offers and the Hub folds them into a cheatsheet section titled with the
compositor's name, so a scrolling-tiler action niri has and Hyprland lacks is
listed without the shell naming either. It resolves the store the way `apply`
does, so a chord a user displaced is already gone from the list. A compositor
whose binds are all shared prints an empty list and gets no section.

Hyprland adds one more verb, `plugins`. niri has no plugin system and says so
in `caps` instead of shipping a verb that would fail.

## Capabilities

A capability is a promise the provider keeps in `act` or `apply`. Claiming one
it cannot perform is worse than omitting it: the desktop would offer a control
that does nothing. The Hub gates settings rows on these, so a row is never shown
with nothing behind it.

Shared by both providers:

    animations  focusHistory  keyboardLayoutSwitch  layerRules  monitorConfig
    nightLight  outputPower  paletteBorder  sessionExit  touchpadToggle
    windowFloat  windowRules  windowWorkspaceMap  workspaces
    workspaceMoveToOutput

Hyprland only:

    configReload  cursorSet  focusGrab  globalShortcuts  liveConfigEval
    outputHdr  outputMirror  persistentScreenCapture  plugins  screenShader
    specialWorkspace  submap  tiledLayout  windowGeometry

niri only:

    columnFill  keyboardGrabSharesPointer  nativeOverview  overviewBackdrop
    overviewState

The absences are each compositor's design, not gaps to fill later. The ones
that change what a user sees:

- **`nativeOverview`** is why the shell's own overview stands aside on niri:
  `Super+Tab` opens the compositor's overview.
- **`windowGeometry`** is absent on niri because tiled windows do not expose an
  on-screen position. Floating windows do expose their output-local tile
  rectangle; the provider converts it to global logical geometry so native
  move/resize placement can be persisted without claiming general geometry.
- **`globalShortcuts`** is absent on niri because it does not implement the
  protocol, so keybinds reach shell surfaces by running `ryoku-shell <verb>`
  instead. Every shell surface remains reachable.
- **`liveConfigEval`** and **`configReload`** are absent on niri because its
  config is file-only and niri watches it. The Hub applies on save rather than
  previewing live.
- **`persistentScreenCapture`** is absent because niri tears its outputs down
  and recreates them on a config reload or a mode change. A one-shot capture is
  fine; a long-lived whole-screen capture can race an output leaving and crash
  the Qt client. The launcher therefore uses a solid drawer on niri.
- **`keyboardGrabSharesPointer`** is absent on Hyprland because a layer surface
  holding the keyboard exclusively takes every pointer event too. The Stage
  Editor's canvas holds the keyboard on niri, so Escape and the arrows reach it
  from the moment it opens; on Hyprland it asks on demand and takes the keyboard
  as the pointer crosses the desktop, which keeps the editor's toolbar and
  catalogue clickable.
- **`submap`**, **`specialWorkspace`**, **`screenShader`**, **`plugins`** and
  **`cursorSet`** have no niri equivalent, so the binds and settings that need
  them are reported by `apply` rather than silently dropped.

`nightLight` is the one shared capability the desktop drives through named
actions rather than a settings row:

- `nightlight.on <K>` warms the screen to a colour temperature; the provider runs
  its own detached backend (`hyprsunset` on Hyprland, `wlsunset` on niri).
- `nightlight.off` stops that backend, and the compositor restores the gamma when
  it goes away.

`touchpadToggle` is the other shared capability the desktop drives through a
named action, plus a Hub switch that reads it live:

- `input.touchpad on|off|toggle` locks or unlocks the touchpad the FN touchpad
  key drives; `status` prints `on` or `off`, and `restore` re-asserts a stored
  off after a config reload. Hyprland flips the device live through `hyprctl
  eval`; niri, which has no runtime input IPC, records the intent in a state
  file and re-emits `off` into the config it watches.

`paletteBorder` is the shared capability that keeps the window border tracking
the wallpaper, driven by a named action every provider honours:

- `decoration.borderColors <active> <inactive>` recolours the border from the
  live palette. Hyprland pushes the colours into the running config through
  `hyprctl eval`; niri, which has no runtime config IPC, records them in a state
  file and regenerates the config it watches. Both are a no-op when the store
  pins a fixed colour (`desktop.appearance.borderFollowsPalette` off), so a
  wallpaper change never overrides a border colour the user chose.

`monitorConfig` adds two output actions beside the display settings it gates:

- `output.cycle` steps the output arrangement one position. Hyprland runs its
  display engine (`ryoku-monitor toggle`); niri walks the outputs over IPC,
  keeping the cycle position in a state file.
- `output.enable <connector> on|off` turns one named output on or off.

`workspaces` also backs `window.summon`, which the desktop's summon keybind
drives on every compositor:

- `window.summon <title>` raises an already-open window to the current
  workspace and focuses it, matched by exact title. A single-instance app
  strands its window on the workspace it first opened on, and a title is the
  only handle when every window of an app shares one app id; no match exits
  non-zero, so the keybind falls through to launching the app.

`liveConfigEval` drives one named action of its own, gated so a file-only
compositor is left alone:

- `decoration.gameMode on|off` strips the compositor's decorations for a
  latency-first gaming pass and reloads the config to restore them. A compositor
  that cannot evaluate its config live has no equivalent, so game mode still
  boosts power there and leaves the look untouched.

## Lock, lid and sleep

This stack is deliberately almost all compositor-neutral, and the one part that
cannot be sits behind the seam like everything else. Four owners, no overlap:

|Owner|Owns|
|---|---|
|**qylock**|the lock surface only, through Quickshell's `WlSessionLock` -- the Wayland session-lock protocol every provider serves, so the same lock runs on any compositor. The greeter is SDDM rendering the same skin; both halves are qylock.|
|**the shell daemon**|the fail-closed login1 transaction behind `ryoku-shell suspend`: the foreground-bound daemon owns one delay inhibitor and hard `sleep` block, secure lock-before-suspend, a confirmed guard release before the `Suspend` call (login1 must stop reporting the block, so the daemon's own inhibitor can never deny its own transaction), output wake on resume, and lighting restore. Before the singleton moves it proves the outgoing session's qylock; all other online same-user sessions have direct session-scoped qylock clients and login1 observers. Only the foreground session may request sleep or unlock. Lost login1 signal or owner connections reconnect in-process without dropping valid protection. Display work reaches the compositor solely through the seam's `output.power`, with a deadline on each wake attempt.|
|**`hypridle`**|the idle timers only -- dim, lock, screen-off, suspend. A suspend timer calls `ryoku-shell suspend`; hypridle never talks to login1 itself.|
|**`ryoku-clamshell`**|lid ownership for the active graphical session: a `handle-lid-switch` inhibitor, the docked AC-plus-external decision, and routing every non-docked close through `ryoku-shell suspend`. It drops ownership when that session becomes inactive, reacquires it when active, and recovers across login1 restarts.|

The lid's panel handoff is the one compositor-shaped piece: Hyprland binds the
lid switch to `ryoku-clamshell lid close|open` from
`ryoku/hyprland/modules/lid.lua`; niri sends both native `lid-close` and
`lid-open` switch events to the helper and retains its native output topology.
Only the active Ryoku session inhibits logind, so without a live owner logind's
safe fallback suspends on lid close. ACPI supplies live physical state when
available; UPower seeds an already-closed startup otherwise, but its
asynchronous value cannot veto a compositor edge. An open edge cancels a close
still waiting on dock or power state. No provider gains a capability for this:
a compositor that serves
`WlSessionLock` and switch events needs nothing further.

The logind drop-in that backs the sessionless lid fallback and the delay budget
is delivered by both lanes: the package ships it and a checkout deploy seeds
it, and an unowned copy is adopted on the next update rather than left to
collide with the package. A system watcher selects one confirmed active Ryoku
session per user and binds that user's target, idle and clamshell owners to its
login1 activity; stale compositor variables in a lingering user manager cannot
take ownership. Login plus live deploy, update, and package cutovers hold a
durable login1 sleep block, reload the active compositor config, replace the
shell, wait for its inhibitor readiness, verify idle and clamshell, and restore
Ryogami before releasing protection. Package hooks do that for every logged-in
Ryoku user. Doctor repair only stages a complete qylock generation under the
generation guard; the next managed shell activation performs the handoff. A
failed live cutover remains blocked until its retry succeeds or the machine
reboots. A staged checkout deploy does not reload live logind. See
`docs/updates.md`.

## Where the config lives

Each provider owns a directory under `~/.config`, and `ryoku/wm/detect.go` is the
only place that mapping exists. `ryoku wm config [name]` prints it, which is how
scripts read it without keeping a second copy.

|  |Hyprland|niri|
|---|---|---|
|Directory|`hypr/`|`niri/`|
|Repo payload|`ryoku/hyprland/`|`ryoku/niri/`|
|Entry|`hyprland.lua`, authored Lua|`config.kdl`, includes the rest|
|Generated by `apply`|`settings.lua`, `rebinds.lua`|`settings.kdl`, `rebinds.kdl`|
|Seeded, machine-owned|`monitors.lua`, `gpu.lua`, `keyboard.lua`, `user.lua`|the same names as `.kdl`, plus `monitors_user.kdl`|
|Portal backend|`hyprland`|`gnome`|

Two niri rules shape its tree:

- A **missing include is a hard config error**, and a niri with an unreadable
  config has no session. Every file `config.kdl` names must exist, so all five
  seeds ship and `apply` always writes both generated files even when they would
  be empty. Hyprland's Lua can skip a missing drop-in; niri cannot.
- **The last include wins**, so the order in `config.kdl` is the override chain:
  seeds, then the generated config, then `user.kdl`. A value you set in
  `user.kdl` beats both.

niri has no unbind. A duplicate chord in one `binds` block is a hard error, so
`rebinds.kdl` is the single, total keybind set: the shipped defaults with
rebinds applied, unbinds removed, and one winner per chord. There is no seeded
bind block to subtract from.

Two seeds are compositor-specific in practice. `ryoku-gpu` keeps its policy in
the Hyprland pin file (`gpu.lua`, the store the Hub GPU page and doctor audit).
niri picks its own render device, so its `gpu.kdl` only carries the cursor
policy.
`ryoku-monitor` follows the same rule for each provider's monitor seed. Every
declared seed exists from first login, and each remains available for manual
overrides.

What a variant package ships is the same rule seen from the packaging side: a
compositor's payload dir holds only what speaks that compositor's own IPC
(`ryoku-monitor`, `ryoku-workspace` under
`ryoku/hyprland/scripts/`). Anything the shell, the Hub, the launcher or a
keybind calls by bare name on every compositor lives in `ryoku/shell/scripts/`
or `system/hardware/` and ships with the shell or the base package, and it
reaches the compositor only through `ryoku wm act`. A variant-only script called
from neutral code is exactly the bug that made a packaged niri box miss its
app keys while a dev checkout, which used to lay every provider's scripts, never
noticed; `deploy.sh` now lays only the live provider's own leaf scripts so the
checkout tells the truth.

The switch lays the target's leaf scripts on a checkout box before declaring it
ready (`syncLeafScripts` in `ryoku/cli/wm.go`, resolving the directory through
`wm.LeafScriptsDir`). It has to: a deploy under one compositor lays only that
compositor's scripts, so switching to the other without this left the next
session's bare-name calls falling through PATH to a stale copy: a `ryoku-monitor`
from before the display engine's `apply` verb existed, which silently failed
every Hub display change and recomputed the scale from DPI at each login. The
sync is additive because the running session still belongs to the compositor
being left; pruning stays deploy's and the package's job.

## Switching

    ryoku wm use <name> [--keep-previous|--remove-previous]

It previews first: what carries over, what the target cannot honour and why, and
that the compositor you are leaving keeps its `wm.<name>.*` settings in the store
so they return if you come back. Then it installs `ryoku-desktop-<name>` as a
plain pacman transaction, which is what lets `ryoku rollback` undo the switch.
Both variants may be installed at once, so the switch never removes the desktop
you are leaving unless you ask it to: a second switch is a config change with no
package transaction at all. The Hub offers the same flow on its Global page.

A checkout box installs nothing: the switch declares the target ready and the
user picks it at the greeter. That promise is only true when the greeter can
actually boot the session, so `deployedProvider` (and the Hub's matching
`wmDeployed`) require `wm.SessionReady`: the compositor binary is installed
and a greeter directory lists `<name>.desktop`. A deployed provider binary and
a config tree are not enough on their own -- they are exactly what a checkout
leaves behind, and a switch built on them alone told the user to log out and
pick a session that was never there.

Keeping the old compositor installed means switching back needs no download, at
the cost of its packages staying on disk. Removing it reclaims those, which
`wm.Reclaim` (`ryoku/wm/reclaim.go`) computes from the outgoing provider's
package list: the compositor package and the satellites `ryoku-desktop-<name>`
installs (the provider declares them in `caps`, so nothing outside the seam
names one), plus the private dependencies that orphan with them. The switch
names how many packages and how much space that is, from the outgoing
compositor's own installed packages, so the choice is offered even on a checkout
box where the meta-package was never installed. A package the incoming
compositor also needs, or one another installed package still depends on, is
kept; and the removal is cross-checked against pacman's own plan and refused if
it would touch anything outside the reviewed set, so a switch can never break
the machine. Either way the settings survive, which is what makes removal safe.
Keeping is the default because it is the reversible choice.

Every `desktop.*` setting is compositor-neutral and carries over. Only
`wm.<name>.*` is compositor-exclusive, and it is never deleted, only left alone
while another compositor is active.

## Adding a window manager

This is the checklist. The walkthrough named at the top is the same job in
prose, with the traps; read that if you are doing this rather than reviewing it.

1. `ryoku/wm/<name>/`, a `package main` implementing the ten verbs. Mirror
   the nearest existing provider rather than inventing a second shape. Pin the
   compositor's own dialect in a test: the argv or request a provider emits is
   its contract, and getting it wrong usually fails silently.
2. Register the name in `ryoku/wm/detect.go`: `Provider<Name>`, its env handle,
   its config directory, and its seeds. That file is the only place allowed to
   know any of it.
3. `ryoku/<name>/`, the shipped config payload.
4. Its `schema`, listing the settings only this compositor has. That is the
   whole of its Hub presence: the rows appear on the Window Manager page with
   no Hub edit, and anything it cannot express is reported by `apply` instead
   of being shown as a control that does nothing.
5. `release/packages/ryoku-desktop-<name>/`, providing the compositor virtual so
   it is mutually exclusive with the others, and depending on the compositor plus
   whatever it needs for Xwayland and screencasting.
6. Offer it in the installers' compositor question.

What you do not do is add a branch anywhere else. If the desktop needs to know
something new about a compositor, that is a new capability or a new field on
`caps`, not a name test. The isolation gate enforces this, and it is the reason a
second compositor was an addition rather than a fork.

The one thing this does not cover is rich bespoke tooling. Hyprland's plugin
manager, animation curve workshop and layer-rule editor are interactions no
settings row can describe, so they remain capability-gated pages in the Hub that
read their rows from the provider. A compositor arriving with tooling of its own
would want to ship its own Hub pages, the way `ryoku/wm/hyprland/qml/` already
ships a QML module; that needs the Hub's component vocabulary published as a
module first.
