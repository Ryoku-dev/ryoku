# Changelog: ryoku/

## Unreleased

### Added
- **One keybind catalogue for both compositors.** `wm/binds.go` names every
  shipped shortcut once (id, label, hint, category, default chord), and each
  provider's `binds` verb reports the full effective legend: what it bound,
  what the user rebound, and what it cannot do with a reason. New shortcuts on
  both desktops: Page Up/Down and Shift for workspace navigation and sending
  windows, Super+Alt+arrows to focus a screen and Shift/Ctrl variants to send a
  window or a whole workspace there, Alt+Tab for the last window, Super+T for
  a tabbed column or group, Super+D to maximise, Super+C to centre, Super+[ ]
  to merge a window into its neighbour, and the number pad for workspaces 1 to
  10 with NumLock on or off. niri also gains reorder-workspace (Super+Ctrl+Page
  Up/Down), first and last column, preset window heights and a shortcut
  inhibit toggle; Hyprland reports those as not available instead of hiding
  them (`wm/niri/config_binds.go`, `wm/hyprland/legend.go`).
- **The niri border follows the wallpaper palette, like Hyprland's.** The
  provider recolours the border from the live palette on every wallpaper or
  scheme change through a shared `paletteBorder` capability, and a new
  `Follow the wallpaper` switch on the Hub's Borders tab pins the fixed colours
  instead on either compositor (`wm/niri/act.go`, `wm/hyprland/act.go`,
  `shell/ipc/matugen.go`, `hub/quickshell/schema/WindowSettings.js`).
- **The niri provider models what niri 26.04 can do.** Blur (global passes and
  noise, per-window and per-app background effects), a border or focus ring
  choice with gradients, the workspace background, tabbed columns, preset
  window heights, the overview backdrop colour and workspace shadow, nine
  per-animation springs and curves, layer rules, the `columnwidth`, `minsize`,
  `maxsize`, `scrollfactor`, `tiledstate`, `babaisfloat`, `noshadow`, `blur`,
  `xray` and `blockout` window-rule actions, and the warp-to-focus, scroll
  cap, auto back-and-forth, mod key, power key and drag-edge input knobs, each
  a store key with a Hub row. The emitter is one concern per file and the
  unhonored reasons name what niri really lacks (`wm/niri/config_*.go`,
  `wm/niri/schema_*.json`, `wm/niri/apply.go`).
- **Seam actions for what used to be Hyprland-only scripts.** `nightlight.on`
  and `nightlight.off` with a `nightLight` capability, `input.touchpad` with
  `touchpadToggle`, `output.cycle` and `output.enable`, `window.summon` and
  `decoration.gameMode`; providers also publish the window-rule actions they
  honour (`wm/caps.go`, `wm/action.go`, `wm/hyprland/act.go`, `wm/niri/act.go`).

### Fixed
- **niri draws the window border the user sized.** niri 26.04 keeps its border
  off unless the block carries an explicit `on`, so the sized border never drew
  and the thickness slider did nothing; per-app overrides resolve the same way
  (`wm/niri/config_layout.go`).
- **A packaged niri box gets every helper the shell calls.** The neutral leaf
  scripts ship with the shell, hypridle and hyprpicker are base dependencies,
  idle management renders its own config from the Hub policy, and the dev
  deploy lays only the live provider's scripts so a checkout no longer masks
  what a package is missing (`release/packages/*/PKGBUILD`, `shell/deploy.sh`,
  `system/hardware/power/ryoku-idle`).
- Settings rejects null desktop save and preview requests before they can
  replace saved preferences. Empty JSON stores containing `null` now recover
  as an editable empty store instead of crashing the next edit.
- Saved window animation overrides using `snap` no longer report a missing
  bezier after switching presets. The base loader supplies the Minimal curve;
  active presets and custom curves can still override it.

### Added
- **`CardColumns`: a page body of blocks, laid into balanced columns.** It takes
  the children a page already declares, measures them at the column width, and
  splits them so the columns end level, with a `fullWidth: true` child taking a
  band across them; `colWidth` is published for a delegate that sizes itself to
  its column.
- **A numeric readout is typed into.** `SettingRow` turns a stepper's number or a
  slider's percentage into a field on click or Enter and reports the typed text,
  which `SettingsSheet` clamps into the row's own range and stores in the kind the
  row speaks.
- **A folded card says what it hides.** `SettingCard.summary` puts a count
  ("4 SWITCHES") in the header of a collapsed group, because a folded card with no
  trace of its contents reads as an empty one.

### Fixed
- **A Hub page holds still.** Section switches used to flicker: the incoming page
  was centred from its measured height and had its rows inflated by a timer that
  fired for up to three seconds after it appeared, so cards hopped while the reader
  was already looking at them. Content now anchors to the top, the fly-in is the
  crossfade alone, and the page's first layout is synchronous.
- **Every Hub scroller answers the wheel.** The rail, the page bodies and the
  popover lists showed a scrollbar that ignored the wheel, so a page taller than the
  window could only be dragged, and the rail's lower sections could not be reached
  by hand. `WheelScroll` scrolls them, clamped to their bounds.
- **Bar Studio uses the window.** BAR STYLE is a full-width band that fills its rows
  with tiles instead of cramming eight into half the page, and the LAYOUT card no
  longer promises controls that are not below it.
- **Widget previews stay inside their card.** A preview could be scaled past its
  natural size and clip flush against the footer, so the clock's digits ran into the
  widget's name and toggle; previews are now capped at their natural size and the
  clock reports its real height.
- **Descriptions are readable again.** `inkMuted` and `inkFaint` sat below the
  4.5:1 floor this document promises once the wallpaper dimmed the palette; both
  tiers are raised.
- **The Profile page says which compositor you are running.** The dossier read
  "Hyprland" from a hardcoded name; it now reads the session's own compositor.
- **A tab never moves its neighbours.** Every tab plate reserves the `//` lead as
  a slot and only inks it when active, so selecting a tab no longer widens that
  plate and shoves the others sideways, on any page.
- **A page's head and grid hold still across tabs.** The sheet's geometry comes
  from the page's width, not from how many groups the open tab happens to have.
  A one-group tab (Window Manager's `BORDERS`, `MOTION`) used to shrink the sheet
  to a narrow column in the middle of the window and drag the title and tab row
  with it, which read as the whole page shuffling.
- **Unfolding a group no longer moves cards between columns.** The column split
  is kept until the set of visible blocks changes, so a drawer that opens spreads
  only its own column; measured: cards in other columns keep their exact x and y.
- **The Updates version pairs are readable.** Each pair is two columns with the
  incoming version carrying more ink than the one it replaces, instead of one
  run-on string at the smallest type.
- **The Hub's page corners are quiet.** The decorative marginalia strips beside the
  FILES and UPDATES buttons are gone: they crowded those controls and carried
  hand-written group indices that were often wrong.

### Changed
- **The Hub's chrome is one fixed layout, not a per-page guess.** The `FILES` and
  `UPDATES` chips are control-sized (`30` tall, `S4` padding) and take the page's
  own right inset, so they line up with the last card instead of floating inside
  it, and Profile's `EDIT` sits below them rather than stacking under `UPDATES`.
- **A tab switch fades instead of snapping.** The card set crossfades on a tab
  change the way a page swap does, and a disabled control stays legible at half
  opacity rather than nearly invisible.
- **Window Manager's cannot-do list folds.** The tab keeps the list, behind a
  header that says how many settings it holds, instead of opening on a wall of
  near-identical lines.
- **Input's keyboard band carries the facts.** The layout, variant, and the caps,
  compose and switch keys sit beside the diagram, and the pointer descriptions
  fit one line each.
- **Every hand-built list is on the settings-row rhythm.** Import's "what it
  brings over" plate, Updates' package and commit rows, and Displays' saved
  profiles drew their own tighter spacing (8px between lines, text close to the
  container edge); they are now cards and rows with `S4` insets and a hairline
  between rows, the same as every schema page.
- **Every description fits its row.** A settings description is one line and 60
  characters at a three-column card, a page blurb 70. Measured on the live Hub:
  no description elides any more (was 8), and the two-line rows fell from 52 to 7,
  all of them rows whose own control is wide.
- **Autostart and Environment are one Session page.** Two thin pages became one
  with two named clusters (what runs at login, the variables the session exports);
  an old deep link or the remembered section still lands on it.
- **`Sw` shows its state twice.** The track tints and the knob fills as the switch
  goes on, because an off switch drawn as an empty outline reads as an unchecked
  box at a glance.

### Added
- **The Hub fills the window it opens in.** A framed page takes the width beside
  the rail instead of a centred `Tokens.pageMax` column, its head sits on the body's
  grid, and the card grid runs as many columns as the measure holds (three on a
  page-wide window). A page with few groups gets fewer, wider columns rather than
  a lone card beside a window-wide gap. `Tokens.contentMax` still caps a stack of
  prose, and a page that reads better short caps itself.
- **The picker's option count is gone from the row.** `PickBar` showed how many
  options the catalogue held beside its chevron; that number means nothing to a
  reader, so the chevron alone is the affordance now.

### Removed
- **The Hub's orphaned rices schema.** `schema/RicesPage.js` was a generated
  inventory with no consumer left after the rices moved to ryogami.
- **The decor level and its rich tier.** `Tokens.decor` and the flags derived
  from it (`decorRich`, `decorMinimal`, `showPosters`, `showGrid`, `showGrain`,
  `showSeals`, `monoHeads`) are gone, along with the `hubDecor` shell.json key
  they read: one setting voice ships now. `fTitle` is 32 everywhere,
  `SettingCard` and `Cell` use sentence case at every width, `Grain` keeps a
  fixed art opacity, and the poster plates `Decor`, `Placard`, `DitherField` and
  the `DecorStore` singleton are deleted from `Ryoku.Ui` (their only callers were
  the Hub's rich tier and ryovm's hero plates).

- **The bar stream can drift again when nothing is playing.** A new "Drift when
  silent" switch keeps the gap stream animating on any power profile, not only
  Performance. It sits next to Gap animation in the bar control centre and under
  Settings > Performance, ships off so a quiet desktop still idles cheap, and
  music animates the bar either way (`shell/services/Perf.qml`,
  `bar/barstyles/qsbar/controlcenter/routes/BarsRoute.qml`).
- **Ryotunes skins are a RyoStore category.** RyoStore serves the community
  Ryotunes-skin catalogue as `ryotunes-skins` (group `wear`): a skin installs
  as a generic product into `~/.local/share/ryoku/ryotunes-skins/<id>/` with a
  receipt, and Ryotunes searches it as its "store" skin source. The store only
  installs and removes; the worn skin is chosen in Ryotunes (Settings ›
  Appearance, or `ryotunes-cli skin use`), and an item reads as active when
  `~/.config/ryotunes/client.json` `skin` equals its id. `ryostore open
  ryotunes-skins` opens the store on it
  (`apps/ryostore/backend/provider_ryotunes_skins.go`).
- **Key sounds: a keyboard sound on every key press, from the compositor.**
  `hyprland/plugins/keysounds` is Ryoku's first compositor plugin: Hyprland
  sees every key before any app does, so one plugin covers every window with
  nothing grabbing input. Samples play through libcanberra (the sound server
  caches them, so a press costs one small request and presses mix there), a
  random variant per press, with dedicated samples for Space, Enter and
  Backspace and their releases when the profile has them. Eleven profiles ship,
  each a recording of the real switch cut from the MIT-licensed Mechvibes packs
  at build time and named for it: cherry-mx-blue, cherry-mx-brown (default),
  cherry-mx-black, cherry-mx-red, topre, creamy, nk-cream, holy-panda, tealios,
  crystal-purple, oreo. `ryoku-keysounds-import <pack>` turns any Mechvibes
  pack into a profile of your own under `~/.local/share/ryoku/keysounds/`, and
  a plain folder of samples named by role is one too. Off by default;
  Settings > Plugins > Key sounds turns it on, picks the switch, sets the
  volume; a change is heard as it is picked. Package `ryoku-keysounds`.
- **Hyprland plugins rebuild themselves after a Hyprland bump.** Every plugin
  copy Ryoku builds carries an `.abi` receipt (the compositor build it was
  compiled for), `ryoku doctor` rebuilds each enabled plugin whose receipts no
  longer match the installed headers on every `ryoku update`, and `deploy.sh`
  builds through the same `ryoku-hub hypr plugins rebuild --stale` instead of
  its own makepkg loop, rebuilding on any ABI change rather than only a
  Hyprland version change, which is what left a cursor-motion plugin refusing
  to load after an aquamarine bump (`cli/internal/doctor/reconcile_hypr_plugins.go`,
  `shell/deploy.sh`; see `docs/hyprland-plugins.md`).
- **A documented fix for TVs and ultrawides that refuse a resolution.** Some
  panels (LG ultrawides, TVs over HDMI) list a mode but snap back to a smaller
  one, with Hyprland logging "REJECTED available mode" beside "atomic drm
  request: failed to commit" -- the DRM backend rejecting buffer-format modifiers
  for that sink. `monitors_user.lua.example` now documents the `AQ_NO_MODIFIERS=1`
  override that cures it (Ryoku already sets it on AMD/Intel but not nvidia, where
  it can crash a hybrid laptop, so a single-GPU nvidia TV box sets it by hand),
  plus a note on Hyprland's whole-pixel scale rule
  (`hyprland/monitors_user.lua.example`).
- **Ryotunes joins the matugen app suite.** The native music client reads its
  palette as a Ryoku skin: matugen fans the Material 3 roles into
  `~/.config/ryotunes/skins/matugen/skin.json`, the user skins dir where the
  client's Skin singleton loads it as the "System" theme, regenerated on every
  wallpaper change (`shell/matugen/templates/ryotunes.json`,
  `shell/matugen/apps.toml`, `shell/ipc/matugen.go`, `../hub/backend/matugen.go`).

### Fixed
- **One shared Bluetooth name resolver, used by every surface.** `Ryoku.Ui`
  gains `BtName`, which turns a Quickshell Bluetooth device into the name to
  show: it keeps a real user alias, otherwise takes the device-reported name,
  and skips a BlueZ alias that is only the device's own address
  (`ui/lib/bluetooth.js`).
- **Niri screen sharing works with the GNOME portal again.** Ryoku's
  `GDK_BACKEND=wayland,x11,*` preference was inherited by
  `xdg-desktop-portal-gnome`, which then treated the Niri session as an
  incompatible display server and exposed Settings only, leaving OBS,
  browsers and Electron clients without a ScreenCast backend. The portal
  service now drops only `GDK_BACKEND`, while normal GTK applications keep
  Ryoku's Wayland-first preference
  (`shell/systemd/user/xdg-desktop-portal-gnome.service.d/10-ryoku.conf`).
- **The gap stream clears when it stops instead of freezing a frame.** A silent
  bar with no drift left the last shader frame stuck in the gaps; it now hides,
  so the stream reads as off, then on when audio returns
  (`bar/barstyles/qsbar/modules/StreamShader.qml`).
- **Settings search for the workspace and launcher marks lands on the right tab
  again.** Those controls moved to the bar control centre's Identity tab, but
  the search index still sent "Workspace marker" and "Launcher mark" to the old
  Widgets page, so a search dropped you where they no longer were. They route to
  Identity now (`shell/modules/bar/barstyles/qsbar/controlcenter/ControlCenter.qml`).
- **Floating windows fit the screen they open on (#147).** Files, Ryoku
  Settings, Ryostore, Ryovm and the other fixed-size floating rules asked for
  1500x850 or bigger, so on a 1366x768 panel Files opened larger than the
  monitor and read as a stuck fullscreen that no dconf setting could undo. Every
  size in `hyprland/modules/window_rules.lua` is now capped at the monitor
  (92% wide, 88% tall) through Hyprland's size expressions, so the same rule
  fits a laptop panel and a desktop monitor.
- **Rashin chat follows a hermes reconfigure (#145).** Switching hermes to a
  new provider in the terminal (say NVIDIA NIM) left the dashboard chat failing
  with "HTTP 404" on every turn: the daemon re-applied the model it had
  remembered from before the switch onto the new endpoint, and the resident
  `hermes acp` process kept the provider and keys it loaded at spawn. The
  remembered pick is now tied to the hermes config it was made under and
  dropped when that changes, and a chat turn after `config.yaml` or `.env`
  changed respawns the session first (`rashin/backend/`).
- **Google Chrome loads pages and shares the keyring, like Chromium.** Ryoku
  pinned native Wayland and the GNOME keyring for Chromium (`chromium-flags.conf`)
  and for Electron apps (`ELECTRON_OZONE_PLATFORM_HINT`), but Google Chrome reads
  a different file, `chrome-flags.conf`, which nothing shipped -- so Chrome alone
  fell back to Xwayland, where on an NVIDIA card it renders pages blank while
  Discord and Chromium (both native Wayland) work. The one flags source now lays
  as both `chromium-flags.conf` and `chrome-flags.conf`, so Chrome gets
  `--ozone-platform=wayland` and `--password-store=gnome-libsecret` too. Delivered
  to fresh and existing boxes by the package and `materialize`.
- **Animations no longer break the desktop on a non-default preset.** Ryoku's
  signature window curves (`ryokuBloom`, `ryokuSettle`, plus `easeOutQuint`,
  `quick`, `almostLinear`, `ryokuWobble`) were defined only by the `ryoku`
  animation preset, yet the Hub's generated `settings.lua` references them for
  window motion on whatever preset is active -- so switching to any other preset
  (the dusky ports: bounce, air, fade, minimal, ...) left those beziers undefined
  and Hyprland threw its config-error overlay ("no bezier ryokuBloom /
  ryokuSettle"). The preset loader (`modules/animations.lua`) now defines these
  base curves before any preset loads, so `settings.lua`'s references always
  resolve; it ships in the desktop package, so `ryoku update` (materialize) heals
  an already-broken `settings.lua` with no regeneration.
- **Screen recording works on hybrid-GPU laptops again.** On a machine whose
  panel is driven by a card gpu-screen-recorder can't enumerate (an AMD iGPU
  display alongside a discrete NVIDIA GPU), the backend probe mistook the webcam
  gsr lists (`/dev/video0|1920x1080@30hz`) for a monitor and launched a KMS
  capture that died at once with "no /dev/dri/cardX device found", so recording
  closed instantly. With a live wallpaper up it fell to wf-recorder, whose VAAPI
  can't encode the compositor's dma-buf here, so it dropped to software and
  choked after ~5s. Such a box now records through gpu-screen-recorder's portal
  capture (PipeWire, GPU-encoded, and it composites the live wallpaper in), and
  the probe no longer counts a webcam as a monitor. KMS capture still leads where
  it works, so ordinary machines are unchanged. Ryoku Motion (studio) capture
  shares the path and is fixed too.
- **Recording no longer saves an empty clip when the desktop portal stalls.** On
  the hybrid boxes that record through gpu-screen-recorder's portal capture, some
  xdg-desktop-portal backends never negotiate a stream -- gsr stays alive but
  writes nothing, and a stalled gsr ignores SIGINT/SIGTERM so even stop couldn't
  end it. The recorder now judges the capture by frames actually landing, force-
  kills a stalled gsr, and falls back to wf-recorder (caching that backend so the
  next capture skips the doomed portal probe).
- **The dock's window preview no longer opens alongside the music card.**
  Hovering the dock's music chip raised its now-playing card but also popped the
  app window-preview strip, because the band suppressed the app preview from a
  cursor coordinate that could desync from the chip. It now keys the suppression
  off the chip's own hover -- the same signal that raises the music card -- so the
  two surfaces are mutually exclusive.
- **Device lighting is re-applied on resume from suspend.** Theme colours only
  reached the RGB devices on a palette change and at login, so after waking from
  suspend an OpenRGB motherboard/RAM/mouse (which reset on power loss) sat on its
  firmware default and the keyboard could hold a stale colour. hypridle's
  `after_sleep_cmd` now also runs `ryoku-hub lighting apply` (backgrounded, so it
  never delays the screen coming back).
- **Video decode no longer freezes on hybrid laptops.** The session forced the
  nvidia VA-API/GLX drivers whenever the nvidia driver merely existed, so on a
  hybrid where the Intel or AMD iGPU drives the panel, video froze. It now takes
  the nvidia path only when nvidia actually drives the internal panel -- read
  from which card's connected eDP/LVDS/DSI connector reports -- and lets mesa
  auto-detect otherwise.
- **Recording no longer fails when a live wallpaper is up on a hybrid GPU.**
  With a live wallpaper the recorder captures through the desktop portal (gsr's
  KMS path drops the wallpaper layer); on some multi-GPU AMD boxes the portal's
  DMA-BUF negotiation fails, but gsr stayed alive long enough to pass the old
  2.5s liveness probe, so the recorder committed to a dead capture and cached the
  portal as working. The probe now requires the output file to actually grow
  before committing; a portal that produces nothing is marked bad and the capture
  falls back to wf-recorder (which records the composite, live wallpaper included).
- **GTK and Qt apps prefer native Wayland, so they scale crisply instead of
  breaking at fractional scales.** The session now sets `GDK_BACKEND`,
  `QT_QPA_PLATFORM`, `QT_AUTO_SCREEN_SCALE_FACTOR` and
  `QT_WAYLAND_DISABLE_WINDOWDECORATION` to name Wayland first (falling back to
  X11). On Wayland an app takes the compositor's fractional scale; under XWayland
  the same app hits `force_zero_scaling` and, at a non-integer or sub-1 scale,
  drew at 1:1 in the corner of a logical-sized window with empty margin around it.
  XWayland output now also uses nearest-neighbour filtering, keeping the residual
  scaled apps pixel-crisp rather than blurred. Ported from caelestia, dusky, and
  omarchy.
- **Rashin's one-click Hermes setup no longer stalls forever at "installing
  chrome" and "uv lock".** Setup ran the upstream installer as `curl | bash`,
  leaving its stdin on the pipe, so its optional-package steps (Playwright
  Chromium, whose deps helper is apt-only and hangs on Arch; and the `uv sync`
  build tools) fell back to a hidden `/dev/tty` prompt and blocked on a y/n and a
  sudo password the floating terminal never showed. Setup now downloads the
  installer and runs it `--non-interactive --skip-browser --skip-computer-use`
  (the desktop's own chromium covers the browser), and `ryoku-rashin` ships `gcc`
  so uv's native builds work without that apt-only helper
  (`rashin/backend/setup.go`, `release/packages/ryoku-rashin/PKGBUILD`).

### Added
- **`Spawn` (`Ryoku.Ui.Singletons`): the one way a surface starts a process.**
  `Spawn.run(argv)` (and `environment: Spawn.env` for a declarative `Process`)
  launches with Quickshell's private crash-recovery variables unset. An instance
  that has crashed once keeps `__QUICKSHELL_CRASH_INFO_FD` in its environment,
  naming the memfd of the config it came back from, and Quickshell reads it before
  it parses any argument, so a child inheriting it relaunched the desktop instead
  of starting itself. Every launch that can reach Quickshell, a desktop entry, an
  app or a terminal now goes through it; `ryoku-app` drops the same variables
  before it execs the terminal, so what a user types there is clean too. The rule
  is in `docs/conventions.md`.
- **The screenshot tool got the upgrade it needed, and learned to pin.** ryoshot
  now carries fourteen single-key tools that remember their own colour, width and
  fill, eight grips that recrop the captured region after the fact, a proper
  redaction that cannot be read back, a spotlight lens, an OCR region grab, Shift
  constraints, a colour picker with an eyedropper, and persisted settings. Ctrl+P
  pins the finished shot to the desktop as a floating card that outlives ryoshot,
  served by a new on-demand `ryopin` surface. The whole surface, Beautify
  included, now retints from the wallpaper palette like the rest of the desktop.
  See `ryoku/shell/CHANGELOG.md` for the detail.
- **`ryoku-app <role> -- <program>`** runs a program inside the app the role
  points at, so the shell can put a `Terminal=true` desktop entry in the user's
  own terminal without every caller learning that kitty and foot take trailing
  argv while alacritty and xterm need `-e` (`hyprland/scripts/ryoku-app`).

### Fixed
- **Rashin's vitals stream stops waking a sleeping discrete GPU.** `gpuVitals`
  called `nvidia-smi` on every tick and the `/ws/vitals` websocket ticks every 2s,
  so any connected Rashin client dragged a runtime-suspended card back out of D3
  (about 10 W on a hybrid laptop) twice a second and held it awake for the whole
  session. It now checks `power/runtime_status` first -- one sysfs read, no process
  spawn, and it never touches the card -- and reports no GPU while it sleeps, the
  same nil it already returned when nvidia-smi was absent
  (`rashin/backend/vitals.go`).
- **quickshell no longer SIGABRTs on every follow-wallpaper palette change.**
  `ryoku-cmd-folders` minted a brand-new timestamped icon-theme directory per
  palette change and switched the active theme straight to it, so the shell
  resolved a folder icon against a theme tree being deleted out from under it and
  aborted inside `QQuickPixmap::load` (a `__cxa_pure_virtual` call under
  `QIcon::pixmap` -> `QPlatformPixmap::fromFile`, issue #47). The script now keeps
  one fixed theme name whose content is published by an atomic symlink rename, so
  a concurrent reader sees either the whole old tree or the whole new one; it
  holds the previous generation one extra cycle so an in-flight lookup still
  resolves, and reaps the per-change directories older boxes leaked under
  `~/.local/share/icons` (`hyprland/scripts/ryoku-cmd-folders`).
- **Chromium's "is sharing your screen" bar no longer sits dead in the middle of
  the desktop.** On native Wayland that widget maps with an empty app_id and its
  geometry computed against the wrong work area (Chromium 517327175), so it lands
  mid-screen and accepts no pointer input at all: with the cursor confirmed on
  Stop sharing, and again on Hide, neither button fires, and there is no Chromium
  flag or policy that suppresses it. A window rule parks it on a special
  workspace instead, silently so the workspace never opens, and `no_focus` stops
  a widget you cannot click from taking the keyboard. The browser's own tab
  indicator and the site's stop control still show a capture is live
  (`hyprland/modules/window_rules.lua`).
- **Ryoku Settings groups keybinds under real section names again.** The legend
  read binds.lua a line at a time and opened a category on every comment line, so
  a section whose comment ran over several lines produced one empty category per
  line and filed its binds under the closing sentence of the paragraph. The
  Keybinds page listed eight empty groups and put the workspace binds under "on.
  Super+Alt+N sends the active window to that slot, staying on this desktop."
  Only the line that opens a comment block titles a category now, cut at its
  first sentence (`hub/backend/keybinds.go`).
- **Screen sharing asks which window or display to share again.** Nothing ever
  stops `graphical-session.target`, so `xdg-desktop-portal`, which is only
  `PartOf=` it, survived logout and kept answering for a compositor that had
  exited. Every ScreenCast request then timed out inside the frontend instead of
  reaching the hyprland backend: no source picker opened, and Discord, Chromium
  and every Electron client were handed nothing and reported the share as simply
  not working, with no error anywhere the user could see. The autostart now
  restarts the portal services once the desktop is up, and `ryoku doctor` heals a
  running session by comparing the frontend's start time against the
  compositor's (`hyprland/modules/autostart.lua`,
  `cli/internal/doctor/reconcile_portal_session.go`).
- **An autostarted or D-Bus-activated app now sees the session's environment.**
  The env push named five variables by hand, so anything systemd or D-Bus
  launched, an `~/.config/autostart` entry or the file manager over
  `org.freedesktop.FileManager1`, missed every other one: without
  `ELECTRON_OZONE_PLATFORM_HINT` an Electron app can land on Xwayland where it
  has no working screen capture, and without `GSK_RENDERER=gl` a GTK4 app takes
  the Mutter-only renderer path that never connects on wlroots and hangs at
  startup. It pushes `--all` now, so a variable added in `env.lua`, the Hub's
  Environment page, or `user.lua` reaches both launch paths with no second list
  to keep in step (`hyprland/modules/autostart.lua`).
- **Hyprland no longer crash-loops at login on hybrid NVIDIA boxes.** The mesa
  screencopy workaround `AQ_NO_MODIFIERS=1` forced modifier-less buffers NVIDIA
  can't import, so the first multi-GPU commit failed `drmModeAddFB2` and SIGABRTed.
  It is now set only when no NVIDIA driver is present (`hyprland/modules/env.lua`).
- **Ryoku Settings no longer goes invisible after switching to another monitor.**
  The window sized itself from `win.screen`, which briefly dangles during a
  monitor switch (non-null but reporting size 0); the unguarded read drove
  min/maximumSize negative and Quickshell forwarded that to the compositor,
  shrinking the toplevel to nothing (invisible and click-dead) while `qs` kept
  the single-instance lock so reopening no-oped. The read now requires a positive
  size (`hub/quickshell/shell.qml`).
- **Unplugging an external monitor no longer strands the remaining display.**
  Hotplug handling only re-ran DPI autoscale on `monitor.added`; `monitor.removed`
  now re-runs it too, so undocking re-lays out and re-scales the monitors that
  stay instead of leaving one at the gone display's offset with a blank wallpaper
  (`hyprland/modules/displays.lua`).
- **The Steam overlay and games no longer lag from the desktop compositor.** A
  game window (Steam, `steam_app_*`, gamescope) already dropped blur and shadow
  but still vsynced through the compositor, so opening the Steam overlay broke
  direct scanout and tanked the frame rate. Game windows now take the low-latency
  tearing path by default (`general.allow_tearing` plus an `immediate` rule on the
  steam-native match), cutting input lag and the overlay's frame-time hit with no
  Game Mode toggle; only an unthrottled fullscreen game tears, never the desktop
  (`hyprland/modules/decoration.lua`, `hyprland/modules/window_rules.lua`).

### Added
- **The screen-share source picker looks like the rest of the desktop.**
  `hyprland-preview-share-picker` shipped with no config, so the chooser opened
  as stock GTK in a themed session. It has one now, and a matugen template
  renders its stylesheet from the live palette on every wallpaper change: one
  click to pick, monitors labelled so the wrong screen is not a coin flip, and
  the restore-token checkbox hidden because `xdph.conf` answers it
  (`apps/hyprland-preview-share-picker/config.yaml`,
  `shell/matugen/templates/share-picker.css`, `hyprland/xdph.conf`).
- **Chromium pins the Wayland backend.** Chromium only recently made Wayland its
  default on Linux, and under Xwayland it never reaches the PipeWire capturer:
  `getDisplayMedia` falls back to X11 capture, which cannot see a native Wayland
  window, so screen sharing offers an empty or black source list. The flag makes
  that independent of the default (`apps/chromium-flags.conf`).
- **Setup verifies the chat backend before declaring success.** After enabling
  the daemon, `ryoku-rashin setup` runs `hermes acp --check` and reports whether
  the chat will actually start, so a working hermes CLI that still cannot run
  the ACP adapter surfaces at setup instead of as a silently dead chat later
  (`rashin/backend/setup.go`).
- **Super+Shift+A restarts audio.** Runs `ryoku-restart-audio` to recover sound
  when it does not come back after an update (`hyprland/modules/binds.lua`).

### Changed
- **Super+H hides the focused window, Super+Alt+H peeks the scratchpad.** Super+H
  was the peek and Super+Shift+H the stash, and the stash forced the window
  floating at 1280x800 centred, so a tiled window never came back to its slot.
  Super+H is now a round trip: it sends the focused window to the scratchpad, or
  brings it back when pressed on one already there, leaving geometry alone so
  hiding works as the minimise Hyprland does not have. Peeking moves to
  Super+Alt+H and Super+Shift+H is retired
  (`hyprland/modules/binds.lua`, `hyprland/scripts/ryoku-workspace`).
- **Super+T opens the Stash Features sidebar.** The floating Features page (the
  Stash board, with room for more panes) now has a keybind, growing from the
  right edge (`ryoku-shell menu stash`); Super+Shift+T (terminal) is unchanged
  (`hyprland/modules/binds.lua`).
- **Administrator passwords are asked for on a Ryoku island.** The shell daemon
  now registers as the PolicyKit1 authentication agent, so a privileged action
  prompts on the same island the keyring prompt uses instead of the stock Qt
  agent's grey dialog, and `hyprpolkitagent` is no longer started
  (`hyprland/modules/env.lua`, `hyprland/modules/autostart.lua`).
- **No raw "scale changed" popup when displays rescale.**
  `misc.disable_scale_notification` is set, so a login/hotplug/undock rescale by
  `ryoku-monitor` no longer flashes Hyprland's own toast over the shell's OSD
  (`hyprland/modules/misc.lua`).

### Changed
- **Super+W opens the wallpaper + theme menu bottom-centre; Super+C is freed.**
  The keybind now grows the frame's wallpaper menu from the bottom-centre edge
  (`ryoku-shell menu wallpaper`) instead of the old bottom-left menu, and the
  former Super+C binding is removed (`hyprland/modules/binds.lua`).
- **The desktop shell now has one Atoll bar and one bar-owned popup.** The
  ilyamiro and Ryoku Atoll looks remain, with their weather, media, connectivity,
  volume, battery and notification readouts left visible but inert. Power still
  opens from `Super+Escape` or the Atoll button; the old renderers, Washi and
  Atoll popup sets, sidebar entry paths, and their commands and settings are
  removed. Both sidebar bodies and their Stash/System logic remain mounted for
  the next UI, while the app launcher and frame stay in place. Ryoku Settings
  exposes only the live Atoll contract, and `ryoku doctor` prunes retired
  `shell.json` keys without touching preserved sidebar state.

### Fixed
- **Screen recording no longer leaves the desktop black or colour-inverted.**
  On GPUs whose DRM buffer modifiers Hyprland's screencopy path mishandles, the
  output was mis-restored when a capture ended, so after any capture (OBS,
  gpu-screen-recorder, hyprpicker, a screenshot) the whole screen could go black
  then negative until a reconfigure (Hyprland #11315, #8134). `AQ_NO_MODIFIERS=1`
  in the aquamarine backend sidesteps it; effective on the next login, and safe
  to drop once the upstream screencopy fix ships (`hyprland/modules/env.lua`).
- **The screen's colours are restored after a recording.** gpu-screen-recorder's
  KMS capture leaves Hyprland's colour management in CM space, washing the whole
  desktop out until the output is reconfigured (Hyprland #11284, #9286). Stopping
  a recording now waits for the recorder to release the capture, then reloads
  Hyprland to reset the screen (`hyprland/scripts/ryoku-cmd-screenrecord`).
- **Steam Big Picture and launched games render native and stay awake.** Steam is
  an XWayland app, so Big Picture and the client (class `steam`), launched games
  (`steam_app_*`) and `gamescope` inherited the desktop blur and shadow (per-frame
  GPU cost and a floating-card look) and the 0.94 inactive opacity, which turned a
  game translucent the moment focus left it; `hypridle` also had no fullscreen
  exception, so controller-only play dimmed at 5 min and locked at 10. A window
  rule now strips blur and shadow, forces them opaque, and inhibits idle while
  fullscreen (`hyprland/modules/window_rules.lua`).
- **Multi-monitor: switching to a workspace on another monitor no longer drags
  its windows to the focused monitor.** `scripts/ryoku-workspace` dropped the
  `workspace.move({ monitor = "current" })` that pulled the target workspace onto
  the active monitor before focusing it; it now just focuses the workspace, which
  already moves focus to the monitor that owns it (single-monitor unaffected).
- **`hyprpolkitagent.service` no longer fails out of the box.** `modules/autostart`
  imports `WAYLAND_DISPLAY` (and the session env) into the systemd user manager
  with `dbus-update-activation-environment --systemd` before starting the session,
  so hyprpolkitagent's `ConditionEnvironment=WAYLAND_DISPLAY` is satisfied, and
  `shell/systemd/user/hyprland-session.target` now Wants/After
  `graphical-session.target` so it is actually activated.
- **`ui/Seg` no longer clips long translated option labels.** Each segment sizes
  to its own text and the group wraps to a second line when its slot is narrow
  (Portuguese `Padrão|Plano|Adaptativo`, `arredondado|plano`); short sets unchanged.

### Added
- **The FN touchpad-lock key now toggles the touchpad.** New
  `scripts/ryoku-cmd-touchpad`, bound to `XF86TouchpadToggle`/`On`/`Off` in
  `modules/binds`, enables or disables every touchpad through `hyprctl eval`
  (`hl.device{ enabled }`, since the Lua config parser rejects the old
  `hyprctl keyword` device path), so the one hardware key that did nothing now
  works like the volume and brightness keys. The Ryoku Settings Keybinds legend
  reads the new binds live.
- **Brightness keys work on laptops AND desktop monitors.** New
  `scripts/ryoku-cmd-brightness`, bound to `XF86MonBrightnessUp`/`Down` in
  `modules/binds`, drives the laptop backlight through `brightnessctl` and every
  external monitor through `ddcutil` (DDC/CI) at once, so brightness is not
  laptop-only. `modules/autostart` also seeds the AI-translation key file
  (`ryoku-i18n ensure`).
- `hyprland` + `system/hardware/power`: **clamshell mode -- close the lid without
  sleeping when docked.** A new `modules/lid.lua` binds the laptop lid switch
  (`bindl switch:Lid Switch`) to `ryoku-clamshell lid`, which blanks the internal
  panel on close when an external display is attached and restores the layout on
  open; autostart launches the `ryoku-clamshell` daemon that keeps the machine
  awake on lid close while on AC power with an external display (macOS-style: both
  are required, else it suspends). The suspend policy and the logind drop-in live
  in `system/hardware/power/`.
- `hyprland` + `shell`: **`Super+Alt+D` opens the right (System) sidebar**, the
  mirror of `Super+D` for the left (Features) sidebar. The bind runs
  `ryoku-shell system`, a new IPC verb that toggles the System control centre;
  the Hub's keybind reference lists it automatically since it reads `binds.lua`
  (`hyprland/modules/binds.lua`, `shell/ipc/daemon.go`).
- `hyprland` + `hub`: **recordings are constant-framerate and crisp by default,
  and now configurable.** `ryoku-cmd-screenrecord` recorded variable-framerate
  with no quality flag, so clips felt like 30fps and imported as ~30 in editors.
  It now defaults to constant 60fps (`-fm cfr`) at `very_high`, and reads
  fps/quality/codec/encoder from `recording.json`, which the Hub's new Recording
  page writes. wf-recorder (the multi-GPU fallback) gained matching quality
  (VAAPI qp / x264 crf). Env vars still override everything.
- `hyprland` + `cli` + `shell`: **`user.lua` ships seeded with a header, not
  empty.** The hand-written override file Hyprland loads last is now seeded on
  install (like `keyboard.lua`) with a comment block spelling out the load
  order: Ryoku's base modules (replaced by updates), then `settings.lua`
  (generated from your Hub choices in `hypr.json`, rebuilt on Save), then
  `user.lua` (yours, never touched). `ryoku materialize` and `deploy.sh` seed
  it only when absent and never clobber it, so a hand-edit sticks across
  updates (`hyprland/user.lua`, `materialize.go`, `deploy.sh`).
- `rashin/backend`: **user.md works on a dev checkout now.** Without a packaged
  `/usr/share/ryoku/config`, Rashin gave up and treated all of `~/.config` as
  potentially user-owned. It now derives the baseline from the checkout
  `ryoku deploy` records (`~/.local/state/ryoku/repo`), diffing that checkout's
  `hyprland` tree, where the Ryoku-vs-user ownership actually lives, against
  `~/.config/hypr`; and even with no baseline at all it still names the
  always-user override files (`user.lua`, `monitors_user.lua`, ...), so an agent
  on a dev box can still tell Ryoku defaults from the user's own edits (`user.go`).
- `hyprland`: **Super+Esc opens the power menu** (`ryoku-shell power`) -- a
  vertical session strip (lock, logout, shutdown, restart, sleep). It is the
  delos bar's power access, since power leaves the island, but the bind works
  in every bar style.
- `rashin/backend` + `apps/fish` + `docs/rashin-terminal.md`: **Rashin in the
  terminal**, a third surface on the one brain (launcher `\`, dashboard, and
  now the command line). A new `rashin` command (the `ryoku-rashin` binary
  under a second name; argv0 routes a bare argument to a terminal ask) turns
  natural language into an answer plus a ready-to-run command plan and drops it
  on the fish prompt: `rashin take me to the fastfetch config`, `rashin scan
  Documents for pngs and move them into Pictures`. It answers on the daemon's
  fast lane (`POST /api/term`), the same direct chat-completions loop as the
  launcher, with a terminal persona, the terminal context (cwd, last command
  and its exit status), the read-only tools, and one action tool, `propose`,
  whose commands the daemon validates (binary on PATH, source paths exist) and
  tiers (read/write/system/danger via a deny-first Go classifier in
  `danger.go`). It never runs anything itself; the buffer is the confirmation
  and the tiers gate `--run`. Heavy asks escalate to the pre-warmed hermes
  session, and session-lane permission prompts are answered right in the
  terminal (`POST /api/perm`, answered exactly once even if the dashboard
  races). A `conf.d/rashin.fish` weave adds the interactive wrapper, an
  **Alt+R** binding that transmutes the current command line, a `fish_postexec`
  hook that reports proposed-vs-ran corrections, and the recipes loader. New
  **habits layer** (`habits.md`) mines this user's XDG directory names, modern
  tool substitutions (eza/zoxide/fd/rg/bat), and fish-history rhythms
  (secret-filtered, opt-out) into both ask lanes, so a command knows the folder
  is really `Pictures`. Repeated asks become saved **recipes** (`rr-<name>` fish
  abbreviations). Every surface reads and writes one ask history, so `\resume`,
  `rashin --resume`, and "continue in dashboard" see one conversation. New
  verbs: `term`, `term --report`; the `rashin` command also passes through
  `status`/`enable`/`disable`/`setup`/`index`.
- `shell/quickshell/overview` + `hyprland`: a new full-screen workspace overview
  (Super+Tab), a launcher-style expo that replaces the pill's workspace switcher.
  The compositor blurs the desktop (an `overview` layer rule) and a filmstrip
  shows the current desktop's workspaces as scaled mini-desktops with LIVE window
  previews (Quickshell `ScreencopyView` captures off-workspace toplevels, no
  compositor plugin). Click a workspace to switch, click a window to focus, drag
  a window between workspaces or up onto the desktop strip, hover a window for a
  ✕ that closes it, scroll/Tab to cycle the selection, Enter to commit, Esc to
  dismiss. Subtle Ryoku chrome: sharp corners, hard offset shadows, one
  vermillion accent, mono zero-padded workspace numerals with a small app-icon
  roster. A second level sits on top: DESKTOPS, each a block of ten workspace
  ids with its own 01..10 set; the top strip switches desktops (or Super+Alt+Tab
  cycles them) so you can keep separate sets of workspaces for different work.
  Empty gaps render as thin numbered slats so they stay visible and reachable
  without eating a full cell. `hyprland/scripts/ryoku-workspace` derives the
  current desktop from the active workspace and makes `Super+N` / `Super+Alt+N`
  desktop-relative (on desktop 2, `Super+3` focuses ws13, never ws3, and windows
  never jump desktops); the redundant Alt+Tab window switcher bind is removed.
- `rashin/backend` + `shell/quickshell/launcher`: the quick ask got real
  powers. The fast lane is now a bounded agent loop, not a one-shot: on a
  direct-provider connection it can call a small set of read-only Go-native
  tools (`system_query` for packages/updates/service/processes/disk/kernel/
  gpu/network, `read_file`, `list_dir`, `search_code` via prowl-agent, and
  `fetch_url`), up to four rounds, then answer, all still in a second or two.
  Tools are deliberately a safe Go set, not hermes's Python toolset; anything
  heavier (file or image generation, a real browser, a skill, system changes)
  replies `TOOLS_REQUIRED` and escalates to the pre-warmed session lane. Tool
  runs surface as cards in the dashboard and as the working label in the
  launcher. Every turn now runs on a background context, so **CONTINUE IN
  DASHBOARD** can open the live turn mid-flight and it keeps going after the
  launcher closes (proven: a SIGKILL'd CLI still completed the turn into
  hermes state.db and the ask history); **CANCEL** / Escape stops it via
  `/api/ask/cancel`. `\resume` lists recent asks from a persisted JSONL
  (`$XDG_STATE_HOME/ryoku/rashin-asks.jsonl`) and recalls a cached answer with
  its chips, no model call. New verbs: `ask --recent`, `ask --cancel`.
- `rashin/` + `hub/quickshell/RashinPage.qml` + `hyprland/modules/autostart`:
  **Ryoku Rashin**, an optional agent OS (off by default). `rashin/backend`
  (`ryoku-rashin`, one Go program) maintains a machine-generated markdown vault
  at `~/.local/share/ryoku/rashin/` (system, desktop, and package maps, fenced
  between `<!-- rashin:generated -->` markers so a reindex never clobbers user or
  agent notes), serves a hand-authored dashboard embedded under
  `rashin/backend/web/` on `127.0.0.1:3600` (localhost only), and bridges the
  Hermes agent over ACP into a web chat. A one-click `setup` installs and
  onboards Hermes, then wires reversible, marker-fenced vault pointers
  (`<!-- ryoku-rashin -->`) into every detected agent's global instructions
  (Claude Code, codex, opencode, omp, Hermes); an existing Hermes is never
  clobbered, and `serve` re-checks the wiring on start with any drift reported by
  `status`. The Hub gains a Rashin page in the Advanced group (enable toggle,
  one-click Hermes setup watched live, open-dashboard button), and `autostart`
  launches `ryoku-rashin serve --if-enabled`, which exits at once until enabled.
  See `docs/rashin.md`.
- `rashin/backend`: the dashboard grows from a glance into a **full local-agent
  utility** (v0.2.0), seven panels. Chat v2: image attach/paste/drag-drop (sent
  as ACP image blocks, downscaled client-side), clickable links, a `/` command
  legend fed live from Hermes's slash commands, a model picker with recents
  (switches over `session/set_model`), a session-history drawer that replays
  stored transcripts (`session/list` + `session/load`), a context-usage meter,
  token fade-in streaming, and a response-ready toast for backgrounded tabs.
  New **Memory** panel: provider detection (builtin or honcho/mem0/supermemory/
  hindsight and friends, plus Obsidian vault detection), a force-directed graph
  of the vault's notes and references, a 26-week activity heatmap, and Hermes
  session history read from `~/.hermes/state.db` (sqlite3, read-only). New
  **Skills** panel: all Hermes skills by category with origin counts (bundled /
  hub / agent-grown), live search, and the enabled toolbelt grouped into
  families. New **About** panel: what Rashin is, live facts, quick start, and a
  command crib pointing at `hermes -h`, `hermes gateway`, `hermes model`. The
  Overview gains a **code intelligence card**: when the user has `prowl-agent`
  installed and an indexed repo, the daemon surfaces doctor finding counts,
  files/symbols, and top hotspots (read-only exec, cached, degrades to hidden;
  prowl stays user-installed because upstream ships no license yet). New API:
  `/api/hermes/skills`, `/api/hermes/memory`, `/api/prowl`, `/api/prowl/search`,
  `/api/about`; the chat WebSocket learns models/commands/history/usage frames.
  Hermes onboarding detection now reads the mapping-form `model:` block, and
  session titles surface correctly.
- `shell/quickshell/launcher` + `rashin/backend`: a quick-ask answer is now a
  launch point, not a dead end. The answer text is selectable (mouse-copy any
  fragment), and `/api/ask` returns an `actions` array of entities the daemon
  detected in the answer and verified against the machine: real files, real
  directories, `http(s)` URLs, backtick commands whose first word is on
  `PATH`, and hex colors. The launcher renders each as a chip that does the
  obvious thing (file opens in nvim, folder in the file manager, URL in the
  browser, command and color copy with a live swatch), plus COPY for the whole
  answer and CONTINUE IN DASHBOARD. Chips walk with the arrow keys and fire
  with ENTER, typing re-asks, and copyables flash COPIED. Nonexistent paths
  and non-runnable spans are dropped so a chip never lies.
- `rashin/backend`: quick asks got fast. `/api/ask` now runs **two lanes**: a
  fabric-style fast lane makes ONE direct streaming chat-completions call on
  the same model connection hermes is configured with (openrouter, openai,
  groq, ollama, or any local endpoint; key read from `~/.hermes/.env`), a
  terse pattern prompt plus the vault maps as context, no Python spawn, no
  agent loop, answers in a second or two; the model replies `TOOLS_REQUIRED`
  when the ask genuinely needs tools, which escalates it to the session lane.
  OAuth backends (openai-codex) go straight to the session lane, and the
  daemon now **pre-warms the hermes session at boot**, cutting the first ask
  on this machine from ~19s to ~8s. The lane's connection is overridable in
  `rashin.json` (`quick.model` / `quick.baseUrl` / `quick.keyEnv`) for a
  cheaper or local quick-answer model. The ask CLI is now a thin pipe over
  `/api/ask`, both lanes land in the shared transcript, and consecutive
  duplicate working markers are deduped.
- `shell/quickshell/launcher` + `rashin/backend`: the launcher learns to ask
  the agent. A `\` prefix routes to Rashin: type `\why is my mic quiet?`,
  ENTER, and a pulsing strip names what hermes is doing (the running tool,
  thinking, writing) until one deliberately terse answer renders inline,
  image results (image_gen, screenshots) previewing as thumbnails. It rides
  a new `ryoku-rashin ask` one-shot that joins the daemon's shared session
  over the chat WebSocket with a quick-mode preamble only the model sees,
  and streams `@working`/`@perm`/`@answer` markers to stdout. Because it is
  the same session, the new CONTINUE IN DASHBOARD button opens the exact
  conversation, already on screen: the chat hub now keeps a per-session
  transcript (capped at 400 frames) and replays it to every joining client,
  which also means refreshing the dashboard no longer blanks the chat. A
  pending tool approval surfaces as APPROVE IN DASHBOARD. The `\` prefix
  joins the launcher help sheet.
- `rashin/systemd` + `rashin/backend` + `hyprland/modules/autostart`: the
  daemon now runs as a **systemd user unit** (`ryoku-rashin.service`) instead
  of riding the Hyprland session. `ryoku-rashin enable` does
  `systemctl --user enable --now`, so the dashboard is up at every login,
  survives compositor restarts, and restarts on crash; `enable --at-boot`
  adds `loginctl enable-linger` so it starts with the machine, before login.
  The unit runs `serve --if-enabled`, keeping `rashin.json` the single gate;
  without systemd everything falls back to the old detached spawn. The
  package ships the unit to `/usr/lib/systemd/user`, `deploy.sh` to
  `~/.config/systemd/user` (ExecStart rewritten to `~/.local/bin`), and the
  autostart.lua line is gone.
- `rashin/backend/web`: a **working strip** under the chat banner: while the
  agent acts, a pulsing dot names what it is doing live from the hermes
  stream (the running tool's title, `thinking`, `writing`, `waiting for your
  approval`), clearing at turn end and staying quiet during history replays.
  The hero and composer copy now call Rashin what it is, the needle (羅針),
  not the compass (羅針盤), whose 盤 is the dashboard itself.
- `rashin/backend` + `cli`: the vault gains two more generated layers, and the
  index follows the system. `ryoku-repo.md` is a **pre-indexed map of the Ryoku
  monorepo itself** (layout with file counts, key entry points, docs list),
  generated at package build by the `ryoku-rashin` PKGBUILD and shipped to
  `/usr/share/ryoku/rashin/ryoku-repo.md` (a dev `deploy.sh` writes the same
  snapshot to `~/.local/state/ryoku/rashin-repo.md`), so agents navigate the
  distro's source without a checkout. `user.md` is the **user-owned changes
  layer**: it hash-diffs the shipped base config against the live `~/.config`
  and lists override files, edited files, and removed files, reindexed
  separately by a 2-minute fingerprint watcher in the daemon whenever the
  user's config drifts. `ryoku update` now reindexes the vault after configs
  land on both channels (checkout and packaged), best effort, so the maps
  always describe the system that is actually running.
- `rashin/backend/web`: the dashboard scales to the viewport instead of hugging
  the left edge on wide screens: the content column centres (up to 1480px),
  and the hero, stat blocks, type, and chat art grow with `clamp()` between
  laptop and desktop sizes.
- `hub/quickshell/PerformancePage` + `shell/quickshell/{visualizer,pill,widgets}`:
  a **Performance Optimizations** section in Ryoku Settings, tweaks for modest
  hardware (most off by default; the visualiser freeze defaults on) and written to
  `~/.config/ryoku/performance.json` (watched live, no reload). Freeze the
  visualiser when no audio plays (it stops drawing at zero CPU and resumes on
  sound), unload the visualiser entirely when silent to free its ~190 MB of
  GPU/scene-graph memory (the daemon parks the process after a 30s silence grace
  and brings it back on audio, gated so a probe failure never drops the surface),
  freeze the pill bead's idle swirl, pause the desktop widgets' animation while
  windows cover them, and unload the widgets entirely once every screen is
  covered to free their ~250-400 MB (reloaded the moment an empty desktop
  returns). The visualiser also runs `cava` only while audio
  actually plays (default on), so a silent desktop no longer samples at 60fps for
  nothing.
- `hyprland/scripts/ryoku-cmd-game-mode` + `system/hardware/network` +
  `shell/quickshell/pill`: a one-click **Game Mode** in the Control Deck. A
  Utilities switch flips `Flags.gameMode`; the shell bridges it to the helper,
  which strips the compositor to its low-latency path through `hyprctl eval` (the
  Lua-parser path, since `hyprctl keyword` is rejected): no blur/shadow/rounding,
  animations off, `allow_tearing` with an immediate rule, and fullscreen-only VRR.
  It disables 802.11 power-save on every WiFi device (a pure latency win, with no
  reconnect and no throughput cap) via the privileged `ryoku-wifi-powersave`
  helper (`iw`), authorized passwordless by a polkit rule so the toggle stays one
  click, and pulls Do-Not-Disturb on. Fully reversible: `hyprctl reload` drops the
  eval overrides, the WiFi helper restores each device's prior power-save, and DND
  returns. Adds `iw` to the base set. Covered by `tests/game-mode.sh` and
  `tests/wifi-powersave.sh`.
- `shell/quickshell/plugins` + `hub/quickshell/PluginsPage`: a shell plugin
  system. A plugin ships a service + one adaptive `content/Widget.qml` (glyph /
  compact / full); the shell owns each host's layer, shape, size, and motion, so
  plugins read as native. v1 hosts: frame popout (fused into the frame blob in the
  pill) and desktop widget (the wallpaper layer). Discovery is
  `plugins/discover.sh` (catalogue + `~/.config/ryoku/plugins.json`), the
  signature kit is the `Ryoku.PluginKit` QML module (`plugins/kit`), placement is
  edited in Ryoku Settings -> Plugins and persisted by `ryoku-plugins-place`, and
  `ryoku-shell plugin <id>` toggles a frame popout. The legacy `wallhaven` plugin
  is reworked as the worked example. See `docs/plugins.md`.
- `hub/quickshell/GpuPage` + `hub/backend/gpu`: a System -> GPU page with a
  hardware-capability engine. Choose the graphics mode (Hybrid, Performance,
  Passthrough) and set up the optional GPU-passthrough stack, gated by checks (CPU
  virt, IOMMU, isolated dGPU group, which GPU drives the display, RAM, the virt
  stack) so it refuses anything unsafe. Dynamic vfio bind/unbind via a libvirt
  hook (no boot-time binding), kvmfr Looking Glass, swtpm + Secure Boot. The
  one-time "Enable passthrough" is reversible. Running virtual machines lives in
  the `apps/ryovm` app (quickemu/quickget), not the hub.
- `hyprland/binds` + `hyprland/resize`: working window resize. `Super + Ctrl +
  arrows` resize the active window directly (repeating); the `Super + R` resize
  mode also accepts `hjkl`, exits on `Super + R`, `Esc`, or `Return`, and shows a
  toast on entry, since entering a submap is otherwise silent.
- `hyprland/binds` + `hyprland/animations`: a scratchpad you can fill. `Super +
  Shift + H` stashes the active window into `special:scratch` as a tidy 1280x800
  centred float, `Super + H` toggles it, and a new `specialWorkspace` slide-and-fade
  drops it in.
- `shell/quickshell/sidebar` QuickStrip: a Night Light quick-toggle joins Do Not
  Disturb and Keep Awake, reading and toggling `hyprsunset` (the warm screen) live
  via the night-light script, so it stays in sync with the `Super + U` utility and
  the hub's Comfort tab.
- `hyprland/binds`: `Super + K` opens the keybind reference, the hub's live
  shortcut legend read from `binds.lua`, so the full shortcut list is one key
  away.
- `shell/quickshell/pill/Bar.qml` + `hub` (Shell -> Bar): an opt-in top bar, an
  alternative to the morphing pill island. The pill draws it inside the frame's
  own blob field, so the frame's top simply thickens into the bar (no separate
  program, no seam): the brand mark and workspace dots on the left, the clock in
  the centre (it opens the calendar), now-playing, the system tray and power on
  the right. Ryoku Settings -> Shell -> Bar turns it on, which hides the resting
  pill island so the two never overlap; surfaces still open from their keybinds
  and melt in and out of the bar centre. Default off.
- `shell/quickshell/switcher` + `hyprland/binds`: an Alt-Tab window switcher. A
  full-screen overlay (its own `qs -c switcher` instance, like ryoshot) lists the
  open windows in most-recently-used order as app-icon + title cards, opens with
  the previous window selected (hold Alt, tap Tab, release to switch back), and
  Tab or the arrows cycle, Enter or a click activates, Escape cancels. Bound to
  `Alt + Tab`; the frame and pill identity are untouched (separate overlay layer).
- `hyprland/themes/{washi,soft_color,mountains,crt,drift}`: five more theme rices.
  `washi` (warm vermilion on dark paper, clinical motion), `soft_color` (dreamy
  peach pastel on slate-blue), `mountains` (desaturated earth tones) and `crt`
  (cyan phosphor glow on near-black) ship fixed palettes; `drift` is a slow, airy,
  breathing look-only rice that follows the wallpaper. All opt-in from Ryoku
  Settings; the frame and island keep the Ryoku identity.
- `hyprland/themes/compact` and `hyprland/themes/glass`: two look-only rices
  (colours still follow the wallpaper). `compact` is dense and tight (small gaps,
  light rounding, no shadow, a soft pop); `glass` is heavy frosted blur with
  translucent windows and a gentle springy pop. Both opt-in from Ryoku Settings and
  keep the frame and island identity.
- `hyprland/themes/cassette`: a new flat, sharp, sepia theme rice (no blur or
  shadow, `rounding 0`, tight gaps) in a muted YoRHa/NieR palette, filling the gap
  left by the rounded, glassy default set. Opt-in from Ryoku Settings; the frame
  and island keep the Ryoku identity, and its fixed palette applies when colours
  are set to the theme rather than the wallpaper.
- `hyprland/monitors_user.lua.example`: a hand-written manual monitor override.
  `hyprland.lua` now `require`s `monitors_user` (a `pcall`, after the generated
  `monitors.lua`), so `~/.config/hypr/monitors_user.lua` wins and lets you force a
  mode, a custom modeline, position, scale, rotation, or mirror for a panel whose
  EDID is wrong (for example a fake/generic EDID). It is never shipped or
  overwritten, and `ryoku-monitor` leaves any output named in it alone.
- `hyprland/themes/`: full-system theme "rices", one folder each, with the look
  (`theme.json`), real Hyprland Lua (`init.lua`: motion and decoration finish), and
  a 16-colour `colors.json` for fixed palettes. Ships **default** (the shipped
  look), Tokyo Night, Aqua, Catppuccin, Gruvbox, Nord, and Rosé Pine. The active
  theme's `init.lua` is loaded by `hyprland.lua` (as `theme`) before `settings.lua`.
  Ryoku Settings applies them and toggles whether colours follow the wallpaper.
- `hyprland/hyprland.lua`: loads a generated `settings.lua` after the base modules
  and before `user.lua`, the override file Ryoku Settings writes. Missing by
  default (a `pcall` no-op); the hub creates it on first use. `window_rules` and
  the `Super + ,` legend now read "Ryoku Settings".
- `hyprland/hyprland.lua`: loads the runtime-generated drop-ins `gpu.lua` and
  `monitors.lua` with `pcall` (like `settings`, `theme`, and `user` already are),
  so a half-written or corrupt one -- which a crash or a GPU reset can leave behind,
  since those fire monitor events that rewrite `monitors.lua` -- falls back to
  Hyprland's defaults instead of dropping the whole config into emergency mode.
  `ryoku doctor` repairs the file and autoscale regenerates it on the next login.
- `hyprland/scripts/ryoku-cmd-nightlight`: `status`, `on [temp]`, and `off`
  subcommands (with the saved temperature persisted) so Ryoku Settings' Comfort
  tab can show and set the night light; the bare call still toggles for Super+U.
- `hyprland/modules/binds`: `Super + P` toggles the displays between mirror
  (duplicate) and extend, via `ryoku-monitor toggle`.
- `hyprland/modules/binds`: `Super + Tab` opens the pill's workspace switcher
  overview (`ryoku-shell workspaces`) for moving windows between workspaces.
- `hyprland/modules/binds`: `Super + M` toggles the desktop audio visualiser
  (`ryoku-shell visualizer`).
- `hyprland/modules/binds`: `Super + Shift + M` raises the visualiser over the
  windows on demand (`ryoku-shell visualizer-overlay`), flipping back to the desktop.
- `hyprland/modules/decoration`: a touch more room around tiled windows
  (`gaps_out` 24 -> 26, `gaps_in` 7 -> 8) for a clear frame-to-window vs
  window-to-window gap hierarchy that reads with the frame's new contact shadow.
- `hyprland/`: the Hyprland config in Lua, modular (entrypoint plus modules for
  input, decoration, animations, binds, window rules, ryoshot, and autostart)
  with hardware-managed gpu/keyboard/monitors. Launches the Ryoku shell and the
  laptop-only idle policy.
- `lockscreen/`: the vendored qylock clockwork theme, its installer, and the SDDM
  setup.
- `apps/`: kitty, fastfetch (with the branded wrapper), fish (greeting off),
  starship, and nautilus notes.
- `assets/`: the 力 brand logo and icons, plus the shipped wallpaper collection
  (`wallpapers/`) that installs to `~/Pictures/Wallpapers`; `ryoku-shell` picks a
  random one on first login.
- `shell/`: the Quickshell desktop UI (pill, sidebar, ryoshot),
  the wallust palette generation, the qt/kde theme, the user session target, and
  the `ryoku-shell` Go control-plane daemon (`ipc/`).
- `hyprland/` autostart and `shell/ipc`: apply wallust colors to
  OpenRGB-compatible keyboards and lighting devices through `ryoku-leds`.
- `hyprland/` autostart: set GTK apps to dark through `gsettings`
  (`color-scheme` prefer-dark, `gtk-theme` Adwaita-dark), so nautilus and other
  GTK apps match the dark Qt and kitty theme.
- `hyprland/` binds and autostart: tap ``Super+` `` to start Handy speech-to-text
  and the live mic wave, tap again to stop (`ryoku-shell voice`); autostart Handy hidden and
  tray-less (it is keybind-driven and configured from app search) when the
  optional `handy` binary is installed.
- `hyprland/` autostart: normalize the default microphone to unity gain on login
  through `ryoku-mic`, so an over-amplified codec does not clip Handy's recording
  or peg the voice wave.

### Changed
- `shell/quickshell/visualizer` + `hub/quickshell`: **the `line` visualiser
  style is now an oscilloscope** that draws the actual playback waveform, not a
  spectrum with sharp points. A new `Waveform` singleton captures the default
  sink's monitor (PipeWire-native, downsampled by `wavecap.py`) and the line
  traces it live: a glowing filament on a baseline that flatlines in silence and
  moves with the music, wearing the heart-monitor look as skin over a real music
  visualiser. Capture runs only while the style is selected and tears down with
  the surface. The Shell settings preview mirrors it and the style picker labels
  it "Monitor" (the `line` key is unchanged) (`Visualizer.qml`, `Waveform.qml`,
  `wavecap.py`, `VizPreview.qml`, `ShellSettingsPage.qml`).
- `shell/quickshell/{pill,widgets}`: the always-on shell layers render on demand
  (the basic Qt loop) instead of the threaded loop, which on NVIDIA spun the
  render thread every vsync whenever a live MultiEffect (card shadows, the bead
  glow) sat in the scene and burned idle CPU on a static desktop. On-demand
  rendering idles properly, roughly halving each layer's idle cost, with no
  visual change. Album art in the music island and OSD is now decoded at the
  thumbnail size instead of at full resolution.
- `hyprland/binds`: `Super + A` floats the active window at a fixed 1000x660,
  centred (press again tiles it back), instead of floating it at its current size.
- `hyprland/modules/binds`: reworked the keymap. `Super + arrow` keys move focus
  between windows and `Super + Shift + arrow` move the active window; `Super + 1..0`
  still focus workspaces but moving the active window there is now `Super + Alt + 1..0`.
  `Super + A` floats and centres the active window as a toggle (press again to tile
  it back), replacing the old `Super + A` / `Super + Shift + A` float/tile pair.
  `Super + R` enters a resize mode (`hyprland/modules/resize`, a submap where the
  arrows resize and Escape exits) and `Super + H` toggles the scratchpad (special
  workspace). `Super + arrow` no longer cycles workspaces (the number row does).
- `hyprland/modules/binds`: `Super + 1..0` now shows that workspace on the monitor
  under the cursor (the workspace is pulled to the focused monitor first) instead
  of yanking focus to wherever the workspace lived, so the number keys drive
  whichever screen the mouse is on rather than always the laptop. `Super + Alt +
  1..0` sends the active window to that workspace, on that screen.
- Tuned Hyprland window decoration and motion for the Ryoku shell: stronger
  shadows, softer translucency, wider breathing room, and branded open/close
  curves.
- Consolidated everything under a single `ryoku/` tree: the former top-level
  `shell/` now lives at `ryoku/shell/`, its modular Hyprland config replaced the
  old flat `ryoku/hyprland` (one Hyprland config now), and the duplicate
  `shell/fish` (with its non-brand greeting) was dropped for `ryoku/apps/fish`.

### Fixed
- `hyprland/modules/autostart.lua`: the first-run welcome walkthrough is no longer
  suppressed forever when it fails to launch. The launch chained `qs -c welcome`,
  `mkdir`, and `touch welcome-seen` with `;`, so the seen-flag was written even
  when `qs` exited without showing the tour, and the walkthrough never returned. It
  now gates the flag on `qs` succeeding (`&&`), so a first-boot launch failure
  retries on the next login instead of marking the tour seen.
- `hyprland/modules/autostart.lua`: booting a snapshot from the Limine menu now
  actually offers the one-click restore. limine-snapper-sync ships its restore
  prompt as an XDG autostart entry, but Hyprland runs no XDG autostart manager,
  so under Ryoku's own session the notification never fired and a user booted
  into a snapshot got no cue to restore it. Autostart now runs
  `limine-snapper-restore --notify` (command-gated); on a normal boot it
  detects no snapshot and exits silently.
- `hyprland/modules/autostart.lua`: the welcome tour's double-fire guard now
  actually guards. `flock -o` closes the lock fd before exec, releasing the
  lock the instant `qs -c welcome` starts, so two racing autostart fires could
  both open the tour; and the lock file lived at a fixed `/tmp` path one user
  owns, so on a multi-user box the second user's flock failed to open and their
  first-login tour was silently skipped (the seen-marker still got written).
  The `-o` is gone and the lock lives under `$XDG_RUNTIME_DIR`.
- `hyprland/modules/env.lua` + `shell/qt6ct`: app logos in the launcher's
  all-apps grid resolve again. `QT_QPA_PLATFORMTHEME` was `kde`, but the `kde`
  platform-theme plugin comes only from `plasma-integration` (a 122-package
  Plasma pull), which this Plasma-free desktop never installed, so Qt resolved
  no icon theme at all and searched hicolor only. Named freedesktop icons like
  the Avahi entries' `network-wired` fell back to the broken-image placeholder.
  Switched back to the `qt6ct` platform theme (its plugin ships with `qt6ct`,
  already in the base set) and ship `qt6ct/qt6ct.conf` with `icon_theme=`
  `Papirus-Dark` and the Fusion style. Removed the now-dead `kde/kdeglobals`:
  its KDE ColorScheme was equally inert without the missing plugin, and keeping
  its `[Icons]` line would duplicate the icon-theme source.
- `cli/doctor`: the limine-snapper-sync checks only run when limine is
  actually installed. A GRUB box with a healthy snapper setup (converted
  CachyOS installs, typically) was flagged inconsistent forever, with a fix
  suggestion that could not work there.
- `hyprland/hyprland.lua`: no more "Your config has errors" flash on a fresh
  first boot. Hyprland reports even a `pcall`'d `require()` of a missing module
  in the config-error overlay, and six optional drop-ins (`monitors_user`,
  `theme`, `settings`, `modules.private`, `ghosttype`, `user`) legitimately do
  not exist on a new home. The loader now probes with `package.searchpath`
  first and only requires files that are actually there.
- `shell/quickshell/{visualizer,pill,widgets}` + `shell/ipc`: a memory leak on this
  Qt 6.11 / NVIDIA stack where any continuously-animating or continuously-composited
  element grows RSS without bound (a plain rotating rectangle leaks ~0.9 MB/min and
  never settles; a frozen visualiser stays flat). Fixes: the visualiser idle wave
  freezes when silent by default (resuming instantly on audio); the pill bead is
  removed entirely (its 12fps idle-swirl Canvas was the worst pill offender) and the
  WaveMeter held static; and the desktop widgets, which ride the wallpaper and are
  invisible whenever a window covers every screen, are unloaded there by the daemon
  and reloaded the instant an empty desktop returns (they otherwise kept rendering
  and ran a day's uptime past 1.5 GB). Active content animates as before.
- `shell/quickshell/pill`: opening a pill surface that grabs the keyboard (the
  control deck, launcher, clipboard, calendar, ...) and closing it left the
  previously focused window un-typeable. The pill is one always-mapped layer that
  toggles its keyboard focus Exclusive while a surface is open and None when it
  closes; Hyprland leaves the keyboard on the released layer (the window still
  reports as active, so a plain refocus is a no-op) until a real focus change.
  On close the shell now hands focus back to the active window by bouncing off the
  next window and refocusing it. A launched app still wins (it maps and grabs
  focus via focus_on_activate after the handback). Verified live with synthetic
  keystrokes: typing is dead after closing the deck without the fix and restored
  with it, and the launcher still focuses the app it launched.
- `hub/backend/qemu`: a windowed VM booted to the UEFI shell / PXE ("failed to
  load Boot0002 ... Not Found", then "Start PXE over IPv4") instead of the
  installer ISO, even with the ISO correctly attached. OVMF boots by its own
  persistent NVRAM order (`*_VARS.fd`) and ignores `-boot order=dc`; that NVRAM
  goes stale (a boot entry pointing at a device that no longer exists) and falls
  through to PXE. Attach the disk and ISO as explicit devices with `bootindex`
  (ISO 1, disk 2), which QEMU passes via fw_cfg and OVMF honours over its saved
  order, so the ISO boots deterministically. Fixes it on already-affected VMs
  without wiping their NVRAM. Verified live: reproduced the PXE screen with the
  real config, then booted Void to the desktop with the same stale NVRAM.
- `hyprland/modules/misc`: a newly opened or re-raised window (notably Discord and
  Vivaldi) sometimes came up un-typeable until you moved it to another monitor or
  reopened it. The modular config refactor had silently dropped the `misc`/`xwayland`
  block, reverting `focus_on_activate` to its `false` default, so an app's
  xdg-activation focus request was ignored whenever the window landed off the focused
  workspace/monitor; `follow_mouse = 2` then removed the pointer fallback that had
  been masking it. Restores the block as a dedicated module: `focus_on_activate = true`,
  `xwayland.force_zero_scaling` (crisp Chromium/Electron on HiDPI/fractional displays),
  and `disable_hyprland_logo`.
- `hyprland/scripts/ryoku-cmd-mirror`: the webcam mirror (力 deck -> Tools) ran at
  5-15 fps and stuttered because mpv negotiated the camera's raw YUYV stream, which
  is USB-bandwidth capped (about 5 fps at 1080p, 10 at 720p). It now asks the camera
  for MJPEG when it offers it (probed with ffmpeg, falling back to the default so a
  raw-only camera still works), restoring the full 30 fps, and renders explicitly
  through `--vo=gpu-next` (libplacebo) so a stray software `vo` in mpv.conf can't
  bog it down.
- Hyprland: DPI autoscale now re-runs when a display is hotplugged, not only at
  login, so an external monitor plugged in mid-session is positioned and scaled
  immediately instead of coming up at 1x until the next relogin.
- Hyprland: a monitor connected mid-session now gets the current wallpaper painted
  onto it automatically. The hotplug handler repaints every output (via `ryoku-shell
  wallpaper refresh`) once autoscale has settled the new mode, so the screen no
  longer comes up on a black background until the next manual wallpaper change.
- Hyprland: the NVIDIA VA-API/GLX env hints (`LIBVA_DRIVER_NAME`,
  `__GLX_VENDOR_LIBRARY_NAME`, the `__GL_*` toggles) were set on every machine,
  breaking hardware video decode and Xwayland GL on AMD and Intel. They now
  apply only when the NVIDIA driver is present; mesa auto-detects elsewhere.
- Hyprland: a window stranded in maximize when a Chromium/Electron app leaves
  page fullscreen (a spurious mode-1 event on exit) is reset to normal, so the
  window returns to its original size instead of staying expanded.
- `hub`: the Shell settings subtabs (Frame, Island, Bar, Visualizer) centred their
  content in the panel, so short tabs dropped their controls into the middle with a
  large empty gap above. The tab content top-aligns now.
- `hub/backend` (`gpu caps`) + `hub/quickshell/GpuPage`: the System -> GPU page
  could sit on "Detecting..." forever. `ryoku-hub gpu caps` shelled out to the
  GPU detector with no time limit, so a wedged host probe (a runtime-suspended
  or stuck `nvidia-smi`) hung the whole call and the page never resolved. The
  caps call now runs under a hard timeout (its own process group, killed on
  expiry so an orphaned probe can't hold the pipe open), and the page surfaces a
  failed or timed-out probe with a Retry instead of an endless spinner.
- `shell/deploy.sh` + `hub/backend` (`gpu caps`): a dev deploy installed
  `ryoku-hub` but not `ryoku-gpu`/`ryoku-gpu-detect`, so the hub called a stale
  detector that predates `detect --json` and prints its table; the parser then
  failed with a cryptic "invalid character 'C'". Deploy now installs the GPU
  detector alongside the hub (fixing both the GPU page and autostart pinning),
  and `gpu caps` reports an out-of-date `ryoku-gpu` plainly instead of leaking
  the parser error. Retry clears the prior failure so it visibly re-checks.
- `hub/backend` (`vm setup`, `gpu apply`) + `hub/quickshell/GpuPage`: "Install
  QEMU" reported success even when pacman failed (the install ran best-effort and
  the "Done" line printed unconditionally), so the page kept asking to install.
  The install now propagates pacman's exit status, verifies `qemu-system-x86_64`
  is actually present, and on failure points at `ryoku update`; the passthrough
  enable aborts the same way instead of writing config over a failed install. The
  Machine tab also re-checks on its own while the install runs, so it advances
  without a manual Recheck.
- `hub/backend` (`vm`, qemu) + `hub/quickshell/GpuPage`: a windowed VM failed to
  start on some machines and left a window that could not be closed. It rendered
  through host GL (`virtio-vga-gl` + `gtk,gl=on`), whose EGL path is brittle under
  Wayland and depends on the host GPU, so it opened on one GPU and failed to start
  on another. It now uses 2D `virtio-vga` in a plain GTK window: no host GL,
  identical on AMD, NVIDIA and Intel, and a picture for every guest (installers
  included). OVMF firmware is detected across edk2 layouts instead of one
  hardcoded path, and a launch that dies is reported with QEMU's log tail (and the
  VM is detected as running, so Stop works) instead of a silent "failed to start".
- `hyprland/modules/input`: a newly opened window was not active until the mouse
  moved onto it. `follow_mouse = 1` refocuses whatever the pointer sits over, so a
  window spawned away from the cursor lost focus at once. `follow_mouse = 2`
  detaches keyboard focus from the pointer: a new window keeps focus, and a click
  moves it.
- `hyprland/hyprland.lua`: a broken optional drop-in says so in the log. An
  `optional()` module that exists but fails to load (a syntax error in a
  hand-edited `user.lua` or `monitors_user.lua`) was swallowed whole, so the
  user's edits silently did nothing; the pcall error is now printed, naming
  the module and the parse failure, while the config still degrades instead
  of hitting the emergency overlay.
