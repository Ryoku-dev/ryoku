// Package engine runs Hyakki's native agent engine and configured team runtimes.
// Native Ask, Agent, team and subagent sessions retain the built-in router,
// tools, budgets and compaction. A team's configured member may instead use
// ACP over local argv, a monitored command, or a synchronous webhook; all
// implement core.Runner and retain the team's actual policy ceilings.
//
// The daemon builds one Factory at start:
//
//	func New(deps Deps) *Factory
//	func (f *Factory) NewAsk(opts core.AskOptions) core.Runner                            // core.Asker
//	func (f *Factory) NewAgent(ctx context.Context, opts core.AgentOptions) (core.Runner, error) // core.Agents
//
// A runner resolves Options (ModeForAsk, ModeForAgent), picks the tool class
// from the model (tool.ClassForModel: compact for light models, full
// otherwise), asks Deps.Tools for the round's tool.Set, and runs the round
// loop with recovery, budgets and compaction. Every tool call goes through a
// tool.Tool with a tool.Env whose Exec is the session's executor.
//
// Hooks (hooks.go) are how optional features join without being imported:
// ContextProvider blocks in the system prompt, StreamGuards on streamed
// deltas, TurnObserver after each turn, and a Judge for light verdicts.
//
// External runtime contracts (runtime*.go):
//
// Local argv is never interpreted as a shell fragment. Commands receive one
// version=1 runtimeWork JSON object on stdin; successful observed exit produces
// an authenticated team report containing bounded real output and exit status.
// Nonzero exit, cancellation or failed monitoring never complete a task. There
// is no command process resume: a later task turn is a new invocation.
//
// ACP uses newline JSON-RPC protocol v1 and negotiated session/load support.
// A saved session that cannot load fails rather than silently losing history.
// Model IDs must be advertised as legacy models or model config options.
// Text is bounded and redacted before publication, including split chunks.
// ACP frames are capped at 1 MiB; captured text, terminal output and text file
// requests at 256 KiB; stderr at 8 KiB. At most 16 terminal handles, 16 ACP
// client requests, four private MCP connections and eight MCP requests exist
// per turn; request overload cancels the run, while excess sockets are refused.
// Unknown capabilities fail explicitly; images and native options are not
// translated. Each turn stops its ACP process and terminals, retaining only a
// versioned opaque session ID/model/configuration fingerprint for later resume.
// The private oni __runtime-mcp command exposes only existing, policy-enabled
// team governance tools on a per-turn Unix socket, never general daemon RPC,
// a public listener, or a run token. Managers can delegate, ask, request help,
// escalate, propose plans and use Dream governance through their existing tools;
// these actions yield, not spuriously finish their parent task.
//
// Webhooks receive runtimeWork JSON (version, kind=task, session, team, task,
// member, turn, text), through the trusted RuntimeHTTP callback. Exactly HTTP
// 200 and a complete <=256 KiB JSON response are required:
//
//	{"version":1,"session":"…","task":"…","turn":1,"status":"completed",
//	 "report":{"title":"…","body":"…","files":[],"quiet":false},
//	 "usage":{"input":123,"output":45}}
//
// Identity/turn must match. Report body is required unless quiet; usage is
// optional and absent accounting is Unmetered, never measured zero. There is
// no retry, polling or callback endpoint. HTTP 202/startup/waiting does not
// complete work. RuntimeHTTP owns credential-host checks, response redaction,
// full-response bounds and cancellation. A webhook/command cannot present or
// finish a Dream; its manager uses native or ACP governance.
// Cancelling a webhook cancels Hyakki's owned HTTP request; it cannot forcibly
// stop an independent remote service after that service accepted the request.
//
// External local work requires the existing team sandbox, never host fallback.
// Its DNS and shared networking remain available for real provider requests;
// loopback/tailnet access and the existing Landlock kernel limitation remain
// the sandbox's documented limits, not an invented no-network guarantee.
// External children never inherit the daemon's login environment, real agent
// homes or daemon sockets; only the private, governed MCP socket is bound back.
// Read-only removes persistent writable mounts and network access, using
// private /tmp for scratch/config/cache, not a foreign CLI's promised mode.
// Starting a writable process authorizes its whole sandboxed execution; ACP
// permission/file/terminal requests also pass the central gate. Resolved secret
// values stay in environment/credential callbacks, never sandbox argv, state,
// event deltas or reports. Exact values, known encoded variants and the run
// token are redacted; arbitrary transformations cannot be recognized reliably.
// ACP/command accounting is Unmetered; external reported webhook usage is a
// report, not independently measured provider billing.
//
// Canonical tool names per mode (tool/names.go holds the table, in this
// order; a name is offered only when its tool is registered and enabled for
// the session, read-only profiles keep only read-only tools):
//
//	ask-read  full     system_query read_file list_dir search_system search_code read_wiki
//	                   fetch_url doctor propose teams_templates teams_list propose_team
//	                   memory_search remember forget session_search skills skill
//	          compact  system_query read_file list_dir search_system fetch_url doctor propose
//	                   remember
//	ask-act   full     ask-read full with run_command write_file after propose
//	          compact  system_query read_file list_dir search_system fetch_url propose
//	                   run_command write_file
//	agent     full     read write edit ls find_files grep bash wait todo task ask goal
//	                   context_notes new_context search find def outline references impact peek
//	                   git_status git_diff git_log git_show fetch_url learn manage_skill
//	                   report_issue
//	                   memory_search remember forget session_search skills skill
//	                   teams_templates teams_list propose_team screen_look screen_click
//	                   screen_move screen_drag screen_scroll screen_type screen_key
//	                   screen_open screen_ask_user screen_stop, then MCP tools
//	          compact  read write edit bash grep find_files todo learn
//	plan      full     read ls find_files grep todo task ask context_notes new_context search
//	                   find def outline references impact peek git_status git_diff git_log
//	                   git_show fetch_url
//	                   learn report_issue memory_search remember session_search skills skill
//	                   teams_templates teams_list, then read-only MCP tools
//	          compact  read ls find_files grep todo learn
//	team      full     agent full without ask, with report ask_user credentials
//	                   http_request send_mail request_credential request_setup delegate
//	                   team_status before the screen tools, then MCP tools
//	          compact  read write edit bash grep find_files report learn
//	dream-manager full team full, then hire release dream_status present
//	          compact  read find_files delegate team_status hire release dream_status present
//	subagent  full     read ls find_files grep bash wait write edit search find def outline
//	                   references impact peek git_status git_diff git_log git_show fetch_url
//	                   memory_search skills skill yield
//	          compact  read find_files grep bash write edit yield
package engine
