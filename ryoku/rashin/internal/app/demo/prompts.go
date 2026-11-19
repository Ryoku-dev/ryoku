package demo

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ryoku-rashin/internal/prompts"
	"ryoku-rashin/internal/protocol"
)

func (s *server) callPrompts(method string, raw json.RawMessage) (any, bool, error) {
	if !strings.HasPrefix(method, "prompts.") {
		return nil, false, nil
	}
	if !s.data.snapshot.Settings.Features["prompts"] {
		return nil, true, protocol.Errorf(protocol.CodeForbidden, "Turn on Prompt library in Settings first")
	}
	if s.prompts == nil {
		s.prompts = prompts.BuiltIns()
		s.prompts = append(s.prompts, protocol.Prompt{
			ID: "demo-weekly-update", Name: "Weekly update", Kind: "template",
			Text:    "Turn these notes into an update for {{team}} in {{lang}}. Keep progress, blockers and next steps separate.\n\n{{content}}",
			Updated: "2026-11-18T09:00:00Z",
		})
	}
	switch method {
	case protocol.MPromptsList:
		return protocol.PromptsListResult{Prompts: append([]protocol.Prompt(nil), s.prompts...)}, true, nil
	case protocol.MPromptsSave:
		var p protocol.PromptSaveParams
		if err := decode(raw, &p); err != nil {
			return nil, true, err
		}
		if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Text) == "" {
			return nil, true, protocol.Errorf(protocol.CodeBadRequest, "Give the prompt a name and some text")
		}
		if p.Kind != "prompt" && p.Kind != "template" && p.Kind != "snippet" {
			return nil, true, protocol.Errorf(protocol.CodeBadRequest, "Choose prompt, template or snippet")
		}
		index := -1
		for i, item := range s.prompts {
			if item.ID == p.ID {
				if item.BuiltIn {
					return nil, true, protocol.Errorf(protocol.CodeBadRequest, "Copy a built-in prompt before changing it")
				}
				index = i
				break
			}
		}
		if p.ID != "" && index < 0 {
			return nil, true, protocol.Errorf(protocol.CodeBadRequest, "That prompt no longer exists")
		}
		if p.ID == "" {
			p.ID = fmt.Sprintf("demo-prompt-%d", time.Now().UnixNano())
		}
		item := protocol.Prompt{ID: p.ID, Name: strings.TrimSpace(p.Name), Kind: p.Kind, Text: p.Text, Updated: time.Now().UTC().Format(time.RFC3339)}
		if index < 0 {
			s.prompts = append(s.prompts, item)
		} else {
			s.prompts[index] = item
		}
		return item, true, nil
	case protocol.MPromptsDelete:
		var p protocol.PromptDeleteParams
		if err := decode(raw, &p); err != nil {
			return nil, true, err
		}
		for index, item := range s.prompts {
			if item.ID == p.ID {
				if item.BuiltIn {
					return nil, true, protocol.Errorf(protocol.CodeBadRequest, "Built-in prompts cannot be deleted")
				}
				s.prompts = append(s.prompts[:index], s.prompts[index+1:]...)
				return protocol.Empty{}, true, nil
			}
		}
		return nil, true, protocol.Errorf(protocol.CodeBadRequest, "That prompt no longer exists")
	case protocol.MPromptsExpand:
		var p protocol.PromptExpandParams
		if err := decode(raw, &p); err != nil {
			return nil, true, err
		}
		for _, item := range s.prompts {
			if item.ID == p.ID {
				text, err := prompts.Expand(item, p.Vars)
				return protocol.PromptExpandResult{Text: text}, true, err
			}
		}
		return nil, true, protocol.Errorf(protocol.CodeBadRequest, "That prompt no longer exists")
	default:
		return nil, false, nil
	}
}
