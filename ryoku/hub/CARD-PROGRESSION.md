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
- A claimed badge can be featured on Home and placed as a canvas sticker.
  **Change featured seal** is immediately accessible from Home, with a
  horizontal gallery of claimed badges. A **Feature** control also appears
  on each claimed achievement in the book.

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
- Home's **Decorate** control enables sticker drag mode. Editing is off by
  default. Select any placed asset and drag its corner handle to resize
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

## Hardware card scoring

- Generation-aware performance bands replace suffix-only GPU rankings.
- CPU threads plus conservative model-generation credit replace thread count
  alone.
- The highest-resolution active monitor is selected. Additional active outputs
  receive a modest, capped bonus.
- Compact shared cards still carry legacy GPU class, CPU thread count and one
  display summary. Do not silently change their signed protocol, or Neuro's TCG
  battle attributes, in this feature branch.

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
