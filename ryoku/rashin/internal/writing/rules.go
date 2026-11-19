package writing

import (
	"regexp"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"ryoku-rashin/internal/protocol"
)

type phraseRule struct {
	pattern     *regexp.Regexp
	category    string
	replacement string
}

var rulesOnce sync.Once
var weaselPattern, clichePattern, passivePattern, therePattern, adverbPattern *regexp.Regexp
var doubleSpacePattern, smartQuotePattern, straightQuotePattern, wordPattern *regexp.Regexp
var wordyRules, inclusiveRules []phraseRule

func initRules() {
	rulesOnce.Do(func() {
		weaselPattern = phrasePattern([]string{
			"are a number", "clearly", "completely", "exceedingly", "excellent", "extremely",
			"fairly", "few", "huge", "interestingly", "is a number", "largely", "many", "mostly",
			"obviously", "quite", "relatively", "remarkably", "several", "significantly",
			"substantially", "surprisingly", "tiny", "usually", "various", "vast", "very",
		})

		clichePattern = phrasePattern([]string{
			"a fate worse than death", "at the end of the day", "between a rock and a hard place",
			"blissful ignorance", "blow a fuse", "cast aspersions", "comparing apples and oranges",
			"conspicuous by its absence", "crystal clear", "cutting edge", "dubious distinction",
			"eyes peeled", "fast and loose", "first and foremost", "gilding the lily", "in the final analysis",
			"innocent bystander", "last but not least", "make a mockery of", "moment of truth",
			"nip in the bud", "on the same page", "quantum leap", "sea change", "the cream of the crop",
			"the straw that broke the camel's back", "thick as thieves", "thinking outside the box",
			"throw the baby out with the bathwater", "various and sundry", "viable alternative",
		})

		wordyRules = []phraseRule{
			{phrasePattern([]string{"a number of"}), "wordiness", "some"},
			{phrasePattern([]string{"at the present time", "at this time"}), "wordiness", "now"},
			{phrasePattern([]string{"because of the fact that", "due to the fact that", "owing to the fact that"}), "wordiness", "because"},
			{phrasePattern([]string{"in order to", "in an effort to", "as a means of"}), "wordiness", "to"},
			{phrasePattern([]string{"in the event that"}), "wordiness", "if"},
			{phrasePattern([]string{"each and every"}), "wordiness", "each"},
			{phrasePattern([]string{"close proximity"}), "wordiness", "nearby"},
			{phrasePattern([]string{"for the purpose of"}), "wordiness", "for"},
			{phrasePattern([]string{"have a tendency to"}), "wordiness", "tend to"},
			{phrasePattern([]string{"in the near future"}), "wordiness", "soon"},
			{phrasePattern([]string{"prior to"}), "wordiness", "before"},
			{phrasePattern([]string{"until such time as"}), "wordiness", "until"},
			{phrasePattern([]string{"utilize"}), "wordiness", "use"},
			{phrasePattern([]string{"with the exception of"}), "wordiness", "except"},
			{phrasePattern([]string{"whether or not"}), "wordiness", "whether"},
			{phrasePattern([]string{"already existing", "all things considered", "as a matter of fact", "as of yet", "during the course of", "in a manner of speaking", "in my opinion", "the point I am trying to make", "various different"}), "wordiness", ""},
		}

		inclusiveRules = []phraseRule{
			{phrasePattern([]string{"freshmen", "freshwomen"}), "inclusive", "first-year students"},
			{phrasePattern([]string{"chairman", "chairwoman"}), "inclusive", "chair"},
			{phrasePattern([]string{"businessman", "businesswoman"}), "inclusive", "businessperson"},
			{phrasePattern([]string{"policeman", "policewoman"}), "inclusive", "police officer"},
			{phrasePattern([]string{"fireman"}), "inclusive", "firefighter"},
			{phrasePattern([]string{"mankind"}), "inclusive", "humanity"},
			{phrasePattern([]string{"manpower"}), "inclusive", "workforce"},
			{phrasePattern([]string{"man-made"}), "inclusive", "human-made"},
			{phrasePattern([]string{"wheelchair-bound", "confined to a wheelchair"}), "inclusive", "wheelchair user"},
			{phrasePattern([]string{"suffers from"}), "inclusive", "lives with"},
			{phrasePattern([]string{"the disabled"}), "inclusive", "people with disabilities"},
			{phrasePattern([]string{"normal people"}), "inclusive", "people without disabilities"},
			{phrasePattern([]string{"illegal alien", "illegal immigrant"}), "inclusive", "undocumented immigrant"},
			{phrasePattern([]string{"master"}), "inclusive", "primary"},
			{phrasePattern([]string{"slave"}), "inclusive", "replica"},
			{phrasePattern([]string{"whitelist"}), "inclusive", "allowlist"},
			{phrasePattern([]string{"blacklist"}), "inclusive", "blocklist"},
		}

		passivePattern = regexp.MustCompile(`(?i)\b(?:am|is|are|was|were|be|been|being)\s+(?:[a-z]+ly\s+)?(?:[a-z]+ed|known|given|made|seen|shown|taken|written|done|built|bought|found|left|sent|told|held|kept|read|sold|taught|thought|understood|won|worn)\b`)
		therePattern = regexp.MustCompile(`(?im)(?:^|[.!?]\s+)(there\s+(?:is|are|was|were))\b`)
		adverbPattern = regexp.MustCompile(`(?i)\b[[:alpha:]]+ly\b`)
		doubleSpacePattern = regexp.MustCompile(`[^\S\r\n]{2,}`)
		smartQuotePattern = regexp.MustCompile("[\u201c\u201d]")
		straightQuotePattern = regexp.MustCompile(`"`)
		wordPattern = regexp.MustCompile(`[\pL\pN]+(?:['\x{2019}][\pL\pN]+)*`)
	})
}

func phrasePattern(phrases []string) *regexp.Regexp {
	parts := make([]string, len(phrases))
	for i, phrase := range phrases {
		parts[i] = strings.ReplaceAll(regexp.QuoteMeta(phrase), " ", `\s+`)
	}
	return regexp.MustCompile("(?i)(?:" + strings.Join(parts, "|") + ")")
}

func wholeWord(text string, start, end int) bool {
	if start > 0 {
		r, _ := utf8.DecodeLastRuneInString(text[:start])
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' {
			return false
		}
	}
	if end < len(text) {
		r, _ := utf8.DecodeRuneInString(text[end:])
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' {
			return false
		}
	}
	return true
}

func ruleMatches(text string, pattern *regexp.Regexp, kind, category, message string, replacements ...string) []protocol.WritingIssue {
	var out []protocol.WritingIssue
	for _, match := range pattern.FindAllStringIndex(text, -1) {
		if wholeWord(text, match[0], match[1]) {
			out = append(out, byteIssue(text, match[0], match[1], kind, category, message, replacements...))
		}
	}
	return out
}

func clarityIssues(text string) []protocol.WritingIssue {
	initRules()
	out := ruleMatches(text, passivePattern, "clarity", "passive", "This may be passive voice, name who does it if that helps.")
	for _, match := range weaselPattern.FindAllStringIndex(text, -1) {
		if !wholeWord(text, match[0], match[1]) {
			continue
		}
		word := strings.ToLower(text[match[0]:match[1]])
		if (word == "many" || word == "few") && match[0] >= 4 && strings.EqualFold(text[match[0]-4:match[0]], "too ") {
			continue
		}
		out = append(out, byteIssue(text, match[0], match[1], "clarity", "weasel", "Consider a more specific word or number."))
	}
	for _, match := range therePattern.FindAllStringSubmatchIndex(text, -1) {
		out = append(out, byteIssue(text, match[2], match[3], "clarity", "wordiness", "Try starting with the subject instead of this opening."))
	}
	for _, match := range adverbPattern.FindAllStringIndex(text, -1) {
		word := strings.ToLower(text[match[0]:match[1]])
		switch word {
		case "family", "only", "early", "holy", "ugly", "lovely", "friendly", "lively", "lonely", "likely", "unlikely", "apply", "reply", "supply", "july":
			continue
		}
		out = append(out, byteIssue(text, match[0], match[1], "clarity", "adverbs", "Would a stronger verb make this adverb unnecessary?"))
	}
	out = append(out, ruleMatches(text, clichePattern, "clarity", "cliches", "This familiar phrase may hide what you mean, try something specific.")...)
	for _, rule := range wordyRules {
		var replacements []string
		if rule.replacement != "" {
			replacements = []string{rule.replacement}
		}
		out = append(out, ruleMatches(text, rule.pattern, "clarity", rule.category, "This could be shorter without losing its meaning.", replacements...)...)
	}
	words := wordPattern.FindAllStringIndex(text, -1)
	for i := 1; i < len(words); i++ {
		prev, word := words[i-1], words[i]
		if strings.TrimSpace(text[prev[1]:word[0]]) == "" && strings.EqualFold(text[prev[0]:prev[1]], text[word[0]:word[1]]) {
			out = append(out, byteIssue(text, prev[0], word[1], "clarity", "repetition", "This word appears twice in a row.", text[prev[0]:prev[1]]))
		}
	}
	for _, match := range doubleSpacePattern.FindAllStringIndex(text, -1) {
		out = append(out, byteIssue(text, match[0], match[1], "style", "formatting", "Use one space between words.", " "))
	}
	if smartQuotePattern.MatchString(text) && straightQuotePattern.MatchString(text) {
		for _, match := range straightQuotePattern.FindAllStringIndex(text, -1) {
			out = append(out, byteIssue(text, match[0], match[1], "style", "consistency", "Straight and curly quotation marks are mixed here."))
		}
	}
	for _, span := range sentenceSpans(text) {
		if len(wordPattern.FindAllStringIndex(text[span[0]:span[1]], -1)) > 35 {
			out = append(out, byteIssue(text, span[0], span[1], "clarity", "readability", "This sentence is long, consider splitting it."))
		}
	}
	return out
}

func inclusiveIssues(text string) []protocol.WritingIssue {
	initRules()
	var out []protocol.WritingIssue
	for _, rule := range inclusiveRules {
		out = append(out, ruleMatches(text, rule.pattern, "inclusive", "inclusive", "Depending on context, a more inclusive alternative may fit.", rule.replacement)...)
	}
	return out
}
