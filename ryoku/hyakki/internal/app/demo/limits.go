package demo

import (
	"time"

	"ryoku-rashin/internal/protocol"
)

// each kind of allowance a provider row can say: a live daily window, credits from an account, a
// documented limit, and a provider that reports nothing
func withDemoLimits(providers []protocol.ProviderInfo, now time.Time) []protocol.ProviderInfo {
	seen := now.Add(-2 * time.Minute)
	count := func(n int64) *int64 { return &n }
	money := func(n float64) *float64 { return &n }
	at := func(d time.Duration) *time.Time { t := now.Add(d); return &t }
	limits := map[string]*protocol.ProviderLimits{
		"openrouter": {
			Windows: []protocol.LimitWindow{
				{Span: protocol.LimitSpanMinute, Unit: protocol.LimitUnitRequests, Limit: 20, Source: protocol.LimitFromDocumented},
				{Span: protocol.LimitSpanDay, Unit: protocol.LimitUnitRequests, Limit: 1000, Remaining: count(964), Reset: at(13 * time.Hour), Source: protocol.LimitFromAccount, Seen: &seen},
			},
			Credits: &protocol.LimitCredits{Limit: money(10), Remaining: money(4.21), Used: 5.79, Currency: "USD", Source: protocol.LimitFromAccount, Seen: seen},
			Today:   protocol.LimitUse{Requests: 36, Tokens: 81400},
			Month:   protocol.LimitUse{Requests: 412, Tokens: 1204800},
		},
		"groq": {
			Windows: []protocol.LimitWindow{
				{Span: protocol.LimitSpanMinute, Unit: protocol.LimitUnitTokens, Limit: 8000, Remaining: count(7310), Reset: at(7 * time.Second), Source: protocol.LimitFromHeaders, Seen: &seen},
				{Span: protocol.LimitSpanMinute, Unit: protocol.LimitUnitRequests, Limit: 30, Source: protocol.LimitFromDocumented},
				{Span: protocol.LimitSpanDay, Unit: protocol.LimitUnitTokens, Limit: 500000, Remaining: count(412300), Reset: at(9 * time.Hour), Source: protocol.LimitFromHeaders, Seen: &seen},
			},
			Today: protocol.LimitUse{Requests: 18, Tokens: 87700},
			Month: protocol.LimitUse{Requests: 240, Tokens: 1310000},
		},
		"nvidia": {
			Windows: []protocol.LimitWindow{{Span: protocol.LimitSpanMinute, Unit: protocol.LimitUnitRequests, Limit: 40, Source: protocol.LimitFromDocumented}},
			Today:   protocol.LimitUse{Requests: 9, Tokens: 18200},
			Month:   protocol.LimitUse{Requests: 77, Tokens: 160400},
		},
		"ovh": {
			Windows: []protocol.LimitWindow{{Span: protocol.LimitSpanMinute, Unit: protocol.LimitUnitRequests, Limit: 2, Source: protocol.LimitFromDocumented}},
		},
		"sambanova": {
			Windows: []protocol.LimitWindow{
				{Span: protocol.LimitSpanMinute, Unit: protocol.LimitUnitRequests, Limit: 20, Source: protocol.LimitFromDocumented},
				{Span: protocol.LimitSpanDay, Unit: protocol.LimitUnitRequests, Limit: 20, Source: protocol.LimitFromDocumented},
				{Span: protocol.LimitSpanDay, Unit: protocol.LimitUnitTokens, Limit: 200000, Source: protocol.LimitFromDocumented},
			},
		},
		"deepseek": {
			Credits: &protocol.LimitCredits{Remaining: money(110), Currency: "CNY", Source: protocol.LimitFromAccount, Seen: seen},
		},
	}
	for i := range providers {
		if providers[i].Local {
			continue
		}
		if l := limits[providers[i].ID]; l != nil {
			providers[i].Limits = l
		} else {
			providers[i].Limits = &protocol.ProviderLimits{}
		}
	}
	return providers
}
