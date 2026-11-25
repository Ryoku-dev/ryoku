package memory

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"

	"go.etcd.io/bbolt"

	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/rank"
)

// Embedder turns texts into unit vectors; the daemon hands memory the potion
// model of the code index.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// errNoVectors says no vectors model is available; search falls back to
// words alone.
var errNoVectors = errors.New("memory: no vectors model")

// What a fact must clear to be found: a word of the question, or a sentence
// cosine of vectorMin, or a word-to-word likeness of wordMin. potion's
// paraphrases score from about 0.1, so these floors only work together with
// keep, the share of the best match's evidence a fact must reach.
const (
	vectorMin = 0.1
	wordMin   = 0.15
	keep      = 0.5
	// lexicalEvidence is how strong a shared word counts against the
	// vector legs.
	lexicalEvidence = 0.5
)

// Embed turns texts into vectors with the model, loading it on first use.
func (s *Store) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	s.embedMu.Lock()
	defer s.embedMu.Unlock()
	return s.embedLocked(ctx, texts)
}

// embedLocked is Embed with embedMu held.
func (s *Store) embedLocked(ctx context.Context, texts []string) ([][]float32, error) {
	if s.embedder == nil {
		if s.cfg.Embed == nil {
			return nil, errNoVectors
		}
		embedder, release, err := s.cfg.Embed()
		if err != nil {
			return nil, err
		}
		if embedder == nil {
			return nil, errNoVectors
		}
		s.embedder, s.release = embedder, release
	}
	vectors, err := s.embedder.Embed(ctx, texts)
	if err == nil && len(vectors) != len(texts) {
		err = errNoVectors
	}
	return vectors, err
}

// embedFacts embeds fact texts, and questions compared with them, as their
// content words.
func (s *Store) embedFacts(ctx context.Context, texts []string) ([][]float32, error) {
	content := make([]string, len(texts))
	for index, text := range texts {
		content[index] = vectorText(text)
	}
	return s.Embed(ctx, content)
}

// wordVectors are the vectors of single words, embedded once and kept until
// Release.
func (s *Store) wordVectors(ctx context.Context, words []string) (map[string][]float32, error) {
	s.embedMu.Lock()
	defer s.embedMu.Unlock()
	if s.words == nil {
		s.words = map[string][]float32{}
	}
	var missing []string
	for _, word := range words {
		if _, ok := s.words[word]; !ok {
			missing = append(missing, word)
		}
	}
	if len(missing) > 0 {
		vectors, err := s.embedLocked(ctx, missing)
		if err != nil {
			return nil, err
		}
		for index, word := range missing {
			s.words[word] = vectors[index]
		}
	}
	out := make(map[string][]float32, len(words))
	for _, word := range words {
		out[word] = s.words[word]
	}
	return out, nil
}

// Release drops the vectors model and the word vectors until they are
// needed again.
func (s *Store) Release() {
	s.embedMu.Lock()
	defer s.embedMu.Unlock()
	if s.release != nil {
		s.release()
	}
	s.embedder, s.release, s.words = nil, nil, nil
}

// ensureVectors embeds the facts that have no vector yet and keeps them.
func (s *Store) ensureVectors(ctx context.Context, facts []*fact) error {
	s.mu.Lock()
	var missing []*fact
	var texts []string
	for _, f := range facts {
		if len(f.Vector) == 0 {
			missing = append(missing, f)
			texts = append(texts, f.Text)
		}
	}
	s.mu.Unlock()
	if len(missing) == 0 {
		return nil
	}
	vectors, err := s.embedFacts(ctx, texts)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Update(func(tx *bbolt.Tx) error {
		for index, f := range missing {
			// the fact may have been replaced by undo meanwhile; the vector goes on what the ledger holds now
			current := s.facts[f.ID]
			if current == nil || current.Text != texts[index] {
				continue
			}
			current.Vector = rank.Quantize(vectors[index])
			if err := putJSON(tx, bucketFacts, current.ID, current); err != nil {
				return err
			}
		}
		return nil
	})
}

// Search finds active memories of the scopes by words and meaning, best
// first: BM25 over the question's content words, the sentence vectors and
// word-to-word likeness, fused with RRF. A fact must clear one of them, and
// come within reach of the best match, so a question that fits little finds
// little rather than k facts of noise. Recent facts get a small boost, and
// MMR keeps near-repeats from crowding the top. No scopes search everything.
func (s *Store) Search(ctx context.Context, query string, scopes []string, k int) ([]protocol.Memory, error) {
	ranked, err := s.ranked(ctx, query, scopes, true)
	if err != nil {
		return nil, err
	}
	rank.Normalize(ranked)
	s.mu.Lock()
	now := s.now()
	for index, item := range ranked {
		f := s.facts[item.ID]
		if f == nil {
			continue
		}
		at := f.Updated
		if f.LastUsed != nil && f.LastUsed.After(at) {
			at = *f.LastUsed
		}
		ranked[index].Score = rank.Boost(item.Score, rank.Recency(now.Sub(at).Hours()/24))
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].Score > ranked[j].Score })
	ranked = rank.MMR(ranked, func(id string) string {
		if f := s.facts[id]; f != nil {
			return f.Text
		}
		return ""
	}, rank.MMRLambda)
	out := s.pick(ranked, k)
	s.mu.Unlock()
	return out, nil
}

// Similar finds the active memories of the scopes closest to text, best
// first, for the learner to weigh a new fact against.
func (s *Store) Similar(ctx context.Context, text string, scopes []string, k int) ([]protocol.Memory, error) {
	ranked, err := s.ranked(ctx, text, scopes, false)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pick(ranked, k), nil
}

// pick turns the first k ranked ids into memories.
func (s *Store) pick(ranked []rank.Scored, k int) []protocol.Memory {
	var out []protocol.Memory
	for _, item := range ranked {
		if k > 0 && len(out) >= k {
			break
		}
		if f := s.facts[item.ID]; f != nil {
			out = append(out, s.memory(f))
		}
	}
	return out
}

// ranked fuses the rankings of the active memories in the scopes that clear
// a leg; cut also drops the ones far below the best match. Without a vectors
// model it ranks by words alone.
func (s *Store) ranked(ctx context.Context, query string, scopes []string, cut bool) ([]rank.Scored, error) {
	s.mu.Lock()
	var candidates []*fact
	for _, f := range s.facts {
		if f.Status == protocol.MemoryActive && inScopes(f.Scope, scopes) {
			candidates = append(candidates, f)
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ID < candidates[j].ID })
	texts := make([]string, len(candidates))
	for index, f := range candidates {
		texts[index] = f.Text
	}
	s.mu.Unlock()
	question := contentWords(query)
	if len(candidates) == 0 || len(question) == 0 {
		return nil, nil
	}
	evidence := make([]float64, len(candidates))
	lexical := rank.BM25(strings.Join(question, " "), texts)
	for index, score := range lexical {
		if score > 0 {
			evidence[index] = lexicalEvidence
		}
	}
	lists := [][]string{rankScores(candidates, lexical, 0)}
	if sentence, words, err := s.likeness(ctx, query, question, candidates, texts); err == nil {
		for index := range candidates {
			if sentence[index] >= vectorMin {
				evidence[index] = max(evidence[index], sentence[index])
			}
			if words[index] >= wordMin {
				evidence[index] = max(evidence[index], words[index])
			}
		}
		lists = append(lists, rankScores(candidates, sentence, vectorMin), rankScores(candidates, words, wordMin))
	} else if !errors.Is(err, errNoVectors) {
		return nil, err
	}
	fused := rank.Fuse(0, lists...)
	if !cut {
		return fused, nil
	}
	byID := make(map[string]float64, len(candidates))
	best := 0.0
	for index, f := range candidates {
		byID[f.ID] = evidence[index]
		best = max(best, evidence[index])
	}
	kept := fused[:0]
	for _, item := range fused {
		if byID[item.ID] >= keep*best {
			kept = append(kept, item)
		}
	}
	return kept, nil
}

// likeness scores each candidate against the question twice: the cosine of
// their content-word vectors, and how well each content word of the
// question is matched by its closest word of the fact, on average.
func (s *Store) likeness(ctx context.Context, query string, question []string, candidates []*fact, texts []string) (sentence, words []float64, err error) {
	vectors, err := s.embedFacts(ctx, []string{query})
	if err != nil {
		return nil, nil, err
	}
	if err := s.ensureVectors(ctx, candidates); err != nil {
		return nil, nil, err
	}
	factWords := make([][]string, len(texts))
	vocabulary := append([]string(nil), question...)
	for index, text := range texts {
		factWords[index] = contentWords(text)
		vocabulary = append(vocabulary, factWords[index]...)
	}
	wordVectors, err := s.wordVectors(ctx, vocabulary)
	if err != nil {
		return nil, nil, err
	}
	sentence = make([]float64, len(candidates))
	words = make([]float64, len(candidates))
	s.mu.Lock()
	for index, f := range candidates {
		sentence[index] = rank.Cosine(vectors[0], f.Vector)
	}
	s.mu.Unlock()
	for index := range candidates {
		var sum float64
		for _, asked := range question {
			closest := 0.0
			for _, word := range factWords[index] {
				closest = max(closest, cosine(wordVectors[asked], wordVectors[word]))
			}
			sum += closest
		}
		words[index] = sum / float64(len(question))
	}
	return sentence, words, nil
}

// cosine compares two float vectors; an empty one scores 0.
func cosine(left, right []float32) float64 {
	var dot, a, b float64
	for index := range min(len(left), len(right)) {
		dot += float64(left[index]) * float64(right[index])
		a += float64(left[index]) * float64(left[index])
		b += float64(right[index]) * float64(right[index])
	}
	if a == 0 || b == 0 {
		return 0
	}
	return dot / math.Sqrt(a*b)
}

// rankScores orders the candidates scoring above floor, best first.
func rankScores(candidates []*fact, scores []float64, floor float64) []string {
	order := make([]int, 0, len(candidates))
	for index, score := range scores {
		if score > floor {
			order = append(order, index)
		}
	}
	sort.SliceStable(order, func(i, j int) bool { return scores[order[i]] > scores[order[j]] })
	out := make([]string, len(order))
	for at, index := range order {
		out[at] = candidates[index].ID
	}
	return out
}
