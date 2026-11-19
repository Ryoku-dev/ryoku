# Hyperion profile progression and canvas

This feature is a Ryoku-wide **local** identity and achievement system. It is
not owned by hardware cards, not dependent on hardware rarity, and not an
online trading economy. Neuro's TCG is deliberately unaffected.

## Experience

Open **Ryoku Settings > Accounts > Achievements & badges**.

### Achievement Book

- Horizontally organised categories: Ryoku, Profile and Card Studio.
- Each category has its own folio of achievements and reward seals.
- Completion unlocks a **Claim** action. It does not immediately issue credits.
- Claiming issues one-time credits and a collectible badge sticker.
- Previously earned v1/v2 achievements are migrated as *claimed*, preserving
  their existing rewards and unlocks.
- A claimed badge can be featured on Home and placed as a canvas sticker.

### Profile Workshop

The Profile Workshop is on a second tab in the Achievements area.

- Purchasable backgrounds cover **the entire Home canvas**, not the trading card.
- A custom local picture may be chosen with Ryoku's existing image chooser. The
  picture remains at its original location. Moving or removing the source file
  will break its display. Only local image URLs are accepted.
- Stickers may be earned badges or decorative shop items. Up to 24 may be placed
  at once, with independently saved position, rotation and size.
- Home's **Decorate** control enables sticker drag mode. Editing is off by default.
- Fonts are separate unlocks, individually assigned to display name, hostname
  and tagline. The font choices use installed font families and safely fall
  back when a family is unavailable.
- The existing Card Studio still handles the displayed profile name and words,
  as well as the hardware card. A shortcut in the Workshop goes to those controls.
- Card palettes remain cosmetic and do not change rarity or TCG attributes.

## One-time local economy

| Achievement group | Maximum credits |
| --- | ---: |
| Ryoku exploration | 360 |
| Profile customisation | 320 |
| Existing Card Studio milestones | 600 |
| **Total** | **1280** |

| Shop category | Total cost |
| --- | ---: |
| Original card palettes | 600 |
| Decorative stickers | 245 |
| Canvas backgrounds | 210 |
| Fonts | 175 |
| **Total** | **1230** |

The full launch catalogue can be collected from finite, one-time achievements
without repeating trivial actions. Collecting everything leaves 50 credits.

This is intentionally a local, user-owned reward system. Editing progress data
is possible and expected for a user's own machine. Neither coins nor badges
should be trusted as online entitlements or multiplayer proofs. There are no
paid boosts, hardware-stat advantages, loot boxes, or background pollers.

## State

Version 3 state resides at
`$XDG_DATA_HOME/ryoku/progression.json`
(or `~/.local/share/ryoku/progression.json`).

The model stores:
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
session. Confirm claim, purchase, sticker placement/drag, decoration removal,
font changes, background selection, achievement page navigation, persistence
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
pictures, glyph stickers, fonts and editable card words. Remote/cloud sharing,
custom externally sourced sticker packs, rich animation packs and a global
theme marketplace are future work. Keep those modular and optional.
