package demo

import (
	"time"

	"ryoku-rashin/internal/protocol"
)

// one model's requests and failures on each of the days before today, newest first; a run longer than the table starts it again
type usageHistory struct {
	provider, model string
	input, output   int64
	estimated       bool
	requests        []int
	failed          []int
}

var demoUsageHistory = []usageHistory{
	{
		provider: "openrouter", model: "free-router", input: 1490, output: 450,
		requests: []int{14, 2, 0, 9, 18, 11, 7, 12, 0, 1, 16, 8, 12, 10},
		failed:   []int{1, 0, 0, 0, 2, 0, 0, 1, 0, 0, 1, 0, 0, 0},
	},
	{
		provider: "puter", model: "gpt-oss-120b", input: 820, output: 340, estimated: true,
		requests: []int{4, 0, 2, 1, 6, 3, 0, 5, 1, 0, 3, 2, 0, 2},
		failed:   []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	},
	{
		provider: "groq", model: "llama-3.3-70b-versatile", input: 1180, output: 380,
		requests: []int{3, 0, 0, 2, 4, 0, 1, 2, 0, 0, 3, 2, 1, 0},
		failed:   []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0},
	},
}

// the gateway's last days of requests per model, ending on the demo's day: a busy weekday, quiet weekends, a few failures
func demoUsage(now time.Time, days int) protocol.GatewayUsage {
	if days <= 0 {
		days = 7
	}
	today := now.Format("2006-01-02")
	usage := protocol.GatewayUsage{Days: days, Rows: []protocol.UsageRow{
		{Day: today, Provider: "openrouter", Model: "free-router", Requests: 41, OK: 38, Failed: 3, InputTokens: 61200, OutputTokens: 18400},
		{Day: today, Provider: "puter", Model: "gpt-oss-120b", Requests: 12, OK: 12, InputTokens: 9800, OutputTokens: 4100, Estimated: true},
	}}
	for back := 1; back < days; back++ {
		day := now.AddDate(0, 0, -back).Format("2006-01-02")
		for _, model := range demoUsageHistory {
			index := (back - 1) % len(model.requests)
			requests, failed := model.requests[index], model.failed[index]
			if requests == 0 {
				continue
			}
			// longer prompts on some days, so tokens do not follow requests exactly
			scale := int64(10 + back%3)
			usage.Rows = append(usage.Rows, protocol.UsageRow{
				Day: day, Provider: model.provider, Model: model.model,
				Requests: requests, OK: requests - failed, Failed: failed,
				InputTokens: int64(requests) * model.input * scale / 10, OutputTokens: int64(requests) * model.output * scale / 10,
				Estimated: model.estimated,
			})
		}
	}
	return usage
}
