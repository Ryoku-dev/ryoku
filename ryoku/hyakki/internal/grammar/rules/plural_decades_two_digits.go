package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

func pluralDecadesTwoDigitsMatchToLint(t []doc.Token, s, decade, suffix []rune, before, after []doc.Token) *lint.Lint {
	get := func(ts []doc.Token, offset int) *doc.Token {
		tok, ok := doc.GetRel(ts, offset)
		if !ok {
			return nil
		}
		return tok
	}
	whitespace := func(tok *doc.Token) bool { return tok != nil && tok.Kind.IsWhitespace() }
	sep := func(tok *doc.Token) bool { return tok != nil && (tok.Kind.IsWhitespace() || tok.Kind.IsHyphen()) }
	word := func(tok *doc.Token, w string) bool {
		return tok != nil && tok.Kind.IsWord() && text.EqStr(tok.GetCh(s), w)
	}
	plus := func(tok *doc.Token) bool { return tok != nil && tok.Kind.IsPlus() }
	// 0: unsure, 1: not a mistake, 2: decade, 3: age range.
	judge := func() int {
		p1, p2, p3, p4 := get(before, -1), get(before, -2), get(before, -3), get(before, -4)
		if p1 != nil {
			if whitespace(p1) && p2 != nil && (p2.Kind.IsWhitespace() || p2.Kind.IsHyphen() || p2.Kind.IsPlus() || p2.Kind.IsWord()) {
				if word(p2, "the") && whitespace(p3) && p4 != nil && p4.Kind.IsPreposition() {
					return 2
				}
				if p2.Kind.IsPossessiveDeterminer() {
					return 3
				}
				if text.EqStr(decade, "10") && (word(p2, "windows") || word(p2, "xcode") || word(p2, "android")) {
					return 1
				}
			}
			if sep(p1) && (word(p2, "early") || word(p2, "mid") || word(p2, "late")) {
				if whitespace(p3) && p4 != nil && p4.Kind.IsPossessiveDeterminer() {
					return 3
				}
				return 2
			}
			if plus(p1) && text.EqStr(decade, "20") && plus(p2) && word(p3, "c") {
				return 1
			}
		}
		if sep(get(after, 0)) && word(get(after, 1), "style") {
			return 2
		}
		return 0
	}
	judgment := judge()
	if judgment == 1 {
		return nil
	}
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Usage
	l.Message = "To refer to a decade the apostrophe must be before the decade. To refer to an age range, use no apostrophe."
	replacement := append([]rune(nil), decade...)
	replacement = append(replacement, suffix...)
	if judgment == 2 {
		l.Suggestions = []lint.Suggestion{lint.ReplaceWith(append([]rune{'\''}, replacement...))}
	} else if judgment == 3 {
		l.Suggestions = []lint.Suggestion{lint.ReplaceWith(replacement)}
	}
	return &l
}
