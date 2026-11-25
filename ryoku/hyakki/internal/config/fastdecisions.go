package config

import (
	"strings"
	"unicode/utf8"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

func validDecisionModel(model string) bool {
	return len(model) <= 256 && utf8.ValidString(model) && !strings.ContainsAny(model, " \t\r\n\x00")
}

func validateFastDecisions(p *protocol.DecisionRoutingSettings) {
	if !protocol.ValidDecisionRoute(p.Route) {
		p.Route = protocol.DecisionRules
	}
	p.BudgetMS = clamp(p.BudgetMS, 100, 5000, protocol.DefaultDecisionBudgetMS)
	p.LocalModel = strings.TrimSpace(p.LocalModel)
	if !validDecisionModel(p.LocalModel) {
		p.LocalModel = ""
	}
	if strings.TrimSpace(p.LocalAddress) == "" {
		p.LocalAddress = protocol.DefaultDecisionAddress
	}
	address, ok := protocol.ParseDecisionAddress(p.LocalAddress)
	if !ok {
		p.LocalModel = ""
		if p.Route == protocol.DecisionLocal {
			p.Route = protocol.DecisionRules
		}
		address = protocol.DefaultDecisionAddress
	}
	p.LocalAddress = address
	if len(p.Features) > 32 {
		p.Features = nil
	}
	if len(p.Features) > 0 {
		features := make(map[string]string, len(p.Features))
		for feature, route := range p.Features {
			if protocol.ValidDecisionFeature(feature) && protocol.ValidDecisionRoute(route) {
				features[feature] = route
			}
		}
		p.Features = features
	}
}

func checkFastDecisions(p protocol.DecisionRoutingSettings) error {
	bad := func(message string) error { return protocol.Errorf(protocol.CodeBadRequest, "%s", message) }
	if p.Route != "" && !protocol.ValidDecisionRoute(p.Route) {
		return bad(i18n.T("Choose built-in rules, local decisions, Ryoku Ember or automatic local-first decisions."))
	}
	if p.BudgetMS != 0 && (p.BudgetMS < 100 || p.BudgetMS > 5000) {
		return bad(i18n.T("The decision time budget must be between 100 and 5000 milliseconds."))
	}
	if !validDecisionModel(strings.TrimSpace(p.LocalModel)) {
		return bad(i18n.T("The local decision model name is invalid."))
	}
	if p.LocalAddress != "" {
		if _, ok := protocol.ParseDecisionAddress(p.LocalAddress); !ok {
			return bad(i18n.T("Local decisions need a loopback address on this computer, without credentials, query or fragment."))
		}
	}
	if len(p.Features) > 32 {
		return bad(i18n.T("Choose at most 32 feature decision routes."))
	}
	for feature, route := range p.Features {
		if !protocol.ValidDecisionFeature(feature) || !protocol.ValidDecisionRoute(route) {
			return bad(i18n.T("A feature decision route is invalid."))
		}
	}
	return nil
}
