package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ryoku-hub activity [days]: how long the machine was on each day, for the
// activity map under Settings' sidebar. The journal keeps every boot's first
// and last entry and systemd-sleep's own lines, so "on" is the union of the
// boots minus the time spent suspended or hibernated, split at local midnight.
// No journal (runit editions without one) prints no days and source "none".

type activityDay struct {
	Date  string  `json:"date"`
	Hours float64 `json:"hours"`
}

type activityReport struct {
	Source string        `json:"source"`
	Days   []activityDay `json:"days"`
	// how far back the journal reaches, so the map can tell "off" from "unknown"
	Since string `json:"since"`
	Boots int    `json:"boots"`
}

type span struct{ from, to time.Time }

type sleepEvent struct {
	at   time.Time
	boot string
	kind string // "enter", "resume", "failed"
}

func runActivity(args []string) error {
	days := 182
	if len(args) > 0 {
		n, err := strconv.Atoi(args[0])
		if err != nil || n < 1 || n > 731 {
			return fmt.Errorf("activity takes a day count from 1 to 731")
		}
		days = n
	}
	return printJSON(readActivity(days, time.Now()))
}

func readActivity(days int, now time.Time) activityReport {
	out, err := exec.Command("journalctl", "--list-boots", "-o", "json", "--no-pager").Output()
	if err != nil {
		return activityReport{Source: "none", Days: []activityDay{}}
	}
	boots, err := parseBoots(out, now)
	if err != nil || len(boots) == 0 {
		return activityReport{Source: "none", Days: []activityDay{}}
	}
	var sleeps []span
	if raw, err := exec.Command("journalctl", "-o", "json", "--no-pager", "-t", "systemd-sleep",
		"--output-fields=MESSAGE,_BOOT_ID").Output(); err == nil {
		sleeps = pairSleeps(parseSleepEvents(raw))
	}
	end := startOfDay(now).AddDate(0, 0, 1)
	start := end.AddDate(0, 0, -days)
	return activityReport{
		Source: "journal",
		Days:   onTimeByDay(boots, sleeps, start, end),
		Since:  startOfDay(boots[0].from).Format("2006-01-02"),
		Boots:  len(boots),
	}
}

// parseBoots reads `journalctl --list-boots -o json`; the boot still running
// lasts until now
func parseBoots(raw []byte, now time.Time) ([]span, error) {
	var rows []struct {
		Index int   `json:"index"`
		First int64 `json:"first_entry"`
		Last  int64 `json:"last_entry"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	out := make([]span, 0, len(rows))
	for _, r := range rows {
		to := time.UnixMicro(r.Last)
		if r.Index == 0 {
			to = now
		}
		from := time.UnixMicro(r.First)
		if to.After(from) {
			out = append(out, span{from, to})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].from.Before(out[j].from) })
	return out, nil
}

func parseSleepEvents(raw []byte) []sleepEvent {
	var out []sleepEvent
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		var e struct {
			Msg  any    `json:"MESSAGE"`
			Boot string `json:"_BOOT_ID"`
			At   string `json:"__REALTIME_TIMESTAMP"`
		}
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		msg, _ := e.Msg.(string)
		us, err := strconv.ParseInt(e.At, 10, 64)
		if err != nil {
			continue
		}
		var kind string
		switch {
		case strings.HasPrefix(msg, "Performing sleep operation"):
			kind = "enter"
		case strings.HasPrefix(msg, "System returned from sleep"):
			kind = "resume"
		case strings.HasPrefix(msg, "Failed to put system to sleep"):
			kind = "failed"
		default:
			continue
		}
		out = append(out, sleepEvent{time.UnixMicro(us), e.Boot, kind})
	}
	return out
}

// pairSleeps turns enter/resume lines into the spans the machine slept: an
// enter closed by a resume in the same boot. A sleep that failed never
// happened, and one that never resumed ended with its boot, which the boot's
// own last entry already marks
func pairSleeps(events []sleepEvent) []span {
	sort.SliceStable(events, func(i, j int) bool { return events[i].at.Before(events[j].at) })
	var out []span
	var open *sleepEvent
	for i := range events {
		e := events[i]
		switch e.kind {
		case "enter":
			open = &events[i]
		case "failed":
			open = nil
		case "resume":
			if open != nil && open.boot == e.boot && e.at.After(open.at) {
				out = append(out, span{open.at, e.at})
			}
			open = nil
		}
	}
	return out
}

// onTimeByDay sums, for each local day in [start, end), the time inside a
// boot and outside every sleep
func onTimeByDay(boots, sleeps []span, start, end time.Time) []activityDay {
	on := subtract(union(boots), union(sleeps))
	var days []activityDay
	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		next := d.AddDate(0, 0, 1)
		var total time.Duration
		for _, s := range on {
			from, to := s.from, s.to
			if from.Before(d) {
				from = d
			}
			if to.After(next) {
				to = next
			}
			if to.After(from) {
				total += to.Sub(from)
			}
		}
		days = append(days, activityDay{Date: d.Format("2006-01-02"), Hours: float64(int(total.Hours()*10+0.5)) / 10})
	}
	return days
}

func union(spans []span) []span {
	s := append([]span(nil), spans...)
	sort.Slice(s, func(i, j int) bool { return s[i].from.Before(s[j].from) })
	var out []span
	for _, x := range s {
		if n := len(out); n > 0 && !x.from.After(out[n-1].to) {
			if x.to.After(out[n-1].to) {
				out[n-1].to = x.to
			}
			continue
		}
		out = append(out, x)
	}
	return out
}

// subtract removes every span of cut from a (both merged and sorted)
func subtract(a, cut []span) []span {
	var out []span
	for _, x := range a {
		from := x.from
		for _, c := range cut {
			if !c.to.After(from) || !c.from.Before(x.to) {
				continue
			}
			if c.from.After(from) {
				out = append(out, span{from, c.from})
			}
			from = c.to
		}
		if x.to.After(from) {
			out = append(out, span{from, x.to})
		}
	}
	return out
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
