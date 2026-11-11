package teams

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	appstate "ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// how a crew chart is drawn: full tiles on a team's page, small ones beside a draft;
// an edge with no alpha draws none
type crewStyle struct {
	tile, gapX, gapY, mark unit.Dp
	plane, edge            color.NRGBA
	mini                   bool
}

// on the pane a tile lifts as a slab, its vein carrying the edge
func fullCrew(th *look.Theme) crewStyle {
	return crewStyle{tile: 200, gapX: th.Sp.S3, gapY: th.Sp.S5, mark: 36, plane: th.R.Slab, edge: th.R.Vein}
}

// the aside is a slab, so its tiles stand one step up on the field; a
// mini tile is a mark and a name, two to a row as wide as the aside lets them
// be, so a name such as Researcher reads whole; its title is in the form beside it
func miniCrew(th *look.Theme) crewStyle {
	return crewStyle{gapX: th.Sp.S2, gapY: th.Sp.S4, mark: 24, plane: th.R.Field, mini: true}
}

type placedTile struct {
	call op.CallOp
	size image.Point
}

// the lead's tile centred on top, a vein down from it and across, the members in rows under it;
// live is nil where the crew is only drafted, as no member of it works yet
func crewChart(gtx layout.Context, th *look.Theme, snapshot appstate.Snapshot, members []protocol.Member, live *crewLive, style crewStyle) layout.Dimensions {
	lead, leadIndex := teamsui.Lead(members)
	if leadIndex < 0 {
		return layout.Dimensions{}
	}
	width := gtx.Constraints.Max.X
	gapX, gapY := gtx.Dp(style.gapX), gtx.Dp(style.gapY)
	tileWidth := min(width, gtx.Dp(style.tile))
	if style.mini {
		tileWidth = max(0, (width-gapX)/2)
	}
	record := func(member protocol.Member) placedTile {
		child := gtx
		child.Constraints = layout.Constraints{Min: image.Pt(tileWidth, 0), Max: image.Pt(tileWidth, unbounded)}
		macro := op.Record(gtx.Ops)
		dimensions := crewTile(child, th, snapshot, member, live, style)
		return placedTile{call: macro.Stop(), size: dimensions.Size}
	}
	top := record(lead)
	perRow := max(1, (width+gapX)/(tileWidth+gapX))
	var rows [][]placedTile
	for index, member := range members {
		if index == leadIndex {
			continue
		}
		if len(rows) == 0 || len(rows[len(rows)-1]) == perRow {
			rows = append(rows, nil)
		}
		rows[len(rows)-1] = append(rows[len(rows)-1], record(member))
	}

	line := max(1, gtx.Dp(1))
	trunk := width / 2
	y := top.size.Y
	type placedRow struct {
		y, height int
		xs        []int
	}
	placed := make([]placedRow, len(rows))
	for index, row := range rows {
		y += gapY
		height := 0
		for _, tile := range row {
			height = max(height, tile.size.Y)
		}
		span := len(row)*tileWidth + (len(row)-1)*gapX
		x := (width - span) / 2
		xs := make([]int, len(row))
		for column := range row {
			xs[column] = x + column*(tileWidth+gapX)
		}
		placed[index] = placedRow{y: y, height: height, xs: xs}
		y += height
	}
	// the veins go under the tiles so a tile's edge is never crossed
	if len(placed) > 0 {
		last := placed[len(placed)-1]
		look.FillRect(gtx, image.Rect(trunk, top.size.Y, trunk+line, last.y-gapY/2), th.R.Vein)
		for _, row := range placed {
			bar := row.y - gapY/2
			left, right := min(trunk, row.xs[0]+tileWidth/2), max(trunk, row.xs[len(row.xs)-1]+tileWidth/2)
			look.FillRect(gtx, image.Rect(left, bar, right+line, bar+line), th.R.Vein)
			for _, x := range row.xs {
				look.FillRect(gtx, image.Rect(x+tileWidth/2, bar, x+tileWidth/2+line, row.y), th.R.Vein)
			}
		}
	}
	draw := func(tile placedTile, at image.Point) {
		stack := op.Offset(at).Push(gtx.Ops)
		bounds, cut := image.Rectangle{Max: tile.size}, gtx.Dp(th.Cut.Mid)
		kit.FillCut(gtx.Ops, bounds, cut, style.plane)
		if style.edge.A > 0 {
			kit.StrokeCut(gtx.Ops, bounds, cut, line, style.edge)
		}
		tile.call.Add(gtx.Ops)
		stack.Pop()
	}
	draw(top, image.Pt((width-tileWidth)/2, 0))
	for index, row := range rows {
		for column, tile := range row {
			draw(tile, image.Pt(placed[index].xs[column], placed[index].y))
		}
	}
	return layout.Dimensions{Size: image.Pt(width, y)}
}

func crewTile(gtx layout.Context, th *look.Theme, snapshot appstate.Snapshot, member protocol.Member, live *crewLive, style crewStyle) layout.Dimensions {
	inset := th.Sp.S3
	if style.mini {
		inset = th.Sp.S2
	}
	identity := func(gtx layout.Context) layout.Dimensions {
		words := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: th.T.Label, Color: th.R.Fg, MaxLines: 1}.Layout(gtx, th.Theme, member.Name)
			}),
		}
		if member.Title != "" && !style.mini {
			words = append(words, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: th.T.Detail, Color: th.R.Fg2, MaxLines: 1}.Layout(gtx, th.Theme, member.Title)
			}))
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(teamsui.Mark(th, member.Name, style.mark, member.Lead)),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, words...)
			}),
		)
	}
	return layout.UniformInset(inset).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		if style.mini {
			return identity(gtx)
		}
		engine := teamsui.MemberEngine(snapshot, member)
		harnessInk := th.R.Fg2
		if engine.Missing {
			harnessInk = th.R.Jasper
		}
		model := engine.Model
		if engine.Extra != "" {
			model = i18n.Tf("%s · %s", model, engine.Extra)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(identity),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			// the engine wraps rather than cuts, a tile in a row as tall as its tallest
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: th.T.Detail, Color: harnessInk, MaxLines: 2}.Layout(gtx, th.Theme, engine.Harness)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: th.T.Detail, Color: th.R.Fg2, MaxLines: 2}.Layout(gtx, th.Theme, model)
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				task, working := teamsui.MemberTask(snapshot, live.team, member)
				now, inHand := teamsui.TaskLive(snapshot, task)
				if !working || !inHand {
					return kit.Text(th.Theme, th.T.Caption, th.R.Fg3, i18n.T("Idle"))(gtx)
				}
				// the name stands at the top of the tile, and what it is on reads while it says nothing yet
				now.Member = ""
				if now.Caption == "" {
					now.Caption = task.Title
				}
				presence := live.presence(member.ID)
				if _, screen := snapshot.Screens[task.Session]; !screen || task.Session == "" {
					return presence.Layout(gtx, th, now)
				}
				// a member working on a screen of its own says so, as the task page shows it
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return presence.Layout(gtx, th, now) }),
					layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(look.IconWidget(look.Icon.Screen, 14, th.R.Fg2)),
							layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
							layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg2, i18n.T("Screen"))),
						)
					}),
				)
			}),
		)
	})
}

// the presence of each member at work on the crew chart shown, kept so a caption cross-fades as it changes
type crewLive struct {
	team    string
	members map[string]*teamsui.Presence
}

// the chart's presence for a team, begun anew when the team shown changes
func (live *crewLive) of(team string) *crewLive {
	if live.team != team {
		*live = crewLive{team: team}
	}
	return live
}

func (live *crewLive) presence(member string) *teamsui.Presence {
	if live.members == nil {
		live.members = make(map[string]*teamsui.Presence)
	}
	presence := live.members[member]
	if presence == nil {
		presence = new(teamsui.Presence)
		live.members[member] = presence
	}
	return presence
}
