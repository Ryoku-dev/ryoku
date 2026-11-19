package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
)

type PronounKnew struct{ expression expr.Expr }

func NewPronounKnew() *PronounKnew {
	p := func(t doc.Token, s []rune) bool {
		return t.Kind.IsUpos(pos.PRON) && t.Kind.IsPersonalPronoun() && t.Kind.IsSubjectPronoun()
	}
	return &PronounKnew{expression: expr.NewLongestMatchOf([]expr.Expr{expr.NewSequenceExprWith(p).TWs().TAco("new"), expr.NewSequenceExprWith(p).TWs().TSet([]string{"always", "never", "also", "often"}).TWs().TAco("new")})}
}
func (r *PronounKnew) Expr() expr.Expr                  { return r.expression }
func (r *PronounKnew) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *PronounKnew) Description() string {
	return "Detects when “new” following a pronoun (optionally with an adverb) is a typo for the past tense “knew.”"
}
func (r *PronounKnew) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span := toks[len(toks)-1].Span
	return &lint.Lint{Span: span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("knew", span.GetContent(src))}, Message: "Did you mean “knew” (the past tense of “know”)?", Priority: 31}
}
