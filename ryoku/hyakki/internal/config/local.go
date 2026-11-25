package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

// validateLocal keeps one usable address for every addressed local provider:
// a missing or unusable one falls back to the default, and unknown ids are
// dropped. Rashin's own models get sane values: a light model the gateway
// cannot route and a binary path that is not absolute are cleared. Whether
// the binary still runs is checked when the router starts, so a file that is
// briefly gone does not lose the setting.
func validateLocal(p *protocol.ProvidersSettings, d protocol.ProvidersSettings) {
	local := make(map[string]string, len(protocol.AddressedLocalProviders))
	for _, id := range protocol.AddressedLocalProviders {
		address, ok := protocol.ParseLocalAddress(p.Local[id])
		if !ok {
			address = d.Local[id]
		}
		local[id] = address
	}
	p.Local = local
	m := &p.Models
	m.IdleMinutes = clamp(m.IdleMinutes, 1, protocol.LocalIdleMinutesMax, d.Models.IdleMinutes)
	m.Binary = strings.TrimSpace(m.Binary)
	if m.Binary != "" {
		if !filepath.IsAbs(m.Binary) {
			m.Binary = ""
		} else {
			m.Binary = filepath.Clean(m.Binary)
		}
	}
	folders := make([]string, 0, min(len(m.Folders), 32))
	for _, folder := range m.Folders {
		folder = filepath.Clean(strings.TrimSpace(folder))
		if !filepath.IsAbs(folder) || slices.Contains(folders, folder) || len(folders) >= 32 {
			continue
		}
		folders = append(folders, folder)
	}
	m.Folders = folders
	m.Device = strings.TrimSpace(m.Device)
	m.Light = strings.TrimSpace(m.Light)
	if m.Light != "" && !protocol.ValidModel(m.Light) {
		m.Light = ""
	}
	overrides := make(map[string]protocol.LocalModelOverride, len(m.Overrides))
	for id, choice := range m.Overrides {
		if !validLocalOverride(id, choice) {
			continue
		}
		overrides[id] = choice
	}
	m.Overrides = overrides
	p.Account, _ = protocol.AccountServer(p.Account)
	validateFastDecisions(&p.Decisions)
}

// checkLocal refuses a patch that names an unknown or unaddressed local
// provider, gives one an address off this computer and the local network,
// points Rashin's llama-server at something that is not an executable file,
// or names a light model the gateway cannot route. The binary is checked only
// when the patch changes it from cur, so a binary that went away later does
// not block every other setting.
func checkLocal(p, cur protocol.ProvidersSettings) error {
	for id, address := range p.Local {
		if !slices.Contains(protocol.AddressedLocalProviders, id) {
			return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Hyakki knows no local provider %s.", id))
		}
		if _, ok := protocol.ParseLocalAddress(address); !ok {
			return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Use an address on this computer or your local network, such as %s.", protocol.DefaultLocalAddresses()[id]))
		}
	}
	if binary := strings.TrimSpace(p.Models.Binary); binary != "" && binary != cur.Models.Binary && !executable(binary) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose the full path of a llama-server program that can run."))
	}
	if len(p.Models.Folders) > 32 {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose at most 32 model folders."))
	}
	for _, folder := range p.Models.Folders {
		if !filepath.IsAbs(folder) {
			return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose a full folder path to search for models."))
		}
		if !slices.Contains(cur.Models.Folders, folder) {
			if info, err := os.Stat(folder); err != nil || !info.IsDir() {
				return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Model folder %s is not readable.", folder))
			}
		}
	}
	if light := strings.TrimSpace(p.Models.Light); light != "" && !protocol.ValidModel(light) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The model for light work is invalid."))
	}
	for id, choice := range p.Models.Overrides {
		if !validLocalOverride(id, choice) {
			return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose an installed model, a supported context and a valid cache type."))
		}
	}
	if _, ok := protocol.AccountServer(p.Account); !ok {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Use an https address for the Ryoku Accounts server, or http on this computer."))
	}
	return checkFastDecisions(p.Decisions)
}

func validLocalOverride(id string, choice protocol.LocalModelOverride) bool {
	if len(id) == 0 || len(id) > 64 || strings.ContainsAny(id, "/\\\r\n\x00") {
		return false
	}
	for _, r := range id {
		if !('a' <= r && r <= 'z' || '0' <= r && r <= '9' || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	if choice.Device != "" && choice.Device != "cpu" {
		if len(choice.Device) > 64 {
			return false
		}
		for _, r := range choice.Device {
			if !('A' <= r && r <= 'Z' || 'a' <= r && r <= 'z' || '0' <= r && r <= '9' || r == '_') {
				return false
			}
		}
	}
	return (choice.Context == 0 || choice.Context >= 128 && choice.Context <= 65536 && choice.Context%128 == 0) &&
		(choice.KVType == "" || choice.KVType == "f16" || choice.KVType == "q8_0")
}

// executable reports an absolute path to a regular file someone may run.
func executable(path string) bool {
	if !filepath.IsAbs(path) {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}
