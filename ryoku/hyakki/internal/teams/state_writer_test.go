package teams

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/vault"
)

// Fail only the selected transaction directory, leaving actual task shutdown
// writes untouched. Calls run under the instance writer's mutex.
func transactionFaults(dir, mode string) stateFileIO {
	var syncs, moves int
	fault := errors.New("injected transaction IO failure")
	return stateFileIO{
		syncDir: func(file *os.File) error {
			if file.Name() == dir {
				syncs++
				if mode != "success" && syncs == 2 || mode == "rollback-sync" && syncs == 3 {
					return fault
				}
			}
			return file.Sync()
		},
		rename: func(from, to string) error {
			if filepath.Dir(to) == dir {
				moves++
				if mode == "rollback-fails" && moves == 2 {
					return fault
				}
			}
			return os.Rename(from, to)
		},
		remove: func(path string) error {
			if mode == "rollback-fails" && filepath.Dir(path) == dir {
				return fault
			}
			return os.Remove(path)
		},
	}
}

func requireUnavailable(t *testing.T, err error) {
	t.Helper()
	var methodError *protocol.Error
	if !errors.As(err, &methodError) || methodError.Code != protocol.CodeUnavailable {
		t.Fatalf("expected explicit unavailable, got %v", err)
	}
}

func installTransactionFaults(st *state, dir, mode string) {
	st.writer.mu.Lock()
	st.writer.io = transactionFaults(dir, mode)
	st.writer.mu.Unlock()
}

func TestStateTransactionStorageBoundary(t *testing.T) {
	for _, previous := range []bool{false, true} {
		for _, mode := range []string{"success", "rollback", "rollback-fails", "rollback-sync"} {
			name := "first-create/" + mode
			if previous {
				name = "replacement/" + mode
			}
			t.Run(name, func(t *testing.T) {
				dir := t.TempDir()
				before := stateFile{MemberHistoryVersion: 1, MemberHistories: map[string]memberHistory{}}
				after := before
				after.Routines = []protocol.Routine{{ID: "candidate-routine"}}
				oldRaw, err := json.Marshal(before)
				if err != nil {
					t.Fatal(err)
				}
				newRaw, err := json.Marshal(after)
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(dir, "state.json")
				if previous {
					if err := writeStateFile(dir, "state.json", oldRaw); err != nil {
						t.Fatal(err)
					}
				}
				writer := stateWriter{io: transactionFaults(dir, mode)}
				err = writer.write(dir, "state.json", newRaw)
				uncertain := mode == "rollback-fails" || mode == "rollback-sync"
				if mode == "success" {
					if err != nil {
						t.Fatal(err)
					}
				} else if uncertain {
					requireUnavailable(t, err)
				} else if err == nil {
					t.Fatal("refused commit reported success")
				}
				raw, readErr := os.ReadFile(path)
				switch {
				case mode == "success" || mode == "rollback-fails":
					if readErr != nil || !bytes.Equal(raw, newRaw) {
						t.Fatalf("candidate entry mismatch: %s, %v", raw, readErr)
					}
				case previous:
					if readErr != nil || !bytes.Equal(raw, oldRaw) {
						t.Fatalf("rollback entry mismatch: %s, %v", raw, readErr)
					}
				default:
					if !errors.Is(readErr, os.ErrNotExist) {
						t.Fatalf("first create rollback retained candidate: %s, %v", raw, readErr)
					}
				}
				_, markerErr := os.Stat(filepath.Join(dir, transactionPrefix+"state.json"))
				if uncertain {
					if markerErr != nil {
						t.Fatal("uncertain transaction lost its durable checkpoint", markerErr)
					}
					if previous {
						backup, err := os.ReadFile(filepath.Join(dir, ".hyakki-previous-state.json"))
						if err != nil || !bytes.Equal(backup, oldRaw) {
							t.Fatal("rollback consumed the retained previous inode", err)
						}
					}
					if err := writer.write(dir, "other.json", []byte("{}")); err == nil {
						t.Fatal("instance fence allowed another write")
					}
					for range 2 {
						if reopened, err := openState(dir); err == nil || reopened != nil {
							t.Fatal("restart treated uncertainty as repair")
						} else {
							requireUnavailable(t, err)
						}
					}
				} else {
					if !errors.Is(markerErr, os.ErrNotExist) {
						t.Fatal("settled transaction retained checkpoint", markerErr)
					}
					reopened, err := openState(dir)
					if err != nil {
						t.Fatal(err)
					}
					_, committed := reopened.routine("candidate-routine")
					if committed != (mode == "success") {
						t.Fatal("reopen resurrected a refused routine or lost a committed one")
					}
				}
			})
		}
	}
}

func TestConfigurationRollbackKeepsCanonicalAndHistoryTogether(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Known rollback", nil)
	ref := configRef(team, 0)
	before := configDocument(t, h, ref)
	history := do[protocol.MemberConfigHistoryResult](t, h, protocol.MTeamsMemberConfigHistory, ref)
	installTransactionFaults(h.s.state, h.dir, "rollback")
	_, err := h.call(protocol.MTeamsMemberConfigSave, protocol.MemberConfigSaveParams{Team: ref.Team, Member: ref.Member, YAML: configTitle(before.YAML, "Refused title"), ExpectedRevision: before.Revision})
	if err == nil {
		t.Fatal("post-rename fsync failure reported success")
	}
	if h.s.state.writer.available() != nil {
		t.Fatal("known durable rollback unnecessarily fenced the board")
	}
	if after := configDocument(t, h, ref); after != before {
		t.Fatal("rollback changed canonical configuration")
	}
	if after := do[protocol.MemberConfigHistoryResult](t, h, protocol.MTeamsMemberConfigHistory, ref); !reflect.DeepEqual(after, history) {
		t.Fatal("rollback changed history")
	}
	reopened, err := openState(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	member, snapshots, err := reopened.memberSnapshots(ref.Team, ref.Member)
	if err != nil || member.Revision != before.Revision || len(snapshots) != len(history.Revisions) {
		t.Fatal("reopen lost canonical/history rollback", err)
	}
}

func TestUncertainConfigurationFencesReadsGraphNoopAndExecution(t *testing.T) {
	for _, mode := range []string{"rollback-fails", "rollback-sync"} {
		t.Run(mode, func(t *testing.T) {
			h := newHarness(t, 4)
			team := h.newTeam(t, "Uncertain configuration", nil)
			other := h.newTeam(t, "Other running policy", nil)
			first := h.newTask(t, team, "Affected execution")
			_ = h.agent(t, first.ID)
			second := h.newTask(t, other, "Independent execution must also stop")
			secondAgent := h.agent(t, second.ID)
			info := h.sessions.info(secondAgent.Session)
			ref := configRef(team, 0)
			beforeMember, beforeHistory, err := h.s.state.memberSnapshots(ref.Team, ref.Member)
			if err != nil {
				t.Fatal(err)
			}
			members := cloneTeam(team).Members
			members[0].Tools = &protocol.TeamTools{}
			installTransactionFaults(h.s.state, h.dir, mode)
			_, err = h.call(protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: team.ID, Patch: mustJSON(t, map[string]any{"members": members})})
			requireUnavailable(t, err)
			if _, ok := h.s.state.team(team.ID); ok {
				t.Fatal("uncertain canonical lookup returned old policy as known")
			}
			if _, _, err := h.s.state.memberSnapshots(ref.Team, ref.Member); err == nil {
				t.Fatal("history asserted a canonical version under uncertainty")
			}
			// The retained in-memory preimage is diagnostic only, never authority.
			h.s.state.mu.Lock()
			if member, _ := protocol.MemberByID(h.s.state.teamSet[team.ID].Members, ref.Member); !reflect.DeepEqual(member, beforeMember) || !reflect.DeepEqual(h.s.state.memberHistories[team.ID][ref.Member], beforeHistory) {
				t.Error("canonical/history preimage diverged")
			}
			h.s.state.mu.Unlock()
			for _, method := range []string{protocol.MTeamsList, protocol.MTeamsMemberConfigGet, protocol.MTeamsMemberConfigHistory} {
				_, err := h.call(method, ref)
				requireUnavailable(t, err)
			}
			_, err = h.s.updateTeamConfig(context.Background(), protocol.TeamUpdateParams{ID: team.ID, Patch: json.RawMessage(`{}`)}, "")
			requireUnavailable(t, err)
			if _, err := h.s.createTask(context.Background(), protocol.TeamTaskCreateParams{Team: team.ID, Brief: "Denied"}); err == nil {
				t.Fatal("direct task creation bypassed storage fence")
			}
			if h.s.memberBudgetReady(other, other.Members[0]) || !h.s.Hold(info) || h.s.ToolContext(info) != nil || h.s.ScreenGranted(info) {
				t.Fatal("existing execution retained authority under uncertainty")
			}
			waitFor(t, "every active execution to stop", func() bool { return h.sessions.info(secondAgent.Session).State == protocol.StateStopped })
			h.s.mu.Lock()
			if h.s.editing[team.ID] == nil || !h.s.taskEdits[second.ID] || h.s.taskEditErrors[second.ID] == nil {
				t.Error("terminal execution fence was cleared")
			}
			h.s.mu.Unlock()
			snapshot := protocol.Snapshot{Teams: []protocol.Team{{ID: "unchanged-output"}}}
			original := snapshot
			requireUnavailable(t, h.s.Snapshot(context.Background(), &snapshot))
			if !reflect.DeepEqual(snapshot, original) {
				t.Fatal("refused snapshot partially published a stale/empty board")
			}
			h.sessions.mu.Lock()
			starts, continuations := len(h.sessions.made), len(h.sessions.continued)
			h.sessions.mu.Unlock()
			h.s.endTeamEdit(team.ID)
			h.s.pump()
			h.sessions.mu.Lock()
			if len(h.sessions.made) != starts || len(h.sessions.continued) != continuations {
				t.Error("fenced pump started or continued execution")
			}
			h.sessions.mu.Unlock()
			if err := h.s.Close(); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if reopened, err := openState(h.dir); err == nil || reopened != nil {
					t.Fatal("close/reopen cleared the durable execution fence")
				} else {
					requireUnavailable(t, err)
				}
			}
		})
	}
}

func TestSharedConsumersCannotClearStorageUncertainty(t *testing.T) {
	for _, consumer := range []string{"task", "routine-history", "heartbeat", "spend"} {
		t.Run(consumer, func(t *testing.T) {
			h := newHarness(t, 0)
			team := h.newTeam(t, "Shared storage", nil)
			task := memberBudgetTask(t, h, team, team.Members[0].ID, "")
			dir := h.dir
			if consumer == "task" {
				dir = filepath.Join(h.dir, "tasks")
			}
			if consumer == "heartbeat" {
				dir = filepath.Join(h.dir, "heartbeats")
			}
			installTransactionFaults(h.s.state, dir, "rollback-fails")
			var err error
			switch consumer {
			case "task":
				task.Title = "Refused task"
				err = h.s.state.putTask(task)
			case "routine-history":
				err = h.s.saveRoutineRun(protocol.RoutineRun{ID: "refused-run", Routine: "r1", Started: h.clock.now()})
			case "heartbeat":
				err = h.s.state.putHeartbeatSchedule(heartbeatSchedule{Team: team.ID, Member: team.Members[0].ID, Next: h.clock.now(), EveryMinutes: 1})
			case "spend":
				err = h.s.addSpend(team.ID, team.Members[0].ID, "", "", h.clock.now(), protocol.TokenUsage{Input: 1})
			}
			requireUnavailable(t, err)
			if h.s.memberBudgetReady(team, team.Members[0]) || h.s.routineAvailable(protocol.Routine{Team: protocol.RoutinePersonal, Kind: protocol.RoutineAsk}) {
				t.Fatal("shared writer failure left execution runnable")
			}
			requireUnavailable(t, h.s.state.putTeam(team))
			requireUnavailable(t, h.s.state.putTask(task))
			requireUnavailable(t, h.s.state.removeTask(task.ID))
			if err := h.s.Close(); err != nil {
				t.Fatal(err)
			}
			if reopened, err := openState(h.dir); err == nil || reopened != nil {
				t.Fatal("startup ignored a shared-consumer checkpoint")
			} else {
				requireUnavailable(t, err)
			}
		})
	}
}

func TestUncertainFirstCreateCannotBecomeCommittedOnRestart(t *testing.T) {
	for _, mode := range []string{"rollback-fails", "rollback-sync"} {
		t.Run(mode, func(t *testing.T) {
			h := newHarness(t, 0)
			installTransactionFaults(h.s.state, h.dir, mode)
			params := protocol.TeamCreateParams{Name: "Refused first policy", Folder: filepath.Join(h.dir, "first"), Members: []protocol.Member{{Name: "Root", Lead: true}}}
			_, err := h.call(protocol.MTeamsCreate, params)
			requireUnavailable(t, err)
			h.s.state.mu.Lock()
			if len(h.s.state.teamSet) != 0 || len(h.s.state.memberHistories) != 0 {
				t.Error("first-create refusal retained canonical/history memory")
			}
			h.s.state.mu.Unlock()
			_, err = h.call(protocol.MTeamsList, protocol.Empty{})
			requireUnavailable(t, err)
			if err := h.s.Close(); err != nil {
				t.Fatal(err)
			}
			v := vault.OpenWith(filepath.Join(t.TempDir(), "restart-vault.db"), fixedKeeper{key: bytes.Repeat([]byte{7}, 32)})
			service, err := open(h.dir, v, func(bool) {}, nil, h.currentSettings)
			if service != nil {
				_ = service.Close()
				t.Fatal("startup opened a service on an unresolved first policy")
			}
			requireUnavailable(t, err)
			for range 2 {
				if reopened, err := openState(h.dir); err == nil || reopened != nil {
					t.Fatal("refused first policy was resurrected by restart")
				} else {
					requireUnavailable(t, err)
				}
			}
		})
	}
}
