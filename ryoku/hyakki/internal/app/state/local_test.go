package state

import (
	"encoding/json"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func applyLocal(t *testing.T, store *Store, event string, data any) {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Apply(protocol.Event{Event: event, Data: raw}); err != nil {
		t.Fatal(err)
	}
}

// a finished or paused row must not keep an old progress blade, while another model's tick stays intact
func TestLocalProgressFollowsADownloadUntilTheModelSettles(t *testing.T) {
	store := New(nil)
	downloading := protocol.LocalResult{Models: []protocol.LocalModel{
		{ID: "lfm3", State: protocol.LocalDownloading, Files: []protocol.LocalFile{{Name: "lfm3.gguf", Size: 100}}},
		{ID: "gemma", State: protocol.LocalDownloading},
	}}
	store.ApplySnapshot(protocol.Snapshot{Local: downloading})
	if got := store.Snapshot(); len(got.Local.Models) != 2 || len(got.LocalProgress) != 0 {
		t.Fatalf("after the snapshot: models %d, progress %v", len(got.Local.Models), got.LocalProgress)
	}
	applyLocal(t, store, protocol.EvtLocalProgress, protocol.LocalProgress{ID: "lfm3", Done: 40, Total: 100, Phase: protocol.LocalPhaseDownload})
	applyLocal(t, store, protocol.EvtLocalProgress, protocol.LocalProgress{ID: "gemma", Done: 10, Total: 50, Phase: protocol.LocalPhaseDownload})
	applyLocal(t, store, protocol.EvtLocalProgress, protocol.LocalProgress{ID: "lfm3", Done: 70, Total: 100, Phase: protocol.LocalPhaseDownload})
	if got := store.Snapshot().LocalProgress["lfm3"]; got.Done != 70 {
		t.Fatalf("lfm3 progress = %+v, want the latest tick", got)
	}
	settled := downloading
	settled.Models = append([]protocol.LocalModel(nil), downloading.Models...)
	settled.Models[0].State = protocol.LocalInstalled
	applyLocal(t, store, protocol.EvtLocalChanged, settled)
	got := store.Snapshot()
	if _, kept := got.LocalProgress["lfm3"]; kept {
		t.Fatal("an installed model kept its download progress")
	}
	if got.LocalProgress["gemma"].Done != 10 {
		t.Fatalf("the other download lost its progress: %v", got.LocalProgress)
	}
	if got.Local.Models[0].State != protocol.LocalInstalled {
		t.Fatalf("local.changed did not replace the models: %+v", got.Local.Models[0])
	}
	applyLocal(t, store, protocol.EvtLocalProgress, protocol.LocalProgress{ID: "gemma", Phase: protocol.LocalPhaseCancelled})
	if _, kept := store.Snapshot().LocalProgress["gemma"]; kept {
		t.Fatal("a cancelled download kept its progress")
	}
}

// a runtime download has no model state to retain its progress, so other model changes must not clear it
func TestRuntimeDownloadProgressOutlivesOtherChanges(t *testing.T) {
	store := New(nil)
	missing := protocol.LocalResult{Runtime: protocol.LocalRuntime{State: protocol.RuntimeMissing}}
	store.ApplySnapshot(protocol.Snapshot{Local: missing})
	applyLocal(t, store, protocol.EvtLocalProgress, protocol.LocalProgress{ID: protocol.LocalRuntimeID, Done: 5, Total: 37, Phase: protocol.LocalPhaseDownload})
	applyLocal(t, store, protocol.EvtLocalChanged, missing)
	if got := store.Snapshot().LocalProgress[protocol.LocalRuntimeID]; got.Done != 5 {
		t.Fatalf("a change elsewhere dropped the runtime's progress: %+v", got)
	}
	applyLocal(t, store, protocol.EvtLocalProgress, protocol.LocalProgress{ID: protocol.LocalRuntimeID, Phase: protocol.LocalPhaseError, Error: "The download did not match."})
	applyLocal(t, store, protocol.EvtLocalChanged, missing)
	if got := store.Snapshot().LocalProgress[protocol.LocalRuntimeID]; got.Error == "" {
		t.Fatal("the failure was dropped while llama.cpp is still missing")
	}
	store.SetLocal(protocol.LocalResult{Runtime: protocol.LocalRuntime{State: protocol.RuntimeStopped, Source: protocol.RuntimeManaged}})
	if _, kept := store.Snapshot().LocalProgress[protocol.LocalRuntimeID]; kept {
		t.Fatal("the failure outlived a runtime that is there")
	}
}

// a later store change must not rewrite a snapshot a frame is already drawing from
func TestLocalSnapshotIsNotRewrittenByLaterChanges(t *testing.T) {
	store := New(nil)
	store.ApplySnapshot(protocol.Snapshot{Local: protocol.LocalResult{
		Models:  []protocol.LocalModel{{ID: "lfm3", Files: []protocol.LocalFile{{Name: "a.gguf", Done: 1}}}},
		Runtime: protocol.LocalRuntime{State: protocol.RuntimeStopped, Loaded: []string{"lfm3"}},
	}})
	held := store.Snapshot()
	store.SetLocalRuntime(protocol.LocalRuntime{State: protocol.RuntimeReady, Address: "http://127.0.0.1:41873/v1"})
	if held.Local.Runtime.State != protocol.RuntimeStopped || len(held.Local.Runtime.Loaded) != 1 {
		t.Fatalf("the held snapshot changed: %+v", held.Local.Runtime)
	}
	if got := store.Snapshot().Local.Runtime; got.State != protocol.RuntimeReady || got.Address == "" {
		t.Fatalf("the runtime was not stored: %+v", got)
	}
}

// daemon input may be reused after publication, but a published snapshot itself is read-only
func TestLocalSnapshotCopiesIncomingPlans(t *testing.T) {
	store := New(nil)
	options := []int{8192, 16384}
	store.SetLocal(protocol.LocalResult{
		Models: []protocol.LocalModel{{
			ID: "lfm3", State: protocol.LocalInstalled,
			Plan: protocol.LocalPlan{Context: 16384, ContextOptions: options},
		}},
		Runtime: protocol.LocalRuntime{Plans: map[string]protocol.LocalPlan{"lfm3": {Context: 16384, ContextOptions: options}}},
	})
	options[0] = 1
	got := store.Snapshot().Local
	if plan := got.Models[0].Plan; plan.ContextOptions[0] != 8192 || plan.ContextOptions[1] != 16384 {
		t.Fatalf("the stored model plan was rewritten from outside: %+v", plan)
	}
	if plan := got.Runtime.Plans["lfm3"]; plan.Context != 16384 || plan.ContextOptions[0] != 8192 {
		t.Fatalf("the stored runtime plan was rewritten from outside: %+v", plan)
	}
}

// the builds on this computer are part of a published snapshot, and an answer to start or stop that names
// none does not make the list go away
func TestLocalSnapshotKeepsTheBuildsAcrossAStartAnswer(t *testing.T) {
	store := New(nil)
	capabilities := []string{"router"}
	builds := []protocol.LocalRuntimeBinary{
		{Path: "/usr/bin/llama-server", Source: protocol.RuntimeSystem, Build: 12480, Router: true, Capabilities: capabilities},
		{Path: "/home/nero/llama.cpp/build/bin/llama-server", Source: "~/llama.cpp/build/bin", Build: 12610},
	}
	store.SetLocal(protocol.LocalResult{Runtime: protocol.LocalRuntime{State: protocol.RuntimeStopped, Path: builds[0].Path, Binaries: builds}})
	builds[1].Path, capabilities[0] = "/elsewhere", "changed"
	held := store.Snapshot().Local.Runtime
	if len(held.Binaries) != 2 || held.Binaries[1].Path != "/home/nero/llama.cpp/build/bin/llama-server" || held.Binaries[0].Capabilities[0] != "router" {
		t.Fatalf("the stored builds were rewritten from outside: %+v", held.Binaries)
	}
	store.SetLocalRuntime(protocol.LocalRuntime{State: protocol.RuntimeReady, Path: "/usr/bin/llama-server"})
	if got := store.Snapshot().Local.Runtime; got.State != protocol.RuntimeReady || len(got.Binaries) != 2 {
		t.Fatalf("a start answer without builds dropped the ones known: %+v", got)
	}
	store.SetLocalRuntime(protocol.LocalRuntime{State: protocol.RuntimeStopped, Binaries: []protocol.LocalRuntimeBinary{{Path: "/usr/bin/llama-server"}}})
	if got := store.Snapshot().Local.Runtime.Binaries; len(got) != 1 {
		t.Fatalf("an answer that names its builds was not taken: %+v", got)
	}
	if len(held.Binaries) != 2 {
		t.Fatalf("the held snapshot changed: %+v", held.Binaries)
	}
}
