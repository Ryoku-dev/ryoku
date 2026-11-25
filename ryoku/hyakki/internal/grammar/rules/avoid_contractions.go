package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type AvoidContractions struct{ expression expr.Expr }

func NewAvoidContractions() *AvoidContractions {
	e := expr.AsExpr(expr.NewWordSet([]string{"aren't", "can't", "could've", "couldn't", "couldn't've", "didn't", "doesn't", "don't", "hadn't", "hasn't", "haven't", "he'll", "how're", "i'll", "i'm", "i've", "isn't", "it'll", "mayn't", "might've", "mightn't", "must've", "mustn't", "needn't", "oughtn't", "shan't", "she'll", "should've", "shouldn't", "that'll", "they'll", "they're", "they've", "wasn't", "we'll", "we're", "we've", "weren't", "what'll", "who'll", "who're", "who've", "won't", "would've", "wouldn't", "you'll", "you're", "you've"}))
	return &AvoidContractions{expression: e}
}
func (r *AvoidContractions) Expr() expr.Expr { return r.expression }
func (r *AvoidContractions) Description() string {
	return "Suggests expanded forms for common contractions, such as `isn't` → `is not` and `we're` → `we are`."
}
func (r *AvoidContractions) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *AvoidContractions) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	tok := t[0]
	normalized := strings.ToLower(strings.ReplaceAll(tok.GetStr(s), "’", "'"))
	expansion, ok := avoidContractionsExpansions[normalized]
	if !ok {
		return nil
	}
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.Style
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(expansion, tok.GetCh(s))}
	l.Message = "Consider expanding this contraction."
	l.Priority = 63
	return &l
}

var avoidContractionsExpansions = map[string]string{"aren't": "are not", "can't": "cannot", "could've": "could have", "couldn't": "could not", "couldn't've": "could not have", "didn't": "did not", "doesn't": "does not", "don't": "do not", "hadn't": "had not", "hasn't": "has not", "haven't": "have not", "he'll": "he will", "how're": "how are", "i'll": "i will", "i'm": "i am", "i've": "i have", "isn't": "is not", "it'll": "it will", "mayn't": "may not", "might've": "might have", "mightn't": "might not", "must've": "must have", "mustn't": "must not", "needn't": "need not", "oughtn't": "ought not", "shan't": "shall not", "she'll": "she will", "should've": "should have", "shouldn't": "should not", "that'll": "that will", "they'll": "they will", "they're": "they are", "they've": "they have", "wasn't": "was not", "we'll": "we will", "we're": "we are", "we've": "we have", "weren't": "were not", "what'll": "what will", "who'll": "who will", "who're": "who are", "who've": "who have", "won't": "will not", "would've": "would have", "wouldn't": "would not", "you'll": "you will", "you're": "you are", "you've": "you have"}
