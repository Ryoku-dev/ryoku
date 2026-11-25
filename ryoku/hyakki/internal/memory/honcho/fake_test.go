package honcho

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/memory/backend"
	"ryoku-rashin/internal/memory/backendtest"
	"ryoku-rashin/internal/protocol"
)

// fake is a Honcho server kept in memory: workspaces, peers, sessions,
// messages, conclusions and cards, with the API's status codes and pages. It
// learns each sentence a user says in a session as a conclusion of every
// peer there that observes others, and refuses what the real server refuses:
// a missing key, a bad id, a message over 25,000 characters and a semantic
// query without both observer and observed.
type fake struct {
	t      *testing.T
	server *httptest.Server
	key    string
	hold   chan struct{} // when set, every request waits for it
	// failPost, when set, is the message post (counted from 1) that fails
	failPost int
	posts    int

	mu              sync.Mutex
	workspaces      map[string]bool
	peers           map[string][]string // workspace: peer ids in order
	sessions        map[string]map[string]peerConfig
	conclusions     []conclusion
	messages        []messageCreate
	sessionMessages map[string][]messageCreate
	next            int
}

func newFake(t *testing.T, key string) *fake {
	f := &fake{t: t, key: key, workspaces: map[string]bool{}, peers: map[string][]string{}, sessions: map[string]map[string]peerConfig{}, sessionMessages: map[string][]messageCreate{}}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fake) id() string {
	f.next++
	return fmt.Sprintf("c%06d", f.next)
}

func write(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}

func invalid(w http.ResponseWriter, msg string) {
	write(w, http.StatusUnprocessableEntity, map[string]any{"detail": []map[string]any{{"loc": []any{"body"}, "msg": msg, "type": "value_error"}}})
}

func (f *fake) serve(w http.ResponseWriter, r *http.Request) {
	if f.hold != nil {
		<-f.hold
	}
	if r.URL.Path == "/health" {
		write(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if r.Header.Get("Authorization") != "Bearer "+f.key {
		write(w, http.StatusUnauthorized, map[string]string{"detail": "bad key"})
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v3/workspaces"), "/")
	var body map[string]json.RawMessage
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	if len(parts) == 1 && r.Method == http.MethodPost {
		var id string
		_ = json.Unmarshal(body["id"], &id)
		if !protocol.ValidHonchoID(id) {
			invalid(w, "bad workspace id")
			return
		}
		f.workspaces[id] = true
		write(w, http.StatusOK, map[string]any{"id": id})
		return
	}
	if len(parts) < 3 || !f.workspaces[parts[1]] {
		write(w, http.StatusNotFound, map[string]string{"detail": "no workspace"})
		return
	}
	workspace, rest := parts[1], parts[2:]
	route := r.Method + " " + strings.Join(rest, "/")
	switch {
	case route == "POST peers":
		var id string
		_ = json.Unmarshal(body["id"], &id)
		if !protocol.ValidHonchoID(id) {
			invalid(w, "bad peer id")
			return
		}
		if !slices.Contains(f.peers[workspace], id) {
			f.peers[workspace] = append(f.peers[workspace], id)
		}
		write(w, http.StatusOK, map[string]any{"id": id})
	case route == "POST peers/list":
		items := []map[string]string{}
		for _, id := range f.peers[workspace] {
			items = append(items, map[string]string{"id": id})
		}
		f.page(w, r, items)
	case len(rest) == 3 && rest[0] == "peers" && rest[2] == "sessions" && r.Method == http.MethodPost:
		items := []map[string]string{}
		for id, peers := range f.sessions {
			if _, ok := peers[rest[1]]; ok {
				items = append(items, map[string]string{"id": id})
			}
		}
		slices.SortFunc(items, func(a, b map[string]string) int { return strings.Compare(a["id"], b["id"]) })
		f.page(w, r, items)
	case len(rest) == 3 && rest[0] == "peers" && rest[2] == "card" && r.Method == http.MethodGet:
		lines := []string{}
		for _, c := range f.conclusions {
			if c.ObserverID == rest[1] && c.ObservedID == r.URL.Query().Get("target") {
				lines = append(lines, c.Content)
			}
		}
		write(w, http.StatusOK, map[string]any{"peer_card": lines})
	case route == "POST sessions":
		var s sessionCreate
		raw, _ := json.Marshal(body)
		_ = json.Unmarshal(raw, &s)
		if !protocol.ValidHonchoID(s.ID) {
			invalid(w, "bad session id")
			return
		}
		for peer := range s.Peers {
			if !slices.Contains(f.peers[workspace], peer) {
				invalid(w, "no peer "+peer)
				return
			}
		}
		if f.sessions[s.ID] == nil {
			f.sessions[s.ID] = map[string]peerConfig{}
		}
		for peer, config := range s.Peers {
			observed := config.ObserveMe != nil && *config.ObserveMe
			if peer == userPeer && !observed || peer != userPeer && (config.ObserveMe == nil || observed) {
				f.t.Errorf("session %s: peer %s has observe_me %v; only the user may be observed", s.ID, peer, config.ObserveMe)
			}
			f.sessions[s.ID][peer] = config
		}
		write(w, http.StatusOK, map[string]any{"id": s.ID, "is_active": true})
	case len(rest) == 3 && rest[0] == "sessions" && rest[2] == "messages" && r.Method == http.MethodPost:
		f.addMessages(w, rest[1], body)
	case len(rest) == 4 && rest[0] == "sessions" && rest[2] == "messages" && rest[3] == "list" && r.Method == http.MethodPost:
		if f.sessions[rest[1]] == nil {
			write(w, http.StatusNotFound, map[string]string{"detail": "no session"})
			return
		}
		f.page(w, r, f.sessionMessages[rest[1]])
	case len(rest) == 2 && rest[0] == "sessions" && r.Method == http.MethodDelete:
		if f.sessions[rest[1]] == nil {
			write(w, http.StatusNotFound, map[string]string{"detail": "no session"})
			return
		}
		delete(f.sessions, rest[1])
		delete(f.sessionMessages, rest[1])
		f.conclusions = slices.DeleteFunc(f.conclusions, func(c conclusion) bool { return c.SessionID != nil && *c.SessionID == rest[1] })
		write(w, http.StatusAccepted, map[string]any{})
	case route == "POST conclusions":
		var in struct {
			Conclusions []conclusionCreate `json:"conclusions"`
		}
		raw, _ := json.Marshal(body)
		_ = json.Unmarshal(raw, &in)
		out := []conclusion{}
		for _, c := range in.Conclusions {
			if c.Content == "" || c.ObserverID == "" || c.ObservedID == "" {
				invalid(w, "a conclusion needs content, observer_id and observed_id")
				return
			}
			made := conclusion{ID: f.id(), Content: c.Content, ObserverID: c.ObserverID, ObservedID: c.ObservedID, Level: "explicit", CreatedAt: time.Now().UTC()}
			if c.SessionID != "" {
				made.SessionID = &c.SessionID
			}
			f.conclusions = append(f.conclusions, made)
			out = append(out, made)
		}
		write(w, http.StatusCreated, out)
	case route == "POST conclusions/list":
		filters := decodeFilters(body)
		items := []conclusion{}
		for _, c := range f.conclusions {
			if matches(c, filters) {
				items = append(items, c)
			}
		}
		f.page(w, r, items)
	case route == "POST conclusions/query":
		f.query(w, body)
	case len(rest) == 2 && rest[0] == "conclusions" && r.Method == http.MethodGet:
		at := slices.IndexFunc(f.conclusions, func(c conclusion) bool { return c.ID == rest[1] })
		if at < 0 {
			write(w, http.StatusNotFound, map[string]string{"detail": "no conclusion"})
			return
		}
		write(w, http.StatusOK, f.conclusions[at])
	case len(rest) == 2 && rest[0] == "conclusions" && r.Method == http.MethodDelete:
		at := slices.IndexFunc(f.conclusions, func(c conclusion) bool { return c.ID == rest[1] })
		if at < 0 {
			write(w, http.StatusNotFound, map[string]string{"detail": "no conclusion"})
			return
		}
		f.conclusions = slices.Delete(f.conclusions, at, at+1)
		w.WriteHeader(http.StatusNoContent)
	case route == "GET queue/status":
		write(w, http.StatusOK, queueStatus{})
	default:
		f.t.Errorf("the fake has no %s %s", r.Method, r.URL.Path)
		write(w, http.StatusNotFound, map[string]string{"detail": "no route"})
	}
}

// addMessages stores messages and learns each sentence the user says.
func (f *fake) addMessages(w http.ResponseWriter, session string, body map[string]json.RawMessage) {
	peers := f.sessions[session]
	if peers == nil {
		write(w, http.StatusNotFound, map[string]string{"detail": "no session"})
		return
	}
	var messages []messageCreate
	_ = json.Unmarshal(body["messages"], &messages)
	for _, m := range messages {
		if len(m.Content) > contentMax {
			invalid(w, "content is longer than 25000 characters")
			return
		}
		if _, ok := peers[m.PeerID]; !ok {
			invalid(w, "the peer is not in the session")
			return
		}
	}
	f.posts++
	if f.posts == f.failPost {
		write(w, http.StatusServiceUnavailable, map[string]string{"detail": "try again"})
		return
	}
	f.messages = append(f.messages, messages...)
	f.sessionMessages[session] = append(f.sessionMessages[session], messages...)
	// Honcho learns about a peer only when the session observes it
	if user := peers[userPeer]; user.ObserveMe == nil || !*user.ObserveMe {
		write(w, http.StatusCreated, messages)
		return
	}
	for _, m := range messages {
		if m.PeerID != userPeer {
			continue
		}
		for observer, config := range peers {
			if config.ObserveOthers == nil || !*config.ObserveOthers {
				continue
			}
			for _, sentence := range strings.SplitAfter(m.Content, ".") {
				if sentence = strings.TrimSpace(sentence); sentence != "" {
					id := session
					f.conclusions = append(f.conclusions, conclusion{ID: f.id(), Content: sentence, ObserverID: observer, ObservedID: userPeer, SessionID: &id, Level: "deductive", CreatedAt: time.Now().UTC()})
				}
			}
		}
	}
	write(w, http.StatusCreated, messages)
}

func decodeFilters(body map[string]json.RawMessage) map[string]string {
	var filters map[string]string
	_ = json.Unmarshal(body["filters"], &filters)
	return filters
}

func matches(c conclusion, filters map[string]string) bool {
	return (filters["observer"] == "" || filters["observer"] == c.ObserverID) && (filters["observed"] == "" || filters["observed"] == c.ObservedID)
}

// query ranks conclusions by the words they share with the query.
func (f *fake) query(w http.ResponseWriter, body map[string]json.RawMessage) {
	filters := decodeFilters(body)
	if filters["observer"] == "" || filters["observed"] == "" {
		invalid(w, "filters need observer and observed")
		return
	}
	var text string
	var topK int
	_ = json.Unmarshal(body["query"], &text)
	_ = json.Unmarshal(body["top_k"], &topK)
	type hit struct {
		c     conclusion
		score int
	}
	var hits []hit
	for _, c := range f.conclusions {
		if score := shared(text, c.Content); matches(c, filters) && score > 0 {
			hits = append(hits, hit{c, score})
		}
	}
	slices.SortStableFunc(hits, func(a, b hit) int { return b.score - a.score })
	out := []conclusion{}
	for _, h := range hits[:min(len(hits), topK)] {
		out = append(out, h.c)
	}
	write(w, http.StatusOK, out)
}

// shared counts the words of four letters or more two texts share.
func shared(a, b string) int {
	words := func(s string) []string {
		return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !(r >= 'a' && r <= 'z') })
	}
	n := 0
	for _, word := range words(a) {
		if len(word) >= 4 && slices.Contains(words(b), word) {
			n++
		}
	}
	return n
}

// page serves one page of items, counted from 1, in the server's shape.
func (f *fake) page(w http.ResponseWriter, r *http.Request, items any) {
	raw, _ := json.Marshal(items)
	var all []json.RawMessage
	_ = json.Unmarshal(raw, &all)
	number, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if number < 1 || size < 1 || size > 100 {
		invalid(w, "page counts from 1 and size is 1 to 100")
		return
	}
	start := min((number-1)*size, len(all))
	pages := (len(all) + size - 1) / size
	write(w, http.StatusOK, map[string]any{"items": all[start:min(start+size, len(all))], "total": len(all), "page": number, "size": size, "pages": pages})
}

func TestContract(t *testing.T) {
	backendtest.Run(t, func(t *testing.T) backend.Backend {
		f := newFake(t, backendtest.Canary)
		return New(f.server.URL, "rashin-desktop", backendtest.Canary)
	})
}
