package ask

import (
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

func TestResumeWindowUsesLatestSavedAsk(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	sessions := map[string]protocol.SessionInfo{
		"old":       {ID: "old", Kind: protocol.KindAsk, Turns: 1, Updated: now.Add(-time.Hour), LastMessageAt: now.Add(-time.Hour)},
		"recent":    {ID: "recent", Kind: protocol.KindAsk, Turns: 1, Updated: now.Add(-5 * time.Minute), LastMessageAt: now.Add(-5 * time.Minute)},
		"archived":  {ID: "archived", Kind: protocol.KindAsk, Turns: 1, Updated: now.Add(-time.Minute), LastMessageAt: now.Add(-time.Minute), Archived: true},
		"temporary": {ID: "temporary", Kind: protocol.KindAsk, Turns: 1, Updated: now, LastMessageAt: now, Temporary: true},
		"agent":     {ID: "agent", Kind: protocol.KindAgent, Turns: 1, Updated: now, LastMessageAt: now},
		"empty":     {ID: "empty", Kind: protocol.KindAsk, Updated: now},
		"metadata":  {ID: "metadata", Kind: protocol.KindAsk, Turns: 1, Updated: now, LastMessageAt: now.Add(-2 * time.Hour)},
	}
	order := []string{"empty", "agent", "temporary", "archived", "old", "recent", "metadata"}
	if got := resumeSession(order, sessions, now, 10*time.Minute); got != "recent" {
		t.Fatalf("resumed %q, want recent", got)
	}
	if got := resumeSession(order, sessions, now, time.Minute); got != "" {
		t.Fatalf("resumed expired conversation %q", got)
	}
}

func TestRenamedLegacyConversationIsNotResumed(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	legacy := protocol.SessionInfo{
		ID: "legacy", Kind: protocol.KindAsk, Turns: 1,
		Title: "Renamed conversation", Created: now.Add(-24 * time.Hour), Updated: now,
	}
	sessions := map[string]protocol.SessionInfo{legacy.ID: legacy}
	if got := resumeSession([]string{legacy.ID}, sessions, now, 10*time.Minute); got != "" {
		t.Fatalf("resumed legacy conversation %q using its metadata timestamp", got)
	}
}
