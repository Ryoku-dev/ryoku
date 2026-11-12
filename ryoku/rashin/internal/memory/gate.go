package memory

import (
	"regexp"
	"strings"
)

// Portions adapted from NousResearch/hermes-agent agent/memory_provider.py @
// fe6649291e60 (MIT, Copyright (c) 2025 Nous Research). Modified by Ryoku:
// RE2 escapes for the quote and dash characters.
var trivialPrompt = regexp.MustCompile(`(?i)^(?:yes|no|ok|okay|sure|thanks|thank you|y|n|yep|nope|yeah|nah|` +
	`hi|hey|hello|yo|sup|` +
	`continue|go ahead|do it|proceed|got it|cool|nice|great|done|next|lgtm|k)` +
	`[\s!?.:;,"'~\x{2018}\x{2019}\x{201c}\x{201d}\x{2014}\x{2013}\x{2026}()\[\]{}<>*&^%$#@+=` + "`" + `\x{00a0}]*$`)

// trivial reports a turn with nothing to recall for or learn from: empty
// text, a slash command, or a bare greeting or acknowledgement.
func trivial(text string) bool {
	text = strings.TrimSpace(text)
	return text == "" || strings.HasPrefix(text, "/") || trivialPrompt.MatchString(text)
}

// Portions adapted from openclaw/openclaw extensions/active-memory/escalation.ts
// @ 3978ab104ee6 (MIT, Copyright (c) 2026 OpenClaw Foundation). Modified by
// Ryoku: the lookaround patterns are rewritten for RE2, with the "what ... did
// we" form checked in Go; the last four patterns, for questions about what
// the user did or said, are Ryoku's own.
var recallIntentPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(?:previously|earlier|last time|used to)\b`),
	regexp.MustCompile(`(?i)\b(?:do|can|could|would)\s+you\s+(?:remember|recall)\b`),
	regexp.MustCompile(`(?i)\b(?:remember|recall)\s+(?:when|what|which|who|where|why|how)\b`),
	regexp.MustCompile(`(?i)\b(?:we|you|i)\s+(?:discussed|decided|agreed|said|talked about|chose)\b`),
	regexp.MustCompile(`(?i)\b(?:previous|earlier|past)\s+(?:decision|conversation|chat|discussion)\b`),
	regexp.MustCompile(`(?i)\b(?:yesterday|the other day|last (?:week|month|year)|(?:\d+|one|two|three|four|five|six|seven|eight|nine|ten)\s+(?:days?|weeks?|months?|years?)\s+ago)\b`),
	regexp.MustCompile(`(?i)\bwhat (?:do|did) i usually\b`),
	regexp.MustCompile(`(?i)\b(?:what|which|when|where|why|how)\s+(?:did|have|had)\s+(?:we|you|i)\s+(?:decide|choose|discuss|agree|say|mention|talk|use|do)\b`),
	regexp.MustCompile(`(?i)\b(?:did|have|had)\s+(?:we|you|i)\s+(?:decide|choose|discuss|agree|say|mention|talk)\b`),
	regexp.MustCompile(`(?i)\b(?:conversation|chat|discussion)s?\s+(?:from|in|during)\s+(?:january|february|march|april|may|june|july|august|september|october|november|december|\d{4})\b`),
	regexp.MustCompile(`(?i)\b(?:summarize|review|find|search)\s+(?:my|our|the)?\s*(?:past|previous|earlier)?\s*(?:conversation|chat|discussion)s?\b`),
	regexp.MustCompile(`(?i)(?:¿?qué\s+(?:decidimos|hablamos)|\b(?:recuerdas|recordar|anteriormente|ayer)\b|(?:^|[^\pL\pN_])última vez(?:$|[^\pL\pN_]))`),
	regexp.MustCompile(`(?i)\bremind\s+(?:me|us)\s+(?:what|which|who|where|when|why|how)\b`),
	regexp.MustCompile(`(?i)(?:^|[.?!,;]\s*|\b(?:when|where|why|how|and|so|but)\s+)did\s+i\s+\pL+`),
	regexp.MustCompile(`(?i)\bhow\s+(?:many|much|often)\b(?:\s+[\pL\pN'’_-]+){0,4}\s+(?:did|have|had)\s+i\b`),
	regexp.MustCompile(`(?i)\bwhat\s+did\s+i\s+(?:tell|say|ask|mention|write|send)\b`),
	regexp.MustCompile(`(?i)\blast\s+time\s+i\b`),
}

// whatDidWe is "what|which, up to four words, did we|you|i": the pronouns may
// not be among the words, which RE2 cannot say, so the words are checked
// after the match.
var whatDidWe = regexp.MustCompile(`(?i)\b(?:what|which)((?:\s+[\pL\pN'’_-]+){0,4})\s+did\s+(?:we|you|i)(?:$|[^\pL\pN_])`)

// recallIntent reports a question that looks back at earlier conversations,
// so recall reaches past the card and the conversations are worth searching.
func recallIntent(text string) bool {
	for _, pattern := range recallIntentPatterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	for _, match := range whatDidWe.FindAllStringSubmatch(text, -1) {
		pronoun := false
		for _, word := range strings.Fields(match[1]) {
			switch strings.ToLower(word) {
			case "we", "you", "i":
				pronoun = true
			}
		}
		if !pronoun {
			return true
		}
	}
	return false
}
