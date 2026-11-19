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
  `disabled`, `unsupported`, `forbidden`, `internal`, `no_vision`, `local_only`.
  Messages are sentences for people.
- `forbidden` refuses an app-only method to any connection not opened by the
  Rashin app binary (`rashin-app` beside the daemon, or `/usr/bin/rashin-app`,
  read from the peer's `/proc/<pid>/exe`). App-only are: a Teams or vault
  method that only the user may call (`teams.UserOnly`), `models.probe`,
  `local.devices`, `local.download`, `local.cancel`, `local.remove`, `local.add`,
  `local.runtime_install`, `local.start`, `local.stop`, and every
  `learning.*` method that changes something or runs a model
  (`learning.skill_delete`, `learning.decision_delete`, `learning.rule_save`,
  `learning.rule_delete`, `learning.rule_forge`,
  `learning.grievances_clear`), plus the methods marked app only below. It
  also refuses, on the main socket, a `teams.agent_*` call, a `session.new`
  naming a team, task or member, and from anything but the app the control
  methods on a team task's session (`session.permission`, `set_model`,
  `set_mode`, `set_option`, `prompt`, `queue`, `unqueue`, `steer`, `goal`,
  `cancel`, `stop`, `rename`, `delete`, `handoff`, `fork`, `revert_turn`).
- Every session runs on Rashin's own engine: a model (a route or
  `provider/model`) and its options, no external harness. A team task's
  session runs every file and shell tool in one tool host child inside a
  bubblewrap sandbox; its model calls, keys, team tools and memory stay in
  the daemon, and its team tools (`teams.agent_*`) are called in-process.
  The sandbox reaches the daemon on a socket of its own,
  `$XDG_RUNTIME_DIR/rashin/agents/<session>/rashin.sock`, seen at the main
  socket's path, so `rashin search` and the like work in its shell. A
  connection there may call only the index methods that read; everything
  else is `forbidden`.
- Optional app features live in `Settings.features`, keyed by `FeatureIDs`.
  They start off, unknown ids are dropped, and any ordinary client can patch
  them with `settings.set`. `ask.resumeMinutes` is 0 for off or 1 to 120,
  clamped on read and write. `memory.profile` holds `name`, `work`, `about`
  and `traits`, and stays behind the existing app-only memory settings gate.
- The self-improvement features `autolearn`, `decisions`, `rules` and
  `grievances` are features like the others, off until turned on. What they
  kept stays listed and removable through `learning.*` after they are turned
  off; `learning.rule_forge` alone answers `disabled` while `rules` is off.
- `SessionInfo.starred` and `archived` are independent conversation flags,
  changed with `session.set_flags`.
  `lastMessageAt` records conversation activity separately from metadata
  updates, so reopening a conversation does not extend its resume window.

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
| `session.set_mode` | `SetModeParams` (an approvals value: `read-only`, `ask`, `auto`) | |
| `session.set_option` | `SetOptionParams` (one of the model's options, or `plan` `on`/`off` for Plan first) | |
| `session.rename` | `RenameParams` | `SessionInfo` |
| `session.delete` | `SessionRef` | |
| `session.stop` | `SessionRef` | |
| `session.handoff` | `HandoffParams` | `SessionInfo` (the new Agent session) |
| `session.goal` | `GoalParams` (set, pause, resume, stop, turns, get, complete, block, clear) | `SessionInfo` |
| `screen.list` | | `ScreensResult` |
| `screen.act` | `ScreenActParams` (by the agent or the user) | `ScreenActResult` (`waiting` when an agent's action was not done yet) |
| `screen.control` | `ScreenControlParams` (take, release, ask, watch, unwatch, stop) | `ScreenState` |
| `ask` | `AskParams` | `AskResult`, caller attached |
| `glance` | `GlanceParams` (capture modes, model and local-only routing) | `AskResult`, caller attached |
| `writing.check` | `WritingCheckParams` | `WritingCheckResult` |
| `writing.transform` | `WritingTransformParams` | `WritingTransformResult` |
| `writing.cancel` | `WritingCancelParams` | `Empty` |
| `writing.voice.get` | `WritingVoiceGetParams` | `WritingVoice` |
| `writing.voice.set` | `WritingVoiceSetParams` | `WritingVoice` |
| `writing.voice.learn` | `WritingVoiceLearnParams` | `WritingVoice` |
| `writing.words` | `WritingWordsParams` | `WritingWords` |
| `writing.words.add` | `WritingWordParams` (`word`) | `WritingWords` |
| `writing.words.remove` | `WritingWordParams` (`word`) | `WritingWords` |
| `prompts.list` | `PromptsListParams` | `PromptsListResult` |
| `prompts.save` | `PromptSaveParams` | `Prompt` |
| `prompts.delete` | `PromptDeleteParams` | `Empty` |
| `prompts.expand` | `PromptExpandParams` | `PromptExpandResult` |
| `compare.start` | `CompareStartParams` | `CompareStartResult` |
| `compare.keep` | `CompareKeepParams` | `SessionInfo` |
| `session.set_flags` | `SessionFlagsParams` | `SessionInfo` |
| `mcp.list` | `MCPListParams` | `MCPListResult` |
| `mcp.save` | `MCPServer` (app only) | `MCPServerInfo` |
| `mcp.delete` | `MCPDeleteParams` (app only) | `Empty` |
| `mcp.check` | `MCPCheckParams` | `MCPServerInfo` |
| `mcp.signin` | `MCPSignInParams` (app only) | `MCPSignInResult` |
| `disk.usage` | `DiskUsageParams` | `DiskUsage` |
| `disk.clean` | `DiskCleanParams` (app only) | `DiskUsage` |
| `artifact.open` | `ArtifactOpenParams` | `ArtifactOpenResult` |
| `artifact.errors` | `ArtifactErrorsParams` | `ArtifactErrorsResult` |
| `artifact.repair` | `ArtifactRepairParams` | `AskResult` |
| `artifact.close` | `ArtifactCloseParams` | `Empty` |
| `collections.list` | `CollectionsListParams` | `CollectionsListResult` |
| `collections.save` | `CollectionSaveParams` | `Collection` |
| `collections.add` | `CollectionAddParams` | `CollectionItem` |
| `collections.items` | `CollectionItemsParams` | `CollectionItemsResult` |
| `collections.remove` | `CollectionRemoveParams` | `Empty` |
| `collections.check` | `CollectionCheckParams` | `CollectionItem` |
| `collections.chat` | `CollectionChatParams` | `AskResult` |
| `teams.routine_runs` | `RoutineRunsParams` | `RoutineRunsResult` |
| `teams.agent_setup` | `AgentSetupParams` | `AgentAnswer` |
| `providers.list` | | `ProvidersResult` |
| `providers.add_key` | `AddKeyParams` | `ProviderInfo` |
| `providers.remove_key` | `RemoveKeyParams` | `ProviderInfo` |
| `providers.test` | `ProviderParams` | `ProviderTestResult` |
| `providers.enable` | `EnableProviderParams` | `ProviderInfo` |
| `providers.limits` | `ProviderParams` | `Empty` (then `providers.changed`) |
| `models.list` | `ModelsParams` | `ModelsResult` |
| `models.probe` | `ModelProbeParams` (app only; one small streamed request with a no-op tool) | `ModelProbeResult` (`ok` false with a `message` when the model did not answer or refused an option) |
| `gateway.usage` | `UsageParams` | `GatewayUsage` |
| `gateway.api` | | `APIInfo` |
| `gateway.api_key` | `APIKeyParams` (rotate) | `APIKey` |
| `local.models` | | `LocalResult` (catalogue, installed models, runtime; starts nothing) |
| `local.devices` | (app only) | `LocalDevicesResult` (probed on demand, cached by runtime binary version) |
| `local.download` | `LocalModelParams` (app only) | `Empty` (progress as `local.progress`) |
| `local.cancel` | `LocalModelParams` (app only; keeps the part files) | `Empty` |
| `local.remove` | `LocalModelParams` (app only) | `LocalResult` |
| `local.search` | `LocalSearchParams` (a Hugging Face repo) | `LocalSearchResult` (per-GGUF parameter count, trained context, tool support and warning; `gated` repos are refused) |
| `local.add` | `LocalAddParams` (app only; the daemon pins the revision) | `LocalModel` (metadata and warning preserved; its download starts) |
| `local.runtime_install` | `LocalRuntimeParams` (app only; `vulkan`, `cpu` or `cuda`) | `Empty` (progress as `local.progress` with id `runtime`) |
| `local.start` | `LocalModelParams` (app only; id optional) | `LocalRuntime` |
| `local.stop` | `LocalModelParams` (app only; id ignored) | `LocalRuntime` (first broadcasts `stopping`, then `stopped` after the process group exits) |
| `learning.skills` | | `LearningSkillsResult` (managed skills; `shadowed` when a built-in or authored skill wins the name) |
| `learning.skill_delete` | `LearningSkillParams` (app only) | `LearningSkillsResult` |
| `learning.decisions` | `LearningDecisionsParams` (`project` folder, empty for every bank) | `LearningDecisionsResult` |
| `learning.decision_delete` | `DecisionDeleteParams` (app only; `area` is `architecture`, `product` or `style`, `line` the decision's text) | `DecisionBank` |
| `learning.rules` | `LearningRulesParams` (global rules, and a `project`'s with it) | `LearningRulesResult` |
| `learning.rule_save` | `RuleSaveParams` (app only; `overwrite` replaces a rule of the same name) | `Rule` |
| `learning.rule_delete` | `RuleRef` (app only) | |
| `learning.rule_forge` | `RuleForgeParams` (app only; needs the `rules` feature; saves nothing) | `RuleForgeResult` |
| `learning.grievances` | `LearningGrievancesParams` | `LearningGrievancesResult` |
| `learning.grievances_clear` | `GrievancesClearParams` (app only; `ids`, else a `tool`, else all) | |
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
| `ui.open` | `UIOpenParams` (`dreamDraft` prefills the app's dream creation sheet) | `UIOpenResult` |
| `teams.list` | | `TeamsResult` |
| `teams.templates` | | `TeamTemplatesResult` |
| `teams.create` | `TeamCreateParams` | `Team` (with its routines) |
| `teams.update` | `TeamUpdateParams` (merge patch) | `Team` |
| `teams.delete` | `TeamRef` (its tasks, routines and grants go; its folder stays) | |
| `teams.pause` | `TeamPauseParams` | `Team` |
| `teams.export` | `TeamRef` | `TeamExportResult` (written into the team's folder, no history or values) |
| `teams.import` | `TeamImportParams` | `Team` (paused, routines off) |
| `teams.tasks` | `TeamTasksParams` | `TeamTasksResult` |
| `teams.task_create` | `TeamTaskCreateParams` (`model` and `options`: an engine for this task only) | `TeamTask` |
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
| `teams.spend` | `TeamSpendParams` | `TeamSpend` (`engines`: ids are the models that answered; `days` count the tasks that ended done or blocked and the routine runs among them) |
| `teams.prune` | `TeamPruneParams` (finished tasks past `teams.keepDays`, or every finished one with `all`; their sessions go too, their files stay) | |
| `teams.dream_create` | `DreamCreateParams` (`model` and `options`: the manager's engine) | `Team` |
| `teams.dream_answer` | `DreamAnswerParams` | `Team` |
| `teams.agent_report` | `AgentReportParams` (`quiet`: nothing changed since the last run) | `AgentAnswer` |
| `teams.agent_delegate` | `AgentDelegateParams` (the lead only) | `AgentAnswer` |
| `teams.agent_status` | `AgentRef` | `AgentStatusResult` (the pieces the task handed out) |
| `teams.agent_ask` | `AgentAskParams` (`options`: 2 to 6 choices of at most 80 characters) | `AgentAnswer` |
| `teams.agent_http` | `AgentHTTPParams` (`saveAs`: a path in the team's folder for the whole answer) | `AgentHTTPResult` (`saved` when it was saved, `waiting` while the user decides) |
| `teams.agent_mail` | `AgentMailParams` | `AgentAnswer` |
| `teams.agent_request` | `AgentCredentialRequestParams` | `AgentAnswer` |
| `teams.agent_credentials` | `AgentRef` | `AgentCredentialsResult` |
| `teams.agent_hire` | `AgentHireParams` (a dream's manager; `model` empty takes the manager's) | `Member` |
| `teams.agent_release` | `AgentReleaseParams` (a dream's manager) | `AgentAnswer` |
| `teams.agent_dream` | `AgentDreamParams` (a dream's manager) | `AgentAnswer` |
| `teams.agent_present` | `AgentPresentParams` (a dream's manager) | `AgentAnswer` |
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
- The `teams.agent_*` methods are the team tools of a team task's agent,
  called in-process by the engine, and each carries an `AgentRef` (session
  and run token) that the daemon checks against that session's live run. No
  socket serves them.

Fields of the Teams shapes that clients and agents rely on:

- `TeamTask.model` and `options` are the engine the task ran on, set when it
  starts and kept after it ends. Before it starts they hold only the engine
  asked for this task alone. `protocol.ResolveEngine` is the one chain from
  a member to its engine: the member's model, else `teams.defaults.model`,
  else `agent.model`, else `auto`; options follow the same chain on their
  own. A stored model that is neither a route nor `provider/model` (an id an
  external harness took) is cleared, never routed.
- `Approval.options` makes a choice question; `choice` keeps the picked
  option. The agent is told "The user chose: <option>" and any note.
- `TeamReport.quiet` marks a run that changed nothing: done, unread not set,
  no notification. `Routine.quietStreak` counts such runs in a row.
- `SessionInfo.turnStarted` is when the running turn began, absent while no
  turn runs; `steps` counts its tool calls, or the last turn's once it ended.
- `Settings.teams` carries `perTeam`, `paused`, `routinesWake`, `defaults`
  (model, options, approvals, budget, notify for new teams),
  `approvalHours`, `quietHours` (`from` and `to` as `HH:MM`), `hidden`
  (null for the built-in list, `[]` for none) and `keepDays`
  (0, 30, 90 or 365).
- `Settings.agent` carries `model` (a route or `provider/model`, default
  `auto`), `options`, `plan` (Plan first for new Agent sessions),
  `approvals` and `idleStopMinutes`. `Settings.providers.models` configures
  Rashin's own llama.cpp: `binary` (an absolute path; empty uses the
  managed runtime, then `llama-server` on PATH), `idleMinutes` (1 to 120,
  default 5), `device`, `light` (the model for light work) and
  `remoteFirst` (false keeps models on this computer first in Auto, Fast and
  Smart). `providers.local` holds addresses for `llamacpp` and
  `ollama-local` only; `rashin-models` has none.

## Events

| Event | Data | Sent to |
|---|---|---|
| `session.event` | `SessionEventData` | connections attached to the session |
| `session.info` | `SessionInfo` | subscribers, and connections attached to it |
| `sessions.changed` | `SessionsChanged` | subscribers |
| `settings.changed` | `Settings` | subscribers |
| `providers.changed` | `ProvidersResult` | subscribers |
| `api.changed` | `APIInfo` | subscribers |
| `local.progress` | `LocalProgress` | subscribers |
| `local.changed` | `LocalResult` | subscribers |
| `learning.changed` | `LearningChanged` (read that part again) | subscribers |
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

`writing.check` always uses the built-in checker for `engine:"local"`, regardless
of language. `auto` checks likely English on this computer and uses the model
for other languages when a provider is configured, otherwise it checks locally.
`model` always uses the model. Dialects are `us`, `uk`, `ca`, `au` and `in`.
Issue offsets are UTF-16 units, including for text with emoji. Categories in
`off` suppress matching issues. `writing.words` returns `{words:[...]}`.
Add and remove accept `{word:"..."}` and return the updated dictionary.
Words keep their typed casing, are deduplicated case-insensitively with the
newest spelling kept, and are accepted in every English dialect. Up to 5,000
words are saved privately in `words.txt` in Rashin's data directory.
These writing methods are open to bar, CLI and Hub.

`glance` takes `session`, `mode`, `question`, `text`, `images`, `language`,
`source`, `target`, `model`, `local` and string-valued `options`.
Modes are `explain`, `ask`, `read`, `translate`, `summarize`, `deepread`,
`solve`, `learn`, `define`, `rewrite`, `tone`, `fluent`, `humanize`,
`reactions`, `reply` and `aicheck`. Options select each mode's style, lens,
form, count, audience or tone; `voice:"on"` uses the saved writing voice.
Without `session` it creates a temporary Ask with origin `sight`, and with one
it continues that conversation. Origins are `app`, `bar`, `cli`, `terminal`,
`fix`, `import`, `team` and `sight`.

A capture uses a short mode prompt without tools, vault context or memory
recall, routing to `model`, else light work on the smallest local model that
fits. `local:true` keeps it to models on this computer. No picture-capable
route produces `no_vision` with the provider guidance.

Model events also carry `local` and `engine`, so a capture can show where its
answer came from. Keeping a temporary session with `session.fork` copies its
runtime-directory images into Rashin's data directory before keeping the log.
Oversized images are downscaled below 8 MiB without changing their shape.

An Agent session reports its `model`, `mode` and `options` when it is created,
without a prompt: its mode is its approvals value, and its options are the
model's knobs (`effort` on a model with reasoning) plus `plan`, Plan first,
which keeps it on read-only tools and a todo list until the user approves the
plan. A session made before the native engine keeps its log; its next prompt
starts a fresh engine transcript and says so in a `notice`. The subscribe
`Snapshot` carries `local`, Rashin's own models and runtime.

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
| `compare` | `CompareStartResult` (`id`, `source`, `seq`, `lanes`; an empty id and lanes clear the comparison at `seq`) |
| `notice` | `NoticeData` (one quiet transcript line; `kind` is `fresh`, `limit`, `loop` or `plan`) |

Comparison lanes are independent temporary sessions. The source conversation
records the comparison event so reattaching restores its lanes.
