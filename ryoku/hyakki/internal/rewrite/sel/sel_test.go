package sel

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fake struct {
	out   []byte
	err   error
	calls [][]string
	limit int
}

func (f *fake) run(_ context.Context, limit int, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	f.limit = limit
	if f.err != nil {
		return nil, f.err
	}
	out := f.out
	if len(out) > limit {
		out = out[:limit]
	}
	return append([]byte(nil), out...), nil
}

func TestAllowedIsTheOnePrimaryRead(t *testing.T) {
	if !Allowed("wl-paste", "--primary", "--no-newline", "--type", "text/plain;charset=utf-8") {
		t.Fatal("the primary read is refused")
	}
	for _, bad := range [][]string{
		{"wl-paste"},
		{"wl-paste", "--no-newline", "--type", "text/plain;charset=utf-8"},
		{"wl-paste", "--primary", "--no-newline"},
		{"wl-paste", "--primary", "--no-newline", "--type", "image/png"},
		{"wl-paste", "--primary", "--no-newline", "--type", "text/plain;charset=utf-8", "sh"},
		{"wl-paste", "--list-types"},
		{"wl-copy", "--primary"},
		{"wl-copy", "hello"},
		{"/usr/bin/wl-paste", "--primary", "--no-newline", "--type", "text/plain;charset=utf-8"},
		{"sh", "-c", "wl-paste --primary"},
		{"xclip", "-o", "-selection", "primary"},
	} {
		if Allowed(bad[0], bad[1:]...) {
			t.Errorf("allowed %q", bad)
		}
	}
}

func TestReadNeverTouchesTheClipboard(t *testing.T) {
	f := &fake{out: []byte("Selected words")}
	got, err := Read(context.Background(), f.run, 100)
	if err != nil || got != "Selected words" {
		t.Fatalf("read %q, %v", got, err)
	}
	if len(f.calls) != 1 || !Allowed(f.calls[0][0], f.calls[0][1:]...) {
		t.Fatalf("calls %q", f.calls)
	}
	for _, c := range f.calls {
		for _, a := range c {
			if a == "wl-copy" || strings.Contains(a, "clipboard") || a == "--paste-once" {
				t.Fatalf("a clipboard argv ran: %q", c)
			}
		}
	}
}

func TestReadRefusesWhatItCannotUse(t *testing.T) {
	for _, tc := range []struct {
		name string
		f    *fake
		max  int
		want error
	}{
		{"nothing selected", &fake{err: ErrEmpty}, 100, ErrEmpty},
		{"white space", &fake{out: []byte(" \n\t ")}, 100, ErrEmpty},
		{"empty", &fake{out: nil}, 100, ErrEmpty},
		{"tool failed", &fake{err: errors.New("broken pipe")}, 100, ErrUnreadable},
		{"tool refused", &fake{err: ErrNotAllowed}, 100, ErrUnreadable},
		{"not utf8", &fake{out: []byte{0xff, 0xfe, 'a'}}, 100, ErrUnreadable},
		{"too many runes", &fake{out: []byte(strings.Repeat("é", 11))}, 10, ErrTooLong},
		{"too many bytes", &fake{out: []byte(strings.Repeat("a", 4*10+5))}, 10, ErrTooLong},
		{"no runner", nil, 100, ErrUnreadable},
	} {
		var run Runner
		if tc.f != nil {
			run = tc.f.run
		}
		if got, err := Read(context.Background(), run, tc.max); !errors.Is(err, tc.want) || got != "" {
			t.Errorf("%s: %q, %v; want %v", tc.name, got, err, tc.want)
		}
	}
	// the read is bounded: never more than max runes' worth of bytes
	f := &fake{out: []byte(strings.Repeat("a", 1000))}
	_, _ = Read(context.Background(), f.run, 10)
	if f.limit != 10*4+1 {
		t.Fatalf("limit %d", f.limit)
	}
}

func TestExecRefusesBeforeStarting(t *testing.T) {
	ctx := context.Background()
	env, err := Env("/run/user/1000", "wayland-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Exec(env)(ctx, 10, "wl-copy", "x"); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("wl-copy: %v", err)
	}
	if _, err := Exec(append(env, "DBUS_SESSION_BUS_ADDRESS=x"))(ctx, 10, Tool, Args...); !errors.Is(err, ErrEnv) {
		t.Fatalf("extra env: %v", err)
	}
	for _, bad := range [][2]string{{"run/user", "wayland-1"}, {"/run/user/../x", "wayland-1"}, {"/run/user/1000", "../wayland-1"}, {"/run/user/1000", ""}} {
		if _, err := Env(bad[0], bad[1]); !errors.Is(err, ErrEnv) {
			t.Errorf("Env(%q, %q) = %v", bad[0], bad[1], err)
		}
	}
}
