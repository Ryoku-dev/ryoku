package kit

import (
	"testing"
	"time"
)

func TestPlacardAgeHoldsWhileHeld(t *testing.T) {
	start := time.Unix(1000, 0)
	entry := &placardEntry{left: 5 * time.Second, last: start}
	if !entry.age(start.Add(4 * time.Second)) {
		t.Fatal("expired before its life ran out")
	}
	entry.held = true
	if !entry.age(start.Add(60 * time.Second)) {
		t.Fatal("a held placard aged")
	}
	entry.held = false
	if !entry.age(start.Add(60*time.Second + 900*time.Millisecond)) {
		t.Fatal("the second left before the hold was lost")
	}
	if entry.age(start.Add(61 * time.Second)) {
		t.Fatal("the placard outlived its life")
	}
}
