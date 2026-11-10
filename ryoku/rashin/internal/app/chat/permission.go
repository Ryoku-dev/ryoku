package chat

import (
	"image"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type permissionVisual struct {
	field  look.Unfold
	record look.Unfold
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
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return visual.field.Layout(gtx, th, open, func(gtx layout.Context) layout.Dimensions {
				return state.rise("permission:"+request).Layout(gtx, th, request, 8, func(gtx layout.Context) layout.Dimensions {
					return state.layoutPermissionField(gtx, th, permission.data)
				})
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return visual.record.Layout(gtx, th, !open, func(gtx layout.Context) layout.Dimensions {
				label := permissionOutcome(permission) + ": " + permission.data.Title
				return state.layoutRecord(gtx, th, "permission:"+request, look.Icon.Key, label, firstLine(permission.data.Input))
			})
		}),
	)
}

func (state *viewState) layoutPermissionField(gtx layout.Context, th *look.Theme, data protocol.PermissionData) layout.Dimensions {
	gtx.Constraints.Min = image.Point{}
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(560))
	record := op.Record(gtx.Ops)
	dimensions := layout.UniformInset(16).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		children := []layout.FlexChild{
			layout.Rigid(kit.Claim(th.Theme, palette.Attention, i18n.T("Needs you"))),
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
	call := record.Stop()
	kit.FillCut(gtx.Ops, image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Mid), th.R.Field(palette.Attention))
	call.Add(gtx.Ops)
	return dimensions
}

func (state *viewState) layoutPermissionActions(gtx layout.Context, th *look.Theme, data protocol.PermissionData) layout.Dimensions {
	return look.Flow{Gap: 8, LineGap: 8}.Layout(gtx, len(data.Options), func(gtx layout.Context, index int) layout.Dimensions {
		option := data.Options[index]
		plaque := state.plaque("permission:" + data.Request + ":" + option.ID)
		plaque.Label, plaque.Small = i18n.T(option.Label), true
		switch option.Kind {
		case protocol.OptAllowOnce:
			plaque.Kind = kit.PlaqueGold
		case protocol.OptAllowAlways:
			plaque.Kind = kit.PlaqueLine
		default:
			plaque.Kind = kit.PlaqueQuiet
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

// enter gives the narrowest consent, escape refuses
func (state *viewState) handlePermissionKeys(gtx layout.Context, model *modelState) {
	var open *permissionState
	for _, request := range model.permissionOrder {
		if candidate := model.permissions[request]; candidate != nil && candidate.resolved == nil {
			open = candidate
			break
		}
	}
	if open == nil {
		return
	}
	answer := func(name key.Name, kinds ...string) {
		for {
			raw, ok := gtx.Event(key.Filter{Name: name})
			if !ok {
				return
			}
			pressed, ok := raw.(key.Event)
			if !ok || pressed.State != key.Press || pressed.Modifiers != 0 {
				continue
			}
			for _, kind := range kinds {
				if option := permissionOption(open.data, kind); option != "" {
					state.act(Action{Kind: ActPermission, Request: open.data.Request, Option: option})
					break
				}
			}
		}
	}
	answer(key.NameReturn, protocol.OptAllowOnce)
	answer(key.NameEscape, protocol.OptRejectOnce, protocol.OptRejectAlways)
}

func permissionOption(data protocol.PermissionData, kind string) string {
	for _, option := range data.Options {
		if option.Kind == kind {
			return option.ID
		}
	}
	return ""
}
