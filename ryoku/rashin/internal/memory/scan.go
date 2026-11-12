package memory

import (
	i18n "ryoku-i18n"
	"ryoku-rashin/internal/memory/scan"
	"ryoku-rashin/internal/protocol"
)

// threatReason says in words why a memory with this threat is not used.
func threatReason(t scan.Threat) string {
	switch {
	case t.Secret:
		return i18n.T("It looks like a password or key.")
	case t.ID == scan.Invisible:
		return i18n.T("It holds hidden characters.")
	default:
		return i18n.T("It reads like an instruction, not a fact.")
	}
}

// threatRefusal is the protocol reason for a refused memory.
func threatRefusal(t scan.Threat) string {
	if t.Secret {
		return protocol.MemoryRefusedSecret
	}
	return protocol.MemoryRefusedInstruction
}

// scanStrict checks text about to be written to memory or read back from a
// memory file: hit is true when it must not be used.
func scanStrict(text string) (reason string, hit bool) {
	t, hit := scan.Strict(text)
	if !hit {
		return "", false
	}
	return threatReason(t), true
}

// scanContext checks recalled text before it is put in front of a model.
func scanContext(text string) (reason string, hit bool) {
	t, hit := scan.Context(text)
	if !hit {
		return "", false
	}
	return threatReason(t), true
}
