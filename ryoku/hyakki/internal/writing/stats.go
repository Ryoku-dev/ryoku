package writing

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"ryoku-rashin/internal/protocol"
)

func sentenceSpans(text string) [][2]int {
	initRules()
	var spans [][2]int
	start := 0
	for i, r := range text {
		if r == '.' || r == '?' || r == '!' || r == '\n' {
			end := i + 1
			if wordPattern.MatchString(text[start:end]) {
				spans = append(spans, [2]int{start, end})
			}
			start = end
		}
	}
	if start < len(text) && wordPattern.MatchString(text[start:]) {
		spans = append(spans, [2]int{start, len(text)})
	}
	return spans
}

func syllables(word string) int {
	word = strings.ToLower(word)
	switch word {
	case "the", "are", "one", "once", "have", "does", "done", "some", "come", "were":
		return 1
	case "people", "every", "business", "queueing":
		return 2
	}
	count := 0
	vowel := false
	for _, r := range word {
		next := strings.ContainsRune("aeiouy", r)
		if next && !vowel {
			count++
		}
		vowel = next
	}
	if count > 1 && strings.HasSuffix(word, "e") && !(strings.HasSuffix(word, "le") && len(word) > 2 && !strings.ContainsRune("aeiouy", rune(word[len(word)-3]))) {
		count--
	}
	if count > 1 && strings.HasSuffix(word, "ed") && !strings.HasSuffix(word, "ted") && !strings.HasSuffix(word, "ded") {
		count--
	}
	if count < 1 {
		count = 1
	}
	return count
}

func Stats(text string) protocol.WritingStats {
	initRules()
	words := wordPattern.FindAllString(text, -1)
	out := protocol.WritingStats{Words: len(words), Sentences: len(sentenceSpans(text))}
	if out.Words == 0 || out.Sentences == 0 {
		return out
	}
	syllableCount := 0
	for _, word := range words {
		syllableCount += syllables(word)
	}
	w, s := float64(out.Words), float64(out.Sentences)
	out.ReadingEase = 206.835 - 1.015*(w/s) - 84.6*(float64(syllableCount)/w)
	out.Grade = 0.39*(w/s) + 11.8*(float64(syllableCount)/w) - 15.59
	out.Minutes = w / 230
	return out
}

var signalOnce sync.Once
var stockPattern, transitionPattern, listPattern *regexp.Regexp

func initSignals() {
	signalOnce.Do(func() {
		stockPattern = phrasePattern([]string{
			"delve", "tapestry", "in today's fast-paced world", "in today’s fast-paced world",
			"it's important to note", "it’s important to note", "a testament to", "in conclusion",
			"in the ever-evolving landscape", "unlock the potential", "a rich tapestry", "a crucial role",
			"it is worth noting", "in the realm of", "navigate the complexities", "game-changer",
			"a journey of discovery", "a beacon of", "underscore the importance", "in summary",
		})
		transitionPattern = phrasePattern([]string{"furthermore", "moreover", "additionally", "consequently", "nevertheless", "in conclusion", "in summary", "on the other hand", "therefore"})
		listPattern = regexp.MustCompile(`^\s*(?:[-*+]\s+|[0-9]+[.)]\s+)([\pL]+)`)
	})
}

func clamp(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}

func AITell(text string) protocol.WritingAITell {
	initRules()
	initSignals()
	out := protocol.WritingAITell{Verdict: "human", Signals: []protocol.WritingSignal{}, Phrases: []protocol.WritingIssue{}}
	words := wordPattern.FindAllString(text, -1)
	if len(words) == 0 {
		return out
	}
	w := float64(len(words))
	unique := make(map[string]bool, len(words))
	for _, word := range words {
		unique[strings.ToLower(word)] = true
	}
	spans := sentenceSpans(text)
	lengths := make([]float64, len(spans))
	var total float64
	for i, span := range spans {
		lengths[i] = float64(len(wordPattern.FindAllString(text[span[0]:span[1]], -1)))
		total += lengths[i]
	}
	uniformity := 0.0
	if len(lengths) >= 4 {
		mean := total / float64(len(lengths))
		variance := 0.0
		for _, length := range lengths {
			variance += (length - mean) * (length - mean)
		}
		cv := math.Sqrt(variance/float64(len(lengths))) / math.Max(1, mean)
		uniformity = clamp(1 - cv/0.65)
	}
	phraseCount := 0
	for _, match := range stockPattern.FindAllStringIndex(text, -1) {
		if wholeWord(text, match[0], match[1]) {
			phraseCount++
			out.Phrases = append(out.Phrases, byteIssue(text, match[0], match[1], "style", "cliches", "A stock phrase often found in generated text, but people use it too."))
		}
	}
	stock := clamp(float64(phraseCount) * 15 / w)
	transitionCount := 0
	for _, match := range transitionPattern.FindAllStringIndex(text, -1) {
		if wholeWord(text, match[0], match[1]) {
			transitionCount++
		}
	}
	transitions := clamp(float64(transitionCount) * 35 / w)
	dashes := strings.Count(text, "\u2014")
	semicolons := strings.Count(text, ";")
	punctuation := clamp(float64(dashes+semicolons) * 35 / w)
	ttr := float64(len(unique)) / w
	repetition := 0.0
	if len(words) >= 40 {
		repetition = clamp((0.72 - ttr) / 0.45)
	}
	var paragraphs []float64
	for _, paragraph := range strings.Split(text, "\n\n") {
		if n := len(wordPattern.FindAllString(paragraph, -1)); n > 0 {
			paragraphs = append(paragraphs, float64(n))
		}
	}
	shape := 0.0
	if len(paragraphs) >= 3 {
		lo, hi := paragraphs[0], paragraphs[0]
		for _, length := range paragraphs[1:] {
			lo = math.Min(lo, length)
			hi = math.Max(hi, length)
		}
		shape = clamp(1 - (hi-lo)/math.Max(1, hi))
	}
	listCount, maxStart := 0, 0
	starts := make(map[string]int)
	for _, line := range strings.Split(text, "\n") {
		if match := listPattern.FindStringSubmatch(line); match != nil {
			listCount++
			key := strings.Map(unicode.ToLower, match[1])
			starts[key]++
			if starts[key] > maxStart {
				maxStart = starts[key]
			}
		}
	}
	parallel := 0.0
	if listCount >= 3 {
		parallel = float64(maxStart) / float64(listCount)
	}
	out.Signals = []protocol.WritingSignal{
		{ID: "burstiness", Label: "Sentence rhythm", Value: uniformity, Detail: "Similar sentence lengths can sound formulaic, this is not evidence of who wrote it."},
		{ID: "vocabulary", Label: "Repeated vocabulary", Value: repetition, Detail: fmt.Sprintf("Distinct-word ratio %.2f, vocabulary varies with topic and length.", ttr)},
		{ID: "stock", Label: "Stock phrases", Value: stock, Detail: fmt.Sprintf("%d familiar phrases, people also write these.", phraseCount)},
		{ID: "transitions", Label: "Transitions", Value: transitions, Detail: "Frequent linking phrases may make prose feel templated."},
		{ID: "punctuation", Label: "Punctuation pattern", Value: punctuation, Detail: "Frequent long dashes or semicolons are a style choice, not proof."},
		{ID: "paragraphs", Label: "Paragraph shape", Value: shape, Detail: "Similar paragraph lengths are only a weak style signal."},
		{ID: "parallelism", Label: "List pattern", Value: parallel, Detail: "Repeated list openings are common in both edited and generated writing."},
	}
	score := stock*45 + transitions*15 + uniformity*12 + repetition*8 + punctuation*8 + shape*6 + parallel*6
	if len(words) < 30 {
		score *= 0.55
	}
	out.Score = int(math.Round(score))
	if out.Score >= 65 {
		out.Verdict = "machine"
	} else if out.Score >= 30 {
		out.Verdict = "mixed"
	}
	out.Phrases = filterIssues(out.Phrases, nil)
	return out
}
