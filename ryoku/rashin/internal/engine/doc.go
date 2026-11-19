// Package engine is Rashin's own agent engine. One engine runs every session
// kind on the built-in model router: Ask is a mode of it (ask-read or
// ask-act), as are Agent sessions (agent, or plan while Plan first holds),
// team tasks (team, dream-manager) and the task tool's children (subagent).
// There is no external harness.
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
