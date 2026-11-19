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
				{ID: "openrouter", Enabled: true, Configured: true, Tier: protocol.TierFree},
				{ID: "anthropic", Enabled: true, Configured: true, Tier: protocol.TierPaid},
			}
		},
		ProviderModels: func(_ context.Context, provider string) []protocol.ModelInfo {
			switch provider {
			case protocol.ProviderOllamaLocal:
				return []protocol.ModelInfo{{ID: protocol.ProviderOllamaLocal + "/qwen3", Available: true, Tools: true}}
			case "openrouter":
				return []protocol.ModelInfo{{ID: "openrouter/some-model", Available: true, Tier: protocol.TierFree, Tools: true}}
			case "anthropic":
				return []protocol.ModelInfo{{ID: "anthropic/claude", Available: true, Tier: protocol.TierPaid, Tools: true}}
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
		{protocol.LearnLocalFirst, "openrouter/some-model", protocol.RouteAuto, true},
		{protocol.LearnLocalFirst, protocol.ProviderOllamaLocal + "/qwen3", protocol.ProviderOllamaLocal + "/qwen3", false},
		{protocol.LearnLocalFirst, "", protocol.RouteAuto, true},
		{protocol.LearnLocal, "openrouter/some-model", "", false},
		{protocol.LearnLocal, "openrouter/some-model", protocol.ProviderOllamaLocal + "/qwen3", true},
		{protocol.LearnAny, "", protocol.RouteAuto, false},
		{protocol.LearnAny, "anthropic/claude", protocol.RouteAuto, false},
	} {
		localUp = c.local
		e.settings.Model = c.choice
		if got := e.learnModel(context.Background(), c.answered); got != c.want {
			t.Errorf("%s, answered %q, local %v: learns with %q, want %q", c.choice, c.answered, c.local, got, c.want)
		}
	}
}

func TestLearningRouteNeverBillsOrChangesRemoteProvider(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name, choice, answered, providerTier, modelTier, want string
		local, available, waiting, wantLocal                  bool
	}{
		{name: "free fallback", choice: protocol.LearnLocalFirst, answered: "openrouter/free", providerTier: protocol.TierFree, modelTier: protocol.TierFree, available: true, want: "openrouter/free"},
		{name: "credits fallback", choice: protocol.LearnLocalFirst, answered: "openrouter/credits", providerTier: protocol.TierFree, modelTier: protocol.TierCredits, available: true, want: "openrouter/credits"},
		{name: "available local first", choice: protocol.LearnLocalFirst, answered: "openrouter/free", local: true, want: protocol.RouteAuto, wantLocal: true},
		{name: "unanswered waits", choice: protocol.LearnLocalFirst, waiting: true},
		{name: "unanswered stays local", choice: protocol.LearnLocalFirst, local: true, want: protocol.RouteAuto, wantLocal: true},
		{name: "answered local stays local", choice: protocol.LearnLocalFirst, answered: protocol.ProviderOllamaLocal + "/qwen3", want: protocol.ProviderOllamaLocal + "/qwen3", wantLocal: true},
		{name: "local waits", choice: protocol.LearnLocal, answered: "openrouter/free", providerTier: protocol.TierFree, modelTier: protocol.TierFree, available: true, waiting: true},
		{name: "local uses local", choice: protocol.LearnLocal, local: true, want: protocol.RouteAuto, wantLocal: true},
		{name: "any follows auto", choice: protocol.LearnAny, answered: "openrouter/free", local: true, want: protocol.RouteAuto},
		{name: "paid provider waits", choice: protocol.LearnLocalFirst, answered: "openai/paid", providerTier: protocol.TierPaid, available: true, waiting: true},
		{name: "paid model at free provider waits", choice: protocol.LearnLocalFirst, answered: "openrouter/paid", providerTier: protocol.TierFree, modelTier: protocol.TierPaid, available: true, waiting: true},
		{name: "unknown tier waits", choice: protocol.LearnLocalFirst, answered: "openrouter/unknown", available: true, waiting: true},
		{name: "unavailable model waits", choice: protocol.LearnLocalFirst, answered: "openrouter/free", providerTier: protocol.TierFree, modelTier: protocol.TierFree, waiting: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model, why := pickModel(ctx, tc.choice, tc.answered, func(context.Context) []protocol.ProviderInfo {
				return []protocol.ProviderInfo{
					{ID: protocol.ProviderOllamaLocal, Enabled: true, Running: tc.local, Models: 1},
					{ID: "openrouter", Enabled: true, Tier: tc.providerTier},
					{ID: "openai", Enabled: true, Tier: tc.providerTier},
				}
			}, func(_ context.Context, provider string) []protocol.ModelInfo {
				if provider == protocol.ProviderOllamaLocal {
					return []protocol.ModelInfo{{ID: provider + "/qwen3", Available: true}}
				}
				return []protocol.ModelInfo{{ID: tc.answered, Tier: tc.modelTier, Available: tc.available, Context: 32768}}
			})
			if model.Model != tc.want || model.Local != tc.wantLocal || (why != "") != tc.waiting {
				t.Fatalf("route=%+v, waiting=%q; want model %q, local %v, waiting %v", model, why, tc.want, tc.wantLocal, tc.waiting)
			}
			if tc.want == "openrouter/free" && model.Context != 32768 {
				t.Fatalf("fallback lost model context: %+v", model)
			}
		})
	}
}

func TestLocalLearningCanWakeAnInstalledRashinModel(t *testing.T) {
	model, why := pickModel(context.Background(), protocol.LearnLocal, "", func(context.Context) []protocol.ProviderInfo {
		return []protocol.ProviderInfo{{ID: protocol.ProviderRashinModels, Enabled: true, Running: false, OnDemand: true, Models: 1}}
	}, func(_ context.Context, provider string) []protocol.ModelInfo {
		if provider != protocol.ProviderRashinModels {
			t.Fatalf("queried %s instead of Rashin models", provider)
		}
		return []protocol.ModelInfo{{ID: provider + "/lfm3", Available: true}}
	})
	if why != "" || model.Model != protocol.RouteAuto || !model.Local {
		t.Fatalf("route=%+v, waiting=%q", model, why)
	}
}

func TestHindsightLocalOnlyChoosesSmallestToolModel(t *testing.T) {
	model, why := hindsightModel(context.Background(), protocol.LearnLocal, "", func(context.Context) []protocol.ProviderInfo {
		return []protocol.ProviderInfo{
			{ID: protocol.ProviderRashinModels, Enabled: true, OnDemand: true, Models: 3},
			{ID: protocol.ProviderOllamaLocal, Enabled: true, Running: true, Models: 1},
		}
	}, func(_ context.Context, provider string) []protocol.ModelInfo {
		if provider == protocol.ProviderRashinModels {
			return []protocol.ModelInfo{
				{ID: provider + "/large", Available: true, Tools: true, Params: 8_000_000_000},
				{ID: provider + "/notools", Available: true, Params: 1_000_000_000},
				{ID: provider + "/small", Available: true, Tools: true, Light: true, Params: 2_000_000_000, Context: 32768},
			}
		}
		return []protocol.ModelInfo{{ID: provider + "/medium", Available: true, Tools: true, Params: 3_000_000_000}}
	})
	if why != "" || model.Model != protocol.ProviderRashinModels+"/small" || !model.Local || model.Context != 32768 {
		t.Fatalf("route=%+v, waiting=%q", model, why)
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
