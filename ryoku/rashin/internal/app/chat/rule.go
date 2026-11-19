package chat

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type ruleVisual struct {
	open, pending, saved bool
	complaint            look.MultiLine
	proposal             *protocol.Rule
	matched              string
	errorText            string
}

func (state *viewState) layoutRule(gtx layout.Context, th *look.Theme, block blockContext) layout.Dimensions {
	visual := &state.turnVisual(block.turn).rule
	if !visual.open || !state.feature(protocol.FeatureRules) || state.call == nil {
		return layout.Dimensions{}
	}
	key := "rule:" + block.key()
	return layout.Inset{Top: th.Sp.S2, Bottom: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return state.slab(gtx, th, th.Sp.S4, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			children := []layout.FlexChild{
				layout.Rigid(kit.Text(th.Theme, th.T.Title, th.R.Fg, i18n.T("Make a rule from this answer"))),
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			}
			if visual.proposal == nil && !visual.saved {
				visual.complaint.Label = i18n.T("What was wrong?")
				visual.complaint.Placeholder = i18n.T("Tell Rashin what it should do differently next time")
				visual.complaint.Lines = 3
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return visual.complaint.Layout(gtx, th) }))
			} else if visual.proposal != nil {
				children = append(children,
					layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg, visual.proposal.Name)),
					layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return state.selectable(gtx, th, key+":body", th.T.Body, th.R.Fg2, visual.proposal.Body)
					}),
				)
				if visual.matched != "" {
					children = append(children,
						layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
						layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, i18n.T("Caught in this conversation"))),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return state.selectable(gtx, th, key+":matched", th.T.Detail, th.R.Fg2, visual.matched)
						}),
					)
				}
			} else {
				children = append(children, layout.Rigid(kit.Text(th.Theme, th.T.Detail, th.R.Fg2, i18n.T("Rule saved"))))
			}
			if visual.errorText != "" {
				children = append(children, layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, visual.errorText)))
			}
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							button := state.plaque(key + ":discard")
							button.Kind, button.Small, button.Label = kit.PlaqueQuiet, true, i18n.T("Discard")
							if button.Clicked(gtx) && !visual.pending {
								visual.open, visual.proposal, visual.matched, visual.errorText = false, nil, "", ""
							}
							return button.Layout(gtx, th.Theme)
						}),
						layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							button := state.plaque(key + ":submit")
							button.Kind, button.Small = kit.PlaqueTonal, true
							button.Label = i18n.T("Propose a rule")
							if visual.proposal != nil {
								button.Label = i18n.T("Save rule")
							}
							if visual.saved {
								return layout.Dimensions{}
							}
							if button.Clicked(gtx) && !visual.pending {
								if visual.proposal != nil {
									visual.pending = true
									var result protocol.Rule
									state.call(protocol.MLearningRuleSave, protocol.RuleSaveParams{Rule: *visual.proposal}, &result, func(err error) {
										visual.pending = false
										if err != nil {
											visual.errorText = err.Error()
										} else {
											visual.saved, visual.proposal, visual.errorText = true, nil, ""
										}
									})
								} else if complaint := strings.TrimSpace(visual.complaint.Text()); complaint != "" {
									visual.pending = true
									var result protocol.RuleForgeResult
									state.call(protocol.MLearningRuleForge, protocol.RuleForgeParams{Session: state.session, Complaint: complaint}, &result, func(err error) {
										visual.pending = false
										switch {
										case err != nil:
											visual.errorText = err.Error()
										case !result.OK:
											visual.errorText = result.Message
										default:
											visual.proposal, visual.matched, visual.errorText = &result.Rule, result.Matched, ""
										}
									})
								} else {
									visual.errorText = i18n.T("Describe what went wrong first.")
								}
							}
							if visual.pending {
								gtx = gtx.Disabled()
							}
							return button.Layout(gtx, th.Theme)
						}),
					)
				}),
			)
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}
