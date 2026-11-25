package memory

import (
	i18n "ryoku-i18n"
	"ryoku-rashin/internal/memory/scan"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sensitive"
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

// numberReason says why memory keeps no card, social security, identity or
// bank account number.
func numberReason() string {
	return i18n.T("Hyakki doesn't keep card, social security, ID or bank account numbers.")
}

// refusal is why the store will not keep text, nil when it will: a card,
// social security, identity or bank account number (sensitive.Number),
// whoever writes it, then what the threat scan finds. A number's refusal
// does not repeat the text.
func refusal(text string) *RefusedError {
	if sensitive.Number(text) != sensitive.None {
		return &RefusedError{Reason: protocol.MemoryRefusedNumber, Detail: numberReason()}
	}
	if th, hit := scan.Strict(text); hit {
		return &RefusedError{Reason: threatRefusal(th), Text: text, Detail: threatReason(th)}
	}
	return nil
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
