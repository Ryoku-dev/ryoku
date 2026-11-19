package engine

// Derived from oh-my-pi (MIT), packages/agent/src/agent.ts@c7fff67

import (
	"strings"
	"sync"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/llm"
)

// steering holds what the user said while a turn ran. The turn takes it at
// its next round boundary, after the round's tool results, so the model
// reads it before its next step instead of after the whole turn.
type steering struct {
	mu      sync.Mutex
	running bool
	queue   []llm.Message
}

// push queues a message for the running turn; false when no turn runs, and
// the caller sends it as a prompt of its own.
func (s *steering) push(m llm.Message) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return false
	}
	s.queue = append(s.queue, m)
	return true
}

// drain takes every queued message.
func (s *steering) drain() []llm.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.queue
	s.queue = nil
	return out
}

// start marks a turn running. A previous turn's queued messages are persisted
// by abandon before another turn is allowed to start.
func (s *steering) start() {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()
}

// finish is the turn's last look: with nothing queued it marks the turn
// ended in the same step, so no message slips in between, and returns nil;
// otherwise it hands the queue back and the turn goes on.
func (s *steering) finish() []llm.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.queue) == 0 {
		s.running = false
		return nil
	}
	out := s.queue
	s.queue = nil
	return out
}

// abandon ends the turn and returns messages not yet given to the model.
func (s *steering) abandon() []llm.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	out := s.queue
	s.queue = nil
	return out
}

// Steer hands the running turn a message the user sent while it worked: it
// joins the conversation at the next round boundary. It reports false when
// no turn runs, or the turn ended before it could read it; the caller then
// prompts with it as usual. The session manager records the user event.
func (r *runner) Steer(in core.PromptInput) bool {
	text := strings.TrimSpace(in.Text)
	if text == "" && len(in.Images) == 0 {
		return false
	}
	msg := llm.Message{Role: llm.RoleUser, Content: text}
	if hooks := r.f.deps.Ask; hooks != nil {
		if m, err := hooks.Message(text, in.Images); err == nil {
			msg = m
		}
	}
	return r.steer.push(msg)
}
