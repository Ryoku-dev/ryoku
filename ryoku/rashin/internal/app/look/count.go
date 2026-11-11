package look

import (
	"fmt"

	"ryoku-i18n"
)

// Count is a number the way a sentence says it: a word from one to nine,
// digits from ten (proofs and facts keep their digits)
func Count(n int) string {
	words := [...]string{
		i18n.T("one"), i18n.T("two"), i18n.T("three"), i18n.T("four"), i18n.T("five"),
		i18n.T("six"), i18n.T("seven"), i18n.T("eight"), i18n.T("nine"),
	}
	if n >= 1 && n <= len(words) {
		return words[n-1]
	}
	return fmt.Sprint(n)
}

// CountStart is Count opening a sentence
func CountStart(n int) string {
	words := [...]string{
		i18n.T("One"), i18n.T("Two"), i18n.T("Three"), i18n.T("Four"), i18n.T("Five"),
		i18n.T("Six"), i18n.T("Seven"), i18n.T("Eight"), i18n.T("Nine"),
	}
	if n >= 1 && n <= len(words) {
		return words[n-1]
	}
	return fmt.Sprint(n)
}
