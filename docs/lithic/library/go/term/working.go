package term

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Working describes one static frame of the working mark.
type Working struct {
	Working bool
	Frame   int
}

// RenderWorking renders one frozen frame of the working mark.
func RenderWorking(th Theme, working Working) string {
	g := glyphs()
	if len(g.WorkingFrames) == 0 {
		return ""
	}
	frame := working.Frame % len(g.WorkingFrames)
	return ink(th.Roles.Lapis).Render(g.WorkingFrames[frame])
}

type workingTick time.Time

// WorkingModel animates only while Working is true.
type WorkingModel struct {
	Theme   Theme
	Working Working
}

func workingTickCmd() tea.Cmd {
	return tea.Tick(tokens.MotionWorking/time.Duration(tokens.SizeWorkingFlakeCount), func(now time.Time) tea.Msg { return workingTick(now) })
}

func (m WorkingModel) Init() tea.Cmd {
	if m.Working.Working {
		return workingTickCmd()
	}
	return nil
}

func (m WorkingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.Working.Working {
		return m, nil
	}
	if _, ok := msg.(workingTick); ok {
		m.Working.Frame = (m.Working.Frame + 1) % tokens.SizeWorkingFlakeCount
		return m, workingTickCmd()
	}
	return m, nil
}

func (m WorkingModel) View() tea.View { return tea.NewView(RenderWorking(m.Theme, m.Working)) }
