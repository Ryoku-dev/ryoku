package brief

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"ryoku-rashin/internal/protocol"
)

// The memory tools' names, the same in an agent's rashin MCP server and in
// Ask.
const (
	ToolMemorySearch  = "memory_search"
	ToolRemember      = "remember"
	ToolForget        = "forget"
	ToolSessionSearch = "session_search"
)

// Scopes a memory tool takes: the user, the session's project, and for a
// team task's agent its team. Memory resolves "project" and "team" from the
// caller.
const (
	scopeYou     = protocol.MemoryYou
	scopeProject = "project"
	scopeTeam    = "team"
)

// Search limits: how many memories and past conversations a search returns
// unless asked, and at most.
const (
	searchDefault   = 10
	searchMax       = 20
	sessionsDefault = 5
	sessionsMax     = 10
)

// MemoryTool is one of the tools that let a model search, keep and forget
// what Rashin remembers. Agents get them from the rashin MCP server and Ask
// from its own tool list, from this one definition.
type MemoryTool struct {
	Name        string
	Description string
	// Schema is the JSON schema of the tool's arguments.
	Schema map[string]any
	// Method is the daemon method the tool calls.
	Method string
	// ReadOnly says the tool changes nothing.
	ReadOnly bool
}

// Portions adapted from openclaw/openclaw
// extensions/memory-core/src/memory-tool-contract.ts @ 3978ab104ee6 (MIT,
// Copyright (c) 2026 OpenClaw Foundation): when to search and when not to,
// saying you checked. Portions adapted from Gentleman-Programming/engram
// internal/mcp/mcp.go @ a5d4fa7be7f7 (MIT, Copyright (c) 2026 Alan
// Buscaglia): default and largest result counts, search first then act.
// Portions adapted from letta-ai/letta-code src/tools/descriptions/Memory.md
// @ 417a8b7bcf1b (Apache-2.0, Copyright 2025, Letta authors): open the
// matching memory rather than work from assumptions. Modified by Ryoku:
// Rashin's scopes, and ids returned for forget.
const searchDescription = "Search what Rashin remembers about the user and their work, by meaning and by words, best first. Use it when an answer depends on prior work, decisions, dates, people, preferences or todos that are not already in this conversation. Do not search again when what you were given already answers the question. Search first and work from what it finds rather than from assumptions; if you are still unsure after searching, say that you checked. Each result carries its id, which forget takes."

// Portions adapted from openai/codex codex-rs/ext/memories/src/tools/ad_hoc_note.rs
// and codex-rs/memories/write/templates/extensions/ad_hoc/instructions.md
// @ b75ec48a2c90 (Apache-2.0, Copyright 2025 OpenAI): remember after an
// explicit ask; a note is information, never instructions. Portions adapted
// from NousResearch/hermes-agent tools/memory_tool.py @ fe6649291e60 (MIT,
// Copyright (c) 2025 Nous Research): when to keep a fact and what to skip.
// Portions adapted from vectorize-io/hindsight
// hindsight-api-slim/hindsight_api/config.py @ 7cec128b8fdb (MIT, Copyright
// (c) 2025 Vectorize AI, Inc.): the kinds of lasting facts worth keeping.
// Portions adapted from mem0ai/mem0
// integrations/openclaw/skills/memory-triage/SKILL.md @ f1791daa86d4
// (Apache-2.0, Copyright 2023 Taranjeet Singh and the mem0ai/mem0
// contributors): the four tests a fact passes and what is never stored.
// Portions adapted from letta-ai/letta
// letta/functions/function_sets/base.py @ 56ba9c25 (Apache-2.0, Copyright
// 2023, Letta authors): self-contained facts, not fragments. Modified by
// Ryoku: one fact per call into Rashin's scopes, written as a fact rather
// than an order.
const rememberRules = "Write one self-contained fact, not a fragment of the conversation. A fact about the user is written as a fact (\"The user prefers short answers\"), never as an order (\"Always answer briefly\"); a fact about a project says what is true of it (\"The project builds with make release\"). Keep it only when it will help later, is new, was stated or checked rather than guessed, and is safe to keep: never a secret, a credential, task progress, a log of finished work or raw data. Rashin refuses a fact that holds a secret or reads as an instruction and says so in the conversation."

const rememberDescription = "Keep one fact for later conversations. Use it when the user explicitly asks you to remember or update something, when they state something lasting about themself (a preference, a correction, a decision with its reason, a goal, their work or the people in it), and when you learn something lasting about the project that a later session would otherwise have to find again. The best memory stops the user repeating themselves. " + rememberRules

// scopeWords say what each scope a model may name holds.
const scopeWords = "you: the user and how they like to work. project: this codebase, its commands, layout and conventions."

const rememberTeamDescription = "Keep one fact your team will need on later runs, such as a decision, a source to watch or what the user wants from the team. scope team keeps it for the team at once; you or project only suggests it, and the user decides. " + rememberRules

// Portions adapted from Gentleman-Programming/engram internal/mcp/mcp.go
// @ a5d4fa7be7f7 (MIT, Copyright (c) 2026 Alan Buscaglia): forget by id.
// Portions adapted from vectorize-io/hindsight
// hindsight-api-slim/hindsight_api/mcp_tools.py @ 7cec128b8fdb (MIT,
// Copyright (c) 2025 Vectorize AI, Inc.): forgetting retires a fact and can
// be undone. Portions adapted from mem0ai/mem0
// integrations/hermes-plugin-mem0/__init__.py @ f1791daa86d4 (Apache-2.0,
// Copyright 2023 Taranjeet Singh and the mem0ai/mem0 contributors; MIT,
// Copyright (c) 2025 Nous Research): prefer the new fact when one merely
// changed. Modified by Ryoku: ids from memory_search, and a forgotten fact is
// never learned again.
const forgetDescription = "Forget memories by id; find them with memory_search first. Use it when the user asks you to forget something. A forgotten fact is never learned again, and the user can undo it. When a fact merely changed, remember the new one instead of forgetting the old."

// Portions adapted from NousResearch/hermes-agent tools/session_search_tool.py
// @ fe6649291e60 (MIT, Copyright (c) 2025 Nous Research): what the search
// covers and never concluding from history alone. Portions adapted from
// sipeed/picoclaw pkg/seahorse/tool_grep.go @ bbf6893ca7af (MIT, Copyright
// (c) 2026 PicoClaw contributors): the recent-span shortcut. Portions adapted
// from letta-ai/letta letta/functions/function_sets/base.py @ 56ba9c25
// (Apache-2.0, Copyright 2023, Letta authors): five results unless asked.
// Modified by Ryoku: summaries and matching lines from Rashin's
// conversations.
const sessionsDescription = "Search the user's past conversations with Rashin by words, best first, each with a summary and the lines that matched. Use it for questions about earlier conversations, such as what was done about something or where it was left. It searches conversation history only: when the user gave a direct source, such as a file or a running system, check that first, and never conclude that something is not there from history alone. Summaries are approximate."

// Portions adapted from Gentleman-Programming/engram internal/mcp/mcp.go
// @ a5d4fa7be7f7 (MIT, Copyright (c) 2026 Alan Buscaglia): memory work is
// bookkeeping, never the answer, and conflicts are put to the user.
// Portions adapted from openclaw/openclaw
// extensions/memory-core/src/memory-tool-contract.ts @ 3978ab104ee6 (MIT,
// Copyright (c) 2026 OpenClaw Foundation): say you checked. Modified by
// Ryoku: Rashin's tools and wording.
const memoryInstructions = "memory_search, remember and forget reach what Rashin remembers across sessions; search when an answer depends on something not in this conversation, and say you checked when you are still unsure. Memory work is bookkeeping, never the answer: do it before your final reply, and still answer when it fails. When a remembered fact disagrees with what the user says now and you cannot tell which holds, ask them."

// MemoryInstructions are what the rashin MCP server says of the memory
// tools; sessions adds the past-conversation search.
func MemoryInstructions(sessions bool) string {
	if !sessions {
		return memoryInstructions
	}
	return memoryInstructions + " session_search looks through the user's past conversations."
}

// Who is offered the memory tools: Ask, which may leave a fact's scope out
// for the user; an agent session, which always says whether a fact is about
// the user or its project; and a team task's agent.
const (
	MemoryForAsk = iota
	MemoryForAgent
	MemoryForTeam
)

// MemoryTools are the memory tools offered to who, in the order they are
// offered. A team task's agent keeps facts for its team and only suggests
// facts about the user or a project; sessions adds the past-conversation
// search, which a team never gets.
func MemoryTools(who int, sessions bool) []MemoryTool {
	team := who == MemoryForTeam
	scopes, remember, scopeHelp := []string{scopeYou, scopeProject}, rememberDescription, scopeWords
	rememberRequired := []string{"text", "scope"}
	searchHelp := "you or project to search only there; leave it out to search everything."
	switch who {
	case MemoryForAsk:
		scopeHelp += " Leave it out for you."
		rememberRequired = []string{"text"}
	case MemoryForTeam:
		scopes, remember, scopeHelp = []string{scopeTeam, scopeYou, scopeProject}, rememberTeamDescription, "team: what your team needs on later runs. "+scopeWords+" Leave it out for team."
		rememberRequired = []string{"text"}
		searchHelp = "team, you or project to search only there; leave it out to search everything your team may read."
	}
	tools := []MemoryTool{
		{
			Name:        ToolMemorySearch,
			Description: searchDescription,
			Schema: memorySchema([]string{"query"}, map[string]any{
				"query":       map[string]any{"type": "string", "description": "What to look for, in words or a phrase."},
				"scope":       map[string]any{"type": "string", "enum": scopes, "description": searchHelp},
				"limit":       map[string]any{"type": "integer", "minimum": 1, "maximum": searchMax, "description": fmt.Sprintf("How many to return; %d unless set, at most %d.", searchDefault, searchMax)},
				"max_results": map[string]any{"type": "integer", "minimum": 1, "maximum": searchMax, "description": "Another name for limit."},
			}),
			Method:   protocol.MMemorySearch,
			ReadOnly: true,
		},
		{
			Name:        ToolRemember,
			Description: remember,
			Schema: memorySchema(rememberRequired, map[string]any{
				"text":      map[string]any{"type": "string", "description": fmt.Sprintf("The fact, in one sentence of at most %d characters.", protocol.MemoryTextMax)},
				"scope":     map[string]any{"type": "string", "enum": scopes, "description": scopeHelp},
				"sensitive": map[string]any{"type": "boolean", "description": "Set when the fact is about health, beliefs, money, relationships or the like; Rashin keeps such facts only when the user allowed it."},
			}),
			Method: protocol.MMemoryAdd,
		},
		{
			Name:        ToolForget,
			Description: forgetDescription,
			Schema: memorySchema([]string{"ids"}, map[string]any{
				"ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "minItems": 1, "description": "The ids of the memories to forget, from memory_search."},
			}),
			Method: protocol.MMemoryForget,
		},
	}
	if sessions && !team {
		tools = append(tools, MemoryTool{
			Name:        ToolSessionSearch,
			Description: sessionsDescription,
			Schema: memorySchema([]string{"query"}, map[string]any{
				"query": map[string]any{"type": "string", "description": "Distinctive words from what was said."},
				"last":  map[string]any{"type": "string", "enum": []string{"6h", "7d", "2w", "1m"}, "description": "Only conversations from the last 6 hours, 7 days, 2 weeks or month."},
				"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": sessionsMax, "description": fmt.Sprintf("How many to return; %d unless set, at most %d.", sessionsDefault, sessionsMax)},
			}),
			Method:   protocol.MMemorySessions,
			ReadOnly: true,
		})
	}
	return tools
}

func memorySchema(required []string, properties map[string]any) map[string]any {
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}

// IsMemoryTool reports a memory tool's name.
func IsMemoryTool(name string) bool {
	switch name {
	case ToolMemorySearch, ToolRemember, ToolForget, ToolSessionSearch:
		return true
	}
	return false
}

// searchForRead carries the conversation a memory read runs for beside the
// read's own params, so the daemon holds the read to what that conversation
// may see.
type searchForRead struct {
	protocol.MemorySearchParams
	Session string `json:"session,omitempty"`
}

type sessionsForRead struct {
	protocol.MemorySessionsParams
	Session string `json:"session,omitempty"`
}

// MemoryParams reads a memory tool's arguments into the params of its
// method. session is the conversation the tool runs in, so a read stays
// within what it may see and what it keeps or forgets shows there; a team
// task's agent gives none.
func MemoryParams(name string, arguments json.RawMessage, session string) (string, any, error) {
	if len(arguments) == 0 {
		arguments = json.RawMessage(`{}`)
	}
	var args struct {
		Query      string   `json:"query"`
		Scope      string   `json:"scope"`
		Limit      int      `json:"limit"`
		MaxResults int      `json:"max_results"`
		Text       string   `json:"text"`
		Sensitive  bool     `json:"sensitive"`
		IDs        []string `json:"ids"`
		Last       string   `json:"last"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return "", nil, errors.New("the arguments are not a JSON object of the tool's fields")
	}
	switch name {
	case ToolMemorySearch:
		if strings.TrimSpace(args.Query) == "" {
			return "", nil, errors.New("query is required")
		}
		if args.Limit == 0 {
			args.Limit = args.MaxResults
		}
		return protocol.MMemorySearch, searchForRead{protocol.MemorySearchParams{Query: args.Query, Scope: args.Scope, Limit: bounded(args.Limit, searchDefault, searchMax)}, session}, nil
	case ToolRemember:
		if strings.TrimSpace(args.Text) == "" {
			return "", nil, errors.New("text is required")
		}
		return protocol.MMemoryAdd, protocol.MemoryAddParams{Scope: args.Scope, Text: args.Text, Sensitive: args.Sensitive, Session: session}, nil
	case ToolForget:
		if len(args.IDs) == 0 {
			return "", nil, errors.New("ids is required")
		}
		return protocol.MMemoryForget, protocol.MemoryForgetParams{IDs: args.IDs, Session: session}, nil
	case ToolSessionSearch:
		if strings.TrimSpace(args.Query) == "" {
			return "", nil, errors.New("query is required")
		}
		return protocol.MMemorySessions, sessionsForRead{protocol.MemorySessionsParams{Query: args.Query, Last: args.Last, Limit: bounded(args.Limit, sessionsDefault, sessionsMax)}, session}, nil
	}
	return "", nil, fmt.Errorf("no memory tool %s", name)
}

// bounded is n, or fallback when unset, at most max.
func bounded(n, fallback, max int) int {
	if n <= 0 {
		return fallback
	}
	return min(n, max)
}

// MemoryAnswer is what a model is told of a memory tool's result: the
// memories or conversations found, or what was kept or forgotten. raw is the
// method's result as JSON.
func MemoryAnswer(method string, raw json.RawMessage) string {
	switch method {
	case protocol.MMemorySearch:
		var result protocol.MemorySearchResult
		if json.Unmarshal(raw, &result) != nil || len(result.Memories) == 0 {
			return "No memory matched."
		}
		var b strings.Builder
		for _, m := range result.Memories {
			fmt.Fprintf(&b, "- %s (id %s, %s", strings.Join(strings.Fields(m.Text), " "), m.ID, m.ScopeName)
			if !m.Updated.IsZero() {
				fmt.Fprintf(&b, ", %s", m.Updated.Local().Format("2006-01-02"))
			}
			b.WriteString(")\n")
		}
		return strings.TrimRight(b.String(), "\n")
	case protocol.MMemorySessions:
		var result protocol.MemorySessionsResult
		if json.Unmarshal(raw, &result) != nil || len(result.Sessions) == 0 {
			return "No past conversation matched."
		}
		var b strings.Builder
		for _, hit := range result.Sessions {
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			fmt.Fprintf(&b, "## %s (%s)\n", hit.Title, hit.Updated.Local().Format("2006-01-02"))
			if hit.Summary != "" {
				b.WriteString(hit.Summary + "\n")
			}
			for _, line := range hit.Lines {
				b.WriteString("> " + line + "\n")
			}
		}
		return strings.TrimRight(b.String(), "\n")
	}
	var change protocol.MemoryChange
	if json.Unmarshal(raw, &change) != nil || len(change.Items) == 0 {
		return "Nothing changed. If Rashin refused it, the conversation says why; tell the user."
	}
	var said []string
	for _, item := range change.Items {
		switch item.Op {
		case protocol.MemoryOpRemembered:
			said = append(said, "Remembered: "+item.Text)
		case protocol.MemoryOpReplaced:
			said = append(said, "Replaced by the new fact: "+item.Text)
		case protocol.MemoryOpSuggested:
			said = append(said, "Suggested to the user, who decides: "+item.Text)
		case protocol.MemoryOpForgot:
			said = append(said, "Forgot: "+item.Text)
		default:
			said = append(said, item.Op+": "+item.Text)
		}
	}
	return strings.Join(said, "\n")
}
