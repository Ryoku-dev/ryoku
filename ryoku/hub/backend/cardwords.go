package main

import (
	"encoding/json"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// A badge holder's card carries their own words to friends' binders: the motto and, on the
// Stand card, the Stand's name. Both are signed into the record and drawn as plain text, so
// they are held to the profile's plain-text rule and more: one canonical spelling (NFC), no
// invisible characters that could make two different words look alike, nothing at the ends.
const (
	recordTaglineLimit   = 128
	recordStandNameLimit = 64
	// the stock motto travels as nothing: a binder draws its own, in its reader's language
	recordStockTagline = "For the sake of power and beauty."
)

// recordLook is what a badge holder's record carries beyond the card code: its built-in style
// and its owner's words. The zero value is a classic card with Ryoku's words, a v2 record.
type recordLook struct {
	Style     string
	Tagline   string
	StandName string
}

func (l recordLook) words() bool { return l.Tagline != "" || l.StandName != "" }

// version is the record version the look needs: v2 classic, v3 a style alone (which binders
// from before words still read), v4 any words
func (l recordLook) version() int {
	switch {
	case l.words():
		return 4
	case l.Style != "":
		return 3
	}
	return 2
}

// hiddenWordRunes look like nothing, or like a space, and would let two different words read alike
const hiddenWordRunes = "<>&\ufffd\u115f\u1160\u3164\uffa0"

func noncharacter(r rune) bool {
	return r >= 0xfdd0 && r <= 0xfdef || r&0xfffe == 0xfffe
}

// cardWordsValid is the rule the gate applies to a record's words, rune for rune
func cardWordsValid(s string, limit int) bool {
	if s == "" || !utf8.ValidString(s) || utf8.RuneCountInString(s) > limit || !norm.NFC.IsNormalString(s) {
		return false
	}
	first, _ := utf8.DecodeRuneInString(s)
	last, _ := utf8.DecodeLastRuneInString(s)
	if unicode.Is(unicode.Zs, first) || unicode.Is(unicode.Zs, last) {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp, unicode.Co, unicode.Cs) || noncharacter(r) || strings.ContainsRune(hiddenWordRunes, r) {
			return false
		}
	}
	return true
}

// cardWordsFrom turns a profile's words into a record's: trimmed, in NFC, cut to limit
// characters; words that still break the rule travel as nothing rather than stop the card
func cardWordsFrom(s string, limit int) string {
	s = norm.NFC.String(strings.TrimFunc(s, func(r rune) bool { return unicode.Is(unicode.Zs, r) }))
	if utf8.RuneCountInString(s) > limit {
		s = string([]rune(s)[:limit])
	}
	s = strings.TrimRightFunc(s, func(r rune) bool { return unicode.Is(unicode.Zs, r) })
	if !cardWordsValid(s, limit) {
		return ""
	}
	return s
}

// recordLookFor is the look this machine's published card wears: a badge holder's style and
// words from the profile; anyone else's card goes out classic with Ryoku's words
func recordLookFor() recordLook {
	raw, e := readProfileFile(profileConfigPath(), maxProfileBytes)
	var p struct {
		Style string `json:"style"`
		Text  struct {
			Tagline   string `json:"tagline"`
			StandName string `json:"standName"`
		} `json:"text"`
	}
	if e != nil || json.Unmarshal(raw, &p) != nil || !badgeHolder() {
		return recordLook{}
	}
	var look recordLook
	if slices.Contains(recordStyles, p.Style) {
		look.Style = p.Style
	}
	if tagline := cardWordsFrom(p.Text.Tagline, recordTaglineLimit); tagline != recordStockTagline {
		look.Tagline = tagline
	}
	// only the Stand card names a Stand; a classic card sending one would only cost old binders
	if look.Style == "stand" {
		look.StandName = cardWordsFrom(p.Text.StandName, recordStandNameLimit)
	}
	return look
}
