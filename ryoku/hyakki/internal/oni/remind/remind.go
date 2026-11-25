// Package remind reads reminder and timer requests in code, with no model,
// and keeps the ones the user set in a small file until they are due. A kept
// reminder holds its own words and its times, never the sentence it was
// asked with.
package remind

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"ryoku-rashin/internal/oni/spec"
)

// The kinds of a kept reminder.
const (
	KindReminder = "reminder"
	KindTimer    = "timer"
)

func limits() spec.ReflexReminders {
	s, err := spec.Load()
	if err != nil {
		panic(err)
	}
	return s.Reflex.Reminders
}

// The spec's reflex.reminders numbers.
var (
	// MaxDays is how many calendar days ahead a reminder may lie.
	MaxDays = limits().MaxDays
	// MaxReminder is MaxDays as elapsed time. A reminder may lie as far as
	// the later of MaxDays on the wall clock and MaxReminder, so a daylight
	// saving change inside the week never refuses "in 7 days".
	MaxReminder = limits().MaxReminder()
	// MaxTimer is the longest timer.
	MaxTimer = limits().MaxTimer()
	// MaxPending is the most reminders and timers kept at once.
	MaxPending = limits().MaxPending
	// LateAfter is how long past its due time a reminder may fire before it
	// counts as late: every wait for a busy user, and a minute more.
	LateAfter = time.Duration(limits().BusyTries)*limits().BusyWait() + time.Minute
)

// MinTimer is the shortest timer.
const MinTimer = time.Second

// MaxTextRunes bounds a reminder's own words.
const MaxTextRunes = 200

// Reminder is one reminder or timer. Text is the reminder's own words, such
// as "call the bank", and may be empty.
type Reminder struct {
	ID      string    `json:"id"`
	Kind    string    `json:"kind"`
	Due     time.Time `json:"due"`
	Created time.Time `json:"created"`
	Text    string    `json:"text,omitempty"`
}

// Late reports whether firing r at at is late: more than LateAfter past
// its due time, as when the daemon was not running when it fell due.
func (r Reminder) Late(at time.Time) bool { return at.Sub(r.Due) > LateAfter }

// idBytes is the random part of an id; an id is its hex.
const idBytes = 8

func newID() string {
	b := make([]byte, idBytes)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func validID(id string) bool {
	if len(id) != 2*idBytes {
		return false
	}
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
