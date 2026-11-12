package teamsui

import (
	"slices"
	"strings"
	"time"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// the harness a member's tasks start on, through the chain the daemon starts them with
func MemberHarness(snapshot state.Snapshot, member protocol.Member) (protocol.HarnessInfo, bool) {
	id, _, _ := protocol.ResolveEngine(member, snapshot.Settings, registryDefault(snapshot.Harnesses))
	return env.HarnessByID(snapshot.Harnesses, id)
}

// the installed harnesses a team's members start on, by name; the team signs in to each once
func TeamHarnesses(snapshot state.Snapshot, team protocol.Team) []protocol.HarnessInfo {
	var harnesses []protocol.HarnessInfo
	for _, member := range team.Members {
		harness, ok := MemberHarness(snapshot, member)
		taken := slices.ContainsFunc(harnesses, func(each protocol.HarnessInfo) bool { return each.ID == harness.ID })
		if ok && harness.Installed && harness.Teams != nil && !taken {
			harnesses = append(harnesses, harness)
		}
	}
	slices.SortFunc(harnesses, func(a, b protocol.HarnessInfo) int { return strings.Compare(a.Name, b.Name) })
	return harnesses
}

// the installed harnesses any team starts on, by name
func TeamsHarnesses(snapshot state.Snapshot) []protocol.HarnessInfo {
	var harnesses []protocol.HarnessInfo
	for _, team := range snapshot.Teams {
		for _, harness := range TeamHarnesses(snapshot, team) {
			if !slices.ContainsFunc(harnesses, func(each protocol.HarnessInfo) bool { return each.ID == harness.ID }) {
				harnesses = append(harnesses, harness)
			}
		}
	}
	slices.SortFunc(harnesses, func(a, b protocol.HarnessInfo) int { return strings.Compare(a.Name, b.Name) })
	return harnesses
}

// the teams that start the harness, by name
func HarnessTeams(snapshot state.Snapshot, harness string) []protocol.Team {
	var teams []protocol.Team
	for _, team := range snapshot.Teams {
		if slices.ContainsFunc(TeamHarnesses(snapshot, team), func(each protocol.HarnessInfo) bool { return each.ID == harness }) {
			teams = append(teams, team)
		}
	}
	slices.SortFunc(teams, func(a, b protocol.Team) int { return strings.Compare(a.Name, b.Name) })
	return teams
}

func SignedIn(harness protocol.HarnessInfo, team string) bool {
	home, ok := harness.Teams.HomeOf(team)
	return ok && home.SignedIn != nil
}

// a team task on it ends blocked until the user signs it in for that team
func NotSignedIn(harness protocol.HarnessInfo, team string) bool {
	return harness.Installed && harness.Teams != nil && !SignedIn(harness, team)
}

// a harness Rashin knows a sign-in for opens it in the team's home, any other a shell there; done may be nil
func SignInForTeams(e *env.Env, harness, team string, done func(error)) {
	info, _ := env.HarnessByID(e.Snapshot().Harnesses, harness)
	e.OpenTeamsHome(harness, team, info.Teams == nil || len(info.Teams.Login) == 0, "", done)
}

// "Signed in today 14:02", "Signing in…" while its terminal is open, else "Not signed in"
func SignInState(now time.Time, harness protocol.HarnessInfo, team string) string {
	home, _ := harness.Teams.HomeOf(team)
	switch {
	case home.Opening:
		return i18n.T("Signing in…")
	case home.SignedIn != nil:
		return i18n.Tf("Signed in %s", look.When(now, *home.SignedIn))
	}
	return i18n.T("Not signed in")
}

// the other team signed in to the harness most lately, whose config a setup may start from
func copySource(snapshot state.Snapshot, harness protocol.HarnessInfo, team string) (protocol.Team, bool) {
	var source protocol.Team
	var at time.Time
	for _, home := range harness.Teams.Homes {
		if home.Team == team || home.SignedIn == nil || !home.SignedIn.After(at) {
			continue
		}
		if other, ok := TeamByID(snapshot.Teams, home.Team); ok {
			source, at = other, *home.SignedIn
		}
	}
	return source, !at.IsZero()
}

// the sign-in, the setup and a setup that starts from another team's config, for one team's home of one harness
type SignInActions struct {
	signIn, setup, copy kit.Plaque
}

// one home as a line: what it is called here and its state, then its actions under it
func (a *SignInActions) Row(gtx layout.Context, e *env.Env, harness protocol.HarnessInfo, team protocol.Team, label string, now time.Time) layout.Dimensions {
	th := e.Theme
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(kit.Text(th.Theme, th.T.Label, th.R.Fg, label)),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Flexed(1, look.Paragraph(th, th.T.Detail, th.R.Fg2, SignInState(now, harness, team.ID))),
			)
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.Layout(gtx, e, harness, team) }),
	)
}

func (a *SignInActions) Layout(gtx layout.Context, e *env.Env, harness protocol.HarnessInfo, team protocol.Team) layout.Dimensions {
	th := e.Theme
	home, _ := harness.Teams.HomeOf(team.ID)
	login := len(harness.Teams.Login) > 0
	source, copyable := copySource(e.Snapshot(), harness, team.ID)
	a.signIn.Kind, a.signIn.Small, a.signIn.Label = kit.PlaqueTonal, true, i18n.T("Sign in")
	a.setup.Kind, a.setup.Small, a.setup.Label = kit.PlaqueTonal, true, i18n.T("Open setup")
	a.copy.Kind, a.copy.Small, a.copy.Label = kit.PlaqueTonal, true, i18n.Tf("Open setup with %s's config", source.Name)
	open := func(plaque *kit.Plaque, setup bool, from string) {
		plaque.Busy = true
		e.OpenTeamsHome(harness.ID, team.ID, setup, from, func(error) { plaque.Busy = false })
	}
	if a.signIn.Clicked(gtx) && login && !home.Opening {
		open(&a.signIn, false, "")
	}
	if a.setup.Clicked(gtx) && !home.Opening {
		open(&a.setup, true, "")
	}
	if a.copy.Clicked(gtx) && copyable && !home.Opening {
		open(&a.copy, true, source.ID)
	}
	type action struct {
		plaque *kit.Plaque
		name   string
	}
	var actions []action
	if login {
		actions = append(actions, action{&a.signIn, i18n.Tf("Sign %s in for %s", harness.Name, team.Name)})
	}
	actions = append(actions, action{&a.setup, i18n.Tf("Open the setup of %s for %s", harness.Name, team.Name)})
	if copyable {
		actions = append(actions, action{&a.copy, i18n.Tf("Open the setup of %s for %s with %s's config", harness.Name, team.Name, source.Name)})
	}
	return look.Flow{Gap: 8, LineGap: 8}.Layout(gtx, len(actions), func(gtx layout.Context, index int) layout.Dimensions {
		// one terminal at a time in a team home
		if home.Opening {
			gtx = gtx.Disabled()
		}
		return look.Named(gtx, actions[index].name, func(gtx layout.Context) layout.Dimensions {
			return actions[index].plaque.Layout(gtx, th.Theme)
		})
	})
}
