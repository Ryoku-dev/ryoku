package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
	"strings"
	"unicode"
)

type MissingTo struct{ expression expr.Expr }

func NewMissingTo() *MissingTo {
	m := expr.NewExprMap[int]()
	m.Insert(expr.NewSequenceExprWith(expr.NewWordSet([]string{
		"eager",
		"fail",
		"failed",
		"failing",
		"fails",
		"incline",
		"inclined",
		"inclines",
		"inclining",
		"manage",
		"managed",
		"manages",
		"managing",
		"ready",
	})).TWs().ThenKindWhere(func(k doc.TokenKind) bool { return k.IsUpos(pos.VERB) }), 0)
	m.Insert(expr.NewSequenceExprWith(expr.NewWordSet([]string{
		"aim",
		"aimed",
		"aiming",
		"aims",
		"agree",
		"agreed",
		"agreeing",
		"agrees",
		"arrange",
		"arranged",
		"arranges",
		"arranging",
		"aspire",
		"aspired",
		"aspires",
		"aspiring",
		"attempt",
		"attempted",
		"attempting",
		"attempts",
		"decide",
		"decided",
		"decides",
		"deciding",
		"endeavor",
		"endeavored",
		"endeavoring",
		"endeavors",
		"endeavour",
		"endeavoured",
		"endeavouring",
		"endeavours",
		"eager",
		"expect",
		"expected",
		"expecting",
		"expects",
		"forget",
		"forgot",
		"forgotten",
		"forgetting",
		"forgets",
		"hope",
		"hoped",
		"hopes",
		"hoping",
		"intend",
		"intended",
		"intending",
		"intends",
		"learn",
		"learned",
		"learning",
		"learns",
		"learnt",
		"long",
		"longed",
		"longing",
		"longs",
		"mean",
		"means",
		"meant",
		"need",
		"needed",
		"needing",
		"needs",
		"neglect",
		"neglected",
		"neglecting",
		"neglects",
		"prepare",
		"prepared",
		"prepares",
		"preparing",
		"refuse",
		"refused",
		"refuses",
		"refusing",
		"resolve",
		"resolved",
		"resolves",
		"resolving",
		"struggle",
		"struggled",
		"struggles",
		"struggling",
		"try",
		"tried",
		"trying",
		"tries",
		"want",
		"wanted",
		"wanting",
		"wants",
	})).TWs().ThenKindWhere(doc.TokenKind.IsVerbLemma), 0)
	return &MissingTo{expression: m}
}
func (r *MissingTo) Expr() expr.Expr { return r.expression }
func (r *MissingTo) Description() string {
	return "Flags verbs and adjectives like `need`, `want`, or `ready` that are missing `to` before an infinitive."
}
func (r *MissingTo) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *MissingTo) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return r.MatchToLintWithContext(t, s, nil)
}
func (r *MissingTo) MatchToLintWithContext(t []doc.Token, s []rune, c *lint.MatchContext) *lint.Lint {
	idx, ok := r.expression.(*expr.ExprMap[int]).Lookup(0, t, s)
	if !ok || idx >= len(t) {
		return nil
	}
	controller := t[idx]
	span := controller.Span
	controllerText := strings.ToLower(controller.GetStr(s))
	if controller.Kind.IsNoun() && lint.PrecededByWord(c, func(t *doc.Token) bool { return t.Kind.IsPossessiveNominal() }) {
		return nil
	}
	if controller.Kind.IsVerb() && controller.Kind.IsAdjective() && lint.PrecededByWord(c, func(t *doc.Token) bool { return t.Kind.IsNominal() }) && lint.FollowedByWord(c, func(t *doc.Token) bool { return t.Kind.IsAuxiliaryVerb() || t.Kind.IsAdverb() }) {
		return nil
	}
	adjectiveController := missingToIn(controllerText, "eager", "inclined", "ready")
	if controller.Kind.IsUpos(pos.ADJ) && !adjectiveController {
		return nil
	}
	if !controller.Kind.IsUpos(pos.VERB) && !adjectiveController {
		return nil
	}
	previous, _, hasPrevious := missingToPreviousWordWithSpan(s, span.Start)
	if missingToIn(previous, "a", "an", "the", "this", "that", "these", "those", "very", "so", "too", "quite", "rather") {
		return nil
	}
	endsDOrEn := strings.HasSuffix(controllerText, "d") || strings.HasSuffix(controllerText, "en")
	if endsDOrEn && missingToPrepositionOrDeterminerWithinFour(c, s, span.Start) {
		return nil
	}
	if hasPrevious && strings.HasSuffix(previous, "ly") && endsDOrEn {
		return nil
	}
	if strings.HasPrefix(controllerText, "hope") && previous == "of" {
		return nil
	}
	if controllerText == "needs" && previous == "must" {
		return nil
	}
	prevChar, hasPrevChar := missingToPreviousNonWhitespaceChar(s, span.Start)
	if controllerText == "prepare" && (!hasPrevChar || prevChar == '.' || prevChar == '!' || prevChar == '?') {
		return nil
	}
	var next *doc.Token
	for i := idx + 1; i < len(t); i++ {
		if !t[i].Kind.IsWhitespace() {
			next = &t[i]
			break
		}
	}
	if next == nil {
		return nil
	}
	nextText := strings.ToLower(next.GetStr(s))
	if strings.HasPrefix(controllerText, "try") && nextText == "and" {
		return nil
	}
	if strings.HasSuffix(nextText, "ing") {
		return nil
	}
	var nextPos *pos.UPOS
	if next.Kind.IsWord() && next.Kind.Metadata != nil {
		nextPos = next.Kind.Metadata.PosTag
	}
	isPos := func(p pos.UPOS) bool { return nextPos != nil && *nextPos == p }
	nextIsVerb := isPos(pos.VERB)
	nextIsNoun := isPos(pos.NOUN) || isPos(pos.PROPN) || isPos(pos.ADJ)
	if next.Kind.IsNpMember() && !nextIsVerb && (previous == "to" || missingToDeterminerWithinThree(s, span.Start)) {
		return nil
	}
	if !nextIsVerb && (isPos(pos.ADV) || isPos(pos.ADJ) || isPos(pos.ADP) || isPos(pos.SCONJ) || isPos(pos.CCONJ)) {
		return nil
	}
	nextNounNotVerb := nextIsNoun && !nextIsVerb
	if missingToIn(controllerText, "learn", "learned", "learning", "learns", "learnt", "mean", "means", "meant", "tried", "tries", "try", "trying", "want", "wanted", "wanting", "wants") && nextNounNotVerb {
		return nil
	}
	if missingToIn(controllerText, "hope", "hoped", "hopes", "hoping") && (nextNounNotVerb || isPos(pos.AUX)) {
		return nil
	}
	nextChar, _ := missingToNextNonWhitespaceChar(s, next.Span.End)
	if missingToIn(controllerText, "need", "needed", "needing", "needs") && (nextNounNotVerb || nextText == "help" || nextChar == '-') {
		return nil
	}
	if isPos(pos.PROPN) && hasPrevChar && (prevChar == '"' || prevChar == '\'' || prevChar == '”' || prevChar == '’' || prevChar == '!' || prevChar == '?' || prevChar == ',') {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.InsertAfter([]rune(" to"))}, Message: "Insert `to` to complete the infinitive (e.g., `need to talk`).", Priority: 62}
}

func missingToIn(word string, words ...string) bool {
	for _, w := range words {
		if w == word {
			return true
		}
	}
	return false
}
func missingToPreviousWordWithSpan(s []rune, start int) (string, int, bool) {
	cursor := start
	for cursor > 0 && unicode.IsSpace(s[cursor-1]) {
		cursor--
	}
	if cursor == 0 {
		return "", 0, false
	}
	end := cursor
	for cursor > 0 {
		ch := s[cursor-1]
		if unicode.IsLetter(ch) || ch == '\'' {
			cursor--
		} else {
			break
		}
	}
	if cursor == end {
		return "", 0, false
	}
	word := append([]rune(nil), s[cursor:end]...)
	for i, c := range word {
		if c >= 'A' && c <= 'Z' {
			word[i] = c + ('a' - 'A')
		}
	}
	return string(word), cursor, true
}
func missingToPreviousNonWhitespaceChar(s []rune, start int) (rune, bool) {
	for cursor := start - 1; cursor >= 0; cursor-- {
		if !unicode.IsSpace(s[cursor]) {
			return s[cursor], true
		}
	}
	return 0, false
}
func missingToNextNonWhitespaceChar(s []rune, start int) (rune, bool) {
	for cursor := start; cursor < len(s); cursor++ {
		if !unicode.IsSpace(s[cursor]) {
			return s[cursor], true
		}
	}
	return 0, false
}
func missingToDeterminerWithinThree(s []rune, start int) bool {
	for i := 0; i < 3; i++ {
		word, next, ok := missingToPreviousWordWithSpan(s, start)
		if !ok {
			break
		}
		start = next
		if missingToIn(word, "and", "or", "but") {
			continue
		}
		if missingToIn(word, "a", "an", "the", "this", "that", "these", "those") {
			return true
		}
	}
	return false
}
func missingToPrepositionOrDeterminerWithinFour(c *lint.MatchContext, s []rune, start int) bool {
	if c != nil {
		checked := 0
		for i := len(c.Before) - 1; i >= 0; i-- {
			t := c.Before[i]
			if t.Kind.IsSpace() || t.Kind.IsNewline() {
				continue
			}
			checked++
			if checked > 4 {
				break
			}
			if t.Kind.IsPunctuation() {
				break
			}
			word := strings.ToLower(t.GetStr(s))
			if t.Kind.IsUpos(pos.ADP) || missingToIn(word, "of", "for", "in", "with", "without", "during", "after", "before", "by", "about", "against", "from", "on", "at", "into", "through") {
				return true
			}
			if checked == 1 && (t.Kind.IsDeterminer() || missingToIn(word, "a", "an", "the", "this", "that", "these", "those")) {
				return true
			}
			if t.Kind.IsConjunction() || missingToIn(word, "and", "or", "but", "nor") || t.Kind.IsVerbProgressiveForm() || t.Kind.IsNoun() || t.Kind.IsAdjective() {
				continue
			}
			if t.Kind.IsVerb() {
				break
			}
		}
		return false
	}
	for i := 0; i < 4; i++ {
		word, next, ok := missingToPreviousWordWithSpan(s, start)
		if !ok {
			break
		}
		start = next
		if missingToIn(word, "and", "or", "but", "nor") {
			continue
		}
		if missingToIn(word, "a", "an", "the", "this", "that", "these", "those", "of", "for", "in", "with", "without", "during", "after", "before", "by", "about", "against", "from", "on", "at", "into", "through") {
			return true
		}
	}
	return false
}
