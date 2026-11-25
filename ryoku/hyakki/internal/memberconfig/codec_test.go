package memberconfig

import (
	"encoding/json"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestCodecRejectsFiniteResourceBoundsInDecodeAndEncode(t *testing.T) {
	current := protocol.Member{ID: "member", Name: "Member", Revision: 1}
	prefix := "version: 1\nteam: team\nmember:\n  id: member\n  revision: 1\n"
	for _, tc := range []struct {
		name, body string
	}{
		{"scalar", "  instructions: " + strings.Repeat("x", MaxScalarBytes+1) + "\n"},
		{"key", "  options:\n    " + strings.Repeat("x", MaxKeyBytes+1) + ": value\n"},
		{"nodes", "  skills:\n" + strings.Repeat("    - skill\n", MaxNodes)},
		{"aggregate", "  skills:\n" + strings.Repeat("    - "+strings.Repeat("x", MaxScalarBytes)+"\n", MaxContentBytes/MaxScalarBytes+1)},
		{"depth", "  skills: " + strings.Repeat("[", MaxDepth+1) + "skill" + strings.Repeat("]", MaxDepth+1) + "\n"},
		{"duplicate", "  name: First\n  name: Second\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Decode(prefix+tc.body, "team", "member", 1, current); err == nil {
				t.Fatal("unsafe document passed codec bounds")
			}
		})
	}
	for _, tc := range []struct {
		name   string
		member protocol.Member
	}{
		{"scalar", protocol.Member{Instructions: strings.Repeat("x", MaxScalarBytes+1)}},
		{"key", protocol.Member{Options: map[string]string{strings.Repeat("x", MaxKeyBytes+1): "value"}}},
		{"nodes", protocol.Member{Skills: repeatedSkills(MaxNodes, "skill")}},
		{"aggregate", protocol.Member{Skills: repeatedSkills(MaxContentBytes/MaxScalarBytes+1, strings.Repeat("x", MaxScalarBytes))}},
	} {
		t.Run("encode "+tc.name, func(t *testing.T) {
			if err := CheckBounds(tc.member); err == nil {
				t.Fatal("shared canonical bounds accepted unsafe member")
			}
			if _, err := Encode("team", Snapshot{Member: tc.member}, 1); err == nil {
				t.Fatal("export accepted unsafe member")
			}
		})
	}
}

func repeatedSkills(count int, value string) *[]string {
	values := make([]string, count)
	for i := range values {
		values[i] = value
	}
	return &values
}

func TestWireTextCountsActualJSONEscaping(t *testing.T) {
	for _, text := range []string{
		"ordinary text",
		"<>&",
		"\"\\\b\f\n\r\t\x01",
		"\u2028\u2029𐍈",
		strings.Repeat("<", MaxBytes/6),
	} {
		raw, err := json.Marshal(text)
		if err != nil {
			t.Fatal(err)
		}
		if err := checkWireTextLimit(text, len(raw)); err != nil {
			t.Fatalf("exact JSON string size was rejected: %v", err)
		}
		if err := checkWireTextLimit(text, len(raw)-1); err == nil {
			t.Fatal("JSON string beyond its wire allowance was accepted")
		}
	}
}

func TestCanonicalBoundsRejectWireUnsafeButStructurallyBoundedConfiguration(t *testing.T) {
	value := strings.Repeat("\x01", 16384)
	member := protocol.Member{
		ID: "member", Name: "Member", Revision: 1,
		Runtime: &protocol.MemberRuntime{
			Kind:    protocol.MemberRuntimeCommand,
			Command: make([]string, 64),
			Env:     make(map[string]string, 64),
		},
		Skills: repeatedSkills(7800, strings.Repeat("\x01", 200)),
	}
	for i := range member.Runtime.Command {
		member.Runtime.Command[i] = value
	}
	for i := range 64 {
		member.Runtime.Env[strings.Repeat("E", i+1)] = value
	}
	if _, err := canonicalBounds(member); err != nil {
		t.Fatalf("fixture violates structural rather than wire bounds: %v", err)
	}
	if err := CheckBounds(member); err == nil {
		t.Fatal("canonical validation accepted an untransmittable YAML export")
	}
	if _, err := Encode("team", Snapshot{Member: member}, 1); err == nil {
		t.Fatal("export accepted an untransmittable document")
	}
}
