package proc

import (
	"context"
	"errors"
	"testing"
)

func only(name string, args ...string) bool { return name == "ok" && len(args) == 1 && args[0] == "1" }

func TestGuardChecksBeforeRunning(t *testing.T) {
	ran := 0
	run := Guard(only, func(context.Context, string, ...string) ([]byte, error) { ran++; return []byte("x"), nil })
	if _, err := run(context.Background(), "ok", "1"); err != nil || ran != 1 {
		t.Fatalf("allowed call: %v ran=%d", err, ran)
	}
	for _, argv := range [][]string{{"ok"}, {"ok", "2"}, {"sh", "-c", "ok 1"}, {"ok", "1", "extra"}} {
		if _, err := run(context.Background(), argv[0], argv[1:]...); !errors.Is(err, ErrNotAllowed) {
			t.Errorf("%q: %v", argv, err)
		}
	}
	if ran != 1 {
		t.Fatalf("a refused call ran (%d)", ran)
	}
	if _, err := Guard(only, nil)(context.Background(), "ok", "1"); err == nil {
		t.Fatal("no runner must be an error, never a silent success")
	}
}

func TestGuardSpawnChecksBeforeStarting(t *testing.T) {
	started := 0
	spawn := GuardSpawn(only, func([]string) (<-chan error, error) { started++; return make(chan error), nil })
	if _, err := spawn([]string{"ok", "1"}); err != nil || started != 1 {
		t.Fatalf("allowed start: %v", err)
	}
	for _, argv := range [][]string{nil, {"ok"}, {"bash", "-c", "ok 1"}} {
		if _, err := spawn(argv); !errors.Is(err, ErrNotAllowed) {
			t.Errorf("%q: %v", argv, err)
		}
	}
	if started != 1 {
		t.Fatalf("a refused argv started (%d)", started)
	}
}
