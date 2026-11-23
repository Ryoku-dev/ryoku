package links

import (
	"fmt"
	"strconv"
	"strings"
)

// MaxExpanded caps how many links one paste may grow to through ranges.
const MaxExpanded = 500

// expandPattern expands [1-10], [001-100], [1-10:2], [10-1] and [a-z] ranges in s,
// every combination leftmost slowest, cut after limit. ranged is false when s has none.
// ported from prism-download-manager crates/prism-engine/src/util.rs (MIT)
func expandPattern(s string, limit int) (out []string, ranged bool) {
	if !strings.Contains(s, "[") {
		return nil, false
	}
	limit = max(limit, 0)
	type part struct {
		lit     string
		choices []string
	}
	var parts []part
	var lit strings.Builder
	rest := s
	for {
		open := strings.IndexByte(rest, '[')
		if open < 0 {
			break
		}
		lit.WriteString(rest[:open])
		end := strings.IndexByte(rest[open+1:], ']')
		if end < 0 {
			lit.WriteString(rest[open:])
			rest = ""
			break
		}
		inner := rest[open+1 : open+1+end]
		if c := parseRange(inner, max(limit, 1)); c != nil {
			parts = append(parts, part{lit: lit.String()}, part{choices: c})
			lit.Reset()
			ranged = true
		} else {
			lit.WriteString("[" + inner + "]")
		}
		rest = rest[open+1+end+1:]
	}
	if !ranged {
		return nil, false
	}
	lit.WriteString(rest)
	parts = append(parts, part{lit: lit.String()})
	out = []string{""}
	for _, p := range parts {
		if p.choices == nil {
			for i := range out {
				out[i] += p.lit
			}
			continue
		}
		next := make([]string, 0, min(limit, len(out)*len(p.choices)))
	fill:
		for _, prefix := range out {
			for _, c := range p.choices {
				if len(next) == limit {
					break fill
				}
				next = append(next, prefix+c)
			}
		}
		out = next
	}
	return out, true
}

// parseRange returns at most limit values of one bracket's range, or nil when
// it is not range syntax or not a valid range.
func parseRange(inner string, limit int) []string {
	rng, stepText, stepped := strings.Cut(inner, ":")
	a, b, ok := strings.Cut(rng, "-")
	if !ok {
		return nil
	}
	step := uint64(1)
	if stepped {
		n, err := strconv.ParseUint(stepText, 10, 64)
		if err != nil {
			return nil
		}
		step = n
	}
	if step == 0 {
		return nil
	}
	digits := func(s string) bool { return s != "" && strings.Trim(s, "0123456789") == "" }
	letter := func(s string) bool {
		return len(s) == 1 && (s[0] >= 'a' && s[0] <= 'z' || s[0] >= 'A' && s[0] <= 'Z')
	}
	var out []string
	switch {
	case digits(a) && digits(b):
		x, err1 := strconv.ParseUint(a, 10, 64)
		y, err2 := strconv.ParseUint(b, 10, 64)
		if err1 != nil || err2 != nil {
			return nil
		}
		width := 0
		if (len(a) > 1 && a[0] == '0') || (len(b) > 1 && b[0] == '0') {
			width = max(len(a), len(b))
		}
		span := diff(x, y)/step + 1
		for i := uint64(0); i < span && len(out) < limit; i++ {
			v := x + i*step
			if x > y {
				v = x - i*step
			}
			out = append(out, fmt.Sprintf("%0*d", width, v))
		}
	case letter(a) && letter(b):
		x, y := a[0], b[0]
		if (x >= 'a') != (y >= 'a') {
			return nil
		}
		span := diff(uint64(x), uint64(y))/step + 1
		for i := uint64(0); i < span && len(out) < limit; i++ {
			v := uint64(x) + i*step
			if x > y {
				v = uint64(x) - i*step
			}
			out = append(out, string(rune(v)))
		}
	default:
		return nil
	}
	return out
}

func diff(x, y uint64) uint64 {
	if x > y {
		return x - y
	}
	return y - x
}
