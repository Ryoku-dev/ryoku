package host

import (
	"fmt"

	"ryoku-cli/internal/profile"
	i18n "ryoku-i18n"
)

func (a *App) profileRefresh() int {
	receipts, err := profile.ReadSystemReceipts(profileRootPath)
	if err != nil {
		return a.profileFail(fmt.Sprintf(i18n.T("Read applied profiles: %v"), err))
	}
	for index := range receipts {
		if receipts[index].Staged == "" {
			continue
		}
		manifest, err := profile.LoadManifest(receipts[index].Staged)
		if err != nil {
			return a.profileFail(fmt.Sprintf(i18n.T("Read staged profile %s: %v"), receipts[index].Profile.ID, err))
		}
		if err := a.applyProfileSystem(manifest, &receipts[index]); err != nil {
			return a.profileFail(err.Error())
		}
		if err := a.pruneProfileSystem(manifest, &receipts[index]); err != nil {
			return a.profileFail(err.Error())
		}
		if err := profile.WriteSystemReceipt(profileRootPath, &receipts[index]); err != nil {
			return a.profileFail(err.Error())
		}
	}
	if err := a.rewriteProfileRefreshHook(receipts); err != nil {
		return a.profileFail(fmt.Sprintf(i18n.T("Write the profile refresh hook: %v"), err))
	}
	if digest, err := hashFile(profileRefreshHookPath); err == nil {
		for index := range receipts {
			for _, file := range receipts[index].System {
				if file.Kind == "pacman-hook" {
					receipts[index].System = replaceSystemFile(receipts[index].System, profile.SystemFile{Kind: "pacman-hook", Path: profileRefreshHookPath, SHA256: digest})
					if err := profile.WriteSystemReceipt(profileRootPath, &receipts[index]); err != nil {
						return a.profileFail(err.Error())
					}
					break
				}
			}
		}
	}
	return ExitOK
}
