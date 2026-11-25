package shell

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/memory/scan"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sensitive"
)

const (
	tourHello = iota
	tourDoors
	tourName
	tourVoice
	tourChord
	tourPlacement
	tourGuide
	tourConsent
	tourTask
	tourCards
)

// The tour owns no authority over Oni. Its only writes are its seen/done
// markers and the name the person explicitly asks memory to keep. Demos are
// confined to widgets in this dialog: no screen capture or desktop input.
type onboarding struct {
	checked bool
	active  bool
	card    int
	dialog  kit.Dialog
	back    kit.Plaque
	skip    kit.Plaque
	next    kit.Plaque
	choices [3]kit.Plaque
	name    kit.TextField
	scratch kit.TextField
	problem string
	notice  string
	busy    bool
	serial  uint64
	guide   int
	consent bool

	// Coalesce writes in order, including closing while the seen write is
	// still in flight, so an older reply cannot overwrite completion.
	wanted protocol.OnboardingSettings
	saving bool
}

func (o *onboarding) firstRun(e *env.Env) {
	if o.checked {
		return
	}
	o.checked = true
	if !e.Snapshot().Settings.Onboarding.Seen {
		o.open(e)
	}
}

func (o *onboarding) open(e *env.Env) {
	o.serial++
	o.checked, o.active, o.card = true, true, tourHello
	o.dialog.Open, o.dialog.Width = true, 620
	o.problem, o.notice, o.busy, o.guide, o.consent = "", "", false, 0, false
	o.name.SetText("")
	o.scratch.SetText("")
	o.wanted = protocol.OnboardingSettings{Seen: true}
	o.persist(e)
}

func (o *onboarding) persist(e *env.Env) {
	if o.saving {
		return
	}
	writing := o.wanted
	o.saving = true
	e.SetSettings(map[string]any{"onboarding": writing}, func(err error) {
		o.saving = false
		if err != nil {
			o.problem = i18n.T("The tour's progress could not be saved. Reopen the tour to try again.")
			e.Report(err)
			return
		}
		if o.wanted != writing {
			o.persist(e)
		}
	})
}

func (o *onboarding) finish(gtx layout.Context, e *env.Env) {
	o.wanted = protocol.OnboardingSettings{Seen: true, Done: true}
	o.persist(e)
	o.leave(gtx)
}

func (o *onboarding) leave(gtx layout.Context) {
	o.serial++
	o.active, o.busy = false, false
	o.name.SetText("")
	o.scratch.SetText("")
	o.dialog.Dismiss(gtx)
}

func (o *onboarding) move(delta int) {
	o.card = max(tourHello, min(tourTask, o.card+delta))
	o.problem, o.notice = "", ""
}

func (o *onboarding) layout(gtx layout.Context, e *env.Env) bool {
	if !o.active {
		return false
	}
	th := e.Theme
	o.back.Kind, o.back.Label = kit.PlaqueQuiet, i18n.T("Back")
	o.skip.Kind, o.skip.Label = kit.PlaqueQuiet, i18n.T("Skip this step")
	o.next.Kind, o.next.Label = kit.PlaqueGold, i18n.T("Continue")
	if o.card == tourHello {
		o.skip.Label = i18n.T("Skip tour")
	}
	if o.card == tourTask {
		o.next.Label = i18n.T("Finish tour")
	}
	if o.back.Clicked(gtx) && !o.busy {
		o.move(-1)
	}
	if o.skip.Clicked(gtx) {
		if o.card == tourHello || o.card == tourTask {
			o.finish(gtx, e)
		} else {
			o.serial++
			o.busy = false
			o.move(1)
		}
	}
	if o.next.Clicked(gtx) && !o.busy {
		if o.card == tourTask {
			o.finish(gtx, e)
		} else {
			o.move(1)
		}
	}
	if !o.active {
		return false
	}
	for index := range o.choices {
		if o.choices[index].Clicked(gtx) && !o.busy {
			o.choose(gtx, e, index)
		}
	}
	if !o.active {
		return false
	}
	title, text := tourWords(o.card)
	o.dialog.Layout(gtx, th.Theme, title, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg2, i18n.Tf("Step %d of %d · optional · Esc leaves", o.card+1, tourCards))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg, text)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		}
		children = append(children, o.body(e)...)
		if o.problem != "" {
			children = append(children, layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, o.problem)))
		}
		if o.notice != "" {
			children = append(children, layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, o.notice)))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	},
		func(gtx layout.Context) layout.Dimensions {
			if o.card == tourHello || o.busy {
				gtx = gtx.Disabled()
			}
			return o.back.Layout(gtx, th.Theme)
		},
		func(gtx layout.Context) layout.Dimensions { return o.skip.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions {
			if o.busy {
				gtx = gtx.Disabled()
			}
			return o.next.Layout(gtx, th.Theme)
		},
	)
	// Esc and outside-click are a refusal, not consent and not completion.
	// Seen suppresses another automatic offer; Settings can restart at hello.
	if !o.dialog.Open {
		o.leave(gtx)
	}
	return o.active
}

func (o *onboarding) body(e *env.Env) []layout.FlexChild {
	th := e.Theme
	var children []layout.FlexChild
	paragraph := func(text string) {
		children = append(children, layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, text)))
	}
	choice := func(index int, text string) {
		button := &o.choices[index]
		button.Kind, button.Label = kit.PlaqueQuiet, text
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if o.busy {
					gtx = gtx.Disabled()
				}
				return look.Named(gtx, text, func(gtx layout.Context) layout.Dimensions { return button.Layout(gtx, th.Theme) })
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		)
	}
	switch o.card {
	case tourHello:
		paragraph(i18n.T("No account, microphone or screen access is needed for this tour. It never enables an optional desktop feature or changes what starts at login. You can use Tab, Shift+Tab and Enter on the buttons."))
	case tourDoors:
		choice(0, i18n.T("Find a local model"))
		choice(1, i18n.T("Use my own provider key"))
		choice(2, i18n.T("Explore optional Ryoku Ember"))
		paragraph(i18n.T("Each door opens its existing setup. Local models and your own keys need no account. Ryoku Account is only for Ember and purchases; provider sign-ins are separate and optional. Re-run the tour from Settings afterwards."))
	case tourName:
		o.name.Label = i18n.T("What should Hyakki call you?")
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return o.name.Layout(gtx, th.Theme) }))
		choice(0, i18n.T("Remember my name locally"))
		paragraph(i18n.T("Only pressing Remember saves this answer as your own memory. This does not enable learning or conversation memory. You can edit or forget it in Memory. Continue leaves the answer unsaved."))
	case tourVoice:
		choice(0, i18n.T("Test microphone once"))
		choice(1, i18n.T("Open voice settings"))
		paragraph(i18n.T("Test listens briefly for sound levels only, on this computer. It saves no recording, performs no transcription and enables neither Talk to Oni nor dictation. It may be unavailable until voice is enabled explicitly in Settings."))
	case tourChord:
		paragraph(i18n.T("Talk to Oni and dictation use the shortcuts you choose in Settings. Hold-to-talk ends when you release the chord. Esc cancels. The tour does not bind keys or listen for a global shortcut."))
		choice(0, i18n.T("Review voice shortcuts"))
	case tourPlacement:
		paragraph(i18n.T("Oni can perch on a window only after you enable its presence. Fullscreen courtesy steps aside. Looking, Guides, voice and Hands have separate switches and permission rules."))
		choice(0, i18n.T("Review presence and character"))
	case tourGuide:
		o.scratch.Label = i18n.T("Tour scratch note — stays in this dialog")
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return o.scratch.Layout(gtx, th.Theme) }))
		paragraph(i18n.Tf("Practice guide · %d of 2 steps complete", o.guide))
		if o.guide == 0 {
			paragraph(i18n.T("1. Type hello in the scratch note, then mark this step done. This guide can read only that note, never another window."))
		} else if o.guide == 1 {
			paragraph(i18n.T("2. Clear the scratch note yourself, then mark the second step done. Oni never types or clicks for this practice."))
		} else {
			paragraph(i18n.T("Practice complete. A real guide reads the selected window only after you enable Guides and ask for help. You remain in control of every action."))
		}
		choice(0, i18n.T("Mark practice step done"))
	case tourConsent:
		paragraph(i18n.T("Example disclosure — not a live request"))
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return look.Facts(gtx, th, []look.FactRow{
				{Label: i18n.T("Picture"), Value: i18n.T("Tour scratch note only")},
				{Label: i18n.T("Destination"), Value: i18n.T("A hosted provider — leaves this computer")},
				{Label: i18n.T("Approval"), Value: i18n.T("One request, never permanent")},
			})
		}))
		choice(0, i18n.T("No, keep it here"))
		choice(1, i18n.T("Practice Allow once — sends nothing"))
		paragraph(i18n.T("The default is No. Both practice answers send nothing: no picture is captured, no model runs and no permission is saved. Real requests name the picture and destination before you decide. Hands never confirms purchases, payments or destructive actions for you."))
	case tourTask:
		choice(0, i18n.T("Start my first Ask"))
		choice(1, i18n.T("Set up a local model first"))
		choice(2, i18n.T("Review Oni's optional features"))
		paragraph(i18n.T("Try asking: Explain a concept I am learning. Type your own question and choose a model; nothing is submitted by the tour. You can re-run this tour in Settings › Hyakki."))
	}
	return children
}

func (o *onboarding) choose(gtx layout.Context, e *env.Env, choice int) {
	switch o.card {
	case tourDoors:
		o.leave(gtx)
		if choice == 2 {
			e.OpenAccount()
		} else {
			e.OpenSettings("providers")
		}
	case tourName:
		if choice == 0 {
			o.remember(e)
		}
	case tourVoice:
		if choice == 0 {
			o.testMic(e)
		} else if choice == 1 {
			o.leave(gtx)
			e.OpenSettings("oni-voice")
		}
	case tourChord:
		o.leave(gtx)
		e.OpenSettings("oni-voice")
	case tourPlacement:
		o.leave(gtx)
		e.OpenSettings("oni")
	case tourGuide:
		switch {
		case o.guide == 0 && strings.EqualFold(strings.TrimSpace(o.scratch.Text()), "hello"):
			o.guide = 1
			o.problem = ""
		case o.guide == 1 && strings.TrimSpace(o.scratch.Text()) == "":
			o.guide = 2
			o.problem = ""
		case o.guide == 0:
			o.problem = i18n.T("Type hello in the tour scratch note first.")
		case o.guide == 1:
			o.problem = i18n.T("Clear the tour scratch note first.")
		}
	case tourConsent:
		o.consent = choice == 1
		o.notice = i18n.T("Practice answered. Nothing was captured, sent or approved for later.")
	case tourTask:
		o.finish(gtx, e)
		switch choice {
		case 0:
			e.NewAsk()
		case 1:
			e.OpenSettings("providers")
		case 2:
			e.OpenSettings("oni")
		}
	}
}

func tourNameText(name string) (string, string) {
	name = strings.TrimSpace(name)
	if sensitive.Number(name) != sensitive.None {
		return "", i18n.T("Hyakki doesn't keep card, social security, ID or bank account numbers.")
	}
	if threat, hit := scan.Strict(name); hit {
		if threat.Secret {
			return "", i18n.T("It looks like a password or key.")
		}
		return "", i18n.T("Use a name, not instructions or hidden characters.")
	}
	if sensitive.Secret(name) {
		return "", i18n.T("It looks like a password or key.")
	}
	if name == "" || utf8.RuneCountInString(name) > 80 || strings.ContainsAny(name, "\r\n") {
		return "", i18n.T("Use one name of at most 80 characters.")
	}
	return "My name is " + name, ""
}

func (o *onboarding) remember(e *env.Env) {
	text, problem := tourNameText(o.name.Text())
	if problem != "" {
		o.problem = problem
		return
	}
	engine := e.Snapshot().Settings.Memory.Engine
	if engine != "" && engine != protocol.MemoryBuiltin {
		o.problem = i18n.T("Your memory engine is external. Choose Built in in Memory settings before saving a name locally.")
		return
	}
	o.busy, o.problem = true, ""
	serial := o.serial
	e.ChangeMemory(protocol.MMemoryAdd, protocol.MemoryAddParams{Scope: protocol.MemoryYou, Text: text}, func(_ protocol.MemoryChange, err error) {
		if serial != o.serial {
			return
		}
		o.busy = false
		if err != nil {
			o.problem = err.Error()
			return
		}
		o.name.SetText("")
		o.notice = i18n.T("Your name was saved locally. No optional feature was enabled.")
	})
}

func (o *onboarding) testMic(e *env.Env) {
	o.busy, o.problem = true, ""
	serial := o.serial
	var result protocol.OniVoiceTestResult
	e.Call(protocol.MOniVoiceTest, protocol.OniVoiceTestParams{}, &result, func(err error) {
		if serial != o.serial {
			return
		}
		o.busy = false
		if err != nil {
			o.problem = err.Error()
			return
		}
		peak := 0.0
		for _, level := range result.Levels {
			peak = max(peak, level)
		}
		o.notice = i18n.Tf("Microphone test finished. Peak level: %s. No recording was saved.", fmt.Sprintf("%.0f%%", peak*100))
	})
}

func tourWords(card int) (string, string) {
	switch card {
	case tourHello:
		return i18n.T("Welcome to Hyakki"), i18n.T("An agent harness for local models and your own providers, with no account required. Oni is an optional companion; The Yokai brings agents together for larger work.")
	case tourDoors:
		return i18n.T("Choose where your model runs"), i18n.T("Use a fitting local model, bring your own provider key, or optionally use Ryoku Ember. Downloading a model and using a hosted provider require network access; opening this tour does not.")
	case tourName:
		return i18n.T("A name, only if you want"), i18n.T("You can use Hyakki without telling it anything about yourself.")
	case tourVoice:
		return i18n.T("Your microphone, your choice"), i18n.T("Voice is optional. No listening starts on this card until you press Test microphone once.")
	case tourChord:
		return i18n.T("Shortcuts without surprises"), i18n.T("Review a shortcut before using it. Talking to Oni, dictation and screen help are different actions, each with its own switch.")
	case tourPlacement:
		return i18n.T("Give Oni a place"), i18n.T("Presence is optional and separate from what Oni is allowed to see or do.")
	case tourGuide:
		return i18n.T("Try a guide safely"), i18n.T("This two-step practice uses only the scratch note below. It cannot see or operate your desktop.")
	case tourConsent:
		return i18n.T("You decide what leaves"), i18n.T("Before screen help sends a picture to a hosted model, a disclosure names its destination. Saying No is always an option.")
	default:
		return i18n.T("Make the first task yours"), i18n.T("The tour has not enabled Oni, memory learning, Hands, MCP, voice or autostart. Existing choices are unchanged. Start with a question, or finish setup at your own pace.")
	}
}
