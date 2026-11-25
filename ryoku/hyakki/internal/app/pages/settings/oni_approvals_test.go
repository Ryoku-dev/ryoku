package settings

import (
	"image"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

	"ryoku-rashin/internal/app/conn"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

// A press or queued click on an old card must not become permission for
// the next card occupying the same screen position.
func TestOniApprovalReplacementDiscardsOldPointerInteraction(t *testing.T) {
	for _, label := range []string{"Approve this Oni action", "Decline this Oni action"} {
		for _, queuedRelease := range []bool{false, true} {
			t.Run(label+map[bool]string{false: "/pressed", true: "/queued-click"}[queuedRelease], func(t *testing.T) {
				connection := conn.NewOptions(state.New(nil), client.Options{Client: protocol.ClientApp, Socket: filepath.Join(t.TempDir(), "absent.sock")})
				t.Cleanup(connection.Close)
				// Do not apply asynchronous completion: busy reveals whether a
				// forbidden RPC was initiated synchronously by the release.
				e := &env.Env{Theme: look.NewWithRoles(palette.Signature(true)), Client: connection, Post: func(func()) {}}
				a := oniApprovals{card: feature.GuardCard("guard-a", "Close the selected feed?"), deadline: time.Now().Add(time.Minute)}
				var router input.Router
				var ops op.Ops
				now := time.Now()
				draw := func() {
					ops.Reset()
					gtx := layout.Context{Ops: &ops, Source: router.Source(), Now: now, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(700, 80))}
					rows := a.rows(e)
					rows[len(rows)-1].Control(gtx)
					router.Frame(&ops)
					now = now.Add(10 * time.Millisecond)
				}
				draw()
				var at f32.Point
				found := false
				for _, node := range router.AppendSemantics(nil) {
					if node.Desc.Label == label && node.Desc.Gestures&input.ClickGesture != 0 {
						bounds := node.Desc.Bounds
						at, found = f32.Pt(float32(bounds.Min.X+bounds.Max.X)/2, float32(bounds.Min.Y+bounds.Max.Y)/2), true
					}
				}
				if !found {
					t.Fatalf("approval button %q was not drawn", label)
				}
				router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, PointerID: 1, Position: at}, pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, PointerID: 1, Buttons: pointer.ButtonPrimary, Position: at})
				draw()
				if queuedRelease {
					router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, PointerID: 1, Position: at})
				}
				// Model delivery of the next daemon card, after A expired.
				a.card, a.deadline = feature.GuardCard("guard-b", "Close the selected feed?"), time.Now().Add(time.Minute)
				draw()
				if !queuedRelease {
					router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, PointerID: 1, Position: at})
					draw()
				}
				// Plaque.Layout may latch the release for Clicked on the
				// following frame; observe that consumer frame too.
				draw()
				if a.busy {
					t.Fatal("an interaction begun on card A submitted an answer for replacement card B")
				}
				if a.card == nil || a.card.ID != "guard-b" {
					t.Fatal("replacement card B did not remain unanswered")
				}
				// Prove the pointer location is an actual enabled control,
				// not a false-negative hit region in this narrow fixture.
				router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, PointerID: 1, Buttons: pointer.ButtonPrimary, Position: at})
				draw()
				router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, PointerID: 1, Position: at})
				draw()
				draw()
				if !a.busy {
					t.Fatal("a fresh click on card B did not initiate its answer; pointer fixture missed the control")
				}
			})
		}
	}
}

func TestOniApprovalsRetainIndependentReplayedCardsUntilAnswered(t *testing.T) {
	var approvals oniApprovals
	cards := []*feature.ConsentFrame{
		feature.GuardCard("guard-first", "Close the selected feed?"),
		feature.ForgetCard("forget-next", "Forget the selected fact?"),
		feature.RecallCard("recall-last", "openai/gpt-4.1", "Send the selected passages?"),
	}
	for _, card := range cards {
		approvals.receive(*card)
	}
	for _, want := range cards {
		if approvals.card == nil || approvals.card.ID != want.ID {
			t.Fatalf("the next unanswered replay was lost: want %s, got %+v", want.ID, approvals.card)
		}
		approvals.remove(want.ID)
	}
	if approvals.card != nil {
		t.Fatal("an answered replay card remained actionable")
	}
}
