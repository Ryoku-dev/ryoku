package settings

import (
	"io"
	"strings"
	"time"

	"gioui.org/io/clipboard"
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// A rewrite review retains only this turn's disclosure/result. It never
// captures a selection merely because Settings is opened.
type oniRewriteReview struct {
	result                            protocol.OniRewriteResult
	deadline                          time.Time
	busy                              bool
	problem                           string
	review, apply, cancel, undo, copy kit.Plaque
}

func (r *oniRewriteReview) call(e *env.Env, action string) {
	if r.busy {
		return
	}
	params := protocol.OniRewriteParams{Action: action}
	if action != protocol.OniRewriteStart {
		params.ID = r.result.ID
	}
	r.busy, r.problem = true, ""
	result := new(protocol.OniRewriteResult)
	e.Call(protocol.MOniRewrite, params, result, func(err error) {
		r.busy = false
		if err != nil {
			r.problem = err.Error()
			return
		}
		r.result = *result
		r.deadline = time.Time{}
		if result.ExpiresMs > 0 {
			r.deadline = time.Now().Add(time.Duration(result.ExpiresMs) * time.Millisecond)
		}
	})
}

func (r *oniRewriteReview) rows(e *env.Env) []look.Setting {
	th := e.Theme
	button := func(plaque *kit.Plaque, title, action string, allowed bool) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			plaque.Kind, plaque.Small, plaque.Label = kit.PlaqueTonal, true, title
			if plaque.Clicked(gtx) && allowed && !r.busy {
				r.call(e, action)
			}
			if !allowed || r.busy {
				gtx = gtx.Disabled()
			}
			return look.Named(gtx, title, func(gtx layout.Context) layout.Dimensions { return plaque.Layout(gtx, th.Theme) })
		}
	}
	current := r.result
	live := r.deadline.IsZero() || time.Now().Before(r.deadline)
	rows := []look.Setting{{Title: i18n.T("Review a rewrite"), Detail: i18n.T("After holding the talk chord to request a rewrite, open its exact text and destination here. Reviewing does not send or type it."),
		Control: button(&r.review, i18n.T("Review rewrite"), protocol.OniRewriteStart, current.Status != protocol.OniRewriteConsent)}}
	if current.Status == protocol.OniRewriteConsent {
		rows = append(rows,
			look.Setting{Title: i18n.T("Text to rewrite"), Detail: current.Text},
			look.Setting{Title: i18n.T("Rewrite destination"), Detail: i18n.Tf("%s · %s · %d characters", current.Destination, current.Route, current.Chars)},
			look.Setting{Title: i18n.T("Apply the reviewed rewrite"), Detail: i18n.T("Only this approval sends the text. Return to the original window after pressing Apply; Oni waits briefly for it and rechecks its selection. A changed window or selection is refused."),
				Control: button(&r.apply, i18n.T("Apply reviewed rewrite"), protocol.OniRewriteApply, live)},
			look.Setting{Title: i18n.T("Discard this rewrite"), Control: button(&r.cancel, i18n.T("Cancel reviewed rewrite"), protocol.OniRewriteCancel, true)},
		)
	}
	if current.Status == protocol.OniRewriteApplied {
		rows = append(rows, look.Setting{Title: i18n.T("Restore the original"), Detail: i18n.T("Return to the original window after pressing Undo. If the replacement can no longer be verified, Oni offers the original to copy instead of typing it."),
			Control: button(&r.undo, i18n.T("Undo this rewrite"), protocol.OniRewriteUndo, live)})
	}
	if current.Status != "" && current.Status != protocol.OniRewriteConsent {
		rows = append(rows, look.Setting{Title: i18n.T("Rewrite result"), Detail: current.Status + " " + current.Refused})
	}
	if !live {
		rows = append(rows, look.Setting{Title: i18n.T("Rewrite expired"), Detail: i18n.T("Request a new rewrite. Nothing is applied after its review expires.")})
	}
	if current.Status == protocol.OniRewriteCopy || current.Status == protocol.OniRewriteKept {
		rows = append(rows, look.Setting{Title: i18n.T("Rewrite text"), Detail: current.Text, Control: func(gtx layout.Context) layout.Dimensions {
			r.copy.Kind, r.copy.Small, r.copy.Label = kit.PlaqueTonal, true, i18n.T("Copy rewrite text")
			if r.copy.Clicked(gtx) {
				gtx.Execute(clipboard.WriteCmd{Type: "text/plain", Data: io.NopCloser(strings.NewReader(current.Text))})
			}
			return r.copy.Layout(gtx, th.Theme)
		}})
	}
	if r.problem != "" {
		rows = append(rows, look.Setting{Title: i18n.T("Rewrite unavailable"), Detail: r.problem})
	}
	return rows
}
