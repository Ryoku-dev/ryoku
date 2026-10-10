package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// checkApp maps an HTTP answer to the tri-state the dashboard draws: a 2xx is up,
// a 4xx/5xx is warn (the service answered but is unhealthy), and no answer at all
// is down.
func TestCheckApp(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ok.Close()
	if s := checkApp("h", App{Name: "svc", URL: ok.URL}, 3*time.Second); s.State != "up" || s.Code != 200 {
		t.Fatalf("reachable 200: want up/200, got %s/%d", s.State, s.Code)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer bad.Close()
	if s := checkApp("h", App{Name: "svc", URL: bad.URL}, 3*time.Second); s.State != "warn" || s.Code != 503 {
		t.Fatalf("answering 503: want warn/503, got %s/%d", s.State, s.Code)
	}

	gone := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := gone.URL
	gone.Close()
	if s := checkApp("h", App{Name: "svc", URL: url}, time.Second); s.State != "down" {
		t.Fatalf("unreachable: want down, got %s", s.State)
	}
}

// pveGuests turns /cluster/resources into the guest list the panel shows: qemu
// and lxc only (node/storage rows dropped), sorted by vmid, with the API token
// carried in the header, never the URL.
func TestPveGuests(t *testing.T) {
	const body = `{"data":[
		{"vmid":200,"name":"ct","status":"running","node":"pve","type":"lxc","cpu":0.1,"mem":100,"maxmem":1000,"uptime":50},
		{"vmid":100,"name":"vm","status":"stopped","node":"pve","type":"qemu","maxmem":2000},
		{"id":"node/pve","node":"pve","type":"node"},
		{"id":"storage/local","type":"storage"}
	]}`
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
		w.Write([]byte(body))
	}))
	defer srv.Close()

	g, err := pveGuests(PVE{URL: srv.URL, Token: "ryoport@pve!hub=secret"})
	if err != nil {
		t.Fatal(err)
	}
	if len(g) != 2 {
		t.Fatalf("want 2 guests (node+storage dropped), got %d", len(g))
	}
	if g[0].VMID != 100 || g[1].VMID != 200 {
		t.Fatalf("want vmid-sorted 100,200, got %d,%d", g[0].VMID, g[1].VMID)
	}
	if g[0].Type != "qemu" || g[1].Type != "lxc" {
		t.Fatalf("types wrong: %s,%s", g[0].Type, g[1].Type)
	}
	if gotAuth != "PVEAPIToken=ryoport@pve!hub=secret" {
		t.Fatalf("auth header wrong: %q", gotAuth)
	}
	if gotPath != "/api2/json/cluster/resources" {
		t.Fatalf("path wrong: %q", gotPath)
	}
}

// pveAction posts to the guest's status endpoint on its owning node.
func TestPveAction(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAuth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		w.Write([]byte(`{"data":"UPID:pve:0"}`))
	}))
	defer srv.Close()

	if err := pveAction(PVE{URL: srv.URL, Token: "u@pam!t=s"}, "pve", "qemu", "100", "start"); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("want POST, got %s", gotMethod)
	}
	if gotPath != "/api2/json/nodes/pve/qemu/100/status/start" {
		t.Fatalf("path wrong: %s", gotPath)
	}
	if gotAuth != "PVEAPIToken=u@pam!t=s" {
		t.Fatalf("auth wrong: %s", gotAuth)
	}
}

// pveValidNode keeps a crafted node segment from reshaping the request path.
func TestPveValidNode(t *testing.T) {
	for _, ok := range []string{"pve", "pve-1", "node.dc1", "n_2"} {
		if !pveValidNode(ok) {
			t.Fatalf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "pve/..", "a b", "n;rm", "../x"} {
		if pveValidNode(bad) {
			t.Fatalf("%q should be rejected", bad)
		}
	}
}

// Ghostty rejects `--class NAME` (its CLI wants `--class=NAME`) and validates the
// value as a GTK application id, which needs a dot: a bare "ryoport-ssh" was
// dropped on the floor, the window kept com.mitchellh.ghostty, and the float
// window rule never matched (issue #280). Every terminal on the switch now gets
// the equals form and the same GTK-valid id.
func TestTerminalArgvClassForm(t *testing.T) {
	if !strings.Contains(ryoportAppID, ".") || strings.Contains(ryoportAppID, "-") {
		t.Fatalf("%q is not a GTK application id (needs a dot, hyphens are rejected)", ryoportAppID)
	}
	for _, term := range []string{"kitty", "ghostty", "foot", "alacritty"} {
		t.Setenv("TERMINAL", term)
		argv := terminalArgv("myhost")
		var hasID, hasBare bool
		for _, a := range argv {
			if strings.HasSuffix(a, "="+ryoportAppID) {
				hasID = true // --class=<id>, or --app-id=<id> on foot
			}
			if a == "--class" || a == "--app-id" {
				hasBare = true
			}
		}
		if !hasID {
			t.Errorf("%s argv = %v, want the GTK id %q passed with =", term, argv, ryoportAppID)
		}
		if hasBare {
			t.Errorf("%s argv = %v, must not pass a valueless class flag", term, argv)
		}
	}
	// wezterm and xterm take no class at all: they must not gain one either way.
	t.Setenv("TERMINAL", "wezterm")
	for _, a := range terminalArgv("myhost") {
		if strings.HasPrefix(a, "--class") {
			t.Fatalf("wezterm argv carries a class flag: %v", a)
		}
	}
}

// probeErr must split "the box is dead" from "the box wants a password ryoport
// does not have": the first is a plain down, the second becomes need-auth so
// the GUI can offer save-password instead of leaving dead graphs.
func TestProbeErrNeedAuth(t *testing.T) {
	cases := []struct {
		stderr      string
		usePassword bool
		wantMsg     string
		wantAuth    bool
	}{
		{"Permission denied (publickey,password).", false, "authentication required", true},
		{"Too many authentication failures", false, "authentication required", true},
		{"Permission denied, please try again.", true, "saved password rejected", false},
		{"Host key verification failed.", false, "host key changed (known_hosts)", false},
		{"ssh: connect to host 10.0.0.1 port 22: No route to host", false,
			"ssh: connect to host 10.0.0.1 port 22: No route to host", false},
	}
	ctx := context.Background()
	for _, c := range cases {
		msg, auth := probeErr(ctx, errors.New("exit status 255"), c.stderr, c.usePassword)
		if msg != c.wantMsg || auth != c.wantAuth {
			t.Errorf("probeErr(%q, usePassword=%v) = (%q,%v), want (%q,%v)",
				c.stderr, c.usePassword, msg, auth, c.wantMsg, c.wantAuth)
		}
	}
}

// The whole console product is a strict argv builder around four external
// clients; every rule the GUI depends on lives here, not in QML.
func TestValidConsole(t *testing.T) {
	good := []Console{
		{Kind: "serial", Name: "sw", Device: "/dev/ttyUSB0", Baud: 115200},
		{Kind: "telnet", Name: "r", Address: "router.home"},
		{Kind: "rdp", Name: "w", Address: "desk.example", Port: 3390, User: "alice"},
		{Kind: "vnc", Name: "v", Address: "10.0.0.5", Port: 5901},
	}
	for _, c := range good {
		d := c
		if err := validConsole(&d); err != nil {
			t.Errorf("valid %+v rejected: %v", c, err)
		}
	}
	// defaults fill in when absent
	d := Console{Kind: "telnet", Name: "r", Address: "h"}
	if err := validConsole(&d); err != nil || d.Port != 23 {
		t.Errorf("telnet default port: got %d, %v", d.Port, err)
	}
	d = Console{Kind: "vnc", Name: "v", Address: "h"}
	if err := validConsole(&d); err != nil || d.Port != 5900 {
		t.Errorf("vnc default port: got %d, %v", d.Port, err)
	}
	bad := []Console{
		{Kind: "ssh", Name: "x"},
		{Kind: "rdp", Name: "x", Address: "-gateway:/evil"},
		{Kind: "rdp", Name: "x", Address: "ok; rm -rf /"},
		{Kind: "rdp", Name: "x", Address: "a\nb"},
		{Kind: "serial", Name: "x", Device: "ttyUSB0", Baud: 9600},
		{Kind: "serial", Name: "x", Device: "/dev/ttyUSB 0", Baud: 9600},
		{Kind: "serial", Name: "x", Device: "/dev/ttyUSB0", Baud: 0},
	}
	for _, c := range bad {
		d := c
		if err := validConsole(&d); err == nil {
			t.Errorf("invalid %+v accepted", c)
		}
	}
}

// FreeRDP's /p: is visible in ps, so a saved password only ever enters the
// private /args-from pipe; the ordinary argv never carries it.
func TestRdpArgvKeepsPasswordOutOfArgv(t *testing.T) {
	c := Console{Kind: "rdp", Name: "w", Address: "desk.example", Port: 3390, User: "alice", Domain: "HQ"}
	argv := rdpArgv("xfreerdp3", c)
	joined := strings.Join(argv, " ")
	if !strings.Contains(joined, "/v:desk.example:3390") || !strings.Contains(joined, "/u:alice") ||
		!strings.Contains(joined, "/d:HQ") || !strings.Contains(joined, "/cert:tofu") {
		t.Fatalf("argv incomplete: %q", joined)
	}
	if strings.Contains(joined, "hunter2") || strings.Contains(joined, "/p:") {
		t.Fatalf("no-saved-password argv must not carry a secret: %q", joined)
	}
	file := rdpArgsFile(c, "hunter2")
	if !strings.Contains(file, "/p:hunter2") {
		t.Fatalf("args file must carry the password line: %q", file)
	}
	// one argument per line, no shell quoting tricks.
	if strings.Contains(file, "\n/p:hunt er2") {
		t.Fatal("password split across lines")
	}
	// every port is spelled, default or not: one argv shape, assembled only
	// through JoinHostPort.
	if !strings.Contains(rdpArgsFile(Console{Kind: "rdp", Name: "w", Address: "h", Port: 3389}, "p"), "/v:h:3389") {
		t.Error("default port should appear in /v:")
	}
}

// The serial launch must stay flag-honest: picocom knows --baud/--flow, and a
// typo'd flag kills the session at startup with an error the GUI cannot see.
func TestSerialArgvUsesRealPicocomFlags(t *testing.T) {
	argv := serialArgv("picocom", Console{Kind: "serial", Name: "s", Device: "/dev/ttyUSB0", Baud: 115200})
	want := []string{"picocom", "--baud", "115200", "--flow", "n", "/dev/ttyUSB0"}
	if strings.Join(argv, " ") != strings.Join(want, " ") {
		t.Fatalf("serial argv = %v, want %v", argv, want)
	}
	for _, a := range argv {
		if a == "--nomap" {
			t.Fatal("picocom has no --nomap option")
		}
	}
}

func TestVncArgvTargetsExplicitPort(t *testing.T) {
	argv := vncArgv("vncviewer", Console{Kind: "vnc", Name: "v", Address: "10.0.0.5", Port: 5901, Fullscreen: true})
	want := []string{"vncviewer", "10.0.0.5::5901", "-FullScreen"}
	if strings.Join(argv, " ") != strings.Join(want, " ") {
		t.Fatalf("vnc argv = %v, want %v", argv, want)
	}
}

func TestTelnetArgv(t *testing.T) {
	argv := telnetArgv("telnet", Console{Kind: "telnet", Name: "r", Address: "router.home", Port: 23})
	if strings.Join(argv, " ") != "telnet router.home 23" {
		t.Fatalf("telnet argv = %v", argv)
	}
}

// Serial consoles report a distinct, actionable fault when the node is absent
// or the user is in the wrong group, instead of a bare "down".
func TestConsoleProbeSerial(t *testing.T) {
	res := consoleProbeOne(Console{ID: "x", Kind: "serial", Name: "s", Device: "/dev/null", Baud: 9600})
	if res["up"] != true {
		t.Fatalf("/dev/null must probe up, got %v", res)
	}
	res = consoleProbeOne(Console{ID: "x", Kind: "serial", Name: "s", Device: "/dev/ryokutest-absent", Baud: 9600})
	if res["up"] == true || !strings.Contains(res["error"].(string), "not present") {
		t.Fatalf("absent device must say so, got %v", res)
	}
	res = consoleProbeOne(Console{ID: "x", Kind: "serial", Name: "s", Device: "/tmp", Baud: 9600})
	if res["up"] == true || !strings.Contains(res["error"].(string), "device node") {
		t.Fatalf("non-device must say so, got %v", res)
	}
}

// The bookkeeping round trip: ids assigned, edit-saves preserve the keyring
// link, removes drop the entry. (Keyring calls no-op without secret-tool.)
func TestConsoleUpsertRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	c := upsertConsole(Console{Kind: "rdp", Name: "work", Address: "h", Port: 3389})
	if c.ID == "" {
		t.Fatal("new console got no id")
	}
	if got := readConsoles(); len(got) != 1 || got[0].Name != "work" {
		t.Fatalf("list after add = %+v", got)
	}
	// an edit-save that loses the Auth flag (the GUI never resends it) must
	// keep the stored keyring link.
	stored := readConsoles()
	stored[0].Auth = "password"
	writeConsoles(stored)
	c = upsertConsole(Console{ID: c.ID, Kind: "rdp", Name: "work-renamed", Address: "h", Port: 3390})
	if c.Auth != "password" {
		t.Fatalf("edit-save dropped the auth link: %+v", c)
	}
	if got := readConsoles(); len(got) != 1 || got[0].Port != 3390 {
		t.Fatalf("edit-save should replace in place: %+v", got)
	}
	dropConsole(c.ID)
	if got := readConsoles(); len(got) != 0 {
		t.Fatalf("remove left %+v", got)
	}
}

// A corrupt or missing book file must degrade to an empty list, never a crash:
// every verb runs through readConsoles before anything else.
func TestReadConsolesTolerant(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "ryoku", "ryoport")
	os.MkdirAll(dir, 0700)
	os.WriteFile(filepath.Join(dir, "consoles.json"), []byte("{ not json"), 0600)
	if cs := readConsoles(); len(cs) != 0 {
		t.Fatalf("corrupt file yielded %+v", cs)
	}
	os.Remove(filepath.Join(dir, "consoles.json"))
	if cs := readConsoles(); cs == nil || len(cs) != 0 {
		t.Fatalf("missing file yielded %#v", cs)
	}
}
