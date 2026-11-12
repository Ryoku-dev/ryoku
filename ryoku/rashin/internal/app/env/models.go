package env

import (
	"reflect"
	"sort"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// the rows of a select that picks Ask's model: the routes, then the models by what they cost, asked again when a provider changes
type ModelMenu struct {
	models    protocol.ModelsResult
	providers []protocol.ProviderInfo
	epoch     int
	loading   bool
	loaded    bool

	// built for one pin, so a pin the list does not offer is shown without rebuilding every frame
	pin     string
	built   bool
	options []look.SelectOption
	ids     []string
}

// a key added or a server started changes what can be picked, so the list follows the providers
func (m *ModelMenu) Load(e *Env, snapshot state.Snapshot) {
	if m.loading || (m.loaded && m.epoch == snapshot.Epoch && sameOffer(m.providers, snapshot.Providers)) {
		return
	}
	m.loading = true
	var result protocol.ModelsResult
	e.Call(protocol.MModelsList, protocol.ModelsParams{}, &result, func(err error) {
		m.loading = false
		if err != nil {
			return
		}
		m.models, m.providers, m.epoch, m.loaded, m.built = result, snapshot.Providers, snapshot.Epoch, true, false
	})
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

// the rows for current, the model in use, which is listed even when nothing offers it now
func (m *ModelMenu) Choices(current string) (options []look.SelectOption, ids []string, selected int) {
	if !m.loaded {
		return []look.SelectOption{{Label: i18n.T("Auto")}}, []string{protocol.RouteAuto}, 0
	}
	if !m.built || m.pin != current {
		m.options, m.ids = modelRows(m.models, m.providers, current)
		m.pin, m.built = current, true
	}
	for index, id := range m.ids {
		if id == current {
			selected = index
		}
	}
	return m.options, m.ids, selected
}

// tiers in the order the list shows them
const (
	groupFree = iota
	groupLocal
	groupPaid
	groupGone
	groups
)

// the routes first, then models grouped free, local and paid; a paid provider without a key shows
// what a key would open, and a pinned model no provider offers now is still shown, unpickable
func modelRows(models protocol.ModelsResult, providers []protocol.ProviderInfo, pin string) (options []look.SelectOption, ids []string) {
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
	labels := [groups]string{i18n.T("Free"), i18n.T("Local"), i18n.T("Paid"), i18n.T("Unavailable")}
	offered := append([]protocol.ModelInfo(nil), models.Models...)
	// what can be picked comes first in its group
	sort.SliceStable(offered, func(i, j int) bool {
		if offered[i].Available != offered[j].Available {
			return offered[i].Available
		}
		return offered[i].Provider+offered[i].Name < offered[j].Provider+offered[j].Name
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
		option := look.SelectOption{Label: model.Name, Detail: id, Group: labels[group], Mark: look.Mark{ID: model.Provider, Name: names[model.Provider]}}
		if !model.ToolsKnown {
			// Auto never sends one, so the reader learns why it is only here by name
			option.Detail = i18n.Tf("%s, tools unknown", id)
		}
		switch {
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
