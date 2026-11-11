package daemon

import (
	"context"
	"encoding/json"
	"time"

	"ryoku-rashin/internal/protocol"
)

// learnedDelay gathers the harnesses that report their capabilities close
// together, as a team starting several tasks does, into one
// harnesses.changed.
const learnedDelay = time.Second

// harnessesLearned answers the registry recording a harness's capabilities:
// the app hears the new model lists within learnedDelay, without a refresh.
func (d *Daemon) harnessesLearned() {
	d.learnedMu.Lock()
	defer d.learnedMu.Unlock()
	if d.learned != nil {
		return
	}
	d.learned = time.AfterFunc(learnedDelay, func() {
		d.learnedMu.Lock()
		d.learned = nil
		d.learnedMu.Unlock()
		d.broadcastHarnesses(context.Background())
	})
}

func (d *Daemon) broadcastHarnesses(ctx context.Context) {
	list, err := d.deps.Harnesses.List(ctx, false)
	if err != nil {
		d.warn("list harnesses: %v", err)
		return
	}
	d.Broadcast(protocol.EvtHarnessesChanged, protocol.HarnessesResult{Harnesses: list})
}

// probeHarness starts a harness once in a team agent's sandbox, for the app
// alone: it runs a program the user chose with the user's accounts.
func (d *Daemon) probeHarness(ctx context.Context, c *connection, raw json.RawMessage) (any, error) {
	if err := requireApp(c); err != nil {
		return nil, err
	}
	p, err := unmarshal[protocol.HarnessProbeParams](raw)
	if err != nil {
		return nil, err
	}
	return d.deps.Harnesses.Probe(ctx, p, d.probeSandbox())
}
