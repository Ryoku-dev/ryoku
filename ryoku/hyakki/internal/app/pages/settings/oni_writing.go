package settings

import (
	"slices"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type oniWriting struct {
	recall, vault, rewrite, voice, assignment, guard kit.Toggle
	rewriteStyle, assignmentStyle                    look.Select
	model                                            env.ModelForm
	modelFilled                                      bool
	modelStored                                      string
	words                                            kit.TextField
	wordsFilled                                      bool
	wordsStored                                      []string
	wordsBaseline                                    string
	saveWords                                        kit.Plaque
	wordsError                                       string
	review                                           oniRewriteReview
	approvals                                        oniApprovals
}

// The switches only grant availability. Recall still discloses hosted
// passages, rewrite still requires its selection card, and the guard is
// armed explicitly for one session, never merely by opening Settings.
func (p *Page) oniWritingGroups(e *env.Env, oni, defaults protocol.OniSettings) []layout.Widget {
	th, w := e.Theme, &p.oni.writing
	switchRow := func(title, detail, section, key string, current, stock bool, toggle *kit.Toggle) look.Setting {
		toggle.Value = current
		patch := func(value bool) { set(e, "oni", map[string]any{section: map[string]any{key: value}}) }
		return look.Setting{Title: title, Detail: detail, Changed: current != stock, Reset: func() { patch(stock) },
			Control: func(gtx layout.Context) layout.Dimensions { return namedSwitch(gtx, th, title, toggle, patch) }}
	}
	recall := []look.Setting{
		switchRow(i18n.T("Recall what I kept"), i18n.T("Answer from imported chats, past conversations and memory facts. Local answers stay here; hosted answers need your disclosure approval."), "recall", "enabled", oni.Recall.Enabled, defaults.Recall.Enabled, &w.recall),
		switchRow(i18n.T("Include the Ryoku notes vault"), i18n.T("Read matching passages from the Ryoku notes folder, if present. This is not the credential vault. Needs Recall what I kept."), "recall", "vault", oni.Recall.Vault, defaults.Recall.Vault, &w.vault),
	}
	rewrite := []look.Setting{
		switchRow(i18n.T("Rewrite in place"), i18n.T("Rewrite selected text only after you approve its card. Oni checks the focused window and selection before replacing anything; undo keeps the original for the current turn."), "rewrite", "enabled", oni.Rewrite.Enabled, defaults.Rewrite.Enabled, &w.rewrite),
		{Title: i18n.T("Default rewrite style"), Changed: oni.Rewrite.Style != defaults.Rewrite.Style,
			Reset: func() { set(e, "oni", map[string]any{"rewrite": map[string]any{"style": defaults.Rewrite.Style}}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				w.rewriteStyle.Label = i18n.T("Rewrite style")
				gtx.Constraints.Min.X = gtx.Dp(selectWidth)
				labels := []string{i18n.T("Grammar"), i18n.T("Friendlier"), i18n.T("Formal"), i18n.T("Casual"), i18n.T("Polite"), i18n.T("Confident"), i18n.T("Shorter"), i18n.T("Longer"), i18n.T("Simpler"), i18n.T("Email"), i18n.T("Bullets")}
				dims := w.rewriteStyle.Layout(gtx, th, e.Overlays, labels, indexOf(protocol.OniRewriteStyles, oni.Rewrite.Style))
				if at, ok := w.rewriteStyle.Changed(); ok && at >= 0 && at < len(protocol.OniRewriteStyles) {
					set(e, "oni", map[string]any{"rewrite": map[string]any{"style": protocol.OniRewriteStyles[at]}})
				}
				return dims
			}},
		switchRow(i18n.T("Rewrite in my voice"), i18n.T("Use only your My voice notes as writing examples, not unrelated facts from memory. Needs Rewrite in place."), "rewrite", "voice", oni.Rewrite.Voice, defaults.Rewrite.Voice, &w.voice),
	}
	if !w.modelFilled || w.modelStored != oni.Assignment.Model {
		w.model.Set(oni.Assignment.Model, nil)
		w.modelStored, w.modelFilled = oni.Assignment.Model, true
	}
	assignment := []look.Setting{
		switchRow(i18n.T("Assignment mode"), i18n.T("Work through problems in the perched window. Needs Oni and looking at a window; a hosted model still asks before a picture leaves this computer."), "assignment", "enabled", oni.Assignment.Enabled, defaults.Assignment.Enabled, &w.assignment),
		{Title: i18n.T("Assignment answers"), Changed: oni.Assignment.Style != defaults.Assignment.Style,
			Reset: func() {
				set(e, "oni", map[string]any{"assignment": map[string]any{"style": defaults.Assignment.Style}})
			},
			Control: func(gtx layout.Context) layout.Dimensions {
				w.assignmentStyle.Label = i18n.T("Assignment style")
				gtx.Constraints.Min.X = gtx.Dp(selectWidth)
				dims := w.assignmentStyle.Layout(gtx, th, e.Overlays, []string{i18n.T("Walk me through it"), i18n.T("Answers only")}, indexOf(protocol.OniAssignmentStyles, oni.Assignment.Style))
				if at, ok := w.assignmentStyle.Changed(); ok && at >= 0 && at < len(protocol.OniAssignmentStyles) {
					set(e, "oni", map[string]any{"assignment": map[string]any{"style": protocol.OniAssignmentStyles[at]}})
				}
				return dims
			}},
		{Title: i18n.T("Assignment model"), Detail: i18n.T("Hyakki's choice tries the smallest local model first and never a paid one. A pinned hosted model is disclosed before use."), Changed: oni.Assignment.Model != defaults.Assignment.Model,
			Reset: func() {
				set(e, "oni", map[string]any{"assignment": map[string]any{"model": defaults.Assignment.Model}})
			},
			Control: func(gtx layout.Context) layout.Dimensions {
				dims := w.model.Layout(gtx, e)
				if _, picked := w.model.Changed(); picked {
					model, _ := w.model.Value()
					if model == protocol.RouteAuto {
						model = ""
					}
					set(e, "oni", map[string]any{"assignment": map[string]any{"model": model}})
				}
				return dims
			}},
	}
	if !w.wordsFilled || !slices.Equal(w.wordsStored, oni.Feeds.Words) {
		w.wordsBaseline = strings.Join(oni.Feeds.Words, "\n")
		w.words.SetText(w.wordsBaseline)
		w.wordsStored, w.wordsFilled = slices.Clone(oni.Feeds.Words), true
	}
	w.words.Editor.MaxLen = protocol.OniFeedWordsMax * (protocol.OniFeedWordMaxRunes + 1)
	guard := []look.Setting{
		switchRow(i18n.T("Focus guard"), i18n.T("Allow an explicitly armed focus session to close matching feed windows and authorized browser tabs, announcing each close. Say focus for a duration to arm it, stop focus to stop it, or let me back to reopen. Turning this on does not arm a session."), "feeds", "guard", oni.Feeds.Guard, defaults.Feeds.Guard, &w.guard),
		{Title: i18n.T("Feed words"), Detail: i18n.T("One app, site or feed word per line. Only the words you choose may match during a focus session."), Below: true,
			Changed: !slices.Equal(oni.Feeds.Words, defaults.Feeds.Words), Reset: func() { set(e, "oni", map[string]any{"feeds": map[string]any{"words": defaults.Feeds.Words}}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return look.Named(gtx, i18n.T("Focus guard feed words"), func(gtx layout.Context) layout.Dimensions { return w.words.Layout(gtx, th.Theme) })
			}},
	}
	changed := w.words.Text() != w.wordsBaseline
	save := func(gtx layout.Context) layout.Dimensions {
		w.saveWords.Kind, w.saveWords.Small, w.saveWords.Label = kit.PlaqueTonal, true, i18n.T("Save feed words")
		if w.saveWords.Clicked(gtx) && changed {
			words := strings.Split(w.words.Text(), "\n")
			if strings.TrimSpace(w.words.Text()) == "" {
				words = nil
			}
			clean, valid := protocol.CleanOniFeedWords(words)
			if !valid {
				w.wordsError = i18n.T("Use at most 32 non-empty feed words, each at most 40 characters.")
			} else {
				w.wordsError = ""
				set(e, "oni", map[string]any{"feeds": map[string]any{"words": clean}})
			}
		}
		if !changed {
			gtx = gtx.Disabled()
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return w.saveWords.Layout(gtx, th.Theme) }),
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, w.wordsError)),
		)
	}
	rewrite = append(rewrite, w.review.rows(e)...)
	return []layout.Widget{
		group(th, i18n.T("Oni approvals"), "", w.approvals.rows(e), nil),
		group(th, i18n.T("Recall"), "", recall, nil),
		group(th, i18n.T("Rewrite"), "", rewrite, nil),
		group(th, i18n.T("Assignments"), "", assignment, nil),
		group(th, i18n.T("Focus"), "", guard, save),
		group(th, i18n.T("Memory by voice"), i18n.T("The Notes switch under Quick commands lets your own spoken remember that words enter memory. Say forget that or forget about a topic to remove your facts; a whole-memory deletion still asks you. Memory must be on. Rewrite in my voice uses your My voice notes only."), nil, nil),
	}
}
