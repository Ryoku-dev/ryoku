package env

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// whose asking a control sets
const (
	AskApprovals = iota
	AgentApprovals
)

var approvalValues = [...]string{protocol.ApprovalsReadOnly, protocol.ApprovalsAsk, protocol.ApprovalsAuto}

// the choice is also the setting, so it reads the same in every composer and in Settings
type Approvals struct {
	control look.Select
	// picked here and not yet confirmed by the daemon, with the value it replaces and where it was picked
	picked, from, scope string
	changed             string
}

func approvalOptions(of int) []look.SelectOption {
	if of == AskApprovals {
		return []look.SelectOption{
			{Label: i18n.T("Read only"), Detail: i18n.T("Reads this machine and proposes commands. It changes nothing.")},
			{Label: i18n.T("Ask first"), Detail: i18n.T("May run commands and write files. Each change waits for you.")},
			{Label: i18n.T("YOLO"), Detail: i18n.T("Changes things without asking. Only a destructive command still waits.")},
		}
	}
	return []look.SelectOption{
		{Label: i18n.T("Ask before changes"), Detail: i18n.T("The agent reads by itself and asks before anything else.")},
		{Label: i18n.T("Ask every time"), Detail: i18n.T("The agent asks before every action, reading too.")},
		{Label: i18n.T("YOLO"), Detail: i18n.T("The agent never asks.")},
	}
}

func ApprovalsNote(of int, value string) string {
	if of == AskApprovals {
		switch value {
		case protocol.ApprovalsAsk:
			return i18n.T("Rashin reads your system to answer, and asks before it changes anything.")
		case protocol.ApprovalsAuto:
			return i18n.T("Rashin reads your system to answer, and may change it without asking.")
		}
		return i18n.T("Rashin reads your system to answer. It never changes it.")
	}
	switch value {
	case protocol.ApprovalsAuto:
		return i18n.T("It may act without asking.")
	case protocol.ApprovalsAsk:
		return i18n.T("It asks before every action.")
	}
	return i18n.T("It asks before it changes anything.")
}

// scope is a session, or "" before one exists: a pick shows only where it was made
func (approvals *Approvals) Value(scope, current string) string {
	if approvals.picked != "" {
		if scope == approvals.scope && current == approvals.from {
			return approvals.picked
		}
		// it moved, to this pick or to one made elsewhere, or the composer shows another session
		approvals.picked = ""
	}
	if !protocol.KnownApprovals(current) {
		return protocol.ApprovalsReadOnly
	}
	return current
}

// focus takes the keyboard back when the list closes
func (approvals *Approvals) Layout(gtx layout.Context, e *Env, of int, scope, current string, focus any) layout.Dimensions {
	value := approvals.Value(scope, current)
	selected := 0
	for index, candidate := range approvalValues {
		if candidate == value {
			selected = index
		}
	}
	control := &approvals.control
	control.Form, control.Return = look.SelectQuiet, focus
	control.Label = i18n.T("Asking")
	control.Icon, control.Tone = look.Icon.Lock, palette.Idle
	switch {
	case value == protocol.ApprovalsAuto:
		control.Icon, control.Tone = look.Icon.Unlock, palette.Attention
	case of == AskApprovals && value == protocol.ApprovalsReadOnly:
		control.Icon = look.Icon.Show
	}
	dimensions := control.LayoutOptions(gtx, e.Theme, e.Overlays, approvalOptions(of), selected)
	if index, changed := control.Changed(); changed && index >= 0 && index < len(approvalValues) && approvalValues[index] != value {
		approvals.picked, approvals.from, approvals.scope = approvalValues[index], current, scope
		approvals.changed = approvalValues[index]
	}
	return dimensions
}

func (approvals *Approvals) Changed() (string, bool) {
	value := approvals.changed
	approvals.changed = ""
	return value, value != ""
}

func (e *Env) SetApprovals(of int, value string) {
	section := "agent"
	if of == AskApprovals {
		section = "ask"
	}
	e.SetSettings(map[string]any{section: map[string]any{"approvals": value}}, func(err error) {
		if err != nil {
			e.Notify(err.Error())
		}
	})
}
