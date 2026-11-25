package teams

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/vault"
)

func TestManagedDirectoryLinksRefuseWritesAndRestart(t *testing.T) {
	for _, linked := range []string{".", "tasks", "heartbeats", "heartbeats/linked"} {
		t.Run(linked, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "board")
			st, err := openState(root)
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(root, linked)
			// Include a real descendant below a linked intermediate directory;
			// checking only the final directory's Lstat would miss this case.
			if linked == "heartbeats/linked" {
				dir = filepath.Join(dir, "real-child")
			}
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			before := []byte("owner's canonical bytes\n")
			if err := st.writer.write(dir, "policy.json", before); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(root, linked)
			real := filepath.Join(t.TempDir(), "retained-directory")
			if err := os.Rename(link, real); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(real, link); err != nil {
				t.Fatal(err)
			}
			err = st.writer.write(dir, "policy.json", []byte("must not replace owner data"))
			requireUnavailable(t, err)
			if !strings.Contains(err.Error(), "symbolic link") || !strings.Contains(err.Error(), link) {
				t.Fatalf("link refusal did not identify the repair path: %v", err)
			}
			if raw, err := os.ReadFile(filepath.Join(dir, "policy.json")); err != nil || !bytes.Equal(raw, before) {
				t.Fatalf("rejected write changed linked owner data: %q, %v", raw, err)
			}
			for range 2 {
				reopened, err := openState(root)
				requireUnavailable(t, err)
				if reopened != nil {
					t.Fatal("restart bypassed the managed-directory link policy")
				}
			}
			if target, err := os.Readlink(link); err != nil || target != real {
				t.Fatalf("refusal changed the owner's link: %q, %v", target, err)
			}
		})
	}
}

func TestLinkedDirectoryCannotHideUncertainTransactionOnRestart(t *testing.T) {
	for _, linked := range []string{".", "tasks", "heartbeats", "heartbeats/linked"} {
		t.Run(linked, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "board")
			st, err := openState(root)
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(root, linked)
			before := []byte("retained preimage\n")
			if err := st.writer.write(dir, "policy.json", before); err != nil {
				t.Fatal(err)
			}
			installTransactionFaults(st, dir, "rollback-sync")
			requireUnavailable(t, st.writer.write(dir, "policy.json", []byte("uncertain candidate\n")))
			real := filepath.Join(t.TempDir(), "retained-directory")
			if err := os.Rename(dir, real); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(real, dir); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				reopened, err := openState(root)
				requireUnavailable(t, err)
				if reopened != nil {
					t.Fatal("restart skipped a checkpoint behind a managed-directory link")
				}
			}
			if raw, err := os.ReadFile(filepath.Join(real, ".hyakki-previous-policy.json")); err != nil || !bytes.Equal(raw, before) {
				t.Fatalf("restart changed retained preimage: %q, %v", raw, err)
			}
			if _, err := os.Stat(filepath.Join(real, transactionPrefix+"policy.json")); err != nil {
				t.Fatal("restart lost the durable checkpoint", err)
			}
		})
	}
}

func TestFencedLedgerAppendPreservesHardlinkedPreimage(t *testing.T) {
	dir := t.TempDir()
	seed := bytes.Repeat([]byte("{\"action\":\"retained\"}\n"), ledgerKeep+ledgerSlack)
	path := filepath.Join(dir, "ledger.jsonl")
	if err := os.WriteFile(path, seed, 0o600); err != nil {
		t.Fatal(err)
	}
	ledger, err := openLedger(dir)
	if err != nil {
		t.Fatal(err)
	}
	ledger.writer.io = transactionFaults(dir, "rollback-sync")
	entry := protocol.Activity{Action: "trigger-compaction"}
	line, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	before := append(bytes.Clone(seed), append(line, '\n')...)
	requireUnavailable(t, ledger.append(entry))
	backup := filepath.Join(dir, ".hyakki-previous-ledger.jsonl")
	canonicalInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	backupInfo, err := os.Stat(backup)
	if err != nil || !os.SameFile(canonicalInfo, backupInfo) {
		t.Fatal("fault did not leave canonical and retained preimage hardlinked", err)
	}
	lines := ledger.lines
	for range 2 {
		requireUnavailable(t, ledger.append(protocol.Activity{Action: "shutdown-must-not-append"}))
		for _, file := range []string{path, backup} {
			raw, err := os.ReadFile(file)
			if err != nil || !bytes.Equal(raw, before) {
				t.Fatalf("fenced append changed %s: %v", file, err)
			}
		}
		if ledger.lines != lines {
			t.Fatal("rejected append changed the in-memory ledger count")
		}
	}
	if reopened, err := openLedger(dir); reopened != nil || err == nil {
		t.Fatal("ledger startup ignored retained transaction evidence")
	} else {
		requireUnavailable(t, err)
	}
}

type scheduledPromptSessions struct {
	*fakeSessions
	prompt func(context.Context, protocol.PromptParams) (protocol.PromptResult, error)
}

func (f *scheduledPromptSessions) Prompt(ctx context.Context, p protocol.PromptParams) (protocol.PromptResult, error) {
	return f.prompt(ctx, p)
}

// Open without Bind: these admission regressions drive the actual routine
// consumer and transaction writer, without unrelated clock or D-Bus workers.
func openScheduledAdmissionService(t *testing.T) *Service {
	t.Helper()
	settings := config.Defaults()
	settings.Features = map[string]bool{"schedules": true}
	settings.Teams.QuietHours = &protocol.QuietHours{From: "00:00", To: "23:59"}
	v := vault.OpenWith(filepath.Join(t.TempDir(), "vault.db"), fixedKeeper{key: bytes.Repeat([]byte{7}, 32)})
	s, err := open(t.TempDir(), v, func(bool) {}, nil, func() protocol.Settings { return settings })
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return time.Date(2026, time.November, 11, 12, 0, 0, 0, time.UTC) }
	return s
}

func TestScheduledQuestionLaunchCannotOutliveStorageRevocation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := openScheduledAdmissionService(t)
		defer s.Close()
		f := &scheduledPromptSessions{fakeSessions: newFakeSessions()}
		s.sessions = f
		entered, resume := make(chan string, 1), make(chan struct{})
		f.prompt = func(_ context.Context, p protocol.PromptParams) (protocol.PromptResult, error) {
			entered <- p.ID
			<-resume
			// Match idle Ask's ability to create a new runner even if Stop
			// already visited the idle session before Prompt's launch.
			f.mu.Lock()
			info := f.infos[p.ID]
			info.State = protocol.StateWorking
			f.infos[p.ID] = info
			f.mu.Unlock()
			// Prompt can synchronously invoke ordinary Teams callbacks. They
			// need service, history and writer locks without deadlocking.
			s.RoutineSessionEnded(info, protocol.TurnEndData{})
			s.record(protocol.Activity{Action: "synchronous-prompt-callback"})
			return protocol.PromptResult{}, nil
		}
		if err := s.state.writer.write(s.state.dir, "policy.json", []byte("before")); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() {
			_, err := s.fireScheduled(context.Background(), protocol.Routine{ID: "scheduled-question", Kind: protocol.RoutineAsk, Team: protocol.RoutinePersonal, Brief: "Answer later"}, protocol.InitiatorYou)
			done <- err
		}()
		session := <-entered
		installTransactionFaults(s.state, s.state.dir, "rollback-sync")
		requireUnavailable(t, s.state.writer.write(s.state.dir, "policy.json", []byte("uncertain")))
		close(resume)
		s.workers.Wait()
		requireUnavailable(t, <-done)
		if info := f.info(session); info.State != protocol.StateStopped || !f.wasStopped(session) {
			t.Fatalf("scheduled Ask survived storage revocation: %+v", info)
		}
		f.mu.Lock()
		starts := len(f.made)
		f.mu.Unlock()
		_, err := s.fireScheduled(context.Background(), protocol.Routine{ID: "later", Kind: protocol.RoutineAsk, Brief: "Must not launch"}, protocol.InitiatorYou)
		requireUnavailable(t, err)
		f.mu.Lock()
		if len(f.made) != starts {
			t.Error("a revoked board admitted another Ask")
		}
		f.mu.Unlock()
	})
}

func TestScheduledQuestionPromptAllowsSynchronousCompletion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := openScheduledAdmissionService(t)
		defer s.Close()
		f := &scheduledPromptSessions{fakeSessions: newFakeSessions()}
		s.sessions = f
		f.prompt = func(_ context.Context, p protocol.PromptParams) (protocol.PromptResult, error) {
			f.mu.Lock()
			info := f.infos[p.ID]
			info.State, info.Turns, info.Preview = protocol.StateIdle, 1, "Completed synchronously"
			f.infos[p.ID] = info
			f.mu.Unlock()
			s.RoutineSessionEnded(info, protocol.TurnEndData{Stop: protocol.StopEnd})
			s.record(protocol.Activity{Action: "completion-callback"})
			return protocol.PromptResult{}, nil
		}
		var task protocol.TeamTask
		var err error
		go func() {
			task, err = s.fireScheduled(context.Background(), protocol.Routine{ID: "immediate", Kind: protocol.RoutineAsk, Team: protocol.RoutinePersonal, Brief: "Answer now"}, protocol.InitiatorYou)
		}()
		synctest.Wait()
		if err != nil || task.Session == "" {
			t.Fatalf("callback-capable Prompt could not complete: %+v, %v", task, err)
		}
		runs, err := s.readRoutineRuns()
		if err != nil || len(runs) != 1 || runs[0].Ended == nil || runs[0].Status != protocol.TaskDone {
			t.Fatalf("synchronous completion lost its terminal history: %+v, %v", runs, err)
		}
	})
}
