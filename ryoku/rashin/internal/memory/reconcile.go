package memory

import (
	"context"
	"errors"
	"log"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"ryoku-rashin/internal/memory/render"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/redact"
)

// errTooMuchReplaced is a learning run that would replace more of what is
// stored than one conversation can justify.
var errTooMuchReplaced = errors.New("memory: the model would replace too many facts at once")

// reconcile turns what the learner proposed into the ops to apply, keeping
// only what passes every check. It is built per run.
type reconcile struct {
	// shown are the existing facts the model saw, under ids "0".."n-1".
	shown []protocol.Memory
	// project is the conversation's project scope, "" for none; kept says
	// the project keeps to itself, so no personal fact is learned there.
	project string
	kept    bool
	// words are the user's own scrubbed words in the conversation.
	words string
	// sensitive keeps facts about sensitive topics.
	sensitive bool
	// active counts the facts the model could have replaced.
	active int
	// suggest makes every op a suggestion for the user to keep.
	suggest bool
	source  protocol.MemorySource
	store   *Store
	// observed is when the conversation took place, which a relative date
	// left in a fact counts from.
	observed time.Time
}

// Portions adapted from mem0ai/mem0 mem0/memory/main.py @ f1791daa86d4
// (Apache-2.0, Copyright 2023 Taranjeet Singh and the mem0ai/mem0
// contributors): short ids for the existing facts. Modified by Ryoku: the
// reverse lookup upstream never wrote; an unknown id drops its op.

// listShown writes the existing facts for the learner's prompt.
func listShown(shown []protocol.Memory, project string) string {
	if len(shown) == 0 {
		return "(none)"
	}
	var b strings.Builder
	for index, fact := range shown {
		kind := "personal"
		if fact.Scope != protocol.MemoryYou {
			kind = "project"
		}
		b.WriteString("[" + strconv.Itoa(index) + "] (" + kind + ") " + strings.Join(strings.Fields(fact.Text), " ") + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// resolve turns a short id back into the fact it stands for.
func (r *reconcile) resolve(id string) (protocol.Memory, bool) {
	index, err := strconv.Atoi(strings.Trim(strings.TrimSpace(id), "[]"))
	if err != nil || index < 0 || index >= len(r.shown) {
		return protocol.Memory{}, false
	}
	return r.shown[index], true
}

// Portions adapted from openclaw/openclaw
// extensions/memory-core/src/dreaming-consolidation.ts @ 3978ab104ee6 (MIT,
// Copyright (c) 2026 OpenClaw Foundation): one op per prior entry, no
// supersede across scopes, and the cap on how much one run may replace.
// Portions adapted from vectorize-io/hindsight
// hindsight-api-slim/hindsight_api/engine/consolidation/consolidator.py @
// 7cec128b8fdb (MIT, Copyright (c) 2025 Vectorize AI, Inc.): defences against
// invented ids. Modified by Ryoku: Go checks over Rashin's ops; a single
// replace always passes the cap, which would otherwise block the first
// correction of a small store.

// ops validates the reply's operations in order and returns those to apply.
// It fails when the run would replace more than a quarter of the facts.
func (r *reconcile) ops(ctx context.Context, reply learnReply) ([]Op, error) {
	var out []Op
	replaced := map[string]bool{}
	seen := map[string]bool{}
	for index, proposed := range reply.Ops {
		op, reason := r.check(ctx, proposed, replaced, seen)
		if reason != "" {
			log.Printf("memory: learning op %d of %s dropped: %s", index, r.source.Session, reason)
			continue
		}
		out = append(out, op)
	}
	if n := len(replaced); n > 1 && n*4 > r.active {
		return nil, errTooMuchReplaced
	}
	return out, nil
}

// factProblem says why a text cannot be a fact, "" when it can: it is
// empty or too long, holds a secret, is shaped like markup, repeats
// memory's own framing or reads as an instruction to a model.
func factProblem(text string) string {
	switch {
	case text == "":
		return "empty"
	case utf8.RuneCountInString(text) > protocol.MemoryTextMax:
		return "too long"
	case redact.Strict(text) != text:
		return "holds a secret"
	case !wellFormed(text):
		return "not a fact"
	case contaminated(text):
		return "repeats memory's own text"
	}
	if reason, hit := scanStrict(text); hit {
		return "threat scan: " + reason
	}
	return ""
}

// check validates one proposed op, or says why it is dropped.
func (r *reconcile) check(ctx context.Context, proposed learnOp, replaced, seen map[string]bool) (Op, string) {
	if proposed.Op != "add" && proposed.Op != "supersede" {
		return Op{}, "no change"
	}
	text := strings.Join(strings.Fields(proposed.Text), " ")
	if problem := factProblem(text); problem != "" {
		return Op{}, problem
	}
	switch {
	case redact.Strict(proposed.Evidence) != proposed.Evidence:
		return Op{}, "holds a secret"
	case proposed.Sensitive && !r.sensitive:
		return Op{}, "sensitive"
	case !r.grounded(proposed.Evidence, text):
		return Op{}, "evidence is not the user's words"
	}
	scope := ""
	switch proposed.Scope {
	case "personal":
		if r.kept {
			return Op{}, "personal fact in a project that keeps to itself"
		}
		scope = protocol.MemoryYou
	case "project":
		if r.project == "" {
			return Op{}, "project fact outside a project"
		}
		scope = r.project
	default:
		return Op{}, "unknown scope"
	}
	key := normalFact(text) + "\x00" + scope
	if seen[key] {
		return Op{}, "repeated in the reply"
	}
	var priors []string
	for _, id := range proposed.Supersedes {
		prior, ok := r.resolve(id)
		if !ok {
			return Op{}, "unknown id " + strconv.Quote(id)
		}
		if prior.Scope != scope {
			return Op{}, "replaces across scopes"
		}
		if replaced[prior.ID] {
			return Op{}, "replaces a fact another op replaces"
		}
		if normalFact(prior.Text) == normalFact(text) {
			return Op{}, "replaces a fact with itself"
		}
		priors = append(priors, prior.ID)
	}
	if proposed.Op == "supersede" && len(priors) == 0 {
		return Op{}, "replace without a fact to replace"
	}
	if proposed.Op == "add" && len(priors) > 0 {
		return Op{}, "add that replaces"
	}
	if gone, err := r.store.Tombstoned(ctx, text, scope); err != nil || gone {
		return Op{}, "forgotten before"
	}
	// a richer text of the fact it replaces may read as a near repeat of it
	if id, dup, err := r.store.Duplicate(ctx, text, scope); err == nil && dup && !slices.Contains(priors, id) {
		return Op{}, "already stored"
	}
	seen[key] = true
	for _, id := range priors {
		replaced[id] = true
	}
	op := Op{
		Kind:      OpAdd,
		Text:      asOf(text, r.observed),
		Scope:     scope,
		Retain:    retention(proposed.Kind),
		Sensitive: proposed.Sensitive,
		Explicit:  proposed.Explicit,
		Source:    r.source,
	}
	if len(priors) > 0 {
		op.Kind, op.ID, op.Replaces = OpReplace, priors[0], priors[1:]
	}
	if r.suggest {
		op.Kind, op.ID, op.Replaces = OpSuggest, "", priors
	}
	return op, ""
}

// relativeTime is English wording that counts from the day it was said.
var relativeTime = regexp.MustCompile(`(?i)\b(?:ago|yesterday|today|tomorrow|tonight|recently|the other day|(?:last|next|this)\s+(?:week|month|year|weekend|monday|tuesday|wednesday|thursday|friday|saturday|sunday))\b`)

// asOf dates a fact that still says when relative to the day it was said,
// "(as of 5 May 2023)", so it reads right on a later day. The fact is cut,
// never the note, to keep within protocol.MemoryTextMax.
func asOf(text string, observed time.Time) string {
	if observed.IsZero() || !relativeTime.MatchString(text) {
		return text
	}
	note := " (as of " + observed.Local().Format("2 January 2006") + ")"
	return render.Cut(text, protocol.MemoryTextMax-utf8.RuneCountInString(note)) + note
}

// retention maps the learner's kind onto how long a memory is meant to
// last; a correction lasts as long as what it corrects usually does.
func retention(kind string) string {
	switch kind {
	case "permanent":
		return protocol.MemoryPermanent
	case "ephemeral":
		return protocol.MemoryEphemeral
	default:
		return protocol.MemoryDurable
	}
}

// Evidence limits: a quote shorter than this says too little to tie a fact
// to the user's words.
const (
	evidenceWords = 3
	evidenceChars = 12
)

// plainWords are too common to tie evidence to a fact.
var plainWords = map[string]bool{"the": true, "and": true, "for": true, "you": true, "user": true, "with": true, "that": true, "this": true, "are": true, "was": true, "but": true, "not": true}

// grounded reports evidence long enough to mean something, found in the
// user's own words (compared without case, punctuation or spacing), that
// shares a word with the fact it supports.
func (r *reconcile) grounded(evidence, fact string) bool {
	evidence = normalFact(evidence)
	if len(strings.Fields(evidence)) < evidenceWords && utf8.RuneCountInString(evidence) < evidenceChars {
		return false
	}
	if !strings.Contains(normalFact(r.words), evidence) {
		return false
	}
	if hasWide(evidence) {
		return strings.ContainsFunc(evidence, func(c rune) bool { return unicode.IsLetter(c) && strings.ContainsRune(fact, c) })
	}
	inFact := map[string]bool{}
	for _, word := range strings.Fields(normalFact(fact)) {
		inFact[word] = true
	}
	for _, word := range strings.Fields(evidence) {
		if utf8.RuneCountInString(word) >= 3 && !plainWords[word] && inFact[word] {
			return true
		}
	}
	return false
}

// normalFact folds text to lower-case letters and digits separated by single
// spaces.
func normalFact(text string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(text), func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	}), " ")
}

// Portions adapted from openclaw/openclaw extensions/memory-lancedb/memory-policy.ts
// @ 3978ab104ee6 (MIT, Copyright (c) 2026 OpenClaw Foundation). Modified by
// Ryoku: Rashin's fence; no capture triggers, since the model chose the fact.
var pictographs = regexp.MustCompile(`[\x{1F300}-\x{1F9FF}]`)

// wellFormed rejects what is shaped like markup or a dump rather than a
// one-line fact.
func wellFormed(text string) bool {
	switch {
	case utf8.RuneCountInString(text) < 10 && !hasWide(text):
		return false
	case strings.Contains(text, "<memory-context>"), strings.HasPrefix(text, "<") && strings.Contains(text, "</"):
		return false
	case strings.Contains(text, "**") && strings.Contains(text, "\n-"):
		return false
	case len(pictographs.FindAllString(text, -1)) > 3:
		return false
	}
	return true
}

// hasWide reports CJK text, where a fact may be short.
func hasWide(text string) bool {
	for _, c := range text {
		if unicode.In(c, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			return true
		}
	}
	return false
}

// Portions adapted from openclaw/openclaw
// extensions/memory-core/src/short-term-promotion-utils.ts @ 3978ab104ee6
// (MIT, Copyright (c) 2026 OpenClaw Foundation). Modified by Ryoku: the
// markers of Rashin's own prompts, fence, card and brief.
var ownText = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^(?:[-*+]\s*)?(?:user|assistant|human user|assistant final):\s`),
	regexp.MustCompile(`(?i)\[(?:human user|assistant final)\]`),
	regexp.MustCompile(`(?i)\b(?:Assistant question|Human reply):`),
	regexp.MustCompile(`(?i)recalled memory context|<user_card|<memory_metadata>|Context from Rashin`),
	regexp.MustCompile(`(?i)\b(?:Existing facts|Conversation context|Observation date|DO NOT RESTORE)\s*:`),
	regexp.MustCompile(`(?i)\[\d+\]\s*\((?:personal|project)\)`),
}

// contaminated reports a fact that repeats memory's own prompt or transcript
// framing instead of something the user said.
func contaminated(text string) bool {
	for _, pattern := range ownText {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}
