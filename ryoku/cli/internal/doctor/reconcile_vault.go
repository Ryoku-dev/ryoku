package doctor

import (
	"ryoku-cli/internal/vault"

	i18n "ryoku-i18n"
)

func reconcileVault(checkOnly bool) recResult {
	status := vault.Inspect()
	healthy := status.Exists && status.Current && !status.LegacyPresent && (status.OptedOut || status.Wired)
	if healthy {
		if status.OptedOut {
			return okRes(i18n.T("the Ryoku vault is current; harness wiring is opted out"))
		}
		return okRes(i18n.T("the Ryoku vault is current and wired into installed harnesses"))
	}
	if checkOnly {
		return wouldRes(i18n.T("the Ryoku vault needs a refresh")).withFix(i18n.T("ryoku vault refresh"))
	}
	if err := vault.Refresh(); err != nil {
		return failRes(i18n.T("the Ryoku vault could not be refreshed: %v"), err).withFix(i18n.T("ryoku vault refresh"))
	}
	status = vault.Inspect()
	if !status.Exists || !status.Current || status.LegacyPresent || (!status.OptedOut && !status.Wired) {
		return failRes(i18n.T("the Ryoku vault refresh finished but its state is incomplete")).withFix(i18n.T("ryoku vault status"))
	}
	if status.OptedOut {
		return fixedRes(i18n.T("refreshed the Ryoku vault and kept harness wiring off"))
	}
	return fixedRes(i18n.T("refreshed and wired the Ryoku vault"))
}
