package state

import (
	"encoding/json"

	"ryoku-rashin/internal/protocol"
)

// localState is the local models part of the store: Rashin's own models with
// the llama.cpp runtime that serves them, and the latest local.progress of
// every download still on its way, by model id or protocol.LocalRuntimeID.
type localState struct {
	result   protocol.LocalResult
	progress map[string]protocol.LocalProgress
}

func newLocalState() localState {
	return localState{progress: make(map[string]protocol.LocalProgress)}
}

// applySnapshotLocked takes the daemon's view whole. Progress starts empty:
// a download that still runs says where it is with its next tick.
func (l *localState) applySnapshotLocked(snapshot protocol.Snapshot) {
	l.result = cloneLocal(snapshot.Local)
	l.progress = make(map[string]protocol.LocalProgress)
}

// applyLocked applies local.progress and local.changed; handled is false for
// any other event.
func (l *localState) applyLocked(event protocol.Event) (handled bool, err error) {
	switch event.Event {
	case protocol.EvtLocalProgress:
		var progress protocol.LocalProgress
		if err = json.Unmarshal(event.Data, &progress); err != nil {
			return true, err
		}
		if progress.Phase == protocol.LocalPhaseDone || progress.Phase == protocol.LocalPhaseCancelled {
			delete(l.progress, progress.ID)
		} else {
			l.progress[progress.ID] = progress
		}
		return true, nil
	case protocol.EvtLocalChanged:
		var result protocol.LocalResult
		if err = json.Unmarshal(event.Data, &result); err != nil {
			return true, err
		}
		l.setLocked(result)
		return true, nil
	}
	return false, nil
}

// setLocked stores a result and drops the progress it has overtaken: a
// model's once the model is no longer downloading, the runtime's once its
// download has failed or the runtime is there.
func (l *localState) setLocked(result protocol.LocalResult) {
	l.result = cloneLocal(result)
	downloading := make(map[string]bool, len(result.Models))
	for _, model := range result.Models {
		downloading[model.ID] = model.State == protocol.LocalDownloading
	}
	for id, progress := range l.progress {
		running := progress.Phase == protocol.LocalPhaseDownload || progress.Phase == protocol.LocalPhaseVerify
		if id == protocol.LocalRuntimeID {
			if !running && result.Runtime.State != protocol.RuntimeMissing && result.Runtime.State != protocol.RuntimeTooOld {
				delete(l.progress, id)
			}
			continue
		}
		if !downloading[id] {
			delete(l.progress, id)
		}
	}
}

func (l *localState) snapshotLocked(into *Snapshot) {
	into.Local = cloneLocal(l.result)
	into.LocalProgress = make(map[string]protocol.LocalProgress, len(l.progress))
	for id, progress := range l.progress {
		into.LocalProgress[id] = progress
	}
}

// SetLocal stores the local models as local.models or local.remove answered
// them, ahead of the local.changed that follows.
func (s *Store) SetLocal(result protocol.LocalResult) {
	s.mu.Lock()
	s.localState.setLocked(result)
	invalidate := s.changedLocked()
	s.mu.Unlock()
	if invalidate != nil {
		invalidate()
	}
}

// SetLocalRuntime stores the runtime as local.start or local.stop answered
// it.
func (s *Store) SetLocalRuntime(runtime protocol.LocalRuntime) {
	s.mu.Lock()
	s.localState.result.Runtime = cloneLocalRuntime(runtime)
	invalidate := s.changedLocked()
	s.mu.Unlock()
	if invalidate != nil {
		invalidate()
	}
}

func cloneLocal(result protocol.LocalResult) protocol.LocalResult {
	models := append([]protocol.LocalModel(nil), result.Models...)
	for index := range models {
		files := append([]protocol.LocalFile(nil), result.Models[index].Files...)
		for at := range files {
			files[at].Plan = cloneLocalPlan(files[at].Plan)
		}
		models[index].Files = files
		models[index].Plan = cloneLocalPlan(models[index].Plan)
	}
	result.Models = models
	result.Runtime = cloneLocalRuntime(result.Runtime)
	return result
}

func cloneLocalRuntime(runtime protocol.LocalRuntime) protocol.LocalRuntime {
	runtime.Devices = append([]protocol.LocalDevice(nil), runtime.Devices...)
	runtime.Loaded = append([]string(nil), runtime.Loaded...)
	runtime.Offer = append([]string(nil), runtime.Offer...)
	if runtime.Plans != nil {
		plans := make(map[string]protocol.LocalPlan, len(runtime.Plans))
		for id, plan := range runtime.Plans {
			plans[id] = cloneLocalPlan(plan)
		}
		runtime.Plans = plans
	}
	return runtime
}

// the context choices are the one part of a plan that is shared memory
func cloneLocalPlan(plan protocol.LocalPlan) protocol.LocalPlan {
	plan.ContextOptions = append([]int(nil), plan.ContextOptions...)
	return plan
}
