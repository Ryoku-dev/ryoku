package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type InflectedVerbAfterTo struct{ dictionary dict.Dictionary }

func NewInflectedVerbAfterTo(dictionary dict.Dictionary) *InflectedVerbAfterTo {
	return &InflectedVerbAfterTo{dictionary: dictionary}
}
func (*InflectedVerbAfterTo) Description() string {
	return "This rule looks for `to verb` where `verb` is not in the infinitive form."
}
func (r *InflectedVerbAfterTo) Lint(d *doc.Document) []lint.Lint {
	var lints []lint.Lint
	src := d.GetSource()
	for _, pi := range d.IterPrepositionIndices() {
		prep, _ := d.GetToken(pi)
		space, sok := d.GetToken(pi + 1)
		word, wok := d.GetToken(pi + 2)
		if !sok || !wok || !space.Kind.IsWhitespace() || !word.Kind.IsWord() || !text.EqCh(prep.GetCh(src), []rune{'t', 'o'}) {
			continue
		}
		chars := word.GetCh(src)
		if len(chars) < 4 {
			continue
		}
		check := func(stem []rune) bool {
			m := r.dictionary.GetWordMetadata(stem)
			return m != nil && m.IsVerb() && !m.IsNoun()
		}
		add := func(stem []rune) {
			l := lint.NewLint()
			l.Span = text.Span{Start: prep.Span.Start, End: word.Span.End}
			l.LintKind = lint.WordChoice
			l.Message = "The base form of the verb is needed here."
			replacement := append([]rune(nil), prep.GetCh(src)...)
			replacement = append(replacement, ' ')
			replacement = append(replacement, stem...)
			l.Suggestions = []lint.Suggestion{lint.ReplaceWith(replacement)}
			lints = append(lints, l)
		}
		expectsInfinitive := func() bool {
			prev, ok := d.GetNextWordFromOffset(pi, -1)
			if !ok {
				return true
			}
			m := r.dictionary.GetWordMetadata(prev.GetCh(src))
			return m != nil && (m.IsAdjective() || m.IsVerb())
		}
		if chars[len(chars)-2] == 'e' && chars[len(chars)-1] == 'd' {
			if check(chars[:len(chars)-2]) && expectsInfinitive() {
				add(chars[:len(chars)-2])
			}
			if check(chars[:len(chars)-1]) {
				add(chars[:len(chars)-1])
			}
		}
		if chars[len(chars)-2] == 'e' && chars[len(chars)-1] == 's' && check(chars[:len(chars)-2]) {
			add(chars[:len(chars)-2])
		}
		if chars[len(chars)-1] == 's' && check(chars[:len(chars)-1]) {
			add(chars[:len(chars)-1])
		}
	}
	return lints
}
