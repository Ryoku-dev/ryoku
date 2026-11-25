package settings

import (
	"context"
	"encoding/json"
	"time"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

// Voice-initiated memory changes and feed closes require the user's app
// click. This observer exists only while Writing and memory is visible;
// nothing subscribes, polls or approves in the background.
type oniApprovals struct {
	drawn      bool
	watch      context.CancelFunc
	generation int
	failedAt   time.Time
	card       *feature.ConsentFrame
	deadline   time.Time
	controls   *oniApprovalControls
	pending    [3]pendingOniApproval
	busy       bool
	message    string
}

// Each card gets distinct clickable addresses. Resetting a Plaque in place
// would keep its handler tag and allow a queued release to target a new card.
type oniApprovalControls struct {
	id          string
	allow, deny kit.Plaque
}

type pendingOniApproval struct {
	frame    feature.ConsentFrame
	deadline time.Time
}

func (a *oniApprovals) stop() {
	if a.watch != nil {
		a.watch()
		a.watch = nil
		a.generation++
	}
	a.card = nil
	a.controls, a.pending = nil, [3]pendingOniApproval{}
}

func (a *oniApprovals) settle(gtx layout.Context, e *env.Env) {
	want := a.drawn && protocol.EffectiveOni(e.Snapshot().Settings).Master
	a.drawn = false
	if !want {
		a.stop()
		return
	}
	if a.watch != nil || e.Client == nil || e.Post == nil || !e.Client.Connected() || !a.failedAt.IsZero() && gtx.Now.Sub(a.failedAt) < watchRetry {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.watch = cancel
	a.generation++
	generation, client, post, started := a.generation, e.Client, e.Post, gtx.Now
	go func() {
		err := client.WatchOni(ctx, func(raw json.RawMessage) {
			payload, err := feature.DecodeFrame(raw)
			if err != nil {
				return
			}
			post(func() {
				if a.generation != generation || a.watch == nil {
					return
				}
				switch f := payload.(type) {
				case *feature.ConsentFrame:
					a.receive(*f)
				case *feature.ConsentAnswerFrame:
					a.remove(f.ID)
				}
			})
		})
		cancel()
		post(func() {
			if a.generation != generation {
				return
			}
			a.watch, a.card = nil, nil
			a.controls, a.pending = nil, [3]pendingOniApproval{}
			if err != nil {
				a.failedAt = started
				a.message = err.Error()
			}
		})
	}()
}

func (a *oniApprovals) answer(e *env.Env, id string, yes bool) {
	if a.card == nil || a.card.ID != id || a.busy || !time.Now().Before(a.deadline) {
		return
	}
	answer := "no"
	if yes {
		answer = "yes"
	}
	a.busy = true
	result := new(protocol.OniAskResult)
	e.Call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: id, Answer: answer}, result, func(err error) {
		a.busy = false
		a.remove(id)
		if err != nil {
			a.message = err.Error()
			return
		}
		a.message = i18n.T("Declined. Nothing was approved.")
		if yes {
			a.message = i18n.T("The app approval was sent. Oni rechecks the action before it runs.")
		}
		if result.Answer != nil {
			a.message = result.Answer.Text
		}
	})
}

func (a *oniApprovals) rows(e *env.Env) []look.Setting {
	th := e.Theme
	rows := []look.Setting{{Title: i18n.T("Pending Oni approval"), Detail: i18n.T("Review a focus close, memory change or hosted recall here. Showing a card approves nothing. A focus close requires your app click, never a spoken yes.")}}
	if a.card != nil && time.Now().Before(a.deadline) {
		card := *a.card
		if a.controls == nil || a.controls.id != card.ID {
			a.controls = &oniApprovalControls{id: card.ID}
		}
		controls := a.controls
		rows = append(rows, look.Setting{Title: card.Question, Detail: i18n.Tf("%s · %s", card.Destination, card.Route), Control: func(gtx layout.Context) layout.Dimensions {
			controls.allow.Kind, controls.allow.Small, controls.allow.Label = kit.PlaqueTonal, true, i18n.T("Approve this Oni action")
			controls.deny.Kind, controls.deny.Small, controls.deny.Label = kit.PlaqueQuiet, true, i18n.T("Decline this Oni action")
			if controls.allow.Clicked(gtx) && !a.busy {
				a.answer(e, card.ID, true)
			}
			if controls.deny.Clicked(gtx) && !a.busy {
				a.answer(e, card.ID, false)
			}
			if a.busy {
				gtx = gtx.Disabled()
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return controls.deny.Layout(gtx, th.Theme) }),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return controls.allow.Layout(gtx, th.Theme) }),
			)
		}})
	}
	if a.message != "" {
		rows = append(rows, look.Setting{Title: i18n.T("Oni approval result"), Detail: a.message})
	}
	return rows
}

func (a *oniApprovals) receive(frame feature.ConsentFrame) {
	index := -1
	switch frame.Feature {
	case feature.FeedGuard:
		index = 0
	case feature.MemoryForget:
		index = 1
	case feature.Recall:
		index = 2
	}
	if index < 0 {
		return
	}
	deadline := time.Now().Add(time.Duration(frame.ExpiresMs) * time.Millisecond)
	old := a.pending[index]
	if old.frame.ID == frame.ID && old.deadline.Before(deadline) {
		deadline = old.deadline
	}
	a.pending[index] = pendingOniApproval{frame: frame, deadline: deadline}
	a.next()
}

func (a *oniApprovals) remove(id string) {
	for index := range a.pending {
		if a.pending[index].frame.ID == id {
			a.pending[index] = pendingOniApproval{}
		}
	}
	if a.card != nil && a.card.ID == id {
		a.card, a.controls = nil, nil
	}
	a.next()
}

// Multiple independent owners may have a pending card when an observer
// joins. Keep each until answered/expired rather than overwriting the first
// two with the last replayed frame.
func (a *oniApprovals) next() {
	now := time.Now()
	active := false
	for index := range a.pending {
		pending := &a.pending[index]
		if !now.Before(pending.deadline) {
			*pending = pendingOniApproval{}
		}
		if a.card != nil && pending.frame.ID == a.card.ID {
			active = true
		}
	}
	if active {
		return
	}
	a.card, a.controls = nil, nil
	for _, pending := range a.pending {
		if pending.frame.ID != "" {
			copy := pending.frame
			a.card, a.deadline = &copy, pending.deadline
			return
		}
	}
}
