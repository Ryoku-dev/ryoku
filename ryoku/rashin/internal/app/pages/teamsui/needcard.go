package teamsui

import (
	"image"
	"sort"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// a detail longer than this unfolds on request
const detailLines = 8

// NeedCards answer what needs the user where it shows: an approval, a
// credential request, a budget stop or a blocked task, each on an amber field
// like a permission request. An answered card folds at once to one line that
// keeps the answer for the rest of the session; a refusal by the daemon
// unfolds it again and says why.
type NeedCards struct {
	// Task is the task page the cards stand on, whose own link they leave out.
	Task string

	cards   map[string]*needCard
	answers map[string]answer
}

type needCard struct {
	field, record look.Unfold

	approve, always, deny, open kit.Plaque
	note, value                 kit.TextField
	more                        kit.Plaque
	team, task                  kit.Click
	// the whole detail is shown
	all bool
}

type answer struct {
	line string
	need Need
}

// NeedKey names a need: its approval, or its task when the task itself waits.
func NeedKey(need Need) string {
	if need.Approval != nil {
		return need.Approval.ID
	}
	return need.Task.ID
}

// Shown returns the needs given and the ones answered here, oldest first so an
// answer folds where its card stood. A card whose need went, answered elsewhere
// or expired, goes with whatever was typed into it.
func (c *NeedCards) Shown(needs []Need) []Need {
	shown := make([]Need, 0, len(needs)+len(c.answers))
	listed := make(map[string]bool, len(needs)+len(c.answers))
	for _, need := range needs {
		key := NeedKey(need)
		listed[key] = true
		shown = append(shown, need)
	}
	for key, answered := range c.answers {
		// on a task page only that task's answers fold in; the Inbox keeps them all
		if !listed[key] && (c.Task == "" || answered.need.taskID() == c.Task) {
			listed[key] = true
			shown = append(shown, answered.need)
		}
	}
	for key, card := range c.cards {
		if !listed[key] {
			card.value.SetText("")
			delete(c.cards, key)
		}
	}
	sort.SliceStable(shown, func(i, j int) bool { return shown[i].At.Before(shown[j].At) })
	return shown
}

// Open counts the needs still waiting for an answer among the shown ones.
func (c *NeedCards) Open(shown []Need) int {
	count := 0
	for _, need := range shown {
		if _, done := c.answers[NeedKey(need)]; !done {
			count++
		}
	}
	return count
}

func (c *NeedCards) card(key string) *needCard {
	if c.cards == nil {
		c.cards = make(map[string]*needCard)
	}
	card := c.cards[key]
	if card == nil {
		card = new(needCard)
		card.value.Secret, card.value.Mono = true, true
		c.cards[key] = card
	}
	return card
}

// Layout draws one need's card: the decision while it is open, one line of
// record once it was answered here.
func (c *NeedCards) Layout(gtx layout.Context, e *env.Env, need Need) layout.Dimensions {
	th := e.Theme
	key := NeedKey(need)
	card := c.card(key)
	answered, done := c.answers[key]
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return card.field.Layout(gtx, th, !done, func(gtx layout.Context) layout.Dimensions {
				return c.layoutField(gtx, e, card, need)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return card.record.Layout(gtx, th, done, func(gtx layout.Context) layout.Dimensions {
				return layoutRecord(gtx, th, answered.line)
			})
		}),
	)
}

// the amber field a need stands on, like a permission request in a conversation
func (c *NeedCards) layoutField(gtx layout.Context, e *env.Env, card *needCard, need Need) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	var children []layout.FlexChild
	if need.Approval != nil {
		children = c.approvalField(gtx, e, snapshot, card, need)
	} else {
		children = c.blockedField(gtx, e, snapshot, card, need)
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	record := op.Record(gtx.Ops)
	dimensions := layout.UniformInset(16).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
	call := record.Stop()
	kit.FillCut(gtx.Ops, image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Mid), th.R.Field(palette.Attention))
	call.Add(gtx.Ops)
	return dimensions
}

func (c *NeedCards) approvalField(gtx layout.Context, e *env.Env, snapshot state.Snapshot, card *needCard, need Need) []layout.FlexChild {
	th := e.Theme
	approval := *need.Approval
	request := approval.Kind == protocol.ApprovalRequest
	vaultOpen := snapshot.Vault.State == protocol.VaultOpen
	note := strings.TrimSpace(card.note.Text())
	value := card.value.Text()
	plaques := []*kit.Plaque{&card.approve}
	card.approve.Kind, card.deny.Kind, card.always.Kind = kit.PlaqueGold, kit.PlaqueQuiet, kit.PlaqueLine
	card.approve.Small, card.deny.Small, card.always.Small = true, true, true
	card.deny.Label = i18n.T("Deny")
	switch approval.Kind {
	case protocol.ApprovalBudget:
		card.approve.Label, card.deny.Label = i18n.T("Raise the limit by half"), i18n.T("Keep it paused")
	case protocol.ApprovalRequest:
		card.approve.Label = i18n.T("Add to the vault")
	default:
		card.approve.Label = i18n.T("Approve")
		if approval.Always != "" {
			card.always.Label = approval.Always
			plaques = append(plaques, &card.always)
		}
	}
	plaques = append(plaques, &card.deny)
	// a value goes only into an open vault
	ready := !request || (vaultOpen && strings.TrimSpace(value) != "")
	if card.approve.Clicked(gtx) && ready {
		switch approval.Kind {
		case protocol.ApprovalBudget:
			c.decide(e, need, protocol.DecideOnce, "", "", i18n.Tf("Raised the limit: %s", approval.Title))
		case protocol.ApprovalRequest:
			// the value goes to the vault once and never stays in the field
			card.value.SetText("")
			c.decide(e, need, protocol.DecideProvide, note, value, i18n.Tf("Added to the vault: %s", approval.Title))
		default:
			c.decide(e, need, protocol.DecideOnce, note, "", i18n.Tf("Approved: %s", approval.Title))
		}
	}
	if approval.Always != "" && card.always.Clicked(gtx) {
		c.decide(e, need, protocol.DecideAlways, note, "", i18n.Tf("Allowed from now on: %s", approval.Always))
	}
	if card.deny.Clicked(gtx) {
		card.value.SetText("")
		line := i18n.Tf("Denied: %s", approval.Title)
		if approval.Kind == protocol.ApprovalBudget {
			line = i18n.Tf("Kept paused: %s", approval.Title)
		}
		c.decide(e, need, protocol.DecideDeny, note, "", line)
	}

	children := []layout.FlexChild{
		layout.Rigid(claimRow(th, ApprovalWord(approval.Kind), look.Ago(gtx.Now, approval.Created))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(look.Paragraph(th, th.T.Title, th.R.Fg, approval.Title)),
	}
	if approval.Detail != "" {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layoutDetail(gtx, th, card, approval.Detail) }),
		)
	}
	children = append(children,
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return c.layoutWhere(gtx, e, snapshot, card, approval.Team, approval.Member, approval.Task)
		}),
	)
	if request {
		card.value.Label = i18n.T("Paste the value")
		card.value.Help = i18n.T("Stored once and never shown again.")
		if !vaultOpen {
			card.value.Help = VaultState(snapshot.Vault)
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(420))
				return card.value.Layout(gtx, th.Theme)
			}),
		)
	}
	// a raised limit is the user's own business, the other answers reach an agent that can read a note
	withNote := approval.Kind != protocol.ApprovalBudget
	card.note.Placeholder = i18n.T("A note for the team (optional)")
	children = append(children,
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			count := len(plaques)
			if withNote {
				count++
			}
			return look.Flow{Gap: 8, LineGap: 8}.Layout(gtx, count, func(gtx layout.Context, index int) layout.Dimensions {
				if index == len(plaques) {
					gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(280))
					return card.note.Layout(gtx, th.Theme)
				}
				if index == 0 && !ready {
					gtx = gtx.Disabled()
				}
				return plaques[index].Layout(gtx, th.Theme)
			})
		}),
	)
	return children
}

// a task that stopped short waits for the user to look, or to try again
func (c *NeedCards) blockedField(gtx layout.Context, e *env.Env, snapshot state.Snapshot, card *needCard, need Need) []layout.FlexChild {
	th := e.Theme
	task := *need.Task
	card.approve.Kind, card.approve.Small, card.approve.Label = kit.PlaqueLine, true, i18n.T("Run it again")
	card.open.Kind, card.open.Small, card.open.Label = kit.PlaqueQuiet, true, i18n.T("Open the task")
	if card.approve.Clicked(gtx) {
		key := NeedKey(need)
		c.fold(key, need, i18n.Tf("Running again: %s", task.Title))
		e.Call(protocol.MTeamsTaskRetry, protocol.TeamTaskRetryParams{ID: task.ID}, new(protocol.TeamTask), c.undo(e, key))
	}
	if card.open.Clicked(gtx) {
		OpenTask(e, task.ID)
	}
	word, _, _ := TaskStanding(task)
	reason := task.Reason
	if reason == "" {
		reason = i18n.T("It stopped before it finished and said nothing more.")
	}
	plaques := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions { return card.approve.Layout(gtx, th.Theme) })}
	if c.Task != task.ID {
		plaques = append(plaques,
			layout.Rigid(layout.Spacer{Width: 8}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return card.open.Layout(gtx, th.Theme) }),
		)
	}
	return []layout.FlexChild{
		layout.Rigid(claimRow(th, word, look.Ago(gtx.Now, task.Updated))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(look.Paragraph(th, th.T.Title, th.R.Fg, task.Title)),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(look.Paragraph(th, th.T.Proof, th.R.Fg2, reason)),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return c.layoutWhere(gtx, e, snapshot, card, task.Team, task.Member, "")
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, plaques...)
		}),
	}
}

func claimRow(th *look.Theme, word, ago string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.W.Layout(gtx, kit.Claim(th.Theme, palette.Attention, word))
			}),
			layout.Rigid(kit.Text(th.Theme, th.T.MonoSm, th.R.Fg3, ago)),
		)
	}
}

// what exactly would happen, in full once asked for
func layoutDetail(gtx layout.Context, th *look.Theme, card *needCard, detail string) layout.Dimensions {
	lines := strings.Split(strings.TrimRight(detail, "\n"), "\n")
	shown := strings.Join(lines, "\n")
	long := len(lines) > detailLines
	if long && !card.all {
		shown = strings.Join(lines[:detailLines], "\n")
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.Label{Style: th.T.Mono, Color: th.R.Fg2}.Layout(gtx, th.Theme, shown)
		}),
	}
	if long {
		card.more.Kind, card.more.Small = kit.PlaqueQuiet, true
		card.more.Label = i18n.Tf("Show all %d lines", len(lines))
		if card.all {
			card.more.Label = i18n.T("Show less")
		}
		if card.more.Clicked(gtx) {
			card.all = !card.all
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return card.more.Layout(gtx, th.Theme) }),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// the team, member and task a need belongs to; the team and the task open where they live
func (c *NeedCards) layoutWhere(gtx layout.Context, e *env.Env, snapshot state.Snapshot, card *needCard, teamID, member, taskID string) layout.Dimensions {
	th := e.Theme
	if card.team.Clicked(gtx) {
		e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabTeams, Team: teamID})
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return look.Link(gtx, th, &card.team, TeamName(snapshot.Teams, teamID))
		}),
	}
	dot := layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, "  ·  "))
	if name := MemberOf(snapshot, teamID, member); name != "" {
		children = append(children, dot, layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, name)))
	}
	if task, ok := snapshot.TeamTasks[taskID]; ok && taskID != c.Task {
		if card.task.Clicked(gtx) {
			OpenTask(e, task.ID)
		}
		children = append(children, dot, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return look.Link(gtx, th, &card.task, task.Title)
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx, children...)
}

// what the user answered, folded to one line
func layoutRecord(gtx layout.Context, th *look.Theme, line string) layout.Dimensions {
	return kit.Slab(gtx, th.R.Stone1, th.Cut.Small, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Inset{Top: 10, Bottom: 10, Left: 13, Right: 13}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(look.IconWidget(look.Icon.Check, 16, th.R.Fg3)),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return kit.Label{Style: th.T.Caption, Color: th.R.Fg2, MaxLines: 1}.Layout(gtx, th.Theme, line)
				}),
			)
		})
	})
}

// the card folds at once, and comes back with the reason if the daemon refused
func (c *NeedCards) decide(e *env.Env, need Need, decision, note, value, line string) {
	key := NeedKey(need)
	c.fold(key, need, line)
	params := protocol.ApprovalDecideParams{ID: need.Approval.ID, Decision: decision, Note: note, Value: value}
	e.Call(protocol.MTeamsApprovalDecide, params, new(protocol.Approval), c.undo(e, key))
}

// the card folds to one line that records the answer for the rest of the session
func (c *NeedCards) fold(key string, need Need, line string) {
	if c.answers == nil {
		c.answers = make(map[string]answer)
	}
	c.answers[key] = answer{line: line, need: need}
}

func (c *NeedCards) undo(e *env.Env, key string) func(error) {
	return func(err error) {
		if err != nil {
			delete(c.answers, key)
			e.Notify(err.Error())
		}
	}
}
