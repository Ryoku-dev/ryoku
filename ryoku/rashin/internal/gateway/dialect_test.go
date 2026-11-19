package gateway

import (
	"testing"

	"ryoku-rashin/internal/gateway/textcalls"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

func TestWrapDialectOnlyScansLocalNativeText(t *testing.T) {
	request := llm.Request{Tools: []llm.Tool{{Name: "read"}}}
	for _, tc := range []struct {
		name, tier, dialect string
		wrapped             bool
	}{
		{name: "remote free native", tier: protocol.TierFree},
		{name: "remote paid native", tier: protocol.TierPaid},
		{name: "local native", tier: protocol.TierLocal, wrapped: true},
		{name: "remote explicit text", tier: protocol.TierFree, dialect: textcalls.Hermes, wrapped: true},
		{name: "remote explicit auto", tier: protocol.TierFree, dialect: textcalls.Auto, wrapped: true},
		{name: "local explicit native", tier: protocol.TierLocal, dialect: textcalls.Native},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inner := &sseAttempt{}
			got := wrapDialect(inner, &provider{Tier: tc.tier}, &model{ToolDialect: tc.dialect}, request)
			if tc.wrapped {
				if wrapped, ok := got.(*dialectAttempt); !ok || wrapped.inner != inner {
					t.Fatalf("wrapDialect = %T, want a dialect attempt", got)
				}
			} else if got != inner {
				t.Fatalf("wrapDialect = %T, want original attempt", got)
			}
		})
	}
	if inner := (&sseAttempt{}); wrapDialect(inner, &provider{Tier: protocol.TierLocal}, &model{ToolDialect: textcalls.Hermes}, llm.Request{}) != inner {
		t.Fatal("request without tools must not scan")
	}
}
