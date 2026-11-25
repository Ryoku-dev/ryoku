package settings

import (
	"slices"
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

// a provider's answers are counted over the last week, the same week the chart draws
const usageDays = 7

type providerRow struct {
	fold foldRow

	enabled kit.Toggle
	key     kit.TextField
	add     kit.Plaque
	test    kit.Plaque
	signup  kit.Plaque
	remove  map[string]*kit.IconPlaque

	// a local provider's address, edited in place until saved
	address     kit.TextField
	addressFrom string
	save        kit.Plaque

	testing bool
	result  string
	failed  bool

	models modelList
}

func (p *Page) providerRow(id string) *providerRow {
	row := p.providers[id]
	if row == nil {
		row = &providerRow{remove: make(map[string]*kit.IconPlaque)}
		p.providers[id] = row
	}
	return row
}

// providerBlocks keeps the tall local manager measurable by the virtual Settings list
func (p *Page) providerBlocks(gtx layout.Context, e *env.Env, blocks []layout.Widget) []layout.Widget {
	th := e.Theme
	p.usage.Load(e, usageDays, gtx.Now)
	p.limits.Show(e, gtx.Now)
	local, online := splitProviders(e.Snapshot().Providers)
	// only providers that are set up earn a row; the rest wait behind Add a provider
	shown, hidden := p.activeProviders(online)
	shown = p.standingOrder(shown)
	blocks = append(blocks, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return sectionHead(gtx, th, sectionProviders) }),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2,
				i18n.T("Ask sends each question to a provider that is up. Your keys are stored encrypted on this machine."))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				trend, loaded := p.usage.Trend()
				if loaded && len(trend.Values) == 0 {
					return layout.Dimensions{}
				}
				// the section already carries the heading, the proof names what the line counts
				trend.Title, trend.Height = "", 72
				return layout.Inset{Bottom: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return look.HoldUntil(gtx, loaded, func(gtx layout.Context) layout.Dimensions { return trend.Layout(gtx, th) })
				})
			}),
		)
	})
	if len(local) > 0 {
		blocks = append(blocks, p.providerGroup(e, i18n.T("On this computer"),
			i18n.T("Models on this computer answer first. Hyakki starts its own when a question needs one and stops it when idle."), local))
	}
	blocks = append(blocks, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			settings := e.Snapshot().Settings.Providers.Models
			p.local.runtime.remote.Value = settings.RemoteFirst
			return look.SettingsSlab(gtx, th, []look.Setting{{
				Title:  i18n.T("Use online models before ones on this computer"),
				Detail: i18n.T("When on, Ask tries an online model first. Your explicit model choice still wins."),
				Control: func(gtx layout.Context) layout.Dimensions {
					return namedSwitch(gtx, th, i18n.T("Use online models before ones on this computer"), &p.local.runtime.remote, func(on bool) {
						setLocalOption(e, "remoteFirst", on, e.Report)
					})
				},
			}})
		})
	}, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			// signed out, Ryoku Ember waits behind Add a provider as the way to the account
			signIn := e.OpenAccount != nil && env.SignInToEmber(e.Snapshot())
			if len(shown) == 0 && len(hidden) == 0 && !signIn {
				return look.SettingsSlab(gtx, th, []look.Setting{{
					Title: i18n.T("No providers yet"), Detail: i18n.T("The list arrives when Hyakki has read its catalogue."),
				}})
			}
			children := []layout.FlexChild{}
			if len(shown) > 0 {
				children = append(children, layout.Rigid(p.providerGroup(e, i18n.T("Online"),
					i18n.T("Models on this computer answer first by default. Auto, Fast and Smart use free online models only; paid models answer only when picked."), shown)))
			}
			if len(hidden) > 0 || signIn {
				if len(shown) > 0 {
					children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout))
				}
				children = append(children, layout.Rigid(p.addProviderRow(e, hidden, signIn)))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
	return p.fastDecisions.blocks(e, blocks)
}

func (p *Page) providerGroup(e *env.Env, title, caption string, providers []protocol.ProviderInfo) layout.Widget {
	th := e.Theme
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(groupHead(th, title, caption)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return foldSlab(gtx, th, len(providers), func(gtx layout.Context, index int) layout.Dimensions {
					return p.layoutProvider(gtx, e, providers[index])
				})
			}),
		)
	}
}

// a provider earns a row once it is set up: a key, no key needed, or picked
// from Add a provider; the rest wait behind it
func (p *Page) activeProviders(providers []protocol.ProviderInfo) (shown, hidden []protocol.ProviderInfo) {
	for _, provider := range providers {
		if provider.Configured || p.revealed[provider.ID] {
			shown = append(shown, provider)
		} else {
			hidden = append(hidden, provider)
		}
	}
	return shown, hidden
}

// the one control that brings a provider nobody set up into the list: pick
// it, and its row opens with the key field ready. With signIn, Ryoku Ember
// leads it as the row that opens the account popover.
func (p *Page) addProviderRow(e *env.Env, hidden []protocol.ProviderInfo, signIn bool) layout.Widget {
	th := e.Theme
	return func(gtx layout.Context) layout.Dimensions {
		options := make([]look.SelectOption, 0, len(hidden)+1)
		if signIn {
			options = append(options, env.EmberSignInRow(e.OpenAccount))
		}
		lead := len(options)
		for _, provider := range hidden {
			options = append(options, look.SelectOption{
				Label: provider.Name, Detail: providerMeta(provider, ""),
				Mark: look.Mark{ID: provider.ID, Name: provider.Name},
			})
		}
		p.addProvider.Label, p.addProvider.Searched = i18n.T("Add a provider"), true
		gtx.Constraints.Min.X = gtx.Dp(selectWidth)
		dimensions := p.addProvider.LayoutOptions(gtx, th, e.Overlays, options, -1)
		if index, ok := p.addProvider.Changed(); ok && index >= lead && index-lead < len(hidden) {
			if p.revealed == nil {
				p.revealed = make(map[string]bool)
			}
			p.revealed[hidden[index-lead].ID] = true
			p.open = hidden[index-lead].ID
		}
		return dimensions
	}
}

// the list retains the routing order even if the daemon returns providers in a different order
func splitProviders(providers []protocol.ProviderInfo) (local, online []protocol.ProviderInfo) {
	for _, id := range protocol.LocalProviders {
		for _, provider := range providers {
			if provider.Local && provider.ID == id {
				local = append(local, provider)
			}
		}
	}
	for _, provider := range providers {
		if !provider.Local {
			online = append(online, provider)
		} else if !slices.Contains(protocol.LocalProviders, provider.ID) {
			local = append(local, provider)
		}
	}
	return local, online
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
	detail := providerMeta(provider, p.usage.Words(provider.ID))
	head := foldHead{Title: provider.Name, Detail: detail, Mark: look.Mark{ID: provider.ID, Name: provider.Name}}
	if provider.ID == protocol.ProviderRashinModels {
		word, tone = p.local.standing(e.Snapshot().Local, provider)
	}
	head.Trail = kit.Claim(th.Theme, tone, word)
	if provider.ID == protocol.ProviderRashinModels && p.open == provider.ID {
		p.local.load(e)
		p.local.loadDevices(e)
		// the one place the daemon is asked to look for model files: the manager, while it is open
		p.local.found.load(e)
	}
	return withLimitLine(gtx, th, provider, p.open == provider.ID, func(gtx layout.Context) layout.Dimensions {
		return row.fold.layout(gtx, th, p.open == provider.ID, head, func(gtx layout.Context) layout.Dimensions {
			return row.layoutDetail(gtx, e, provider, &p.local)
		})
	})
}

func (row *providerRow) layoutDetail(gtx layout.Context, e *env.Env, provider protocol.ProviderInfo, local *localPanel) layout.Dimensions {
	th := e.Theme
	row.update(gtx, e, provider)
	children := make([]layout.FlexChild, 0, 12)
	// a local provider's note is the how-to its head already shows
	if provider.Note != "" && !provider.Local {
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
	if provider.ID == protocol.ProviderRashinModels {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return local.layout(gtx, e)
		}))
	} else if provider.Local {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(kit.Text(th.Theme, th.T.Detail, th.R.Fg2, i18n.T("Address"))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					// the label is drawn above, so Named gives the field its accessible name
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return look.Named(gtx, i18n.Tf("%s address", provider.Name), func(gtx layout.Context) layout.Dimensions { return row.address.Layout(gtx, th.Theme) })
					}),
					layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
					layout.Rigid(groupSave(th, &row.save, row.addressChanged(provider))),
				)
			}),
		)
	} else if !provider.Keyless {
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
	children = append(children, limitsDetail(th, provider)...)
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
	if provider.ID != protocol.ProviderRashinModels {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S5}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return row.models.layout(gtx, e, provider) }),
		)
	}
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
		remove = &kit.IconPlaque{Icon: look.Icon.Remove, Description: i18n.T("Remove key"), Kind: kit.PlaqueQuiet, Small: true}
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
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return remove.Layout(gtx, th.Theme) }),
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
	if provider.Local && provider.ID != protocol.ProviderRashinModels {
		row.signup.Label = i18n.Tf("Get %s", provider.Name)
		row.updateAddress(gtx, e, provider)
	}
	if row.signup.Clicked(gtx) && provider.Signup != "" {
		e.Open(provider.Signup)
	}
}

func (row *providerRow) updateAddress(gtx layout.Context, e *env.Env, provider protocol.ProviderInfo) {
	// emptied, the field shows the address saving it goes back to
	row.address.Placeholder, row.address.Mono = protocol.DefaultLocalAddresses()[provider.ID], true
	row.address.Editor.SingleLine, row.address.Editor.Submit = true, true
	// what the daemon says replaces the field only when it changed there, never what is being typed
	if provider.Address != row.addressFrom {
		row.address.SetText(provider.Address)
		row.addressFrom = provider.Address
	}
	submitted := false
	for {
		event, ok := row.address.Editor.Update(gtx)
		if !ok {
			break
		}
		switch event.(type) {
		case widget.SubmitEvent:
			submitted = true
		case widget.ChangeEvent:
			// a refusal is about the text it was given, not what is typed since
			row.address.Error = ""
		}
	}
	if (!row.save.Clicked(gtx) && !submitted) || !row.addressChanged(provider) || row.save.Busy {
		return
	}
	// null drops the saved address, which leaves the provider's usual one
	var address any
	if typed := strings.TrimSpace(row.address.Text()); typed != "" {
		address = typed
	}
	row.save.Busy = true
	e.SetSettings(map[string]any{"providers": map[string]any{"local": map[string]any{provider.ID: address}}}, func(err error) {
		row.save.Busy = false
		if err != nil {
			row.address.Error = err.Error()
			return
		}
		row.address.Error = ""
		// the next frame shows the address the daemon kept, the usual one when the field was emptied
		row.addressFrom = ""
	})
}

func (row *providerRow) addressChanged(provider protocol.ProviderInfo) bool {
	return strings.TrimSpace(row.address.Text()) != provider.Address
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
	case provider.OnDemand && provider.Available > 0 && !provider.Running:
		return i18n.T("Starts when needed"), palette.Alive
	case provider.Local && !provider.Running:
		// a server that is not started is the reader's choice, not a fault
		return i18n.T("Not running"), palette.Idle
	case provider.Available > 0:
		return i18n.T("Ready"), palette.Alive
	case provider.CooldownUntil != nil && provider.CooldownUntil.After(time.Now()):
		return i18n.T("Resting"), palette.Paused
	case provider.Local && provider.Models == 0:
		return i18n.T("No model yet"), palette.Idle
	case !provider.Configured:
		// a provider nobody set up is not a problem to solve, so it does not take the colour of one
		return i18n.T("Needs a key"), palette.Idle
	default:
		return i18n.T("Not answering"), palette.Failed
	}
}

func providerMeta(provider protocol.ProviderInfo, used string) string {
	models := modelCount(provider)
	switch {
	case provider.OnDemand && provider.Running:
		return withUsage(i18n.T("Running")+", "+models, used)
	case provider.OnDemand:
		return withUsage(i18n.T("Starts when needed")+", "+models, used)
	case provider.Local && provider.Running:
		return withUsage(i18n.T("Running")+", "+models, used)
	case provider.Local:
		// the one line that gets it running is a whole sentence, so nothing is added to it
		return provider.Note
	case provider.Keyless:
		return withUsage(i18n.T("Needs no key")+", "+models, used)
	}
	paid := provider.Tier == protocol.TierPaid
	keys := i18n.Tf("%d keys", len(provider.Keys))
	switch {
	case len(provider.Keys) == 0 && paid:
		return withUsage(i18n.T("Paid, no key yet")+", "+models, used)
	case len(provider.Keys) == 0:
		keys = i18n.T("No key yet")
	case len(provider.Keys) == 1:
		keys = i18n.T("1 key")
	}
	if paid {
		return withUsage(i18n.T("Paid")+", "+keys+", "+models, used)
	}
	return withUsage(keys+", "+models, used)
}

// how many models a provider has and how many cost nothing: 139 models, all free
func modelCount(provider protocol.ProviderInfo) string {
	switch {
	case provider.Models == 1 && provider.Free == 1 && !provider.Local:
		return i18n.T("1 model, free")
	case provider.Models == 1:
		return i18n.T("1 model")
	case provider.Local || provider.Free == 0:
		return i18n.Tf("%s models", look.Grouped(provider.Models))
	case provider.Free == provider.Models:
		return i18n.Tf("%s models, all free", look.Grouped(provider.Models))
	}
	return i18n.Tf("%s models, %s free", look.Grouped(provider.Models), look.Grouped(provider.Free))
}

func withUsage(meta, used string) string {
	if used == "" {
		return meta
	}
	return meta + ", " + used
}
