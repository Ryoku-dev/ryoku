package memory

import (
	"context"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/memory/render"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/rank"
	"ryoku-rashin/internal/redact"
)

// Past-conversation search: how many conversations answer by default and at
// most, how many matching lines each shows, how long a shown line may be,
// and how much of one line is kept to search.
const (
	chatsDefault  = 5
	chatsMax      = 20
	chatLines     = 3
	chatLineChars = 400
	chatLineKept  = rowMax
)

// chatDoc is one past conversation as the search reads it: its info when
// read, up to which event, its title, its lines, and the project it belongs
// to ("" for none).
type chatDoc struct {
	info  protocol.SessionInfo
	seq   int64
	title string
	scope string
	lines []chatLine
}

// chatLine is one line of a past conversation: the user's words or a final
// reply, scrubbed, and the day of its turn.
type chatLine struct {
	reply bool
	text  string
	at    time.Time
}

// show is the line as a search shows it: who said it, and the part of it
// that answers the question when it is too long to show whole. weights come
// from the line's conversation (see weigh).
func (l chatLine) show(weights map[string]float64) string {
	who := i18n.T("User")
	if l.reply {
		who = i18n.T("Reply")
	}
	return who + ": " + excerpt(weights, l.text, chatLineChars)
}

// chatHit is one conversation a search found: its scanned summary and the
// positions of its lines that best match, in order.
type chatHit struct {
	doc     *chatDoc
	summary string
	lines   []int
}

// chatIndex holds the past conversations read so far. A conversation is read
// again only when it has moved on.
type chatIndex struct {
	docs map[string]*chatDoc
}

// drop forgets a conversation, so it is read again or not at all.
func (c *chatIndex) drop(id string) {
	delete(c.docs, id)
}

// searchable reports a conversation the search may read: neither temporary,
// nor kept from memory, nor a team task's.
func searchable(info protocol.SessionInfo) bool {
	return !info.Temporary && info.Memory != protocol.MemoryNone && info.Team == ""
}

// Portions adapted from sipeed/picoclaw pkg/seahorse/short_retrieval.go @
// bbf6893ca7af (MIT, Copyright (c) 2026 PicoClaw contributors). Modified by
// Ryoku: the pattern is compiled once and the error is translated.
var lastPattern = regexp.MustCompile(`^(\d+)([hdwm])$`)

// parseLast reads a span such as 6h, 7d, 2w or 1m (30 days).
func parseLast(s string) (time.Duration, error) {
	matches := lastPattern.FindStringSubmatch(strings.TrimSpace(s))
	if matches == nil {
		return 0, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%q is not a span: use hours, days, weeks or months, such as 6h, 7d, 2w or 1m.", s))
	}
	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%q is not a span: use hours, days, weeks or months, such as 6h, 7d, 2w or 1m.", s))
	}
	switch matches[2] {
	case "h":
		return time.Duration(value) * time.Hour, nil
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "w":
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	default:
		return time.Duration(value) * 30 * 24 * time.Hour, nil
	}
}

// sessionsSearch finds past conversations by their summaries and by what
// was said in them: words with BM25, and the summaries' meaning when the
// embedding model is at hand, fused by rank. in is the conversation that
// asks, zero for the user's own search of every conversation: one in a
// project that keeps to itself finds only that project's conversations, and
// no other finds them. Nothing is found while the search is off.
func (e *Engine) sessionsSearch(ctx context.Context, in protocol.SessionInfo, p protocol.MemorySessionsParams) (protocol.MemorySessionsResult, error) {
	result := protocol.MemorySessionsResult{Sessions: []protocol.MemorySessionHit{}}
	limit := p.Limit
	if limit <= 0 {
		limit = chatsDefault
	}
	var since time.Time
	if p.Last != "" {
		span, err := parseLast(p.Last)
		if err != nil {
			return result, err
		}
		since = e.learn.now().Add(-span)
	}
	hits, err := e.findChats(ctx, in, p.Query, since, min(limit, chatsMax))
	for _, hit := range hits {
		lines := make([]string, len(hit.lines))
		for at, index := range hit.lines {
			lines[at] = render.Neutral(hit.doc.lines[index].show(hit.doc.weigh(p.Query)))
		}
		result.Sessions = append(result.Sessions, protocol.MemorySessionHit{
			Session: hit.doc.info.ID,
			Kind:    hit.doc.info.Kind,
			Title:   render.Neutral(hit.doc.title),
			Updated: hit.doc.info.Updated,
			Summary: render.Neutral(hit.summary),
			Lines:   lines,
		})
	}
	return result, err
}

// findChats is the search behind sessionsSearch: the conversations in may
// see, updated since since, best first.
func (e *Engine) findChats(ctx context.Context, in protocol.SessionInfo, query string, since time.Time, limit int) ([]chatHit, error) {
	query = strings.TrimSpace(query)
	sessions := e.sessionsOf()
	if query == "" || sessions == nil || !e.settings().Sessions {
		return nil, nil
	}
	store, err := e.store()
	if err != nil {
		return nil, err
	}
	notes, err := store.Notes()
	if err != nil {
		return nil, err
	}
	summaries := map[string]ChatNote{}
	for _, note := range notes {
		summaries[note.Session] = note
	}
	docs := e.visibleChats(in, e.learn.chatDocs(e, sessions, since))
	if len(docs) == 0 {
		return nil, nil
	}
	texts := make([]string, len(docs))
	for index, doc := range docs {
		var b strings.Builder
		b.WriteString(summaries[doc.info.ID].Summary + "\n" + doc.title)
		for _, line := range doc.lines {
			b.WriteString("\n" + line.text)
		}
		texts[index] = b.String()
	}
	lists := [][]string{ranked(docs, rank.BM25(query, texts))}
	if vectors, err := store.Embed(ctx, []string{query}); err == nil && len(vectors) == 1 {
		scores := make([]float64, len(docs))
		for index, doc := range docs {
			if note := summaries[doc.info.ID]; len(note.Vector) > 0 {
				scores[index] = rank.Cosine(vectors[0], note.Vector)
			}
		}
		lists = append(lists, ranked(docs, scores))
	}
	byID := map[string]*chatDoc{}
	for _, doc := range docs {
		byID[doc.info.ID] = doc
	}
	var hits []chatHit
	for _, found := range rank.Fuse(limit, lists...) {
		doc := byID[found.ID]
		summary := summaries[doc.info.ID].Summary
		if _, bad := scanContext(summary); bad {
			summary = ""
		}
		hits = append(hits, chatHit{doc: doc, summary: summary, lines: matchingLines(query, doc.lines)})
	}
	return hits, nil
}

// visibleChats keeps the conversations in may find.
func (e *Engine) visibleChats(in protocol.SessionInfo, docs []*chatDoc) []*chatDoc {
	if in.ID == "" {
		return docs
	}
	scope, kept := e.sessionScope(in)
	e.mu.Lock()
	keeps := func(doc *chatDoc) bool { return e.modes[doc.scope] == protocol.ProjectMemoryKept }
	out := docs[:0:0]
	for _, doc := range docs {
		switch {
		case doc.info.ID == in.ID:
		case kept && doc.scope == scope, !kept && !keeps(doc):
			out = append(out, doc)
		}
	}
	e.mu.Unlock()
	return out
}

// ranked lists the ids of the documents that scored above zero, best first.
func ranked(docs []*chatDoc, scores []float64) []string {
	order := make([]int, 0, len(docs))
	for index, score := range scores {
		if score > 0 {
			order = append(order, index)
		}
	}
	sort.SliceStable(order, func(i, j int) bool { return scores[order[i]] > scores[order[j]] })
	out := make([]string, len(order))
	for at, index := range order {
		out[at] = docs[index].info.ID
	}
	return out
}

// matchingLines are the positions of the lines of a conversation that best
// match the query, in order. A question about what the assistant said takes
// the matching replies before the user's lines.
func matchingLines(query string, lines []chatLine) []int {
	texts := make([]string, len(lines))
	for index, line := range lines {
		texts[index] = line.text
	}
	scores := rank.BM25(query, texts)
	replies := asksWhatWasSaid(query)
	order := make([]int, 0, len(lines))
	for index, score := range scores {
		if score > 0 {
			order = append(order, index)
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := lines[order[i]], lines[order[j]]
		if replies && a.reply != b.reply {
			return a.reply
		}
		return scores[order[i]] > scores[order[j]]
	})
	order = order[:min(len(order), chatLines)]
	sort.Ints(order)
	return order
}

// sentence is one sentence of a line with what follows it up to the next.
var sentence = regexp.MustCompile(`[^.!?\n]+(?:[.!?]+|\n|$)\s*`)

// askWords say how a question about a conversation is asked, beside the
// stop words every question shares.
var askWords = map[string]bool{"remind": true, "tell": true, "told": true, "say": true, "said": true, "back": true, "going": true, "previous": true, "earlier": true, "conversation": true}

// terms are the words of text worth matching: its content words, without
// those that only say how a question is asked.
func terms(text string) []string {
	words := contentWords(text)
	out := words[:0]
	for _, word := range words {
		if !askWords[word] {
			out = append(out, word)
		}
	}
	return out
}

// stemPrefix is how many leading runes two words must share to count as the
// same word ("seal" and "sealant").
const stemPrefix = 4

// related reports the same word, or two words sharing a stem.
func related(a, b string) bool {
	if a == b {
		return true
	}
	for shared := 0; a != "" && b != ""; {
		ra, na := utf8.DecodeRuneInString(a)
		rb, nb := utf8.DecodeRuneInString(b)
		if ra != rb {
			return false
		}
		if shared++; shared == stemPrefix {
			return true
		}
		a, b = a[na:], b[nb:]
	}
	return false
}

// mentions reports a word in words related to word.
func mentions(words []string, word string) bool {
	for _, each := range words {
		if related(each, word) {
			return true
		}
	}
	return false
}

// userEcho scales a question word the user's own lines already hold: it
// names what the conversation was about, not what in a reply answers it.
const userEcho = 0.1

// weigh gives each word of query a weight within this conversation: rarer
// across its sentences weighs more, and a word the user said in it weighs
// little. Words the conversation never mentions are left out.
func (d *chatDoc) weigh(query string) map[string]float64 {
	var units [][]string
	var said []string
	for _, line := range d.lines {
		for _, part := range sentence.FindAllString(line.text, -1) {
			unit := terms(part)
			units = append(units, unit)
			if !line.reply {
				said = append(said, unit...)
			}
		}
	}
	weights := map[string]float64{}
	for _, word := range terms(query) {
		found := 0
		for _, unit := range units {
			if mentions(unit, word) {
				found++
			}
		}
		if found == 0 {
			continue
		}
		weight := math.Log(1 + float64(len(units))/float64(1+found))
		if mentions(said, word) {
			weight *= userEcho
		}
		weights[word] = weight
	}
	return weights
}

// excerpt is text cut to max characters around the sentence whose words
// weigh most (see weigh), then the sentence after it, which often holds the
// answer to the setup, then those either side as room allows, with "…"
// where it was cut. Text that fits is returned whole.
func excerpt(weights map[string]float64, text string, max int) string {
	text = strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(text) <= max {
		return text
	}
	parts := sentence.FindAllString(text, -1)
	if len(parts) == 0 {
		return render.Cut(text, max)
	}
	words := make([]string, 0, len(weights))
	for word := range weights {
		words = append(words, word)
	}
	sort.Strings(words)
	best, top := 0, 0.0
	for index, part := range parts {
		said, score := terms(part), 0.0
		for _, word := range words {
			if mentions(said, word) {
				score += weights[word]
			}
		}
		if score > top {
			best, top = index, score
		}
	}
	first, last := best, best
	used := utf8.RuneCountInString(parts[best]) + 2
	for grew := true; grew; {
		grew = false
		if last+1 < len(parts) && used+utf8.RuneCountInString(parts[last+1]) <= max {
			last++
			used += utf8.RuneCountInString(parts[last])
			grew = true
		}
		if first > 0 && used+utf8.RuneCountInString(parts[first-1]) <= max {
			first--
			used += utf8.RuneCountInString(parts[first])
			grew = true
		}
	}
	out := strings.TrimSpace(strings.Join(parts[first:last+1], ""))
	if first == last {
		out = render.Cut(out, max-2)
	}
	if first > 0 {
		out = "…" + out
	}
	if last < len(parts)-1 {
		out += "…"
	}
	return out
}

// chatDocs brings the index up to date with the stored conversations that
// may be searched, updated since since, and returns them. Conversations are
// read outside the learner's lock.
func (l *learner) chatDocs(e *Engine, sessions core.MemorySessions, since time.Time) []*chatDoc {
	var infos []protocol.SessionInfo
	for _, kind := range []protocol.SessionKind{protocol.KindAsk, protocol.KindAgent} {
		infos = append(infos, sessions.List(kind)...)
	}
	l.mu.Lock()
	if l.chats == nil {
		l.chats = &chatIndex{docs: map[string]*chatDoc{}}
	}
	cached := make(map[string]*chatDoc, len(l.chats.docs))
	for id, doc := range l.chats.docs {
		cached[id] = doc
	}
	l.mu.Unlock()
	listed := map[string]bool{}
	var out []*chatDoc
	for _, info := range infos {
		listed[info.ID] = true
		if !searchable(info) || info.Updated.Before(since) {
			continue
		}
		doc := cached[info.ID]
		if doc == nil || doc.seq != info.LastSeq {
			doc = readChat(e, sessions, info)
		} else {
			fresh := *doc
			fresh.info = info
			doc = &fresh
		}
		cached[info.ID] = doc
		out = append(out, doc)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.chats == nil {
		l.chats = &chatIndex{docs: map[string]*chatDoc{}}
	}
	for id, doc := range cached {
		if listed[id] {
			l.chats.docs[id] = doc
		} else {
			l.chats.drop(id)
		}
	}
	return out
}

// readChat reads one conversation's title, project and lines: the user's
// words and the final replies, scrubbed.
func readChat(e *Engine, sessions core.MemorySessions, info protocol.SessionInfo) *chatDoc {
	doc := &chatDoc{info: info, seq: info.LastSeq, title: redact.Strict(stripPrivate(info.Title))}
	doc.scope, _ = e.sessionScope(info)
	events, err := sessions.Events(info.ID)
	if err != nil {
		return doc
	}
	conv := readConversation(events, 0)
	scrubTurns(conv.turns)
	for _, turn := range conv.turns {
		if turn.User != "" {
			doc.lines = append(doc.lines, chatLine{text: cutBytes(turn.User, chatLineKept), at: turn.At})
		}
		if turn.Reply != "" {
			doc.lines = append(doc.lines, chatLine{reply: true, text: cutBytes(turn.Reply, chatLineKept), at: turn.At})
		}
	}
	return doc
}
