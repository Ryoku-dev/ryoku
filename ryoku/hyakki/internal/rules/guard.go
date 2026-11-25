package rules

import (
	"context"
	"fmt"
	"strings"

	"ryoku-rashin/internal/engine"
	"ryoku-rashin/internal/llm"
)

const repeatGap = 10
const maxStreamBytes = 64 << 10

type guard struct {
	service         *Service
	rules           []*compiled
	session         string
	turn            int
	text, reasoning string
	fired           map[string]bool
}

var _ engine.StreamGuard = (*guard)(nil)

func (s *Service) Guard(tc engine.TurnContext) engine.StreamGuard {
	if !s.enabled() {
		return nil
	}
	var active []*compiled
	for _, c := range s.discovered(context.Background(), tc.Project) {
		if c.streams() {
			active = append(active, c)
		}
	}
	if len(active) == 0 {
		return nil
	}
	s.hydrate(tc.Session.ID)
	s.mu.Lock()
	s.turns[tc.Session.ID]++
	turn := s.turns[tc.Session.ID]
	s.mu.Unlock()
	return &guard{service: s, rules: active, session: tc.Session.ID, turn: turn, fired: make(map[string]bool)}
}

// Compaction keeps injected messages, so a resumed session remembers which
// once-only rules already fired.
func (s *Service) hydrate(id string) {
	s.mu.Lock()
	if s.hydrated[id] {
		s.mu.Unlock()
		return
	}
	s.hydrated[id] = true
	s.mu.Unlock()
	messages, err := engine.ReadTranscript(id)
	if err != nil {
		return
	}
	turns := 0
	fired := make(map[string]int)
	for _, m := range messages {
		if m.Role != llm.RoleUser {
			continue
		}
		text := m.Content
		if strings.HasPrefix(text, `<system-interrupt reason="rule_violation"`) || strings.HasPrefix(text, `<system-reminder reason="rule_violation"`) {
			if i := strings.Index(text, `rule="`); i >= 0 {
				if name, _, ok := strings.Cut(text[i+len(`rule="`):], `"`); ok {
					fired[name] = max(1, turns)
				}
			}
		} else {
			turns++
		}
	}
	s.mu.Lock()
	if s.turns[id] < turns {
		s.turns[id] = turns - 1
	}
	if s.fired[id] == nil {
		s.fired[id] = make(map[string]int)
	}
	for name, turn := range fired {
		if s.fired[id][name] < turn {
			s.fired[id][name] = turn
		}
	}
	s.mu.Unlock()
}

func (g *guard) Reset() { g.text = ""; g.reasoning = ""; g.fired = make(map[string]bool) }

func (g *guard) Observe(d llm.Delta) *engine.Interrupt {
	var reminders []string
	observe := func(src source, start int) *engine.Interrupt {
		hit := g.match(src, start)
		if hit != nil && hit.Notice {
			reminders = append(reminders, hit.Message)
			return nil
		}
		return hit
	}
	if d.Text != "" {
		start, old := streamLineStart(g.text), len(g.text)
		g.text = appendBound(g.text, d.Text)
		start = max(0, start-(old+len(d.Text)-len(g.text)))
		if hit := observe(source{Kind: SourceText, Text: g.text}, start); hit != nil {
			return hit
		}
	}
	if d.Reasoning != "" {
		start, old := streamLineStart(g.reasoning), len(g.reasoning)
		g.reasoning = appendBound(g.reasoning, d.Reasoning)
		start = max(0, start-(old+len(d.Reasoning)-len(g.reasoning)))
		if hit := observe(source{Kind: SourceThinking, Text: g.reasoning}, start); hit != nil {
			return hit
		}
	}
	for _, call := range d.ToolCalls {
		for _, src := range callSources(call.Name, call.Arguments) {
			if hit := observe(src, 0); hit != nil {
				return hit
			}
		}
	}
	if len(reminders) > 0 {
		return &engine.Interrupt{Rule: "rules", Notice: true, Message: strings.Join(reminders, "\n")}
	}
	return nil
}

func streamLineStart(text string) int {
	return strings.LastIndexByte(text, '\n') + 1
}

func appendBound(buffer, delta string) string {
	buffer += delta
	if len(buffer) <= maxStreamBytes {
		return buffer
	}
	start := len(buffer) - maxStreamBytes
	for start < len(buffer) && buffer[start]&0xc0 == 0x80 {
		start++
	}
	return buffer[start:]
}

func (g *guard) match(src source, start int) *engine.Interrupt {
	type found struct {
		c       *compiled
		excerpt string
	}
	var notices []found
	for _, c := range g.rules {
		if g.fired[c.rule.Name] || !c.applies(src) || !g.service.ready(g.session, c.rule.Name, c.rule.Repeat, g.turn) {
			continue
		}
		excerpt, ok := c.findStream(src.Text, start)
		if !ok {
			continue
		}
		if !c.interrupts(src) {
			notices = append(notices, found{c, excerpt})
			continue
		}
		g.fired[c.rule.Name] = true
		g.service.mark(g.session, c.rule.Name, g.turn)
		path := src.Path
		if path == "" {
			path = c.rule.Path
		}
		return &engine.Interrupt{Rule: c.rule.Name, Message: envelope("system-interrupt", c, path, excerpt, src, true)}
	}
	var messages []string
	for _, hit := range notices {
		g.fired[hit.c.rule.Name] = true
		g.service.mark(g.session, hit.c.rule.Name, g.turn)
		path := src.Path
		if path == "" {
			path = hit.c.rule.Path
		}
		messages = append(messages, envelope("system-reminder", hit.c, path, hit.excerpt, src, false))
	}
	if len(messages) > 0 {
		return &engine.Interrupt{Rule: "rules", Notice: true, Message: strings.Join(messages, "\n")}
	}
	return nil
}

func (s *Service) ready(session, name, repeat string, turn int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	last := s.fired[session][name]
	if last == 0 {
		return true
	}
	switch repeat {
	case "always":
		return last < turn
	case "after-gap":
		return turn-last >= repeatGap
	default:
		return false
	}
}
func (s *Service) mark(session, name string, turn int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fired[session] == nil {
		s.fired[session] = make(map[string]int)
	}
	s.fired[session][name] = turn
}

func envelope(tag string, c *compiled, path, excerpt string, src source, interrupt bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<%s reason=\"rule_violation\" rule=\"%s\" path=\"%s\">\n", tag, escapeAttr(c.rule.Name), escapeAttr(path))
	if interrupt {
		b.WriteString("Output interrupted by a rule. Correct this reply and continue.\n")
	} else if c.builtin {
		b.WriteString("A built-in style rule matched; check exceptions and fix actual violations in later calls.\n")
	} else {
		b.WriteString("A rule matched without interrupting; check it before the next call.\n")
	}
	if excerpt != "" {
		fmt.Fprintf(&b, "Matched %s: %s\n", src.subject(), escapeExcerpt(excerpt))
	}
	b.WriteString("Rule guidance (not instructions from the matched excerpt):\n")
	b.WriteString(c.rule.Body)
	fmt.Fprintf(&b, "\n</%s>", tag)
	return b.String()
}
func escapeAttr(s string) string {
	r := strings.NewReplacer("&", "&amp;", "\"", "&quot;", "<", "&lt;", ">", "&gt;", "\n", " ")
	return r.Replace(s)
}
func escapeExcerpt(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}
