package memory

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.etcd.io/bbolt"

	"ryoku-rashin/internal/protocol"
)

// statusDays is how many days the memory chart shows.
const statusDays = 30

// builtinCaps are what the built-in engine can do: everything, on this
// computer.
var builtinCaps = protocol.MemoryCaps{Edit: true, Replace: true, Pin: true, Versions: true, ForgetSession: true}

// Status is memory as a whole. While memory has never been opened it reads
// the settings alone and creates nothing.
func (e *Engine) Status(ctx context.Context) protocol.MemoryStatus {
	s := e.settings()
	engine := s.Engine
	if engine == "" {
		engine = protocol.MemoryBuiltin
	}
	out := protocol.MemoryStatus{Engine: engine, EngineState: protocol.EngineReady, Caps: builtinCaps, Learner: e.learn.state(), Path: e.deps.Dir}
	out.Card.Limit = protocol.MemoryCardMax
	st, ok := e.storeIfAny()
	if ok {
		out.Counts, out.Scopes = st.counts()
		out.Days = st.days(time.Now(), out.Counts)
		for _, day := range out.Days {
			out.Counts.LearnedMonth += day.Learned
			out.Counts.ForgotMonth += day.Forgot
		}
		if card, err := st.Card(); err == nil {
			out.Card = card
		}
		e.mu.Lock()
		for index, scope := range out.Scopes {
			out.Scopes[index].Mode = e.modes[scope.ID]
		}
		e.mu.Unlock()
	}
	out.Bytes = folderBytes(e.deps.Dir) + folderBytes(e.deps.TeamDir)
	if engine == protocol.MemoryBuiltin {
		return out
	}
	if !e.on() {
		// an external engine is not opened, let alone started, while memory is off
		out.EngineState = protocol.EngineStopped
		return out
	}
	b, err := e.external(ctx)
	if err != nil || b == nil {
		out.EngineState = protocol.EngineUnreachable
		if engine == protocol.MemoryHindsight && s.Hindsight.Local {
			out.EngineState = protocol.EngineSetup
		}
		if err != nil {
			out.EngineDetail = err.Error()
		}
		return out
	}
	status := b.Status(ctx)
	out.EngineState, out.EngineDetail = status.State, status.Detail
	caps := b.Caps()
	out.Caps = caps.MemoryCaps
	if status.State != protocol.EngineReady {
		// counting would start a stopped local engine; the state says why there are no numbers
		return out
	}
	suggested, blocked := out.Counts.Suggested, out.Counts.Blocked
	out.Counts = protocol.MemoryCounts{Suggested: suggested, Blocked: blocked, LearnedMonth: out.Counts.LearnedMonth, ForgotMonth: out.Counts.ForgotMonth}
	for _, group := range []struct {
		scope string
		count *int
	}{{"", &out.Counts.All}, {protocol.MemoryYou, &out.Counts.You}, {protocol.MemoryProjects, &out.Counts.Projects}, {protocol.MemoryTeams, &out.Counts.Teams}} {
		if _, total, err := b.List(ctx, group.scope, 0, 0); err == nil {
			*group.count = total
		}
	}
	if caps.Card {
		if text, err := b.Card(ctx); err == nil && text != "" {
			out.Card = protocol.MemoryCard{Text: text, Limit: protocol.MemoryCardMax, Built: time.Now()}
		}
	}
	return out
}

// counts tallies the memories by scope group and state, and lists every
// scope with active memories, you first, then by name.
func (s *Store) counts() (protocol.MemoryCounts, []protocol.MemoryScope) {
	s.mu.Lock()
	var c protocol.MemoryCounts
	byScope := map[string]int{}
	for _, f := range s.facts {
		switch f.Status {
		case protocol.MemorySuggested:
			c.Suggested++
		case protocol.MemoryReplaced:
			c.Replaced++
		case protocol.MemoryBlocked:
			c.Blocked++
		case protocol.MemoryActive:
			c.All++
			byScope[f.Scope]++
			switch {
			case f.Scope == protocol.MemoryYou:
				c.You++
			case strings.HasPrefix(f.Scope, protocol.MemoryProjectPrefix):
				c.Projects++
			case strings.HasPrefix(f.Scope, protocol.MemoryTeamPrefix):
				c.Teams++
			}
		}
	}
	s.mu.Unlock()
	scopes := make([]protocol.MemoryScope, 0, len(byScope))
	for scope, count := range byScope {
		scopes = append(scopes, protocol.MemoryScope{ID: scope, Name: s.scopeName(scope), Count: count})
	}
	sort.Slice(scopes, func(i, j int) bool {
		if (scopes[i].ID == protocol.MemoryYou) != (scopes[j].ID == protocol.MemoryYou) {
			return scopes[i].ID == protocol.MemoryYou
		}
		if scopes[i].Name != scopes[j].Name {
			return scopes[i].Name < scopes[j].Name
		}
		return scopes[i].ID < scopes[j].ID
	})
	return c, scopes
}

// days is the chart: the last statusDays days, oldest first, with what was
// learned and forgotten on each and how many memories were in use at its
// end, worked back from today's count.
func (s *Store) days(now time.Time, counts protocol.MemoryCounts) []protocol.MemoryDay {
	out := make([]protocol.MemoryDay, statusDays)
	_ = s.db.View(func(tx *bbolt.Tx) error {
		for index := range out {
			day := now.AddDate(0, 0, index-statusDays+1).Format(time.DateOnly)
			var count dayCount
			_, _ = getJSON(tx, bucketDays, day, &count)
			out[index] = protocol.MemoryDay{Day: day, Learned: count.Learned, Forgot: count.Forgot}
		}
		return nil
	})
	inUse := counts.All
	for index := len(out) - 1; index >= 0; index-- {
		out[index].InUse = max(0, inUse)
		inUse = inUse - out[index].Learned + out[index].Forgot
	}
	return out
}

// folderBytes is the size of the memory files in dir and its projects
// folder.
func folderBytes(dir string) int64 {
	var total int64
	for _, pattern := range []string{filepath.Join(dir, "*.md"), filepath.Join(dir, "projects", "*.md")} {
		matches, _ := filepath.Glob(pattern)
		for _, path := range matches {
			if info, err := os.Stat(path); err == nil {
				total += info.Size()
			}
		}
	}
	return total
}
