// Package decisions is project decision memory (protocol.FeatureDecisions):
// after every prompt a light model looks for a decision the user had to
// fight for (a correction, a regression, a rule a newcomer would get wrong),
// quoted from the prompt itself; what it finds waits in a queue, and after
// five quiet minutes another light call folds the queue into three notes per
// project, architecture.md, product.md and style.md, which join the system
// prompt of every later session in that project.
//
// Banks live under paths.DecisionsDir(), one folder per project named by its
// basename and a FNV-1a hash of its path. Nothing runs while the feature is
// off: no timer, no model call, no disk access beyond what the Learning page
// asks to list or delete.
package decisions

// Derived from oh-my-pi (MIT), packages/coding-agent/src/sharpshooter/*.ts@c7fff67

import (
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// Deps is what the service needs from the daemon.
type Deps struct {
	// Client is the gateway. When it also reports context windows
	// (core.Gateway does), consolidation sizes its input to the light
	// model's window.
	Client llm.Client
	// Settings is read on every call; Features[protocol.FeatureDecisions]
	// switches everything.
	Settings func() protocol.Settings
	// Broadcast tells the app a bank changed (learning.changed); nil is fine.
	Broadcast core.Broadcaster
}

// Service is the decisions store, the engine's decisions ContextProvider
// and its decision-extracting TurnObserver in one.
type Service struct {
	deps Deps

	// Fixed by New; tests replace them before the first call.
	root  func() string
	now   func() time.Time
	quiet time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu         sync.Mutex
	closed     bool
	active     int
	banks      map[string]*bankState
	extracting map[string]bool
	recent     map[string]string
	order      []string
}

// bankState is one bank's in-process bookkeeping. mu serializes every write
// to the bank's files; timer is the pending consolidation, nil when none.
type bankState struct {
	mu      sync.Mutex
	timer   *time.Timer
	checked bool
}

var (
	_ engine.ContextProvider = (*Service)(nil)
	_ engine.TurnObserver    = (*Service)(nil)
	_ core.Decisions         = (*Service)(nil)
)

// Quiet is how long a bank's queue rests after its last new decision before
// it is consolidated.
const Quiet = 5 * time.Minute

// recentSessions bounds how many sessions' last reply the service keeps for
// resolving short answers ("option 2", "go for it").
const recentSessions = 64

// New builds the service. It starts nothing: timers exist only while a
// bank has queued decisions and the feature is on.
func New(deps Deps) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{
		deps:       deps,
		root:       paths.DecisionsDir,
		now:        time.Now,
		quiet:      Quiet,
		ctx:        ctx,
		cancel:     cancel,
		banks:      map[string]*bankState{},
		extracting: map[string]bool{},
		recent:     map[string]string{},
	}
}

// Busy reports an extraction or consolidation at work.
func (s *Service) Busy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active > 0
}

// Close stops pending consolidations, cancels model calls in flight and
// waits for them. Queued decisions stay on disk for the next run.
func (s *Service) Close() error {
	s.mu.Lock()
	s.closed = true
	for _, bank := range s.banks {
		if bank.timer != nil {
			bank.timer.Stop()
			bank.timer = nil
		}
	}
	s.mu.Unlock()
	s.cancel()
	s.wg.Wait()
	return nil
}

func (s *Service) enabled() bool {
	if s.deps.Settings == nil {
		return false
	}
	return s.deps.Settings().Features[protocol.FeatureDecisions]
}

// begin registers background work; false once the service is closed, so
// nothing starts after Close began waiting.
func (s *Service) begin() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	s.active++
	s.wg.Add(1)
	return true
}

func (s *Service) end() {
	s.mu.Lock()
	s.active--
	s.mu.Unlock()
	s.wg.Done()
}

func (s *Service) bank(id string) *bankState {
	s.mu.Lock()
	defer s.mu.Unlock()
	bank := s.banks[id]
	if bank == nil {
		bank = &bankState{}
		s.banks[id] = bank
	}
	return bank
}

func (s *Service) broadcast(project string) {
	if s.deps.Broadcast == nil {
		return
	}
	s.deps.Broadcast.Broadcast(protocol.EvtLearningChanged, protocol.LearningChanged{Kind: protocol.LearningDecisions, Project: project})
}

// learns reports whether a mode's prompts come from the user working on a
// project. Ask is system chat, and team, dream and subagent prompts are
// written by another model, so none of them teach decisions.
func learns(mode tool.Mode) bool {
	return mode == tool.ModeAgent || mode == tool.ModePlan
}

// informs reports whether a mode's prompt carries the decisions: every mode
// that works on a project.
func informs(mode tool.Mode) bool {
	return mode != tool.ModeAskRead && mode != tool.ModeAskAct
}

// projectRoot is the folder a turn's bank belongs to: the Git top level, or
// the working folder outside Git. The home folder and the file system root
// are not projects.
func projectRoot(tc engine.TurnContext) string {
	root := tc.Project
	if root == "" {
		root = tc.Cwd
	}
	return cleanProject(root)
}

func cleanProject(root string) string {
	root = strings.TrimSpace(root)
	if root == "" || !filepath.IsAbs(root) {
		return ""
	}
	root = filepath.Clean(root)
	if root == string(filepath.Separator) {
		return ""
	}
	if home, err := os.UserHomeDir(); err == nil && filepath.Clean(home) == root {
		return ""
	}
	return root
}

// BankID names a project's bank: its basename made safe for a folder name
// and the FNV-1a 64 hash of its cleaned absolute path, so two projects with
// the same name stay apart and the id never changes between runs.
func BankID(project string) string {
	project = filepath.Clean(project)
	hash := fnv.New64a()
	hash.Write([]byte(project))
	return fmt.Sprintf("%s-%016x", safeName(filepath.Base(project)), hash.Sum64())
}

func safeName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
		if b.Len() >= 40 {
			break
		}
	}
	out := strings.Trim(b.String(), ".-")
	if out == "" {
		return "project"
	}
	return out
}

func (s *Service) bankDir(id string) string { return filepath.Join(s.root(), id) }
