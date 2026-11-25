package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"strings"
)

// ThrowRubbish is related to ThrowBabyWithBathwater.
type ThrowRubbish struct{}

func NewThrowRubbish() *ThrowRubbish { return &ThrowRubbish{} }
func (*ThrowRubbish) Description() string {
	return "Checks for throwing rubbish rather than throwing it away."
}
func (*ThrowRubbish) Lint(document *doc.Document) []lint.Lint {
	var lints []lint.Lint
	for _, chunk := range doc.IterChunks(document.Tokens()) {
		for _, verbI := range doc.IterVerbIndices(chunk) {
			verbToken := chunk[verbI]
			if !text.EqAnyIgnoreASCIICaseStr(verbToken.GetCh(document.GetSource()), []string{"throw", "throws", "threw", "thrown", "throwing"}) {
				continue
			}
			advPrepSeen, junkSeen, lastI := false, false, -1
			for restI, token := range chunk[verbI+1:] {
				chunkI := verbI + restI + 1
				chars := token.GetCh(document.GetSource())
				if text.EqAnyIgnoreASCIICaseStr(chars, []string{"away", "out", "back", "everywhere", "in", "into", "at", "on"}) {
					advPrepSeen, lastI = true, chunkI
					if junkSeen {
						break
					}
				}
				if text.EqAnyIgnoreASCIICaseStr(chars, []string{"rubbish", "trash", "garbage", "junk"}) {
					if next, ok := document.GetNextWordFromOffset(chunkI, 1); ok && next.Kind.IsNoun() && !throwRubbishIsProgressiveVerbForm(document, next) {
						continue
					}
					junkSeen, lastI = true, chunkI
					if advPrepSeen {
						break
					}
				}
			}
			if junkSeen && !advPrepSeen {
				span := text.Span{Start: verbToken.Span.Start, End: chunk[lastI].Span.End}
				verb := verbToken.GetStr(document.GetSource())
				rest := document.GetSpanContentStr(text.Span{Start: verbToken.Span.End, End: span.End})
				var suggestions []lint.Suggestion
				for _, adv := range []string{"away", "out"} {
					suggestions = append(suggestions, lint.ReplaceWith([]rune(verb+" "+adv+rest)), lint.ReplaceWith([]rune(verb+rest+" "+adv)))
				}
				lints = append(lints, lint.Lint{Span: span, LintKind: lint.Miscellaneous, Suggestions: suggestions, Message: "To dispose of rubbish we don't just throw it, we throw it away", Priority: 63})
			}
		}
	}
	return lints
}
func throwRubbishIsProgressiveVerbForm(document *doc.Document, token *doc.Token) bool {
	return token.Kind.IsVerbProgressiveForm() && strings.HasSuffix(strings.ToLower(token.GetStr(document.GetSource())), "ing")
}
