// Package ask holds the Ask-only hooks of Rashin's native engine.
package ask

import (
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// Mode supplies Ask's turn handling to the shared engine.
type Mode struct {
	client llm.Client
	habits habitCache
}

// New constructs the Ask hooks used by the native engine.
func New(client llm.Client) *Mode { return &Mode{client: client} }

// Message carries the current question and its attachments into the model.
func (m *Mode) Message(text string, attachments []protocol.Attachment) (llm.Message, error) {
	return userMessage(text, attachments)
}

// History replays older Ask turns until an engine transcript exists.
func (m *Mode) History(events []protocol.SessionEvent) []llm.Message {
	return historyMessages(events, historyBudgetBytes)
}

// Habits returns the user's optional Ask prompt section.
func (m *Mode) Habits(settings protocol.Settings) string {
	if !settings.Ask.Habits {
		return ""
	}
	return m.habits.summary()
}
