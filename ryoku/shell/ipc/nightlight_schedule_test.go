package main

import (
	"encoding/json"
	"path/filepath"
	wm "ryoku-wm"
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
	if err := n.setSchedule(nlSchedSun, &margin, nil, nil); err != nil {
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
	if err := n.setSchedule("eclipse", nil, nil, nil); err != errNightlightSchedule {
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

func TestInClockWindow(t *testing.T) {
	now := time.Now()
	at := func(h, m int) (int, int) { return h*60 + m, h*60 + m }
	_ = at
	cur := now.Hour()*60 + now.Minute()
	// A window that contains the current minute, built around now so the
	// test does not depend on when it runs.
	inStart := (cur - 120 + 24*60) % (24 * 60)
	inStop := (cur + 120) % (24 * 60)
	if !inClockWindow(now, inStart, inStop) {
		t.Fatalf("now must sit inside the window built around it: %d..%d", inStart, inStop)
	}
	// The complementary window excludes now: start after it, stop before it.
	outStart := (cur + 1) % (24 * 60)
	outStop := (cur - 1 + 24*60) % (24 * 60)
	if inClockWindow(now, outStart, outStop) {
		t.Fatal("now must sit outside the complementary window")
	}
	// Equal bounds mean all day.
	if !inClockWindow(now, cur, cur) {
		t.Fatal("equal bounds must mean the light is wanted all day")
	}
	// Half-open edges: the start minute is in, the stop minute is out.
	if !inClockWindow(time.Date(2026, 1, 1, 22, 0, 0, 0, time.Local), 22*60, 7*60) {
		t.Fatal("the start minute belongs to the window")
	}
	if inClockWindow(time.Date(2026, 1, 1, 7, 0, 0, 0, time.Local), 22*60, 7*60) {
		t.Fatal("the stop minute ends the window")
	}
}

func TestNextNightlightScheduleEdge(t *testing.T) {
	now := time.Date(2026, 1, 1, 21, 30, 15, 0, time.Local)
	if got, want := nextClockScheduleEdge(now, 22*60, 7*60), 29*time.Minute+45*time.Second; got != want {
		t.Fatalf("next clock edge = %v, want %v", got, want)
	}
	if got := nextClockScheduleEdge(now, 22*60, 22*60); got != nlSlowReconcile {
		t.Fatalf("all-day clock window wait = %v, want %v", got, nlSlowReconcile)
	}
	sunrise := time.Date(2026, 1, 1, 6, 30, 0, 0, time.Local)
	sunset := time.Date(2026, 1, 1, 19, 30, 0, 0, time.Local)
	sunNow := time.Date(2026, 1, 1, 18, 15, 0, 0, time.Local)
	if got, want := nextSunScheduleEdge(sunNow, sunrise, sunset, time.Hour), 2*time.Hour+15*time.Minute; got != want {
		t.Fatalf("next sun edge = %v, want %v", got, want)
	}
}

func TestNightlightClockScheduleRoundTrip(t *testing.T) {
	dir := t.TempDir()
	n := &nightlightState{
		stateDir:       dir,
		schedFile:      filepath.Join(dir, "ryoku-nightlight-schedule.json"),
		schedMode:      nlSchedOff,
		schedMarginMin: nlDefaultMarginMin,
		schedStartMin:  nlDefaultClockStart,
		schedStopMin:   nlDefaultClockStop,
	}
	start, stop := 21*60+30, 6*60+45
	if err := n.setSchedule(nlSchedClock, nil, &start, &stop); err != nil {
		t.Fatalf("setSchedule clock: %v", err)
	}
	m := &nightlightState{schedFile: n.schedFile, schedMode: nlSchedOff, schedStartMin: nlDefaultClockStart, schedStopMin: nlDefaultClockStop}
	m.loadSchedule()
	if m.scheduleMode() != nlSchedClock || m.schedStartMin != start || m.schedStopMin != stop {
		t.Fatalf("reloaded clock = %q/%d/%d, want clock/%d/%d", m.scheduleMode(), m.schedStartMin, m.schedStopMin, start, stop)
	}
	// A wild bound clamps into the day instead of wrapping it.
	wild := 9999
	if err := n.setSchedule(nlSchedClock, nil, &wild, nil); err != nil {
		t.Fatalf("setSchedule wild: %v", err)
	}
	if got := n.schedStartMin; got != 23*60+59 {
		t.Fatalf("wild start clamped to %d, want 23:59", got)
	}
}

// The clock window needs no weather: an all-day window turns the light on at
// the next tick, and moving the window away from now turns it off. This is
// the whole feature's edge loop driven through the real intent path.
func TestNightlightClockTickEdges(t *testing.T) {
	f := buildNlFake(t, "nlfakeclock", false)
	topic := newStateTopic()
	sub := topic.subscribe()
	defer topic.unsubscribe(sub)
	n := &nightlightState{
		topic: topic, wmc: wm.OpenNamed("testwm"),
		stateDir: f.state, tempFile: f.temp,
		schedFile:      filepath.Join(f.state, "ryoku-nightlight-schedule.json"),
		schedMode:      nlSchedOff,
		schedMarginMin: nlDefaultMarginMin,
		schedStartMin:  nlDefaultClockStart,
		schedStopMin:   nlDefaultClockStop,
	}

	now := time.Now()
	cur := now.Hour()*60 + now.Minute()
	all := cur
	if err := n.setSchedule(nlSchedClock, nil, &all, &all); err != nil {
		t.Fatalf("setSchedule all-day: %v", err)
	}
	n.tickSchedule()
	if got := waitForFrame(t, sub, func(fr nlFrame) bool { return fr.On }, "on:true"); !got.On {
		t.Fatalf("all-day window must turn the light on: %s", got.Raw)
	}

	outStart := (cur + 1) % (24 * 60)
	outStop := (cur - 1 + 24*60) % (24 * 60)
	if err := n.setSchedule(nlSchedClock, nil, &outStart, &outStop); err != nil {
		t.Fatalf("setSchedule away: %v", err)
	}
	n.tickSchedule()
	if got := waitForFrame(t, sub, func(fr nlFrame) bool { return !fr.On }, "on:false"); got.On {
		t.Fatalf("a window away from now must turn the light off: %s", got.Raw)
	}
}

func TestNightlightScheduleFrameCarriesClock(t *testing.T) {
	dir := t.TempDir()
	topic := newStateTopic()
	sub := topic.subscribe()
	defer topic.unsubscribe(sub)
	n := &nightlightState{
		topic: topic, tempFile: filepath.Join(dir, "ryoku-nightlight"),
		schedFile: filepath.Join(dir, "ryoku-nightlight-schedule.json"),
		schedMode: nlSchedClock, schedStartMin: 21*60 + 15, schedStopMin: 6*60 + 30,
	}
	n.publish(false)
	var got struct {
		Schedule string `json:"schedule"`
		StartAt  int    `json:"startAt"`
		StopAt   int    `json:"stopAt"`
	}
	if err := json.Unmarshal(<-sub.frames, &got); err != nil {
		t.Fatalf("frame not json: %v", err)
	}
	if got.Schedule != "clock" || got.StartAt != 21*60+15 || got.StopAt != 6*60+30 {
		t.Fatalf("frame = %+v, want clock/1275/390", got)
	}
}

// Enabling a schedule while the backend is already running must reconcile it
// on the next tick: a window that wants the light off turns it off right
// away instead of waiting for the next edge, and the pass is one-shot, so a
// settled schedule never fights the state again.
func TestNightlightScheduleReconcilesRunningBackend(t *testing.T) {
	f := buildNlFake(t, "nlfakerecon", false)
	topic := newStateTopic()
	sub := topic.subscribe()
	defer topic.unsubscribe(sub)
	n := &nightlightState{
		topic: topic, wmc: wm.OpenNamed("testwm"),
		stateDir: f.state, tempFile: f.temp,
		schedFile:      filepath.Join(f.state, "ryoku-nightlight-schedule.json"),
		schedMode:      nlSchedOff,
		schedMarginMin: nlDefaultMarginMin,
		schedStartMin:  nlDefaultClockStart,
		schedStopMin:   nlDefaultClockStop,
	}

	// The light is on before the schedule exists, like a manual toggle.
	if err := n.intent("on"); err != nil {
		t.Fatalf("intent on: %v", err)
	}
	if got := waitForFrame(t, sub, func(fr nlFrame) bool { return fr.On }, "on:true"); !got.On {
		t.Fatalf("the backend must be on before the schedule is enabled: %s", got.Raw)
	}

	now := time.Now()
	cur := now.Hour()*60 + now.Minute()
	outStart := (cur + 1) % (24 * 60)
	outStop := (cur - 1 + 24*60) % (24 * 60)
	if err := n.setSchedule(nlSchedClock, nil, &outStart, &outStop); err != nil {
		t.Fatalf("setSchedule daylight window: %v", err)
	}
	n.tickSchedule()
	if got := waitForFrame(t, sub, func(fr nlFrame) bool { return !fr.On }, "on:false"); got.On {
		t.Fatalf("a daylight window must reconcile the running backend off: %s", got.Raw)
	}

	n.tickSchedule()
	select {
	case fr := <-sub.frames:
		t.Fatalf("a settled schedule must not act again: %s", fr)
	case <-time.After(50 * time.Millisecond):
	}
}

// The boot path gets the same reconciliation: a machine that shut down with
// the light on and a daylight schedule persisted comes back up, loads the
// schedule, and turns the light off on the first tick.
func TestNightlightScheduleReconcilesAfterLoad(t *testing.T) {
	f := buildNlFake(t, "nlfakeboot", false)
	topic := newStateTopic()
	sub := topic.subscribe()
	defer topic.unsubscribe(sub)
	n := &nightlightState{
		topic: topic, wmc: wm.OpenNamed("testwm"),
		stateDir: f.state, tempFile: f.temp,
		schedFile:      filepath.Join(f.state, "ryoku-nightlight-schedule.json"),
		schedMode:      nlSchedOff,
		schedMarginMin: nlDefaultMarginMin,
		schedStartMin:  nlDefaultClockStart,
		schedStopMin:   nlDefaultClockStop,
	}

	if err := n.intent("on"); err != nil {
		t.Fatalf("intent on: %v", err)
	}
	if got := waitForFrame(t, sub, func(fr nlFrame) bool { return fr.On }, "on:true"); !got.On {
		t.Fatalf("the backend must be on before the reboot: %s", got.Raw)
	}
	now := time.Now()
	cur := now.Hour()*60 + now.Minute()
	outStart := (cur + 1) % (24 * 60)
	outStop := (cur - 1 + 24*60) % (24 * 60)
	if err := n.setSchedule(nlSchedClock, nil, &outStart, &outStop); err != nil {
		t.Fatalf("setSchedule daylight window: %v", err)
	}
	n.tickSchedule()
	if got := waitForFrame(t, sub, func(fr nlFrame) bool { return !fr.On }, "on:false"); got.On {
		t.Fatalf("the daylight window must be off: %s", got.Raw)
	}

	// A manual toggle during the day still wins over the schedule.
	if err := n.intent("on"); err != nil {
		t.Fatalf("intent on: %v", err)
	}
	if got := waitForFrame(t, sub, func(fr nlFrame) bool { return fr.On }, "on:true"); !got.On {
		t.Fatalf("the manual toggle must win: %s", got.Raw)
	}

	// The fresh daemon boots with the schedule file and the light running.
	m := &nightlightState{
		topic: topic, wmc: wm.OpenNamed("testwm"),
		stateDir: f.state, tempFile: f.temp,
		schedFile:      n.schedFile,
		schedMode:      nlSchedOff,
		schedMarginMin: nlDefaultMarginMin,
		schedStartMin:  nlDefaultClockStart,
		schedStopMin:   nlDefaultClockStop,
	}
	m.loadSchedule()
	m.tickSchedule()
	if got := waitForFrame(t, sub, func(fr nlFrame) bool { return !fr.On }, "on:false"); got.On {
		t.Fatalf("a booted schedule must reconcile the light off during its day: %s", got.Raw)
	}
}
