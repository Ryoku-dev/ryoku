package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type EverEvery struct{ expression expr.Expr }

func NewEverEvery() *EverEvery {
	return &EverEvery{expression: expr.ButNot(expr.NewSequenceExprAnyOf([]expr.Expr{expr.AsExpr(expr.NewWordSet([]string{"are", "aren't", "arent", "did", "didn't", "didnt", "do", "does", "doesn't", "doesnt", "dont", "don't", "had", "hadn't", "hadnt", "has", "hasn't", "hasnt", "have", "haven't", "havent", "is", "isn't", "isnt", "was", "wasn't", "wasnt", "were", "weren't", "werent"})), expr.AsExpr(expr.NewModalVerbWithCommonErrors())}).TWs().ThenSubjectPronoun().TWs().TAco("every"), expr.NewSequenceExprAnything().TAny().TAco("it"))}
}
func (r *EverEvery) Expr() expr.Expr { return r.expression }
func (r *EverEvery) Description() string {
	return "Tries to correct typos of `every` instead of `ever`."
}
func (r *EverEvery) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *EverEvery) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span := toks[4].Span
	content := src[span.Start:span.End]
	suggestions := []lint.Suggestion{lint.ReplaceWithMatchCase(content[:len(content)-1], content)}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Typo
	result.Message = "Is this `every` a typo that should be `ever`?"
	result.Suggestions = suggestions
	return &result
}
