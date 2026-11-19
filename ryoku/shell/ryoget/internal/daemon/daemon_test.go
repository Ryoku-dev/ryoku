package daemon

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoget/internal/engine"
	"ryoget/internal/proto"
)

func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "ryoget-daemon-test-")
	if err != nil {
		panic(err)
	}
	for _, k := range []string{"XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_RUNTIME_DIR", "XDG_DOWNLOAD_DIR"} {
		os.Setenv(k, filepath.Join(root, strings.ToLower(k)))
	}
	os.Setenv("HOME", root)
	// no session bus: notifications fail quietly instead of reaching the desktop
	os.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(root, "no-bus"))
	os.Unsetenv("RYOGET_IDLE_SECONDS")
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}

type running struct {
	opts Options
	dir  string
	errc chan error
	stop context.CancelFunc
}

func startDaemon(t *testing.T, idle time.Duration) *running {
	t.Helper()
	root := t.TempDir()
	dl := filepath.Join(root, "dl")
	conf := filepath.Join(root, "downloads.json")
	b, _ := json.Marshal(map[string]any{"dir": dl})
	os.WriteFile(conf, b, 0o600)
	r := &running{dir: dl, errc: make(chan error, 1), opts: Options{
		Socket:    filepath.Join(root, "run", "ryoget.sock"),
		Lock:      filepath.Join(root, "run", "ryoget.lock"),
		IdleAfter: idle,
		Engine:    engine.Options{StateFile: filepath.Join(root, "state", "tasks.json"), ConfigFile: conf},
	}}
	ctx, cancel := context.WithCancel(context.Background())
	r.stop = cancel
	go func() { r.errc <- Run(ctx, r.opts) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if c, err := net.Dial("unix", r.opts.Socket); err == nil {
			c.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("daemon did not listen")
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Cleanup(func() {
		cancel()
		select {
		case <-r.errc:
		case <-time.After(10 * time.Second):
			t.Error("daemon did not stop")
		}
	})
	return r
}

func dial(t *testing.T, r *running) *Client {
	t.Helper()
	c, err := Dial(context.Background(), r.opts.Socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestProtocolRoundTrip(t *testing.T) {
	data := make([]byte, 2<<20)
	rand.Read(data)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"1"`)
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	r := startDaemon(t, time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	c := dial(t, r)
	h, err := c.Hello(ctx, "bar")
	if err != nil {
		t.Fatal(err)
	}
	if h.Version != 1 || h.Settings.MaxActive != 3 || h.Settings.Dir != r.dir || len(h.Tasks) != 0 || len(h.Stats.History) != 60 {
		t.Fatalf("hello = %+v", h)
	}

	var task proto.Task
	if err := c.Call(ctx, "add", proto.AddParams{URL: srv.URL + "/file.bin"}, &task); err != nil {
		t.Fatal(err)
	}
	if task.ID == "" || (task.State != proto.StateQueued && task.State != proto.StateResolving) || task.Dir != r.dir {
		t.Fatalf("add = %+v", task)
	}
	var final proto.Task
	sawStats := false
	for final.State != proto.StateDone {
		select {
		case ev, ok := <-c.Events():
			if !ok {
				t.Fatal("connection closed")
			}
			switch ev.Name {
			case "task":
				var tk proto.Task
				json.Unmarshal(ev.Data, &tk)
				if tk.ID == task.ID {
					final = tk
					if tk.State == proto.StateError {
						t.Fatalf("task failed: %+v", tk.Error)
					}
				}
			case "stats":
				sawStats = true
			}
		case <-ctx.Done():
			t.Fatalf("no done event; last %+v", final)
		}
	}
	if got, _ := os.ReadFile(final.Path); !bytes.Equal(got, data) {
		t.Fatal("downloaded content differs")
	}
	waitEvent(t, ctx, c, "stats", &sawStats)

	var s proto.Settings
	if err := c.Call(ctx, "settings.set", map[string]any{"patch": map[string]any{"maxActive": 2, "downLimit": 512}}, &s); err != nil {
		t.Fatal(err)
	}
	if s.MaxActive != 2 || s.DownLimit != 512 {
		t.Fatalf("settings.set = %+v", s)
	}
	seen := false
	waitEvent(t, ctx, c, "settings", &seen)
	if b, _ := os.ReadFile(r.opts.Engine.ConfigFile); !strings.Contains(string(b), `"maxActive": 2`) {
		t.Fatalf("settings file = %s", b)
	}
	var got proto.Settings
	if err := c.Call(ctx, "settings.get", nil, &got); err != nil || got.MaxActive != 2 {
		t.Fatalf("settings.get = %+v, %v", got, err)
	}

	var pe *proto.Error
	if err := c.Call(ctx, "warp", nil, nil); !errors.As(err, &pe) || pe.Code != proto.CodeInvalid {
		t.Fatalf("unknown method err = %v", err)
	}
	if err := c.Call(ctx, "pause", proto.IDParams{ID: "nope"}, nil); !errors.As(err, &pe) || pe.Code != proto.CodeNotFound {
		t.Fatalf("missing id err = %v", err)
	}
	if err := c.Call(ctx, "remove", map[string]any{"id": task.ID}, nil); err != nil {
		t.Fatal(err)
	}
	seen = false
	waitEvent(t, ctx, c, "removed", &seen)

	// a second daemon must refuse while this one holds the lock
	if err := Run(context.Background(), r.opts); !errors.Is(err, ErrRunning) {
		t.Fatalf("second daemon err = %v", err)
	}

	// oversized frames end the connection rather than the daemon
	raw, _ := net.Dial("unix", r.opts.Socket)
	defer raw.Close()
	raw.Write(bytes.Repeat([]byte("x"), proto.MaxFrame+10))
	raw.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 64)
	if _, err := raw.Read(buf); err == nil {
		t.Fatal("oversized frame answered")
	}
	if _, err := c.Hello(ctx, "cli"); err != nil {
		t.Fatal("daemon unhealthy after oversized frame:", err)
	}
}

func waitEvent(t *testing.T, ctx context.Context, c *Client, name string, seen *bool) {
	t.Helper()
	for !*seen {
		select {
		case ev, ok := <-c.Events():
			if !ok {
				t.Fatal("connection closed")
			}
			if ev.Name == name {
				*seen = true
			}
		case <-ctx.Done():
			t.Fatalf("no %s event", name)
		}
	}
}

func TestIdleExit(t *testing.T) {
	r := startDaemon(t, 300*time.Millisecond)
	c := dial(t, r)
	if _, err := c.Hello(context.Background(), "cli"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-r.errc:
		t.Fatalf("exited with a client connected: %v", err)
	case <-time.After(800 * time.Millisecond):
	}
	c.Close()
	select {
	case err := <-r.errc:
		if err != nil {
			t.Fatal(err)
		}
		r.errc <- nil
	case <-time.After(5 * time.Second):
		t.Fatal("daemon did not exit when idle")
	}
	if _, err := os.Stat(r.opts.Socket); !os.IsNotExist(err) {
		t.Fatal("socket left behind")
	}
}

func TestIdleSecondsFromEnv(t *testing.T) {
	t.Setenv("RYOGET_IDLE_SECONDS", "0.5")
	if got := idleFromEnv(); got != 500*time.Millisecond {
		t.Fatalf("idle = %v", got)
	}
}

func TestTaskHistory(t *testing.T) {
	data := make([]byte, 1<<20)
	rand.Read(data)
	gate := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"h"`)
		if r.Method == http.MethodGet && r.Header.Get("Range") != "bytes=0-0" {
			// the body waits for the gate, so the task stays active while history is read
			select {
			case <-gate:
			case <-r.Context().Done():
				return
			}
		}
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	r := startDaemon(t, time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c := dial(t, r)

	var task proto.Task
	if err := c.Call(ctx, "add", proto.AddParams{URL: srv.URL + "/h.bin"}, &task); err != nil {
		t.Fatal(err)
	}
	type history struct {
		Down []int64 `json:"down"`
		Up   []int64 `json:"up"`
	}
	var h history
	deadline := time.Now().Add(10 * time.Second)
	for len(h.Down) < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("no samples while active: %+v", h)
		}
		time.Sleep(200 * time.Millisecond)
		if err := c.Call(ctx, "history", proto.IDParams{ID: task.ID}, &h); err != nil {
			t.Fatal(err)
		}
	}
	if len(h.Down) > 60 || len(h.Up) != len(h.Down) {
		t.Fatalf("history = %+v", h)
	}

	close(gate)
	deadline = time.Now().Add(10 * time.Second)
	for {
		var tk proto.Task
		var hello proto.Hello
		if err := c.Call(ctx, "hello", map[string]any{"client": "cli", "version": 1}, &hello); err != nil {
			t.Fatal(err)
		}
		for _, x := range hello.Tasks {
			if x.ID == task.ID {
				tk = x
			}
		}
		if tk.State == proto.StateDone {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("task stuck: %+v", tk)
		}
		time.Sleep(50 * time.Millisecond)
	}
	var raw json.RawMessage
	if err := c.Call(ctx, "history", proto.IDParams{ID: task.ID}, &raw); err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"down":[],"up":[]}` {
		t.Fatalf("history after done = %s", raw)
	}
	var pe *proto.Error
	if err := c.Call(ctx, "history", proto.IDParams{ID: "nope"}, nil); !errors.As(err, &pe) || pe.Code != proto.CodeNotFound {
		t.Fatalf("unknown id err = %v", err)
	}
}

func TestLinksAndAddMany(t *testing.T) {
	data := []byte("payload")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"m"`)
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	r := startDaemon(t, time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c := dial(t, r)

	var lr proto.Links
	text := "see " + srv.URL + "/a.bin?utm_source=x, and <" + srv.URL + "/a.bin> plus " + srv.URL + "/b.bin."
	if err := c.Call(ctx, "links", map[string]string{"text": text}, &lr); err != nil {
		t.Fatal(err)
	}
	if lr.Total != 3 || lr.Duplicates != 1 || lr.Cleaned != 2 || len(lr.Links) != 3 {
		t.Fatalf("links = %+v", lr)
	}
	if lr.Links[0].URL != srv.URL+"/a.bin" || !lr.Links[1].Duplicate || lr.Links[2].URL != srv.URL+"/b.bin" {
		t.Fatalf("links = %+v", lr.Links)
	}

	var res proto.AddManyResult
	items := []proto.AddManyItem{{URL: srv.URL + "/a.bin", Title: "Spring - Blender Open Movie"}, {URL: "ftp://nope.test/x"}, {URL: srv.URL + "/b.bin", Mode: "auto"}}
	if err := c.Call(ctx, "addMany", map[string]any{"items": items, "paused": true}, &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Tasks) != 2 || len(res.Errors) != 1 || res.Errors[0].Index != 1 || res.Errors[0].Code != proto.CodeInvalid {
		t.Fatalf("addMany = %+v", res)
	}
	for _, tk := range res.Tasks {
		if tk.State != proto.StatePaused || tk.Dir != r.dir {
			t.Fatalf("task = %+v", tk)
		}
	}
	if res.Tasks[0].Title != "Spring - Blender Open Movie" || res.Tasks[1].Title != "b.bin" {
		t.Fatalf("titles = %q, %q", res.Tasks[0].Title, res.Tasks[1].Title)
	}
	// a paused task keeps the client's title, newlines folded, in later snapshots too
	var one proto.Task
	if err := c.Call(ctx, "add", proto.AddParams{URL: "https://www.youtube.com/watch?v=WhWc3b3KhnY", Title: "Spring -\n Blender Open Movie", Paused: true}, &one); err != nil {
		t.Fatal(err)
	}
	var h proto.Hello
	if err := c.Call(ctx, "hello", map[string]any{"client": "cli", "version": 1}, &h); err != nil {
		t.Fatal(err)
	}
	for _, tk := range h.Tasks {
		if tk.ID == one.ID && (tk.Title != "Spring - Blender Open Movie" || tk.State != proto.StatePaused) {
			t.Fatalf("paused task = %+v", tk)
		}
	}

	var raw json.RawMessage
	if err := c.Call(ctx, "addMany", map[string]any{"items": []proto.AddManyItem{{URL: "nope"}}}, &raw); err != nil {
		t.Fatal(err)
	}
	var all proto.AddManyResult
	json.Unmarshal(raw, &all)
	if !strings.Contains(string(raw), `"tasks":[]`) || len(all.Errors) != 1 {
		t.Fatalf("all-failed addMany = %s", raw)
	}
}
