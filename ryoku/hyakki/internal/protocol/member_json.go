package protocol

import "encoding/json"

// MarshalJSON preserves the skill policy's three states: a nil pointer inherits,
// a non-nil empty allowlist denies all, and a nonempty allowlist narrows access.
// In particular, a pointer to a nil slice must encode as [] rather than null;
// ordinary JSON decoding intentionally continues to treat null as inheritance.
func (member Member) MarshalJSON() ([]byte, error) {
	type memberJSON Member
	if member.Skills != nil && *member.Skills == nil {
		empty := []string{}
		member.Skills = &empty
	}
	return json.Marshal(memberJSON(member))
}
