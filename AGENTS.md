# Ryoku

A hand-built Linux distribution built on Arch Linux: a Hyprland or niri desktop
(the Ryoku shell), a guided installer, and the system definition that produces
both. This repository is the single source of truth. It deploys one way, into a
live system; live machines are never the source.

New here? Read these in order, then keep them open while you work:

- `docs/ryoku.md` what Ryoku is, who it is for, and how the parts fit.
- `docs/structure.md` the repo map: where everything lives and the one job it has.
- `docs/compositors.md` the window-manager seam: the provider contract, what
  each compositor can do, and how to add another.
- `docs/adding-a-window-manager.md` the walkthrough for putting Ryoku on a
  compositor it has never met.
- `docs/conventions.md` how code and configuration are written here.
- `docs/ui-ux.md` the desktop's look and motion, and how to build or replicate it.
- `docs/development.md` the workflow: deploy, test, the commit gates, and research.
- `docs/updates.md` how a change reaches a running machine, and the delivery contract.

## Cardinal rules

These are not negotiable. Most are enforced by the git hooks in `.githooks/`.

1. **Organization is the point.** Every file and every folder has exactly one
   purpose. Before adding anything, search the repo first; if it already exists,
   reuse it. Never keep two copies of the same thing. See `docs/structure.md`.

2. **A compositor's config is authored in that compositor's own language.**
   Hyprland is Lua modules under `ryoku/hyprland/`; niri is KDL under
   `ryoku/niri/`. One concern per file, and never a hand-written
   `hyprland.conf`. A standalone daemon or app that cannot read either keeps its
   own native config under its own directory (for example `hypridle.conf`,
   `matugen/config.toml`, `kitty.conf`); that is the only reason another config
   format exists. Nothing outside `ryoku/wm/` may name a compositor at all:
   ask capabilities, see `docs/compositors.md`.

3. **One concern per file.** A Lua module does one thing. A QML component is one
   component in one file. Split things out; do not pile unrelated logic together.

4. **The repo is the source of truth.** Deployment is one way: repo to
   `~/.config` (and a few system paths). Never hand-copy a live tweak back into
   the repo; change the repo and redeploy.

5. **Always pass the git hooks. Never bypass them** (`--no-verify` is forbidden).
   Commit subjects start with an area label
   `[global|installation|system|ryoku|docs|test|tooling|release]`, stay 72
   characters or fewer, and end without a period. No em-dash, no
   authorship/attribution trailers, no filler. For anything a user would notice,
   add a plain-language `Note: New|Fixed|Removed: ...` trailer; the release bot
   harvests it into the GitHub release notes. See `CONTRIBUTING.md`.

6. **Do not bury code in comments.** Code and config should read on their own.
   Comment the *why* when it is not obvious, never the *what*. Delete dead code
   instead of commenting it out. A file that is mostly comments is a smell.

7. **The desktop ships as signed packages.** The Go programs and the QML plugin
   build from source into the `[ryoku]` pacman repo (`release/packages/`); the
   installer adds that repo and installs `ryoku-desktop`, and AUR packages
   install in the post-install step. The live ISO prebuilds only the installer;
   the installed target has no build toolchain assumptions. See
   `docs/development.md`.

8. **Every change must reach users.** A dev box runs the checkout; users run
   packages, and `ryoku update` delivers them through `materialize` (the config)
   and `doctor` (drift). A user-facing config must be shipped by a package or
   seeded by the installer; user edits live in the `user_edits` overlay
   (`~/.config/ryoku/user_edits`), never in shipped files, and survive updates. A
   removed or renamed `shell.json` key needs a doctor
   reconciler, and work reaches users only once `main` fast-forwards. See
   `docs/updates.md`; `ryoku-dev-verify-delivery` enforces it.

## Top-level map

| Path | Purpose |
|---|---|
| `ryoku/` | The desktop: app configs, the window-manager seam and its per-compositor configs (Hyprland in Lua, niri in KDL), the shell UI, the lockscreen, brand assets. |
| `system/` | The machine definition: boot chain, hardware policy, package sets. |
| `installation/` | How a machine is built: the TUI, the backend installer, the ISO profile. |
| `release/` | Packaging: the desktop PKGBUILDs, the `[ryoku]` repo, the signing keyring. |
| `docs/` | These guides. |
| `.githooks/` | The commit/push gates every change must pass. |

Drill into each in `docs/structure.md`.

<!-- prowl-agent -->
## Prowl project context

This repo has a Prowl index of its files, symbols, and how they connect. For any
semantic or structural question -- where code is, what it does, who calls it, or
what a change touches -- **run the read-only prowl-agent CLI first**; do not grep
or read whole files just to locate things. Prowl reindexes what changed before
each query, so answers stay current and are cited to file:line, returned in one
call instead of a grep hit list you then open files to disambiguate.

| Question | First command |
|---|---|
| Map the repository | `prowl-agent overview` |
| Locate a feature or concept | `prowl-agent search "<question>"` |
| Locate a named symbol | `prowl-agent find <name>` |
| Read one symbol's source | `prowl-agent def <name-or-id>` |
| Inspect a file's structure | `prowl-agent outline <path>` |
| Trace who uses a symbol | `prowl-agent references <name-or-id>` |
| Size a change's blast radius | `prowl-agent impact <path>` |
| Inspect uncommitted work | `prowl-agent wip` / `prowl-agent changed` |
| Read a located line range | `prowl-agent peek <file:start-end>` |

Keep grep for exact literal or regex text and glob for filename patterns. CLI
output is token-lean TOON by default; add --format human|toon|json|markdown. If
your harness also wires Prowl as an MCP server, the same index is reachable
there; the CLI needs no server and is the first choice.
<!-- /prowl-agent -->

<!-- prowl-agent:map -->
## Prowl project map

Auto-generated from the Prowl index, refreshed on each `overview`/`init`. Prefer retrieving from Prowl (and reading the cited files) over grepping or relying on training memory; this is the current shape of the repo.

- size: 3349 files, 313749 symbols, 10246 edges (resolved 3303, external deps 5241, unresolved 1702)
- languages: go:1660 qml:865 bash:245 javascript:177 markdown:108 json:77 yaml:58 lua:43
- subsystems: ryoku/shell(631,qml) · ryoku/hub(65,qml) · ryoku/ui(52,qml) · ryoku/apps(50,qml) · ryoku/rashin(16,javascript) · ryoku/hyprland(15,lua) · ryoku/shell(15,css) · ryoku/lockscreen(12,qml)
- entrypoints: ryoku/shell/quickshell/shell/shell.qml · ryoku/shell/ryogami/wall-ui/qml/wallpaper/WallpaperSelector.qml · ryoku/hub/quickshell/pages/InputPage.qml · ryoku/hub/quickshell/pages/AnimationsPage.qml · ryoku/shell/quickshell/shell/modules/bar/MenuWidgetHost.qml · ryoku/hub/quickshell/pages/RecordingPage.qml · ryoku/hub/quickshell/pages/AddonsPage.qml · ryoku/hub/quickshell/pages/DisplaysPage.qml · (+191 more)
- central files (most depended-on): ryoku/lockscreen/qylock/themes/clockwork/orbital/i18n/I18n.qml · ryoku/ui/Singletons/Tokens.qml · ryoku/shell/quickshell/shell/modules/bar/barstyles/qsbar/Theme.qml · ryoku/shell/quickshell/shell/services/Perf.qml · ryoku/shell/ryogami/wall-ui/qml/Config.qml
- read these guides first: README.md · AGENTS.md · CONTRIBUTING.md · docs/development.md · docs/structure.md

Depth on demand: `prowl-agent find|def|outline|references <name>`, `search <text>`, `context search "<question>"`, `sketch <ui>`.
<!-- /prowl-agent:map -->
