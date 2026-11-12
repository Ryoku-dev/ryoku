# The launcher

The launcher is the Ryoku command palette: a standalone, centered overlay
summoned with `Super + Space`. It searches and launches apps, calculates, runs
system actions, manages clipboard and snippets, switches windows, searches the
web, finds files, installs packages, controls any media player, and searches and
plays free music from YouTube Music. It is a full rebuild of the old pill
app-list, dropped from the pill so it has the room a Raycast/Alfred-class palette
needs.

It lives in `ryoku/shell/quickshell/launcher/`, a Quickshell component supervised
by the `ryoku-shell` daemon (a peer of `pill`/`ryoshot`), kept warm so it opens
instantly and toggles with `ryoku-shell launcher`.

## Launcher variants

Hero is the default launcher. Ryoku Settings -> App Launcher offers the three
shipped styles, the launchers that installed subshells and bar styles provide,
and any launcher installed from RyoStore (see
[Store launchers](#store-launchers)), and saves the choice as `variant` in
`~/.config/ryoku/launcher.json`:

- **Main** is a compact command palette with the centered RestDashboard image
  card and the shared provider, result, and action contracts.
- **Hero** is the full image-backed Shutter launcher with its dashboard, local
  frost, result drawer, action shelf, and complete provider set.
- **OkShell** is the lean applications-only list: search, hidden-app toggle, and
  sliding row selection.

An item contributes a launcher with `[[provides.launcher]]` in its `item.toml`
(see `docs/subshells.md`). The shipped Kairos subshell provides **Kairos**, the
island launcher: the pill grows out of Kairos's resting island at the top of the
screen into an Android-style app drawer, with a search field, the five apps you
open most and every app A to Z in a grid. Store items provide
others, such as Spotlight from the Shima bar style and Python from the Python
subshell. The page tags them `FROM <ITEM>`. Selecting one puts its item in use,
so a packed item is unpacked first, and the default launcher previews and runs
until it is ready.

The resident `shell.qml` selector loads the saved catalog entry without changing
the stable `ryoku-shell launcher` command or socket. An unknown saved ID resolves
to Hero. If a selected QML entrypoint cannot load, the selector tries OkShell
once. Switching an open launcher closes it before replacing the component.

## Structure and contracts

- `catalog.json` is the ordered registry of the shipped variants. It names the
  default, load-error fallback, entrypoint, preview, and capabilities for every
  launcher.
- `shell.qml` owns only selection, switching, fallback, and the stable command
  surface.
- `shared/` owns the providers, singletons, JavaScript libraries, result/action
  contracts, and art reused by more than one variant.
- `variants/<id>/Main.qml` is the runtime root. It implements `show(mon)`,
  `hide()`, `toggle(mon)`, `stateDump()`, and the read-only `shown` property.
  An item's launcher contribution keeps the same contract.
- `variants/<id>/Preview.qml` is the visual-only 720 x 250 Ryoku Settings
  preview. It accepts `settings` and emits `editRequested(key, value)` when the
  preview supports editing.

Preview controls are capability-gated by the catalog, so adding a variant does
not add irrelevant settings to other designs.

## Adding a launcher variant

1. Create `variants/<id>/` with one component per QML file.
2. Implement the runtime contract in `variants/<id>/Main.qml`.
3. Add a visual-only `variants/<id>/Preview.qml` using shared tokens and art.
4. Add one safe `variants/<id>/...qml` row to `catalog.json`, including only the
   capabilities the preview actually supports.
5. Select it in Ryoku Settings and smoke-test show, hide, search, execution, and
   switching from an already-open launcher.
6. Add focused unit and live scenarios under `launcher/shared/lib/` and
   `launcher/qa/`; run the launcher QA suite and review its screenshot.

## Store launchers

Community launchers come from **RyoStore -> App launchers**, the `launchers/`
catalogue of the ryostore repository (its `launchers/AUTHORING.md` is the
author's guide). A store launcher keeps the variant contract above: a folder
with a `Main.qml` runtime root and, optionally, a `Preview.qml`.

- Installing writes the receipt-owned folder, publishes
  `~/.local/state/ryoku/store/launchers.json` (rows of `id`, `version`,
  `preview`, `view`) with a content-addressed copy under
  `launcher-views/<id>/<digest>/`, and bumps the Store `revision.json`. It never
  changes `variant`.
- `shared/Singletons/LauncherProducts.qml` reads that index and follows the
  revision. `shared/lib/catalog.js` `withStore()` appends its rows to the shipped
  catalog as entries whose `url` is the view's file URL. A shipped id always wins,
  and a row whose view is not `launcher-views/<its id>/<digest>` is dropped.
- The per-screen host (`modules/launcher/Launcher.qml`) waits for the index
  before resolving the saved id, so a saved store launcher opens on the first
  Super+Space after login. An update moves the view, so the host swaps the new
  copy in the way it switches variants.
- Ryoku Settings -> App Launcher lists installed store launchers beside the
  shipped ones with a STORE tag, previews each with its own `Preview.qml` (or the
  store art when it has none), and offers UPDATE and BROWSE RYOSTORE. Picking and
  saving one is the same as picking a shipped variant.
- A store launcher opts into Settings controls with a `settings.json` beside its
  `Main.qml`, which the install carries like any other file of the folder.
  `capabilities` names what it honours; `sections` lists at most 64 groups it
  files apps under, for the `sections` capability. Section ids use lowercase
  letters, digits and dashes, and labels are non-empty text of at most 80
  characters. Settings reads the descriptor as data with
  `hub/quickshell/LauncherSettings.js`; it ignores unknown names and rejects
  malformed or oversized files, so content there is never executed.

  | Capability | Settings card | `launcher.json` keys |
  | --- | --- | --- |
  | `background` | Backdrop frost | `bgBlur` |
  | `overlay` | Overlay | `overlayOpacity` (0.3 to 1) |
  | `hiddenApps` | Hidden apps | `hiddenApps` (desktop entry ids) |
  | `grid` | Grid | `columns`, `matWidth` (px) |
  | `sections` | Sections | `sectionOrder`, `hiddenSections` (section ids) |
  | `pinnedApps` | Favorites | `pinnedApps` (desktop entry ids) |

  The launcher reads these keys itself from `launcher.json`, with
  `Quickshell.Io`'s `FileView` and `JsonAdapter`, and must fall back to the
  defaults when one is absent. It never writes the file: Settings owns it, and
  a write from here would drop the keys the launcher does not know.
  The shipped Hero and Main honour `hiddenApps` through `shared/providers/apps`.
- Removing the launcher in use resets `variant` to `""`, the default, inside the
  same journaled Store transaction. The shipped ids (`main`, `hero`, `okshell`)
  are reserved: the Store refuses to install, list, or remove them, and the
  catalogue validator rejects them.

## Hero anatomy

- `variants/hero/Launcher.qml` is the card body: the search row over the rest
  dashboard, the all-apps grid (`Ctrl+A`), action-mode tabs (`/` prefix), or the
  ranked result list.
- `variants/hero/SearchRow.qml` is the 力 glyph, query field, result counter,
  Google Lens, and music-recognition controls.
- `variants/hero/ResultList.qml`, `ResultGrid.qml`, `NowPlaying.qml`, and
  `RestDashboard.qml` are the main views. `ActionPanel.qml` is the per-item verb
  sheet (`Ctrl+K`), and `CategoryTabs.qml` is the action-mode tab bar.

## Providers

Each capability is a provider: its own folder under `shared/providers/<name>/`,
a single QML component implementing the `Provider` contract (`providerId`,
optional `prefix`, `query(text) -> rows`). The `Dispatcher` singleton routes a
prefixed query to one provider and fans an unprefixed one across the default
set, merged by score. Adding a provider is a folder plus one line in
`shared/providers/Providers.qml`; the dispatcher discovers it by registration,
never by an edit to the routing.

| Provider | Prefix | What it does |
|---|---|---|
| apps | (default), `>` | launch desktop apps, fuzzy + launch-frequency ranked |
| calc | `=`, leading digit | qalc: math, units, currency |
| actions | `/` | system actions (lock, wallpaper, screenshot, night light, media keys, settings) in category tabs |
| clipboard | `;` | Ryoku clipboard history, copy or delete |
| windows | (default) | switch Hyprland windows |
| web | `?` | web search with `!bang` site shortcuts, plus an inline DuckDuckGo instant answer |
| files | (default, 3+ chars) | fd file search, open or reveal |
| snippets | (default) | text expander (`{date}`/`{clipboard}`/`{selection}`/`{cursor}`) + quicklinks (`{query}`) |
| packages | `install`/`remove`/`search` | GPK across every package manager |
| mpris | (default, media words) | now-playing + transport for any player |
| script | per-script keyword | run rofi-script / dmenu scripts |
| rashin ask | `\` | ask Rashin a read-only question about this machine through its free-model router. The answer renders as selectable text with actions for files, folders, URLs and proposed commands. Work that needs changes can move to an Agent session in the Rashin app. Needs Rashin on; see `rashin.md` |

Ranking and protocol logic live as testable JavaScript in `shared/lib/` and each
provider's folder (`fuzzy.js`, `dispatch.js`, `rofiscript.js`, `wave.js`,
per-provider parsers), each with a `.test.mjs` run by `node`.

## Media

The launcher surfaces whatever is already playing so you can control it without
leaving the palette.

- **Any player** (mpris): the now-playing row controls whatever is playing
  (Spotify, a browser tab, an app) with play/pause/next/prev. It appears when the
  query mentions a media word or matches the current track.
- The now-playing card on the rest screen shows album art, title/artist, elapsed
  and total time, and the signature wavy seekbar (a sine that animates only while
  playing); the fill advances off a 500ms MPRIS position poll. A live cava wave
  sweeps behind it while a track plays, gated so the analyser runs only while the
  launcher is open and something is playing. For a player with no cover (some
  browsers) it fetches one from the keyless iTunes Search API by artist and title
  (noise-stripped for a better hit).

## Extending it

- **Scripts** (no code): drop a `{ keyword, name, exec }` entry in
  `~/.config/ryoku/launcher-scripts.json`. The script speaks the rofi-script /
  fuzzel dmenu protocol (newline rows, `\0key\x1fvalue` directives,
  `ROFI_RETV`/`ROFI_INFO` env), so existing rofi/fuzzel scripts work unchanged.
- **Snippets / quicklinks**: `~/.config/ryoku/launcher-snippets.json` and
  `launcher-quicklinks.json`.
- **A native provider**: add `shared/providers/<name>/<Name>.qml` (a `Provider`),
  register it in `shared/providers/Providers.qml`. Shell-backed providers run
  their process async with a debounce and call `Dispatcher.notifyAsync()` when
  results land.

## Performance

The launcher is one resident process. While open it has no RAM ceiling (album-art
decode, mpv, blur). At rest it sheds the heavy work: no media process runs until a
track is played, the clipboard is read on demand (not watched), and file/package
searches only fork past a length gate. Keystroke-to-result never animates; the
motion budget is spent on the window show/hide and the action-panel open
(`shared/Singletons/Motion.qml`).

## Theming

Colors, spacing, and motion are tokens in
`shared/Singletons/{Theme,Metrics,Motion}.qml`;
components read them, never hardcoded values. With Settings -> Shell -> Match
wallpaper on, `Theme` resolves to the live wallpaper palette (the same `shell.json`
flag and `colors.json` the rest of the shell uses), so the launcher recolors with
the system theme.
