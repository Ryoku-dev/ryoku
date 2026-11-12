package rank

import "sort"

// RRFK is the reciprocal rank fusion constant.
const RRFK = 60

// Scored is one fused result.
type Scored struct {
	ID    string
	Score float64
}

// Fuse merges ranked id lists with reciprocal rank fusion: each list adds
// 1/(RRFK+rank) to an id. Ties keep first-seen order; limit 0 keeps all.
func Fuse(limit int, lists ...[]string) []Scored {
	byID := map[string]int{}
	var out []Scored
	for _, list := range lists {
		for index, id := range list {
			at, ok := byID[id]
			if !ok {
				at = len(out)
				byID[id] = at
				out = append(out, Scored{ID: id})
			}
			out[at].Score += 1 / float64(RRFK+index+1)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
