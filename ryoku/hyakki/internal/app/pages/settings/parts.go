package settings

import (
	"fmt"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// partsBlock is the optional parts: what Hyakki downloaded or made, each
// with its size, what uses it and Remove behind a question, and the programs
// a distribution provides, each with why Oni would run it. Nothing is read
// until the groups are first drawn.
type partsBlock struct {
	loaded, busy bool
	result       protocol.ComponentsResult
	err          string
	refresh      kit.Plaque
	buttons      map[string]*kit.Plaque
	confirm      kit.Confirm
	selected     protocol.Component
}

func (b *partsBlock) load(e *env.Env) {
	if b.busy {
		return
	}
	b.loaded, b.busy = true, true
	result := new(protocol.ComponentsResult)
	e.Call(protocol.MComponents, protocol.Empty{}, result, func(err error) {
		b.busy = false
		if err != nil {
			b.err = err.Error()
			return
		}
		b.result, b.err = *result, ""
	})
}

// groups are the parts Hyakki holds and the programs it may run, as two
// Settings groups.
func (b *partsBlock) groups(e *env.Env) []layout.Widget {
	th := e.Theme
	if !b.loaded {
		b.load(e)
	}
	if b.buttons == nil {
		b.buttons = make(map[string]*kit.Plaque)
	}
	owned, programs := partSplit(b.result.Components)
	b.refresh.Kind, b.refresh.Small, b.refresh.Label = kit.PlaqueQuiet, true, i18n.T("Measure again")
	rows := []look.Setting{{Title: i18n.T("Parts on this computer"), Detail: partsSummary(owned, b.err),
		Control: func(gtx layout.Context) layout.Dimensions {
			if b.refresh.Clicked(gtx) {
				b.load(e)
			}
			if b.busy {
				gtx = gtx.Disabled()
			}
			return look.Named(gtx, i18n.T("Measure the optional parts again"), func(gtx layout.Context) layout.Dimensions { return b.refresh.Layout(gtx, th.Theme) })
		}}}
	for _, c := range owned {
		rows = append(rows, b.row(e, c))
	}
	programRows := make([]look.Setting, 0, len(programs))
	for _, c := range programs {
		programRows = append(programRows, look.Setting{Title: c.Title, Detail: partDetail(c)})
	}
	return []layout.Widget{
		group(th, i18n.T("Optional parts"), i18n.T("Models, voices and records Hyakki downloaded or made. Each can go; Oni turns the feature that needs it back to what works without it."), rows, nil),
		group(th, i18n.T("Optional programs"), i18n.T("Your distribution installs these. Oni runs one only for the setting that needs it; Hyakki never installs or removes one."), programRows, nil),
	}
}

// row is one part Hyakki holds, with Remove while it is on this computer.
func (b *partsBlock) row(e *env.Env, c protocol.Component) look.Setting {
	th := e.Theme
	s := look.Setting{Title: c.Title, Detail: partDetail(c)}
	if !partRemovable(c) {
		return s
	}
	button := b.buttons[c.ID]
	if button == nil {
		button = new(kit.Plaque)
		b.buttons[c.ID] = button
	}
	button.Kind, button.Small, button.Label = kit.PlaqueQuiet, true, i18n.T("Remove")
	s.Control = func(gtx layout.Context) layout.Dimensions {
		if button.Clicked(gtx) && c.InUse == "" && !b.busy {
			b.selected, b.confirm.Dialog.Open, b.confirm.Danger = c, true, true
		}
		if c.InUse != "" || b.busy {
			gtx = gtx.Disabled()
		}
		return look.Named(gtx, i18n.Tf("Remove %s", c.Title), func(gtx layout.Context) layout.Dimensions { return button.Layout(gtx, th.Theme) })
	}
	return s
}

// overlay is the question before a part is removed.
func (b *partsBlock) overlay(gtx layout.Context, e *env.Env) bool {
	if !b.confirm.Dialog.Open {
		return false
	}
	c := b.selected
	if b.confirm.Layout(gtx, e.Theme.Theme, i18n.Tf("Remove %s?", c.Title), partConfirmBody(c), i18n.T("Remove it"), "") && !b.busy {
		b.busy = true
		result := new(protocol.ComponentsResult)
		e.Call(protocol.MComponentRemove, protocol.ComponentRemoveParams{ID: c.ID}, result, func(err error) {
			b.busy = false
			if err != nil {
				b.err = err.Error()
				e.Report(err)
				return
			}
			b.result, b.err = *result, ""
		})
	}
	return b.confirm.Dialog.Open
}

// partSplit puts the programs apart from the parts Hyakki holds, each in
// the order listed.
func partSplit(components []protocol.Component) (owned, programs []protocol.Component) {
	for _, c := range components {
		if c.Kind == protocol.ComponentProgram {
			programs = append(programs, c)
		} else {
			owned = append(owned, c)
		}
	}
	return owned, programs
}

// partRemovable is whether a part gets Remove: Hyakki made it and some of it
// is on this computer.
func partRemovable(c protocol.Component) bool {
	return c.Kind != protocol.ComponentProgram && c.Removable && c.State != protocol.ComponentAbsent
}

// partsSummary is how much the parts on this computer take, or why they
// could not be read.
func partsSummary(owned []protocol.Component, problem string) string {
	if problem != "" {
		return problem
	}
	var total int64
	count := 0
	for _, c := range owned {
		if c.State != protocol.ComponentAbsent {
			total += max(0, c.Bytes)
			count++
		}
	}
	if count == 0 {
		return i18n.T("None of them is on this computer.")
	}
	return i18n.Tf("%d on this computer, %s in all.", count, partSize(total))
}

// partDetail says what uses a part, where it stands and, when it cannot go
// now or ever, why.
func partDetail(c protocol.Component) string {
	lines := []string{}
	if c.Uses != "" {
		lines = append(lines, c.Uses)
	}
	lines = append(lines, partStateWords(c))
	switch {
	case c.InUse != "":
		lines = append(lines, c.InUse)
	case c.Why != "":
		lines = append(lines, c.Why)
	}
	return strings.Join(lines, "\n")
}

// partStateWords is a part's state and size, or a program's place, in words.
func partStateWords(c protocol.Component) string {
	where := ""
	if c.Path != "" && c.Kind != protocol.ComponentProgram {
		where = " · " + paths.Short(c.Path)
	}
	if c.Kind == protocol.ComponentProgram {
		switch c.State {
		case protocol.ComponentPresent:
			return i18n.Tf("Installed: %s", c.Path)
		case protocol.ComponentUnknown:
			return i18n.T("Not looked for")
		default:
			if len(c.Needs) > 0 {
				return i18n.Tf("Not installed: %s", strings.Join(c.Needs, ", "))
			}
			return i18n.T("Not installed")
		}
	}
	switch c.State {
	case protocol.ComponentPresent:
		return i18n.Tf("On this computer, %s", partSize(c.Bytes)) + where
	case protocol.ComponentPartial:
		return i18n.Tf("Partly downloaded, %s", partSize(c.Bytes)) + where
	case protocol.ComponentUnusable:
		return i18n.Tf("Damaged and not used, %s", partSize(c.Bytes)) + where
	case protocol.ComponentUnknown:
		return i18n.T("Not looked for")
	default:
		return i18n.T("Not on this computer")
	}
}

// partConfirmBody is the question's body: what goes, how much, and what
// happens without it.
func partConfirmBody(c protocol.Component) string {
	body := i18n.Tf("This deletes %s from this computer, %s. Nothing outside Hyakki's own folders is touched.", c.Title, partSize(c.Bytes))
	if c.Uses != "" {
		body += "\n" + c.Uses
	}
	return body
}

// partSize is a size in KB, MB or GB.
func partSize(bytes int64) string {
	const kib, mib, gib = 1024, 1024 * 1024, 1024 * 1024 * 1024
	switch b := max(0, bytes); {
	case b >= gib:
		return fmt.Sprintf("%.1f GB", float64(b)/gib)
	case b >= mib:
		return fmt.Sprintf("%.1f MB", float64(b)/mib)
	default:
		return fmt.Sprintf("%d KB", (b+kib-1)/kib)
	}
}
