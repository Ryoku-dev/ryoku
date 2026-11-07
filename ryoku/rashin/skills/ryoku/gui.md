# The Ryoku GUI map

Ryoku has a GUI for almost every setting: the Ryoku Hub for system and shell
config, the wallpaper picker for looks, the Stage Editor for the desktop and the
dock, and the active subshell or bar style's own settings window for its bar.
When a user asks how to change something, name the surface first, then give
the command the page wraps (SKILL.md, "Answer policy: GUI first"). This file is
the lookup from an intent to its surface, its keybind, and the command behind it.

## Opening the Hub

Ryoku Hub is the settings app; it replaces the old reference settings window.
Super+comma opens it. To jump straight to a page, deep-link it by its section
key:

```bash
ryoku-shell hub open              # the Hub, last page
ryoku-shell hub open keybinds     # straight to Keybinds
ryoku-shell hub open gpu          # Graphics & Power
```

The keys are the ones in the table below; they are the same strings the Hub's
rail uses, so `ryoku-shell hub open <key>` and clicking the rail reach the same
page. Escape closes the Hub.

## The Hub pages

Every row is one page in the Hub rail. `hub open <key>` is shorthand for
`ryoku-shell hub open <key>`. Pages marked advanced show only when the Hub's
advanced view is on; gated pages appear only when the active compositor backs
them (see below), never as dead settings.

| Rail name | `hub open` key | Page | What it owns |
|---|---|---|---|
| Profile | `profile` | ProfilePage.qml | the user profile, avatar, and hero |
| General | `global` | GlobalPage.qml | language, regional formats, time zone and units, the system font and its size, blur and dim behind open windows, and Users account management |
| Updates | `updates` | UpdatesPage.qml | system updates and the release channel |
| Displays | `displays` | DisplaysPage.qml | monitors, resolution, arrangement, night light (gated: `monitorConfig`) |
| Connections | `connections` | ConnectionsPage.qml | wifi and bluetooth |
| Input | `input` | InputPage.qml | keyboard, mouse, and touchpad |
| Graphics & Power | `gpu` | GpuPage.qml | GPU mode, power profile, and the idle/lock timers |
| Animations | `animations` | AnimationsPage.qml | compositor animation curves (Hyprland only) |
| Lockscreen | `lockscreen` | LockscreenPage.qml | the qylock theme and lock behaviour |
| Window Manager | `windowmanager` | WindowManagerPage.qml | gaps, borders, tiling (gated: needs a backing provider) |
| Plugins | `plugins` | PluginsPage.qml | compositor plugins (gated: `plugins`) |
| Layer Rules | `layerrules` | LayerRulesPage.qml | layer-shell rules (advanced; shows only with rules present) |
| Subshells | `subshells` | SubshellsPage.qml | the gallery of complete shells (shipped Orpheus and Kairos, installed and Ryostore subshells) and the selected one's settings |
| Bar Studio | `bar-studio` | BarStudioPage.qml | the gallery of subshells and bar styles, the selected item's settings, the parts installed items provide (dock designs, launchers, desktop widget sets), and the built-in editors some bar styles use (Sumi's frame and rails, Obi, Nacre) |
| Desktop | `desktop` | DesktopPage.qml | the desktop stage and its surfaces |
| Controls | `controls` | ControlsPage.qml | the Super+Escape panel's section order and visibility |
| Overlays | `overlays` | OverlaysPage.qml | the worn OSD, notification card, and Super+Escape sidebar designs, with Try actions for each |
| App Launcher | `launcher` | LauncherPage.qml | how the Super+Space launcher behaves |
| Keybinds | `keybinds` | KeybindsPage.qml | every shortcut; the one compositor-specific page kept out of the compositor group |
| App Overrides | `appoverrides` | AppOverridesPage.qml | per-app tweaks (advanced) |
| Window Rules | `windowrules` | WindowRulesPage.qml | window placement rules (advanced) |
| Performance | `performance` | PerformancePage.qml | performance and gaming scripts (Hyprland only) |
| Session | `session` | SessionPage.qml | session and login behaviour |
| Recording | `recording` | RecordingPage.qml | screen recording |
| Dictation | `dictation` | DictationPage.qml | voice typing |
| Fastfetch | `fastfetch` | FastfetchPage.qml | the fastfetch splash (advanced) |
| Import config | `import` | ImportPage.qml | import settings from another config (advanced) |
| Add-ons | `addons` | AddonsPage.qml | Ryostore add-ons and extras |
| Rashin | `rashin` | RashinPage.qml | the Rashin agent OS: enable, wire, the vault, the dashboard |
| Credits | `credits` | CreditsPage.qml | credits |

When someone asks about the look of an OSD, a notification card, or the
Super+Escape sidebar, send them to Settings -> Overlays. The page keeps
installation separate from selection and has a Try action for each surface; the
OSD action flashes the volume display through `qs -c shell ipc call osd
preview`.

When someone says the desktop blurs or darkens behind an open app, on any
compositor, send them to Settings -> General -> Appearance -> *Blur and dim
behind windows*. Off removes both the blur and the darkening; it is not the
window shadow, which lives on Window Manager -> Look.

## Installable profiles

Profile discovery starts in **Ryostore > Profiles**. **Install** places the
official profile definition. **Create account** then opens **Ryoku Hub > Global
> Users > Add person > From a profile**, where the user chooses the separate
standard account, tools and extra packages, reviews the complete plan, and
approves it.

An applied profile account has a Profile tools section on its Users card:

- **Edit tools** changes core, optional and extra packages through a new plan.
- **Reset settings** previews and restores profile-managed files. It never
  touches user-data folders or anything the profile did not lay.
- **Checkpoints** saves, lists, reverts or deletes btrfs home checkpoints.
  Revert is unavailable while the account is logged in.
- **Remove profile** previews packages and system pieces before offering
  Archive home, Keep account or Erase home.

The profile account's own Global > Users view also has a Profile account card
for reset preview, reset and checkpoint save. A profile account stays standard
and cannot manage users.

## Surfaces outside the Hub

Wallpaper, theme, the dock, the active item's bar layout, the launcher,
Controls, ryoshot, and Ask are outside the Hub. Reach them here:

| Intent | GUI surface | How to open | The command behind it |
|---|---|---|---|
| Change wallpaper or theme | Wallpaper picker | Super+W | `ryogami wallpaper ui` (picker), `ryogami wallpaper set\|next\|random` |
| Pick the subshell or bar style | Ryoku Hub > Subshells or Bar Studio | `ryoku-shell hub open subshells`, `ryoku-shell hub open bar-studio` | `ryostore install <subshells\|barstyles> <id>`, then `ryoku-shell barstyle <id>` |
| The active item's own settings (bar layout, modules, look) | Its settings window: Bar Studio > OPEN SETTINGS, or the item's own click (Orpheus: left-click its 力 logo) | only while that item is active | the call desktop.md lists for it, e.g. `qs -c shell ipc call orpheus settings ""`, `qs -c shell ipc call kairos settings`, `qs -c shell ipc call qsbar settings ""` |
| QS Bar layout and widgets (while QS Bar is active) | QS Bar Settings | the bar's launcher mark, or `qs -c shell ipc call qsbar settings <route>` | `qs -c shell ipc call qsbar ...` (see bar.md) |
| Dock | Stage Editor (Super+Shift+D), Dock section; Bar Studio > OPEN DOCK SETTINGS lands there too | `qs -c shell ipc call desktop editSection dock ""` | `ryoku-shell dock ...` (see bar.md) |
| Launch an app | App launcher | Super+Space | the shell launcher (the App Launcher page tunes it and picks launchers installed items provide) |
| System activity, connections, levels, and session actions | Controls (its look is picked in Hub > Overlays) | Super+Escape | `ryoku-shell quicksettings` |
| Screenshots and recordings | ryoshot capture bar | Super+Shift+S | `ryoku-shell screenshot` |
| Read-only answers, the persistent machine Agent, tools, and web search | Rashin bar | Alt+Space | `ryoku-shell ask`, `ryoku-shell ask chat`, `ryoku-shell ask tools`, `ryoku-shell compress`, `ryoku-shell install` |
| Agent, Ask, plain Chat, vault, models, and Prowl controls | Rashin console | Super+Alt+Space | `rashin-app` (single-instance: a second press focuses it) |
| Install or create a separate account profile | Ryostore > Profiles, then Ryoku Hub > Global > Users | open Ryostore, choose Profiles | `ryostore settings profiles <id>` hands off to the Hub's `users fromProfile` IPC |

An item that is installed but not active is not loaded, so its settings window
and IPC target do not exist; Bar Studio offers SWITCH NOW for it instead. QS Bar
Settings routes are `bars`, `identity`, `layout`, `widgets`, `dock` and
`community`; its Dock page hands off to the Stage Editor. bar.md covers
subshells, bar styles and the dock in full.

## Compositor-gated pages

The Hub shows no page for a capability the running compositor lacks. It gates
Displays (`monitorConfig`), Plugins (`plugins`), Window Manager (a backing
provider), and Layer Rules (shown only with rules present); each appears only
when the active compositor supports it. Animations and the performance and gaming
scripts are Hyprland-specific and do nothing under another compositor. Keybinds
is the deliberate exception: it is compositor-specific yet stays a top-level page
rather than sitting in the compositor group. A setting you change for a
compositor applies only after switching to that compositor.

## A worked answer

The user asks: "how do I make the screen lock after ten minutes?" The GUI-first
answer names the page first:

> Open Ryoku Hub with Super+comma and go to Graphics & Power > Idle, or jump
> straight there with `ryoku-shell hub open gpu`, then set the lock timer to ten
> minutes.

When the user asks YOU to set it, drive the idle daemon directly
(`ryoku-power idle set ac.lockSec 600`, then `ryoku-idle apply`), then tell them
it now locks after ten minutes and that Graphics & Power > Idle is where they see
or change it. Same shape for wallpaper: point them at Super+W first; when you set
it yourself, use `ryogami wallpaper set <path>` and tell them the picker is
Super+W.
