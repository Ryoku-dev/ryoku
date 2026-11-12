package env

import (
	"gioui.org/io/key"
	"gioui.org/layout"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// a temporary conversation is never saved or learned from and ends when it is closed, so the switch stands only where one can start
type Temporary struct {
	On bool
}

// Ctrl Shift N, wherever a new conversation can start
func (temporary *Temporary) Pressed(gtx layout.Context) bool {
	pressed := false
	for {
		event, ok := gtx.Event(key.Filter{Name: "N", Required: key.ModShortcut | key.ModShift})
		if !ok {
			return pressed
		}
		if event, ok := event.(key.Event); ok && event.State == key.Press {
			pressed = true
		}
	}
}

// the header's icon plaque, in the sel fill while on
func (temporary *Temporary) Action() look.HeaderAction {
	return look.HeaderAction{Icon: look.Icon.Temporary, Description: i18n.T("Temporary conversation") + "  Ctrl Shift N", On: temporary.On}
}

// amber because it will be destroyed, first among the header's facts
func TemporaryMark() look.HeaderFact {
	return look.HeaderFact{Text: i18n.T("Temporary"), Kind: look.FactMark}
}

// the composer's field says what the conversation is, where it always fits
func TemporaryHint() string {
	return i18n.T("Temporary: not saved, nothing remembered. It ends when you close it.")
}

// an agent's own program keeps its log wherever it does, so its hint promises only what Rashin keeps
func TemporaryAgentHint() string {
	return i18n.T("Temporary: Rashin keeps and learns nothing. The agent's own program may keep its log.")
}

// nothing asks and nothing can bring it back; one that was kept has its copy to show instead of a word
func (e *Env) EndTemporary(id string, say bool) {
	e.Call(protocol.MSessionDelete, protocol.SessionRef{ID: id}, new(protocol.Empty), func(err error) {
		if err != nil {
			e.Report(err)
			return
		}
		if say {
			e.Notify(i18n.T("Temporary conversation closed."))
		}
	})
}

// the copy goes to history and is still never learned from; the caller ends the temporary one as it leaves it
func (e *Env) KeepTemporary(info protocol.SessionInfo, kept func(protocol.SessionInfo)) {
	var copied protocol.SessionInfo
	e.Call(protocol.MSessionFork, protocol.ForkParams{ID: info.ID, Seq: info.LastSeq}, &copied, func(err error) {
		if err != nil {
			e.Report(err)
			return
		}
		kept(copied)
	})
}
