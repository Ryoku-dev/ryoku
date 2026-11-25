package settings

import (
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
)

// Each control has one home. These links navigate there without duplicating
// a switch or starting the feature being visited.
func (p *Page) oniPageLinks(e *env.Env) []look.Setting {
	ids := [...]int{sectionOniVoice, sectionOniScreen, sectionOniCommands, sectionOniWriting, sectionOniPrivacy, sectionParts}
	rows := make([]look.Setting, 0, len(ids))
	all := sections()
	for index, id := range ids {
		item, button := all[id], &p.oni.links[index]
		button.Kind, button.Small, button.Label = kit.PlaqueQuiet, true, item.label
		rows = append(rows, look.Setting{Title: item.label, Control: func(gtx layout.Context) layout.Dimensions {
			if button.Clicked(gtx) {
				p.oni.destination = id
				p.oni.navigate = true
				gtx.Execute(op.InvalidateCmd{})
			}
			return look.Named(gtx, i18n.Tf("Open %s settings", item.label), func(gtx layout.Context) layout.Dimensions { return button.Layout(gtx, e.Theme.Theme) })
		}})
	}
	return rows
}

func (p *Page) oniSubpageBlocks(gtx layout.Context, e *env.Env, section int, blocks []layout.Widget) []layout.Widget {
	th := e.Theme
	oni, defaults := e.Snapshot().Settings.Oni, p.defaults.Oni
	blocks = append(blocks, func(gtx layout.Context) layout.Dimensions { return sectionHead(gtx, th, section) })
	var groups []layout.Widget
	switch section {
	case sectionOniVoice:
		groups = []layout.Widget{
			group(th, i18n.T("Voice"), i18n.T("Everything you say is heard on this computer. The microphone opens per press of the chord unless you turn on a continuous microphone below."), p.oniVoiceRows(e, oni, defaults), nil),
			group(th, i18n.T("Dictation"), i18n.T("Dictation types into your own windows, not through Oni. The dictionary, snippets and styles stay in Hyakki's state on this computer."), p.oniDictationRows(e, oni, defaults), nil),
		}
	case sectionOniScreen:
		guides, kept := p.oniGuideBlocks(e, oni, defaults)
		groups = []layout.Widget{
			group(th, i18n.T("Looking at a window"), i18n.T("Drag Oni onto a window, or run hyakki oni perch with the window's id, then ask about it here. Only that window is captured, and nothing is sent before you allow it."), p.oniLookingRows(e, oni, defaults), nil),
			group(th, i18n.T("Guides"), i18n.T("Oni leads you to a goal in the perched window, a step per turn. It only points: you click, Oni never does."), guides, nil),
			group(th, i18n.T("Kept walkthroughs"), i18n.T("Each holds the goal, the app and every step's sentence and control, in Hyakki's state on this computer. None holds a picture, a window title or anything you typed."), kept, nil),
		}
	case sectionOniCommands:
		commands, reminders, routines, log := p.oniReflexBlocks(e, oni, defaults)
		groups = []layout.Widget{
			group(th, i18n.T("Quick commands"), i18n.T("Short spoken commands Oni carries out at once: open an app, a site or a new tab, place a window, the volume and music, a screenshot, reminders and notes. None sends, buys or deletes anything, and Oni never types into another window."), commands, nil),
			group(th, i18n.T("Reminders and timers"), i18n.T("Kept in Hyakki's state on this computer with only their own words, until they are said or cancelled."), reminders, nil),
			group(th, i18n.T("Routines"), i18n.T("Quick-command routines, separate from Teams routines. Each holds its commands and nothing said around them."), routines, nil),
			group(th, i18n.T("Quick command log"), i18n.T("What each quick command came to, kept on this computer. A row names the command, never the words said or a transcript."), log, nil),
		}
	case sectionOniWriting:
		groups = p.oniWritingGroups(e, oni, defaults)
	case sectionOniPrivacy:
		groups = p.privacyGroups(e)
		groups = append(groups, group(th, i18n.T("Tools for other programs"), i18n.T("hyakki mcp offers the tools turned on here to programs you approve, over their own pipe. A program never changes these settings."), p.oniMCPRows(gtx, e, oni, defaults), nil))
	case sectionParts:
		groups = p.parts.groups(e)
	}
	if section == sectionOniWriting {
		for index, draw := range groups {
			groups[index] = func(gtx layout.Context) layout.Dimensions {
				p.oni.writing.approvals.drawn = true
				return draw(gtx)
			}
		}
	}
	return spacedBlocks(th, blocks, groups...)
}
