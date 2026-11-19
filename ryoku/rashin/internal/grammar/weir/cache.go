package weir

import "sync"

type parseParams struct {
	code     string
	optimize bool
}

var parseCache struct {
	sync.RWMutex
	trees map[parseParams]*Ast
	order []parseParams
	next  int
}

// ParseStr returns a shared immutable AST. Like Harper's peek-based LRU,
// cache hits do not refresh the eviction order; at most 10,000 trees are held.
func ParseStr(code string, optimize bool) (*Ast, error) {
	key := parseParams{code, optimize}
	parseCache.RLock()
	a := parseCache.trees[key]
	parseCache.RUnlock()
	if a != nil {
		return a, nil
	}
	a, err := parseStrUncached(code, optimize)
	if err != nil {
		return nil, err
	}
	parseCache.Lock()
	defer parseCache.Unlock()
	if previous := parseCache.trees[key]; previous != nil {
		return previous, nil
	}
	if parseCache.trees == nil {
		parseCache.trees = make(map[parseParams]*Ast)
	}
	if len(parseCache.order) == 10000 {
		delete(parseCache.trees, parseCache.order[parseCache.next])
		parseCache.order[parseCache.next] = key
		parseCache.next = (parseCache.next + 1) % 10000
	} else {
		parseCache.order = append(parseCache.order, key)
	}
	parseCache.trees[key] = a
	return a, nil
}
