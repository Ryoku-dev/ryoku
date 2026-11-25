package teams

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// These fixtures keep the real handler, authorization and durable store. Only
// the owner policy transition or model-list boundary is controlled by the test.
func dreamGovernanceEdit(t *testing.T, h *harness, id string, edit func(*protocol.Team)) protocol.Team {
	t.Helper()
	h.s.mu.Lock()
	defer h.s.mu.Unlock()
	team, ok := h.s.state.team(id)
	if !ok {
		t.Fatal("missing Dream")
	}
	edit(&team)
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	saved, ok := h.s.state.team(id)
	if !ok {
		t.Fatal("saved Dream disappeared")
	}
	return saved
}

func dreamGovernanceStore(t *testing.T, h *harness) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(h.s.state.dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func dreamGovernanceRefusal(t *testing.T, h *harness, method string, params any, code string) {
	t.Helper()
	before := dreamGovernanceStore(t, h)
	tasks := h.s.state.tasks(nil)
	refused(t, h, method, params, code)
	if !bytes.Equal(before, dreamGovernanceStore(t, h)) || !reflect.DeepEqual(tasks, h.s.state.tasks(nil)) {
		t.Fatalf("refused %s changed durable Dream/task state", method)
	}
}

func dreamGovernanceTree(t *testing.T, h *harness, dream protocol.Team) protocol.Team {
	t.Helper()
	return dreamGovernanceEdit(t, h, dream.ID, func(team *protocol.Team) {
		team.Members = append(team.Members,
			protocol.Member{ID: "branch", Name: "Branch", Instructions: "Coordinate the specialists.", ReportsTo: dream.Dream.Manager, Rank: "duke", Revision: 7, Capabilities: "Review", Persona: "precise"},
			protocol.Member{ID: "leaf", Name: "Leaf", Instructions: "Verify.", ReportsTo: "branch", Rank: "king", Revision: 9, Spirit: "reviewer", Seat: "verification"},
		)
	})
}

func TestDreamGovernanceHirePreservesTreeAndNarrowsGrantsWithoutRuntimeInheritance(t *testing.T) {
	h := newHarness(t, 0)
	dream, ref := h.newDream(t)
	dreamGovernanceTree(t, h, dream)
	before := dreamGovernanceEdit(t, h, dream.ID, func(team *protocol.Team) {
		manager := &team.Members[0]
		manager.Rank = "knight"
		manager.Trust = protocol.ApprovalsAsk
		manager.Tools = &protocol.TeamTools{}
		skills := []string{"review"}
		manager.Skills = &skills
		manager.Runtime = &protocol.MemberRuntime{Kind: protocol.MemberRuntimeCLI, Command: []string{"local-agent"}, Model: "foreign-cli-model", Env: map[string]string{"PUBLIC": "value"}, SecretEnv: map[string]string{"TOKEN": "owner-vault-entry"}}
	})
	hired := do[protocol.Member](t, h, protocol.MTeamsAgentHire, protocol.AgentHireParams{AgentRef: ref, Name: "New worker", Instructions: "Build the missing component."})
	if hired.ReportsTo != dream.Dream.Manager || hired.Lead || hired.Revision != 1 || hired.Runtime != nil || hired.Model != "test/big" || hired.Options["effort"] != "high" || hired.Trust != protocol.ApprovalsAsk || hired.Tools == nil || hired.Tools.Screen || hired.Tools.Mail || hired.Skills == nil || !slices.Equal(*hired.Skills, []string{"review"}) {
		t.Fatalf("hire exceeded current manager grants or inherited its foreign runtime: %+v", hired)
	}
	current, _ := h.s.state.team(dream.ID)
	if err := protocol.ValidateReporting(current); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.Members, current.Members[:len(before.Members)]) || current.Dream.Hired != before.Dream.Hired+1 {
		t.Fatalf("hire rewrote existing seats or did not count the hire: %+v", current)
	}
	published, ok := h.events.lastTeam(dream.ID)
	if !ok || !reflect.DeepEqual(published.Members, current.Members) {
		t.Fatal("validated roster/revisions were not published")
	}
}

func TestDreamGovernanceReleaseNeverOrphansAndRetainsCompletedWork(t *testing.T) {
	h := newHarness(t, 0)
	dream, ref := h.newDream(t)
	dreamGovernanceTree(t, h, dream)
	dreamGovernanceRefusal(t, h, protocol.MTeamsAgentRelease, protocol.AgentReleaseParams{AgentRef: ref, Member: "branch"}, protocol.CodeConflict)
	now := h.clock.now()
	piece := protocol.TeamTask{ID: "governance-piece", Team: dream.ID, Member: "leaf", Parent: dream.Dream.Task, Goal: dream.Purpose, Title: "Completed review", Status: protocol.TaskDone, Created: now, Updated: now, Ended: &now, Sessions: []string{"historic-worker"}}
	if err := h.s.state.putTask(piece); err != nil {
		t.Fatal(err)
	}
	h.answer(t, protocol.MTeamsAgentRelease, protocol.AgentReleaseParams{AgentRef: ref, Member: "leaf"})
	if got := h.task(t, piece.ID); !reflect.DeepEqual(got, piece) {
		t.Fatalf("releasing a seat changed its task history/goal parentage: %+v", got)
	}
	h.answer(t, protocol.MTeamsAgentRelease, protocol.AgentReleaseParams{AgentRef: ref, Member: "branch"})
	current, _ := h.s.state.team(dream.ID)
	if len(current.Members) != 1 || current.Members[0].ID != dream.Dream.Manager {
		t.Fatalf("unexpected survivors: %+v", current.Members)
	}
	if err := protocol.ValidateReporting(current); err != nil {
		t.Fatal(err)
	}
}

func TestDreamGovernanceReleaseRejectsOpenWorkWithoutChangingState(t *testing.T) {
	h := newHarness(t, 0)
	dream, ref := h.newDream(t)
	dreamGovernanceTree(t, h, dream)
	now := h.clock.now()
	piece := protocol.TeamTask{ID: "open-governance-piece", Team: dream.ID, Member: "leaf", Parent: dream.Dream.Task, Goal: dream.Purpose, Title: "Still needed", Status: protocol.TaskReady, Created: now, Updated: now}
	if err := h.s.state.putTask(piece); err != nil {
		t.Fatal(err)
	}
	dreamGovernanceRefusal(t, h, protocol.MTeamsAgentRelease, protocol.AgentReleaseParams{AgentRef: ref, Member: "leaf"}, protocol.CodeConflict)
}

func TestDreamGovernanceHireRechecksLivePolicyAfterModelLookup(t *testing.T) {
	cases := []struct {
		name string
		code string
		edit func(*harness, protocol.Team, protocol.AgentRef)
	}{
		{"revoked token", protocol.CodeConflict, func(h *harness, d protocol.Team, _ protocol.AgentRef) {
			h.s.runs[d.Dream.Task].Token = "replacement-token"
		}},
		{"replaced session", protocol.CodeConflict, func(h *harness, d protocol.Team, _ protocol.AgentRef) {
			h.s.runs[d.Dream.Task].Session = "replacement-session"
		}},
		{"task edit barrier", protocol.CodeConflict, func(h *harness, d protocol.Team, _ protocol.AgentRef) { h.s.taskEdits[d.Dream.Task] = true }},
		{"member edit barrier", protocol.CodeConflict, func(h *harness, d protocol.Team, _ protocol.AgentRef) {
			if h.s.editing[d.ID] == nil {
				h.s.editing[d.ID] = make(map[string]bool)
			}
			h.s.editing[d.ID][d.Dream.Manager] = true
		}},
		{"reassigned task", protocol.CodeConflict, func(h *harness, d protocol.Team, _ protocol.AgentRef) {
			task, _ := h.s.state.task(d.Dream.Task)
			task.Member = "branch"
			if err := h.s.state.putTask(task); err != nil {
				t.Fatal(err)
			}
		}},
		{"paused team", protocol.CodeConflict, func(h *harness, d protocol.Team, _ protocol.AgentRef) {
			d.Paused = true
			if err := h.s.state.putTeam(d); err != nil {
				t.Fatal(err)
			}
		}},
		{"paused member", protocol.CodeConflict, func(h *harness, d protocol.Team, _ protocol.AgentRef) {
			d.Members[0].Paused = true
			if err := h.s.state.putTeam(d); err != nil {
				t.Fatal(err)
			}
		}},
		{"read-only team", protocol.CodeForbidden, func(h *harness, d protocol.Team, _ protocol.AgentRef) {
			d.Approvals = protocol.ApprovalsReadOnly
			if err := h.s.state.putTeam(d); err != nil {
				t.Fatal(err)
			}
		}},
		{"read-only manager", protocol.CodeForbidden, func(h *harness, d protocol.Team, _ protocol.AgentRef) {
			d.Members[0].Trust = protocol.ApprovalsReadOnly
			if err := h.s.state.putTeam(d); err != nil {
				t.Fatal(err)
			}
		}},
		{"pending strategy", protocol.CodeForbidden, func(h *harness, d protocol.Team, _ protocol.AgentRef) {
			d.PlanApproval = true
			if err := h.s.state.putTeam(d); err != nil {
				t.Fatal(err)
			}
		}},
		{"broken tree", protocol.CodeConflict, func(h *harness, d protocol.Team, _ protocol.AgentRef) {
			d.Members[2].ReportsTo = "missing-member"
			if err := h.s.state.putTeam(d); err != nil {
				t.Fatal(err)
			}
		}},
		{"manager model changed", protocol.CodeConflict, func(h *harness, d protocol.Team, _ protocol.AgentRef) {
			d.Members[0].Model = "test/small"
			if err := h.s.state.putTeam(d); err != nil {
				t.Fatal(err)
			}
		}},
		{"root moved despite rank", protocol.CodeForbidden, func(h *harness, d protocol.Team, _ protocol.AgentRef) {
			d.Members[0].Lead, d.Members[0].Rank, d.Members[0].ReportsTo = false, "king", "branch"
			d.Members[1].Lead, d.Members[1].ReportsTo = true, ""
			d.Dream.Manager = "branch"
			if err := h.s.state.putTeam(d); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, 0)
			dream, ref := h.newDream(t)
			dream = dreamGovernanceTree(t, h, dream)
			var committed []byte
			var tasks []protocol.TeamTask
			h.s.SetModels(func(ctx context.Context, p protocol.ModelsParams) (protocol.ModelsResult, error) {
				h.s.mu.Lock()
				func() {
					defer h.s.mu.Unlock()
					tc.edit(h, dream, ref)
				}()
				committed = dreamGovernanceStore(t, h)
				tasks = h.s.state.tasks(nil)
				return listModels(ctx, p)
			})
			refused(t, h, protocol.MTeamsAgentHire, protocol.AgentHireParams{AgentRef: ref, Name: "Stale hire", Instructions: "Must not run."}, tc.code)
			if committed == nil || !bytes.Equal(committed, dreamGovernanceStore(t, h)) || !reflect.DeepEqual(tasks, h.s.state.tasks(nil)) {
				t.Fatal("late refusal changed the owner's current durable state")
			}
		})
	}
}

func TestDreamGovernancePaidPinIsCurrentAtHireCommit(t *testing.T) {
	h := newHarness(t, 0)
	dream, ref := h.newDream(t)
	h.settingsMu.Lock()
	h.settings.Agent.Model = "test/paid"
	h.settingsMu.Unlock()
	before := dreamGovernanceStore(t, h)
	h.s.SetModels(func(ctx context.Context, p protocol.ModelsParams) (protocol.ModelsResult, error) {
		h.settingsMu.Lock()
		h.settings.Agent.Model = "test/small"
		h.settingsMu.Unlock()
		out, err := listModels(ctx, p)
		out.Models = append(out.Models, protocol.ModelInfo{ID: "test/paid", Provider: "test", Tier: protocol.TierPaid, Tools: true, Available: true})
		return out, err
	})
	refused(t, h, protocol.MTeamsAgentHire, protocol.AgentHireParams{AgentRef: ref, Name: "Paid hire", Instructions: "Build.", Model: "test/paid"}, protocol.CodeForbidden)
	current, _ := h.s.state.team(dream.ID)
	if !bytes.Equal(before, dreamGovernanceStore(t, h)) || len(current.Members) != 1 || current.Dream.Hired != 0 {
		t.Fatal("revoked paid pin still changed the roster")
	}
}

func TestDreamGovernanceAdministrativeHandlersRespectReadOnlyAndStrategy(t *testing.T) {
	for _, policy := range []string{"team", "member", "strategy"} {
		t.Run(policy, func(t *testing.T) {
			h := newHarness(t, 0)
			dream, ref := h.newDream(t)
			dreamGovernanceTree(t, h, dream)
			dreamGovernanceEdit(t, h, dream.ID, func(d *protocol.Team) {
				d.Members[0].Rank = "king"
				switch policy {
				case "team":
					d.Approvals = protocol.ApprovalsReadOnly
				case "member":
					d.Members[0].Trust = protocol.ApprovalsReadOnly
				case "strategy":
					d.PlanApproval = true
				}
			})
			dreamGovernanceRefusal(t, h, protocol.MTeamsAgentHire, protocol.AgentHireParams{AgentRef: ref, Name: "Refused", Instructions: "No."}, protocol.CodeForbidden)
			dreamGovernanceRefusal(t, h, protocol.MTeamsAgentRelease, protocol.AgentReleaseParams{AgentRef: ref, Member: "leaf"}, protocol.CodeForbidden)
			dreamGovernanceRefusal(t, h, protocol.MTeamsAgentDream, protocol.AgentDreamParams{AgentRef: ref, Phase: protocol.DreamBuilding, Summary: "Must not change."}, protocol.CodeForbidden)
		})
	}
}

func TestDreamGovernanceRankDoesNotAuthorizeAnyManagerHandler(t *testing.T) {
	h := newHarness(t, 0)
	dream, _ := h.newDream(t)
	dreamGovernanceTree(t, h, dream)
	now := h.clock.now()
	task := protocol.TeamTask{ID: "ranked-governance-task", Team: dream.ID, Member: "leaf", Parent: dream.Dream.Task, Goal: dream.Purpose, Session: "ranked-session", Sessions: []string{"ranked-session"}, Title: "Review", Status: protocol.TaskWorking, Created: now, Updated: now, Started: &now}
	if err := h.s.state.putTask(task); err != nil {
		t.Fatal(err)
	}
	ref := protocol.AgentRef{Session: task.Session, Token: "ranked-token"}
	h.s.mu.Lock()
	h.s.runs[task.ID] = &run{Session: task.Session, Task: task.ID, Team: dream.ID, Member: "leaf", Token: ref.Token, Started: now}
	h.s.mu.Unlock()
	dreamGovernanceRefusal(t, h, protocol.MTeamsAgentHire, protocol.AgentHireParams{AgentRef: ref, Name: "Forbidden", Instructions: "No."}, protocol.CodeForbidden)
	dreamGovernanceRefusal(t, h, protocol.MTeamsAgentRelease, protocol.AgentReleaseParams{AgentRef: ref, Member: "branch"}, protocol.CodeForbidden)
	dreamGovernanceRefusal(t, h, protocol.MTeamsAgentDream, protocol.AgentDreamParams{AgentRef: ref, Phase: protocol.DreamBuilding, Summary: "No."}, protocol.CodeForbidden)
	dreamGovernanceRefusal(t, h, protocol.MTeamsAgentPresent, protocol.AgentPresentParams{AgentRef: ref, Title: "No", Body: "No."}, protocol.CodeForbidden)
}

func TestDreamGovernancePresentationRechecksRevocationAtCommit(t *testing.T) {
	h := newHarness(t, 0)
	dream, ref := h.newDream(t)
	h.readyToPresent(t, dream, ref)
	before := dreamGovernanceStore(t, h)
	h.clock.beforeNextRead(func() {
		h.s.mu.Lock()
		h.s.runs[dream.Dream.Task].Token = "revoked-before-presentation"
		h.s.mu.Unlock()
	})
	_, err := h.call(protocol.MTeamsAgentPresent, protocol.AgentPresentParams{AgentRef: ref, Title: "Stale presentation", Body: "Must not commit."})
	var pe *protocol.Error
	if !errors.As(err, &pe) || pe.Code != protocol.CodeConflict {
		t.Fatalf("revoked presentation returned %v", err)
	}
	if !bytes.Equal(before, dreamGovernanceStore(t, h)) {
		t.Fatal("revoked manager committed a presentation or approval")
	}
}

func TestDreamGovernanceReadOnlyPresentationDoesNotGrantWriteAuthority(t *testing.T) {
	for _, pendingStrategy := range []bool{false, true} {
		t.Run(map[bool]string{false: "read-only", true: "pending-strategy"}[pendingStrategy], func(t *testing.T) {
			h := newHarness(t, 0)
			dream, ref := h.newDream(t)
			h.readyToPresent(t, dream, ref)
			dreamGovernanceEdit(t, h, dream.ID, func(d *protocol.Team) {
				d.Members[0].Trust = protocol.ApprovalsReadOnly
				d.PlanApproval = pendingStrategy
			})
			h.answer(t, protocol.MTeamsAgentPresent, protocol.AgentPresentParams{AgentRef: ref, Title: "Evidence for the Summoner", Body: "Review this result; permissions remain unchanged."})
			current, _ := h.s.state.team(dream.ID)
			if current.Members[0].Trust != protocol.ApprovalsReadOnly || current.PlanApproval != pendingStrategy || current.Dream.Phase != protocol.DreamPresenting {
				t.Fatalf("presentation widened governance: %+v", current)
			}
			dreamGovernanceRefusal(t, h, protocol.MTeamsAgentHire, protocol.AgentHireParams{AgentRef: ref, Name: "Not authorized", Instructions: "No."}, protocol.CodeForbidden)
		})
	}
}

func TestDreamGovernanceEveryManagerHandlerRejectsRevokedOrInvalidAuthority(t *testing.T) {
	for _, policy := range []string{"revoked", "invalid-tree", "former-root", "done"} {
		t.Run(policy, func(t *testing.T) {
			h := newHarness(t, 0)
			dream, ref := h.newDream(t)
			dreamGovernanceTree(t, h, dream)
			code := protocol.CodeConflict
			if policy == "revoked" {
				h.s.mu.Lock()
				h.s.taskEdits[dream.Dream.Task] = true
				h.s.mu.Unlock()
			} else {
				dreamGovernanceEdit(t, h, dream.ID, func(d *protocol.Team) {
					switch policy {
					case "invalid-tree":
						d.Members[1].ReportsTo = "leaf"
					case "former-root":
						d.Members[0].Lead, d.Members[0].ReportsTo, d.Members[0].Rank = false, "branch", "king"
						d.Members[1].Lead, d.Members[1].ReportsTo = true, ""
						d.Dream.Manager = "branch"
						code = protocol.CodeForbidden
					case "done":
						now := d.Updated
						d.Dream.Done = &now
					}
				})
			}
			dreamGovernanceRefusal(t, h, protocol.MTeamsAgentHire, protocol.AgentHireParams{AgentRef: ref, Name: "Rejected", Instructions: "No."}, code)
			dreamGovernanceRefusal(t, h, protocol.MTeamsAgentRelease, protocol.AgentReleaseParams{AgentRef: ref, Member: "leaf"}, code)
			dreamGovernanceRefusal(t, h, protocol.MTeamsAgentDream, protocol.AgentDreamParams{AgentRef: ref, Phase: protocol.DreamBuilding, Summary: "No."}, code)
			dreamGovernanceRefusal(t, h, protocol.MTeamsAgentPresent, protocol.AgentPresentParams{AgentRef: ref, Title: "No", Body: "No."}, code)
		})
	}
}
