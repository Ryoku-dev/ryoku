package honcho

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/memory/backend"
	"ryoku-rashin/internal/protocol"
)

// userPeer is the peer every fact is about. Each scope is its own observing
// peer (backend.ScopeID), so a scope's facts are that peer's conclusions
// about the user and nothing in one scope reaches another.
const userPeer = "user"

// Honcho is memory kept on a Honcho server. It learns from what Observe
// hands it, on the server's own clock.
type Honcho struct {
	client *client
	leaves bool
	calls  backend.Calls

	mu    sync.Mutex
	ready map[string]bool // the workspace ("") and peers known to exist
}

// New opens Honcho at base in workspace with key, which may be empty for a
// server that asks for none. Nothing is sent until the first call.
func New(base, workspace, key string) *Honcho {
	return &Honcho{client: newClient(base, key, workspace), leaves: backend.LeavesMachine(base), ready: map[string]bool{}}
}

// Caps: a conclusion cannot be edited in place, so an edit deletes it and
// makes a new one; Honcho keeps no pin or earlier text.
func (h *Honcho) Caps() backend.Caps {
	return backend.Caps{
		MemoryCaps: protocol.MemoryCaps{Edit: true, ForgetSession: true, LeavesMachine: h.leaves},
		Learns:     true,
		Card:       true,
	}
}

// ensure makes the workspace and the peers once per Honcho value.
func (h *Honcho) ensure(ctx context.Context, peers ...string) error {
	for _, id := range append([]string{""}, peers...) {
		h.mu.Lock()
		known := h.ready[id]
		h.mu.Unlock()
		if known {
			continue
		}
		var err error
		if id == "" {
			err = h.client.ensureWorkspace(ctx)
		} else {
			err = h.client.ensurePeer(ctx, id)
		}
		if err != nil {
			return err
		}
		h.mu.Lock()
		h.ready[id] = true
		h.mu.Unlock()
	}
	return nil
}

// fact turns a conclusion into a memory; ok is false for one Rashin did not
// make (another observer on a shared workspace).
func fact(c conclusion) (protocol.Memory, bool) {
	scope, ok := backend.ScopeOf(c.ObserverID)
	if !ok || c.ObservedID != userPeer {
		return protocol.Memory{}, false
	}
	m := protocol.Memory{ID: c.ID, Scope: scope, Text: c.Content, Status: protocol.MemoryActive, Created: c.CreatedAt, Updated: c.CreatedAt}
	if c.SessionID != nil {
		m.Source.Session = strings.TrimPrefix(*c.SessionID, sessionPrefix)
	}
	return m, true
}

func scopeFilter(scope string) map[string]any {
	return map[string]any{"observer": backend.ScopeID(scope), "observed": userPeer}
}

// Remember stores each fact as an explicit conclusion of its scope's peer.
func (h *Honcho) Remember(ctx context.Context, facts []protocol.Memory) ([]protocol.Memory, error) {
	defer h.calls.Begin()()
	out := make([]protocol.Memory, 0, len(facts))
	for _, f := range facts {
		peer := backend.ScopeID(f.Scope)
		if peer == "" {
			return out, errors.New(i18n.Tf("%s is not a memory scope.", f.Scope))
		}
		if err := h.ensure(ctx, userPeer, peer); err != nil {
			return out, err
		}
		made, err := h.client.createConclusions(ctx, []conclusionCreate{{Content: f.Text, ObserverID: peer, ObservedID: userPeer}})
		if err != nil {
			return out, err
		}
		if len(made) != 1 {
			return out, errors.New(i18n.T("Honcho did not keep the memory."))
		}
		kept, _ := fact(made[0])
		kept.Kind, kept.Pinned, kept.Sensitive, kept.Source = f.Kind, f.Pinned, f.Sensitive, f.Source
		out = append(out, kept)
	}
	return out, nil
}

// Edit forgets the conclusion and makes one with the new text in its place.
func (h *Honcho) Edit(ctx context.Context, id, text string) (protocol.Memory, error) {
	defer h.calls.Begin()()
	old, err := h.client.getConclusion(ctx, id)
	if err != nil {
		if errors.Is(err, errNotFound) {
			return protocol.Memory{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That memory is gone."))
		}
		return protocol.Memory{}, err
	}
	if _, ok := fact(old); !ok {
		return protocol.Memory{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That memory is gone."))
	}
	made, err := h.client.createConclusions(ctx, []conclusionCreate{{Content: text, ObserverID: old.ObserverID, ObservedID: old.ObservedID, SessionID: deref(old.SessionID)}})
	if err != nil {
		return protocol.Memory{}, err
	}
	if len(made) != 1 {
		return protocol.Memory{}, errors.New(i18n.T("Honcho did not keep the memory."))
	}
	if err := h.client.deleteConclusion(ctx, id); err != nil {
		return protocol.Memory{}, err
	}
	edited, _ := fact(made[0])
	return edited, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// scopes returns the scopes Rashin has peers for on the server.
func (h *Honcho) scopes(ctx context.Context) ([]string, error) {
	var scopes []string
	for number := 1; ; number++ {
		p, err := h.client.peers(ctx, number)
		if err != nil {
			return nil, err
		}
		for _, peer := range p.Items {
			if scope, ok := backend.ScopeOf(peer.ID); ok {
				scopes = append(scopes, scope)
			}
		}
		if number >= p.Pages || len(p.Items) == 0 {
			return scopes, nil
		}
	}
}

// resolve turns a scope or a group of scopes (every scope when empty) into
// the exact scopes it covers.
func (h *Honcho) resolve(ctx context.Context, group string) ([]string, error) {
	if backend.Exact(group) {
		return []string{group}, nil
	}
	all, err := h.scopes(ctx)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(all, func(scope string) bool { return !backend.InGroup(scope, group) }), nil
}

// Recall searches each scope's conclusions by meaning and takes the best of
// each in turn.
func (h *Honcho) Recall(ctx context.Context, q backend.Query) ([]protocol.Memory, error) {
	defer h.calls.Begin()()
	limit := min(max(q.Limit, 1), pageSize)
	var scopes []string
	for _, group := range q.Scopes {
		resolved, err := h.resolve(ctx, group)
		if err != nil {
			return nil, err
		}
		scopes = append(scopes, resolved...)
	}
	if len(q.Scopes) == 0 {
		var err error
		if scopes, err = h.resolve(ctx, ""); err != nil {
			return nil, err
		}
	}
	var lists [][]protocol.Memory
	for _, scope := range scopes {
		found, err := h.client.queryConclusions(ctx, q.Text, limit, scopeFilter(scope))
		if errors.Is(err, errNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var list []protocol.Memory
		for _, c := range found {
			if m, ok := fact(c); ok {
				list = append(list, m)
			}
		}
		lists = append(lists, list)
	}
	return backend.Interleave(lists, limit), nil
}

// sessionPrefix marks the sessions Rashin makes.
const sessionPrefix = "rashin-"

// sessionID names a Rashin conversation on the server.
func sessionID(session string) string {
	id := sessionPrefix + session
	if protocol.ValidHonchoID(id) {
		return id
	}
	var b strings.Builder
	for _, r := range id {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()[:min(b.Len(), 512)]
}

// Observe puts the turns into the conversation's session, where the scope's
// peer observes the user and Honcho learns from them.
func (h *Honcho) Observe(ctx context.Context, t backend.Transcript) error {
	defer h.calls.Begin()()
	peer := backend.ScopeID(t.Scope)
	if peer == "" {
		return errors.New(i18n.Tf("%s is not a memory scope.", t.Scope))
	}
	if err := h.ensure(ctx, userPeer, peer); err != nil {
		return err
	}
	// the user is the one observed; the scope's peer observes them and is
	// never observed itself
	no, yes := false, true
	session := sessionCreate{
		ID:       sessionID(t.Session),
		Metadata: map[string]any{"kind": t.Kind, "title": t.Title},
		Peers:    map[string]peerConfig{userPeer: {ObserveMe: &yes}, peer: {ObserveMe: &no, ObserveOthers: &yes}},
	}
	if err := h.client.ensureSession(ctx, session); err != nil {
		return err
	}
	sent, err := h.client.messageKeys(ctx, session.ID)
	if err != nil {
		return err
	}
	var messages []messageCreate
	add := func(turn backend.Turn, role, peerID, text string) {
		for index, part := range split(text) {
			key := messageKey(turn, role, index)
			if !sent[key] {
				messages = append(messages, messageCreate{Content: part, PeerID: peerID, CreatedAt: turn.At, Metadata: map[string]any{keyField: key}})
			}
		}
	}
	for _, turn := range t.Turns {
		add(turn, "user", userPeer, strings.TrimSpace(strings.Join(append([]string{turn.User}, turn.Answers...), "\n\n")))
		add(turn, "reply", peer, strings.TrimSpace(turn.Reply))
	}
	for chunk := range slices.Chunk(messages, pageSize) {
		if err := h.client.addMessages(ctx, session.ID, chunk); err != nil {
			return err
		}
	}
	return nil
}

// keyField is the message metadata that names which turn and part a
// message is, so handing the same turns again after a failure sends only
// what the server does not have.
const keyField = "rashin_part"

func messageKey(turn backend.Turn, role string, index int) string {
	at := turn.Seq
	if at == 0 {
		at = turn.At.UnixNano()
	}
	return fmt.Sprintf("%d.%s.%d", at, role, index)
}

// split cuts text into parts the server takes, at a line or space when it
// can.
func split(text string) []string {
	var parts []string
	for text != "" {
		if len(text) <= contentMax {
			return append(parts, text)
		}
		cut := contentMax
		for !utf8Start(text, cut) {
			cut--
		}
		if at := strings.LastIndexAny(text[:cut], "\n "); at > contentMax/2 {
			cut = at + 1
		}
		parts = append(parts, strings.TrimSpace(text[:cut]))
		text = strings.TrimSpace(text[cut:])
	}
	return parts
}

func utf8Start(text string, at int) bool { return at == 0 || text[at]&0xC0 != 0x80 }

// List pages through one scope's conclusions on the server; a group of
// scopes is gathered whole and paged here.
func (h *Honcho) List(ctx context.Context, scope string, offset, limit int) ([]protocol.Memory, int, error) {
	defer h.calls.Begin()()
	if backend.Exact(scope) && (limit <= 0 || offset%limit == 0 && limit <= pageSize) {
		number, size := offset/max(limit, 1)+1, max(limit, 1)
		p, err := h.client.listConclusions(ctx, scopeFilter(scope), number, size)
		if errors.Is(err, errNotFound) {
			return []protocol.Memory{}, 0, nil
		}
		if err != nil {
			return nil, 0, err
		}
		out := []protocol.Memory{}
		for _, c := range p.Items {
			if m, ok := fact(c); ok && limit > 0 {
				out = append(out, m)
			}
		}
		return out, p.Total, nil
	}
	all, err := h.all(ctx, scope)
	if err != nil {
		return nil, 0, err
	}
	return backend.Page(all, offset, limit), len(all), nil
}

// all gathers every conclusion in a scope or group of scopes, newest first.
func (h *Honcho) all(ctx context.Context, scope string) ([]protocol.Memory, error) {
	f := map[string]any{"observed": userPeer}
	if backend.Exact(scope) {
		f = scopeFilter(scope)
	}
	var out []protocol.Memory
	for number := 1; ; number++ {
		p, err := h.client.listConclusions(ctx, f, number, pageSize)
		if errors.Is(err, errNotFound) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		for _, c := range p.Items {
			if m, ok := fact(c); ok && backend.InGroup(m.Scope, scope) {
				out = append(out, m)
			}
		}
		if number >= p.Pages || len(p.Items) == 0 {
			break
		}
	}
	slices.SortStableFunc(out, func(a, b protocol.Memory) int { return b.Created.Compare(a.Created) })
	return out, nil
}

// Forget deletes conclusions; Honcho keeps no copy.
func (h *Honcho) Forget(ctx context.Context, ids []string) error {
	defer h.calls.Begin()()
	for _, id := range ids {
		if err := h.client.deleteConclusion(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// ForgetScope deletes the conclusions and sessions of a scope, of every
// scope in a group, or of every scope when scope is empty.
func (h *Honcho) ForgetScope(ctx context.Context, scope string) error {
	defer h.calls.Begin()()
	scopes, err := h.resolve(ctx, scope)
	if err != nil {
		return err
	}
	for _, scope := range scopes {
		if err := h.forgetScope(ctx, scope); err != nil {
			return err
		}
	}
	return nil
}

func (h *Honcho) forgetScope(ctx context.Context, scope string) error {
	peer := backend.ScopeID(scope)
	var sessions []string
	for number := 1; ; number++ {
		p, err := h.client.peerSessions(ctx, peer, number)
		if errors.Is(err, errNotFound) {
			break
		}
		if err != nil {
			return err
		}
		for _, s := range p.Items {
			sessions = append(sessions, s.ID)
		}
		if number >= p.Pages || len(p.Items) == 0 {
			break
		}
	}
	for _, id := range sessions {
		if err := h.client.deleteSession(ctx, id); err != nil {
			return err
		}
	}
	facts, err := h.all(ctx, scope)
	if err != nil {
		return err
	}
	for _, f := range facts {
		if err := h.client.deleteConclusion(ctx, f.ID); err != nil {
			return err
		}
	}
	return nil
}

// Card is the peer card Honcho wrote about the user from the personal
// scope's side.
func (h *Honcho) Card(ctx context.Context) (string, error) {
	defer h.calls.Begin()()
	lines, err := h.client.card(ctx, backend.ScopeID(protocol.MemoryYou), userPeer)
	if errors.Is(err, errNotFound) {
		return "", nil
	}
	return strings.Join(lines, "\n"), err
}

// Status asks the server whether it answers and how much it still has to
// learn.
func (h *Honcho) Status(ctx context.Context) backend.Status {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := h.client.health(ctx); err != nil {
		return backend.Status{State: protocol.EngineUnreachable, Detail: err.Error()}
	}
	queue, err := h.client.queue(ctx)
	if err != nil && !errors.Is(err, errNotFound) {
		return backend.Status{State: protocol.EngineUnreachable, Detail: err.Error()}
	}
	return backend.Status{State: protocol.EngineReady, Learning: queue.Pending + queue.InProgress}
}

// Busy is true while a call is in flight.
func (h *Honcho) Busy() bool { return h.calls.Busy() }

// Settle has nothing to do: Honcho learns on its own server.
func (h *Honcho) Settle(context.Context) {}

// Close ends idle connections.
func (h *Honcho) Close() error {
	h.client.http.CloseIdleConnections()
	return nil
}
