package remind_test

import (
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

	"ryoku-rashin/internal/oni/remind"
	"ryoku-rashin/internal/oni/voice/route"
)

// Every confirmation and refusal is one short sentence, as anything Oni says
// aloud must be, and a fire line leads with one.
func TestLinesKeepTheVoiceDiscipline(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 23, 10, 0, 0, 0, loc)
	for _, said := range []string{
		"remind me in 20 minutes to stretch",
		"remind me in 2 hours and 15 minutes",
		"remind me in 3 days",
		"remind me in a week",
		"remind me next Monday at 8:45",
		"remind me on Friday at 9:30 pm",
		"remind me tomorrow at midnight",
		"remind me the day after tomorrow at noon",
		"remind me at 4",
		"set a timer for an hour and a half",
		"set a timer for 45 seconds",
		"remind me in 8 days",
		"remind me today at 9 am",
		"set a timer for 25 hours",
	} {
		r, line, ok := remind.Parse(said, now, loc)
		if !ok {
			t.Errorf("%q not claimed", said)
			continue
		}
		if !route.Fits(line) {
			t.Errorf("%q: %q does not fit", said, line)
		}
		if r.Kind == "" {
			continue
		}
		for _, at := range []time.Time{r.Due, r.Due.Add(time.Hour)} {
			fire := remind.FireLine(r, at)
			if route.LeadWords(fire) > route.MaxLeadWords {
				t.Errorf("%q fires as %q", said, fire)
			}
		}
	}
	long := remind.Reminder{Kind: remind.KindReminder, Due: now, Text: strings.Repeat("water the plants on the balcony ", 6)}
	if route.LeadWords(remind.FireLine(long, now)) > route.MaxLeadWords {
		t.Error("long words lengthen the lead")
	}
}
