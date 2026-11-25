package teams

import (
	"fmt"

	"gioui.org/layout"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type memberConfigEditor struct {
	team, member                                       string
	generation                                         uint64
	asked, busy, historyBusy, previewBusy              bool
	current                                            protocol.MemberConfigDocument
	preview                                            protocol.MemberConfigDocument
	editor, historical                                 look.MultiLine
	history                                            []protocol.MemberConfigRevision
	retention                                          int
	problem                                            string
	save, reload, historyReload, restore, closePreview kit.Plaque
	previews                                           map[uint64]*kit.Plaque
	review                                             kit.Toggle
}

func (x *memberConfigEditor) reset(team, member string) {
	x.generation++
	x.team, x.member = team, member
	x.asked, x.busy, x.historyBusy, x.previewBusy = false, false, false, false
	x.current, x.preview = protocol.MemberConfigDocument{}, protocol.MemberConfigDocument{}
	x.editor.SetText("")
	x.historical.SetText("")
	x.history, x.problem, x.previews, x.review.Value = nil, "", nil, false
	x.retention = protocol.MemberConfigHistoryLimit
}
func (x *memberConfigEditor) active(e *env.Env, generation uint64, epoch int) bool {
	if x.generation != generation {
		return false
	}
	if e.Snapshot().Epoch != epoch {
		x.generation++
		x.busy, x.historyBusy, x.previewBusy = false, false, false
		x.problem = i18n.T("The daemon connection changed. Your edits are kept; reload current YAML deliberately before retrying.")
		return false
	}
	return true
}
func (x *memberConfigEditor) get(e *env.Env, discard bool) {
	if x.busy {
		return
	}
	x.asked, x.busy, x.problem = true, true, ""
	generation, epoch, initial := x.generation, e.Snapshot().Epoch, x.editor.Text()
	var doc protocol.MemberConfigDocument
	e.Call(protocol.MTeamsMemberConfigGet, protocol.MemberConfigRef{Team: x.team, Member: x.member}, &doc, func(err error) {
		if !x.active(e, generation, epoch) {
			return
		}
		x.busy = false
		if err != nil {
			x.problem = err.Error()
			return
		}
		if x.editor.Text() != initial || !discard && x.current.Revision != 0 && initial != x.current.YAML {
			x.problem = i18n.T("A newer current configuration is available. Your YAML edits are kept; copy them before deliberately reloading.")
			return
		}
		x.current = doc
		x.editor.SetText(doc.YAML)
		x.review.Value = false
	})
}
func (x *memberConfigEditor) revisions(e *env.Env) {
	if x.historyBusy {
		return
	}
	x.historyBusy = true
	generation, epoch := x.generation, e.Snapshot().Epoch
	var result protocol.MemberConfigHistoryResult
	e.Call(protocol.MTeamsMemberConfigHistory, protocol.MemberConfigRef{Team: x.team, Member: x.member}, &result, func(err error) {
		if !x.active(e, generation, epoch) {
			return
		}
		x.historyBusy = false
		if err != nil {
			x.problem = err.Error()
			return
		}
		x.history = result.Revisions
		x.retention = result.RetentionLimit
	})
}
func (x *memberConfigEditor) show(e *env.Env, revision uint64) {
	if x.previewBusy {
		return
	}
	x.previewBusy, x.review.Value = true, false
	generation, epoch := x.generation, e.Snapshot().Epoch
	var doc protocol.MemberConfigDocument
	e.Call(protocol.MTeamsMemberConfigGet, protocol.MemberConfigRef{Team: x.team, Member: x.member, Revision: revision}, &doc, func(err error) {
		if !x.active(e, generation, epoch) {
			return
		}
		x.previewBusy = false
		if err != nil {
			x.problem = err.Error()
			return
		}
		x.preview = doc
		x.historical.SetText(doc.YAML)
		x.review.Value = false
	})
}
func (x *memberConfigEditor) commit(e *env.Env, restore bool) {
	if x.busy || x.current.Revision == 0 || restore && (x.previewBusy || !x.review.Value || x.preview.Revision == 0) {
		return
	}
	x.busy, x.problem = true, ""
	generation, epoch, submitted := x.generation, e.Snapshot().Epoch, x.editor.Text()
	var result protocol.MemberConfigDocument
	done := func(err error) {
		if !x.active(e, generation, epoch) {
			return
		}
		x.busy = false
		if err != nil {
			x.problem = i18n.Tf("Configuration was not changed: %s. Your YAML and form edits are kept. Reload current YAML deliberately to resolve a stale revision.", err.Error())
			return
		}
		x.current = result
		if x.editor.Text() == submitted {
			x.editor.SetText(result.YAML)
		}
		x.preview, x.review.Value = protocol.MemberConfigDocument{}, false
		x.revisions(e)
	}
	if restore {
		e.Call(protocol.MTeamsMemberConfigRestore, protocol.MemberConfigRestoreParams{Team: x.team, Member: x.member, Revision: x.preview.Revision, ExpectedRevision: x.current.Revision}, &result, done)
	} else {
		e.Call(protocol.MTeamsMemberConfigSave, protocol.MemberConfigSaveParams{Team: x.team, Member: x.member, YAML: submitted, ExpectedRevision: x.current.Revision}, &result, done)
	}
}
func (x *memberConfigEditor) rows(e *env.Env) []layout.Widget {
	if !x.asked {
		x.get(e, false)
		x.revisions(e)
	}
	th := e.Theme
	rows := []layout.Widget{quiet(th, i18n.T("Versioned YAML edits the same saved spirit as the form. Save form edits first, or keep them and reload the form deliberately after YAML changes. Local work needs no Account or Ember.")), quiet(th, i18n.T("Vault references are masked as <vault-reference>. Keep a placeholder unchanged to preserve its exact current reference; delete the field to revoke it, or explicitly name another existing vault entry. Never paste actual keys."))}
	revisionLabel := i18n.T("Loading current configuration")
	if x.current.Revision != 0 {
		revisionLabel = i18n.Tf("Current revision %d · updated %s", x.current.Revision, x.current.Updated.Local().Format("2 Jan 2006 15:04:05"))
	}
	rows = append(rows, quiet(th, revisionLabel))
	x.editor.Label, x.editor.Lines, x.editor.MaxLines = i18n.T("Current spirit YAML"), 14, 14
	rows = append(rows, func(gtx layout.Context) layout.Dimensions {
		if x.busy || x.current.Revision == 0 {
			gtx = gtx.Disabled()
		}
		return x.editor.Layout(gtx, th)
	}, action(e, &x.save, i18n.T("Save YAML against loaded revision"), !x.busy && x.current.Revision != 0 && x.editor.Text() != x.current.YAML, func() { x.commit(e, false) }), action(e, &x.reload, i18n.T("Reload current YAML (discard YAML edits)"), !x.busy, func() { x.get(e, true) }), action(e, &x.historyReload, i18n.T("Refresh retained revisions"), !x.historyBusy, func() { x.revisions(e) }), quiet(th, i18n.Tf("Only the newest %d revisions are retained, including current and prior configuration. History never contains vault values.", x.retention)))
	if x.previews == nil {
		x.previews = map[uint64]*kit.Plaque{}
	}
	for _, rev := range x.history {
		if x.previews[rev.Revision] == nil {
			x.previews[rev.Revision] = new(kit.Plaque)
		}
		label := fmt.Sprintf("%s · %s · %s", i18n.Tf("Preview revision %d", rev.Revision), rev.At.Local().Format("2 Jan 2006 15:04:05"), rev.Name)
		if rev.Current {
			label += " · " + i18n.T("current")
		}
		rows = append(rows, action(e, x.previews[rev.Revision], label, !x.previewBusy, func() { x.show(e, rev.Revision) }))
	}
	if x.preview.Revision != 0 {
		x.historical.Label, x.historical.Lines, x.historical.MaxLines = i18n.Tf("Read-only revision %d", x.preview.Revision), 12, 12
		x.historical.Editor.ReadOnly = true
		review := toggleField(e, &x.review, i18n.T("I reviewed this historical configuration and its reporting / runtime changes"))
		rows = append(rows,
			func(gtx layout.Context) layout.Dimensions { return x.historical.Layout(gtx, th) },
			quiet(th, i18n.T("Restore revalidates today's graph, grants and policy, then creates a new current revision. It cannot restore deleted spirits, team grants or vault contents. Unsaved YAML must be saved or deliberately discarded first.")),
			func(gtx layout.Context) layout.Dimensions {
				if x.previewBusy || x.busy {
					gtx = gtx.Disabled()
				}
				return review(gtx)
			},
			action(e, &x.restore, i18n.T("Restore preview as a new revision"), !x.busy && !x.previewBusy && x.review.Value && x.current.Revision != 0 && x.editor.Text() == x.current.YAML && x.preview.Revision != x.current.Revision, func() { x.commit(e, true) }),
			action(e, &x.closePreview, i18n.T("Close historical preview"), !x.busy && !x.previewBusy, func() { x.preview = protocol.MemberConfigDocument{}; x.review.Value = false }),
		)
	}
	if x.problem != "" {
		rows = append(rows, look.Paragraph(th, th.T.Detail, th.R.Jasper, x.problem))
	}
	return rows
}
