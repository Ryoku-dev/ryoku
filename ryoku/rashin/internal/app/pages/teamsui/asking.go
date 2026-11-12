package teamsui

import (
	"slices"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// the values the selects below offer, in the order they offer them
var (
	ApprovalValues = [...]string{protocol.ApprovalsReadOnly, protocol.ApprovalsAsk, protocol.ApprovalsAuto}
	NotifyValues   = [...]string{protocol.NotifyAll, protocol.NotifyNeeds, protocol.NotifyNone}
)

// the value that stops the asking reads in amber with an open lock, as it does for Agents
func ApprovalsSelect(gtx layout.Context, e *env.Env, control *look.Select, value string) layout.Dimensions {
	selected := max(0, slices.Index(ApprovalValues[:], value))
	control.Form, control.Label = look.SelectQuiet, i18n.T("Permissions")
	control.Icon, control.Tone = look.Icon.Lock, palette.Idle
	if value == protocol.ApprovalsAuto {
		control.Icon, control.Tone = look.Icon.Unlock, palette.Attention
	}
	options := []look.SelectOption{
		{Label: ApprovalsWord(protocol.ApprovalsReadOnly), Detail: i18n.T("Reads and reports. It changes no file.")},
		{Label: ApprovalsWord(protocol.ApprovalsAsk), Detail: i18n.T("Reads by itself and asks you before it changes anything.")},
		{Label: ApprovalsWord(protocol.ApprovalsAuto), Detail: i18n.T("Changes files and runs commands without asking you.")},
	}
	return control.LayoutOptions(gtx, e.Theme, e.Overlays, options, selected)
}

// a permissions value in the words its select shows
func ApprovalsWord(value string) string {
	switch value {
	case protocol.ApprovalsReadOnly:
		return i18n.T("Read only")
	case protocol.ApprovalsAuto:
		return i18n.T("Act without asking")
	}
	return i18n.T("Ask before changes")
}

func NotifySelect(gtx layout.Context, e *env.Env, control *look.Select, value string) layout.Dimensions {
	selected := max(0, slices.Index(NotifyValues[:], value))
	control.Form, control.Label = look.SelectQuiet, i18n.T("Notifications")
	options := []look.SelectOption{
		{Label: i18n.T("Reports and when it needs me"), Detail: i18n.T("A desktop notification for every report and every question.")},
		{Label: i18n.T("Only when it needs me"), Detail: i18n.T("Reports wait in the Inbox; questions still reach the desktop.")},
		{Label: i18n.T("Nothing"), Detail: i18n.T("Nothing reaches the desktop. Look in the Inbox.")},
	}
	return control.LayoutOptions(gtx, e.Theme, e.Overlays, options, selected)
}
