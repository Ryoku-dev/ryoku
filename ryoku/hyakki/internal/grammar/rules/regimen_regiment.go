package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type RegimenRegiment struct{ expression expr.Expr }

func NewRegimenRegiment() *RegimenRegiment {
	return &RegimenRegiment{expression: expr.ButNot(expr.NewSequenceExprAnyOf([]expr.Expr{expr.NewSequenceExprWordSet([]string{"antibiotic", "antiphlogistic", "chemo", "chemotherapeutic", "chemotherapy", "conditioning", "dosage", "dose", "dosing", "drug", "metastatic", "patient's", "patients", "therapeutic", "treating", "treatment"}).TWs().TSet([]string{"regiment", "regiments"}), expr.NewSequenceExprWordSet([]string{"regiment", "regiments"}).TWs().TAco("with").TWs().TSet([]string{"autologous", "hepatitis"})}), expr.NewFirstMatchOf([]expr.Expr{expr.NewSequenceExprWordSet([]string{"artillery", "cavalry", "engineering", "infantry", "parachute"}).TWs().TSet([]string{"regiment", "regiments"}), expr.NewSequenceExprWordSet([]string{"regiment", "regiments"}).TWs().TAco("for").TWs().TSet([]string{"duty", "foreign", "service"})}))}
}
func (r *RegimenRegiment) Expr() expr.Expr                  { return r.expression }
func (r *RegimenRegiment) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*RegimenRegiment) Description() string {
	return "Corrects mistaken use of `regiment` (military unit) when `regimen` (routine) was intended."
}
func (r *RegimenRegiment) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	tok := lint.FindTheOnlyTokenMatching(t, s, func(tok *doc.Token, s []rune) bool {
		v := strings.ToLower(tok.GetStr(s))
		return v == "regiment" || v == "regiments"
	})
	if tok == nil {
		return nil
	}
	chars := tok.GetCh(s)
	last := chars[len(chars)-1]
	correction := append([]rune(nil), chars[:len(chars)-1]...)
	switch last {
	case 's', 'S':
		correction = append(correction[:len(correction)-1], last)
	case 't', 'T':
	default:
		return nil
	}
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.WordChoice
	l.Suggestions = []lint.Suggestion{lint.ReplaceWith(correction)}
	l.Message = "Did you mean `regimen` (a routine)? A `regiment` is a military unit."
	return &l
}
