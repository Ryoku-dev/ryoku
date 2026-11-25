package reflex

import (
	"time"

	i18n "ryoku-i18n"
)

// starts are the intents that start something: each target starts at most
// once per reflex.launchCooldownS.
var starts = map[Intent]bool{OpenApp: true, OpenSite: true, WebSearch: true, NewTab: true, Screenshot: true, RunRoutine: true}

// Session plans utterance after utterance and keeps the bounds that span
// them: a verb said on its own ("open") waits reflex.danglingVerbS for its
// object; the same app, site or search starts at most once per
// reflex.launchCooldownS; and at most reflex.maxActionsPerMinute actions
// pass in any minute. Every time is handed in; a Session never reads the
// clock. The zero Session is ready to use. A Session is not safe for
// concurrent use.
type Session struct {
	verb    string
	verbAt  time.Time
	started map[string]time.Time
	passed  []time.Time
}

// Plan plans text heard at at. A verb left waiting is put in front of the
// next utterance when that one is claimed by nothing on its own. The
// result is a Parse result trimmed by the bounds; a plan whose actions were
// all held back says so in its Line.
func (s *Session) Plan(text string, at time.Time, cat Catalog) (Plan, bool) {
	p, verb, ok := s.plan(text, at, cat)
	if !ok {
		return Plan{}, false
	}
	if verb != "" {
		s.verb, s.verbAt = verb, at
		return Plan{}, true
	}
	return s.admit(p, at), true
}

// Early is for a transcript still being spoken: it returns the one action
// it plans when that is opening an app or a site at a confidence of at
// least reflex.earlyFireMin and the bounds let it pass, and counts it, so
// the full transcript that follows does not open it again.
func (s *Session) Early(text string, at time.Time, cat Catalog) (Action, bool) {
	p, verb, ok := s.plan(text, at, cat)
	if !ok || verb != "" || len(p.Actions) != 1 {
		return Action{}, false
	}
	a := p.Actions[0]
	if (a.Intent != OpenApp && a.Intent != OpenSite) || a.Confidence < limits().EarlyFireMin {
		return Action{}, false
	}
	if q := s.admit(p, at); len(q.Actions) == 1 {
		return a, true
	}
	return Action{}, false
}

// plan parses text, first on its own and then after a waiting verb.
func (s *Session) plan(text string, at time.Time, cat Catalog) (Plan, string, bool) {
	if s.verb != "" && (at.Before(s.verbAt) || at.Sub(s.verbAt) > limits().DanglingVerb()) {
		s.verb = ""
	}
	p, verb, ok := plan(text, cat)
	if s.verb != "" && (!ok || len(p.Actions) == 0 && verb == "") {
		if q, v, ok2 := plan(s.verb+" "+clean(text), cat); ok2 && len(q.Actions) > 0 {
			p, verb, ok = q, v, true
		}
	}
	if ok {
		s.verb = ""
	}
	return p, verb, ok
}

// admit drops the actions the bounds hold back and counts the rest.
func (s *Session) admit(p Plan, at time.Time) Plan {
	lim := limits()
	cool := lim.LaunchCooldown()
	for k, t := range s.started {
		if at.Sub(t) >= cool || at.Before(t) {
			delete(s.started, k)
		}
	}
	kept := s.passed[:0]
	for _, t := range s.passed {
		if at.Sub(t) < time.Minute && !at.Before(t) {
			kept = append(kept, t)
		}
	}
	s.passed = kept

	var out []Action
	cooled, capped := false, false
	for _, a := range p.Actions {
		key := string(a.Intent) + "\x00" + a.Arg
		if starts[a.Intent] {
			if _, hot := s.started[key]; hot {
				cooled = true
				continue
			}
		}
		if len(s.passed) >= lim.MaxActionsPerMinute {
			capped = true
			continue
		}
		if starts[a.Intent] {
			if s.started == nil {
				s.started = map[string]time.Time{}
			}
			s.started[key] = at
		}
		s.passed = append(s.passed, at)
		out = append(out, a)
	}
	if len(out) == len(p.Actions) {
		return p
	}
	q := Plan{Actions: out}
	switch {
	case len(out) == 1:
		q.Line = line(out[0])
	case len(out) > 1:
		q.Line = i18n.T(lineMany)
	case capped:
		q.Line = i18n.T(lineEnough)
	case cooled:
		q.Line = i18n.T(lineAlready)
	}
	return q
}
