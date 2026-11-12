package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/memory/render"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/redact"
)

// cardRewriteEvery is how often at most the model rewrites a card that no
// longer fits, unless the user asks for a rebuild.
const cardRewriteEvery = 24 * time.Hour

// cardReplyTokens bounds the card rewrite's reply.
const cardReplyTokens = 1024

// cardGet is the card, built first when asked or when there is none yet.
func (e *Engine) cardGet(ctx context.Context, rebuild bool) (protocol.MemoryCard, error) {
	store, err := e.store()
	if err != nil {
		return protocol.MemoryCard{}, err
	}
	card, err := store.Card()
	if err != nil || (!rebuild && !card.Built.IsZero()) {
		return card, err
	}
	return e.buildCard(ctx, store, func() learnModel {
		model, _ := e.chooseModel(ctx, e.learn.lastModel())
		return model
	}, rebuild)
}

// lastModel is the model the learner last used, "" for none yet.
func (l *learner) lastModel() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.model.Model
}

// refreshCard rebuilds the card from the facts alone, without a model and
// without any line of the old card, so a fact that was forgotten, replaced
// or edited leaves the card at once. The store calls it after every such
// change to a fact about the user.
func (e *Engine) refreshCard(ctx context.Context, store *Store) error {
	facts, _, err := cardFacts(store)
	if err != nil {
		return err
	}
	lines := make([]string, len(facts))
	for index, fact := range facts {
		lines[index] = cardLine(fact.Text)
	}
	text := pick(lines, render.Fit(lines, protocol.MemoryCardMax+1))
	return store.SetCard(protocol.MemoryCard{Text: text, Built: time.Now(), From: len(facts), Limit: protocol.MemoryCardMax})
}

// cardFacts are the facts the card is made of, in the order it keeps them:
// what the user stated in their own words, then what they pinned, then the
// most used, the most recently used and the newest. Sensitive facts stay out
// of the card, which every conversation and agent is given.
func cardFacts(store *Store) ([]protocol.Memory, map[string]bool, error) {
	active, err := store.Active(protocol.MemoryYou)
	if err != nil {
		return nil, nil, err
	}
	facts := active[:0]
	for _, fact := range active {
		if _, hit := scanContext(fact.Text); !fact.Sensitive && !hit {
			facts = append(facts, fact)
		}
	}
	ids := make([]string, len(facts))
	for index, fact := range facts {
		ids[index] = fact.ID
	}
	explicit := store.Explicit(ids)
	cardOrder(facts, explicit)
	return facts, explicit, nil
}

// cardOrder sorts facts the way the card keeps them: what the user stated in
// their own words, then what they pinned, then the most used, the most
// recently used and the newest.
func cardOrder(facts []protocol.Memory, explicit map[string]bool) {
	sort.SliceStable(facts, func(i, j int) bool {
		a, b := facts[i], facts[j]
		if explicit[a.ID] != explicit[b.ID] {
			return explicit[a.ID]
		}
		if a.Pinned != b.Pinned {
			return a.Pinned
		}
		if a.Uses != b.Uses {
			return a.Uses > b.Uses
		}
		if lastA, lastB := usedAt(a), usedAt(b); !lastA.Equal(lastB) {
			return lastA.After(lastB)
		}
		return a.Updated.After(b.Updated)
	})
}

// usedAt is when a fact was last used, zero when never.
func usedAt(m protocol.Memory) time.Time {
	if m.LastUsed == nil {
		return time.Time{}
	}
	return *m.LastUsed
}

// cardLine is one fact as the card writes it.
func cardLine(text string) string {
	return "- " + strings.Join(strings.Fields(text), " ")
}

// Portions adapted from openai/codex codex-rs/memories/write/src/workspace.rs
// @ b75ec48a2c90 (Apache-2.0, Copyright 2025 OpenAI). Modified by Ryoku: the
// card's shape is one fact per line under the character cap, and nothing
// replaced, forgotten, secret or instruction-like may be in it.

// validCard reports a card fit to keep.
func validCard(ctx context.Context, store *Store, text string, replaced []protocol.Memory) bool {
	if text == "" || utf8.RuneCountInString(text) > protocol.MemoryCardMax || redact.Strict(text) != text {
		return false
	}
	if _, hit := scanStrict(text); hit {
		return false
	}
	gone := map[string]bool{}
	for _, fact := range replaced {
		gone[normalFact(fact.Text)] = true
	}
	for _, line := range strings.Split(text, "\n") {
		fact, ok := strings.CutPrefix(line, "- ")
		if !ok || strings.TrimSpace(fact) == "" || gone[normalFact(fact)] {
			return false
		}
		if forgotten, err := store.Tombstoned(ctx, fact, protocol.MemoryYou); err != nil || forgotten {
			return false
		}
	}
	return true
}

// buildCard builds the card and keeps it. It is assembled in order under the
// cap without a model. Only when the facts no longer fit, and at most once a
// day unless rebuild, does model rewrite it; until then, and when the rewrite
// is not fit to keep, the old card and the newest material share the room.
func (e *Engine) buildCard(ctx context.Context, store *Store, choose func() learnModel, rebuild bool) (protocol.MemoryCard, error) {
	facts, explicit, err := cardFacts(store)
	if err != nil {
		return protocol.MemoryCard{}, err
	}
	old, _ := store.Card()
	lines := make([]string, len(facts))
	for index, fact := range facts {
		lines[index] = cardLine(fact.Text)
	}
	picked := render.Fit(lines, protocol.MemoryCardMax+1)
	text := pick(lines, picked)
	if len(picked) < len(lines) {
		replaced, _ := store.Replaced(protocol.MemoryYou)
		rewritten := ""
		if model := choose(); model.Model != "" && e.learn.mayRewrite(rebuild) {
			rewritten = e.rewriteCard(ctx, model, old.Text, facts, explicit, replaced)
		}
		switch {
		case rewritten != "" && validCard(ctx, store, rewritten, replaced):
			text = rewritten
		case validCard(ctx, store, old.Text, replaced):
			text = shareCard(old.Text, lines)
		}
	}
	card := protocol.MemoryCard{Text: text, Built: time.Now(), From: len(facts), Limit: protocol.MemoryCardMax}
	if text == old.Text && !rebuild && !old.Built.IsZero() {
		return old, nil
	}
	return card, store.SetCard(card)
}

// pick joins the lines at the positions given.
func pick(lines []string, positions []int) string {
	kept := make([]string, len(positions))
	for at, index := range positions {
		kept[at] = lines[index]
	}
	return strings.Join(kept, "\n")
}

// Portions adapted from HKUDS/nanobot nanobot/agent/memory.py @ 9f7ae6fb7b60
// (MIT, Copyright (c) 2025-present Xubin Ren and the nanobot contributors).
// Modified by Ryoku: counts characters, and works on card lines.

// shareCard gives half the card's room to the old card and the rest to the
// facts in order, so neither crowds the other out.
func shareCard(old string, lines []string) string {
	previous := strings.Split(old, "\n")
	kept := pick(previous, render.Fit(previous, protocol.MemoryCardMax/2))
	seen := map[string]bool{}
	for _, line := range strings.Split(kept, "\n") {
		seen[line] = true
	}
	var fresh []string
	for _, line := range lines {
		if !seen[line] {
			fresh = append(fresh, line)
		}
	}
	room := protocol.MemoryCardMax - utf8.RuneCountInString(kept)
	added := pick(fresh, render.Fit(fresh, room))
	if added == "" || utf8.RuneCountInString(kept)+1+utf8.RuneCountInString(added) > protocol.MemoryCardMax {
		return kept
	}
	return kept + "\n" + added
}

// mayRewrite reports whether the model may rewrite the card now, and if so
// counts this as the day's rewrite.
func (l *learner) mayRewrite(rebuild bool) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if !rebuild && !l.rewrote.IsZero() && now.Sub(l.rewrote) < cardRewriteEvery {
		return false
	}
	l.rewrote = now
	return true
}

// rewriteCard asks the model for a card that fits, "" when it fails.
func (e *Engine) rewriteCard(ctx context.Context, model learnModel, old string, facts []protocol.Memory, explicit map[string]bool, replaced []protocol.Memory) string {
	var list strings.Builder
	for _, fact := range facts {
		list.WriteString(cardLine(fact.Text))
		switch {
		case explicit[fact.ID]:
			list.WriteString(" (stated)")
		case fact.Pinned:
			list.WriteString(" (pinned)")
		}
		list.WriteString("\n")
	}
	var gone strings.Builder
	for _, fact := range replaced {
		gone.WriteString(cardLine(fact.Text) + "\n")
	}
	if strings.TrimSpace(old) == "" {
		old = "(empty)"
	}
	none := func(text string) string {
		if text = strings.TrimSpace(text); text == "" {
			return "(none)"
		}
		return text
	}
	zero := 0.0
	reply, _, err := complete(ctx, e.deps.Client, llm.Request{
		Model:       model.Model,
		Strict:      true,
		Temperature: &zero,
		MaxTokens:   cardReplyTokens,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: cardPrompt},
			{Role: llm.RoleUser, Content: fmt.Sprintf(cardUser, old, none(list.String()), none(gone.String()))},
		},
	})
	if err != nil {
		return ""
	}
	var lines []string
	for _, line := range strings.Split(stripThink(reply), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
