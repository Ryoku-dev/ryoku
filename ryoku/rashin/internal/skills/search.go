package skills

import (
	"sort"
	"strings"

	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/rank"
)

// How many skills a search returns unless asked, and at most.
const (
	searchDefault = 5
	searchMax     = 20
)

// Search ranks the skills of the sets that are on against what is about to
// be done, by BM25 over each skill's name and description, best first. An
// empty query lists every one of them.
func (l *Library) Search(params protocol.SkillsSearchParams, off []string) (protocol.SkillsSearchResult, error) {
	root, err := l.Root()
	if err != nil {
		return protocol.SkillsSearchResult{}, err
	}
	var candidates []entry
	for _, e := range l.entries() {
		if on(e.set, off) {
			candidates = append(candidates, e)
		}
	}
	result := protocol.SkillsSearchResult{Skills: []protocol.Skill{}}
	if strings.TrimSpace(params.Query) == "" {
		for _, e := range candidates {
			result.Skills = append(result.Skills, e.skill(root))
		}
		return result, nil
	}
	docs := make([]string, len(candidates))
	for i, e := range candidates {
		docs[i] = stems(strings.ReplaceAll(e.name, "-", " ") + " " + e.description)
	}
	scores := rank.BM25(stems(params.Query), docs)
	order := make([]int, 0, len(candidates))
	for i, score := range scores {
		if score > 0 {
			order = append(order, i)
		}
	}
	sort.SliceStable(order, func(a, b int) bool { return scores[order[a]] > scores[order[b]] })
	limit := params.Limit
	if limit <= 0 {
		limit = searchDefault
	}
	limit = min(limit, searchMax, len(order))
	for _, i := range order[:limit] {
		result.Skills = append(result.Skills, candidates[i].skill(root))
	}
	return result, nil
}

// stems is text as words cut to a rough stem, so "writing" finds "write" and
// "tests" finds "test".
func stems(text string) string {
	words := rank.Tokens(text)
	for i, word := range words {
		words[i] = stem(word)
	}
	return strings.Join(words, " ")
}

// stem drops the commonest English endings from a lower-case word.
func stem(word string) string {
	for _, suffix := range []string{"ing", "ed", "es", "s", "e"} {
		if len(word) > len(suffix)+2 && strings.HasSuffix(word, suffix) {
			return strings.TrimSuffix(word, suffix)
		}
	}
	return word
}
