package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

const commaFixesSpaceBefore = "Don't use a space before a comma."
const commaFixesAvoidAsian = "Avoid East Asian commas in English contexts."
const commaFixesSpaceAfter = "Use a space after a comma."

// CommaFixes fixes missing spaces after commas, erroneous spaces before commas,
// and East Asian commas in English contexts. Commas between numbers are not checked.
type CommaFixes struct{}

func NewCommaFixes() *CommaFixes { return &CommaFixes{} }
func (*CommaFixes) Description() string {
	return "Fix common comma errors such as no space after, erroneous space before, etc., Asian commas instead of English commas, etc."
}
func (*CommaFixes) Lint(d *doc.Document) []lint.Lint {
	var result []lint.Lint
	toks, source := d.Tokens(), d.GetSource()
	for _, index := range doc.IterCommaIndices(toks) {
		tok := toks[index]
		chars := tok.GetCh(source)
		if len(chars) == 0 {
			continue
		}
		asian := chars[0] == '、' || chars[0] == '，'
		normal := chars[0] == ','
		kindAt := func(offset int) doc.TokenKind {
			at := index + offset
			if at < 0 || at >= len(toks) {
				return doc.TokenKind{}
			}
			return toks[at].Kind
		}
		previousWord := kindAt(-1).IsWord()
		beforeSpace := kindAt(-2).IsWord() && kindAt(-1).IsSpace()
		afterSpace := kindAt(1).IsSpace() && kindAt(2).IsWord()
		nextWord := kindAt(1).IsWord()
		span := tok.Span
		var suggestion lint.Suggestion
		var messages []string
		switch {
		case previousWord && asian && afterSpace:
			suggestion = lint.ReplaceWith([]rune{','})
			messages = []string{commaFixesAvoidAsian}
		case beforeSpace && normal && afterSpace:
			span = toks[index-1].Span
			suggestion = lint.Remove()
			messages = []string{commaFixesSpaceBefore}
		case beforeSpace && asian && afterSpace:
			span.Start = toks[index-1].Span.Start
			suggestion = lint.ReplaceWith([]rune{','})
			messages = []string{commaFixesSpaceBefore, commaFixesAvoidAsian}
		case previousWord && normal && nextWord:
			suggestion = lint.InsertAfter([]rune{' '})
			messages = []string{commaFixesSpaceAfter}
		case previousWord && asian && nextWord:
			suggestion = lint.ReplaceWith([]rune{',', ' '})
			messages = []string{commaFixesAvoidAsian, commaFixesSpaceAfter}
		case beforeSpace && normal && nextWord:
			span.Start = toks[index-1].Span.Start
			suggestion = lint.ReplaceWith([]rune{',', ' '})
			messages = []string{commaFixesSpaceBefore, commaFixesSpaceAfter}
		case beforeSpace && asian && nextWord:
			span.Start = toks[index-1].Span.Start
			suggestion = lint.ReplaceWith([]rune{',', ' '})
			messages = []string{commaFixesSpaceBefore, commaFixesAvoidAsian, commaFixesSpaceAfter}
		case asian:
			if (index > 0 && kindAt(-1).IsUnlintable()) || (index+1 < len(toks) && kindAt(1).IsUnlintable()) {
				continue
			}
			suggestion = lint.ReplaceWith([]rune{','})
			messages = []string{commaFixesAvoidAsian}
		default:
			continue
		}
		result = append(result, lint.Lint{Span: span, LintKind: lint.Punctuation, Suggestions: []lint.Suggestion{suggestion}, Message: strings.Join(messages, " "), Priority: 32})
	}
	return result
}
