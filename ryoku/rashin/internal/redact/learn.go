package redact

import (
	"regexp"
	"slices"
	"strings"
	"sync"
)

// tokenPrefix is the prefix that names a token secretToken matches, longest
// first, so an OpenAI project key's body starts after sk-proj-.
var tokenPrefix = regexp.MustCompile(`(?i)^(?:sk-(?:proj-|ant-(?:api\d+-)?)?|gsk_|hf_|github_pat_|gh[oprsu]_|glpat-|[sr]k_live_|AIza|xapp-|npm_|AKIA|xox[a-z]-)`)

// minLearnedBody is the shortest token body a Learner remembers on its own:
// a shorter one would match ordinary words.
const minLearnedBody = 16

// Learner remembers the tokens the patterns found in one conversation, so
// that a model restating one later, its prefix left off or its pieces
// streamed apart, is still scrubbed. It holds each token whole and its body
// after the prefix when the body is at least 16 characters long. It keeps
// them in memory only, for as long as its owner lives; nothing it holds is
// ever written or logged. The zero value is ready to use.
type Learner struct {
	mu       sync.Mutex
	literals []string
}

// Wrap returns scrub with the learner around it: each text goes through
// scrub, and then has every learned token replaced with Mark, longest
// first, so a token scrub finds whole is never cut by a learned body. Each
// token scrub took out of the text is learned, and taken out of the rest of
// the same text too.
func (l *Learner) Wrap(scrub func(string) string) func(string) string {
	return func(text string) string {
		if text == "" {
			return text
		}
		scrubbed := scrub(text)
		l.learn(text, scrubbed)
		return l.replace(scrubbed)
	}
}

// Len is how many literals the learner holds.
func (l *Learner) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.literals)
}

// learn remembers every token of text that scrub found, so that it is not
// in out, and reports whether it learned anything new.
func (l *Learner) learn(text, out string) bool {
	var found []string
	for _, match := range append(secretToken.FindAllString(text, -1), jwtToken.FindAllString(text, -1)...) {
		if strings.Contains(out, match) {
			continue
		}
		found = append(found, match)
		if prefix := tokenPrefix.FindString(match); prefix != "" && len(match)-len(prefix) >= minLearnedBody {
			found = append(found, match[len(prefix):])
		}
	}
	if len(found) == 0 {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	// a new slice, so a replace reading the old one never sees it change
	next := slices.Clone(l.literals)
	for _, literal := range found {
		if !slices.Contains(next, literal) {
			next = append(next, literal)
		}
	}
	if len(next) == len(l.literals) {
		return false
	}
	slices.SortStableFunc(next, func(a, b string) int { return len(b) - len(a) })
	l.literals = next
	return true
}

// replace puts Mark in place of every learned literal in text.
func (l *Learner) replace(text string) string {
	l.mu.Lock()
	literals := l.literals
	l.mu.Unlock()
	for _, literal := range literals {
		text = strings.ReplaceAll(text, literal, Mark)
	}
	return text
}
