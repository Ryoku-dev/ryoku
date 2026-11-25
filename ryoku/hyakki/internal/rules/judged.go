package rules

import (
	"context"
	"strings"
	"time"

	"ryoku-rashin/internal/engine"
	"ryoku-rashin/internal/judge/question"
)

var _ engine.TurnObserver = (*Service)(nil)

// A judged rule reads completed output: unlike a stream condition it cannot
// undo a tool call. Its warning belongs to the next turn's rules block.
func (s *Service) TurnEnded(ctx context.Context, rec engine.TurnRecord) {
	if !s.enabled() || s.deps.Judge == nil {
		return
	}
	var candidates []*compiled
	for _, c := range s.discovered(ctx, rec.Project) {
		if c.judged() {
			candidates = append(candidates, c)
		}
	}
	if len(candidates) == 0 {
		return
	}
	var outputs []source
	if rec.Text != "" {
		outputs = append(outputs, source{Kind: SourceText, Text: rec.Text})
	}
	for _, call := range rec.Calls {
		outputs = append(outputs, callSources(call.Name, string(call.Args))...)
	}
	if len(outputs) == 0 {
		return
	}
	// The observer runs after the turn and must not delay a new prompt.
	go func() {
		judgeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		for _, c := range candidates {
			if !s.ready(rec.Session.ID, c.rule.Name, c.rule.Repeat, rec.Turn) {
				continue
			}
			for _, src := range outputs {
				if !c.applies(src) {
					continue
				}
				if len(c.conditions) > 0 {
					if _, ok := c.find(src.Text); !ok {
						continue
					}
				}
				verdict, err := s.deps.Judge.Ask(judgeCtx, question.RuleViolated(c.rule.Question, src.subject(), src.Text), question.YesNo)
				if err != nil || question.YesProbability(verdict.JSON, verdict.Probability) < question.RuleThreshold {
					continue
				}
				s.mark(rec.Session.ID, c.rule.Name, rec.Turn)
				note := "A judged rule found a problem in the previous reply: " + c.rule.Name + ". " + strings.TrimSpace(c.rule.Body)
				s.mu.Lock()
				s.pending[rec.Session.ID] = append(s.pending[rec.Session.ID], note)
				s.mu.Unlock()
				break
			}
		}
	}()
}
