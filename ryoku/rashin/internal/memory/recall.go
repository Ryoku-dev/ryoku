package memory

import (
	"context"
	"strings"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/memory/backend"
	"ryoku-rashin/internal/memory/render"
	"ryoku-rashin/internal/protocol"
)

// Recall bounds: the facts fetched for one Ask turn, the past conversations
// added when the question looks back at one, the facts a brief or an
// engine's card is drawn from, and how long an engine is waited for.
const (
	recallFacts = 8
	recallChats = 2
	briefFacts  = 100
	engineWait  = 1500 * time.Millisecond
)

// recallScopes are the scopes a conversation draws on: the user and its
// project, or only the project when the project keeps to itself.
func recallScopes(scope string, kept bool) []string {
	if kept {
		return []string{scope}
	}
	if scope == "" {
		return []string{protocol.MemoryYou}
	}
	return []string{protocol.MemoryYou, scope}
}

// reader reads memory where it is kept: the built-in store, or the engine
// in use, which is given engineWait in all. failed is set once the engine
// did not answer, and nothing more is asked of it.
type reader struct {
	e      *Engine
	store  *Store
	engine backend.Backend
	ctx    context.Context
	failed bool
}

// read opens a reader; ok is false when the built-in store cannot open.
// Only the engine's calls are bounded by engineWait; cancel ends that wait.
func (e *Engine) read(ctx context.Context) (r *reader, cancel func(), ok bool) {
	waited, cancel := context.WithTimeout(ctx, engineWait)
	engine, err := e.external(waited)
	if err != nil || engine != nil {
		return &reader{e: e, engine: engine, ctx: waited, failed: err != nil}, cancel, true
	}
	store, err := e.store()
	return &reader{e: e, store: store, ctx: ctx}, cancel, err == nil
}

// fail notes that the engine did not answer.
func (r *reader) fail(err error) bool {
	if err != nil {
		r.failed = true
	}
	return err != nil
}

// search finds the facts that bear on text in the scopes, best first.
func (r *reader) search(text string, scopes []string, k int) []protocol.Memory {
	if r.store != nil {
		found, _ := r.store.Search(r.ctx, text, scopes, k)
		return found
	}
	if r.failed {
		return nil
	}
	found, err := r.engine.Recall(r.ctx, backend.Query{Text: text, Scopes: scopes, Limit: k})
	if r.fail(err) {
		return nil
	}
	return found
}

// active lists up to briefFacts facts of one scope, and how many it has.
func (r *reader) active(scope string) ([]protocol.Memory, int) {
	if r.store != nil {
		facts, _ := r.store.Active(scope)
		return facts, len(facts)
	}
	if r.failed {
		return nil, 0
	}
	facts, total, err := r.engine.List(r.ctx, scope, 0, briefFacts)
	if r.fail(err) {
		return nil, 0
	}
	return facts, total
}

// card is the card about the user: the built-in one, or the engine's own,
// or one made from the engine's facts about the user.
func (r *reader) card() string {
	if r.store != nil {
		built, err := r.e.cardGet(r.ctx, false)
		if err != nil {
			return ""
		}
		return built.Text
	}
	if r.failed {
		return ""
	}
	var lines []string
	if r.engine.Caps().Card {
		text, err := r.engine.Card(r.ctx)
		if r.fail(err) {
			return ""
		}
		lines = strings.Split(text, "\n")
	} else {
		facts, _ := r.active(protocol.MemoryYou)
		render.OrderForBrief(facts)
		for _, fact := range facts {
			if !fact.Sensitive {
				lines = append(lines, cardLine(fact.Text))
			}
		}
	}
	var kept []string
	for _, line := range lines {
		if _, hit := scanContext(line); strings.TrimSpace(line) != "" && !hit {
			kept = append(kept, strings.TrimSpace(line))
		}
	}
	return pick(kept, render.Fit(kept, protocol.MemoryCardMax+1))
}

// usable drops the facts a conversation set aside and those that read as an
// instruction.
func usable(facts []protocol.Memory, suppressed map[string]bool) []protocol.Memory {
	var out []protocol.Memory
	for _, fact := range facts {
		if _, hit := scanContext(fact.Text); !hit && !suppressed[fact.ID] {
			out = append(out, fact)
		}
	}
	return out
}

// recall is memory for one Ask turn: the rules and the card for the system
// prompt, and, unless the turn is trivial, the facts for the user message:
// every fact the conversation may read when they all fit the fence, else
// those that bear on the question. A question that looks back, or one no
// fact bears on, also gets the matching lines of past conversations. When
// the engine in use does not answer in time, the turn goes without memory
// and Used says so.
func (e *Engine) recall(ctx context.Context, info protocol.SessionInfo, question string) core.Recall {
	if info.Memory == protocol.MemoryNone {
		return core.Recall{}
	}
	r, cancel, ok := e.read(ctx)
	defer cancel()
	if !ok {
		return core.Recall{}
	}
	settings := e.settings()
	scope, kept := e.sessionScope(info)
	scopes := recallScopes(scope, kept)
	card := ""
	if !kept {
		card = r.card()
	}
	var all []protocol.Memory
	facts := 0
	for _, each := range scopes {
		list, total := r.active(each)
		all = append(all, list...)
		facts += total
	}
	conversations := 0
	if sessions := e.sessionsOf(); settings.Sessions && sessions != nil {
		for _, kind := range []protocol.SessionKind{protocol.KindAsk, protocol.KindAgent} {
			for _, other := range sessions.List(kind) {
				if other.ID != info.ID && searchable(other) {
					conversations++
				}
			}
		}
	}
	recall := core.Recall{
		System:   render.AskSystem(card, true, settings.Sessions, facts, conversations),
		Tools:    true,
		Sessions: settings.Sessions,
	}
	if card != "" {
		recall.Used = &protocol.MemoryEventData{Kind: protocol.MemoryUsed, Card: true}
	}
	var sections []render.Section
	found := 0
	if !trivial(question) {
		suppressed := e.suppressed(info)
		whole := usable(all, suppressed)
		cardOrder(whole, r.explicit(whole))
		sections = e.factSections(scope, whole)
		found = len(whole)
		if _, fenced := render.Fence(sections); facts > len(all) || len(fenced) < len(whole) {
			searched := usable(r.search(question, scopes, recallFacts), suppressed)
			sections, found = e.factSections(scope, searched), len(searched)
		}
	}
	if r.failed {
		recall.Used = &protocol.MemoryEventData{Kind: protocol.MemoryUsed, Text: i18n.T("Memory did not answer in time, so this answer draws on none of it.")}
		return recall
	}
	if trivial(question) {
		return recall
	}
	if settings.Sessions && (recallIntent(question) || found == 0) {
		sections = append(sections, e.recallPast(ctx, info, question)...)
	}
	text, used := render.Fence(sections)
	if text == "" {
		return recall
	}
	recall.Context = text
	var items []protocol.MemoryItem
	var ids []string
	for _, fact := range used {
		if fact.ID == "" {
			continue
		}
		ids = append(ids, fact.ID)
		items = append(items, protocol.MemoryItem{ID: fact.ID, Text: fact.Text, Scope: fact.Scope, ScopeName: fact.ScopeName})
	}
	if len(items) > 0 {
		recall.Used = &protocol.MemoryEventData{Kind: protocol.MemoryUsed, Used: items, Card: card != ""}
		if r.store != nil {
			e.used(info, ids)
		}
	}
	return recall
}

// explicit reports which facts the user stated themselves: from the ledger,
// or for an engine's facts, those the user added.
func (r *reader) explicit(facts []protocol.Memory) map[string]bool {
	if r.store != nil {
		ids := make([]string, len(facts))
		for index, fact := range facts {
			ids[index] = fact.ID
		}
		return r.store.Explicit(ids)
	}
	out := map[string]bool{}
	for _, fact := range facts {
		if fact.Source.Kind == protocol.MemoryFromYou {
			out[fact.ID] = true
		}
	}
	return out
}

// factSections groups facts for the fence: about the user, then the
// conversation's project.
func (e *Engine) factSections(scope string, facts []protocol.Memory) []render.Section {
	sections := []render.Section{{Heading: i18n.T("About you")}}
	if scope != "" {
		sections = append(sections, render.Section{Heading: e.scopeName(scope)})
	}
	for _, fact := range facts {
		at := 0
		if fact.Scope != protocol.MemoryYou {
			at = len(sections) - 1
		}
		sections[at].Facts = append(sections[at].Facts, fact)
	}
	return sections
}

// recallPast is what past conversations say about a question, as fence
// sections with no memory ids: up to chatLines of their scrubbed lines,
// each with its day and conversation's title, then the summaries of up to
// recallChats of them. The search keeps to what the conversation may see.
func (e *Engine) recallPast(ctx context.Context, info protocol.SessionInfo, question string) []render.Section {
	said := render.Section{Heading: i18n.T("Said in earlier conversations")}
	summaries := render.Section{Heading: i18n.T("Earlier conversations")}
	hits, _ := e.findChats(ctx, info, question, time.Time{}, chatLines)
	for _, hit := range hits {
		for _, index := range hit.lines {
			if len(said.Facts) == chatLines {
				break
			}
			line := hit.doc.lines[index]
			at := line.at
			if at.IsZero() {
				at = hit.doc.info.Updated
			}
			said.Facts = append(said.Facts, protocol.Memory{Text: hit.doc.title + ": " + line.show(question), Updated: at})
		}
		if hit.summary != "" && len(summaries.Facts) < recallChats {
			summaries.Facts = append(summaries.Facts, protocol.Memory{Text: hit.doc.title + ": " + hit.summary, Updated: hit.doc.info.Updated})
		}
	}
	return []render.Section{said, summaries}
}

// brief is the memory part of an agent session's brief: the card and the
// project's facts, or "" when agents do not get memory, the session uses
// none, or the engine in use did not answer in time.
func (e *Engine) brief(info protocol.SessionInfo) string {
	if !e.settings().Agents || info.Memory == protocol.MemoryNone || info.Team != "" {
		return ""
	}
	r, cancel, ok := e.read(context.Background())
	defer cancel()
	if !ok {
		return ""
	}
	scope, kept := e.sessionScope(info)
	card := ""
	if !kept {
		card = r.card()
	}
	var facts []protocol.Memory
	name := ""
	if scope != "" {
		facts, _ = r.active(scope)
		facts = usable(facts, nil)
		render.OrderForBrief(facts)
		name = e.scopeName(scope)
	}
	if r.failed {
		return ""
	}
	return render.Brief(card, name, facts)
}

// teamBrief is what a team's members are told the team remembers, "" when
// the team keeps nothing or the engine in use did not answer in time.
func (e *Engine) teamBrief(team protocol.Team) string {
	keeps := team.TeamMemory()
	if !keeps.Remember {
		return ""
	}
	r, cancel, ok := e.read(context.Background())
	defer cancel()
	if !ok {
		return ""
	}
	facts, _ := r.active(protocol.MemoryTeamPrefix + team.ID)
	facts = usable(facts, nil)
	render.OrderForBrief(facts)
	card := ""
	if keeps.ReadsYou {
		card = r.card()
	}
	if r.failed {
		return ""
	}
	text, _ := render.TeamBrief(facts, card, keeps.ReadsYou)
	return text
}
