package env

import (
	"strings"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// two providers offer a model under one name and the id tells them apart, an id that only repeats the name says nothing
func ChoiceOption(choice protocol.Choice) look.SelectOption {
	name, detail := choice.Name, choice.Description
	if name == "" {
		name = choice.ID
	}
	if detail == "" && !strings.EqualFold(choice.ID, name) {
		detail = choice.ID
	}
	return look.SelectOption{Label: name, Detail: detail}
}

// a session's choices laid out for a select, Selected -1 when the current one is not among them
type ChoiceList struct {
	Options  []look.SelectOption
	IDs      []string
	Selected int
}

func ListOf(choices []protocol.Choice, current string) ChoiceList {
	list := ChoiceList{Options: make([]look.SelectOption, len(choices)), IDs: make([]string, len(choices)), Selected: -1}
	for index, choice := range choices {
		list.Options[index], list.IDs[index] = ChoiceOption(choice), choice.ID
		if choice.ID == current {
			list.Selected = index
		}
	}
	return list
}
