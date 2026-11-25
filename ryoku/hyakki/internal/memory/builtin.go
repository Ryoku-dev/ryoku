package memory

import (
	"context"

	"ryoku-rashin/internal/memory/backend"
	"ryoku-rashin/internal/protocol"
)

// builtin is the built-in store in the shape every engine has, for the
// engines' shared contract and for copying memory between engines. Its
// learning is Rashin's own learner, so Observe and Card do nothing here.
type builtin struct {
	s *Store
}

// Builtin wraps a store as a backend.Backend.
func Builtin(s *Store) backend.Backend { return builtin{s: s} }

func (b builtin) Caps() backend.Caps { return backend.Caps{MemoryCaps: builtinCaps} }

func (b builtin) Remember(ctx context.Context, facts []protocol.Memory) ([]protocol.Memory, error) {
	out := make([]protocol.Memory, 0, len(facts))
	for _, m := range facts {
		op := Op{Kind: OpAdd, Scope: m.Scope, Text: m.Text, Retain: m.Kind, Pinned: m.Pinned, Sensitive: m.Sensitive, Source: m.Source}
		change, err := b.s.Apply(Origin{By: protocol.MemoryByYou}, []Op{op})
		if err != nil {
			return out, err
		}
		id := ""
		if len(change.Items) > 0 {
			id = change.Items[0].ID
		} else if same, ok, err := b.s.Duplicate(ctx, m.Text, m.Scope); err == nil && ok {
			id = same
		}
		if kept, ok := b.s.Get(id); ok {
			out = append(out, kept)
		}
	}
	return out, nil
}

func (b builtin) Edit(_ context.Context, id, text string) (protocol.Memory, error) {
	if _, err := b.s.Apply(Origin{By: protocol.MemoryByYou}, []Op{{Kind: OpEdit, ID: id, Text: text}}); err != nil {
		return protocol.Memory{}, err
	}
	m, ok := b.s.Get(id)
	if !ok {
		return protocol.Memory{}, errNotFound(id)
	}
	return m, nil
}

func (b builtin) Recall(ctx context.Context, q backend.Query) ([]protocol.Memory, error) {
	return b.s.Search(ctx, q.Text, q.Scopes, q.Limit)
}

func (b builtin) Observe(context.Context, backend.Transcript) error { return nil }

func (b builtin) List(_ context.Context, scope string, offset, limit int) ([]protocol.Memory, int, error) {
	if limit <= 0 {
		result := b.s.List(protocol.MemoryListParams{Scope: scope, Offset: offset, Limit: 1})
		return nil, result.Total, nil
	}
	result := b.s.List(protocol.MemoryListParams{Scope: scope, Offset: offset, Limit: limit})
	return result.Memories, result.Total, nil
}

func (b builtin) Forget(_ context.Context, ids []string) error {
	_, err := b.s.Apply(Origin{By: protocol.MemoryByYou}, idOps(OpForget, ids))
	return err
}

func (b builtin) ForgetScope(_ context.Context, scope string) error {
	_, err := b.s.ForgetAll(scope)
	return err
}

func (b builtin) Card(context.Context) (string, error) { return "", nil }

func (b builtin) Status(context.Context) backend.Status {
	return backend.Status{State: protocol.EngineReady}
}

func (b builtin) Busy() bool { return false }

func (b builtin) Settle(context.Context) {}

func (b builtin) Close() error { return b.s.Close() }
