package routines

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/protocol"
)

type removeDialog struct {
	routine protocol.Routine
	confirm kit.Confirm
}

func (d *removeDialog) open(routine protocol.Routine) {
	d.routine, d.confirm.Dialog.Open, d.confirm.Danger = routine, true, true
}

func (d *removeDialog) layout(gtx layout.Context, e *env.Env, page *Page) {
	if d.routine.ID == "" {
		return
	}
	body := i18n.Tf("“%s” stops running. The tasks it made stay.", d.routine.Title)
	if d.confirm.Layout(gtx, e.Theme.Theme, i18n.T("Delete this routine?"), body, i18n.T("Delete routine"), "") {
		id := d.routine.ID
		deleteRoutine(e, id, func(err error) {
			if err != nil {
				e.Notify(err.Error())
				return
			}
			if page.selected == id {
				openBoard(e)
			}
		})
	}
	if d.confirm.Closed() {
		d.routine = protocol.Routine{}
	}
}
