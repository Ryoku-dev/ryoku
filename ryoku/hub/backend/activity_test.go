package main

import (
	"strconv"
	"testing"
	"time"
)

func at(day, hour, min int) time.Time {
	return time.Date(2026, 11, day, hour, min, 0, 0, time.UTC)
}

func hoursOf(days []activityDay) map[string]float64 {
	out := map[string]float64{}
	for _, d := range days {
		out[d.Date] = d.Hours
	}
	return out
}

// A boot that runs past midnight counts on both days, and a suspend inside it
// counts on neither: the map is what the machine was really doing.
func TestOnTimeSplitsAtMidnightAndSkipsSleep(t *testing.T) {
	boots := []span{{at(10, 20, 0), at(11, 8, 0)}}
	sleeps := []span{{at(10, 23, 0), at(11, 6, 0)}}
	got := hoursOf(onTimeByDay(boots, sleeps, at(10, 0, 0), at(12, 0, 0)))
	if got["2026-11-10"] != 3 || got["2026-11-11"] != 2 {
		t.Fatalf("got %v, want 3 h on the 10th and 2 h on the 11th", got)
	}
}

// Overlapping boots (the journal's first and last entries can overlap across a
// quick reboot) are not counted twice, and a day with no boot is 0.
func TestOnTimeMergesOverlapsAndKeepsEmptyDays(t *testing.T) {
	boots := []span{{at(10, 9, 0), at(10, 12, 0)}, {at(10, 11, 0), at(10, 13, 0)}}
	days := onTimeByDay(boots, nil, at(9, 0, 0), at(11, 0, 0))
	got := hoursOf(days)
	if len(days) != 2 || got["2026-11-09"] != 0 || got["2026-11-10"] != 4 {
		t.Fatalf("got %v, want 0 h then 4 h", days)
	}
}

// A sleep that failed never happened, and a resume only closes a sleep entered
// in the same boot.
func TestPairSleepsIgnoresFailedAndCrossBoot(t *testing.T) {
	events := []sleepEvent{
		{at(10, 1, 0), "a", "enter"},
		{at(10, 1, 1), "a", "failed"},
		{at(10, 2, 0), "a", "enter"},
		{at(10, 3, 0), "a", "resume"},
		{at(10, 4, 0), "a", "enter"},
		{at(10, 5, 0), "b", "resume"},
	}
	got := pairSleeps(events)
	if len(got) != 1 || !got[0].from.Equal(at(10, 2, 0)) || !got[0].to.Equal(at(10, 3, 0)) {
		t.Fatalf("got %v, want only the 02:00 to 03:00 sleep", got)
	}
}

// The running boot lasts until now, whatever its last journal entry says.
func TestParseBootsRunsTheCurrentBootToNow(t *testing.T) {
	now := at(15, 12, 0)
	raw := []byte(`[{"index":-1,"first_entry":` + micro(at(14, 8, 0)) + `,"last_entry":` + micro(at(14, 10, 0)) +
		`},{"index":0,"first_entry":` + micro(at(15, 9, 0)) + `,"last_entry":` + micro(at(15, 9, 30)) + `}]`)
	boots, err := parseBoots(raw, now)
	if err != nil || len(boots) != 2 || !boots[1].to.Equal(now) || !boots[0].to.Equal(at(14, 10, 0)) {
		t.Fatalf("got %v, %v", boots, err)
	}
}

func micro(t time.Time) string {
	return strconv.FormatInt(t.UnixMicro(), 10)
}
