package env

import (
	"ryoku-i18n"

	"ryoku-rashin/internal/app/chat"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// a line opened long after its change still has to know whether it was undone, so the daemon is asked once for any change the store has not heard of
type MemoryLines struct {
	asked map[string]bool
	// the transcript and the point in it last looked through, so a frame that brought nothing new costs nothing
	model *chat.Model
	seq   int64
}

// before the view is laid out
func (lines *MemoryLines) Sync(e *Env, snapshot state.Snapshot, view *chat.View, model *chat.Model) {
	view.SetMemoryChanges(snapshot.MemoryChanges)
	if snapshot.Memory == nil || model == nil || model == lines.model && model.LastSeq() == lines.seq {
		return
	}
	lines.model, lines.seq = model, model.LastSeq()
	if lines.asked == nil {
		lines.asked = make(map[string]bool)
	}
	var missing []string
	for _, id := range model.MemoryChanges() {
		if _, known := snapshot.MemoryChanges[id]; !known && !lines.asked[id] {
			lines.asked[id] = true
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return
	}
	var result protocol.MemoryChangesResult
	e.Call(protocol.MMemoryChanges, protocol.MemoryChangesParams{IDs: missing}, &result, func(err error) {
		if err == nil {
			e.Store.SetMemoryChanges(result.Changes)
		}
	})
}

// what a transcript's memory lines asked for, in the conversation id
func (e *Env) memoryAction(id string, action chat.Action) {
	switch action.Kind {
	case chat.ActMemoryUndo, chat.ActMemoryRedo:
		method := protocol.MMemoryUndo
		if action.Kind == chat.ActMemoryRedo {
			method = protocol.MMemoryRedo
		}
		e.ChangeMemory(method, protocol.MemoryChangeParams{Change: action.Value}, func(change protocol.MemoryChange, err error) {
			if err != nil {
				e.Report(err)
				return
			}
			e.Store.SetMemoryChanges([]protocol.MemoryChange{change})
		})
	case chat.ActMemoryEdit:
		if e.OpenMemory != nil {
			e.OpenMemory(MemoryPlace{ID: action.Value})
		}
	case chat.ActMemorySuppress:
		e.Call(protocol.MMemorySuppress, protocol.MemorySuppressParams{Session: id, ID: action.Value}, new(protocol.Empty), func(err error) {
			if err != nil {
				e.Report(err)
				return
			}
			e.Notify(i18n.T("Rashin won't use this here again."))
		})
	case chat.ActMemoryKeep, chat.ActMemoryDismiss:
		e.AnswerSuggestions([]string{action.Value}, action.Kind == chat.ActMemoryKeep)
	case chat.ActRememberOnce:
		params := protocol.MemoryAddParams{Scope: protocol.MemoryYou, Text: action.Value, Sensitive: true, Session: id}
		e.ChangeMemory(protocol.MMemoryAdd, params, func(change protocol.MemoryChange, err error) {
			if err != nil {
				e.Report(err)
				return
			}
			e.NotifyChange(i18n.T("Remembered."), change)
		})
	case chat.ActMemoryCard:
		if e.OpenMemory != nil {
			e.OpenMemory(MemoryPlace{Card: true})
		}
	case chat.ActMemorySettings:
		if e.OpenSettings != nil {
			e.OpenSettings("memory")
		}
	case chat.ActOpenVault:
		if e.NavigateTeams != nil {
			e.NavigateTeams(TeamsPlace{Tab: protocol.TabVault})
		}
	case chat.ActOpenSession:
		tab := protocol.TabAsk
		if protocol.SessionKind(action.Option) == protocol.KindAgent {
			tab = protocol.TabAgent
		}
		e.Navigate(tab, action.Value)
	}
}
