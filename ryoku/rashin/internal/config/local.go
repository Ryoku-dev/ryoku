package config

import (
	"slices"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

// validateLocal keeps one usable address for every local provider: a missing
// or unusable one falls back to the default, and unknown ids are dropped.
func validateLocal(p *protocol.ProvidersSettings, d protocol.ProvidersSettings) {
	local := make(map[string]string, len(protocol.LocalProviders))
	for _, id := range protocol.LocalProviders {
		address, ok := protocol.ParseLocalAddress(p.Local[id])
		if !ok {
			address = d.Local[id]
		}
		local[id] = address
	}
	p.Local = local
}

// checkLocal refuses a patch that names an unknown local provider or gives
// one an address off this computer and the local network.
func checkLocal(p protocol.ProvidersSettings) error {
	for id, address := range p.Local {
		if !slices.Contains(protocol.LocalProviders, id) {
			return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Rashin knows no local provider %s.", id))
		}
		if _, ok := protocol.ParseLocalAddress(address); !ok {
			return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Use an address on this computer or your local network, such as %s.", protocol.DefaultLocalAddresses()[id]))
		}
	}
	return nil
}
