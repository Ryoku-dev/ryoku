package shell

import (
	"image"
	"image/color"
	"strings"
	"time"
	"unicode/utf8"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/protocol"
)

// rowClick is a click whose hover fades in and out
type rowClick struct {
	kit.Click
	hover anim.Tween
}

// view is what the popover holds, drawn from the store as it is this frame
func (p *accountPanel) view(gtx layout.Context) layout.Dimensions {
	e := p.env
	th := e.Theme
	snapshot := e.Snapshot()
	st := snapshot.Account
	var main []layout.Widget
	var footer layout.Widget
	problem := firstText(p.problem, st.Error)
	view := st.State
	switch {
	case !snapshot.AccountKnown:
		view = "loading"
		main = p.loading(th)
	case st.State == protocol.AccountUnconfigured:
		main = p.unconfigured(th)
	case st.State == protocol.AccountSigningIn:
		main = p.signingIn(gtx, th, st)
	case st.State == protocol.AccountSignedIn:
		main, footer = p.signedIn(th, st)
	default:
		main, footer = p.signedOut(th, st)
	}
	children := make([]layout.FlexChild, 0, len(main)+6)
	for index, part := range main {
		children = append(children, layout.Rigid(part))
		// a refusal reads right under the act that caused it, never after the explanations
		if problem != "" && index == problemAfter(st.State, len(main)) {
			top := unit.Dp(0)
			if st.State == protocol.AccountSignedIn {
				top = th.Sp.S3
			}
			children = append(children, layout.Rigid(section(th, top, th.Sp.S3, problemBlock(th, problem))))
		}
	}
	children = append(children, layout.Rigid(rule(th)), layout.Rigid(p.security(th)))
	if footer != nil {
		children = append(children, layout.Rigid(rule(th)), layout.Rigid(footer))
	}
	return labelled(gtx, i18n.T("Account panel"), func(gtx layout.Context) layout.Dimensions {
		// each view arrives with the house rise, so a change of view reads as one move
		return p.rise.Layout(gtx, th, view, th.Sp.S3, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

// the main part a problem follows: the sign-in act, else the last part
func problemAfter(state string, parts int) int {
	if state == protocol.AccountSignedOut && parts > 1 {
		return 1
	}
	return parts - 1
}

func problemBlock(th *look.Theme, text string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		record := op.Record(gtx.Ops)
		dims := layout.UniformInset(th.Sp.S3).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					size := gtx.Dp(16)
					gtx.Constraints = layout.Exact(image.Pt(size, size))
					return look.Icon.Error.Layout(gtx, th.R.Jasper)
				}),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Flexed(1, look.Paragraph(th, th.T.Caption, th.R.Fg, text)),
			)
		})
		call := record.Stop()
		bounds := image.Rectangle{Max: dims.Size}
		kit.Fill(gtx, th.Theme, bounds, gtx.Dp(th.Cut.Small), palette.WithAlpha(th.R.Jasper, .12))
		kit.Stroke(gtx, th.Theme, bounds, gtx.Dp(th.Cut.Small), max(1, gtx.Dp(1)), palette.WithAlpha(th.R.Jasper, .45))
		call.Add(gtx.Ops)
		return dims
	}
}

// the panel's own name for a screen reader, over exactly its bounds
func labelled(gtx layout.Context, name string, content layout.Widget) layout.Dimensions {
	record := op.Record(gtx.Ops)
	dimensions := content(gtx)
	call := record.Stop()
	area := clip.Rect(image.Rectangle{Max: dimensions.Size}).Push(gtx.Ops)
	semantic.LabelOp(name).Add(gtx.Ops)
	call.Add(gtx.Ops)
	area.Pop()
	return dimensions
}

// section keeps a part of the panel on its margins
func section(th *look.Theme, top, bottom unit.Dp, content layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: top, Bottom: bottom, Left: th.Sp.S4, Right: th.Sp.S4}.Layout(gtx, content)
	}
}

func rule(th *look.Theme) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions { return look.Rule(gtx, th) }
}

// gated draws a control that cannot be used while on is false
func gated(gtx layout.Context, on bool) layout.Context {
	if !on {
		return gtx.Disabled()
	}
	return gtx
}

// full is a plaque across the width it is given
func full(th *look.Theme, act *wideAct, on bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return act.Layout(gated(gtx, on), th)
	}
}

func (p *accountPanel) loading(th *look.Theme) []layout.Widget {
	return []layout.Widget{section(th, th.Sp.S4, th.Sp.S4, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.Working{Active: true, Size: 16}.Layout(gtx, th.Theme)
			}),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg2, i18n.T("Reading your account"))),
		)
	})}
}

func (p *accountPanel) unconfigured(th *look.Theme) []layout.Widget {
	p.server.Label = i18n.T("Ryoku Account server")
	p.server.Help = i18n.Tf("Ryoku's own is %s. Any https address, or http for a local server.", "https://ai.ryoku.dev")
	p.server.Mono = true
	p.save.Kind, p.save.Label = kit.PlaqueGold, i18n.T("Save")
	return []layout.Widget{
		section(th, th.Sp.S4, th.Sp.S3, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(look.Paragraph(th, th.T.Title, th.R.Fg, i18n.T("Ryoku Account"))),
				layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg2, i18n.T("Ryoku Account is off on this computer. Everything else keeps working without an account."))),
			)
		}),
		section(th, 0, th.Sp.S3, func(gtx layout.Context) layout.Dimensions { return p.server.Layout(gtx, th.Theme) }),
		section(th, 0, th.Sp.S4, full(th, &p.save, strings.TrimSpace(p.server.Text()) != "")),
	}
}

func (p *accountPanel) signedOut(th *look.Theme, st protocol.AccountState) ([]layout.Widget, layout.Widget) {
	p.signIn.Kind, p.signIn.Label, p.signIn.Busy = kit.PlaqueGold, i18n.T("Sign in to Ryoku"), st.Busy
	main := []layout.Widget{
		section(th, th.Sp.S4, th.Sp.S3, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(look.RyokuSeal(th, 32)),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(look.Paragraph(th, th.T.Title, th.R.Fg, i18n.T("Ryoku Account"))),
						layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg2, i18n.T("One account for Ryoku Ember and your purchases."))),
					)
				}),
			)
		}),
		section(th, 0, th.Sp.S3, full(th, &p.signIn, true)),
		section(th, 0, th.Sp.S3, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				// what signing in gets, before what works without it
				layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg, i18n.T("Sign in to use Ryoku Ember's hosted models, paid with pay-as-you-go Ember credits."))),
				layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg2, i18n.T("Local models and your own keys work without an account."))),
			}
			if st.Notice != "" {
				children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg3, st.Notice)))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		}),
	}
	return main, p.serverLine(th, st, nil)
}

func (p *accountPanel) signedIn(th *look.Theme, st protocol.AccountState) ([]layout.Widget, layout.Widget) {
	p.signOut.Kind, p.signOut.Small, p.signOut.Label = kit.PlaqueQuiet, true, i18n.T("Sign out")
	page := protocol.AccountManage(st.Manage)
	main := []layout.Widget{p.identity(th, st), rule(th), p.balance(th, st)}
	// a fresh account has nothing to spend: buying credits is the next step, so it leads
	if emptyBalance(st) {
		p.add.Kind, p.add.Icon, p.add.Label = kit.PlaqueGold, look.Icon.OpenInNew, i18n.T("Add Ember credits")
		main = append(main, section(th, 0, th.Sp.S3, full(th, &p.add, protocol.AccountManageAt(st.Manage, protocol.AccountPagePlans) != "")))
	}
	main = append(main,
		section(th, 0, th.Sp.S4, look.Paragraph(th, th.T.Caption, th.R.Fg2, i18n.T("Ryoku Ember runs hosted AI models. Each request spends Ember credits."))),
		rule(th), p.manageRow(th, page),
	)
	if page == "" {
		main = append(main, section(th, th.Sp.S2, 0, look.Paragraph(th, th.T.Caption, th.R.Fg2, i18n.T("The server names no page for managing the account."))))
	}
	// keys are for other apps: a quiet act after the ones this popover is for
	p.emberKey.Kind, p.emberKey.Small, p.emberKey.Icon, p.emberKey.Label = kit.PlaqueQuiet, true, look.Icon.OpenInNew, i18n.T("Get an Ember API key")
	keys := protocol.AccountManageAt(st.Manage, protocol.AccountPageKeys) != ""
	main = append(main, section(th, th.Sp.S2, th.Sp.S2, func(gtx layout.Context) layout.Dimensions {
		// the quiet plaque's label, not its edge, stands on the margin
		return layout.Inset{Left: -th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return p.emberKey.Layout(gated(gtx, keys), th.Theme) })
	}))
	// the quiet plaque's label, not its edge, stands on the margin
	action := func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Right: -th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return p.signOut.Layout(gtx, th.Theme) })
	}
	return main, p.serverLine(th, st, action)
}

// emptyBalance is a balance read as nothing to spend: it shows as 0.00
func emptyBalance(st protocol.AccountState) bool {
	return st.Balance != nil && protocol.AccountCredits(st.Balance.Available, st.UnitsPerCredit) == protocol.AccountCredits(0, st.UnitsPerCredit)
}

func (p *accountPanel) identity(th *look.Theme, st protocol.AccountState) layout.Widget {
	name, id := "", ""
	if st.Account != nil {
		name, id = firstText(st.Account.Display, st.Account.ID), st.Account.ID
	}
	// the id under a name that is not the id; a bare id has nothing to say under it
	line := kit.Text(th.Theme, th.T.Caption, th.R.Fg2, i18n.T("Ryoku Account"))
	if id != "" && id != name {
		line = kit.Text(th.Theme, th.T.Mono, th.R.Fg2, id)
	}
	initial := "?"
	if name != "" {
		r, _ := utf8.DecodeRuneInString(name)
		initial = strings.ToUpper(string(r))
	}
	return section(th, th.Sp.S4, th.Sp.S4, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				size := gtx.Dp(36)
				bounds := image.Rect(0, 0, size, size)
				kit.Fill(gtx, th.Theme, bounds, gtx.Dp(th.Cut.Small), th.R.Sel)
				look.Place(gtx, bounds, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, kit.RawText(th.Theme, th.T.Title, th.R.OnSel, initial))
				})
				return layout.Dimensions{Size: bounds.Max}
			}),
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(kit.Text(th.Theme, th.T.Row, th.R.Fg, name)),
					layout.Rigid(line),
				)
			}),
		)
	})
}

// the balance in the numerals of a title: the whole credits strong, the
// fraction dimmer
func (p *accountPanel) amount(th *look.Theme, value string) layout.Widget {
	strong, light := th.T.Mono, th.T.Mono
	strong.Size, strong.Line = 34, 40
	light.Size, light.Line = 34, 40
	strong.Face.Weight = font.Bold
	whole, fraction := value, ""
	if at := strings.IndexByte(value, '.'); at >= 0 {
		whole, fraction = value[:at], value[at+1:]
	}
	return func(gtx layout.Context) layout.Dimensions {
		return look.Named(gtx, value, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{layout.Rigid(kit.RawText(th.Theme, strong, th.R.Fg, whole))}
			if fraction != "" {
				// a mono point has a digit's room on both sides, tucked in so the fraction reads as part of the number
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: -6, Right: -6}.Layout(gtx, kit.RawText(th.Theme, light, th.R.Fg2, "."))
				}), layout.Rigid(kit.RawText(th.Theme, light, th.R.Fg2, fraction)))
			}
			return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, children...)
		})
	}
}

func (p *accountPanel) balance(th *look.Theme, st protocol.AccountState) layout.Widget {
	p.refresh.Kind, p.refresh.Small, p.refresh.Icon, p.refresh.Description, p.refresh.Busy = kit.PlaqueQuiet, true, look.Icon.Retry, i18n.T("Refresh"), st.Busy
	available, held, read := "…", "", time.Time{}
	if st.Balance != nil {
		available = protocol.AccountCredits(st.Balance.Available, st.UnitsPerCredit)
		if st.Balance.Held > 0 {
			held = protocol.AccountCredits(st.Balance.Held, st.UnitsPerCredit)
		}
		read = st.Balance.Read
	}
	return section(th, th.Sp.S4, th.Sp.S4, func(gtx layout.Context) layout.Dimensions {
		rows := []layout.FlexChild{
			layout.Rigid(kit.Text(th.Theme, th.T.Group, th.R.Fg2, strings.ToUpper(i18n.T("Ember credits")))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(p.amount(th, available)),
		}
		if held != "" {
			rows = append(rows, layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg2, i18n.Tf("%s held for a request in progress", held))))
		}
		if !read.IsZero() {
			rows = append(rows, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg3, i18n.Tf("Updated %s", read.Local().Format("15:04")))))
		}
		return layout.Flex{Alignment: layout.Start}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
			}),
			// the icon's glyph, not its plate, stands on the margin
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: -8, Right: -7}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return p.refresh.Layout(gtx, th.Theme) })
			}),
		)
	})
}

// manageRow is the view's one gold act, set as a row: the label and the
// website it goes to
func (p *accountPanel) manageRow(th *look.Theme, page string) layout.Widget {
	on := page != ""
	return func(gtx layout.Context) layout.Dimensions {
		return p.manage.Layout(gated(gtx, on), func(gtx layout.Context) layout.Dimensions {
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp(i18n.T("Manage account")).Add(gtx.Ops)
			if on {
				pointer.CursorPointer.Add(gtx.Ops)
			}
			hover := look.Transition(gtx, &p.manage.hover, on && (p.manage.Hovered() || p.manage.Pressed()), th.Motion.HoverIn, th.Motion.HoverOut)
			size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(48))
			bounds := image.Rectangle{Max: size}
			ink := th.R.GoldInk
			if on {
				look.FillRect(gtx, bounds, palette.WithAlpha(th.R.Gold, .075+.09*hover))
			} else {
				ink = th.R.Fg3
			}
			gtx.Constraints = layout.Exact(size)
			layout.Inset{Left: th.Sp.S4, Right: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{layout.Rigid(kit.RawText(th.Theme, th.T.Row, ink, i18n.T("Manage account")))}
				if on {
					children = append(children,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.E.Layout(gtx, kit.RawText(th.Theme, th.T.Mono, th.R.Fg2, hostOf(page)))
						}),
						layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
						layout.Rigid(look.IconWidget(look.Icon.OpenInNew, 18, ink)),
					)
				}
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
			})
			return layout.Dimensions{Size: size}
		})
	}
}

// the server the account belongs to, named only when it is not Ryoku's own,
// with the one quiet act for the view at its end; nil with neither to show
func (p *accountPanel) serverLine(th *look.Theme, st protocol.AccountState, action layout.Widget) layout.Widget {
	server := serverHost(st.Server)
	if server == "" && action == nil {
		return nil
	}
	return section(th, th.Sp.S2, th.Sp.S2, func(gtx layout.Context) layout.Dimensions {
		// a plaque's height keeps the row the same with or without it
		gtx.Constraints.Min.Y = gtx.Dp(32)
		children := []layout.FlexChild{layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} })}
		if server != "" {
			children = []layout.FlexChild{
				layout.Rigid(kit.RawText(th.Theme, th.T.Caption, th.R.Fg2, i18n.T("Server"))),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Flexed(1, kit.RawText(th.Theme, th.T.Mono, th.R.Fg, server)),
			}
		}
		if action != nil {
			children = append(children, layout.Rigid(action))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

// serverHost is the server's host for the user to check, "" on Ryoku's own,
// which tells the user nothing: the account page's host is the one they meet
func serverHost(raw string) string {
	if server, ok := protocol.AccountServer(raw); ok && server == config.DefaultAccountServer {
		return ""
	}
	return hostOf(raw)
}

func (p *accountPanel) signingIn(gtx layout.Context, th *look.Theme, st protocol.AccountState) []layout.Widget {
	p.copyCode.Kind, p.copyCode.Icon, p.copyCode.Label = kit.PlaqueTonal, look.Icon.Copy, i18n.T("Copy code")
	if st.Code != nil && p.copied == st.Code.UserCode {
		p.copyCode.Icon, p.copyCode.Label = look.Icon.Check, i18n.T("Copied")
	}
	p.openPage.Kind, p.openPage.Icon, p.openPage.Label = kit.PlaqueGold, look.Icon.OpenInNew, i18n.T("Open page")
	p.cancel.Kind, p.cancel.Small, p.cancel.Label = kit.PlaqueQuiet, true, i18n.T("Cancel")
	code, left, fraction := "", time.Duration(0), float32(0)
	if st.Code != nil {
		code = st.Code.UserCode
		// the provider's expiry is wall-clock time, whatever clock the frame runs on
		left = max(0, time.Until(st.Code.ExpiresAt).Truncate(time.Second))
		// a code is known only by what it had left when first seen
		if p.spanFor != code {
			p.spanFor, p.span = code, left
		}
		p.span = max(p.span, left)
		if p.span > 0 {
			fraction = float32(left) / float32(p.span)
		}
		// the countdown ticks only while this view shows
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(time.Second - time.Duration(gtx.Now.Nanosecond()))})
	}
	page := signInPage(st.Code)
	// the code large in the mono face, to read against the browser at a glance
	caps := th.T.Mono
	caps.Size, caps.Line = 20, 28
	shown := kit.RawText(th.Theme, caps, th.R.Fg, strings.Join(strings.Split(code, ""), " "))
	slab := func(gtx layout.Context) layout.Dimensions {
		return kit.OutlinedSlabContent(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			centred := func(w layout.Widget) layout.Widget {
				return func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Center.Layout(gtx, w)
				}
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(centred(func(gtx layout.Context) layout.Dimensions { return look.Named(gtx, code, shown) })),
				layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
				layout.Rigid(countdownBar(th, fraction)),
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(centred(kit.RawText(th.Theme, th.T.Caption, th.R.Fg2, waitingLine(left)))),
			)
		})
	}
	facts := [][2]string{{i18n.T("Sign-in page"), hostOf(page)}}
	if server := serverHost(st.Server); server != "" {
		facts = append(facts, [2]string{i18n.T("Server"), server})
	}
	return []layout.Widget{
		section(th, th.Sp.S4, th.Sp.S2, kit.Text(th.Theme, th.T.Label, th.R.Fg2, i18n.T("Enter this code if asked"))),
		section(th, 0, th.Sp.S3, slab),
		section(th, 0, th.Sp.S3, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(1, full(th, &p.copyCode, st.Code != nil)),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Flexed(1, full(th, &p.openPage, page != "")),
			)
		}),
		section(th, 0, th.Sp.S3, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.End}.Layout(gtx,
				layout.Flexed(1, factTable(th, facts)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Right: -th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return p.cancel.Layout(gtx, th.Theme) })
				}),
			)
		}),
	}
}

func waitingLine(left time.Duration) string {
	if left <= 0 {
		return i18n.T("Waiting for the code to expire")
	}
	return i18n.Tf("Expires in %d:%02d", int(left.Minutes()), int(left.Seconds())%60)
}

// countdownBar is the time a code has left, shrinking to its expiry
func countdownBar(th *look.Theme, fraction float32) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Constraints.Max.X, max(2, gtx.Dp(3)))
		look.FillRect(gtx, image.Rectangle{Max: size}, th.R.Vein)
		look.FillRect(gtx, image.Rectangle{Max: image.Pt(int(float32(size.X)*min(max(fraction, 0), 1)), size.Y)}, th.R.Stone(palette.Becoming))
		return layout.Dimensions{Size: size}
	}
}

// factTable sets each label and its value in mono beside it, the values on one line
func factTable(th *look.Theme, rows [][2]string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		measure := gtx
		measure.Constraints.Min = image.Point{}
		widest := 0
		for _, row := range rows {
			record := op.Record(gtx.Ops)
			dimensions := kit.RawText(th.Theme, th.T.Caption, th.R.Fg2, row[0])(measure)
			record.Stop()
			widest = max(widest, dimensions.Size.X)
		}
		children := make([]layout.FlexChild, 0, 2*len(rows))
		for index, row := range rows {
			if index > 0 {
				children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout))
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = widest
						return kit.RawText(th.Theme, th.T.Caption, th.R.Fg2, row[0])(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
					layout.Flexed(1, kit.RawText(th.Theme, th.T.Mono, th.R.Fg, row[1])),
				)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}

// security names the official sources: a modified client can expose the
// account session and spend Ember credits
func (p *accountPanel) security(th *look.Theme) layout.Widget {
	return section(th, th.Sp.S3, th.Sp.S3, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 1}.Layout(gtx, look.IconWidget(look.Icon.Warning, 16, th.R.Amber))
			}),
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg2, i18n.T("Only install Ryoku and Hyakki from ryoku.dev or the ryoku-dev organization on GitHub. A modified copy or fork can access your Ryoku Account and spend your Ember credits."))),
					layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(officialLink(th, &p.site, "ryoku.dev")),
							layout.Rigid(layout.Spacer{Width: th.Sp.S4}.Layout),
							layout.Rigid(officialLink(th, &p.org, "github.com/ryoku-dev")),
						)
					}),
				)
			}),
		)
	})
}

// officialLink is an address that opens in the browser: underlined, and
// brighter under the pointer
func officialLink(th *look.Theme, link *rowClick, name string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return link.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp(name).Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			hover := look.Transition(gtx, &link.hover, link.Hovered() || link.Pressed(), th.Motion.HoverIn, th.Motion.HoverOut)
			ink := palette.Mix(th.R.Fg, th.R.GoldInk, hover)
			return layout.Inset{Top: th.Sp.S1, Bottom: th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				dimensions := kit.RawText(th.Theme, th.T.Group, ink, name)(gtx)
				// a baseline counts from the bottom of the text
				under := dimensions.Size.Y - dimensions.Baseline + gtx.Dp(3)
				line := image.Rect(0, under, dimensions.Size.X, under+max(1, gtx.Dp(1)))
				look.FillRect(gtx, line, palette.WithAlpha(ink, .45+.35*hover))
				return dimensions
			})
		})
	}
}

// wideAct is a gold or tonal plaque that takes the whole width it is given;
// kit.Plaque keeps the width of its label
type wideAct struct {
	kit.Click
	Kind  kit.PlaqueKind
	Icon  *widget.Icon
	Label string
	Busy  bool

	hover, press anim.Tween
}

// Clicked reports one activation unless the act is busy.
func (a *wideAct) Clicked(gtx layout.Context) bool {
	if !a.Busy {
		return a.Click.Clicked(gtx)
	}
	for a.Click.Clicked(gtx) {
	}
	return false
}

func (a *wideAct) Layout(gtx layout.Context, th *look.Theme) layout.Dimensions {
	original := gtx
	interaction := gtx
	if a.Busy {
		interaction = interaction.Disabled()
	}
	return a.Click.Layout(interaction, func(gtx layout.Context) layout.Dimensions {
		gtx.Source = original.Source
		enabled := original.Source.Enabled() && !a.Busy
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(a.Label).Add(gtx.Ops)
		semantic.EnabledOp(enabled).Add(gtx.Ops)
		if enabled {
			pointer.CursorPointer.Add(gtx.Ops)
		}
		pressed := look.Transition(original, &a.press, a.Pressed() && !a.Busy, th.Motion.Press, th.Motion.Press)
		hover := look.Transition(original, &a.hover, a.Hovered() && !a.Busy, th.Motion.HoverIn, th.Motion.HoverOut)
		fill, ink := th.R.Gold, th.R.OnGold
		if a.Kind == kit.PlaqueTonal {
			fill, ink = palette.Mix(th.R.Field, th.R.Fg, tokens.AlphaHoverPlane*hover), th.R.Fg
		} else {
			fill = palette.Mix(fill, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}, tokens.AlphaHoverFill*hover)
		}
		if !original.Source.Enabled() {
			fill, ink = palette.WithAlpha(fill, .35), th.R.Fg3
		}
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(tokens.SizePlaqueHeight)))
		cut := gtx.Dp(th.Cut.Mid) + int(float32(gtx.Dp(unit.Dp(tokens.SizeControlPressCutGrow)))*pressed+.5)
		sink := int(float32(gtx.Dp(unit.Dp(tokens.SizeControlPressSink)))*pressed + .5)
		kit.Fill(gtx, th.Theme, image.Rectangle{Max: size}, cut, fill)

		content := gtx
		content.Constraints.Min = image.Point{}
		record := op.Record(gtx.Ops)
		var lead layout.Widget
		switch {
		case a.Busy:
			lead = func(gtx layout.Context) layout.Dimensions {
				return kit.Working{Active: true, Size: 16}.Layout(gtx, th.Theme)
			}
		case a.Icon != nil:
			lead = look.IconWidget(a.Icon, 18, ink)
		}
		children := []layout.FlexChild{layout.Rigid(kit.RawText(th.Theme, th.T.Label, ink, a.Label))}
		if lead != nil {
			children = []layout.FlexChild{layout.Rigid(lead), layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout), children[0]}
		}
		dimensions := layout.Flex{Alignment: layout.Middle}.Layout(content, children...)
		call := record.Stop()
		offset := op.Offset(image.Pt((size.X-dimensions.Size.X)/2, (size.Y-dimensions.Size.Y)/2+sink)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		return layout.Dimensions{Size: size}
	})
}
