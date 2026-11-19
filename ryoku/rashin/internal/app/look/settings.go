package look

import (
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
)

const (
	SettingsColumn  unit.Dp = 760
	SettingsControl unit.Dp = 320
)

// Setting is one row of a settings card, drawn by kit.SettingRow. A page
// builds its rows every frame and keeps no state for them.
type Setting struct {
	Title, Detail string
	Control       layout.Widget
	// puts the control under the text, for one too wide to stand beside it
	Below bool
	// the value differs from its default: the row says so and offers Reset
	Changed bool
	Reset   func()
	// why it cannot be set here; said in the detail's place, and the control is disabled
	Unavailable string
	// makes the whole row a button
	Activate func()
	// 0 to 1, the glow of a row a search landed on
	Highlight float32
}

func SettingsSection(gtx layout.Context, th *Theme, head Head, rows []Setting) layout.Dimensions {
	return Section(gtx, th, head, func(gtx layout.Context) layout.Dimensions {
		return settingsSlab(gtx, th, head.Title, rows)
	})
}

func SettingsSlab(gtx layout.Context, th *Theme, rows []Setting) layout.Dimensions {
	return settingsSlab(gtx, th, "", rows)
}

func settingsSlab(gtx layout.Context, th *Theme, head string, rows []Setting) layout.Dimensions {
	return kit.SlabContent(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		children := make([]layout.FlexChild, len(rows))
		for index, row := range rows {
			state := settingFor(gtx, head, row.Title)
			children[index] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layoutSetting(gtx, th, state, row, index == 0)
			})
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func layoutSetting(gtx layout.Context, th *Theme, row *kit.SettingRow, setting Setting, first bool) layout.Dimensions {
	row.Title, row.Detail = setting.Title, setting.Detail
	row.Changed, row.Unavailable = setting.Changed, setting.Unavailable
	row.NoReset = setting.Reset == nil
	row.Highlight, row.Stacked, row.First = setting.Highlight, setting.Below, first
	row.Activatable = setting.Activate != nil
	if row.Reset.Clicked(gtx) && setting.Reset != nil {
		setting.Reset()
	}
	if row.Activatable && row.Row.Clicked(gtx) {
		setting.Activate()
	}
	return row.Layout(gtx, th.Theme, setting.Control)
}

// rows are rebuilt every frame, so the kit row each one draws with (its hover, press and reset) is kept here by name
type settingKey struct {
	head, title string
	nth         int
}

type settingState struct {
	row  kit.SettingRow
	used time.Time
}

const (
	settingsRepeats = 16
	settingsKept    = 512
	settingsStale   = 30 * time.Second
)

var settings struct {
	rows map[settingKey]*settingState
	seen map[settingKey]int
	ops  *op.Ops
	now  time.Time
}

// the same head and title twice in one frame are two rows: each takes its own state in the order they are laid out
func settingFor(gtx layout.Context, head, title string) *kit.SettingRow {
	if settings.rows == nil {
		settings.rows = make(map[settingKey]*settingState)
		settings.seen = make(map[settingKey]int)
	}
	if gtx.Ops != settings.ops || !gtx.Now.Equal(settings.now) {
		settings.ops, settings.now = gtx.Ops, gtx.Now
		clear(settings.seen)
	}
	key := settingKey{head: head, title: title}
	key.nth = min(settings.seen[key], settingsRepeats-1)
	settings.seen[settingKey{head: head, title: title}]++
	state := settings.rows[key]
	if state == nil {
		if len(settings.rows) >= settingsKept {
			for old, kept := range settings.rows {
				if gtx.Now.Sub(kept.used) > settingsStale {
					delete(settings.rows, old)
				}
			}
		}
		state = &settingState{}
		settings.rows[key] = state
	}
	state.used = gtx.Now
	return &state.row
}

// a frame has ended: the next layout starts counting rows anew even when its clock and ops are the same
func endSettingsFrame() { settings.ops = nil }
