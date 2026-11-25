package teamsui

import (
	"fmt"
	"slices"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// a proposed team is read in full before it is made, since creating it grants what it may do and starts its asking
func (c *NeedCards) proposalField(gtx layout.Context, e *env.Env, snapshot state.Snapshot, card *needCard, need Need, gold bool) []layout.FlexChild {
	th := e.Theme
	approval := *need.Approval
	proposal := *approval.Proposal
	team := normalizedProposal(proposal.Team)
	card.approve.Kind, card.deny.Kind = kit.PlaqueTonal, kit.PlaqueQuiet
	if gold {
		card.approve.Kind = kit.PlaqueGold
	}
	card.approve.Small, card.deny.Small = true, true
	card.approve.Label, card.deny.Label = i18n.T("Create team"), i18n.T("Not now")
	if card.approve.Clicked(gtx) {
		c.decide(e, need, protocol.DecideOnce, "", "", i18n.Tf("Created the team %s", team.Name))
	}
	if card.deny.Clicked(gtx) {
		c.decide(e, need, protocol.DecideDeny, "", "", i18n.Tf("Not now: %s", team.Name))
	}
	if card.proposer.Clicked(gtx) {
		if info, ok := snapshot.Sessions[approval.Session]; ok {
			e.Navigate(sessionTab(info), info.ID)
		}
	}

	gap := layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout)
	children := []layout.FlexChild{
		layout.Rigid(claimRow(th, palette.Attention, ApprovalWord(approval.Kind), look.Ago(gtx.Now, approval.Created))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(look.Paragraph(th, th.T.Title, th.R.Fg, team.Name)),
	}
	if team.Purpose != "" {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg2, team.Purpose)),
		)
	}
	children = append(children,
		layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		layout.Rigid(proposalPowers(th, snapshot, team)),
		gap,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutProposer(gtx, th, snapshot, card, approval.Session, proposal.Reason)
		}),
		gap,
		layout.Rigid(proposalPart(th, i18n.T("Members"), proposalMembers(th, snapshot, team))),
	)
	if len(team.Routines) > 0 {
		children = append(children, gap, layout.Rigid(proposalPart(th, i18n.T("Routines"), c.proposalRoutines(th, team))))
	}
	if len(proposal.Credentials) > 0 {
		children = append(children, gap, layout.Rigid(proposalPart(th, i18n.T("It will ask for"), proposalCredentials(th, proposal.Credentials))))
	}
	children = append(children,
		layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			plaques := [...]*kit.Plaque{&card.approve, &card.deny}
			return look.Flow{Gap: 8, LineGap: 8}.Layout(gtx, len(plaques), func(gtx layout.Context, index int) layout.Dimensions {
				return plaques[index].Layout(gtx, th.Theme)
			})
		}),
	)
	return children
}

// where a session lives in Hyakki mode
func sessionTab(info protocol.SessionInfo) string {
	if info.Kind == protocol.KindAsk {
		return protocol.TabAsk
	}
	return protocol.TabAgent
}

// what the team could do, set apart by space: its permissions, the one that stops the asking in amber, then its tools
func proposalPowers(th *look.Theme, snapshot state.Snapshot, team protocol.TeamCreateParams) layout.Widget {
	approvals := team.Approvals
	if !protocol.KnownApprovals(approvals) {
		approvals = snapshot.Settings.Teams.Defaults.Approvals
	}
	ink := th.R.Fg2
	if approvals == protocol.ApprovalsAuto {
		ink = th.R.Stone(palette.Attention)
	}
	words := []layout.Widget{kit.Text(th.Theme, th.T.Detail, ink, ApprovalsWord(approvals))}
	if team.Tools.Screen {
		words = append(words, kit.Text(th.Theme, th.T.Detail, th.R.Fg2, i18n.T("Uses its own screen")))
	}
	if team.Tools.Mail {
		words = append(words, kit.Text(th.Theme, th.T.Detail, th.R.Fg2, i18n.T("Sends mail once you approve it")))
	}
	return func(gtx layout.Context) layout.Dimensions {
		return look.Flow{Gap: 12, LineGap: 4}.Layout(gtx, len(words), func(gtx layout.Context, index int) layout.Dimensions {
			return words[index](gtx)
		})
	}
}

// the reason in the proposer's words, then the conversation it came from, which opens where it lives
func layoutProposer(gtx layout.Context, th *look.Theme, snapshot state.Snapshot, card *needCard, session, reason string) layout.Dimensions {
	from := layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, i18n.T("Proposed with the hyakki command")))
	if info, ok := snapshot.Sessions[session]; ok {
		from = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, i18n.T("Proposed in"))),
				layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return look.Link(gtx, th, &card.proposer, info.Title) }),
			)
		})
	} else if session != "" {
		from = layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, i18n.T("Proposed by an agent")))
	}
	children := []layout.FlexChild{}
	if reason != "" {
		children = append(children,
			layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg, reason)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, append(children, from)...)
}

// a part of the proposal under its name
func proposalPart(th *look.Theme, title string, rows []layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, title)),
		}
		for _, row := range rows {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout), layout.Rigid(row))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}

// a name in a column of its own, so the facts after it line up row under row
func proposalRow(th *look.Theme, name layout.Widget, facts string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				width := min(gtx.Dp(160), gtx.Constraints.Max.X/2)
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
				return name(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
			layout.Flexed(1, look.Paragraph(th, th.T.Detail, th.R.Fg2, facts)),
		)
	}
}

// Proposed seats keep their reporting relationships before the team is created.
func proposalMembers(th *look.Theme, snapshot state.Snapshot, team protocol.TeamCreateParams) []layout.Widget {
	proposed := protocol.Team{Members: team.Members, Reporting: team.Reporting}
	rows := make([]layout.Widget, 0, len(team.Members))
	for _, branch := range crewHierarchy(proposed) {
		member := team.Members[branch.Member]
		facts := i18n.Tf("%s, %s", reportingLine(proposed, member), MemberEngine(snapshot, member).Line())
		rows = append(rows, proposalRow(th, look.Paragraph(th, th.T.Label, th.R.Fg, member.Name), facts))
	}
	return rows
}

// each routine with its schedule and who runs it; a routine made off waits for the user to turn it on
func (c *NeedCards) proposalRoutines(th *look.Theme, team protocol.TeamCreateParams) []layout.Widget {
	rows := make([]layout.Widget, 0, len(team.Routines))
	for _, routine := range team.Routines {
		facts := i18n.Tf("%s, run by %s", c.schedules.Sentence(protocol.Routine{Schedule: routine.Schedule}), proposalRunner(team, routine.Member))
		if !routine.Enabled {
			facts = i18n.Tf("%s, off until you turn it on", facts)
		}
		rows = append(rows, proposalRow(th, look.Paragraph(th, th.T.Label, th.R.Fg, routine.Title), facts))
	}
	return rows
}

// A routine names a seat by id or name, or uses the explicit default.
func proposalRunner(team protocol.TeamCreateParams, member string) string {
	team = normalizedProposal(team)
	if member != "" {
		for _, candidate := range team.Members {
			if candidate.ID == member || strings.EqualFold(candidate.Name, member) {
				return candidate.Name
			}
		}
	} else if lead, at := Lead(protocol.Team{Members: team.Members, Reporting: team.Reporting}); at >= 0 {
		return lead.Name
	}
	return i18n.T("Choose a member")
}

func normalizedProposal(team protocol.TeamCreateParams) protocol.TeamCreateParams {
	if team.Reporting != "" || len(team.Members) == 0 {
		return team
	}
	team.Reporting = protocol.ReportingTree
	team.Members = slices.Clone(team.Members)
	occupied := make(map[string]bool, len(team.Members)*2)
	for _, member := range team.Members {
		occupied[strings.TrimSpace(member.ID)] = true
		occupied[strings.TrimSpace(member.ReportsTo)] = true
	}
	lead := 0
	for index, member := range team.Members {
		if member.Lead {
			lead = index
			break
		}
	}
	for index := range team.Members {
		member := &team.Members[index]
		member.ID = strings.TrimSpace(member.ID)
		if member.ID == "" {
			id := fmt.Sprintf("proposal-seat-%d", index)
			for occupied[id] {
				id += "-"
			}
			member.ID, occupied[id] = id, true
		}
		member.Name, member.Title = strings.TrimSpace(member.Name), strings.TrimSpace(member.Title)
		member.Lead = index == lead
	}
	for index := range team.Members {
		member := &team.Members[index]
		member.ReportsTo = strings.TrimSpace(member.ReportsTo)
		if !member.Lead && member.ReportsTo == "" {
			member.ReportsTo = team.Members[lead].ID
		}
	}
	return team
}

// each credential by its literal name and hosts, with the one sentence on why
func proposalCredentials(th *look.Theme, needs []protocol.CredentialNeed) []layout.Widget {
	rows := make([]layout.Widget, 0, len(needs))
	for _, need := range needs {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
						layout.Rigid(kit.Text(th.Theme, th.T.Mono, th.R.Fg, need.Name)),
						layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
						layout.Flexed(1, look.Paragraph(th, th.T.Mono, th.R.Fg2, strings.Join(need.Hosts, " "))),
					)
				}),
				layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, need.Why)),
			)
		})
	}
	return rows
}
