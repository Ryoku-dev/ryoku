package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"unicode"
)

// OfCourse corrects mistaken forms of "of course" while ignoring valid phrases
// like "kind of curse".
type OfCourse struct{ expression expr.Expr }

func NewOfCourse() *OfCourse {
	return &OfCourse{expr.NewLongestMatchOf([]expr.Expr{expr.NewSequenceExprAco("of").TWs().TSet([]string{"curse", "corse"}), expr.NewSequenceExprAco("off").TWs().TSet([]string{"course", "coarse"}), expr.NewSequenceExprAco("o").TWs().TAco("course"), expr.AsExpr(expr.NewWordSet([]string{"ofcourse"}))})}
}
func (r *OfCourse) Expr() expr.Expr                  { return r.expression }
func (r *OfCourse) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*OfCourse) Description() string {
	return "Corrects common mistaken forms of `of course`, including `of curse`, `off course`, and `ofcourse`, while ignoring valid phrases like `kind of curse`."
}
func (r *OfCourse) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	phrase := span.GetContent(s)
	if text.EqStr(phrase, "of curse") || text.EqStr(phrase, "of corse") {
		prev := ofCoursePrecedingWord(s, span.Start)
		if prev == "kind" || prev == "sort" {
			return nil
		}
	}
	if text.EqStr(phrase, "off course") {
		for i := span.Start - 1; i >= 0; i-- {
			if !unicode.IsSpace(s[i]) {
				if unicode.IsLetter(s[i]) || unicode.IsNumber(s[i]) {
					return nil
				}
				break
			}
		}
	}
	return &lint.Lint{Span: span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("of course", span.GetContent(s))}, Message: "Did you mean `of course`?", Priority: 31}
}
func ofCoursePrecedingWord(s []rune, offset int) string {
	if offset > len(s) {
		return ""
	}
	i := offset - 1
	for i >= 0 && unicode.IsSpace(s[i]) {
		i--
	}
	if i < 0 {
		return ""
	}
	end := i + 1
	for i >= 0 && !unicode.IsSpace(s[i]) {
		i--
	}
	value := append([]rune(nil), s[i+1:end]...)
	for i, c := range value {
		if c >= 'A' && c <= 'Z' {
			value[i] = c + 'a' - 'A'
		}
	}
	return string(value)
}
