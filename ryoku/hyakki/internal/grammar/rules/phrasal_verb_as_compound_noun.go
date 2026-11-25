package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"strings"
)

// PhrasalVerbAsCompoundNoun detects phrasal verbs written as compound nouns.
type PhrasalVerbAsCompoundNoun struct{ dictionary dict.Dictionary }

func NewPhrasalVerbAsCompoundNoun() *PhrasalVerbAsCompoundNoun {
	return &PhrasalVerbAsCompoundNoun{dictionary: dict.Curated()}
}
func (*PhrasalVerbAsCompoundNoun) Description() string {
	return "This rule looks for phrasal verbs written as compound nouns."
}
func (r *PhrasalVerbAsCompoundNoun) analyse(d *doc.Document, i int, tok doc.Token) ([]rune, bool, bool) {
	if tok.Kind.IsProperNoun() || tok.Kind.IsVerb() {
		return nil, false, false
	}
	chars := tok.GetCh(d.GetSource())
	word := string(chars)
	if strings.ContainsAny(word, " -'’") {
		return nil, false, false
	}
	if text.EqAnyIgnoreASCIICaseStr(chars, []string{"gallon", "dragon"}) {
		return nil, false, false
	}
	particleLen := 0
	for _, ending := range []string{"around", "back", "down", "in", "on", "off", "out", "over", "up"} {
		if text.EndsWithIgnoreASCIICaseStr(chars, ending) {
			particleLen = len(ending)
			break
		}
	}
	if particleLen == 0 {
		return nil, false, false
	}
	split := len(chars) - particleLen
	verbPart := chars[:split]
	phrasal := make([]rune, 0, len(chars)+1)
	phrasal = append(phrasal, verbPart...)
	phrasal = append(phrasal, ' ')
	phrasal = append(phrasal, chars[split:]...)
	verbMetadata, phrasalMetadata := r.dictionary.GetWordMetadata(verbPart), r.dictionary.GetWordMetadata(phrasal)
	verbIsVerb, phrasalIsVerb := verbMetadata != nil && verbMetadata.Verb != nil, phrasalMetadata != nil && phrasalMetadata.Verb != nil
	if !verbIsVerb && !phrasalIsVerb {
		return nil, false, false
	}
	prev, hasPrev := d.GetNextWordFromOffset(i, -1)
	next, hasNext := d.GetNextWordFromOffset(i, 1)
	if !hasPrev && !hasNext {
		return nil, false, false
	}
	source := d.GetSource()
	if hasPrev {
		if prev.Kind.IsAdjective() || prev.Kind.IsDeterminer() {
			return nil, false, false
		}
		if (prev.Kind.IsNoun() && !prev.Kind.IsPluralNoun()) || text.EqStr(prev.GetCh(source), "settings") {
			return nil, false, false
		}
		if phrasalVerbAsCompoundNounIsPartOfNounList(d, i) {
			return nil, false, false
		}
		if prev.Kind.IsPreposition() && !text.EqStr(prev.GetCh(source), "to") {
			return nil, false, false
		}
		if prev.Kind.IsOov() {
			return nil, false, false
		}
	}
	if !hasPrev && hasNext && next.Kind.IsPreposition() {
		return nil, false, false
	}
	if hasNext && next.Kind.IsPronoun() && text.EqAnyIgnoreASCIICaseStr(next.GetCh(source), []string{"that", "which"}) {
		return nil, false, false
	}
	if !hasPrev && hasNext && text.EqAnyIgnoreASCIICaseStr(next.GetCh(source), []string{"can", "could", "may", "might", "must", "shall", "should", "will", "would"}) {
		return nil, false, false
	}
	if hasNext && next.Kind.IsNoun() {
		var suffixes []string
		switch {
		case text.EqStr(chars, "backup"):
			suffixes = []string{"file", "images", "link", "links", "location", "plan", "sites", "snapshots"}
		case text.EqStr(chars, "callback"):
			suffixes = []string{"function", "handlers"}
		case text.EqStr(chars, "layout"):
			suffixes = []string{"estimation"}
		case text.EqStr(chars, "markup"):
			suffixes = []string{"language", "languages"}
		case text.EqStr(chars, "mouseover"):
			suffixes = []string{"hints"}
		case text.EqStr(chars, "playback"):
			suffixes = []string{"latency", "speed"}
		case text.EqStr(chars, "plugin"):
			suffixes = []string{"architecture", "classes", "development", "developer", "docs", "ecosystem", "files", "interface", "name", "packages", "suite", "support"}
		case text.EqStr(chars, "popup"):
			suffixes = []string{"window"}
		case text.EqStr(chars, "rollout"):
			suffixes = []string{"logic", "status"}
		case text.EqStr(chars, "startup"):
			suffixes = []string{"environments"}
		case text.EqStr(chars, "throwback"):
			suffixes = []string{"machine"}
		case text.EqStr(chars, "workout"):
			suffixes = []string{"constraints", "preference"}
		}
		if text.EqAnyIgnoreASCIICaseStr(next.GetCh(source), suffixes) {
			return nil, false, false
		}
	}
	if hasNext && (next.Kind.IsVerbThirdPersonSingularPresentForm() || next.Kind.IsVerbSimplePastForm() || next.Kind.IsVerbPastForm()) && !next.Kind.IsPluralNoun() {
		return nil, false, false
	}
	return phrasal, phrasalIsVerb, true
}
func (r *PhrasalVerbAsCompoundNoun) Lint(d *doc.Document) []lint.Lint {
	var result []lint.Lint
	for _, i := range doc.IterNounIndices(d.Tokens()) {
		tok := d.Tokens()[i]
		if replacement, definite, ok := r.analyse(d, i, tok); ok {
			message := "This word might be a phrasal verb rather than a compound noun."
			if definite {
				message = "This word should be a phrasal verb, not a compound noun."
			}
			result = append(result, lint.Lint{Span: tok.Span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWith(replacement)}, Message: message, Priority: 63})
		}
	}
	return result
}

// phrasalVerbAsCompoundNounIsPartOfNounList checks whether the current token is part of a list of nouns.
func phrasalVerbAsCompoundNounIsPartOfNounList(d *doc.Document, i int) bool {
	prev, ok := d.GetNextWordFromOffset(i, -1)
	if !ok || !prev.Kind.IsConjunction() {
		return false
	}
	tok, ok := d.GetTokenOffset(i, -3)
	if !ok {
		return false
	}
	if tok.Kind.IsComma() {
		return true
	}
	if tok.Kind.IsWhitespace() {
		before, ok := d.GetTokenOffset(i, -4)
		return ok && (before.Kind.IsNoun() || before.Kind.IsComma())
	}
	return false
}
