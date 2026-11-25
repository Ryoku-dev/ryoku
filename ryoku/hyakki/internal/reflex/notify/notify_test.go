package notify

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"

	"ryoku-rashin/internal/reflex/bus"
)

type fakeBus struct {
	dials  int
	closed int
	calls  [][]any
	method string
}

func (f *fakeBus) dial(context.Context) (bus.Conn, error) { f.dials++; return f, nil }

func (f *fakeBus) Call(_ context.Context, dest, path, method string, args []any, reply ...any) error {
	f.method = dest + " " + path + " " + method
	f.calls = append(f.calls, args)
	*reply[0].(*uint32) = 7
	return nil
}

func (f *fakeBus) Close() error { f.closed++; return nil }

func hints() map[string]dbus.Variant {
	return map[string]dbus.Variant{"desktop-entry": dbus.MakeVariant(DesktopEntry)}
}

func TestPostSendsOneNotify(t *testing.T) {
	f := &fakeBus{}
	id, err := New(f.dial).Post(context.Background(), "Time's up.", "Tea")
	if err != nil || id != 7 {
		t.Fatalf("Post = %d, %v", id, err)
	}
	if f.method != "org.freedesktop.Notifications /org/freedesktop/Notifications org.freedesktop.Notifications.Notify" {
		t.Fatalf("called %q", f.method)
	}
	args := f.calls[0]
	if args[0] != "Hyakki" || args[1] != uint32(0) || args[2] != "" || args[3] != "Time's up." || args[4] != "Tea" ||
		len(args[5].([]string)) != 0 || args[7] != int32(-1) {
		t.Fatalf("args %#v", args)
	}
	if f.closed != 1 {
		t.Fatal("the connection must close after the notice")
	}
}

func TestPostRefusesBadNoticesWithoutDialing(t *testing.T) {
	f := &fakeBus{}
	n := New(f.dial)
	for _, c := range [][2]string{{"", "body"}, {strings.Repeat("x", MaxSummary+1), ""}, {"ok", strings.Repeat("y", MaxBody+1)}} {
		if _, err := n.Post(context.Background(), c[0], c[1]); !errors.Is(err, ErrBadNotice) {
			t.Errorf("%q: %v", c[0], err)
		}
	}
	if f.dials != 0 {
		t.Fatal("a refused notice must not open the bus")
	}
}

func TestAllowedCallIsExact(t *testing.T) {
	good := []any{AppName, uint32(0), "", "Summary", "Body", []string{}, hints(), int32(-1)}
	if !AllowedCall(dest, path, method, good) {
		t.Fatal("the one Notify must be allowed")
	}
	with := func(i int, v any) []any {
		a := append([]any(nil), good...)
		a[i] = v
		return a
	}
	bad := map[string][]any{
		"other app name":  with(0, "Someone"),
		"replaces an id":  with(1, uint32(3)),
		"an icon path":    with(2, "/tmp/x.png"),
		"empty summary":   with(3, ""),
		"long summary":    with(3, strings.Repeat("s", MaxSummary+1)),
		"long body":       with(4, strings.Repeat("b", MaxBody+1)),
		"actions":         with(5, []string{"default", "Open"}),
		"extra hint":      with(6, map[string]dbus.Variant{"desktop-entry": dbus.MakeVariant(DesktopEntry), "urgency": dbus.MakeVariant(byte(2))}),
		"other entry":     with(6, map[string]dbus.Variant{"desktop-entry": dbus.MakeVariant("other")}),
		"own timeout":     with(7, int32(0)),
		"wrong arg types": with(1, 0),
		"short":           good[:7],
	}
	for name, args := range bad {
		if AllowedCall(dest, path, method, args) {
			t.Errorf("%s: allowed", name)
		}
	}
	if AllowedCall(dest, path, "org.freedesktop.Notifications.CloseNotification", []any{uint32(1)}) ||
		AllowedCall(dest, "/other", method, good) || AllowedCall("org.example.Other", path, method, good) {
		t.Fatal("only Notify on the service's own path is allowed")
	}
}
