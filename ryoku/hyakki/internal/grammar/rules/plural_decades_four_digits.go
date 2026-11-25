package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

func pluralDecadesFourDigitsMatchToLint(t []doc.Token, s, decade, suffix []rune, pre, post []doc.Token) *lint.Lint {
	var before, after *doc.Token
	beforeHyphen := false
	if len(pre) >= 2 {
		sep, pw := pre[len(pre)-1], &pre[len(pre)-2]
		if (sep.Kind.IsWhitespace() || sep.Kind.IsHyphen()) && pw.Kind.IsWord() {
			before = pw
			beforeHyphen = sep.Kind.IsHyphen()
		}
	}
	if len(post) >= 2 {
		sep, nw := post[0], &post[1]
		if (sep.Kind.IsWhitespace() || sep.Kind.IsHyphen()) && nw.Kind.IsWord() {
			after = nw
		}
	}
	mistake := false
	switch {
	case before != nil && text.EqAnyIgnoreASCIICaseStr(before.GetCh(s), []string{"early", "mid", "late"}):
		mistake = true
	case before != nil && beforeHyphen:
		return nil
	case after != nil && text.EqStr(after.GetCh(s), "style"):
		mistake = true
	case before != nil && !beforeHyphen && text.EqStr(before.GetCh(s), "the"):
		mistake = len(pre) >= 4 && pre[len(pre)-3].Kind.IsWhitespace() && pre[len(pre)-4].Kind.IsPreposition()
	}
	if !mistake {
		return nil
	}
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Usage
	l.Message = "Plural decades do not use an apostrophe before the `s`"
	replacement := append([]rune(nil), decade...)
	replacement = append(replacement, suffix...)
	l.Suggestions = []lint.Suggestion{lint.ReplaceWith(replacement)}
	return &l
}
