package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type WithOpenArms struct{ expression expr.Expr }

func NewWithOpenArms() *WithOpenArms {
	return &WithOpenArms{expression: expr.ButNot(expr.NewSequenceExprAnyOf([]expr.Expr{expr.AsExpr(expr.NewInflectionOfBe()), expr.AsExpr(expr.NewWordSet([]string{"get", "gets", "getting", "got", "gotten"}))}).TWs().TSet([]string{"greet", "greeted", "welcome", "welcomed"}).TWs().TAco("with").TWs().TSet([]string{"open", "opened"}).TWs().TSet([]string{"arm", "arms"}), expr.NewSequenceExprWordSet([]string{"greeted", "welcomed"}).TAny().TAny().TAny().TAco("open").TAny().TAco("arms"))}
}
func (r *WithOpenArms) Expr() expr.Expr { return r.expression }
func (r *WithOpenArms) Description() string {
	return "Corrects wrong variants of the idiom `welcome/greet with open arms`."
}
func (r *WithOpenArms) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *WithOpenArms) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) < 6 {
		return nil
	}
	tail := t[2:]
	if len(tail) < 4 {
		return nil
	}
	verb := tail[0].GetCh(s)
	open := tail[len(tail)-3].GetCh(s)
	arms := tail[len(tail)-1].GetCh(s)
	notPassive := verb[len(verb)-1] != 'd' && verb[len(verb)-1] != 'D'
	openPast := open[len(open)-1] == 'd' || open[len(open)-1] == 'D'
	singular := arms[len(arms)-1] != 's' && arms[len(arms)-1] != 'S'
	correction := append([]rune{}, verb...)
	if notPassive {
		last := verb[len(verb)-1]
		if verb[0] == 'g' || verb[0] == 'G' {
			e := rune(byte(last - ('t' - 'e')))
			correction = append(correction, e, rune(byte(e-1)))
		} else {
			correction = append(correction, rune(byte(last-1)))
		}
	}
	middle, ok := doc.GetCh(tail[1:len(tail)-3], s)
	if !ok {
		return nil
	}
	correction = append(correction, middle...)
	if openPast {
		open = open[:len(open)-2]
	}
	correction = append(correction, open...)
	correction = append(correction, tail[len(tail)-2].GetCh(s)...)
	correction = append(correction, arms...)
	if singular {
		correction = append(correction, rune(byte(arms[len(arms)-1]+('s'-'m'))))
	}
	span, ok := doc.Span(tail)
	if !ok {
		return nil
	}
	message := "The correct form of the idiom is `with open arms`."
	if notPassive {
		message = "The idiom `with open arms` requires the passive mood, so the verb must be in the past tense."
	}
	return &lint.Lint{Span: span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWith(correction)}, Message: message, Priority: 127}
}
