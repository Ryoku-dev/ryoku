package teamsui

import (
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// the vault's state as one sentence: what it holds and what locks it, or why it is closed
func VaultState(vault protocol.VaultInfo) string {
	switch vault.State {
	case protocol.VaultOpen:
		keeper := vault.Keeper
		if keeper == "" {
			keeper = i18n.T("your login keyring")
		}
		switch count := len(vault.Credentials); count {
		case 0:
			return i18n.Tf("No credentials yet, locked with %s.", keeper)
		case 1:
			return i18n.Tf("One credential, locked with %s.", keeper)
		default:
			return i18n.Tf("%s credentials, locked with %s.", look.CountStart(count), keeper)
		}
	case protocol.VaultClosed:
		return i18n.T("The vault is closed: your login keyring is locked.")
	}
	return i18n.T("The vault stays closed: there is no login keyring.")
}
