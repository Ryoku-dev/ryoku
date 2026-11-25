package activity

import (
	"fmt"
	"time"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// how many entries one call reads
const pageSize = 100

// the entries shown for one filter and team, newest first
type ledger struct {
	entries []protocol.Activity
	// the filter and team the entries are for, and the teams' state they were read at
	key, pulse string
	loading    bool
	// a full page came back last time, so older entries may wait
	more bool
}

// reads the newest page when the filter changes or the teams moved on, never on a clock
func (l *ledger) ensure(e *env.Env, filter, team, pulse string) {
	if l.loading {
		return
	}
	if key := filter + "\x00" + team; key != l.key || pulse != l.pulse {
		l.load(e, filter, team, pulse, "")
	}
}

// the page before the oldest entry shown
func (l *ledger) older(e *env.Env, filter, team string) {
	if l.loading || len(l.entries) == 0 {
		return
	}
	l.load(e, filter, team, l.pulse, l.entries[len(l.entries)-1].Time.Format(time.RFC3339Nano))
}

func (l *ledger) load(e *env.Env, filter, team, pulse, before string) {
	key := filter + "\x00" + team
	l.loading = true
	params := protocol.TeamActivityParams{Team: team, Filter: filter, Limit: pageSize, Before: before}
	var result protocol.ActivityResult
	e.Call(protocol.MTeamsActivity, params, &result, func(err error) {
		l.loading = false
		if err != nil {
			l.key, l.pulse = key, pulse
			e.Notify(err.Error())
			return
		}
		page, full := result.Activity, len(result.Activity) == pageSize
		switch {
		case before != "":
			l.entries, l.more = append(l.entries, page...), full
		case key == l.key && !full:
			l.entries = refreshed(page, l.entries)
		default:
			l.entries, l.more = page, full
		}
		l.key, l.pulse = key, pulse
	})
}

// the newest page, then what was loaded before it that it does not cover
func refreshed(page, kept []protocol.Activity) []protocol.Activity {
	if len(page) == 0 {
		return page
	}
	oldest := page[len(page)-1].Time
	entries := append([]protocol.Activity(nil), page...)
	for _, entry := range kept {
		if entry.Time.Before(oldest) {
			entries = append(entries, entry)
		}
	}
	return entries
}

// changes whenever anything a ledger entry records could have happened
func pulse(snapshot state.Snapshot) string {
	var latest time.Time
	note := func(at time.Time) {
		if at.After(latest) {
			latest = at
		}
	}
	for _, team := range snapshot.Teams {
		note(team.Updated)
	}
	for _, task := range snapshot.TeamTasks {
		note(task.Updated)
	}
	for _, approval := range snapshot.Approvals {
		note(approval.Created)
		if approval.Decided != nil {
			note(*approval.Decided)
		}
	}
	for _, routine := range snapshot.Routines {
		note(routine.Updated)
	}
	uses := 0
	for _, credential := range snapshot.Vault.Credentials {
		uses += credential.Uses + credential.Version
	}
	return fmt.Sprint(len(snapshot.Teams), len(snapshot.TeamTasks), len(snapshot.Approvals), len(snapshot.Routines),
		len(snapshot.Vault.Credentials), uses, latest.UnixNano())
}
