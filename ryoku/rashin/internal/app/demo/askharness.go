package demo

import (
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// an ask Claude Code answered in Read only: it ran what only looks, and the write asked of it after was refused before it ran
func harnessAsk(now time.Time) (protocol.SessionInfo, []protocol.SessionEvent) {
	info := protocol.SessionInfo{
		ID: "ask-20261110-101000-harn", Kind: protocol.KindAsk, Title: i18n.T("Which kernel is this machine running?"),
		Harness: "claude", Model: "haiku", Mode: protocol.ApprovalsReadOnly, Cwd: demoHome,
		Origin: protocol.OriginApp, State: protocol.StateIdle,
		Created: now.Add(-35 * time.Minute), Updated: now.Add(-31 * time.Minute), Turns: 2, LastSeq: 16,
		Preview: i18n.T("This Ask only reads, so nothing was written."),
	}
	refusal := i18n.T("This Ask is Read only, so Rashin refused it: redirects output into a file. Switch the conversation to Ask first to let it make changes.")
	events := []protocol.SessionEvent{
		event(1, protocol.EvUser, protocol.UserData{Text: info.Title}),
		event(2, protocol.EvModel, protocol.ChoiceData{Current: "haiku", Options: []protocol.Choice{
			{ID: "opus", Name: "Opus 5.5"}, {ID: "sonnet", Name: "Sonnet 5"}, {ID: "haiku", Name: "Haiku 5"},
		}}),
		event(3, protocol.EvTurnStart, protocol.TurnStartData{Turn: 1}),
		event(4, protocol.EvTool, protocol.ToolData{
			Call: "toolu-uname", Title: "uname -r", Kind: protocol.ToolExecute,
			Status: protocol.ToolDone, Input: "uname -r", Output: "6.12.58-1-lts",
		}),
		event(5, protocol.EvText, protocol.TextData{Message: "answer-1", Delta: i18n.T("This machine runs kernel `6.12.58-1-lts`, the long-term support kernel.")}),
		event(6, protocol.EvTurnEnd, protocol.TurnEndData{Turn: 1, Stop: protocol.StopEnd, Usage: &protocol.TokenUsage{Input: 1840, Output: 96}}),
		event(7, protocol.EvTitle, protocol.TitleData{Title: info.Title}),
		event(8, protocol.EvUser, protocol.UserData{Text: i18n.T("Save that to ~/kernel.txt.")}),
		event(9, protocol.EvTurnStart, protocol.TurnStartData{Turn: 2}),
		event(10, protocol.EvTool, protocol.ToolData{
			Call: "toolu-save", Title: "uname -r > ~/kernel.txt", Kind: protocol.ToolExecute,
			Status: protocol.ToolPending, Input: "uname -r > ~/kernel.txt",
		}),
		event(11, protocol.EvPermission, protocol.PermissionData{
			Request: "permission-refused", Call: "toolu-save", Title: "uname -r > ~/kernel.txt",
			Kind: protocol.ToolExecute, Input: refusal,
			Options: []protocol.PermissionOption{{ID: "refused", Label: i18n.T("Refused"), Kind: protocol.OptRejectOnce}},
		}),
		event(12, protocol.EvPermissionResolved, protocol.PermissionResolvedData{
			Request: "permission-refused", Option: "refused", Outcome: "selected", By: "policy",
		}),
		event(13, protocol.EvTool, protocol.ToolData{
			Call: "toolu-save", Title: "uname -r > ~/kernel.txt", Kind: protocol.ToolExecute,
			Status: protocol.ToolFailed, Output: i18n.T("Tool call rejected by user (bash)"),
		}),
		event(14, protocol.EvText, protocol.TextData{Message: "answer-2", Delta: i18n.T("This Ask only reads, so nothing was written. Run this yourself, or switch this conversation to Ask first and ask again:") + "\n\n```sh\nuname -r > ~/kernel.txt\n```"}),
		event(15, protocol.EvTurnEnd, protocol.TurnEndData{Turn: 2, Stop: protocol.StopEnd, Usage: &protocol.TokenUsage{Input: 2210, Output: 74}}),
		event(16, protocol.EvTitle, protocol.TitleData{Title: info.Title}),
	}
	return info, events
}
