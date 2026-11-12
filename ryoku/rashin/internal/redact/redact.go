// Package redact is Rashin's one scrubber for secrets in text. Exact takes
// out what is certainly a secret: the current value of every credential a
// registered store holds, and tokens whose shape gives them away. Strict
// also takes out what only looks like one: the value of an assignment to a
// secret-sounding name, a password inside a URL and a bearer header. Both
// keep the text's line count, so a redacted file still lines up with its
// line numbers.
//
// The stores register at startup. A store that cannot open its values, such
// as the vault while its key is locked away, scrubs nothing, and the patterns
// still apply.
package redact

import (
	"regexp"
	"strings"
	"sync"
)

// Mark is what a secret found by its shape is replaced with. A stored value
// is replaced with its store's name for it instead.
const Mark = "[REDACTED]"

// PrivateKeyMark replaces each line of a private key block.
const PrivateKeyMark = "[REDACTED PRIVATE KEY]"

var (
	secretName        = `[A-Za-z0-9_.-]*(?:password|passwd|pwd|secret|token|api[-_]?key|apikey|access[-_]?key|auth|credential|private[-_]?key)[A-Za-z0-9_.-]*`
	quotedSecretValue = `(?:"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*')`
	secretAssignment  = regexp.MustCompile(`(?i)(^|[\s{,;])(["']?` + secretName + `["']?\s*(?::|=)\s*)(` + quotedSecretValue + `|[^\s,;}]+)`)
	secretWhitespace  = regexp.MustCompile(`(?i)^(\s*(?:export\s+)?["']?` + secretName + `["']?\s+)(` + quotedSecretValue + `|\S+)`)
	fishSecret        = regexp.MustCompile(`(?i)^(\s*set\s+(?:-[A-Za-z]+\s+)*["']?` + secretName + `["']?\s+)(` + quotedSecretValue + `|\S+)`)
	urlCredential     = regexp.MustCompile(`(?i)([A-Za-z][A-Za-z0-9+.-]*://[^\s/:@]+:)[^\s@/]+(@)`)
	bearerToken       = regexp.MustCompile(`(?i)\bBearer\s+\S+`)
	jwtToken          = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`)
	secretToken       = regexp.MustCompile(`(?i)\b(?:sk-[A-Za-z0-9_-]{8,}|gsk_[A-Za-z0-9_-]{8,}|hf_[A-Za-z0-9_-]{8,}|github_pat_[A-Za-z0-9_]{8,}|gh[oprsu]_[A-Za-z0-9]{8,}|glpat-[A-Za-z0-9_-]{8,}|[sr]k_live_[A-Za-z0-9_-]{8,}|AIza[A-Za-z0-9_-]{8,}|xapp-[A-Za-z0-9-]{8,}|npm_[A-Za-z0-9_-]{8,}|AKIA[A-Z0-9]{12,}|xox[a-z]-[A-Za-z0-9-]{8,})`)
	privateKeyBegin   = regexp.MustCompile(`(?i)-----BEGIN [A-Z0-9 ]*PRIVATE KEY(?: BLOCK)?-----`)
	privateKeyEnd     = regexp.MustCompile(`(?i)-----END [A-Z0-9 ]*PRIVATE KEY(?: BLOCK)?-----`)
)

// sources are the registered stores, in the order they came.
var sources struct {
	sync.RWMutex
	list []*source
}

type source struct {
	open  func() func(string) string
	ready func() bool
}

// Register adds a store's scrubber, which replaces every current value the
// store holds in a text. Exact and Strict run every registered scrubber
// before the patterns, so a stored value keeps the store's name for it. The
// returned function removes the scrubber again.
func Register(scrub func(text string) string) (remove func()) {
	return RegisterStore(func() func(string) string { return scrub }, nil)
}

// RegisterStore adds a store by what opens its values: open opens every
// current value once and returns a scrubber over them, which Snapshot keeps
// for a whole pass. ready reports whether the store can open its values
// now, false for the vault while its key is locked away; nil is always
// ready.
func RegisterStore(open func() func(text string) string, ready func() bool) (remove func()) {
	entry := &source{open, ready}
	sources.Lock()
	sources.list = append(sources.list, entry)
	sources.Unlock()
	return func() {
		sources.Lock()
		defer sources.Unlock()
		for index, registered := range sources.list {
			if registered == entry {
				sources.list = append(sources.list[:index:index], sources.list[index+1:]...)
				return
			}
		}
	}
}

// Complete reports whether every registered store can open its values now,
// so Exact and Strict take every stored value out. While it is false they
// still take out what the open stores hold and what the patterns find.
func Complete() bool {
	sources.RLock()
	list := sources.list
	sources.RUnlock()
	for _, entry := range list {
		if entry.ready != nil && !entry.ready() {
			return false
		}
	}
	return true
}

// Values are the registered stores' values, opened once by Snapshot.
type Values struct {
	scrubs   []func(string) string
	complete bool
}

// Snapshot opens every registered store's values once, for a pass over
// many texts, such as every event of every session. It holds the values in
// memory, so a caller keeps it for one pass and no longer: a value changed
// or added after it was taken is not in it.
func Snapshot() Values {
	sources.RLock()
	list := sources.list
	sources.RUnlock()
	snapshot := Values{complete: true}
	for _, entry := range list {
		if entry.ready != nil && !entry.ready() {
			snapshot.complete = false
		}
		snapshot.scrubs = append(snapshot.scrubs, entry.open())
	}
	return snapshot
}

// Complete reports whether every store could open its values when the
// snapshot was taken.
func (v Values) Complete() bool { return v.complete }

// Exact is the package's Exact over the snapshot's values.
func (v Values) Exact(text string) string {
	if text == "" {
		return text
	}
	return lines(v.stored(text), false, nil)
}

// Strict is the package's Strict over the snapshot's values.
func (v Values) Strict(text string) string {
	if text == "" {
		return text
	}
	return lines(v.stored(text), true, nil)
}

// stored runs every store's scrubber over text.
func (v Values) stored(text string) string {
	for _, scrub := range v.scrubs {
		text = scrub(text)
	}
	return text
}

// Exact replaces what is certainly a secret: every current value of every
// registered store, the tokens whose prefix or shape names them (OpenAI and
// Anthropic sk-, GitHub ghp_, gho_, ghs_, ghu_, ghr_ and github_pat_, GitLab
// glpat-, Slack xox*- and xapp-, AWS AKIA, Google AIza, Hugging Face hf_,
// Groq gsk_, Stripe live keys, npm_, JSON web tokens) and every private key
// block. Ordinary prose is left as it was. It opens every store for the one
// text; a pass over many takes a Snapshot.
func Exact(text string) string {
	if text == "" {
		return text
	}
	return Snapshot().Exact(text)
}

// Strict is Exact and also what only looks like a secret: the value given to
// a name such as password, token or api_key in an assignment, a config
// line or a fish set, the password of a URL such as https://user:pass@host,
// and the credential after Bearer.
func Strict(text string) string {
	if text == "" {
		return text
	}
	return Snapshot().Strict(text)
}

// Patterns is Strict without the stored values, for text read in bulk such
// as the files the index reads, where opening every store for each piece
// would cost too much. inPrivateKey carries a private key block that is
// still open from one piece of a file to the next; nil reads text whole.
func Patterns(text string, inPrivateKey *bool) string {
	if text == "" {
		return text
	}
	return lines(text, true, inPrivateKey)
}

// lines scrubs text line by line, so the line count stays and a private key
// block is replaced line for line.
func lines(text string, strict bool, inPrivateKey *bool) string {
	if inPrivateKey == nil {
		inPrivateKey = new(bool)
	}
	split := strings.Split(text, "\n")
	for index, line := range split {
		if privateKeyBegin.MatchString(line) {
			*inPrivateKey = true
		}
		if *inPrivateKey {
			split[index] = PrivateKeyMark
			if privateKeyEnd.MatchString(line) {
				*inPrivateKey = false
			}
			continue
		}
		if strict {
			line = urlCredential.ReplaceAllString(line, "${1}"+Mark+"${2}")
			line = secretAssignment.ReplaceAllString(line, "${1}${2}"+Mark)
			line = fishSecret.ReplaceAllString(line, "${1}"+Mark)
			line = secretWhitespace.ReplaceAllString(line, "${1}"+Mark)
			line = bearerToken.ReplaceAllString(line, "Bearer "+Mark)
		}
		line = jwtToken.ReplaceAllString(line, Mark)
		split[index] = secretToken.ReplaceAllStringFunc(line, token)
	}
	return strings.Join(split, "\n")
}

// token replaces a match of secretToken that holds a digit. A real token's
// random part all but always does, while a word such as sk-learn-tutorial
// that only shares a prefix does not.
func token(match string) string {
	if strings.ContainsAny(match, "0123456789") {
		return Mark
	}
	return match
}
