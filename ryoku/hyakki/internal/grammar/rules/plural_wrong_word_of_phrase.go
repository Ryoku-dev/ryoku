package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"strings"
)

type PluralWrongWordOfPhrase struct{ expression expr.Expr }

func NewPluralWrongWordOfPhrase() *PluralWrongWordOfPhrase {
	var mistakes []expr.Expr
	for _, p := range pluralWrongWordOfPhrasePatterns {
		main, last := p.main+"s", p.last+"s"
		if p.mainPlural != "" {
			main = p.mainPlural
		}
		if p.lastPlural != "" {
			last = p.lastPlural
		}
		mistakes = append(mistakes, expr.NewSequenceExprAnyOf([]expr.Expr{expr.NewSequenceExprAco(p.main), expr.NewSequenceExprAco(main)}).TWsH().ThenFixedPhrase(p.mid).TWsH().TAco(last))
	}
	return &PluralWrongWordOfPhrase{expr.NewSequenceExprAnyOf(mistakes)}
}
func (r *PluralWrongWordOfPhrase) Expr() expr.Expr                  { return r.expression }
func (r *PluralWrongWordOfPhrase) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*PluralWrongWordOfPhrase) Description() string {
	return "Corrects noun phrases that pluralize the last noun instead of the main noun."
}
func (r *PluralWrongWordOfPhrase) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) < 2 {
		return nil
	}
	a, b := t[0].GetCh(s), t[len(t)-1].GetCh(s)
	for _, p := range pluralWrongWordOfPhrasePatterns {
		if !text.StartsWithIgnoreASCIICaseStr(a, p.main) || !text.StartsWithIgnoreASCIICaseStr(b, p.last) {
			continue
		}
		main := p.mainPlural
		if main == "" {
			main = p.main + "s"
		}
		sep := " "
		mid := strings.ReplaceAll(p.mid, "-", " ")
		if t[1].Kind.IsHyphen() {
			sep = "-"
			mid = strings.ReplaceAll(p.mid, " ", "-")
		}
		value := main + sep + mid + sep + p.last
		span, _ := doc.Span(t)
		return &lint.Lint{Span: span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr(value, span.GetContent(s))}, Message: "This phrase is pluralized on the main noun, not on the last noun.", Priority: 127}
	}
	return nil
}

type pluralWrongWordOfPhrasePattern struct{ main, mainPlural, mid, last, lastPlural string }

var pluralWrongWordOfPhrasePatterns = []pluralWrongWordOfPhrasePattern{{"body", "bodies", "in", "white", ""}, {"flash", "flashes", "in the", "pan", ""}, {"flash", "flashes", "in-the", "pan", ""}, {"line", "", "of", "code", ""}, {"moment", "", "of", "truth", ""}, {"part", "", "of", "speech", "speeches"}, {"point", "", "of", "view", ""}, {"proof", "", "of", "concept", ""}, {"rule", "", "of", "thumb", ""}}
