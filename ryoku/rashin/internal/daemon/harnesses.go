package daemon

import (
	"context"
	"encoding/json"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/harness"
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
// alone: it runs a program the user chose with the user's accounts. It runs
// in the team's own home for the harness, put back as it was signed in
// first, so Check says what that team's task would meet.
func (d *Daemon) probeHarness(ctx context.Context, c *connection, raw json.RawMessage) (any, error) {
	if err := requireApp(c); err != nil {
		return nil, err
	}
	p, err := unmarshal[protocol.HarnessProbeParams](raw)
	if err != nil {
		return nil, err
	}
	if p.Team != "" {
		if _, err := d.teamName(p.Team); err != nil {
			return nil, err
		}
		if err := d.restoreHome(p.ID, p.Team); err != nil {
			return protocol.HarnessProbeResult{OK: false, Message: err.Error()}, nil
		}
	}
	return d.deps.Harnesses.Probe(ctx, p, d.probeSandbox())
}

// teamName is the name of a team that exists, else a not-found error.
func (d *Daemon) teamName(team string) (string, error) {
	if d.deps.Teams == nil {
		return team, nil
	}
	name, ok := d.deps.Teams.TeamName(team)
	if !ok {
		return "", protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No team %s.", team))
	}
	return name, nil
}

// restoreHome puts a team's home for a harness back as it was signed in,
// through Teams when it runs so the user hears what moved.
func (d *Daemon) restoreHome(id, team string) error {
	if d.deps.Teams != nil {
		return d.deps.Teams.RestoreHome(id, team)
	}
	_, err := harness.Restore(id, team)
	return err
}

// openTeamsHome readies a sign-in or the setup in a team's home for a harness
// and says what the app runs in a terminal there; the app's alone. A home a
// task of that team works in now is refused, so nothing a run does lands in
// what the user signs in.
func (d *Daemon) openTeamsHome(c *connection, raw json.RawMessage) (any, error) {
	if err := requireApp(c); err != nil {
		return nil, err
	}
	p, err := unmarshal[protocol.HarnessTeamsOpenParams](raw)
	if err != nil {
		return nil, err
	}
	settings, err := config.Load()
	if err != nil {
		return nil, err
	}
	if !harness.Known(p.ID, settings) {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("No harness %s on this computer.", p.ID))
	}
	if p.Team == "" {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Name the team to sign in."))
	}
	teamName, err := d.teamName(p.Team)
	if err != nil {
		return nil, err
	}
	if p.CopyFrom != "" {
		if _, err := d.teamName(p.CopyFrom); err != nil {
			return nil, err
		}
	}
	name := harness.Name(p.ID, settings)
	if d.deps.Teams != nil && d.deps.Teams.Working(p.ID, p.Team) {
		return nil, protocol.Errorf(protocol.CodeConflict, "%s", i18n.Tf("A task of %s is working on %s. Let it finish or stop it, then try again.", teamName, name))
	}
	rashin := d.commandPath()
	if rashin == "" {
		return nil, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("The rashin command is not installed beside Rashin."))
	}
	if err := d.restoreHome(p.ID, p.Team); err != nil {
		return nil, err
	}
	opened, err := harness.OpenHome(p.ID, p.Team, settings, p.Setup, p.CopyFrom)
	if err != nil {
		return nil, err
	}
	d.broadcastHarnesses(context.Background())
	return protocol.HarnessTeamsOpenResult{Argv: opened.Argv(name, teamName, rashin), Home: opened.Home}, nil
}

// finishTeamsHome ends a sign-in or setup. Anything may call it, as the
// terminal's rashin command does; only the token Rashin handed that terminal
// ends one. A team now signed in is told to Teams, so a dream that waited on
// it goes on.
func (d *Daemon) finishTeamsHome(raw json.RawMessage) (any, error) {
	p, err := unmarshal[protocol.HarnessTeamsDoneParams](raw)
	if err != nil {
		return nil, err
	}
	id, team, signed, err := harness.FinishHome(p.Token, p.Status)
	if id != "" {
		d.broadcastHarnesses(context.Background())
	}
	if err != nil {
		return nil, err
	}
	settings, _ := config.Load()
	name := harness.Name(id, settings)
	teamName, _ := d.teamName(team)
	if !signed {
		return protocol.HarnessTeamsDoneResult{Text: i18n.Tf("%s is not signed in to %s: the sign-in ended with status %d. Sign in again from Rashin.", teamName, name, p.Status)}, nil
	}
	if d.deps.Teams != nil {
		d.deps.Teams.HomeSignedIn(id, team)
	}
	return protocol.HarnessTeamsDoneResult{SignedIn: true, Text: i18n.Tf("%s is signed in to %s. Rashin keeps this setup as it is now and puts it back before every task of the team.", teamName, name)}, nil
}
