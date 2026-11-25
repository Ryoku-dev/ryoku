package rules

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/engine"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func ruleSettings(on bool) func() protocol.Settings {
	return func() protocol.Settings {
		return protocol.Settings{Features: map[string]bool{protocol.FeatureRules: on}}
	}
}

func TestRuleLookaheadInterruptsOncePerAttempt(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	s := New(Deps{Settings: ruleSettings(true)})
	_, err := s.Save(context.Background(), protocol.RuleSaveParams{Rule: protocol.Rule{Name: "avoid-any", Description: "Avoid any", Body: "Choose a precise type.", Condition: []string{`(?m)^(?!\s*//).*:\s*any\b`}, Scope: "tool:write(*.ts)", Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	tc := engine.TurnContext{Session: protocol.SessionInfo{ID: "one"}, Project: "", Profile: tool.Profile{Class: tool.ClassFull}}
	g := s.Guard(tc)
	if g == nil {
		t.Fatal("enabled rule did not install guard")
	}
	g.Reset()
	call := llm.ToolCall{Name: tool.Write, Arguments: `{"path":"src/widget.ts","content":"const x: any = value"}`}
	hit := g.Observe(llm.Delta{ToolCalls: []llm.ToolCall{call}})
	if hit == nil || hit.Notice || !strings.HasPrefix(hit.Message, `<system-interrupt reason="rule_violation"`) {
		t.Fatalf("unexpected hit: %#v", hit)
	}
	if next := g.Observe(llm.Delta{ToolCalls: []llm.ToolCall{call}}); next != nil && strings.Contains(next.Message, `rule="avoid-any"`) {
		t.Fatalf("duplicate in attempt: %#v", next)
	}
	g.Reset()
	if next := g.Observe(llm.Delta{ToolCalls: []llm.ToolCall{call}}); next != nil && strings.Contains(next.Message, `rule="avoid-any"`) {
		t.Fatalf("retry interrupted again: %#v", next)
	}
	if next := s.Guard(tc).Observe(llm.Delta{ToolCalls: []llm.ToolCall{call}}); next != nil && strings.Contains(next.Message, `rule="avoid-any"`) {
		t.Fatalf("once rule fired again in next turn: %#v", next)
	}
}
func TestOnceRuleSurvivesRestoredTranscript(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	id := "restored-rule"
	path := paths.EngineTranscript(id)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: "First prompt"},
		{Role: llm.RoleUser, Content: `<system-interrupt reason="rule_violation" rule="avoid-any" path="file.go">Rule</system-interrupt>`},
		{Role: llm.RoleUser, Content: "New prompt"},
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range messages {
		data, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(append(data, '\n')); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	s := New(Deps{Settings: ruleSettings(true)})
	if _, err := s.Save(context.Background(), protocol.RuleSaveParams{Rule: protocol.Rule{Name: "avoid-any", Description: "Avoid any", Body: "Use a precise type", Condition: []string{`any`}, Scope: "text", Enabled: true}}); err != nil {
		t.Fatal(err)
	}
	g := s.Guard(engine.TurnContext{Session: protocol.SessionInfo{ID: id}})
	if hit := g.Observe(llm.Delta{Text: "any"}); hit != nil {
		t.Fatalf("once rule lost after transcript restore: %#v", hit)
	}
}

func TestRuleOffDoesNotReadOrOpenRuleFolder(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", root)
	s := New(Deps{Settings: ruleSettings(false)})
	tc := engine.TurnContext{Session: protocol.SessionInfo{ID: "off"}}
	if s.Guard(tc) != nil || len(s.Blocks(context.Background(), tc)) != 0 {
		t.Fatal("disabled rule supplied context")
	}
	if _, err := os.Stat(paths.RulesDir()); !os.IsNotExist(err) {
		t.Fatalf("rules directory unexpectedly exists: %v", err)
	}
	if _, err := s.Forge(context.Background(), protocol.RuleForgeParams{Session: "off", Complaint: "bad"}); err == nil {
		t.Fatal("disabled forge ran")
	}
}

func TestForgeValidatesHistoryAndNarrowScope(t *testing.T) {
	src := []source{{Kind: SourceText, Text: "I will do it tomorrow"}, {Kind: SourceTool, Tool: tool.Write, Path: "src/main.go", Text: "fmt.Println(\"secret\")"}}
	s := New(Deps{Settings: ruleSettings(true)})
	r := protocol.Rule{Name: "avoid-secret", Description: "No secret", Body: "Use approved credentials.", Scope: "tool:write(*.go)", Condition: []string{`secret`}, Enabled: true}
	if hit, err := s.validate(context.Background(), r, src); err != nil || !strings.Contains(hit, "secret") {
		t.Fatalf("valid rule: hit %q err %v", hit, err)
	}
	r.Scope = "text, tool"
	if _, err := s.validate(context.Background(), r, src); err == nil {
		t.Fatal("broad file-edit rule accepted")
	}
	r.Scope = "tool:write(*.rs)"
	if _, err := s.validate(context.Background(), r, src); err == nil {
		t.Fatal("unmatched rule accepted")
	}
	draft := `{"name":"avoid-secret","description":"No secret","scope":["tool:write(*.go)"],"body":"Use approved credentials.","condition":"secret"}`
	proposed, err := parseProposal("```json\n" + draft + "\n```")
	if err != nil {
		t.Fatal(err)
	}
	if raw, err := json.Marshal(proposed); err != nil || len(raw) == 0 {
		t.Fatalf("proposal JSON: %v", err)
	}
}

type forgeClient struct {
	replies []string
}

func (c *forgeClient) Stream(context.Context, llm.Request) (llm.Stream, error) {
	reply := c.replies[0]
	c.replies = c.replies[1:]
	return &forgeStream{text: reply}, nil
}

type forgeStream struct {
	text string
	done bool
}

func (s *forgeStream) Next() (llm.Delta, error) {
	if s.done {
		return llm.Delta{}, io.EOF
	}
	s.done = true
	return llm.Delta{Text: s.text}, nil
}
func (*forgeStream) Final() llm.Final { return llm.Final{} }
func (*forgeStream) Close() error     { return nil }

func TestForgeRetriesUntilCandidateActuallyMatches(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	id := "forge-session"
	path := paths.EngineTranscript(id)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	output, _ := json.Marshal(llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{Name: tool.Write, Arguments: `{"path":"src/main.go","content":"fmt.Println(\"secret\")"}`}}})
	if err := os.WriteFile(path, append(output, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	c := &forgeClient{replies: []string{
		`{"name":"avoid-secret","description":"No secret","body":"Do not print secrets.","scope":"tool:write(*.rs)","condition":"secret"}`,
		`{"name":"avoid-secret","description":"No secret","body":"Do not print secrets.","scope":"tool:write(*.go)","condition":"secret"}`,
	}}
	s := New(Deps{Settings: ruleSettings(true), Client: c})
	got, err := s.Forge(context.Background(), protocol.RuleForgeParams{Session: id, Complaint: "Stop writing secret literals"})
	if err != nil || !got.OK || got.Attempts != 2 || !strings.Contains(got.Matched, "secret") {
		t.Fatalf("forge: %#v, %v", got, err)
	}
}

func TestStreamRulesKeepCurrentLineAcrossDeltasWithoutRescanningEarlierLines(t *testing.T) {
	s := New(Deps{})
	r := protocol.Rule{Name: "current-line", Description: "Current line", Body: "Stop.", Condition: []string{`echo .*TOKEN`}, Scope: "text", Enabled: true}
	g := &guard{service: s, rules: []*compiled{compile(r, false)}, session: "current-line", turn: 1, fired: map[string]bool{}}
	for _, delta := range []string{"earlier line\n", "echo some ", "args ", "TOKEN"} {
		hit := g.Observe(llm.Delta{Text: delta})
		if delta == "TOKEN" && hit == nil {
			t.Fatal("a pattern split across deltas on one line was missed")
		}
		if delta != "TOKEN" && hit != nil {
			t.Fatalf("rule fired before its completed line: %+v", hit)
		}
	}
	g.Reset()
	r.Name = "cross-line"
	r.Scope = "thinking"
	r.Condition = []string{`(?s)before.*after`}
	g.rules = []*compiled{compile(r, false)}
	if hit := g.Observe(llm.Delta{Reasoning: "before\n"}); hit != nil {
		t.Fatalf("multiline rule fired early: %+v", hit)
	}
	if hit := g.Observe(llm.Delta{Reasoning: "after"}); hit == nil {
		t.Fatal("multiline condition lost the prior line")
	}
}
