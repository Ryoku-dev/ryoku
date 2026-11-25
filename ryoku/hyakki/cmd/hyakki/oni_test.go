package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/daemon"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// realOniDaemon runs a daemon in temporary XDG dirs, on for the test and
// stopped when it ends, once the user characters are in place.
func realOniDaemon(t *testing.T, install func()) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("HYAKKI_SOCKET", "")
	// a short runtime dir keeps the socket path within the Unix limit
	runtime, err := os.MkdirTemp("", "hyakki-oni")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(runtime) })
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	if err := config.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	install()
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- daemon.New(daemon.Deps{Version: "oni-cli-test"}).Run(ctx) }()
	t.Cleanup(func() {
		stop()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("daemon did not stop")
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for {
		dialCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		conn, err := client.Dial(dialCtx, client.Options{Client: protocol.ClientCLI})
		cancel()
		if err == nil {
			conn.Close()
			return
		}
		select {
		case runErr := <-done:
			t.Fatalf("daemon exited before serving: %v", runErr)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("daemon did not listen: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func installCLIOniCharacter(t *testing.T, source, id string, edit func(map[string]any)) {
	t.Helper()
	dir := filepath.Join(paths.CharactersDir(), id)
	if err := os.MkdirAll(paths.CharactersDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(dir, os.DirFS(filepath.Join("..", "..", "internal", "oni", "profile", "testdata", source))); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "profile.json")
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]any
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	p["id"] = id
	if edit != nil {
		edit(p)
	}
	if raw, err = json.Marshal(p); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestOniCharactersJSONPrintsTheReportsOfARealDaemon(t *testing.T) {
	realOniDaemon(t, func() {
		installCLIOniCharacter(t, "testmoth", "testmoth", nil)
		installCLIOniCharacter(t, "broken-capabilities", "hostile", nil)
		installCLIOniCharacter(t, "testmoth", "partial", func(p map[string]any) {
			delete(p["animations"].(map[string]any)["states"].(map[string]any), "listening")
		})
	})

	out, code := captured(t, func() int { return run([]string{"oni", "characters", "--json"}) })
	if code != 0 {
		t.Fatalf("oni characters --json exited %d: %s", code, out)
	}
	var result protocol.OniCharactersResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not the characters result: %v\n%s", err, out)
	}
	if result.Active != protocol.OniDefaultCharacter {
		t.Fatalf("active = %q", result.Active)
	}
	byID := map[string]protocol.OniCharacter{}
	for _, c := range result.Characters {
		byID[c.ID] = c
	}
	if len(result.Characters) != 4 || result.Characters[0].ID != protocol.OniDefaultCharacter || !result.Characters[0].Shipped || !result.Characters[0].Valid {
		t.Fatalf("characters = %+v", result.Characters)
	}
	if moth := byID["testmoth"]; !moth.Valid || len(moth.Fallbacks) != 0 || len(moth.Errors) != 0 {
		t.Fatalf("testmoth = %+v", moth)
	}
	if hostile := byID["hostile"]; hostile.Valid || !(profile.Report{Errors: hostile.Errors}).Has(profile.ReasonAuthorityKey) {
		t.Fatalf("hostile = %+v", hostile)
	}
	want := []protocol.OniFallback{{Slot: "animations.states.listening", Reason: profile.FallbackMissing, Used: profile.UsedStaticPose}}
	if partial := byID["partial"]; !partial.Valid || len(partial.Errors) != 0 || !reflect.DeepEqual(partial.Fallbacks, want) {
		t.Fatalf("partial = %+v, want fallbacks %+v", partial, want)
	}

	out, code = captured(t, func() int { return run([]string{"oni", "status", "--json"}) })
	if code != 0 {
		t.Fatalf("oni status --json exited %d: %s", code, out)
	}
	var status protocol.OniStatus
	if err := json.Unmarshal([]byte(out), &status); err != nil {
		t.Fatalf("output is not the status: %v\n%s", err, out)
	}
	if status.Character != protocol.OniDefaultCharacter || !status.Enabled || status.Active || !reflect.DeepEqual(status.Characters, result.Characters) {
		t.Fatalf("status = %+v", status)
	}
	for _, c := range status.Capabilities.Features {
		if c.Allow {
			t.Fatalf("%s is allowed with Oni off", c.ID)
		}
	}

	if code := run([]string{"oni", "select", "testmoth"}); code != 2 {
		t.Fatalf("the CLI offered to select a character: exit %d", code)
	}
}

type lockedLines struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *lockedLines) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *lockedLines) lines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Split(strings.TrimSuffix(l.buf.String(), "\n"), "\n")
}

func (l *lockedLines) waitFor(t *testing.T, n int) []string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if lines := l.lines(); len(lines) >= n && lines[0] != "" {
			return lines
		}
		if time.Now().After(deadline) {
			t.Fatalf("watch printed %q, want %d lines", l.lines(), n)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestOniWatchPrintsTheFramesOfARealDaemon(t *testing.T) {
	realOniDaemon(t, func() {})
	setOni := func(patch string) {
		t.Helper()
		conn, cancel, err := dial(true)
		if err != nil {
			t.Fatal(err)
		}
		defer cancel()
		defer conn.Close()
		if err := conn.Call(context.Background(), protocol.MSettingsSet, protocol.SettingsSetParams{Patch: json.RawMessage(patch)}, nil); err != nil {
			t.Fatal(err)
		}
	}
	setOni(`{"oni":{"master":true}}`)

	dialCtx, cancelDial := context.WithTimeout(context.Background(), 5*time.Second)
	conn, err := client.Dial(dialCtx, client.Options{Client: protocol.ClientCLI})
	cancelDial()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	out := &lockedLines{}
	ctx, stop := context.WithCancel(context.Background())
	exited := make(chan int, 1)
	go func() { exited <- watchOni(ctx, conn, out) }()

	frame := func(line string) feature.Payload {
		t.Helper()
		p, err := feature.DecodeFrame([]byte(line))
		if err != nil {
			t.Fatalf("watch printed %q: %v", line, err)
		}
		return p
	}
	lines := out.waitFor(t, 5)
	var kinds []string
	for _, line := range lines[:5] {
		kinds = append(kinds, frame(line).Kind())
	}
	if want := []string{feature.KindState, feature.KindCharacter, feature.KindMotion, feature.KindPlacement, feature.KindVoice}; !reflect.DeepEqual(kinds, want) {
		t.Fatalf("watch began with %q, want %q", kinds, want)
	}
	if st := frame(lines[0]).(*feature.StateFrame); st.State != "idle" || st.Reason != "" {
		t.Fatalf("first state = %+v", st)
	}
	if c := frame(lines[1]).(*feature.CharacterFrame); c.ID != protocol.OniDefaultCharacter {
		t.Fatalf("character = %+v", c)
	}

	if code := run([]string{"oni", "hide"}); code != 0 {
		t.Fatalf("oni hide exited %d", code)
	}
	lines = out.waitFor(t, 6)
	if st, ok := frame(lines[5]).(*feature.StateFrame); !ok || st.State != "sleeping" || st.Reason != "hidden" {
		t.Fatalf("hiding printed %q", lines[5])
	}
	setOni(`{"oni":{"master":false}}`)
	lines = out.waitFor(t, 7)
	if st, ok := frame(lines[6]).(*feature.StateFrame); !ok || st.State != "sleeping" || st.Reason != "oni-off" {
		t.Fatalf("turning Oni off printed %q", lines[6])
	}
	if code := run([]string{"oni", "show"}); code != 1 {
		t.Fatalf("oni show with Oni off exited %d", code)
	}
	stored, err := config.Load()
	if err != nil || stored.Oni.Hidden {
		t.Fatalf("oni show left oni.hidden = %v, %v", stored.Oni.Hidden, err)
	}
	stop()
	select {
	case code := <-exited:
		if code != 0 {
			t.Fatalf("watch exited %d", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("watch did not stop")
	}
}
