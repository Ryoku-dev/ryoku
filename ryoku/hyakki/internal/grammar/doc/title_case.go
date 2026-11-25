package doc

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/text"
	"slices"
	"strings"
	"unicode"
)

func isTimeAmPm(index int, tokens []Token, source []rune) bool {
	w := tokens[index].GetStr(source)
	if !strings.EqualFold(w, "am") && !strings.EqualFold(w, "pm") {
		return false
	}
	for i := index - 1; i >= 0; i-- {
		k := tokens[i].Kind
		if k.IsWord() {
			return false
		}
		if k.IsNumber() {
			return true
		}
		if !k.IsSpace() && !k.isPunct(text.Colon) {
			break
		}
	}
	return false
}
func MakeTitleCaseStr(source string, p Parser, dictionary dict.Dictionary) string {
	return string(MakeTitleCaseChars([]rune(source), p, dictionary))
}
func MakeTitleCaseChars(source []rune, p Parser, dictionary dict.Dictionary) []rune {
	document := NewFromChars(source, p, dictionary)
	return MakeTitleCase(document.GetTokens(), source, dictionary)
}
func TryMakeTitleCase(tokens []Token, source []rune, dictionary dict.Dictionary) ([]rune, bool) {
	if len(tokens) == 0 {
		return nil, false
	}
	span, _ := Span(tokens)
	relevant := source[span.Start:span.End]
	var out []rune
	set := func(i int, c rune) {
		if (out != nil && out[i] != c) || relevant[i] != c {
			if out == nil {
				out = slices.Clone(relevant)
			}
			out[i] = c
		}
	}
	words := IterWordLikeIndices(tokens)
	previous := 0
	seen := false
	for wi, index := range words {
		word := tokens[index]
		alphabetic := false
		for _, c := range word.GetCh(source) {
			alphabetic = alphabetic || unicode.IsLetter(c)
		}
		if isTimeAmPm(index, tokens, source) {
			seen = seen || alphabetic
			continue
		}
		camel := false
		if word.Kind.IsProperNoun() {
			if correct, ok := dictionary.GetCorrectCapitalizationOf(word.GetCh(source)); ok {
				for i, c := range correct {
					if unicode.IsLetter(c) {
						set(word.Span.Start-span.Start+i, c)
					}
				}
				camel = word.Kind.IsLowerCamel()
			}
		}
		afterColon := false
		for _, t := range tokens[previous:index] {
			afterColon = afterColon || t.Kind.isPunct(text.Colon)
		}
		capitalize := afterColon || shouldCapitalizeToken(word, source) || alphabetic && !seen || wi == len(words)-1
		if !camel {
			if capitalize {
				c := relevant[word.Span.Start-span.Start]
				if c >= 'a' && c <= 'z' {
					c -= 32
				}
				set(word.Span.Start-span.Start, c)
			} else {
				for i := word.Span.Start; i < word.Span.End; i++ {
					c := relevant[i-span.Start]
					if c >= 'A' && c <= 'Z' {
						c += 32
					}
					set(i-span.Start, c)
				}
			}
		}
		seen = seen || alphabetic
		previous = index
	}
	if slices.Equal(out, relevant) {
		return nil, false
	}
	return out, out != nil
}
func MakeTitleCase(tokens []Token, source []rune, dictionary dict.Dictionary) []rune {
	if out, ok := TryMakeTitleCase(tokens, source, dictionary); ok {
		return out
	}
	s, _ := Span(tokens)
	return slices.Clone(source[s.Start:s.End])
}
func shouldCapitalizeToken(t Token, source []rune) bool {
	if !t.Kind.IsWord() || t.Kind.Metadata == nil {
		return true
	}
	w := strings.ToLower(t.GetStr(source))
	if w == "all" {
		return true
	}
	short := t.Kind.IsPreposition() && t.Span.End-t.Span.Start <= 4
	return !short && !slices.Contains([]string{"and", "but", "for", "or", "nor", "as", "a", "an", "the"}, w) && t.GetStr(source) != "x"
}
func MostUsedDialects(d *Document) dict.DialectFlags {
	dialects := []dict.Dialect{dict.American, dict.Canadian, dict.Australian, dict.British, dict.Indian}
	counts := make([]int, len(dialects))
	maximum := 0
	for _, word := range IterWords(d.tokens) {
		if word.Kind.Metadata != nil {
			for i, dialect := range dialects {
				if word.Kind.Metadata.Dialects.IsDialectEnabled(dialect) {
					counts[i]++
					if counts[i] > maximum {
						maximum = counts[i]
					}
				}
			}
		}
	}
	var flags dict.DialectFlags
	for i, n := range counts {
		if n == maximum {
			flags |= dict.DialectFlagsFromDialect(dialects[i])
		}
	}
	return flags
}
func TryGuessDialect(d *Document) (dict.Dialect, bool) {
	return dict.DialectFromFlags(MostUsedDialects(d))
}
