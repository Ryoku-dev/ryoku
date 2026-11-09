package term

import (
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// BladeMeter describes a quiet resource meter.
type BladeMeter struct {
	Width    int
	Fraction float64
}

// RenderBladeMeter renders a quiet meter that turns amber at 90 percent.
func RenderBladeMeter(th Theme, meter BladeMeter) string {
	tone := palette.Idle
	if meter.Fraction >= tokens.SizeBladeMeterAlertAt {
		tone = palette.Attention
	}
	return RenderBlade(th, meter.Width, meter.Fraction, tone)
}
