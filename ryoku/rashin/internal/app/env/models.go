package env

import (
	"reflect"
	"slices"
	"sort"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/symbols"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// the models Rashin can run and the rows of every select that picks one: the routes, then the models by where they run and what they cost, asked again when a provider changes
type ModelMenu struct {
	models    protocol.ModelsResult
	providers []protocol.ProviderInfo
	epoch     int
	loading   bool
	loaded    bool
	// asked for these providers already, so a refusal is not asked again every frame
	tried bool
	// moves with every load, so what was built from the list knows it is stale
	revision uint64

	byID  map[string]protocol.ModelInfo
	names map[string]string
	// built once per pin and use, so a list of hundreds is not sorted again every frame
	built map[listKey]*modelList
}

type listKey struct {
	pin    string
	agents bool
}

type modelList struct {
	options []look.SelectOption
	ids     []string
}

// a key added, a server started or a model downloaded changes what can be picked, so the list follows the providers
func (m *ModelMenu) Load(e *Env, snapshot state.Snapshot) {
	if m.loading || (m.tried && m.epoch == snapshot.Epoch && sameOffer(m.providers, snapshot.Providers)) {
		return
	}
	m.loading = true
	var result protocol.ModelsResult
	e.Call(protocol.MModelsList, protocol.ModelsParams{}, &result, func(err error) {
		m.loading, m.tried = false, true
		m.providers, m.epoch = snapshot.Providers, snapshot.Epoch
		if err != nil {
			return
		}
		m.take(result, snapshot.Providers)
	})
}

func (m *ModelMenu) take(models protocol.ModelsResult, providers []protocol.ProviderInfo) {
	m.models, m.providers, m.loaded = models, providers, true
	m.revision++
	m.byID = make(map[string]protocol.ModelInfo, len(models.Models))
	for _, model := range models.Models {
		m.byID[model.ID] = model
	}
	m.names = make(map[string]string, len(providers))
	for _, provider := range providers {
		m.names[provider.ID] = provider.Name
	}
	m.built = nil
}

// a provider's allowance moves with every answer and changes nothing that can be picked
func sameOffer(kept, now []protocol.ProviderInfo) bool {
	if len(kept) != len(now) {
		return false
	}
	for i := range now {
		a, b := kept[i], now[i]
		a.Limits, b.Limits = nil, nil
		if !reflect.DeepEqual(a, b) {
			return false
		}
	}
	return true
}

func (m *ModelMenu) Loaded() bool { return m.loaded }

func (m *ModelMenu) Info(id string) (protocol.ModelInfo, bool) {
	info, ok := m.byID[id]
	return info, ok
}

// the models served on this computer or the local network, Rashin's own first, then by name; nil until the list has loaded
func (m *ModelMenu) Local() []protocol.ModelInfo {
	var local []protocol.ModelInfo
	for _, model := range m.models.Models {
		if model.Tier == protocol.TierLocal {
			local = append(local, model)
		}
	}
	sort.SliceStable(local, func(i, j int) bool { return localBefore(local[i], local[j]) })
	return local
}

func localBefore(a, b protocol.ModelInfo) bool {
	if a.Provider != b.Provider {
		return localRank(a.Provider) < localRank(b.Provider)
	}
	return a.Name < b.Name
}

func localRank(provider string) int {
	if at := slices.Index(protocol.LocalProviders, provider); at >= 0 {
		return at
	}
	return len(protocol.LocalProviders)
}

// the rows for current, the model in use, which is listed even when nothing offers it now
func (m *ModelMenu) Choices(current string) (options []look.SelectOption, ids []string, selected int) {
	options, ids = m.list(current, false)
	for index, id := range ids {
		if id == current {
			selected = index
		}
	}
	return options, ids, selected
}

// agents leaves a model that cannot call tools in the list, unpickable, with the reason
func (m *ModelMenu) list(pin string, agents bool) ([]look.SelectOption, []string) {
	if !m.loaded {
		return []look.SelectOption{{Label: i18n.T("Auto")}}, []string{protocol.RouteAuto}
	}
	key := listKey{pin: pin, agents: agents}
	if built := m.built[key]; built != nil {
		return built.options, built.ids
	}
	// a form that is typed through leaves a pin per model behind
	if m.built == nil || len(m.built) > 8 {
		m.built = make(map[listKey]*modelList)
	}
	built := new(modelList)
	built.options, built.ids = modelRows(m.models, m.providers, pin, agents)
	m.built[key] = built
	return built.options, built.ids
}

// tiers in the order the list shows them: routing is local first, so the list is too
const (
	groupLocal = iota
	groupFree
	groupPaid
	groupGone
	groups
)

// the routes first, then models grouped by where they run and what they cost; a paid provider without a key shows
// what a key would open, and a pinned model no provider offers now is still shown, unpickable
func modelRows(models protocol.ModelsResult, providers []protocol.ProviderInfo, pin string, agents bool) (options []look.SelectOption, ids []string) {
	for _, route := range models.Routes {
		options, ids = append(options, look.SelectOption{Label: route.Name, Detail: route.Description}), append(ids, route.ID)
		if route.ID == pin {
			pin = ""
		}
	}
	keyless := make(map[string]bool, len(providers))
	names := make(map[string]string, len(providers))
	for _, provider := range providers {
		keyless[provider.ID] = provider.Keyless || provider.Configured
		names[provider.ID] = provider.Name
	}
	labels := [groups]string{i18n.T("On this computer"), i18n.T("Free"), i18n.T("Paid"), i18n.T("Unavailable")}
	offered := append([]protocol.ModelInfo(nil), models.Models...)
	// what can be picked comes first in its group
	sort.SliceStable(offered, func(i, j int) bool {
		a, b := offered[i], offered[j]
		if a.Available != b.Available {
			return a.Available
		}
		if a.Tier == protocol.TierLocal && b.Tier == protocol.TierLocal {
			return localBefore(a, b)
		}
		return a.Provider+a.Name < b.Provider+b.Name
	})
	var grouped [groups][]look.SelectOption
	var groupedIDs [groups][]string
	for _, model := range offered {
		id := model.ID
		group := groupFree
		switch model.Tier {
		case protocol.TierLocal:
			group = groupLocal
		case protocol.TierPaid:
			group = groupPaid
		}
		option := look.SelectOption{
			Label: model.Name, Detail: look.Clauses(id, contextWords(model.Context)), Group: labels[group],
			Mark: look.Mark{ID: model.Provider, Name: names[model.Provider]}, Chips: modelChips(model),
		}
		if !model.ToolsKnown {
			// Auto never sends one, so the reader learns why it is only here by name
			option.Detail = look.Clauses(option.Detail, i18n.T("tools unknown"))
		}
		switch {
		case model.Provider == protocol.ProviderRashinModels && (!model.Tools || model.Context < 8192 || model.Params < 500_000_000):
			option.Detail, option.Disabled = look.Clauses(option.Detail, i18n.T("Cannot serve Ask or Agents; use a model with tools, 8K context and at least 0.5B parameters")), true
		case agents && model.ToolsKnown && !model.Tools && (model.Available || id == pin):
			option.Detail, option.Disabled = look.Clauses(option.Detail, i18n.T("Cannot call tools, which agents need")), true
		case model.Available:
		// a provider's own listing of what a key would open is hundreds long, so only Rashin's picks stand for it
		case group == groupPaid && !keyless[model.Provider] && !model.Listed:
			option.Detail, option.Disabled = i18n.T("Add a key in Providers"), true
		case id == pin:
			option.Detail, option.Disabled = i18n.Tf("%s is not available now", id), true
		default:
			continue
		}
		if id == pin {
			pin = ""
		}
		grouped[group], groupedIDs[group] = append(grouped[group], option), append(groupedIDs[group], id)
	}
	if pin != "" {
		grouped[groupGone] = []look.SelectOption{{Label: pin, Detail: i18n.T("No provider offers this model now"), Group: labels[groupGone], Disabled: true}}
		groupedIDs[groupGone] = []string{pin}
	}
	for group := range groups {
		options, ids = append(options, grouped[group]...), append(ids, groupedIDs[group]...)
	}
	return options, ids
}

// what a model can do beside answering, as the short words its row and its line carry
func modelChips(model protocol.ModelInfo) []string {
	var chips []string
	if model.Light {
		chips = append(chips, i18n.T("Light"))
	}
	if model.Tools && model.ToolsKnown {
		chips = append(chips, i18n.T("Tools"))
	}
	if model.Vision {
		chips = append(chips, i18n.T("Pictures"))
	}
	if model.Audio {
		chips = append(chips, i18n.T("Sound"))
	}
	return chips
}

// "32K", the window as people say it; nothing when it is unknown
func contextWords(tokens int) string {
	switch {
	case tokens >= 1_000_000:
		return i18n.Tf("%dM", tokens/1_000_000)
	case tokens >= 1000:
		return i18n.Tf("%dK", tokens/1000)
	}
	return ""
}

func routeWords(id string) string {
	switch id {
	case protocol.RouteFast:
		return i18n.T("Fast")
	case protocol.RouteSmart:
		return i18n.T("Smart")
	}
	return i18n.T("Auto")
}

func (m *ModelMenu) route(id string) (protocol.Choice, bool) {
	if !protocol.IsRoute(id) {
		return protocol.Choice{}, false
	}
	for _, route := range m.models.Routes {
		if route.ID == id {
			return route, true
		}
	}
	return protocol.Choice{ID: id, Name: routeWords(id)}, true
}

// where a model runs, for the line beside its name
func (m *ModelMenu) where(model protocol.ModelInfo) string {
	if model.Provider == protocol.ProviderRashinModels {
		return i18n.T("On this computer")
	}
	return m.names[model.Provider]
}

// "LFM3 2B on this computer", "Claude Sonnet from Anthropic", "Auto"; the bare id while the list has not loaded or nothing offers it
func (m *ModelMenu) Words(id string) string {
	if id == "" {
		id = protocol.RouteAuto
	}
	if route, ok := m.route(id); ok {
		return route.Name
	}
	model, ok := m.byID[id]
	if !ok {
		return id
	}
	provider := m.names[model.Provider]
	switch {
	case model.Provider == protocol.ProviderRashinModels:
		return i18n.Tf("%s on this computer", model.Name)
	case provider == "":
		return model.Name
	case model.Tier == protocol.TierLocal:
		return i18n.Tf("%s on %s", model.Name, provider)
	}
	return i18n.Tf("%s from %s", model.Name, provider)
}

// some model that calls tools can run now; true while the list still loads, so nothing flashes
func (m *ModelMenu) agentReady() bool {
	if !m.loaded {
		return true
	}
	for _, model := range m.models.Models {
		if model.Available && model.Tools && model.Tier != protocol.TierPaid {
			return true
		}
	}
	return false
}

// the window's one list: every page reads names and facts from it, and only the store it was loaded for
var window struct {
	store *state.Store
	menu  *ModelMenu
}

// the frame calls it once a frame so the list is there for every page; a page may call it too
func Models(e *Env) *ModelMenu {
	if window.menu == nil || window.store != e.Store {
		// a late answer for another window's list lands in the menu it was asked for
		window.store, window.menu = e.Store, new(ModelMenu)
	}
	window.menu.Load(e, e.Snapshot())
	return window.menu
}

// a model the list does not name is really not offered only once it has loaded
func ModelsLoaded() bool { return window.menu != nil && window.menu.loaded }

// until the catalogue answers, readiness is not guessed from stale provider facts
func AgentModelsReadyOf() bool { return window.menu == nil || window.menu.agentReady() }

// read without an Env, for words made where only a snapshot is in hand
func ModelInfoOf(id string) (protocol.ModelInfo, bool) {
	if window.menu == nil {
		return protocol.ModelInfo{}, false
	}
	return window.menu.Info(id)
}

// a model's name and where it runs, a route's name, or the bare id while nothing names it
func ModelWordsOf(id string) string {
	if window.menu == nil {
		if id == "" || protocol.IsRoute(id) {
			return routeWords(id)
		}
		return id
	}
	return window.menu.Words(id)
}

// asks for the list on the way, so the words arrive without the page doing anything else
func ModelWords(e *Env, id string) string { return Models(e).Words(id) }

// what will run, in one line: the provider's mark, the name, where it runs, its window and what it can do beside answering
func ModelLine(gtx layout.Context, e *Env, id string) layout.Dimensions {
	th := e.Theme
	menu := Models(e)
	if id == "" {
		id = protocol.RouteAuto
	}
	name, where := id, ""
	var mark look.Mark
	var chips []string
	if route, ok := menu.route(id); ok {
		name, where = route.Name, route.Description
	} else if model, ok := menu.Info(id); ok {
		name, where = model.Name, look.Clauses(menu.where(model), contextWords(model.Context))
		mark = look.Mark{ID: model.Provider, Name: menu.names[model.Provider]}
		chips = modelChips(model)
	}
	head := func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, 5)
		if mark.ID != "" {
			children = append(children,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.Logo(gtx, th, mark, 16, th.R.Fg2) }),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			)
		}
		children = append(children, layout.Rigid(kit.RawText(th.Theme, th.T.Label, th.R.Fg, name)))
		if where != "" {
			children = append(children,
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(kit.RawText(th.Theme, th.T.Caption, th.R.Fg3, where)),
			)
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	}
	return look.Flow{Gap: th.Sp.S2, LineGap: th.Sp.S1}.Layout(gtx, 1+len(chips), func(gtx layout.Context, index int) layout.Dimensions {
		if index == 0 {
			return head(gtx)
		}
		return look.Chip(th, nil, chips[index-1], palette.Idle)(gtx)
	})
}

// the one sentence for the state in which nothing can run an agent, for boards and next-step rows
func NoAgentModel() string { return i18n.T("No model can run agents yet") }

var iconDownload = kit.Icon(symbols.Download)

// whether anything can run an agent now and, when nothing can, the one way to fix it
type AgentReady struct {
	// a view that already has its gold keeps it: the action is tonal there
	Tonal bool

	get kit.Plaque
}

// true while the list still loads, so the card never flashes on a box that is ready
func (r *AgentReady) Ready(e *Env) bool { return Models(e).agentReady() }

func (r *AgentReady) Layout(gtx layout.Context, e *Env) layout.Dimensions {
	th := e.Theme
	r.get.Kind, r.get.Label, r.get.Icon = kit.PlaqueGold, i18n.T("Get a local model"), iconDownload
	if r.Tonal {
		r.get.Kind = kit.PlaqueTonal
	}
	if r.get.Clicked(gtx) && e.OpenSettings != nil {
		e.OpenSettings("providers")
	}
	return kit.SlabContent(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(look.Paragraph(th, th.T.Title, th.R.Fg, NoAgentModel())),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2,
				i18n.T("An agent runs on a model that can call tools. Rashin can download one and run it on this computer, or you can add a provider's key. Both are in Settings, under Providers."))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = 0
				return r.get.Layout(gtx, th.Theme)
			}),
		)
	})
}
