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
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/app/demo"
	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/keep"
	"ryoku-rashin/internal/protocol"
)

// roundTrip decodes stdout strictly into v and checks that encoding v again
// gives back exactly what was printed
func roundTrip(t *testing.T, out *bytes.Buffer, v any) {
	t.Helper()
	printed := bytes.Clone(out.Bytes())
	decodeOne(t, out, v)
	var again bytes.Buffer
	enc := json.NewEncoder(&again)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again.Bytes(), printed) {
		t.Fatalf("the JSON does not round-trip:\nprinted %s\nagain   %s", printed, again.Bytes())
	}
}

func guide(t *testing.T, stdin string, args ...string) (protocol.OniAskResult, string) {
	t.Helper()
	var out, errs bytes.Buffer
	var reader = strings.NewReader(stdin)
	if code := runOniGuide(append(args, "--json"), reader, &out, &errs); code != 0 {
		t.Fatalf("guide %v exited %d: %s", args, code, errs.String())
	}
	var result protocol.OniAskResult
	roundTrip(t, &out, &result)
	return result, errs.String()
}

// A walkthrough against the demo: the first picture asks on stdin, each
// step and the end print as JSON that round-trips, the keep offer is
// answered by oni keep, and the same goal again replays with no picture.
func TestOniGuideWalksKeepsAndReplaysOverJSON(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOniGuide)
	step1, prompt := guide(t, "y\n", "start", "how", "do", "I", "save", "this", "file")
	if a := step1.Answer; step1.Status != protocol.OniAskAnswered || a == nil || a.Mode != protocol.OniModeGuide || a.Step != 1 || a.Point == nil || a.Replay {
		t.Fatalf("step 1 = %+v", step1)
	}
	if !strings.Contains(prompt, "Allow once?") {
		t.Fatalf("the first picture did not ask: %q", prompt)
	}
	circled, _ := guide(t, "", "circle")
	if circled.Answer.Step != 1 || circled.Answer.Seq <= step1.Answer.Seq {
		t.Fatalf("circle = %+v", circled.Answer)
	}
	step2, prompt := guide(t, "", "next")
	if step2.Answer.Step != 2 || strings.Contains(prompt, "Allow once?") {
		t.Fatalf("step 2 = %+v, asked %q", step2.Answer, prompt)
	}
	end, _ := guide(t, "", "next")
	if end.Status != protocol.OniAskEnded || !end.Answer.Done || end.Keep == nil || end.Keep.ExpiresMs != feature.KeepOfferTTL().Milliseconds() {
		t.Fatalf("the end = %+v", end)
	}

	var out, errs bytes.Buffer
	if code := runOniKeep("keep", []string{end.Keep.ID, "--json"}, &out, &errs); code != 0 {
		t.Fatalf("keep exited %d: %s", code, errs.String())
	}
	var kept protocol.OniKeepResult
	roundTrip(t, &out, &kept)
	// the guide demo starts with three kept
	if !kept.Kept || kept.Entries != 4 {
		t.Fatalf("keep = %+v", kept)
	}
	out.Reset()
	errs.Reset()
	if code := runOniKeep("keep", []string{end.Keep.ID}, &out, &errs); code != 1 || !strings.Contains(errs.String(), "not waiting") {
		t.Fatalf("keeping twice: exit %d, %q", code, errs.String())
	}

	stdin := &untouched{}
	out.Reset()
	errs.Reset()
	if code := runOniGuide([]string{"start", "how do I save this file"}, stdin, &out, &errs); code != 0 || stdin.read {
		t.Fatalf("the replay: exit %d, read stdin %v, %s", code, stdin.read, errs.String())
	}
	if out.String() != "Kept from before, step 1: Open the File menu.\n" || !strings.Contains(errs.String(), "no picture was sent") {
		t.Fatalf("the replay printed %q, %q", out.String(), errs.String())
	}
	stopped, _ := guide(t, "", "stop")
	if stopped.Status != protocol.OniAskEnded || stopped.Answer == nil || stopped.Answer.Done || !stopped.Answer.Replay {
		t.Fatalf("stop = %+v", stopped)
	}
}

// Without --json a step says what to do and how to move on, the end says
// how to answer the keep offer, skip keeps nothing, and a no to the
// picture sends nothing.
func TestOniGuideTextSkipsAndRefusals(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOniGuide)
	var out, errs bytes.Buffer
	if code := runOniGuide([]string{"start", "export", "the", "list"}, strings.NewReader("n\n"), &out, &errs); code != 1 || out.Len() != 0 || !strings.Contains(errs.String(), "Cancelled. Nothing was sent.") {
		t.Fatalf("no to the picture: exit %d, %q, %q", code, out.String(), errs.String())
	}
	if code := runOniGuide([]string{"next"}, strings.NewReader(""), &out, &errs); code != 1 || !strings.Contains(errs.String(), "No guide is running") {
		t.Fatalf("next after the no: exit %d, %q", code, errs.String())
	}
	out.Reset()
	errs.Reset()
	if code := runOniGuide([]string{"start", "export the list"}, strings.NewReader("y\n"), &out, &errs); code != 0 {
		t.Fatalf("start exited %d: %s", code, errs.String())
	}
	if out.String() != "Step 1: Open the File menu.\n" || !strings.Contains(errs.String(), "Click it in the window, or run hyakki oni guide next.") {
		t.Fatalf("step 1 printed %q, %q", out.String(), errs.String())
	}
	for range 2 {
		out.Reset()
		if code := runOniGuide([]string{"next"}, strings.NewReader(""), &out, &errs); code != 0 {
			t.Fatalf("next exited %d: %s", code, errs.String())
		}
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 || lines[0] != "Done. The packing list is saved." || lines[1] != "Keep these steps for another time?" || !strings.HasPrefix(lines[2], "Run hyakki oni keep keep-1 within 12 seconds") {
		t.Fatalf("the end printed %q", out.String())
	}
	out.Reset()
	if code := runOniKeep("skip", []string{"keep-1"}, &out, &errs); code != 0 || out.String() != "Not kept.\n" {
		t.Fatalf("skip: exit %d, %q", code, out.String())
	}
	errs.Reset()
	if code := runOniKeep("keep", []string{"keep-1"}, &out, &errs); code != 1 || !strings.Contains(errs.String(), "nothing was kept") {
		t.Fatalf("keep after skip: exit %d, %q", code, errs.String())
	}
	out.Reset()
	if code := runOniGuide([]string{"stop"}, strings.NewReader(""), &out, &errs); code != 0 || out.String() != "No guide was running.\n" {
		t.Fatalf("stop with no guide: exit %d, %q", code, out.String())
	}

	for _, args := range [][]string{
		{"oni", "guide"}, {"oni", "guide", "start"}, {"oni", "guide", "next", "now"}, {"oni", "guide", "begin", "x"}, {"oni", "guide", "start", "x", "--preview"},
		{"oni", "guide", "start", "x", "--text", "y"}, {"oni", "keep"}, {"oni", "skip", "a", "b"}, {"oni", "keep", "a", "--all"},
		{"oni", "keeps", "x"}, {"oni", "keeps", "--forget"}, {"oni", "keeps", "--all", "--forget", "k"},
		{"oni", "pick", "now"}, {"oni", "pick", "--text"}, {"oni", "pick", "--text", " "}, {"oni", "pick", "cancel", "--text", "q"},
	} {
		if code := run(args); code != 2 {
			t.Fatalf("%v exited %d, want the usage", args, code)
		}
	}
}

func TestOniGuideWithGuidesOffIsRefused(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOni)
	var out, errs bytes.Buffer
	if code := runOniGuide([]string{"start", "save this file", "--json"}, strings.NewReader("y\n"), &out, &errs); code != 1 || out.Len() != 0 || !strings.Contains(errs.String(), feature.ReasonFeatureOff) {
		t.Fatalf("a guide with guides off: exit %d, %q, %q", code, out.String(), errs.String())
	}
}

// oni keeps lists this computer's keeps by key, app and goal with no
// picture or title, forgets one or all, and moves a damaged file aside.
func TestOniKeepsListsAndForgets(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOniGuide)
	store := keep.NewStore("")
	step := func(text, label string) keep.Step {
		return keep.Step{Text: text, Label: label, Role: "menu", Box: feature.Box{X: .1, Y: .1, W: .1, H: .05}, Mode: feature.StrandModeLasso, Trusted: true}
	}
	for _, e := range []keep.Entry{
		{App: "org.example.Notes", Goal: "print the list", Steps: []keep.Step{step("Open the File menu.", "File"), step("Click Print.", "Print")}},
		{App: "org.example.Mail", Goal: "add a signature", Steps: []keep.Step{step("Open Settings.", "Settings")}},
	} {
		if err := store.Put(e); err != nil {
			t.Fatal(err)
		}
	}
	keeps := func(args ...string) oniKeepsResult {
		t.Helper()
		var out, errs bytes.Buffer
		if code := runOniKeeps(append(args, "--json"), &out, &errs); code != 0 {
			t.Fatalf("keeps %v exited %d: %s", args, code, errs.String())
		}
		if strings.Contains(strings.ToLower(out.String()), "title") || strings.Contains(out.String(), "picture") {
			t.Fatalf("the list names a title or picture: %s", out.String())
		}
		var result oniKeepsResult
		roundTrip(t, &out, &result)
		return result
	}
	listed := keeps()
	byGoal := map[string]oniKept{}
	for _, k := range listed.Keeps {
		byGoal[k.Goal] = k
	}
	mail, notes := byGoal["add a signature"], byGoal["print the list"]
	if len(listed.Keeps) != 2 || mail.App != "org.example.Mail" || mail.Steps != 1 || notes.App != "org.example.Notes" || notes.Steps != 2 || mail.Key == "" || notes.Key == "" {
		t.Fatalf("the list = %+v", listed)
	}
	var out, errs bytes.Buffer
	if code := runOniKeeps(nil, &out, &errs); code != 0 || !strings.Contains(out.String(), "\t2 steps\t") || !strings.Contains(out.String(), "\tprint the list\n") {
		t.Fatalf("keeps: exit %d, %q", code, out.String())
	}
	forgot := keeps("--forget", mail.Key)
	if forgot.Forgot != mail.Key || len(forgot.Keeps) != 1 || forgot.Keeps[0].Key != notes.Key {
		t.Fatalf("forget = %+v", forgot)
	}
	errs.Reset()
	if code := runOniKeeps([]string{"--forget", mail.Key}, &out, &errs); code != 1 || !strings.Contains(errs.String(), "not there any more") {
		t.Fatalf("forgetting twice: exit %d, %q", code, errs.String())
	}
	if all := keeps("--all"); !all.All || len(all.Keeps) != 0 {
		t.Fatalf("forget all = %+v", all)
	}

	if err := os.MkdirAll(filepath.Dir(store.Path()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.Path(), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	errs.Reset()
	if code := runOniKeeps(nil, &out, &errs); code != 1 || !strings.Contains(errs.String(), keep.ReasonDamaged) {
		t.Fatalf("a damaged file: exit %d, %q", code, errs.String())
	}
	moved := keeps("--all")
	if moved.MovedAside != store.DamagedPath() || len(moved.Keeps) != 0 {
		t.Fatalf("forget all over a damaged file = %+v", moved)
	}
	if raw, err := os.ReadFile(store.DamagedPath()); err != nil || string(raw) != "{not json" {
		t.Fatalf("the damaged file was not kept aside: %q, %v", raw, err)
	}
}

// oni keeps --all also clears the record of hints offered, and moves a
// hint record that cannot be read aside, after which hints start over.
func TestOniKeepsAllStartsTheHintRecordOver(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOniGuide)
	hints := keep.NewStore("").Hints()
	at := time.Date(2026, 11, 22, 9, 0, 0, 0, time.Local)
	key := keep.KeyFor("org.example.Notes", "print the list")
	if got, err := hints.Offer("org.example.Notes", []string{key}, at); err != nil || got != key {
		t.Fatalf("offer = %q, %v", got, err)
	}
	all := func() oniKeepsResult {
		t.Helper()
		var out, errs bytes.Buffer
		if code := runOniKeeps([]string{"--all", "--json"}, &out, &errs); code != 0 {
			t.Fatalf("keeps --all exited %d: %s", code, errs.String())
		}
		var result oniKeepsResult
		roundTrip(t, &out, &result)
		return result
	}
	if cleared := all(); !cleared.All || cleared.HintsMovedAside != "" {
		t.Fatalf("forget all = %+v", cleared)
	}
	if _, err := os.Stat(hints.Path()); !os.IsNotExist(err) {
		t.Fatalf("the hint record outlived forget all: %v", err)
	}

	if err := os.WriteFile(hints.Path(), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := hints.Offer("org.example.Notes", []string{key}, at); got != "" || err == nil {
		t.Fatalf("a damaged record offered %q, %v", got, err)
	}
	moved := all()
	if moved.HintsMovedAside != hints.DamagedPath() || moved.MovedAside != "" {
		t.Fatalf("forget all over a damaged hint record = %+v", moved)
	}
	if raw, err := os.ReadFile(hints.DamagedPath()); err != nil || string(raw) != "{not json" {
		t.Fatalf("the damaged record was not kept aside: %q, %v", raw, err)
	}
	if got, err := hints.Offer("org.example.Notes", []string{key}, at); err != nil || got != key {
		t.Fatalf("after starting over, offer = %q, %v", got, err)
	}
	var out, errs bytes.Buffer
	if err := os.WriteFile(hints.Path(), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := runOniKeeps([]string{"--all"}, &out, &errs); code != 0 || !strings.Contains(out.String(), hints.DamagedPath()) {
		t.Fatalf("keeps --all over a damaged record: exit %d, %q, %q", code, out.String(), errs.String())
	}
}

// lockedBuffer is a buffer a command writes while the test reads it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// pickRun is one oni pick running until its click.
type pickRun struct {
	out, errs lockedBuffer
	code      chan int
}

func startPick(t *testing.T, stdin io.Reader, args ...string) *pickRun {
	t.Helper()
	r := &pickRun{code: make(chan int, 1)}
	go func() { r.code <- runOniPick(args, stdin, &r.out, &r.errs) }()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(r.errs.String(), "Click the spot to ask about.") {
		if time.Now().After(deadline) {
			t.Fatalf("the pick never armed: %q", r.errs.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
	return r
}

func (r *pickRun) exit(t *testing.T) int {
	t.Helper()
	select {
	case code := <-r.code:
		return code
	case <-time.After(10 * time.Second):
		t.Fatalf("the pick never ended: %q", r.errs.String())
		return -1
	}
}

// clickPick is the presenter's one click on the packing list.
func clickPick(t *testing.T) protocol.OniAskResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := client.Dial(ctx, client.Options{Client: protocol.ClientSurface})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var res protocol.OniAskResult
	if err := conn.Call(ctx, protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickPoint, Output: "DP-1", X: 700, Y: 300}, &res); err != nil {
		t.Fatal(err)
	}
	return res
}

// oni pick keeps its connection after arming and answers the click's
// picture on stdin exactly as oni ask does: the same question on stderr,
// end of input a no with nothing sent, y the answer printed and the preview
// written; a cancel from elsewhere ends the wait with nothing sent.
func TestOniPickWaitsForTheClickAndAnswersItsConsentOnStdin(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOniGuide)
	asks := []string{"960 × 620 pixels", "through toplevel", "It goes to openrouter/qwen/qwen2.5-vl-72b-instruct and leaves this computer.", "Allow once?"}

	// end of input is a no
	pick := startPick(t, strings.NewReader(""))
	if clicked := clickPick(t); clicked.Status != protocol.OniAskConsent || clicked.Consent == nil {
		t.Fatalf("the click = %+v", clicked)
	}
	if code := pick.exit(t); code != 1 || pick.out.String() != "" || !strings.Contains(pick.errs.String(), "Cancelled. Nothing was sent.") {
		t.Fatalf("end of input: exit %d, stdout %q, stderr %q", code, pick.out.String(), pick.errs.String())
	}
	for _, want := range asks {
		if !strings.Contains(pick.errs.String(), want) {
			t.Fatalf("stderr lacks %q:\n%s", want, pick.errs.String())
		}
	}

	// y sends it, still asked: the no allowed nothing
	preview := filepath.Join(t.TempDir(), "preview.jpg")
	stdin, answer := io.Pipe()
	defer answer.Close()
	pick = startPick(t, stdin, "--text", "what is this line?", "--preview", preview, "--json")
	clickPick(t)
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(pick.errs.String(), "Allow once?") {
		if time.Now().After(deadline) {
			t.Fatalf("the pick never asked: %q", pick.errs.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := answer.Write([]byte("y\n")); err != nil {
		t.Fatal(err)
	}
	if code := pick.exit(t); code != 0 {
		t.Fatalf("pick exited %d: %s", code, pick.errs.String())
	}
	var answered protocol.OniAskResult
	if err := json.Unmarshal([]byte(pick.out.String()), &answered); err != nil || answered.Status != protocol.OniAskAnswered || answered.Answer == nil || answered.Answer.Mode != protocol.OniModeSelect || answered.Answer.Text == "" {
		t.Fatalf("--json printed %q: %v", pick.out.String(), err)
	}
	if raw, err := os.ReadFile(preview); err != nil {
		t.Fatal(err)
	} else if thumb, err := jpeg.Decode(bytes.NewReader(raw)); err != nil || thumb.Bounds().Dx() != 480 {
		t.Fatalf("the preview file is not the thumbnail shown: %v", err)
	}

	// a cancel ends the wait
	pick = startPick(t, &untouched{})
	var out, errs bytes.Buffer
	if code := runOniPick([]string{"cancel"}, &untouched{}, &out, &errs); code != 0 || out.String() != "Nothing waits for a click any more.\n" {
		t.Fatalf("pick cancel: exit %d, %q", code, out.String())
	}
	if code := pick.exit(t); code != 1 || pick.out.String() != "" || !strings.Contains(pick.errs.String(), "No spot was picked") {
		t.Fatalf("the cancelled pick: exit %d, stdout %q, stderr %q", code, pick.out.String(), pick.errs.String())
	}
	for _, args := range [][]string{{"oni", "pick", "cancel", "--preview", "x.jpg"}, {"oni", "pick", "--preview"}, {"oni", "pick", "now"}} {
		if code := run(args); code != 2 {
			t.Fatalf("%v exited %d, want the usage", args, code)
		}
	}
}
