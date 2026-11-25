package session

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/redact"
)

// largeSession writes an agent session of many turns, each with tool
// output and streamed text, the shape of a long coding session.
func largeSession(t *testing.T, id string, turns int, secret string) {
	t.Helper()
	l, err := Create(protocol.SessionInfo{ID: id, Kind: protocol.KindAgent, Title: "Fix it"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := strings.Repeat("some tool output line with words\n", 60)
	for turn := 1; turn <= turns; turn++ {
		appendOK(t, l, protocol.EvUser, protocol.UserData{Text: "please fix it"})
		appendOK(t, l, protocol.EvTurnStart, protocol.TurnStartData{Turn: turn})
		for i := range 40 {
			output := out
			if turn == turns && i == 0 {
				output += secret
			}
			appendOK(t, l, protocol.EvTool, protocol.ToolData{Call: fmt.Sprint("c", i), Title: "read file", Status: protocol.ToolDone, Output: output})
			appendOK(t, l, protocol.EvText, protocol.TextData{Message: fmt.Sprint("m", turn), Delta: "a few words "})
		}
		appendOK(t, l, protocol.EvTurnEnd, protocol.TurnEndData{Turn: turn})
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
}

func appendOK(t *testing.T, l *Log, kind string, data any) {
	t.Helper()
	if _, err := l.Append(kind, data); err != nil {
		t.Fatal(err)
	}
}

// A pass over a large session through a snapshot opens the stored values
// once, not once for every string, whether the session is open or not.
func TestAScrubPassOpensTheStoresOnce(t *testing.T) {
	isolate(t)
	const secret = "credential-value-07-abcdefghijkl"
	largeSession(t, "agent-20261112-000000-aaaa", 8, secret)
	largeSession(t, "agent-20261112-000000-bbbb", 8, secret)
	var opens, scrubs atomic.Int64
	// a store whose opening costs what unsealing ten values does
	block, _ := aes.NewCipher(make([]byte, 32))
	aead, _ := cipher.NewGCM(block)
	nonce := make([]byte, aead.NonceSize())
	var sealed [][]byte
	for i := range 10 {
		sealed = append(sealed, aead.Seal(nil, nonce, fmt.Appendf(nil, "credential-value-%02d-abcdefghijkl", i), nil))
	}
	defer redact.RegisterStore(func() func(string) string {
		opens.Add(1)
		var values []string
		for _, box := range sealed {
			plain, _ := aead.Open(nil, nonce, box, nil)
			values = append(values, string(plain))
		}
		return func(text string) string {
			scrubs.Add(1)
			for _, value := range values {
				text = strings.ReplaceAll(text, value, "‹KEY›")
			}
			return text
		}
	}, nil)()

	start := time.Now()
	values := redact.Snapshot()
	prepared, err := PrepareScrub("agent-20261112-000000-aaaa", values.Exact)
	if err != nil || prepared == nil {
		t.Fatalf("prepare: %v, %v", prepared, err)
	}
	if err := prepared.Commit(); err != nil {
		t.Fatal(err)
	}
	open, err := Open("agent-20261112-000000-bbbb")
	if err != nil {
		t.Fatal(err)
	}
	defer open.Close()
	if changed, err := open.ScrubAll(values.Exact); err != nil || !changed {
		t.Fatalf("the open session: %v, %v", changed, err)
	}
	t.Logf("%d events per session, %d stored-value scrubs, %d store opens, took %v", open.Info().LastSeq, scrubs.Load(), opens.Load(), time.Since(start))
	if opens.Load() != 1 {
		t.Fatalf("the stores were opened %d times for one pass, want once", opens.Load())
	}
	if scrubs.Load() < 1000 {
		t.Fatalf("only %d strings were scrubbed; the session is not the size meant", scrubs.Load())
	}
	for _, id := range []string{"agent-20261112-000000-aaaa", "agent-20261112-000000-bbbb"} {
		if text := onDisk(t, id); strings.Contains(text, secret) || !strings.Contains(text, "‹KEY›") {
			t.Fatalf("%s still holds the value", id)
		}
	}
}

// A prepared scrub is put in place only if nothing wrote to the session
// since it was read; a session gone before or after reads as not existing;
// one with nothing to scrub prepares nothing.
func TestAPreparedScrubCommitsOnlyOverWhatItRead(t *testing.T) {
	isolate(t)
	id := "ask-20261112-000000-cccc"
	l, err := Create(protocol.SessionInfo{ID: id, Kind: protocol.KindAsk, Title: "Use " + printedToken}, nil)
	if err != nil {
		t.Fatal(err)
	}
	appendOK(t, l, protocol.EvUser, protocol.UserData{Text: "push with " + printedToken})
	prepared, err := PrepareScrub(id, redact.Exact)
	if err != nil || prepared == nil {
		t.Fatalf("prepare: %v, %v", prepared, err)
	}
	appendOK(t, l, protocol.EvTurnEnd, protocol.TurnEndData{Turn: 1})
	if err := prepared.Commit(); !errors.Is(err, ErrMoved) {
		t.Fatalf("a commit over a log that grew: %v", err)
	}
	prepared.Discard()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if text := onDisk(t, id); !strings.Contains(text, printedToken) || !strings.Contains(text, `"turn_end"`) {
		t.Fatalf("a refused commit changed the log: %s", text)
	}
	if prepared, err = PrepareScrub(id, redact.Exact); err != nil || prepared == nil {
		t.Fatalf("prepare again: %v, %v", prepared, err)
	}
	if err := prepared.Commit(); err != nil {
		t.Fatal(err)
	}
	if text := onDisk(t, id); strings.Contains(text, printedToken) {
		t.Fatalf("the commit left the value: %s", text)
	}
	if info, err := ReadInfo(id); err != nil || strings.Contains(info.Title, printedToken) {
		t.Fatalf("the title: %v, %q", err, info.Title)
	}
	if again, err := PrepareScrub(id, redact.Exact); err != nil || again != nil {
		t.Fatalf("a clean session prepared %v, %v", again, err)
	}
	entries, _ := os.ReadDir(paths.SessionDir(id))
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".scrub") {
			t.Fatalf("%s was left behind", entry.Name())
		}
	}

	if prepared, err = PrepareScrub("ask-20261112-000000-dddd", redact.Exact); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a session that never was: %v, %v", prepared, err)
	}
	gone := "ask-20261112-000000-eeee"
	l, err = Create(protocol.SessionInfo{ID: gone, Kind: protocol.KindAsk}, nil)
	if err != nil {
		t.Fatal(err)
	}
	appendOK(t, l, protocol.EvUser, protocol.UserData{Text: printedToken})
	l.Close()
	if prepared, err = PrepareScrub(gone, redact.Exact); err != nil || prepared == nil {
		t.Fatalf("prepare: %v, %v", prepared, err)
	}
	if err := os.RemoveAll(filepath.Join(paths.SessionDir(gone))); err != nil {
		t.Fatal(err)
	}
	if err := prepared.Commit(); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a commit into a deleted session: %v", err)
	}
	prepared.Discard()
}
