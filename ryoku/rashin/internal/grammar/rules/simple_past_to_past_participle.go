package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

// SimplePastToPastParticiple corrects simple past tense verbs to past participle after auxiliary verbs like "have" or "be".
type SimplePastToPastParticiple struct{ expression expr.Expr }

func NewSimplePastToPastParticiple() *SimplePastToPastParticiple {
	return &SimplePastToPastParticiple{expression: expr.NewAll([]expr.Expr{expr.NewSequenceExprAnyOf([]expr.Expr{expr.AsExpr(expr.NewWordSet([]string{"have", "had", "has", "having"})), expr.AsExpr(expr.NewInflectionOfBe()), expr.AsExpr(expr.NewWordSet([]string{"I've", "I'd", "we've", "we'd", "you've", "you'd", "he's", "he'd", "she's", "she'd", "it's", "it'd", "they've", "they'd"})), expr.AsExpr(expr.NewWordSet([]string{"Ive", "Id", "weve", "wed", "youve", "youd", "hes", "hed", "shes", "shed", "its", "itd", "theyve", "theyd"}))}).TWs().ThenKindIsButIsNot(doc.TokenKind.IsVerbSimplePastForm, doc.TokenKind.IsVerbPastParticipleForm), expr.NewSequenceExprUnless(expr.NewFirstMatchOf([]expr.Expr{expr.NewSequenceExprWith(expr.NewInflectionOfBe()).TAny().TAco("woke"), expr.NewSequenceExprAco("id").TAny().ThenWordSet([]string{"came", "did", "went"})}))})}
}
func (r *SimplePastToPastParticiple) Expr() expr.Expr { return r.expression }
func (r *SimplePastToPastParticiple) Description() string {
	return "Corrects simple past tense verbs to past participle after auxiliary verbs like \"have\" or \"be\"."
}
func (r *SimplePastToPastParticiple) Lint(d *doc.Document) []lint.Lint {
	return lint.RunExprLinter(r, d)
}
func (r *SimplePastToPastParticiple) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) != 3 || !t[1].Kind.IsWhitespace() || !t[2].Kind.IsVerb() {
		return nil
	}
	simple := t[2].GetStr(s)
	participle, ok := dict.CuratedIrregularVerbs().GetPastParticipleForPreterite(simple)
	if !ok || strings.EqualFold(participle, simple) {
		return nil
	}
	return &lint.Lint{Span: t[2].Span, LintKind: lint.Grammar, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr(participle, t[2].GetCh(s))}, Message: fmt.Sprintf("Use the past participle `%s` instead of `%s` when using compound tenses or passive voice.", participle, simple), Priority: 127}
}
