package index

import (
	"bytes"
	"encoding/binary"
	"math"
	"sort"
	"strconv"

	"go.etcd.io/bbolt"
)

func (s *indexStore) bm25(query string, limit int) ([]rankedChunk, error) {
	terms := tokenFrequencies(query)
	if len(terms) == 0 {
		return nil, nil
	}
	documentsRaw, err := s.meta("bm25_documents")
	if err != nil {
		return nil, err
	}
	totalRaw, err := s.meta("bm25_total_terms")
	if err != nil {
		return nil, err
	}
	documents, _ := strconv.Atoi(documentsRaw)
	totalLength, _ := strconv.Atoi(totalRaw)
	if documents == 0 {
		documents, totalLength, err = s.lexicalStats()
		if err != nil {
			return nil, err
		}
	}
	if documents == 0 {
		return nil, nil
	}
	average := float64(totalLength) / float64(documents)
	if average == 0 {
		average = 1
	}
	termFrequency := map[string]map[string]int{}
	documentFrequency := map[string]int{}
	err = s.view(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketTerms)
		for term := range terms {
			prefix := []byte(term + "\x00")
			cursor := bucket.Cursor()
			for key, value := cursor.Seek(prefix); key != nil && bytes.HasPrefix(key, prefix); key, value = cursor.Next() {
				id := string(key[len(prefix):])
				if termFrequency[id] == nil {
					termFrequency[id] = map[string]int{}
				}
				termFrequency[id][term] = int(binary.BigEndian.Uint32(value))
				documentFrequency[term]++
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(termFrequency))
	for id := range termFrequency {
		ids = append(ids, id)
	}
	byID, err := s.chunksByID(ids)
	if err != nil {
		return nil, err
	}
	ranked := make([]rankedChunk, 0, len(termFrequency))
	for id, frequencies := range termFrequency {
		chunk, ok := byID[id]
		if !ok {
			continue
		}
		var score float64
		for term, frequency := range frequencies {
			tf := float64(frequency)
			df := float64(documentFrequency[term])
			idf := math.Log(1 + (float64(documents)-df+0.5)/(df+0.5))
			score += idf * tf * 2.2 / (tf + 1.2*(1-0.75+0.75*float64(chunk.Length)/average))
		}
		ranked = append(ranked, rankedChunk{chunk: chunk, score: score})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		if ranked[i].chunk.Path != ranked[j].chunk.Path {
			return ranked[i].chunk.Path < ranked[j].chunk.Path
		}
		return ranked[i].chunk.Start < ranked[j].chunk.Start
	})
	if limit > 0 && len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked, nil
}
