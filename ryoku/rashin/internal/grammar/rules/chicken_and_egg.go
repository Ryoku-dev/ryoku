package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ChickenAndEgg struct{ expression expr.Expr }

func NewChickenAndEgg() *ChickenAndEgg                    { return &ChickenAndEgg{expression: chickenAndEggExpression()} }
func (r *ChickenAndEgg) Expr() expr.Expr                  { return r.expression }
func (r *ChickenAndEgg) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*ChickenAndEgg) Description() string {
	return "Corrects wrong variants of the `chicken-and-egg` idiom."
}
func (r *ChickenAndEgg) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 7 && t[0].Kind.IsSingularNoun() && ((t[1].Kind.IsWhitespace() && t[3].Kind.IsWhitespace()) || (t[1].Kind.IsHyphen() && t[3].Kind.IsHyphen())) && t[4].Kind.IsSingularNoun() && text.EqStr(t[0].GetCh(s), "chicken") && text.EqAnyIgnoreASCIICaseStr(t[2].GetCh(s), []string{"and", "&"}) && text.EqStr(t[4].GetCh(s), "egg") {
		return nil
	}
	sep := " "
	if t[1].Kind.IsHyphen() {
		sep = "-"
	}
	conj := "&"
	switch len(t) {
	case 7, 9:
		if t[2].Kind.IsWord() {
			conj = "and"
		}
	case 5:
		if t[1].Kind.IsWord() || sep == "-" {
			conj = "and"
		}
	default:
		return nil
	}
	span, ok := doc.Span(t[:len(t)-2])
	if !ok {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Usage
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("chicken"+sep+conj+sep+"egg", s[span.Start:span.End])}
	l.Message = "If you're referring to “which coame first”, use the standard idiom."
	return &l
}
func chickenAndEggExpression() expr.Expr {
	standard := expr.NewSequenceExprAco("chicken").ThenAnyOf([]expr.Expr{expr.NewSequenceExprWhitespace().TAco("and").ThenHyphen().TAco("egg"), expr.NewSequenceExprWhitespace().TAco("and").TWs().TAco("egg")})
	word := expr.NewWordSet([]string{"and", "an", "or", "vs.", "versus"})
	sym := expr.AsExpr(func(t doc.Token, _ []rune) bool { return t.Kind.IsAmpersand() || t.Kind.IsSlash() })
	separated := expr.NewSequenceExpr().TWsH().ThenAnyOf([]expr.Expr{expr.AsExpr(word), sym}).TWsH()
	separatedDet := expr.NewSequenceExpr().TWsH().ThenAnyOf([]expr.Expr{expr.AsExpr(word), sym}).TWsH().ThenDeterminer().TWsH()
	conjunction := expr.NewFirstMatchOf([]expr.Expr{separated, sym, expr.AsExpr(expr.NewSpaceOrHyphen())})
	return expr.ButNot(expr.NewSequenceExprWordSet([]string{"chicken", "chickens"}).ThenOptional(expr.NewLongestMatchOf([]expr.Expr{conjunction, separatedDet})).TSet([]string{"egg", "eggs"}).TWs().TSet([]string{"argument", "arguments", "conundrum", "conundrums", "dilemma", "dilemmas", "problem", "problems", "question", "questions", "scenario", "scenarios", "situation", "situations", "syndrome", "syndromes"}), standard)
}
