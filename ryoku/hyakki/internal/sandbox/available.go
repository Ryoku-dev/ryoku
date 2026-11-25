package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"
)

// trialTimeout bounds the trial sandbox; a working bwrap exits in
// milliseconds.
const trialTimeout = 5 * time.Second

// ready remembers a trial that worked. A failed trial is not remembered, so
// installing bubblewrap takes effect without restarting the daemon.
var ready struct {
	sync.Mutex
	ok bool
}

// unavailable is why sandboxes cannot run here. Its text is for the user;
// the cause, kept for the daemon's log, is what bwrap or the lookup said.
type unavailable struct{ cause error }

func (e *unavailable) Error() string {
	return i18n.T("Teams run their agents in a sandbox, which needs bubblewrap. Install the bubblewrap package.")
}

func (e *unavailable) Unwrap() error { return e.cause }

// Available reports whether sandboxes can run here: bwrap on PATH and a trial
// run of the base sandbox works. Success is cached after the first one. The
// error's text is for the user; errors.Unwrap gives the cause for the log.
func Available() error {
	ready.Lock()
	defer ready.Unlock()
	if ready.ok {
		return nil
	}
	path, err := exec.LookPath("bwrap")
	if err != nil {
		return &unavailable{cause: err}
	}
	ctx, cancel := context.WithTimeout(context.Background(), trialTimeout)
	defer cancel()
	var stderr bytes.Buffer
	trial := exec.CommandContext(ctx, path, append(baseArgs(), "--", "true")...)
	trial.Stderr = &stderr
	if err := trial.Run(); err != nil {
		return &unavailable{cause: fmt.Errorf("sandbox: trial %s: %w: %s", path, err, strings.TrimSpace(stderr.String()))}
	}
	ready.ok = true
	return nil
}
