package chat

import (
	"image"
	"time"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type permissionVisual struct {
	field  kit.Disclosure
	record kit.Disclosure
	// when the request was first drawn, and the last frame it was drawn in
	seen  time.Time
	drawn int
}

func (state *viewState) layoutPermission(gtx layout.Context, th *look.Theme, permission *permissionState) layout.Dimensions {
	if permission == nil {
		return layout.Dimensions{}
	}
	request := permission.data.Request
	visual := state.permissions[request]
	if visual == nil {
		visual = new(permissionVisual)
		state.permissions[request] = visual
	}
	open := permission.resolved == nil
	if visual.seen.IsZero() {
		visual.seen = gtx.Now
	}
	visual.drawn = state.frame
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			visual.field.Open = open
			return visual.field.Layout(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
				return state.rise("permission:"+request).Layout(gtx, th, request, 8, func(gtx layout.Context) layout.Dimensions {
					return state.layoutPermissionField(gtx, th, permission.data)
				})
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			visual.record.Open = !open
			return visual.record.Layout(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
				label := permissionOutcome(permission) + ": " + permission.data.Title
				return state.layoutRecord(gtx, th, "permission:"+request, look.Icon.Key, label, firstLine(permission.data.Input))
			})
		}),
	)
}

func (state *viewState) layoutPermissionField(gtx layout.Context, th *look.Theme, data protocol.PermissionData) layout.Dimensions {
	gtx.Constraints.Min = image.Point{}
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(560))
	return state.slab(gtx, th, th.Sp.S4, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		claim, tone := i18n.T("Needs you"), palette.Attention
		if data.Kind == protocol.PermissionQuestion {
			claim, tone = i18n.T("Question"), palette.Becoming
		} else if data.Kind == protocol.PermissionPlan {
			claim = i18n.T("Plan ready")
		}
		children := []layout.FlexChild{
			layout.Rigid(kit.Claim(th.Theme, tone, claim)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Title, th.R.Fg, data.Title)),
		}
		if data.Input != "" {
			input, _ := firstLines(data.Input, 6)
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return state.selectable(gtx, th, "permission:"+data.Request+":input", th.T.Mono, th.R.Fg2, input)
				}),
			)
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return state.layoutPermissionActions(gtx, th, data) }),
		)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (state *viewState) layoutPermissionActions(gtx layout.Context, th *look.Theme, data protocol.PermissionData) layout.Dimensions {
	return look.Flow{Gap: th.Sp.S2, LineGap: th.Sp.S2}.Layout(gtx, len(data.Options), func(gtx layout.Context, index int) layout.Dimensions {
		option := data.Options[index]
		plaque := state.plaque("permission:" + data.Request + ":" + option.ID)
		plaque.Label, plaque.Small = i18n.T(option.Label), true
		switch option.Kind {
		case protocol.OptAllowOnce:
			if data.Request == state.waiting {
				plaque.Kind = kit.PlaqueGold
			} else {
				plaque.Kind = kit.PlaqueTonal
			}
		case protocol.OptAllowAlways:
			plaque.Kind = kit.PlaqueTonal
		default:
			plaque.Kind = kit.PlaqueQuiet
		}
		if data.Kind == protocol.PermissionQuestion && index == 0 && data.Request == state.waiting {
			plaque.Kind = kit.PlaqueGold
		}
		if plaque.Clicked(gtx) {
			state.act(Action{Kind: ActPermission, Request: data.Request, Option: option.ID})
		}
		return plaque.Layout(gtx, th.Theme)
	})
}

func permissionOutcome(permission *permissionState) string {
	if permission.resolved == nil || permission.resolved.Outcome != "selected" {
		return i18n.T("Denied")
	}
	if permission.data.Kind == protocol.PermissionQuestion {
		return i18n.T("Answered")
	}
	if permission.data.Kind == protocol.PermissionPlan {
		if permission.resolved.Option == "build" {
			return i18n.T("Building")
		}
		return i18n.T("Still planning")
	}
	for _, option := range permission.data.Options {
		if option.ID != permission.resolved.Option {
			continue
		}
		switch option.Kind {
		case protocol.OptAllowOnce:
			return i18n.T("Allowed once")
		case protocol.OptAllowAlways:
			return i18n.T("Always allowed")
		case protocol.OptRejectOnce, protocol.OptRejectAlways:
			return i18n.T("Denied")
		}
	}
	return i18n.T("Allowed")
}

func oldestOpen(model *modelState) string {
	if model == nil {
		return ""
	}
	for _, request := range model.permissionOrder {
		if permission := model.permissions[request]; permission != nil && permission.resolved == nil {
			return request
		}
	}
	return ""
}

// how long a request has to have been on screen before a key can answer it
const answerAfter = 500 * time.Millisecond

// the oldest request still open, if it was drawn in the last frame
func (state *viewState) asked(model *modelState) (*permissionState, *permissionVisual) {
	if model == nil {
		return nil, nil
	}
	for _, request := range model.permissionOrder {
		open := model.permissions[request]
		if open == nil || open.resolved != nil {
			continue
		}
		if visual := state.permissions[request]; visual != nil && visual.drawn == state.frame-1 {
			return open, visual
		}
		return nil, nil
	}
	return nil, nil
}

func (state *viewState) asking(model *modelState) AnswerKeys {
	open, _ := state.asked(model)
	if open == nil {
		return AnswerKeys{}
	}
	return AnswerKeys{
		Allow:  open.data.Kind != protocol.PermissionQuestion && permissionOption(open.data, protocol.OptAllowOnce) != "",
		Refuse: open.data.Kind != protocol.PermissionQuestion && permissionOption(open.data, protocol.OptRejectOnce, protocol.OptRejectAlways) != "",
		Asked:  true,
		Plan:   open.data.Kind == protocol.PermissionPlan,
	}
}

// enter gives the narrowest consent, escape refuses
func (state *viewState) answer(model *modelState, kind ActionKind, now time.Time) (Action, bool) {
	open, visual := state.asked(model)
	if open == nil || now.Sub(visual.seen) < answerAfter {
		return Action{}, false
	}
	option := ""
	switch kind {
	case ActAllow:
		option = permissionOption(open.data, protocol.OptAllowOnce)
	case ActRefuse:
		option = permissionOption(open.data, protocol.OptRejectOnce, protocol.OptRejectAlways)
	}
	return Action{Kind: ActPermission, Request: open.data.Request, Option: option}, option != ""
}

// the first of kinds the request offers
func permissionOption(data protocol.PermissionData, kinds ...string) string {
	for _, kind := range kinds {
		for _, option := range data.Options {
			if option.Kind == kind {
				return option.ID
			}
		}
	}
	return ""
}
