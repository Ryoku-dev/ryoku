package launch

import (
	"context"
	"testing"
)

func TestOneSurfaceHoldsTheLock(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	if Running() {
		t.Fatal("a surface runs before any took the lock")
	}
	first, held, err := Acquire()
	if err != nil || !held {
		t.Fatalf("first acquire: held=%v err=%v", held, err)
	}
	if !Running() {
		t.Fatal("the held lock does not count as running")
	}
	if _, held, err := Acquire(); err != nil || held {
		t.Fatalf("second acquire: held=%v err=%v, want refused", held, err)
	}
	if err := Launch(context.Background()); err != nil {
		t.Fatalf("launch beside a running surface = %v, want a no-op", err)
	}
	first.Release()
	if Running() {
		t.Fatal("a released lock still counts as running")
	}
	if err := Stop(context.Background()); err != nil {
		t.Fatalf("stop with nothing running = %v", err)
	}
}

func TestLaunchNeedsWayland(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("WAYLAND_DISPLAY", "")
	if err := Launch(context.Background()); err != ErrNoDisplay {
		t.Fatalf("launch without a display = %v, want ErrNoDisplay", err)
	}
}
