//go:build handsengine

package page_test

import (
	"bufio"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/hands/chrome"
	"ryoku-rashin/internal/hands/page"
	"ryoku-rashin/internal/oni/hands/consent"
)

const form = `<!doctype html><html><head><title>Contact</title></head><body>
<form action="/submit" method="post">
<label>Name <input id="name" name="name" type="text"></label>
<label>Message <textarea id="message" name="message"></textarea></label>
<button type="submit" id="send">Send</button>
</form>
<script>
const f = document.forms[0];
f.addEventListener("submit", (e) => e.preventDefault());
setInterval(() => { document.getElementById("send").click(); f.requestSubmit(); }, 50);
</script></body></html>`

// TestRealBrowserFillsOverThePipe runs only with -tags handsengine and an
// installed Chromium-family browser: headless, a private /dev/shm profile,
// a local file page.
func TestRealBrowserFillsOverThePipe(t *testing.T) {
	exec := engineBrowser(t)
	base := "/dev/shm"
	if _, err := os.Stat(base); err != nil {
		base = ""
	}
	dir, err := os.MkdirTemp(base, "hyakki-hands-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	pagePath := filepath.Join(dir, "form.html")
	if err := os.WriteFile(pagePath, []byte(form), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	b, err := chrome.Launch(ctx, chrome.Options{
		Exec: []string{exec}, ProfileDir: filepath.Join(dir, chrome.ProfileName), Class: "hyakki-hands-engine",
		StartURL: "file://" + pagePath, Headless: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("started %q", b.Argv())
	// Right after Start the child may still be inside exec: read until the
	// browser's own argv shows.
	var cmdline []byte
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		cmdline, err = os.ReadFile("/proc/" + strconv.Itoa(b.PID()) + "/cmdline")
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(string(cmdline), b.Argv()[0]+"\x00") || time.Now().After(deadline) {
			break
		}
	}
	if !strings.Contains(string(cmdline), "\x00--remote-debugging-pipe\x00") || strings.Contains(string(cmdline), "remote-debugging-port") {
		t.Fatalf("cmdline %q", cmdline)
	}

	s, err := page.Open(ctx, b.Conn(), page.Limits{MaxControls: 200, MaxNameRunes: 80, MaxTextRunes: 6000, MaxFillRunes: 2000, Act: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	var tab page.Tab
	for tab.ID == "" {
		tabs, err := s.Tabs(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, tb := range tabs {
			if tb.Origin == "file://" {
				tab = tb
			}
		}
		if tab.ID == "" {
			select {
			case <-ctx.Done():
				t.Fatal("the page's tab never appeared")
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
	if _, err := s.Authorize(ctx, tab.ID); err != nil {
		t.Fatal(err)
	}
	var snap page.Snapshot
	for {
		snap, err = s.Page(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(snap.Controls) >= 3 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("controls %+v", snap.Controls)
		case <-time.After(100 * time.Millisecond):
		}
	}
	t.Logf("controls %+v, picture %d bytes", snap.Controls, len(snap.Picture))
	find := func(name string) string {
		for _, c := range snap.Controls {
			if c.Name == name {
				return c.ID
			}
		}
		t.Fatalf("no %q in %+v", name, snap.Controls)
		return ""
	}
	res, err := s.Act(ctx, page.Act{Verb: consent.VerbFill, Control: find("Name"), Text: "Ada"})
	if err != nil || !res.Done || !res.ReadBack {
		t.Fatalf("fill %+v, %v", res, err)
	}
	res, err = s.Act(ctx, page.Act{Verb: consent.VerbFill, Control: find("Message"), Text: "hello\nthere"})
	if err != nil || !res.ReadBack {
		t.Fatalf("fill message %+v, %v", res, err)
	}
	res, err = s.Act(ctx, page.Act{Verb: consent.VerbClick, Control: find("Send")})
	if !errors.Is(err, page.ErrNotSafe) || res.Class != consent.Handoff || res.Done {
		t.Fatalf("send %+v, %v", res, err)
	}
	// The page clicks Send and submits its form from script every 50 ms:
	// none of that is the person's press.
	if err := s.Await(ctx, find("Send")); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-s.Events():
		t.Fatalf("an untrusted press was reported: %+v", e)
	case <-time.After(600 * time.Millisecond):
	}
	// A trusted press (the browser's own input, here a safe click Hands
	// takes on the Name field) is reported once.
	if err := s.Await(ctx, find("Name")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Act(ctx, page.Act{Verb: consent.VerbClick, Control: find("Name")}); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-s.Events():
		if e.Kind != page.Pressed || e.Control != find("Name") {
			t.Fatalf("event %+v", e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a trusted press was not reported")
	}
	if png, err := s.Shot(ctx); err != nil || len(png) < 8 || string(png[1:4]) != "PNG" {
		t.Fatalf("shot %d bytes, %v", len(png), err)
	}

	listening := listeners(t)
	group := groupSockets(t, b.PID())
	for inode := range group {
		if listening[inode] {
			t.Fatalf("the browser listens on a network socket (inode %s)", inode)
		}
	}
	t.Logf("browser group holds %d sockets, %d network listeners on the box, none the browser's", len(group), len(listening))

	s.Close()
	start := time.Now()
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	took := time.Since(start)
	exit := <-b.Wait()
	t.Logf("browser exited %v after %v once the pipe closed", exit, took)
	if took >= chrome.CloseGrace {
		t.Fatalf("the browser did not exit on the pipe's end (took %v)", took)
	}
	// Close killed whatever the browser left in its group; give the
	// namespace's reaper a moment.
	deadline := time.Now().Add(3 * time.Second)
	for left := groupPIDs(b.PID()); len(left) > 0; left = groupPIDs(b.PID()) {
		if time.Now().After(deadline) {
			t.Fatalf("processes left in the browser's group: %v", left)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// listeners are the inodes of listening TCP sockets and bound UDP sockets.
func listeners(t *testing.T) map[string]bool {
	out := map[string]bool{}
	for _, f := range []string{"/proc/net/tcp", "/proc/net/tcp6", "/proc/net/udp", "/proc/net/udp6"} {
		fh, err := os.Open(f)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(fh)
		sc.Scan()
		for sc.Scan() {
			fs := strings.Fields(sc.Text())
			if len(fs) < 10 {
				continue
			}
			if strings.HasPrefix(f, "/proc/net/udp") || fs[3] == "0A" {
				out[fs[9]] = true
			}
		}
		fh.Close()
	}
	return out
}

func groupPIDs(pgid int) []int {
	var out []int
	ents, _ := os.ReadDir("/proc")
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		stat, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue
		}
		s := string(stat)
		fs := strings.Fields(s[strings.LastIndexByte(s, ')')+1:])
		if len(fs) > 2 && fs[0] != "Z" && fs[2] == strconv.Itoa(pgid) {
			out = append(out, pid)
		}
	}
	return out
}

func groupSockets(t *testing.T, pgid int) map[string]bool {
	out := map[string]bool{}
	for _, pid := range groupPIDs(pgid) {
		dir := "/proc/" + strconv.Itoa(pid) + "/fd"
		ents, _ := os.ReadDir(dir)
		for _, e := range ents {
			l, err := os.Readlink(filepath.Join(dir, e.Name()))
			if err == nil && strings.HasPrefix(l, "socket:[") {
				out[strings.TrimSuffix(strings.TrimPrefix(l, "socket:["), "]")] = true
			}
		}
	}
	return out
}

func engineBrowser(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome-stable", "google-chrome", "brave", "brave-browser", "vivaldi-stable", "microsoft-edge-stable"} {
		if _, err := chrome.Command(chrome.Options{Exec: []string{name}, ProfileDir: "/x/" + chrome.ProfileName, Class: "c", StartURL: "about:blank"}); err == nil {
			return name
		}
	}
	t.Skip("no Chromium-family browser installed")
	return ""
}

func engineReading(t *testing.T, body string) page.Snapshot {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "page.html")
	document := "<!doctype html><meta charset=\"utf-8\"><style>body{font:16px/1.4 sans-serif;max-width:760px}</style>" + body
	if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, s := engineSession(t, dir, "file://"+path)
	readCtx, stop := context.WithTimeout(ctx, 3*time.Second)
	defer stop()
	start := time.Now()
	snap, err := s.Page(readCtx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("native page read took %v", time.Since(start))
	return snap
}

func engineSession(t *testing.T, dir, startURL string) (context.Context, *page.Session) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	b, err := chrome.Launch(ctx, chrome.Options{Exec: []string{engineBrowser(t)},
		ProfileDir: filepath.Join(dir, chrome.ProfileName), Class: "hyakki-bounded-page", StartURL: startURL, Headless: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	s, err := page.Open(ctx, b.Conn(), page.Limits{MaxControls: 200, MaxNameRunes: 80, MaxTextRunes: 6000, MaxFillRunes: 2000, Act: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	tabID := ""
	for tabID == "" {
		tabs, err := s.Tabs(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, tab := range tabs {
			if tab.Origin == page.Origin(startURL) {
				tabID = tab.ID
				break
			}
		}
		if tabID == "" {
			select {
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			case <-time.After(50 * time.Millisecond):
			}
		}
	}
	if _, err := s.Authorize(ctx, tabID); err != nil {
		t.Fatal(err)
	}
	return ctx, s
}

func TestRealBrowserDoesNotReadAParsingDocument(t *testing.T) {
	release := make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/result" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><html><head><title>Contact result</title></head><body><p>Preparing delivery</p>"))
		w.(http.Flusher).Flush()
		<-release
		_, _ = w.Write([]byte("<h1>Message sent</h1></body></html>"))
	}))
	t.Cleanup(server.Close)
	ctx, s := engineSession(t, t.TempDir(), server.URL+"/result")
	readCtx, stop := context.WithTimeout(ctx, 3*time.Second)
	defer stop()
	if snap, err := s.Page(readCtx); !errors.Is(err, page.ErrStale) {
		t.Fatalf("a parsing document became a completed reading: text %q, error %v", snap.Text, err)
	}
	close(release)
	for {
		snap, err := s.Page(readCtx)
		if errors.Is(err, page.ErrStale) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(snap.Text, "Message sent") {
			t.Fatalf("the completed document lost its result: %q", snap.Text)
		}
		return
	}
}

func TestRealBrowserBoundsLargeAndManyPages(t *testing.T) {
	t.Run("large text leaf", func(t *testing.T) {
		large := strings.Repeat("long text ", (5<<20)/10+1)[:5<<20]
		snap := engineReading(t, "<label>Name<input></label><p>"+large+"</p>")
		if len(snap.Controls) != 1 || snap.Controls[0].Name != "Name" ||
			!snap.Truncated || !strings.HasPrefix(snap.Text, "Name long text") ||
			len(snap.Text) < 5998 || len(snap.Text) > 6000 {
			t.Fatalf("controls %+v, truncated %v, text bytes %d", snap.Controls, snap.Truncated, len(snap.Text))
		}
	})
	t.Run("many controls", func(t *testing.T) {
		var body strings.Builder
		for i := range 10000 {
			body.WriteString("<button>Item ")
			body.WriteString(strconv.Itoa(i + 1))
			body.WriteString("</button>")
		}
		snap := engineReading(t, body.String())
		if len(snap.Controls) != 200 || snap.Controls[0].Name != "Item 1" ||
			snap.Controls[199].Name != "Item 200" || !snap.Truncated || len(snap.Text) > 6000 {
			t.Fatalf("controls %d, truncated %v, text bytes %d", len(snap.Controls), snap.Truncated, len(snap.Text))
		}
	})
}

func TestRealBrowserTextLeafKeepsRenderedContent(t *testing.T) {
	for _, tc := range []struct {
		name, body, text string
		controls         int
	}{
		{"unicode", "<p>a\u200e b 😀 \uFFFD c</p>", "a b 😀 c", 0},
		{"mixed content", "<p>Before <span><button>Save</button></span> After</p>", "Before Save After", 1},
		{"generated content", "<style>p::before{content:'Generated '}</style><p>Body</p>", "Generated Body", 0},
		{"closed shadow host", `<p id="host">Unused light text</p><script>document.getElementById("host").attachShadow({mode:"closed"}).innerHTML="<button>Shadow action</button>";</script>`, "Shadow action", 1},
		{"rendered transform", `<p style="text-transform:uppercase">Body</p>`, "BODY", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snap := engineReading(t, tc.body)
			if snap.Text != tc.text || len(snap.Controls) != tc.controls || snap.Truncated {
				t.Fatalf("text %q, controls %+v, truncated %v", snap.Text, snap.Controls, snap.Truncated)
			}
		})
	}
}
