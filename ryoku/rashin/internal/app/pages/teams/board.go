package teams

import (
	"strings"
	"time"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	appstate "ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// every team as a tile under a sentence of how they stand, or the templates to start the first one from
type board struct {
	header look.PaneHeader
	scroll look.Scroller
	tiles  map[string]*teamTile
	create slab
	start  teamsui.TeamStart
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

// the sentence that says how the teams stand and one line of proof under it, as the Inbox leads
func (b *board) layoutLead(gtx layout.Context, e *env.Env, snapshot appstate.Snapshot) layout.Dimensions {
	th := e.Theme
	// the column already stands S4 under the header, S3 more meets the Inbox's lead
	return layout.Inset{Top: th.Sp.S3, Bottom: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(look.Paragraph(th, th.T.Sentence, th.R.Fg, boardSentence(snapshot.Teams))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Proof, th.R.Fg2, boardProof(gtx.Now, snapshot))),
		)
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
		tile.give.Kind, tile.give.Small, tile.give.Label = kit.PlaqueQuiet, true, i18n.T("Give a task")
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
	if found {
		title := newest.Report.Title
		if title == "" {
			title = newest.Title
		}
		sentences = append(sentences, i18n.Tf("%s sent “%s” %s.", teamsui.TeamName(snapshot.Teams, newest.Team), title, look.Ago(now, newest.Report.At)))
	}
	sentences = append(sentences, i18n.Tf("%s this week, %s tokens this month.", tasksWord(tasks), teamsui.Tokens(tokens)))
	return strings.Join(sentences, " ")
}
