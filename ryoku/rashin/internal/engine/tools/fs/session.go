package fs

import (
	"crypto/sha256"
	"sync"
)

// The file tools remember a little per session, in the daemon's memory:
// which hashline tags read and edit handed out (so a wrong tag is explained
// as stale, foreign or invented), the named registers of hashline CUTs, and
// the spill counter. Sessions past the newest maxSessions are forgotten;
// a forgotten tag is then reported as "not from this session", which is
// still a correct rejection.
const (
	maxSessions     = 64
	maxTaggedPaths  = 256
	tagsPerPath     = 4
	maxRegisters    = 32
	maxRegisterSize = 1 << 20
)

type sessionState struct {
	mu        sync.Mutex
	tags      map[string][]string            // path -> tags issued, newest last
	byTag     map[string]map[string]bool     // tag -> paths it was issued for
	digests   map[string]map[string][32]byte // path -> tag -> source fingerprint
	collided  map[string]map[string]bool     // a tag was issued for distinct content of one path
	order     []string                       // tagged paths, oldest first
	registers map[string][]string
	spill     *spillState
}

type sessionTable struct {
	mu    sync.Mutex
	byID  map[string]*sessionState
	order []string
}

var sessions = sessionTable{byID: map[string]*sessionState{}}

// state returns the session's memory, making it on first use. An empty id
// (a call outside any session) gets a fresh, unremembered state.
func state(id string) *sessionState {
	if id == "" {
		return newSessionState()
	}
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	if s, ok := sessions.byID[id]; ok {
		for i, v := range sessions.order {
			if v == id {
				sessions.order = append(sessions.order[:i], sessions.order[i+1:]...)
				break
			}
		}
		sessions.order = append(sessions.order, id)
		return s
	}
	s := newSessionState()
	sessions.byID[id] = s
	sessions.order = append(sessions.order, id)
	if len(sessions.order) > maxSessions {
		delete(sessions.byID, sessions.order[0])
		sessions.order = sessions.order[1:]
	}
	return s
}

func newSessionState() *sessionState {
	return &sessionState{
		tags:      map[string][]string{},
		byTag:     map[string]map[string]bool{},
		digests:   map[string]map[string][32]byte{},
		collided:  map[string]map[string]bool{},
		registers: map[string][]string{},
	}
}

// A 16-bit tag can collide, so the session also remembers the exact
// fingerprint behind each anchor. Ambiguous tags fail closed.
func (s *sessionState) issue(path, tag, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	digest := sha256.Sum256([]byte(content))
	list, known := s.tags[path]
	if old, exists := s.digests[path][tag]; exists && old != digest {
		if s.collided[path] == nil {
			s.collided[path] = map[string]bool{}
		}
		s.collided[path][tag] = true
		return
	}
	for _, t := range list {
		if t == tag {
			return
		}
	}
	if !known {
		s.order = append(s.order, path)
		if len(s.order) > maxTaggedPaths {
			s.forget(s.order[0])
			s.order = s.order[1:]
		}
	}
	list = append(list, tag)
	if len(list) > tagsPerPath {
		s.untag(path, list[0])
		list = list[1:]
	}
	s.tags[path] = list
	if s.digests[path] == nil {
		s.digests[path] = map[string][32]byte{}
	}
	s.digests[path][tag] = digest
	if s.byTag[tag] == nil {
		s.byTag[tag] = map[string]bool{}
	}
	s.byTag[tag][path] = true
}

func (s *sessionState) forget(path string) {
	for _, t := range s.tags[path] {
		s.untag(path, t)
	}
	delete(s.tags, path)
	delete(s.digests, path)
	delete(s.collided, path)
}

func (s *sessionState) untag(path, tag string) {
	if d := s.digests[path]; d != nil {
		delete(d, tag)
	}
	if c := s.collided[path]; c != nil {
		delete(c, tag)
	}
	if m := s.byTag[tag]; m != nil {
		delete(m, path)
		if len(m) == 0 {
			delete(s.byTag, tag)
		}
	}
}

// issuedFor reports whether an anchor identifies this exact content, and
// whether its tag has become ambiguous for this path.
func (s *sessionState) issuedFor(path, tag, content string) (valid bool, other []string, collision bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for p := range s.byTag[tag] {
		if p != path {
			other = append(other, p)
		}
	}
	if s.collided[path][tag] {
		return false, other, true
	}
	digest, known := s.digests[path][tag]
	return known && digest == sha256.Sum256([]byte(content)), other, false
}

func (s *sessionState) register(name string) ([]string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.registers[name]
	return r, ok
}

// keep publishes named registers once their edit has landed.
func (s *sessionState) keep(regs map[string][]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for name, rows := range regs {
		size := 0
		for _, r := range rows {
			size += len(r) + 1
		}
		if size > maxRegisterSize {
			continue
		}
		if _, ok := s.registers[name]; !ok && len(s.registers) >= maxRegisters {
			for k := range s.registers {
				delete(s.registers, k)
				break
			}
		}
		s.registers[name] = rows
	}
}
