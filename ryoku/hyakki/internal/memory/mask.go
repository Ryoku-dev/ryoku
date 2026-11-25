package memory

import (
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sensitive"
)

// A memory kept before the door refused card, social security, identity and
// bank account numbers may still hold one. Masking hides such a number
// wherever memory shows a memory: every memory.* result, the change events,
// the card and every fact recalled into a prompt. It is presentation only:
// the stored record keeps its text until the user edits or forgets it, so a
// move between engines copies it as it is (and the built-in store's door
// then leaves it behind).

// maskMemory is m with every number in its texts masked.
func maskMemory(m protocol.Memory) protocol.Memory {
	m.Text = sensitive.Mask(m.Text)
	m.ReplacedByText = sensitive.Mask(m.ReplacedByText)
	if len(m.Versions) > 0 {
		versions := make([]protocol.MemoryVersion, len(m.Versions))
		for index, v := range m.Versions {
			v.Text = sensitive.Mask(v.Text)
			versions[index] = v
		}
		m.Versions = versions
	}
	return m
}

// maskMemories is a masked copy of memories; nil stays nil.
func maskMemories(memories []protocol.Memory) []protocol.Memory {
	if memories == nil {
		return nil
	}
	out := make([]protocol.Memory, len(memories))
	for index, m := range memories {
		out[index] = maskMemory(m)
	}
	return out
}

// maskItems is a masked copy of a change's items; nil stays nil.
func maskItems(items []protocol.MemoryItem) []protocol.MemoryItem {
	if items == nil {
		return nil
	}
	out := make([]protocol.MemoryItem, len(items))
	for index, item := range items {
		item.Text = sensitive.Mask(item.Text)
		item.Replaces = sensitive.Mask(item.Replaces)
		out[index] = item
	}
	return out
}

// maskChange is ch with every item's texts masked.
func maskChange(ch protocol.MemoryChange) protocol.MemoryChange {
	ch.Items = maskItems(ch.Items)
	return ch
}

// masked is a memory.* result as a client may see it.
func masked(result any) any {
	switch r := result.(type) {
	case protocol.Memory:
		return maskMemory(r)
	case protocol.MemoryListResult:
		r.Memories = maskMemories(r.Memories)
		return r
	case protocol.MemorySearchResult:
		r.Memories = maskMemories(r.Memories)
		return r
	case protocol.MemoryChange:
		return maskChange(r)
	case protocol.MemoryChangesResult:
		changes := make([]protocol.MemoryChange, len(r.Changes))
		for index, ch := range r.Changes {
			changes[index] = maskChange(ch)
		}
		r.Changes = changes
		return r
	case protocol.MemoryCard:
		r.Text = sensitive.Mask(r.Text)
		return r
	}
	return result
}
