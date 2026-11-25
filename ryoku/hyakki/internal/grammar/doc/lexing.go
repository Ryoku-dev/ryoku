package doc

import (
	"math"
	"ryoku-rashin/internal/grammar/text"
	"strconv"
	"strings"
	"unicode"
)

type FoundToken struct {
	NextIndex int
	Token     TokenKind
}

func LexWith(source []rune, lexer func([]rune) FoundToken) []Token {
	var tokens []Token
	// English and Weir both commonly alternate words and separators. Reserving
	// that token density avoids repeatedly copying the large token values while
	// compiling the embedded rules; append still handles denser input normally.
	if capacity := len(source) / 3; capacity > 0 {
		tokens = make([]Token, 0, capacity)
	}
	for cursor := 0; cursor < len(source); {
		found := lexer(source[cursor:])
		tokens = append(tokens, Token{Span: text.Span{Start: cursor, End: cursor + found.NextIndex}, Kind: found.Token})
		cursor += found.NextIndex
	}
	return tokens
}
func LexEnglishToken(source []rune) FoundToken { return lexToken(source, true) }
func LexWeirToken(source []rune) FoundToken    { return lexToken(source, false) }
func lexToken(s []rune, regex bool) FoundToken {
	lexers := []func([]rune) (FoundToken, bool){lexPunctuation, lexTabs, lexSpaces, lexNewlines, lexPluralDigit, lexHexNumber, lexLongDecade, lexNumber, LexUrl, LexEmailAddress, LexHostnameToken, lexWord}
	if regex {
		if f, ok := lexRegexish(s); ok {
			return f
		}
	}
	for _, lex := range lexers {
		if f, ok := lex(s); ok {
			return f
		}
	}
	return FoundToken{NextIndex: 1, Token: TokenKind{Kind: Unlintable}}
}
func asciiAlpha(c rune) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func asciiDigit(c rune) bool { return c >= '0' && c <= '9' }
func alphaNum(c rune) bool {
	return unicode.IsLetter(c) || unicode.Is(unicode.Other_Alphabetic, c) || unicode.IsNumber(c)
}
func apostrophe(c rune) bool { return c == '\'' || c == '’' }
func lexWord(s []rune) (FoundToken, bool) {
	i := 0
	for i < len(s) && (text.IsEnglishLingual(s[i]) || asciiDigit(s[i]) || apostrophe(s[i])) {
		i++
	}
	for i > 0 && apostrophe(s[i-1]) {
		i--
	}
	return FoundToken{NextIndex: i, Token: BlankWord()}, i > 0
}
func lexPunctuation(s []rune) (FoundToken, bool) {
	if len(s) == 0 {
		return FoundToken{}, false
	}
	if s[0] == '"' || s[0] == '“' || s[0] == '”' {
		return FoundToken{NextIndex: 1, Token: TokenKind{Kind: Punctuation, Punctuation: text.Punctuation{Kind: text.QuotePunctuation}}}, true
	}
	p, ok := text.PunctuationFromChar(s[0])
	return FoundToken{NextIndex: 1, Token: TokenKind{Kind: Punctuation, Punctuation: p}}, ok
}
func lexRepeated(s []rune, c rune, k TokenKindTag, mul int) (FoundToken, bool) {
	i := 0
	for i < len(s) && s[i] == c {
		i++
	}
	return FoundToken{NextIndex: i, Token: TokenKind{Kind: k, Count: i * mul}}, i > 0
}
func lexTabs(s []rune) (FoundToken, bool)     { return lexRepeated(s, '\t', Space, 2) }
func lexSpaces(s []rune) (FoundToken, bool)   { return lexRepeated(s, ' ', Space, 1) }
func lexNewlines(s []rune) (FoundToken, bool) { return lexRepeated(s, '\n', Newline, 1) }
func lexPluralDigit(s []rune) (FoundToken, bool) {
	if len(s) == 0 || !alphaNum(s[0]) {
		return FoundToken{}, false
	}
	i := 1
	if i < len(s) && s[i] == '\'' {
		i++
	}
	if i < len(s) && s[i] == 's' {
		i++
		if i == len(s) || !alphaNum(s[i]) {
			return FoundToken{NextIndex: i, Token: BlankWord()}, true
		}
	}
	return FoundToken{}, false
}
func lexLongDecade(s []rune) (FoundToken, bool) {
	ok := len(s) >= 5 && (s[0] == '1' || s[0] == '2') && asciiDigit(s[1]) && asciiDigit(s[2]) && s[3] == '0' && s[4] == 's'
	return FoundToken{NextIndex: 5, Token: TokenKind{Kind: Decade}}, ok
}
func lexHexNumber(s []rune) (FoundToken, bool) {
	if len(s) < 3 || s[0] != '0' || s[1] != 'x' || !isHex(s[2]) {
		return FoundToken{}, false
	}
	i := 2
	for i < len(s) {
		if !isHex(s[i]) {
			if alphaNum(s[i]) {
				return FoundToken{}, false
			}
			break
		}
		i++
	}
	v, err := strconv.ParseUint(string(s[2:i]), 16, 64)
	return FoundToken{NextIndex: i, Token: TokenKind{Kind: Number, Number: text.Number{Value: float64(v), Radix: 16}}}, err == nil
}
func lexNumber(s []rune) (FoundToken, bool) {
	if len(s) == 0 || !unicode.IsNumber(s[0]) {
		return FoundToken{}, false
	}
	end := len(s)
	for i, c := range s {
		if !asciiDigit(c) && c != '.' && c != 'e' && c != 'E' && c != '+' && c != '-' {
			end = i
			break
		}
	}
	for end > 0 && !asciiDigit(s[end-1]) {
		end--
	}
	for end > 0 {
		v, err := strconv.ParseFloat(string(s[:end]), 64)
		if (err == nil || math.IsInf(v, 0)) && s[end-1] != '.' {
			precision := 0
			for i := end - 1; i >= 0; i-- {
				if s[i] == '.' {
					precision = end - i - 1
					break
				}
			}
			return FoundToken{NextIndex: end, Token: TokenKind{Kind: Number, Number: text.Number{Value: v, Radix: 10, Precision: precision}}}, true
		}
		end--
	}
	return FoundToken{}, false
}
func lexRegexish(s []rune) (FoundToken, bool) {
	if len(s) == 0 || s[0] != '[' {
		return FoundToken{}, false
	}
	i := 1
	for {
		if i >= len(s) || !alphaNum(s[i]) {
			return FoundToken{}, false
		}
		i++
		if i < len(s) && s[i] == '-' {
			i++
			if i >= len(s) || !alphaNum(s[i]) {
				return FoundToken{}, false
			}
			i++
		}
		if i < len(s) && s[i] == ']' {
			return FoundToken{NextIndex: i + 1, Token: TokenKind{Kind: Regexish}}, true
		}
	}
}
func LexHostname(s []rune) (int, bool) {
	if len(s) == 0 || !(asciiAlpha(s[0]) || asciiDigit(s[0])) {
		return 0, false
	}
	i := 0
	for i < len(s) && (asciiAlpha(s[i]) || asciiDigit(s[i]) || s[i] == '-' || s[i] == '.') {
		i++
	}
	return i, true
}
func LexHostnameToken(s []rune) (FoundToken, bool) {
	n, ok := LexHostname(s)
	if !ok || n <= 1 || s[n-1] == '.' || !strings.ContainsRune(string(s[1:n-1]), '.') {
		return FoundToken{}, false
	}
	for _, tld := range []string{"com", "net", "org", "edu", "gov", "mil", "txt", "io", "co", "us", "uk", "de", "ca", "au", "jp"} {
		if strings.HasSuffix(string(s[:n]), tld) {
			return FoundToken{NextIndex: n, Token: TokenKind{Kind: Hostname}}, true
		}
	}
	return FoundToken{}, false
}
func validUnquoted(c rune) bool {
	return asciiAlpha(c) || asciiDigit(c) || c > 127 || strings.ContainsRune("!#$%&'*+-/=?^_`{|}~.", c)
}
func validateLocalPart(s []rune) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	quoted := s[0] == '"' && s[len(s)-1] == '"'
	if quoted {
		if len(s) < 2 {
			return false
		}
		s = s[1 : len(s)-1]
		for i := 0; i < len(s); i++ {
			if s[i] == '\\' {
				i++
				continue
			}
			if !validUnquoted(s[i]) && !strings.ContainsRune("(),:;<>@[] ", s[i]) {
				return false
			}
		}
	} else {
		if s[0] == '.' || s[len(s)-1] == '.' {
			return false
		}
		for i, c := range s {
			if !validUnquoted(c) || i > 0 && c == '.' && s[i-1] == '.' {
				return false
			}
		}
	}
	return true
}
func LexEmailAddress(s []rune) (FoundToken, bool) {
	if len(s) > 254 {
		s = s[:254]
	}
	quote := false
	for i, c := range s {
		if c == '"' {
			quote = !quote
		}
		if c == ' ' && !quote {
			return FoundToken{}, false
		}
		if c == '@' && !quote {
			if !validateLocalPart(s[:i]) {
				return FoundToken{}, false
			}
			n, ok := LexHostname(s[i+1:])
			if !ok || n == 0 {
				return FoundToken{}, false
			}
			return FoundToken{NextIndex: i + 1 + n, Token: TokenKind{Kind: EmailAddress}}, true
		}
	}
	return FoundToken{}, false
}
func isHex(c rune) bool { return asciiDigit(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }
func lexUchar(s []rune) int {
	if len(s) == 0 {
		return 0
	}
	c := s[0]
	if asciiAlpha(c) || asciiDigit(c) || strings.ContainsRune("$-_.+!*'(),", c) {
		return 1
	}
	if len(s) >= 3 && c == '%' && isHex(s[1]) && isHex(s[2]) {
		return 3
	}
	return 0
}
func ucharPlus(s []rune) bool {
	for i := 0; i < len(s); {
		if strings.ContainsRune(";?&=", s[i]) {
			i++
			continue
		}
		n := lexUchar(s[i:])
		if n == 0 {
			return false
		}
		i += n
	}
	return true
}
func lexLogin(s []rune) (int, bool) {
	start := 0
	at := -1
	for i, c := range s {
		if c == '@' {
			at = i
			break
		}
	}
	if at >= 0 {
		for i, c := range s[:at] {
			if c == ':' {
				if !ucharPlus(s[i+1 : at]) {
					return 0, false
				}
				break
			}
		}
		if !ucharPlus(s[:at]) {
			return 0, false
		}
		start = at + 1
	}
	n, ok := LexHostname(s[start:])
	if !ok {
		return 0, false
	}
	if start+n < len(s) && s[start+n] == ':' {
		n = 0
		for n < len(s)-start && asciiDigit(s[start+n]) {
			n++
		}
	}
	return start + n, true
}
func LexUrl(s []rune) (FoundToken, bool) {
	sep := -1
	for i, c := range s {
		if i == 0 && !asciiAlpha(c) {
			return FoundToken{}, false
		}
		if c == ':' {
			sep = i
			break
		}
		if !asciiAlpha(c) && !asciiDigit(c) && !strings.ContainsRune(".-+", c) {
			return FoundToken{}, false
		}
	}
	if sep < 0 || len(s) < sep+3 || s[sep+1] != '/' || s[sep+2] != '/' {
		return FoundToken{}, false
	}
	rest := s[sep+3:]
	cursor, _ := lexLogin(rest)
	for cursor < len(rest) && rest[cursor] == '/' {
		cursor++
		n := 0
		for cursor+n < len(rest) {
			if strings.ContainsRune(";/? :@&=#", rest[cursor+n]) && rest[cursor+n] != ' ' {
				n++
				continue
			}
			step := lexUchar(rest[cursor+n:])
			if step == 0 {
				break
			}
			n += step
		}
		if n == 0 {
			break
		}
		cursor += n
	}
	return FoundToken{NextIndex: sep + 3 + cursor, Token: TokenKind{Kind: Url}}, true
}
