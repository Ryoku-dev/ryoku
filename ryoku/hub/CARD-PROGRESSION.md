# Ryoku Profile Progression v4

Ryoku's profile, achievements, and Points Shop are global to Ryoku, not tied
to a hardware card. None of these rewards changes card scoring or Neuro's TCG.

## Entry points and interface

Home includes **Achievements** and **Points Shop** before Binder.
Achievements are organised into Ryoku, Profile, Card Studio, Collection,
Mastery and Classified. The latter shows only *discovered* achievements.
The header reports the true completed/total catalogue, including hidden
requirements, so 100% cannot appear before completion.

The user can feature any claimed badge using Customise > Layout > Featured
badge. A claimed badge becomes an engraved seal sticker. Decorative shop
stickers are genuine Ryoku art, not copies of achievement badges.

All shop assets are owned locally and never give competitive advantages.
Customise > Layout > Decorate allows drag, resize and rotation with the
profile text above decorative elements.

## Rebalanced v4 achievements

| Collection | Achievements | Total award potential |
| --- | ---: | ---: |
| Ryoku | 11 | 4920 |
| Profile | 14 | 2690 |
| Card Studio | 11 | 1580 |
| Collection | 15 | 3060 |
| Mastery | 6 | 2000 |
| Classified | 15 | 4720 |
| **Total** | **72** | **18970** |

Four new Legendary Mastery awards cover Ryoku, Profile, Card Studio and
Collection individually. **Living Archive** is the new Holographic overall
Mastery badge. The classified **Final Seal** is reserved for completion
of every other achievement, including the hidden ones.

The original alpha achievement cohort remains independently eligible for
**First Archive** and **First Seal**. Both keep their previous rewards
(230 and 420 credits respectively). This is deliberate: people who legitimately completed v3
retain those badges, but they do not receive the new v4 100% automatically.

The new milestones demand distinct achievements, page categories and unique
sticker designs, rather than copies of one sticker. Long-term achievements
track *different local calendar days*. Important milestones require seven,
fourteen and thirty days. Opening Settings 100 times in a day still counts
as **one day**. Visiting the Points Shop and changing the profile are tracked
as separate once-per-day activities. No background polling is needed.

A user can earn an achievement once, **claim** it once and spend its credits
once. Hidden achievements only enter the book when genuinely discovered.
The local save is user-owned, not anti-cheat-secure, and no online rewards
or achievements can be trusted from this file alone.

## Original Ryoku artwork

Shop stickers reuse ten real images from upstream
`ryoku/assets/ryodecors/`, copied into the self-contained Hub asset directory.
The old synthetic SVG pack has been removed. The profile backgrounds are
illustrated from the same original high-quality image assets, with curated
Lithic treatments rather than generic icons and geometry-only previews.

| Shop group | Items | Total catalogue cost |
| --- | ---: | ---: |
| Card palettes | 3 | 1290 |
| Original Ryoku art stickers | 10 | 2118 |
| Full-profile illustrated backgrounds | 13 | 6686 |
| Fonts | 6 | 1269 |
| **Total** | **32** | **11363** |

Prices are higher for newly purchased items. New time-gated rewards ensure
the shop can ultimately be collected without repeatable farming. Purchasing
every item is **not expected in the first session**.

## Wallet compatibility

The save is at `$XDG_DATA_HOME/ryoku/progression.json` (typically
`~/.local/share/ryoku/progression.json`).

Version 4 adds purchase receipts, unique-day counters, histories of real
background/featured badge choices, and a maximum sticker count.
Previously earned and claimed achievements and owned items are preserved.
For each pre-v4 owned item, the migration records its **original paid price**.
Changing the current shop price does *not* deduct extra credits from an
existing purchaser's wallet.

The balance is always derived from claimed rewards minus each item's recorded
paid price. Badges, original artwork choices, sticker positions and font
assignments survive migration. No external service or telemetry is used.

## Settings Keepsake

The existing Settings Easter egg is ported from the earlier Ryoku Hub into
Hyperion. Its original window-local key sequence, story, contributor portraits,
and escape-to-close behaviour are preserved. It never intercepts typing in
text fields and is omitted from normal navigation. Revealing it triggers one
hidden achievement. No bundled soundtrack or extra audio process is launched.

A separate future achievement mentioned by the user has **not** been invented.
A crash-triggered badge is intentionally deferred until a reliably verified
unexpected crash event is available. Ordinary restarts must not be mistaken
for a crash, and users should never be encouraged to crash Ryoku on purpose.

## Collectible Archive and the Black Prism reward

The Achievement Book uses `AchievementCard.qml`, with the original
`ArchiveSeal` artwork on each card. Rarity affects the engraved edge,
plate material, ornament and progress accent. Mastery gets a broader-cut
engraved enclosure and Classified entries remain anonymous until earned.
Category tabs scroll horizontally on compact windows, and turning a page
resets the vertical scroll to the top.

When an **earned** badge is manually claimed, `AchievementReveal.qml`
provides a short, modal seal-unfurl and credit acknowledgement.
Escape, the return button or the dark scrim dismiss the reveal.
Animations are suppressed when `Lithic.reduceMotion` is set.

Every objective with a measurable counter includes a live progress
bar, with the correct numerator and requirement. Complex objectives
retain their full description rather than inventing percentages.

**Black Prism** is an original animated profile background earned for
owning all **32** regular shop cosmetics, the same event as Complete
Collection. It is not one of the purchasable items, has no points price,
cannot be acquired through the shop API, and is never required for its
own unlock condition. The reward remains secret until earned, then
appears in the Points Shop with a free Equip action.

The rendering is pure QML and static Canvas geometry with a slow
rotating prismatic accent. Nothing runs in the background while Home
is closed, and reduced motion leaves a still, readable scene. The shop's
reward preview is always static to avoid offscreen animations.
Old saves with all items already owned automatically gain the reward.
Unfinished collections cannot retain an unequipped bonus via save edits.

Balance refinements avoid paying twice for identical work:

- Behind Every Door requires both 40 distinct Settings pages and
  eight distinct Settings areas, rather than duplicating Cartographer.
- Variety Show requires two distinct earned badge seals **and** three
  different purchased shop art stickers simultaneously, rather than
  duplicating Mosaic's five designs.
- The Long Game secret requires 21 distinct days, instead of being an
  identical 14-day copy of Habit Formed.

These requirements are additive and possible using the existing
Settings pages and cosmetic catalogue. Previously earned badges
remain earned and claimed after migration, without clawing back rewards.
The 30-distinct-day milestone still sets the earliest genuine endgame
completion, and the progression system stays offline and cosmetic-only.

## Validation and rollout

Use:

`node --test ryoku/hub/quickshell/registry/*.test.mjs ryoku/hub/quickshell/KeepsakeSequence.test.mjs`

Parse changed QML with `qmlformat`. The private test session should be
started with `qs -p <new-hub-directory>` and an isolated `XDG_DATA_HOME`
to protect actual user purchases and achievement state.

Do not automatically overwrite the user's existing live NixOS Hub or change
its system generation during development. This branch follows the prior
feature/hyperion-card-progression work and needs a separate NixOS port and
package validation before distribution.

## Card rarity scoring: Steam October 2026

These nine **card** rarities are separate from the seven achievement-badge
rarities. Signed card records and TCG battle power remain unchanged.

Steam's October 2026 survey separately reports RTX 3060, 16 GB RAM,
1920x1080 and eight physical CPU cores as modal components. It does not
report one exact most-common complete PC or a CPU model/refresh rate.
For reproducible calibration, we use an eight-core, sixteen-thread Ryzen
7 5700X and 60 Hz as explicitly chosen assumptions, NOT Steam statistics.

**Reference score: 52 = Epic**, the middle of nine tiers.

| Rarity | Minimum |
| --- | ---: |
| Common | 0 |
| Uncommon | 18 |
| Rare | 27 |
| Ultra Rare | 38 |
| Epic | 49 |
| Legendary | 60 |
| Mythical | 72 |
| Holographic | 85 |
| Master | 97 |

Scoring has four independent components:

- GPU (up to 44): model/generation plus laptop, Ti, SUPER and XT/XTX variants.
  A 5060 outranks a 3060; suffix alone does not set performance.
- CPU (up to 30): thread count and conservative, known model-generation
  credit for Intel Core/Core Ultra and Ryzen.
- RAM (up to 14): capacity bands from 8 GB to 128 GB; diminishing returns.
- Displays (up to 25): ALL active physical outputs are considered. The
  highest-resolution screen defines the main component; each of the next
  five screens earns a diminishing score based on its own pixels and refresh.
  Extra-display points are capped at eight. Displays beyond six are counted
  but contribute no further bonus, preventing screen farming. Connector IDs
  are deduplicated. The Share tab shows each screen's actual contribution.

The signed shared-card gpuClass remains 0..5; when a compatible actual GPU
name is provided, the receiving scorer can use a finer estimate. Older
records without names/CPU models use a conservative fallback. We do not
invent missing historical data or alter the TCG battle HP.

### Nine stock card finishes

All finishes derive from **existing Lithic colours and shaders**, not newly
invented colour palettes:

- Common: existing bone and graphite.
- Uncommon: bone lightly mixed with jade.
- Rare: original selection-metal tone.
- Ultra Rare: selection metal with the original gold accent.
- Epic: selection metal and lapis.
- Legendary: original gilded gold.
- Mythical: original jasper accent, richer face foil.
- Holographic: original multi-mineral prismatic shader.
- Master: existing prismatic diamond with a second gold/ivory carved rim.

Official designs and purchased card palettes keep their own colours and do
not change the score. This is an ordinal collectible classification, not a
hardware benchmark or a real-world market value; periodically recalibrate
when hardware usage changes.

## Validation

Run:
`node --test ryoku/hub/quickshell/registry/*.test.mjs`

Use `qmlformat <file.qml> >/dev/null` for syntax checks. Actual Qt imports
and interactive rendering must also be exercised in a running Quickshell Hub
session. Confirm claim, purchase, inline seal selection, navigation to Achievements
and Points Shop from Home, vector sticker loading, direct resize/drag/wheel
scaling, decoration removal, font changes, background selection, persistence
after Settings restart, and preview rollback.

The user's current live NixOS test is a backed-up overlay of the materialised
`~/.config/quickshell/hub` files, **not** a NixOS rebuild. The rollback
manifest and script live under
`~/.local/state/ryoku/hyperion-card-preview/`.

## Future integration

Gaming and cross-app achievements require meaningful application events or
server-verified contributions. Don't infer them from opening a menu. A public
profile and social inventory also need authentication and an authoritative
server-side entitlement ledger. These systems are **not implemented here**.

The profile canvas currently has curated backgrounds, user-picked local
pictures, procedural seal stickers, fonts and editable card words. Remote/cloud sharing,
custom externally sourced sticker packs, rich animation packs and a global
theme marketplace are future work. Keep those modular and optional.
