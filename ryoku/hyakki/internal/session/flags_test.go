package session

import (
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

func TestFlagsPersistWithoutChangingMessageRecency(t *testing.T) {
	isolate(t)
	at := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	log, err := Create(protocol.SessionInfo{ID: "ask-20261118-120000-a123", Kind: protocol.KindAsk, Updated: at, Created: at}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := log.AppendCopy(protocol.SessionEvent{Seq: 1, Kind: protocol.EvUser, TS: at}); err != nil {
		t.Fatal(err)
	}
	on := true
	if err := log.SetFlags(&on, &on); err != nil {
		t.Fatal(err)
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	log, err = Open("ask-20261118-120000-a123")
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	info := log.Info()
	if !info.Starred || !info.Archived || !info.Updated.Equal(at) || !info.LastMessageAt.Equal(at) {
		t.Fatalf("flags or recency changed: %#v", info)
	}
	off := false
	if err := log.SetFlags(&off, nil); err != nil {
		t.Fatal(err)
	}
	if info := log.Info(); info.Starred || !info.Archived {
		t.Fatalf("partial update changed another flag: %#v", info)
	}
	if err := log.Update(func(info *protocol.SessionInfo) { info.Unread = false }, nil); err != nil {
		t.Fatal(err)
	}
	if !log.Info().LastMessageAt.Equal(at) {
		t.Fatal("metadata update moved the last message")
	}
}
