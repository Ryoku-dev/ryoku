# Hyperion profile progression and canvas

This feature is a Ryoku-wide **local** identity and achievement system. It is
not owned by hardware cards, not dependent on hardware rarity, and not an
online trading economy. Neuro's TCG is deliberately unaffected.

## Experience

Open Ryoku Settings **Home** and use the first two profile actions before
Binder: **Achievements** and **Points Shop**. These are separate destinations
and do not appear in the Accounts navigation tree.

### Achievement Book

- Horizontally organised categories: Ryoku, Profile, Card Studio, Collection,
  Mastery and Classified.
- Each category has its own folio of achievements and reward seals.
- 41 achievements use seven colour-coded rarities: common, uncommon, rare,
  epic, legendary, mythic and holographic. Each QML-drawn seal uses engraved
  radial lettering, concentric rims, a category sigil and technical ticks.
- Four classified achievements stay absent from the standard album until
  discovered. The final seal remains completely hidden until **all other**
  achievements, including other classified achievements, are completed.
- Holographic rewards get a motion-sensitive, iridescent finish. The renderer
  is procedural QML, not a generic PNG icon pack.
- Completion unlocks a **Claim** action. It does not immediately issue credits.
- Claiming issues one-time credits and a collectible badge sticker.
- Previously earned v1/v2 achievements are migrated as *claimed*, preserving
  their existing rewards and unlocks.
- A claimed achievement badge can be featured on Home or placed as a sticker.
  To choose one, open **Customise > Layout > Featured badge** or click the
  current badge on Home. The selection gallery lists only claimed badges.
  Claimed achievements also have a **Feature** action in the archive.

### Points Shop

Points Shop has its own Home entry and standalone page. It is not a tab
inside Achievements.

- Purchasable backgrounds cover **the entire Home canvas**, not the trading card.
- A custom local picture may be chosen with Ryoku's existing image chooser. The
  picture remains at its original location. Moving or removing the source file
  will break its display. Only local image URLs are accepted.
- Achievement badges retain their engraved circular seals. Shop stickers use
  **ten original SVG illustrations** in `assets/stickers/`, including a stone
  bust, lunar diagram, calligraphy sheet, archive clearance paper, floral
  sculpture and distinct flame and astronomical designs. These are not badge
  recolours.
- Stickers may be earned badges or decorative shop items. Up to 24 may be placed
  at once, with independently saved position, rotation and size.
- **Customise > Layout > Decorate profile** starts the canvas editor and
  returns to Home. The top-right action changes to **Finish** until editing
  is complete. Select any placed asset and drag its corner handle to resize
  directly on the profile, or scroll over it to scale in 14-pixel steps.
  The size range is 36 to 300 pixels. The shop also has size and rotation
  controls for every placed decoration.
- Home's identity text and controls explicitly sit above the sticker layer,
  regardless of sticker placement or user rotation. The featured badge also
  uses the archive seal renderer.
- Fonts are separate unlocks, individually assigned to display name, hostname
  and tagline. The font choices use installed font families and safely fall
  back when a family is unavailable.
- The existing Card Studio still handles the displayed profile name and words,
  as well as the hardware card. A shortcut in the Workshop goes to those controls.
- Card palettes remain cosmetic and do not change rarity or TCG attributes.

## One-time local economy

| Achievement group | Count | Maximum credits |
| --- | ---: | ---: |
| Ryoku | 7 | 810 |
| Profile | 10 | 1240 |
| Card Studio | 9 | 1010 |
| Collection | 10 | 1250 |
| Mastery | 1 | 230 |
| Classified | 4 | 1100 |
| **Total** | **41** | **5640** |

| Shop category | Count | Total cost |
| --- | ---: | ---: |
| Card palettes | 3 | 600 |
| Engraved sticker relics | 10 | 985 |
| Canvas backgrounds | 7 | 960 |
| Fonts | 6 | 590 |
| **Total** | **26** | **3135** |

Completing and claiming all achievements provides enough credits to collect
every launch cosmetic, leaving a surplus of 2505 credits for future catalogues.
This is a long-term maximum, not an up-front grant. Certain purchases and
creative acts are required for later achievements and secret combinations.
Each reward and purchase can occur only once. No daily farming or repeatable
currency is available.

This is intentionally a local, user-owned reward system. Editing progress data
is possible and expected for a user's own machine. Neither coins nor badges
should be trusted as online entitlements or multiplayer proofs. There are no
paid boosts, hardware-stat advantages, loot boxes, or background pollers.

## State

Version 3 state resides at
`$XDG_DATA_HOME/ryoku/progression.json`
(or `~/.local/share/ryoku/progression.json`).

The model stores:
- an internal secret-signal interaction counter
- earned, claimed and visited milestones
- purchased cosmetics and the active card palette
- one featured badge
- canvas background, optional local picture URL, fonts and sticker placements

The balance is **always derived** from claimed achievements minus purchased
cosmetics, never an editable balance field. Claims and purchases are idempotent.
Unknown IDs, unowned placements, invalid fonts and unsafe image URLs are
rejected during state normalisation. Old progress files are migrated in memory
without erasing the previous purchases or claimed rewards.

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
