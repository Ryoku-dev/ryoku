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
  `disabled`, `unsupported`, `internal`. Messages are sentences for people.

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
