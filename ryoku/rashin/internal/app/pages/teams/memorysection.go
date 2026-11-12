package teams

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/memoryui"
	"ryoku-rashin/internal/app/pages/teamsui"
	appstate "ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/memory/render"
	"ryoku-rashin/internal/protocol"
)

// a team keeps far fewer, and the brief has to see them all
const teamMemoryRead = 500

// managing them belongs to the Memory page, so the Overview only shows the newest
const teamMemoryRows = 3

// one place changes memories, so the Overview reads and links rather than edits
type memorySection struct {
	facts teamMemories
	card  memoryCard

	preview, open, close kit.Plaque
	report               kit.Click
	sheet                memoryui.Sheet
	sheetScroll          look.Scroller
}

type teamMemories struct {
	load     load
	team     string
	memories []protocol.Memory
	total    int
}

func (m *teamMemories) want(e *env.Env, team string) {
	m.load.want(e, fmt.Sprintf("%s/%d", team, e.Snapshot().MemoryRevision), func(done func()) {
		params := protocol.MemoryListParams{Scope: teamScope(team), Sort: protocol.MemorySortNewest, Limit: teamMemoryRead}
		e.ListMemory(params, func(result protocol.MemoryListResult, err error) {
			done()
			if err != nil {
				e.Report(err)
				return
			}
			m.team, m.memories, m.total = team, result.Memories, result.Total
		})
	})
}

// a team switch must never flash the other team's memories
func (m *teamMemories) of(team string) ([]protocol.Memory, int, bool) {
	if m.team != team {
		return nil, 0, false
	}
	return m.memories, m.total, true
}

// a team that may not know the user never asks for the card
type memoryCard struct {
	load load
	text string
}

func (c *memoryCard) want(e *env.Env) {
	c.load.want(e, fmt.Sprint(e.Snapshot().MemoryRevision), func(done func()) {
		e.MemoryCard(false, func(card protocol.MemoryCard, err error) {
			done()
			if err != nil {
				e.Report(err)
				return
			}
			c.text = card.Text
		})
	})
}

func teamScope(team string) string { return protocol.MemoryTeamPrefix + team }

func (m *memorySection) layout(gtx layout.Context, e *env.Env, team protocol.Team) layout.Dimensions {
	th := e.Theme
	m.facts.want(e, team.ID)
	if team.TeamMemory().ReadsYou {
		m.card.want(e)
	}
	all, total, loaded := m.facts.of(team.ID)
	m.open.Kind, m.open.Small, m.open.Icon, m.open.Label = kit.PlaqueTonal, true, look.Icon.Memory, i18n.T("Open memory")
	if m.open.Clicked(gtx) {
		e.OpenMemory(env.MemoryPlace{Filter: env.MemoryTeams, Scope: teamScope(team.ID)})
	}
	return kit.Section(th.Theme, i18n.T("Memory"), func(gtx layout.Context) layout.Dimensions {
		return look.HoldUntil(gtx, loaded, func(gtx layout.Context) layout.Dimensions {
			claim, proof := memoryLead(team, all, total)
			children := []layout.FlexChild{
				layout.Rigid(look.Paragraph(th, th.T.Proof, th.R.Fg, strings.TrimSpace(claim+" "+proof))),
				layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.layoutCarried(gtx, e, team, all) }),
			}
			for _, memory := range all[:min(len(all), teamMemoryRows)] {
				children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return memoryui.FactText(gtx, th, th.R.Fg, memory.Text) }),
						layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg3, memoryui.Proof(gtx.Now, memory))),
					)
				}))
			}
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return look.Flow{Gap: 8, LineGap: 8}.Layout(gtx, 2, func(gtx layout.Context, index int) layout.Dimensions {
						if index == 0 {
							return m.preview.Layout(gtx, th.Theme)
						}
						return m.open.Layout(gtx, th.Theme)
					})
				}),
			)
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})(gtx)
}

// a team with memory off says so instead of a count that means nothing
func memoryLead(team protocol.Team, all []protocol.Memory, total int) (string, string) {
	if !team.TeamMemory().Remember {
		return i18n.Tf("%s remembers nothing between runs.", team.Name), ""
	}
	if total == 0 {
		return i18n.Tf("%s remembers nothing yet.", team.Name), i18n.T("What its members learn in a run is kept for the next one.")
	}
	claim := i18n.Tf("%s remembers %s things.", team.Name, look.Count(total))
	if total == 1 {
		claim = i18n.Tf("%s remembers one thing.", team.Name)
	}
	var last *protocol.Memory
	for index := range all {
		if all[index].Source.Task != "" && (last == nil || all[index].Created.After(last.Created)) {
			last = &all[index]
		}
	}
	if last == nil {
		return claim, i18n.T("None of it came from a run yet.")
	}
	return claim, i18n.Tf("It last learned from the run on %s.", look.DayName(last.Created.Local().Format("2006-01-02")))
}

// the link opens the task, so the report the next run gets can be read in full
func (m *memorySection) layoutCarried(gtx layout.Context, e *env.Env, team protocol.Team, all []protocol.Memory) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	settings := team.TeamMemory()
	m.preview.Kind, m.preview.Small, m.preview.Label = kit.PlaqueTonal, true, i18n.T("Preview the brief")
	if m.preview.Clicked(gtx) {
		m.sheet.Open(e)
	}
	report, reported := teamsui.NewestReport(snapshot, team.ID)
	if m.report.Clicked(gtx) && reported {
		teamsui.OpenTask(e, report.ID)
	}
	lastReport := func(gtx layout.Context) layout.Dimensions {
		if !reported {
			return kit.Text(th.Theme, th.T.Detail, th.R.Fg2, i18n.T("No run has reported yet."))(gtx)
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.Link(gtx, th, &m.report, reportName(report)) }),
			layout.Rigid(kit.Text(th.Theme, th.T.Detail, th.R.Fg2, i18n.Tf(", %s", look.DayName(report.Report.At.Local().Format("2006-01-02"))))),
		)
	}
	brief, kept := teamBrief(team, all, m.card.text)
	inBrief := i18n.Tf("%d, about %s characters", kept, roughly(len([]rune(brief))))
	switch {
	case !settings.Remember:
		inBrief = i18n.T("None, Remember between runs is off.")
	case kept == 0:
		inBrief = i18n.T("None yet.")
	}
	fact := func(name string, value layout.Widget) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, name)),
					layout.Rigid(value),
				)
			})
		})
	}
	children := []layout.FlexChild{
		fact(i18n.T("Last report"), lastReport),
		fact(i18n.T("Memories in its brief"), kit.Text(th.Theme, th.T.Detail, th.R.Fg, inBrief)),
	}
	if !settings.CarryReport {
		children = append(children, layout.Rigid(quiet(th, i18n.T("Not carried, Carry the last report is off."))))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func reportName(task protocol.TeamTask) string {
	if task.Report.Title != "" {
		return task.Report.Title
	}
	return task.Title
}

// a size the brief only roughly keeps reads better rounded
func roughly(count int) string {
	if count >= 100 {
		count = (count + 50) / 100 * 100
	}
	digits := strconv.Itoa(count)
	for at := len(digits) - 3; at > 0; at -= 3 {
		digits = digits[:at] + "," + digits[at:]
	}
	return digits
}

// built with the daemon's own functions so the preview never drifts from what a run gets
func teamBrief(team protocol.Team, all []protocol.Memory, card string) (string, int) {
	settings := team.TeamMemory()
	if !settings.Remember {
		return "", 0
	}
	facts := slices.Clone(all)
	render.OrderForBrief(facts)
	return render.TeamBrief(facts, card, settings.ReadsYou)
}

// quiet runs said nothing new, so the daemon carries the last spoken report and so does the preview
func carriedReport(snapshot appstate.Snapshot, team protocol.Team) (protocol.Routine, string) {
	if !team.TeamMemory().CarryReport {
		return protocol.Routine{}, ""
	}
	runs := teamsui.Upcoming(snapshot, team.ID)
	if len(runs) == 0 {
		return protocol.Routine{}, ""
	}
	routine := runs[0]
	var newest *protocol.TeamReport
	for _, task := range snapshot.TeamTasks {
		report := task.Report
		if task.Routine != routine.ID || task.Status != protocol.TaskDone || report == nil || report.Quiet || strings.TrimSpace(report.Body) == "" {
			continue
		}
		if newest == nil || report.At.After(newest.At) {
			newest = report
		}
	}
	if newest == nil {
		return routine, ""
	}
	return routine, render.CarriedReport(newest.Title, newest.At, newest.Body)
}

func (m *memorySection) layoutSheet(gtx layout.Context, e *env.Env, team protocol.Team) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	all, _, _ := m.facts.of(team.ID)
	brief, _ := teamBrief(team, all, m.card.text)
	routine, report := carriedReport(snapshot, team)
	text := strings.TrimSpace(strings.Join([]string{brief, report}, "\n\n"))
	if text == "" {
		text = i18n.T("Nothing yet: the next task starts with its brief alone.")
	}
	proof := i18n.T("Exactly what the next task is handed about what the team remembers.")
	if routine.ID != "" {
		proof = i18n.Tf("Exactly what the next run of %s is handed about what the team remembers.", routine.Title)
	}
	m.close.Kind, m.close.Small, m.close.Label = kit.PlaqueQuiet, true, i18n.T("Close")
	if m.close.Clicked(gtx) {
		m.sheet.Close(e)
	}
	return m.sheetScroll.Layout(gtx, th, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return layout.UniformInset(th.Sp.S5).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return memoryui.SheetBody(gtx, th, i18n.T("What the next run gets"), proof, text,
				i18n.T("This is made from the list. To change it, change the memories."),
				func(gtx layout.Context) layout.Dimensions { return m.close.Layout(gtx, th.Theme) })
		})
	})
}
