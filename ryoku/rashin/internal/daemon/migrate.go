package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/redact"
	store "ryoku-rashin/internal/session"
)

func (d *Daemon) runMigrations(ctx context.Context) error {
	if err := d.migrateProwl(ctx); err != nil {
		return err
	}
	return d.migrateAskHistory()
}

// The session scrub runs in two steps. scrubPatterns marks the pass that
// took out what the open stores hold and what the patterns find;
// scrubValues marks one that ran with every store open, the vault's key
// included, so every stored value is out too.
const (
	scrubPatterns = "scrub-sessions"
	scrubValues   = "scrub-sessions-values"
)

// scrubRounds is how many times a pass takes up a session that kept
// changing under it, scrubRetry how long it waits between, and scrubRearm
// how long a failed pass waits before it looks for the vault again.
var (
	scrubRounds = 3
	scrubRetry  = time.Second
	scrubRearm  = time.Minute
)

// errScrubStopped ends a pass the daemon stopped before it finished; it is
// owed again at the next start.
var errScrubStopped = errors.New("the daemon stopped before the sessions were scrubbed")

// migrations are the session scrub passes running in the background, so
// the daemon stops them before it closes what they use.
type migrations struct {
	mu      sync.Mutex
	stopped bool
	done    chan struct{}
	running sync.WaitGroup
}

// start runs pass in the background unless the daemon is stopping.
func (m *migrations) start(pass func()) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopped {
		return false
	}
	if m.done == nil {
		m.done = make(chan struct{})
	}
	m.running.Add(1)
	go func() {
		defer m.running.Done()
		pass()
	}()
	return true
}

// stop asks every pass to end after the session it is on and waits for it.
func (m *migrations) stop() {
	m.mu.Lock()
	if !m.stopped {
		m.stopped = true
		if m.done == nil {
			m.done = make(chan struct{})
		}
		close(m.done)
	}
	m.mu.Unlock()
	m.running.Wait()
}

// stopping reports whether the daemon asked the passes to end.
func (m *migrations) stopping() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopped
}

// wait waits for duration or until the daemon stops, and reports whether
// it waited the whole time.
func (m *migrations) wait(duration time.Duration) bool {
	m.mu.Lock()
	done := m.done
	m.mu.Unlock()
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-done:
		return false
	}
}

// scrubInBackground runs the session scrub after the socket listens, so a
// first start over many large sessions still answers at once.
func (d *Daemon) scrubInBackground() {
	d.migrations.start(func() {
		if err := d.migrateScrubSessions(); err != nil && !errors.Is(err, errScrubStopped) {
			d.warn("startup migration: %v", err)
		}
	})
}

// migrateScrubSessions takes the secrets out of the sessions written before
// each turn was scrubbed as it ended. With every store open one pass does
// it all. While the vault is locked the pass runs without its values, and
// the full pass waits for the first moment it opens while the daemon is
// up (awaitStoredValues).
func (d *Daemon) migrateScrubSessions() error {
	if !migrationPending(scrubValues) {
		return nil
	}
	values := redact.Snapshot()
	if values.Complete() || migrationPending(scrubPatterns) {
		if err := d.scrubSessions(values); err != nil {
			return err
		}
		if err := markMigrated(scrubPatterns); err != nil {
			return err
		}
		if values.Complete() {
			return markMigrated(scrubValues)
		}
	}
	d.awaitStoredValues()
	return nil
}

// awaitStoredValues runs the full scrub once every store opens. It looks
// again at each event the daemon broadcasts, as the vault opening and the
// work after it send some, and stops looking once it has started the pass.
// A pass that fails looks again a while later; one the daemon stopped is
// owed at the next start.
func (d *Daemon) awaitStoredValues() {
	remove := make(chan func(), 1)
	var once sync.Once
	remove <- d.events.add(func(string, any) {
		if !redact.Complete() {
			return
		}
		once.Do(func() {
			d.migrations.start(func() {
				(<-remove)()
				err := d.scrubOwedValues()
				if err == nil || errors.Is(err, errScrubStopped) {
					return
				}
				d.warn("startup migration: %v", err)
				if d.migrations.wait(scrubRearm) {
					d.awaitStoredValues()
				}
			})
		})
	})
}

// scrubOwedValues is the full pass awaitStoredValues starts. A vault that
// locked again before it began leaves the pass owed.
func (d *Daemon) scrubOwedValues() error {
	if !migrationPending(scrubValues) {
		return nil
	}
	values := redact.Snapshot()
	if !values.Complete() {
		return errors.New("the vault locked again before the sessions were scrubbed")
	}
	if err := d.scrubSessions(values); err != nil {
		return err
	}
	return markMigrated(scrubValues)
}

// scrubSessions scrubs every stored session with values, opened once for
// the pass. A session that is gone or cannot be read is left; one that
// changed under the pass is taken up again after the rest, a few times,
// before the pass gives up and stays owed.
func (d *Daemon) scrubSessions(values redact.Values) error {
	list, err := store.List("")
	if err != nil {
		return err
	}
	pending := make([]string, 0, len(list))
	for _, info := range list {
		pending = append(pending, info.ID)
	}
	for round := 0; len(pending) > 0; round++ {
		if round == scrubRounds {
			return fmt.Errorf("%d sessions kept changing while they were scrubbed", len(pending))
		}
		if round > 0 && !d.migrations.wait(scrubRetry) {
			return errScrubStopped
		}
		var again []string
		for _, id := range pending {
			if d.migrations.stopping() {
				return errScrubStopped
			}
			err := d.manager.scrubStored(id, values.Exact)
			switch {
			case err == nil, errors.Is(err, errSessionGone):
			case errors.Is(err, store.ErrMoved):
				again = append(again, id)
			case errors.Is(err, errUnreadableSession):
				d.warn("scrub session %s: %v", id, err)
			default:
				return err
			}
		}
		pending = again
	}
	return nil
}

// errUnreadableSession is a stored session whose log cannot be read;
// errSessionGone one deleted before the pass reached it or while it ran.
var (
	errUnreadableSession = errors.New("the session cannot be read")
	errSessionGone       = errors.New("the session is gone")
)

// scrubStored runs scrub over the whole of a stored session. A session that
// is loaded is scrubbed through its own log, its running turn left to the
// scrub at the turn's end. Any other is read and scrubbed beside its files
// holding nothing, and put in place under the manager's lock, so get
// cannot load it alongside, only if it is still not loaded and its files
// did not change; else it is store.ErrMoved, as is a loaded session closed
// under the pass, and the pass takes it up again.
func (m *sessionManager) scrubStored(id string, scrub func(string) string) error {
	m.mu.Lock()
	s := m.items[id]
	m.mu.Unlock()
	if s != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		_, err := s.log.ScrubAll(scrub)
		if errors.Is(err, os.ErrClosed) {
			if _, statErr := os.Stat(paths.SessionDir(id)); errors.Is(statErr, fs.ErrNotExist) {
				return errSessionGone
			}
			return store.ErrMoved
		}
		return err
	}
	prepared, err := store.PrepareScrub(id, scrub)
	if errors.Is(err, fs.ErrNotExist) {
		return errSessionGone
	}
	if err != nil {
		return fmt.Errorf("%w: %v", errUnreadableSession, err)
	}
	if prepared == nil {
		return nil
	}
	defer prepared.Discard()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.items[id] != nil {
		return store.ErrMoved
	}
	err = prepared.Commit()
	if errors.Is(err, fs.ErrNotExist) {
		return errSessionGone
	}
	return err
}

func migrationPending(name string) bool {
	_, err := os.Stat(paths.MigrationMarker(name))
	return errors.Is(err, os.ErrNotExist)
}

func markMigrated(name string) error {
	path := paths.MigrationMarker(name)
	if err := paths.EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(time.Now().Format(time.RFC3339)+"\n"), 0o600)
}

func (d *Daemon) migrateProwl(ctx context.Context) error {
	if !migrationPending("prowl-keys") {
		return nil
	}
	if _, err := os.Stat(paths.ProwlGatewayDir()); err == nil {
		if _, err := d.deps.Gateway.ImportProwl(ctx, paths.ProwlGatewayDir()); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return markMigrated("prowl-keys")
}

type legacyAsk struct {
	ID               string                `json:"id"`
	At               time.Time             `json:"at"`
	Kind             string                `json:"kind"`
	Question         string                `json:"q"`
	Answer           string                `json:"a"`
	Images           []string              `json:"images"`
	Actions          []protocol.Action     `json:"actions"`
	Plan             *protocol.CommandPlan `json:"plan"`
	Via              string                `json:"via"`
	NeedsAgent       bool                  `json:"needsAgent"`
	NeedsAgentReason string                `json:"needsAgentReason"`
}

func (d *Daemon) migrateAskHistory() error {
	if !migrationPending("ask-history") {
		return nil
	}
	f, err := os.Open(paths.LegacyAskHistory())
	if errors.Is(err, os.ErrNotExist) {
		return markMigrated("ask-history")
	}
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, protocol.MaxLine)
	for scanner.Scan() {
		var old legacyAsk
		if json.Unmarshal(scanner.Bytes(), &old) != nil || old.Question == "" {
			continue
		}
		created := old.At
		if created.IsZero() {
			created = time.Now()
		}
		id, err := store.NewID(protocol.KindAsk, created)
		if err != nil {
			return err
		}
		attachments := make([]protocol.Attachment, 0, len(old.Images))
		for _, path := range old.Images {
			attachments = append(attachments, protocol.Attachment{Path: path})
		}
		info := protocol.SessionInfo{ID: id, Kind: protocol.KindAsk, Title: old.Question, Origin: protocol.OriginImport, State: protocol.StateIdle, Created: created, Updated: created, Turns: 1}
		log, err := store.Create(info, nil)
		if err != nil {
			return err
		}
		if _, err = log.Append(protocol.EvUser, protocol.UserData{Text: old.Question, Images: attachments}); err == nil && old.Answer != "" {
			_, err = log.Append(protocol.EvText, protocol.TextData{Message: "import", Delta: old.Answer})
		}
		if err == nil {
			_, err = log.Append(protocol.EvAnswer, protocol.AnswerData{Actions: old.Actions, Plan: old.Plan, Via: old.Via, NeedsAgent: old.NeedsAgent, Reason: old.NeedsAgentReason})
		}
		if err == nil {
			_, err = log.Append(protocol.EvTurnEnd, protocol.TurnEndData{Turn: 1, Stop: protocol.StopEnd})
		}
		if err == nil {
			err = log.Update(func(i *protocol.SessionInfo) { i.Preview = lastRunes(old.Answer, 140) }, nil)
		}
		closeErr := log.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return markMigrated("ask-history")
}
