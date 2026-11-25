package gateway

import "ryoku-rashin/internal/protocol"

// documentedLimit is one published free limit: Limit of Unit per Span.
type documentedLimit struct {
	Span  string
	Unit  string
	Limit int64
}

// documentedLimits are the free limits the two MIT-licensed lists the
// catalogue comes from publish for providers that report none live:
// github.com/open-free-llm-api/awesome-freellm-apis (README "Rate Limit"
// column and status.json) and github.com/nejib1/Free-LLM (README provider
// directory), as of November 2026. Limits per hour, per second or in
// provider units such as Cloudflare neurons have no window here and are left
// out; a live figure for the same window always wins.
var documentedLimits = map[string][]documentedLimit{
	"nvidia":     {{protocol.LimitSpanMinute, protocol.LimitUnitRequests, 40}},
	"modelscope": {{protocol.LimitSpanDay, protocol.LimitUnitRequests, 2000}},
	"openrouter": {
		{protocol.LimitSpanMinute, protocol.LimitUnitRequests, 20},
		{protocol.LimitSpanDay, protocol.LimitUnitRequests, 50},
	},
	"groq": {
		{protocol.LimitSpanMinute, protocol.LimitUnitRequests, 30},
		{protocol.LimitSpanDay, protocol.LimitUnitRequests, 1000},
	},
	"mistral": {{protocol.LimitSpanMinute, protocol.LimitUnitTokens, 500000}},
	"cohere": {
		{protocol.LimitSpanMinute, protocol.LimitUnitRequests, 20},
		{protocol.LimitSpanMonth, protocol.LimitUnitRequests, 1000},
	},
	"aion": {
		{protocol.LimitSpanMinute, protocol.LimitUnitRequests, 15},
		{protocol.LimitSpanDay, protocol.LimitUnitTokens, 20000},
	},
	"cerebras": {{protocol.LimitSpanDay, protocol.LimitUnitTokens, 1000000}},
	"sambanova": {
		{protocol.LimitSpanMinute, protocol.LimitUnitRequests, 20},
		{protocol.LimitSpanDay, protocol.LimitUnitRequests, 20},
		{protocol.LimitSpanDay, protocol.LimitUnitTokens, 200000},
	},
	"agnes": {{protocol.LimitSpanMinute, protocol.LimitUnitRequests, 30}},
	"llm7": {
		{protocol.LimitSpanMinute, protocol.LimitUnitRequests, 60},
		{protocol.LimitSpanDay, protocol.LimitUnitTokens, 100000},
	},
	"ovh": {{protocol.LimitSpanMinute, protocol.LimitUnitRequests, 2}},
}
