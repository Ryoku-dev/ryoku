package teamsui

import (
	"image"
	"slices"
	"sort"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// a detail longer than this unfolds on request
const detailLines = 8

// NeedCards answer what needs the user where it shows: an approval, a
// credential request, a budget stop or a blocked task, each a notched slab
// with its tone's notch and claim. Each card has one filled action, and only
// the oldest open card's is gold; that card is the view's hero, a slab toned
// by attention, so a page holds one of each. An answered card folds at once
// to one line that keeps the answer for the rest of the session; a refusal by
// the daemon unfolds it again and says why.
type NeedCards struct {
	// Task is the task page the cards stand on, whose own link they leave out.
	Task string

	cards   map[string]*needCard
	answers map[string]answer
	// the oldest open need, whose card carries the page's one gold action
	urgent string
	// on a task page, the oldest open choice, which takes its keys while nothing else holds the keyboard
	keyed string
}

type needCard struct {
	tablet, record look.Unfold

	approve, always, deny, open kit.Plaque
	note, value                 kit.TextField
	more                        kit.Plaque
	team, task                  kit.Click
	// the options of a choice question, and whether the card holds the keys that pick them
	choices [protocol.ApprovalOptionsMax]kit.Click
	focused bool
	// the whole detail is shown
	all bool
}

// the key each option of a choice question answers to, by letter or by number
const (
	choiceLetters = "ABCDEF"
	choiceDigits  = "123456"
)

type answer struct {
	line string
	need Need
}

// "Chose B: beta": the letter it was picked by and the option
func choseLine(options []string, choice string) string {
	index := slices.Index(options, choice)
	if index < 0 || index >= len(choiceLetters) {
		return i18n.Tf("Chose %s", choice)
	}
	return i18n.Tf("Chose %s: %s", choiceLetters[index:index+1], choice)
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
	c.urgent, c.keyed = "", ""
	for _, need := range shown {
		key := NeedKey(need)
		if _, done := c.answers[key]; done {
			continue
		}
		if c.urgent == "" {
			c.urgent = key
		}
		if c.keyed == "" && c.Task != "" && need.Approval != nil && len(need.Approval.Options) > 0 {
			c.keyed = key
		}
	}
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
			return card.tablet.Layout(gtx, th, !done, func(gtx layout.Context) layout.Dimensions {
				return c.layoutTablet(gtx, e, card, need)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return card.record.Layout(gtx, th, done, func(gtx layout.Context) layout.Dimensions {
				return layoutRecord(gtx, th, answered.line)
			})
		}),
	)
}

// a need stands on its pane as a notched slab, its tone in the notch and the claim word; the urgent one is the hero
func (c *NeedCards) layoutTablet(gtx layout.Context, e *env.Env, card *needCard, need Need) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	gold := NeedKey(need) == c.urgent
	tone := palette.Attention
	var children []layout.FlexChild
	if need.Approval != nil {
		children = c.approvalField(gtx, e, snapshot, card, need, gold)
	} else {
		_, tone, _ = TaskStanding(*need.Task)
		children = c.blockedField(gtx, e, snapshot, card, need, gold)
	}
	body := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	var dimensions layout.Dimensions
	if gold {
		dimensions = HeroSlab(gtx, th, tone, body)
	} else {
		dimensions = NotchedSlab(gtx, th, tone, th.Sp.S4, body)
	}
	if need.Approval != nil && len(need.Approval.Options) > 0 {
		holdKeys(gtx, th, card, dimensions.Size)
	}
	return dimensions
}

func (c *NeedCards) approvalField(gtx layout.Context, e *env.Env, snapshot state.Snapshot, card *needCard, need Need, gold bool) []layout.FlexChild {
	th := e.Theme
	approval := *need.Approval
	request := approval.Kind == protocol.ApprovalRequest
	vaultOpen := snapshot.Vault.State == protocol.VaultOpen
	note := strings.TrimSpace(card.note.Text())
	value := card.value.Text()
	options := approval.Options[:min(len(approval.Options), protocol.ApprovalOptionsMax)]
	choice := len(options) > 0
	plaques := []*kit.Plaque{&card.approve}
	card.approve.Kind, card.deny.Kind, card.always.Kind = kit.PlaqueTonal, kit.PlaqueQuiet, kit.PlaqueQuiet
	if gold {
		card.approve.Kind, card.always.Kind = kit.PlaqueGold, kit.PlaqueLine
	}
	card.approve.Small, card.deny.Small, card.always.Small = true, true, true
	card.deny.Label = i18n.T("Deny")
	switch {
	case choice:
		// the options are the answers, so only a refusal of them all stays a plaque
		plaques = plaques[:0]
		card.deny.Label = i18n.T("None of these")
	case approval.Kind == protocol.ApprovalBudget:
		card.approve.Label, card.deny.Label = i18n.T("Raise the limit by half"), i18n.T("Keep it paused")
	case request:
		card.approve.Label = i18n.T("Add to the vault")
	default:
		card.approve.Label = i18n.T("Approve")
		if approval.Always != "" {
			card.always.Label = approval.Always
			plaques = append(plaques, &card.always)
		}
	}
	plaques = append(plaques, &card.deny)
	if choice {
		if index, ok := pickedChoice(gtx, card, len(options), NeedKey(need) == c.keyed); ok {
			line := choseLine(options, options[index])
			params := protocol.ApprovalDecideParams{ID: approval.ID, Decision: protocol.DecideChoose, Note: note, Choice: options[index]}
			c.send(e, need, params, line)
			// the folded card no longer answers to keys
			card.focused = false
			gtx.Execute(key.FocusCmd{})
		}
	}
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
		switch {
		case choice:
			line = i18n.Tf("Chose none: %s", approval.Title)
		case approval.Kind == protocol.ApprovalBudget:
			line = i18n.Tf("Kept paused: %s", approval.Title)
		}
		c.decide(e, need, protocol.DecideDeny, note, "", line)
	}

	children := []layout.FlexChild{
		layout.Rigid(claimRow(th, palette.Attention, ApprovalWord(approval.Kind), look.Ago(gtx.Now, approval.Created))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(look.Paragraph(th, th.T.Title, th.R.Fg, approval.Title)),
	}
	if approval.Detail != "" {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layoutDetail(gtx, th, card, approval) }),
		)
	}
	children = append(children,
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return c.layoutWhere(gtx, e, snapshot, card, approval.Team, approval.Member, approval.Task)
		}),
	)
	if choice {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout))
		for index, option := range options {
			if index > 0 {
				children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout))
			}
			click, letter := &card.choices[index], choiceLetters[index:index+1]
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layoutChoice(gtx, th, click, letter, option)
			}))
		}
	}
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
func (c *NeedCards) blockedField(gtx layout.Context, e *env.Env, snapshot state.Snapshot, card *needCard, need Need, gold bool) []layout.FlexChild {
	th := e.Theme
	task := *need.Task
	card.approve.Kind, card.approve.Small, card.approve.Label = kit.PlaqueTonal, true, i18n.T("Run it again")
	if gold {
		card.approve.Kind = kit.PlaqueGold
	}
	// the card's one filled action is the retry, so the way to the task stays quiet
	card.open.Kind, card.open.Small, card.open.Label = kit.PlaqueQuiet, true, i18n.T("Open the task")
	if card.approve.Clicked(gtx) {
		key := NeedKey(need)
		c.fold(key, need, i18n.Tf("Running again: %s", task.Title))
		e.Call(protocol.MTeamsTaskRetry, protocol.TeamTaskRetryParams{ID: task.ID}, new(protocol.TeamTask), c.undo(e, key))
	}
	if card.open.Clicked(gtx) {
		OpenTask(e, task.ID)
	}
	word, tone, _ := TaskStanding(task)
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
		layout.Rigid(claimRow(th, tone, word, look.Ago(gtx.Now, task.Updated))),
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

// the claim word in its tone's ink, the notch already marks the corner
func claimRow(th *look.Theme, tone palette.Tone, word, ago string) layout.Widget {
	ink := th.R.Stone(tone)
	if tone == palette.Idle {
		ink = th.R.Fg2
	}
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: th.T.Label, Color: ink, MaxLines: 1}.Layout(gtx, th.Theme, word)
			}),
			layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, ago)),
		)
	}
}

// what exactly would happen, in full once asked for; a request or a mail is shown as it would go out,
// what an agent asks or a limit says is prose
func layoutDetail(gtx layout.Context, th *look.Theme, card *needCard, approval protocol.Approval) layout.Dimensions {
	lines := strings.Split(strings.TrimRight(approval.Detail, "\n"), "\n")
	shown := strings.Join(lines, "\n")
	long := len(lines) > detailLines
	if long && !card.all {
		shown = strings.Join(lines[:detailLines], "\n")
	}
	style := th.T.Detail
	if approval.Kind == protocol.ApprovalCredential || approval.Kind == protocol.ApprovalMail {
		style = th.T.Mono
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.Label{Style: style, Color: th.R.Fg2}.Layout(gtx, th.Theme, shown)
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
	// facts are set apart by space, never a dot
	gap := layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout)
	if name := MemberOf(snapshot, teamID, member); name != "" {
		children = append(children, gap, layout.Rigid(kit.Text(th.Theme, th.T.Detail, th.R.Fg2, name)))
	}
	if task, ok := snapshot.TeamTasks[taskID]; ok && taskID != c.Task {
		if card.task.Clicked(gtx) {
			OpenTask(e, task.ID)
		}
		children = append(children, gap, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return look.Link(gtx, th, &card.task, task.Title)
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx, children...)
}

// what the user answered, folded to one line on the slab the card stood on
func layoutRecord(gtx layout.Context, th *look.Theme, line string) layout.Dimensions {
	record := op.Record(gtx.Ops)
	dimensions := layout.Inset{Top: 10, Bottom: 10, Left: 13, Right: 13}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(look.IconWidget(look.Icon.Check, 16, th.R.Fg3)),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: th.T.Detail, Color: th.R.Fg2, MaxLines: 1}.Layout(gtx, th.Theme, line)
			}),
		)
	})
	call := record.Stop()
	bounds, cut := image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Small)
	kit.SlabSurface(gtx, th.Theme, bounds, cut, th.R.Slab)
	call.Add(gtx.Ops)
	return dimensions
}

// one option of a choice question: its key, then what it says; a click picks it
func layoutChoice(gtx layout.Context, th *look.Theme, click *kit.Click, letter, option string) layout.Dimensions {
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		// an option is a tonal row on the card, lifted a step above the slab
		ink := th.R.Field
		if click.Hovered() {
			ink = palette.Mix(th.R.Field, th.R.Fg, tokens.AlphaHoverPlane)
		}
		dimensions := kit.Slab(gtx, ink, th.Cut.Small, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Inset{Top: 8, Bottom: 8, Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Dp(24)
						return kit.Label{Style: th.T.Mono, Color: th.R.Fg2}.Layout(gtx, th.Theme, letter)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return kit.Label{Style: th.T.Body, Color: th.R.Fg}.Layout(gtx, th.Theme, option)
					}),
				)
			})
		})
		if gtx.Focused(&click.Clickable) {
			kit.FocusRing(gtx, th.Theme, image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Small))
		}
		return dimensions
	})
}

// the option a click on its row picked, or its letter or number while the card holds the keys or, free on a task page, while nothing does
func pickedChoice(gtx layout.Context, card *needCard, count int, free bool) (int, bool) {
	picked, ok := 0, false
	for index := range count {
		if card.choices[index].Clicked(gtx) && !ok {
			picked, ok = index, true
		}
	}
	tag := &card.focused
	keys := event.Tag(tag)
	if free && gtx.Focused(nil) {
		keys = nil
	}
	filters := make([]event.Filter, 0, 2+2*count)
	filters = append(filters, key.FocusFilter{Target: tag}, pointer.Filter{Target: tag, Kinds: pointer.Press})
	for index := range count {
		filters = append(filters,
			key.Filter{Focus: keys, Name: key.Name(choiceLetters[index : index+1]), Optional: key.ModShift},
			key.Filter{Focus: keys, Name: key.Name(choiceDigits[index : index+1])},
		)
	}
	for {
		raw, more := gtx.Event(filters...)
		if !more {
			break
		}
		switch event := raw.(type) {
		case key.FocusEvent:
			card.focused = event.Focus
		case pointer.Event:
			// a press anywhere on the card gives it the keys; a field pressed takes them back after
			gtx.Execute(key.FocusCmd{Tag: tag})
		case key.Event:
			index := strings.Index(choiceLetters, string(event.Name))
			if index < 0 {
				index = strings.Index(choiceDigits, string(event.Name))
			}
			if event.State == key.Press && index >= 0 && !ok {
				picked, ok = index, true
			}
		}
	}
	return picked, ok
}

// a choice question's card takes the keys that pick its options, and shows when it holds them
func holdKeys(gtx layout.Context, th *look.Theme, card *needCard, size image.Point) {
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops)
	event.Op(gtx.Ops, &card.focused)
	area.Pop()
	pass.Pop()
	if card.focused {
		kit.FocusRing(gtx, th.Theme, image.Rectangle{Max: size}, gtx.Dp(th.Cut.Mid))
	}
}

// the card folds at once, and comes back with the reason if the daemon refused
func (c *NeedCards) decide(e *env.Env, need Need, decision, note, value, line string) {
	c.send(e, need, protocol.ApprovalDecideParams{ID: need.Approval.ID, Decision: decision, Note: note, Value: value}, line)
}

func (c *NeedCards) send(e *env.Env, need Need, params protocol.ApprovalDecideParams, line string) {
	key := NeedKey(need)
	c.fold(key, need, line)
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
