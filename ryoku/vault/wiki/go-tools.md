# Go Tools in Ryoku

Ryoku keeps presentation in QML and puts contracts, validation, daemons, and
shared behavior in Go. The command name tells you which part owns a job.

| Tool | Checkout source | Job |
|---|---|---|
| `ryoku` | `ryoku/cli/` | updates, rollback, ownership, doctor, plugins, and the `wm` front door |
| `ryoku-shell` | `ryoku/shell/ipc/` | shell daemon, bar and dock stores, live shell actions |
| `ryoku-wm-<name>` | `ryoku/wm/` | provider capabilities, state, actions, config generation, binds, and outputs |
| `rashind`, `rashin`, and `rashin-app` | `ryoku/rashin/` | optional Ask, Agent, sessions, free-model routing, and indexes |

The user-facing `ryoku wm` command resolves the live provider. Shared callers do
not launch a compositor command directly.

## Discover commands and code

Start with each tool's usage:

```bash
ryoku --help
ryoku wm
ryoku-shell
rashin --help
```

When Rashin is on, ask its cited index before opening broad files:

```bash
rashin find <symbol>
rashin def <symbol>
rashin outline <path>
rashin references <symbol>
rashin search "how does this work?"
```

`find` locates a named function, type, setting, or component. `def` reads one
definition. `outline` maps one file without dumping its bodies. `references`
shows callers and uses. `search` answers a structural question. Use grep only
after that for an exact literal or regular expression. Without Rashin, use the
pre-indexed map in `~/.local/share/ryoku/vault/ryoku-repo.md`.

## Add a verb in a checkout

Develop against a clone or worktree of the Ryoku repository, never against
`/usr/share/ryoku/config`, `~/.config/quickshell/`, or an installed binary.
Those paths are update targets.

A small command change follows this loop:

1. Use `rashin find` when Rashin is on, or the shipped `ryoku-repo.md` map, to
   locate the command dispatcher and the nearest existing verb.
2. Read its definition and references. Match the existing argument, error, and
   output conventions.
3. Keep dispatch thin. Put the behavior in the package that owns the concern.
4. Add a behavior test in the same Go module. Test outputs, boundaries, errors,
   and state transitions a caller can observe.
5. Update the usage text and the user-facing command reference.
6. Run the module checks, then invoke the new verb from the checkout and inspect
   its real result.

For a window-manager verb, define neutral behavior in `ryoku/wm/` and implement
it in each provider that claims the matching capability. Never branch on a
provider name in the shell, Hub, or general CLI.

## Focused checks

Run Go checks from each module you changed:

```bash
go build ./...
go vet ./...
go test ./...
```

Window-manager changes also run:

```bash
bin/ryoku-dev-verify-wm-isolation
bin/ryoku-dev-verify-delivery
```

QML changes need `qmllint` when available and a real surface load. Lua changes
need `luac -p <file>`. Shell scripts need `bash -n <file>`. Tests and parsers are
gates, not the final observation: run the command or surface and perform the
user action it changed.

A checkout reaches a development machine through `ryoku deploy`. A packaged
machine receives built binaries and materialized config through `ryoku update`.
Changes flow from source to the machine, never from a hand-edited live file back
to source.

Next: build a first [Quickshell plugin](quickshell-qml.md), or read the
[window-manager guides](README.md) before touching provider code.
