package teams

import (
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/protocol"
)

// the team's tasks on the Work board, under a field that gives it one
type workTab struct {
	scroll look.Scroller
	brief  kit.TextField
	start  kit.Plaque
	board  teamsui.TaskBoard
	// the field was typed for this team, another team's page starts it empty
	team string
}

func (w *workTab) layout(gtx layout.Context, e *env.Env, team protocol.Team) layout.Dimensions {
	th := e.Theme
	if w.team != team.ID {
		w.team = team.ID
		w.brief.SetText("")
	}
	w.handle(gtx, e, team)
	w.board.Team, w.board.Empty = team.ID, i18n.T("The team has no task yet. Tell it what to do above.")
	rows, _ := w.board.Rows(gtx, e.Snapshot())
	return w.scroll.Layout(gtx, th, 1+rows, func(gtx layout.Context, index int) layout.Dimensions {
		return look.Centred(gtx, e.Class, look.BoardWidth, func(gtx layout.Context) layout.Dimensions {
			if index == 0 {
				return layout.Inset{Top: th.Sp.S4, Bottom: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return w.layoutField(gtx, th, team)
				})
			}
			return w.board.LayoutRow(gtx, e, index-1)
		})
	})
}

func (w *workTab) handle(gtx layout.Context, e *env.Env, team protocol.Team) {
	submitted := false
	for {
		event, ok := w.brief.Editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := event.(widget.SubmitEvent); ok {
			submitted = true
		}
	}
	if w.start.Clicked(gtx) {
		submitted = true
	}
	brief := strings.TrimSpace(w.brief.Text())
	if !submitted || brief == "" || w.start.Busy {
		return
	}
	w.start.Busy = true
	var task protocol.TeamTask
	e.Call(protocol.MTeamsTaskCreate, protocol.TeamTaskCreateParams{Team: team.ID, Brief: brief}, &task, func(err error) {
		w.start.Busy = false
		if err != nil {
			e.Notify(err.Error())
			return
		}
		w.brief.SetText("")
		e.Notify(i18n.Tf("%s has the task: %s", team.Name, task.Title))
	})
}

func (w *workTab) layoutField(gtx layout.Context, th *look.Theme, team protocol.Team) layout.Dimensions {
	w.brief.Editor.Submit = true
	w.brief.Placeholder = i18n.Tf("Tell %s what to do", team.Name)
	w.start.Kind, w.start.Label = kit.PlaqueGold, i18n.T("Start")
	empty := strings.TrimSpace(w.brief.Text()) == ""
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return look.PaneField(gtx, th, gtx.Focused(&w.brief.Editor), func(gtx layout.Context) layout.Dimensions {
				return w.brief.Layout(gtx, th.Theme)
			})
		}),
		layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if empty {
				gtx = gtx.Disabled()
			}
			return w.start.Layout(gtx, th.Theme)
		}),
	)
}
