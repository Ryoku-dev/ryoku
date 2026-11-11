# Rashin wire protocol

`rashind` listens on `$XDG_RUNTIME_DIR/rashin/rashin.sock` (override with
`RASHIN_SOCKET`). The transport is a Unix stream socket carrying compact JSON,
one object per line, at most 16 MiB a line. Go types for every shape below are
in this package; `methods.go` is the registry the daemon dispatches from.

```
request   {"id": 1, "method": "hello", "params": {"client": "bar", "version": 1}}
response  {"id": 1, "result": {"daemon": "rashind", "version": "2.0.0", "protocol": 1, "enabled": true}}
error     {"id": 2, "error": {"code": "not_found", "message": "No session ask-20261109-233200-a1f3."}}
event     {"event": "session.event", "data": {"session": "ask-...", "seq": 4, "ts": "...", "kind": "text", "data": {"message": "m1", "delta": "Hello"}}}
```

- `hello` is the first request on every connection. `client` is `app`, `bar`,
  `cli` or `hub`.
- Requests on one connection may be in flight together; responses carry the
  request id and may come back in any order. Events interleave with responses.
- Error codes: `bad_request`, `not_found`, `unavailable`, `conflict`,
  `disabled`, `unsupported`, `forbidden`, `internal`. Messages are sentences
  for people.
- `forbidden` refuses a Teams or vault method that only the user may call
  (`teams.UserOnly`), and `harnesses.probe`, to any connection not opened by
  the Rashin app binary (`rashin-app` beside the daemon, or
  `/usr/bin/rashin-app`, read from the peer's `/proc/<pid>/exe`). It also
  refuses, on the main socket, a
  `teams.agent_*` call, a `session.new` naming a team, task or member, and
  from anything but the app the control methods on a team task's session
  (`session.permission`, `set_model`, `set_mode`, `set_option`, `prompt`,
  `queue`, `unqueue`, `steer`, `goal`, `cancel`, `stop`, `start`, `rename`,
  `delete`, `handoff`, `fork`, `revert_turn`).
- A team task's session runs its agent in a bubblewrap sandbox and is served
  on a socket of its own, `$XDG_RUNTIME_DIR/rashin/agents/<session>/rashin.sock`,
  which the sandbox sees at the main socket's path. A connection there may call
  only the `teams.agent_*` methods as that session, its own `session.goal`, its
  own `screen.act` as the agent and `screen.control` ask and stop, and the
  index methods that read; everything else is `forbidden`.

## Methods

| Method | Params | Result |
|---|---|---|
| `hello` | `HelloParams` | `HelloResult` |
| `status` | | `Status` |
| `subscribe` | | `Snapshot`, then global events |
| `shutdown` | | |
| `session.list` | `SessionListParams` | `SessionListResult` |
| `session.new` | `SessionNewParams` | `SessionInfo` |
| `session.attach` | `SessionAttachParams` | `SessionAttachResult`, then `session.event` |
| `session.detach` | `SessionRef` | |
| `session.prompt` | `PromptParams` (text and attachments, or for Ask `run`: a command the user runs) | `PromptResult` |
| `session.cancel` | `SessionRef` | |
| `session.permission` | `PermissionParams` | |
| `session.set_model` | `SetModelParams` | |
| `session.set_mode` | `SetModeParams` (a harness mode, or for Ask an approvals value) | |
| `session.set_option` | `SetOptionParams` | |
| `session.start` | `SessionRef` (bring the agent up with no prompt) | |
| `session.rename` | `RenameParams` | `SessionInfo` |
| `session.delete` | `SessionRef` | |
| `session.stop` | `SessionRef` | |
| `session.handoff` | `HandoffParams` | `SessionInfo` (the new Agent session) |
| `session.goal` | `GoalParams` (set, pause, resume, stop, turns, get, complete, block, clear) | `SessionInfo` |
| `screen.list` | | `ScreensResult` |
| `screen.act` | `ScreenActParams` (by the agent or the user) | `ScreenActResult` (`waiting` when an agent's action was not done yet) |
| `screen.control` | `ScreenControlParams` (take, release, ask, watch, unwatch, stop) | `ScreenState` |
| `ask` | `AskParams` | `AskResult`, caller attached |
| `providers.list` | | `ProvidersResult` |
| `providers.add_key` | `AddKeyParams` | `ProviderInfo` |
| `providers.remove_key` | `RemoveKeyParams` | `ProviderInfo` |
| `providers.test` | `ProviderParams` | `ProviderTestResult` |
| `providers.enable` | `EnableProviderParams` | `ProviderInfo` |
| `models.list` | `ModelsParams` | `ModelsResult` |
| `gateway.usage` | `UsageParams` | `GatewayUsage` |
| `gateway.api` | | `APIInfo` |
| `gateway.api_key` | `APIKeyParams` (rotate) | `APIKey` |
| `harnesses.list` | `HarnessesListParams` | `HarnessesResult` |
| `harnesses.probe` | `HarnessProbeParams` (start once in a team sandbox, no prompt; pick `model` and `options` when given) | `HarnessProbeResult` (`ok` false with the harness's `message` when it did not start or refused one) |
| `index.list` | | `IndexesResult` |
| `index.add` | `IndexAddParams` | `IndexInfo` |
| `index.remove` | `IndexRef` | |
| `index.refresh` | `IndexRef` | `IndexInfo` |
| `index.status` | `IndexRef` | `IndexInfo` |
| `index.search` | `SearchParams` | `SearchResult` |
| `index.find` | `NameParams` | `SymbolsResult` |
| `index.def` | `NameParams` | `DefResult` |
| `index.outline` | `PathParams` | `SymbolsResult` |
| `index.references` | `NameParams` | `ReferencesResult` |
| `index.peek` | `PeekParams` | `PeekResult` |
| `index.overview` | `IndexRef` | `Overview` |
| `index.impact` | `PathParams` | `Impact` |
| `index.wip` | `WipParams` | `WipResult` |
| `index.changed` | `WipParams` | `WipResult` |
| `settings.get` | | `Settings` |
| `settings.set` | `SettingsSetParams` (merge patch) | `Settings` |
| `ui.open` | `UIOpenParams` | `UIOpenResult` |
| `teams.list` | | `TeamsResult` |
| `teams.templates` | | `TeamTemplatesResult` |
| `teams.create` | `TeamCreateParams` | `Team` (with its routines) |
| `teams.update` | `TeamUpdateParams` (merge patch) | `Team` |
| `teams.delete` | `TeamRef` (its tasks, routines and grants go; its folder stays) | |
| `teams.pause` | `TeamPauseParams` | `Team` |
| `teams.export` | `TeamRef` | `TeamExportResult` (written into the team's folder, no history or values) |
| `teams.import` | `TeamImportParams` | `Team` (paused, routines off) |
| `teams.tasks` | `TeamTasksParams` | `TeamTasksResult` |
| `teams.task_create` | `TeamTaskCreateParams` (`model` and `options`: an engine for this task only, on the member's harness) | `TeamTask` |
| `teams.task_stop` | `TeamTaskRef` (and what it delegated) | `TeamTask` |
| `teams.task_retry` | `TeamTaskRetryParams` | `TeamTask` |
| `teams.task_read` | `TeamTaskRef` | `TeamTask` |
| `teams.task_delete` | `TeamTaskRef` (its sessions stay) | |
| `teams.routines` | `TeamRef` (empty id: every team) | `RoutinesResult` |
| `teams.routine_create` | `RoutineCreateParams` | `Routine` |
| `teams.routine_update` | `RoutineUpdateParams` (merge patch) | `Routine` |
| `teams.routine_delete` | `RoutineRef` (its tasks stay) | |
| `teams.routine_run` | `RoutineRef` | `TeamTask` |
| `teams.routine_preview` | `RoutinePreviewParams` | `RoutinePreview` |
| `teams.approvals` | `ApprovalsParams` | `ApprovalsResult` |
| `teams.approval_decide` | `ApprovalDecideParams` (once, always, deny, provide with a value, or choose with a `choice` among the approval's `options`) | `Approval` |
| `teams.activity` | `TeamActivityParams` | `ActivityResult` |
| `teams.spend` | `TeamSpendParams` | `TeamSpend` (`engines`: ids are harness and model joined by `\x1f`) |
| `teams.prune` | `TeamPruneParams` (finished tasks past `teams.keepDays`, or every finished one with `all`; their sessions go too, their files stay) | |
| `teams.agent_report` | `AgentReportParams` (`quiet`: nothing changed since the last run) | `AgentAnswer` |
| `teams.agent_delegate` | `AgentDelegateParams` (the lead only) | `AgentAnswer` |
| `teams.agent_status` | `AgentRef` | `AgentStatusResult` (the pieces the task handed out) |
| `teams.agent_ask` | `AgentAskParams` (`options`: 2 to 6 choices of at most 80 characters) | `AgentAnswer` |
| `teams.agent_http` | `AgentHTTPParams` (`saveAs`: a path in the team's folder for the whole answer) | `AgentHTTPResult` (`saved` when it was saved, `waiting` while the user decides) |
| `teams.agent_mail` | `AgentMailParams` | `AgentAnswer` |
| `teams.agent_request` | `AgentCredentialRequestParams` | `AgentAnswer` |
| `teams.agent_credentials` | `AgentRef` | `AgentCredentialsResult` |
| `vault.info` | | `VaultInfo` (never a value) |
| `vault.add` | `VaultAddParams` | `Credential` |
| `vault.update` | `VaultUpdateParams` | `Credential` |
| `vault.remove` | `VaultRef` | |
| `vault.events` | `VaultEventsParams` | `VaultEventsResult` |
| `vault.mail` | `MailAccountParams` (set or remove) | `VaultInfo` |
| `vault.mail_test` | | `MailTestResult` |

Who may call the Teams and vault methods (`internal/teams/authority.go`,
`internal/daemon/gate.go`):

- Anything on the main socket may call the ones that read: `teams.list`,
  `teams.templates`, `teams.tasks`, `teams.routines`,
  `teams.routine_preview`, `teams.approvals`, `teams.activity`,
  `teams.spend`, `vault.info` and `vault.events`. None of them returns a
  credential value, and what they show is redacted before it is recorded.
- Every other `teams.*` and `vault.*` method is the user's and is served only
  to the Rashin app. A method added later is the user's until it is listed as
  readable.
- The `teams.agent_*` methods are served only on a team task's own socket,
  and each carries an `AgentRef` (session and run token) that the daemon
  checks against that session's live run.

Fields of the Teams shapes that clients and agents rely on:

- `TeamTask.harness`, `model` and `options` are the engine the task ran on,
  set when it starts and kept after it ends. Before it starts only `model`
  and `options` may be set: the engine asked for this task alone.
  `protocol.ResolveEngine` is the one chain from a member to its engine:
  the member's harness, else `teams.defaults.harness`, else
  `agent.harness`, else the registry's default harness.
- `Approval.options` makes a choice question; `choice` keeps the picked
  option. The agent is told "The user chose: <option>" and any note.
- `TeamReport.quiet` marks a run that changed nothing: done, unread not set,
  no notification. `Routine.quietStreak` counts such runs in a row.
- `SessionInfo.turnStarted` is when the running turn began, absent while no
  turn runs; `steps` counts its tool calls, or the last turn's once it ended.
- `HarnessCapabilities.modelList`, `defaultModel`, `options` and `learned`
  are what a harness reported at its last start; `defaultModel` comes only
  from a new session.
- `Settings.teams` carries `perTeam`, `paused`, `routinesWake`, `defaults`
  (harness, model, options, approvals, budget, notify for new teams),
  `approvalHours`, `quietHours` (`from` and `to` as `HH:MM`), `hidden`
  (null for the built-in list, `[]` for none) and `keepDays`
  (0, 30, 90 or 365).

## Events

| Event | Data | Sent to |
|---|---|---|
| `session.event` | `SessionEventData` | connections attached to the session |
| `session.info` | `SessionInfo` | subscribers, and connections attached to it |
| `sessions.changed` | `SessionsChanged` | subscribers |
| `settings.changed` | `Settings` | subscribers |
| `providers.changed` | `ProvidersResult` | subscribers |
| `api.changed` | `APIInfo` | subscribers |
| `harnesses.changed` | `HarnessesResult` | subscribers |
| `index.progress` | `IndexProgress` | subscribers |
| `index.changed` | `IndexInfo` | subscribers |
| `ui.focus` | `UIOpenParams` | app subscribers |
| `notice` | `Notice` | subscribers |
| `screen` | `ScreenState` (`stopped` when it ended) | subscribers |
| `team` | `Team` | subscribers |
| `team.removed` | `TeamRemoved` | subscribers |
| `team.task` | `TeamTask` | subscribers |
| `team.task.removed` | `TeamRemoved` | subscribers |
| `team.approval` | `Approval` | subscribers |
| `routine` | `Routine` | subscribers |
| `routine.removed` | `TeamRemoved` | subscribers |
| `vault` | `VaultInfo` (never a value) | subscribers |

## Session events

A session log is the ordered list of `SessionEvent{seq, ts, kind, data}`.
`seq` starts at 1 and only grows. A client applies events in order and
remembers the last `seq`; reattaching with that `since` resumes without gaps.
A replay that does not fit one frame comes in pages: while the result has
`more`, the client attaches again with `since` set to the last event's `seq`.
The connection receives live `session.event`s only after the last page.

| Kind | Data type |
|---|---|
| `user` | `UserData` |
| `text`, `thought` | `TextData` (append `delta` to message `message`) |
| `tool` | `ToolData` (merge by `call`; `agents` are the subagents a call runs, `parent` the call a subagent's call belongs to) |
| `permission` | `PermissionData` |
| `permission_resolved` | `PermissionResolvedData` |
| `plan` | `PlanData` (replaces the plan) |
| `terminal` | `TerminalData` (append `delta` to terminal `terminal`) |
| `turn_start` | `TurnStartData` |
| `turn_end` | `TurnEndData` |
| `error` | `ErrorData` |
| `model`, `mode` | `ChoiceData` |
| `options` | `OptionsData` (replaces the options) |
| `commands` | `CommandsData` |
| `answer` | `AnswerData` |
| `title` | `TitleData` |
| `goal` | `GoalData` (the goal as it now stands) |
