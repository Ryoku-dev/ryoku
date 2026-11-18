package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type DigUnderTheHood struct{ expression expr.Expr }

func NewDigUnderTheHood() *DigUnderTheHood {
	return &DigUnderTheHood{expression: expr.NewSequenceExprWordSet([]string{"dig", "digging", "digs", "dug"}).TWs().ThenWordSeq([]string{"under", "the"}).TWs().TSet([]string{"hood", "bonnet"})}
}
func (r *DigUnderTheHood) Expr() expr.Expr { return r.expression }
func (r *DigUnderTheHood) Description() string {
	return "Detects the mixed metaphor of `digging under the hood/bonnet`."
}
func (r *DigUnderTheHood) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *DigUnderTheHood) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Usage

	tok := toks[0]
	ch := tok.GetCh(src)
	if len(ch) < 2 {
		return nil
	}
	var replacement string
	switch strings.ToLower(string(ch[len(ch)-2:])) {
	case "ig":
		replacement = "look"
	case "ng":
		replacement = "looking"
	case "gs":
		replacement = "looks"
	case "ug":
		replacement = "looked"
	default:
		return nil
	}
	result.Span = tok.Span
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, ch)}
	result.Message = "If the context is not automechanics, you may be mixing metaphors."
	return &result
}
