package teamsui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// a tile measures against no height limit
const unbounded = 1 << 24

// a full tile holds a name, a title and a model line that wraps once; a mini tile's face is a crew strip's
const (
	crewTile     unit.Dp = 200
	crewMiniMark unit.Dp = 24
)

// CrewStyle is how a crew chart is drawn: full tiles on a team's page, small
// ones beside a draft
type CrewStyle struct {
	tile, gapX, gapY, mark unit.Dp
	mini                   bool
	// Head stands above the lead on a tile of its own, as a dream's CEO does
	Head layout.Widget
}

// on the pane a tile is the design's card: drawn by its hairline in Lithic, a tonal card where corners are round
func FullCrew(th *look.Theme) CrewStyle {
	return CrewStyle{tile: crewTile, gapX: th.Sp.S3, gapY: th.Sp.S5, mark: unit.Dp(tokens.SizeTabletMark)}
}

// the aside is a card already, so its tiles are drawn by a hairline alone; a
// mini tile is a mark and a name, two to a row as wide as the aside lets them
// be, so a name such as Researcher reads whole; its title is in the form beside it
func MiniCrew(th *look.Theme) CrewStyle {
	return CrewStyle{gapX: th.Sp.S2, gapY: th.Sp.S4, mark: crewMiniMark, mini: true}
}

// CrewIdentity is a tile's mark beside a name and a title, as every crew tile opens
func CrewIdentity(th *look.Theme, style CrewStyle, name, title string, lead bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		words := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: th.T.Label, Color: th.R.Fg, MaxLines: 1}.Layout(gtx, th.Theme, name)
			}),
		}
		if title != "" && !style.mini {
			words = append(words, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: th.T.Detail, Color: th.R.Fg2, MaxLines: 1}.Layout(gtx, th.Theme, title)
			}))
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(Mark(th, name, style.mark, lead)),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, words...)
			}),
		)
	}
}

// CrewTileInset is the space a tile keeps inside its plane
func CrewTileInset(th *look.Theme, style CrewStyle) unit.Dp {
	if style.mini {
		return th.Sp.S2
	}
	return th.Sp.S3
}

type placedTile struct {
	call op.CallOp
	size image.Point
}

// CrewChart draws the lead's tile centred on top, a guide line down from it
// and across, the members in rows under it; live is nil where the crew is
// only drafted, as no member of it works yet
func CrewChart(gtx layout.Context, th *look.Theme, snapshot state.Snapshot, members []protocol.Member, live *CrewLive, style CrewStyle) layout.Dimensions {
	lead, leadIndex := Lead(members)
	if leadIndex < 0 {
		return layout.Dimensions{}
	}
	width := gtx.Constraints.Max.X
	gapX, gapY := gtx.Dp(style.gapX), gtx.Dp(style.gapY)
	tileWidth := min(width, gtx.Dp(style.tile))
	if style.mini {
		tileWidth = max(0, (width-gapX)/2)
	}
	place := func(tile layout.Widget) placedTile {
		child := gtx
		child.Constraints = layout.Constraints{Min: image.Pt(tileWidth, 0), Max: image.Pt(tileWidth, unbounded)}
		macro := op.Record(gtx.Ops)
		dimensions := tile(child)
		return placedTile{call: macro.Stop(), size: dimensions.Size}
	}
	record := func(member protocol.Member) placedTile {
		return place(func(gtx layout.Context) layout.Dimensions {
			return layoutCrewTile(gtx, th, snapshot, member, live, style)
		})
	}
	var head placedTile
	leadY := 0
	if style.Head != nil {
		head = place(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(CrewTileInset(th, style)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return style.Head(gtx)
			})
		})
		leadY = head.size.Y + gapY
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

	line := max(1, gtx.Dp(unit.Dp(tokens.SizeHairline)))
	trunk := width / 2
	y := leadY + top.size.Y
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
	// who answers to whom is a guide line, as the tree's rails are; it goes under the tiles so a tile's edge is never crossed
	guide := th.R.Guide()
	if style.Head != nil {
		look.FillRect(gtx, image.Rect(trunk, head.size.Y, trunk+line, leadY), guide)
	}
	if len(placed) > 0 {
		last := placed[len(placed)-1]
		look.FillRect(gtx, image.Rect(trunk, leadY+top.size.Y, trunk+line, last.y-gapY/2), guide)
		for _, row := range placed {
			bar := row.y - gapY/2
			left, right := min(trunk, row.xs[0]+tileWidth/2), max(trunk, row.xs[len(row.xs)-1]+tileWidth/2)
			look.FillRect(gtx, image.Rect(left, bar, right+line, bar+line), guide)
			for _, x := range row.xs {
				look.FillRect(gtx, image.Rect(x+tileWidth/2, bar, x+tileWidth/2+line, row.y), guide)
			}
		}
	}
	// a tile is as tall as the tallest of its row, so a row reads as one band
	draw := func(tile placedTile, at image.Point, height int) {
		stack := op.Offset(at).Push(gtx.Ops)
		bounds := image.Rect(0, 0, tile.size.X, height)
		if style.mini {
			kit.SlabOutline(gtx, th.Theme, bounds, gtx.Dp(th.Cut.Small))
		} else {
			kit.SlabSurface(gtx, th.Theme, bounds, gtx.Dp(th.Cut.Mid), th.R.Slab)
		}
		tile.call.Add(gtx.Ops)
		stack.Pop()
	}
	if style.Head != nil {
		draw(head, image.Pt((width-tileWidth)/2, 0), head.size.Y)
	}
	draw(top, image.Pt((width-tileWidth)/2, leadY), top.size.Y)
	for index, row := range rows {
		for column, tile := range row {
			draw(tile, image.Pt(placed[index].xs[column], placed[index].y), placed[index].height)
		}
	}
	return layout.Dimensions{Size: image.Pt(width, y)}
}

// who it is, what it runs on, and what it does while it works; a member at rest says nothing more
func layoutCrewTile(gtx layout.Context, th *look.Theme, snapshot state.Snapshot, member protocol.Member, live *CrewLive, style CrewStyle) layout.Dimensions {
	identity := CrewIdentity(th, style, member.Name, member.Title, member.Lead)
	return layout.UniformInset(CrewTileInset(th, style)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		if style.mini {
			return identity(gtx)
		}
		engine := MemberEngine(snapshot, member)
		line, ink := engine.Line(), th.R.Fg2
		if engine.Missing {
			line, ink = i18n.Tf("%s cannot run now", engine.Short()), th.R.Stone(palette.Failed)
		}
		children := []layout.FlexChild{
			layout.Rigid(identity),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			// the model wraps rather than cuts
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: th.T.Detail, Color: ink, MaxLines: 2}.Layout(gtx, th.Theme, line)
			}),
		}
		if live != nil {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return live.layout(gtx, th, snapshot, member)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// CrewLive is the presence of each member at work on the crew chart shown,
// kept so a caption cross-fades as it changes and a tile unfolds as its
// member takes a task
type CrewLive struct {
	team    string
	members map[string]*crewMember
}

type crewMember struct {
	presence Presence
	fold     kit.Disclosure
	// what it showed last, drawn while the tile folds after the task ended
	last   Live
	screen bool
}

// Of is the chart's presence for a team, begun anew when the team shown changes
func (live *CrewLive) Of(team string) *CrewLive {
	if live.team != team {
		*live = CrewLive{team: team}
	}
	return live
}

func (live *CrewLive) member(id string) *crewMember {
	if live.members == nil {
		live.members = make(map[string]*crewMember)
	}
	member := live.members[id]
	if member == nil {
		member = new(crewMember)
		live.members[id] = member
	}
	return member
}

// what a member does now unfolds under its model and folds away when the task leaves its hands
func (live *CrewLive) layout(gtx layout.Context, th *look.Theme, snapshot state.Snapshot, member protocol.Member) layout.Dimensions {
	shown := live.member(member.ID)
	task, working := MemberTask(snapshot, live.team, member)
	now, inHand := TaskLive(snapshot, task)
	shown.fold.Open = working && inHand
	if shown.fold.Open {
		// the name stands at the top of the tile, and what it is on reads while it says nothing yet
		now.Member = ""
		if now.Caption == "" {
			now.Caption = task.Title
		}
		_, screen := snapshot.Screens[task.Session]
		shown.last, shown.screen = now, screen && task.Session != ""
	}
	return shown.fold.Layout(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			if !shown.screen {
				return shown.presence.Layout(gtx, th, shown.last)
			}
			// a member working on a screen of its own says so, as the task page shows it
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return shown.presence.Layout(gtx, th, shown.last) }),
				layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(look.IconWidget(look.Icon.Screen, crewScreenIcon, th.R.Fg2)),
						layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
						layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg2, i18n.T("Screen"))),
					)
				}),
			)
		})
	})
}

// the screen glyph beside a caption line
const crewScreenIcon unit.Dp = 14
