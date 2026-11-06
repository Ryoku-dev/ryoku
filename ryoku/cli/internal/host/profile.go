package host

import (
	i18n "ryoku-i18n"
)

func (a *App) Profile(args []string) int {
	if len(args) == 0 {
		return a.profileFail(i18n.T("Choose a profile action."))
	}
	switch args[0] {
	case "api":
		if len(args) != 1 {
			return a.profileFail(i18n.T("profile api does not accept arguments."))
		}
		return a.profileAPI()
	case "status":
		return a.profileStatus(args[1:])
	case "plan":
		return a.profilePlan(args[1:])
	case "plan-remove":
		return a.profilePlanRemove(args[1:])
	case "checkpoint":
		if len(args) >= 2 && args[1] == "list" {
			return a.profileCheckpoint(args[1:])
		}
	}
	if a.cfg.UID != 0 {
		return a.profileFail(i18n.T("Administrator authentication is required."))
	}
	switch args[0] {
	case "apply":
		return a.profileApply(args[1:])
	case "remove":
		return a.profileRemove(args[1:])
	case "reset":
		return a.profileReset(args[1:])
	case "checkpoint":
		return a.profileCheckpoint(args[1:])
	case "refresh":
		if len(args) != 1 {
			return a.profileFail(i18n.T("profile refresh does not accept arguments."))
		}
		return a.profileRefresh()
	default:
		return a.profileFail(i18n.T("Unknown profile action."))
	}
}
