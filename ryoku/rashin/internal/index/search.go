package index

import (
	"encoding/binary"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.etcd.io/bbolt"

	"ryoku-rashin/internal/protocol"
)

func codeTokens(text string) []string {
	var tokens []string
	var word []rune
	flush := func() {
		if len(word) == 0 {
			return
		}
		parts := splitIdentifier(string(word))
		tokens = append(tokens, parts...)
		word = word[:0]
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			word = append(word, r)
		} else {
			flush()
		}
	}
	flush()
	return tokens
}
func splitIdentifier(value string) []string {
	value = strings.Trim(value, "_")
	if value == "" {
		return nil
	}
	var out []string
	for _, section := range strings.FieldsFunc(value, func(r rune) bool { return r == '_' || r == '.' }) {
		runes := []rune(section)
		start := 0
		for i := 1; i < len(runes); i++ {
			boundary := (unicode.IsLower(runes[i-1]) && unicode.IsUpper(runes[i])) || (i+1 < len(runes) && unicode.IsUpper(runes[i-1]) && unicode.IsUpper(runes[i]) && unicode.IsLower(runes[i+1])) || (unicode.IsDigit(runes[i-1]) != unicode.IsDigit(runes[i]))
			if boundary {
				if i > start {
					out = append(out, strings.ToLower(string(runes[start:i])))
				}
				start = i
			}
		}
		if start < len(runes) {
			out = append(out, strings.ToLower(string(runes[start:])))
		}
	}
	return out
}

const maxTermsPerChunk = 16

func tokenFrequencies(text string) map[string]int {
	all := map[string]int{}
	for _, token := range codeTokens(text) {
		first, _ := utf8.DecodeRuneInString(token)
		if len(token) > 1 || unicode.IsDigit(first) {
			all[token]++
		}
	}
	if len(all) <= maxTermsPerChunk {
		return all
	}
	terms := make([]string, 0, len(all))
	for term := range all {
		terms = append(terms, term)
	}
	sort.Slice(terms, func(left, right int) bool {
		if all[terms[left]] != all[terms[right]] {
			return all[terms[left]] > all[terms[right]]
		}
		if len(terms[left]) != len(terms[right]) {
			return len(terms[left]) > len(terms[right])
		}
		return terms[left] < terms[right]
	})
	out := make(map[string]int, maxTermsPerChunk)
	for _, term := range terms[:maxTermsPerChunk] {
		out[term] = all[term]
	}
	return out
}
func termCount(terms map[string]int) int {
	n := 0
	for _, count := range terms {
		n += count
	}
	return n
}

type rankedChunk struct {
	chunk chunkRecord
	score float64
}

func quantizeVector(vector []float32) []byte {
	var maxValue float32
	for _, value := range vector {
		if value < 0 {
			value = -value
		}
		if value > maxValue {
			maxValue = value
		}
	}
	scale := maxValue / 127
	if scale == 0 {
		scale = 1
	}
	out := make([]byte, 4+len(vector))
	binary.LittleEndian.PutUint32(out, math.Float32bits(scale))
	for i, value := range vector {
		q := int(math.Round(float64(value / scale)))
		if q > 127 {
			q = 127
		}
		if q < -127 {
			q = -127
		}
		out[4+i] = byte(int8(q))
	}
	return out
}
func cosineQuantized(query []float32, raw []byte) float64 {
	if len(raw) != len(query)+4 {
		return 0
	}
	scale := math.Float32frombits(binary.LittleEndian.Uint32(raw))
	var dot, norm float64
	for i, value := range query {
		v := float64(int8(raw[4+i])) * float64(scale)
		dot += float64(value) * v
		norm += v * v
	}
	if norm == 0 {
		return 0
	}
	return dot / math.Sqrt(norm)
}
func (s *indexStore) putVector(id string, vector []float32) error {
	return s.putVectors([]string{id}, [][]float32{vector})
}
func (s *indexStore) putVectors(ids []string, vectors [][]float32) error {
	return s.update(func(tx *bbolt.Tx) error {
		for index, id := range ids {
			if index >= len(vectors) {
				break
			}
			if tx.Bucket(bucketChunks).Get([]byte(id)) == nil {
				continue
			}
			if err := tx.Bucket(bucketVectors).Put([]byte(id), quantizeVector(vectors[index])); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *indexStore) vectorSearch(query []float32, limit int) ([]rankedChunk, error) {
	type scoredID struct {
		id    string
		score float64
	}
	var scores []scoredID
	err := s.view(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketVectors).ForEach(func(key, value []byte) error {
			scores = append(scores, scoredID{id: string(key), score: cosineQuantized(query, value)})
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(scores, func(i, j int) bool { return scores[i].score > scores[j].score })
	if limit > 0 && len(scores) > limit {
		scores = scores[:limit]
	}
	ids := make([]string, len(scores))
	for index, item := range scores {
		ids[index] = item.id
	}
	chunks, err := s.chunksByID(ids)
	if err != nil {
		return nil, err
	}
	out := make([]rankedChunk, 0, len(scores))
	for _, item := range scores {
		if chunk, ok := chunks[item.id]; ok {
			out = append(out, rankedChunk{chunk: chunk, score: item.score})
		}
	}
	return out, nil
}
func fuseRanks(limit int, lists ...[]rankedChunk) []rankedChunk {
	byID := map[string]*rankedChunk{}
	var order []string
	for _, list := range lists {
		for index, item := range list {
			if byID[item.chunk.ID] == nil {
				copy := item
				copy.score = 0
				byID[item.chunk.ID] = &copy
				order = append(order, item.chunk.ID)
			}
			byID[item.chunk.ID].score += 1 / float64(60+index+1)
		}
	}
	sort.SliceStable(order, func(i, j int) bool { return byID[order[i]].score > byID[order[j]].score })
	if limit > 0 && len(order) > limit {
		order = order[:limit]
	}
	out := make([]rankedChunk, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out
}
func searchHits(items []rankedChunk, query string) []protocol.SearchHit {
	terms := codeTokens(query)
	out := make([]protocol.SearchHit, 0, len(items))
	for _, item := range items {
		lines := strings.Split(item.chunk.Text, "\n")
		first := 0
		for index, line := range lines {
			lower := strings.ToLower(line)
			matched := false
			for _, term := range terms {
				if strings.Contains(lower, term) {
					matched = true
					break
				}
			}
			if matched {
				first = max(0, index-2)
				break
			}
		}
		last := min(len(lines), first+8)
		snippetLines := append([]string(nil), lines[first:last]...)
		for index, line := range snippetLines {
			snippetLines[index] = truncateUTF8(line, 400)
		}
		snippet := strings.Join(snippetLines, "\n")
		out = append(out, protocol.SearchHit{Path: item.chunk.Path, Start: item.chunk.Start + first, End: min(item.chunk.End, item.chunk.Start+last-1), Score: item.score, Snippet: snippet, Symbol: item.chunk.Symbol, Language: item.chunk.Language})
	}
	return out
}
func truncateUTF8(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(value[cut]) {
		cut--
	}
	return value[:cut]
}
