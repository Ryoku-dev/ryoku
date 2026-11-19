package chat

import (
	"image"
	"strings"
	"unicode/utf16"

	"gioui.org/io/semantic"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	i18n "ryoku-i18n"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type draftIssueActions struct {
	apply, add kit.Plaque
	message    inlineState
	locale     system.Locale
	preview    string
}

func (a *askComposerState) reviewWriting(draft string, result *protocol.WritingCheckResult) {
	a.writingResult, a.writingDraft = result, draft
	a.issueActions = make([]draftIssueActions, len(result.Issues))
	units := utf16.Encode([]rune(draft))
	for index, issue := range result.Issues {
		if len(issue.Replacements) == 0 || issue.Start < 0 || issue.End < issue.Start || issue.End > len(units) {
			continue
		}
		original := string(utf16.Decode(units[issue.Start:issue.End]))
		a.issueActions[index].preview = i18n.Tf("%s → %s", original, issue.Replacements[0])
	}
}

func (a *askComposerState) closeWritingMenu() {
	if a.hooks.Overlays != nil {
		a.hooks.Overlays.Close(&a.writing.Clickable)
	}
}

func (a *askComposerState) writingActions() []kit.MenuItem {
	if a.writingChoices == nil {
		a.writingChoices = make([]kit.MenuItem, 5, 6)
		a.writingChoices[0] = kit.MenuItem{Label: i18n.T("Fix writing")}
		a.writingChoices[1] = kit.MenuItem{Label: i18n.T("Rewrite")}
		a.writingChoices[2] = kit.MenuItem{Label: i18n.T("Shorter")}
		a.writingChoices[3] = kit.MenuItem{Label: i18n.T("More formal")}
		a.writingChoices[4] = kit.MenuItem{Label: i18n.T("More friendly")}
	}
	if a.voice && len(a.writingChoices) == 5 {
		a.writingChoices = append(a.writingChoices, kit.MenuItem{Label: i18n.T("Sound like me")})
	} else if !a.voice {
		a.writingChoices = a.writingChoices[:5]
	}
	return a.writingChoices
}

func (state *composerState) layoutWritingMenu(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	a := state.ask
	a.writing.Kind, a.writing.Small, a.writing.Busy = kit.PlaqueQuiet, true, a.busy
	a.writing.Label, a.writing.Icon = i18n.T("Writing"), writingPromptIcon
	overlays := a.hooks.Overlays
	tag := &a.writing.Clickable
	if a.writing.Clicked(gtx) && overlays != nil {
		if overlays.IsOpen(tag) {
			overlays.Close(tag)
		} else {
			items := a.writingActions()
			overlays.Open(tag, overlays.AnchorOf(tag, a.writingMenuSize), look.Placement{Return: &state.editor}, func(gtx layout.Context) layout.Dimensions {
				dimensions := a.writingMenu.Layout(gtx, th, items)
				if index, clicked := a.writingMenu.Clicked(gtx); clicked {
					overlays.Close(tag)
					if !a.busy && a.feature("askwriting") {
						switch index {
						case 0:
							state.checkDraft()
						case 1:
							state.transformDraft("rewrite", map[string]string{"variant": "rephrase"}, i18n.T("Rewritten, Ctrl+Z to undo"))
						case 2:
							state.transformDraft("rewrite", map[string]string{"variant": "shorter"}, i18n.T("Shortened, Ctrl+Z to undo"))
						case 3:
							state.transformDraft("tone", map[string]string{"tone": "formal"}, i18n.T("Tone changed, Ctrl+Z to undo"))
						case 4:
							state.transformDraft("tone", map[string]string{"tone": "friendly"}, i18n.T("Tone changed, Ctrl+Z to undo"))
						case 5:
							if a.voice {
								state.transformDraft("rewrite", map[string]string{"variant": "rephrase", "voice": "on"}, i18n.T("Rewritten in your voice, Ctrl+Z to undo"))
							}
						}
					}
				}
				return dimensions
			})
		}
	}
	if overlays != nil && overlays.IsOpen(tag) {
		a.writing.Kind = kit.PlaqueOn
	}
	dimensions := a.writing.Layout(gtx, th)
	a.writingMenuSize = dimensions.Size
	return dimensions
}
func writingMessageSpans(source string) []inlineSpan {
	var spans []inlineSpan
	plain := 0
	for at := 0; at < len(source); {
		next := strings.IndexByte(source[at:], '`')
		if next < 0 {
			break
		}
		at += next
		code, end, ok := codeSpan(source, at)
		if !ok {
			at += runLength(source, at)
			continue
		}
		if at > plain {
			spans = append(spans, inlineSpan{text: source[plain:at]})
		}
		spans = append(spans, inlineSpan{text: code, style: inlineCode})
		at, plain = end, end
	}
	if plain < len(source) {
		spans = append(spans, inlineSpan{text: source[plain:]})
	}
	return spans
}

func (actions *draftIssueActions) layoutMessage(gtx layout.Context, th *theme.Theme, source string) layout.Dimensions {
	message := &actions.message
	if !message.parsed || message.source != source {
		message.source, message.parsed, message.set = source, true, nil
		// only backticks have meaning here; markup from a checker stays literal
		message.spans = writingMessageSpans(source)
	}
	if len(message.spans) == 0 {
		return layout.Dimensions{}
	}
	current := look.Theme{Theme: th}
	faces := inlineFaces{base: th.T.Detail, mono: codeStyle(&current)}
	width := gtx.Constraints.Max.X
	if message.set == nil || message.width != width || message.faces != faces || message.metric != gtx.Metric || actions.locale != gtx.Locale {
		message.set = setInline(th.Shaper, gtx.Metric, gtx.Locale, faces, message.spans, width)
		message.width, message.faces, message.metric, actions.locale = width, faces, gtx.Metric, gtx.Locale
	}
	set := message.set
	size := image.Pt(max(set.size.X, gtx.Constraints.Min.X), set.size.Y)
	area := clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops)
	semantic.LabelOp(set.label).Add(gtx.Ops)
	area.Pop()
	var painter viewState
	painter.paintInline(gtx, &current, message, th.R.Fg2, -1)
	return layout.Dimensions{Size: size, Baseline: size.Y - set.baseline}
}

func (state *composerState) layoutWritingIssues(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	a := state.ask
	if !a.feature("askwriting") || a.writingResult == nil {
		return layout.Dimensions{}
	}
	result := a.writingResult
	attribution := i18n.T("Checked on this computer")
	if result.Engine == "model" {
		attribution = i18n.T("Checked with your model")
	}
	if len(result.Issues) == 0 {
		attribution += ". " + i18n.T("No writing fixes needed")
	}
	a.applyWriting.Kind, a.applyWriting.Small, a.applyWriting.Label = kit.PlaqueTonal, true, i18n.T("Apply fixes")
	a.dismissWriting.Kind, a.dismissWriting.Small, a.dismissWriting.Label = kit.PlaqueQuiet, true, i18n.T("Dismiss")
	if a.applyWriting.Clicked(gtx) {
		state.replaceDraft(applyDraftFixes(a.writingDraft, result.Issues), i18n.T("Writing fixed, Ctrl+Z to undo"))
		a.writingResult = nil
		return layout.Dimensions{}
	}
	if a.dismissWriting.Clicked(gtx) {
		a.writingResult = nil
		return layout.Dimensions{}
	}
	return layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceEnd}.Layout(gtx,
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(kit.Text(th, th.T.Detail, th.R.Fg2, attribution)),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(180))
			gtx.Constraints.Min.Y = 0
			record := op.Record(gtx.Ops)
			current := look.Theme{Theme: th}
			dimensions := a.issueList.Layout(gtx, &current, len(result.Issues), func(gtx layout.Context, index int) layout.Dimensions {
				issue := result.Issues[index]
				actions := &a.issueActions[index]
				return layout.Inset{Top: th.Sp.S1, Bottom: th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return actions.layoutMessage(gtx, th, issue.Message)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if actions.preview == "" {
								return layout.Dimensions{}
							}
							return kit.Text(th, codeStyle(&current), th.R.Fg2, actions.preview)(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							var controls []layout.Widget
							if len(issue.Replacements) > 0 {
								actions.apply.Kind, actions.apply.Small, actions.apply.Label = kit.PlaqueTonal, true, i18n.T("Apply")
								if actions.apply.Clicked(gtx) {
									state.replaceDraft(applyDraftFixes(a.writingDraft, []protocol.WritingIssue{issue}), i18n.T("Writing fixed, Ctrl+Z to undo"))
									a.writingResult = nil
								}
								controls = append(controls, func(gtx layout.Context) layout.Dimensions { return actions.apply.Layout(gtx, th) })
							}
							if issue.Category == "spelling" {
								actions.add.Kind, actions.add.Small, actions.add.Label, actions.add.Busy = kit.PlaqueQuiet, true, i18n.T("Add to dictionary"), a.busy
								if actions.add.Clicked(gtx) {
									state.addDraftWord(issue)
								}
								controls = append(controls, func(gtx layout.Context) layout.Dimensions { return actions.add.Layout(gtx, th) })
							}
							return look.Flow{Gap: 8, LineGap: 8}.Layout(gtx, len(controls), func(gtx layout.Context, i int) layout.Dimensions { return controls[i](gtx) })
						}),
					)
				})
			})
			call := record.Stop()
			bounds := clip.Rect(image.Rectangle{Max: dimensions.Size}).Push(gtx.Ops)
			call.Add(gtx.Ops)
			bounds.Pop()
			return dimensions
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if len(result.Issues) == 0 {
						return layout.Dimensions{}
					}
					return a.applyWriting.Layout(gtx, th)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.dismissWriting.Layout(gtx, th) }),
			)
		}),
	)
}

func (state *composerState) addDraftWord(issue protocol.WritingIssue) {
	a := state.ask
	if a.busy || !a.feature("askwriting") || a.writingDraft != state.editor.Text() {
		return
	}
	units := utf16.Encode([]rune(a.writingDraft))
	if issue.Start < 0 || issue.End <= issue.Start || issue.End > len(units) {
		return
	}
	word := string(utf16.Decode(units[issue.Start:issue.End]))
	epoch, draft := a.epoch, a.writingDraft
	request := state.beginWriting("askwriting", draft)
	if request == "" {
		return
	}
	a.hooks.Call(protocol.MWritingWordsAdd, protocol.WritingWordParams{Word: word}, new(protocol.WritingWords), func(err error) {
		if !state.finishWriting(request, err) {
			return
		}
		if epoch == a.epoch && draft == state.editor.Text() && a.feature("askwriting") {
			state.checkDraft()
		}
	})
}
