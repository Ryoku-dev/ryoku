package settings

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
)

type keyRow struct {
	does string
	// each chord is its keys, pressed together; several chords do related things
	chords [][]string
}

func everywhereKeys() []keyRow {
	return []keyRow{
		{i18n.T("Search sessions, projects and commands"), [][]string{{"Ctrl", "K"}}},
		{i18n.T("Start something new where you are"), [][]string{{"Ctrl", "N"}}},
		{i18n.T("Ask, Agents, Projects, Index"), [][]string{{"Ctrl", "1"}, {"Ctrl", "2"}, {"Ctrl", "3"}, {"Ctrl", "4"}}},
		{i18n.T("Settings"), [][]string{{"Ctrl", ","}}},
		{i18n.T("Fold the sidebar"), [][]string{{"Ctrl", "B"}}},
		{i18n.T("Show the work beside a session"), [][]string{{"Ctrl", "\\"}}},
		{i18n.T("Back and forward"), [][]string{{"Alt", "←"}, {"Alt", "→"}}},
		{i18n.T("Zoom in, out and back"), [][]string{{"Ctrl", "+"}, {"Ctrl", "-"}, {"Ctrl", "0"}}},
		{i18n.T("Close what is open on top"), [][]string{{"Esc"}}},
	}
}

func writingKeys() []keyRow {
	return []keyRow{
		{i18n.T("Send"), [][]string{{"Enter"}}},
		{i18n.T("Start a new line"), [][]string{{"Shift", "Enter"}}},
		{i18n.T("Steer the turn that is running"), [][]string{{"Ctrl", "Enter"}}},
		{i18n.T("Bring back what you sent, from an empty field"), [][]string{{"↑"}, {"↓"}}},
		{i18n.T("List the harness's commands"), [][]string{{"/"}}},
		{i18n.T("Paste text, or an image as an attachment"), [][]string{{"Ctrl", "V"}}},
	}
}

func (p *Page) layoutKeys(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	slab := func(rows []keyRow) layout.Widget {
		settings := make([]look.Setting, len(rows))
		for index, row := range rows {
			settings[index] = look.Setting{Title: row.does, Control: keyChords(th, row.chords)}
		}
		return func(gtx layout.Context) layout.Dimensions { return look.SettingsSlab(gtx, th, settings) }
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(kit.Text(th.Theme, th.T.Section, th.R.Fg, i18n.T("Keyboard"))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(slab(everywhereKeys())),
		layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
		layout.Rigid(kit.Text(th.Theme, th.T.Title, th.R.Fg, i18n.T("While writing"))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(slab(writingKeys())),
	)
}

func keyChords(th *look.Theme, chords [][]string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, 4*len(chords))
		for index, chord := range chords {
			if index > 0 {
				children = append(children, layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout))
			}
			for at, name := range chord {
				if at > 0 {
					children = append(children, layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout))
				}
				children = append(children, layout.Rigid(look.TokenChip(th, name)))
			}
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	}
}
