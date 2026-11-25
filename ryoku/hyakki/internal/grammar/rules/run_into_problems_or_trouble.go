package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"unicode"
)

type RunIntoProblemsOrTrouble struct{ expression expr.Expr }

func NewRunIntoProblemsOrTrouble() *RunIntoProblemsOrTrouble {
	return &RunIntoProblemsOrTrouble{expression: expr.NewSequenceExprWordSet([]string{"run", "ran", "running", "runs"}).TWs().TAco("into").TWs().ThenAnyOf([]expr.Expr{expr.NewSequenceExprWordSet([]string{"problem", "troubles"}), expr.NewSequenceExprWordSeq([]string{"a", "trouble"}), expr.NewSequenceExprWordSeq([]string{"further", "troubles"})})}
}
func (r *RunIntoProblemsOrTrouble) Expr() expr.Expr                  { return r.expression }
func (r *RunIntoProblemsOrTrouble) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *RunIntoProblemsOrTrouble) Description() string {
	return "Corrects `running into` `problems` or `trouble` with wrong article, singular, or plural forms."
}
func (r *RunIntoProblemsOrTrouble) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	four := false
	switch len(toks) {
	case 5:
	case 7:
		four = true
	default:
		return nil
	}
	from := 4
	if four && !text.EqStr(toks[4].GetCh(src), "a") {
		from = 6
	}
	span, ok := doc.Span(toks[from:])
	if !ok {
		return nil
	}
	noun := toks[len(toks)-1].GetCh(src)
	if len(noun) == 0 {
		return nil
	}
	first, last := unicode.ToLower(noun[0]), unicode.ToLower(noun[len(noun)-1])
	var replacements []string
	var msg string
	switch {
	case !four && first == 'p' && last == 'm':
		replacements = []string{"problems", "a problem"}
		msg = "The noun `problem` is countable. Use the plural `problems` or add the article `a`."
	case four && first == 't' && last == 'e':
		replacements = []string{"trouble"}
		msg = "The noun `trouble` is uncountable. Drop the article `a`."
	case first == 't' && last == 's':
		replacements = []string{"trouble"}
		msg = "The idiom is `run into trouble`. Use the singular form."
	default:
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Usage
	l.Suggestions = lint.ToReplaceSuggestions(replacements, span.GetContent(src))
	l.Message = msg
	return &l
}
