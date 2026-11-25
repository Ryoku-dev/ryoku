package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ToteTout struct{ expression expr.Expr }

func NewToteTout() *ToteTout {
	tote := []string{"tote", "toted", "totes", "toting"}
	tout := []string{"tout", "touted", "touting", "touts"}
	return &ToteTout{expr.NewFirstMatchOf([]expr.Expr{expr.NewSequenceExprWordSet([]string{"ads", "highly", "much", "often", "ticket", "widely"}).TWsH().TSet(tote), expr.NewSequenceExprWordSet([]string{"canvas", "gun", "pistol", "plastic"}).TWsH().TSet(tout), expr.NewSequenceExprWordSet(tote).TWs().TSet([]string{"as"}), expr.NewSequenceExprWordSet(tout).TWs().TSet([]string{"bag", "bags", "boxes"})})}
}
func (r *ToteTout) Expr() expr.Expr                  { return r.expression }
func (r *ToteTout) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*ToteTout) Description() string                { return "Flags places where `tote` and `tout` may be confused." }
func (r *ToteTout) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	w := lint.FindTheOnlyTokenMatching(t, s, func(w *doc.Token, s []rune) bool {
		return text.EqAnyIgnoreASCIICaseStr(w.GetCh(s), []string{"tote", "toted", "totes", "toting", "tout", "touted", "touting", "touts"})
	})
	if w == nil {
		return nil
	}
	ch := w.GetCh(s)
	if len(ch) < 3 {
		return nil
	}
	correction := ""
	switch ch[2] {
	case 'u':
		switch ch[len(ch)-1] {
		case 't':
			correction = "tote"
		case 'd':
			correction = "toted"
		case 'g':
			correction = "toting"
		case 's':
			correction = "totes"
		}
	case 't':
		switch ch[len(ch)-1] {
		case 'e':
			correction = "tout"
		case 'd':
			correction = "touted"
		case 's':
			correction = "touts"
		case 'g':
			correction = "touting"
		}
	}
	if correction == "" {
		return nil
	}
	return &lint.Lint{Span: w.Span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr(correction, ch)}, Message: "Are you confusng `tote` (carry, bag) with `tout` (promote, promoter)?", Priority: 127}
}
