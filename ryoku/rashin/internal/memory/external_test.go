package memory

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"ryoku-rashin/internal/memory/backend"
	"ryoku-rashin/internal/protocol"
)

func TestHindsightLearnsWithTheSharedPolicy(t *testing.T) {
	localUp := false
	e := &engines{deps: EngineDeps{
		Models: func(context.Context) []protocol.ProviderInfo {
			return []protocol.ProviderInfo{
				{ID: protocol.ProviderOllamaLocal, Enabled: true, Running: localUp, Models: 1, Local: true},
				{ID: "openrouter", Enabled: true, Configured: true},
			}
		},
		ProviderModels: func(_ context.Context, provider string) []protocol.ModelInfo {
			if provider == protocol.ProviderOllamaLocal {
				return []protocol.ModelInfo{{ID: protocol.ProviderOllamaLocal + "/qwen3", Available: true}}
			}
			return nil
		},
	}}
	for _, c := range []struct {
		choice, answered, want string
		local                  bool
	}{
		{protocol.LearnLocalFirst, "", "", false},
		{protocol.LearnLocalFirst, "anthropic/claude", "", false},
		{protocol.LearnLocalFirst, "openrouter/some-model", "openrouter/some-model", false},
		{protocol.LearnLocal, "openrouter/some-model", "", false},
		{protocol.LearnAny, "", protocol.RouteAuto, false},
		{protocol.LearnLocalFirst, "openrouter/some-model", protocol.ProviderOllamaLocal + "/qwen3", true},
	} {
		localUp = c.local
		e.settings.Model = c.choice
		if got := e.learnModel(context.Background(), c.answered); got != c.want {
			t.Errorf("%s, answered %q, local %v: learns with %q, want %q", c.choice, c.answered, c.local, got, c.want)
		}
	}
}

// moveTarget is an engine that keeps what it is given and fails the
// Remember call numbered failAt.
type moveTarget struct {
	backend.Backend
	leaves bool
	failAt int
	calls  int
	next   int
	kept   []protocol.Memory
}

func (m *moveTarget) Caps() backend.Caps {
	return backend.Caps{MemoryCaps: protocol.MemoryCaps{LeavesMachine: m.leaves}}
}

func (m *moveTarget) Remember(_ context.Context, facts []protocol.Memory) ([]protocol.Memory, error) {
	m.calls++
	if m.calls == m.failAt {
		return nil, errors.New("the engine went away")
	}
	out := make([]protocol.Memory, 0, len(facts))
	for _, f := range facts {
		f.ID = fmt.Sprintf("e%d", m.next)
		m.next++
		m.kept = append(m.kept, f)
		out = append(out, f)
	}
	return out, nil
}

func (m *moveTarget) Forget(_ context.Context, ids []string) error {
	m.kept = slices.DeleteFunc(m.kept, func(f protocol.Memory) bool { return slices.Contains(ids, f.ID) })
	return nil
}

func facts(n int) []protocol.Memory {
	out := make([]protocol.Memory, n)
	for i := range out {
		out[i] = protocol.Memory{ID: fmt.Sprintf("m%07d", i), Scope: protocol.MemoryYou, Text: fmt.Sprintf("Fact %d.", i), Sensitive: i%10 == 0}
	}
	return out
}

func TestFailedMoveLeavesNothing(t *testing.T) {
	target := &moveTarget{failAt: 2}
	if err := moveInto(context.Background(), target, facts(2*moveBatch)); err == nil {
		t.Fatal("the failed move was not reported")
	}
	if len(target.kept) != 0 {
		t.Fatalf("a failed move left %d memories behind", len(target.kept))
	}
	if err := moveInto(context.Background(), target, facts(2*moveBatch)); err != nil {
		t.Fatal(err)
	}
	if len(target.kept) != 2*moveBatch {
		t.Fatalf("moving again kept %d memories, want %d", len(target.kept), 2*moveBatch)
	}
}

func TestSensitiveMemoriesStayOnThisComputer(t *testing.T) {
	away := &moveTarget{leaves: true}
	if err := moveInto(context.Background(), away, facts(20)); err != nil {
		t.Fatal(err)
	}
	if len(away.kept) != 18 || slices.ContainsFunc(away.kept, func(m protocol.Memory) bool { return m.Sensitive }) {
		t.Fatalf("an engine off this computer got %d memories, some sensitive", len(away.kept))
	}
	here := &moveTarget{}
	if err := moveInto(context.Background(), here, facts(20)); err != nil {
		t.Fatal(err)
	}
	if len(here.kept) != 20 {
		t.Fatalf("a local engine got %d of 20 memories", len(here.kept))
	}
}
