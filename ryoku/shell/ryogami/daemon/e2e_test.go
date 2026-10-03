package main

// Wiring proof against the real binary, ported from the Rust daemon's e2e
// suite: the socket serves the wallpaper topic, verb lines, JSON requests with
// full payloads, and pushed events after a JSON subscribe. Headless
// (RYOGAMI_HEADLESS=1) so no quickshell is spawned.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type testDaemon struct {
	t    *testing.T
	cmd  *exec.Cmd
	sock string
	root string
}

func startDaemon(t *testing.T) *testDaemon {
	t.Helper()
	root := t.TempDir()
	runtime := filepath.Join(root, "run")
	if err := os.MkdirAll(runtime, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "ryogami-test-bin")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}
	cmd := exec.Command(bin, "daemon")
	cmd.Env = append(os.Environ(),
		"RYOGAMI_HEADLESS=1",
		"HOME="+root,
		"XDG_RUNTIME_DIR="+runtime,
		"XDG_CONFIG_HOME="+filepath.Join(root, "config"),
		"XDG_CACHE_HOME="+filepath.Join(root, "cache"),
		"XDG_STATE_HOME="+filepath.Join(root, ".local", "state"),
		"XDG_DATA_HOME="+filepath.Join(root, ".local", "share"),
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	d := &testDaemon{t: t, cmd: cmd, sock: filepath.Join(runtime, "ryogami.sock"), root: root}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := os.Stat(d.sock); err == nil {
			return d
		}
		if time.Now().After(deadline) {
			t.Fatal("daemon socket never appeared")
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func (d *testDaemon) connect() net.Conn {
	d.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err := net.Dial("unix", d.sock)
		if err == nil {
			_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
			return conn
		}
		if time.Now().After(deadline) {
			d.t.Fatalf("connect: %v", err)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func (d *testDaemon) send(line string) string {
	d.t.Helper()
	conn := d.connect()
	defer conn.Close()
	fmt.Fprintf(conn, "%s\n", line)
	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		d.t.Fatalf("reply: %v", err)
	}
	return strings.TrimSpace(reply)
}

func readJSONLine(t *testing.T, r *bufio.Reader) map[string]interface{} {
	t.Helper()
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var v map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &v); err != nil {
		t.Fatalf("not JSON: %v: %q", err, line)
	}
	return v
}

func writeE2EPNG(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
}

func TestE2EWallpaperTopicPublishesFrameOnSet(t *testing.T) {
	d := startDaemon(t)
	sub := d.connect()
	defer sub.Close()
	fmt.Fprintln(sub, "subscribe wallpaper")
	r := bufio.NewReader(sub)
	initial := readJSONLine(t, r)
	def := initial["default"].(map[string]interface{})
	initRev := def["revision"].(float64)

	img := filepath.Join(d.root, "wall.png")
	writeE2EPNG(t, img)
	if got := d.send("wallpaper set " + img); got != "ok" {
		t.Fatalf("set reply: %s", got)
	}
	frame := readJSONLine(t, r)
	def = frame["default"].(map[string]interface{})
	if def["path"] != img {
		t.Fatalf("frame path = %v", def["path"])
	}
	if def["revision"].(float64) <= initRev {
		t.Fatal("revision not bumped")
	}
	for _, key := range []string{"path", "revision", "fit", "live", "transition", "depth", "depthRev"} {
		if _, has := def[key]; !has {
			t.Fatalf("entry missing contract key: %s", key)
		}
	}
	if _, has := frame["outputs"]; !has {
		t.Fatal("frame missing outputs map")
	}
}

func TestE2ESubscribeToUnknownTopicErrors(t *testing.T) {
	d := startDaemon(t)
	if got := d.send("subscribe nope"); !strings.HasPrefix(got, "err") {
		t.Fatalf("reply: %s", got)
	}
}

func TestE2EDepthSetAndClearFoldIntoFrame(t *testing.T) {
	d := startDaemon(t)
	img := filepath.Join(d.root, "wall.png")
	writeE2EPNG(t, img)
	if got := d.send("wallpaper set " + img); got != "ok" {
		t.Fatalf("set reply: %s", got)
	}
	body := fmt.Sprintf(`{"screen":"","source":%q,"out":"/d/wall-depth.png","rev":7}`, img)
	if got := d.send("depth set " + body); got != "ok" {
		t.Fatalf("depth set reply: %s", got)
	}
	sub := d.connect()
	defer sub.Close()
	fmt.Fprintln(sub, "subscribe wallpaper")
	frame := readJSONLine(t, bufio.NewReader(sub))
	def := frame["default"].(map[string]interface{})
	if def["depth"] != "/d/wall-depth.png" || def["depthRev"].(float64) != 7 {
		t.Fatalf("depth not folded: %v %v", def["depth"], def["depthRev"])
	}
	// A cutout for a superseded wallpaper is dropped.
	stale := `{"screen":"","source":"/w/old.png","out":"/d/old.png","rev":9}`
	if got := d.send("depth set " + stale); got != "ok" {
		t.Fatalf("stale reply: %s", got)
	}
	if got := d.send("depth clear"); got != "ok" {
		t.Fatalf("clear reply: %s", got)
	}
}

func TestE2EJSONRequestsAndEventsShareOneConnection(t *testing.T) {
	d := startDaemon(t)
	img := filepath.Join(d.root, "Pictures", "Wallpapers", "a.png")
	writeE2EPNG(t, img)

	conn := d.connect()
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	r := bufio.NewReader(conn)

	fmt.Fprintln(conn, `{"method":"status","id":7}`)
	status := readJSONLine(t, r)
	if status["id"].(float64) != 7 {
		t.Fatalf("id = %v", status["id"])
	}
	if _, has := status["result"]; !has {
		t.Fatal("status carries no result")
	}

	fmt.Fprintln(conn, `{"method":"wall.list","id":8}`)
	list := readJSONLine(t, r)
	if list["id"].(float64) != 8 {
		t.Fatal("second request on the same connection failed")
	}
	if _, has := list["result"].(map[string]interface{})["wallpapers"]; !has {
		t.Fatal("wall.list returns no wallpapers array")
	}

	fmt.Fprintln(conn, `{"method":"subscribe","params":{"prefixes":["ryogami."]},"id":9}`)
	sub := readJSONLine(t, r)
	if _, has := sub["result"].(map[string]interface{})["subscribed"]; !has {
		t.Fatal("subscribe not acked")
	}

	if got := d.send("wallpaper set " + img); got != "ok" {
		t.Fatalf("set reply: %s", got)
	}
	ev := readJSONLine(t, r)
	name, _ := ev["event"].(string)
	if !strings.HasPrefix(name, "ryogami.wall.") {
		t.Fatalf("pushed line is not a ryogami event: %v", ev)
	}
}

func TestE2EWallApplyPublishesTopicFrame(t *testing.T) {
	d := startDaemon(t)
	img := filepath.Join(d.root, "Pictures", "Wallpapers", "a.png")
	writeE2EPNG(t, img)

	sub := d.connect()
	defer sub.Close()
	fmt.Fprintln(sub, "subscribe wallpaper")
	r := bufio.NewReader(sub)
	readJSONLine(t, r) // retained empty frame

	req := fmt.Sprintf(`{"method":"wall.apply","params":{"path":%q,"type":"static"},"id":11}`, img)
	reply := d.send(req)
	if !strings.Contains(reply, `"applied"`) {
		t.Fatalf("apply replied: %s", reply)
	}
	frame := readJSONLine(t, r)
	if frame["default"].(map[string]interface{})["path"] != img {
		t.Fatal("apply did not publish the frame")
	}
}

func TestE2EWallpaperNextAdvances(t *testing.T) {
	d := startDaemon(t)
	a := filepath.Join(d.root, "Pictures", "Wallpapers", "a.png")
	b := filepath.Join(d.root, "Pictures", "Wallpapers", "b.png")
	writeE2EPNG(t, a)
	writeE2EPNG(t, b)
	if got := d.send(`{"method":"wall.cache_rebuild","id":1}`); !strings.Contains(got, "started") {
		t.Fatalf("rebuild reply: %s", got)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		reply := d.send(`{"method":"wall.list","id":2}`)
		var resp map[string]interface{}
		_ = json.Unmarshal([]byte(reply), &resp)
		if res, okRes := resp["result"].(map[string]interface{}); okRes && res["count"].(float64) >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("catalog never scanned: %s", reply)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if got := d.send("wallpaper set " + a); got != "ok" {
		t.Fatalf("set reply: %s", got)
	}
	if got := d.send("wallpaper next"); got != "ok" {
		t.Fatalf("next reply: %s", got)
	}
	sub := d.connect()
	defer sub.Close()
	fmt.Fprintln(sub, "subscribe wallpaper")
	frame := readJSONLine(t, bufio.NewReader(sub))
	if got := frame["default"].(map[string]interface{})["path"]; got != b {
		t.Fatalf("next advanced to %v, want %s", got, b)
	}
}

func TestE2EFavouriteSurvivesRescan(t *testing.T) {
	d := startDaemon(t)
	img := filepath.Join(d.root, "Pictures", "Wallpapers", "a.png")
	writeE2EPNG(t, img)
	d.send(`{"method":"wall.cache_rebuild","id":1}`)
	deadline := time.Now().Add(10 * time.Second)
	var key string
	for {
		reply := d.send(`{"method":"wall.list","id":2}`)
		var resp map[string]interface{}
		_ = json.Unmarshal([]byte(reply), &resp)
		if res, okRes := resp["result"].(map[string]interface{}); okRes {
			if walls, okW := res["wallpapers"].([]interface{}); okW && len(walls) == 1 {
				key = walls[0].(map[string]interface{})["key"].(string)
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("catalog never scanned")
		}
		time.Sleep(100 * time.Millisecond)
	}
	req := fmt.Sprintf(`{"method":"wall.set_favourite","params":{"key":%q,"favourite":true},"id":3}`, key)
	if got := d.send(req); !strings.Contains(got, `"ok"`) {
		t.Fatalf("favourite reply: %s", got)
	}
	reply := d.send(fmt.Sprintf(`{"method":"wall.list","params":{"favourites":true},"id":4}`))
	var resp map[string]interface{}
	_ = json.Unmarshal([]byte(reply), &resp)
	res := resp["result"].(map[string]interface{})
	if res["count"].(float64) != 1 {
		t.Fatalf("favourites filter returned %v", res["count"])
	}
}

// The picker's in-memory Library only updates a wallpaper's heart when the
// daemon echoes the mutated entry over ryogami.wall.cached; without it the
// favourite toggle looks dead until a full rescan.
func TestE2ESetFavouriteBroadcastsCached(t *testing.T) {
	d := startDaemon(t)
	img := filepath.Join(d.root, "Pictures", "Wallpapers", "a.png")
	writeE2EPNG(t, img)
	d.send(`{"method":"wall.cache_rebuild","id":1}`)
	deadline := time.Now().Add(10 * time.Second)
	var key string
	for {
		reply := d.send(`{"method":"wall.list","id":2}`)
		var resp map[string]interface{}
		_ = json.Unmarshal([]byte(reply), &resp)
		if res, okRes := resp["result"].(map[string]interface{}); okRes {
			if walls, okW := res["wallpapers"].([]interface{}); okW && len(walls) == 1 {
				key = walls[0].(map[string]interface{})["key"].(string)
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("catalog never scanned")
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Subscribe only after the scan settled so the scan's own cached frames
	// are already flushed and the next one is the favourite echo.
	sub := d.connect()
	defer sub.Close()
	_ = sub.SetDeadline(time.Now().Add(15 * time.Second))
	r := bufio.NewReader(sub)
	fmt.Fprintln(sub, `{"method":"subscribe","params":{"prefixes":["ryogami.wall."]},"id":9}`)
	readJSONLine(t, r) // subscribe ack

	req := fmt.Sprintf(`{"method":"wall.set_favourite","params":{"key":%q,"favourite":true},"id":3}`, key)
	if got := d.send(req); !strings.Contains(got, `"ok"`) {
		t.Fatalf("favourite reply: %s", got)
	}

	found := false
	for i := 0; i < 40 && !found; i++ {
		ev := readJSONLine(t, r)
		if ev["event"] != "ryogami.wall.cached" {
			continue
		}
		data, _ := ev["data"].(map[string]interface{})
		if data == nil {
			t.Fatalf("cached event carried no entry: %v", ev)
		}
		if data["key"] != key {
			continue
		}
		if fav, _ := data["favourite"].(float64); fav != 1 {
			t.Fatalf("cached entry favourite = %v, want 1", data["favourite"])
		}
		found = true
	}
	if !found {
		t.Fatal("wall.set_favourite did not broadcast ryogami.wall.cached for the toggled key")
	}
}

// Regression: ryogami owns the wallpaper now, but rice capture, the overview
// backdrop and the Super+W on-air dot still read ~/.local/state/ryoku-wallpaper.
// A `wall.apply` must write that legacy state file with the applied path, or
// those readers see a stale wallpaper (a saved rice grabs the wrong wall).
func TestE2EWallApplyWritesLegacyState(t *testing.T) {
	d := startDaemon(t)
	img := filepath.Join(d.root, "Pictures", "Wallpapers", "b.png")
	writeE2EPNG(t, img)

	reply := d.send(fmt.Sprintf(`{"method":"wall.apply","params":{"path":%q,"type":"static"},"id":21}`, img))
	if !strings.Contains(reply, `"applied"`) {
		t.Fatalf("apply replied: %s", reply)
	}
	statePath := filepath.Join(d.root, ".local", "state", "ryoku-wallpaper")
	deadline := time.Now().Add(5 * time.Second)
	var got string
	for {
		if b, err := os.ReadFile(statePath); err == nil {
			if got = strings.TrimSpace(string(b)); got != "" {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("state file %s never held the applied wallpaper (got %q)", statePath, got)
		}
		time.Sleep(25 * time.Millisecond)
	}
	if got != img {
		t.Fatalf("state file = %q, want %q", got, img)
	}
}

// TestE2EDaemonDoesNotPreloadPicker keeps the wallpaper service cheap while idle.
// The picker is a demand-owned UI process: login must not spawn it, but the first
// wallpaper ui request must still cold-start it.
func TestE2EDaemonDoesNotPreloadPicker(t *testing.T) {
	root := t.TempDir()
	runtime := filepath.Join(root, "run")
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(runtime, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}

	bin := filepath.Join(root, "ryogami-test-bin")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}

	marker := filepath.Join(root, "picker-launches")
	fakeQuickShell := filepath.Join(binDir, "quickshell")
	body := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + marker + "\n"
	if err := os.WriteFile(fakeQuickShell, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	qml := filepath.Join(root, "shell.qml")
	if err := os.WriteFile(qml, []byte("// picker probe\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	env := append(os.Environ(),
		"HOME="+filepath.Join(root, "home"),
		"XDG_RUNTIME_DIR="+runtime,
		"XDG_CONFIG_HOME="+filepath.Join(root, "config"),
		"XDG_CACHE_HOME="+filepath.Join(root, "cache"),
		"XDG_STATE_HOME="+filepath.Join(root, "state"),
		"XDG_DATA_HOME="+filepath.Join(root, "data"),
		"RYOGAMI_SHELL_QML="+qml,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)

	cmd := exec.Command(bin, "daemon")
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	sock := filepath.Join(runtime, "ryogami.sock")
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(sock); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("daemon socket never appeared")
		}
		time.Sleep(25 * time.Millisecond)
	}

	time.Sleep(250 * time.Millisecond)
	if b, err := os.ReadFile(marker); err == nil && len(b) != 0 {
		t.Fatalf("picker launched during daemon boot: %q", b)
	} else if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	client := exec.Command(bin, "wallpaper", "ui")
	client.Env = env
	if out, err := client.CombinedOutput(); err != nil {
		t.Fatalf("wallpaper ui: %v: %s", err, out)
	}

	deadline = time.Now().Add(3 * time.Second)
	for {
		if b, err := os.ReadFile(marker); err == nil && len(b) != 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("wallpaper ui did not launch the picker")
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func TestPickerReleaseDefaultIsBounded(t *testing.T) {
	spec, ok := specFor("performance.releaseAfterHideSeconds")
	if !ok {
		t.Fatal("releaseAfterHideSeconds setting missing")
	}
	if got, ok := spec.Default.(float64); !ok || got <= 0 {
		t.Fatalf("releaseAfterHideSeconds default = %#v, want a bounded nonzero delay", spec.Default)
	}
}
