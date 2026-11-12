package vault

import (
	"sort"
	"strings"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/protocol"
)

func credentialByID(vault protocol.VaultInfo, id string) (protocol.Credential, bool) {
	if id == "" {
		return protocol.Credential{}, false
	}
	for _, credential := range vault.Credentials {
		if credential.ID == id {
			return credential, true
		}
	}
	return protocol.Credential{}, false
}

// the mail account names its password by id, an older one by name
func mailCredential(vault protocol.VaultInfo, ref string) (protocol.Credential, bool) {
	for _, credential := range vault.Credentials {
		if ref != "" && (credential.ID == ref || credential.Name == ref) {
			return credential, true
		}
	}
	return protocol.Credential{}, false
}

func sortedCredentials(vault protocol.VaultInfo) []protocol.Credential {
	credentials := append([]protocol.Credential(nil), vault.Credentials...)
	sort.Slice(credentials, func(i, j int) bool { return credentials[i].Name < credentials[j].Name })
	return credentials
}

// what became of one use of a credential, in a word and its tone
func outcomeWord(outcome string) (string, palette.Tone) {
	switch outcome {
	case protocol.VaultUsed:
		return i18n.T("Sent"), palette.Alive
	case protocol.VaultAsked:
		return i18n.T("Asked you"), palette.Attention
	case protocol.VaultDenied:
		return i18n.T("Denied"), palette.Paused
	case protocol.VaultRefused:
		return i18n.T("Refused"), palette.Failed
	case protocol.VaultFailed:
		return i18n.T("Failed"), palette.Failed
	case protocol.VaultAdded:
		return i18n.T("Added"), palette.Idle
	case protocol.VaultRotated:
		return i18n.T("Value replaced"), palette.Idle
	case protocol.VaultRemoved:
		return i18n.T("Removed"), palette.Idle
	}
	return outcome, palette.Idle
}

// the teams that may use a credential, by name, as a sentence without its period
func teamNames(teams []protocol.Team, ids []string) string {
	if len(ids) == 0 {
		return i18n.T("No team may use it")
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		names = append(names, teamsui.TeamName(teams, id))
	}
	return i18n.Tf("%s may use it", strings.Join(names, ", "))
}
