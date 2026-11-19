package fs

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// selection is the line part of a read path (omp's selector grammar, the
// subset that applies to local text): `:N`, `:N-`, `:A-B`, `:A..B`, `:A+C`,
// `:-N`, comma lists of those, and `:raw` before or after them.
type selection struct {
	Ranges []lineRange // sorted, merged; To 0 runs to the default limit
	Last   int         // `-N`: the last N lines
	Raw    bool
}

type lineRange struct{ From, To int }

func (s selection) empty() bool { return len(s.Ranges) == 0 && s.Last == 0 }

// splitSelector cuts selector suffixes off a path. A suffix that does not
// look like a selector stays part of the path; one that looks like a broken
// selector (`:0`, `:9-3`) is an error the model can fix.
func splitSelector(path string) (string, selection, error) {
	var sel selection
	single := false
	for range 2 {
		i := strings.LastIndexByte(path, ':')
		if i <= 0 || i == len(path)-1 {
			break
		}
		token := path[i+1:]
		if token == "raw" {
			if sel.Raw {
				break
			}
			sel.Raw = true
			path = path[:i]
			continue
		}
		if !selectorShaped(token) || !sel.empty() {
			break
		}
		parsed, err := parseRanges(token)
		if err != nil {
			return path, sel, err
		}
		sel.Ranges, sel.Last = parsed.Ranges, parsed.Last
		single = token != "" && strings.IndexFunc(token, func(r rune) bool { return r < '0' || r > '9' }) == -1
		path = path[:i]
	}
	if sel.Raw && single && len(sel.Ranges) == 1 {
		sel.Ranges[0].To = sel.Ranges[0].From
	}
	return path, sel, nil
}

func selectorShaped(token string) bool {
	for _, r := range token {
		if !(r >= '0' && r <= '9') && !strings.ContainsRune("-+.,", r) {
			return false
		}
	}
	return strings.ContainsAny(token, "0123456789")
}

func parseRanges(token string) (selection, error) {
	var sel selection
	for _, part := range strings.Split(token, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if rest, ok := strings.CutPrefix(part, "-"); ok {
			n, err := lineNumber(rest)
			if err != nil {
				return sel, err
			}
			sel.Last = max(sel.Last, n)
			continue
		}
		r, err := parseRange(part)
		if err != nil {
			return sel, err
		}
		sel.Ranges = append(sel.Ranges, r)
	}
	sel.Ranges = mergeRanges(sel.Ranges)
	return sel, nil
}

func parseRange(part string) (lineRange, error) {
	if a, c, ok := strings.Cut(part, "+"); ok {
		from, err := lineNumber(a)
		if err != nil {
			return lineRange{}, err
		}
		count, err := strconv.Atoi(c)
		if err != nil || count < 1 {
			return lineRange{}, fmt.Errorf("line selector %q: the count after + must be at least 1", part)
		}
		return lineRange{From: from, To: from + count - 1}, nil
	}
	sep := "-"
	if strings.Contains(part, "..") {
		sep = ".."
	}
	a, b, ok := strings.Cut(part, sep)
	from, err := lineNumber(a)
	if err != nil {
		return lineRange{}, err
	}
	if !ok || b == "" {
		return lineRange{From: from}, nil
	}
	to, err := lineNumber(b)
	if err != nil {
		return lineRange{}, err
	}
	if to < from {
		return lineRange{}, fmt.Errorf("line selector %q ends before it starts; use :%d-%d", part, to, from)
	}
	return lineRange{From: from, To: to}, nil
}

func lineNumber(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("line selector %q is not a number", s)
	}
	if n < 1 {
		return 0, fmt.Errorf("line selector %d is invalid; lines are 1-indexed. Use :1", n)
	}
	return n, nil
}

// mergeRanges sorts ranges and joins the ones that touch. An open range
// (To 0) absorbs everything after its start.
func mergeRanges(rs []lineRange) []lineRange {
	if len(rs) < 2 {
		return rs
	}
	sort.Slice(rs, func(i, j int) bool { return rs[i].From < rs[j].From })
	out := []lineRange{rs[0]}
	for _, r := range rs[1:] {
		last := &out[len(out)-1]
		if last.To == 0 {
			continue
		}
		if r.From <= last.To+1 {
			if r.To == 0 || r.To > last.To {
				last.To = r.To
			}
			continue
		}
		out = append(out, r)
	}
	return out
}
