package memory

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/memory/backend"
	"ryoku-rashin/internal/memory/hindsight"
	"ryoku-rashin/internal/memory/honcho"
	"ryoku-rashin/internal/protocol"
)

// Engines opens the engines that keep memory besides the built-in one:
// Hindsight, run on this computer or reached at an address, and Honcho.
type Engines interface {
	// Open returns the engine s names. The same engine is returned while
	// its address and key stay the same; Engines owns it, so callers never
	// close it.
	Open(ctx context.Context, s protocol.MemorySettings) (backend.Backend, error)
	// Test says whether an engine answers.
	Test(ctx context.Context, s protocol.MemorySettings, engine string) protocol.MemoryEngineTest
	// Setup installs a local Hindsight in the background, saying how far it
	// has come through progress; it returns at once.
	Setup(ctx context.Context, s protocol.MemorySettings, engine string, progress func(protocol.MemorySetupProgress)) error
	// Move copies memories into an engine. It does not switch to it.
	Move(ctx context.Context, s protocol.MemorySettings, engine string, active []protocol.Memory) error
	// Busy is true while an engine works or setup runs.
	Busy() bool
	// Settle is called at the daemon's idle deadline.
	Settle(ctx context.Context)
	// Release stops a local Hindsight at the idle exit; it starts again on
	// the next call.
	Release()
	// Close stops everything at the daemon's exit.
	Close() error
}

// EngineDeps is what the engines need from the daemon. Gateway serves a
// local Hindsight's model calls; Secrets reads engine keys from the vault by
// credential name and keeps the ones Rashin makes; Models and
// ProviderModels find a local model for Hindsight to learn with.
type EngineDeps struct {
	Gateway        hindsight.Gateway
	Secrets        backend.Secrets
	Models         func(ctx context.Context) []protocol.ProviderInfo
	ProviderModels func(ctx context.Context, provider string) []protocol.ModelInfo
	// Layout places a local Hindsight; empty means DefaultLayout.
	Layout hindsight.Layout
}

// moveBatch is how many memories Move hands an engine at once.
const moveBatch = 50

// testWait bounds a test of an engine that is already running or remote; a
// local Hindsight is given its whole start.
const testWait = 10 * time.Second

type engines struct {
	deps  EngineDeps
	local *hindsight.Process

	mu       sync.Mutex
	open     backend.Backend
	openKey  engineKey
	settings protocol.MemorySettings
	setup    bool
}

// engineKey is what makes an engine a different one: its kind, where it is
// and which credential opens it.
type engineKey struct {
	engine, address, key, workspace string
	local                           bool
}

func keyOf(s protocol.MemorySettings, engine string) engineKey {
	switch engine {
	case protocol.MemoryHindsight:
		if s.Hindsight.Local {
			return engineKey{engine: engine, local: true}
		}
		return engineKey{engine: engine, address: s.Hindsight.URL, key: s.Hindsight.Key}
	case protocol.MemoryHoncho:
		return engineKey{engine: engine, address: s.Honcho.URL, key: s.Honcho.Key, workspace: s.Honcho.Workspace}
	}
	return engineKey{engine: engine}
}

// NewEngines makes the engines. Nothing starts and no file is touched until
// one is opened.
func NewEngines(d EngineDeps) Engines {
	if d.Layout == (hindsight.Layout{}) {
		d.Layout = hindsight.DefaultLayout()
	}
	e := &engines{deps: d}
	e.local = hindsight.NewProcess(hindsight.Config{Layout: d.Layout, Secrets: d.Secrets, Gateway: d.Gateway, Model: e.learnModel})
	return e
}

// learnModel is the model a local Hindsight learns with: memory.model's
// policy, the same one the built-in learner follows, applied to the model
// that answered the conversation Hindsight learns from. "" waits.
func (e *engines) learnModel(ctx context.Context, answered string) string {
	e.mu.Lock()
	choice := e.settings.Model
	e.mu.Unlock()
	model, _ := pickModel(ctx, choice, answered, e.deps.Models, e.deps.ProviderModels)
	return model.Model
}

func (e *engines) Open(ctx context.Context, s protocol.MemorySettings) (backend.Backend, error) {
	key := keyOf(s, s.Engine)
	e.mu.Lock()
	e.settings = s
	if e.open != nil && e.openKey == key {
		open := e.open
		e.mu.Unlock()
		return open, nil
	}
	e.mu.Unlock()
	made, err := e.make(s, s.Engine)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	old := e.open
	if e.open != nil && e.openKey == key {
		// another caller made it first
		made, old = e.open, made
	} else {
		e.open, e.openKey = made, key
	}
	e.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return made, nil
}

// make opens an engine from settings. It reads the engine's key from the
// vault; it starts nothing.
func (e *engines) make(s protocol.MemorySettings, engine string) (backend.Backend, error) {
	switch engine {
	case protocol.MemoryHindsight:
		if s.Hindsight.Local {
			return hindsight.Local(e.local), nil
		}
		if s.Hindsight.URL == "" {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Give Hindsight's address first."))
		}
		key, err := e.secret(s.Hindsight.Key)
		if err != nil {
			return nil, err
		}
		return hindsight.Remote(s.Hindsight.URL, key), nil
	case protocol.MemoryHoncho:
		if s.Honcho.URL == "" {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Give Honcho's address first."))
		}
		key, err := e.secret(s.Honcho.Key)
		if err != nil {
			return nil, err
		}
		workspace := s.Honcho.Workspace
		if workspace == "" {
			workspace = config.HonchoWorkspace
		}
		return honcho.New(s.Honcho.URL, workspace, key), nil
	}
	return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%s is not a memory engine Rashin can open.", engine))
}

// secret reads a credential by name; no name means the engine asks for no
// key.
func (e *engines) secret(name string) (string, error) {
	if name == "" {
		return "", nil
	}
	if e.deps.Secrets == nil {
		return "", protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("The vault is not available."))
	}
	return e.deps.Secrets.Secret(name)
}

// borrow returns the open engine when it is the one asked for, or a new one
// with the func that closes it.
func (e *engines) borrow(s protocol.MemorySettings, engine string) (backend.Backend, func(), error) {
	key := keyOf(s, engine)
	e.mu.Lock()
	if e.open != nil && e.openKey == key {
		open := e.open
		e.mu.Unlock()
		return open, func() {}, nil
	}
	e.mu.Unlock()
	made, err := e.make(s, engine)
	if err != nil {
		return nil, nil, err
	}
	return made, func() { _ = made.Close() }, nil
}

func (e *engines) Test(ctx context.Context, s protocol.MemorySettings, engine string) protocol.MemoryEngineTest {
	b, done, err := e.borrow(s, engine)
	if err != nil {
		return protocol.MemoryEngineTest{Detail: err.Error()}
	}
	defer done()
	local := engine == protocol.MemoryHindsight && s.Hindsight.Local
	if !local {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, testWait)
		defer cancel()
	}
	// a stopped local Hindsight answers only once something starts it
	if local {
		if status := b.Status(ctx); status.State == protocol.EngineSetup {
			return protocol.MemoryEngineTest{Detail: status.Detail}
		}
		if _, _, err := b.List(ctx, protocol.MemoryYou, 0, 1); err != nil {
			return protocol.MemoryEngineTest{Detail: err.Error()}
		}
	}
	status := b.Status(ctx)
	if status.State != protocol.EngineReady {
		return protocol.MemoryEngineTest{Detail: status.Detail}
	}
	return protocol.MemoryEngineTest{OK: true}
}

func (e *engines) Setup(_ context.Context, s protocol.MemorySettings, engine string, progress func(protocol.MemorySetupProgress)) error {
	if engine != protocol.MemoryHindsight || !s.Hindsight.Local {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Only Hindsight on this computer is set up by Rashin."))
	}
	if e.deps.Secrets == nil {
		return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("The vault is not available."))
	}
	e.mu.Lock()
	if e.setup {
		e.mu.Unlock()
		return protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Hindsight is already being set up."))
	}
	e.setup = true
	e.mu.Unlock()
	go func() {
		defer func() {
			e.mu.Lock()
			e.setup = false
			e.mu.Unlock()
		}()
		last := protocol.MemorySetupProgress{Engine: engine}
		err := e.local.Setup(context.Background(), func(step string, part, parts int) {
			last = protocol.MemorySetupProgress{Engine: engine, Step: step, Part: part, Parts: parts}
			progress(last)
		})
		last.Done = true
		if err != nil {
			last.Error = err.Error()
		} else {
			last.Step, last.Part = i18n.T("Hindsight is ready."), last.Parts
		}
		progress(last)
	}()
	return nil
}

// Move copies memories into an engine. Sensitive ones stay behind when the
// engine leaves this computer. A move that fails part way forgets what it
// already copied, so moving again makes no duplicates.
func (e *engines) Move(ctx context.Context, s protocol.MemorySettings, engine string, active []protocol.Memory) error {
	b, done, err := e.borrow(s, engine)
	if err != nil {
		return err
	}
	defer done()
	return moveInto(ctx, b, active)
}

func moveInto(ctx context.Context, b backend.Backend, active []protocol.Memory) error {
	if b.Caps().LeavesMachine {
		active = slices.DeleteFunc(slices.Clone(active), func(m protocol.Memory) bool { return m.Sensitive })
	}
	var copied []string
	for batch := range slices.Chunk(active, moveBatch) {
		kept, err := b.Remember(ctx, batch)
		for _, m := range kept {
			copied = append(copied, m.ID)
		}
		if err != nil {
			// a fresh context, since the move's own may be what ended
			undo, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			return errors.Join(err, b.Forget(undo, copied))
		}
	}
	return nil
}

func (e *engines) Busy() bool {
	e.mu.Lock()
	open, setup := e.open, e.setup
	e.mu.Unlock()
	return setup || open != nil && open.Busy()
}

func (e *engines) Settle(ctx context.Context) {
	e.mu.Lock()
	open := e.open
	e.mu.Unlock()
	if open != nil {
		open.Settle(ctx)
	}
}

func (e *engines) Release() {
	e.mu.Lock()
	open := e.open
	e.mu.Unlock()
	if local, ok := open.(*hindsight.Hindsight); ok {
		local.Release()
	}
}

func (e *engines) Close() error {
	e.mu.Lock()
	open := e.open
	e.open = nil
	e.mu.Unlock()
	var err error
	if open != nil {
		err = open.Close()
	}
	// a setup's first start may still be running
	e.local.Stop()
	return err
}
