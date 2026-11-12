package doctor

import (
	"ryoku-cli/internal/updater"

	i18n "ryoku-i18n"
)

func accountSeedDetail(p updater.AccountSeedPlan) string {
	switch {
	case len(p.Copies) > 0 && p.QueueRice:
		return i18n.Tf("%d Ryoku file(s) missing from this account (brand art, decor art, npm prefix), and the default rice was never applied", len(p.Copies))
	case p.QueueRice:
		return i18n.T("this new account never had the default rice applied")
	}
	return i18n.Tf("%d Ryoku file(s) missing from this account (brand art, decor art, npm prefix)", len(p.Copies))
}

// an account the installer never set up (a second user, one added in the Hub)
// gets what the first account got; the default rice only if its shell never ran
func reconcileAccountSeed(checkOnly bool) recResult {
	plan := updater.PlanAccountSeed()
	if plan.Empty() {
		return okRes(i18n.T("this account has its brand art, decor art and npm prefix"))
	}
	if checkOnly {
		return wouldRes("%s", accountSeedDetail(plan)).withFix("ryoku doctor")
	}
	if err := updater.ApplyAccountSeed(plan); err != nil {
		return failRes(i18n.T("could not set up this account's Ryoku files: %v"), err).withFix("ryoku doctor")
	}
	if plan.QueueRice {
		return fixedRes(i18n.T("seeded %d Ryoku file(s) into this account; the default rice is applied when the desktop starts"), len(plan.Copies))
	}
	return fixedRes(i18n.T("seeded %d Ryoku file(s) into this account"), len(plan.Copies))
}
