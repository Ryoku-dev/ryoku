package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type OldestInTheBook struct{ expression expr.Expr }

func NewOldestInTheBook() *OldestInTheBook {
	adj := func(t doc.Token, s []rune) bool {
		k := t.Kind
		return (k.IsNpMember() || k.IsAdjective()) && !k.IsNoun() && !text.EqStr(t.GetCh(s), "in")
	}
	adjseq := expr.NewRepeating(expr.NewSequenceExprWith(adj).TWs(), 0)
	noun := func(t doc.Token, s []rune) bool {
		k := t.Kind
		return (k.IsNpMember() || k.IsNoun() || k.IsOov()) && !text.EqStr(t.GetCh(s), "in")
	}
	nounseq := expr.NewSequenceExprWith(noun).ThenOptional(expr.NewRepeating(expr.NewSequenceExpr().TWs().Then(noun), 1))
	np := expr.NewSequenceExprOptional(adjseq).Then(nounseq)
	return &OldestInTheBook{expression: expr.NewSequenceExprFixedPhrase("oldest ").Then(np).TWs().ThenWordSeq([]string{"in", "the", "books"})}
}
func (r *OldestInTheBook) Expr() expr.Expr                  { return r.expression }
func (r *OldestInTheBook) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *OldestInTheBook) Description() string {
	return "Detects the idiom `oldest X in the books`, which should use singular `book`."
}
func (r *OldestInTheBook) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 6 {
		return nil
	}
	tricky := false
	for _, t := range toks[2 : len(toks)-4] {
		if text.EqAnyIgnoreASCIICaseStr(t.GetCh(src), []string{"trick", "tricks"}) {
			tricky = true
			break
		}
	}
	message := "If this is a play on the idiom `oldest trick in the book`, it should use singular `book` instead of plural `books`."
	if tricky {
		message = "This idiom should use singular `book` instead of plural `books`."
	}
	tok := toks[len(toks)-1]
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.Usage
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("book", tok.GetCh(src))}
	l.Message = message
	return &l
}
