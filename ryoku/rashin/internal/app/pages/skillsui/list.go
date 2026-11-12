// Package skillsui holds what Settings and a team's Settings share about the
// built-in skill sets: the daemon's list of them, asked for once per visit.
package skillsui

import (
	"slices"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/protocol"
)

// List is the daemon's answer to skills.list. Whether a set is on comes from
// the settings, which change without the list being asked again.
type List struct {
	result  *protocol.SkillsResult
	asking  bool
	problem string
}

// Sets asks the daemon the first time it is wanted and says whether the
// answer has arrived. Problem is what went wrong when it could not be read.
func (l *List) Sets(e *env.Env) (protocol.SkillsResult, bool) {
	if l.result == nil && !l.asking && l.problem == "" {
		l.asking = true
		var result protocol.SkillsResult
		e.Call(protocol.MSkillsList, nil, &result, func(err error) {
			l.asking = false
			if err != nil {
				l.problem = err.Error()
				return
			}
			l.result = &result
		})
	}
	if l.result == nil {
		return protocol.SkillsResult{}, false
	}
	return *l.result, true
}

// Problem says why the list could not be read, empty while it is fine.
func (l *List) Problem() string { return l.problem }

// Forget drops the answer so the next visit asks again: a command a set needs
// may have been installed since.
func (l *List) Forget() {
	if !l.asking {
		l.result, l.problem = nil, ""
	}
}

// On says whether a set is on for every model under these settings.
func On(settings protocol.SkillsSettings, set string) bool {
	return !slices.Contains(settings.Off, set)
}

// Turn is off with set turned on or off. Ids it doesn't know stay, so
// turning one set on never drops another's choice.
func Turn(off []string, set string, on bool) []string {
	kept := make([]string, 0, len(off)+1)
	for _, id := range off {
		if id != set {
			kept = append(kept, id)
		}
	}
	if !on {
		kept = append(kept, set)
	}
	return kept
}
