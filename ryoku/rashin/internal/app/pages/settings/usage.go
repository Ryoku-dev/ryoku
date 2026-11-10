package settings

import (
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/protocol"
)

const usageDays = 7

// what each provider answered in the last week, read when Settings opens
type usage struct {
	result protocol.GatewayUsage
	asked  bool
}

func (u *usage) load(e *env.Env) {
	if u.asked {
		return
	}
	u.asked = true
	e.Call(protocol.MGatewayUsage, protocol.UsageParams{Days: usageDays}, &u.result, func(err error) {
		if err != nil {
			u.asked = false
		}
	})
}

// words says how much a provider was used, or nothing when it was not
func (u *usage) words(provider string) string {
	answered, failed := 0, 0
	for _, row := range u.result.Rows {
		if row.Provider == provider {
			answered += row.OK
			failed += row.Failed
		}
	}
	switch {
	case answered == 0 && failed == 0:
		return ""
	case failed == 0:
		return i18n.Tf("%d answers this week", answered)
	}
	return i18n.Tf("%d answers and %d failures this week", answered, failed)
}
