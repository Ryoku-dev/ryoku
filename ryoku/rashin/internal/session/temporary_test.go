package session

import (
	"os"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/redact"
)

// A temporary log keeps every event in memory, scrubs its turns there, and
// writes nothing: no directory, no meta.json, no events.jsonl, and List
// never sees it. Closing it drops its events.
func TestTemporaryLogWritesNothing(t *testing.T) {
	isolate(t)
	info := protocol.SessionInfo{ID: "ask-20261112-120000-abcd", Kind: protocol.KindAsk, Title: "Use " + typedToken, Origin: protocol.OriginApp, State: protocol.StateIdle, Created: time.Now(), Temporary: true}
	log, err := Create(info, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := log.Append(protocol.EvUser, protocol.UserData{Text: "my token is " + typedToken}); err != nil {
		t.Fatal(err)
	}
	if _, err := log.ScrubLast(redact.Exact); err != nil {
		t.Fatal(err)
	}
	for _, delta := range []string{"The key is ", "sk-proj-12345", "67890abcdefghijklmnop", " and that is all."} {
		if _, err := log.Append(protocol.EvText, protocol.TextData{Message: "m", Delta: delta}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := log.Append(protocol.EvTurnEnd, protocol.TurnEndData{Turn: 1, Stop: protocol.StopEnd}); err != nil {
		t.Fatal(err)
	}
	if _, err := log.ScrubTurn(redact.Exact); err != nil {
		t.Fatal(err)
	}
	if err := log.Update(func(i *protocol.SessionInfo) { i.Turns = 1 }, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths.SessionsDir()); !os.IsNotExist(err) {
		t.Fatalf("the sessions directory exists: %v", err)
	}
	events, err := log.Since(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 6 || events[5].Seq != 6 {
		t.Fatalf("events = %#v", events)
	}
	var text strings.Builder
	for _, event := range events {
		text.Write(event.Data)
	}
	for _, secret := range []string{typedToken, "12345", "67890abcdef"} {
		if strings.Contains(text.String(), secret) {
			t.Fatalf("the held events keep %q: %s", secret, text.String())
		}
	}
	if got := log.Info(); got.Title != "Use "+redact.Mark || got.LastSeq != 6 || got.Turns != 1 {
		t.Fatalf("info = %#v", got)
	}
	if later, _ := log.Since(5); len(later) != 1 || later[0].Kind != protocol.EvTurnEnd {
		t.Fatalf("since 5 = %#v", later)
	}
	if list, err := List(""); err != nil || len(list) != 0 {
		t.Fatalf("list = %#v, %v", list, err)
	}
	if _, err := ReadInfo(info.ID); !os.IsNotExist(err) {
		t.Fatalf("read info = %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	if left, _ := log.Since(0); len(left) != 0 {
		t.Fatalf("a closed temporary log keeps %d events", len(left))
	}
	if _, err := log.Append(protocol.EvText, protocol.TextData{Delta: "late"}); err == nil {
		t.Fatal("a closed log took an event")
	}
}

// A temporary Ask does not count toward Ask retention and evicts no stored
// conversation, even when the stored ones are at the limit.
func TestTemporaryAskKeepsRetention(t *testing.T) {
	isolate(t)
	start := time.Now().Add(-time.Hour)
	for index := range askRetention {
		info := protocol.SessionInfo{Kind: protocol.KindAsk, Origin: protocol.OriginApp, State: protocol.StateIdle, Created: start.Add(time.Duration(index) * time.Second)}
		log, err := Create(info, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := log.Close(); err != nil {
			t.Fatal(err)
		}
	}
	temporary, err := Create(protocol.SessionInfo{Kind: protocol.KindAsk, Origin: protocol.OriginApp, State: protocol.StateIdle, Temporary: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer temporary.Close()
	list, err := List(protocol.KindAsk)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != askRetention {
		t.Fatalf("stored asks = %d, want %d", len(list), askRetention)
	}
	if !list[len(list)-1].Created.Equal(start) {
		t.Fatalf("the oldest stored ask was evicted: %v", list[len(list)-1].Created)
	}
}
