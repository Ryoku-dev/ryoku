package settings

import (
	"slices"
	"strings"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type profileBlock struct {
	filled            bool
	stored            protocol.MemoryProfile
	name, work, about kit.TextField
	traits            [8]kit.Plaque
	selected          []string
	save              kit.Plaque
}

var answerTraits = []string{"brief", "detailed", "plain words", "technical", "warm", "direct", "with examples", "step by step"}

func (p *Page) profileBlocks(gtx layout.Context, e *env.Env, blocks []layout.Widget) []layout.Widget {
	b, th := &p.profile, e.Theme
	stored := e.Snapshot().Settings.Memory.Profile
	if !b.filled || stored.Name != b.stored.Name || stored.Work != b.stored.Work || stored.About != b.stored.About || !slices.Equal(stored.Traits, b.stored.Traits) {
		b.stored, b.filled = stored, true
		b.stored.Traits = slices.Clone(stored.Traits)
		b.name.SetText(stored.Name)
		b.work.SetText(stored.Work)
		b.about.SetText(stored.About)
		b.selected = slices.Clone(stored.Traits)
	}
	b.name.Editor.SingleLine, b.name.Editor.MaxLen = true, 80
	b.work.Editor.SingleLine, b.work.Editor.MaxLen = true, 240
	b.about.Editor.MaxLen = 600
	field := func(f *kit.TextField, name string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return look.Named(gtx, name, func(gtx layout.Context) layout.Dimensions { return f.Layout(gtx, th.Theme) })
		}
	}
	rows := []look.Setting{
		{Title: i18n.T("Name to call you"), Below: true, Control: field(&b.name, i18n.T("Name to call you"))},
		{Title: i18n.T("What you do"), Below: true, Control: field(&b.work, i18n.T("What you do"))},
		{Title: i18n.T("How you like answers"), Below: true, Control: func(gtx layout.Context) layout.Dimensions {
			labels := []string{i18n.T("Brief"), i18n.T("Detailed"), i18n.T("Plain words"), i18n.T("Technical"), i18n.T("Warm"), i18n.T("Direct"), i18n.T("With examples"), i18n.T("Step by step")}
			chip := func(index int) layout.Widget {
				return func(gtx layout.Context) layout.Dimensions {
					trait, button := answerTraits[index], &b.traits[index]
					selected := slices.Contains(b.selected, trait)
					button.Kind, button.Small, button.Label, button.Icon = kit.PlaqueTonal, true, labels[index], nil
					if selected {
						button.Kind, button.Icon = kit.PlaqueOn, look.Icon.Check
					}
					if button.Clicked(gtx) {
						if at := slices.Index(b.selected, trait); at >= 0 {
							b.selected = slices.Delete(b.selected, at, at+1)
						} else {
							b.selected = append(b.selected, trait)
						}
					}
					semantic.SelectedOp(selected).Add(gtx.Ops)
					return button.Layout(gtx, th.Theme)
				}
			}
			children := make([]layout.FlexChild, 0, len(answerTraits))
			for index := 0; index < len(answerTraits); index += 2 {
				index := index
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{}.Layout(gtx, layout.Flexed(1, chip(index)), layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout), layout.Flexed(1, chip(index+1)))
				}), layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		}},
		{Title: i18n.T("Anything else about your answers"), Detail: i18n.T("Used only while memory is on. You can leave any field empty."), Below: true, Control: field(&b.about, i18n.T("Anything else about your answers"))},
	}
	edited := protocol.MemoryProfile{Name: strings.TrimSpace(b.name.Text()), Work: strings.TrimSpace(b.work.Text()), About: strings.TrimSpace(b.about.Text()), Traits: slices.Clone(b.selected)}
	changed := edited.Name != stored.Name || edited.Work != stored.Work || edited.About != stored.About || !slices.Equal(edited.Traits, stored.Traits)
	save := func(gtx layout.Context) layout.Dimensions {
		if b.save.Clicked(gtx) && changed {
			b.save.Busy = true
			e.SetSettings(map[string]any{"memory": map[string]any{"profile": edited}}, func(err error) {
				b.save.Busy = false
				if err != nil {
					e.Notify(err.Error())
				}
			})
		}
		return groupSave(th, &b.save, changed)(gtx)
	}
	return spacedBlocks(th, blocks, teamsGroup(th, i18n.T("About you"), i18n.T("A few things Hyakki should not have to guess."), rows, save))
}
