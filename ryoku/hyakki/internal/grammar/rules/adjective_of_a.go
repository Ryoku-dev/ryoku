package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"strings"
)

// Detect sequences of words of the form "adjective of a".
type AdjectiveOfA struct{}

func NewAdjectiveOfA() *AdjectiveOfA { return &AdjectiveOfA{} }

var adjectiveOfAWhitelist = []string{"bad", "big", "good", "large", "long", "vague"}
var adjectiveOfAContextWords = []string{"as", "how", "that", "this", "too"}
var adjectiveOfABlacklist = []string{"much", "part"}

func adjectiveOfAHasContextWord(d *doc.Document, idx int) bool {
	if idx < 2 {
		return false
	}
	toks := d.Tokens()
	if !toks[idx-1].Kind.IsWhitespace() || !toks[idx-2].Kind.IsWord() {
		return false
	}
	return text.EqAnyIgnoreASCIICaseStr(toks[idx-2].GetCh(d.GetSource()), adjectiveOfAContextWords)
}
func (*AdjectiveOfA) Description() string {
	return "This rule looks for sequences of words of the form `adjective of a`."
}
func (*AdjectiveOfA) Lint(d *doc.Document) []lint.Lint {
	var out []lint.Lint
	toks := d.Tokens()
	src := d.GetSource()
	for _, i := range doc.IterAdjectiveIndices(toks) {
		adjective := toks[i]
		adj := strings.ToLower(adjective.GetStr(src))
		if !text.EqAnyIgnoreASCIICaseStr([]rune(adj), adjectiveOfAWhitelist) && !adjectiveOfAHasContextWord(d, i) {
			continue
		}
		if text.EqAnyIgnoreASCIICaseStr([]rune(adj), adjectiveOfABlacklist) {
			continue
		}
		if strings.HasSuffix(adj, "er") || strings.HasSuffix(adj, "st") {
			continue
		}
		if strings.HasSuffix(adj, "ing") && (adjective.Kind.IsNoun() || adjective.Kind.IsVerb()) {
			continue
		}
		if i+4 >= len(toks) {
			continue
		}
		space1, of, space2, article := toks[i+1], toks[i+2], toks[i+3], toks[i+4]
		if !space1.Kind.IsWhitespace() || !of.Kind.IsWord() || strings.ToLower(of.GetStr(src)) != "of" || !space2.Kind.IsWhitespace() || !article.Kind.IsWord() {
			continue
		}
		articleStr := strings.ToLower(article.GetStr(src))
		if articleStr != "a" && articleStr != "an" {
			continue
		}
		first := adjective.GetStr(src) + space1.GetStr(src) + article.GetStr(src)
		second := adjective.GetStr(src) + space2.GetStr(src) + article.GetStr(src)
		suggestions := []lint.Suggestion{lint.ReplaceWith([]rune(first))}
		if first != second {
			suggestions = append(suggestions, lint.ReplaceWith([]rune(second)))
		}
		out = append(out, lint.Lint{Span: text.Span{Start: adjective.Span.Start, End: article.Span.End}, LintKind: lint.Style, Suggestions: suggestions, Message: "The word `of` is not needed here.", Priority: 63})
	}
	return out
}
