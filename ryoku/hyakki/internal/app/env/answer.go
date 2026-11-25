package env

import (
	"gioui.org/layout"

	"ryoku-rashin/internal/app/chat"
)

// a key the transcript will not take as an answer is dropped, everything else passes through
func Answered(gtx layout.Context, view *chat.View, model *chat.Model, actions []chat.Action) []chat.Action {
	kept := actions[:0]
	for _, action := range actions {
		if action.Kind == chat.ActAllow || action.Kind == chat.ActRefuse {
			answer, ok := view.Answer(model, action.Kind, gtx.Now)
			if !ok {
				continue
			}
			action = answer
		}
		kept = append(kept, action)
	}
	return kept
}
