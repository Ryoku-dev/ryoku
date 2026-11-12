package teamsui

import (
	"sort"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// an import can bring hundreds; the Memory page holds the rest
const suggestionsRead = 200

// suggestions have no snapshot event of their own, so they are read again whenever memory moves
type Suggestions struct {
	revision uint64
	read     bool
	busy     bool
	memories []protocol.Memory
}

// a failed read waits for the next change rather than asking every frame
func (s *Suggestions) Want(e *env.Env) {
	snapshot := e.Snapshot()
	if snapshot.Connection != state.Ready || s.busy || s.read && snapshot.MemoryRevision == s.revision {
		return
	}
	revision := snapshot.MemoryRevision
	s.busy = true
	e.ListMemory(protocol.MemoryListParams{Status: protocol.MemorySuggested, Sort: protocol.MemorySortOldest, Limit: suggestionsRead},
		func(result protocol.MemoryListResult, err error) {
			s.busy, s.read, s.revision = false, true, revision
			if err != nil {
				e.Report(err)
				return
			}
			s.memories = result.Memories
		})
}

// a card is as old as its oldest suggestion, so a run's later facts never jump the queue
func (s *Suggestions) Needs() []Need {
	var needs []Need
	at := make(map[string]int)
	for _, memory := range s.memories {
		key := suggestionKey(memory)
		index, ok := at[key]
		if !ok {
			at[key] = len(needs)
			needs = append(needs, Need{Memories: []protocol.Memory{memory}, At: memory.Created})
			continue
		}
		needs[index].Memories = append(needs[index].Memories, memory)
		if memory.Created.Before(needs[index].At) {
			needs[index].At = memory.Created
		}
	}
	return needs
}

// suggestions stand among the other needs in age order, so the gold still goes to the oldest
func WithSuggestions(needs, suggestions []Need) []Need {
	all := append(append(make([]Need, 0, len(needs)+len(suggestions)), needs...), suggestions...)
	sort.SliceStable(all, func(i, j int) bool { return all[i].At.Before(all[j].At) })
	return all
}

// suggestions from one task, one conversation or one import share a card
func suggestionKey(memory protocol.Memory) string {
	source := memory.Source
	switch {
	case source.Task != "":
		return "memory:task:" + source.Task
	case source.Session != "":
		return "memory:session:" + source.Session
	case source.Kind == protocol.MemoryFromImport:
		return "memory:import:" + source.From + ":" + memory.Created.Format("2006-01-02")
	}
	return "memory:" + memory.ID
}
