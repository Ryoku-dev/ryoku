package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"strings"
)

type MassPlurals struct {
	expression expr.Expr
	dict       dict.Dictionary
}

func NewMassPlurals(d dict.Dictionary) *MassPlurals {
	oov := expr.NewSequenceExpr().ThenOov()
	plural := expr.NewSequenceExprWith(func(t doc.Token, s []rune) bool { return text.EndsWithIgnoreASCIICaseStr(t.GetCh(s), "s") })
	phrases := expr.NewFirstMatchOf([]expr.Expr{expr.NewFixedPhraseFromPhrase("real estates"), expr.NewFixedPhraseFromPhrase("source codes"), expr.NewFixedPhraseFromPhrase("wear and tears")})
	return &MassPlurals{expression: expr.NewFirstMatchOf([]expr.Expr{expr.NewAll([]expr.Expr{oov, plural}), phrases}), dict: d}
}
func (r *MassPlurals) Expr() expr.Expr                  { return r.expression }
func (r *MassPlurals) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *MassPlurals) Description() string {
	return "Looks for plural forms of mass nouns that have no plural."
}
func (r *MassPlurals) isMass(chars []rune) bool {
	m := r.dict.GetWordMetadata(chars)
	return m != nil && m.IsMassNounOnly()
}
func (r *MassPlurals) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	var singulars []string
	add := func(s []rune) {
		v := string(s)
		for _, x := range singulars {
			if x == v {
				return
			}
		}
		singulars = append(singulars, v)
	}
	if len(toks) != 1 {
		phrase := span.GetContent(src)
		if len(phrase) == 0 {
			return nil
		}
		add(phrase[:len(phrase)-1])
	} else {
		chars := toks[0].GetCh(src)
		if len(chars) > 0 && chars[len(chars)-1] == 's' {
			chars = chars[:len(chars)-1]
			if r.isMass(chars) {
				add(chars)
			}
			if len(chars) > 0 && chars[len(chars)-1] == 'e' {
				chars = chars[:len(chars)-1]
				if r.isMass(chars) {
					add(chars)
				}
				if len(chars) > 0 && chars[len(chars)-1] == 'i' {
					ys := append(append([]rune(nil), chars[:len(chars)-1]...), 'y')
					if r.isMass(ys) {
						add(ys)
					}
				}
			}
		}
	}
	if len(singulars) == 0 {
		return nil
	}
	term := "word"
	if len(toks) != 1 {
		term = "term"
	}
	return &lint.Lint{Span: span, LintKind: lint.Grammar, Suggestions: lint.ToReplaceSuggestions(singulars, span.GetContent(src)), Message: fmt.Sprintf("The %s `%s` is a mass noun and should not be pluralized.", term, strings.Join(singulars, "`, `")), Priority: 30}
}
