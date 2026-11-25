package sensitive

import (
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// Kind is the sort of number Number finds.
type Kind string

// The numbers memory never keeps.
const (
	None    Kind = ""
	Card    Kind = "card"    // a payment card number that passes the Luhn checksum
	ID      Kind = "id"      // a social security or national identity number
	Account Kind = "account" // a bank account number
)

// Shapes of the numbers. Each is checked further in numbers: a checksum
// where the number has one, the ranges a real one uses where it has none,
// and digit-only shapes must stand alone, not inside a longer digit group.
var (
	// cardDigits is a run of 13 to 19 digits, single spaces or hyphens
	// allowed between them, the length of a payment card number.
	cardDigits = regexp.MustCompile(`\d(?:[ \-]?\d){12,18}`)
	// ssnShape is a United States social security number, 123-45-6789.
	ssnShape = regexp.MustCompile(`\b(\d{3})[- ](\d{2})[- ](\d{4})\b`)
	// ninoShape is a United Kingdom national insurance number, AB123456C.
	ninoShape = regexp.MustCompile(`\b([A-CEGHJ-PR-TW-Z][A-CEGHJ-NPR-TW-Z]) ?\d{2} ?\d{2} ?\d{2} ?[A-D]\b`)
	// dniShape is a Spanish DNI (12345678Z) or NIE (X1234567L).
	dniShape = regexp.MustCompile(`(?i)\b([XYZ]-?\d{7}|\d{8})-?([A-Z])\b`)
	// cpfShape is a Brazilian CPF, 111.444.777-35.
	cpfShape = regexp.MustCompile(`\b\d{3}\.\d{3}\.\d{3}-\d{2}\b`)
	// aadhaarShape is an Indian Aadhaar number, 2345 6789 0124.
	aadhaarShape = regexp.MustCompile(`\b[2-9]\d{3}[ -]\d{4}[ -]\d{4}\b`)
	// nirShape is a French social security number, 1 85 05 78 006 084 91.
	nirShape = regexp.MustCompile(`\b[12] ?\d{2} ?\d{2} ?(?:\d{2}|2[AB]) ?\d{3} ?\d{3} ?\d{2}\b`)
	// ibanShape is an international bank account number, GB82 WEST 1234
	// 5698 7654 32, grouped in fours or written together.
	ibanShape = regexp.MustCompile(`\b[A-Z]{2}\d{2}(?: ?[A-Z0-9]{4}){2,7}(?: ?[A-Z0-9]{1,3})?\b`)
	// namedRun is six or more digits, single spaces or hyphens allowed
	// between them, in text that names an identity, card or account number.
	namedRun = regexp.MustCompile(`\d(?:[ \-]?\d){5,}`)
	// dateShape is a date written with hyphens, which a named run skips.
	dateShape = regexp.MustCompile(`^(?:\d{4}-\d{1,2}-\d{1,2}|\d{1,2}-\d{1,2}-\d{2,4})$`)
)

// named is the kind of number a private-field word names, for a run of
// digits beside it.
var named = map[string]Kind{
	whatCard: Card, whatCVV: Card,
	whatBank: Account, whatRouting: Account, whatSortCode: Account,
	whatSSN: ID, whatNI: ID, whatID: ID, whatPassport: ID, whatLicence: ID, whatTax: ID,
}

// span is one number found in a text.
type span struct {
	start, end int
	kind       Kind
}

// Number reports the first card, identity or bank account number in s, or
// None. It finds a Luhn-valid card number; a United States social security
// number; a United Kingdom national insurance number; a Spanish DNI or NIE,
// a Brazilian CPF, an Indian Aadhaar number or a French social security
// number with a valid check; an IBAN with a valid check; and any run of six
// or more digits in a text that names such a number ("my passport number
// is 123456789").
func Number(s string) Kind {
	if found := numbers(s); len(found) > 0 {
		return found[0].kind
	}
	return None
}

// Mask hides every number Number finds in s, letter and digit alike, and
// keeps the separators; a card keeps its last four digits so its owner can
// tell which one it is. The rest of s is unchanged.
func Mask(s string) string {
	found := numbers(s)
	if len(found) == 0 {
		return s
	}
	var b strings.Builder
	at := 0
	for _, sp := range found {
		if sp.start < at {
			sp.start = at
		}
		if sp.end <= sp.start {
			continue
		}
		b.WriteString(s[at:sp.start])
		b.WriteString(maskSpan(s[sp.start:sp.end], sp.kind == Card))
		at = sp.end
	}
	b.WriteString(s[at:])
	return b.String()
}

// maskSpan replaces each letter and digit of one number with a dot, keeping
// the last four digits when keepLast.
func maskSpan(s string, keepLast bool) string {
	keep := 0
	if keepLast {
		keep = 4
	}
	out := []rune(s)
	for i := len(out) - 1; i >= 0; i-- {
		r := out[i]
		if !isAlnum(r) {
			continue
		}
		if keep > 0 && r >= '0' && r <= '9' {
			keep--
			continue
		}
		out[i] = '•'
	}
	return string(out)
}

func isAlnum(r rune) bool {
	return r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z'
}

// numbers finds every number in s, in order of where it starts.
func numbers(s string) []span {
	var out []span
	add := func(start, end int, kind Kind) { out = append(out, span{start, end, kind}) }
	for _, m := range cardDigits.FindAllStringIndex(s, -1) {
		if luhn(s[m[0]:m[1]]) {
			add(m[0], m[1], Card)
		}
	}
	for _, m := range ssnShape.FindAllStringSubmatchIndex(s, -1) {
		area, group, serial := s[m[2]:m[3]], s[m[4]:m[5]], s[m[6]:m[7]]
		if area != "000" && area != "666" && area[0] != '9' && group != "00" && serial != "0000" && alone(s, m[0], m[1]) {
			add(m[0], m[1], ID)
		}
	}
	for _, m := range ninoShape.FindAllStringSubmatchIndex(s, -1) {
		switch s[m[2]:m[3]] {
		case "BG", "GB", "KN", "NK", "NT", "TN", "ZZ":
		default:
			add(m[0], m[1], ID)
		}
	}
	for _, m := range dniShape.FindAllStringSubmatchIndex(s, -1) {
		if dni(s[m[2]:m[3]], s[m[4]:m[5]]) && alone(s, m[0], m[1]) {
			add(m[0], m[1], ID)
		}
	}
	for _, m := range cpfShape.FindAllStringIndex(s, -1) {
		if cpf(digitsOf(s[m[0]:m[1]])) && alone(s, m[0], m[1]) {
			add(m[0], m[1], ID)
		}
	}
	for _, m := range aadhaarShape.FindAllStringIndex(s, -1) {
		if verhoeff(digitsOf(s[m[0]:m[1]])) && alone(s, m[0], m[1]) {
			add(m[0], m[1], ID)
		}
	}
	for _, m := range nirShape.FindAllStringIndex(s, -1) {
		if nir(strings.ReplaceAll(s[m[0]:m[1]], " ", "")) && alone(s, m[0], m[1]) {
			add(m[0], m[1], ID)
		}
	}
	for _, m := range ibanShape.FindAllStringIndex(s, -1) {
		if iban(strings.ReplaceAll(s[m[0]:m[1]], " ", "")) {
			add(m[0], m[1], Account)
		}
	}
	kind := None
	labels(s, func(what string) bool {
		kind = named[what]
		return kind == None
	})
	if kind != None {
		for _, m := range namedRun.FindAllStringIndex(s, -1) {
			if !dateShape.MatchString(s[m[0]:m[1]]) {
				add(m[0], m[1], kind)
			}
		}
	}
	slices.SortStableFunc(out, func(a, b span) int { return a.start - b.start })
	return out
}

// alone says whether the digits at s[start:end] stand on their own: no
// digit touches them and no further digit group follows or precedes them
// across one separator, so a part of a longer number is not read as one.
func alone(s string, start, end int) bool {
	before, after := s[:start], s[end:]
	if r, n := utf8.DecodeLastRuneInString(before); n > 0 {
		if isDigit(r) {
			return false
		}
		if strings.ContainsRune(" -.", r) {
			if r2, n2 := utf8.DecodeLastRuneInString(before[:len(before)-n]); n2 > 0 && isDigit(r2) {
				return false
			}
		}
	}
	if r, n := utf8.DecodeRuneInString(after); n > 0 {
		if isDigit(r) {
			return false
		}
		if strings.ContainsRune(" -.", r) {
			if r2, n2 := utf8.DecodeRuneInString(after[n:]); n2 > 0 && isDigit(r2) {
				return false
			}
		}
	}
	return true
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

// digitsOf keeps only the digits of s.
func digitsOf(s string) string {
	return strings.Map(func(r rune) rune {
		if isDigit(r) {
			return r
		}
		return -1
	}, s)
}

// luhn reports whether the digits of s pass the payment card checksum.
func luhn(s string) bool {
	sum, n := 0, 0
	for k := len(s) - 1; k >= 0; k-- {
		c := s[k]
		if c < '0' || c > '9' {
			continue
		}
		d := int(c - '0')
		if n%2 == 1 {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		n++
	}
	return n >= 13 && sum%10 == 0
}

// dni checks a Spanish DNI's or NIE's control letter.
func dni(number, letter string) bool {
	number = strings.ToUpper(strings.ReplaceAll(number, "-", ""))
	switch number[0] {
	case 'X':
		number = "0" + number[1:]
	case 'Y':
		number = "1" + number[1:]
	case 'Z':
		number = "2" + number[1:]
	}
	n := 0
	for _, c := range number {
		n = n*10 + int(c-'0')
	}
	return "TRWAGMYFPDXBNJZSQVHLCKE"[n%23] == strings.ToUpper(letter)[0]
}

// cpf checks a Brazilian CPF's two check digits; a number of one repeated
// digit is never real.
func cpf(d string) bool {
	if len(d) != 11 || strings.Count(d, d[:1]) == 11 {
		return false
	}
	check := func(n int) byte {
		sum := 0
		for i := range n {
			sum += int(d[i]-'0') * (n + 1 - i)
		}
		r := sum % 11
		if r < 2 {
			return '0'
		}
		return byte('0' + 11 - r)
	}
	return check(9) == d[9] && check(10) == d[10]
}

// Verhoeff tables, for the Aadhaar check digit.
var (
	verhoeffD = [10][10]int{
		{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, {1, 2, 3, 4, 0, 6, 7, 8, 9, 5}, {2, 3, 4, 0, 1, 7, 8, 9, 5, 6},
		{3, 4, 0, 1, 2, 8, 9, 5, 6, 7}, {4, 0, 1, 2, 3, 9, 5, 6, 7, 8}, {5, 9, 8, 7, 6, 0, 4, 3, 2, 1},
		{6, 5, 9, 8, 7, 1, 0, 4, 3, 2}, {7, 6, 5, 9, 8, 2, 1, 0, 4, 3}, {8, 7, 6, 5, 9, 3, 2, 1, 0, 4},
		{9, 8, 7, 6, 5, 4, 3, 2, 1, 0},
	}
	verhoeffP = [8][10]int{
		{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, {1, 5, 7, 6, 2, 8, 3, 0, 9, 4}, {5, 8, 0, 3, 7, 9, 6, 1, 4, 2},
		{8, 9, 1, 6, 0, 4, 3, 5, 2, 7}, {9, 4, 5, 3, 1, 2, 6, 8, 7, 0}, {4, 2, 8, 6, 5, 7, 3, 9, 0, 1},
		{2, 7, 9, 3, 8, 0, 6, 4, 1, 5}, {7, 0, 4, 6, 9, 1, 3, 2, 5, 8},
	}
)

// verhoeff reports whether the digits d pass the Verhoeff checksum.
func verhoeff(d string) bool {
	c := 0
	for i := range len(d) {
		c = verhoeffD[c][verhoeffP[i%8][int(d[len(d)-1-i]-'0')]]
	}
	return c == 0
}

// nir checks a French social security number's two-digit key: 97 less the
// first thirteen digits modulo 97, Corsica's 2A and 2B read as 19 and 18.
func nir(s string) bool {
	if len(s) != 15 {
		return false
	}
	body := strings.NewReplacer("2A", "19", "2B", "18").Replace(s[:13])
	n := 0
	for _, c := range body {
		if !isDigit(c) {
			return false
		}
		n = (n*10 + int(c-'0')) % 97
	}
	key := int(s[13]-'0')*10 + int(s[14]-'0')
	return key == 97-n
}

// iban checks an IBAN's length and its ISO 7064 mod-97 check.
func iban(s string) bool {
	if len(s) < 15 || len(s) > 34 {
		return false
	}
	n := 0
	for _, c := range s[4:] + s[:4] {
		switch {
		case isDigit(c):
			n = (n*10 + int(c-'0')) % 97
		case c >= 'A' && c <= 'Z':
			n = (n*100 + int(c-'A') + 10) % 97
		default:
			return false
		}
	}
	return n == 1
}
