package memory

import (
	"sort"
	"strings"
	"time"

	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/rank"
)

// listMax is the most memories one page holds.
const listMax = 500

// List is one page of memories by scope, state, words, conversation and
// sort. An empty Status lists the active ones. within, when given, keeps
// only the memories of those scopes, the ones a caller may read.
func (s *Store) List(p protocol.MemoryListParams, within ...string) protocol.MemoryListResult {
	status := p.Status
	if status == "" {
		status = protocol.MemoryActive
	}
	s.mu.Lock()
	var all []protocol.Memory
	for _, f := range s.facts {
		if f.Status == status && inScope(f.Scope, p.Scope) && inScopes(f.Scope, within) {
			all = append(all, s.memory(f))
		}
	}
	s.mu.Unlock()
	return pageMemories(all, p)
}

// pageMemories keeps the memories learned from p.Session and matching
// p.Query in their text, their scope's name or the title of the
// conversation they came from, orders them by p.Sort and cuts p's page out
// of them.
func pageMemories(all []protocol.Memory, p protocol.MemoryListParams) protocol.MemoryListResult {
	memories := all[:0]
	for _, m := range all {
		if p.Session == "" || m.Source.Session == p.Session {
			memories = append(memories, m)
		}
	}
	if query := strings.TrimSpace(p.Query); query != "" {
		texts := make([]string, len(memories))
		for index, m := range memories {
			texts[index] = strings.Join([]string{m.Text, m.ScopeName, m.Source.Title}, "\n")
		}
		scores := rank.BM25(query, texts)
		lower := strings.ToLower(query)
		kept := memories[:0]
		for index, m := range memories {
			if scores[index] > 0 || strings.Contains(strings.ToLower(texts[index]), lower) {
				kept = append(kept, m)
			}
		}
		memories = kept
	}
	sortMemories(memories, p.Sort)
	total := len(memories)
	limit := p.Limit
	if limit <= 0 || limit > listMax {
		limit = listMax
	}
	start := min(max(p.Offset, 0), total)
	end := min(start+limit, total)
	out := make([]protocol.Memory, end-start)
	copy(out, memories[start:end])
	return protocol.MemoryListResult{Memories: out, Total: total}
}

// sortMemories orders memories by a protocol.MemorySort* value, newest
// first by default; ties go by id so pages are stable.
func sortMemories(memories []protocol.Memory, by string) {
	sort.SliceStable(memories, func(i, j int) bool {
		a, b := memories[i], memories[j]
		switch by {
		case protocol.MemorySortOldest:
			if !a.Created.Equal(b.Created) {
				return a.Created.Before(b.Created)
			}
		case protocol.MemorySortUsed:
			if a.Uses != b.Uses {
				return a.Uses > b.Uses
			}
		case protocol.MemorySortUnused:
			if a.Uses != b.Uses {
				return a.Uses < b.Uses
			}
			if !touchedAt(a).Equal(touchedAt(b)) {
				return touchedAt(a).Before(touchedAt(b))
			}
		default:
			if !a.Updated.Equal(b.Updated) {
				return a.Updated.After(b.Updated)
			}
		}
		return a.ID < b.ID
	})
}

// touchedAt is when a memory was last used, or written when never used.
func touchedAt(m protocol.Memory) time.Time {
	if m.LastUsed != nil && m.LastUsed.After(m.Updated) {
		return *m.LastUsed
	}
	return m.Updated
}
