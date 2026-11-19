package teamsui

import (
	"slices"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// paths a sandbox adds or hides, one row each with its own remove, and a field that adds one; the caller saves what Changed reports
type PathList struct {
	// names the folder chooser's window
	Title       string
	Placeholder string

	field   env.PathField
	add     kit.Plaque
	removes map[string]*kit.IconPlaque
	next    []string
	changed bool
}

// the whole list after the add or remove made since it was last asked
func (l *PathList) Changed() ([]string, bool) {
	changed := l.changed
	l.changed = false
	return l.next, changed
}

func (l *PathList) Layout(gtx layout.Context, e *env.Env, list []string) layout.Dimensions {
	th := e.Theme
	if l.removes == nil {
		l.removes = make(map[string]*kit.IconPlaque)
	}
	// a path taken off the list lets its control go
	for path := range l.removes {
		if !slices.Contains(list, path) {
			delete(l.removes, path)
		}
	}
	children := make([]layout.FlexChild, 0, len(list)+1)
	for _, path := range list {
		remove := l.removes[path]
		if remove == nil {
			remove = new(kit.IconPlaque)
			l.removes[path] = remove
		}
		// the cross at the end of a path needs no word beside it; a screen reader hears which path it takes off
		remove.Kind, remove.Small, remove.Icon = kit.PlaqueQuiet, true, look.Icon.Close
		remove.Description = i18n.Tf("Take %s off the list", path)
		if remove.Clicked(gtx) {
			l.next = slices.DeleteFunc(slices.Clone(list), func(each string) bool { return each == path })
			l.changed = true
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, look.Paragraph(th, th.T.MonoSm, th.R.Fg, path)),
					layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return remove.Layout(gtx, th.Theme) }),
				)
			})
		}))
	}
	path := l.field.Path()
	addable := path != "" && !slices.Contains(list, path)
	l.add.Kind, l.add.Small, l.add.Icon, l.add.Label = kit.PlaqueTonal, true, look.Icon.Add, i18n.T("Add")
	if l.add.Clicked(gtx) && addable {
		l.next = append(slices.Clone(list), path)
		l.changed = true
		l.field.SetText("")
	}
	l.field.Title, l.field.Placeholder = l.Title, l.Placeholder
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return l.field.Layout(gtx, e) }),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !addable {
						gtx = gtx.Disabled()
					}
					return l.add.Layout(gtx, th.Theme)
				}),
			)
		})
	}))
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// sandbox groups share names across team and global settings
func HiddenKindName(kind string) string {
	switch kind {
	case protocol.HiddenGit:
		return i18n.T("Git credentials")
	case protocol.HiddenCloud:
		return i18n.T("Cloud and cluster logins")
	case protocol.HiddenPackages:
		return i18n.T("Package registry logins")
	case protocol.HiddenBrowsers:
		return i18n.T("Browser profiles")
	case protocol.HiddenMail:
		return i18n.T("Mail")
	case protocol.HiddenChat:
		return i18n.T("Chat apps")
	case protocol.HiddenHistories:
		return i18n.T("Shell histories")
	case protocol.HiddenRyoku:
		return i18n.T("Rashin and Ryoku private state")
	case protocol.HiddenAgents:
		return i18n.T("Other AI tools' logins and state")
	case protocol.HiddenHost:
		return i18n.T("Host daemons")
	}
	return kind
}
