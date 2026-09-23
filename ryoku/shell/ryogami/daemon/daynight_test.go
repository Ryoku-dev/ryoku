package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDayNightPickNoRepeatCyclesPool(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a.mp4", "b.mp4", "c.mp4"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := newDayNightRotation()
	r.dayDir = dir
	r.nightDir = dir
	r.noRepeat = true
	r.dayKey = ""

	seen := map[string]bool{}
	// Three picks must cover the whole pool without repeating.
	for i := 0; i < 3; i++ {
		p := r.pick(true)
		if p == "" {
			t.Fatalf("pick %d empty", i)
		}
		b := filepath.Base(p)
		if seen[b] {
			t.Fatalf("repeat before pool exhausted: %s", b)
		}
		seen[b] = true
	}
	if len(seen) != 3 {
		t.Fatalf("expected 3 distinct, got %d", len(seen))
	}
	// Fourth pick refills the day's set and returns something.
	if r.pick(true) == "" {
		t.Fatal("refill returned empty")
	}
}

func TestDayNightPickAllowsRepeatWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "only.mp4"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := newDayNightRotation()
	r.dayDir = dir
	r.noRepeat = false
	// no-repeat off: a single-clip pool still yields that clip every time.
	if r.pick(true) == "" || r.pick(true) == "" {
		t.Fatal("expected a pick with noRepeat off")
	}
}

func TestDayNightEmptyPoolYieldsNothing(t *testing.T) {
	r := newDayNightRotation()
	r.dayDir = filepath.Join(t.TempDir(), "missing")
	r.noRepeat = true
	if r.pick(true) != "" {
		t.Fatal("missing pool must yield empty")
	}
}
