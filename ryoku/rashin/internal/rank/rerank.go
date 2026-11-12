package rank

// Portions adapted from vectorize-io/hindsight
// hindsight-api-slim/hindsight_api/engine/search/reranking.py:37-83
// @ 7cec128b8fdb (MIT, Copyright (c) 2025 Vectorize AI, Inc.). Modified by
// Ryoku: only the linear recency signal and its multiplicative boost, applied
// to fused ranks normalised to [0.1, 1].
const (
	recencyAlpha      = 0.2
	recencyWindowDays = 365.0
)

// Recency maps an age in days to a freshness signal in [0.1, 1], 0.5 being
// neutral; the future counts as today.
func Recency(daysAgo float64) float64 {
	return max(0.1, min(1.0, 1.0-daysAgo/recencyWindowDays))
}

// Boost scales a relevance score by freshness: at most about 10 percent up
// for today and down for a year or more.
func Boost(score, recency float64) float64 {
	return score * (1 + recencyAlpha*(recency-0.5))
}

// Normalize maps fused scores onto [0.1, 1] so a boost acts on relevance, not
// on the tiny raw RRF sums.
func Normalize(items []Scored) {
	if len(items) == 0 {
		return
	}
	low, high := items[0].Score, items[0].Score
	for _, item := range items {
		low = min(low, item.Score)
		high = max(high, item.Score)
	}
	for index := range items {
		if high == low {
			items[index].Score = 1
			continue
		}
		items[index].Score = 0.1 + 0.9*(items[index].Score-low)/(high-low)
	}
}

// Portions adapted from openclaw/openclaw extensions/memory-core/src/memory/mmr.ts:8-78
// @ 3978ab104ee6 (MIT, Copyright (c) 2026 OpenClaw Foundation). Modified by
// Ryoku: Go, Jaccard over Tokens, items carry their text.

// MMRLambda weighs relevance against novelty.
const MMRLambda = 0.7

// MMR reorders scored items, best first, trading relevance for variety:
// each pick maximises lambda*relevance - (1-lambda)*similarity to the picks
// before it. text gives an item's words.
func MMR(items []Scored, text func(id string) string, lambda float64) []Scored {
	if len(items) < 2 || lambda >= 1 {
		return items
	}
	type prepared struct {
		item      Scored
		words     map[string]bool
		relevance float64
		similar   float64
	}
	low, high := items[0].Score, items[0].Score
	for _, item := range items {
		low = min(low, item.Score)
		high = max(high, item.Score)
	}
	remaining := make([]*prepared, len(items))
	for index, item := range items {
		words := map[string]bool{}
		for _, token := range Tokens(text(item.ID)) {
			words[token] = true
		}
		relevance := 1.0
		if high > low {
			relevance = (item.Score - low) / (high - low)
		}
		remaining[index] = &prepared{item: item, words: words, relevance: relevance}
	}
	out := make([]Scored, 0, len(items))
	for len(remaining) > 0 {
		best := 0
		bestScore := lambda*remaining[0].relevance - (1-lambda)*remaining[0].similar
		for index, candidate := range remaining[1:] {
			score := lambda*candidate.relevance - (1-lambda)*candidate.similar
			if score > bestScore || score == bestScore && candidate.item.Score > remaining[best].item.Score {
				best, bestScore = index+1, score
			}
		}
		picked := remaining[best]
		out = append(out, picked.item)
		remaining = append(remaining[:best], remaining[best+1:]...)
		for _, candidate := range remaining {
			candidate.similar = max(candidate.similar, jaccard(candidate.words, picked.words))
		}
	}
	return out
}

// jaccard is the overlap of two word sets; two empty sets are equal.
func jaccard(left, right map[string]bool) float64 {
	if len(left) == 0 && len(right) == 0 {
		return 1
	}
	shared := 0
	for word := range left {
		if right[word] {
			shared++
		}
	}
	union := len(left) + len(right) - shared
	if union == 0 {
		return 0
	}
	return float64(shared) / float64(union)
}
