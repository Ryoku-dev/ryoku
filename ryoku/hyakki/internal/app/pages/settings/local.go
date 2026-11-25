package settings

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type localPanel struct {
	asked, loading bool
	problem        string
	deviceKey      string
	deviceAsked    bool
	deviceBusy     bool
	devices        []protocol.LocalDevice
	deviceError    string
	refresh        kit.Plaque
	runtime        localRuntimePanel
	models         map[string]*localModelRow
	add            localAddPanel
	confirm        kit.Dialog
	removeConfirm  kit.Confirm
	removing       string
	removingPath   string
	found          localFoundPanel
	accept, cancel kit.Plaque
	licenceLink    kit.Click
	pendingModel   string
	pendingAdd     *protocol.LocalAddParams
	pendingLicense protocol.LocalLicense
	pendingWarning string
}

func (p *localPanel) standing(local protocol.LocalResult, provider protocol.ProviderInfo) (string, palette.Tone) {
	if !provider.Enabled {
		return i18n.T("Off"), palette.Idle
	}
	switch local.Runtime.State {
	case protocol.RuntimeMissing:
		if usableBuilds(local.Runtime) > 0 {
			return i18n.T("Choose llama.cpp"), palette.Attention
		}
		return i18n.T("Get llama.cpp"), palette.Attention
	case protocol.RuntimeFailed:
		return i18n.T("Could not start"), palette.Failed
	case protocol.RuntimeStarting:
		return i18n.T("Starting"), palette.Becoming
	case protocol.RuntimeStopping:
		return i18n.T("Stopping"), palette.Becoming
	}
	for _, model := range local.Models {
		if model.State == protocol.LocalInstalled {
			if local.Runtime.State == protocol.RuntimeReady {
				return i18n.T("Ready"), palette.Alive
			}
			return i18n.T("Starts when needed"), palette.Alive
		}
	}
	return i18n.T("No model yet"), palette.Idle
}

func (p *localPanel) load(e *env.Env) {
	if p.asked || p.loading {
		return
	}
	p.asked, p.loading = true, true
	var result protocol.LocalResult
	e.Call(protocol.MLocalModels, protocol.Empty{}, &result, func(err error) {
		p.loading = false
		if err != nil {
			p.problem = err.Error()
			return
		}
		p.problem = ""
		e.Store.SetLocal(result)
	})
}

func (p *localPanel) loadDevices(e *env.Env) {
	snapshot := e.Snapshot()
	key := snapshot.Settings.Providers.Models.Binary + "\x00" + snapshot.Local.Runtime.Path
	if key != p.deviceKey {
		p.deviceKey, p.deviceAsked, p.deviceBusy, p.devices, p.deviceError = key, false, false, nil, ""
	}
	if p.deviceAsked || p.deviceBusy {
		return
	}
	p.deviceAsked, p.deviceBusy = true, true
	e.LocalDevices(func(devices []protocol.LocalDevice, err error) {
		if key != p.deviceKey {
			return
		}
		p.deviceBusy = false
		if err != nil {
			p.deviceError = err.Error()
			return
		}
		p.devices, p.deviceError = devices, ""
	})
}

func (p *localPanel) layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	local := snapshot.Local
	if p.deviceAsked {
		snapshot.Local.Runtime.Devices = p.devices
	}
	if p.models == nil {
		p.models = make(map[string]*localModelRow)
	}
	p.refresh.Kind, p.refresh.Small, p.refresh.Label, p.refresh.Busy = kit.PlaqueQuiet, true, i18n.T("Refresh models"), p.loading
	if p.refresh.Clicked(gtx) {
		p.asked = false
		p.deviceAsked = false
		p.found.asked = false
		p.loadDevices(e)
		p.load(e)
		p.found.load(e)
	}
	children := []layout.FlexChild{
		layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.T("Hyakki keeps model files here and starts llama.cpp only when a question needs one."))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
		layout.Rigid(look.SectionHead(th, look.Head{Title: i18n.T("Engine"), Number: 1, Of: 4})),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.runtime.layout(gtx, e, snapshot) }),
		layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
		layout.Rigid(look.SectionHead(th, look.Head{Title: i18n.T("Models"), Number: 2, Of: 4})),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
	}
	if p.loading && len(local.Models) == 0 {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.Working{Active: true}.Layout(gtx, th.Theme)
		}))
	}
	for _, model := range local.Models {
		row := p.models[model.ID]
		if row == nil {
			row = &localModelRow{}
			p.models[model.ID] = row
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return row.layout(gtx, e, model, snapshot.LocalProgress[model.ID], snapshot, p)
		}), layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout))
	}
	children = append(children,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.add.layout(gtx, e, p) }),
		layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
		layout.Rigid(look.SectionHead(th, look.Head{Title: i18n.T("Found on this computer"), Number: 3, Of: 4})),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.found.layout(gtx, e, snapshot, p) }),
		layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
		layout.Rigid(look.SectionHead(th, look.Head{Title: i18n.T("Routing"), Number: 4, Of: 4})),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.runtime.routing(gtx, e, snapshot) }),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.refresh.Layout(gtx, th.Theme) }),
	)
	if p.problem != "" {
		children = append(children, layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, p.problem)))
	}
	if p.deviceError != "" {
		children = append(children, layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, p.deviceError)))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (p *localPanel) requestDownload(model protocol.LocalModel) {
	p.pendingModel, p.pendingAdd, p.pendingLicense = model.ID, nil, model.License
	p.pendingWarning = model.Warning
	p.problem = ""
	p.confirm.Open = true
}

func (p *localPanel) requestAdd(params protocol.LocalAddParams, licence protocol.LocalLicense, warning string) {
	p.pendingModel, p.pendingLicense, p.pendingWarning = "", licence, warning
	p.pendingAdd = &params
	p.problem = ""
	p.confirm.Open = true
}

func (p *localPanel) overlay(gtx layout.Context, e *env.Env) bool {
	if p.removeConfirm.Dialog.Open {
		title, body, action := i18n.T("Remove this model?"), i18n.T("Its downloaded files and any partial download are removed. You can download it again later."), i18n.T("Remove model")
		// a file used where it lies is its owner's: Hyakki only forgets it, and nothing about that is dangerous
		p.removeConfirm.Danger = p.removingPath == ""
		if p.removingPath != "" {
			title, action = i18n.T("Remove this model from Hyakki?"), i18n.T("Remove from Hyakki")
			body = i18n.Tf("Hyakki stops using it. The file stays where it is: %s", env.LocalPlace(p.removingPath))
		}
		if p.removeConfirm.Layout(gtx, e.Theme.Theme, title, body, action, "") {
			e.LocalRemove(p.removing, func(err error) {
				if err != nil {
					p.problem = err.Error()
				} else {
					p.problem = ""
				}
			})
		}
		return true
	}
	if !p.confirm.Open {
		return false
	}
	th := e.Theme
	p.accept.Kind, p.accept.Label = kit.PlaqueGold, i18n.T("Download model")
	p.cancel.Kind, p.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	if p.accept.Clicked(gtx) && !p.accept.Busy {
		p.accept.Busy = true
		if p.pendingAdd != nil {
			params := *p.pendingAdd
			e.LocalAdd(params, func(_ protocol.LocalModel, err error) { p.finishDownload(err) })
		} else {
			e.LocalDownload(p.pendingModel, p.finishDownload)
		}
	}
	if p.cancel.Clicked(gtx) {
		p.confirm.Dismiss(gtx)
	}
	if p.licenceLink.Clicked(gtx) && p.pendingLicense.URL != "" {
		e.Open(p.pendingLicense.URL)
	}
	licence := p.pendingLicense.Name
	if licence == "" {
		licence = i18n.T("terms listed on its model card")
	}
	p.confirm.Layout(gtx, th.Theme, i18n.T("Read the model licence"), func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg2, i18n.Tf("This model is offered under %s. Read its terms before downloading it to this computer.", licence))),
		}
		if p.pendingLicense.Note != "" {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, p.pendingLicense.Note)))
		}
		if p.pendingWarning != "" {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Jasper, p.pendingWarning)))
		}
		if p.pendingLicense.URL != "" {
			label := i18n.T("Read the full licence")
			if p.pendingAdd != nil {
				label = i18n.T("Read the model card and licence")
			}
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return look.Link(gtx, th, &p.licenceLink, label)
			}))
		}
		if strings.Contains(strings.ToLower(p.pendingLicense.Name), "lfm") {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.T("Commercial use is allowed only below US$10M annual revenue."))))
		}
		if p.problem != "" {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, p.problem)))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}, func(gtx layout.Context) layout.Dimensions { return p.cancel.Layout(gtx, th.Theme) }, func(gtx layout.Context) layout.Dimensions {
		return p.accept.Layout(gtx, th.Theme)
	})
	if p.confirm.Closed() {
		p.pendingAdd, p.pendingModel = nil, ""
	}
	return p.confirm.Open
}

func (p *localPanel) finishDownload(err error) {
	p.accept.Busy = false
	if err != nil {
		p.problem = err.Error()
		return
	}
	p.problem = ""
	if p.pendingAdd != nil {
		p.add.added()
	}
	p.confirm.Open = false
	p.asked = false
}
