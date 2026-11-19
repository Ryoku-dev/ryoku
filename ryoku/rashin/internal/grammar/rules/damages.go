package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type Damages struct{ expression expr.Expr }

func NewDamages() *Damages {
	return &Damages{expression: expr.NewSequenceExprWordSet([]string{"damages", "damage"})}
}
func (r *Damages) Expr() expr.Expr                  { return r.expression }
func (r *Damages) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*Damages) Description() string {
	return "Checks for plural `damages` not in the context of a court case."
}
func (r *Damages) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}

var damagesKeywords = []string{"case", "cases", "claim", "claims", "judgment", "judgments", "liabilities", "liability", "liable", "settlement", "settlements", "warranty"}

func (*Damages) Unit() lint.DocumentIterator { return lint.Sentence{} }
func (*Damages) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if ctx == nil || len(toks) == 0 {
		return nil
	}
	pre, post := ctx.Before, ctx.After
	tok := toks[0]
	chars := tok.GetCh(src)
	if text.EqStr(chars, "damage") {
		return nil
	}
	if len(post) >= 2 && post[0].Kind.IsWhitespace() && post[1].Kind.IsWord() && (post[1].Kind.IsObjectPronoun() || post[1].Kind.IsNoun()) {
		return nil
	}
	noun, verb := false, false
	if len(pre) >= 2 && pre[len(pre)-1].Kind.IsWhitespace() && pre[len(pre)-2].Kind.IsWord() {
		p := pre[len(pre)-2]
		noun = (p.Kind.IsAdjective() || p.Kind.IsDeterminer() || p.Kind.IsPreposition()) && !text.EqStr(p.GetCh(src), "to")
		verb = p.Kind.IsAuxiliaryVerb() || (p.Kind.IsSubjectPronoun() && p.Kind.IsThirdPersonSingularPronoun())
	}
	if verb && !noun {
		return nil
	}
	payDet := expr.NewSequenceExprWordSet([]string{"paid", "pay", "paying", "pays"}).ThenOptional(expr.NewSequenceExpr().TWs().ThenDeterminer()).TWs()
	count := 0
	for i := len(pre) - 2; i >= 0; i-- {
		if _, ok := payDet.Run(i, pre, src); ok {
			break
		}
		count++
	}
	if count < len(pre)/2 {
		return nil
	}
	for _, tokens := range [][]doc.Token{pre, post} {
		for _, t := range tokens {
			if text.EqAnyIgnoreASCIICaseStr(t.GetCh(src), damagesKeywords) {
				return nil
			}
		}
	}
	result := lint.NewLint()
	result.Span = tok.Span
	result.LintKind = lint.Usage
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCase(chars[:6], chars)}
	result.Message = "Singular `damage` is correct when not referring to a court case."
	return &result
}
