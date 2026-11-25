package demo

import (
	i18n "ryoku-i18n"

	"ryoku-rashin/internal/localmodels"
	"ryoku-rashin/internal/protocol"
)

// FeatureOniSmallComputer is no feature: it starts the demo as FeatureOni
// does, on a computer under the local decision hardware gate, so a look for
// the local decision server says this computer is too small.
const FeatureOniSmallComputer = "oni-small-computer"

// The demo computer: big enough for a local decision model, or, with
// FeatureOniSmallComputer, not.
var (
	demoComputer      = localmodels.Hardware{RAMMiB: 16 << 10, Cores: 8}
	demoSmallComputer = localmodels.Hardware{RAMMiB: 4 << 10, Cores: 2}
)

// localDecisions answers fastdecisions.local as the daemon does, through the
// same hardware gate, on a demo computer whose local decision server
// answers at the default address and nowhere else.
func (s *server) localDecisions() protocol.DecisionLocalStatus {
	address := s.data.snapshot.Settings.Providers.Decisions.LocalAddress
	if address == "" {
		address = protocol.DefaultDecisionAddress
	}
	base, ok := protocol.ParseDecisionAddress(address)
	if !ok {
		return protocol.DecisionLocalStatus{State: protocol.DecisionLocalRefused, Reason: i18n.T("The local decision address is not a loopback address on this computer, so nothing was asked there.")}
	}
	hardware := demoComputer
	if s.data.oni.smallComputer {
		hardware = demoSmallComputer
	}
	status := protocol.DecisionLocalStatus{State: protocol.DecisionLocalDetected, Address: base, RAMMiB: hardware.RAMMiB, Cores: hardware.Cores}
	if fits, reason := localmodels.DecisionHardware(hardware); !fits {
		status.State, status.Reason = protocol.DecisionLocalSmall, reason
		return status
	}
	if served, _ := protocol.ParseDecisionAddress(protocol.DefaultDecisionAddress); base != served {
		status.State = protocol.DecisionLocalAbsent
		status.Reason = i18n.Tf("No local decision server answers at %s. Hyakki installs and starts none; start the one you installed, or choose Built-in rules.", base)
	}
	return status
}
