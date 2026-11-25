package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/app/demo"
	"ryoku-rashin/internal/protocol"
)

// demoOniDaemon serves the app's demo daemon, which answers the Oni calls
// as the daemon does over a fake desktop, on a socket the CLI dials.
func demoOniDaemon(t *testing.T, features ...string) {
	t.Helper()
	root := t.TempDir()
	for name, dir := range map[string]string{"HOME": "home", "XDG_CONFIG_HOME": "config", "XDG_STATE_HOME": "state", "XDG_CACHE_HOME": "cache", "XDG_DATA_HOME": "data"} {
		t.Setenv(name, filepath.Join(root, dir))
	}
	// a short directory keeps the socket path within the Unix limit
	runtime, err := os.MkdirTemp("", "hyakki-oni-ask")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	socket := filepath.Join(runtime, "oni.sock")
	t.Setenv("HYAKKI_SOCKET", socket)
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- demo.Serve(ctx, socket, features...) }()
	t.Cleanup(func() {
		stop()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("the demo daemon did not stop")
		}
		_ = os.RemoveAll(runtime)
	})
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(socket); err == nil {
			return
		}
		select {
		case err := <-done:
			t.Fatalf("the demo daemon stopped before serving: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("the demo daemon did not listen")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// untouched fails a test that reads it: an answer that needed no question
// must not wait on stdin
type untouched struct{ read bool }

func (u *untouched) Read([]byte) (int, error) { u.read = true; return 0, io.EOF }

func TestOniPerchAndDetachSayWhyAWindowIsRefused(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOni)
	var out, errs bytes.Buffer
	if code := runOniPerch([]string{"21", "--json"}, &out, &errs); code != 0 {
		t.Fatalf("perch 21 exited %d: %s", code, errs.String())
	}
	var perched protocol.OniPerchResult
	if err := json.Unmarshal(out.Bytes(), &perched); err != nil || perched.Window != "21" || perched.Box == nil || perched.Title != "Packing list" || perched.Seq == 0 {
		t.Fatalf("perch printed %q: %v", out.String(), err)
	}
	for window, reason := range map[string]string{"22": "(no-geometry)", "99": "(closed)"} {
		out.Reset()
		errs.Reset()
		if code := runOniPerch([]string{window}, &out, &errs); code != 1 || !strings.Contains(errs.String(), reason) || out.Len() != 0 {
			t.Fatalf("perch %s: exit %d, stderr %q, stdout %q", window, code, errs.String(), out.String())
		}
	}
	out.Reset()
	if code := runOniDetach(nil, &out, &errs); code != 0 || !strings.Contains(out.String(), "let go") {
		t.Fatalf("detach: exit %d, %q", code, out.String())
	}
	errs.Reset()
	if code := runOniAsk([]string{"what is this?"}, &untouched{}, &out, &errs); code != 1 || !strings.Contains(errs.String(), "not perched") {
		t.Fatalf("an ask with nothing perched: exit %d, %q", code, errs.String())
	}
	for _, args := range [][]string{{"oni", "perch"}, {"oni", "perch", "21", "22"}, {"oni", "detach", "21"}, {"oni", "ask"}, {"oni", "ask", "--preview"}, {"oni", "ask", "--yes", "hi"}} {
		if code := run(args); code != 2 {
			t.Fatalf("%v exited %d, want the usage", args, code)
		}
	}
}

// The ask shows what would be sent and where, and sends it only on y.
func TestOniAskSendsOnlyAfterYesOnStdin(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOni)
	preview := filepath.Join(t.TempDir(), "preview.jpg")
	var out, errs bytes.Buffer
	if code := runOniAsk([]string{"--preview", preview, "what", "is", "this", "window", "for?"}, strings.NewReader("y\n"), &out, &errs); code != 0 {
		t.Fatalf("ask exited %d: %s", code, errs.String())
	}
	prompt := errs.String()
	for _, want := range []string{"960 × 620 pixels", "through toplevel", "It goes to openrouter/qwen/qwen2.5-vl-72b-instruct and leaves this computer.", "Allow once?", "Answered by openrouter/qwen/qwen2.5-vl-72b-instruct"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("stderr lacks %q:\n%s", want, prompt)
		}
	}
	if !strings.HasPrefix(out.String(), "It is a packing list") {
		t.Fatalf("stdout = %q", out.String())
	}
	raw, err := os.ReadFile(preview)
	if err != nil {
		t.Fatal(err)
	}
	if thumb, err := jpeg.Decode(bytes.NewReader(raw)); err != nil || thumb.Bounds().Dx() != 480 {
		t.Fatalf("the preview file is not the thumbnail shown: %v", err)
	}

	// once per session: the same route answers without a question
	stdin := &untouched{}
	out.Reset()
	errs.Reset()
	if code := runOniAsk([]string{"--json", "where is the rain jacket?"}, stdin, &out, &errs); code != 0 || stdin.read {
		t.Fatalf("the second ask: exit %d, read stdin %v, %s", code, stdin.read, errs.String())
	}
	var result protocol.OniAskResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.Status != protocol.OniAskAnswered || result.Answer == nil || result.Answer.Point == nil {
		t.Fatalf("--json printed %q: %v", out.String(), err)
	}
}

// Anything but y, and end of input, cancel: nothing is sent, so the next
// ask still asks.
func TestOniAskCancelsOnAnythingButYes(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOni, demo.FeatureOniLocal)
	for _, answer := range []string{"n\n", "", "yes please\n", "Y es\n"} {
		var out, errs bytes.Buffer
		if code := runOniAsk([]string{"what is this?"}, strings.NewReader(answer), &out, &errs); code != 1 {
			t.Fatalf("answer %q exited %d: %s", answer, code, out.String())
		}
		if out.Len() != 0 || !strings.Contains(errs.String(), "It stays on this computer: rashin-models/gemma-3-4b-it reads it here.") || !strings.Contains(errs.String(), "Cancelled. Nothing was sent.") {
			t.Fatalf("answer %q: stdout %q, stderr %q", answer, out.String(), errs.String())
		}
	}
}

func TestOniConsentStopsWaitingAtItsDeadlineOrAnInterrupt(t *testing.T) {
	never, writer := io.Pipe()
	defer writer.Close()
	if yes, expired := readOniConsent(context.Background(), never, 20*time.Millisecond); yes || !expired {
		t.Fatalf("a question nobody answers: yes %v, expired %v", yes, expired)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if yes, expired := readOniConsent(ctx, never, time.Minute); yes || expired {
		t.Fatalf("an interrupt: yes %v, expired %v", yes, expired)
	}
	for answer, want := range map[string]bool{"y\n": true, "YES\n": true, " yes \n": true, "no\n": false, "\n": false} {
		if yes, _ := readOniConsent(context.Background(), strings.NewReader(answer), time.Minute); yes != want {
			t.Fatalf("%q read as %v", answer, yes)
		}
	}
}
