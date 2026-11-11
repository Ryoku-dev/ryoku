package teamsui

import (
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
)

// the token limit's bounds and the steps it moves in: 100k under a million,
// 500k from there
const (
	tokensLeast int64 = 100_000
	tokensMost  int64 = 100_000_000
	tokensFine  int64 = 100_000
	tokensWide  int64 = 500_000
	tokensTurn  int64 = 1_000_000
)

// TokenStepper sets a monthly limit in tokens and shows it the way the rest
// of the app counts them, 2.0M rather than 2000000.
type TokenStepper struct {
	Value      int64
	less, more kit.IconPlaque
	changed    bool
}

// Changed reports and clears whether a press moved the value.
func (s *TokenStepper) Changed() bool {
	changed := s.changed
	s.changed = false
	return changed
}

func (s *TokenStepper) Layout(gtx layout.Context, th *look.Theme) layout.Dimensions {
	s.Value = min(max(s.Value, tokensLeast), tokensMost)
	if s.less.Clicked(gtx) && s.Value > tokensLeast {
		step := tokensFine
		if s.Value > tokensTurn {
			step = tokensWide
		}
		s.Value = max(tokensLeast, roundDown(s.Value-1, step))
		s.changed = true
	}
	if s.more.Clicked(gtx) && s.Value < tokensMost {
		step := tokensFine
		if s.Value >= tokensTurn {
			step = tokensWide
		}
		s.Value = min(tokensMost, roundDown(s.Value, step)+step)
		s.changed = true
	}
	s.less.Kind, s.less.Small, s.less.Icon, s.less.Description = kit.PlaqueQuiet, true, look.Icon.Remove, i18n.T("Fewer tokens")
	s.more.Kind, s.more.Small, s.more.Icon, s.more.Description = kit.PlaqueQuiet, true, look.Icon.Add, i18n.T("More tokens")
	less, more := gtx, gtx
	if s.Value <= tokensLeast {
		less = less.Disabled()
	}
	if s.Value >= tokensMost {
		more = more.Disabled()
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(tokens.SizeStepperGap))}.Layout(gtx,
		layout.Rigid(func(layout.Context) layout.Dimensions { return s.less.Layout(less, th.Theme) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(kit.Text(th.Theme, th.T.Mono, th.R.Fg, Tokens(s.Value))),
				layout.Rigid(layout.Spacer{Width: unit.Dp(tokens.SizeStepperUnitGap)}.Layout),
				layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, i18n.T("tokens a month"))),
			)
		}),
		layout.Rigid(func(layout.Context) layout.Dimensions { return s.more.Layout(more, th.Theme) }),
	)
}

// value down to a whole number of steps
func roundDown(value, step int64) int64 { return value / step * step }
