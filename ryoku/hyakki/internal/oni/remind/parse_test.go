package remind

import (
	"strings"
	"testing"
	"time"
	_ "time/tzdata"
)

func zone(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// wallAt is a wall time in loc, "2006-01-02 15:04" with optional seconds.
func wallAt(t *testing.T, loc *time.Location, s string) time.Time {
	t.Helper()
	v, err := time.ParseInLocation("2006-01-02 15:04:05", s, loc)
	if err != nil {
		v, err = time.ParseInLocation("2006-01-02 15:04", s, loc)
	}
	if err != nil {
		t.Fatal(err)
	}
	return v
}

type parseCase struct {
	said string
	due  string // wall time in loc
	text string
	line string
}

func runParse(t *testing.T, now time.Time, loc *time.Location, kind string, cases []parseCase) {
	t.Helper()
	for _, c := range cases {
		r, line, ok := Parse(c.said, now, loc)
		if !ok {
			t.Errorf("%q: not claimed", c.said)
			continue
		}
		if r.Kind != kind {
			t.Errorf("%q: kind %q, line %q", c.said, r.Kind, line)
			continue
		}
		if want := wallAt(t, loc, c.due); !r.Due.Equal(want) {
			t.Errorf("%q: due %s, want %s", c.said, r.Due.In(loc).Format(time.RFC3339), want.Format(time.RFC3339))
		}
		if r.Text != c.text {
			t.Errorf("%q: text %q, want %q", c.said, r.Text, c.text)
		}
		if c.line != "" && line != c.line {
			t.Errorf("%q: line %q, want %q", c.said, line, c.line)
		}
		if !validID(r.ID) || !r.Created.Equal(now) {
			t.Errorf("%q: id %q created %v", c.said, r.ID, r.Created)
		}
	}
}

// Monday 23 November 2026, 10:00 in New York.
func monday(t *testing.T) (time.Time, *time.Location) {
	loc := zone(t)
	return wallAt(t, loc, "2026-11-23 10:00"), loc
}

func TestParseDurations(t *testing.T) {
	now, loc := monday(t)
	runParse(t, now, loc, KindReminder, []parseCase{
		{"remind me in 20 minutes to stretch", "2026-11-23 10:20", "stretch", "Reminder in 20 minutes."},
		{"Remind me to stretch in twenty minutes.", "2026-11-23 10:20", "stretch", "Reminder in 20 minutes."},
		{"remind me in an hour to call Sam", "2026-11-23 11:00", "call Sam", "Reminder in 1 hour."},
		{"remind me in half an hour", "2026-11-23 10:30", "", "Reminder in 30 minutes."},
		{"remind me in an hour and a half to leave", "2026-11-23 11:30", "leave", "Reminder in 90 minutes."},
		{"remind me in one and a half hours", "2026-11-23 11:30", "", ""},
		{"remind me in 1.5 hours", "2026-11-23 11:30", "", ""},
		{"remind me in 2 hours and 15 minutes to check the oven", "2026-11-23 12:15", "check the oven", "Reminder in 135 minutes."},
		{"remind me in forty five seconds", "2026-11-23 10:00:45", "", "Reminder in 45 seconds."},
		{"remind me in a couple of minutes", "2026-11-23 10:02", "", ""},
		{"remind me in a quarter of an hour to flip the bread", "2026-11-23 10:15", "flip the bread", ""},
		{"remind me in ninety nine minutes", "2026-11-23 11:39", "", ""},
		{"in 10 minutes remind me to stretch", "2026-11-23 10:10", "stretch", ""},
		{"remind me 5 minutes from now to stand up", "2026-11-23 10:05", "stand up", ""},
		{"Can you please remind me in 5 minutes' time to text Ana, thanks", "2026-11-23 10:05", "text Ana", ""},
		{"set a reminder for 10 minutes", "2026-11-23 10:10", "", ""},
		{"remind me in 3 days to renew the permit", "2026-11-26 10:00", "renew the permit", "Reminder, Thursday 10 AM."},
		{"remind me in a week to water the fern", "2026-11-30 10:00", "water the fern", "Reminder, next Monday 10 AM."},
		{"remind me in 7 days", "2026-11-30 10:00", "", ""},
	})
}

func TestParseClockTimes(t *testing.T) {
	now, loc := monday(t)
	runParse(t, now, loc, KindReminder, []parseCase{
		{"set a reminder for 5 pm to call mom", "2026-11-23 17:00", "call mom", "Reminder, 5 PM."},
		{"remind me at 4 to water the plants", "2026-11-23 16:00", "water the plants", "Reminder, 4 PM."},
		{"remind me at 9 to lock up", "2026-11-23 21:00", "lock up", ""},
		{"remind me at 10 to lock up", "2026-11-23 22:00", "lock up", ""},
		{"remind me at 11 to lock up", "2026-11-23 11:00", "lock up", "Reminder, 11 AM."},
		{"remind me at half past four to leave", "2026-11-23 16:30", "leave", "Reminder, 4:30 PM."},
		{"remind me at a quarter to five", "2026-11-23 16:45", "", ""},
		{"remind me at ten past six", "2026-11-23 18:10", "", ""},
		{"remind me at twenty five to nine pm", "2026-11-23 20:35", "", ""},
		{"remind me at four thirty pm", "2026-11-23 16:30", "", ""},
		{"remind me at nine oh five", "2026-11-23 21:05", "", ""},
		{"remind me at 7 in the morning", "2026-11-24 07:00", "", "Reminder, tomorrow 7 AM."},
		{"remind me at 16:45 to take the bread out", "2026-11-23 16:45", "take the bread out", ""},
		{"remind me at 0930", "2026-11-24 09:30", "", ""},
		{"remind me at 6 o'clock", "2026-11-23 18:00", "", ""},
		{"remind me to feed the cat at 5:15 p.m.", "2026-11-23 17:15", "feed the cat", ""},
		{"remind me at noon to eat", "2026-11-23 12:00", "eat", "Reminder, noon."},
		{"remind me at midnight to sleep", "2026-11-24 00:00", "sleep", "Reminder, midnight."},
		{"remind me today at 9", "2026-11-23 21:00", "", ""},
		{"remind me at 12 to eat", "2026-11-23 12:00", "eat", ""},
	})
}

func TestParseDays(t *testing.T) {
	now, loc := monday(t)
	runParse(t, now, loc, KindReminder, []parseCase{
		{"remind me tomorrow at 9:30 to call the bank", "2026-11-24 09:30", "call the bank", "Reminder, tomorrow 9:30 AM."},
		{"remind me tomorrow to call Sam at 9", "2026-11-24 09:00", "call Sam", ""},
		{"remind me tomorrow at 2 to call Sam", "2026-11-24 14:00", "call Sam", ""},
		{"remind me tomorrow at 8 to call Sam", "2026-11-24 08:00", "call Sam", ""},
		{"remind me tomorrow morning to pack", "2026-11-24 09:00", "pack", ""},
		{"remind me tomorrow evening", "2026-11-24 18:00", "", "Reminder, tomorrow 6 PM."},
		{"remind me tomorrow", "2026-11-24 09:00", "", ""},
		{"remind me tonight to take out the trash", "2026-11-23 20:00", "take out the trash", "Reminder, 8 PM."},
		{"remind me tonight at 11", "2026-11-23 23:00", "", ""},
		{"remind me tonight at 1 to check the laundry", "2026-11-24 01:00", "check the laundry", ""},
		{"remind me this evening", "2026-11-23 18:00", "", ""},
		{"remind me at 9 a.m. tomorrow", "2026-11-24 09:00", "", ""},
		{"remind me on Friday to pay rent", "2026-11-27 09:00", "pay rent", "Reminder, Friday 9 AM."},
		{"remind me friday afternoon to pay rent", "2026-11-27 15:00", "pay rent", ""},
		{"remind me next Monday at 8 to renew it", "2026-11-30 08:00", "renew it", "Reminder, next Monday 8 AM."},
		{"remind me on Monday at 11", "2026-11-23 11:00", "", ""},
		{"remind me on Monday at 9", "2026-11-30 09:00", "", ""},
		{"remind me the day after tomorrow at noon", "2026-11-25 12:00", "", "Reminder, Wednesday noon."},
		{"remind me tomorrow at midnight", "2026-11-25 00:00", "", "Reminder, tomorrow midnight."},
		{"remind me in 3 days at 8 pm", "2026-11-26 20:00", "", ""},
		{"set a reminder for tomorrow at 7 to stretch", "2026-11-24 07:00", "stretch", ""},
	})
}

func TestParseTimers(t *testing.T) {
	now, loc := monday(t)
	runParse(t, now, loc, KindTimer, []parseCase{
		{"set a timer for 5 minutes", "2026-11-23 10:05", "", "Timer set, 5 minutes."},
		{"set a 10 minute timer", "2026-11-23 10:10", "", "Timer set, 10 minutes."},
		{"start a timer for ninety nine minutes", "2026-11-23 11:39", "", "Timer set, 99 minutes."},
		{"timer for an hour and a half", "2026-11-23 11:30", "", "Timer set, 90 minutes."},
		{"set a timer for 1 second", "2026-11-23 10:00:01", "", "Timer set, 1 second."},
		{"set an hour timer", "2026-11-23 11:00", "", "Timer set, 1 hour."},
		{"set a timer for 5 minutes for the pasta", "2026-11-23 10:05", "the pasta", ""},
		{"please set a timer for 3 minutes for me", "2026-11-23 10:03", "", ""},
		{"set a timer for 24 hours", "2026-11-24 10:00", "", "Timer set, 24 hours."},
	})
}

func TestParseRefusals(t *testing.T) {
	now, loc := monday(t)
	for said, want := range map[string]string{
		"remind me in 8 days to renew the permit": "7 days is my limit.",
		"remind me in 2 weeks":                    "7 days is my limit.",
		"remind me in 169 hours":                  "7 days is my limit.",
		"remind me next Monday at 11":             "7 days is my limit.",
		"remind me today at 9 am":                 "That time has passed.",
		"set a timer for 25 hours":                "Timers stop at 24 hours.",
		"set a timer for 2 days":                  "Timers stop at 24 hours.",
	} {
		r, line, ok := Parse(said, now, loc)
		if !ok || r.Kind != "" || line != want {
			t.Errorf("%q: claimed %v kind %q line %q, want refusal %q", said, ok, r.Kind, line, want)
		}
	}
}

func TestParseLeavesOtherRequests(t *testing.T) {
	now, loc := monday(t)
	for _, said := range []string{
		"remind me",
		"Remind me.",
		"remind me to call mom",
		"remind me about the meeting later",
		"remind me in",
		"remind me in a bit",
		"remind me today",
		"remind me how to set a timer for 5 minutes",
		"remind me what I said at 5 pm",
		"set a timer",
		"what time is it",
		"set an alarm for 7",
		"open the browser",
		"set a timer for zero seconds",
		"remind me in half a second",
		"remind me at 25",
		"remind me at 13 pm",
		"remind me tomorrow in 5 minutes",
		"timer for 5 minutes and also open the music",
		"",
	} {
		if r, line, ok := Parse(said, now, loc); ok {
			t.Errorf("%q claimed: %+v %q", said, r, line)
		}
	}
}

// Midnight: a duration or a clock time said late at night lands on the
// next day, and "tomorrow" said just after midnight is the next date.
func TestParseAcrossMidnight(t *testing.T) {
	loc := zone(t)
	late := wallAt(t, loc, "2026-11-23 23:50")
	runParse(t, late, loc, KindReminder, []parseCase{
		{"remind me in 20 minutes", "2026-11-24 00:10", "", "Reminder in 20 minutes."},
		{"remind me at 12:05 am", "2026-11-24 00:05", "", "Reminder, tomorrow 12:05 AM."},
		{"remind me at midnight", "2026-11-24 00:00", "", "Reminder, midnight."},
		{"remind me tomorrow at 9", "2026-11-24 09:00", "", "Reminder, tomorrow 9 AM."},
		{"remind me at 9", "2026-11-24 09:00", "", ""},
	})
	runParse(t, late, loc, KindTimer, []parseCase{
		{"set a timer for 30 minutes", "2026-11-24 00:20", "", ""},
	})
	early := wallAt(t, loc, "2026-11-24 00:30")
	runParse(t, early, loc, KindReminder, []parseCase{
		{"remind me at 9", "2026-11-24 09:00", "", "Reminder, 9 AM."},
		{"remind me tomorrow at 9", "2026-11-25 09:00", "", ""},
		{"remind me tonight at 10", "2026-11-24 22:00", "", ""},
	})
}

// Daylight saving: clock times keep the wall clock, durations in hours keep
// elapsed time, days keep the hour, a skipped time moves past the gap and a
// repeated one takes its next occurrence.
func TestParseAcrossDaylightSaving(t *testing.T) {
	loc := zone(t)
	// Clocks go forward at 2:00 on Sunday 8 March 2026.
	sat := wallAt(t, loc, "2026-03-07 10:00")
	runParse(t, sat, loc, KindReminder, []parseCase{
		{"remind me tomorrow at 9", "2026-03-08 09:00", "", ""},
		{"remind me tomorrow at 2:30 am", "2026-03-08 03:30", "", ""},
		{"remind me in 1 day", "2026-03-08 10:00", "", ""},
		{"remind me in 24 hours", "2026-03-08 11:00", "", ""},
	})
	if r, _, _ := Parse("remind me tomorrow at 9", sat, loc); r.Due.Sub(sat) != 22*time.Hour {
		t.Errorf("spring forward: %v elapsed, want 22h", r.Due.Sub(sat))
	}
	// Clocks go back at 2:00 on Sunday 1 November 2026; 1:30 happens twice.
	fall := wallAt(t, loc, "2026-10-31 10:00")
	r, _, _ := Parse("remind me tomorrow at 1:30 am", fall, loc)
	if want := time.Date(2026, 11, 1, 5, 30, 0, 0, time.UTC); !r.Due.Equal(want) {
		t.Errorf("first 1:30: %v, want %v", r.Due.UTC(), want)
	}
	between := time.Date(2026, 11, 1, 5, 45, 0, 0, time.UTC) // 1:45 before the change
	r, _, _ = Parse("remind me at 1:30 am", between, loc)
	if want := time.Date(2026, 11, 1, 6, 30, 0, 0, time.UTC); !r.Due.Equal(want) {
		t.Errorf("second 1:30: %v, want %v", r.Due.UTC(), want)
	}
	// "In 7 days" keeps the hour across the change, though it is 169 hours.
	sun := wallAt(t, loc, "2026-10-25 10:00")
	r, line, ok := Parse("remind me in 7 days", sun, loc)
	if !ok || r.Kind != KindReminder || r.Due.Sub(sun) != 169*time.Hour {
		t.Errorf("in 7 days across fall back: %v %q", r.Due.Sub(sun), line)
	}
	// "In 168 hours" stays allowed across spring forward, though the wall
	// clock week is 167 hours.
	march := wallAt(t, loc, "2026-03-01 10:00")
	if r, line, ok := Parse("remind me in 168 hours", march, loc); !ok || r.Kind != KindReminder {
		t.Errorf("168 hours across spring forward refused: %q", line)
	}
	if r, _, ok := Parse("remind me in 169 hours", march, loc); !ok || r.Kind != "" {
		t.Errorf("169 hours across spring forward kept")
	}
}

func TestParseKeepsOnlyTheReminderWords(t *testing.T) {
	now, loc := monday(t)
	r, _, ok := Parse("Hey, could you remind me in ten minutes to stretch my back, please?", now, loc)
	if !ok || r.Text != "stretch my back" {
		t.Fatalf("text %q", r.Text)
	}
	r, _, ok = Parse("remind me to "+strings.Repeat("buy more bread ", 40)+"in 5 minutes", now, loc)
	if !ok || len([]rune(r.Text)) > MaxTextRunes || !strings.HasSuffix(r.Text, "bread buy") {
		t.Fatalf("long text %d %q", len([]rune(r.Text)), r.Text)
	}
	if r, _, _ := Parse("remind me [BLANK_AUDIO] in 5 minutes to stretch", now, loc); r.Text != "stretch" {
		t.Fatalf("annotation: %q", r.Text)
	}
}

func TestParseCancel(t *testing.T) {
	for said, want := range map[string]CancelKind{
		"cancel the timer":                    CancelTimers,
		"Stop my timer.":                      CancelTimers,
		"cancel all timers":                   CancelTimers,
		"never mind the timer":                CancelTimers,
		"cancel my reminder":                  CancelReminder,
		"cancel the reminder please":          CancelReminder,
		"cancel my reminders":                 CancelReminders,
		"clear all my reminders":              CancelReminders,
		"get rid of all the reminders":        CancelReminders,
		"cancel all my reminders and timers":  CancelAll,
		"cancel the timers and the reminders": CancelAll,
	} {
		if got, ok := ParseCancel(said); !ok || got != want {
			t.Errorf("%q: %q %v, want %q", said, got, ok, want)
		}
	}
	for _, said := range []string{"cancel", "cancel the meeting", "stop the music", "cancel the timer for the pasta tomorrow", "cancel my timers and timers", "remind me to cancel the timer"} {
		if got, ok := ParseCancel(said); ok {
			t.Errorf("%q claimed as %q", said, got)
		}
	}
}

func TestParseList(t *testing.T) {
	for _, said := range []string{
		"what are my reminders", "What are my reminders?", "list my reminders", "show me my timers",
		"do I have any reminders", "what reminders do I have", "any reminders", "my reminders",
		"how much time is left on the timer", "how long is left", "what's left on my timer",
		"tell me all my reminders and timers", "are there any timers set",
	} {
		if !ParseList(said) {
			t.Errorf("%q not a list request", said)
		}
	}
	for _, said := range []string{"remind me", "what time is it", "what are my plans", "how much time is left in the year", "list my files"} {
		if ParseList(said) {
			t.Errorf("%q taken as a list request", said)
		}
	}
}

func TestLateAndFireLines(t *testing.T) {
	now, _ := monday(t)
	r := Reminder{ID: newID(), Kind: KindReminder, Due: now, Created: now.Add(-time.Hour), Text: "call the bank"}
	if r.Late(now) || r.Late(now.Add(LateAfter)) || !r.Late(now.Add(LateAfter+time.Nanosecond)) {
		t.Fatal("late boundary")
	}
	if LateAfter < 4*15*time.Second {
		t.Fatalf("LateAfter %v is shorter than the busy waits", LateAfter)
	}
	if got := FireLine(r, now.Add(30*time.Second)); got != "Reminder. Call the bank." {
		t.Errorf("on time: %q", got)
	}
	if got := FireLine(r, now.Add(time.Hour)); got != "Reminder, a bit late. Call the bank." {
		t.Errorf("late: %q", got)
	}
	timer := Reminder{ID: newID(), Kind: KindTimer, Due: now, Created: now.Add(-time.Minute)}
	if got := FireLine(timer, now); got != "Time's up." {
		t.Errorf("timer: %q", got)
	}
	if got := FireLine(timer, now.Add(time.Hour)); got != "Your timer ended earlier." {
		t.Errorf("late timer: %q", got)
	}
}

func TestListLine(t *testing.T) {
	now, loc := monday(t)
	if got := ListLine(nil, now, loc); got != "No reminders or timers." {
		t.Errorf("empty: %q", got)
	}
	rs := []Reminder{
		{Kind: KindReminder, Due: now.Add(-time.Minute), Text: "water the fern"},
		{Kind: KindTimer, Due: now.Add(4*time.Minute + 10*time.Second)},
		{Kind: KindReminder, Due: wallAt(t, loc, "2026-11-24 09:30"), Text: "call the bank"},
	}
	want := "2 reminders and 1 timer. Water the fern, overdue. Timer, 5 minutes left. Call the bank, tomorrow 9:30 AM."
	if got := ListLine(rs, now, loc); got != want {
		t.Errorf("list:\n got %q\nwant %q", got, want)
	}
}
