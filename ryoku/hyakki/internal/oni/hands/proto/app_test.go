package proto

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

var regions = []string{"r1", "r2"}

func tool(name string, args map[string]any) string {
	b, err := json.Marshal(map[string]any{"type": "tool_call", "tool": name, "args": args, "confirm": false, "permission": "normal"})
	if err != nil {
		panic(err)
	}
	return string(b)
}

func TestEachScopeHasItsFixedTools(t *testing.T) {
	want := map[string][]string{
		ScopeBrowser: {"click", "fill", "select", "scroll", "wait", "done", "ask_user"},
		ScopeWindow:  {"click", "type", "press", "scroll", "wait", "done", "ask_user"},
		ScopeNested:  {"click", "type", "press", "scroll", "wait", "open_app", "find_files", "show_file", "done", "ask_user"},
		ScopeDesktop: {"click", "type", "press", "scroll", "wait", "open_app", "find_files", "show_file", "done", "ask_user"},
	}
	for _, s := range Scopes {
		if got := ToolsFor(s); !slices.Equal(got, want[s]) {
			t.Errorf("ToolsFor(%s) = %v, want %v", s, got, want[s])
		}
		if !KnownScope(s) {
			t.Errorf("KnownScope(%s) = false", s)
		}
	}
	for _, s := range []string{"", "app", "Window", "live", "whole"} {
		if KnownScope(s) || ToolsFor(s) != nil {
			t.Errorf("scope %q is known", s)
		}
	}
	if Nested(ScopeWindow) || Nested(ScopeBrowser) || !Nested(ScopeNested) || !Nested(ScopeDesktop) {
		t.Fatal("only nested and desktop are the separate desktop")
	}
}

func TestParseAcceptsEachAppTool(t *testing.T) {
	o := Offer{Scope: ScopeNested, Controls: regions, Files: 3}
	cases := []struct {
		raw  string
		want Args
	}{
		{tool("click", map[string]any{"id": "r1"}), Args{ID: "r1"}},
		{tool("type", map[string]any{"text": "  hello  "}), Args{Text: "  hello  "}},
		{tool("press", map[string]any{"keys": "Shift+tab"}), Args{Keys: "shift+Tab"}},
		{tool("press", map[string]any{"keys": "Ctrl+Z"}), Args{Keys: "ctrl+z"}},
		{tool("press", map[string]any{"keys": "Enter"}), Args{Keys: "Return"}},
		{tool("scroll", map[string]any{"dir": "down", "id": "r2"}), Args{Dir: "down", ID: "r2"}},
		{tool("wait", map[string]any{"ms": 300}), Args{Ms: 300}},
		{tool("open_app", map[string]any{"name": " Text\u200b Editor "}), Args{Name: "Text Editor"}},
		{tool("find_files", map[string]any{"query": " tax 2026 "}), Args{Query: "tax 2026"}},
		{tool("show_file", map[string]any{"index": 3}), Args{Index: 3}},
		{tool("done", map[string]any{"text": "Typed it."}), Args{Text: "Typed it."}},
	}
	for _, tc := range cases {
		got, err := Parse(tc.raw, o)
		if err != nil {
			t.Errorf("Parse(%s): %v", tc.raw, err)
			continue
		}
		if got.Args != tc.want || got.Type != TypeToolCall {
			t.Errorf("Parse(%s) = %+v, want args %+v", tc.raw, got, tc.want)
		}
	}
}

func TestAToolForeignToTheScopeIsInvented(t *testing.T) {
	cases := []struct {
		scope string
		raw   string
	}{
		{ScopeBrowser, tool("type", map[string]any{"text": "x"})},
		{ScopeBrowser, tool("press", map[string]any{"keys": "Tab"})},
		{ScopeBrowser, tool("open_app", map[string]any{"name": "Files"})},
		{ScopeWindow, tool("open_app", map[string]any{"name": "Files"})},
		{ScopeWindow, tool("find_files", map[string]any{"query": "tax"})},
		{ScopeWindow, tool("show_file", map[string]any{"index": 1})},
		{ScopeWindow, tool("fill", map[string]any{"id": "r1", "text": "x"})},
		{ScopeNested, tool("select", map[string]any{"id": "r1", "option": "x"})},
		{ScopeDesktop, tool("fill", map[string]any{"id": "r1", "text": "x"})},
	}
	for _, tc := range cases {
		if _, err := Parse(tc.raw, Offer{Scope: tc.scope, Controls: regions, Files: 1}); !errors.Is(err, ErrInvented) {
			t.Errorf("%s: Parse(%s) = %v, want ErrInvented", tc.scope, tc.raw, err)
		}
	}
	if _, err := Parse(tool("click", map[string]any{"id": "r1"}), Offer{Scope: "whole", Controls: regions}); !errors.Is(err, ErrInvented) {
		t.Fatalf("an unknown scope offers nothing: %v", err)
	}
}

// No scope offers a drag, a paste, a look, a run, a handoff or a plan, and
// one reply is one action everywhere.
func TestNoScopeOffersMore(t *testing.T) {
	ok := tool("press", map[string]any{"keys": "Tab"})
	for _, scope := range Scopes {
		o := Offer{Scope: scope, Controls: regions, Files: 2}
		cases := []struct {
			raw  string
			want error
		}{
			{tool("drag", map[string]any{"id": "r1"}), ErrInvented},
			{tool("paste", map[string]any{}), ErrInvented},
			{tool("look", map[string]any{}), ErrInvented},
			{tool("run", map[string]any{"name": "x"}), ErrInvented},
			{tool("open_file", map[string]any{"index": 1}), ErrInvented},
			{tool("handoff", map[string]any{"id": "r1"}), ErrInvented},
			{`{"type":"plan_call","plan":"x","args":{},"confirm":true,"permission":"critical"}`, ErrInvented},
			{ok + ok, ErrMultiple},
			{"[" + ok + "]", ErrMultiple},
			{"[DO:PRESS Tab]", ErrMultiple},
			{"press Tab", ErrJSON},
		}
		for _, tc := range cases {
			if _, err := Parse(tc.raw, o); !errors.Is(err, tc.want) {
				t.Errorf("%s: Parse(%s) = %v, want %v", scope, tc.raw, err, tc.want)
			}
		}
	}
}

func TestAppArgumentRefusals(t *testing.T) {
	o := Offer{Scope: ScopeDesktop, Controls: regions, Files: 2}
	cases := []struct {
		name string
		raw  string
	}{
		{"click by point", tool("click", map[string]any{"x": 0.5, "y": 0.5})},
		{"click unlisted", tool("click", map[string]any{"id": "r9"})},
		{"click with text", tool("click", map[string]any{"id": "r1", "text": "x"})},
		{"type nothing", tool("type", map[string]any{"text": "  "})},
		{"type into an id", tool("type", map[string]any{"text": "x", "id": "r1"})},
		{"press two keys", tool("press", map[string]any{"keys": "Tab Tab"})},
		{"press unknown", tool("press", map[string]any{"keys": "hyper+x"})},
		{"press a list", tool("press", map[string]any{"keys": []string{"Tab"}})},
		{"press text", tool("press", map[string]any{"text": "Tab"})},
		{"open nothing", tool("open_app", map[string]any{"name": "\u200b"})},
		{"open a long name", tool("open_app", map[string]any{"name": strings.Repeat("a", MaxAppNameRunes+1)})},
		{"open with argv", tool("open_app", map[string]any{"name": "sh", "text": "-c rm"})},
		{"find a path", tool("find_files", map[string]any{"query": "../.ssh"})},
		{"find absolute", tool("find_files", map[string]any{"query": "/etc/passwd"})},
		{"show zero", tool("show_file", map[string]any{"index": 0})},
		{"show past the list", tool("show_file", map[string]any{"index": 3})},
		{"show by path", tool("show_file", map[string]any{"query": "/home"})},
	}
	for _, tc := range cases {
		if _, err := Parse(tc.raw, o); !errors.Is(err, ErrArgs) {
			t.Errorf("%s: Parse = %v, want ErrArgs", tc.name, err)
		}
	}
	if _, err := Parse(tool("show_file", map[string]any{"index": 1}), Offer{Scope: ScopeNested}); !errors.Is(err, ErrArgs) {
		t.Fatalf("show_file with no files listed: %v", err)
	}
}
