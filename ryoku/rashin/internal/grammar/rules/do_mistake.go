package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type DoMistake struct{ expression expr.Expr }

func NewDoMistake() *DoMistake {
	alternatives := []expr.Expr{expr.NewSequenceExprWordSet([]string{"a", "an", "the", "that", "these", "this", "those", "another", "many", "several", "some", "my", "our", "your", "his", "her", "its", "their"})}
	for _, p := range []string{"a lot of", "lots of", "that kind of", "these kinds of", "this kind of", "those kinds of", "so many", "too many", "tons of", "tonnes of"} {
		alternatives = append(alternatives, expr.NewFixedPhraseFromPhrase(p))
	}
	return &DoMistake{expression: expr.NewSequenceExprWordSet([]string{"do", "did", "does", "doing", "done"}).TWs().ThenLongestOf(alternatives).TWs().ThenWordSet([]string{"mistake", "mistakes"})}
}
func (r *DoMistake) Expr() expr.Expr                  { return r.expression }
func (r *DoMistake) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *DoMistake) Description() string              { return "Corrects `do a mistake` to `make a mistake`." }
func (r *DoMistake) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
func (r *DoMistake) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	if lint.FollowedByWord(ctx, func(t *doc.Token) bool { return t.Kind.IsVerb() && !t.Kind.IsVerbProgressiveForm() }) {
		return nil
	}
	span := toks[0].Span
	chars := span.GetContent(src)
	var make string
	switch {
	case text.EqStr(chars, "do"):
		make = "make"
	case text.EqStr(chars, "did") || text.EqStr(chars, "done"):
		make = "made"
	case text.EqStr(chars, "does"):
		make = "makes"
	case text.EqStr(chars, "doing"):
		make = "making"
	default:
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Usage
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(make, chars)}
	l.Message = "In English we `make` mistakes, not `do` them"
	return &l
}
