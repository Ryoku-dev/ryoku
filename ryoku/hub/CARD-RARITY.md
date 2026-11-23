# Card rarity scoring: Steam October 2026

Signed card records and TCG battle power remain unchanged.

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

## Nine stock card finishes

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

Official designs keep their own colours and do not change the score. This is
an ordinal collectible classification, not a hardware benchmark or a
real-world market value; periodically recalibrate when hardware usage changes.

The code is `quickshell/registry/rarity.js`, tested by
`quickshell/registry/rarity*.test.mjs`.
