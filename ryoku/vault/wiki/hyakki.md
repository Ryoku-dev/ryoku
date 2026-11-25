# Hyakki and Oni

Hyakki is Ryoku's optional native AI app. It is off by default. The Ryoku
vault, wiki, skill and source map remain available when it is off or absent.

## Open and configure

- **Super+Alt+Space** opens or focuses `hyakki-app`, desktop app id
  `dev.ryoku.Hyakki`. From a terminal, use `hyakki open`.
- **Alt+Space** opens the shell's Hyakki bar. Quick Ask is a read-only client;
  web search and downloads also work without Hyakki.
- **Ryoku Settings > Apps > Hyakki** installs and turns it on, or stops and
  removes the app while retaining settings and conversations. The page links
  to native settings rather than keeping another Oni settings form.

Ask, Agents, Projects and Index live in the native app. Teams mode includes
Inbox, Teams, Dreams, Work, Routines, Vault and Activity; Memory, Learning and
Settings have their own destinations. Choose models, optional features and
Oni capabilities in the app. Its styles and colours follow the desktop.

```text
hyakki status
hyakki status --json
hyakki on
hyakki off
hyakki open --tab settings --view oni
hyakki open --tab settings --view oni-screen
hyakki open --tab settings --view oni-privacy
```

`status` does not start the daemon. `hyakki off` stops it without uninstalling
the package or deleting saved data. Native systemd user units are
`oni.service/socket`, `oni-routines.service/timer` and
`hyakki-usage.service/timer`; runit counterparts ship dormant. Keep lifecycle
under the owning command, not unconditional login activation.

## Ask and source search

```text
hyakki ask "why did the shell stop?"
hyakki ask -c "what about the previous error?"
hyakki ask --last
hyakki sessions
hyakki search "where is the dock state written?"
hyakki find <symbol>
hyakki def <symbol>
hyakki outline <path>
hyakki references <symbol>
```

Terminal Ask is read-only. Fish, Bash and Zsh's interactive `hyakki <words>`
wrapper and Alt+R put proposed commands on the prompt; they do not execute
them. Review the buffer before pressing Enter. Use `hyakki --help` for the
full command surface. Do not turn it on just to inspect the desktop: start
from `desktop.md`, `logs.md` and `ryoku-repo.md` in this vault instead.

## Oni and privacy

Oni is Hyakki's optional desktop companion. Turn it and individual capabilities
on in native Settings; changing its character grants no permission. Native
screen, voice, writing and Hands controls stay there. Opening a settings page
does not enable capture, listening or input control.

`hyakki oni show` displays an already-enabled Oni; `hyakki oni hide` hides it.
`hyakki oni pick --text "What does this do?"` chooses a window with one click
at the selected spot. It is not a frozen-screen drag-box selector. Screen asks
and guides disclose what is captured and the model destination under the
configured disclosure policy. A screen-region crop can include content drawn
over the window; read its disclosure instead of assuming exact window capture.
Hands requires user authorization for its scope and hands risky actions back
to the user.

Remote models can receive the prompt and supplied context. Choose models and
review privacy in the native app. Optional local models and runtimes download
on request. The local API and stdio MCP interface are separate opt-ins; MCP
also requires an approved client. Neither is needed to read the Ryoku vault.
Clipboard OCR, Ryoku Voice typing and ryoshot screenshots are independent of
Oni. The old Theia/Sight shell pages and gestures are retired.

## Existing installations and data

The native package is `hyakki`, replacing and conflicting with `ryoku-rashin`.
Before retiring old activation, install Hyakki, save and quit the old Rashin
GUI and finish its work normally. Run as the desktop user, never with sudo:

```text
ryoku-host hyakki migrate
```

Doctor and the Hub use the same coordinator. It refuses active legacy
GUI/workers and an uninspectable supervisor, preserves the existing opt-in,
and restores legacy activation if native migration fails. Follow its error
rather than killing a worker or discarding data. Native migration retains
private backups under `$XDG_STATE_HOME/hyakki-migration-backups/`.

App settings are `$XDG_CONFIG_HOME/hyakki/hyakki.json`; state, data and cache
use their XDG bases under `hyakki/`. Clients use
`$XDG_RUNTIME_DIR/hyakki/oni.sock`, or `HYAKKI_SOCKET` when explicitly set.
The machine-map vault stays at `$XDG_DATA_HOME/ryoku/vault/` and refreshes with
`ryoku vault refresh` independently of the app. The Teams credential Vault is
separate from this machine map.

Source lives at `ryoku/hyakki/`, with physical command siblings `hyakki`,
`oni`, `hyakki-app` and `oni-surface` under `cmd/`. The internal Go module name
`ryoku-rashin` is not a runtime alias. The complete source-tree guide is
`docs/hyakki.md`.

Return to the [desktop tour](desktop.md), or read [Linux basics](linux-basics.md)
for services, logs, updates and config ownership.
