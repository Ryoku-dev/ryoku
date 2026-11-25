package memory

import (
	"regexp"
	"sort"
	"strings"

	"ryoku-rashin/internal/redact"
)

// Portions adapted from Gentleman-Programming/engram internal/store/store.go @
// a5d4fa7be7f7 (MIT, Copyright (c) 2026 Alan Buscaglia). Modified by Ryoku:
// keeps the text's surrounding whitespace.
var privateTag = regexp.MustCompile(`(?is)<private>.*?</private>`)

// stripPrivate takes out what the user wrapped in <private>, before anything
// else sees the text.
func stripPrivate(text string) string {
	return privateTag.ReplaceAllString(text, redact.Mark)
}

// Portions adapted from HKUDS/nanobot nanobot/utils/helpers.py @ 9f7ae6fb7b60
// (MIT, Copyright (c) 2025-present Xubin Ren and the nanobot contributors).
// Modified by Ryoku: the named back-reference is three alternations and the
// lookahead a character class check.
var (
	thinkBlock     = regexp.MustCompile(`<think>[\s\S]*?</think>|<thinking>[\s\S]*?</thinking>|<thought>[\s\S]*?</thought>`)
	thinkOpenToEnd = regexp.MustCompile(`^\s*<(?:think|thinking|thought)>[\s\S]*$`)
	thinkSelfStart = regexp.MustCompile(`^\s*<thinking/>\s*`)
	thinkSelfEnd   = regexp.MustCompile(`\s*<thinking/>\s*$`)
	thinkMalformed = regexp.MustCompile(`<(?:thinking|thought|think)([^A-Za-z0-9_\-:>/]|$)`)
	thinkCloseHead = regexp.MustCompile(`^\s*</(?:think|thinking|thought)>\s*`)
	thinkCloseTail = regexp.MustCompile(`\s*</(?:think|thinking|thought)>\s*$`)
	channelHead    = regexp.MustCompile(`^\s*<\|?channel\|?>\s*`)
	partialTail    = regexp.MustCompile(`(?:</?(?:thinking|thought|thinkin|thinki|think|thin|thoug|thou|tho|thi|th|t)>?|<\|?(?:channel|channe|chann|chan|cha|ch|c)(?:\|?>?)?)$`)
	openTail       = regexp.MustCompile(`^\s*<\|?$`)
)

// stripThink removes reasoning blocks, unclosed or malformed think tags and
// channel markers a model left in a reply. Markers mid-text are left alone
// so prose that mentions them survives.
func stripThink(text string) string {
	if !strings.Contains(text, "<") {
		return strings.TrimSpace(text)
	}
	text = thinkBlock.ReplaceAllString(text, "")
	text = thinkOpenToEnd.ReplaceAllString(text, "")
	text = thinkSelfStart.ReplaceAllString(text, "")
	text = thinkSelfEnd.ReplaceAllString(text, "")
	text = thinkMalformed.ReplaceAllString(text, "$1")
	text = thinkCloseHead.ReplaceAllString(text, "")
	text = thinkCloseTail.ReplaceAllString(text, "")
	text = channelHead.ReplaceAllString(text, "")
	text = partialTail.ReplaceAllString(text, "")
	text = openTail.ReplaceAllString(text, "")
	return strings.TrimSpace(text)
}

// Portions adapted from NousResearch/hermes-agent agent/memory_manager.py @
// fe6649291e60 (MIT, Copyright (c) 2025 Nous Research). Modified by Ryoku:
// a scrubber value that lives for one hand-off.
var tokenShape = regexp.MustCompile(`[A-Za-z0-9_\-./+]{16,}`)

// scrubber runs redact.Strict over every text of one hand-off and remembers
// each token it took out, so a secret caught once, say in an assignment, is
// also caught where the conversation repeats it bare.
type scrubber struct {
	learned map[string]bool
}

func newScrubber() *scrubber { return &scrubber{learned: map[string]bool{}} }

// strict takes out private spans and secrets, learning the tokens removed.
func (s *scrubber) strict(text string) string {
	text = stripPrivate(text)
	out := redact.Strict(text)
	if out != text {
		left := map[string]bool{}
		for _, token := range tokenShape.FindAllString(out, -1) {
			left[token] = true
		}
		for _, token := range tokenShape.FindAllString(text, -1) {
			if !left[token] {
				s.learned[token] = true
			}
		}
	}
	return out
}

// mask takes out every token learned so far.
func (s *scrubber) mask(text string) string {
	if len(s.learned) == 0 {
		return text
	}
	tokens := make([]string, 0, len(s.learned))
	for token := range s.learned {
		tokens = append(tokens, token)
	}
	sort.Slice(tokens, func(i, j int) bool { return len(tokens[i]) > len(tokens[j]) })
	pairs := make([]string, 0, 2*len(tokens))
	for _, token := range tokens {
		pairs = append(pairs, token, redact.Mark)
	}
	return strings.NewReplacer(pairs...).Replace(text)
}

// scrubTexts scrubs a hand-off in order: private spans, then redact.Strict,
// then every token learned anywhere in it. Texts are changed in place.
func scrubTexts(texts []*string) {
	s := newScrubber()
	for _, text := range texts {
		*text = s.strict(*text)
	}
	for _, text := range texts {
		*text = s.mask(*text)
	}
}
