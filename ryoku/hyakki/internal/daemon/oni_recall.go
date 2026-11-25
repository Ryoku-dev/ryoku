package daemon

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/imports/chats"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/oni/recall"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/voice/route"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/redact"
	"ryoku-rashin/internal/sensitive"
)

// Recall answers "what did I ask about" and "what do my notes say" from
// what the user kept: the chats they imported, past Hyakki conversations,
// memory facts and, only with oni.recall.vault on, the Ryoku vault, read
// only. Every search reads an index as it already is and never builds one.
// The passages go to the model fenced as data. A route on this computer
// sends nothing off it and asks nothing; a hosted route first shows a card
// naming the route and the passages' titles, and nothing leaves before a
// yes.

// oniRecall is the recall card waiting on its answer.
type oniRecall struct {
	importMu sync.Mutex
	mu       sync.Mutex
	card     *recallCard
}

// recallCard holds what a yes sends: the question, the fenced passages and
// the plan, in memory only, and who may answer it.
type recallCard struct {
	id       string
	conn     *connection
	question string
	fence    string
	sources  []protocol.OniRecallSource
	plan     llm.Plan
	timer    presence.Timer
}

// recallLimits is the spec's recall section.
func recallLimits() spec.Recall {
	if s, err := oniPhase16Spec(); err == nil {
		return s.Recall
	}
	return spec.Recall{}
}

// recallRoot is where imported chats are kept, one index root.
func recallRoot() string { return filepath.Join(paths.DataDir(), "imports", "chats") }

// Why a recall was refused.
const (
	recallRefusedVault   = "vault"
	recallRefusedNoModel = "no-model"
)

// recallRefusal says why recall may not run, or nil.
func recallRefusal(settings protocol.OniSettings) error {
	decision := feature.Decide(feature.Request{Feature: feature.Recall}, settings)
	if decision.Allow {
		return nil
	}
	message := i18n.T("Recall is off. Turn it on in Oni's settings.")
	if decision.Reason == feature.ReasonOniOff {
		message = i18n.T("Oni is off.")
	}
	return &protocol.Error{Code: protocol.CodeDisabled, Message: message, Reason: decision.Reason}
}

// voiceRecallOn reports whether recall questions are on.
func (d *Daemon) voiceRecallOn() bool {
	return recallRefusal(protocol.EffectiveOni(d.Settings())) == nil
}

// oniRecallCall serves oni.recall for the app and the CLI.
func (d *Daemon) oniRecallCall(c *connection, p protocol.OniRecallParams) (protocol.OniRecallResult, error) {
	settings := protocol.EffectiveOni(d.Settings())
	if err := recallRefusal(settings); err != nil {
		return protocol.OniRecallResult{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), oniAskTimeout)
	defer cancel()
	if p.Action == protocol.OniRecallKnow {
		return d.recallKnow(ctx, strings.TrimSpace(p.Text))
	}
	q, ok := recall.Detect(p.Text)
	if !ok {
		q = recall.Query{Topic: strings.TrimSpace(p.Text)}
	}
	return d.recallAsk(ctx, c, q, p.Text, settings)
}

// recallKnow says what memory holds about topic, or about the user: no
// model, nothing leaves the computer.
func (d *Daemon) recallKnow(ctx context.Context, topic string) (protocol.OniRecallResult, error) {
	if d.deps.Memory == nil {
		return protocol.OniRecallResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Memory is not available."))
	}
	facts, err := d.memoryFacts(ctx, topic)
	if err != nil {
		return protocol.OniRecallResult{}, err
	}
	res := protocol.OniRecallResult{Status: protocol.OniRecallStatusAnswered, Answer: knowLine(facts, topic), Destination: "local"}
	if len(facts) == 0 {
		res.Status = protocol.OniRecallStatusEmpty
	}
	for _, f := range facts {
		res.Sources = append(res.Sources, protocol.OniRecallSource{Kind: protocol.OniRecallFact, Title: oniText(f.Text, 80), From: f.ScopeName, At: f.Updated})
	}
	return res, nil
}

// recallAsk finds the passages for q and answers question from them, or
// shows the card a hosted route needs first.
func (d *Daemon) recallAsk(ctx context.Context, c *connection, q recall.Query, question string, settings protocol.OniSettings) (protocol.OniRecallResult, error) {
	if q.Where == recall.Notes && !settings.Recall.Vault {
		return protocol.OniRecallResult{Status: protocol.OniRecallStatusRefused, Refused: recallRefusedVault,
			Answer: i18n.T("Reading your vault is off. Turn it on in Recall's settings.")}, nil
	}
	if d.deps.Gateway == nil {
		return protocol.OniRecallResult{Status: protocol.OniRecallStatusRefused, Refused: recallRefusedNoModel, Answer: i18n.T("No model is set up to answer.")}, nil
	}
	passages := d.recallPassages(ctx, q, settings)
	l := recallLimits()
	fence, n := recall.Fence(passages, recall.Limits{Passages: l.Passages, Bytes: l.PromptBytes, PassageRunes: l.PassageTokens * 4})
	if n == 0 {
		return protocol.OniRecallResult{Status: protocol.OniRecallStatusEmpty, Answer: i18n.T("I found nothing about that in what you kept."), Destination: "local"}, nil
	}
	sources := make([]protocol.OniRecallSource, n)
	for i, p := range passages[:n] {
		sources[i] = protocol.OniRecallSource{Kind: p.Kind, Title: oniText(p.Title, 120), From: p.From, At: p.At}
	}
	plan, err := d.deps.Gateway.Plan(ctx, recallRequest("", ""))
	if err != nil {
		return protocol.OniRecallResult{}, noRouteError(err)
	}
	if c != nil {
		if err := hostedNeedsApp(c, plan.Route()); err != nil {
			return protocol.OniRecallResult{}, err
		}
	}
	card := &recallCard{conn: c, question: question, fence: fence, sources: sources, plan: plan}
	if plan.Local {
		return d.recallSend(card)
	}
	return d.recallCardOut(card)
}

// recallPassages are the passages for q, best source first: imported chats,
// past conversations, the vault when allowed, then memory facts. Each is
// scrubbed of secrets and private numbers before anything else sees it.
func (d *Daemon) recallPassages(ctx context.Context, q recall.Query, settings protocol.OniSettings) []recall.Passage {
	limit := max(recallLimits().Passages, 1)
	scrub := redact.Snapshot()
	clean := func(s string) string { return sensitive.Mask(scrub.Strict(s)) }
	var out []recall.Passage
	add := func(p recall.Passage) {
		p.Title, p.Text = clean(p.Title), clean(p.Text)
		if strings.TrimSpace(p.Text) != "" {
			out = append(out, p)
		}
	}
	if q.Where != recall.Notes {
		for _, p := range d.recallChats(ctx, q, limit) {
			add(p)
		}
		if d.deps.Memory != nil {
			var res protocol.MemorySessionsResult
			if d.memoryAsUser(ctx, protocol.MMemorySessions, protocol.MemorySessionsParams{Query: q.Topic, Limit: limit}, &res) == nil {
				for _, h := range res.Sessions {
					add(recall.Passage{Kind: protocol.OniRecallConversation, Title: h.Title, From: "Hyakki", At: h.Updated,
						Text: strings.TrimSpace(h.Summary + " " + strings.Join(h.Lines, " "))})
				}
			}
		}
	}
	if settings.Recall.Vault {
		for _, p := range d.recallVault(ctx, q.Topic, limit) {
			add(p)
		}
	}
	if q.Where != recall.Notes && d.deps.Memory != nil {
		if facts, err := d.memoryFacts(ctx, q.Topic); err == nil {
			for _, f := range naming(facts, q.Topic) {
				add(recall.Passage{Kind: protocol.OniRecallFact, Title: oniText(f.Text, 60), From: f.ScopeName, At: f.Updated, Text: f.Text})
			}
		}
	}
	return out
}

// recallChatsIndex is the id of the imported chats' index, "" when none
// is registered.
func (d *Daemon) recallChatsIndex(ctx context.Context) string {
	if d.deps.Indexer == nil {
		return ""
	}
	list, err := d.deps.Indexer.List(ctx)
	if err != nil {
		return ""
	}
	root := recallRoot()
	for _, info := range list {
		if info.Root == root || len(info.Roots) == 1 && info.Roots[0] == root {
			return info.ID
		}
	}
	return ""
}

// recallChats are the imported chats about q's topic, the source q names
// first when it names one.
func (d *Daemon) recallChats(ctx context.Context, q recall.Query, limit int) []recall.Passage {
	id := d.recallChatsIndex(ctx)
	if id == "" {
		return nil
	}
	res, err := d.deps.Indexer.Search(ctx, protocol.SearchParams{IndexRef: protocol.IndexRef{ID: id}, Query: q.Topic, Limit: limit * 2, Existing: true})
	if err != nil {
		return nil
	}
	root := recallRoot()
	var named, rest []recall.Passage
	for _, h := range res.Hits {
		full := filepath.Join(root, filepath.FromSlash(h.Path))
		if !strings.HasPrefix(full, root+string(filepath.Separator)) {
			continue
		}
		title, source, at := chats.Head(readHead(full))
		text := h.Snippet
		if text == "" {
			text = readLines(full, h.Start, h.End)
		}
		p := recall.Passage{Kind: protocol.OniRecallChat, Title: title, From: source, At: at, Text: text}
		if q.Who != "" && strings.EqualFold(source, q.Who) {
			named = append(named, p)
		} else {
			rest = append(rest, p)
		}
	}
	return append(named, rest...)
}

// recallVault are the vault's passages about topic: hits of the system
// index under its vault root alone, read only.
func (d *Daemon) recallVault(ctx context.Context, topic string, limit int) []recall.Passage {
	if d.deps.Indexer == nil {
		return nil
	}
	res, err := d.deps.Indexer.Search(ctx, protocol.SearchParams{IndexRef: protocol.IndexRef{ID: protocol.IndexSystem}, Query: topic, Limit: limit * 2, Existing: true})
	if err != nil {
		return nil
	}
	vault := paths.VaultDir()
	var out []recall.Passage
	for _, h := range res.Hits {
		rel, ok := strings.CutPrefix(h.Path, "vault/")
		if !ok {
			continue
		}
		full := filepath.Join(vault, filepath.FromSlash(rel))
		if !strings.HasPrefix(full, vault+string(filepath.Separator)) {
			continue
		}
		text := h.Snippet
		if text == "" {
			text = readLines(full, h.Start, h.End)
		}
		out = append(out, recall.Passage{Kind: protocol.OniRecallVault, Title: rel, From: "vault", Text: text})
	}
	return out
}

// readHead is the first lines of a file, where an imported chat keeps its
// title, source and date.
func readHead(name string) string {
	f, err := os.Open(name)
	if err != nil {
		return ""
	}
	defer f.Close()
	b, _ := io.ReadAll(io.LimitReader(f, 1024))
	return string(b)
}

// readLines is lines start to end of a file, counted from 1, at most 8 KiB.
func readLines(name string, start, end int) string {
	f, err := os.Open(name)
	if err != nil {
		return ""
	}
	defer f.Close()
	var b strings.Builder
	sc := bufio.NewScanner(io.LimitReader(f, 4<<20))
	for n := 1; sc.Scan() && n <= end && b.Len() < 8<<10; n++ {
		if n >= start {
			b.WriteString(sc.Text())
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// recallRequest is a recall answer's request: light work, so with no pin
// the smallest model that fits answers it, a local one first and never a
// paid one.
func recallRequest(system, user string) llm.Request {
	return llm.Request{Light: true, Messages: []llm.Message{
		{Role: llm.RoleSystem, Content: system},
		{Role: llm.RoleUser, Content: user},
	}}
}

// recallCardOut shows the card a hosted route needs, naming the route and
// the passages' titles; the passages wait in memory for the answer.
func (d *Daemon) recallCardOut(card *recallCard) (protocol.OniRecallResult, error) {
	svc := d.oni.service.Load()
	if svc == nil {
		return protocol.OniRecallResult{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Open Oni to see where your notes would go."))
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return protocol.OniRecallResult{}, err
	}
	card.id = feature.RecallCardPrefix + hex.EncodeToString(b[:])
	titles := make([]string, len(card.sources))
	for i, s := range card.sources {
		titles[i] = s.Title
	}
	route := card.plan.Route()
	question := oniText(i18n.Tf("Send %d passages from what you kept (%s) to %s to answer this?", len(titles), strings.Join(titles, "; "), route), 400)
	r := &d.oni.recall
	r.mu.Lock()
	d.recallCardCloseLocked(false)
	card.timer = d.reflexClock().AfterFunc(feature.ConsentTTL, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.card == card {
			d.recallCardCloseLocked(false)
		}
	})
	r.card = card
	r.mu.Unlock()
	if err := svc.Consent(*feature.RecallCard(card.id, route, question)); err != nil {
		r.mu.Lock()
		if r.card == card {
			d.recallCardCloseLocked(false)
		}
		r.mu.Unlock()
		return protocol.OniRecallResult{}, presenceRefusal(err)
	}
	return protocol.OniRecallResult{Status: protocol.OniRecallStatusConsent, Route: route, Destination: "hosted", Sources: card.sources,
		Consent: &protocol.OniRecallConsent{ID: card.id, Route: route, Question: question, Titles: titles, ExpiresMs: feature.ConsentTTL.Milliseconds()}}, nil
}

// recallCardCloseLocked drops the card, clearing what it held. r.mu is
// held.
func (d *Daemon) recallCardCloseLocked(yes bool) {
	r := &d.oni.recall
	card := r.card
	if card == nil {
		return
	}
	r.card = nil
	card.timer.Stop()
	if !yes {
		card.fence, card.question = "", ""
	}
	answer := "no"
	if yes {
		answer = "yes"
	}
	if svc := d.oni.service.Load(); svc != nil {
		_ = svc.ConsentAnswer(feature.ConsentAnswerFrame{ID: card.id, Answer: answer})
	}
}

// recallCardWaiting is the id of the recall card waiting, or "".
func (d *Daemon) recallCardWaiting() string {
	r := &d.oni.recall
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.card == nil {
		return ""
	}
	return r.card.id
}

// recallCardTake answers the card id names: yes sends what it named to
// the route it named, no drops it. may refuses a caller who cannot answer.
func (d *Daemon) recallCardTake(id string, allow bool, may func(*recallCard) error) (protocol.OniAskResult, error) {
	r := &d.oni.recall
	r.mu.Lock()
	card := r.card
	if card == nil || card.id != id {
		r.mu.Unlock()
		return protocol.OniAskResult{}, oniOfferGone()
	}
	if may != nil {
		if err := may(card); err != nil {
			r.mu.Unlock()
			return protocol.OniAskResult{}, err
		}
	}
	d.recallCardCloseLocked(allow)
	r.mu.Unlock()
	if !allow {
		return protocol.OniAskResult{Status: protocol.OniAskDeclined}, nil
	}
	if err := recallRefusal(protocol.EffectiveOni(d.Settings())); err != nil {
		return protocol.OniAskResult{}, err
	}
	res, err := d.recallSend(card)
	if err != nil {
		return protocol.OniAskResult{}, err
	}
	if svc := d.oni.service.Load(); svc != nil {
		_ = svc.Bubble(0, "answer", oniText(res.Answer, oniBubbleMax))
	}
	return protocol.OniAskResult{Status: protocol.OniAskAnswered, Answer: &protocol.OniAnswer{Text: res.Answer, Route: res.Route, Destination: res.Destination}}, nil
}

// recallCardAnswer serves oni.consent.answer for the recall card: the
// connection that asked, or the app, answers it. It reports false when id
// is not the card.
func (d *Daemon) recallCardAnswer(c *connection, p protocol.OniConsentAnswerParams) (protocol.OniAskResult, bool, error) {
	if !strings.HasPrefix(p.ID, feature.RecallCardPrefix) {
		return protocol.OniAskResult{}, false, nil
	}
	res, err := d.recallCardTake(p.ID, p.Answer == "yes", func(card *recallCard) error {
		if card.conn != c && !c.app {
			return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only whoever asked, or the Hyakki app, answers this question."))
		}
		return nil
	})
	return res, true, err
}

// recallSend asks the planned route alone, pinned, with no tools, and
// clears the passages when it returns.
func (d *Daemon) recallSend(card *recallCard) (protocol.OniRecallResult, error) {
	defer func() { card.fence, card.question = "", "" }()
	l := recallLimits()
	ctx, cancel := context.WithTimeout(context.Background(), l.Request())
	defer cancel()
	req := recallRequest(recall.System(), recall.User(card.fence, card.question))
	req.Model, req.Strict, req.LocalOnly, req.MaxTokens = card.plan.Route(), true, card.plan.Local, l.AnswerTokens
	text, _, err := d.streamText(ctx, req)
	if err != nil {
		return protocol.OniRecallResult{}, noRouteError(err)
	}
	destination := "hosted"
	if card.plan.Local {
		destination = "local"
	}
	answer := strings.TrimSpace(text)
	if answer == "" {
		answer = i18n.T("I found nothing about that in what you kept.")
	}
	return protocol.OniRecallResult{Status: protocol.OniRecallStatusAnswered, Answer: answer, Sources: card.sources, Route: card.plan.Route(), Destination: destination}, nil
}

// streamText streams one request to its end: the reply's text, at most
// oniReplyMax bytes, and how the stream finished.
func (d *Daemon) streamText(ctx context.Context, req llm.Request) (string, llm.Final, error) {
	stream, err := d.deps.Gateway.Stream(ctx, req)
	if err != nil {
		return "", llm.Final{}, err
	}
	defer stream.Close()
	var reply strings.Builder
	for {
		delta, err := stream.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", stream.Final(), err
		}
		if reply.Len()+len(delta.Text) <= oniReplyMax {
			reply.WriteString(delta.Text)
		}
	}
	return reply.String(), stream.Final(), nil
}

// voiceRecall answers a spoken recall question and says the answer, or
// points at the card a hosted route needs.
func (d *Daemon) voiceRecall(q recall.Query, question string, turn uint64, source string, gen uint64) {
	v := &d.oni.voice
	defer func() {
		v.mu.Lock()
		v.work--
		v.mu.Unlock()
		d.stateChanged()
	}()
	settings := protocol.EffectiveOni(d.Settings())
	ctx, cancel := context.WithTimeout(context.Background(), oniAskTimeout)
	defer cancel()
	var line string
	res, err := d.recallAsk(ctx, nil, q, question, settings)
	switch {
	case err != nil:
		var perr *protocol.Error
		line = i18n.T("I could not get an answer.")
		if errors.As(err, &perr) && perr.Message != "" {
			line = perr.Message
		}
	case res.Status == protocol.OniRecallStatusConsent:
		line = i18n.T(lineMemoryCard)
	default:
		line = res.Answer
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if gen != v.gen || !v.turn.Accept(turn) {
		return
	}
	if !d.voiceSayLocked(settings, turn, source, route.Spoken(line, false)) {
		if svc := d.oni.service.Load(); svc != nil {
			_ = svc.Bubble(0, "answer", oniText(line, oniBubbleMax))
		}
	}
	d.voiceRestLocked(turn)
}

// recallSettings follows the settings: with recall off, the card goes
// unanswered and what it held is cleared.
func (d *Daemon) recallSettings(next protocol.Settings) {
	if recallRefusal(protocol.EffectiveOni(next)) == nil {
		return
	}
	r := &d.oni.recall
	r.mu.Lock()
	defer r.mu.Unlock()
	d.recallCardCloseLocked(false)
}

// oniRecallImportCall serves oni.recall.import: the export files the user
// chose are read where they are, their chats kept as Markdown under one
// index root, and the root registered, or refreshed, so recall reads them.
func (d *Daemon) oniRecallImportCall(p protocol.OniRecallImportParams) (protocol.OniRecallImportResult, error) {
	d.oni.recall.importMu.Lock()
	defer d.oni.recall.importMu.Unlock()
	if err := recallRefusal(protocol.EffectiveOni(d.Settings())); err != nil {
		return protocol.OniRecallImportResult{}, err
	}
	root := recallRoot()
	res, err := chats.Import(root, p.Paths)
	out := protocol.OniRecallImportResult{Chats: res.Chats, Skipped: res.Skipped, Sources: res.List(), Root: root}
	if out.Sources == nil {
		out.Sources = []protocol.OniRecallImported{}
	}
	if err != nil {
		switch {
		case errors.Is(err, chats.ErrTraversal):
			out.Refused = "traversal"
		case errors.Is(err, chats.ErrTooBig):
			out.Refused = "too-big"
		case errors.Is(err, chats.ErrFormat):
			out.Refused = "format"
		default:
			out.Refused = "unreadable"
		}
	}
	if res.Chats > 0 && d.deps.Indexer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), oniAskTimeout)
		defer cancel()
		if id := d.recallChatsIndex(ctx); id != "" {
			if _, err := d.deps.Indexer.Refresh(ctx, protocol.IndexRef{ID: id}); err != nil {
				d.warn("oni recall import: refresh: %v", err)
			}
		} else if _, err := d.deps.Indexer.Add(ctx, protocol.IndexAddParams{Path: root, Name: i18n.T("Imported chats")}); err != nil {
			return out, protocol.Errorf(protocol.CodeInternal, "%s", i18n.Tf("The chats were imported but could not be indexed: %v.", err))
		}
	}
	return out, nil
}
