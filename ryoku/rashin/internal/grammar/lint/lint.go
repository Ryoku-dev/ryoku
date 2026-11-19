// Package lint implements Harper's lint values and rule execution machinery.
package lint

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/text"
)

type LintKind uint8

const (
	Agreement LintKind = iota
	BoundaryError
	Capitalization
	Eggcorn
	Enhancement
	Formatting
	Grammar
	Malapropism
	Miscellaneous
	Nonstandard
	Punctuation
	Readability
	Redundancy
	Regionalism
	Repetition
	Spelling
	Style
	Typo
	Usage
	WordChoice
	WordOrder
)

var kindNames = [...]string{"Agreement", "BoundaryError", "Capitalization", "Eggcorn", "Enhancement", "Formatting", "Grammar", "Malapropism", "Miscellaneous", "Nonstandard", "Punctuation", "Readability", "Redundancy", "Regionalism", "Repetition", "Spelling", "Style", "Typo", "Usage", "WordChoice", "WordOrder"}

func (k LintKind) ToStringKey() string {
	if int(k) >= len(kindNames) {
		return ""
	}
	return kindNames[k]
}
func (k LintKind) String() string {
	if k == WordChoice {
		return "Word Choice"
	}
	if k == WordOrder {
		return "Word Order"
	}
	return k.ToStringKey()
}
func LintKindFromStringKey(s string) (LintKind, bool) {
	for k, n := range kindNames {
		if n == s && LintKind(k) != WordOrder {
			return LintKind(k), true
		}
	}
	return 0, false
}
func (k LintKind) MarshalJSON() ([]byte, error) { return json.Marshal(k.ToStringKey()) }

type SuggestionKind uint8

const (
	ReplaceWithKind SuggestionKind = iota
	InsertAfterKind
	RemoveKind
)

type Suggestion struct {
	Kind        SuggestionKind
	Replacement []rune
}

func ReplaceWith(v []rune) Suggestion    { return Suggestion{Kind: ReplaceWithKind, Replacement: v} }
func InsertAfter(v []rune) Suggestion    { return Suggestion{Kind: InsertAfterKind, Replacement: v} }
func Remove() Suggestion                 { return Suggestion{Kind: RemoveKind} }
func (s Suggestion) IsReplaceWith() bool { return s.Kind == ReplaceWithKind }
func (s Suggestion) IsInsertAfter() bool { return s.Kind == InsertAfterKind }
func (s Suggestion) IsRemove() bool      { return s.Kind == RemoveKind }
func (s Suggestion) Apply(span text.Span, source *[]rune) {
	p := *source
	if s.Kind == InsertAfterKind {
		span.Start = span.End
	}
	value := s.Replacement
	if s.Kind == RemoveKind {
		value = nil
	}
	oldLen := len(p)
	newLen := oldLen - (span.End - span.Start) + len(value)
	if newLen > cap(p) {
		q := make([]rune, newLen)
		copy(q, p[:span.Start])
		copy(q[span.Start:], value)
		copy(q[span.Start+len(value):], p[span.End:])
		*source = q
		return
	}
	if newLen > oldLen {
		p = p[:newLen]
	}
	copy(p[span.Start+len(value):], p[span.End:oldLen])
	copy(p[span.Start:], value)
	*source = p[:newLen]
}
func (s Suggestion) String() string {
	switch s.Kind {
	case ReplaceWithKind:
		return fmt.Sprintf("Replace with: “%s”", string(s.Replacement))
	case InsertAfterKind:
		return fmt.Sprintf("Insert “%s”", string(s.Replacement))
	default:
		return "Remove error"
	}
}
func ReplaceWithMatchCase(v, template []rune) Suggestion {
	return ReplaceWith(text.CopyCasing(template, v))
}
func ReplaceWithMatchCaseStr(v string, template []rune) Suggestion {
	return ReplaceWithMatchCase([]rune(v), template)
}
func ToReplaceSuggestions(values []string, template []rune) []Suggestion {
	out := make([]Suggestion, 0, len(values))
	for _, v := range values {
		out = append(out, ReplaceWithMatchCaseStr(v, template))
	}
	return out
}

type Lint struct {
	Span        text.Span
	LintKind    LintKind
	Suggestions []Suggestion
	Message     string
	Priority    uint8
}

func NewLint() Lint                        { return Lint{LintKind: Miscellaneous, Priority: 127} }
func (l Lint) GetCh(source []rune) []rune  { return source[l.Span.Start:l.Span.End] }
func (l Lint) GetStr(source []rune) string { return string(l.GetCh(source)) }

// SpanlessHash compares lint contents within this implementation, not across Harper versions.
func (l Lint) SpanlessHash() uint64 {
	l.Span = text.Span{}
	b, _ := json.Marshal(l)
	h := fnv.New64a()
	_, _ = h.Write(b)
	return h.Sum64()
}

type Linter interface {
	Lint(*doc.Document) []Lint
	Description() string
}

const MaxSuggestionTransformationDepth = 100
