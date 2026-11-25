package teams

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"ryoku-rashin/internal/memberconfig"
	"ryoku-rashin/internal/protocol"
)

// Exercise the production wire encoder/decoder in both directions, including
// RawMessage wrapping and a maximum-width request ID, rather than just handlers.
func configWireDo[T any](t *testing.T, h *harness, method string, params any) T {
	t.Helper()
	const id = int64(1<<63 - 1)
	var wire bytes.Buffer
	raw, err := protocol.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	if err := protocol.NewEncoder(&wire).Encode(protocol.Request{ID: id, Method: method, Params: raw}); err != nil {
		t.Fatal(err)
	}
	request, err := protocol.NewDecoder(&wire).Next()
	if err != nil {
		t.Fatal(err)
	}
	out, err := h.call(request.Method, request.Params)
	if err != nil {
		t.Fatal(err)
	}
	result, err := protocol.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := protocol.NewEncoder(&wire).Encode(protocol.Response{ID: id, Result: result}); err != nil {
		t.Fatal(err)
	}
	response, err := protocol.NewDecoder(&wire).Next()
	if err != nil {
		t.Fatal(err)
	}
	var value T
	if err := json.Unmarshal(response.Result, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestMemberConfigYAMLRejectsCaseFoldedSchemaWithoutDurableChanges(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Exact YAML fields", func(p *protocol.TeamCreateParams) {
		p.Reporting = protocol.ReportingTree
		p.Members = []protocol.Member{{ID: "root", Name: "Root", Lead: true}, {ID: "worker", Name: "Worker", ReportsTo: "root"}}
	})
	member := &team.Members[1]
	member.Runtime = &protocol.MemberRuntime{Kind: protocol.MemberRuntimeCommand, Command: []string{"fixture"}, Env: map[string]string{"Path": "mixed", "PATH": "upper"}}
	member.Budget = &protocol.TeamBudget{}
	member.Tools = &protocol.TeamTools{}
	member.Heartbeat = &protocol.MemberHeartbeat{Modes: []string{protocol.HeartbeatInterval}, EveryMinutes: 10}
	denied := []string{}
	member.Skills = &denied
	team = do[protocol.Team](t, h, protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: team.ID, Patch: mustJSON(t, map[string]any{"members": team.Members})})
	ref := configRef(team, 1)
	original := configDocument(t, h, ref)
	history := do[protocol.MemberConfigHistoryResult](t, h, protocol.MTeamsMemberConfigHistory, ref)
	for _, tc := range []struct{ name, from, to string }{
		{"top version", "version:", "Version:"},
		{"top member", "member:\n", "Member:\n"},
		{"member title", "  title:", "  Title:"},
		{"member reporting", "  reportsTo:", "  reportsto:"},
		{"semantic duplicate", "  reportsTo: " + team.Members[0].ID, "  reportsTo: " + team.Members[0].ID + "\n  reportsto: " + team.Members[0].ID},
		{"runtime kind", "    kind:", "    Kind:"},
		{"runtime environment", "    env:\n", "    Env:\n"},
		{"budget", "    monthTokens:", "    MonthTokens:"},
		{"tool", "  tools: {}", "  tools:\n    Mail: false"},
		{"heartbeat", "    everyMinutes:", "    EveryMinutes:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(original.YAML, tc.from) {
				t.Fatalf("fixture lacks %q", tc.from)
			}
			text := configTitle(strings.Replace(original.YAML, tc.from, tc.to, 1), "Must not persist")
			if _, err := h.call(protocol.MTeamsMemberConfigSave, protocol.MemberConfigSaveParams{Team: ref.Team, Member: ref.Member, YAML: text, ExpectedRevision: original.Revision}); err == nil {
				t.Fatal("case-folded schema field was accepted")
			}
			if after := configDocument(t, h, ref); after != original {
				t.Fatal("rejected YAML changed the current configuration")
			}
			if after := do[protocol.MemberConfigHistoryResult](t, h, protocol.MTeamsMemberConfigHistory, ref); !reflect.DeepEqual(after, history) {
				t.Fatal("rejected YAML changed history")
			}
		})
	}
	restartHeartbeatHarness(t, h)
	if reopened := configDocument(t, h, ref); reopened != original {
		t.Fatal("rejected YAML changed durable configuration")
	}
	saved := do[protocol.MemberConfigDocument](t, h, protocol.MTeamsMemberConfigSave, protocol.MemberConfigSaveParams{Team: ref.Team, Member: ref.Member, YAML: configTitle(original.YAML, "Legal edit"), ExpectedRevision: original.Revision})
	restartHeartbeatHarness(t, h)
	live, _ := h.s.state.team(team.ID)
	got := live.Members[1]
	if got.Runtime.Env["Path"] != "mixed" || got.Runtime.Env["PATH"] != "upper" || len(got.Runtime.Env) != 2 || got.Skills == nil || len(*got.Skills) != 0 || got.Revision != saved.Revision {
		t.Fatal("legal map key casing or explicit skill denial changed")
	}
}

func TestMemberConfigLargeRuntimeFormYAMLHistoryAndRestart(t *testing.T) {
	for _, tc := range []struct {
		name                                  string
		arguments, argumentBytes, environment int
		unit                                  string
	}{
		{"seventeen large arguments", 18, 16000, 0, "x"},
		{"maximum argv and environment", 64, 16384, 64, "x"},
		{"quoted maximum argv and environment", 64, 16384, 64, "\x01"},
		{"HTML-escaped maximum argv and environment", 64, 16384, 64, "<"},
		{"multiline maximum argv and environment", 64, 16384, 64, "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, 0)
			team := h.newTeam(t, "Large runtime configuration", nil)
			value := strings.Repeat(tc.unit, tc.argumentBytes)
			argv := make([]string, tc.arguments)
			argv[0] = "fixture"
			for i := 1; i < len(argv); i++ {
				argv[i] = value
			}
			env := make(map[string]string, tc.environment)
			for i := range tc.environment {
				env[fmt.Sprintf("Env_%d", i)] = value
			}
			member := &team.Members[1]
			member.Runtime = &protocol.MemberRuntime{Kind: protocol.MemberRuntimeCommand, Command: argv, Env: env}
			member.Options = map[string]string{strings.Repeat("𐍈", protocol.EngineOptionIDMax): "enabled", "<<": "literal key", "Effort": "upper", "effort": "lower"}
			member.Instructions = strings.Repeat("i", memberconfig.MaxScalarBytes)
			denied := []string{}
			member.Skills = &denied
			team = configWireDo[protocol.Team](t, h, protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: team.ID, Patch: mustJSON(t, map[string]any{"members": team.Members})})
			ref := configRef(team, 1)
			original := configWireDo[protocol.MemberConfigDocument](t, h, protocol.MTeamsMemberConfigGet, ref)
			if len(original.YAML) <= 256<<10 || len(original.YAML) > memberconfig.MaxBytes {
				t.Fatalf("exported document has unexpected length %d", len(original.YAML))
			}
			saved := configWireDo[protocol.MemberConfigDocument](t, h, protocol.MTeamsMemberConfigSave, protocol.MemberConfigSaveParams{Team: ref.Team, Member: ref.Member, YAML: configTitle(original.YAML, "Large title edit"), ExpectedRevision: original.Revision})
			history := configWireDo[protocol.MemberConfigHistoryResult](t, h, protocol.MTeamsMemberConfigHistory, ref)
			if saved.Revision != original.Revision+1 || len(history.Revisions) < 2 {
				t.Fatal("YAML title save lost revision history")
			}
			restartHeartbeatHarness(t, h)
			if reopened := configWireDo[protocol.MemberConfigDocument](t, h, protocol.MTeamsMemberConfigGet, ref); reopened != saved {
				t.Fatal("large configuration changed after restart")
			}
			if reopened := configWireDo[protocol.MemberConfigHistoryResult](t, h, protocol.MTeamsMemberConfigHistory, ref); !reflect.DeepEqual(reopened, history) {
				t.Fatal("large configuration history changed after restart")
			}
			prior := configWireDo[protocol.MemberConfigDocument](t, h, protocol.MTeamsMemberConfigGet, protocol.MemberConfigRef{Team: ref.Team, Member: ref.Member, Revision: original.Revision})
			if prior.YAML != original.YAML || prior.CurrentRevision != saved.Revision {
				t.Fatal("large historical configuration did not survive")
			}
			live, _ := h.s.state.team(team.ID)
			got := live.Members[1]
			if !reflect.DeepEqual(got.Runtime, team.Members[1].Runtime) || !reflect.DeepEqual(got.Options, team.Members[1].Options) || got.Instructions != team.Members[1].Instructions || got.Title != "Large title edit" || got.Skills == nil || len(*got.Skills) != 0 {
				t.Fatal("large roundtrip altered canonical configuration")
			}
		})
	}
}

func TestMemberConfigOrdinaryFormRejectsUnexportableBounds(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Finite configuration", nil)
	ref := configRef(team, 1)
	original := configDocument(t, h, ref)
	for _, tc := range []struct {
		name   string
		change func(*protocol.Member)
	}{
		{"scalar", func(m *protocol.Member) {
			m.Instructions = strings.Repeat("x", memberconfig.MaxScalarBytes+1)
		}},
		{"heartbeat scalar", func(m *protocol.Member) {
			m.Heartbeat = &protocol.MemberHeartbeat{Instructions: strings.Repeat("x", memberconfig.MaxScalarBytes+1)}
		}},
		{"nodes", func(m *protocol.Member) {
			ids := make([]string, memberconfig.MaxNodes)
			for i := range ids {
				ids[i] = fmt.Sprintf("skill-%d", i)
			}
			m.Skills = &ids
		}},
		{"wire", func(m *protocol.Member) {
			ids := make([]string, 7800)
			for i := range ids {
				ids[i] = fmt.Sprintf("%04d%s", i, strings.Repeat("\x01", 196))
			}
			m.Skills = &ids
			value := strings.Repeat("\x01", 16384)
			m.Runtime = &protocol.MemberRuntime{Kind: protocol.MemberRuntimeCommand, Command: make([]string, 64), Env: make(map[string]string, 64)}
			m.Runtime.Command[0] = "fixture"
			for i := 1; i < len(m.Runtime.Command); i++ {
				m.Runtime.Command[i] = value
			}
			for i := range 64 {
				m.Runtime.Env[fmt.Sprintf("ENV_%d", i)] = value
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := cloneTeam(team)
			tc.change(&candidate.Members[1])
			if _, err := h.call(protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: team.ID, Patch: mustJSON(t, map[string]any{"members": candidate.Members})}); err == nil {
				t.Fatal("ordinary form accepted an unexportable configuration")
			}
			if after := configDocument(t, h, ref); after != original {
				t.Fatal("rejected ordinary edit changed configuration")
			}
		})
	}
}
