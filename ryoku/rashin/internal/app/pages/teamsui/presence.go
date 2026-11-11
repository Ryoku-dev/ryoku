package teamsui

import (
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// a caption stays this long before the next one fades in, so a streaming answer reads instead of flickering
const captionHold = 2 * time.Second

// what a task in hand shows of the member on it
type Live struct {
	// empty where the name already stands beside it
	Member string
	// set while the task waits on the user
	Waiting bool
	// what the member does now, and the latest it said or what it waits on
	Status, Caption string
	// when its turn or its wait began, zero when that is not known
	Since time.Time
	Steps int
}

// a working or waiting task's presence; false for a task not in hand
func TaskLive(snapshot state.Snapshot, task protocol.TeamTask) (Live, bool) {
	if task.Status != protocol.TaskWorking && task.Status != protocol.TaskNeedsYou {
		return Live{}, false
	}
	info := snapshot.Sessions[task.Session]
	live := Live{Member: MemberOf(snapshot, task.Team, task.Member), Caption: strings.Join(strings.Fields(info.Preview), " "), Steps: info.Steps}
	if task.Status == protocol.TaskNeedsYou || task.Waiting == protocol.WaitingOnYou || info.Pending > 0 {
		live.Waiting, live.Status, live.Since = true, i18n.T("Needs you"), task.Updated
		for _, approval := range snapshot.Approvals {
			if approval.Task == task.ID && approval.Status == protocol.ApprovalPending {
				live.Caption, live.Since = approval.Title, approval.Created
				break
			}
		}
		return live, true
	}
	switch {
	case info.Activity != "":
		live.Status = info.Activity
	case info.TurnStarted != nil:
		live.Status = i18n.T("Thinking")
	default:
		live.Status, _, _ = TaskStanding(task)
	}
	if info.TurnStarted != nil {
		live.Since = *info.TurnStarted
	}
	return live, true
}

// the task a member has in hand, working or waiting on the user, the newest when there are more
func MemberTask(snapshot state.Snapshot, team string, member protocol.Member) (protocol.TeamTask, bool) {
	var found protocol.TeamTask
	ok := false
	for _, task := range snapshot.TeamTasks {
		mine := task.Member == member.ID || task.Member == "" && member.Lead
		live := task.Status == protocol.TaskWorking || task.Status == protocol.TaskNeedsYou
		if task.Team == team && mine && live && (!ok || Started(task).After(Started(found))) {
			found, ok = task, true
		}
	}
	return found, ok
}

// "Working for 1m 24s · 7 steps", or "Waiting for 3m" while it waits on the user
func (live Live) Clock(now time.Time) string {
	if live.Waiting {
		if live.Since.IsZero() {
			return ""
		}
		return i18n.Tf("Waiting for %s", span(now.Sub(live.Since), false))
	}
	switch {
	case live.Since.IsZero() && live.Steps == 0:
		return ""
	case live.Since.IsZero():
		return stepsWord(live.Steps)
	case live.Steps == 0:
		return i18n.Tf("Working for %s", span(now.Sub(live.Since), true))
	}
	return i18n.Tf("Working for %s · %s", span(now.Sub(live.Since), true), stepsWord(live.Steps))
}

// the time alone, for a row with room only for it
func (live Live) Elapsed(now time.Time) string {
	if live.Since.IsZero() {
		return ""
	}
	return span(now.Sub(live.Since), !live.Waiting)
}

// how a finished task's work went, for the title over its transcript, and the tone
// of a step that failed or waited on the user, Idle when none did
func Worked(snapshot state.Snapshot, task protocol.TeamTask) (string, palette.Tone) {
	tone := palette.Idle
	for _, approval := range snapshot.Approvals {
		if approval.Task == task.ID {
			tone = palette.Attention
			break
		}
	}
	// a finished task may have let go of its session; the last one it worked in is the one that ended it
	session := task.Session
	if session == "" && len(task.Sessions) > 0 {
		session = task.Sessions[len(task.Sessions)-1]
	}
	info := snapshot.Sessions[session]
	if task.Status == protocol.TaskBlocked || info.Error != "" {
		tone = palette.Failed
	}
	if task.Started == nil {
		return i18n.T("How it went"), tone
	}
	worked := span(Ended(task).Sub(*task.Started), true)
	if info.Steps == 0 {
		return i18n.Tf("Worked for %s", worked), tone
	}
	return i18n.Tf("Worked for %s · %s", worked, stepsWord(info.Steps)), tone
}

// how a member at work shows wherever its work is watched; it keeps the caption
// shown so a new one cross-fades in, and asks for a frame only when the clock's
// words change, while the working flakes run
type Presence struct {
	caption look.Swap
	shown   string
	at      time.Time
}

// the flakes, the member and what it does now; the latest it said; how long and in how many steps
func (p *Presence) Layout(gtx layout.Context, th *look.Theme, live Live) layout.Dimensions {
	live.tick(gtx)
	caption := p.settle(gtx, live.Caption)
	clock := live.Clock(gtx.Now)
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			row := []layout.FlexChild{
				layout.Rigid(presenceMark(th, live.Waiting, 14)),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			}
			if live.Member != "" {
				row = append(row,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return kit.Label{Style: th.T.Label, Color: th.R.Fg, MaxLines: 1}.Layout(gtx, th.Theme, live.Member)
					}),
					layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				)
			}
			row = append(row, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: th.T.Caption, Color: statusInk(th, live.Waiting), MaxLines: 1}.Layout(gtx, th.Theme, live.Status)
			}))
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, row...)
		}),
	}
	if caption != "" {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return p.caption.Layout(gtx, th, caption, 0, th.Motion.Select, func(gtx layout.Context, text string) layout.Dimensions {
					return kit.Label{Style: th.T.Caption, Color: th.R.Fg2, MaxLines: 1}.Layout(gtx, th.Theme, text)
				})
			}),
		)
	}
	if clock != "" {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(kit.Text(th.Theme, th.T.MonoSm, th.R.Fg3, clock)),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// one line for a row with room for no more: the flakes, the lead words, then what
// the member does now, which cross-fades as it changes
func (p *Presence) Line(gtx layout.Context, th *look.Theme, live Live, lead string) layout.Dimensions {
	live.tick(gtx)
	status := p.settle(gtx, live.Status)
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(presenceMark(th, live.Waiting, 12)),
		layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.Label{Style: th.T.Caption, Color: th.R.Fg3, MaxLines: 1}.Layout(gtx, th.Theme, i18n.Tf("%s ·", lead))
		}),
		layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return p.caption.Layout(gtx, th, status, 0, th.Motion.Select, func(gtx layout.Context, text string) layout.Dimensions {
				return kit.Label{Style: th.T.Caption, Color: statusInk(th, live.Waiting), MaxLines: 1}.Layout(gtx, th.Theme, text)
			})
		}),
	)
}

// a new text replaces the shown one only once that has stood for captionHold
func (p *Presence) settle(gtx layout.Context, text string) string {
	if text == p.shown {
		return text
	}
	if p.at.IsZero() || gtx.Now.Sub(p.at) >= captionHold {
		p.shown, p.at = text, gtx.Now
	} else {
		gtx.Execute(op.InvalidateCmd{At: p.at.Add(captionHold)})
	}
	return p.shown
}

// a frame when the clock's words next change: each second while it works or
// has waited under a minute, each minute after
func (live Live) tick(gtx layout.Context) {
	if live.Since.IsZero() {
		return
	}
	elapsed := gtx.Now.Sub(live.Since)
	step := time.Second
	if live.Waiting && elapsed >= time.Minute {
		step = time.Minute
	}
	gtx.Execute(op.InvalidateCmd{At: live.Since.Add((elapsed/step + 1) * step)})
}

// the working flakes, or one still attention flake while it waits on the user
func presenceMark(th *look.Theme, waiting bool, size unit.Dp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		side := gtx.Dp(size)
		gtx.Constraints = layout.Exact(image.Pt(side, side))
		if !waiting {
			return kit.Working{Active: true, Size: size}.Layout(gtx, th.Theme)
		}
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			return kit.Flake(gtx, th.Theme, palette.Attention, 1, size*6/10)
		})
	}
}

func statusInk(th *look.Theme, waiting bool) color.NRGBA {
	if waiting {
		return th.R.Stone(palette.Attention)
	}
	return th.R.Stone(palette.Becoming)
}

func stepsWord(count int) string {
	if count == 1 {
		return i18n.T("1 step")
	}
	return i18n.Tf("%d steps", count)
}

// a length of time in its two largest units; minutes are the finest unit once a wait passes one
func span(length time.Duration, seconds bool) string {
	length = max(length, 0)
	switch {
	case length < time.Minute:
		return i18n.Tf("%ds", int(length.Seconds()))
	case length < time.Hour && seconds:
		return i18n.Tf("%dm %ds", int(length.Minutes()), int(length.Seconds())%60)
	case length < time.Hour:
		return i18n.Tf("%dm", int(length.Minutes()))
	}
	return i18n.Tf("%dh %dm", int(length.Hours()), int(length.Minutes())%60)
}
