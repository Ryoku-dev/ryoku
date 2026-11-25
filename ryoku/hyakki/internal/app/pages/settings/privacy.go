package settings

import (
	"fmt"
	"strings"
	"time"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	i18n "ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// privacyBlock holds only this view's read result and confirmation state.
// The daemon reads and clears the feature owners; this is not another store.
type privacyBlock struct {
	loaded, busy bool
	result       protocol.OniActivityResult
	err          string
	refresh, all kit.Plaque
	buttons      map[string]*kit.Plaque
	confirm      kit.Confirm
	selected     string
}

func (b *privacyBlock) load(e *env.Env) {
	if b.busy {
		return
	}
	b.loaded, b.busy = true, true
	result := new(protocol.OniActivityResult)
	e.Call(protocol.MOniActivity, protocol.Empty{}, result, func(err error) {
		b.busy = false
		if err != nil {
			b.err = err.Error()
			b.result = protocol.OniActivityResult{}
			return
		}
		b.result, b.err = *result, ""
	})
}

func (p *Page) privacyGroups(e *env.Env) []layout.Widget {
	b, th := &p.privacy, e.Theme
	if !b.loaded {
		b.load(e)
	}
	if b.buttons == nil {
		b.buttons = make(map[string]*kit.Plaque)
	}
	b.refresh.Kind, b.refresh.Small, b.refresh.Label = kit.PlaqueQuiet, true, i18n.T("Refresh activity")
	detail := i18n.T("Records stay on this computer. The last 50 ledger rows contain action codes, timings and usage only, not the words, pictures or titles you shared.")
	if len(b.result.Errors) != 0 {
		detail += "\n" + i18n.T("Some stores could not be read. Their counts are unknown; healthy stores can still be cleared individually.")
	}
	if b.err != "" {
		detail = b.err
	}
	rows := []look.Setting{{Title: i18n.T("Local activity"), Detail: detail, Control: func(gtx layout.Context) layout.Dimensions {
		if b.refresh.Clicked(gtx) && !b.busy {
			b.load(e)
		}
		if b.busy {
			gtx = gtx.Disabled()
		}
		return look.Named(gtx, i18n.T("Refresh local activity"), func(gtx layout.Context) layout.Dimensions { return b.refresh.Layout(gtx, th.Theme) })
	}}}
	for _, store := range protocol.OniActivityStores {
		button := b.buttons[store]
		if button == nil {
			button = new(kit.Plaque)
			b.buttons[store] = button
		}
		button.Kind, button.Small, button.Label = kit.PlaqueQuiet, true, i18n.T("Clear")
		count := i18n.T("Not read")
		if n, known := b.result.Counts[store]; known {
			count = i18n.Tf("%d records", n)
			if store == protocol.OniActivityHandsLog {
				count = i18n.Tf("%d ledger rows; Clear also forgets the task journal", n)
			}
		} else if problem := b.result.Errors[store]; problem != "" {
			count += " · " + problem
		}
		rows = append(rows, look.Setting{Title: privacyStoreName(store), Detail: count, Control: func(gtx layout.Context) layout.Dimensions {
			if button.Clicked(gtx) && b.canClear(store) {
				b.selected, b.confirm.Dialog.Open, b.confirm.Danger = store, true, true
			}
			if !b.canClear(store) {
				gtx = gtx.Disabled()
			}
			return look.Named(gtx, i18n.Tf("Clear %s", privacyStoreName(store)), func(gtx layout.Context) layout.Dimensions { return button.Layout(gtx, th.Theme) })
		}})
	}
	b.all.Kind, b.all.Small, b.all.Label = kit.PlaqueQuiet, true, i18n.T("Clear all Oni records")
	rows = append(rows, look.Setting{Title: i18n.T("Clear Oni's records"), Detail: i18n.T("Turn Oni off first and wait for running work to finish. Clearing reminders cancels them; clearing Hands also forgets its tasks and kept task projects. Memory facts, conversations, original export files, browser sign-ins and the Ryoku vault are not touched. Daily assignment budget totals and safety limits remain."), Control: func(gtx layout.Context) layout.Dimensions {
		if b.all.Clicked(gtx) && b.canClear(protocol.OniActivityAll) {
			b.selected, b.confirm.Dialog.Open, b.confirm.Danger = protocol.OniActivityAll, true, true
		}
		if !b.canClear(protocol.OniActivityAll) {
			gtx = gtx.Disabled()
		}
		return look.Named(gtx, i18n.T("Clear all Oni records"), func(gtx layout.Context) layout.Dimensions { return b.all.Layout(gtx, th.Theme) })
	}})
	values, ticks := privacyGraph(b.result.Rows)
	proof := i18n.T("Rows returned in this view, by day. At most 50 rows; this is not an all-time usage graph or an estimate of time saved.")
	if len(b.result.Errors) != 0 {
		proof += "\n" + i18n.T("Partial activity view: unreadable stores are marked above and have no assumed count.")
	}
	empty := i18n.T("No activity has been recorded.")
	if b.result.Counts == nil {
		empty = i18n.T("Refresh to read activity.")
	}
	out := []layout.Widget{
		group(th, i18n.T("Activity and privacy"), i18n.T("A view of the actual local records, not a second history. Refresh to read changes made elsewhere."), rows, nil),
		func(gtx layout.Context) layout.Dimensions {
			return (look.Trend{Title: i18n.T("Recent ledger activity"), Proof: proof, Values: values, Ticks: ticks, Empty: empty}).Layout(gtx, th)
		},
	}
	ledger := []look.Setting{}
	for _, row := range b.result.Rows {
		ledger = append(ledger, look.Setting{Title: privacyStoreName(row.Store) + " · " + row.Action, Detail: privacyRowDetail(row)})
	}
	if len(ledger) == 0 {
		detail := i18n.T("Counts above can include kept items even when the in-memory action logs are empty.")
		if b.result.Counts == nil {
			detail = i18n.T("Refresh to read the local records. No counts are available yet.")
		}
		ledger = append(ledger, look.Setting{Title: i18n.T("No ledger rows"), Detail: detail})
	}
	out = append(out, group(th, i18n.T("Last 50 ledger rows"), i18n.T("Token counts are reported only where measured. Estimated and unmetered usage are marked; missing pricing is never presented as free. No saved-minutes estimate is calculated."), ledger, nil))
	return out
}

func (b *privacyBlock) overlay(gtx layout.Context, e *env.Env) bool {
	if !b.confirm.Dialog.Open {
		return false
	}
	name := privacyStoreName(b.selected)
	body := i18n.Tf("Clear %s from this computer? This cannot be undone. Turn Oni off first. Running work must finish before anything is cleared.", name)
	if b.selected == protocol.OniActivityAll || b.selected == protocol.OniActivityReminders {
		body += "\n" + i18n.T("Reminders and timers are cancelled.")
	}
	if b.selected == protocol.OniActivityAll || b.selected == protocol.OniActivityHandsLog {
		body += "\n" + i18n.T("Hands' task history and kept task projects are forgotten.")
	}
	body += "\n" + i18n.T("Only the selected records are cleared. Memory facts, conversations, original exports, the browser profile and the vault stay. Assignment budget totals and safety limits stay; clearing history does not reset a cap.")
	if b.confirm.Layout(gtx, e.Theme.Theme, i18n.Tf("Clear %s?", name), body, i18n.T("Clear records"), "") && !b.busy {
		b.busy = true
		result := new(protocol.OniActivityResult)
		e.Call(protocol.MOniActivityClear, protocol.OniActivityClearParams{Stores: []string{b.selected}}, result, func(err error) {
			b.busy = false
			if err != nil {
				b.err = err.Error()
				// An interrupted multi-store clear can have completed earlier
				// owners. Never leave their previous counts presented as current.
				b.result = protocol.OniActivityResult{}
				e.Report(err)
				return
			}
			b.result, b.err = *result, ""
		})
	}
	return b.confirm.Dialog.Open
}

// Healthy selected stores remain available in a partial aggregate. All
// requires every owner to be known; transport failures are not partial reads.
func (b *privacyBlock) canClear(store string) bool {
	if b.busy || !b.loaded || b.err != "" {
		return false
	}
	if store == protocol.OniActivityAll {
		return len(b.result.Errors) == 0 && len(b.result.Counts) == len(protocol.OniActivityStores)
	}
	_, known := b.result.Counts[store]
	return known && b.result.Errors[store] == ""
}

func privacyStoreName(store string) string {
	switch store {
	case protocol.OniActivityReflexLog:
		return i18n.T("Quick command log")
	case protocol.OniActivityHandsLog:
		return i18n.T("Hands log and tasks")
	case protocol.OniActivityAssignment:
		return i18n.T("Assignment log")
	case protocol.OniActivityWalkthroughs:
		return i18n.T("Kept walkthroughs")
	case protocol.OniActivityRoutines:
		return i18n.T("Kept routines")
	case protocol.OniActivityReminders:
		return i18n.T("Reminders and timers")
	case protocol.OniActivityRecallImports:
		return i18n.T("Imported recall chats")
	default:
		return i18n.T("All Oni records")
	}
}

func privacyRowDetail(row protocol.OniActivityRow) string {
	bits := []string{row.At.Local().Format("2006-01-02 15:04:05"), row.Outcome}
	if row.Route != "" {
		bits = append(bits, row.Route)
	}
	if row.Ms > 0 {
		bits = append(bits, fmt.Sprintf("%d ms", row.Ms))
	}
	switch {
	case row.Unmetered:
		bits = append(bits, i18n.T("Usage unmetered; cost unavailable"))
	case row.Estimated:
		bits = append(bits, i18n.Tf("Estimated tokens: %d in / %d out; cost unavailable", row.Input, row.Output))
	default:
		bits = append(bits, i18n.Tf("Measured tokens: %d in / %d out; price unavailable", row.Input, row.Output))
	}
	return strings.Join(bits, " · ")
}

// The graph ends at the newest returned row, not the machine clock, so
// fixed demo records remain meaningful and the labels never invent activity.
func privacyGraph(rows []protocol.OniActivityRow) ([]float64, []string) {
	if len(rows) == 0 {
		return nil, nil
	}
	end := rows[0].At.Local()
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())
	values, ticks := make([]float64, 7), make([]string, 7)
	for i := range values {
		day := end.AddDate(0, 0, i-6)
		ticks[i] = day.Format("Jan 2")
		for _, row := range rows {
			if row.At.In(day.Location()).Format(time.DateOnly) == day.Format(time.DateOnly) {
				values[i]++
			}
		}
	}
	return values, ticks
}
