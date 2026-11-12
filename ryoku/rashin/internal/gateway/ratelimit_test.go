package gateway

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

// Every rate-limit header family a provider sends is read into the window it
// counts over, with what is left and when it refills; a window that is not a
// minute, a day or a month is left out rather than misnamed.
func TestEveryRateLimitHeaderFamilyIsRead(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	type want struct {
		span, unit       string
		limit, remaining int64
		reset            time.Duration
	}
	cases := []struct {
		name     string
		provider string
		headers  map[string]string
		want     []want
	}{
		{
			name:     "OpenAI counts requests and tokens a minute",
			provider: "openai",
			headers: map[string]string{
				"x-ratelimit-limit-requests": "60", "x-ratelimit-remaining-requests": "59", "x-ratelimit-reset-requests": "1s",
				"x-ratelimit-limit-tokens": "150000", "x-ratelimit-remaining-tokens": "149984", "x-ratelimit-reset-tokens": "6m0s",
			},
			want: []want{
				{protocol.LimitSpanMinute, protocol.LimitUnitRequests, 60, 59, time.Second},
				{protocol.LimitSpanMinute, protocol.LimitUnitTokens, 150000, 149984, 6 * time.Minute},
			},
		},
		{
			name:     "Groq's request headers are a day, its token headers a minute",
			provider: "groq",
			headers: map[string]string{
				"x-ratelimit-limit-requests": "14400", "x-ratelimit-remaining-requests": "14370", "x-ratelimit-reset-requests": "2m59.56s",
				"x-ratelimit-limit-tokens": "18000", "x-ratelimit-remaining-tokens": "17997", "x-ratelimit-reset-tokens": "7.66s",
			},
			want: []want{
				{protocol.LimitSpanMinute, protocol.LimitUnitTokens, 18000, 17997, 7660 * time.Millisecond},
				{protocol.LimitSpanDay, protocol.LimitUnitRequests, 14400, 14370, 2*time.Minute + 59560*time.Millisecond},
			},
		},
		{
			name:     "Cerebras names the day and the minute",
			provider: "cerebras",
			headers: map[string]string{
				"x-ratelimit-limit-requests-day": "14400", "x-ratelimit-remaining-requests-day": "14399", "x-ratelimit-reset-requests-day": "33011.5",
				"x-ratelimit-limit-tokens-minute": "60000", "x-ratelimit-remaining-tokens-minute": "59000", "x-ratelimit-reset-tokens-minute": "11.5",
			},
			want: []want{
				{protocol.LimitSpanMinute, protocol.LimitUnitTokens, 60000, 59000, 11500 * time.Millisecond},
				{protocol.LimitSpanDay, protocol.LimitUnitRequests, 14400, 14399, 33011500 * time.Millisecond},
			},
		},
		{
			name:     "GitHub Models says each window's length",
			provider: "github",
			headers: map[string]string{
				"x-ratelimit-limit-requests": "150", "x-ratelimit-remaining-requests": "149", "x-ratelimit-renewalperiod-requests": "86400",
				"x-ratelimit-limit-tokens": "64000", "x-ratelimit-remaining-tokens": "63000", "x-ratelimit-renewalperiod-tokens": "60", "x-ratelimit-timeremaining-tokens": "30",
			},
			want: []want{
				{protocol.LimitSpanMinute, protocol.LimitUnitTokens, 64000, 63000, 30 * time.Second},
				{protocol.LimitSpanDay, protocol.LimitUnitRequests, 150, 149, 0},
			},
		},
		{
			name:     "the bare trio OpenRouter sends with epoch milliseconds",
			provider: "openrouter",
			headers: map[string]string{
				"X-RateLimit-Limit": "20", "X-RateLimit-Remaining": "0", "X-RateLimit-Reset": strconv.FormatInt(now.Add(30*time.Second).UnixMilli(), 10),
			},
			want: []want{{protocol.LimitSpanMinute, protocol.LimitUnitRequests, 20, 0, 30 * time.Second}},
		},
		{
			name:     "Anthropic counts a minute and resets at a time",
			provider: "anthropic",
			headers: map[string]string{
				"anthropic-ratelimit-requests-limit": "50", "anthropic-ratelimit-requests-remaining": "49", "anthropic-ratelimit-requests-reset": now.Add(time.Second).Format(time.RFC3339),
				"anthropic-ratelimit-tokens-limit": "40000", "anthropic-ratelimit-tokens-remaining": "39000", "anthropic-ratelimit-tokens-reset": now.Add(2 * time.Second).Format(time.RFC3339),
				"anthropic-ratelimit-input-tokens-limit": "30000",
			},
			want: []want{
				{protocol.LimitSpanMinute, protocol.LimitUnitRequests, 50, 49, time.Second},
				{protocol.LimitSpanMinute, protocol.LimitUnitTokens, 40000, 39000, 2 * time.Second},
			},
		},
		{
			name:     "the first IETF draft with a policy",
			provider: "somewhere",
			headers:  map[string]string{"RateLimit-Limit": "100", "RateLimit-Remaining": "50", "RateLimit-Reset": "30", "RateLimit-Policy": "100;w=86400"},
			want:     []want{{protocol.LimitSpanDay, protocol.LimitUnitRequests, 100, 50, 30 * time.Second}},
		},
		{
			name:     "the structured IETF fields",
			provider: "somewhere",
			headers:  map[string]string{"RateLimit-Policy": `"default";q=1000;w=60`, "RateLimit": `"default";r=998;t=12`},
			want:     []want{{protocol.LimitSpanMinute, protocol.LimitUnitRequests, 1000, 998, 12 * time.Second}},
		},
		{
			name:     "OVHcloud's Kong headers beside the IETF trio, as it sent them on a 429",
			provider: "ovh",
			headers: map[string]string{
				"Retry-After": "25", "x-ratelimit-remaining-minute": "0", "x-ratelimit-limit-minute": "2",
				"ratelimit-remaining": "0", "ratelimit-limit": "2", "ratelimit-reset": "25",
			},
			want: []want{{protocol.LimitSpanMinute, protocol.LimitUnitRequests, 2, 0, 25 * time.Second}},
		},
		{
			name:     "an hour is not a window here",
			provider: "kilo",
			headers:  map[string]string{"x-ratelimit-limit-requests-hour": "200", "x-ratelimit-remaining-requests-hour": "199", "x-ratelimit-limit-hour": "200"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			header := http.Header{}
			for name, value := range c.headers {
				header.Set(name, value)
			}
			got := headerWindows(c.provider, header, now)
			if len(got) != len(c.want) {
				t.Fatalf("read %d windows, want %d: %+v", len(got), len(c.want), got)
			}
			for i, w := range c.want {
				g := got[i]
				if g.Span != w.span || g.Unit != w.unit || g.Limit != w.limit || g.Remaining == nil || *g.Remaining != w.remaining {
					t.Fatalf("window %d is %s %s %d left %v, want %+v", i, g.Span, g.Unit, g.Limit, g.Remaining, w)
				}
				if g.Source != protocol.LimitFromHeaders || g.Seen == nil || !g.Seen.Equal(now) {
					t.Fatalf("window %d is not marked as read from the headers now: %+v", i, g)
				}
				switch {
				case w.reset == 0 && g.Reset != nil:
					t.Fatalf("window %d has a reset no header gave: %v", i, g.Reset)
				case w.reset != 0 && (g.Reset == nil || !g.Reset.Equal(now.Add(w.reset))):
					t.Fatalf("window %d resets at %v, want %v", i, g.Reset, now.Add(w.reset))
				}
			}
		})
	}
}
