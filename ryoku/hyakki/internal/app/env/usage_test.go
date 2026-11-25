package env

import (
	"testing"

	"ryoku-rashin/internal/protocol"
)

// one answer is "1 answer", not "1 answers", in a provider's row and in the trend's proof
func TestUsageCountsReadInTheSingularForOne(t *testing.T) {
	for _, c := range []struct {
		ok, failed int
		want       string
	}{
		{0, 0, ""},
		{1, 0, "1 answer this week"},
		{2, 0, "2 answers this week"},
		{1, 1, "1 answer and 1 failure this week"},
		{3, 2, "3 answers and 2 failures this week"},
		{0, 1, "0 answers and 1 failure this week"},
	} {
		usage := Usage{result: protocol.GatewayUsage{Rows: []protocol.UsageRow{{Day: "2026-11-11", Provider: "kilo", OK: c.ok, Failed: c.failed}}}}
		if got := usage.Words("kilo"); got != c.want {
			t.Errorf("answered %d, failed %d reads %q, want %q", c.ok, c.failed, got, c.want)
		}
	}
	days := []string{"2026-11-11"}
	one := protocol.GatewayUsage{Rows: []protocol.UsageRow{{Day: "2026-11-11", Provider: "kilo", Requests: 1, OK: 1}}}
	if got := answersTrend(one, days, true).Proof; got != "1 answer this week, none failed" {
		t.Errorf("the week's proof reads %q", got)
	}
	if got := answersTrend(one, days, false).Proof; got != "1 answer in two weeks, none failed" {
		t.Errorf("two weeks' proof reads %q", got)
	}
}
