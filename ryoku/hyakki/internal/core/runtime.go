package core

import (
	"context"

	"ryoku-rashin/internal/protocol"
)

// TeamRuntimes is trusted daemon-to-team access for configured external runtimes.
// These methods are not socket methods or model tools. They resolve only the
// calling run's configured references; the runner owns the central action gate.
type TeamRuntimes interface {
	RuntimeEnvironment(context.Context, protocol.SessionInfo) (map[string]string, error)
	RuntimeHTTP(context.Context, protocol.SessionInfo, []byte) (protocol.AgentHTTPResult, error)
}
