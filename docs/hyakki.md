# Hyakki and Oni

Hyakki is Ryoku's optional native AI app and command line. Ask answers questions,
Agents work with tools under the session's permissions, and Index searches
project and system source. Teams, Dreams, Work and routines live in the same
app. Oni is its optional desktop companion, not a second shell assistant.

Hyakki is off by default. Installing it, migrating old data and opening a
settings page are not permission to enable screen capture, listening or input
control. The Ryoku vault remains available without Hyakki; see
[vault.md](vault.md).

## Open it and choose settings

- **Super+Alt+Space** opens or focuses the native app, `hyakki-app`, with desktop
  app id `dev.ryoku.Hyakki`. `hyakki open` opens it from a terminal.
- **Alt+Space** opens the shell's Hyakki bar: Quick Ask, web search and downloads.
  Quick Ask is a thin, read-only client. Web search and downloads remain usable
  without Hyakki; see [sidebars.md](sidebars.md#hyakki-bar).
- **Ryoku Settings > Apps > Hyakki** manages installation and lifecycle and
  links to settings in the native app. It does not duplicate Oni's settings.

The native app has Ask, Agents, Projects and Index destinations; Teams mode
contains Inbox, Teams, Dreams, Work, Routines, Vault and Activity. Memory,
Learning and Settings have their own destinations. The Teams credential Vault
is not the Ryoku machine-map vault.

Models, providers, optional features, Oni's character and Oni's permissions
belong in Hyakki's native Settings. Direct links include:

```text
hyakki open --tab settings --view providers
hyakki open --tab settings --view features
hyakki open --tab settings --view oni
hyakki open --tab settings --view oni-screen
hyakki open --tab settings --view oni-voice
hyakki open --tab settings --view oni-writing
hyakki open --tab settings --view oni-privacy
hyakki open --tab settings --view hands
hyakki open --tab settings --view parts
```

Hyakki uses the Lithic Go library: selected-row washes, resource tablets,
state marks, tabs and loading skeletons. It follows the desktop's four styles
and palette rather than keeping a separate style picker. See
[UI and UX](ui-ux.md) and [styles and colours](lithic/styles.md).

## Lifecycle and installation

Use the Hub's **Use Hyakki** control to install and turn it on. Turning that
control off stops it and removes the app while retaining conversations and
settings. For an already installed app, the CLI lifecycle is:

```text
hyakki status
hyakki status --json
hyakki on
hyakki off
```

`status` reads settings and an already-running daemon without starting it.
`off` keeps the package and saved data. On systemd it stops and disables the
native daemon, socket, routines and usage units. `on` enables the native
socket and usage timer; it does not grant Oni capabilities. Runit counterparts
ship dormant; the lifecycle command prints the supervision steps needed there.
Do not turn optional services into unconditional login services.

The installer choice is **Hyakki**, default No. The shell installer and current
account-provisioning commands use `--hyakki`; unattended installation uses
`RYOKU_HYAKKI` (default `0`).

The desktop package is `hyakki`, replacing and conflicting with `ryoku-rashin`.
Its desktop integration is in `release/packages/hyakki/`. The app's native
packaging under `ryoku/hyakki/release/packages/` includes Arch pacman, Fedora
RPM and Void XBPS recipes, plus a Nix derivation. These install the native
binaries and resources; there are no predecessor-named runtime launchers.
Installed users receive packages, not a requirement to build Go or Gio locally.

The native program module lives in `ryoku/hyakki/`. Its window snapshots and
allowlisted window operations are canonical Go submodules under
`ryoku/wm/oniwindow/` and `ryoku/wm/reflex/`. Build and export from the complete
monorepo layout: distributable source includes these inputs and their tests,
alongside the shared Lithic and translation modules. Testing the native main
module alone does not run the two adapter modules' test suites.

Native systemd user units are `oni.service`, `oni.socket`,
`oni-routines.service`, `oni-routines.timer`, `hyakki-usage.service` and
`hyakki-usage.timer`. The runit services are `oni`, `oni-routines` and
`hyakki-usage`.

## Migrating an existing desktop

Install the approved Hyakki package, save input in the old Rashin window,
finish its tasks, downloads and model work, and quit it normally. Before
retiring legacy activation or development binaries, run as the desktop session
user, **never root**, without sudo or pkexec:

```text
ryoku-host hyakki migrate
```

Doctor and the Hub use this host coordinator. It inspects the old supervisor
and ownership, refuses an active GUI, workers or an uninspectable supervisor,
quiesces legacy activation, then calls native `hyakki migrate`. If native
migration fails it restores legacy activation and reports the error. It does
not kill workers or replace the user's original consent with a fresh opt-in.
Finish the reported work or repair the supervisor normally before retrying.

Inherited global legacy activation is suppressed only for the migrating user,
not disabled for other accounts. Its original local unit definition is retained
in a private `.hyakki-retired-units-*` directory beside the user units. A failed
migration restores that definition and the original local activation links;
after success its per-user mask stays so global links cannot restart it.

Native migration relocates supported old app configuration, state, cache and
app-owned data into Hyakki's namespace. The old
`$XDG_CONFIG_HOME/ryoku/rashin.json` and app data under the corresponding
`ryoku/rashin` state, cache and data paths are migration inputs, not active
configuration locations or command aliases. The shared machine vault is not
moved as an app data tree. Migration creates private backups under
`$XDG_STATE_HOME/hyakki-migration-backups/` and refuses conflicting destination
data rather than overwriting it.

For project-local configuration, `hyakki migrate --project /absolute/project`
imports supported `.rashin` configuration into `.hyakki`. For recovery,
`hyakki migrate --restore /absolute/backup` restores the original legacy
objects from a migration backup while retaining current Hyakki data. Close
active owners first; recovery refuses conflicting newer legacy content. These
native data operations do not replace the host coordinator for desktop service
retirement. The older Rashin 1 machine-vault migration is a separate core
operation described in [vault.md](vault.md#migration-from-rashin-1).

## Terminal Ask and source search

```text
hyakki ask why is my laptop not suspending
hyakki ask -c what about the previous error
hyakki ask --last
hyakki ask --buffer -- replace spaces in these filenames
hyakki sessions
hyakki search "where is the dock state written?"
hyakki find <symbol>
hyakki def <symbol>
hyakki outline <path>
hyakki references <symbol>
```

Terminal Ask stays read-only. `-c` continues the last terminal Ask session;
`--last` prints its last answer, and `--buffer` prints proposed commands for
review, not execution. Fish, Bash and Zsh's interactive wrappers turn
`hyakki <words>` into a buffer request; explicit subcommands pass through to
the CLI. Alt+R proposes a replacement for the current prompt. Nothing executes
until the user presses Enter. The hooks supply `HYAKKI_LAST_CMD` and
`HYAKKI_LAST_STATUS` with terminal context.

Use `hyakki --help` for the current command list. Index management is under
`hyakki index`; the cited `search`, `find`, `def`, `outline`, `references` and
`peek` commands inspect registered source. None of these is a reason to enable
Hyakki on behalf of someone who left it off. The shipped vault source map is
available independently.

## Oni, privacy and optional interfaces

Turn Oni and its individual capabilities on in the native app. A character
changes presentation, not permissions. `hyakki oni status --json` reports its
capabilities and reasons a feature is unavailable. `hyakki oni show` displays
an already-enabled Oni; `hyakki oni hide` hides it without turning it off.

`hyakki oni pick --text "What does this do?"` selects a window with one click
at the chosen spot and asks about it. It does not freeze the desktop for a
dragged box. Screen questions and guides disclose the capture path and model
destination before sending under the configured disclosure policy. Exact
window capture and a screen-region crop have different privacy properties:
a crop can include content drawn over the window's region, and the disclosure
says so. Do not treat it as exact window capture.

Voice, dictation, writing and Hands have their own native controls. Hands
requires the user's authorization for its scope; risky actions are handed back
to the user. Enabling Hyakki or changing a character does not grant broader
access. Remote model use can send the prompt and explicitly supplied context
off this computer; choose models and review disclosure in native Settings.
Optional local models and runtimes are downloaded on request.

The local model API is optional (`hyakki api on`, `hyakki api off`); it is not
a web dashboard. The stdio MCP interface, `hyakki mcp`, requires its feature to
be enabled and the client approved in Settings. Do not enable either interface
just to open the app or use the Ryoku vault.

The shell's Theia/Sight readers, pages and gestures are retired. Oni is not a
compatibility version of those gestures. Clipboard OCR uses its own settings;
Ryoku Voice typing and ryoshot screenshots remain independent.

## Existing local models

Settings > Providers > Local models can discover a user-chosen `llama-server`,
the managed runtime, builds on the daemon and login-shell PATH, and common
source-build and user-install locations. Capability probes determine supported
flags; a build without multi-model support serves one selected model.
The user can choose a discovered binary without copying or changing it.

The "Found on this computer" GGUF search runs only when the page requests it,
not at login or in the background. Additional folders are opt-in. Using a
found model registers the existing file in place; split sets count as one
model, and a matching adjacent mmproj is paired. Forgetting a found model
removes its manifest entry, never the user's file. Removing a model downloaded
by Hyakki still removes its managed copy.

The model process listens on a random loopback port with a fresh per-start
key. It stops after the configured idle interval and dies with Oni. Discovery
does not download a model or start inference while Hyakki is off.

## Source and data map

The source is `ryoku/hyakki/`. Its Go module name remains `ryoku-rashin` for
internal imports; that name is not a public command alias.

| Physical source sibling | Installed program | Responsibility |
|---|---|---|
| `cmd/hyakki/` | `hyakki` | CLI, lifecycle, Ask, index and native settings links |
| `cmd/oni/` | `oni` | Native engine, sessions, tools and Oni capabilities |
| `cmd/hyakki-app/` | `hyakki-app` | Gio app |
| `cmd/oni-surface/` | `oni-surface` | Optional desktop companion surface |

`internal/protocol/` defines socket methods and events, `internal/client/`
connects clients, `internal/engine/` owns native sessions,
`internal/localmodels/` handles local models, `internal/index/` owns source
search, and `internal/oni/` defines the companion's capabilities.

| Location | Owns |
|---|---|
| `$XDG_CONFIG_HOME/hyakki/hyakki.json` | App settings |
| `$XDG_STATE_HOME/hyakki/` | Sessions and persistent app state |
| `$XDG_DATA_HOME/hyakki/` | Models, attachments and app-owned data |
| `$XDG_CACHE_HOME/hyakki/` | Rebuildable app cache |
| `$XDG_RUNTIME_DIR/hyakki/oni.sock` | Native Unix socket; clients may override it with `HYAKKI_SOCKET` |
| `$XDG_DATA_HOME/ryoku/vault/` | Ryoku-core machine map, independent of Hyakki |

Without XDG overrides these use the usual `~/.config`, `~/.local/state`,
`~/.local/share` and `~/.cache` bases. Refresh the machine map with
`ryoku vault refresh`, whether Hyakki is on, off or absent.
