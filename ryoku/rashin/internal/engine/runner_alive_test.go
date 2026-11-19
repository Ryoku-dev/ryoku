package engine

import (
	"testing"

	"ryoku-rashin/internal/engine/exec"
)

type idleToolHost struct{ exec.Executor }

func (idleToolHost) Alive() bool       { return true }
func (idleToolHost) Running() []string { return nil }

func TestRunnerAliveCountsIdleToolHost(t *testing.T) {
	r := &runner{executor: idleToolHost{}}
	if !r.Alive() {
		t.Fatal("a tool host with no running terminal process was treated as idle")
	}
}
