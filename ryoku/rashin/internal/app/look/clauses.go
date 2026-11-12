package look

import "strings"

// facts read as words with commas, never as a list set apart by dots: "Curator on News digest, 2 min ago"
func Clauses(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, ", ")
}
