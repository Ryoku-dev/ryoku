package rank

import "math"

// BM25Term scores one query term in one document: tf is its count there, df
// the number of documents holding it, documents the corpus size, length the
// document's term count and average the corpus mean.
func BM25Term(tf, df float64, documents int, length int, average float64) float64 {
	idf := math.Log(1 + (float64(documents)-df+0.5)/(df+0.5))
	return idf * tf * 2.2 / (tf + 1.2*(1-0.75+0.75*float64(length)/average))
}

// BM25 scores every document of a small in-memory corpus against query; a
// document sharing no term scores 0.
func BM25(query string, docs []string) []float64 {
	scores := make([]float64, len(docs))
	terms := Frequencies(query)
	if len(terms) == 0 || len(docs) == 0 {
		return scores
	}
	frequencies := make([]map[string]int, len(docs))
	lengths := make([]int, len(docs))
	documentFrequency := map[string]int{}
	total := 0
	for index, doc := range docs {
		frequencies[index] = Frequencies(doc)
		lengths[index] = TermCount(frequencies[index])
		total += lengths[index]
		for term := range terms {
			if frequencies[index][term] > 0 {
				documentFrequency[term]++
			}
		}
	}
	average := float64(total) / float64(len(docs))
	if average == 0 {
		average = 1
	}
	for index := range docs {
		for term := range terms {
			tf := frequencies[index][term]
			if tf == 0 {
				continue
			}
			scores[index] += BM25Term(float64(tf), float64(documentFrequency[term]), len(docs), lengths[index], average)
		}
	}
	return scores
}
