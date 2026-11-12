package teams

import (
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	appstate "ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

const (
	// from this width the month's two lines stand side by side, the tokens taking the larger share
	chartsWide  unit.Dp = 860
	chartsGap   unit.Dp = 34
	chartsShare         = 0.6
)

// every team as a tile under a sentence of how they stand, then the month across them all, or the templates to start the first one from
type board struct {
	header look.PaneHeader
	scroll look.Scroller
	tiles  map[string]*teamTile
	create slab
	start  teamsui.TeamStart
	// every team's month, read once the board opens and again when the snapshot says it moved
	month teamsui.MonthSpend
}

func (b *board) layout(gtx layout.Context, e *env.Env, p *Page) layout.Dimensions {
	snapshot := e.Snapshot()
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return b.layoutHeader(gtx, e, p) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return b.scroll.Layout(gtx, e.Theme, 1, func(gtx layout.Context, _ int) layout.Dimensions {
				return boardColumn(gtx, e, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return b.layoutLead(gtx, e, snapshot) }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if len(snapshot.Teams) == 0 {
								return b.start.Layout(gtx, e, p.templates.List(e))
							}
							return b.layoutTiles(gtx, e, p, snapshot)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if len(snapshot.Teams) == 0 {
								return layout.Dimensions{}
							}
							return b.layoutMonth(gtx, e)
						}),
					)
				})
			})
		}),
	)
}

func (b *board) layoutHeader(gtx layout.Context, e *env.Env, p *Page) layout.Dimensions {
	content := look.PaneHeaderContent{
		Title:    i18n.T("Teams"),
		Scrolled: b.scroll.Position.First > 0 || b.scroll.Position.Offset > 0,
		Actions: []look.HeaderAction{
			{Icon: look.Icon.Add, Label: i18n.T("New team")},
			{Icon: look.Icon.Import, Label: i18n.T("Import team"), Description: i18n.T("Make a team from a file another team was exported to")},
		},
	}
	dimensions := b.header.Layout(gtx, e.Theme, e.Overlays, content)
	if index, ok := b.header.ActionClicked(); ok {
		if index == 0 {
			p.New(e)
		} else {
			p.imports.choose(e)
		}
	}
	return dimensions
}

// the sentence that says how the teams stand, one line of proof under it as the Inbox leads, and what keeps Rashin up
func (b *board) layoutLead(gtx layout.Context, e *env.Env, snapshot appstate.Snapshot) layout.Dimensions {
	th := e.Theme
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return look.Monument(gtx, th, boardSentence(snapshot.Teams), boardProof(gtx.Now, snapshot))
		}),
	}
	// with no team yet there is nothing that could keep it up
	if len(snapshot.Teams) > 0 {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg3, upSentence(gtx.Now, snapshot))),
		)
	}
	// the column already stands S4 under the header, S3 more meets the Inbox's lead
	return layout.Inset{Top: th.Sp.S3, Bottom: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (b *board) tile(id string) *teamTile {
	if b.tiles == nil {
		b.tiles = make(map[string]*teamTile)
	}
	tile := b.tiles[id]
	if tile == nil {
		tile = new(teamTile)
		b.tiles[id] = tile
	}
	return tile
}

// tiles of removed teams go with them
func (b *board) prune(teams []protocol.Team) {
	if len(b.tiles) <= len(teams) {
		return
	}
	kept := make(map[string]*teamTile, len(teams))
	for _, team := range teams {
		if tile := b.tiles[team.ID]; tile != nil {
			kept[team.ID] = tile
		}
	}
	b.tiles = kept
}

// the teams' tiles, then the quiet one that starts another
func (b *board) layoutTiles(gtx layout.Context, e *env.Env, p *Page, snapshot appstate.Snapshot) layout.Dimensions {
	teams := snapshot.Teams
	b.prune(teams)
	// clicks are taken before the grid measures its tiles, so measuring never eats one
	for _, team := range teams {
		tile := b.tile(team.ID)
		// each tile's line is its own team's month, kept with the tile for the page's life
		tile.month.Team = team.ID
		tile.month.Want(e, gtx.Now)
		tile.give.Kind, tile.give.Small, tile.give.Label = kit.PlaqueTonal, true, i18n.T("Give a task")
		// the tile under the button hears the same click, both are drained so it never opens the team a frame later
		give, open := tile.give.Clicked(gtx), tile.click.Clicked(gtx)
		switch {
		case give:
			e.NewTeamTask(team.ID)
		case open:
			openTeam(e, team.ID, env.TeamOverview)
		}
	}
	if b.create.click.Clicked(gtx) {
		p.New(e)
	}
	working := teamsui.Working(snapshot)
	return tileGrid(gtx, len(teams)+1, func(gtx layout.Context, index int) layout.Dimensions {
		if index == len(teams) {
			return b.layoutCreate(gtx, e.Theme)
		}
		return b.layoutTile(gtx, e, snapshot, working, teams[index])
	})
}

// the month across every team: tokens a day beside the tasks that ended each day, stacked on a narrow pane
func (b *board) layoutMonth(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	b.month.Want(e, gtx.Now)
	spend := b.month.Spend()
	tokens := teamsui.TokensTrend(gtx.Now, spend, i18n.T("This month"))
	finished := teamsui.FinishedTrend(gtx.Now, spend, i18n.T("Finished"))
	loaded := b.month.Loaded()
	left := func(gtx layout.Context) layout.Dimensions {
		return look.HoldUntil(gtx, loaded, func(gtx layout.Context) layout.Dimensions { return tokens.Layout(gtx, th) })
	}
	right := func(gtx layout.Context) layout.Dimensions {
		return look.HoldUntil(gtx, loaded, func(gtx layout.Context) layout.Dimensions { return finished.Layout(gtx, th) })
	}
	return layout.Inset{Top: th.Sp.S6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		if gtx.Constraints.Max.X < gtx.Dp(chartsWide) {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, layout.Rigid(left), layout.Rigid(right))
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
			layout.Flexed(chartsShare, left),
			layout.Rigid(layout.Spacer{Width: chartsGap}.Layout),
			layout.Flexed(1-chartsShare, right),
		)
	})
}

// how many teams there are and how many work or need the user, each count said whole so it reads right
func boardSentence(teams []protocol.Team) string {
	count := len(teams)
	if count == 0 {
		return i18n.T("No team yet.")
	}
	working, needs, paused := 0, 0, 0
	for _, team := range teams {
		_, tone, busy := teamsui.TeamStanding(team)
		switch {
		case busy:
			working++
		case tone == palette.Attention:
			needs++
		case tone == palette.Paused:
			paused++
		}
	}
	if count == 1 {
		name := teams[0].Name
		switch {
		case working == 1:
			return i18n.Tf("%s is working.", name)
		case needs == 1:
			return i18n.Tf("%s needs you.", name)
		case paused == 1:
			return i18n.Tf("%s is paused.", name)
		}
		return i18n.Tf("%s is quiet.", name)
	}
	teamCount := look.CountStart(count)
	switch {
	case working == 1 && needs == 1:
		return i18n.Tf("%s teams. One is working and one needs you.", teamCount)
	case working == 1 && needs > 1:
		return i18n.Tf("%s teams. One is working and %s need you.", teamCount, look.Count(needs))
	case working > 1 && needs == 1:
		return i18n.Tf("%s teams. %s are working and one needs you.", teamCount, look.CountStart(working))
	case working > 1 && needs > 1:
		return i18n.Tf("%s teams. %s are working and %s need you.", teamCount, look.CountStart(working), look.Count(needs))
	case working == 1:
		return i18n.Tf("%s teams. One is working.", teamCount)
	case working > 1:
		return i18n.Tf("%s teams. %s are working.", teamCount, look.CountStart(working))
	case needs == 1:
		return i18n.Tf("%s teams. One needs you.", teamCount)
	case needs > 1:
		return i18n.Tf("%s teams. %s need you.", teamCount, look.CountStart(needs))
	case paused == count:
		return i18n.Tf("%s teams, all paused.", teamCount)
	}
	return i18n.Tf("%s teams, all quiet.", teamCount)
}

// the newest report, then the week's tasks and the month's tokens across every team
func boardProof(now time.Time, snapshot appstate.Snapshot) string {
	if len(snapshot.Teams) == 0 {
		return i18n.T("A team is a lead and its members, with one purpose, working in a folder of their own while you do something else.")
	}
	tasks := 0
	var tokens int64
	for _, team := range snapshot.Teams {
		tasks += tasksThisWeek(snapshot, team.ID, now)
		tokens += team.Stats.MonthTokens
	}
	newest, found := teamsui.NewestReport(snapshot, "")
	if !found && tasks == 0 && tokens == 0 {
		return i18n.T("Give a team a task and its report lands in the Inbox.")
	}
	var sentences []string
	switch {
	case found && newest.Report.Quiet:
		sentences = append(sentences, i18n.Tf("%s found nothing new %s.", teamsui.TeamName(snapshot.Teams, newest.Team), look.Ago(now, newest.Report.At)))
	case found:
		title := newest.Report.Title
		if title == "" {
			title = newest.Title
		}
		sentences = append(sentences, i18n.Tf("%s sent “%s” %s.", teamsui.TeamName(snapshot.Teams, newest.Team), title, look.Ago(now, newest.Report.At)))
	}
	sentences = append(sentences, i18n.Tf("%s this week, %s tokens this month.", tasksWord(tasks), teamsui.Tokens(tokens)))
	return strings.Join(sentences, " ")
}

// what keeps Rashin running once its window closes: the routines that are on, the local API, or nothing
func upSentence(now time.Time, snapshot appstate.Snapshot) string {
	on := 0
	var next time.Time
	// a pause of every team holds every routine, whatever each says
	if !snapshot.Settings.Teams.Paused {
		for _, routine := range snapshot.Routines {
			team, ok := teamsui.TeamByID(snapshot.Teams, routine.Team)
			if !routine.Enabled || !ok || team.Paused {
				continue
			}
			on++
			if routine.NextRun != nil && (next.IsZero() || routine.NextRun.Before(next)) {
				next = *routine.NextRun
			}
		}
	}
	api := snapshot.Settings.API.Enabled
	var sentences []string
	switch {
	case on == 1:
		sentences = append(sentences, i18n.T("Rashin stays up while one routine is on."))
	case on > 1:
		sentences = append(sentences, i18n.Tf("Rashin stays up while %d routines are on.", on))
	case api:
		return i18n.T("The local API keeps Rashin up while it is on.")
	default:
		return i18n.T("Nothing keeps Rashin up; it stops when you close the window.")
	}
	if !next.IsZero() {
		sentences = append(sentences, i18n.Tf("Next run %s.", look.When(now, next)))
	}
	if api {
		sentences = append(sentences, i18n.T("The local API keeps it up too."))
	}
	return strings.Join(sentences, " ")
}
