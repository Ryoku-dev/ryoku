// Package dictate turns one dictated utterance into the text to type: Exact
// keeps the transcript as heard, Clean tidies it by fixed rules, Polished
// asks a model to rewrite the Clean text and falls back to Clean whenever the
// rewrite cannot be trusted. It decides nothing about where the text goes and
// types nothing; finding and checking the target window belongs to the
// callers that insert.
package dictate

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sensitive"
)

// Mode is how much of the transcript dictation changes.
type Mode uint8

const (
	// Exact types the transcript byte for byte.
	Exact Mode = iota
	// Clean removes fillers, applies spoken repairs, the dictionary,
	// snippets and spoken punctuation, and fixes spacing and capitals.
	Clean
	// Polished hands the Clean text to a Polisher and keeps its rewrite only
	// when the rewrite passes every check.
	Polished
)

var modeNames = [...]string{Exact: "exact", Clean: "clean", Polished: "polished"}

func (m Mode) String() string {
	if int(m) < len(modeNames) {
		return modeNames[m]
	}
	return fmt.Sprintf("mode(%d)", m)
}

// ParseMode reads a mode name: exact, clean or polished.
func ParseMode(name string) (Mode, error) {
	for m, n := range modeNames {
		if strings.EqualFold(strings.TrimSpace(name), n) {
			return Mode(m), nil
		}
	}
	return 0, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose exact, clean or polished dictation."))
}

// MarshalText writes the mode's name.
func (m Mode) MarshalText() ([]byte, error) {
	if int(m) >= len(modeNames) {
		return nil, fmt.Errorf("dictate: unknown mode %d", m)
	}
	return []byte(modeNames[m]), nil
}

// UnmarshalText reads a mode's name.
func (m *Mode) UnmarshalText(b []byte) error {
	parsed, err := ParseMode(string(b))
	if err != nil {
		return err
	}
	*m = parsed
	return nil
}

// Polisher rewrites Clean text. A hosted polisher sends it off this machine;
// Options.Hosted says which kind the caller passed.
type Polisher interface {
	Polish(ctx context.Context, clean string) (string, error)
}

const (
	// MaxText is the longest utterance Process takes, in bytes.
	MaxText = 16 << 10
	// DefaultPolishTimeout is how long Polished waits for the polisher.
	DefaultPolishTimeout = 5 * time.Second
	// DefaultMaxExpansion is the most a rewrite may grow, as a ratio of the
	// Clean text's characters. Short texts may always grow by ExpansionFloor
	// characters, so a polisher can add a mark or two to a single word.
	DefaultMaxExpansion = 1.5
	// ExpansionFloor is the growth in characters allowed at any length.
	ExpansionFloor = 12
)

// Options is one utterance's settings.
type Options struct {
	// Mode is the mode asked for. A per-app style that names a mode wins.
	Mode Mode
	// App is the target window's app id; it picks the per-app style.
	App string
	// Context is what is known around the target, such as its window title.
	// It is read only to decide whether the dictation looks like a secret.
	Context string
	// Personal is the user's dictionary, snippets and styles.
	Personal Personal
	// Polisher rewrites text for Polished; nil means Polished gives Clean.
	Polisher Polisher
	// Hosted says Polisher sends text off this machine. Text that looks
	// like a secret is never given to a hosted polisher.
	Hosted bool
	// Timeout bounds the polisher; zero means DefaultPolishTimeout.
	Timeout time.Duration
	// MaxExpansion bounds a rewrite's growth; zero means DefaultMaxExpansion.
	MaxExpansion float64
}

// Fallback is why Polished gave the Clean text instead of a rewrite.
type Fallback string

const (
	FallbackNone Fallback = ""
	// FallbackUnavailable: no polisher was given.
	FallbackUnavailable Fallback = "unavailable"
	// FallbackSecret: the text or its target looks like a secret and the
	// polisher is hosted, so the text never left this machine.
	FallbackSecret Fallback = "secret"
	// FallbackTimeout: the polisher did not answer within the timeout.
	FallbackTimeout Fallback = "timeout"
	// FallbackFailed: the polisher returned an error.
	FallbackFailed Fallback = "failed"
	// FallbackInvalid: the rewrite was empty, not UTF-8 or held control
	// characters.
	FallbackInvalid Fallback = "invalid"
	// FallbackExpansion: the rewrite grew past the allowed ratio.
	FallbackExpansion Fallback = "expansion"
	// FallbackProtected: the rewrite lost or changed a protected value.
	FallbackProtected Fallback = "protected"
)

// Note codes. Notes never carry dictated words, so callers may log them.
const (
	NoteFillers     = "fillers"
	NoteRepair      = "repair"
	NotePunctuation = "punctuation"
	NoteDictionary  = "dictionary"
	NoteSnippet     = "snippet"
	NoteStyle       = "style"
	NotePolished    = "polished"
	NoteFallback    = "fallback"
)

// Note is one thing Process did, with a sentence a user can read.
type Note struct {
	Code string `json:"code"`
	Text string `json:"text"`
}

// Result is the text to type and how it was made.
type Result struct {
	Text string `json:"text"`
	// Mode is the mode whose rules made Text: Clean when Polished fell back
	// or had nothing to polish.
	Mode Mode `json:"mode"`
	// Fallback is why Polished fell back; empty when it did not.
	Fallback Fallback `json:"fallback,omitempty"`
	Notes    []Note   `json:"notes,omitempty"`
}

// Process makes the text to type from one utterance.
func Process(text string, o Options) (Result, error) {
	return ProcessContext(context.Background(), text, o)
}

// ProcessContext is Process bounded by ctx: a cancelled ctx returns its
// error and no text, so a cancelled dictation types nothing.
func ProcessContext(ctx context.Context, text string, o Options) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if !utf8.ValidString(text) {
		return Result{}, badRequest(i18n.T("The transcript is not valid text."))
	}
	if len(text) > MaxText {
		return Result{}, badRequest(i18n.Tf("Dictate up to %d bytes at a time.", MaxText))
	}
	if int(o.Mode) >= len(modeNames) {
		return Result{}, badRequest(i18n.T("Choose exact, clean or polished dictation."))
	}
	if o.Timeout < 0 {
		return Result{}, badRequest(i18n.T("The polish timeout cannot be negative."))
	}
	if o.MaxExpansion != 0 && (o.MaxExpansion < 1 || math.IsNaN(o.MaxExpansion) || math.IsInf(o.MaxExpansion, 0)) {
		return Result{}, badRequest(i18n.T("The polish growth limit must be at least 1."))
	}

	var notes []Note
	mode := o.Mode
	style, styled := o.Personal.style(o.App)
	if styled {
		if style.Mode != "" {
			if m, err := ParseMode(style.Mode); err == nil {
				mode = m
			}
		}
		notes = append(notes, Note{NoteStyle, i18n.T("Used this app's dictation style.")})
	}
	if mode == Exact {
		return Result{Text: text, Mode: Exact, Notes: notes}, nil
	}

	c := clean(text, o.Personal, style)
	notes = append(notes, c.notes...)
	res := Result{Text: c.text, Mode: Clean, Notes: notes}
	if mode == Clean || c.snippet || strings.TrimSpace(c.text) == "" {
		return res, nil
	}
	return polish(ctx, text, c, style, o, res)
}

func badRequest(message string) error {
	return protocol.Errorf(protocol.CodeBadRequest, "%s", message)
}

// polish runs the polisher on the Clean text and keeps its rewrite only when
// every check passes.
func polish(ctx context.Context, raw string, c cleaned, style Style, o Options, res Result) (Result, error) {
	fall := func(f Fallback, text string) (Result, error) {
		res.Fallback = f
		res.Notes = append(res.Notes, Note{NoteFallback, text})
		return res, nil
	}
	if o.Polisher == nil {
		return fall(FallbackUnavailable, i18n.T("No polish model is set up, so this is the clean text."))
	}
	// the words are judged as words, the window and app as places: a long
	// mixed-case app id or a title with a link is no token
	if o.Hosted && (sensitive.Secret(raw) || sensitive.Secret(c.text) || sensitive.Place(o.Context) || sensitive.Place(o.App)) {
		return fall(FallbackSecret, i18n.T("This looks like a secret, so it stayed on this computer and was not polished."))
	}
	timeout := o.Timeout
	if timeout == 0 {
		timeout = DefaultPolishTimeout
	}
	pctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	type answer struct {
		text string
		err  error
	}
	// A polisher that ignores its context must not hold dictation past the
	// timeout, so it runs apart and the buffered channel lets it finish late.
	done := make(chan answer, 1)
	go func() {
		text, err := o.Polisher.Polish(pctx, c.text)
		done <- answer{text, err}
	}()
	var got answer
	select {
	case got = <-done:
	case <-pctx.Done():
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if pctx.Err() != nil && (got.err != nil || got.text == "") {
		return fall(FallbackTimeout, i18n.T("Polishing took too long, so this is the clean text."))
	}
	if got.err != nil {
		if errors.Is(got.err, context.DeadlineExceeded) {
			return fall(FallbackTimeout, i18n.T("Polishing took too long, so this is the clean text."))
		}
		return fall(FallbackFailed, i18n.T("Polishing failed, so this is the clean text."))
	}
	out := strings.TrimSpace(got.text)
	if !validRewrite(out) {
		return fall(FallbackInvalid, i18n.T("The polished text was not usable, so this is the clean text."))
	}
	ratio := o.MaxExpansion
	if ratio == 0 {
		ratio = DefaultMaxExpansion
	}
	n := utf8.RuneCountInString(c.text)
	limit := max(int(math.Ceil(ratio*float64(n))), n+ExpansionFloor)
	if utf8.RuneCountInString(out) > limit {
		return fall(FallbackExpansion, i18n.T("The polished text grew too much, so this is the clean text."))
	}
	if !keepsAll(out, c.protected) {
		return fall(FallbackProtected, i18n.T("Polishing changed a link, address, path, code or number, so this is the clean text."))
	}
	res.Text = endPolished(out, style.End)
	res.Mode = Polished
	res.Notes = append(res.Notes, Note{NotePolished, i18n.T("Polished.")})
	return res, nil
}

// validRewrite rejects empty text and control characters other than line
// breaks and tabs.
func validRewrite(s string) bool {
	if s == "" || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r != '\n' && r != '\t' && (unicode.IsControl(r) || r == utf8.RuneError) {
			return false
		}
	}
	return true
}

// keepsAll reports whether out holds every protected value, whole, at least
// as many times as the Clean text did.
func keepsAll(out string, protected []string) bool {
	need := make(map[string]int, len(protected))
	for _, p := range protected {
		need[p]++
	}
	for p, n := range need {
		if countWhole(out, p) < n {
			return false
		}
	}
	return true
}

// countWhole counts occurrences of v in s not run into by letters or digits
// on either side, so "3.140" does not hold "3.14".
func countWhole(s, v string) int {
	n := 0
	for at := 0; ; {
		k := strings.Index(s[at:], v)
		if k < 0 {
			return n
		}
		start, end := at+k, at+k+len(v)
		before, _ := utf8.DecodeLastRuneInString(s[:start])
		after, _ := utf8.DecodeRuneInString(s[end:])
		first, _ := utf8.DecodeRuneInString(v)
		last, _ := utf8.DecodeLastRuneInString(v)
		if !(start > 0 && wordRune(first) && wordRune(before)) && !(end < len(s) && wordRune(last) && wordRune(after)) {
			n++
		}
		at = start + max(1, len(v))
	}
}

// endPolished applies a style's end rule to a rewrite, the one style rule
// that still holds once a model has rewritten the text.
func endPolished(s, end string) string {
	switch end {
	case EndNone:
		if strings.HasSuffix(s, ".") && !strings.HasSuffix(s, "..") {
			return s[:len(s)-1]
		}
	case EndPeriod:
		r, _ := utf8.DecodeLastRuneInString(s)
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return s + "."
		}
	}
	return s
}
