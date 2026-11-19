package ask

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

// what the plaque beside the claim does
const (
	actProviders = iota
	actStart
)

// what will answer the next question: a claim, one line about it and the way to change it
type answering struct {
	word   string
	tone   palette.Tone
	detail string
	label  string
	act    int
	// the plaque is the next thing to do, so it stands out from the line
	leads bool
	// the local model a start loads
	model string
}

// said under a new ask before the question is written
type readiness struct {
	act  kit.Plaque
	swap look.Swap
	// two states are held while one leaves and the next arrives
	now, was answering
	starting bool
}

func (r *readiness) layout(gtx layout.Context, e *env.Env, model string) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	state := whatAnswers(snapshot.Local, snapshot.Providers, snapshot.Settings.Providers.Models.RemoteFirst, model, env.ModelWords(e, model))
	if state.key() != r.now.key() {
		r.was, r.now = r.now, state
	}
	r.act.Kind, r.act.Small, r.act.Label = kit.PlaqueQuiet, true, state.label
	if state.leads {
		r.act.Kind = kit.PlaqueTonal
	}
	r.act.Busy = r.starting && state.act == actStart
	if r.act.Clicked(gtx) {
		switch state.act {
		case actStart:
			r.starting = true
			e.LocalStart(state.model, func(err error) {
				r.starting = false
				e.Report(err)
			})
		default:
			if e.OpenSettings != nil {
				e.OpenSettings("providers")
			}
		}
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return r.swap.Layout(gtx, th, state.key(), th.Sp.S1, th.Motion.Quick, func(gtx layout.Context, key string) layout.Dimensions {
				shown := r.now
				if key != r.now.key() {
					shown = r.was
				}
				return shown.layout(gtx, th)
			})
		}),
		layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return r.act.Layout(gtx, th.Theme) }),
	)
}

func (a answering) key() string { return a.word + "\x1f" + a.detail }

func (a answering) layout(gtx layout.Context, th *look.Theme) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(kit.Claim(th.Theme, a.tone, a.word)),
		layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
		layout.Flexed(1, look.Paragraph(th, th.T.Detail, th.R.Fg2, a.detail)),
	)
}

// local comes first, as the routes do: Rashin's own model, then a server on this computer, then the free providers
func whatAnswers(local protocol.LocalResult, providers []protocol.ProviderInfo, remoteFirst bool, model, name string) answering {
	if pinned(model) {
		return pinnedAnswers(local, providers, model, name)
	}
	server, served, remote, models := "", 0, 0, 0
	for _, provider := range providers {
		if !provider.Enabled || provider.Available == 0 || provider.ID == protocol.ProviderRashinModels {
			continue
		}
		switch {
		case !provider.Local:
			remote++
			models += provider.Available
		case provider.Running && server == "":
			server, served = provider.Name, provider.Available
		}
	}
	away := answering{word: i18n.T("Ready"), tone: palette.Alive, detail: providersWords(remote, models), label: i18n.T("Providers")}
	if remoteFirst && remote > 0 {
		return away
	}
	stand, own := env.LocalStand(local)
	switch stand {
	case env.LocalReady:
		return answering{word: i18n.T("Ready on this computer"), tone: palette.Alive, detail: modelFacts(own), label: i18n.T("Models")}
	case env.LocalStarting:
		return answering{word: i18n.T("Starting"), tone: palette.Becoming, detail: i18n.Tf("%s is loading on this computer.", own.Name), label: i18n.T("Models")}
	case env.LocalStopping:
		return answering{word: i18n.T("Stopping"), tone: palette.Becoming, detail: i18n.Tf("%s is leaving this computer's memory.", own.Name), label: i18n.T("Models")}
	case env.LocalResting:
		return answering{
			word: i18n.T("On this computer"), tone: palette.Idle, detail: i18n.Tf("%s starts with your question.", own.Name),
			label: i18n.T("Start now"), act: actStart, leads: true, model: own.ID,
		}
	}
	if server != "" {
		detail := i18n.Tf("%s, with %d models.", server, served)
		if served == 1 {
			detail = i18n.Tf("%s, with one model.", server)
		}
		return answering{word: i18n.T("Ready on this computer"), tone: palette.Alive, detail: detail, label: i18n.T("Models")}
	}
	switch {
	case stand == env.LocalFailed && remote > 0:
		return answering{
			word: i18n.T("The local model failed"), tone: palette.Failed, detail: i18n.T("Free providers answer meanwhile."),
			label: i18n.T("Open providers"), leads: true,
		}
	case stand == env.LocalFailed:
		return answering{
			word: i18n.T("The local model failed"), tone: palette.Failed, detail: i18n.T("Providers shows why and how to start it again."),
			label: i18n.T("Open providers"), leads: true,
		}
	case remote == 0:
		return answering{
			word: i18n.T("Nothing can answer"), tone: palette.Attention, detail: i18n.T("Get a local model, or add a free key."),
			label: i18n.T("Open providers"), leads: true,
		}
	case stand == env.LocalNoEngine:
		away.detail, away.label = i18n.Tf("Free providers answer for now. %s needs llama.cpp to run here.", own.Name), i18n.T("Set it up")
	default:
		away.detail, away.label = i18n.T("Free providers answer for now. A local model keeps your questions on this computer."), i18n.T("Get a local model")
	}
	return away
}

// a model the reader picked answers alone, so the line is about that model and nothing else
func pinnedAnswers(local protocol.LocalResult, providers []protocol.ProviderInfo, model, name string) answering {
	id, selected, _ := strings.Cut(model, "/")
	unavailable := answering{
		word: i18n.T("Cannot answer now"), tone: palette.Attention, detail: i18n.Tf("%s is not available. Pick another model, or open Providers.", name),
		label: i18n.T("Open providers"), leads: true,
	}
	for _, provider := range providers {
		if provider.ID != id || !provider.Enabled {
			continue
		}
		if id == protocol.ProviderRashinModels {
			for _, own := range local.Models {
				if own.ID != selected || own.State != protocol.LocalInstalled {
					continue
				}
				switch local.Runtime.State {
				case protocol.RuntimeMissing, protocol.RuntimeTooOld, protocol.RuntimeFailed:
					return unavailable
				case protocol.RuntimeStarting:
					return answering{word: i18n.T("Starting"), tone: palette.Becoming, detail: i18n.Tf("%s is loading on this computer.", own.Name), label: i18n.T("Models")}
				case protocol.RuntimeReady:
					for _, loaded := range local.Runtime.Loaded {
						if loaded == own.ID {
							return answering{word: i18n.T("Ready on this computer"), tone: palette.Alive, detail: modelFacts(own), label: i18n.T("Models")}
						}
					}
				}
				return answering{
					word: i18n.T("On this computer"), tone: palette.Idle, detail: i18n.Tf("%s starts with your question.", own.Name),
					label: i18n.T("Start now"), act: actStart, leads: true, model: own.ID,
				}
			}
			return unavailable
		}
		if provider.Local {
			if provider.Running && provider.Available > 0 {
				return answering{word: i18n.T("Ready on this computer"), tone: palette.Alive, detail: name, label: i18n.T("Providers")}
			}
			return unavailable
		}
		if provider.Available > 0 {
			return answering{word: i18n.T("Ready"), tone: palette.Alive, detail: name, label: i18n.T("Providers")}
		}
	}
	return unavailable
}

func modelFacts(model protocol.LocalModel) string {
	if model.Context <= 0 {
		return model.Name
	}
	return i18n.Tf("%s · %s context", model.Name, env.ShortContext(model.Context))
}

func providersWords(ready, models int) string {
	switch {
	case ready == 1 && models == 1:
		return i18n.T("One provider can answer now, with one model.")
	case ready == 1:
		return i18n.Tf("One provider can answer now, with %d models.", models)
	}
	return i18n.Tf("%d providers can answer now, with %d models between them.", ready, models)
}
