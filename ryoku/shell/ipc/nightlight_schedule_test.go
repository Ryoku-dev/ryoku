package main

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func dayWindow(t *testing.T) (sunriseISO, sunsetISO string) {
	t.Helper()
	// A generic mid-latitude day: sunrise 06:30, sunset 19:30, local ISO --
	// the exact shape the forecast carries.
	now := time.Now()
	f := func(h, m int) string {
		d := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, time.Local)
		return d.Format("2006-01-02T15:04")
	}
	return f(6, 30), f(19, 30)
}

func TestIsDaytime(t *testing.T) {
	sunriseISO, sunsetISO := dayWindow(t)
	sr, ok1 := parseISOTime(sunriseISO)
	ss, ok2 := parseISOTime(sunsetISO)
	if !ok1 || !ok2 {
		t.Fatalf("window strings must parse: %v %v", ok1, ok2)
	}
	cases := []struct {
		name string
		hour int
		want bool
	}{
		{"before sunrise is night", 5, false},
		{"mid-morning is day", 9, true},
		{"midday is day", 12, true},
		{"mid-afternoon is day", 16, true},
		{"an hour before sunset is day", 18, true},
		{"an hour after sunset is night", 20, false},
		{"midnight is night", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			now := time.Now()
			at := time.Date(now.Year(), now.Month(), now.Day(), c.hour, 45, 0, 0, time.Local)
			if got := isDaytime(at, sr, ss); got != c.want {
				t.Errorf("isDaytime(%v) = %v, want %v", at.Format("15:04"), got, c.want)
			}
		})
	}
}

func TestNocturnalWindowSpansMidnight(t *testing.T) {
	sunriseISO, sunsetISO := dayWindow(t)
	sr, _ := parseISOTime(sunriseISO)
	ss, _ := parseISOTime(sunsetISO)
	now := time.Now()
	at := func(hour int) time.Time {
		return time.Date(now.Year(), now.Month(), now.Day(), hour, 45, 0, 0, time.Local)
	}
	cases := []struct {
		name   string
		at     time.Time
		margin time.Duration
		want   bool
	}{
		{"evening after the margin opens the window", at(20), time.Hour, true},
		{"evening inside the margin stays off", at(20), 90 * time.Minute, false},
		{"deep night is on", at(2), time.Hour, true},
		{"morning before the margin-close stays on", at(4), time.Hour, true},
		{"morning inside the margin turns off", at(5), time.Hour, false},
		{"daytime is off", at(12), time.Hour, false},
		{"a margin longer than the night collapses it", at(2), 13 * time.Hour, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := inNocturnalWindow(c.at, sr, ss, c.margin); got != c.want {
				t.Errorf("inNocturnalWindow(%v, margin=%v) = %v, want %v",
					c.at.Format("15:04"), c.margin, got, c.want)
			}
		})
	}
}

func TestNightlightScheduleRoundTrip(t *testing.T) {
	dir := t.TempDir()
	n := &nightlightState{
		stateDir:       dir,
		schedFile:      filepath.Join(dir, "ryoku-nightlight-schedule.json"),
		schedMode:      nlSchedOff,
		schedMarginMin: nlDefaultMarginMin,
	}
	if got := n.scheduleMode(); got != nlSchedOff {
		t.Fatalf("default mode = %q, want off", got)
	}
	margin := 30
	if err := n.setSchedule(nlSchedSun, &margin); err != nil {
		t.Fatalf("setSchedule: %v", err)
	}
	if got := n.scheduleMode(); got != nlSchedSun {
		t.Fatalf("mode = %q, want sun", got)
	}
	// A fresh daemon (restart) reads the same schedule back.
	m := &nightlightState{schedFile: n.schedFile, schedMode: nlSchedOff, schedMarginMin: nlDefaultMarginMin}
	m.loadSchedule()
	if m.scheduleMode() != nlSchedSun || m.schedMarginMin != 30 {
		t.Fatalf("reloaded schedule = %q/%d, want sun/30", m.scheduleMode(), m.schedMarginMin)
	}
	if err := n.setSchedule("eclipse", nil); err != errNightlightSchedule {
		t.Fatalf("unknown mode error = %v, want the named refusal", err)
	}
}

// The schedule fails safe with no sun data: a tick must not publish until the
// weather poll has observed a real window.
func TestNightlightTickFailsSafeWithoutSun(t *testing.T) {
	dir := t.TempDir()
	topic := newStateTopic()
	sub := topic.subscribe()
	defer topic.unsubscribe(sub)
	if daySun != nil && daySun.windowObserved() {
		t.Skip("package sun window already observed by another test")
	}
	n := &nightlightState{
		topic:          topic,
		schedFile:      filepath.Join(dir, "ryoku-nightlight-schedule.json"),
		schedMode:      nlSchedSun,
		schedMarginMin: 0,
	}
	n.tickSchedule()
	select {
	case f := <-sub.frames:
		t.Fatalf("tick with no sun data published a frame: %s", f)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestNightlightScheduleFrameShape(t *testing.T) {
	dir := t.TempDir()
	topic := newStateTopic()
	sub := topic.subscribe()
	defer topic.unsubscribe(sub)
	n := &nightlightState{
		topic:          topic,
		tempFile:       filepath.Join(dir, "ryoku-nightlight"),
		schedFile:      filepath.Join(dir, "ryoku-nightlight-schedule.json"),
		schedMode:      nlSchedSun,
		schedMarginMin: 45,
	}
	n.publish(true)
	var got struct {
		On          bool   `json:"on"`
		Temperature int    `json:"temperature"`
		Schedule    string `json:"schedule"`
		MarginMin   int    `json:"marginMin"`
	}
	if err := json.Unmarshal(<-sub.frames, &got); err != nil {
		t.Fatalf("frame not json: %v", err)
	}
	if !got.On || got.Schedule != "sun" || got.MarginMin != 45 {
		t.Fatalf("frame = %+v, want on/sun/45", got)
	}
}
