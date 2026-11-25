package teamsui

import (
	"fmt"

	"ryoku-i18n"
)

// a token count as people read it: 820, 12.4k, 1.2M
func Tokens(count int64) string {
	switch {
	case count < 1000:
		return fmt.Sprint(count)
	case count < 10_000:
		return fmt.Sprintf("%.1fk", float64(count)/1000)
	case count < 1_000_000:
		return fmt.Sprintf("%dk", count/1000)
	case count < 10_000_000:
		return fmt.Sprintf("%.1fM", float64(count)/1_000_000)
	}
	return fmt.Sprintf("%dM", count/1_000_000)
}

// tokens used against a cap, as a fact line
func TokensOf(used, limit int64) string {
	if limit <= 0 {
		return i18n.Tf("%s tokens, no limit", Tokens(used))
	}
	return i18n.Tf("%s of %s tokens", Tokens(used), Tokens(limit))
}

// the share of a cap used, for a meter: 0 without a cap
func Share(used, limit int64) float32 {
	if limit <= 0 {
		return 0
	}
	return min(1, float32(used)/float32(limit))
}
