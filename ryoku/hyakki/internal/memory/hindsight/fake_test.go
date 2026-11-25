package hindsight

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// fakeServer is a Hindsight API kept in memory, in the shapes of the
// official spec, for the calls Rashin makes. A retain with the exact
// strategy stores the text as it is; any other learns each sentence the
// user says as one fact, at once. Every /v1 call needs the key.
type fakeServer struct {
	key     string
	pending int32 // what stats reports as pending operations

	mu    sync.Mutex
	banks map[string]*fakeBank
	order []string
	next  int
}

type fakeBank struct {
	name       string
	strategies map[string]any
	units      []*fakeUnit
	models     map[string]string
}

type fakeUnit struct {
	ID        string   `json:"id"`
	Text      string   `json:"text"`
	FactType  string   `json:"fact_type"`
	Document  string   `json:"document_id"`
	Tags      []string `json:"tags"`
	State     string   `json:"state"`
	Mentioned string   `json:"mentioned_at"`
}

func newFakeServer(key string) *fakeServer {
	return &fakeServer{key: key, banks: map[string]*fakeBank{}}
}

func reply(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (f *fakeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/health" {
		reply(w, http.StatusOK, map[string]string{"status": "healthy", "database": "connected"})
		return
	}
	if r.Header.Get("Authorization") != "Bearer "+f.key {
		reply(w, http.StatusUnauthorized, map[string]string{"detail": "Invalid API key"})
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	rest, ok := strings.CutPrefix(r.URL.Path, "/v1/default/banks")
	if !ok {
		reply(w, http.StatusNotFound, map[string]string{"detail": "Not Found"})
		return
	}
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	for i := range parts {
		parts[i], _ = url.PathUnescape(parts[i])
	}
	var body map[string]json.RawMessage
	_ = json.NewDecoder(r.Body).Decode(&body)
	if rest == "" && r.Method == http.MethodGet {
		f.listBanks(w, r)
		return
	}
	id := parts[0]
	b := f.banks[id]
	route := r.Method + " " + strings.Join(parts[1:], "/")
	if route == "PUT " {
		if b == nil {
			var name string
			_ = json.Unmarshal(body["name"], &name)
			f.banks[id] = &fakeBank{name: name, models: map[string]string{}}
			f.order = append(f.order, id)
		}
		reply(w, http.StatusOK, map[string]any{"bank_id": id, "name": f.banks[id].name, "mission": "", "disposition": disposition()})
		return
	}
	if b == nil {
		reply(w, http.StatusNotFound, map[string]string{"detail": "Bank not found"})
		return
	}
	switch {
	case route == "DELETE ":
		delete(f.banks, id)
		f.order = slices.DeleteFunc(f.order, func(each string) bool { return each == id })
		reply(w, http.StatusOK, map[string]any{"success": true})
	case route == "PATCH config":
		var update struct {
			Updates map[string]any `json:"updates"`
		}
		raw, _ := json.Marshal(body)
		_ = json.Unmarshal(raw, &update)
		if strategies, ok := update.Updates["retain_strategies"].(map[string]any); ok {
			b.strategies = strategies
		}
		reply(w, http.StatusOK, map[string]any{"bank_id": id, "config": map[string]any{}, "overrides": update.Updates})
	case route == "POST memories":
		f.retain(w, id, b, body)
	case route == "GET memories/list":
		f.list(w, r, b)
	case route == "POST memories/recall":
		f.recall(w, b, body)
	case len(parts) == 3 && parts[1] == "memories" && (r.Method == http.MethodGet || r.Method == http.MethodPatch):
		at := slices.IndexFunc(b.units, func(u *fakeUnit) bool { return u.ID == parts[2] })
		if at < 0 {
			reply(w, http.StatusNotFound, map[string]string{"detail": "Memory not found"})
			return
		}
		u := b.units[at]
		if r.Method == http.MethodPatch {
			var text, state string
			_ = json.Unmarshal(body["text"], &text)
			_ = json.Unmarshal(body["state"], &state)
			if text != "" {
				u.Text = text
			}
			if state != "" {
				u.State = state
			}
		}
		reply(w, http.StatusOK, u)
	case len(parts) == 4 && parts[1] == "memories" && parts[3] == "observations" && r.Method == http.MethodDelete:
		reply(w, http.StatusOK, map[string]any{"deleted_count": 0})
	case len(parts) == 3 && parts[1] == "documents" && r.Method == http.MethodDelete:
		before := len(b.units)
		b.units = slices.DeleteFunc(b.units, func(u *fakeUnit) bool { return u.Document == parts[2] })
		if before == len(b.units) {
			reply(w, http.StatusNotFound, map[string]string{"detail": "Document not found"})
			return
		}
		reply(w, http.StatusOK, map[string]any{"success": true, "message": "deleted", "document_id": parts[2], "memory_units_deleted": before - len(b.units)})
	case route == "GET stats":
		reply(w, http.StatusOK, map[string]any{
			"bank_id": id, "total_nodes": len(b.units), "total_links": 0, "total_documents": 0, "failed_operations": 0,
			"pending_operations": f.pending, "nodes_by_fact_type": map[string]int{}, "links_by_link_type": map[string]int{},
			"links_by_fact_type": map[string]int{}, "links_breakdown": map[string]any{},
		})
	case len(parts) == 3 && parts[1] == "mental-models" && r.Method == http.MethodGet:
		content, ok := b.models[parts[2]]
		if !ok {
			reply(w, http.StatusNotFound, map[string]string{"detail": "Mental model not found"})
			return
		}
		reply(w, http.StatusOK, map[string]any{"bank_id": id, "id": parts[2], "name": "card", "content": content})
	case route == "POST mental-models":
		var model string
		_ = json.Unmarshal(body["id"], &model)
		b.models[model] = ""
		reply(w, http.StatusOK, map[string]any{"operation_id": "op-1"})
	default:
		reply(w, http.StatusNotFound, map[string]string{"detail": "no route " + route})
	}
}

func disposition() map[string]int {
	return map[string]int{"empathy": 3, "literalism": 3, "skepticism": 3}
}

func (f *fakeServer) listBanks(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	items := []map[string]any{}
	for _, id := range f.order[min(offset, len(f.order)):min(offset+limit, len(f.order))] {
		items = append(items, map[string]any{"bank_id": id, "name": f.banks[id].name, "disposition": disposition()})
	}
	reply(w, http.StatusOK, map[string]any{"banks": items, "total": len(f.order), "limit": limit, "offset": offset})
}

func (f *fakeServer) retain(w http.ResponseWriter, id string, b *fakeBank, body map[string]json.RawMessage) {
	var items []struct {
		Content   string   `json:"content"`
		Document  string   `json:"document_id"`
		Tags      []string `json:"tags"`
		Strategy  string   `json:"strategy"`
		Timestamp string   `json:"timestamp"`
	}
	var async bool
	_ = json.Unmarshal(body["items"], &items)
	_ = json.Unmarshal(body["async"], &async)
	for _, item := range items {
		b.units = slices.DeleteFunc(b.units, func(u *fakeUnit) bool { return u.Document == item.Document })
		mentioned := item.Timestamp
		if mentioned == "" {
			mentioned = time.Now().UTC().Add(time.Duration(f.next) * time.Millisecond).Format(time.RFC3339Nano)
		}
		var texts []string
		if strategy, ok := b.strategies[item.Strategy].(map[string]any); ok && strategy["retain_extraction_mode"] == "chunks" {
			texts = []string{item.Content}
		} else {
			for line := range strings.SplitSeq(item.Content, "\n") {
				if said, ok := strings.CutPrefix(line, "User: "); ok {
					for _, sentence := range strings.SplitAfter(said, ".") {
						if sentence = strings.TrimSpace(sentence); sentence != "" {
							texts = append(texts, sentence)
						}
					}
				}
			}
		}
		for _, text := range texts {
			f.next++
			b.units = append(b.units, &fakeUnit{ID: fmt.Sprintf("u-%06d", f.next), Text: text, FactType: "world", Document: item.Document, Tags: item.Tags, State: "valid", Mentioned: mentioned})
		}
	}
	reply(w, http.StatusOK, map[string]any{"success": true, "bank_id": id, "items_count": len(items), "async": async})
}

func (f *fakeServer) list(w http.ResponseWriter, r *http.Request, b *fakeBank) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	items := []*fakeUnit{}
	for _, u := range slices.Backward(b.units) {
		if u.State != "valid" || q.Get("document_id") != "" && u.Document != q.Get("document_id") || q.Get("q") != "" && !strings.Contains(u.Text, q.Get("q")) ||
			q.Get("tags") != "" && !slices.Contains(u.Tags, q.Get("tags")) {
			continue
		}
		items = append(items, u)
	}
	total := len(items)
	items = items[min(offset, total):min(offset+limit, total)]
	reply(w, http.StatusOK, map[string]any{"items": items, "total": total, "limit": limit, "offset": offset})
}

func (f *fakeServer) recall(w http.ResponseWriter, b *fakeBank, body map[string]json.RawMessage) {
	var query string
	var types []string
	_ = json.Unmarshal(body["query"], &query)
	_ = json.Unmarshal(body["types"], &types)
	type hit struct {
		u     *fakeUnit
		score int
	}
	var hits []hit
	for _, u := range b.units {
		if score := sharedWords(query, u.Text); u.State == "valid" && score > 0 && (len(types) == 0 || slices.Contains(types, u.FactType)) {
			hits = append(hits, hit{u, score})
		}
	}
	slices.SortStableFunc(hits, func(a, b hit) int { return b.score - a.score })
	results := []map[string]any{}
	for _, h := range hits {
		results = append(results, map[string]any{"id": h.u.ID, "text": h.u.Text, "type": h.u.FactType, "document_id": h.u.Document, "tags": h.u.Tags, "mentioned_at": h.u.Mentioned})
	}
	reply(w, http.StatusOK, map[string]any{"results": results})
}

// sharedWords counts the words of four letters or more two texts share.
func sharedWords(a, b string) int {
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
