package daemon

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"sync"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/teams"
)

// The Yokai's approvals on Oni's pill: a Hands request a team member Hyakki
// hosts raised (teams.agent_hands), and a permission request a hosted
// harness raised in its team task's conversation. One card shows at a time,
// the oldest first, while a presenter draws Oni. The presenter may allow a
// read- or write-tier request once, a permission for its conversation, or
// say no; a danger-tier permission and anything that would reach the whole
// desktop show only "answer it in the app". Leaving a card to the app
// decides nothing and takes it off the pill; the app's card stays open.
// Nothing here runs or holds anything while no request waits.

// yokaiHands is what the Teams service offers the pill and Hands.
type yokaiHands interface {
	SetHands(teams.HandsStart)
	HandsApproval(id string) bool
	HandsAsked(id string) (scope, window string, ok bool)
	PresenterDecide(ctx context.Context, id, decision string) (protocol.Approval, error)
}

// oniApprovals are the waiting cards and the one on the pill.
type oniApprovals struct {
	mu       sync.Mutex
	ops      []func()
	draining bool

	cards []*approvalCard
	seq   uint64
	// shown is the card on the pill, "" when none is
	shown string
}

// approvalCard is one waiting request: a Hands approval by its id, or a
// permission by its session and request, with the buttons its card offers
// and, for a permission, the option each button picks.
type approvalCard struct {
	id       string
	approval string
	session  string
	request  string
	title    string
	scope    string
	options  []string
	picks    map[string]string
	// inApp: the presenter left it to the app; it stays off the pill
	inApp bool
}

// approvalsBind lets Teams start Hands for an allowed member request and
// follows the approvals and permissions that may go on the pill.
func (d *Daemon) approvalsBind() {
	if y, ok := d.deps.Teams.(yokaiHands); ok {
		y.SetHands(d.handsGranted)
	}
	d.manager.permissionSeen = d.approvalsPermission
	// a session that ends takes its waiting requests with it
	ended := d.manager.ended
	d.manager.ended = func(id string) {
		if ended != nil {
			ended(id)
		}
		d.approvalsDo(func() { d.approvalsDrop(func(c *approvalCard) bool { return c.session == id }) })
	}
	d.events.add(d.approvalsEvent)
}

// yokai is the Teams service as the pill uses it, nil without one.
func (d *Daemon) yokai() yokaiHands {
	y, _ := d.deps.Teams.(yokaiHands)
	return y
}

// approvalsDo runs f on the approvals' own goroutine, after what is
// queued; the caller never waits and may hold any lock of its own.
func (d *Daemon) approvalsDo(f func()) {
	a := &d.approvals
	a.mu.Lock()
	a.ops = append(a.ops, f)
	if a.draining {
		a.mu.Unlock()
		return
	}
	a.draining = true
	a.mu.Unlock()
	go func() {
		a.mu.Lock()
		for len(a.ops) > 0 {
			next := a.ops[0]
			a.ops[0] = nil
			a.ops = a.ops[1:]
			a.mu.Unlock()
			next()
			a.mu.Lock()
		}
		a.ops = nil
		a.draining = false
		a.mu.Unlock()
	}()
}

// approvalsEvent follows Hands approvals as Teams announces them.
func (d *Daemon) approvalsEvent(event string, data any) {
	if event != protocol.EvtApproval {
		return
	}
	approval, ok := data.(protocol.Approval)
	if !ok || approval.Kind != protocol.ApprovalHands {
		return
	}
	d.approvalsDo(func() {
		if approval.Status != protocol.ApprovalPending {
			d.approvalsDrop(func(c *approvalCard) bool { return c.approval == approval.ID })
			return
		}
		y := d.yokai()
		if y == nil {
			return
		}
		// a request nothing binds any more, or one for a scope a member may
		// not ask for, is the app's alone and never shows on the pill
		scope, _, asked := y.HandsAsked(approval.ID)
		if !asked || !slices.Contains(protocol.AgentHandsScopes, scope) {
			return
		}
		card := &approvalCard{approval: approval.ID, title: oniText(approval.Title, feature.ApprovalTitleMax), scope: scope,
			options: []string{feature.ApprovalOnce, feature.ApprovalNo, feature.ApprovalInApp}}
		d.approvalsAdd(card, func(c *approvalCard) bool { return c.approval == approval.ID })
	})
}

// approvalsPermission follows a team session's permission requests: open
// as it is asked, closed once resolved. The session manager calls it with
// the session's lock held, so it only queues.
func (d *Daemon) approvalsPermission(session string, p protocol.PermissionData, open bool) {
	same := func(c *approvalCard) bool { return c.session == session && c.request == p.Request }
	if !open {
		d.approvalsDo(func() { d.approvalsDrop(same) })
		return
	}
	if p.Tier == "" {
		// a question or a plan is the app's to answer and never shows
		return
	}
	card := &approvalCard{session: session, request: p.Request, title: oniText(p.Title, feature.ApprovalTitleMax), options: []string{feature.ApprovalInApp}}
	if p.Tier == protocol.TierRead || p.Tier == protocol.TierWrite {
		picks := map[string]string{}
		for _, option := range p.Options {
			switch option.Kind {
			case protocol.OptAllowOnce:
				picks[feature.ApprovalOnce] = option.ID
			case protocol.OptAllowAlways:
				picks[feature.ApprovalAlways] = option.ID
			case protocol.OptRejectOnce:
				picks[feature.ApprovalNo] = option.ID
			}
		}
		if picks[feature.ApprovalOnce] != "" && picks[feature.ApprovalNo] != "" {
			card.picks, card.options = picks, nil
			for _, button := range feature.ApprovalOptions {
				if button == feature.ApprovalInApp || picks[button] != "" {
					card.options = append(card.options, button)
				}
			}
		}
	}
	if card.title == "" {
		card.title = i18n.T("A team's agent asks for permission")
	}
	d.approvalsDo(func() { d.approvalsAdd(card, same) })
}

// approvalsAdd queues card unless one like it waits. On the approvals'
// goroutine.
func (d *Daemon) approvalsAdd(card *approvalCard, same func(*approvalCard) bool) {
	a := &d.approvals
	a.mu.Lock()
	if slices.ContainsFunc(a.cards, same) {
		a.mu.Unlock()
		return
	}
	a.seq++
	prefix := feature.ApprovalPermissionPrefix
	if card.approval != "" {
		prefix = feature.ApprovalHandsPrefix
	}
	card.id = prefix + strconv.FormatUint(a.seq, 10)
	a.cards = append(a.cards, card)
	a.mu.Unlock()
	d.approvalsShow(false)
}

// approvalsDrop takes the cards same matches out of the queue. On the
// approvals' goroutine.
func (d *Daemon) approvalsDrop(same func(*approvalCard) bool) {
	a := &d.approvals
	a.mu.Lock()
	a.cards = slices.DeleteFunc(a.cards, same)
	if len(a.cards) == 0 {
		a.cards = nil
	}
	a.mu.Unlock()
	d.approvalsShow(false)
}

// approvalsShow puts the oldest card not left to the app on the pill, and
// takes down the one it replaces; again sends the shown card once more, for
// a presenter that just opened. On the approvals' goroutine.
func (d *Daemon) approvalsShow(again bool) {
	a := &d.approvals
	a.mu.Lock()
	var next *approvalCard
	for _, c := range a.cards {
		if !c.inApp {
			next = c
			break
		}
	}
	was := a.shown
	a.shown = ""
	var open *feature.ApprovalFrame
	if next != nil {
		a.shown = next.id
		open = &feature.ApprovalFrame{ID: next.id, Open: true, Title: next.title, Scope: next.scope, Options: slices.Clone(next.options)}
	}
	a.mu.Unlock()
	if was != "" && (next == nil || was != next.id) {
		d.oniEmit(&feature.ApprovalFrame{ID: was})
	}
	if open != nil && (again || was != open.ID) {
		d.oniEmit(open)
	}
}

// approvalsResend sends the waiting card to a presenter that just opened.
func (d *Daemon) approvalsResend() {
	a := &d.approvals
	a.mu.Lock()
	idle := len(a.cards) == 0
	a.mu.Unlock()
	if !idle {
		d.approvalsDo(func() { d.approvalsShow(true) })
	}
}

// approvalCardAnswer answers an approval card from oni.consent.answer. ok
// is false for any other card. Only the surface drawing Oni and the app
// answer one; an answer the card does not offer is refused.
func (d *Daemon) approvalCardAnswer(c *connection, p protocol.OniConsentAnswerParams) (protocol.OniAskResult, bool, error) {
	if !strings.HasPrefix(p.ID, feature.ApprovalHandsPrefix) && !strings.HasPrefix(p.ID, feature.ApprovalPermissionPrefix) {
		return protocol.OniAskResult{}, false, nil
	}
	if !c.app && !d.oniIsPresenter(c) {
		return protocol.OniAskResult{}, true, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the surface drawing Oni, or the Hyakki app, answers this card."))
	}
	a := &d.approvals
	a.mu.Lock()
	var card approvalCard
	at := slices.IndexFunc(a.cards, func(x *approvalCard) bool { return x.id == p.ID })
	if at >= 0 {
		card = *a.cards[at]
	}
	a.mu.Unlock()
	if at < 0 {
		return protocol.OniAskResult{}, true, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That card was answered or is gone."))
	}
	if !slices.Contains(card.options, p.Answer) {
		return protocol.OniAskResult{}, true, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Answer this in the Hyakki app."))
	}
	if p.Answer == feature.ApprovalInApp {
		d.approvalsDo(func() {
			a.mu.Lock()
			for _, x := range a.cards {
				if x.id == p.ID {
					x.inApp = true
				}
			}
			a.mu.Unlock()
			d.approvalsShow(false)
		})
		return protocol.OniAskResult{Status: protocol.OniAskDeclined}, true, nil
	}
	allow := p.Answer != feature.ApprovalNo
	var err error
	if card.approval != "" {
		y := d.yokai()
		if y == nil {
			return protocol.OniAskResult{}, true, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Teams is not available."))
		}
		decision := protocol.DecideDeny
		if allow {
			decision = protocol.DecideOnce
		}
		_, err = y.PresenterDecide(context.Background(), card.approval, decision)
	} else {
		err = d.manager.permission(protocol.PermissionParams{ID: card.session, Request: card.request, Option: card.picks[p.Answer]})
	}
	if err != nil {
		return protocol.OniAskResult{}, true, err
	}
	if !allow {
		return protocol.OniAskResult{Status: protocol.OniAskDeclined}, true, nil
	}
	return protocol.OniAskResult{Status: protocol.OniAskAnswered}, true, nil
}

// handsDecideGate keeps a Hands approval's teams.approval_decide to the
// Hyakki app: the pill answers it through its card, nobody else at all.
func (d *Daemon) handsDecideGate(c *connection, raw json.RawMessage) error {
	if c.app {
		return nil
	}
	y := d.yokai()
	if y == nil {
		return nil
	}
	p, err := unmarshal[protocol.ApprovalDecideParams](raw)
	if err != nil {
		return err
	}
	if y.HandsApproval(p.ID) {
		return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the Hyakki app, or Oni's pill, answers a Hands request."))
	}
	return nil
}
