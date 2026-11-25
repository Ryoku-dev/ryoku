package demo

import (
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestHistoryDemoFixturesAndFlagUpdates(t *testing.T) {
	s := &server{data: makeFixtures("history")}
	starred, ok := s.session("ask-20261110-094200-a17c")
	if !ok || !starred.Starred {
		t.Fatal("missing starred conversation")
	}
	archived, ok := s.session("ask-20261020-090000-old")
	if !ok || !archived.Archived {
		t.Fatal("missing archived conversation")
	}
	before := starred.Updated
	off := false
	result, handled, err := s.callFlags(protocol.MSessionSetFlags, marshal(protocol.SessionFlagsParams{ID: starred.ID, Starred: &off}))
	if err != nil || !handled {
		t.Fatalf("set flags = %v, %v", handled, err)
	}
	changed := result.(protocol.SessionInfo)
	if changed.Starred || changed.Archived || !changed.Updated.Equal(before) {
		t.Fatalf("incorrect partial flag update: %#v", changed)
	}
	s.data.snapshot.Settings.Features["history"] = false
	if _, _, err := s.callFlags(protocol.MSessionSetFlags, marshal(protocol.SessionFlagsParams{ID: starred.ID, Starred: &off})); err == nil {
		t.Fatal("disabled history still accepted a flag change")
	}
}
