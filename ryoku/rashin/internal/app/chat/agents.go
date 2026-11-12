package chat

import (
	"image"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// agentClock keeps a subagent's time running between the reports that carry it
type agentClock struct {
	millis int64
	at     time.Time
}

func (clock *agentClock) elapsed(now time.Time, millis int64, running bool) time.Duration {
	if millis != clock.millis || clock.at.IsZero() {
		clock.millis, clock.at = millis, now
	}
	elapsed := time.Duration(clock.millis) * time.Millisecond
	if running {
		elapsed += now.Sub(clock.at)
	}
	return elapsed
}

func agentRunning(agent protocol.AgentWork) bool {
	return agent.Status == protocol.ToolRunning || agent.Status == protocol.ToolPending
}

// the box a call that runs subagents is drawn as: one line that counts them
// and says how they are doing, open while they run, and a row for each that
// opens to its steps and what it returned
func (state *viewState) layoutAgents(gtx layout.Context, th *look.Theme, model *modelState, tool *protocol.ToolData, key string) layout.Dimensions {
	visual := state.workVisual(key)
	running := toolRunning(tool)
	failed := false
	var longest time.Duration
	for _, agent := range tool.Agents {
		live := agentRunning(agent) && running
		running = running || live
		failed = failed || agent.Status == protocol.ToolFailed
		clock := state.agentClock(key + ":" + agent.ID)
		longest = max(longest, clock.elapsed(gtx.Now, agent.Millis, live && state.busy))
	}
	visual.follow(running)
	line := workLine{icon: look.Icon.Agents, label: agentsLabel(tool.Agents, running), prose: agentsSummary(tool.Agents), running: running, expandable: true}
	if longest >= time.Second {
		line.waited = waitedFor(longest)
	}
	if running && state.busy {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(time.Second)})
	}
	if len(tool.Agents) == 1 {
		agent := tool.Agents[0]
		if agent.Name != "" {
			line.label = agent.Name
		}
		line.prose, line.failed = agent.Task, failed && !running
		children := model.children[tool.Call]
		line.expandable = len(agent.Steps) > 0 || agent.Output != "" || len(children) > 0
		return state.layoutUnfolding(gtx, th, visual, line, func(gtx layout.Context) layout.Dimensions {
			return slab(gtx, th, 13, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return state.layoutAgentDetail(gtx, th, model, agent, children, key+":"+agent.ID)
			})
		})
	}
	return state.layoutUnfolding(gtx, th, visual, line, func(gtx layout.Context) layout.Dimensions {
		return slab(gtx, th, 8, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			children := make([]layout.FlexChild, 0, len(tool.Agents)*2)
			for index, agent := range tool.Agents {
				if index > 0 {
					children = append(children, layout.Rigid(layout.Spacer{Height: 2}.Layout))
				}
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return state.layoutAgentRow(gtx, th, model, agent, key+":"+agent.ID, running)
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

func (state *viewState) agentClock(key string) *agentClock {
	visual := state.workVisual(key)
	if visual.clock == nil {
		visual.clock = new(agentClock)
	}
	return visual.clock
}

// one subagent: what it is doing, or how it ended, and on a click its steps
// and its answer
func (state *viewState) layoutAgentRow(gtx layout.Context, th *look.Theme, model *modelState, agent protocol.AgentWork, key string, toolLive bool) layout.Dimensions {
	visual := state.workVisual(key)
	live := agentRunning(agent) && toolLive
	expandable := len(agent.Steps) > 0 || agent.Output != ""
	if visual.click.Clicked(gtx) && expandable {
		visual.open = !visual.open
	}
	elapsed := state.agentClock(key).elapsed(gtx.Now, agent.Millis, live && state.busy)
	height := gtx.Dp(32)
	head := func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, height)
		gtx.Constraints.Max.Y = height
		name := agent.Name
		if name == "" {
			name = i18n.T("Subagent")
		}
		meta := agentMeta(agent, elapsed, live)
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return state.layoutAgentMark(gtx, th, agent, live)
			}),
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
			layout.Rigid(kit.RawText(th.Theme, th.T.Label, th.R.Fg, name)),
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
			layout.Flexed(1, kit.RawText(th.Theme, th.T.Detail, th.R.Fg2, agent.Task)),
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(260))
				gtx.Constraints.Min = image.Point{}
				if agent.Status == protocol.ToolFailed {
					return kit.Claim(th.Theme, palette.Failed, i18n.T("Failed"))(gtx)
				}
				return kit.RawText(th.Theme, th.T.Caption, th.R.Fg3, meta)(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !expandable {
					return layout.Dimensions{Size: image.Pt(gtx.Dp(16), height)}
				}
				glyph := look.Icon.ExpandMore
				if visual.open {
					glyph = look.Icon.ExpandLess
				}
				return visual.morph.Layout(gtx, th, glyph, th.R.Fg3, 16)
			}),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
		)
	}
	record := op.Record(gtx.Ops)
	headSize := head(gtx).Size
	content := record.Stop()
	dimensions, hover := visual.hover.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		if !expandable {
			return layout.Dimensions{Size: headSize}
		}
		return visual.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: headSize} })
	})
	if expandable && hover > 0 {
		// the rows stand on a slab, so hover lifts toward the ink rather than to another stone
		kit.FillCut(gtx.Ops, image.Rectangle{Max: headSize}, gtx.Dp(th.Cut.Small), palette.WithAlpha(palette.Mix(th.R.Slab, th.R.Fg, tokens.AlphaHoverPlane), hover))
	}
	content.Add(gtx.Ops)
	if !expandable {
		return dimensions
	}
	indent := gtx.Dp(36)
	offset := op.Offset(image.Pt(indent, headSize.Y)).Push(gtx.Ops)
	body := gtx
	body.Constraints.Max.X = max(0, gtx.Constraints.Max.X-indent-gtx.Dp(8))
	body.Constraints.Min = image.Point{}
	unfolded := visual.unfold.Layout(body, th, visual.open, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: th.Sp.S1, Bottom: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return state.layoutAgentDetail(gtx, th, model, agent, nil, key)
		})
	})
	offset.Pop()
	return layout.Dimensions{Size: image.Pt(dimensions.Size.X, headSize.Y+unfolded.Size.Y)}
}

func (state *viewState) layoutAgentMark(gtx layout.Context, th *look.Theme, agent protocol.AgentWork, live bool) layout.Dimensions {
	size := image.Pt(gtx.Dp(16), gtx.Dp(16))
	return look.MiddleLeft(gtx, 0, size.Y, func(gtx layout.Context) layout.Dimensions {
		switch {
		case live && agent.Status == protocol.ToolRunning:
			return kit.Working{Active: true, Size: 16}.Layout(gtx, th.Theme)
		case agent.Status == protocol.ToolDone:
			return look.IconWidget(look.Icon.Check, 16, th.R.Jade)(gtx)
		case agent.Status == protocol.ToolFailed:
			return look.IconWidget(look.Icon.Error, 16, th.R.Jasper)(gtx)
		}
		return look.MiddleLeft(gtx, gtx.Dp(2), size.Y, func(gtx layout.Context) layout.Dimensions {
			return kit.Flake(gtx, th.Theme, palette.Idle, 1, 9)
		})
	})
}

// the steps a subagent took, its own calls when the harness reports them,
// then what it returned
func (state *viewState) layoutAgentDetail(gtx layout.Context, th *look.Theme, model *modelState, agent protocol.AgentWork, children []*protocol.ToolData, key string) layout.Dimensions {
	var rows []layout.FlexChild
	switch {
	case len(children) > 0:
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return state.layoutToolLines(gtx, th, model, children, key+":calls")
		}))
	case len(agent.Steps) > 0:
		if hidden := agent.StepCount - len(agent.Steps); hidden > 0 {
			earlier := i18n.Tf("%d earlier steps", hidden)
			if hidden == 1 {
				earlier = i18n.T("1 earlier step")
			}
			rows = append(rows, layout.Rigid(kit.RawText(th.Theme, th.T.Caption, th.R.Fg3, earlier)))
		}
		for index, step := range agent.Steps {
			if index > 0 {
				rows = append(rows, layout.Rigid(layout.Spacer{Height: 2}.Layout))
			}
			rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				ink := th.R.Fg2
				if step.Failed {
					ink = th.R.Jasper
				}
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return look.MiddleLeft(gtx, 0, gtx.Dp(24), func(gtx layout.Context) layout.Dimensions {
							return kit.Flake(gtx, th.Theme, palette.Idle, 0.6, 7)
						})
					}),
					layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
					layout.Flexed(1, kit.RawText(th.Theme, th.T.Caption, ink, step.Title)),
				)
			}))
		}
	}
	if output := strings.TrimSpace(agent.Output); output != "" {
		if len(rows) > 0 {
			rows = append(rows, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout))
		}
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			ink := th.R.Fg
			if agent.Status == protocol.ToolFailed {
				ink = th.R.Fg2
			}
			return state.selectable(gtx, th, key+":output", th.T.Body, ink, output)
		}))
	}
	if facts := agentFacts(agent); facts != "" {
		rows = append(rows,
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(kit.RawText(th.Theme, th.T.Caption, th.R.Fg3, facts)),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}

// "Running 3 subagents" while any works, "Ran 3 subagents" once all ended
func agentsLabel(agents []protocol.AgentWork, running bool) string {
	if len(agents) == 1 {
		if running {
			return i18n.T("Running a subagent")
		}
		return i18n.T("Ran a subagent")
	}
	if running {
		return i18n.Tf("Running %d subagents", len(agents))
	}
	return i18n.Tf("Ran %d subagents", len(agents))
}

// "2 working, 1 done, 1 failed", or a lone subagent's task
func agentsSummary(agents []protocol.AgentWork) string {
	if len(agents) == 1 {
		return agents[0].Task
	}
	var working, done, failed, stopped, waiting int
	for _, agent := range agents {
		switch agent.Status {
		case protocol.ToolRunning:
			working++
		case protocol.ToolDone:
			done++
		case protocol.ToolFailed:
			failed++
		case protocol.ToolStopped:
			stopped++
		default:
			waiting++
		}
	}
	var parts []string
	for _, part := range []struct {
		count  int
		phrase string
	}{{working, "%d working"}, {waiting, "%d waiting"}, {done, "%d done"}, {failed, "%d failed"}, {stopped, "%d stopped"}} {
		if part.count > 0 {
			parts = append(parts, i18n.Tf(part.phrase, part.count))
		}
	}
	return look.Clauses(parts...)
}

// what sits at the end of a row: the step it is on while it runs, else its
// steps and time, "7 steps in 1m 24s"
func agentMeta(agent protocol.AgentWork, elapsed time.Duration, live bool) string {
	if live && agent.Doing != "" {
		return agent.Doing
	}
	switch agent.Status {
	case protocol.ToolPending:
		return i18n.T("Waiting")
	case protocol.ToolStopped:
		return i18n.T("Stopped")
	}
	steps := ""
	switch {
	case agent.StepCount == 1:
		steps = i18n.T("1 step")
	case agent.StepCount > 1:
		steps = i18n.Tf("%d steps", agent.StepCount)
	}
	switch {
	case elapsed < time.Second:
		return steps
	case steps == "":
		return waitedFor(elapsed)
	}
	return i18n.Tf("%s in %s", steps, waitedFor(elapsed))
}

// the tokens a subagent used and the model it ran on, "18,200 tokens on haiku-4-5"
func agentFacts(agent protocol.AgentWork) string {
	switch {
	case agent.Tokens > 0 && agent.Model != "":
		return i18n.Tf("%s tokens on %s", look.Grouped(agent.Tokens), agent.Model)
	case agent.Tokens > 0:
		return i18n.Tf("%s tokens", look.Grouped(agent.Tokens))
	}
	return agent.Model
}
