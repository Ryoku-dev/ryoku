# Ryoku Settings

Ryoku Settings is the desktop's settings app: one floating window that opens
on Home, the reader's profile plate, with a sidebar that holds the whole of
Settings as a folder tree, and a search over every page and row. The
product is "Ryoku Settings"; the code, the binary, the package and the config
directory keep the internal name `hub`. This guide is how it is built and how to
extend it.

| Path | What it holds |
|---|---|
| `ryoku/hub/quickshell/shell.qml` | the entry point `qs -c hub` loads: the root-only pragma and a loader for `ShellBody.qml` |
| `ryoku/hub/quickshell/ShellBody.qml` | the `FloatingWindow`, the palette watchers and the IPC targets |
| `ryoku/hub/quickshell/Hub.qml` | the frame: sidebar, page pane, search, navigation, notices |
| `ryoku/hub/quickshell/registry/` | `registry.js` (categories, pages, their glyphs, aliases, hand-offs, Needs you probes), `summaries.js` (each page's live line, shown when the tree peeks into it), `provider-map.js` (where window-manager rows go) |
| `ryoku/hub/quickshell/schema/` | one `<page-id>.js` per page: its sections and rows; `index.js` lists them for search |
| `ryoku/hub/quickshell/pages/` | one `<Name>.qml` per page, loaded into the page pane |
| `ryoku/hub/quickshell/components/` | the shared parts (`SettingsPage`, `PageHeader`, `RowControl`, `Home`, `SearchDoor`, `SearchResults`, `DesignShelf`, ...), the sidebar in `nav/`, the pages' live previews in `previews/`, and one `<page-id>/` folder per page for that page's own parts and stores |
| `ryoku/hub/quickshell/Singletons/` | `Changes` (the write path), `Settings` (the shell settings), `Tone` (its surfaces and motion), `Ambient` (life at rest), `ProviderSchema`, `Attention`, `Live`, `Host` |
| `ryoku/hub/backend/` | `ryoku-hub`, the Go program the pages call for anything outside the shell daemon |

The UI is drawn with the Lithic QML library (`docs/lithic/library/qml/Lithic`,
see `docs/lithic/`) and reads the shared singletons in `Ryoku.Ui.Singletons`
(`I18n`, `Bus`, `ShellSettings`, `Spawn`). It holds no palette of its own, and
reduce motion follows the desktop's `theme.motion.reduce` setting.

### Design and colours

Settings wears the desktop's look, set on the **Style** page (Appearance) and
kept in the shell settings, so every Ryoku surface wears it too:

- **Design** (`theme.design`): Lithic, Material, Adwaita or Fluent. Four tiles
  show each design's shapes in small (`components/style/StyleChoice.qml`);
  resting on one shows it below in a piece of Settings at its real size, made of
  real controls you can use (`StylePreview.qml`, inside a Lithic `DesignScope`
  so every `CutRect` in it draws that design). The preview stays on the last
  design rested on until one is chosen. Lithic cuts its corners and draws every
  plane by its hairline on one paper; Material lifts the page and its cards in
  steps, Adwaita sets the sidebar apart, Fluent outlines layered planes
  (`Lithic.planes()`, which `Singletons/Tone.qml` reads). The rule every app
  meets is `docs/lithic/styles.md`.
- **Colours** (`theme.colours`): Ryoku's own signature or the desktop's
  palette (the wallpaper's, or a fixed scheme). The same in every design:
  changing the design never changes the colours.
- **Light or dark**: the desktop's, ryogami's `theme.mode` (Dark, Light,
  Automatic from the wallpaper, Sun from the time of day), set through
  `ryogami settings`. Ryoku's colours take it too.
- **Contrast** and, for Lithic on the wallpaper's colours, **Tint**
  (`theme.contrast`, `theme.tint`).

The Lithic library reads all of it from the files (docs/lithic/palette.md §2a)
and `ShellBody.qml` reloads it whenever `shell.json`, `theme.json`,
`matugen.json` or the generated `colors.json` changes. A `LITHIC_MODE` in the
environment (a gallery or test run) wins over the colours, and `LITHIC_DESIGN`
over the design. `ryoku/ui` Tokens rounds the shell's own surfaces with the
design. Settings once had colours of its own (`hub.colours`, `hub.contrast`,
`hub.tint`, under Customise on Home); `ryoku doctor` carries them into these.

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
the first graphics card, the battery, the active displays, the host and user
names, the time up and the memory from one small `sh` read in
`Singletons/SysInfo.qml`. Its four readings come from `Singletons/LiveStats.qml`,
one small read of `/proc` every 3 s while Home is on screen and the window has
the focus, every 15 s while it does not, and never once Home is left.
`ryoku-sysinfo` and `ryoku-profile-stats` run only when
About calls `SysInfo.load()`. The provider schema is read once a page or search
needs it, the search index is built the first time search is used, and the
confirmation dialog and Home's editor are created the first time they are
asked for. Keep it that
way: something Home does not show waits until the page that shows it opens.

## The layout

`Hub.qml` owns the frame, so a page only draws its content. The sidebar lies on
the ground and the page pane beside it; `Singletons/Tone.qml` takes every
surface from the design and palette in use, and every shape is a `CutRect`, so
its corners follow the design.

- **The sidebar** (`components/nav/HubSidebar.qml`). Its title line holds the
  Ryoku mark and SETTINGS, quiet, with one action at its end that shuts every
  folder but the open page's. The search field sits under it, then the tree,
  whose first line is Home, and Updates at the foot, with a dot while an
  update is ready.
- **The tree** (`nav/NavTree.qml`, `NavFolder.qml`, `NavRow.qml`). Every
  category is a folder and every page a leaf hung on its folder's guide line,
  like a project tree, and the tree reads as a sheet's index, as ryoku.dev
  numbers its sections: Home is 00, each folder its number in mono (01
  Appearance to 08 System, 09 Updates at the foot) and its name in spaced
  capitals, with a leader line to the count of its pages
  (`registry/numbering.js`). Pages carry no glyph. Every line ends in its
  Japanese gloss, the real word (`kanji` in `registry.js`). A
  folder opens and shuts on a click or Right and Left; its pages slide in one
  after another, 18 ms apart. The open page's folder opens with it and shuts
  again when the reader goes on to another folder, unless they opened it
  themselves. One
  wash marks the open page: it slides between pages of one folder and grows in
  at a page in another, and rests on the folder while that is shut. A shut
  folder shows how many pages it holds. Up and Down walk the lines on show,
  the open page's sections included; Down from the search enters the tree and
  Up from its first line goes back. Return opens a line, Right opens a folder,
  Left shuts it (from a page, its own). Ctrl+Tab and Ctrl+Shift+Tab leaf
  through the pages in the tree's order from anywhere in Settings. A page that
  hands off to another app shows an open-in-new mark. The room the tree
  leaves under itself holds the machine's live figures and an activity map
  (`components/sheet/Activity.qml`): one square a day for the last weeks, as
  a contribution graph draws it, lit by how long the machine was on that
  day, with the days on, the hours a day and the streak under it; resting on
  a square reads that day out. `ryoku-hub activity [days]` computes the hours
  from the journal: every boot's first to last entry, less the time
  systemd-sleep spent suspended or hibernated, split at local midnight. With
  no journal it reports source `none` and the map stays hidden.
- **The outline** (`nav/NavOutline.qml`). Under the open page hang its
  sections. The one being read is marked as the page scrolls, and choosing
  one brings it to the top. A page offers it through `outline` (the section
  names), `currentSection` and `chooseSection(index)`; `SettingsPage`,
  `Displays` and `Shortcuts` each do, and the frame reads it from the page or
  from the settings sheet the page wraps (its direct child, while on show).
- **The peek.** Resting the pointer on a line shows a card beside it with
  that part's live line from `registry/summaries.js`: `pageSummary(id, ctx)`
  for a page, with the fixed lines in its `still` table, and
  `categorySentence(id, ctx)` for a folder. They read live state through
  `ctx` (shell settings, the window manager store, daemon topics, watched
  files, Needs you items), so a line changes when the setting does. Moving
  along the tree slides the card with the pointer.
- **The search field** replaces the tree with results while there is a query.
  `/`, `Ctrl+K`, `Ctrl+F` and the search on Home focus it, wherever Settings
  is; Up, Down and Return move through the results. An empty search that
  loses the keys closes again. Besides pages, sections and rows it finds
  Home's own entry, Customise Home, which opens Home's editor.
- **The sheet** (`components/sheet/`). The page pane is printed like an
  instrument sheet: a faint registration grid with corner crosses
  (`RegSheet`), a masthead over the page with its number and place
  (`§ 08.1 SYSTEM / POWER & BATTERY`), its place among all pages, the window
  manager, the bar and the clock, over a tick ruler whose marker follows the
  scroll (`Masthead`), and a colophon at its foot with the page's id as a
  Code 39 barcode (`Colophon`). A page that fits leaves its foot to its folio, its
  number set huge and faint, which rolls to the next number when another
  page opens (`Folio`). Sections are numbered and ruled (`01 / 07`).
  Japanese runs through the sheet as it did through the stable Hub: the
  masthead carries the category's and page's kanji, the page's kanji stands
  vertically beside its plate or preview (`Tategaki`), the folio sets it
  huge beside the number, and the colophon carries 力と美のために. All of it
  follows Fonts & icons › Show Japanese glosses (`UiPreferences`).
- **Life at rest** (`Singletons/Ambient.qml`). Only while Settings is the
  focused window and motion is not reduced: the clock ticks, the figures
  sample, and every 11 s a scan crosses the plates at 30 frames a second.
  Between scans nothing moves, and an unfocused window draws nothing.
- **The page pane.** Home (`components/Home.qml`), to the pane's edges, or the
  open page, which slides in as it opens. A page starts with
  `components/PageHeader.qml`: its name with the page's own actions (Reset
  page) on the same line, what it is for, any note, and a spec strip of its
  sections, settings and changed settings, then its first section. A page may
  show a live preview at its head's right
  (`SettingsPage.headerPreview`, drawn from `components/previews/`): Window
  style draws a window in front of another in the page's corners, frame,
  see-through, dimming and shadow, Window behaviour three tiled windows in its
  gaps, Animations a window opening and closing in the chosen effect and
  speed (standing still while animations are off or motion is reduced), and
  Fonts & icons the chosen faces and size. Each reads the same binds
  as the rows and follows a change as it is made; they illustrate the
  settings, they are not pictures of the screen. Any other page shows its
  category's plate there (`components/sheet/Plate.qml`): a classical
  sculpture from ryoku.dev's plates set as a live ordered dither in the ink
  (`shaders/plate.frag`), which develops in from the top as it arrives, with
  its credit under it (`registry/plates.js`, `art/plates/`). The close button
  stands at the masthead's end. Escape and
  Alt+Left step back: out of the results, then out of the search, then out of
  a page's own sub-view (`back()` on the page), then Home, and then the window
  quits.
- **Sizes.** From 880 wide the sidebar stands beside the pane. Narrower, it is
  a drawer the pane's menu button draws out over a shade, and the search on
  Home draws it out too. Compact Home lists the categories under the profile.

### Home

Home is the profile plate, the old Hub's Profile rebuilt in Lithic and
composed like the cover of ryoku.dev. It fills the page pane to its edges,
with no slab:

- **The statue and the word.** The stock figure is the site's own: the
  Caracalla Farnese, a real photograph (`art/caracalla.png`, Marie-Lan Nguyen,
  CC BY 2.5). Its museum label stands upright in a gutter at the plate's outer
  edge, beside the figure and off the marble, wherever the figure stands
  beside the column; a narrow plate leaves it to About's credits, and the root
  `NOTICE` carries the source, as the licence asks. The system card draws
  the same photograph (`components/about/CardFigure.qml`); a figure the reader
  chose (a gallery figure or their own picture) takes its place on
  both. Behind it the word Ryoku is carved, the one line set in capitals:
  oxblood on the black paper (the sun mixed 34% into it), cut into the stone
  on the light paper (the ink mixed 18% into it, since the sun's mix turns
  salmon there). The reader's first big word replaces it. Text never lies on
  the marble: the column keeps to its side, and a narrow plate stands the
  figure small beside the name, under the top bar.
- **Depth.** On a roomy Home the plate answers the pointer in three planes:
  the figure turns toward it in perspective on its own foot (up to 9° and
  5°, following it through `components/Chase.qml`, which closes 90% of a move
  in about 80 ms and never overshoots) and
  catches the accent on the edges that face it (`shaders/rim.frag`), the
  carved word drifts the other way behind it, and the column moves a little
  with it. A light of 1-bit dots follows the pointer, drawn only in a box
  around itself so moving it repaints a patch of the plate; left alone, it
  rests behind the figure's head, with a few motes of dust drifting up
  through it at 15 frames a second while Settings is in front
  (`shaders/light.frag`). A still plate (the studio's preview) stands flat
  on its own paper.
- **The name and the motto.** The name is the plate's one monument, over the
  user and host names and the clock, then the reader's line, by default the
  motto "For the sake of power and beauty." with its gloss, and a tick ruler
  whose first segment is the sun (`components/TickRuler.qml`).
- **Its entrance.** As Home opens, the column's groups rise in turn, the
  carved word stamps in and the readings count up, each once
  (`components/Reveal.qml`); nothing moves after, and reduced motion plays
  none of it. This is the cover pattern of `docs/lithic/layout.md`.
- **What waits.** With nothing waiting, one claim line says Ryoku is up to
  date and which release it is. An update that is ready, the Attention items
  and failed writes stand on the one toned slab, Needs you, and the first of
  them holds the plate's one gold action.
- **This computer.** A datasheet says what stands still (the release and the
  edition's name from `os-release`, the window manager, the time up, the
  network, the battery): keys in mono, one vein column rule, values beside it.
  Then four readings draw as engraved lines: processor, graphics, memory and
  network. About keeps the table of every fact.
- **Recently changed** lists the session's changes with Undo, once there are
  any.
- **The top bar** holds "Continue in" the page the reader left off on, an
  expand button that shows the system card large over a shade
  (`components/about/CardViewer.qml`; Escape or a click beside it closes it),
  Binder, Share and Customise; in a narrow window, whose sidebar is put away, the search too.
  Share copies the system card to the clipboard as a picture that carries its
  machine, words and figure (`CardShare`), drawing it out of sight once the full
  read-out is in; the button says Copied, and a copy that failed says why on
  the bar, left of the buttons.

Customise opens the **card studio** (`components/about/CardEditor.qml`), where
everything about the reader's look and system card lives. Its preview shows
the card or Home's plate as they will be; its tabs are:

- **Picture**: the stock Caracalla, the reader's own picture, chosen by eye in
  `components/ImageChooser.qml` (a folder's pictures as thumbnails with a large
  preview, and Pictures, Wallpapers, Downloads and Home to jump to), or a
  gallery figure. A badge holder's picture may move (see Moving pictures
  below), and the tab offers **Use my live wallpaper** when Ryogami's wallpaper
  is a video within the size; everyone else reads that moving pictures come
  with a team badge. A badge holder with their own picture also gets
  **Cutout for the figure breaking the frame** (see Card styles below).
- **Adjust** (own picture): the look (Photo, Black & white or Dots), Fill or
  Whole picture, zoom, position, brightness, contrast, colour (photo) or dot
  size and ink (dots), and Reset. Dragging the picture on the preview moves it
  and the wheel zooms it. A new picture starts as a photo filling its frame.
- **Words**: the name, the line under it and the big words; with the Stand
  card style, the Stand's name too.
- **Layout**: a badge holder's **Card style** (Classic or Stand), whether Home
  shows the plate or the card (`homeView`), the plate's parts and the figure's
  side.
- **Share**: the card's rarity, the tier ladder (resting on a tier shows its
  look on the preview), Copy, Save, Binder and Back to stock
  (`components/about/CardShare.qml`). Binder closes the studio and opens the collection.
- **Badge**: the badge and certificate, request code, activation, team design
  checks and local preview, with issuing and revocation tools for the owner.

Search reaches the studio on its tabs: Customise Home, Adjust your card's
picture, System card, Badge and Show the card on Home. Card binder opens the collection
from any page. The plate, the card and the studio share one design: they read `Ryoku.Ui.Singletons.ProfileStore`
(`profile.json`), draw the figure with `components/about/CardFigure.qml`, and
are edited through the `card` store, so every change has its notice and Undo.
CardFigure frames the reader's picture (`zoom`, `focalX`, `focalY`, `fit`),
tunes it (`brightness`, `contrast`, `saturation`) and draws it in its `style`:
`photo`, `mono`, or `dither` through `shaders/plate.frag` (a picture chosen
before there was a style was dithered, and stays so).

**Moving pictures.** Every picture the studio takes goes through
`ryoku-hub profile picture <path>` (`backend/profilepicture.go`), which reads
what the file is from its bytes, never its name, and keeps it beside
`profile.json` as `profile/hero.<ext>`. A name that says video on a picture,
or picture on a video, is refused. The rule for who may move is the one
Settings shows as `Badge.ownBadge`: an activated badge the team list has not
revoked (`ownBadge` in `backend/badge.go`, which `badge status` reports too).
A holder may keep an animated GIF or WebP, or an MP4, WebM or MKV video, up to
10 MB; ffprobe must find one picture stream (sound is allowed and never
played) and at most 60 seconds. ffmpeg keeps the first frame of a video or
WebP beside it as `hero-<ext>.png`. Anyone else keeps a still: a moving GIF or
WebP becomes its first frame as `hero.png`, and a video is refused. A refusal
names the size or length it measured. `ryoku-hub profile wallpaper` reports
Ryogami's current wallpaper (`~/.local/state/ryoku-wallpaper`, then its
`outputs.json`) when it is a video, with its size, for **Use my live
wallpaper**.

CardFigure draws a moving picture through `components/about/CardMotion.qml`
(`AnimatedImage`, or `MediaPlayer` and `VideoOutput` with no audio output),
loaded by URL inside the same frame, so zoom, position, the three looks and the
dither all apply to it. It plays only while its host passes `live` (the card or
plate pointed at) and `Ambient.on` (Settings focused, motion not reduced); at
rest the loader is inactive, nothing decodes, and the figure shows the first
frame. A badge lost later leaves the still. The shared card picture, the
record's picture and a `.ryoprofile` stay still: the card is grabbed at rest,
and export packs the first frame as `hero.png`.

**Card styles.** A badge holder chooses how their own card is drawn, per
machine, in Layout: `style` in `profile.json`, `classic` (the trading card,
the default) or `stand`. The rule is the moving pictures' one: TradingCard
draws a style only while `Badge.ownBadge` holds, so a lost badge brings the
classic card back, and a `.ryoprofile` whose `style` is `stand` is refused on a
machine without a badge ("the Stand card style comes with a team badge, and
this machine holds none"). A style is Ryoku's own code, never part of a team
design pack, which carries no QML.

The Stand style (`components/about/styles/StandCard.qml`) draws the card as a
JoJo Stand card on a manga page: the machine is the holder's Stand and its
parts are its parameters. TradingCard keeps computing every fact and keeps
drawing the card code's rulers over the page, so the code, the seal (`Emblem`,
stuck on as a sticker with a star), the picture (`CardFigure`, moving pictures
included), sharing and the binder all work as on the classic card. The page
holds:

- the title 「NAMEの奇妙な機械」 with "NAME'S BIZARRE MACHINE", set smaller to fit
  a long name, and the part line, whose part is the release's major version;
- the holder's own picture in the panel, with a speech balloon of the motto, the
  オラオラオラ sound effect in its burst and ゴゴゴ and ドドド at the edges;
- the Stand user's bar: name, edition line, up time, packages and kernel, and 力
  with the power;
- the three parts, each with the grade of the parameter it feeds;
- the Stand's parameter chart (`StandChart.qml`) and the Stand's name in 『』:
  the Words field, or one taken from the motto's last words
  (`registry/stand.js standName`); the stock motto names it POWER & BEAUTY;
- TO BE CONTINUED, carrying the card number, tier and points.

The grades (`registry/stand.js`, tested in `stand.test.mjs`) are A 超スゴイ to E
超ニガテ: the highest letter whose floor the value reaches, else E. Floors run D,
C, B, A on the steps `registry/rarity.js` already scores, and the card prints
them as a legend.

| Parameter | Read from | D | C | B | A |
|---|---|---|---|---|---|
| 破壊力 Destructive power | graphics class | 2 | 3 | 4 | 5 |
| スピード Speed | threads | 4 | 8 | 16 | 32 |
| 射程距離 Range | display width | 1280 | 1920 | 2560 | 3840 |
| 持続力 Durability | memory, GB | 8 | 16 | 32 | 64 |
| 精密動作性 Precision | refresh, Hz | 60 | 90 | 144 | 240 |
| 成長性 Potential | rarity points (the tier's) | 6 | 9 | 12 | 15 |

A holder may add a cutout with `ryoku-hub profile cutout <path>`
(`backend/profilecutout.go`). It is the figure cut from their own picture on a
clear ground, and the Stand card lays it over the panel's border, above and
beside the panel, never below it where the code runs. The cutout checks:

- it needs a badge and the holder's own picture;
- it is a PNG of up to 8 MB, 64 to 4096 pixels a side;
- at least 2% of its pixels are clear and 2% solid;
- its shape is within 2% of the picture's (still frame or video poster).

It is kept as `profile/hero-cutout.png` with `hero.cutout: true`, and
`--clear` removes it. A new picture clears it, and a moving picture draws
without it. The cutout also lays the focus lines and a purple Stand behind the
figure. Without a cutout, the panel is a clean manga panel. A `.ryoprofile`
carries the cutout beside the picture. Import checks it as a PNG with a clear
ground, and sets `hero.cutout` to false when none came with it.

At rest the page is one still and nothing runs. Pointed at, THE WORLD stops time
from the pointer: an inverted sphere with a magenta and a cyan skin covers the
page, 「ザ・ワールド」 lands, and after 「時は動き出す」 the sphere closes. The card
code's rulers are drawn above it and never change. Then the chart draws in
corner by corner, each grade landing, and the page stirs:

- the menacing glyphs rise and throb;
- the focus lines flicker;
- the picture splits red and cyan;
- the Stand pulses;
- the sound effect shakes.

Let go, the frame freezes yellowed (`shaders/stand-world.frag`), TO BE
CONTINUED slides in above the tint, and the page settles. Reduced motion keeps
the still. The share still and the record's picture are drawn at rest; the
record's still keeps the panel clean of the balloon, sound effect, ゴゴゴ and
cutout, as it leaves off the seal.

#### Badges and team designs

Open **Home > Customise > Badge**, or search for Badge. Copy **Your request
code** and run `/badge` in the **claim-badge** channel, under **Badges** on the
Ryoku Discord, for Tester or Contributor. Team members send the request code to
nero instead. Paste the returned badge code into Activate. The Badges category
also holds the private **core** and **nixos-team** channels, where team designs
are submitted, and nero's **card-review** channel, where they are approved.
Codes are signed and tied to this machine's device key, not unlocked by a setting
or by copying the codebase. The 16-hex card id comes from the first eight bytes
of SHA-256 over `ryoku-card-id-v1`, a zero byte and the device public key, not
from the hostname or machine-id. Keep the device key private.

The badge is a seal Ryoku draws itself (`components/badge/Emblem.qml`), after
the layout of certification badges: the role spelled round the rim with the
Ryoku mark and its kanji, a short code read clockwise four times round a
diamond, the card code's ruler ticks bent into the rim, and a hanko at the
centre with the role's kanji knocked out of a plate in the role's ink (試
tester, 貢 contributor, 雪 NixOS, 核 core, the Ryoku mark for the founder).
Each role keeps one ink whatever the theme, matching its Discord role. While in
view the rim turns slowly and a light runs round the code; reduced motion holds
it still. **View certificate** opens the details beside a large seal. Mika can
sign Tester and Contributor badges, and nero can sign every role, including
NixOS, Core and Founder. Every badge reads as issued by neur0map; the
certificate's fine print names the key that signed it (Mika or neur0map), and
Mika keeps revoking the tester badges it signed when someone leaves the testers.
Each GitHub login may hold at most eight active badges; revoke an old machine's
badge before adding a ninth.
The owner's signing tools appear only when the owner key is available in the
keyring. **Make a code** takes the member's request, login and role, then returns
a code to copy. **Revoke** accepts that code or its badge id.

Team roles can load a design folder, read its check results and preview it on
their own card. A preview is local and disappears when the studio closes.
**Save for Mika** packs the checked folder to the chosen path, initially
`~/Downloads/ryoku-design-<name>.zip`. Use `/card submit` in the team channel.
Mika handles the submission and nero approves it before other people see it.
An owner can paste the SHA from Mika's review to preview a pending submission.
The newest signed approval timestamp per login wins. Reposting the same approval
does nothing, and an older approval or a different one with the same timestamp
cannot replace it.

A design contains `design.json` and flat PNG, JPEG or WebP files. Animated WebP
is allowed. There is no QML, JavaScript, shader, executable, external URL or
subdirectory. The JSON gives the badge its centre mark, code word, kanji (up to
four characters) and ink (the seal itself is always Ryoku's), then the card palette,
background, one to four art layers, an optional frame and glow mask, up to six
stock card effects, and optional tagline and epithet. Ryoku still draws the
name, hardware, machine code, moves, traits and footer itself.

The pack is at most 8 MB, expanding to at most 24 MB and 24 files. Images are
bounded to 2048 x 2048, with at most 96 animation frames per image and 64 million
decoded pixels across the whole pack, counting every frame's full canvas and
each still once, including unused files. Design images decode at no more than
twice their drawn dimensions; animations play only while live and do not cache
all frames. Unknown fields, links, repeated names, traversal paths and overly
compressed files are rejected. Numeric effect settings are clamped and text
contrast must reach 4.5:1. Run `ryoku-hub design check <folder-or-zip>` for the
JSON result and `ryoku-hub design pack <folder> <out.zip>` to make the checked,
deterministic zip. Fetching a design verifies its hash before extraction.

Settings reads badge status on opening, after badge or design actions and
every ten minutes while open.

**Live cards.** Anyone can keep their card online: the switch **Keep my card up
to date for friends** (`components/about/CardOnlineSwitch.qml`, on the studio's
Share and Badge tabs, with an info button that explains it plainly) runs
`ryoku-hub badge online on|off` and starts off for everyone, badge holders too. While it
is on, Settings publishes a signed card record three seconds after profile or
hardware-code changes settle, and only once the machine's full read-out and the
card's code (with its distro) are in; Settings starts the read-out for it when
no page has. Published sooner, a record carried no threads, refresh rate or
width and dropped the card's rarity in every binder. The record carries the
machine's public key (`device`), whose hash is the card id, so it needs no
badge: only that machine can sign it, and binders check the signature
themselves. Turning the switch off sends a DELETE signed by the same key. A
custom picture is drawn into a fresh temporary still without the seal, which
overlaps the figure's corner and would leave a clipped copy in the picture; the
backend crops and bounds it, then the temporary still is deleted. A local
design preview is never published. Each record is stamped after the last one
this machine published, so a clock that stepped back can't get it refused as
older. When the gate refuses a record because this computer's clock is more
than ten minutes off, it is stamped again in the gate's time from its `Date`
header; when a gate replica hasn't seen a badge published a moment ago, it is
sent again six seconds later. Any other refusal is shown with the gate's
reason. A sync or status error clears itself once the next one succeeds.
team.ryoku.dev deletes a record nobody has changed in 90 days; binders keep the
copy they have.

A card with a style is published as a version 3 record. It has the version 2
fields plus `style`, signed with the domain `ryoku-card-v3` over the version 2
payload, then the style's byte length as two big-endian bytes, then the style
in UTF-8. Only a machine whose badge holds builds one. A classic card stays
version 2 and never says `classic`. team.ryoku.dev refuses a styled record from
a machine without an active badge. A binder draws a friend's style only when
the gate vouches for an active badge on that card whose device signed the
record. A style id it doesn't know yet is drawn classic, and a card added from
its picture alone is always classic.

**The binder's check.** One request (`POST /v1/cards/batch`,
`backend/cardpull.go`) carries every card id with the version the binder holds,
and returns only the records that changed, the active badges and team designs
for those cards, and the server's next check time. The binder asks nothing
before that time (`$XDG_CACHE_HOME/ryoku/team/pull.json`); all times are the
server's UTC epoch seconds, adjusted by the measured offset, so a time zone or
clock change on the reader's machine buys no extra check, and the gate rate
limits checks per address on its side. A badge holder signs each check with the
badge's device key and is exempt: no wait between checks and no per-address
check limit, only the 120-a-minute flood limit everyone has. When Cloudflare's
daily request cap is reached, the binder says updates are paused and resume at
the next UTC midnight, taken from the server's `Date` header. Cards show as
**Live** (its owner's changes reach you), **no longer online** (the last
version you got, after its owner turned the switch off or left it unchanged for
90 days), or a picture only. Verified badges still show their role, issuer,
date and certificate. Records and pictures are kept in
`$XDG_CACHE_HOME/ryoku/team` (`record-<card id>.json` and `.jpg`), so an
unchanged picture isn't downloaded again and the last verified copy stands in
whenever the gate can't answer.

A card's picture file is rewritten in place when it changes, and Qt keeps an
image it has loaded by its URL, so the figure's URL carries a revision: a
friend's picture has the picture's digest (`hero.rev` in `ryoku-hub cards`
output), and your own `ProfileStore.pictureRevision`, bumped by the studio and
Use their picture when they install a new one.


The system card is a trading card (`components/about/TradingCard.qml`, 630 x
880, a playing card's shape): the name and its 力 power (threads, memory and a
discrete graphics card, in tens), the figure in a holo window (the reader's
own picture fills it), a type line with the first six hex characters of the
device-derived card id, the distro, window manager and graphics card, the processor, display
and memory as its moves, uptime, packages and kernel as its traits, the motto
as flavour text, and the figure's credit as "Illus.". Its text is drawn as outlines
(`Text.QtRendering`), so it stays sharp at any scale it is shown or saved at. It asks SysInfo for the
full read-out when it is drawn. Home shows it in the figure's place when
Layout says so, turning toward the pointer. The machine sets its rarity
(`registry/rarity.js`, tested by `rarity.test.mjs`): points for threads,
memory, the strongest graphics card's class and the display's rate and width.
Graphics built into the processor count too: class 1, 2 for the strong ones
(Radeon 680M to 890M, Arc 130V and 140V) and 3 for Strix Halo, recognised from
the full name or the eight characters a card carries. Built-in graphics beside a
discrete card add one point on top of the card's class. Only a discrete card adds
power, and it is the one counted when it ties with the built-in graphics. There are
six tiers from Common (no foil, a bone border) through Uncommon, Rare (the
sun's border, holo window), Holo rare (foil face, gold edge) and Ultra rare
(gold border, full art, sparkles) to Secret rare: a wider border cut like a
diamond (`shaders/diamond.frag`, facets that catch the minerals' colours as the
card tilts, some flashing white), an etched foil of every mineral, a gilded
name, twice the sparkles drifting and turning, beating stars and a light that
crosses the whole card every few seconds (`shaders/sweep.frag`), all at 30
frames a second while the card is under the pointer. The foil is
`shaders/foil.frag`: a soft wash of the four state colours and one sheen of
light, smooth and held under 60% opacity, sliding as the card tilts.

**Copy** and **Save** draw a still card at twice its size (`CardShare`).
The card travels in its pixels, because a chat app re-encodes a picture
and keeps nothing else: the three tick rulers that rule off the card's parts are
its **card code** (`backend/cardcode.go`, read by `Singletons/CardCode.qml`
through `ryoku-hub profile code`). Each tick is one of four heights, two bits;
the first and last tick of each ruler stand full height to calibrate ink, paper
and scale. Version 4 carries the name, the line under it, the first big word,
the figure (stock, a gallery figure by name, or the reader's own picture),
threads, memory, refresh rate, display width, graphics class and short name,
whether built-in graphics sit beside a discrete card (the low bit of the first
byte), distro, window manager and the eight-byte device-derived card id. Version
3 cards, the same without that bit, still open. A card that
decodes cleanly but carries another code version is refused by name: the binder
says whether the friend or the reader has to update. The body has a length, a
CRC and 20 bytes of Reed-Solomon parity (`backend/reedsolomon.go`) that repair
up to 10 misread bytes. Ticks are measured by the ink they lay down over a
period, so a resize keeps them readable. The still is drawn at twice the
display scale, so on a 133% display a 2-unit tick spans 5.33 pixels; ticks are
drawn with antialiased edges so each one lays down its exact ink. Cards shared
before that from such a display have ticks snapped to 5 or 6 pixels, so when
the ink alone does not decode, the reader divides each tick's ink by its own
width along the first unit, which every tick covers, and tries again; the CRC
and parity accept only a clean read. An own picture comes back lifted from the
card's window. No hidden profile or full-size picture is attached to a share.

**Binder** (`components/about/Binder.qml`) collects friends' cards. Ask a friend
to press Share on their Home and send the picture, then **Paste a card** or
**Open a card** here (`ryoku-hub cards add <path>|--clipboard`). Pictures Qt can
read but Go cannot, such as WebP, are turned into PNG first. A crop, part of a
screenshot or a picture shrunk too far is refused. Cards live in
`$XDG_DATA_HOME/ryoku/cards` (normally `~/.local/share/ryoku/cards`); adding a newer
card from the same machine replaces it but keeps the original added date.

The binder lays the rarest cards first, and cards as rare as each other newest
first (`rarity.js` `rarest`). It draws live cards, including their foil and
pointer tilt. Open one to compare threads, memory, refresh rate, display width,
graphics class, rarity points and power with your own machine; the higher
number is gold. **Use their picture** takes only the custom picture or gallery
figure through the `card` store, with Undo. Your words, layout and machine stay
yours. **Remove** asks first, then forgets the collected card with its cached
record and picture. Trading never replaces your design.

A traded card reads every fact from its saved entry, not from your computer.
Its traits show From, Added and Rarity points, and its footer is stamped
**TRADED · from the owner's name**. It has no copy, save or share action.
Adding your own card is refused: the binder is for friends, and your own card
always shows your own machine.

A page drawn by `components/SettingsPage.qml` is a sheet, with the same shape
everywhere:

- **The head** (`components/PageHeader.qml`). The schema's title (the masthead
  above it says where the page lives) with the page's own actions on the same
  line; its one-sentence `lead`, a state line when the page passes one, and a
  note when the window manager hides some rows. At the head's right stands the
  page's live preview when it has one, otherwise its category's dithered plate.
- **The entrance.** The first sections rise in turn as the page opens, 40 ms
  apart; a rebuild after that moves nothing, and reduced motion plays none of
  it.
- **The outline.** The page's sections hang under its line in the sidebar and
  follow the reading position; there is no index or picker on the page.
- **Sections.** Each is a title over a card of rows (`Tone.card`): a cut
  hairline box in Lithic, a filled card in the round designs. Rows
  marked `more: true` fold under **More options** at the foot of their
  section. A section whose rows were all hidden by the window manager says so
  with its `missing` text, or disappears when it has none.
- **Related.** A schema's `related` page ids end the page as links, skipping
  pages that are not available.

There is no Save and no action bar: a change applies when it is made. See
"Changes" below.

## The registry

`registry/registry.js` is the one list of what exists.

- `categories` is the tree's folders, in order, each with its `id`, `label`,
  `glyph` and its `pages` in list order.
- `pages` maps a page id to `title`, `category`, `file` (the `pages/<file>.qml`
  to load), `keywords` for search, and optionally `needs` or `handoff`. Its
  glyph in the tree comes from `glyphs`.
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
`longRunning` and `cancellable`. A package install, update or removal that
prints gpk's progress events runs in `Ryoku.Ui`'s `PackageJob`, which Settings
and RyoStore share, under `components/installed-apps/JobLine.qml`, whose
counts fill the blade.

### Installed apps

Apps › Installed apps lists every app app search can open, read from
`gpk apps --json` (`components/installed-apps/InstalledAppsStore.qml`, one row
per shown desktop entry), with Ryoku's own apps from `ryoku apps list --json`
pinned above them. `apps.js` holds the pure parts (size groups, source kinds,
the space overview that counts each package once, the commands) and
`apps.test.mjs` covers it; the progress events are read by `Ryoku.Ui`'s
`PackageJob` and `lib/jobevents.js` (`jobevents.test.mjs`).
Show in app search writes the launcher's `hiddenApps` (the `launcher` store),
so it has Undo; a launcher style that cannot hide apps disables it. Remove
first runs `gpk remove -m <manager> --json` as the reader: a package others
need (`required_by`) is never offered, the row names them instead. Otherwise
the confirmation names the apps of the same package and the space that comes
back, and the removal runs where it may: `pkexec ryoku-host apps --stdin-cancel
remove --manager <m>` spawned by Settings itself (so polkit's keep lasts the
session; Cancel closes its stdin), `gpk remove -m flatpak|local --yes --events`
as the reader (Cancel sends SIGTERM), or the AUR in a terminal. Ryoku's own
apps update and remove through `ryoku-host apps --stdin-cancel ryoku
update|remove <id>`; `ryoku-desktop` entries are never removable. Get more
apps opens RyoStore (`ryostore open apps|ryoku-apps|sources`). The store asks
`gpk --version` first: a gpk older than 0.7.0 takes `apps` for a search, so the
page says plainly that listing apps needs gpk 0.7.0, which comes with the next
update, and points at Updates; Ryoku's own apps still show. Output that is a
package search rather than desktop entries reads the same way, never as an
empty list.
`tests/ui/installed-apps-flow-probe.sh` drives the job and its line with a
stand-in pkexec and ryoku-host.

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
     join, when the page reshapes them. Game bar uses it for rows that need
     several switches on: such a row lists them in `whenAll`, outermost first,
     and `adapt()` in `schema/game-bar.js` sets its `when` to the first one that
     is off, so the row waits on that switch and a deep link lands on it.

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
