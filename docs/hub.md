# Ryoku Settings

Ryoku Settings is the desktop's settings app: one floating window with a rail
of categories, each category's pages, and a search over every page and row. The
product is "Ryoku Settings"; the code, the binary, the package and the config
directory keep the internal name `hub`. This guide is how it is built and how to
extend it.

| Path | What it holds |
|---|---|
| `ryoku/hub/quickshell/shell.qml` | the entry point `qs -c hub` loads: the root-only pragma and a loader for `ShellBody.qml` |
| `ryoku/hub/quickshell/ShellBody.qml` | the `FloatingWindow`, the palette watchers and the IPC targets |
| `ryoku/hub/quickshell/Hub.qml` | the frame: rail, list pane, search, work pane, navigation, notices |
| `ryoku/hub/quickshell/registry/` | `registry.js` (categories, pages, aliases, hand-offs, Needs you probes), `summaries.js` (the list pane's lines), `provider-map.js` (where window-manager rows go) |
| `ryoku/hub/quickshell/schema/` | one `<page-id>.js` per page: its sections and rows; `index.js` lists them for search |
| `ryoku/hub/quickshell/pages/` | one `<Name>.qml` per page, loaded into the work pane |
| `ryoku/hub/quickshell/components/` | the shared parts (`SettingsPage`, `RowControl`, `Home`, `SearchResults`, `DesignShelf`, ...) and one `<page-id>/` folder per page for that page's own parts and stores |
| `ryoku/hub/quickshell/Singletons/` | `Changes` (the write path), `Settings` (the shell settings), `ProviderSchema`, `Attention`, `Live`, `Host` |
| `ryoku/hub/backend/` | `ryoku-hub`, the Go program the pages call for anything outside the shell daemon |

The UI is drawn with the Lithic QML library (`docs/lithic/library/qml/Lithic`,
see `docs/lithic/`) and reads the shared singletons in `Ryoku.Ui.Singletons`
(`I18n`, `Bus`, `ShellSettings`, `Spawn`). It holds no palette of its own:
`ShellBody.qml` reloads Lithic when `shell.json`, `theme.json` or the generated
`colors.json` changes, and reduce motion follows the desktop's
`theme.motion.reduce` setting.

## How it opens

Everything opens Settings through the shell daemon, which keeps one instance.

- `Super+,` runs `ryoku-shell settings`, the shell's `settings` surface, which
  calls `hub open`. The launcher action, the desktop menus, the bar's quick
  actions and `ryoku-hub.desktop` (`Exec=ryoku-shell hub open`) do the same.
- `ryoku-shell hub open [<page>[/<row>]]` raises a running Settings and points
  it at the page, or starts one (`flock -n -o /tmp/ryoku-hub.lock qs -c hub`,
  or `qs -p` on the checkout on a dev box) with the page in `RYOKU_HUB_START`,
  so it opens there without building Home first. A page gated on what the
  desktop supports waits for the settings frame, up to 3 s, and lands on Home
  when it is not offered. `ryoku-shell hub open users <profile-id>` opens Users on one
  account profile, and `ryoku-shell hub close` asks it to quit. The verb lives
  in `ryoku/shell/ipc/control.go`.
- A few retired ids (`widgets`, `stage`, `visualizer` and their old
  `desktop-scene` forms) open the Stage Editor instead, since those settings
  moved there.

`ShellBody.qml` serves three IPC targets:

| Target | Function | Does |
|---|---|---|
| `nav` | `open(section)` | navigates to a page id, alias or `page/row` |
| `nav` | `section()` | returns the open page id, `""` on Home |
| `users` | `fromProfile(id)` | opens Users on an account profile |
| `hub` | `close()` | flushes pending writes and quits |

A deep link is a page id (`window-style`), an alias (`windowrules`), or either
with a row id after a slash (`performance/low-power`). `Registry.resolve()`
lowercases it and drops the row part; the `aliases` table in `registry.js` maps
old section names and retired page ids to the page that owns them now, so links
from older code and docs still land. An unknown id, or a page the running
window manager does not offer, lands on Home. A row id that is hidden by its
`when` condition lands on the row it depends on, and a row inside **More
options** opens the fold first.

Settings remembers the last page with `ryoku-hub config set section <id>`
(`~/.config/ryoku/hub.toml`), and Home offers "Continue in" that page.

Settings answers `nav`, `users` and `hub` on the bus (`ryoku-shell hub open
<page>`) and, under the same names, on Quickshell's own IPC, so `qs ipc call nav
open <page>` drives one instance directly. An instance run from a checkout
registers on the bus too, so run it with the installed Settings closed, or in a
sandbox whose bus is a relay. Any instance started from outside
`~/.config/quickshell` reloads QML files as you save them.

### What a start costs

`shell.qml` holds only what Quickshell reads from the root file and loads
`ShellBody.qml` by file URL, because Qt's disk cache skips `qs:` URLs and
everything imported relative to them. From the second start on, every unit but
`shell.qml` loads from that cache. Home reads its facts cheaply: the distro,
the first graphics card, the battery and the active displays from one small
`sh` read in `Singletons/SysInfo.qml`, and the power mode from the daemon's
`powerprofiles` topic. `ryoku-sysinfo` and `ryoku-profile-stats` run only when
About calls `SysInfo.load()`. The provider schema is read once a page or search
needs it, the search index is built the first time search is used, and the
confirmation dialog is created the first time it is asked for. Keep it that
way: something Home does not show waits until the page that shows it opens.

## The layout

`Hub.qml` owns the frame, a Lithic `AppFrame`, so a page only draws its content.

- **The rail.** The Ryoku mark at the top goes Home. The categories follow in
  `registry.js` order, with Updates at the foot; Updates carries a notch while an
  update is ready and opens its page directly.
- **The list pane.** On Home it lists every category with a sentence about it,
  under the title Settings. In a category it shows the category's name, its
  sentence, and its pages, each title over a one-line summary. The lines come
  from `registry/summaries.js`: `categorySentence(id, ctx)` and
  `pageSummary(id, ctx)`, with the fixed lines in its `still` table. They read
  live state through `ctx` (shell settings, the window manager store, daemon
  topics, watched files, Needs you items), so a summary changes when the
  setting does. A page that hands off to another app shows an open-in-new mark.
- **The search field** sits above the list and replaces it with results while
  there is a query. `/`, `Ctrl+K` and `Ctrl+F` focus it; Up, Down and Return
  move through the list or the results.
- **The work pane.** Home (`components/Home.qml`), or the open page. Escape and
  Alt+Left step back: out of the results, then out of a page's own sub-view
  (`back()` on the page), then out of the page, then from a category to the
  list of every category, and then the window quits.
- **Sizes.** Expanded shows the list and the page side by side. Medium shows
  one at a time with a Back plaque. Compact drills down from categories to pages
  to the page.

A page drawn by `components/SettingsPage.qml` has the same shape everywhere:

- **The head.** The schema's title and its one-sentence `lead`, a state line
  when the page passes one, and a note when the window manager hides some rows.
- **The section index.** A page with four or more sections gets one, beside
  the column when the page is wide enough and above it as a picker when it is
  not. It follows the reading position.
- **Sections.** Each is a titled slab of rows. Rows marked `more: true` fold
  under **More options** at the foot of their section. A section whose rows were
  all hidden by the window manager says so with its `missing` text, or
  disappears when it has none.
- **Related.** A schema's `related` page ids end the page as links, skipping
  pages that are not available.

There is no Save and no action bar: a change applies when it is made. See
"Changes" below.

## The registry

`registry/registry.js` is the one list of what exists.

- `categories` is the rail, in order, each with its `id`, `label`, `glyph` and
  its `pages` in list order.
- `pages` maps a page id to `title`, `category`, `file` (the `pages/<file>.qml`
  to load), `keywords` for search, and optionally `needs` or `handoff`.
- `needs: { cap: "<capability>" }` shows the page only when the window manager
  claims that capability (`docs/compositors.md`); `needs: { provider: true }`
  shows it only while a provider is known. Until the first settings frame
  arrives every page shows.
- `handoff: [argv]` makes the page a hand-off: it never opens in Settings, and
  choosing it, from the list, search or a deep link, runs the command through
  `Spawn`. Wallpaper & colours is the one there is: it opens Ryogami's own
  settings (`ryogami wallpaper settings colours`), and `handoffEntries` adds
  Wallpaper, Colours, Theme, Palette and the like to search with their own
  Ryogami commands.
- `aliases` maps old ids to current pages.
- `attention` lists the Needs you probes; see "Needs you".

## Schemas

A page's rows live in `schema/<page-id>.js`, a `.pragma library` file with:

- `title` and `lead`, the page head.
- `sections`: `[{ id, title, missing }]` in reading order. `missing` is the line
  shown when the window manager hides every row of the section.
- `rows`: one object per setting, in reading order within each section.
- `related`: page ids to link at the foot.

A row:

| Field | Meaning |
|---|---|
| `id` | the row's id, unique on the page; deep links and search land on it |
| `section` | the section it sits in |
| `label`, `detail` | the row's name and one line about it |
| `keywords` | extra search words, English |
| `control` | which control draws it (below) |
| `bind` | where its value is read and written (see "Binds and stores") |
| `default` | the value Reset puts back; desktop rows take the provider's default when they have none |
| `more` | `true` folds it under More options |
| `when` | `{ bind, is: [values] }`: shown only while that value is one of `is` |
| `needs` | gates on what the system can do (below) |
| `template` | `true` keeps it out of the drawn rows; the page draws it and the row only feeds search |
| `changeLabel` | the name the notice and Recently changed use, when `label` alone is unclear |
| `options` | `[{ value, label }]` for `segmented` and `picker` |
| `min`, `max`, `step`, `unit`, `pct` | for `slider` and `stepper`; `pct` shows a 0 to 1 value as a percentage |
| `placeholder`, `mono` | for `text` and `path` |
| `command` | the argv an `action` or `handoff` row runs |
| `link` | the page or `page/row` a `link` row opens |
| `actionText` | the plaque's word for `action`, `handoff` and `link` rows |
| `activatable` | `true` makes a `custom` row clickable as a whole |
| `unavailable` | a reason shown in place of the detail, with the control disabled |

`components/RowControl.qml` draws the control from `control`:

| `control` | Draws | Writes |
|---|---|---|
| `toggle` | a toggle; the whole row toggles it | `true` or `false` |
| `segmented` | a segmented control over `options` | the chosen option's `value` |
| `picker` | a picker over `options` | the chosen option's `value` |
| `slider` | a slider with its readout | a number |
| `stepper` | a stepper you can type into | a number |
| `text` | a text field, written when editing ends | a string |
| `color` | a colour field with the palette's swatches | `#rrggbb` |
| `chord` | a key chord recorder | the chord, or `""` when cleared |
| `path` | a file or folder field with a chooser (`kind`, `nameFilters`, `asUrl`) | a path, or a `file://` URL |
| `action` | a plaque that runs `command` | nothing |
| `handoff` | a plaque with an open-in-new mark that runs `command` in another app | nothing |
| `link` | a plaque that opens `link` in Settings | nothing |
| `custom` | the page's own component for this row id (`SettingsPage.controls`) | whatever it commits |

A custom control's root may declare `property var row`, `property var value`
and `property var commit`: `RowControl` fills in the row, keeps `value` bound to
the stored value, and hands over `commit(v)`, which writes through `Changes`.
A `function activate()` runs when an `activatable` row is clicked. Every built-in
control is held to the stored value, so Undo, Reset and a failed write all show
on it at once.

### Gating

A row is shown only when the system can back it, never dead. `rowAvailable()`
in `Hub.qml` checks, in turn:

- `needs.cap`: the window manager claims the capability.
- `needs.hostCap`: `ryoku-host capabilities` reports it supported (`users`,
  `snapshots`, `aur`), through `Singletons/Host.qml`. One name or a list.
- a `link` row's target page is available.
- `needs.ruleAction` and `needs.layerRule`: the provider lists that window rule
  action, or that layer rule field or action.
- a `desktop` row: the provider models the key and the store holds it.

Search applies the same gate, so it never offers a row that is not there. `when`
is not a gate: it shows a row only while another value is right (blur size
while blur is on), and search still finds the row and lands on the row it waits
on.

### Search keywords

Search indexes three things per page: the page itself (its title, its registry
`keywords` and its category), each section (landing on its first row), and
each available row (its label, detail, `keywords` and option labels). Titles and
labels are indexed in English and in the current language, so either finds a
row, and keywords stay English. Write keywords for the words people type when
they do not know Ryoku's name for a thing ("transparency" on the opacity rows,
"potato" on low power mode).

## Binds and stores

A row's `bind` names a store and a key. `Changes.value(bind)` reads it,
`Changes.set(bind, value, label, pageId)` writes it, and `bind.fallback` is
what an unset key reads as.

- `{ store: "settings", key }` is the shell's own settings (`shell.json`). The
  daemon owns the file, its schema and its validation, so Settings never parses
  or writes it: `Singletons/Settings.qml` reads the `settings` topic through
  `ShellSettings` and writes over the bus as `settings.patch` calls, batched and
  acknowledged. A write the daemon does not answer in time fails. `bind.after`
  names a daemon verb to run once the write has landed (the bar style runs
  `shells commit`).
- `{ store: "desktop", key }` is the window manager's neutral store
  (`~/.config/ryoku/desktop.json`). Keys drop the `desktop.` prefix; `wm.<name>.`
  keys keep theirs. An edit goes live through `ryoku-hub desktop preview`, is
  saved with `ryoku-hub desktop save` once edits pause, and the active provider
  writes its own config from it. `ryoku-hub desktop defaults` gives the
  defaults Reset uses.
- `{ store: "command", get: [argv], set: [argv] }` reads a value by running
  `get` (once, parsed as JSON when it is JSON) and writes it by running `set`
  with the value in place of `"{value}"`, or appended when there is none. The
  value is read again after every write.
- Any other store name is a page store: a QML object the page registers for a
  source that is neither of those (a file, a daemon topic, a set of
  `ryoku-hub` calls).

### A page store

A store is a `QtObject` in the page's `components/<page-id>/` folder with:

- `read(key)`: the current value, `undefined` until known.
- `write(key, value, done)`: apply the value, then call `done(true)` or
  `done(false, error)`. Call it exactly once per write, also when the user
  cancels a prompt the write raised: until it is called there is no notice and
  Settings keeps waiting for the store on quit.
- `unsettled` (optional): `true` while a write is still going, so quitting
  waits for it.
- `flush()` (optional): send anything held back, called on quit.

It calls `Changes.touch()` whenever its source changes, so every reader
rebinds, and registers itself with `Changes.registerStore(name, store)` in
`Component.onCompleted`. The page creates it with
`Changes.ensureStore(name, Qt.resolvedUrl("../components/<page-id>/<Name>Store.qml"))`
and reads it as `Changes.stores[name]`. `ensureStore` builds the store in
`Changes`' own context, so it outlives the page and Undo still works after you
leave. `components/performance/PerformanceStore.qml` (a JSON file kept key by
key) and `components/graphics/GraphicsStore.qml` (`ryoku-hub gpu` calls, with
prompts and a Needs you probe) are the two to read first.

## Changes

`Singletons/Changes.qml` is the one write path. A row that writes anywhere else
looks fine and never reaches the desktop, and it gets no Undo.

- **A change applies when it is made.** Shell settings and window manager rows
  apply and announce at once. A command or page store write announces only
  once the store reports success, since it can wait on a password prompt or
  fail.
- **Every change gets a notice with Undo.** A placard at the foot of the window
  says "<label> changed" and offers Undo while it drains. A drag or a held key on
  one setting is one change and updates its own notice. A change with a notice
  of its own (`Changes.set`'s fifth argument, already translated: a list item
  added or removed, a design put on) gets its own placard and never merges.
  Home keeps the session's recent changes with Undo too.
- **Undo puts back only what that change changed.** For a list or a map it
  restores the items that change touched and keeps later changes to the same
  value.
- **Ctrl+Z is scoped to the page.** On a page it undoes the newest change made
  on that page; on Home, the newest of all.
- **A failed write changed nothing.** Its notice closes, its Undo entry is
  withdrawn, the control goes back to the stored value, and the row shows the
  error with Retry. Home lists it under Needs you as "<label> did not change"
  with Retry. A failed window manager save also puts the old value back live.
- **Changed and Reset.** A row whose value differs from its default shows
  Changed and a reset plaque; Reset is an ordinary change, so it can be undone.
- **Quitting flushes.** Closing the window, from the close plaque, Escape or the
  compositor, sends every pending write, calls each store's `flush()`, and quits
  once nothing is unsettled. A dead daemon or backend cannot hold the window
  open: the wait is capped at a few seconds.

### Long work

A write or an action that can take a while says so where it was started, under
its row or in its dialog: nothing for the first second, then the Working mark
with a word for what it is doing, and past ten seconds a progress blade with
Cancel when the job can be stopped (or a line saying how to stop it when it
cannot). An error says what happened and offers Retry. There is no shared
component for this, so a page copies the pattern from the closest one:
`components/backup-import/WorkLine.qml` for a job the page runs itself,
`components/add-ons/TaskLine.qml` for a task object that reports `working`,
`longRunning` and `cancellable`.

## Needs you

Home's Needs you section lists what waits on the user: an update that is ready,
the items in `Singletons/Attention.qml`, and failed writes.

An item is `{ id, title, detail, actionText, page, row, command, args }`, with
English strings that Home translates and fills `%1`, `%2` into from `args`. Its
action runs `command` when there is one and otherwise opens `page/row`; the row
itself opens the page.

- The registry's `attention` probes run once when Settings opens. A probe is a
  command that prints nothing, `null`, `false` or `{ "pending": false }` when
  nothing waits, or a JSON object whose fields override the registry entry's.
  `ryoku-hub wm pending`, `gpu mode pending` and `gpu mux pending` are the
  probes today, so a switch that waits for a log out or a restart is still
  listed after Settings was closed and opened again.
- A page raises and clears items as it acts (`Attention.raise(id, item)`,
  `Attention.clear(id)`), or runs a registry probe again after it changed what
  the probe reads (`Attention.probe(id)`).

## Window manager rows

A window manager provider declares the settings only it has as rows from its
`schema` verb (`docs/compositors.md`, `docs/adding-a-window-manager.md`).
`Singletons/ProviderSchema.qml` reads them once from `ryoku-hub desktop schema`,
with the list of what it cannot honour from `ryoku-hub desktop unhonored`.

`registry/provider-map.js` places each row on a page and section by the row's
`page|tab|group` tag (a row with no page counts as `windowmanager`), trying the exact tag
first, then the tab with any group, then the group on any tab, then anything on
that page; `byKey` places one row by its store key. A `null`
place drops the rows because a page draws them itself (App rules edits shell
layers on its own). A third element gates the rows on a capability when the
row names none. `normalize()` turns a provider row into a schema row: `ctl`
picks the control (`sw` a toggle, `slid` a slider, `step` a stepper or a
slider for fractions, `seg` and `chips` a segmented control or a picker when
the labels are long, `pick` a picker, `text`, `color`, `action`), `adv` becomes
`more`, `caps` becomes `needs.cap`, and the row binds to the `desktop` store.

`SettingsPage` joins these rows to the schema's on the page they were placed
on, and search indexes them the same way. A provider row whose tag the table
does not know is left out; run Settings with `RYOKU_HUB_DEV=1` and it logs
every such row's key.

## Search

`Hub.qml` builds the entries the first time search is used (the field focused
or a query typed) and rebuilds them once per change after that. They come from
the registry, every available page's schema
(through `schema/index.js`), the provider rows and the Ryogami hand-offs, and
hands them to `HubSearchIndex` from `Ryoku.Text`, which ranks them. Its synonym
table sits in `Hub.qml` (`transparency` for `opacity`, `keybind` for
`shortcut`). `_` and `-` split words, so `wpa_supplicant` finds what `wpa
supplicant` would. Words are letters and digits in any script, compared with
full case folding ("Straße" is "strasse") and, at a slightly lower score,
without accents, so "unscharfe" finds "Unschärfe". Japanese, Chinese and Thai
run words together, so a query is also found inside a longer run:
"ショートカット" finds "キーボードショートカット". The query is split into words
the same way, and `ryoku/shell/plugin/tests/tst_hubsearch.cpp` holds the
English ranking to a fixture of the real entries (`hubsearch-en.json`).
Results are grouped by page, each with its
"Category › Page › Section" crumb. When nothing matches, the pages that match
the first three letters of a word in the query are offered instead. Choosing a
result navigates to its page and row, or runs a hand-off.

A row with `template: true` is how a page puts what it draws itself into
search: landing on one scrolls to its section, and opens More options when the
row is `more: true` and the section has bespoke content there.

## Adding things

### A row

1. Add it to `rows` in `schema/<page-id>.js`, in the section and place it
   should read.
2. Pick the `control` from the table above, and bind it to the store that
   really holds the value. A shell setting is validated by the daemon when its
   namespace is one the daemon owns a schema for (`ryoku/shell/ipc/settings.go`),
   and a renamed or removed key needs a doctor reconciler (`docs/updates.md`). A
   new window manager setting needs the provider to model it
   (`docs/compositors.md`).
3. Gate it with `needs` when not every system can back it, and use `when` when
   it only means something while another setting is on.
4. Write `label`, `detail` and `keywords` for someone who has never seen the
   setting. `title`, `lead`, `label`, `detail`, `missing` and `actionText` are
   picked up for translation as written (`docs/i18n.md`); text you build in QML
   goes through `I18n.tr`.
5. Open the page from search with a few of the words someone would type, and
   change the value: the control moves, the notice offers Undo, and the change
   is live on the desktop.

### A page

1. Add the page to `pages` in `registry/registry.js` and its id to its
   category's `pages`, in the order it should be listed. Gate it with `needs`
   when it only means something on some systems.
2. Write `schema/<page-id>.js` and add it to `byFile` in `schema/index.js`, or
   search never reaches its rows.
3. Write `pages/<Name>.qml`, the `file` the registry names. A page made only of
   rows is a `SettingsPage` with its `pageId` and `schema`
   (`pages/WindowStyle.qml`). A page that needs more wraps one in an `Item`
   that declares `required property var hub` and `function focusRow(rowId)`
   (passed on to the `SettingsPage`), and gives the `SettingsPage`:
   - `controls`: row id to `Component`, for `custom` rows.
   - `bodies` and `footers`: section id to `Component`, drawn above or below the
     section's slab.
   - `slabBodies`: drawn inside the slab, above its rows.
   - `moreBodies`: drawn inside More options, after its rows.
   - `stateLine`: the line under the lead.
   - `adaptRows`: a function from rows to rows, applied after the provider rows
     join, when the page reshapes them.

   A page can also pass a schema it builds from live data, as Graphics does from
   what the cards report. A page that is one large surface (Displays, Keyboard
   shortcuts) draws the head, index and sections itself from its schema and
   keeps the same `focusRow` contract. Use `template: true` rows for what the
   page draws itself, so search still lands on it. A page with a sub-view of its
   own adds `function back()` and returns `true` when it stepped out of it.
4. Give it a summary line in `registry/summaries.js`: a fixed line in `still`,
   or a case in `pageSummary` that reads live state through `ctx`.
5. Put its own parts in `components/<page-id>/`, with a `qmldir`.
6. Confirmations go through `hub.confirm(options, yes, no)`, which draws over
   the whole window (`options.timeout` counts down and answers no by itself);
   popovers and pickers take `hub.overlayItem` as their `overlayParent`.

### A store

Write it as described in "A page store", register it, create it from the page
with `Changes.ensureStore`, and bind rows to `{ store: "<name>", key }`. Report
every write through `done`, and set `unsettled` while one is in flight. If the
store's source can change outside Settings, watch it and call `Changes.touch()`
when it does.

### A window manager row

The provider adds the row to its `schema`. When its `page|tab|group` tag is new,
add one line to `table` in `registry/provider-map.js` naming the page and a
section that page's schema already has.

### A Needs you item

For state that outlives the window (a switch that waits for a log out), add a
`ryoku-hub` command that prints the pending state as JSON and an entry to
`attention` in `registry.js` that runs it. For state only a page knows, raise
and clear the item from the page.

## Verifying a change

- **QML loads.** `bin/ryoku-dev-lint-qml ryoku/hub/quickshell` fails on the
  qmllint findings that mean a file will not load (a handler with no signal, a
  property that does not exist, a type that is not a type, a parse error). It
  resolves `Lithic`, `Ryoku.Ui` and `Ryoku.Text` against the installed modules,
  so run it after `ryoku/shell/deploy.sh`. CI's QML Lint job runs qmllint on the
  changed files with syntax errors fatal, plus `bin/ryoku-dev-verify-qml-bindings`,
  which the pre-commit hook runs too.
- **The backend.** `go build ./... && go vet ./... && go test ./...` in
  `ryoku/hub/backend`, as the Go unit tests workflow does.
- **The JavaScript helpers.** `tests/shell-unit-tests.sh` runs every
  `*.test.mjs` under `ryoku/shell`, `ryoku/hub` and `ryoku/ui` with node. In
  Settings that covers the pure helpers that have one beside them, such as
  `Singletons/SettingsWrites.test.mjs` (how shell settings writes are batched
  and acknowledged) and the Displays arrangement model. A new helper with
  arithmetic in it gets a test there; the runner finds it by name.
- **The Lithic components.** A change under `docs/lithic/library/qml/Lithic`
  runs its behaviour tests (`docs/lithic/working.md`):
  `QT_QPA_PLATFORM=offscreen "$(qmake6 -query QT_HOST_BINS)/qmltestrunner" -import docs/lithic/library/qml -input docs/lithic/library/qml/tests`.
- **The page itself.** Deploy with `ryoku/shell/deploy.sh` and open the page
  with `ryoku-shell hub open <page>/<row>`, or run
  `qs -p ryoku/hub/quickshell` from the checkout and open it
  with `qs -p ryoku/hub/quickshell ipc call nav open <page>/<row>`. Search for
  the row too. Change it, undo it from the notice and with Ctrl+Z, and check the
  desktop really changed. With `QT_FORCE_STDERR_LOGGING=1` the QML warnings stay
  in the terminal.
