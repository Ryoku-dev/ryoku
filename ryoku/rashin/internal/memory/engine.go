// Package memory is what Rashin remembers between conversations: facts about
// the user, each project and each team, kept as Markdown in the vault with a
// ledger beside them, learned from quiet conversations and recalled into
// Ask, agents and teams. It is opened lazily and costs nothing while off.
package memory

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/memory/backend"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// Deps are what memory needs from the daemon.
type Deps struct {
	Settings func() protocol.Settings
	// Client is the gateway the learner calls.
	Client llm.Client
	// Models finds the providers that are up and their models;
	// ProviderModels lists one provider's models with their sizes.
	Models         func(ctx context.Context) []protocol.ProviderInfo
	ProviderModels func(ctx context.Context, provider string) []protocol.ModelInfo
	Broadcast      func(event string, data any)
	// Names gives team and project names, and project roots, for scopes.
	Names Names
	// Embed loads the vectors model and returns the release of it.
	Embed func() (Embedder, func(), error)
	// Engines opens Hindsight or Honcho; nil keeps the built-in engine only.
	Engines Engines
	// SetEngine sets Settings.Memory.Engine as an app settings change would.
	SetEngine func(engine string) error
	// Dir, TeamDir and DB place the store; empty means the vault's
	// memory/rashin, the state folder's memory/teams and memory/memory.db.
	Dir, TeamDir, DB string
}

// On reports whether memory is in use: the first-run card was answered and
// Use is on.
func On(s protocol.Settings) bool {
	return s.Memory.Asked && s.Memory.Use
}

// Uses reports whether a conversation draws on memory.
func Uses(s protocol.Settings, info protocol.SessionInfo) bool {
	return On(s) && info.Memory != protocol.MemoryNone
}

// Engine is Rashin's memory, the core.Memory the daemon holds. Open costs
// nothing: the store is opened by the first call that needs it while memory
// is on, or by a memory method.
type Engine struct {
	deps  Deps
	keys  projectKeys
	learn *learner

	mu       sync.Mutex
	st       *Store
	sessions core.MemorySessions
	// modes are the projects' memory modes seen so far, by scope.
	modes map[string]string
	// quiet are the memories set aside in temporary conversations, which
	// never reach the ledger.
	quiet map[string]map[string]bool
	// deleted are conversations deleted while the ledger was closed; what it
	// keeps of them is dropped when it opens.
	deleted []string
}

var _ core.Memory = (*Engine)(nil)

// Open makes the engine; nothing is read or started.
func Open(d Deps) *Engine {
	if d.Dir == "" {
		d.Dir = filepath.Join(paths.VaultDir(), "memory", "rashin")
	}
	if d.TeamDir == "" {
		d.TeamDir = filepath.Join(paths.StateDir(), "memory", "teams")
	}
	if d.DB == "" {
		d.DB = filepath.Join(paths.StateDir(), "memory", "memory.db")
	}
	e := &Engine{deps: d, modes: map[string]string{}, quiet: map[string]map[string]bool{}}
	e.learn = newLearner(e)
	return e
}

// Bind hands memory the session manager.
func (e *Engine) Bind(sessions core.MemorySessions) {
	e.mu.Lock()
	e.sessions = sessions
	e.mu.Unlock()
}

// sessionsOf is the bound session manager, nil before Bind.
func (e *Engine) sessionsOf() core.MemorySessions {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sessions
}

// settings are memory's settings now.
func (e *Engine) settings() protocol.MemorySettings {
	if e.deps.Settings == nil {
		return protocol.MemorySettings{}
	}
	return e.deps.Settings().Memory
}

// on reports whether memory is in use now.
func (e *Engine) on() bool {
	if e.deps.Settings == nil {
		return false
	}
	return On(e.deps.Settings())
}

// store opens the built-in store on first use and reads back any memory file
// edited by hand since.
func (e *Engine) store() (*Store, error) {
	e.mu.Lock()
	st := e.st
	var deleted []string
	if st == nil {
		var err error
		st, err = OpenStore(StoreConfig{Dir: e.deps.Dir, TeamDir: e.deps.TeamDir, DB: e.deps.DB, Names: e.deps.Names, Embed: e.deps.Embed})
		if err != nil {
			e.mu.Unlock()
			return nil, err
		}
		e.st = st
		deleted, e.deleted = e.deleted, nil
	}
	e.mu.Unlock()
	for _, id := range deleted {
		_ = st.SessionDeleted(id)
	}
	err := st.Sync()
	for _, ch := range st.takeFileChanges() {
		e.touchCard(st, ch, false)
		// a hand edit is found by whichever call opened the store, so it is announced apart from it
		go e.announce(ch)
	}
	return st, err
}

// touchCard rebuilds the card at once, without a model, when a change took
// away or changed a fact about the user, so the card never keeps what was
// forgotten. Undo and redo (flipped) always count.
func (e *Engine) touchCard(st *Store, ch protocol.MemoryChange, flipped bool) {
	if !cardTouched(ch, flipped) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = e.refreshCard(ctx, st)
}

// cardTouched reports a change that may leave the card saying something no
// longer remembered.
func cardTouched(ch protocol.MemoryChange, flipped bool) bool {
	for _, item := range ch.Items {
		if item.Scope != protocol.MemoryYou {
			continue
		}
		switch {
		case flipped:
			return true
		case item.Op == protocol.MemoryOpForgot || item.Op == protocol.MemoryOpEdited || item.Op == protocol.MemoryOpRestored || item.Op == protocol.MemoryOpReplaced:
			return true
		case item.Op == protocol.MemoryOpRemembered && item.Replaces != "":
			return true
		}
	}
	return false
}

// storeIfAny is the store when the ledger already exists; it never creates
// anything.
func (e *Engine) storeIfAny() (*Store, bool) {
	e.mu.Lock()
	open := e.st != nil
	e.mu.Unlock()
	if !open {
		if _, err := os.Stat(e.deps.DB); err != nil {
			return nil, false
		}
	}
	st, err := e.store()
	return st, err == nil
}

// external is the engine in use when it is not the built-in one, nil
// otherwise. Forgets put off while the engine was stopped run first.
func (e *Engine) external(ctx context.Context) (backend.Backend, error) {
	b, err := e.openExternal(ctx)
	if err != nil || b == nil {
		return b, err
	}
	if st, ok := e.storeIfAny(); ok {
		for _, scope := range st.pendingForgets() {
			if err := b.ForgetScope(ctx, scope); err != nil {
				return nil, err
			}
			if err := st.clearPendingForget(scope); err != nil {
				return nil, err
			}
		}
	}
	return b, nil
}

// openExternal is the external engine without anything run in it: opening
// starts nothing, so Status and forget_all can look at its state.
func (e *Engine) openExternal(ctx context.Context) (backend.Backend, error) {
	s := e.settings()
	if s.Engine == "" || s.Engine == protocol.MemoryBuiltin || e.deps.Engines == nil {
		return nil, nil
	}
	return e.deps.Engines.Open(ctx, s)
}

// changed rebuilds the card when the change calls for it, then announces
// the change.
func (e *Engine) changed(ch protocol.MemoryChange) {
	if len(ch.Items) == 0 {
		return
	}
	if st, ok := e.storeIfAny(); ok {
		e.touchCard(st, ch, false)
	}
	e.announce(ch)
}

// announce tells of a change: into the conversation it was made in, then to
// every client with the status after it.
func (e *Engine) announce(ch protocol.MemoryChange) {
	if len(ch.Items) == 0 {
		return
	}
	if ch.Session != "" {
		if sessions := e.sessionsOf(); sessions != nil {
			change := ch
			sessions.EmitMemory(ch.Session, protocol.MemoryEventData{Kind: protocol.MemoryUpdated, Change: &change})
		}
	}
	if e.deps.Broadcast != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		change := ch
		e.deps.Broadcast(protocol.EvtMemoryChanged, protocol.MemoryChanged{Status: e.Status(ctx), Change: &change})
	}
}

// emit puts a memory line into a conversation's transcript.
func (e *Engine) emit(session string, data protocol.MemoryEventData) {
	if session == "" {
		return
	}
	if sessions := e.sessionsOf(); sessions != nil {
		sessions.EmitMemory(session, data)
	}
}

// Recall is memory for one Ask turn; zero when the session or the settings
// use none.
func (e *Engine) Recall(ctx context.Context, info protocol.SessionInfo, question string) core.Recall {
	if e.deps.Settings == nil || !Uses(e.deps.Settings(), info) {
		return core.Recall{}
	}
	return e.recall(ctx, info, question)
}

// Brief is the memory part of an agent session's brief.
func (e *Engine) Brief(info protocol.SessionInfo) string {
	if e.deps.Settings == nil || !Uses(e.deps.Settings(), info) || !e.settings().Agents {
		return ""
	}
	return e.brief(info)
}

// TeamBrief is what a team's members are told the team remembers.
func (e *Engine) TeamBrief(team protocol.Team) string {
	if !e.on() || !team.TeamMemory().Remember {
		return ""
	}
	return e.teamBrief(team)
}

// TurnEnded lets memory learn from a session once it goes quiet.
func (e *Engine) TurnEnded(info protocol.SessionInfo) {
	if !e.on() || !e.settings().Learn {
		return
	}
	e.learn.turnEnded(info)
}

// SessionDeleted drops what memory kept of a deleted session. While memory
// is off the ledger stays closed, and the session is dropped from it when it
// next opens.
func (e *Engine) SessionDeleted(id string) {
	e.learn.sessionDeleted(id)
	e.mu.Lock()
	delete(e.quiet, id)
	open := e.st != nil
	if !open && !e.on() {
		e.deleted = append(e.deleted, id)
	}
	e.mu.Unlock()
	if !open && !e.on() {
		return
	}
	if st, ok := e.storeIfAny(); ok {
		_ = st.SessionDeleted(id)
	}
}

// callerInfo is the conversation a call comes from, as the session manager
// knows it; only its id when it is not known.
func (e *Engine) callerInfo(session string) protocol.SessionInfo {
	if sessions := e.sessionsOf(); sessions != nil && session != "" {
		if info, ok := sessions.Info(session); ok {
			return info
		}
	}
	return protocol.SessionInfo{ID: session}
}

// used counts that an answer drew on ids. A temporary conversation leaves
// no trace in the ledger.
func (e *Engine) used(info protocol.SessionInfo, ids []string) {
	if info.Temporary || info.ID == "" || len(ids) == 0 {
		return
	}
	if st, err := e.store(); err == nil {
		_ = st.Used(info.ID, ids)
	}
}

// suppressed are the memories a conversation set aside: kept in the ledger,
// or in the engine alone for a temporary conversation.
func (e *Engine) suppressed(info protocol.SessionInfo) map[string]bool {
	out := map[string]bool{}
	if info.ID == "" {
		return out
	}
	e.mu.Lock()
	for id := range e.quiet[info.ID] {
		out[id] = true
	}
	e.mu.Unlock()
	if info.Temporary {
		return out
	}
	if st, ok := e.storeIfAny(); ok {
		for id := range st.Suppressed(info.ID) {
			out[id] = true
		}
	}
	return out
}

// suppress sets a memory aside in one conversation.
func (e *Engine) suppress(info protocol.SessionInfo, id string) error {
	if !info.Temporary {
		st, err := e.store()
		if err != nil {
			return err
		}
		return st.Suppress(info.ID, id)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.quiet[info.ID] == nil {
		e.quiet[info.ID] = map[string]bool{}
	}
	e.quiet[info.ID][id] = true
	return nil
}

// Settle learns from every conversation still waiting, ahead of the idle
// exit, and lets an external engine finish its work.
func (e *Engine) Settle() {
	e.learn.settle()
	if e.deps.Engines != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		e.deps.Engines.Settle(ctx)
	}
}

// Busy reports learning or an external engine's work in progress.
func (e *Engine) Busy() bool {
	return e.learn.busy() || e.deps.Engines != nil && e.deps.Engines.Busy()
}

// Release drops the vectors model, the learner's caches and a running local
// engine.
func (e *Engine) Release() {
	e.learn.release()
	e.mu.Lock()
	st := e.st
	e.mu.Unlock()
	if st != nil {
		st.Release()
	}
	if e.deps.Engines != nil {
		e.deps.Engines.Release()
	}
}

// Close stops learning and closes the store and any external engine.
func (e *Engine) Close() error {
	e.learn.close()
	e.mu.Lock()
	st := e.st
	e.st = nil
	e.mu.Unlock()
	var err error
	if st != nil {
		err = st.Close()
	}
	if e.deps.Engines != nil {
		if closeErr := e.deps.Engines.Close(); err == nil {
			err = closeErr
		}
	}
	return err
}
