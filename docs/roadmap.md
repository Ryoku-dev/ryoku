# Ryoku Roadmap

This roadmap is about how the desktop *feels*: how fast it gets to first frame
after login, how quiet it idles, and whether interactions stay smooth while the
shell is doing real work. It supersedes the earlier feature-derived roadmap. The
features list still lives in the docs that describe each surface; this file is
for the performance work and the people who want to do it.

One rule governs every item in here: **it lands with a number or it does not
land.** I'd rather close a ticket with a measurement than a theory. The shell
has already been tuned this way once (the idle-cost pass that killed the
always-on analysers), so the loop is proven: measure, change, measure again,
keep the change only if the number moved.

Roughly 190k lines of QML across 870 files stand between login and a usable
desktop. That is not a problem by itself. It is a budget. This roadmap spends
that budget deliberately, cheapest wins first.

## Where we stand before touching anything

I refuse to guess. The first deliverable is a baseline, and it is also the
cheapest item in this file.

**What we need to know, on a clean reference session, for every compositor:**

- login to first painted surface
- per-surface frame cost while idling, while animating, and while playing audio
- resident memory of every QML process and daemon at login
- idle CPU and power draw, matching the numbers in `docs/power.md`

**How:** reuse what exists. `Perf.qml` already owns the shell's eye-candy
policy; `ryoku-compositor-resource-compare` measures footprint across
compositors; the dev-deploy and scripted `grim` workflow already exercises live
surfaces. What is missing is a single repeatable script that produces the whole
table and a place the numbers live where reviewers can see them.

**Done means:**

- one committed script that prints the full baseline table on a clean session
- baseline recorded on the reference hardware for every compositor
- the numbers are visible in the repo, not in someone's notes

This phase has no hard part and no design decisions. It is also the prerequisite
for every other phase, because nothing below can claim a win without it.

**Where it is:** the script exists (`bin/ryoku-dev-perf-baseline`) and the first
baseline, with the October 2026 before and after, lives in
`docs/performance.md`. It was recorded on niri only; the Hyprland row is still
to take on the reference hardware.

## Phase 1: stop paying the parse tax

Easiest real change in this file, and likely the biggest win per hour spent.

Today the gates lint QML (`qmllint`, via `bin/ryoku-dev-lint-qml`) but nothing
**compiles** it. Every process that loads a surface parses and compiles its QML
at runtime, at login, separately: the shell, the hub, and each of the smaller
surfaces each pay the same tax on the same 190k lines.

**Change:** verify the compiled-cache path Quickshell supports, build the
compiled QML into the package, ship the cache, and make the publish gate
compile QML the same way it lints it today.

**Done means:**

- cold login-to-first-surface is measurably faster than the baseline
- the compiled cache ships inside the package and is exercised by the publish
  gate, so a shipped box really gets it
- the dev gate and the publish gate agree on how QML is compiled, so a config
  that builds locally cannot pass CI while failing the packaged build

This is a packaging and CI change. It touches no QML semantics. It is the one
item on this page I would take even if users never notice a single frame.

## Phase 2: build only what you see

The tree already leans on `Loader` for deferred construction, but nothing audits
who actually constructs at startup. Every singleton that initializes eagerly in
the shell, the hub, and the peripheral surfaces is a bill paid at login that
should be paid on first use.

**Change:** audit startup construction surface by surface; move everything that
is not visible to the user at login behind a loader or a lazy singleton;
preserve the load-on-open timings so a lazy surface does not feel slower when
it is finally opened.

**Done means:**

- the startup construction log for each resident process fits on one screen
- no user-facing surface constructs until it is requested
- opening a lazily loaded surface shows no regression against today's timing

## Phase 3: make animation cost what it shows

The shell animates a lot, and most of it stops when it should (that is
`Perf.qml`'s job and it is good at it). What is left is the visible-and-audible
case: waveform and particle surfaces ticking on JavaScript `Timer`s at
25–40fps, and view models rebuilt from scratch on change.

**Change:** convert hot surfaces from timer-driven ticks to frame-driven
updates; make the hot per-frame path allocate nothing; sweep the binding-heavy
surfaces (the bar carries over a thousand `Behavior` and `NumberAnimation`
declarations) for storms that recompute more than they draw. Extend `Perf.qml`'s
policy, never bypass it: the idle, battery, and game-mode rules stay the single
source of truth.

**Done means:**

- a profiler run on the bar shows no per-frame JavaScript allocation
- idle bar CPU is effectively zero on a default session
- every surface animates only when heard and seen, and only at the rate it needs

## Phase 4: fewer resident processes

This one is hard because it is deliberate, not because it is tricky. A login
today brings up several QML processes, each with its own QML runtime, startup
parse, and memory: the shell, the hub, the screenshot surface, the pin surface,
the wallpaper UI, plus the Go daemons.

**Change:** cost every resident process in the baseline table, then decide per
process whether it stays resident or becomes a spawn-on-demand surface inside
the shell. Earlier decisions kept some of these separate deliberately for crash
isolation and safety; folding any of them back in must preserve what the
isolation bought. A surface that spawns on demand keeps that property while
losing the always-on cost. This is a case for a maintainer conversation, not a
lone PR.

**Done means:**

- the baseline table lists every resident process with its cost
- each fold keeps feature parity and states, in the commit, what isolation
  property it preserves and how
- net resident process count and login memory are down, measured

## Phase 5: the hardest surface

The wallpaper engine is the largest per-frame consumer in the desktop: video
decode, shader surfaces, and cache management in one place. It is also the most
visible surface when it misbehaves.

**Change:** move decode work off the UI thread, bound the shader surfaces,
cost the cache policy. If the baseline says some of this is already fine, the
phase shrinks to what the numbers justify; the numbers decide, not this file.

**Done means:**

- decode never blocks the UI thread, measured under load
- the worst-case wallpaper per-frame cost is recorded and under the target set
  after the baseline

## Phase 6: keep it from creeping back

A regression gate is the difference between a tuning pass and a discipline. The
repo already gates shell IPC parity in CI; performance deserves the same
treatment.

**Change:** a CI check on the reference profile that fails when idle CPU, login
time, or the resident process count creeps past the recorded baseline. Nominate
a perf guardian: the person who owns the numbers, reviews the per-frame
changes, and turns every "the shell feels heavy" report into a measurement
first.

**Done means:**

- the gate runs on every change touching the shell and is green on main
- the first real regression caught by it is fixed with the number it produced,
  as proof the gate works

## For contributors

The same ordering works for people. Pick the row that matches where you are;
every row is finished work, not a training exercise.

**First patch (no shell experience needed)**

- run the baseline script, file the results, and open a ticket for anything
  that looks off; reporting a regression with a number is already a
  contribution
- audit `Loader` usage and startup construction; the audit is the deliverable,
  one list per surface
- `qmllint` cleanups on files you have read that pass the existing gates

**Building confidence (know the shell a little)**

- Phase 1: compile QML in the package and the gates; you will learn the whole
  delivery pipeline and it is all packaging, none of it QML semantics
- Phase 3, one surface: convert one hot timer-driven animation to frame-driven
  and prove it with the profiler
- Phase 2, one surface: defer one eagerly constructed surface and keep the
  open timing honest

**Hard hat (know the shell and the decisions behind it)**

- Phase 4: process consolidation, paired with a maintainer, because it revises
  earlier architecture decisions
- Phase 5: the wallpaper engine threading work
- Phase 6: build the perf gate and volunteer as its first guardian

If you are new here, start with the first row and the baseline. Everything else
in this file will still be waiting with the numbers attached.