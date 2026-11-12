package env

import (
	"slices"
	"strings"

	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// MemoryPlace is a place on the Memory page: a filter (MemoryAll to
// MemoryReplaced), one exact scope within it when Scope is set, and the
// memory open in its editor, if any.
type MemoryPlace struct {
	Filter string
	Scope  string
	ID     string
	// Card opens What Ask sees beside the list
	Card bool
}

// Filters of the Memory page, as MemoryPlace.Filter and the routes
// memory/<filter> name them.
const (
	MemoryAll       = ""
	MemoryAbout     = "about"
	MemoryProjects  = "projects"
	MemoryTeams     = "teams"
	MemorySuggested = "suggested"
	MemoryReplaced  = "replaced"
)

// MemoryFilters are the Memory page's filters in the order its index lists
// them.
var MemoryFilters = []string{MemoryAll, MemoryAbout, MemoryProjects, MemoryTeams, MemorySuggested, MemoryReplaced}

// ValidMemoryFilter reports whether filter names one of MemoryFilters.
func ValidMemoryFilter(filter string) bool { return slices.Contains(MemoryFilters, filter) }

// MemoryListParams is what memory.list is asked for a place: the scope group
// or exact scope, and the state it shows.
func MemoryListParams(place MemoryPlace) protocol.MemoryListParams {
	var params protocol.MemoryListParams
	switch place.Filter {
	case MemoryAbout:
		params.Scope = protocol.MemoryYou
	case MemoryProjects:
		params.Scope = protocol.MemoryProjects
	case MemoryTeams:
		params.Scope = protocol.MemoryTeams
	case MemorySuggested:
		params.Status = protocol.MemorySuggested
	case MemoryReplaced:
		params.Status = protocol.MemoryReplaced
	}
	if place.Scope != "" {
		params.Scope = place.Scope
	}
	return params
}

// PlainMemory is a memory's text with its code span marks taken out, for a
// placard or a screen reader.
func PlainMemory(text string) string { return strings.ReplaceAll(text, "`", "") }

// ListMemory calls memory.list.
func (e *Env) ListMemory(params protocol.MemoryListParams, done func(protocol.MemoryListResult, error)) {
	var result protocol.MemoryListResult
	e.Call(protocol.MMemoryList, params, &result, func(err error) {
		if done != nil {
			done(result, err)
		}
	})
}

// GetMemory calls memory.get: one memory with its earlier versions.
func (e *Env) GetMemory(id string, done func(protocol.Memory, error)) {
	var result protocol.Memory
	e.Call(protocol.MMemoryGet, protocol.MemoryRefParams{ID: id}, &result, func(err error) {
		if done != nil {
			done(result, err)
		}
	})
}

// ChangeMemory calls one of the memory methods that answer with the change
// they wrote: memory.add, edit, forget, restore, keep, dismiss, undo, redo,
// forget_all and import. A refusal is reported unless done is set.
func (e *Env) ChangeMemory(method string, params any, done func(protocol.MemoryChange, error)) {
	var result protocol.MemoryChange
	e.Call(method, params, &result, func(err error) {
		if done != nil {
			done(result, err)
			return
		}
		e.Report(err)
	})
}

// MemoryCard calls memory.card, building the card again first when rebuild
// is set.
func (e *Env) MemoryCard(rebuild bool, done func(protocol.MemoryCard, error)) {
	var result protocol.MemoryCard
	e.Call(protocol.MMemoryCard, protocol.MemoryCardParams{Rebuild: rebuild}, &result, func(err error) {
		if done != nil {
			done(result, err)
		}
	})
}

// UndoMemory undoes a change, telling a refusal.
func (e *Env) UndoMemory(change string) {
	e.ChangeMemory(protocol.MMemoryUndo, protocol.MemoryChangeParams{Change: change}, nil)
}

// NotifyChange tells what a change did with Undo beside it for the life of
// the placard.
func (e *Env) NotifyChange(text string, change protocol.MemoryChange) {
	if e.NotifyAction == nil || change.ID == "" {
		e.Notify(text)
		return
	}
	id := change.ID
	e.NotifyAction(text, i18n.T("Undo"), func() { e.UndoMemory(id) })
}

// ForgetMemory forgets one memory and says so with Undo.
func (e *Env) ForgetMemory(memory protocol.Memory) {
	e.ChangeMemory(protocol.MMemoryForget, protocol.MemoryForgetParams{IDs: []string{memory.ID}}, func(change protocol.MemoryChange, err error) {
		if err != nil {
			e.Report(err)
			return
		}
		e.NotifyChange(i18n.Tf("Forgot \"%s\".", PlainMemory(memory.Text)), change)
	})
}

// AnswerSuggestions keeps (keep set) or dismisses suggested memories and
// says so.
func (e *Env) AnswerSuggestions(ids []string, keep bool) {
	method, text := protocol.MMemoryDismiss, i18n.T("Dismissed.")
	if keep {
		method, text = protocol.MMemoryKeep, i18n.T("Kept.")
	}
	e.ChangeMemory(method, protocol.MemoryIDsParams{IDs: ids}, func(change protocol.MemoryChange, err error) {
		if err != nil {
			e.Report(err)
			return
		}
		e.NotifyChange(text, change)
	})
}
