package memory

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"ryoku-rashin/internal/rank"
)

// stopWords say nothing about what a fact or a question is about: English,
// with the commonest of Spanish, Portuguese, French, Italian, German and
// Dutch. "user" is among them because every fact is about the user; words that are
// also English nouns (war, den, era, dove) are left out.
var stopWords = func() map[string]bool {
	lists := []string{
		// English
		"a an the and or but if of to in on at by for with from as is are was were be been being am do does did done have has had having " +
			"i me my mine myself you your yours we our us he him his she her it its they them their this that these those " +
			"what which who whom whose when where why how there here not no can could would should will shall may might must " +
			"about into over under up down out than then so very just also too each every some any all both more most other such " +
			"only own same few again once while because until during before after above below between through off nor " +
			"user users s t d ll m re ve don doesn didn isn aren wasn weren won",
		// Spanish and Portuguese
		"el la los las un una unos unas y o de del al en por para con que qué es fue ser estar está están mi mis tu tus su sus " +
			"yo me te se nos le les lo como cómo cuál cuándo dónde muy más pero o a e os as um uma uns umas do da dos das no na nos nas " +
			"pelo pela com não eu meu minha meus minhas seu sua você ele ela isso esse essa qual quando onde",
		// French
		"le la les un une des et ou de du au aux en dans pour avec sans que qui quoi est sont était être je me moi mon ma mes " +
			"tu te ton ta tes il elle on nous vous ils elles se sa ses ce cet cette ces quel quelle quand où comment très plus mais ne pas",
		// Italian
		"il lo la i gli le un uno una e o di del della dei delle a al alla in nel nella da con per su che chi cosa è sono essere " +
			"io mi mio mia miei mie tu ti tuo tua lui lei noi voi loro si suo sua quale quando come molto più ma non",
		// German
		"der das dem des ein eine einen einem einer und oder von zu im in an auf mit für aus bei nach ist sind waren sein " +
			"ich mich mir mein meine du dich dir dein deine er sie es wir ihr ihnen sich was wer wie wo wann welche welcher sehr mehr aber nicht kein",
		// Dutch
		"de het een en of van te in op aan voor uit bij naar is zijn was waren ik mij me mijn jij je jou jouw hij zij ze wij " +
			"jullie hun wat wie waar wanneer welke zeer meer maar niet geen",
	}
	out := map[string]bool{}
	for _, list := range lists {
		for _, word := range strings.Fields(list) {
			out[word] = true
		}
	}
	return out
}()

// contentWords are the words of text that carry meaning: its tokens without
// stop words and single letters.
func contentWords(text string) []string {
	var out []string
	for _, token := range rank.Tokens(text) {
		first, _ := utf8.DecodeRuneInString(token)
		if stopWords[token] || len(token) < 2 && !unicode.IsDigit(first) {
			continue
		}
		out = append(out, token)
	}
	return out
}

// vectorText is what a fact or a question is embedded as: its content
// words, so the words every fact shares do not pull unrelated facts
// together. A text of stop words alone is embedded as it is.
func vectorText(text string) string {
	if words := contentWords(text); len(words) > 0 {
		return strings.Join(words, " ")
	}
	return text
}
