package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type AdjectiveDoubleDegree struct{ expression expr.Expr }

func NewAdjectiveDoubleDegree() *AdjectiveDoubleDegree {
	return &AdjectiveDoubleDegree{expression: expr.NewSequenceExprWordSet([]string{"more", "most"}).TWs().ThenKindWhere(func(k doc.TokenKind) bool { return k.IsComparativeAdjective() || k.IsSuperlativeAdjective() })}
}
func (r *AdjectiveDoubleDegree) Expr() expr.Expr                  { return r.expression }
func (r *AdjectiveDoubleDegree) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*AdjectiveDoubleDegree) Description() string {
	return "Finds adjectives that are used as double degrees (e.g. `more prettier`)."
}
func (r *AdjectiveDoubleDegree) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	adj := toks[len(toks)-1].GetCh(src)
	k := toks[len(toks)-1].Kind
	l := lint.NewLint()
	l.Span = span
	l.Priority = 126
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCase(adj, span.GetContent(src))}
	if text.EqStr(toks[0].GetCh(src), "more") && k.IsComparativeAdjective() && !k.IsSuperlativeAdjective() {
		l.LintKind = lint.Redundancy
		l.Message = "Using `more` and the comparative form of the adjective together is redundant."
	} else if text.EqStr(toks[0].GetCh(src), "most") && !k.IsComparativeAdjective() && k.IsSuperlativeAdjective() {
		l.LintKind = lint.Redundancy
		l.Message = "Using `most` and the superlative form of the adjective together is redundant."
	} else {
		var other []rune
		switch string(adj) {
		case "better":
			other = []rune("best")
		case "best":
			other = []rune("better")
		case "worse":
			other = []rune("worst")
		case "worst":
			other = []rune("worse")
		default:
			if len(adj) > 0 && adj[len(adj)-1] == 'r' {
				other = append(append([]rune(nil), adj[:len(adj)-1]...), 's', 't')
			} else if len(adj) >= 2 && string(adj[len(adj)-2:]) == "st" {
				other = append(append([]rune(nil), adj[:len(adj)-2]...), 'r')
			} else {
				return nil
			}
		}
		l.LintKind = lint.WordChoice
		l.Message = "The degree of the adverb conflicts with the degree of the adjective."
		l.Suggestions = append(l.Suggestions, lint.ReplaceWithMatchCase(other, span.GetContent(src)))
	}
	return &l
}
