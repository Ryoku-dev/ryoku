package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

import "unicode"

type MergeWords struct{ dictionary *dict.FstDictionary }

func NewMergeWords() *MergeWords { return &MergeWords{dict.Curated()} }
func (*MergeWords) Description() string {
	return "Accidentally inserting a space inside a word is common. This rule looks for valid words that are split by whitespace."
}
func (r *MergeWords) Lint(d *doc.Document) []lint.Lint {
	var lints []lint.Lint
	var merged []rune
	toks, src := d.Tokens(), d.GetSource()
	for i := 0; i+2 < len(toks); i++ {
		a, w, b := toks[i], toks[i+1], toks[i+2]
		if !a.Kind.IsWord() || !w.Kind.IsWhitespace() || !b.Kind.IsWord() {
			continue
		}
		ac, bc := a.GetCh(src), b.GetCh(src)
		if (len(ac) == 1 && unicode.IsUpper(ac[0])) || (len(bc) == 1 && unicode.IsUpper(bc[0])) || string(ac) == "a" || string(bc) == "a" {
			continue
		}
		merged = append(merged[:0], ac...)
		merged = append(merged, bc...)
		if r.dictionary.ContainsWord(merged) && (!r.dictionary.ContainsWord(ac) || !r.dictionary.ContainsWord(bc)) {
			lints = append(lints, lint.Lint{Span: text.Span{Start: a.Span.Start, End: b.Span.End}, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWith(append([]rune(nil), merged...))}, Message: "It seems these words would go better together.", Priority: 63})
		}
		merged = append(merged[:0], ac...)
		merged = append(merged, '\'')
		merged = append(merged, bc...)
		if r.dictionary.ContainsWord(merged) && (!r.dictionary.ContainsWord(ac) || !r.dictionary.ContainsWord(bc)) {
			lints = append(lints, lint.Lint{Span: text.Span{Start: a.Span.Start, End: b.Span.End}, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWith(append([]rune(nil), merged...))}, Message: "It seems you intended to make this a contraction.", Priority: 63})
		}
	}
	return lints
}
