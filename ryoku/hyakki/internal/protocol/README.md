# Hyakki wire protocol

`oni` listens on `$XDG_RUNTIME_DIR/hyakki/oni.sock` (override with
`HYAKKI_SOCKET`). The transport is a Unix stream socket carrying compact JSON,
one object per line, at most 16 MiB a line. Go types for every shape below are
in this package; `methods.go` is the registry the daemon dispatches from.

```
request   {"id": 1, "method": "hello", "params": {"client": "bar", "version": 1}}
response  {"id": 1, "result": {"daemon": "oni", "version": "dev", "protocol": 1, "enabled": true}}
error     {"id": 2, "error": {"code": "not_found", "message": "No session ask-20261109-233200-a1f3."}}
event     {"event": "session.event", "data": {"session": "ask-...", "seq": 4, "ts": "...", "kind": "text", "data": {"message": "m1", "delta": "Hello"}}}
```

- `hello` is the first request on every connection. `client` is `app`, `bar`,
  `cli`, `hub`, `surface` (a process drawing Oni; see Oni) or `mcp` (Hyakki's
  MCP bridge, `hyakki mcp`, speaking for one program; see MCP bridge). Only
  `mcp` sends `name`, the program's own name, one trimmed line of at most 64
  characters; another kind naming one, an `mcp` hello without one and any
  other kind are `bad_request`, and an `mcp` hello while `oni.mcp.enabled` is
  off is `forbidden`.
- Requests on one connection may be in flight together; responses carry the
  request id and may come back in any order. Events interleave with responses.
- Error codes: `bad_request`, `not_found`, `unavailable`, `conflict`,
  `disabled`, `unsupported`, `forbidden`, `internal`, `no_vision`, `local_only`.
  Messages are sentences for people; an error may add `reason`, a short code
  naming why (an Oni perch reason). A session `error` event for a Ryoku Ember
  request also carries `account_consent_required` (the account terms are not
  accepted yet) or `insufficient_credits` (not enough Ember credits); the app
  answers them with the account page's `#consent` or `#plans`.
- `forbidden` refuses an app-only method to any connection not opened by the
  Hyakki app binary (`hyakki-app` beside the daemon, or `/usr/bin/hyakki-app`,
  read from the peer's `/proc/<pid>/exe`). App-only are: a Teams or vault
  method that only the user may call (`teams.UserOnly`), `models.probe`,
  `local.devices`, `local.found`, `local.use_found`, `local.download`,
  `local.cancel`, `local.remove`, `local.add`, `local.runtime_install`,
  `local.start`, `local.stop`, every `account.*` method, and every
  `learning.*` method that changes something or runs a model
  (`learning.skill_delete`, `learning.decision_delete`, `learning.rule_save`,
  `learning.rule_delete`, `learning.rule_forge`,
  `learning.grievances_clear`), plus the methods marked app only below. It
  also refuses, on the main socket, a `teams.agent_*` call, a `session.new`
  naming a team, task or member, and from anything but the app the control
  methods on a team task's session (`session.permission`, `set_model`,
  `set_mode`, `set_option`, `prompt`, `queue`, `unqueue`, `steer`, `goal`,
  `cancel`, `stop`, `rename`, `delete`, `handoff`, `fork`, `revert_turn`). A
  Ryoku Ember model (`ember/<model>`) selected through the linked Account session in `session.new`,
  `session.set_model`, `ask`, `glance` or `compare.start`, or set as
  `ask.model`, is `forbidden` to anything but the app: its credits are the
  user's, and `providers.test`, `providers.enable` and `providers.remove_key`
  refuse the `ember` provider for everyone.
- Current native sessions run on Hyakki's own engine: a model (a route or
  `provider/model`) and its options, no implicit external harness. A native
  team session runs every file and shell tool in one tool-host child inside a
  bubblewrap sandbox; its model calls, keys, team tools and memory stay in
  the daemon, and its team tools (`teams.agent_*`) are called in-process.
  The sandbox reaches the daemon on a socket of its own,
  `$XDG_RUNTIME_DIR/hyakki/agents/<session>/oni.sock`, seen at the main
  socket's path, so `hyakki search` and the like work in its shell. A
  connection there may call only the index methods that read; everything
  else is `forbidden`.
- An explicitly configured external team runtime uses its declared adapter,
  not the native model gateway. ACP receives only a private per-turn MCP
  governance bridge. Its peer may expose MCP calls directly or through its
  own tool presentation; neither grants general daemon RPC access.
- Optional app features live in `Settings.features`, keyed by `FeatureIDs`.
  They start off, unknown ids are dropped, and any ordinary client can patch
  them with `settings.set`. `ask.resumeMinutes` is 0 for off or 1 to 120,
  clamped on read and write. `memory.profile` holds `name`, `work`, `about`
  and `traits`, and stays behind the existing app-only memory settings gate.
- `Settings.oni` holds Oni's settings (`OniSettings`); see Oni below. It is
  edited only through `settings.set`; there is no Oni settings call.
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
| `components.list` | | `ComponentsResult{components}`; see Optional parts |
| `components.remove` | `ComponentRemoveParams{id}` (app only) | `ComponentsResult{components}` |
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
| `fastdecisions.evaluate` | `FastDecisionParams` (app only): a `DecisionRequest{model?, state, questions}` with optional `route`, `feature`, `budgetMs`, `idempotencyKey` and `accountID` | `FastDecisionResult{route, resolved, decision?, notice, elapsedMs, emberUnits?, creditsExhausted?}` |
| `fastdecisions.local` | (app only) | `DecisionLocalStatus{state, address?, ramMiB?, cores?, reason?}`: one on-demand look for the local decision server, never polled. `state` is `detected`, `absent` (nothing answered, an answer that is no model list, or no answer within 750 ms), `small` (this computer is under the hardware gate) or `refused` (the address is not a loopback one, so nothing was asked); `reason` is a plain sentence for every state but `detected` |
| `gateway.usage` | `UsageParams` | `GatewayUsage` |
| `gateway.api` | | `APIInfo` |
| `gateway.api_key` | `APIKeyParams` (rotate) | `APIKey` |
| `local.models` | | `LocalResult` (catalogue, installed models, runtime; starts nothing) |
| `local.found` | (app only; scan requested by the Local models page) | `LocalFoundResult` (existing GGUFs with planner verdicts, no copies) |
| `local.use_found` | `LocalUseFoundParams` (app only; absolute GGUF path) | `LocalModel` (registered in place; deleting the registration never deletes the source file) |
| `local.devices` | (app only) | `LocalDevicesResult` (probed on demand, cached by runtime binary version) |
| `local.download` | `LocalModelParams` (app only) | `Empty` (progress as `local.progress`) |
| `local.cancel` | `LocalModelParams` (app only; keeps the part files) | `Empty` |
| `local.remove` | `LocalModelParams` (app only) | `LocalResult` |
| `local.search` | `LocalSearchParams` (a Hugging Face repo) | `LocalSearchResult` (per-GGUF parameter count, trained context, tool support and warning; `gated` repos are refused) |
| `local.add` | `LocalAddParams` (app only; the daemon pins the revision) | `LocalModel` (metadata and warning preserved; its download starts) |
| `local.runtime_install` | `LocalRuntimeParams` (app only; `vulkan`, `cpu` or `cuda`) | `Empty` (progress as `local.progress` with id `runtime`) |
| `local.start` | `LocalModelParams` (app only; id optional) | `LocalRuntime` |
| `local.stop` | `LocalModelParams` (app only; id ignored) | `LocalRuntime` (first broadcasts `stopping`, then `stopped` after the process group exits) |
| `account.status` | (app only) | `AccountState` as last read (identity, balance and `manage`, the website page where the account is managed); tokens are never part of it |
| `account.signin` | (app only) | `AccountState` with `code` to approve in the browser (OAuth Device Authorization Grant); the outcome arrives as `account.changed` |
| `account.cancel` | (app only) | `AccountState` (the sign-in stops polling) |
| `account.refresh` | (app only) | `AccountState` read again from the Ryoku Account server; a reply for an account signed out meanwhile is dropped |
| `account.signout` | (app only) | `AccountState` (tokens forgotten, refresh token revoked, hosted requests of that account cancelled) |
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
| `ui.open` | `UIOpenParams` (`dreamDraft` prefills dream creation; optional `activationToken` is a one-shot native desktop grant, never saved) | `UIOpenResult` |
| `teams.list` | | `TeamsResult` |
| `teams.templates` | | `TeamTemplatesResult` |
| `teams.structures` | | `TeamStructuresResult` (visible, hidden and deleted recipes, with source identities) |
| `teams.structure_draft` | `StructureDraftParams` (zero count uses the recipe default) | `StructureDraftResult` (non-running editable roster) |
| `teams.structure_suggest` | `StructureAnswers` | `StructureSuggestionsResult` (local advice, no model call) |
| `teams.structure_save` | `StructureSaveParams` (expected revision) | `StructureTemplate` |
| `teams.structure_clone` | `StructureCloneParams` | `StructureTemplate` (independent custom recipe) |
| `teams.structure_delete` | `StructureRef` (expected revision) | empty (hide built-in or tombstone custom recipe) |
| `teams.structure_restore` | `StructureRef` (expected revision) | `StructureTemplate` |
| `teams.create` | `TeamCreateParams` | `Team` (with its routines) |
| `teams.update` | `TeamUpdateParams` (merge patch; optional `expectedUpdated` rejects a stale whole-board edit) | `Team` |
| `teams.delete` | `TeamRef` (its tasks, routines and grants go; its folder stays) | |
| `teams.pause` | `TeamPauseParams` | `Team` |
| `teams.export` | `TeamRef` | `TeamExportResult` (written into the team's folder, no history or values) |
| `teams.member_config_get` | `MemberConfigRef` (app only; optional `revision` previews retained history) | `MemberConfigDocument` (versioned, masked YAML and current revision) |
| `teams.member_config_save` | `MemberConfigSaveParams` (app only; exact `expectedRevision` and immutable IDs) | `MemberConfigDocument` |
| `teams.member_config_history` | `MemberConfigRef` (app only) | `MemberConfigHistoryResult` (newest first, retention limit) |
| `teams.member_config_restore` | `MemberConfigRestoreParams` (app only; reviewed historical and expected current revisions) | `MemberConfigDocument` (a new current revision, not an in-place rewind) |
| `teams.import` | `TeamImportParams` | `Team` (paused, routines off) |
| `teams.tasks` | `TeamTasksParams` | `TeamTasksResult` |
| `teams.task_context` | `TeamTaskRef` (app only) | `TeamTaskContext` (preserved goal and nearest-parent-first because chain) |
| `teams.task_create` | `TeamTaskCreateParams` (`model` and `options`: an engine for this task only) | `TeamTask` |
| `teams.task_stop` | `TeamTaskRef` (and what it delegated) | `TeamTask` |
| `teams.task_retry` | `TeamTaskRetryParams` | `TeamTask` |
| `teams.task_read` | `TeamTaskRef` | `TeamTask` |
| `teams.task_delete` | `TeamTaskRef` (its sessions stay) | |
| `teams.messages` | `TeamMessagesParams` (app only; team/task filter and cursor) | `TeamMessagesResult` |
| `teams.task_comment` | `TeamTaskCommentParams` (app only) | `TeamMessage` |
| `teams.task_reassign` | `TeamTaskReassignParams` (app only; stops and archives the affected execution family first) | `TeamTask` |
| `teams.task_plan` | `TeamTaskPlanParams` (app only; strategy acceptance never widens grants) | `TeamTask` |
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
| `teams.agent_report` | `AgentReportParams` (`quiet`: routine-only, nothing changed since its last run) | `AgentAnswer` |
| `teams.agent_delegate` | `AgentDelegateParams` (a permitted manager, direct reports only, within delegation depth) | `AgentAnswer` |
| `teams.agent_status` | `AgentRef` | `AgentStatusResult` (goal, ancestry, pieces and a safe member directory) |
| `teams.agent_ask` | `AgentAskParams` (`options`: 2 to 6 choices of at most 80 characters) | `AgentAnswer` |
| `teams.agent_plan` | `AgentPlanParams` | `AgentAnswer` (strategy proposal and hold for the Summoner) |
| `teams.agent_help` | `AgentHelpParams` | `AgentAnswer` (request through the common manager, not a peer assignment) |
| `teams.agent_escalate` | `AgentEscalateParams` | `AgentAnswer` (nearest manager or Summoner) |
| `teams.agent_hands` | `AgentHandsParams{goal, scope, window?}`: `scope` `nested` or `window` (with the window's id from the picker); `desktop` and `browser` are `forbidden`; refused unless Oni is on with Hands set to `app`, and for a read-only task | `AgentAnswer` (raises a single-use `hands` approval; once allowed, Hyakki's own Hands loop works toward the goal and the agent gets no stream of its actions; the grant is bound in memory to the task, its run token, the scope and the window, expires after 30 minutes, and dies with the run or a restart, after which the agent is told to ask again). A `hands` approval is answered with `teams.approval_decide` by the app only, or on Oni's pill |
| `teams.agent_http` | `AgentHTTPParams` (`saveAs`: a path in the team's folder for the whole answer) | `AgentHTTPResult` (`saved` when it was saved, `waiting` while the user decides) |
| `teams.agent_mail` | `AgentMailParams` | `AgentAnswer` |
| `teams.agent_request` | `AgentCredentialRequestParams` | `AgentAnswer` |
| `teams.agent_credentials` | `AgentRef` | `AgentCredentialsResult` |
| `teams.agent_hire` | `AgentHireParams` (a dream's manager; `model` empty takes the manager's) | `Member` |
| `teams.agent_release` | `AgentReleaseParams` (a dream's manager) | `AgentAnswer` |
| `teams.agent_dream` | `AgentDreamParams` (a dream's manager) | `AgentAnswer` |
| `teams.agent_present` | `AgentPresentParams` (a dream's manager) | `AgentAnswer` |
| `vault.info` | | `VaultInfo` (never a value) |
| `vault.add` | `VaultAddParams` (`runtimeConsent` required for Runtime-use creation) | `Credential` |
| `vault.update` | `VaultUpdateParams` (`runtimeConsent` required for new Runtime team grants, not revocation) | `Credential` |
| `vault.remove` | `VaultRef` | |
| `vault.events` | `VaultEventsParams` | `VaultEventsResult` |
| `vault.mail` | `MailAccountParams` (set or remove) | `VaultInfo` |
| `vault.mail_test` | | `MailTestResult` |

Who may call the Teams and vault methods (`internal/teams/authority.go`,
`internal/daemon/gate.go`):

- Anything on the main socket may call the ones that read: `teams.list`,
  `teams.templates`, `teams.structures`, `teams.structure_draft`,
  `teams.structure_suggest`, `teams.tasks`, `teams.routines`,
  `teams.routine_preview`, `teams.routine_runs`, `teams.approvals`, `teams.activity`,
  `teams.spend`, `vault.info` and `vault.events`. None of them returns a
  credential value, and what they show is redacted before it is recorded.
- Every other `teams.*` and `vault.*` method is the user's and is served only
  to the Hyakki app. A method added later is the user's until it is listed as
  readable.
- The `teams.agent_*` methods are the team tools of a team task's agent,
  called in-process by the engine, and each carries an `AgentRef` (session
  and run token) that the daemon checks against that session's live run.
  Ordinary daemon and tool-host RPC sockets do not serve these methods.
  A configured ACP runtime receives a private per-turn MCP bridge offering
  only its current governance tools; it is not general socket authority.

Fields of the Teams shapes that clients and agents rely on:

- `TeamTask.waiting` distinguishes `you` (governance decision), `permission`
  (native conversation permission) and `members`. Only the guarded delivery
  may consume a governance hold and increment `resumeVersion`; a session
  mode or progress event may only clear its own native permission hold.
- `StructureTemplate` is a local roster recipe, not a policy grant or a
  functional `TeamTemplate` workflow preset. Exact draft counts are validated,
  never clamped. Saved recipes must generate valid member names and
  capabilities at every supported count, including collision suffixes.
- Structure mutations are app-only. Revision conflicts preserve the saved
  record; deleted custom snapshots can be restored. A team is created from
  its explicit reviewed members, not rehydrated from the recipe ID.

- `TeamTask.runtime` names the actual adapter. For native tasks, `model`
  and `options` resolve through the member, `teams.defaults`, `agent`, then
  `auto`. Before start they hold the task's requested override; afterward
  they preserve the execution's attribution. A CLI's `runtime.model` is an
  advertised foreign model id, not permission to route it through Hyakki's
  native gateway. Unsupported foreign execution never falls back to native.
- `Approval.options` makes a choice question; `choice` keeps the picked
  option. The agent is told "The user chose: <option>" and any note.
- `TeamReport.quiet` is accepted only for a real routine run: done, unread
  not set, no notification. An ordinary task needs a non-empty report.
  `Routine.quietStreak` counts unchanged runs in a row.
- `SessionInfo.turnStarted` is when the running turn began, absent while no
  turn runs; `steps` counts its tool calls, or the last turn's once it ended.
- `Team.reporting` is `tree` or `peers`; `Member.reportsTo` defines the
  tree, while direct reports are derived. Rank and spirit are presentation
  and discovery, never authority. Optional member tools, skills, trust and
  budget narrow team policy. A missing skills restriction inherits;
  an explicit empty allowlist denies every skill.
- `TeamTask.parent` identifies the assigning task and `goal` preserves the
  root purpose. `resumeVersion` makes release of a held session single-use.
  A stopped or replaced source cannot reuse its queued message;
  `TeamMessage.retired` preserves that history without claiming delivery.
- `Member.runtime` selects `native`, `cli`, `command` or `webhook`. Local
  argv is never interpreted as a shell fragment. Environment secrets name
  separately consented Runtime-use vault entries, not plaintext values.
  A webhook's credential remains host-bound and daemon-mediated.
- `TokenUsage.unmetered` is not measured zero. ACP and command runs cannot
  independently establish token use; positive monthly caps fail closed
  when the relevant ledger includes unknown use.
- `Settings.teams` carries `perTeam`, `paused`, `routinesWake`, `defaults`
  (model, options, approvals, budget, notify for new teams),
  `approvalHours`, `quietHours` (`from` and `to` as `HH:MM`), `hidden`
  (null for the built-in list, `[]` for none) and `keepDays`
  (0, 30, 90 or 365).
- `Settings.agent` carries `model` (a route or `provider/model`, default
  `auto`), `options`, `plan` (Plan first for new Agent sessions),
  `approvals` and `idleStopMinutes`. `Settings.providers.models` configures
  Hyakki's own llama.cpp: `binary` (an absolute user-chosen path; empty
  discovers managed, PATH, login-shell PATH and common user installations),
  `folders` (additional absolute folders, scanned only by `local.found`),
  `idleMinutes` (1 to 120, default 5), `device`, `light` (the model for
  light work) and `remoteFirst` (false keeps models on this computer
  first in Auto, Fast and Smart). `providers.local` holds addresses for `llamacpp` and
  `ollama-local` only; `rashin-models` has none.
- `Settings.providers.decisions` (`DecisionRoutingSettings`) routes typed
  decisions: `route` `rules` (the default; nothing is started), `local`,
  `ember` or `auto`; `localAddress`, a loopback address only (default
  `http://127.0.0.1:11435/v1`); `localModel`; `budgetMs` (100 to 5000,
  default 1500); `allowHosted`; and `features`, at most 32 routes by
  feature (Oni's quick commands are `oni.reflex`). A call's own `route`
  wins, then its feature's, then `route`. `auto` asks the local model when
  one is set, then Ryoku Ember only with `allowHosted`; `ember` falls back
  to the local model when one is set. A model pin is refused on `rules` and
  `auto`, and a hosted attempt needs the reviewed `accountID`. A result
  that is not `resolved` means: carry on with built-in rules. An answer
  never grants a permission. `creditsExhausted` says the Ember attempt was
  refused for want of credits, so nothing hosted was dispatched; it is set
  even when the local model then answered, and a consumer reads it rather
  than the `notice` wording.
- Before the local route is asked, by `fastdecisions.evaluate` or
  `fastdecisions.local`, the daemon checks it: the address must be a
  loopback one, this computer must pass the hardware gate (at least 6 GiB
  of total memory and 4 physical cores, read by the local models' own
  `/proc` and `/sys` scan), and one `GET <localAddress>/models`, with no
  key, body, proxy, redirect or retry and at most 750 ms, must answer 200
  with a JSON `data` list. Otherwise the local route is refused and the
  `notice` gives the reason; no decision data is sent. Hyakki bundles,
  installs and starts no decision server.

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
| `account.changed` | `AccountState` | subscribers opened by the Hyakki app binary |
| `learning.changed` | `LearningChanged` (read that part again) | subscribers |
| `index.progress` | `IndexProgress` | subscribers |
| `index.changed` | `IndexInfo` | subscribers |
| `ui.focus` | `UIOpenParams` (each delivery activates independently, including identical requests; a bare open keeps the current route) | app subscribers |
| `notice` | `Notice` | subscribers |
| `screen` | `ScreenState` (`stopped` when it ended) | subscribers |
| `team` | `Team` | subscribers |
| `team.removed` | `TeamRemoved` | subscribers |
| `teams.structures_changed` | empty (invalidate and reload the local catalogue) | subscribers |
| `team.task` | `TeamTask` | subscribers |
| `team.task.removed` | `TeamRemoved` | subscribers |
| `team.approval` | `Approval` | subscribers |
| `routine` | `Routine` | subscribers |
| `routine.removed` | `TeamRemoved` | subscribers |
| `vault` | `VaultInfo` (never a value) | subscribers |
| `oni` | one Oni frame (see Oni) | connections that called `oni.open` only |

## Oni

Oni is Hyakki's character. Its policy and feature state live in this daemon;
Ryoku's surfaces are clients. A character profile changes how Oni looks and
talks, never what it may do: `internal/oni/feature` decides from settings
alone and cannot import the profile package (a test walks its imports).

### Settings

`Settings.oni` (`OniSettings`), every feature off on a machine that never
changed a setting:

| Key | Default | Meaning |
|---|---|---|
| `master` | `false` | Oni's own switch. It counts only while Hyakki's `enabled` is on (`EffectiveOni`). |
| `character` | `"oni"` | The selected character id: lowercase letters, digits and inner hyphens, at most 32. |
| `inspect` | `false` | Window inspection. |
| `disclosure` | `"session"` | Ask before a capture leaves the computer once a `session` or `always`. |
| `voice` | `false` | Talking to Oni: hold-to-talk, and, with `voiceSettings`, the wake phrase, open mic and spoken replies. |
| `chord` | `"ctrl+alt"` | The talk chord, stored canonical: `ctrl`, `alt`, `shift`, `super` in that order, then at most one key (`a`-`z`, `0`-`9`, `f1`-`f24`, `space`). It needs `ctrl`, `alt` or `super`. |
| `hands` | `"off"` | The hands stage: `off` or `browser`, Hands in one browser tab the user authorized. It is the Hyakki app's setting alone: a `settings.set` patch from any other program that names `oni.hands` in any spelling, or replaces `oni` whole, is `forbidden`, `off` included. `app` and `desktop` are refused with `unsupported` (not in this build), any other value with `bad_request`; a stored one of them loads as `off`. |
| `reflex` | `false` | Reversible quick commands. |
| `feeds` | `{"guard":false}` | The focus guard and its `words`, at most 32 of up to 40 characters, stored trimmed, lowercase and unique. |
| `size` | `"normal"` | Oni's size on screen: `small`, `normal` or `big`. |
| `reduceMotion` | `"follow"` | `follow` the desktop's reduced-motion setting, or force it `on` or `off`. |
| `courtesy` | `true` | Step aside while a window covers Oni or is fullscreen. |
| `hidden` | `false` | Keep Oni off screen without turning it off (`hyakki oni hide`, `show`). |
| `sounds` | `false` | Let the renderer play Oni's short motifs. It never plays while the `voice` frame says the microphone is live, and fails silent when it does not know. |
| `perchFollowsFocus` | `false` | A perched Oni moves to the window that takes focus. Off, a perch stays on its window. |
| `voiceSettings.wake` | `{"enabled":false,"phrase":"hey oni"}` | The wake phrase, a continuous microphone. `phrase` is 2 to 4 words of at least 2 letters, at most 40 characters, not starting with a filler sound; stored as compared, lowercase without punctuation; `""` is `hey oni`. |
| `voiceSettings.openMic` | `false` | Open mic, a continuous microphone, for at most the spec's `voice.press.openMicS`. |
| `voiceSettings.speakReplies` | `false` | Speak answers aloud. |
| `voiceSettings.mic` | `""` | A PipeWire source node name (letters, digits, `.` `_` `:` `@` `-`, not starting with `-`, at most 255 bytes); `""` is the default source. |
| `voiceSettings.stt` | `{"engine":"","model":"","language":""}` | `engine` `whispercpp` or `transcribecpp`, `model` a speech model id (lowercase letters, digits, `.` `_` `-`, at most 64), `language` a two or three letter code; `""` is the default for each. |
| `voiceSettings.tts` | `{"engine":""}` | Oni's voice: `system`, `clips` or `kokoro`; `""` is the character's `voice.engine`. |
| `voiceSettings.dictation` | `{"enabled":false,"mode":"clean","polish":"local"}` | The `voice.dictation` feature's switch; `mode` `exact`, `clean` or `polished`; `polish` `local` (a model on this computer) or `route` (the Ask route, disclosed before anything leaves). |
| `guide` | `{"enabled":false,"keep":false,"hints":false}` | `enabled` the `guide` feature, guided walkthroughs of the perched window; `keep` offers to keep a finished guide, asking each time, and replays kept guides on this computer; `hints` offers a kept guide while Oni is perched on its app. A switch that is no boolean is refused in a patch and loads `false` on its own. |
| `reflexSettings` | `{"routines":false,"reminders":false,"notes":false,"decisions":false}` | The switches under `reflex`, each counting only while `reflex` is on: `routines` offers to keep a run of quick commands, asking each time, and replays one with no model; `reminders` keeps reminders on this computer and says them when due (a timer needs only `voice`); `notes` keeps "remember that" as a memory fact through `memory.add`, never in a store of its own; `decisions` lets the typed decision route under `providers.decisions` classify what the rules miss. The route stays in `providers.decisions`, which only the app changes; a hosted decision also needs a press and its disclosure card answered by the app or a press. A switch that is no boolean is refused in a patch and loads `false` on its own. |
| `recall` | `{"enabled":false,"vault":false}` | `enabled` the `recall` feature: answers from imported chats, past conversations and memory facts; `vault` adds the Ryoku vault, read only, and no vault passage is used while it is off. App-only, as below. |
| `rewrite` | `{"enabled":false,"style":"grammar","voice":false}` | `enabled` the `rewrite` feature, rewriting the selection in place; `style` the style used when none is named: `grammar`, `friendly`, `formal`, `casual`, `polite`, `confident`, `shorter`, `longer`, `simpler`, `email` or `bullets`; `voice` writes in the user's own voice from their "My voice" notes, the only memory a rewrite reads. App-only. |
| `assignment` | `{"enabled":false,"style":"walkthrough","model":""}` | `enabled` assignment mode, which also needs `inspect`; `style` `walkthrough` (the steps) or `answers` (the answer alone); `model` the user's own pin for the solve, `""` the smallest local model first and never a paid one. App-only. |
| `mcp` | `{"enabled":false}` | The MCP bridge: `enabled` its one switch; `tools` the tools turned on, by id (`reminders_list`, `reminders_add`, `notes_search`, `notes_add`, `focus_status`, `notify`, `screen_ask`), an unknown id refused in a patch and dropped on load, a false one dropped; `clients` the programs the user approved, by the name each gives at hello, at most 16, each once. App-only. |

A patch with an invalid Oni value is refused; a stored invalid value falls
back to its default on load. The `recall`, `rewrite`, `assignment` and `mcp`
blocks are the Hyakki app's alone: a `settings.set` patch from any other
program that names any of them in any spelling, or replaces `oni` whole,
is `forbidden`.

`Settings.onboarding` (`OnboardingSettings{seen, done}`, both `false` on a
machine that never changed a setting) is the first-run tour's state: `seen`
once the tour was shown, `done` once it was finished or skipped. The tour
turns nothing on by itself. It is the Hyakki app's alone in the same way.

### Features

`Stage` is how far a feature reaches in this build, `egress` where its data
may go (`local` stays on this computer; `disclosed` may reach the chosen
model route and shows what leaves and where first), `consent` what the user
must do first.

| Feature | Stage | Setting | Egress | Consent |
|---|---|---|---|---|
| `vision.inspect` | `read-only` | `oni.inspect` | `disclosed` | `disclosure` |
| `voice` | `read-only` | `oni.voice` | `disclosed` | `user-initiated` |
| `voice.dictation` | `reversible` | `oni.voiceSettings.dictation.enabled` | `disclosed` | `user-initiated` |
| `reflex` | `reversible` | `oni.reflex` | `disclosed` | `announced` |
| `feeds.guard` | `reversible` | `oni.feeds.guard` | `local` | `per-action` |
| `reflex.routines` | `reversible` | `oni.reflexSettings.routines` | `local` | `per-action` |
| `reflex.reminders` | `reversible` | `oni.reflexSettings.reminders` | `local` | `announced` |
| `reflex.notes` | `reversible` | `oni.reflexSettings.notes` | `local` | `announced` |
| `reflex.decisions` | `reversible` | `oni.reflexSettings.decisions` | `disclosed` | `disclosure` |
| `guide` | `read-only` | `oni.guide.enabled` | `disclosed` | `disclosure` |
| `guide.keep` | `read-only` | `oni.guide.keep` | `local` | `per-action` |
| `guide.hints` | `read-only` | `oni.guide.hints` | `local` | `announced` |
| `memory.forget` | `reversible` | `oni.reflexSettings.notes` | `local` | `per-action` |
| `recall` | `read-only` | `oni.recall.enabled` | `disclosed` | `disclosure` |
| `rewrite` | `reversible` | `oni.rewrite.enabled` | `disclosed` | `disclosure` |
| `assignment` | `read-only` | `oni.assignment.enabled` | `disclosed` | `disclosure` |
| `hands.browser` | `guarded` | `oni.hands` | `disclosed` | `firm` |
| `hands.app` | `unavailable` | `oni.hands` | `disclosed` | `firm` |
| `hands.desktop` | `unavailable` | `oni.hands` | `disclosed` | `firm` |

A decision (`OniCapability.reason`) is checked in this order: `unknown-feature`,
`unavailable` (any feature not compiled in, whatever the settings say),
`oni-off` (Oni's or Hyakki's switch is off), `feature-off`, else `allowed`.
Oni's switch alone turns on no feature. The stage `guarded` drives input,
but only inside one scope the user picked and authorized. `hands.browser`
is allowed only while `oni.hands` is `browser` and `oni.inspect` is on, for
Hands looks at its tab as a guide looks at its window; otherwise its reason
is `feature-off`. The app and desktop hands executors are not in this
build: no setting reaches `hands.app` or `hands.desktop`.
A guide feature's switch counts only with the ones it builds on: `guide`
needs `oni.inspect`, `guide.keep` needs `guide`, and `guide.hints` needs
`guide.keep`; otherwise the reason is `feature-off`. The guide rows sit
before the hands rows; the rows before them keep their order and bytes.
Each `reflex.*` feature needs `oni.reflex` and its own switch; the four
rows sit after `feeds.guard` and before the guide rows, and every other
row keeps its bytes and order. A reflex acts only from the voice loop, so
without `voice` it does nothing. Making `hands.browser` guarded changed
its row alone; `hands.app` and `hands.desktop` keep their bytes.
The four Phase 16 rows sit after `guide.hints` and before the hands rows,
and every other row keeps its bytes and order. `memory.forget` shares the
notes switch (`reflex` and `reflexSettings.notes`), as correcting the last
note does; `recall` and `rewrite` need only their own switch;
`assignment` needs `oni.inspect` too, for it reads the perched window.

A voice ask is an Ask turn on the same route a typed one takes, with the
same disclosure, so `voice` takes the typed Ask's egress. `voice.dictation`
types into the window focused when it started, checked before every chunk,
and is not hands: only a polish on the `route` leaves the computer,
disclosed first, and never for text that looks like a secret.

Consent classes: `user-initiated` starts from the user's own act;
`disclosure` shows exactly what leaves and where, as `disclosure` says;
`announced` announces every action and one switch stops it; `per-action`
asks each time; `firm` sorts each action on its own: a safe step (filling
a field, an ordinary click, a scroll) is announced and run, and anything
risky, or anything the rules cannot place, is handed to the user, who
presses it themselves. Hands never presses it, no answer makes it, and a
spoken yes approves nothing. A consent question expires within 120 s.

### Methods

| Method | Params | Result |
|---|---|---|
| `oni.status` | | `OniStatus` (`enabled` Hyakki's switch, `master` Oni's, `active` both, `character` the one in use, `capabilities`, `characters` with their reports) |
| `oni.characters` | | `OniCharactersResult` (the shipped character first, each with `fallbacks` `{slot, reason, used}` or `errors`) |
| `oni.character.select` | `OniCharacterSelectParams` | `OniStatus` (app only) |
| `oni.character.reset` | | `OniStatus` (back to `oni`; app only) |
| `oni.spec` | | `OniSpecResult` (the motion spec as shipped, the feature table, frame kinds and the reserved calls, an empty list now that every Oni call is served) |

The presence methods are under Presence below; the perch and the screen
questions under Perch and Screen questions; the voice calls under Voice;
the guide, keep and pick calls under Guides, Keep and select-and-ask; the
reminders, routines and ledger calls under Reflex; the Hands calls under
Hands; rewrite, recall, the focus guard and assignment under Recall,
rewrite, the focus guard and assignment.

Selecting another character changes `character` and presentation fields only;
`capabilities` stay byte-identical. An invalid character cannot be selected
and the shipped `oni` cannot be shadowed.

### Frames

Frames are `{"kind", "data"}` (`feature.Frame`). They decode strictly: exact
field names, every field without `omitempty` present, no unknown or duplicate
keys, no nulls, no trailing data, values in range. Fixtures are in
`internal/oni/feature/testdata/frames`.

| Kind | Data | Direction |
|---|---|---|
| `state` | `StateFrame{state, reason?}`, a motion spec state id | daemon to renderer |
| `character` | `CharacterFrame{id, name, fallbacks?}` | daemon to renderer |
| `perch` | `PerchFrame{seq, attached, window?, app?, title?, corner?, output?, box?}`; `box` `Box{x, y, w, h}` is the window's frame in output-local logical pixels, present exactly when `attached`; `corner` `top-left` or `top-right`; detached names no window. `seq` is the perch's own counter from 1, never the strand's | daemon to renderer |
| `strand.target` | `StrandTargetFrame{seq, x, y, w, h, label?, mode?, region?, quiet?}`: the target as fractions of the perched window's `box`, or of the character's output when not perched; `mode` `point` (also when absent), `lasso`, `region` or `perch`; `region` is a `Box` present exactly in `region` mode; a `perch` target is the perched window's own frame, `x`=`y`=0, `w`=`h`=1, sent only while a perch is attached; `quiet` plays no sound. `seq` grows from 1 for as long as presence runs | daemon to renderer |
| `strand.arrive` | `StrandArriveFrame{seq, skipped?}`: the target landed, or `skipped` `covers-screen`, `off-output`, `covered` or `hidden` says it could not be drawn; sent as `oni.strandArrived` | renderer to daemon |
| `strand.retract` | `StrandRetractFrame{seq}`: rewind the strand that went out for `seq` | daemon to renderer |
| `bubble` | `BubbleFrame{seq, tone, text}`, tone `say`, `answer` or `warn` | daemon to renderer |
| `consent` | `ConsentFrame{id, feature, class, destination, route?, question, expiresMs}`; never for an unavailable feature, never `hosted` for a `local` one. A `guide.keep` question is the keep offer: `per-action`, `local`, no `route`, and `expiresMs` at most the spec's `guide.keep.chipS` (12 s). A `guide.hints` one carries a hint beside its `nudging` guide frame: `announced`, `local`, no `route`, `expiresMs` at most the spec's `guide.hints.retractS` (45 s). A `reflex.routines` one is the offer to keep a routine: `per-action`, `local`, no `route`, `expiresMs` at most the spec's `reflex.routines.chipS` (12 s). A `reflex.decisions` one is the card shown before a decision goes to the hosted route: `disclosure`, `hosted`, naming its `route`. `reflex`, `reflex.reminders` and `reflex.notes` announce what they do and never ask. A `hands.browser` one is a handoff card: Hands points at a risky control and waits for the user to press it themselves. It is `firm`, `local`, no `route`, its `id` starts `h-`, `expiresMs` at most the spec's `hands.consentTtlS` (120 s), and it has no yes: it ends when the user presses the control, on a stop, or when it lapses | daemon to client |
| `consent.answer` | `ConsentAnswerFrame{id, answer}`, `yes` or `no`; the daemon also sends one when a keep offer is kept, skipped, lapses or is dropped. `oni.consent.answer` refuses any `id` starting `h-` with `bad_request`: a hint is kept or skipped, and a Hands handoff card is never answered. An approval card (`ta-`, `tp-`) is answered with one of its `options` | client to daemon, and daemon to client for a keep offer |
| `voice` | `VoiceFrame{phase, text?, source?}`, phase `idle`, `armed`, `listening`, `transcribing` or `speaking`; `source` `chord`, `toggle`, `wake`, `open` or `dictation`. `idle` carries neither; `armed` (the microphone open between presses) carries no text and the source `wake` or `open`; `listening` no text; `transcribing` the words heard and `speaking` the line spoken, at most 600 characters. A `dictation` source carries no text in any phase and never speaks | daemon to renderer |
| `hands` | `HandsFrame{stage, active, reason, scope?, phase?, step?, max?, pause?, end?}`; `stage` `off` or `browser` (`app` and `desktop` are not in this build). Inactive it carries only `reason`, the `hands.browser` decision: `oni-off`, `feature-off`, or `allowed` while Hands may work but runs no task (never with stage `off`). Active is a running task: stage `browser`, reason `allowed`, `scope` `browser`, `phase` `working`, `waiting` (a handoff card waits for the user) or `paused`, `max` 1 to the spec's `hands.budget` steps and extra (25), `step` 0 to `max`, and `pause` exactly while paused: `focus`, `window`, `navigation`, `hidden`, `covered`, `dialog`, `popup` or `download`. The later scopes `window`, `nested` and `desktop` and their pauses `output`, `title`, `geometry`, `self`, `private`, `key` and `presenter` are named on the wire and refused in a frame. The frame that ends a task is inactive, and every indication goes with it. Only that frame carries `end`, how the task ended: `done` (the goal was reached and the page agreed), `asked`, `answered`, `stopped`, `budget`, `repeat`, `unconfirmed`, `refused`, `expired`, `away`, `timeout`, `failed`, `closed` or `unread`; it may have stage `off` when turning Hands off stopped the task. The presenter sounds `done-notes` only for `done` after a task of at least the spec's `sound.cues.handsDoneMinS` | daemon to client |
| `gesture` | `GestureFrame{seq, id}`, a motion spec gesture id played once | daemon to renderer |
| `placement` | `PlacementFrame{seq, corner, size, outputs, active?}`: `corner` `tl`, `tr`, `bl`, `br` or `""` for the saved spot, `size` `small`, `normal` or `big`, `outputs` at most 16 `{id, fx, fy}` with unique ids and fractions in [0, 1] | daemon to renderer |
| `motion` | `MotionFrame{reduce, batterySaver, courtesy, sounds, chord?}`, `reduce` `follow`, `on` or `off`; `batterySaver` is power-profiles-daemon's `power-saver` profile, which the daemon follows on the system bus while presence runs (false without it); `sounds` is `oni.sounds`; `chord` is `oni.chord` in canonical form while `oni.voice` is on and absent otherwise, so a presenter's own key reader runs only while voice is on | daemon to renderer |
| `level` | `LevelFrame{mic, speech}`, each in [0, 1]: the microphone's and Oni's speech loudness, which the renderer feeds its motion. Sent at most the spec's `voice.levels.maxHz` (15) times a second, only while the voice phase is `listening` or `speaking` and Oni is shown; never in a snapshot | daemon to renderer |
| `guide` | `GuideFrame{seq, phase, step?, kicker?, text?, plan?, actions?, replay?, target?}`, phase one of the spec's `guide.phases`: `idle`, `reading`, `pointing`, `waiting`, `explaining`, `keeping`, `done`, `winding` or `nudging`. `idle` carries nothing but `seq`; `reading`, `keeping` and `winding` no `text`; the others their line, at most 200 characters on one line; `pointing` and `waiting` the `target` strand `seq` they point with, `explaining` may, the rest none. `step` counts from 1 to the spec's `guide.maxSteps` (12), and is required but in `done`, `winding` and `nudging` (none in `nudging`). `kicker` is the line above the step, at most 48 characters. `plan` `GuidePlan{done?, current?, next?}` lists at most the spec's `session.planDoneMax` (6) steps done and `planNextMax` (3) ahead, never empty and never in `winding` or `nudging`. `actions` are what the card offers: `reading` `stop`; `pointing`, `waiting` and `explaining` `next`, `stop`, `circle` (circle only with a `target`); `keeping` `keep`, `skip`; `nudging` `next`, `skip`; none twice, never empty. A guide frame has no field for a picture, text read off the screen, a window title or the user's words | daemon to renderer |
| `pick` | `PickFrame{seq, armed}`: `armed` on `oni.pick arm`, disarmed on the click, a cancel, an expiry or a stop. The presenter maps one transparent input surface per output only while the latest pick frame is armed, sends `oni.pick point` once and disarms on its own. Never in a snapshot, so a presenter that returns starts disarmed | daemon to renderer |
| `approval` | `ApprovalFrame{id, open, title?, scope?, options?}`: one Yokai approval card beside Oni, the oldest waiting first. `ta-N` is a team member's Hands request (`scope` `nested` or `window`; `options` `once`, `no`, `app`); `tp-N` is a permission a hosted harness raised in its team task's conversation (no `scope`; a read- or write-tier one offers `once`, `always` where the request offers it, `no` and `app`; a danger-tier or untiered one offers only `app`). `app` leaves it to the Hyakki app: it decides nothing and takes the card off the pill. A closed frame carries only its `id`. Never in a snapshot: the waiting card is sent again to a presenter that opens | daemon to renderer |
| `focus` | `FocusFrame{phase, leftMs, closed}`: the focus guard, `phase` `off`, `armed` or `done`. `off` carries `leftMs` and `closed` 0; `armed` the time left, 1 ms to the spec's `focus.maxMinutes` (480); `done` no time left and how many windows or tabs it closed. It never names a feed, a window, a title or an app; the renderer picks its look from its own palette roles | daemon to renderer |

A `state` frame with reason `oni-off` tells a renderer to exit; reason
`hidden` to unmap until a state without it. Asleep, hidden and off the daemon
arms no timer.

The Phase 16 cards are `consent` frames whose `id` starts with their
feature's prefix, so an answer is routed by id alone: `forget-` for
`memory.forget` (`per-action`, `local`, the facts a spoken forget found,
at most the spec's `memory.forgetMax`), `guard-` for `feeds.guard`
(`per-action`, `local`, the feed and the app before each close),
`recall-` for `recall` (`disclosure`, always `hosted` with its `route`:
the passages' titles; a local recall asks nothing), `rewrite-` for
`rewrite` and `assign-` for `assignment` (`disclosure`, `local` or
`hosted`). Each lapses within 120 s and is answered by
`oni.consent.answer` or by a press; a spoken yes from the wake phrase or
open mic answers none of them. A card's question is all a frame carries:
the selection, the passages and the picture go only in the call's result
to the app or the CLI.

### Presence

The `oni` event carries one frame (`feature.Frame`) to the connections that
called `oni.open`, and to no other: it never reaches subscribers, never
passes the broadcast and never resets the idle clock. Presence runs only
while a connection has it open; the last `oni.close` or disconnect stops it.
The `oni.open` reply reaches the connection before any `oni` frame, and every
frame after it is newer than the snapshot the reply carries.

| Method | Params | Result |
|---|---|---|
| `oni.open` | `OniOpenParams{renderer}`: `native`, `ryoku` or `observer` | `OniOpenResult{presenter, reason?, frames}`: `frames` are `state`, `character`, `motion`, `placement`, the `perch` while one is attached, `voice`, the `guide` while one runs, and the `hands` frame while a Hands task runs, followed by the handoff `consent` card while the task waits on it, in that order, or only the `state` (`oni-off`) while Oni is off. An inactive `hands` frame is never in a snapshot |
| `oni.close` | | empty |
| `oni.place` | `OniPlaceParams{output, fx, fy, corner}`, every key present: a corner pins Oni and keeps every saved spot; `corner` `""` saves `fx`, `fy` for `output`, which becomes active | empty; the change arrives as a `placement` frame |
| `oni.interact` | `OniInteractParams{kind}`: `poke`, `wake`, `drag` or `drop` | empty |
| `oni.character.assets` | `OniCharacterAssetsParams{id}` | `OniCharacterAssets{id, files}`: a valid character's `profile.json` and each SVG it declares, as validated at load, keyed by path in its folder; at most 4 MiB together. `not_found` for no such character, `bad_request` for one that does not load |
| `oni.strandArrived` | `OniStrandArrivedParams{seq, skipped?}`, as `StrandArriveFrame` | empty, whether or not `seq` was the one awaited |

- Only a connection whose hello says `surface` may open as `native`, and one
  presenter draws at a time: the others get `presenter: false` with reason
  `presenter-busy`. `ryoku` is answered `unavailable` in this build,
  `observer` always `observer`, and while Oni is off every open is `oni-off`.
  Non-presenters still receive the frames.
- A verified app opening an `observer` connection without a session also
  receives still-pending focus-guard, memory correction/forget and hosted
  recall disclosures in its snapshot. Their TTL is the remaining original
  TTL, never a fresh approval window. Answers, expiry, Oni off and close
  remove them. Other observers and general snapshots receive no such replay.
  The observer remains read-only: the app answers on its ordinary connection.
- A `surface` connection may call only `status`, `oni.status`,
  `oni.characters`, `oni.spec`, the presence methods above, `oni.perch`,
  `oni.detach`, `oni.overlay`, `oni.talk`, `oni.speak`, `oni.dictate`, and,
  with the actions under Guides below, `oni.guide`, `oni.skip` and
  `oni.pick`; anything else is `forbidden`. It never
  keeps the daemon up: idle exit ignores it, and a surface that loses the
  daemon while Oni sleeps redials when touched.
- `oni.open`, `oni.close`, `oni.place`, `oni.interact`, `oni.strandArrived`
  and the perch and screen question calls below are `forbidden` to a
  session's tools through the tool host and on a team session's socket.
  `oni.character.select` and `oni.character.reset` stay app only.
- The state follows the sessions: a turn waiting on the user is `ready`, one
  in a tool `working`, one with the model `thinking`; a turn that ends plays
  `done`, one that fails plays `error` and stays `error` until the next turn
  or touch, else `idle`. After the spec's `sleep.idleMs` (90 s) with nothing
  running, no strand target awaited and no touch, the daemon sends `gesture
  sleep` then `state sleeping` and arms nothing more. A touch, a session or a
  strand target then sends `gesture wake` and the state it wakes to.
- The `voice` frame is the microphone's phase, and presence is the one place
  it comes from; it is `idle` until the voice pipeline reports. A phase goes
  out before anything it sets off, so a renderer's sound gate closes before
  a `wake` motif. The spec's `voice.phases` derive the state: `listening`
  shows `listening`, `transcribing` `thinking` and `speaking` `speaking`,
  ahead of the sessions' state; each wakes a sleeping Oni and keeps it from
  sleeping. `armed` derives no state; the spec gives it the persistent mic
  mark, which the native presenter draws beside the character while the
  voice frame says `armed`, and alone where the character would be when Oni
  is hidden. A
  `level` frame that comes sooner than `1/maxHz` after the last, or outside
  `listening` and `speaking`, is dropped; no timer holds it.
- The arrival handshake: the daemon sends `strand.target` with the next
  `seq` and waits for that one. Only the presenter may call
  `oni.strandArrived`; any other connection gets `forbidden`. A report for
  the awaited `seq` advances the waiting work once, landed or `skipped`; a
  stale, repeated or unknown `seq` is answered and changes nothing. A newer
  target supersedes the awaited one, and a `strand.retract`, the presenter
  leaving, Oni turning off, or the spec's `session.arrivalTimeoutMs` (10 s)
  with no report cancels it without advancing. That timer is armed only while
  a target is awaited. With no presenter the daemon sends no target at all.
  Strand frames and reports never reset the idle clock.
- Where Oni stands is saved in `$XDG_STATE_HOME/hyakki/oni-placement.json`,
  `{"version": 1, "corner", "outputs", "active"?}`, read as strictly as a
  frame; a file that does not read is ignored and replaced by the next save.
  Before any save Oni is pinned `br`.

### Perch

| Method | Params | Result |
|---|---|---|
| `oni.perch` | `OniPerchParams{window?, output?, x?, y?, hover?}`: a seam `window` id, or, from the presenter's drop, an `output` and a point `x`, `y` on it in output-local logical pixels the daemon hit-tests (floating over tiled, then the most recently focused). `hover` only hit-tests | `OniPerchResult{seq?, window?, app?, title?, output?, corner?, box?, reason?}`. A hover never fails for want of a window; `reason` says why there is none Oni could perch on |
| `oni.detach` | `OniDetachParams{reason?}`: `user` (the default), or from the presenter `dragged-off` or `hidden` | empty |
| `oni.overlay` | `OniOverlayParams{output, rects}`: the presenter's own rects, at most 16, in output-local logical pixels; `[]` clears them | empty |

- Perch and detach reasons are a closed set: `closed`, `user`,
  `dragged-off`, `output-gone`, `hidden`, `no-geometry`, `oni-off`,
  `presenter-lost`. A refused perch is an error whose `reason` is one of
  them: `not_found` `closed` for a window that is gone or a drop on empty
  desktop, `unavailable` `no-geometry` for one the desktop does not place
  (niri's tiled windows), `hidden` for one off screen or while Oni is
  hidden, `presenter-lost` with nothing drawing Oni, `disabled` `oni-off`.
  A refused perch sends no frame.
- A perch sends the `perch` frame, then one `perch` strand target. A move
  or resize sends a new `perch` frame with the next `seq` and the new `box`
  and nothing else; a new output sends the frame and a new `perch` target.
  A close, a hide, a lost output, `oni.detach`, the presenter leaving,
  Oni hidden or turned off detach: `strand.retract` for the strand out,
  then a detached `perch`. With `perchFollowsFocus` on, focus moving to a
  window Oni can perch on moves the perch there.
- The desktop is read first when a perch or a hover asks. Its window watch
  and geometry poll run only while a perch is attached and stop with it;
  with no perch nothing is watched, polled or armed, and perching never
  resets the idle clock or changes `capabilities`.
- `oni.perch`, `oni.detach` and `oni.overlay` are the presenter's, the app's
  and the CLI's (`oni.overlay` the presenter's alone); a surface that is not
  the presenter, a connection that opened `oni` as an observer, the bar and
  the Hub get `forbidden`, as do a session's tools and a team session's
  socket.

### Screen questions

| Method | Params | Result |
|---|---|---|
| `oni.inspect` | | `OniAskResult` |
| `oni.ask` | `OniAskParams{text}` | `OniAskResult` |
| `oni.consent.answer` | `OniConsentAnswerParams{id, answer}`, `yes` or `no` | `OniAskResult` |

`OniAskResult{status, consent?, answer?}`: `status` `consent`, `answered` or
`declined`. `OniConsent{id, destination, route, path, disclosure, scopeExact,
question, preview, width, height, expiresMs}`: `destination` `local` or
`hosted`, `route` the `provider/model` that will answer, `path` the capture
path that ran (`toplevel`, `niri-window`, `output-crop`, `portal`, `x11`) and
`disclosure` what it means, `preview` a JPEG thumbnail of the exact picture,
`width` × `height` the picture that would be sent. `OniAnswer{text, mode,
refused?, point?, seq?, route, destination, path}`: `point` is the box
pointed at in fractions of the window and `seq` the strand target sent to it;
`refused` a refusal code (`sensitive` for a private field) with `text` the
line shown instead.

- `vision.inspect` must be allowed (`disabled` with the decision's `reason`
  otherwise) and Oni perched (`conflict`). Nothing is captured before
  `oni.inspect` or `oni.ask`, and nothing when no model reads pictures
  (`no_vision`). The route is planned first and pinned: the request goes to
  that model or fails, never elsewhere. An Ember route needs the app
  (`forbidden`).
- Only the perched window is pictured, in memory: never to disk, a session or
  an event. On the crop path the presenter's `oni.overlay` rects are painted
  white.
- With `disclosure` `always`, or the first time this route is used while
  presence runs with `session`, the result is `consent` and a `consent` frame
  (`vision.inspect`, `disclosure`, its `destination` and `route`, no
  picture) goes out; no byte leaves before `yes`. The question is answered
  by the connection that asked or the app; it ends, and its picture is
  zeroed, on `no`, after 120 s, when the asker disconnects, on a new
  question, a detach or re-perch, or when looking is turned off or the
  disclosure setting changes. Otherwise the result is `answered` at once.
- The model gets one JPEG (long side 1568 at quality 80 to point, 1280 at 78
  to answer in words) and no tools. Its reply only moves the strand and
  fills the `bubble`; a point at a private field is refused in code. The
  picture is zeroed once the answer is in.
- `oni.inspect`, `oni.ask` and `oni.consent.answer` are the app's and the
  CLI's alone: the presenter, other surfaces, observers, the bar, the Hub, a
  session's tools and a team session's socket get `forbidden`. The one
  exception is an approval card (`ta-`, `tp-`): only the presenter and the
  app answer it, with an option the card offers; the CLI and anyone else get
  `forbidden`.

### Voice

| Method | Params | Result |
|---|---|---|
| `oni.talk` | `OniTalkParams{action}`: `start`, `stop` or `cancel` one press | `OniVoiceResult{phase}` |
| `oni.speak` | `OniSpeakParams{action}`: `stop` | `OniVoiceResult{phase}` |
| `oni.dictate` | `OniDictateParams{action, mode?}`: `start`, `stop` or `cancel` one dictation into the window focused at `start`; `mode` `exact`, `clean` or `polished` overrides the setting on a `start` only | `OniDictateResult{phase, mode?, fallback?, landed?, rest?, unsure?, refused?}`: on `stop` the mode that ran, why a polish fell back, the characters typed, the text not typed (only here, to copy), the characters of a chunk the typing tool failed partway through, and why the window was refused (`closed`, `unfocused`, `moved-output`, `hidden`, `unavailable`) |
| `oni.voice.test` | `OniVoiceTestParams{mic?, ms?}`: a source node, `""` for the setting; 100 to 5000 ms, 0 for 1500 | `OniVoiceTestResult{mic, levels, sources}`: the levels in [0, 1] for Settings' meter and the sources the chooser offers (`OniMicSource{name, desc}`, no monitors). Nothing is recorded to disk |

- `oni.talk start` opens the microphone for one press: what Oni is saying
  stops mid-word and a new turn begins. The presenter's call is the held
  chord, any other caller's a toggle. `stop` answers `transcribing` when
  speech was heard (the transcript and the answer follow on frames) or the
  rest phase; `cancel` drops the press. A press shorter than
  `voice.press.tapDropMs` is dropped, one held past `maxHoldS` is cut.
- Voice off (`disabled` with the decision's `reason`), no recorder, no sound
  or a failed capture, no speech engine or no downloaded model are refused
  with `unavailable` and reason `no_recorder`, `no_microphone`,
  `mic_failed`, `no_speech_engine` or `no_speech_model`; nothing is started.
- The frames of one press go out in order: `listening`, `transcribing` with
  each partial, `transcribing` with the final transcript, `speaking` with an
  acknowledgement when the answer needs a model, `speaking` with the answer,
  then `idle`, or `armed` while the wake phrase or open mic listens. With
  `speakReplies` off nothing is spoken and the answer goes to the `bubble`.
- `oni.talk`, `oni.speak` and `oni.dictate` are the app's, the CLI's and the
  presenter's (its chord reader is a key source); `oni.voice.test` the app's
  and the CLI's. A surface that is not the presenter, a connection that
  opened `oni` as an observer, the bar, the Hub, a session's tools and a team
  session's socket get `forbidden`, decided by the gate before any handler.
- A voice ask is a temporary read-only Ask session with origin `voice`, run
  like a typed Ask on the Ask route; it uses memory and never teaches it.
  The model hears the voice block (first sentence at most five words) with
  the question; the first sentence of the reply is said.
- Only a press answers a consent card by voice. Wake-phrase and open-mic
  speech saying yes or no hears "Press the key to answer." and leaves the
  card waiting.
- Wake phrase and open mic arm with their switches and keep the daemon up
  only while armed. A wake session lasts `voice.wake.sessionS`; wake
  disarms after `armedMaxS` and open mic after `voice.press.openMicS`, each
  turning its switch off. Speech that does not start with the wake phrase is
  dropped where it is heard: no frame, event, log line or file holds it.
  Oni's own speech heard back within the echo window is dropped too.
- A dictation's words go only into its window, checked before every chunk
  of at most 40 characters, or back in `rest`: never on a frame, an event,
  a log or a ledger line, and never on the clipboard. A polish that fails
  falls back to clean and names why in `fallback`.
- `oni.voice.test` runs with voice off, so Settings can try the microphone
  first.

### Guides, Keep and select-and-ask

The daemon serves these calls; their params decode strictly.

| Method | Params | Result |
|---|---|---|
| `oni.guide` | `OniGuideParams{action, goal?}`: `start` with a `goal` of 1 to 300 characters on one line, or `next`, `stop` or `circle` with none | `OniAskResult` |
| `oni.keep` | `OniKeepParams{id}`: the keep offer's id, or the routine offer's (`OniKeepResult.entries` then counts the routines kept) | `OniKeepResult{kept, entries}` |
| `oni.skip` | `OniKeepParams{id}`: the keep offer's, the hint's or the routine offer's id, from its `guide.keep`, `guide.hints` or `reflex.routines` consent frame | empty |
| `oni.pick` | `OniPickParams{action, output?, x?, y?, text?}`: `arm`, with an optional question `text` of at most 2000 characters; `point`, the one click, with `output` and `x`, `y` in its output-local logical pixels and no `text`; `cancel` and `wait` with nothing | `OniAskResult` |

- `OniAskResult` gains `status` `ended` (a guide finished or stopped, or a
  pick cancelled; `answer`, when present, carries the closing line) and
  `armed` (a pick waiting for its click), and `keep`
  `OniKeepOffer{id, question, expiresMs}`, the offer to keep a guide that
  just ended, which `oni.keep` or `oni.skip` answers and which lapses to skip
  after the spec's `guide.keep.chipS`. A hint is taken up with `oni.guide`
  `next` and let go with `oni.skip` on its id. `oni.consent.answer` never
  answers a keep offer or a hint.
- `OniAnswer` gains `step`, `done` and `replay` (a guide's step, its last,
  and a step replayed from a kept guide with no model) and `walk` (a
  select answer offering to walk through it); `mode` may be `guide` or
  `select`.
- A guide and a keep need their features (`disabled` with the decision's
  `reason` otherwise); a pick needs `vision.inspect` and a presenter
  drawing Oni, and an armed pick lapses after 120 s.
- All four are the app's and the CLI's. The presenter may send only
  `oni.guide` `next`, `stop` and `circle`, `oni.skip`, and `oni.pick`
  `point` and `cancel` (its overlay takes every click and Esc never reaches
  it); its other actions and `oni.keep` are `forbidden`, and its params
  are read strictly by the gate (`bad_request`). Another surface, an
  observer, the bar, the Hub, a session's tools and a team session's socket
  get `forbidden`. Calls without a type take `{}`.
- `oni.guide start` returns once step 1 is pointed at (`answered`, with the
  step's `text`, `point` and strand `seq`), once a step's picture needs the
  user's yes (`consent`; `oni.consent.answer` then returns the guide's next
  result the same way, and `no` ends the guide as `declined`), or once the
  guide ended. `next` and `circle` return when the guide rests again: the
  next step, a line explaining an unchanged screen, or the end with any
  keep offer. A `next` before the strand lands is held until it does. A
  step advances only after its strand landed; a stale or superseded landing
  advances nothing. Each step's picture is asked about as `oni.ask`'s is,
  every step with disclosure `always`, once per route with `session`; a
  kept guide replays on this computer with a capture and its words read
  locally, nothing sent and nothing asked. A read gets 75 seconds
  (`guide.readTimeoutS`) from when it starts; while its picture waits on
  the user's yes the deadline is held, bounded by the consent's own 120 s,
  and a yes gives the read a whole 75 seconds from then. A guide takes at
  most 12 steps. Dismissing, detaching, the window closing, Oni off, the
  presenter leaving, a settings change and a stale or unanswered consent
  each wind the strand back, cancel the read, clear the pictures and drop
  any offer; a reply that comes after changes nothing. The closing frame
  stays while its line is read, then an `idle` guide frame ends the card.
- `oni.keep` writes the offered guide and nothing else writes the store;
  `oni.skip`, the offer lapsing and a newer guide write nothing. An offer
  answered or dropped is followed by a `consent.answer` frame with its id.
- `oni.pick point` perches Oni on the window under the click, reads the
  words beside the spot on this computer and refuses a private field
  before any model call (`answered`, the answer's `refused` `sensitive`);
  with no text reader it is `unavailable` with reason `ocr-unavailable`. A
  click within the spec's `guide.pickRepeatGuardMs` of the last pick is
  ignored (`ended`). A select answer with `walk` puts a `nudging` guide
  frame with `next` beside Oni for the spec's `guide.hints.retractS`;
  `oni.guide next` then starts a guide toward the pick's question.
- `oni.pick wait`, from the connection that armed the latest pick, holds
  until its click, cancel or lapse and returns what the click came to: a
  `consent` that connection answers with `oni.consent.answer` as
  `oni.ask`'s caller does, an answer, a refusal, or `ended` for a cancel, a
  lapse, an ignored repeat or Oni stopping. A click before the wait is kept
  for it. Any other connection, or a second wait, is `conflict`; a
  connection that closes ends its wait.
- By voice, only a press answers a guide step's consent question or a
  keep offer, and only a press's "next" moves a guide on or takes a walk
  or hint offer; wake-phrase and open-mic speech change nothing. "Stop" and
  "circle it" work from any source. With Oni perched and `guide` allowed, a
  spoken question about doing something on screen starts a guide instead
  of an Ask.

### Reflex

All three are registered and served; their params decode strictly.

| Method | Params | Result |
|---|---|---|
| `oni.reminders` | `OniRemindersParams{action, id?}`: `list`, `cancel` with the reminder's `id` (16 lowercase hex digits) or `cancel_all`; only a `cancel` names an `id` | `OniRemindersResult{reminders, noWake?}`: what is kept after the call, timers included, soonest first, each `OniReminderInfo{id, kind, due, text?, late?}`, `kind` `reminder` or `timer`, `text` the reminder's own words; `noWake` says the shared wake unit cannot be armed here, so the daemon stays up while a timer or reminder of a kind that is on is pending |
| `oni.routines` | `OniRoutinesParams{action, name?}`: `list`, `run` (the most recently used routine, or the one `name` names), `forget` with a `name`, or `forget_all`; a `name` is one trimmed line of at most 60 characters | `OniRoutinesResult{routines, ran?}`: what is kept after the call, most recently used first, each `OniRoutineInfo{name, steps, saved, used}`; `ran` says a `run` replayed its routine |
| `oni.reflex.log` | | `OniReflexLogResult{rows}`: the reflex ledger, newest first, each `OniReflexRow{at, intent, route, ms, confidence, outcome}`, `route` `rules`, `local` or `ember`, `outcome` `done`, `unsupported`, `refused`, `cooldown`, `skipped` or `failed` |

- `oni.reminders` needs `reflex.reminders`, `oni.routines`
  `reflex.routines` and `oni.reflex.log` `reflex` (`disabled` with the
  decision's `reason` otherwise). A `cancel`, `run` or `forget` naming
  nothing kept is `not_found`.
- No reminder, routine or ledger row carries a transcript: a reminder keeps
  only its own words, a routine its steps, a row no words at all.
- All three are the app's and the CLI's. The surface drawing Oni, another
  surface, an observer, the bar, the Hub, a session's tools and a team
  session's socket get `forbidden`; none is a presenter action.
- A routine offer is answered with `oni.keep` (keep it) or `oni.skip` (let
  it go), by its `reflex.routines` consent frame's id, or by a spoken yes
  or no from a press; the surface drawing Oni may only skip. Skipping, the
  offer lapsing, routines or voice turning off and a newer run write
  nothing. The hosted decision card (an id starting `d-`) is answered with
  `oni.consent.answer` from the app alone, or by a spoken yes or no from a
  press; a yes binds the account the card named, if it is still the one
  signed in, and a no keeps the card from showing again for that account.
- No call runs a single quick command: a reflex acts only on speech, and
  Settings says so while `voice` is off. `oni.routines run` replays only a
  routine the user kept, through the same executors and caps.
- Quick commands from wake or open mic never run a press-only intent (a
  note, a routine replay or a setting): Oni says "Press the key for that."
  Open-mic speech is planned only when it names the character. A setting
  changed by voice, which needs a press, can only narrow what Oni does or
  change how it looks or sounds (sounds on or off, reduced motion on or
  off, hide, voice off, quick commands off).
- A timer needs only `voice`; a reminder needs `reflex.reminders`. Both
  are kept in the reminders store and the daemon arms one timer, for the
  next one due.
- With decisions on, an utterance the rules miss goes to the typed
  decision route as feature `oni.reflex` with one `intent` question, a
  choice over the intents a press is not needed for, plus `none`; the
  state is the utterance alone. The answer counts only at the spec's
  `reflex.decisions.minConfidence` and `minMargin`, and anything else goes
  to the slow path. A hosted decision also needs a press, its card answered
  for the account signed in now, and fewer than `hostedPerMin` this minute.
  On `creditsExhausted` Oni says "Credits ran out; rules only." once and
  asks no hosted decision again until the account changes or the card is
  answered again.

### Hands

Hands works in one browser tab the user authorized, in a Hands browser
Hyakki starts on its own profile. Both calls decode strictly; they are
served once the daemon registers them.

| Method | Params | Result |
|---|---|---|
| `oni.hands` | `OniHandsParams{action, tab?, goal?}`: `open` the Hands browser, list its `tabs`, `authorize` the one `tab` names (its id: 1 to 64 letters and digits), `start` a task toward `goal` (one line of at most 300 characters) in the authorized tab, `stop` it, `resume` a paused one, report the `status`, or `forget` the Hands browser, deleting its profile (`conflict` while it is open); only `authorize` names a `tab` and only `start` a `goal` | `OniHandsResult{state, browser?, tabs?, origin?, step?, max?, pause?, line?}`: `state` `closed` (no Hands browser), `open` (no tab authorized), `authorized`, `working`, `waiting` or `paused`; `browser` the Chromium-family program Hands would open, by its command's base name (lowercase letters, digits, `-` and `_`, at most 32), on every answer and absent when none is installed; `tabs` with `tabs` only, each `OniHandsTab{id, origin, title}`, `origin` `scheme://host[:port]` of an `http` or `https` page or `file://`, `title` page text on one line of at most 200 characters and never kept; `origin` the authorized tab's; `step` of `max` and `pause` as the `hands` frame has them |
| `oni.hands.log` | | `OniHandsLogResult{rows}`: the Hands ledger, newest first, each `OniHandsRow{at, task, step, tool, outcome, reason?, ms}`, `tool` the tool asked for, `outcome` `done`, `handoff`, `refused` or `failed`, `reason` the rule's code; no row holds a control's name, page text or a typed word |

- Both need `hands.browser` (`disabled` with the decision's `reason`
  otherwise).
- Who sends what: the app every action, and it alone `forget`. The CLI
  `open`, `tabs`, `stop` and `status`, never `authorize`, `start`,
  `resume` or `forget`. The surface drawing Oni `authorize`, `start`,
  `stop` and `resume`, from its pill and card, and nothing else.
  `oni.hands.log` is the app's and the CLI's. Another surface, an
  observer, the bar, the Hub, a program that is not the app, a session's
  tools and a team session's socket get `forbidden` for every action,
  `stop` included.
- Each step's picture, page text and control names go out behind an
  `inspect` consent card whose id starts `hs-`, answered with
  `oni.consent.answer` by the app or the connection that started the
  task; a `consent.answer` frame closes it, as does the task ending.
- A safe step is announced and run; anything risky is handed to the user
  with a `hands.browser` consent card that has no yes. No call and no
  spoken word presses it for them.

### Recall, rewrite, the focus guard and assignment

Each call decodes strictly; it is served once the daemon registers it.

| Method | Params | Result |
|---|---|---|
| `oni.rewrite` | `OniRewriteParams{action, id?, style?, instruction?}`: `start` reads the selection (the primary selection, never the clipboard) and shows it with the route on a `rewrite-` card; `apply` answers that card yes, so the text is sent and a one-paragraph result typed back over the selection; `undo` puts the original back; `cancel` drops the card. `apply`, `undo` and `cancel` name the card or record by `id`, a short code; only `start` takes a `style` (one of the setting's styles, or `custom` with an `instruction` of one trimmed line of at most 300 characters) | `OniRewriteResult{status, id?, style?, chars?, destination?, route?, text?, landed?, rest?, refused?, undoVerified?, expiresMs?}`: `status` `consent`, `applied`, `same` (it already read well; nothing typed), `copy` (offered to copy instead of typed, as any result of more than one paragraph is), `undone`, `kept` (undo could not be checked, so the original is offered to copy), `declined` or `refused`; `text` is the selection on `consent`, the result on `copy` and the original on `kept`, and only this result carries it; `refused` `off`, `no-selection`, `too-long`, `secret`, `private-window`, `self`, `terminal`, `changed`, `window`, `facts-added`, `model`, `needs-press`, `no-undo` or `unknown-id`; `undoVerified` is true only where putting the original back was checked for that kind of window |
| `oni.recall` | `OniRecallParams{action, text?}`: `ask` a question (one line, at most 2000 characters) or `know` what Oni knows about the user, or about the topic `text` names (at most 300) | `OniRecallResult{status, consent?, answer?, sources?, route?, destination?, refused?}`: `status` `consent` (a hosted route's card waits: `OniRecallConsent{id, route, question, titles, expiresMs}`, answered with `oni.consent.answer`), `answered`, `empty` or `refused`; each source `OniRecallSource{kind, title, from?, at?}`, `kind` `chat`, `vault`, `conversation` or `fact`, never the passage itself |
| `oni.recall.import` | `OniRecallImportParams{paths}`: 1 to 16 ChatGPT, Claude or Gemini export files, each an absolute, clean path ending `.json` or `.zip`, each named once | `OniRecallImportResult{chats, skipped, sources, root, refused?}`: the chats imported and skipped, `{from, chats}` per source (`chatgpt`, `claude`, `gemini`), and the index root they are kept under; the export files are read and left where they are |
| `oni.focus` | `OniFocusParams{action, words?, minutes?, untilApp?}`: report the guard's `status`, `arm` it for `minutes` (1 to the spec's `focus.maxMinutes`) or until the app `untilApp` names closes (one trimmed line of at most 128 characters), never both, `stop` it, or `back` to lift it; `words`, cleaned as `oni.feeds.words` is and refused when cleaning drops any, replace the Settings words for this guard; only `arm` takes any field | `OniFocusResult{armed, leftMs?, untilApp?, words?, closed?, announced?, reopen?, refused?}`: `refused` `off`, `no-events` (the desktop reports no window events), `no-desktop`, `no-words` or `too-long`; no title is ever in it |
| `oni.assignment` | `OniAssignmentParams{action, style?}`: `start` watching the perched window, with `style` `walkthrough` or `answers` overriding the setting; `next` and `back` through the solved views; `done`; `stop`; or `status` | `OniAssignmentResult{status, consent?, answer?, steps?, step?, cached?, day, refused?}`: `status` `consent` (the picture and route wait on an `assign-` card, as `oni.ask`'s do), `watching`, `answered`, `stopped`, `capped` (today's cap is reached; nothing was sent), `refused` or `idle`; `cached` says the view came back with no request; `day` is `OniAssignmentDay{day, attempts, tokens, attemptsCap, tokensCap, estimated?, unmetered?}` |
| `oni.assignment.log` | | `OniAssignmentLogResult{day, rows}`: today's spend and the ledger, newest first, each `OniAssignmentRow{at, lane, route, input, output, estimated?, unmetered?, outcome}`, `lane` `fast`, `checker` or `judge`; no row holds a picture, a problem, an answer or a step |

- Each needs its feature (`disabled` with the decision's `reason`
  otherwise): `oni.rewrite` `rewrite`, `oni.recall` and
  `oni.recall.import` `recall`, `oni.focus` `feeds.guard`,
  `oni.assignment` and `oni.assignment.log` `assignment`.
- Who sends what: the app and the CLI every action. The surface drawing
  Oni only `oni.focus` `stop` and `oni.assignment` `next`, `back`, `done`
  and `stop`, from its pill and card. Another surface, an observer, the
  bar, the Hub, a session's tools and a team session's socket get
  `forbidden` for every call.
- Voice reaches these through a press only: the wake phrase and open mic
  get the needs-a-press line and change nothing.
- A deliberate pressed selected-text rewrite request prepares a card but
  does not approve it. The verified app can retrieve that voice-started
  card with `oni.rewrite start`, review its exact text/destination/route,
  then explicitly apply or cancel its ID. Spoken yes sends and applies
  nothing. App apply and undo wait only for a bounded user-driven return
  to the original window and verify its identity. Apply also rechecks PRIMARY
  before sending and typing;
  they never focus the target, use the clipboard or type into Settings.
- Undo says "Ctrl+Z puts it back" only where undo was checked for that
  kind of window; otherwise the line says the original was kept.
- Unmetered usage is flagged in `day` and in the ledger, never counted as
  free. A hosted or paid route stops at the cap and says so; a local route
  is recorded, not capped.

#### Focus guard

- A guard needs window events from the desktop: arming is refused with
  `no-events` when the compositor reports none, and `no-desktop` with no
  desktop at all. It runs one deadline timer and one window watch, only
  while armed, and the daemon stays up only for that.
- Before each close a `guard-` card names the feed and the app. Closing
  requires the app's exact `oni.consent.answer`; spoken agreement is not
  enough. Dedicated feed apps close through the desktop's window seam,
  with the same ID, app identity and feed match rechecked before closing.
  General browser identities are announcement-only, even if the app ID
  matches a configured feed word. Browser metadata is rechecked at close.
- The separate Hands browser can close only its authorized target through
  its private CDP pipe, after rechecking the target ID, page origin and feed
  match. A tab in the user's own browser is never inspected or closed.
- `back` lifts the guard and may recreate the last closed Hands tab within
  ten minutes, only in the same surviving Hands browser. The recreated tab
  needs fresh Hands authorization before any action. Dedicated app windows
  are not relaunched. `reopen` counts a retained tab still eligible for that
  attempt, not the number just reopened by `back`.
- The guard disarms when its time runs out, when the `untilApp` app
  closes, on `stop` or `back`, and when the settings turn it off.

### Optional parts, activity and the MCP bridge

Each call decodes strictly; it is served once the daemon registers it.

| Method | Params | Result |
|---|---|---|
| `components.list` | | `ComponentsResult{components}`: each `Component{id, kind, title, state, bytes, path?, removable, feature?, uses?, inUse?, needs?, why?}`; `id` a lowercase word, a colon and a name (lowercase letters, digits, `.` `_` `-` `:`, at most 128 bytes); `kind` `model`, `pack`, `profile`, `data`, `cache` or `program`; `state` `absent`, `present`, `partial`, `unusable` or `unknown` (not looked for); `inUse` says why a removal is refused now; a program carries the packages that provide it in `needs` and why Hyakki would run it in `why` |
| `components.remove` | `ComponentRemoveParams{id}` | `ComponentsResult{components}` after the removal; a part Hyakki did not put in its own folders is `forbidden`, one in use `conflict`, an unknown one `not_found` |
| `oni.activity` | | `OniActivityResult{counts, rows, since?, errors?}`: healthy counts per store (`reflex_log`, `hands_log`, `assignment`, `walkthroughs`, `routines`, `reminders`, `recall_imports`); `rows` the newest metadata rows across them, at most 50, each `OniActivityRow{at, store, action, route?, ms?, input?, output?, estimated?, unmetered?, outcome}`, never a transcript, selection, passage, picture, feed or window title; `since` the oldest counted ledger row before truncation; `errors` marks unreadable stores whose counts are unknown, not zero |
| `oni.activity.clear` | `OniActivityClearParams{stores}`: one or more stores, each once, or `all` alone | `OniActivityResult` after deletion through each selected owner; requires Oni off and no active work, preflights selected owners before deletion, and preserves spending/safety counters, memory facts, conversations, original exports, vault and browser profile; unrelated unreadable stores remain reported in `errors` |
| `oni.notify` | `OniNotifyParams{text}`: one line of at most 120 characters | `Empty`; the line shows on Oni's pill with the program's name |
| `oni.mcp` | | `OniMCPStatus{enabled, tools, clients}`: the switch, the tools turned on, and each program `OniMCPClient{name, state, connected?}`, `state` `approved` or `waiting`; a program asking through the bridge sees its own entry alone |

- `components.list` downloads, starts and probes nothing; `components.remove`
  and `oni.activity.clear` are the app's alone. `components.list` and
  `oni.activity` are the app's, the CLI's and the Hub's. `oni.notify` is the
  app's and the bridge's, `oni.mcp` the app's, the CLI's and the bridge's.
  The bar, a surface, an observer, a session's tools and a team session's
  socket get `forbidden` for every one.
- `oni.reminders` gains `add` with `kind` (`reminder` or `timer`), `due`
  and `text` (one line of at most 120 characters, required for a
  reminder); no other action carries them.

#### MCP bridge

An `mcp` connection reaches nothing while `oni.mcp.enabled` is off. While
it is on, it may read `oni.mcp`; once `oni.mcp.clients` names the program it
said hello as, it may make only the calls behind the tools turned on:

| Tool | Call |
|---|---|
| `reminders_list` | `oni.reminders` `list` |
| `reminders_add` | `oni.reminders` `add` |
| `notes_search` | `memory.search` |
| `notes_add` | `memory.import`, kept as suggestions the user keeps or dismisses in the Inbox, never as a fact |
| `focus_status` | `oni.focus` `status` |
| `notify` | `oni.notify` |
| `screen_ask` | `oni.ask`, whose consent card only the person answers |

Every other call and action is `forbidden`, `settings.*`, `subscribe` and
`oni.consent.answer` among them; params that decode for nobody are
`bad_request`. The bridge's connection never keeps the daemon up.

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
words are saved privately in `words.txt` in Hyakki's data directory.
These writing methods are open to bar, CLI and Hub.

`glance` takes `session`, `mode`, `question`, `text`, `images`, `language`,
`source`, `target`, `model`, `local` and string-valued `options`.
Modes are `explain`, `ask`, `read`, `translate`, `summarize`, `deepread`,
`solve`, `learn`, `define`, `rewrite`, `tone`, `fluent`, `humanize`,
`reactions`, `reply` and `aicheck`. Options select each mode's style, lens,
form, count, audience or tone; `voice:"on"` uses the saved writing voice.
Without `session` it creates a temporary Ask with origin `sight`, and with one
it continues that conversation. Origins are `app`, `bar`, `cli`, `terminal`,
`fix`, `import`, `team`, `sight` and `voice`.

A capture uses a short mode prompt without tools, vault context or memory
recall, routing to `model`, else light work on the smallest local model that
fits. `local:true` keeps it to models on this computer. No picture-capable
route produces `no_vision` with the provider guidance.

Model events also carry `local` and `engine`, so a capture can show where its
answer came from. Keeping a temporary session with `session.fork` copies its
runtime-directory images into Hyakki's data directory before keeping the log.
Oversized images are downscaled below 8 MiB without changing their shape.

An Agent session reports its `model`, `mode` and `options` when it is created,
without a prompt: its mode is its approvals value, and its options are the
model's knobs (`effort` on a model with reasoning) plus `plan`, Plan first,
which keeps it on read-only tools and a todo list until the user approves the
plan. A session made before the native engine keeps its log; its next prompt
starts a fresh engine transcript and says so in a `notice`. The subscribe
`Snapshot` carries `local`, Hyakki's own models and runtime.

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
| `memory` | `MemoryEventData` (one memory line: `kind` is `updated`, `used`, `looked`, `refused` or `narrowed`) |

Comparison lanes are independent temporary sessions. The source conversation
records the comparison event so reattaching restores its lanes.

A `refused` memory line's `reason` says why nothing was kept: `secret` (a
password, key or token; no `text`), `number` (a payment card, social
security, national identity or bank account number; no `text`), `sensitive`
(a sensitive topic the settings leave out), `instruction` (text that reads as
an order to a model, or hides characters) or `off` (memory is off). `number`
is refused from every caller, the app included, whatever the settings: the
check is `internal/sensitive.Number` (a Luhn-valid card, a United States
SSN, a United Kingdom national insurance number, a Spanish DNI or NIE, a
Brazilian CPF, an Indian Aadhaar or a French social security number with a
valid check, a valid IBAN, or six or more digits in text that names such a
number). A memory kept before this door holds its number until the user
edits or forgets it; every `memory.*` result, `memory.changed` event, memory
line and the card show it masked (dots, a card keeping its last four
digits), and recall puts only the masked text in a prompt. Masking is
presentation: the stored record is unchanged.
