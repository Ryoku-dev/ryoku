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
	// Head stands above the reporting roots, as a dream's CEO does.
	Head layout.Widget
	// Editor is present only for an explicit settings draft, never a live chart.
	Editor *CrewEditor
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

// crewBranch keeps the source member index and its actual manager index.
// Parent -1 reports to the Summoner (or the Dream CEO), not another member.
type crewBranch struct {
	Member, Parent, Depth int
}

// crewHierarchy orders a reporting forest without assigning authority to list
// order or rank. Empty reporting is the legacy lead-and-members star.
func crewHierarchy(team protocol.Team) []crewBranch {
	parents := make([]int, len(team.Members))
	byID := make(map[string]int, len(team.Members))
	for index, member := range team.Members {
		if member.ID != "" {
			byID[member.ID] = index
		}
		parents[index] = -1
	}
	_, lead := Lead(team)
	for index, member := range team.Members {
		switch team.Reporting {
		case protocol.ReportingPeers:
			// A synthesizer is a peer, not a reporting manager.
		case "":
			if lead >= 0 && index != lead {
				parents[index] = lead
			}
		default:
			if parent, ok := byID[member.ReportsTo]; ok && parent != index {
				parents[index] = parent
			}
		}
	}
	children := make([][]int, len(parents))
	for index, parent := range parents {
		if parent >= 0 {
			children[parent] = append(children[parent], index)
		}
	}
	branches := make([]crewBranch, 0, len(parents))
	seen := make([]bool, len(parents))
	var visit func(int, int, int)
	visit = func(index, parent, depth int) {
		if seen[index] {
			return
		}
		seen[index] = true
		branches = append(branches, crewBranch{Member: index, Parent: parent, Depth: depth})
		for _, child := range children[index] {
			visit(child, index, depth+1)
		}
	}
	if lead >= 0 && parents[lead] < 0 && team.Reporting != protocol.ReportingPeers {
		visit(lead, -1, 0)
	}
	for index, parent := range parents {
		if parent < 0 {
			visit(index, -1, 0)
		}
	}
	// An incomplete edit must not hide seats. Invalid/disconnected cycles have
	// no invented manager at their displayed root; validation owns rejection.
	for index := range parents {
		if !seen[index] {
			visit(index, -1, 0)
		}
	}
	return branches
}

// CrewChart uses the containing page's scroll area: trees are indented in
// reporting order, peers share equal grid rows under the Summoner.
func CrewChart(gtx layout.Context, th *look.Theme, snapshot state.Snapshot, team protocol.Team, live *CrewLive, style CrewStyle) layout.Dimensions {
	branches := crewHierarchy(team)
	if len(branches) == 0 {
		return layout.Dimensions{}
	}
	width := gtx.Constraints.Max.X
	gapX, gapY := gtx.Dp(style.gapX), gtx.Dp(style.gapY)
	tileWidth := min(width, gtx.Dp(style.tile))
	if style.mini {
		tileWidth = max(1, (width-gapX)/2)
	}
	place := func(tile layout.Widget) placedTile {
		child := gtx
		child.Constraints = layout.Constraints{Min: image.Pt(tileWidth, 0), Max: image.Pt(tileWidth, unbounded)}
		macro := op.Record(gtx.Ops)
		dimensions := tile(child)
		return placedTile{call: macro.Stop(), size: dimensions.Size}
	}
	headIdentity := style.Head
	if headIdentity == nil {
		headIdentity = CrewIdentity(th, style, i18n.T("You"), i18n.T("Summoner"), false)
	}
	head := place(func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(CrewTileInset(th, style)).Layout(gtx, headIdentity)
	})
	tiles := make([]placedTile, len(team.Members))
	positions := make([]image.Point, len(team.Members))
	heights := make([]int, len(team.Members))
	for index, member := range team.Members {
		tiles[index] = place(func(gtx layout.Context) layout.Dimensions {
			return layoutCrewTile(gtx, th, snapshot, member, live, style, reportingLine(team, member))
		})
	}
	y := head.size.Y + gapY
	headX := (width - tileWidth) / 2
	peers := team.Reporting == protocol.ReportingPeers
	if peers {
		perRow := max(1, (width+gapX)/max(1, tileWidth+gapX))
		for start := 0; start < len(branches); start += perRow {
			end := min(len(branches), start+perRow)
			height := 0
			for _, branch := range branches[start:end] {
				height = max(height, tiles[branch.Member].size.Y)
			}
			span := (end-start)*tileWidth + (end-start-1)*gapX
			for column, branch := range branches[start:end] {
				positions[branch.Member] = image.Pt((width-span)/2+column*(tileWidth+gapX), y)
				heights[branch.Member] = height
			}
			y += height + gapY
		}
	} else {
		maxDepth := 0
		for _, branch := range branches {
			maxDepth = max(maxDepth, branch.Depth)
		}
		// Keep the full tile readable even for a 100-seat chain; captions
		// preserve the exact manager when the available indent is shallow.
		indent := min(gtx.Dp(th.Sp.S5), max(0, width-tileWidth)/max(1, maxDepth))
		span := tileWidth + maxDepth*indent
		for _, branch := range branches {
			positions[branch.Member] = image.Pt((width-span)/2+branch.Depth*indent, y)
			heights[branch.Member] = tiles[branch.Member].size.Y
			y += heights[branch.Member] + gapY
		}
	}
	line := max(1, gtx.Dp(unit.Dp(tokens.SizeHairline)))
	guide := th.R.Guide()
	for _, branch := range branches {
		at := positions[branch.Member]
		if branch.Parent < 0 {
			// All roots report to the head. The outer rail never crosses a
			// peer's tile in a later grid row.
			rail := max(0, (width-tileWidth)/2-gapX/2)
			bar := head.size.Y + gapY/2
			look.FillRect(gtx, image.Rect(rail, bar, headX+tileWidth/2+line, bar+line), guide)
			look.FillRect(gtx, image.Rect(headX+tileWidth/2, head.size.Y, headX+tileWidth/2+line, bar), guide)
			if peers {
				rowBar := at.Y - gapY/2
				look.FillRect(gtx, image.Rect(0, bar, line, rowBar+line), guide)
				look.FillRect(gtx, image.Rect(0, bar, rail+line, bar+line), guide)
				look.FillRect(gtx, image.Rect(0, rowBar, at.X+tileWidth/2+line, rowBar+line), guide)
				look.FillRect(gtx, image.Rect(at.X+tileWidth/2, rowBar, at.X+tileWidth/2+line, at.Y), guide)
			} else {
				look.FillRect(gtx, image.Rect(rail, bar, rail+line, at.Y+tiles[branch.Member].size.Y/2), guide)
				look.FillRect(gtx, image.Rect(rail, at.Y+tiles[branch.Member].size.Y/2, at.X+line, at.Y+tiles[branch.Member].size.Y/2+line), guide)
			}
			continue
		}
		parent := positions[branch.Parent]
		rail := min(parent.X+gapX/2, at.X-line)
		from, to := parent.Y+heights[branch.Parent], at.Y+heights[branch.Member]/2
		look.FillRect(gtx, image.Rect(rail, from, rail+line, to+line), guide)
		look.FillRect(gtx, image.Rect(rail, to, at.X+line, to+line), guide)
	}
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
	draw(head, image.Pt(headX, 0), head.size.Y)
	for _, branch := range branches {
		draw(tiles[branch.Member], positions[branch.Member], heights[branch.Member])
	}
	if style.Editor != nil {
		style.Editor.layout(gtx, th, image.Pt(width, y-gapY), positions, tiles, image.Rect(headX, 0, headX+tileWidth, head.size.Y))
	}
	return layout.Dimensions{Size: image.Pt(width, y-gapY)}
}

func reportingLine(team protocol.Team, member protocol.Member) string {
	if team.Reporting == protocol.ReportingPeers {
		if member.Lead {
			return i18n.T("Synthesizer · reports to Summoner")
		}
		return i18n.T("Reports to Summoner")
	}
	if team.Dream != nil && team.Dream.Manager == member.ID {
		return i18n.T("Manager · reports to CEO")
	}
	if member.Lead {
		return i18n.T("Lead · reports to Summoner")
	}
	manager := member.ReportsTo
	if team.Reporting == "" {
		lead, _ := Lead(team)
		manager = lead.ID
	}
	for _, parent := range team.Members {
		if parent.ID == manager && manager != "" {
			return i18n.Tf("Reports to %s", parent.Name)
		}
	}
	return i18n.T("No reporting manager configured")
}

// who it is, what it runs on, and what it does while it works; a member at rest says nothing more
func layoutCrewTile(gtx layout.Context, th *look.Theme, snapshot state.Snapshot, member protocol.Member, live *CrewLive, style CrewStyle, reporting string) layout.Dimensions {
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
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: th.T.Caption, Color: th.R.Fg3, MaxLines: 2}.Layout(gtx, th.Theme, reporting)
			}),
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
