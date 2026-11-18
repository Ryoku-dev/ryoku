# Hyperion card rarity and progression

This change adds an offline, cosmetic-only progression layer to the existing
Hyperion card studio. It does not define battle rules or online account rewards.

## Hardware rarity

- Local graphics scoring uses performance bands keyed by GPU family, generation,
  tier and supported mobile variants. The legacy five-level GPU class used in
  card codes and Stand attributes is deliberately unchanged.
- The CPU score combines threads with a conservative generation bonus for
  recognised Intel Core and AMD Ryzen models. Unknown models score from threads.
- Memory scores saturate to avoid making very large RAM capacities dominate.
- Display scoring uses the active output list provided by `ryoku-hub outputs`.
  The highest-resolution active screen is the primary screen for scoring.
  One or two additional active screens add a small capped bonus.
- The card editor shows the four category contributions and next rarity threshold.
- The old shared card protocol is intentionally unchanged. Shared cards have
  neither the full CPU model nor the complete output list, so their ratings may
  remain coarser than those of the machine's live card. Updating the signed
  card record and optical card code requires a coordinated protocol migration.

The rarity algorithm is a rough hardware classification, not a benchmark result.
Adjust its model table only with test coverage and measured comparisons.

## Offline progression

| Achievement | Credits |
| --- | ---: |
| First Contact | 50 |
| Under the Hood | 80 |
| Signature | 100 |
| A New Face | 100 |
| Art Director | 120 |
| Showtime | 150 |

All six milestones award exactly once, for a total of 600 credits.

| Stock card palette | Cost |
| --- | ---: |
| Midnight | 150 |
| Ember | 200 |
| Jade | 250 |

The catalogue totals 600 credits. There is no grind, daily reset, repeat
reward, random loot, paid currency or hardware-dependent progression.

The user-owned ledger lives at
`$XDG_DATA_HOME/ryoku/progression.json` or
`~/.local/share/ryoku/progression.json`. Balance is *derived* from
known earned milestones minus known purchased cosmetics. An edited balance
field is ignored, and duplicate rewards or purchases are rejected.

This storage is deliberately not tamper-proof. Users fully control local data.
Do not represent its credits, ownership or achievements as cryptographic proof,
allow trading, or confer server-side entitlements. Any later online economy
requires independent server authority and a separate security design.

The progression palettes affect stock cards locally. An approved or custom
card design retains its own authored palette. The existing signed team badge
programme is separate and unchanged.

## Validation

Run `node --test ryoku/hub/quickshell/registry/*.test.mjs` and exercise the
card studio interactively with one and multiple screens, profile customisation,
one-time exports, file persistence, palette selection and restart.
