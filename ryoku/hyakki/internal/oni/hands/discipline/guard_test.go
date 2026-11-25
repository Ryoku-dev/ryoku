package discipline

import (
	"sync"
	"testing"
)

// The key reader can say only that a key went down: KeyDown takes nothing.
var _ func() = (*Guard)(nil).KeyDown

func TestGuardPausesOnAnyKeyThePersonPresses(t *testing.T) {
	steps := []struct {
		name string
		do   func(*Guard)
		want Pause
	}{
		{"no key reader has said it listens", nil, PauseKey},
		{"the key reader listens and no key went down", func(g *Guard) { g.Listening(true) }, OK},
		{"a key went down", func(g *Guard) { g.KeyDown() }, PauseKey},
		{"still paused until Resume", nil, PauseKey},
		{"Resume counts afresh", func(g *Guard) { g.Resume() }, OK},
		{"keys while checking count again", func(g *Guard) { g.KeyDown(); g.KeyDown() }, PauseKey},
		{"resumed again", func(g *Guard) { g.Resume() }, OK},
		{"the key reader stopped", func(g *Guard) { g.Listening(false) }, PauseKey},
		{"a resume cannot stand in for a key reader", func(g *Guard) { g.Resume() }, PauseKey},
		{"the key reader came back", func(g *Guard) { g.Listening(true) }, OK},
	}
	g := NewGuard(picked(t), self, 2)
	for _, st := range steps {
		if st.do != nil {
			st.do(g)
		}
		if got := g.Check(scene()); got != st.want {
			t.Fatalf("%s: Check = %q, want %q", st.name, got, st.want)
		}
	}
}

func TestGuardNamesTheWindowChangeBeforeTheKey(t *testing.T) {
	g := NewGuard(picked(t), self, 2)
	g.Listening(true)
	g.KeyDown()
	s := scene()
	at(&s, "0xa").Title = "notes *"
	if got := g.Check(s); got != PauseTitle {
		t.Fatalf("Check = %q, want %q", got, PauseTitle)
	}
}

func TestGuardKeepsItsOwnCopyOfSelf(t *testing.T) {
	ids := []string{hyakki}
	g := NewGuard(picked(t), ids, 2)
	g.Listening(true)
	ids[0] = "something.else"
	s := scene()
	s.Windows = append(s.Windows, floating("0xc", hyakki, 500, 400))
	if got := g.Check(s); got != PauseSelf {
		t.Fatalf("Check = %q, want %q", got, PauseSelf)
	}
}

func TestGuardHearsKeysFromOtherGoroutines(t *testing.T) {
	g := NewGuard(picked(t), self, 2)
	g.Listening(true)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 100 {
				g.KeyDown()
			}
		})
	}
	wg.Wait()
	if got := g.Check(scene()); got != PauseKey {
		t.Fatalf("Check = %q, want %q", got, PauseKey)
	}
	g.Resume()
	if got := g.Check(scene()); got != OK {
		t.Fatalf("after Resume Check = %q", got)
	}
}
