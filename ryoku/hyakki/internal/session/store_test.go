package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func isolate(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(root, "run"))
}

func TestAppendReplaySinceDropsTruncatedTail(t *testing.T) {
	isolate(t)
	info := protocol.SessionInfo{ID: "ask-20261110-120000-abcd", Kind: protocol.KindAsk, Origin: protocol.OriginCLI, State: protocol.StateIdle, Created: time.Now()}
	log, err := Create(info, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := log.Append(protocol.EvUser, protocol.UserData{Text: "one"}); err != nil {
		t.Fatal(err)
	}
	if _, err := log.Append(protocol.EvText, protocol.TextData{Message: "m", Delta: "two"}); err != nil {
		t.Fatal(err)
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(paths.SessionDir(info.ID), "events.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"seq":3,"kind":"text"`); err != nil {
		t.Fatal(err)
	}
	f.Close()
	log, err = Open(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	events, err := log.Since(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Seq != 2 {
		t.Fatalf("replay = %#v", events)
	}
}

func TestAskRetentionKeepsNewestTwoHundred(t *testing.T) {
	isolate(t)
	base := time.Date(2026, 11, 10, 1, 0, 0, 0, time.Local)
	for i := range 201 {
		id, err := NewID(protocol.KindAsk, base.Add(time.Duration(i)*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		log, err := Create(protocol.SessionInfo{ID: id, Kind: protocol.KindAsk, Origin: protocol.OriginImport, State: protocol.StateIdle, Created: base.Add(time.Duration(i) * time.Second)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := log.Close(); err != nil {
			t.Fatal(err)
		}
	}
	all, err := List(protocol.KindAsk)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 200 {
		t.Fatalf("kept %d sessions", len(all))
	}
	if !all[0].Created.Equal(base.Add(200*time.Second)) || !all[199].Created.Equal(base.Add(time.Second)) {
		t.Fatalf("wrong retention range: %s to %s", all[0].Created, all[199].Created)
	}
}

func TestAgentSessionsAreNeverRetainedAway(t *testing.T) {
	isolate(t)
	base := time.Date(2026, 11, 10, 1, 0, 0, 0, time.Local)
	for i := range 201 {
		id, err := NewID(protocol.KindAgent, base.Add(time.Duration(i)*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		log, err := Create(protocol.SessionInfo{ID: id, Kind: protocol.KindAgent, Origin: protocol.OriginApp, State: protocol.StateStopped, Created: base.Add(time.Duration(i) * time.Second)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := log.Close(); err != nil {
			t.Fatal(err)
		}
	}
	all, err := List(protocol.KindAgent)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 201 {
		t.Fatalf("kept %d Agent sessions", len(all))
	}
}

func TestCoalescerMergesAndFlushesBeforeOtherKind(t *testing.T) {
	var got []protocol.SessionEvent
	c := NewCoalescer(func(kind string, data any) error {
		raw, _ := json.Marshal(data)
		got = append(got, protocol.SessionEvent{Kind: kind, Data: raw})
		return nil
	})
	if err := c.Emit(protocol.EvText, protocol.TextData{Message: "m", Delta: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := c.Emit(protocol.EvText, protocol.TextData{Message: "m", Delta: "b"}); err != nil {
		t.Fatal(err)
	}
	if err := c.Emit(protocol.EvTool, map[string]string{"call": "c"}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Kind != protocol.EvText || got[1].Kind != protocol.EvTool {
		t.Fatalf("events = %#v", got)
	}
	var text protocol.TextData
	if err := json.Unmarshal(got[0].Data, &text); err != nil || text.Delta != "ab" {
		t.Fatalf("text = %#v, err = %v", text, err)
	}
}

func TestCoalescerWindowEmitsPendingDelta(t *testing.T) {
	done := make(chan struct{}, 1)
	c := NewCoalescer(func(kind string, data any) error {
		done <- struct{}{}
		return nil
	})
	if err := c.Emit(protocol.EvThought, protocol.TextData{Message: "m", Delta: "a"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(4 * CoalesceWindow):
		t.Fatal("pending delta was not emitted")
	}
}
