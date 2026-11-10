package projects

import (
	appstate "ryoku-rashin/internal/app/state"
)

// narrow enough to stand in for in a test
type projectEnvironment interface {
	Snapshot() appstate.Snapshot
	LoadWorktrees(string, func(error))
	LoadTasks(string, func(error))
	WatchProject(string)
	UnwatchProject(string)
	OnGitChanged(string, func()) func()
}

type projectLoad struct {
	attemptEpoch int
	successEpoch int
	attempted    bool
	succeeded    bool
	inFlight     bool
}

func (p *pageState) ensureProjectData(e projectEnvironment, project string) {
	if e.Snapshot().Connection != appstate.Ready {
		return
	}
	p.requestProjectData(e, project)
}

func (p *pageState) requestProjectData(e projectEnvironment, project string) {
	p.requestLoad(e, project, p.worktreeLoads, false, e.LoadWorktrees)
	p.requestLoad(e, project, p.taskLoads, false, e.LoadTasks)
}

// once per daemon connection, a new connection is a new daemon state so everything is asked for again
func (p *pageState) requestLoad(
	e projectEnvironment,
	project string,
	loads map[string]*projectLoad,
	force bool,
	loadFn func(string, func(error)),
) {
	epoch := e.Snapshot().Epoch
	load := loads[project]
	if load == nil {
		load = new(projectLoad)
		loads[project] = load
	}
	if load.inFlight {
		return
	}
	if !force {
		if load.succeeded && load.successEpoch == epoch {
			return
		}
		if load.attempted && load.attemptEpoch == epoch {
			return
		}
	}
	load.inFlight, load.attempted, load.attemptEpoch = true, true, epoch
	loadFn(project, func(err error) {
		load.inFlight = false
		if err != nil {
			p.fail(err)
		} else {
			load.succeeded, load.successEpoch = true, epoch
		}
		snapshot := e.Snapshot()
		if snapshot.Connection == appstate.Ready && snapshot.Epoch != epoch {
			p.requestLoad(e, project, loads, false, loadFn)
		}
	})
}

// a load that failed on this connection can be tried again when the project is opened once more
func (p *pageState) allowFailedRetry(loads map[string]*projectLoad, project string, epoch int) {
	load := loads[project]
	if load == nil || load.inFlight {
		return
	}
	if load.succeeded && load.successEpoch == epoch {
		return
	}
	if load.attemptEpoch == epoch {
		load.attempted = false
	}
}
