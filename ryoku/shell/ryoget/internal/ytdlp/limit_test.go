package ytdlp

import (
	"context"
	"testing"

	"golang.org/x/time/rate"

	"ryoget/internal/plan"
)

func TestLimitRateTakesTheSmallerLimit(t *testing.T) {
	lim := func(bps int) *rate.Limiter { return rate.NewLimiter(rate.Limit(bps), 256<<10) }
	unlimited := rate.NewLimiter(rate.Inf, 256<<10)
	for _, c := range []struct {
		name        string
		shared, own *rate.Limiter
		want        int64
	}{
		{"neither", nil, nil, 0},
		{"both unlimited", unlimited, unlimited, 0},
		{"shared only", lim(512 << 10), nil, 512 << 10},
		{"own only", unlimited, lim(64 << 10), 64 << 10},
		{"own below shared", lim(512 << 10), lim(64 << 10), 64 << 10},
		{"shared below own", lim(32 << 10), lim(64 << 10), 32 << 10},
	} {
		t.Run(c.name, func(t *testing.T) {
			env := &plan.Env{}
			if c.shared != nil {
				env.Limit = c.shared
			}
			ctx := context.Background()
			if c.own != nil {
				ctx = plan.WithLimit(ctx, c.own)
			}
			if got := limitRate(ctx, env); got != c.want {
				t.Fatalf("limitRate = %d, want %d", got, c.want)
			}
		})
	}
	if got := limitRate(plan.WithLimit(context.Background(), lim(8<<10)), nil); got != 8<<10 {
		t.Fatalf("limitRate with no env = %d", got)
	}
}
