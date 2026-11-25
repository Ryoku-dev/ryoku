package ask

import (
	"encoding/json"
	"strings"

	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

const historyBudgetBytes = 48 << 10

func historyMessages(events []protocol.SessionEvent, budget int) []llm.Message {
	events = mediaHistory(events)
	var messages []llm.Message
	asked := make(map[int]protocol.UserData)
	assistantByID := make(map[string]int)
	for _, event := range events {
		switch event.Kind {
		case protocol.EvUser:
			var data protocol.UserData
			if len(event.Data) == 0 || json.Unmarshal(event.Data, &data) != nil {
				continue
			}
			if data.Run != "" {
				messages = append(messages, llm.Message{Role: llm.RoleUser, Content: "I ran this command myself: " + data.Run})
			} else if strings.TrimSpace(data.Text) != "" || len(data.Images) > 0 {
				asked[len(messages)] = data
				messages = append(messages, llm.Message{Role: llm.RoleUser, Content: data.Text})
			}
		case protocol.EvText:
			var data protocol.TextData
			if json.Unmarshal(event.Data, &data) != nil || data.Delta == "" {
				continue
			}
			if i, ok := assistantByID[data.Message]; ok {
				messages[i].Content += data.Delta
				continue
			}
			assistantByID[data.Message] = len(messages)
			messages = append(messages, llm.Message{Role: llm.RoleAssistant, Content: data.Delta})
		}
	}
	used := 0
	start := len(messages)
	for i := len(messages) - 1; i >= 0; i-- {
		size := len(messages[i].Content)
		if used+size > budget && start < len(messages) {
			break
		}
		used += size
		start = i
	}
	imageRoom, textRoom := historyImageBytes, historyTextBytes
	for i := len(messages) - 1; i >= start; i-- {
		if data, ok := asked[i]; ok && len(data.Images) > 0 {
			messages[i] = earlierMessage(data, &imageRoom, &textRoom)
		}
	}
	return messages[start:]
}
