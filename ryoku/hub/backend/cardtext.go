package main

import (
	"strings"
	"unicode"
)

func cleanCardText(s string) string {
	return strings.Map(func(r rune) rune {
		// Invisible direction and join controls let a name impersonate another card.
		if unicode.IsControl(r) || r == '\u061c' || r == '\u200e' || r == '\u200f' ||
			(r >= '\u202a' && r <= '\u202e') || (r >= '\u2066' && r <= '\u2069') ||
			(r >= '\u200b' && r <= '\u200d') || r == '\u2060' || r == '\ufeff' {
			return -1
		}
		return r
	}, s)
}

func validGalleryName(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
