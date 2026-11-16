# Copy

Ryoku talks the way the Hub talks: plain sentences addressed to you, about
what you see and what happens next. This page is written from the Hub's real
strings; each example names the file it comes from (paths under
`ryoku/hub/quickshell/`, or `Lithic/` for `docs/lithic/library/qml/Lithic/`).
The words for the parts of a page (masthead, spec
strip, placard, plaque tiers and the rest) are defined in
[design.md](design.md#2-vocabulary).

## The voice

1. **Second person, plain verbs.** "Drag your picture to move it, scroll to
   zoom" (`components/about/CardEditor.qml:277`). "Your machine draws a %1
   card" (`CardEditor.qml:594`).
2. **Say what it does, in the reader's words.** A page lead says what the page
   governs: "How every Ryoku surface looks: its design, its colours, and light
   or dark." (`schema/style.js:4`). "How windows look: their corners, frames,
   see-through surfaces and the effects around them."
   (`schema/window-style.js:4`).
3. **Errors say what happened and what to do next.** "Could not install %1:
   %2. Check the connection, then try again."
   (`components/DesignChooser.qml:250`). "Could not use this file: copying it
   took too long. Try again, or choose another file."
   (`components/desktop-dock/ReloadCover.qml:117`).
4. **A consequence is named before it happens.** "Log out now?" / "Every open
   app closes, and work you have not saved is lost." / "Log out"
   (`Hub.qml:369-373`). "Your figure, words and choices are forgotten. Undo is
   offered right after." (`components/about/CardShare.qml:130`).
5. **Calm about failure.** "The desktop service is not answering, so what
   Settings shows may be out of date and changes may not take. Settings picks
   it up again by itself once it is back." (`components/Home.qml:791`).
6. **Never** an em-dash, an exclamation mark or an emoji, and never a mention
   of AI or of the tools a surface was designed with. None of the Hub's 3,145
   translated strings has one.
7. **The product is Settings** in the interface ("How Settings would look in
   %1", `pages/Style.qml:107`). "Hub" is a name for code and docs only.

## Sentence case, and where the capitals come from

Every string is written in sentence case: titles, row titles, details,
plaques, notices, claims. Names of places keep their capital inside a
sentence: "Open Updates" (`components/Home.qml:818-824`), "Continue in %1"
(`Home.qml:634`), "Design set to %1" (`pages/Style.qml:35`).

A few running lines show in **spaced capitals**: the tree's folder names, the
sidebar title, the masthead, the spec-strip labels, the Activity summary, the
card head and the "EXAMPLE" chip. Their text is still written in sentence
case ("Appearance", "Sections", "Settings"); the capitals and the tracking
are set on the font (`font.capitalization: Font.AllUppercase`,
`font.letterSpacing`), for example `components/nav/NavRow.qml:164-165` and
`components/PageHeader.qml:129-130`. That keeps every string translatable as
written. Never type capitals into a string. The full list with tracking is in
[design.md](design.md#4-type).

## Periods

| Kind | Period | Example |
|---|---|---|
| page lead, note, dialog text, error, notice body | yes | "Some options are hidden because this window manager does not support them." (`components/SettingsPage.qml:728`) |
| row detail | no (536 of 556 schema details) | "How rounded window corners are; 0 keeps them square" (`schema/window-style.js:21`) |
| title, row title, plaque, claim, section name | no | "Window style", "Corner radius", "Reset page", "Up to date" |
| a question title | a question mark | "Log out now?" |

A row detail may hold two sentences and still drop the last period: "For the
whole desktop. Automatic follows the wallpaper's brightness, Sun the time of
day" (`schema/style.js:42`).

## Plaques and actions

A plaque says its verb, in sentence case, one or two words when that is
enough: "Reset page", "Retry", "Undo", "Copy", "Save picture", "Paste a
card", "Open a card", "Done", "Back", "Remove", "Activate". A confirm's action
plaque repeats the verb of its question ("Log out now?" / "Log out"), beside
"Cancel". No plaque says "OK" or "Yes".

The tier carries weight, not the word: one gold verb per view, the rest quiet
([design.md](design.md#1-principles)).

## Notices

Every change says so on a placard, with Undo (`Hub.qml:1152-1154`):

- "%1 changed" with the setting's own name, or the change's own notice:
  "Design set to %1" (`pages/Style.qml:35`).
- "Undo" on the placard.
- A countdown confirm says what it will do by itself: "Reverting in %1 s."
  (`Hub.qml:1069`).
- A typed confirmation names the word: "Type %1 to confirm"
  (`Lithic/Confirm.qml:91`; `components/users/RemovePersonDialog.qml:109`).

## States, waiting and empty

| Situation | Hub words |
|---|---|
| claim with its proof | "Up to date" over "Ryoku %1, %2"; "Checking for updates"; "Updates can't be checked" (`components/Home.qml:769-786`) |
| something waits for you | "Needs you", then "An update is ready" (`Home.qml:807-816`) |
| attention after a choice | "Log out to finish switching to %1" / "Your current session stays as it is until you log out." (`registry/registry.js:148-150`) |
| working | a present participle naming the work: "Checking", "Installing", "Updating", "Looking for displays", "Reading your binder…" (`components/updates/RunView.qml:268`; `components/DesignShelf.qml:122`; `components/displays/ArrangeCanvas.qml:168`; `components/about/Binder.qml:376`) |
| nothing found | "No settings match “%1”", then "Nearby pages" (`components/SearchResults.qml:214, 223`) |
| empty place | "A place for your friends' cards" with a sentence on how it fills (`components/about/Binder.qml:434`); "No badge yet" (`components/about/BadgeTab.qml:69`) |
| a value not chosen | "Not set" (`components/default-apps/RoleControl.qml:19`) |
| not possible here | "Not available on this system", "Cannot run here" (`components/DesignShelf.qml:111-113`) |
| a value has moved off its default | "Changed" beside the row title (`Lithic/SettingRow.qml:169-224`) |

A failure that the reader can fix says how: "That copy is too small to read,
likely the chat's preview. Open the picture at full size first, or save it
and use Open a card." (`components/about/Binder.qml:146`).

## Kanji gloss

- **A gloss is the real Japanese word for the place**, never decoration:
  "Latin names the thing, the kanji is its seal; every gloss is the real
  word, never decoration" (`registry/registry.js:188-206`). The words live in
  one table there; take a new page's word from it or add the real word to it.
- **It follows the Latin name** and is set smaller and fainter: 外観 after
  "Appearance", 設定 after "Settings" ([design.md](design.md#kanji-gloss)).
- **It shows only while `UiPreferences.showJapaneseGloss` is on.** With the
  switch off the line reads in Latin alone and nothing else changes.
- **Category and page join with ・** in the masthead: 外観・窓飾
  (`components/sheet/Masthead.qml:77`).
- **The motto's gloss** 力と美のために stands under "For the sake of power
  and beauty." on Home (only when there is no custom tagline) and in the
  colophon.
- **Seals are not glosses.** 力 before the card's power and the role kanji in
  a badge hanko stay when glosses are off.

## Numbers, units and separators

**Counting the sheet** is mono and zero-padded, written by the code rather
than by the string: `00`, `01.3`, `P 02/39`, `01 / 08`, `SECTIONS 08`,
`CHANGED 03`, `51 DAYS · 21.3 H/DAY · STREAK 17`
(`registry/numbering.js:7-9`; `components/sheet/Activity.qml:259-270`).

**Live figures** are sans with tabular figures: Home's clock and readings,
slider values.

**Units** sit after the number with a space, short and lower case: "%1 h %2
min" (`components/Home.qml:312`), "%1 GB" (`Home.qml:303`), "MB/s"
(`Home.qml:209`), "Reverting in %1 s." Percentages take no space: "35%".
Relative times are words: "a minute ago", "%1 minutes ago", "an hour ago",
"a day ago" (`pages/Updates.qml:164-169`).

**Separators** in data lines:

| Separator | Where | Source |
|---|---|---|
| ` › ` | search crumbs, "Category › Page › Section" | `Hub.qml:551-554` |
| `  /  ` | the masthead's place | `components/sheet/Masthead.qml:28` |
| ` · ` | facts and summaries, the card | `components/sheet/Activity.qml:264-270` |
| `・` | between kanji | `Masthead.qml:77` |

These separators belong to running heads, crumbs and summaries, not to
sentences.

**Quoting** what the reader typed uses curly quotes: "No settings match
“%1”".

## Search words

Search finds the reader's word, not only ours: a synonyms table maps everyday
words to the Hub's, such as "transparency" to "opacity" and "hotkey" to
"shortcut" (`Hub.qml:538-549`). When a page uses a word people may not
search for, add the everyday word there.

## Translation

Every string goes through `I18n.tr` with `%1` placeholders filled by
`.replace` or `.arg`, as the Hub does throughout. Capitals and tracking stay
on the font, never in the string.
