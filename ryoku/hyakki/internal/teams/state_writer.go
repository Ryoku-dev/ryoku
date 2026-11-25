package teams

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

const transactionPrefix = ".hyakki-transaction-"

// stateFileIO is instance-scoped. Tests inject failures at actual filesystem
// operations; file contents, hard links and directory entries remain real.
type stateFileIO struct {
	syncDir func(*os.File) error
	rename  func(string, string) error
	remove  func(string) error
}

func (io stateFileIO) sync(directory *os.File) error {
	if io.syncDir != nil {
		return io.syncDir(directory)
	}
	return directory.Sync()
}
func (io stateFileIO) move(from, to string) error {
	if io.rename != nil {
		return io.rename(from, to)
	}
	return os.Rename(from, to)
}
func (io stateFileIO) unlink(name string) error {
	if io.remove != nil {
		return io.remove(name)
	}
	return os.Remove(name)
}

// stateWriter fences all consumers of a board, including tasks, spending,
// messages, heartbeats and routine history, not merely state.json. Its mutex is
// independent of state.mu and spendMu. The callback never runs under its lock.
type stateWriter struct {
	mu        sync.Mutex
	root      string
	io        stateFileIO
	failure   error
	onFailure func()
}

func storageUnavailable() error {
	return protocol.Errorf(protocol.CodeUnavailable, "Team storage has an unresolved transaction. Stop the daemon and reconcile the checkpoint and retained previous file before restarting; restart is not repair.")
}

func (w *stateWriter) available() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.failure != nil {
		return storageUnavailable()
	}
	return nil
}

func (w *stateWriter) write(dir, name string, data []byte) error {
	return w.mutate(dir, func() error { return writeStateFileIO(dir, name, data, w.io) })
}

// mutate orders in-place mutations with transaction replacement and revocation.
// In particular, an append must never touch a retained hard-linked preimage.
func (w *stateWriter) mutate(dir string, change func() error) error {
	w.mu.Lock()
	if w.failure != nil {
		w.mu.Unlock()
		return storageUnavailable()
	}
	err := refuseLinkedStoragePath(w.root, dir)
	if err == nil {
		err = change()
	}
	var uncertain *uncertainStateWrite
	if errors.As(err, &uncertain) {
		w.failure = err
		notify := w.onFailure
		w.mu.Unlock()
		if notify != nil {
			notify()
		}
		return storageUnavailable()
	}
	w.mu.Unlock()
	return err
}

// Refuse unresolved transactions before reading any canonical state, including
// files whose ordinary reader otherwise tolerates damage or missing contents.
// There is deliberately no automatic repair: neither valid JSON nor a restart
// establishes which policy an interrupted/refused transaction committed.
func refusePendingTransactions(dir string) error {
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := refuseLinkedStorageEntry(path, entry.Type()); err != nil {
			return err
		}
		if strings.HasPrefix(entry.Name(), transactionPrefix) {
			return storageUnavailable()
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// WalkDir does not follow directory links, whereas ordinary file operations do.
// Refuse them rather than silently scanning a different board from the writer.
// Owner data and links are left untouched for explicit relocation/reconciliation.
func refuseLinkedStorageEntry(path string, mode fs.FileMode) error {
	if mode&fs.ModeSymlink == 0 {
		return nil
	}
	info, err := os.Stat(path)
	if err == nil && !info.IsDir() {
		return nil
	}
	return protocol.Errorf(protocol.CodeUnavailable, "Team storage directory %q is a symbolic link or has an unresolved link target. Stop the daemon and relocate it to a real directory before restarting; existing data and links have not been changed.", path)
}

// Check the managed root and every directory used by this mutation. Startup
// checks all descendants; checking only the target here would miss a linked
// root (or linked intermediate directory) followed by ordinary filesystem IO.
func refuseLinkedStoragePath(root, dir string) error {
	if root == "" {
		root = dir
	}
	root, dir = filepath.Clean(root), filepath.Clean(dir)
	for path := dir; ; path = filepath.Dir(path) {
		info, err := os.Lstat(path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err == nil {
			if err := refuseLinkedStorageEntry(path, info.Mode()); err != nil {
				return err
			}
		}
		if path == root {
			return nil
		}
		if filepath.Dir(path) == path {
			return errors.New("team storage mutation is outside its managed root")
		}
	}
}

// uncertainStateWrite means rollback or its durability could not be established.
// The durable checkpoint remains; no consumer may assert either version won.
type uncertainStateWrite struct{ error }

func (e *uncertainStateWrite) Error() string {
	return "Storage transaction requires explicit reconciliation: " + e.error.Error()
}
func (e *uncertainStateWrite) Unwrap() error { return e.error }

func writeStateFile(dir, name string, data []byte) error {
	return writeStateFileIO(dir, name, data, stateFileIO{})
}

// A checkpoint and previous inode become durable BEFORE replacement. On a
// refused commit, rollback is durable before returning an ordinary error. A
// failed rollback leaves both the checkpoint and preimage intact, even when
// rollback's rename succeeded but its fsync failed. Startup then fails closed.
func writeStateFileIO(dir, name string, data []byte, io stateFileIO) error {
	if err := paths.EnsureDir(dir); err != nil {
		return err
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	checkpoint := filepath.Join(dir, transactionPrefix+name)
	if _, err := os.Lstat(checkpoint); err == nil {
		return &uncertainStateWrite{storageUnavailable()}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+name+"-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	keep := false
	defer func() {
		_ = tmp.Close()
		if !keep {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	destination := filepath.Join(dir, name)
	backup := filepath.Join(dir, ".hyakki-previous-"+name)
	// A leftover preimage without a checkpoint belongs to an already settled
	// transaction. The canonical entry was synced before checkpoint cleanup.
	if err := os.Remove(backup); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	hadPrevious := true
	if err := os.Link(destination, backup); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		hadPrevious = false
	}
	journal, err := os.OpenFile(checkpoint, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(struct {
		Destination string `json:"destination"`
		Candidate   string `json:"candidate"`
		Previous    bool   `json:"previous"`
	}{name, filepath.Base(tmpName), hadPrevious})
	if err == nil {
		_, err = journal.Write(append(raw, '\n'))
	}
	if err == nil {
		err = journal.Sync()
	}
	err = errors.Join(err, journal.Close())
	if err == nil {
		err = io.sync(directory)
	}
	if err != nil {
		// Replacement has not happened: canonical authority is still known.
		// Only enter the uncertainty protocol after this checkpoint is durable.
		if os.Remove(checkpoint) == nil {
			_ = directory.Sync()
			_ = os.Remove(backup)
		}
		return err
	}
	settled := func() {
		// Cleanup cannot turn a known durable commit/rollback into a refused
		// one. A cleanup failure can only leave a conservative startup fence.
		if os.Remove(checkpoint) != nil {
			return
		}
		if directory.Sync() != nil {
			return
		}
		_ = os.Remove(backup)
	}
	if err := io.move(tmpName, destination); err != nil {
		settled()
		return err
	}
	if err := io.sync(directory); err != nil {
		var rollback error
		if hadPrevious {
			// Link, do not consume the only retained preimage on rollback.
			rollback = os.Link(backup, tmpName)
			if rollback == nil {
				rollback = io.move(tmpName, destination)
			}
		} else {
			rollback = io.unlink(destination)
		}
		if rollback == nil {
			rollback = io.sync(directory)
		}
		if rollback != nil {
			keep = true
			return &uncertainStateWrite{errors.Join(err, rollback)}
		}
		settled()
		return err
	}
	settled()
	return nil
}

// Fence revokes existing run capabilities without persisting guessed outcomes.
// Session stops run outside service/state locks so final-turn callbacks drain.
func (s *Service) fenceStorage() {
	s.scheduledAskMu.Lock()
	defer s.scheduledAskMu.Unlock()
	s.mu.Lock()
	sessions := make(map[string]bool)
	failure := storageUnavailable()
	for id, run := range s.runs {
		s.taskEdits[id], s.taskEditErrors[id] = true, failure
		if run.timer != nil {
			run.timer.Stop()
			run.timer = nil
		}
		if run.Session != "" {
			sessions[run.Session] = true
		}
	}
	s.mu.Unlock()
	s.routineRunsMu.Lock()
	for session := range s.scheduledQuestions {
		sessions[session] = true
	}
	s.routineRunsMu.Unlock()
	if s.clock != nil {
		s.clock.stop()
	}
	s.state.disarmExpiry()
	for session := range sessions {
		s.halt(session)
	}
}
