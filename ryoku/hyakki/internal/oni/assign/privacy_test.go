package assign

import (
	"path/filepath"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

func TestClearRowsPreservesHostedSpendAcrossRestart(t *testing.T) {
	at := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "day.json")
	caps := Caps{Attempts: 1, Tokens: 30}
	ledger := NewLedger(path, caps, 10, time.UTC)
	if err := ledger.Attempt(at, false); err != nil {
		t.Fatal(err)
	}
	if err := ledger.Record(protocol.OniAssignmentRow{At: at, Lane: protocol.OniAssignmentLaneFast, Input: 23, Output: 7, Estimated: true, Unmetered: true, Outcome: "ok"}, false); err != nil {
		t.Fatal(err)
	}
	before := ledger.Day(at)
	ledger.ClearRows(at)
	if len(ledger.Rows(at)) != 0 || ledger.Day(at) != before {
		t.Fatal("history clear either kept rows or reset spend")
	}
	if got := NewLedger(path, caps, 10, time.UTC); got.Day(at) != before || got.Allow(at, false) == nil {
		t.Fatal("clearing history bypassed the persisted hosted cap")
	}
}
