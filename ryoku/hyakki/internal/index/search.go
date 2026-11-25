package index

import (
	"sort"
	"strings"
	"unicode/utf8"

	"go.etcd.io/bbolt"

	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/rank"
)

// maxTermsPerChunk is how many distinct terms one chunk keeps for BM25.
const maxTermsPerChunk = rank.MaxTerms

// tokenFrequencies counts the BM25 terms of a chunk or a query.
func tokenFrequencies(text string) map[string]int {
	return rank.Frequencies(text)
}

type rankedChunk struct {
	chunk chunkRecord
	score float64
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
			if err := tx.Bucket(bucketVectors).Put([]byte(id), rank.Quantize(vectors[index])); err != nil {
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
			scores = append(scores, scoredID{id: string(key), score: rank.Cosine(query, value)})
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

// fuseRanks merges ranked chunk lists with reciprocal rank fusion.
func fuseRanks(limit int, lists ...[]rankedChunk) []rankedChunk {
	chunks := map[string]chunkRecord{}
	ids := make([][]string, len(lists))
	for index, list := range lists {
		ids[index] = make([]string, len(list))
		for at, item := range list {
			ids[index][at] = item.chunk.ID
			if _, ok := chunks[item.chunk.ID]; !ok {
				chunks[item.chunk.ID] = item.chunk
			}
		}
	}
	fused := rank.Fuse(limit, ids...)
	out := make([]rankedChunk, 0, len(fused))
	for _, item := range fused {
		out = append(out, rankedChunk{chunk: chunks[item.ID], score: item.Score})
	}
	return out
}
func searchHits(items []rankedChunk, query string) []protocol.SearchHit {
	terms := rank.Tokens(query)
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
