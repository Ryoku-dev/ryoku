package cron

import (
	"testing"
	"time"
	_ "time/tzdata"
)

func TestParseErrors(t *testing.T) {
	for _, test := range []struct {
		expr, want string
	}{
		{"", "A schedule has five parts: minute, hour, day of the month, month and day of the week."},
		{"* * * *", "A schedule has five parts: minute, hour, day of the month, month and day of the week."},
		{"* * * * * *", "A schedule has five parts: minute, hour, day of the month, month and day of the week."},
		{"60 * * * *", "The minute must be between 0 and 59."},
		{"abc * * * *", "The minute must be between 0 and 59."},
		{"* 24 * * *", "The hour must be between 0 and 23."},
		{"* -1 * * *", "The hour must be between 0 and 23."},
		{"* * 0 * *", "The day of the month must be between 1 and 31."},
		{"* * 32 * *", "The day of the month must be between 1 and 31."},
		{"* * * 13 *", "The month must be between 1 and 12, or jan to dec."},
		{"* * * foo *", "The month must be between 1 and 12, or jan to dec."},
		{"* * * * 8", "The day of the week must be between 0 and 7, or sun to sat."},
		{"* * * * monday", "The day of the week must be between 0 and 7, or sun to sat."},
		{"5-1 * * * *", "A range goes from the smaller value to the larger, such as 1-5."},
		{"*/0 * * * *", "A step after / must be a whole number of 1 or more."},
		{"*/x * * * *", "A step after / must be a whole number of 1 or more."},
		{"5/2 * * * *", "A step needs * or a range before it, such as */15 or 0-30/5."},
		{"1,,2 * * * *", "The minute must be between 0 and 59."},
		{"@yearly", "The shortcuts are @hourly, @daily, @weekly and @monthly."},
		{"0 0 30 2 *", "That day never comes in the months chosen, so this schedule would never run."},
		{"0 0 31 4,6,9,11 *", "That day never comes in the months chosen, so this schedule would never run."},
	} {
		_, err := Parse(test.expr)
		if err == nil {
			t.Errorf("Parse(%q) succeeded, want %q", test.expr, test.want)
			continue
		}
		if err.Error() != test.want {
			t.Errorf("Parse(%q) = %q, want %q", test.expr, err.Error(), test.want)
		}
	}
}

func at(t *testing.T, loc *time.Location, text string) time.Time {
	t.Helper()
	v, err := time.ParseInLocation("2006-01-02 15:04", text, loc)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func zone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// onClock runs the rest of the test with loc as the computer's clock.
func onClock(t *testing.T, loc *time.Location) {
	t.Helper()
	previous := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = previous })
}

// 2026-11-11 is a Wednesday.
func TestNextFieldShapes(t *testing.T) {
	onClock(t, time.UTC)
	for _, test := range []struct {
		expr, after string
		want        []string
	}{
		{"* * * * *", "2026-11-11 08:00", []string{"2026-11-11 08:01", "2026-11-11 08:02"}},
		{"30 9 * * *", "2026-11-11 08:00", []string{"2026-11-11 09:30", "2026-11-12 09:30"}},
		{"30 9 * * *", "2026-11-11 09:30", []string{"2026-11-12 09:30"}},
		{"*/15 * * * *", "2026-11-11 08:07", []string{"2026-11-11 08:15", "2026-11-11 08:30", "2026-11-11 08:45", "2026-11-11 09:00"}},
		{"10-20/5 8 * * *", "2026-11-11 08:12", []string{"2026-11-11 08:15", "2026-11-11 08:20", "2026-11-12 08:10"}},
		{"0,30 9,17 * * *", "2026-11-11 09:00", []string{"2026-11-11 09:30", "2026-11-11 17:00", "2026-11-11 17:30", "2026-11-12 09:00"}},
		{"0 12-14 * * *", "2026-11-11 12:00", []string{"2026-11-11 13:00", "2026-11-11 14:00", "2026-11-12 12:00"}},
		{"0 8 * * mon-fri", "2026-11-13 09:00", []string{"2026-11-16 08:00", "2026-11-17 08:00"}},
		{"0 8 * * MON,thu", "2026-11-11 09:00", []string{"2026-11-12 08:00", "2026-11-16 08:00"}},
		{"0 8 * * 7", "2026-11-11 09:00", []string{"2026-11-15 08:00", "2026-11-22 08:00"}},
		{"0 8 * * 5-7", "2026-11-11 09:00", []string{"2026-11-13 08:00", "2026-11-14 08:00", "2026-11-15 08:00", "2026-11-20 08:00"}},
		{"0 7 1 * *", "2026-11-11 09:00", []string{"2026-12-01 07:00", "2027-01-01 07:00"}},
		{"0 0 1 jan,Jul *", "2026-11-11 09:00", []string{"2027-01-01 00:00", "2027-07-01 00:00"}},
		{"0 0 31 * *", "2026-11-11 09:00", []string{"2026-12-31 00:00", "2027-01-31 00:00", "2027-03-31 00:00"}},
		{"0 0 29 2 *", "2026-11-11 09:00", []string{"2028-02-29 00:00", "2032-02-29 00:00"}},
		{"@hourly", "2026-11-11 08:59", []string{"2026-11-11 09:00", "2026-11-11 10:00"}},
		{"@daily", "2026-11-11 08:59", []string{"2026-11-12 00:00"}},
		{"@weekly", "2026-11-11 08:59", []string{"2026-11-15 00:00"}},
		{"@monthly", "2026-11-11 08:59", []string{"2026-12-01 00:00"}},
	} {
		s, err := Parse(test.expr)
		if err != nil {
			t.Fatalf("Parse(%q): %v", test.expr, err)
		}
		from := at(t, time.UTC, test.after)
		for _, want := range test.want {
			got := s.Next(from)
			if !got.Equal(at(t, time.UTC, want)) {
				t.Errorf("%q after %s = %s, want %s", test.expr, from.Format("2006-01-02 15:04"), got.Format("2006-01-02 15:04"), want)
				break
			}
			from = got
		}
	}
}

func TestNextDayRule(t *testing.T) {
	onClock(t, time.UTC)
	for _, test := range []struct {
		expr, after string
		want        []string
	}{
		// both day fields restricted: either matches
		{"0 0 15 * 1", "2026-11-11 00:00", []string{"2026-11-15 00:00", "2026-11-16 00:00", "2026-11-23 00:00"}},
		{"0 0 13 * fri", "2026-11-01 00:00", []string{"2026-11-06 00:00", "2026-11-13 00:00", "2026-11-20 00:00"}},
		// a leading * keeps both rules: odd days that are Mondays
		{"0 0 */2 * 1", "2026-11-01 00:00", []string{"2026-11-09 00:00", "2026-11-23 00:00"}},
		// * in the day of the month: only the day of the week counts
		{"0 0 * * 1", "2026-11-01 00:00", []string{"2026-11-02 00:00", "2026-11-09 00:00"}},
		// a day of the week written 0-6 is restricted, so it takes every day
		{"0 0 15 * 0-6", "2026-11-11 00:00", []string{"2026-11-12 00:00"}},
	} {
		s, err := Parse(test.expr)
		if err != nil {
			t.Fatalf("Parse(%q): %v", test.expr, err)
		}
		from := at(t, time.UTC, test.after)
		for _, want := range test.want {
			got := s.Next(from)
			if !got.Equal(at(t, time.UTC, want)) {
				t.Errorf("%q after %s = %s, want %s", test.expr, from.Format("2006-01-02 15:04"), got.Format("2006-01-02 15:04"), want)
				break
			}
			from = got
		}
	}
}

func TestNextAcrossDaylightSaving(t *testing.T) {
	berlin := zone(t, "Europe/Berlin")
	york := zone(t, "America/New_York")
	for _, test := range []struct {
		name  string
		loc   *time.Location
		expr  string
		after string
		// want are UTC instants
		want []string
	}{
		// 2026-03-29 02:00 CET jumps to 03:00 CEST: 02:30 fires at 03:00
		{"berlin skipped", berlin, "30 2 * * *", "2026-03-28 03:00", []string{"2026-03-29 01:00", "2026-03-30 00:30"}},
		// the quarter hours inside the gap fire once, at the end of the gap
		{"berlin skipped steps", berlin, "*/15 * * * *", "2026-03-29 01:45", []string{"2026-03-29 01:00", "2026-03-29 01:15"}},
		// 2026-10-25 03:00 CEST goes back to 02:00 CET: 02:30 fires once
		{"berlin repeated", berlin, "30 2 * * *", "2026-10-24 03:00", []string{"2026-10-25 00:30", "2026-10-26 01:30"}},
		{"berlin repeated steps", berlin, "*/30 * * * *", "2026-10-25 01:50", []string{"2026-10-25 00:00", "2026-10-25 00:30", "2026-10-25 02:00", "2026-10-25 02:30"}},
		// 2026-03-08 02:00 EST jumps to 03:00 EDT
		{"new york skipped", york, "30 2 * * *", "2026-03-07 12:00", []string{"2026-03-08 07:00", "2026-03-09 06:30"}},
		// 2026-11-01 02:00 EDT goes back to 01:00 EST: 01:30 fires once
		{"new york repeated", york, "30 1 * * *", "2026-10-31 12:00", []string{"2026-11-01 05:30", "2026-11-02 06:30"}},
		{"new york hourly", york, "0 * * * *", "2026-11-01 00:30", []string{"2026-11-01 05:00", "2026-11-01 07:00", "2026-11-01 08:00"}},
	} {
		s, err := Parse(test.expr)
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		onClock(t, test.loc)
		from := at(t, test.loc, test.after)
		for _, want := range test.want {
			got := s.Next(from)
			if !got.Equal(at(t, time.UTC, want)) {
				t.Errorf("%s: after %s = %s, want %s UTC", test.name, from, got.UTC().Format("2006-01-02 15:04"), want)
				break
			}
			if got.Location() != test.loc {
				t.Errorf("%s: next is in %s, want %s", test.name, got.Location(), test.loc)
			}
			from = got
		}
	}
}

// A time saved in winter keeps its winter offset when read back; the next
// run still lands on the summer wall clock.
func TestNextIgnoresASavedOffset(t *testing.T) {
	onClock(t, zone(t, "America/New_York"))
	s, err := Parse("0 9 * * *")
	if err != nil {
		t.Fatal(err)
	}
	saved := time.Date(2026, time.June, 1, 12, 0, 0, 0, time.FixedZone("", -5*3600))
	if got, want := s.Next(saved), at(t, time.UTC, "2026-06-02 13:00"); !got.Equal(want) {
		t.Fatalf("next = %s, want %s", got.UTC(), want)
	}
}

func TestSentence(t *testing.T) {
	for _, test := range []struct {
		expr, want string
	}{
		{"30 * * * *", "Every hour at :30"},
		{"0 9 * * *", "Every day at 09:00"},
		{"0 9 * * 1-5", "Every weekday at 09:00"},
		{"0 8 * * 1", "Every Monday at 08:00"},
		{"0 8 * * mon", "Every Monday at 08:00"},
		{"0 8 * * 1,4", "Every week on Mon and Thu at 08:00"},
		{"0 10 * * 0,6", "Every week on Sat and Sun at 10:00"},
		{"0 7 1 * *", "On day 1 of every month at 07:00"},
		{"0 7 1,15 * *", "On days 1 and 15 of every month at 07:00"},
		{"15 9,17 * 3 1-5", "At 09:15 and 17:15 on weekdays in March"},
		{"0 9,13,17 * * *", "Every day at 09:00, 13:00 and 17:00"},
		{"*/15 * * * *", "Every 15 minutes"},
		{"* * * * *", "Every minute"},
		{"0,30 * * * *", "Every 30 minutes"},
		{"5,35 * * * *", "Every hour at :05 and :35"},
		{"*/30 9-17 * * 1-5", "Every 30 minutes from 09:00 to 17:30 on weekdays"},
		{"0 7-11,14-18 * * *", "At :00 past each hour from 07:00 to 11:00 and 14:00 to 18:00"},
		{"0 9 1 * 1", "At 09:00 on day 1 of the month or on Mon"},
		{"0 9 * 1,7 *", "At 09:00 every day in January and July"},
		{"0 0 * * 0,6", "Every week on Sat and Sun at 00:00"},
		{"0 * * * 0,6", "Every hour at :00 on weekends"},
		{"@weekly", "Every Sunday at 00:00"},
		{"@monthly", "On day 1 of every month at 00:00"},
	} {
		s, err := Parse(test.expr)
		if err != nil {
			t.Fatalf("Parse(%q): %v", test.expr, err)
		}
		if got := s.Sentence(); got != test.want {
			t.Errorf("Sentence(%q) = %q, want %q", test.expr, got, test.want)
		}
	}
}

func TestMinInterval(t *testing.T) {
	for _, test := range []struct {
		expr string
		want time.Duration
	}{
		{"* * * * *", time.Minute},
		{"*/5 * * * *", 5 * time.Minute},
		{"0,50 * * * *", 10 * time.Minute},
		{"0 * * * *", time.Hour},
		{"0 9 * * *", 24 * time.Hour},
		{"0 9,17 * * *", 8 * time.Hour},
		{"0 23 * * 1", 7 * 24 * time.Hour},
		{"0 8 * * 1,4", 3 * 24 * time.Hour},
		{"0 23,1 * * 1,2", 2 * time.Hour},
		{"*/15 9-17 * * *", 15 * time.Minute},
	} {
		s, err := Parse(test.expr)
		if err != nil {
			t.Fatalf("Parse(%q): %v", test.expr, err)
		}
		if got := s.MinInterval(); got != test.want {
			t.Errorf("MinInterval(%q) = %s, want %s", test.expr, got, test.want)
		}
	}
}

func TestPreview(t *testing.T) {
	onClock(t, time.UTC)
	from := at(t, time.UTC, "2026-11-11 08:00")
	p := Preview("0 9 * * 1-5", from, 0)
	if p.Error != "" || p.Sentence != "Every weekday at 09:00" || len(p.Next) != PreviewCount {
		t.Fatalf("preview = %+v", p)
	}
	if !p.Next[2].Equal(at(t, time.UTC, "2026-11-13 09:00")) || !p.Next[3].Equal(at(t, time.UTC, "2026-11-16 09:00")) {
		t.Fatalf("preview runs = %v", p.Next)
	}
	if p := Preview("* * * * *", from, 100); len(p.Next) != PreviewMax {
		t.Fatalf("preview count = %d, want %d", len(p.Next), PreviewMax)
	}
	if p := Preview("0 25 * * *", from, 3); p.Error != "The hour must be between 0 and 23." || p.Sentence != "" || len(p.Next) != 0 {
		t.Fatalf("bad preview = %+v", p)
	}
}
