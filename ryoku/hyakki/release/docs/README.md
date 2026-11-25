# Hyakki native distribution

Hyakki ships four native Linux programs: `hyakki`, `oni`, `hyakki-app` and
`oni-surface`. The app and surface remain physical trusted siblings of Oni.
The Go module identity remains `ryoku-rashin`; public package and runtime
paths use Hyakki and Oni.

Read [native-distribution.md](native-distribution.md) for build prerequisites,
the exact source/build/install commands, dormant optional service activation,
all native recipes and the isolated package lifecycle proof commands.
[licensing.md](licensing.md) explains the combined GPLv3-only distribution;
[oni-licensing.md](oni-licensing.md) records optional programs and asset grants.
The complete retained notices accompany the source and every package.

Build from `ryoku/hyakki/`, not a nested `app/` directory. Corresponding-source
archives preserve that same layout and the canonical shared Lithic/i18n paths.
No module cache, sibling checkout, provider account or live service activation
is required by the offline build/staging path.
