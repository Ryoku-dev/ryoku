package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type AsSoonAsPossible struct{ expression expr.Expr }

func NewAsSoonAsPossible() *AsSoonAsPossible {
	return &AsSoonAsPossible{expression: expr.NewSequenceExprAnyCapitalizationOf("asap").ThenOptional(expr.NewSequenceExprWhitespace().ThenWordSeq([]string{"as", "possible"}))}
}
func (r *AsSoonAsPossible) Expr() expr.Expr                  { return r.expression }
func (r *AsSoonAsPossible) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*AsSoonAsPossible) Description() string {
	return "Looks for redundant use of the initialism `ASAP` and offers to expand when not redundant."
}
func (r *AsSoonAsPossible) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}

func (r *AsSoonAsPossible) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	offer := false
	switch len(toks) {
	case 1:
	case 5:
		offer = true
		if ctx != nil && len(ctx.Before) >= 2 {
			before := ctx.Before
			word, ws := before[len(before)-2], before[len(before)-1]
			if ws.Kind.IsWhitespace() && word.Kind.IsWord() && text.EqStr(word.GetCh(src), "as") {
				span.Start = word.Span.Start
			}
		}
	default:
		return nil
	}
	var suggestions []lint.Suggestion
	if offer {
		suggestions = append(suggestions, lint.ReplaceWith([]rune("ASAP")))
	}
	expansion := "as soon as possible"
	if toks[0].GetStr(src) == "ASAP" && len(toks) > 2 && toks[2].GetStr(src) == "AS" {
		expansion = "AS SOON AS POSSIBLE"
	}
	suggestions = append(suggestions, lint.ReplaceWith([]rune(expansion)))
	result := lint.NewLint()
	result.Span = span
	result.Suggestions = suggestions
	result.Message = "Try expanding this initialism."
	if len(suggestions) > 1 {
		result.LintKind = lint.Redundancy
		result.Message = "You can avoid redundancy since `ASAP` already means `as soon as possible`."
	}
	return &result
}
