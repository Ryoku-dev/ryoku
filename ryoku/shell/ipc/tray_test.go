package main

import (
	"bufio"
	"bytes"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// argbToRGBA rewrites each pixel from ARGB (network order) to RGBA.
func TestArgbToRGBA(t *testing.T) {
	in := []byte{1, 2, 3, 4, 5, 6, 7, 8} // (A,R,G,B) = (1,2,3,4) and (5,6,7,8)
	want := []byte{2, 3, 4, 1, 6, 7, 8, 5}
	if got := argbToRGBA(in); !bytes.Equal(got, want) {
		t.Errorf("argbToRGBA = %v, want %v", got, want)
	}
}

// selectBestPixmap picks the size closest to the target and rejects a pixmap
// whose byte length cannot hold its claimed dimensions.
func TestSelectBestPixmap(t *testing.T) {
	px := func(w, h int) trayPixmap { return trayPixmap{W: w, H: h, Data: make([]byte, w*h*4)} }
	pixmaps := []trayPixmap{px(16, 16), px(32, 32), px(22, 22), px(48, 48)}
	if best := selectBestPixmap(pixmaps, 24); best == nil || best.W != 22 {
		t.Fatalf("selectBestPixmap picked %v, want 22x22 (nearest 24)", best)
	}
	invalid := []trayPixmap{{W: 24, H: 24, Data: []byte{1, 2, 3}}, {W: 0, H: 0}}
	if selectBestPixmap(invalid, 24) != nil {
		t.Errorf("selectBestPixmap accepted an undersized or empty pixmap")
	}
	if selectBestPixmap(nil, 24) != nil {
		t.Errorf("selectBestPixmap(nil) = non-nil")
	}
}

// resolveTrayIcon is the full precedence chain: per-item themed path, then theme
// name, then nearest pixmap converted to RGBA, then the generic fallback, with
// the Ryoku attention-icon override on top.
func TestResolveTrayIcon(t *testing.T) {
	none := func(string) bool { return false }

	got := resolveTrayIcon(trayIconInput{IconName: "app", IconThemePath: "/icons"},
		func(p string) bool { return p == "/icons/app.png" })
	if got.Path != "/icons/app.png" || got.Name != "" {
		t.Errorf("themed path = %+v, want Path=/icons/app.png", got)
	}

	got = resolveTrayIcon(trayIconInput{IconName: "app", IconThemePath: "/icons"}, none)
	if got.Name != "app" || got.Path != "" {
		t.Errorf("theme-name fallback = %+v, want Name=app", got)
	}

	got = resolveTrayIcon(trayIconInput{IconName: "app"}, none)
	if got.Name != "app" {
		t.Errorf("plain name = %+v, want Name=app", got)
	}

	px := trayPixmap{W: 2, H: 1, Data: []byte{1, 2, 3, 4, 5, 6, 7, 8}}
	got = resolveTrayIcon(trayIconInput{IconPixmaps: []trayPixmap{px}}, none)
	if got.RGBA == nil || got.W != 2 || !bytes.Equal(got.RGBA, []byte{2, 3, 4, 1, 6, 7, 8, 5}) {
		t.Errorf("pixmap fallback = %+v, want converted rgba", got)
	}

	got = resolveTrayIcon(trayIconInput{}, none)
	if got.Name != "application-x-executable-symbolic" {
		t.Errorf("empty input = %+v, want the generic executable icon", got)
	}

	// Ryoku divergence: NeedsAttention prefers the attention icon.
	got = resolveTrayIcon(trayIconInput{Status: "NeedsAttention", IconName: "app", AttentionName: "urgent"}, none)
	if got.Name != "urgent" {
		t.Errorf("attention name = %+v, want Name=urgent", got)
	}
	apx := trayPixmap{W: 1, H: 1, Data: []byte{9, 8, 7, 6}}
	got = resolveTrayIcon(trayIconInput{Status: "NeedsAttention", AttentionPixmaps: []trayPixmap{apx}}, none)
	if got.RGBA == nil || !bytes.Equal(got.RGBA, []byte{8, 7, 6, 9}) {
		t.Errorf("attention pixmap = %+v, want converted attention rgba", got)
	}
}

// parseTrayService splits a registration string into bus name and object path,
// defaulting the path and falling back to the caller when only a path is given.
func TestParseTrayService(t *testing.T) {
	cases := []struct {
		service, sender, wantBus, wantPath string
	}{
		{":1.42", "", ":1.42", "/StatusNotifierItem"},
		{"", ":1.7", ":1.7", "/StatusNotifierItem"},
		{"/org/x/Item", ":1.9", ":1.9", "/org/x/Item"},
		{":1.5/org/ayatana/NotificationItem/x", "", ":1.5", "/org/ayatana/NotificationItem/x"},
	}
	for _, c := range cases {
		bus, path := parseTrayService(c.service, c.sender)
		if bus != c.wantBus || string(path) != c.wantPath {
			t.Errorf("parseTrayService(%q,%q) = (%q,%q), want (%q,%q)", c.service, c.sender, bus, path, c.wantBus, c.wantPath)
		}
	}
}

func TestTrayRegisteredSignalCarriesBusName(t *testing.T) {
	bin, err := exec.LookPath("dbus-daemon")
	if err != nil {
		t.Skip("dbus-daemon not installed")
	}
	cmd := exec.Command(bin, "--session", "--nofork", "--print-address=1")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	addr, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	connect := func() *dbus.Conn {
		c, err := dbus.Connect(strings.TrimSpace(addr))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = c.Close() })
		return c
	}
	watcher, host, app := connect(), connect(), connect()
	ts := &trayState{conn: watcher, items: map[string]*trayItem{}, isWatcher: true}
	if err := watcher.Export(ts, dbus.ObjectPath(watcherPath), watcherIface); err != nil {
		t.Fatal(err)
	}
	if err := host.AddMatchSignal(dbus.WithMatchInterface(watcherIface)); err != nil {
		t.Fatal(err)
	}
	sigs := make(chan *dbus.Signal, 16)
	host.Signal(sigs)
	if err := watcher.AddMatchSignal(dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchInterface("org.freedesktop.DBus"), dbus.WithMatchMember("NameOwnerChanged")); err != nil {
		t.Fatal(err)
	}
	owners := make(chan *dbus.Signal, 16)
	watcher.Signal(owners)
	go func() {
		for sig := range owners {
			ts.dispatchSignal(sig)
		}
	}()
	register := func(service string) error {
		return app.Object(watcher.Names()[0], dbus.ObjectPath(watcherPath)).
			Call(watcherIface+".RegisterStatusNotifierItem", 0, service).Err
	}
	waitSignal := func(member, want string) {
		t.Helper()
		timeout := time.After(5 * time.Second)
		for {
			select {
			case sig := <-sigs:
				if sig.Name != watcherIface+"."+member {
					continue
				}
				if len(sig.Body) != 1 || sig.Body[0] != want {
					t.Fatalf("%s body = %v, want [%s]", member, sig.Body, want)
				}
				return
			case <-timeout:
				t.Fatalf("no %s signal", member)
			}
		}
	}
	path := "/org/ayatana/NotificationItem/x"
	want := app.Names()[0] + path
	if err := register(path); err != nil {
		t.Fatal(err)
	}
	waitSignal("StatusNotifierItemRegistered", want)
	if err := register(path); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"/bad-path", "/bad//path", "/bad\npath", host.Names()[0]} {
		if err := register(bad); err == nil {
			t.Fatalf("accepted invalid or foreign registration %q", bad)
		}
	}
	ts.addItem("/bad-path", app.Names()[0])
	if got := ts.serviceList(); len(got) != 1 || got[0] != want {
		t.Fatalf("items = %v, want [%s]", got, want)
	}
	select {
	case sig := <-sigs:
		t.Fatalf("duplicate registration emitted %s", sig.Name)
	case <-time.After(50 * time.Millisecond):
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	waitSignal("StatusNotifierItemUnregistered", want)
	if got := ts.serviceList(); len(got) != 0 {
		t.Fatalf("disconnected app left items: %v", got)
	}

	app = connect()
	name := "org.example.TrayTest"
	if _, err := app.RequestName(name, dbus.NameFlagAllowReplacement); err != nil {
		t.Fatal(err)
	}
	if err := register(name); err != nil {
		t.Fatal(err)
	}
	want = app.Names()[0] + "/StatusNotifierItem"
	waitSignal("StatusNotifierItemRegistered", want)
	if _, err := host.RequestName(name, dbus.NameFlagReplaceExisting); err != nil {
		t.Fatal(err)
	}
	waitSignal("StatusNotifierItemUnregistered", want)
}
