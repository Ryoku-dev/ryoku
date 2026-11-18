package rules

import (
	"math"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
)

// SpelledNumbers checks that small integers (less than ten) are spelled out.
type SpelledNumbers struct{}

func NewSpelledNumbers() *SpelledNumbers { return &SpelledNumbers{} }
func (*SpelledNumbers) Description() string {
	return "Most style guides recommend that you spell out numbers less than ten."
}
func (*SpelledNumbers) Lint(d *doc.Document) []lint.Lint {
	var result []lint.Lint
	for _, tok := range d.IterNumbers() {
		n, ok := tok.Kind.AsNumber()
		if !ok || n.Suffix != nil {
			continue
		}
		v := n.Value
		if math.Abs(v-math.Floor(v)) < 2.220446049250313e-16 && v < 10 {
			num := uint64(0)
			if v > 0 {
				num = uint64(v)
			}
			word, ok := spelledNumbersSpellOutNumber(num)
			if !ok {
				continue
			}
			result = append(result, lint.Lint{Span: tok.Span, LintKind: lint.Readability, Suggestions: []lint.Suggestion{lint.ReplaceWith([]rune(word))}, Message: "Try to spell out numbers less than ten.", Priority: 63})
		}
	}
	return result
}

// spelledNumbersSpellOutNumber converts numbers up to 999 to their spelled-out variant.
func spelledNumbersSpellOutNumber(n uint64) (string, bool) {
	if n > 999 {
		return "", false
	}
	units := [20]string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen"}
	tens := [10]string{"", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}
	if n < 20 {
		return units[n], true
	}
	if n >= 100 {
		parent, _ := spelledNumbersSpellOutNumber(n / 100)
		word := parent + " hundred"
		if n%100 != 0 {
			child, _ := spelledNumbersSpellOutNumber(n % 100)
			word += " " + child
		}
		return word, true
	}
	word := tens[n/10]
	if n%10 != 0 {
		word += "-" + units[n%10]
	}
	return word, true
}
