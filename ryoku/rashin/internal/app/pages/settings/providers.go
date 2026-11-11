package settings

import (
	"sort"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type providerRow struct {
	fold foldRow

	enabled kit.Toggle
	key     kit.TextField
	add     kit.Plaque
	test    kit.Plaque
	signup  kit.Plaque
	remove  map[string]*look.IconAction

	testing bool
	result  string
	failed  bool
}

func (p *Page) providerRow(id string) *providerRow {
	row := p.providers[id]
	if row == nil {
		row = &providerRow{remove: make(map[string]*look.IconAction)}
		p.providers[id] = row
	}
	return row
}

func (p *Page) layoutProviders(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	p.usage.load(e)
	providers := p.standingOrder(e.Snapshot().Providers)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(kit.Text(th.Theme, th.T.Section, th.R.Fg, i18n.T("Providers"))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2,
			i18n.T("Ask sends each question to a provider that is up. Your keys are stored encrypted on this machine."))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if len(providers) == 0 {
				return look.SettingsSlab(gtx, th, []look.Setting{{
					Title: i18n.T("No providers yet"), Detail: i18n.T("The list arrives when Rashin has read its catalogue."),
				}})
			}
			return foldSlab(gtx, th, len(providers), func(gtx layout.Context, index int) layout.Dimensions {
				return p.layoutProvider(gtx, e, providers[index])
			})
		}),
	)
}

func (p *Page) layoutProvider(gtx layout.Context, e *env.Env, provider protocol.ProviderInfo) layout.Dimensions {
	th := e.Theme
	row := p.providerRow(provider.ID)
	if row.fold.toggled(gtx) {
		if p.open == provider.ID {
			p.open = ""
		} else {
			p.open = provider.ID
		}
	}
	word, tone := providerState(provider)
	detail := providerMeta(provider)
	if used := p.usage.words(provider.ID); used != "" {
		detail += ", " + used
	}
	head := foldHead{Title: provider.Name, Detail: detail, Trail: kit.Claim(th.Theme, tone, word)}
	return row.fold.layout(gtx, th, p.open == provider.ID, head, func(gtx layout.Context) layout.Dimensions {
		return row.layoutDetail(gtx, e, provider)
	})
}

func (row *providerRow) layoutDetail(gtx layout.Context, e *env.Env, provider protocol.ProviderInfo) layout.Dimensions {
	th := e.Theme
	row.update(gtx, e, provider)
	children := make([]layout.FlexChild, 0, 12)
	if provider.Note != "" {
		children = append(children,
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, provider.Note)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		)
	}
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, kit.Text(th.Theme, th.T.Body, th.R.Fg, i18n.T("Use this provider"))),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, i18n.Tf("Use %s", provider.Name), &row.enabled, func(on bool) {
					call(e, protocol.MProvidersEnable, protocol.EnableProviderParams{Provider: provider.ID, Enabled: on})
				})
			}),
		)
	}))
	for _, key := range provider.Keys {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return row.layoutKey(gtx, e, provider, key) }),
		)
	}
	if !provider.Keyless {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return row.key.Layout(gtx, th.Theme) }),
					layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return row.add.Layout(gtx, th.Theme) }),
				)
			}),
		)
	}
	children = append(children,
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return row.test.Layout(gtx, th.Theme) }),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if provider.Signup == "" {
						return layout.Dimensions{}
					}
					return row.signup.Layout(gtx, th.Theme)
				}),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Flexed(1, row.resultLine(th)),
			)
		}),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (row *providerRow) resultLine(th *look.Theme) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		switch {
		case row.testing:
			return kit.Working{Active: true, Size: 16}.Layout(gtx, th.Theme)
		case row.result == "":
			return layout.Dimensions{}
		case row.failed:
			return kit.RawText(th.Theme, th.T.Caption, th.R.Jasper, row.result)(gtx)
		default:
			return kit.RawText(th.Theme, th.T.Caption, th.R.Fg2, row.result)(gtx)
		}
	}
}

func (row *providerRow) layoutKey(gtx layout.Context, e *env.Env, provider protocol.ProviderInfo, key protocol.KeyInfo) layout.Dimensions {
	th := e.Theme
	remove := row.remove[key.ID]
	if remove == nil {
		remove = &look.IconAction{Icon: look.Icon.Remove, Description: i18n.T("Remove key")}
		row.remove[key.ID] = remove
	}
	if remove.Clicked(gtx) {
		call(e, protocol.MProvidersRemoveKey, protocol.RemoveKeyParams{Provider: provider.ID, Key: key.ID})
	}
	word, ink := i18n.T("Not checked yet"), th.R.Fg3
	switch {
	case key.LastError != "":
		word, ink = key.LastError, th.R.Jasper
	case key.Status == protocol.KeyError:
		word, ink = i18n.T("Not accepted"), th.R.Jasper
	case key.Status == protocol.KeyHealthy:
		word = i18n.T("Working")
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(look.TokenChip(th, key.Masked)),
		layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
		layout.Flexed(1, kit.RawText(th.Theme, th.T.Caption, ink, word)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return remove.Layout(gtx, th, th.R.Fg3) }),
	)
}

func (row *providerRow) update(gtx layout.Context, e *env.Env, provider protocol.ProviderInfo) {
	row.enabled.Value = provider.Enabled
	row.key.Label, row.key.Placeholder, row.key.Secret, row.key.Mono = i18n.T("Add a key"), provider.KeyHint, true, true
	row.key.Editor.SingleLine, row.key.Editor.Submit = true, true
	row.add.Kind, row.add.Label = kit.PlaqueTonal, i18n.T("Add key")
	row.test.Kind, row.test.Small, row.test.Label = kit.PlaqueTonal, true, i18n.T("Test")
	row.signup.Kind, row.signup.Small, row.signup.Label, row.signup.Icon = kit.PlaqueQuiet, true, i18n.T("Get a key"), look.Icon.OpenInNew
	submitted := false
	for {
		event, ok := row.key.Editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := event.(widget.SubmitEvent); ok {
			submitted = true
		}
	}
	if key := strings.TrimSpace(row.key.Text()); (row.add.Clicked(gtx) || submitted) && key != "" {
		e.Call(protocol.MProvidersAddKey, protocol.AddKeyParams{Provider: provider.ID, Key: key}, new(protocol.ProviderInfo), func(err error) {
			if err != nil {
				row.key.Error = err.Error()
				return
			}
			row.key.Error = ""
			row.key.SetText("")
		})
	}
	if row.test.Clicked(gtx) && !row.testing {
		row.testing, row.result, row.failed = true, "", false
		var result protocol.ProviderTestResult
		e.Call(protocol.MProvidersTest, protocol.ProviderParams{Provider: provider.ID}, &result, func(err error) {
			row.testing = false
			switch {
			case err != nil:
				row.result, row.failed = err.Error(), true
			case result.Message != "":
				row.result, row.failed = result.Message, !result.OK
			case result.OK:
				row.result = i18n.T("It answered.")
			default:
				row.result, row.failed = i18n.T("It did not answer."), true
			}
		})
	}
	if row.signup.Clicked(gtx) && provider.Signup != "" {
		e.Open(provider.Signup)
	}
}

func call(e *env.Env, method string, params any) {
	e.Call(method, params, new(protocol.ProviderInfo), func(err error) {
		if err != nil {
			e.Notify(err.Error())
		}
	})
}

// held from when Settings opens: sorted live, a row leaves from under the pointer when its switch flips
func (p *Page) standingOrder(providers []protocol.ProviderInfo) []protocol.ProviderInfo {
	known := make(map[string]protocol.ProviderInfo, len(providers))
	for _, provider := range providers {
		known[provider.ID] = provider
	}
	held := len(p.providerOrder) == len(providers)
	for _, id := range p.providerOrder {
		if _, ok := known[id]; !ok {
			held = false
		}
	}
	if !held {
		p.providerOrder = p.providerOrder[:0]
		for _, provider := range orderedProviders(providers) {
			p.providerOrder = append(p.providerOrder, provider.ID)
		}
	}
	ordered := make([]protocol.ProviderInfo, len(p.providerOrder))
	for index, id := range p.providerOrder {
		ordered[index] = known[id]
	}
	return ordered
}

func orderedProviders(providers []protocol.ProviderInfo) []protocol.ProviderInfo {
	ordered := append([]protocol.ProviderInfo(nil), providers...)
	sort.SliceStable(ordered, func(left, right int) bool {
		leftPriority, rightPriority := providerPriority(ordered[left]), providerPriority(ordered[right])
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		leftName, rightName := strings.ToLower(ordered[left].Name), strings.ToLower(ordered[right].Name)
		if leftName != rightName {
			return leftName < rightName
		}
		return ordered[left].ID < ordered[right].ID
	})
	return ordered
}

func providerPriority(provider protocol.ProviderInfo) int {
	switch {
	case provider.Enabled && provider.Available > 0 && !provider.Keyless:
		return 0
	case provider.Enabled && provider.Available > 0:
		return 1
	default:
		return 2
	}
}

func providerState(provider protocol.ProviderInfo) (string, palette.Tone) {
	switch {
	case !provider.Enabled:
		return i18n.T("Off"), palette.Idle
	case provider.Available > 0:
		return i18n.T("Ready"), palette.Alive
	case provider.CooldownUntil != nil && provider.CooldownUntil.After(time.Now()):
		return i18n.T("Resting"), palette.Paused
	case !provider.Configured:
		// a provider nobody set up is not a problem to solve, so it does not take the colour of one
		return i18n.T("Needs a key"), palette.Idle
	default:
		return i18n.T("Not answering"), palette.Failed
	}
}

func providerMeta(provider protocol.ProviderInfo) string {
	models := i18n.Tf("%d models", provider.Models)
	if provider.Models == 1 {
		models = i18n.T("1 model")
	}
	switch {
	case provider.Keyless:
		return i18n.T("Needs no key") + ", " + models
	case len(provider.Keys) == 0:
		return i18n.T("No key yet") + ", " + models
	case len(provider.Keys) == 1:
		return i18n.T("1 key") + ", " + models
	default:
		return i18n.Tf("%d keys", len(provider.Keys)) + ", " + models
	}
}
