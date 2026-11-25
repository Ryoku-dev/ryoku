package settings

import (
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

// a folded provider's line says the most useful thing it knows in one plain sentence
func TestAProviderRowSaysItsAllowanceInOneLine(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	seen := now.Add(-2 * time.Minute)
	count := func(n int64) *int64 { return &n }
	money := func(n float64) *float64 { return &n }
	cases := []struct {
		name     string
		provider protocol.ProviderInfo
		want     string
	}{
		{
			name: "a live daily window",
			provider: protocol.ProviderInfo{Name: "Groq", Tier: protocol.TierFree, Configured: true, Limits: &protocol.ProviderLimits{Windows: []protocol.LimitWindow{
				{Span: protocol.LimitSpanMinute, Unit: protocol.LimitUnitRequests, Limit: 30, Source: protocol.LimitFromDocumented},
				{Span: protocol.LimitSpanDay, Unit: protocol.LimitUnitTokens, Limit: 500000, Remaining: count(412300), Source: protocol.LimitFromHeaders, Seen: &seen},
			}}},
			want: "Free tokens today: 412,300 of 500,000 left, as of 2 min ago.",
		},
		{
			name: "credits",
			provider: protocol.ProviderInfo{Name: "OpenRouter", Tier: protocol.TierCredits, Configured: true, Limits: &protocol.ProviderLimits{
				Credits: &protocol.LimitCredits{Limit: money(10), Remaining: money(4.21), Currency: "USD", Source: protocol.LimitFromAccount, Seen: seen},
			}},
			want: "Credits: $4.21 of $10 left.",
		},
		{
			name:     "a provider nobody set up and nothing sent",
			provider: protocol.ProviderInfo{Name: "Cohere", Tier: protocol.TierFree, Limits: &protocol.ProviderLimits{}},
		},
		{
			name:     "a local server",
			provider: protocol.ProviderInfo{Name: "Ollama", Local: true, Configured: true},
		},
	}
	for _, c := range cases {
		if got := limitLine(c.provider, now); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
