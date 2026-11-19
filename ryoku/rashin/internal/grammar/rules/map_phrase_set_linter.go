package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type MapPhraseSetLinter struct {
	description                        string
	expression                         *expr.LongestMatchOf
	wrongFormsToCorrectForms           [][2]string
	multiWrongFormsToMultiCorrectForms [][2][]string
	message                            string
	lintKind                           lint.LintKind
}

func NewMapPhraseSetLinterOneToOne(pairs [][2]string, message, description string, kind *lint.LintKind) *MapPhraseSetLinter {
	r := &MapPhraseSetLinter{description: description, expression: expr.NewLongestMatchOf(), wrongFormsToCorrectForms: pairs, message: message, lintKind: lint.Miscellaneous}
	if kind != nil {
		r.lintKind = *kind
	}
	for _, pair := range pairs {
		r.expression.Add(expr.NewFixedPhraseFromPhrase(pair[0]))
	}
	return r
}

func NewMapPhraseSetLinterManyToMany(pairs [][2][]string, message, description string, kind *lint.LintKind) *MapPhraseSetLinter {
	r := &MapPhraseSetLinter{description: description, expression: expr.NewLongestMatchOf(), multiWrongFormsToMultiCorrectForms: pairs, message: message, lintKind: lint.Miscellaneous}
	if kind != nil {
		r.lintKind = *kind
	}
	for _, pair := range pairs {
		for _, wrong := range pair[0] {
			r.expression.Add(expr.NewFixedPhraseFromPhrase(wrong))
		}
	}
	return r
}

func (r *MapPhraseSetLinter) Expr() expr.Expr                  { return r.expression }
func (r *MapPhraseSetLinter) Description() string              { return r.description }
func (r *MapPhraseSetLinter) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *MapPhraseSetLinter) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
	span, ok := doc.Span(tokens)
	if !ok {
		return nil
	}
	matched := span.GetContent(source)
	var suggestions []lint.Suggestion
	for _, pair := range r.wrongFormsToCorrectForms {
		if text.EqStr(matched, pair[0]) {
			suggestions = append(suggestions, lint.ReplaceWithMatchCaseStr(pair[1], matched))
		}
	}
	for _, pair := range r.multiWrongFormsToMultiCorrectForms {
		for _, wrong := range pair[0] {
			if text.EqStr(matched, wrong) {
				for _, correct := range pair[1] {
					suggestions = append(suggestions, lint.ReplaceWithMatchCaseStr(correct, matched))
				}
			}
		}
	}
	if len(suggestions) == 0 {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: r.lintKind, Suggestions: suggestions, Message: r.message, Priority: 31}
}
