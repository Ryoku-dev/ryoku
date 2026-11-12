// Package hindsight keeps Rashin's memory in Hindsight, through Hindsight's
// official Go client: a server the user runs (Remote) or one Rashin installs
// and runs on this computer only while it needs it (Local).
package hindsight

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	hs "github.com/vectorize-io/hindsight/hindsight-clients/go"
	i18n "ryoku-i18n"
	"ryoku-rashin/internal/memory/backend"
	"ryoku-rashin/internal/protocol"
)

const (
	// manualPrefix starts the document of a fact Rashin remembered as it
	// is; each has its own document, so forgetting it deletes it for good.
	manualPrefix = "manual-"
	// sessionDoc starts the document a conversation's turns go into.
	sessionDoc = "session-"
	// exactStrategy is the bank's retain strategy that stores text as it
	// is, without asking a model to extract facts from it.
	exactStrategy = "rashin-exact"
	// cardModel is the mental model Hindsight writes the card in.
	cardModel = "rashin-card"
	// callTimeout bounds one call to a running server.
	callTimeout = 30 * time.Second
	// drainWait is how long the idle exit waits once for Hindsight to finish
	// the work it has queued.
	drainWait = 10 * time.Minute
	// listMax caps how many facts a list gathers from one bank.
	listMax = 10000
)

// cardQuery asks Hindsight what its card about the user holds.
const cardQuery = "Which lasting facts about the user would help an assistant: their work, projects, tools, preferences and how they like answers? State facts only."

// learned are the fact types Rashin shows and forgets. Observations are
// Hindsight's own summaries of them and cannot be forgotten one by one.
var learned = []string{"world", "experience"}

// Hindsight is memory kept in Hindsight. It learns from what Observe hands
// it.
type Hindsight struct {
	conn   endpoint
	proc   *Process
	leaves bool
	calls  backend.Calls
	now    func() time.Time

	mu      sync.Mutex
	banks   map[string]bool
	drainTo time.Time
	drained bool
}

// endpoint gives a client for the server, starting a local one first.
type endpoint interface {
	api(ctx context.Context) (*hs.APIClient, error)
}

type remote struct{ client *hs.APIClient }

func (r remote) api(context.Context) (*hs.APIClient, error) { return r.client, nil }

// Remote opens Hindsight at address with key, which may be empty for a
// server that asks for none.
func Remote(address, key string) *Hindsight {
	cfg := hs.NewConfiguration()
	cfg.Servers = hs.ServerConfigurations{{URL: strings.TrimRight(address, "/")}}
	cfg.HTTPClient = &http.Client{Timeout: callTimeout}
	if key != "" {
		cfg.AddDefaultHeader("Authorization", "Bearer "+key)
	}
	return newHindsight(remote{client: hs.NewAPIClient(cfg)}, nil, backend.LeavesMachine(address))
}

// local builds a client for each start of the child, whose port changes.
type local struct {
	proc *Process

	mu     sync.Mutex
	base   string
	client *hs.APIClient
}

func (l *local) api(ctx context.Context) (*hs.APIClient, error) {
	base, key, err := l.proc.endpoint(ctx)
	if err != nil {
		return nil, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.client == nil || l.base != base {
		l.base, l.client = base, hs.NewAPIClientWithTimeout(base, key, callTimeout)
	}
	return l.client, nil
}

// Local keeps memory in the Hindsight p runs on this computer.
func Local(p *Process) *Hindsight {
	return newHindsight(&local{proc: p}, p, false)
}

func newHindsight(conn endpoint, proc *Process, leaves bool) *Hindsight {
	return &Hindsight{conn: conn, proc: proc, leaves: leaves, now: time.Now, banks: map[string]bool{}}
}

// Caps: learned facts are curated in place and kept by Hindsight with no
// pin or earlier text; a conversation can be forgotten whole.
func (h *Hindsight) Caps() backend.Caps {
	return backend.Caps{
		MemoryCaps: protocol.MemoryCaps{Edit: true, ForgetSession: true, LeavesMachine: h.leaves},
		Learns:     true,
		Card:       true,
	}
}

// failure words a call's error. A 401 means the key, and any other answer
// gives its status; the key is never part of it.
func failure(err error, resp *http.Response) error {
	if err == nil {
		return nil
	}
	if resp == nil {
		return fmt.Errorf("%s: %w", i18n.T("Hindsight cannot be reached"), backend.Bare(err))
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return errors.New(i18n.T("Hindsight refused the key."))
	}
	var api *hs.GenericOpenAPIError
	if errors.As(err, &api) {
		var detail struct {
			Detail any `json:"detail"`
		}
		if json.Unmarshal(api.Body(), &detail) == nil && detail.Detail != nil {
			return errors.New(i18n.Tf("Hindsight answered %d: %v", resp.StatusCode, detail.Detail))
		}
	}
	return errors.New(i18n.Tf("Hindsight answered %d.", resp.StatusCode))
}

func missing(resp *http.Response) bool { return resp != nil && resp.StatusCode == http.StatusNotFound }

func text(s string) hs.NullableString { return *hs.NewNullableString(&s) }

// factID joins a bank and a memory unit into one id, and unit splits it.
func factID(bank, unit string) string { return bank + "." + unit }

func unit(id string) (bank, unit string, ok bool) {
	bank, unit, ok = strings.Cut(id, ".")
	if _, scoped := backend.ScopeOf(bank); !ok || !scoped || unit == "" {
		return "", "", false
	}
	return bank, unit, true
}

// bank gets or makes a scope's bank, with the strategy that stores a fact as
// it is.
func (h *Hindsight) bank(ctx context.Context, api *hs.APIClient, scope string) (string, error) {
	bank := backend.ScopeID(scope)
	if bank == "" {
		return "", errors.New(i18n.Tf("%s is not a memory scope.", scope))
	}
	h.mu.Lock()
	known := h.banks[bank]
	h.mu.Unlock()
	if known {
		return bank, nil
	}
	_, resp, err := api.BanksAPI.CreateOrUpdateBank(ctx, bank).CreateBankRequest(hs.CreateBankRequest{Name: text(scope)}).Execute()
	if err != nil {
		return "", failure(err, resp)
	}
	strategies := map[string]any{exactStrategy: map[string]any{"retain_extraction_mode": "chunks"}}
	_, resp, err = api.BanksAPI.UpdateBankConfig(ctx, bank).BankConfigUpdate(hs.BankConfigUpdate{Updates: map[string]any{"retain_strategies": strategies}}).Execute()
	if err != nil {
		return "", failure(err, resp)
	}
	h.mu.Lock()
	h.banks[bank] = true
	h.mu.Unlock()
	return bank, nil
}

// rashinBanks lists the banks Rashin made, by scope.
func (h *Hindsight) rashinBanks(ctx context.Context, api *hs.APIClient) (map[string]string, error) {
	banks := map[string]string{}
	for offset := int32(0); ; offset += 100 {
		list, resp, err := api.BanksAPI.ListBanks(ctx).Limit(100).Offset(offset).Execute()
		if err != nil {
			return nil, failure(err, resp)
		}
		for _, b := range list.Banks {
			if scope, ok := backend.ScopeOf(b.BankId); ok {
				banks[scope] = b.BankId
			}
		}
		if len(list.Banks) < 100 {
			return banks, nil
		}
	}
}

func randomID() string {
	raw := make([]byte, 8)
	_, _ = rand.Read(raw)
	return hex.EncodeToString(raw)
}

// Remember stores each fact as it is, in a document of its own.
func (h *Hindsight) Remember(ctx context.Context, facts []protocol.Memory) ([]protocol.Memory, error) {
	defer h.calls.Begin()()
	api, err := h.conn.api(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]protocol.Memory, 0, len(facts))
	for _, f := range facts {
		bank, err := h.bank(ctx, api, f.Scope)
		if err != nil {
			return out, err
		}
		doc := manualPrefix + randomID()
		item := hs.MemoryItem{
			Content: hs.TextContent(f.Text), Context: text(i18n.T("Something the user asked Rashin to remember.")),
			DocumentId: text(doc), Tags: tags(f.Source), Strategy: text(exactStrategy),
		}
		_, resp, err := api.MemoryAPI.RetainMemories(ctx, bank).RetainRequest(hs.RetainRequest{Items: []hs.MemoryItem{item}, Async: hs.PtrBool(false)}).Execute()
		if err != nil {
			return out, failure(err, resp)
		}
		list, resp, err := api.MemoryAPI.ListMemories(ctx, bank).DocumentId(doc).Limit(10).Execute()
		if err != nil {
			return out, failure(err, resp)
		}
		if len(list.Items) == 0 {
			return out, errors.New(i18n.T("Hindsight did not keep the memory."))
		}
		kept := fact(bank, f.Scope, list.Items[0])
		kept.Kind, kept.Pinned, kept.Sensitive, kept.Source = f.Kind, f.Pinned, f.Sensitive, f.Source
		out = append(out, kept)
	}
	return out, nil
}

// tags mark where a fact came from, so a list can say it.
func tags(source protocol.MemorySource) []string {
	var out []string
	if source.Kind != "" {
		out = append(out, "origin:"+source.Kind)
	}
	if source.Session != "" {
		out = append(out, "session:"+source.Session)
	}
	return out
}

// fact turns a listed memory unit into a memory.
func fact(bank, scope string, item hs.MemoryUnitListItem) protocol.Memory {
	m := protocol.Memory{ID: factID(bank, item.Id), Scope: scope, Status: protocol.MemoryActive}
	if item.Text != nil {
		m.Text = *item.Text
	}
	m.Created = when(item.MentionedAt.Get(), item.Date)
	m.Updated = m.Created
	if edited := when(item.EditedAt.Get()); !edited.IsZero() {
		m.Updated = edited
	}
	from(&m.Source, item.Tags)
	return m
}

// from reads a fact's origin back out of its tags.
func from(source *protocol.MemorySource, tags []string) {
	for _, tag := range tags {
		if kind, ok := strings.CutPrefix(tag, "origin:"); ok {
			source.Kind = kind
		}
		if session, ok := strings.CutPrefix(tag, "session:"); ok {
			source.Session = session
		}
	}
}

// when parses the first of the times that is set.
func when(times ...*string) time.Time {
	for _, t := range times {
		if t == nil {
			continue
		}
		if parsed, err := time.Parse(time.RFC3339Nano, *t); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

// Edit curates a fact's text in place; Hindsight keeps its id.
func (h *Hindsight) Edit(ctx context.Context, id, newText string) (protocol.Memory, error) {
	defer h.calls.Begin()()
	bank, memory, ok := unit(id)
	if !ok {
		return protocol.Memory{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That memory is gone."))
	}
	api, err := h.conn.api(ctx)
	if err != nil {
		return protocol.Memory{}, err
	}
	_, resp, err := api.MemoryAPI.UpdateMemory(ctx, bank, memory).UpdateMemoryRequest(hs.UpdateMemoryRequest{Text: text(newText)}).Execute()
	if missing(resp) {
		return protocol.Memory{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That memory is gone."))
	}
	if err != nil {
		return protocol.Memory{}, failure(err, resp)
	}
	scope, _ := backend.ScopeOf(bank)
	return protocol.Memory{ID: id, Scope: scope, Text: newText, Status: protocol.MemoryActive, Updated: h.now()}, nil
}

// Recall searches each scope's bank and takes the best of each in turn.
func (h *Hindsight) Recall(ctx context.Context, q backend.Query) ([]protocol.Memory, error) {
	defer h.calls.Begin()()
	api, err := h.conn.api(ctx)
	if err != nil {
		return nil, err
	}
	banks, err := h.rashinBanks(ctx, api)
	if err != nil {
		return nil, err
	}
	var scopes []string
	for scope := range banks {
		if len(q.Scopes) == 0 || slices.ContainsFunc(q.Scopes, func(group string) bool { return backend.InGroup(scope, group) }) {
			scopes = append(scopes, scope)
		}
	}
	slices.Sort(scopes)
	limit := max(q.Limit, 1)
	budget := hs.LOW
	var lists [][]protocol.Memory
	for _, scope := range scopes {
		bank, ok := banks[scope]
		if !ok {
			continue
		}
		found, resp, err := api.MemoryAPI.RecallMemories(ctx, bank).RecallRequest(hs.RecallRequest{
			Query: q.Text, Types: learned, Budget: &budget, MaxTokens: hs.PtrInt32(2048),
		}).Execute()
		if err != nil {
			return nil, failure(err, resp)
		}
		var list []protocol.Memory
		for _, r := range found.Results {
			m := protocol.Memory{ID: factID(bank, r.Id), Scope: scope, Text: r.Text, Status: protocol.MemoryActive}
			m.Created = when(r.MentionedAt.Get(), r.OccurredStart.Get())
			m.Updated = m.Created
			from(&m.Source, r.Tags)
			list = append(list, m)
		}
		lists = append(lists, list)
	}
	return backend.Interleave(lists, limit), nil
}

// Observe adds each turn to the conversation's document, which Hindsight
// learns from in the background.
func (h *Hindsight) Observe(ctx context.Context, t backend.Transcript) error {
	defer h.calls.Begin()()
	if h.proc != nil && t.Model != "" {
		h.proc.setAnswered(t.Model)
	}
	api, err := h.conn.api(ctx)
	if err != nil {
		return err
	}
	bank, err := h.bank(ctx, api, t.Scope)
	if err != nil {
		return err
	}
	items := make([]hs.MemoryItem, 0, len(t.Turns))
	for _, turn := range t.Turns {
		var b strings.Builder
		fmt.Fprintf(&b, "User: %s", turn.User)
		for _, answer := range turn.Answers {
			fmt.Fprintf(&b, "\n\nUser: %s", answer)
		}
		if turn.Reply != "" {
			fmt.Fprintf(&b, "\n\nAssistant: %s", turn.Reply)
		}
		at := turn.At.UTC().Format(time.RFC3339)
		items = append(items, hs.MemoryItem{
			Content:    hs.TextContent(b.String()),
			Timestamp:  *hs.NewNullableTimestamp(&hs.Timestamp{String: &at}),
			Context:    text(i18n.Tf("A conversation with Rashin: %s", t.Title)),
			DocumentId: text(turnDoc(t.Session, turn)),
			Tags:       tags(protocol.MemorySource{Kind: t.Kind, Session: t.Session}),
		})
	}
	if len(items) == 0 {
		return nil
	}
	_, resp, err := api.MemoryAPI.RetainMemories(ctx, bank).RetainRequest(hs.RetainRequest{Items: items, Async: hs.PtrBool(true)}).Execute()
	return failure(err, resp)
}

// turnDoc names the document one turn goes into. Hindsight keeps no text to
// append to, so each turn is a document of its own, and handing the same
// turn again replaces it rather than adding it twice.
func turnDoc(session string, turn backend.Turn) string {
	at := turn.Seq
	if at == 0 {
		at = turn.At.UnixNano()
	}
	return fmt.Sprintf("%s%s-%d", sessionDoc, session, at)
}

// List gathers the facts of one bank, or of every bank in a group, newest
// first, and pages them.
func (h *Hindsight) List(ctx context.Context, scope string, offset, limit int) ([]protocol.Memory, int, error) {
	defer h.calls.Begin()()
	api, err := h.conn.api(ctx)
	if err != nil {
		return nil, 0, err
	}
	all, err := h.all(ctx, api, scope)
	if err != nil {
		return nil, 0, err
	}
	return backend.Page(all, offset, limit), len(all), nil
}

func (h *Hindsight) all(ctx context.Context, api *hs.APIClient, group string) ([]protocol.Memory, error) {
	banks, err := h.rashinBanks(ctx, api)
	if err != nil {
		return nil, err
	}
	var out []protocol.Memory
	for scope, bank := range banks {
		if !backend.InGroup(scope, group) {
			continue
		}
		for offset := int32(0); offset < listMax; offset += 100 {
			list, resp, err := api.MemoryAPI.ListMemories(ctx, bank).Limit(100).Offset(offset).Execute()
			if missing(resp) {
				break
			}
			if err != nil {
				return nil, failure(err, resp)
			}
			for _, item := range list.Items {
				if item.FactType.Get() != nil && slices.Contains(learned, *item.FactType.Get()) {
					out = append(out, fact(bank, scope, item))
				}
			}
			if len(list.Items) < 100 {
				break
			}
		}
	}
	slices.SortStableFunc(out, func(a, b protocol.Memory) int { return b.Created.Compare(a.Created) })
	return out, nil
}

// Forget deletes a remembered fact with its document. A learned fact shares
// its document with the rest of its conversation, so it is invalidated
// instead, which takes it out of recall, and what Hindsight concluded from
// it is cleared.
func (h *Hindsight) Forget(ctx context.Context, ids []string) error {
	defer h.calls.Begin()()
	api, err := h.conn.api(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		bank, memory, ok := unit(id)
		if !ok {
			continue
		}
		got, resp, err := api.MemoryAPI.GetMemory(ctx, bank, memory).Execute()
		if missing(resp) {
			continue
		}
		if err != nil {
			return failure(err, resp)
		}
		doc, _ := got.(map[string]any)["document_id"].(string)
		if strings.HasPrefix(doc, manualPrefix) {
			if _, resp, err := api.DocumentsAPI.DeleteDocument(ctx, bank, doc).Execute(); err != nil && !missing(resp) {
				return failure(err, resp)
			}
			continue
		}
		_, resp, err = api.MemoryAPI.UpdateMemory(ctx, bank, memory).UpdateMemoryRequest(hs.UpdateMemoryRequest{
			State: text("invalidated"), Reason: text(i18n.T("Forgotten in Rashin.")),
		}).Execute()
		if err != nil && !missing(resp) {
			return failure(err, resp)
		}
		if _, resp, err := api.MemoryAPI.ClearMemoryObservations(ctx, bank, memory).Execute(); err != nil && !missing(resp) {
			return failure(err, resp)
		}
	}
	return nil
}

// ForgetScope deletes a scope's bank, or every bank Rashin made.
func (h *Hindsight) ForgetScope(ctx context.Context, scope string) error {
	defer h.calls.Begin()()
	api, err := h.conn.api(ctx)
	if err != nil {
		return err
	}
	banks, err := h.rashinBanks(ctx, api)
	if err != nil {
		return err
	}
	for each, bank := range banks {
		if !backend.InGroup(each, scope) {
			continue
		}
		if _, resp, err := api.BanksAPI.DeleteBank(ctx, bank).Execute(); err != nil && !missing(resp) {
			return failure(err, resp)
		}
		h.mu.Lock()
		delete(h.banks, bank)
		h.mu.Unlock()
	}
	return nil
}

// Card is the mental model Hindsight keeps about the user in the personal
// bank, made on first use and refreshed by Hindsight after it learns. It is
// empty until Hindsight first writes it.
func (h *Hindsight) Card(ctx context.Context) (string, error) {
	defer h.calls.Begin()()
	api, err := h.conn.api(ctx)
	if err != nil {
		return "", err
	}
	bank, err := h.bank(ctx, api, protocol.MemoryYou)
	if err != nil {
		return "", err
	}
	model, resp, err := api.MentalModelsAPI.GetMentalModel(ctx, bank, cardModel).Execute()
	if missing(resp) {
		refresh := true
		_, resp, err = api.MentalModelsAPI.CreateMentalModel(ctx, bank).CreateMentalModelRequest(hs.CreateMentalModelRequest{
			Id: text(cardModel), Name: i18n.T("About the user"), SourceQuery: cardQuery,
			MaxTokens: hs.PtrInt32(512), Trigger: &hs.MentalModelTriggerInput{RefreshAfterConsolidation: &refresh},
		}).Execute()
		return "", failure(err, resp)
	}
	if err != nil {
		return "", failure(err, resp)
	}
	if content := model.Content.Get(); content != nil {
		return *content, nil
	}
	return "", nil
}

// Status says whether Hindsight answers and how much it still has to learn.
// A local Hindsight that is not running is not started for it.
func (h *Hindsight) Status(ctx context.Context) backend.Status {
	if h.proc != nil {
		if !h.proc.Layout().Installed() {
			return backend.Status{State: protocol.EngineSetup, Detail: i18n.T("Set up Hindsight first, in Settings > Memory.")}
		}
		starting, running := h.proc.state()
		switch {
		case starting:
			return backend.Status{State: protocol.EngineStarting}
		case !running:
			return backend.Status{State: protocol.EngineStopped, Detail: i18n.T("Starts when needed.")}
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	api, err := h.conn.api(ctx)
	if err != nil {
		return backend.Status{State: protocol.EngineUnreachable, Detail: err.Error()}
	}
	if _, resp, err := api.MonitoringAPI.HealthEndpointHealthGet(ctx).Execute(); err != nil {
		return backend.Status{State: protocol.EngineUnreachable, Detail: failure(err, resp).Error()}
	}
	pending, err := h.pending(ctx, api)
	if err != nil {
		return backend.Status{State: protocol.EngineUnreachable, Detail: err.Error()}
	}
	return backend.Status{State: protocol.EngineReady, Learning: pending}
}

// pending counts the work Hindsight has queued in Rashin's banks.
func (h *Hindsight) pending(ctx context.Context, api *hs.APIClient) (int, error) {
	banks, err := h.rashinBanks(ctx, api)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, bank := range banks {
		stats, resp, err := api.BanksAPI.GetAgentStats(ctx, bank).Execute()
		if missing(resp) {
			continue
		}
		if err != nil {
			return 0, failure(err, resp)
		}
		total += int(stats.PendingOperations)
		if stats.PendingConsolidation != nil {
			total += int(*stats.PendingConsolidation)
		}
	}
	return total, nil
}

// Busy is true while a call is in flight, a local Hindsight is starting, or
// the idle exit is giving it one drain window to finish its work.
func (h *Hindsight) Busy() bool {
	if h.calls.Busy() {
		return true
	}
	if h.proc != nil {
		if starting, _ := h.proc.state(); starting {
			return true
		}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.now().Before(h.drainTo)
}

// Settle asks a local Hindsight that is running once whether it still has
// work. If it has, Busy stays true for one drain window; after that the
// daemon may stop it whatever is left, which waits in its database for the
// next start. A remote Hindsight works on without Rashin, so it never holds
// the daemon up.
func (h *Hindsight) Settle(ctx context.Context) {
	h.mu.Lock()
	drained := h.drained
	h.mu.Unlock()
	if drained || h.proc == nil {
		return
	}
	if _, running := h.proc.state(); !running {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	api, err := h.conn.api(ctx)
	if err != nil {
		return
	}
	pending, err := h.pending(ctx, api)
	if err != nil || pending == 0 {
		return
	}
	h.mu.Lock()
	h.drained, h.drainTo = true, h.now().Add(drainWait)
	h.mu.Unlock()
}

// Release stops a local Hindsight at the idle exit; the next call starts it
// again with a fresh drain window.
func (h *Hindsight) Release() {
	if h.proc == nil {
		return
	}
	h.proc.Stop()
	h.mu.Lock()
	h.drained, h.drainTo = false, time.Time{}
	h.mu.Unlock()
}

// Close stops a local Hindsight.
func (h *Hindsight) Close() error {
	h.Release()
	return nil
}
