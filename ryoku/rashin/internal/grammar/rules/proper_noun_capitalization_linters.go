package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

// ProperNounCapitalizationLinter corrects multi-word proper nouns to their supplied canonical capitalization.
// Additions to the canonical proper nouns belong in proper_noun_rules.json.
type ProperNounCapitalizationLinter struct {
	patternMap  *expr.ExprMap[*doc.Document]
	description string
}

// NewProperNounCapitalizationLinter corrects the capitalization of the provided phrases.
func NewProperNounCapitalizationLinter(canonicalVersions []string, description string) *ProperNounCapitalizationLinter {
	patterns := expr.NewExprMap[*doc.Document]()
	for _, canonical := range canonicalVersions {
		d := doc.NewBasicTokenize(canonical, doc.PlainEnglish{})
		patterns.Insert(expr.NewFixedPhraseFromDocument(d), d)
	}
	return &ProperNounCapitalizationLinter{patternMap: patterns, description: description}
}
func (r *ProperNounCapitalizationLinter) Expr() expr.Expr     { return r.patternMap }
func (r *ProperNounCapitalizationLinter) Description() string { return r.description }
func (r *ProperNounCapitalizationLinter) Lint(d *doc.Document) []lint.Lint {
	return lint.RunExprLinter(r, d)
}
func (r *ProperNounCapitalizationLinter) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	canonical, ok := r.patternMap.Lookup(0, toks, src)
	if !ok {
		return nil
	}
	broken := false
	for i, t := range canonical.FatTokens() {
		if i >= len(toks) {
			break
		}
		if toks[i].GetStr(src) != string(t.Content) {
			broken = true
			break
		}
	}
	if !broken {
		return nil
	}
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Capitalization, Suggestions: []lint.Suggestion{lint.ReplaceWith(canonical.GetSource())}, Message: r.description, Priority: 31}
}
func properNounCapitalizationLintersLintGroupFromJSON(data []byte) *lint.LintGroup {
	entries := properNounCapitalizationLintersReadEntries(data)
	group := lint.NewLintGroup()
	for name, entry := range entries {
		group.AddChunkExprLinter(name, NewProperNounCapitalizationLinter(entry.Canonical, entry.Description))
	}
	enabled := true
	group.SetAllRulesTo(&enabled)
	return group
}
func NewProperNounCapitalizationLintersLintGroup() *lint.LintGroup {
	return properNounCapitalizationLintersLintGroupFromJSON(properNounCapitalizationLintersData)
}
