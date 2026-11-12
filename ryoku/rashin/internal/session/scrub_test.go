package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/redact"
)

const (
	typedToken   = "ghp_1234567890abcdefghij1234567890abcdef"
	printedToken = "glpat-1234567890abcdefghij"
	splitToken   = "sk-proj-1234567890abcdefghijklmnop"
)

func onDisk(t *testing.T, id string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(paths.SessionDir(id), "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// The deltas of one message add up to the scrubbed text, and only the
// deltas the secret touched change.
func TestScrubStreamKeepsTheDeltasTheSecretMissed(t *testing.T) {
	deltas := []string{"The key is ", "sk-proj-12345", "67890abcdef", " and ", "that is all."}
	got, changed := scrubStream(deltas, redact.Exact)
	if !changed {
		t.Fatal("the split secret was not found")
	}
	if strings.Join(got, "") != "The key is "+redact.Mark+" and that is all." {
		t.Fatalf("the deltas add up to %q", strings.Join(got, ""))
	}
	if got[0] != deltas[0] || got[3] != deltas[3] || got[4] != deltas[4] || got[2] != "" {
		t.Fatalf("deltas = %q", got)
	}
	plain := []string{"nothing ", "secret here"}
	if same, changed := scrubStream(plain, redact.Exact); changed || strings.Join(same, "") != "nothing secret here" {
		t.Fatalf("plain deltas changed: %q", same)
	}
}

// A secret typed by the user, one a tool printed and one streamed across
// deltas all leave the log on disk; everything else stays as it was, and
// the log keeps appending after the rewrite.
func TestATurnIsScrubbedOnDisk(t *testing.T) {
	isolate(t)
	info := protocol.SessionInfo{ID: "agent-20261111-120000-abcd", Kind: protocol.KindAgent, Title: "Use " + typedToken, Origin: protocol.OriginCLI, State: protocol.StateIdle, Created: time.Now()}
	log, err := Create(info, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	appendAll := func(events ...any) {
		t.Helper()
		for index := 0; index < len(events); index += 2 {
			if _, err := log.Append(events[index].(string), events[index+1]); err != nil {
				t.Fatal(err)
			}
		}
	}
	appendAll(protocol.EvUser, protocol.UserData{Text: "Deploy with " + typedToken + " please"})
	if changed, err := log.ScrubLast(redact.Exact); err != nil || !changed {
		t.Fatalf("scrubbing the prompt: %v, %v", changed, err)
	}
	if text := onDisk(t, info.ID); strings.Contains(text, typedToken) || !strings.Contains(text, "Deploy with "+redact.Mark+" please") {
		t.Fatalf("the prompt on disk: %s", text)
	}
	appendAll(
		protocol.EvTurnStart, protocol.TurnStartData{Turn: 1},
		protocol.EvText, protocol.TextData{Message: "m", Delta: "Your token "},
		protocol.EvText, protocol.TextData{Message: "m", Delta: splitToken[:12]},
		protocol.EvTool, protocol.ToolData{Call: "c1", Title: "cat .env", Kind: protocol.ToolExecute, Status: protocol.ToolDone, Output: "TOKEN=" + printedToken + "\nHOME=/home/me"},
		protocol.EvText, protocol.TextData{Message: "m", Delta: splitToken[12:]},
		protocol.EvText, protocol.TextData{Message: "m", Delta: " is set."},
		protocol.EvTerminal, protocol.TerminalData{Terminal: "t1", Command: "ls", Delta: "go.mod\nmain.go\n"},
		protocol.EvTurnEnd, protocol.TurnEndData{Turn: 1, Stop: protocol.StopEnd},
	)
	if changed, err := log.ScrubTurn(redact.Exact); err != nil || !changed {
		t.Fatalf("scrubbing the turn: %v, %v", changed, err)
	}
	text := onDisk(t, info.ID)
	for _, secret := range []string{typedToken, printedToken, splitToken[:12], splitToken[12:]} {
		if strings.Contains(text, secret) {
			t.Fatalf("%q is still on disk: %s", secret, text)
		}
	}
	for _, kept := range []string{`"delta":"Your token "`, `"delta":" is set."`, `HOME=/home/me`, `go.mod\nmain.go\n`, `"title":"cat .env"`} {
		if !strings.Contains(text, kept) {
			t.Fatalf("%s was lost: %s", kept, text)
		}
	}
	if got := log.Info(); strings.Contains(got.Title, typedToken) {
		t.Fatalf("the title keeps the secret: %q", got.Title)
	}
	if _, err := log.Append(protocol.EvUser, protocol.UserData{Text: "next"}); err != nil {
		t.Fatal(err)
	}
	events, err := log.Since(0)
	if err != nil || len(events) != 10 || events[9].Seq != 10 {
		t.Fatalf("after the rewrite the log reads %d events: %v", len(events), err)
	}
	var merged strings.Builder
	for _, ev := range events {
		if ev.Kind == protocol.EvText {
			var data protocol.TextData
			if err := json.Unmarshal(ev.Data, &data); err != nil {
				t.Fatal(err)
			}
			merged.WriteString(data.Delta)
		}
	}
	if merged.String() != "Your token "+redact.Mark+" is set." {
		t.Fatalf("the message replays as %q", merged.String())
	}
}

// A whole-log scrub while a turn runs moves the events before that turn;
// the turn's own scrub at its end still finds where it starts, and a secret
// split across the two scrubs is found whole.
func TestAWholeLogScrubMidTurnKeepsTheTurnsPlace(t *testing.T) {
	isolate(t)
	info := protocol.SessionInfo{ID: "agent-20261112-120000-abcd", Kind: protocol.KindAgent, Origin: protocol.OriginCLI, State: protocol.StateIdle, Created: time.Now()}
	log, err := Create(info, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	add := func(kind string, data any) {
		t.Helper()
		if _, err := log.Append(kind, data); err != nil {
			t.Fatal(err)
		}
	}
	add(protocol.EvUser, protocol.UserData{Text: "Clone with " + typedToken})
	add(protocol.EvTurnEnd, protocol.TurnEndData{Turn: 1, Stop: protocol.StopEnd})
	keep := func(text string) string { return text }
	if _, err := log.ScrubTurn(keep); err != nil {
		t.Fatal(err)
	}
	add(protocol.EvUser, protocol.UserData{Text: "Now push"})
	add(protocol.EvText, protocol.TextData{Message: "m", Delta: "Pushed with " + splitToken[:12]})
	if changed, err := log.ScrubAll(redact.Exact); err != nil || !changed {
		t.Fatalf("the whole-log scrub: %v, %v", changed, err)
	}
	add(protocol.EvText, protocol.TextData{Message: "m", Delta: splitToken[12:] + "."})
	add(protocol.EvTurnEnd, protocol.TurnEndData{Turn: 2, Stop: protocol.StopEnd})
	if changed, err := log.ScrubTurn(redact.Exact); err != nil || !changed {
		t.Fatalf("the turn's scrub after it: %v, %v", changed, err)
	}
	text := onDisk(t, info.ID)
	for _, secret := range []string{typedToken, splitToken[:12], splitToken[12:]} {
		if strings.Contains(text, secret) {
			t.Fatalf("%q is still on disk: %s", secret, text)
		}
	}
	if !strings.Contains(text, "Pushed with "+redact.Mark) || !strings.Contains(text, "Now push") {
		t.Fatalf("the turn reads: %s", text)
	}
}

// A turn with nothing secret is never rewritten, and a reopened log scrubs
// only the turn after its last ended one.
func TestOrdinaryTurnsAreLeftAlone(t *testing.T) {
	isolate(t)
	info := protocol.SessionInfo{ID: "ask-20261111-120000-abcd", Kind: protocol.KindAsk, Origin: protocol.OriginCLI, State: protocol.StateIdle, Created: time.Now()}
	log, err := Create(info, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		kind string
		data any
	}{
		{protocol.EvUser, protocol.UserData{Text: "How do I set password = in sshd_config?"}},
		{protocol.EvText, protocol.TextData{Message: "m", Delta: "Set PasswordAuthentication no, then run sk-learn-tutorial."}},
		{protocol.EvTurnEnd, protocol.TurnEndData{Turn: 1, Stop: protocol.StopEnd}},
	} {
		if _, err := log.Append(step.kind, step.data); err != nil {
			t.Fatal(err)
		}
	}
	before := onDisk(t, info.ID)
	if changed, err := log.ScrubTurn(redact.Exact); err != nil || changed {
		t.Fatalf("an ordinary turn: %v, %v", changed, err)
	}
	if after := onDisk(t, info.ID); after != before {
		t.Fatalf("the log changed:\n%s\n%s", before, after)
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.Append(protocol.EvText, protocol.TextData{Message: "n", Delta: "and " + printedToken}); err != nil {
		t.Fatal(err)
	}
	if changed, err := reopened.ScrubTurn(redact.Exact); err != nil || !changed {
		t.Fatalf("the next turn: %v, %v", changed, err)
	}
	if text := onDisk(t, info.ID); strings.Contains(text, printedToken) || !strings.HasPrefix(text, before) {
		t.Fatalf("the reopened log: %s", text)
	}
}

// A token found in one turn is scrubbed from a later turn that restates its
// body alone, and a stored session scrubbed whole finds the body even where
// it comes before the token that teaches it.
func TestATokenFoundOnceIsScrubbedEverywhereInTheSession(t *testing.T) {
	isolate(t)
	body := typedToken[len("ghp_"):]
	info := protocol.SessionInfo{ID: "agent-20261112-130000-abcd", Kind: protocol.KindAgent, Origin: protocol.OriginCLI, State: protocol.StateIdle, Created: time.Now()}
	log, err := Create(info, nil)
	if err != nil {
		t.Fatal(err)
	}
	appendOK(t, log, protocol.EvUser, protocol.UserData{Text: "use " + typedToken})
	if _, err := log.ScrubLast(redact.Exact); err != nil {
		t.Fatal(err)
	}
	appendOK(t, log, protocol.EvTurnEnd, protocol.TurnEndData{Turn: 1})
	if _, err := log.ScrubTurn(redact.Exact); err != nil {
		t.Fatal(err)
	}
	appendOK(t, log, protocol.EvTool, protocol.ToolData{Call: "c", Output: "remote says " + body[:20]})
	appendOK(t, log, protocol.EvThought, protocol.TextData{Message: "r", Delta: "it was " + body[:15]})
	appendOK(t, log, protocol.EvThought, protocol.TextData{Message: "r", Delta: body[15:]})
	appendOK(t, log, protocol.EvTurnEnd, protocol.TurnEndData{Turn: 2})
	if changed, err := log.ScrubTurn(redact.Exact); err != nil || !changed {
		t.Fatalf("the later turn: %v, %v", changed, err)
	}
	if text := onDisk(t, info.ID); strings.Contains(text, body[15:]) || strings.Contains(text, "it was "+body[:15]) || !strings.Contains(text, "it was "+redact.Mark) {
		t.Fatalf("the restated body: %s", text)
	}
	if !strings.Contains(onDisk(t, info.ID), "remote says "+body[:20]) {
		t.Fatal("a fragment shorter than the body was taken as the body")
	}
	log.Close()

	stored := "ask-20261112-130000-abcd"
	old, err := Create(protocol.SessionInfo{ID: stored, Kind: protocol.KindAsk, Title: "about " + body}, nil)
	if err != nil {
		t.Fatal(err)
	}
	appendOK(t, old, protocol.EvText, protocol.TextData{Message: "a", Delta: "body " + body})
	appendOK(t, old, protocol.EvUser, protocol.UserData{Text: "the token is " + typedToken})
	old.Close()
	prepared, err := PrepareScrub(stored, redact.Exact)
	if err != nil || prepared == nil {
		t.Fatalf("prepare: %v, %v", prepared, err)
	}
	if err := prepared.Commit(); err != nil {
		t.Fatal(err)
	}
	if text := onDisk(t, stored); strings.Contains(text, body) {
		t.Fatalf("the stored session keeps the body: %s", text)
	}
	if got, err := ReadInfo(stored); err != nil || strings.Contains(got.Title, body) {
		t.Fatalf("the stored title: %v, %q", err, got.Title)
	}
}
