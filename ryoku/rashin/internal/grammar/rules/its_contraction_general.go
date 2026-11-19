package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
	"strings"
	"unicode"
)

type General struct{ expression expr.Expr }

func NewGeneral() *General {
	positive := expr.NewSequenceExprAco("its").TWs().Then(expr.Or(expr.Or(expr.NewUPOSSet([]pos.UPOS{pos.VERB, pos.AUX, pos.DET, pos.PRON}), expr.NewWordSet([]string{"anywhere", "everywhere", "somewhere", "nowhere"})), expr.NewWordSet([]string{"because"})))
	exceptions := expr.NewSequenceExprAnything().ThenAnything().ThenWordSet([]string{"own", "intended"})
	inverted := expr.NewSequenceExprUnless(exceptions)
	expression := expr.OrLongest(expr.NewAll([]expr.Expr{positive, inverted}), expr.NewSequenceExprAco("its").TWs().Then(expr.NewUPOSSet([]pos.UPOS{pos.ADJ})).TWs().Then(expr.NewUPOSSet([]pos.UPOS{pos.SCONJ, pos.PART})))
	return &General{expression: expression}
}
func (*General) Description() string {
	return "Detects the possessive `its` before `had`, `been`, or `got` and offers `it's` or `it has`."
}
func (r *General) Lint(d *doc.Document) []lint.Lint {
	var result []lint.Lint
	source := d.GetSource()
	for _, chunk := range d.IterChunks() {
		for _, span := range expr.IterMatches(r.expression, chunk, source) {
			var preceding *doc.Token
			for i := span.Start - 1; i >= 0; i-- {
				if !chunk[i].Kind.IsWhitespace() {
					preceding = &chunk[i]
					break
				}
			}
			if l := r.matchToLint(chunk[span.Start:], preceding, source); l != nil {
				result = append(result, *l)
			}
		}
	}
	return result
}
func (r *General) matchToLint(tokens []doc.Token, preceding *doc.Token, source []rune) *lint.Lint {
	if len(tokens) < 3 {
		return nil
	}
	target := tokens[0]
	modifier := tokens[2]
	modifierText := modifier.GetStr(source)
	lower := strings.ToLower(modifierText)
	var next *doc.TokenKind
	if len(tokens) > 4 {
		next = &tokens[4].Kind
	}
	if itsContractionGeneralPrecedingWord(source, target.Span.Start) == "at" {
		switch modifierText {
		case "highest", "lowest", "best", "worst":
			return nil
		}
	}
	strong := false
	switch lower {
	case "had", "been", "got", "called", "named", "known", "termed", "titled":
		strong = true
	}
	if modifier.Kind.IsUpos(pos.VERB) && !strong && preceding != nil && preceding.Kind.IsPreposition() {
		return nil
	}
	shouldConsider := false
	switch lower {
	case "anybody", "anyone", "anything", "anywhere", "everybody", "everyone", "everything", "everywhere", "nobody", "nothing", "nowhere", "somebody", "someone", "something", "somewhere", "because", "a", "an", "my", "your", "his", "her", "our", "their", "this", "that":
		shouldConsider = true
	default:
		if modifier.Kind.IsUpos(pos.ADJ) {
			shouldConsider = (lower == "common" || lower == "easy" || lower == "hard") && next != nil && (next.IsUpos(pos.SCONJ) || next.IsUpos(pos.PART))
		} else if modifier.Kind.IsUpos(pos.VERB) || modifier.Kind.IsUpos(pos.AUX) {
			blocks := false
			switch itsContractionGeneralNextNonWhitespaceWord(source, modifier.Span.End) {
			case "is", "was", "were", "be", "been", "being", "to":
				blocks = true
			}
			blocks = !strong && (blocks || (next != nil && (next.IsNoun() || next.IsProperNoun())))
			shouldConsider = !blocks
		}
	}
	if !shouldConsider {
		return nil
	}
	if modifier.Kind.IsUpos(pos.VERB) && !itsContractionGeneralIsLikelyPredicativeParticiple(modifier, source) {
		if _, ok := expr.NewNominalPhrase().Matches(tokens[2:], source); ok {
			return nil
		}
	}
	if len(tokens) > 4 && modifier.Kind.IsVerbPastParticipleForm() && tokens[3].Kind.IsWhitespace() && (tokens[4].Kind.IsNoun() || tokens[4].Kind.IsProperNoun()) && !itsContractionGeneralIsLikelyPredicativeParticiple(modifier, source) && lower != "had" && lower != "been" && lower != "got" {
		return nil
	}
	return &lint.Lint{Span: target.Span, LintKind: lint.Punctuation, Priority: 54, Message: "Use `it's` (short for `it has` or `it is`) here, not the possessive `its`.", Suggestions: lint.ToReplaceSuggestions([]string{"it's", "it has"}, target.GetCh(source))}
}
func itsContractionGeneralIsLikelyPredicativeParticiple(token doc.Token, source []rune) bool {
	switch strings.ToLower(token.GetStr(source)) {
	case "called", "named", "known", "termed", "titled":
		return true
	}
	return false
}
func itsContractionGeneralPrecedingWord(source []rune, offset int) string {
	if offset > len(source) || offset <= 0 {
		return ""
	}
	i := offset - 1
	for i >= 0 && unicode.IsSpace(source[i]) {
		i--
	}
	if i < 0 {
		return ""
	}
	end := i + 1
	for i >= 0 && !unicode.IsSpace(source[i]) {
		i--
	}
	return strings.ToLower(string(source[i+1 : end]))
}
func itsContractionGeneralNextNonWhitespaceWord(source []rune, offset int) string {
	if offset < 0 || offset >= len(source) {
		return ""
	}
	i := offset
	for i < len(source) && unicode.IsSpace(source[i]) {
		i++
	}
	start := i
	for i < len(source) && !unicode.IsSpace(source[i]) && !(source[i] < 128 && ((source[i] >= 33 && source[i] <= 47) || (source[i] >= 58 && source[i] <= 64) || (source[i] >= 91 && source[i] <= 96) || (source[i] >= 123 && source[i] <= 126))) {
		i++
	}
	return strings.ToLower(string(source[start:i]))
}
