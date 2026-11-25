package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type TheyreConfusions struct{ *mergeLintersLinter }

func NewTheyreConfusions() *TheyreConfusions {
	return &TheyreConfusions{&mergeLintersLinter{rules: []lint.Linter{NewOverTheyreToThere(), NewTypographicTheyreToTheir()}, description: "Detects apostrophe and locative edge cases that are awkward to model with standard contraction checks."}}
}

func theyreConfusionsTokenIsTheyre(token doc.Token, source []rune) bool {
	return token.Kind.IsWord() && string(text.Normalized(token.GetCh(source))) == "they're"
}
func theyreConfusionsTokenIsTypographicTheyre(token doc.Token, source []rune) bool {
	if !token.Kind.IsWord() {
		return false
	}
	chars := token.GetCh(source)
	normalized := text.Normalized(chars)
	return string(normalized) == "they're" && string(chars) != string(normalized)
}
func theyreConfusionsTokenIsLikelyTheirPossession(token doc.Token, source []rune) bool {
	if !token.Kind.IsWord() {
		return false
	}
	switch string(text.Normalized(token.GetCh(source))) {
	case "backpacks", "patience", "dog", "proposal", "laughter", "ladder", "apartment", "mittens", "answer", "sketches", "server", "backup", "evidence", "garden", "maps", "team", "past", "needs", "pawn", "ability", "return", "hearing", "house", "coats", "problems", "understanding", "new":
		return true
	}
	return false
}
