package rules

import (
	"container/list"
	"fmt"
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"sync"
	"unicode"
)

type spellCheckCacheEntry struct {
	word        string
	suggestions [][]rune
}
type SpellCheck struct {
	dictionary  dict.Dictionary
	dialect     dict.Dialect
	properNouns *expr.LongestMatchOf
	mutex       sync.Mutex
	cache       map[string]*list.Element
	recent      *list.List
}

func NewSpellCheck(dictionary dict.Dictionary, dialect dict.Dialect) *SpellCheck {
	return &SpellCheck{dictionary: dictionary, dialect: dialect, properNouns: properNounCapitalizationLintersProperNounsExpr(), cache: make(map[string]*list.Element), recent: list.New()}
}
func (r *SpellCheck) suggestCorrectSpelling(word []rune) [][]rune {
	key := string(word)
	if hit, ok := r.cache[key]; ok {
		r.recent.MoveToFront(hit)
		return spellCheckCloneSuggestions(hit.Value.(spellCheckCacheEntry).suggestions)
	}
	suggestions := r.uncachedSuggestCorrectSpelling(word)
	r.cache[key] = r.recent.PushFront(spellCheckCacheEntry{key, spellCheckCloneSuggestions(suggestions)})
	if r.recent.Len() > 10000 {
		last := r.recent.Back()
		delete(r.cache, last.Value.(spellCheckCacheEntry).word)
		r.recent.Remove(last)
	}
	return suggestions
}
func spellCheckCloneSuggestions(values [][]rune) [][]rune {
	out := make([][]rune, len(values))
	for i, v := range values {
		out[i] = append([]rune(nil), v...)
	}
	return out
}
func (r *SpellCheck) uncachedSuggestCorrectSpelling(word []rune) [][]rune {
	for dist := uint8(2); dist < 5; dist++ {
		var out [][]rune
		seen := make(map[string]bool)
		for _, v := range dict.SuggestCorrectSpelling(word, 200, dist, r.dictionary) {
			metadata := r.dictionary.GetWordMetadata(v)
			if metadata == nil || !metadata.Dialects.IsDialectEnabled(r.dialect) {
				continue
			}
			key := string(text.ToLower(v))
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, append([]rune(nil), v...))
			if len(out) == 3 {
				break
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}
func spellCheckParentheticalPluralSExpr() *expr.Filter {
	return expr.NewFilter([]expr.Expr{
		expr.NewSequenceExpr().ThenAnyWord().ThenKind(doc.TokenKind.IsOpenRound).TAco("s").ThenKind(doc.TokenKind.IsCloseRound),
		expr.NewSequenceExpr().ThenKind(doc.TokenKind.IsOpenRound).TAco("s").ThenKind(doc.TokenKind.IsCloseRound),
		expr.NewSequenceExprAco("s"),
	})
}
func (r *SpellCheck) Lint(document *doc.Document) []lint.Lint {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	var out []lint.Lint
	for _, word := range document.IterWords() {
		chars := word.GetCh(document.GetSource())
		if informalLaughterIsInformalLaughter(chars) {
			continue
		}
		metadata, _ := word.Kind.AsWord()
		if metadata != nil && metadata.Dialects.IsDialectEnabled(r.dialect) && (r.dictionary.ContainsExactWord(chars) || r.dictionary.ContainsExactWord(text.Normalized(chars)) || r.dictionary.ContainsExactWord(text.ToLower(text.Normalized(chars)))) {
			continue
		}
		possibilities := r.suggestCorrectSpelling(chars)
		if len(chars) > 0 && unicode.IsUpper(chars[0]) {
			for _, v := range possibilities {
				all := true
				for _, c := range v[1:] {
					if unicode.IsUpper(c) {
						all = false
						break
					}
				}
				if all && len(v) > 0 {
					first := v[0]
					v[0] = unicode.ToUpper(first)
					if v[0] == first && !unicode.IsUpper(first) {
						if expanded := text.Upper.ApplyTo(first); len(expanded) > 0 {
							v[0] = expanded[0]
						}
					}
				}
			}
		}
		if len(chars) == 2 {
			if canonical, ok := r.dictionary.GetCorrectCapitalizationOf(chars); ok {
				all := true
				for _, c := range canonical {
					if !unicode.IsUpper(c) {
						all = false
						break
					}
				}
				if all {
					possibilities = append(possibilities, append([]rune(nil), canonical...))
				}
			}
		}
		suggestions := make([]lint.Suggestion, 0, len(possibilities))
		for _, v := range possibilities {
			suggestions = append(suggestions, lint.ReplaceWith(v))
		}
		message := fmt.Sprintf("Did you mean to spell `%s` this way?", string(chars))
		if len(suggestions) == 1 {
			message = fmt.Sprintf("Did you mean `%s`?", string(possibilities[0]))
		}
		out = append(out, lint.Lint{Span: word.Span, LintKind: lint.Spelling, Suggestions: suggestions, Message: message, Priority: 63})
	}
	out = lint.RemoveLintsOverlappingExpr(spellCheckParentheticalPluralSExpr(), document, out)
	return lint.RemoveLintsOverlappingExpr(r.properNouns, document, out)
}
func (*SpellCheck) Description() string {
	return "Looks and provides corrections for misspelled words."
}
