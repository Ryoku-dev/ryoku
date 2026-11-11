// Package work is the Work destination: every team task on one board, a task's
// page with its report and how it went, and the dialog that gives a team a
// task.
package work

import (
	"gioui.org/layout"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
)

type Page struct {
	// the task the place opened, "" for the board
	selected string

	list look.SideList
	rise look.Rise

	board board
	task  taskView

	ask    askDialog
	retry  retryDialog
	remove removeDialog
}

func New() *Page { return &Page{} }

func (p *Page) Select(e *env.Env, place env.TeamsPlace) {
	if place.Task == p.selected {
		return
	}
	p.selected = place.Task
	p.task.open(place.Task)
}

// Ask opens the dialog that gives a team a task, with team picked when it is not empty.
func (p *Page) Ask(e *env.Env, team string) { p.ask.open(team) }

func (p *Page) Content(gtx layout.Context, e *env.Env) layout.Dimensions {
	task, known := e.Snapshot().TeamTasks[p.selected]
	// a task not loaded yet, or one just deleted, leaves the board in its place
	key := ""
	if known {
		key = task.ID
	}
	return p.rise.Layout(gtx, e.Theme, key, 8, func(gtx layout.Context) layout.Dimensions {
		if !known {
			return p.board.layout(gtx, e, p)
		}
		return p.task.layout(gtx, e, p, task)
	})
}

// the board's team filter is the team a new task most likely goes to
func (p *Page) New(e *env.Env) { p.Ask(e, p.board.tasks.Team) }

func (p *Page) NewLabel() string { return i18n.T("New task") }

func (p *Page) Overlay(gtx layout.Context, e *env.Env) bool {
	p.ask.layout(gtx, e)
	p.retry.layout(gtx, e)
	p.remove.layout(gtx, e, p)
	return look.AnyOpen(&p.ask.dialog, &p.retry.dialog, &p.remove.confirm.Dialog)
}
