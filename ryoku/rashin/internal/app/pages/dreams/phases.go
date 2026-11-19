package dreams

import (
	"image"
	"slices"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

const (
	// a step narrower than this cannot hold its phase's word, so the steps wrap instead
	stepMin unit.Dp = 88
	// the tick ruler's marker, which the strip stands under the phase the dream is in
	rulerMarker unit.Dp = 24
)

// the phases as numbered steps over a tick ruler: its marker stands under the phase the
// dream is in and moves on with the pieces finished, and one line says what that rests on
type phaseStrip struct {
	ruler kit.TickRuler
}

func (s *phaseStrip) layout(gtx layout.Context, th *look.Theme, snapshot state.Snapshot, dream protocol.Team) layout.Dimensions {
	phases := protocol.DreamPhases
	current, within := phasePlace(snapshot, dream)
	share, caption := progress(snapshot, dream)
	width := gtx.Constraints.Max.X
	column := width / max(1, len(phases))
	step := func(gtx layout.Context, index int, stacked bool) layout.Dimensions {
		numeral, word := th.R.Fg3, th.R.Fg3
		switch {
		case index == current:
			// the accent marks the place, the word reads in full ink
			numeral, word = th.R.Sel, th.R.Fg
		case index < current:
			word = th.R.Fg2
		}
		ordinal := kit.Text(th.Theme, th.T.MonoSm, numeral, kit.Pad2(index+1))
		name := func(gtx layout.Context) layout.Dimensions {
			return kit.Label{Style: th.T.Label, Color: word, MaxLines: 1}.Layout(gtx, th.Theme, phaseWord(phases[index]))
		}
		if stacked {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(ordinal),
				layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(name),
			)
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(ordinal),
			layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
			layout.Rigid(name),
		)
	}
	// the marker reads how far the whole dream is, unless every step has a column to stand under
	read := float32(share)
	steps := func(gtx layout.Context) layout.Dimensions {
		return look.Flow{Gap: th.Sp.S4, LineGap: th.Sp.S2}.Layout(gtx, len(phases), func(gtx layout.Context, index int) layout.Dimensions {
			return step(gtx, index, false)
		})
	}
	if column >= gtx.Dp(stepMin) {
		marker := gtx.Dp(rulerMarker)
		// inside its phase the marker moves no further than its own column
		at := float64(current*column) + within*float64(max(0, column-marker))
		read = float32(at) / float32(max(1, width-marker))
		steps = func(gtx layout.Context) layout.Dimensions {
			height := 0
			for index := range phases {
				cell := gtx
				cell.Constraints = layout.Constraints{Max: image.Pt(max(0, column-gtx.Dp(th.Sp.S2)), gtx.Constraints.Max.Y)}
				offset := op.Offset(image.Pt(index*column, 0)).Push(gtx.Ops)
				height = max(height, step(cell, index, true).Size.Y)
				offset.Pop()
			}
			return layout.Dimensions{Size: image.Pt(width, height)}
		}
	}
	return stack(gtx,
		steps,
		gap(th.Sp.S2),
		func(gtx layout.Context) layout.Dimensions { return s.ruler.Layout(gtx, th.Theme, min(max(read, 0), 1)) },
		gap(th.Sp.S2),
		look.Paragraph(th, th.T.Caption, th.R.Fg2, caption),
	)
}

// the step the dream stands on and how far through it, which only building measures: by its pieces finished
func phasePlace(snapshot state.Snapshot, dream protocol.Team) (index int, within float64) {
	phase := dream.Dream.Phase
	if finished(dream) {
		phase = protocol.DreamDone
	}
	index = max(0, slices.Index(protocol.DreamPhases, phase))
	if phase != protocol.DreamBuilding {
		return index, 0
	}
	pieces := roundPieces(snapshot, dream)
	if len(pieces) == 0 {
		return index, 0
	}
	done := 0
	for _, piece := range pieces {
		if piece.Status == protocol.TaskDone {
			done++
		}
	}
	return index, float64(done) / float64(len(pieces))
}

// what the manager is doing or held on, in a sentence
func heldSentence(now time.Time, snapshot state.Snapshot, dream protocol.Team) string {
	task, _ := standingTask(snapshot, dream)
	switch {
	case dream.Dream.Done != nil:
		return i18n.Tf("You accepted it %s.", look.Ago(now, *dream.Dream.Done))
	case finished(dream):
		return i18n.T("You accepted it.")
	case dream.Paused:
		return i18n.T("Paused. Resume lets the team go on.")
	case stalled(snapshot, dream) && managerMissing(snapshot, dream):
		return i18n.T("The manager's model cannot run now.")
	case stalled(snapshot, dream):
		return i18n.T("The manager is not running. Resume starts it again.")
	case dream.Dream.Phase == protocol.DreamPresenting:
		return i18n.T("The manager presented the dream and waits for your answer.")
	case task.Waiting == protocol.WaitingOnYou || task.Status == protocol.TaskNeedsYou:
		return i18n.T("The manager waits for your answer to its question.")
	case task.Waiting == protocol.WaitingOnMembers:
		return i18n.Tf("The manager waits on its team: %s.", working(snapshot, dream))
	case task.Status == protocol.TaskWorking:
		return i18n.T("The manager is at work.")
	}
	return i18n.T("The manager starts when there is room.")
}

// nothing can run the model the manager is on: its provider is off, has no key, or its server is down
func managerMissing(snapshot state.Snapshot, dream protocol.Team) bool {
	lead, at := teamsui.Lead(dream.Members)
	return at >= 0 && teamsui.MemberEngine(snapshot, lead).Missing
}

// how many of the hires are at work, said whole
func working(snapshot state.Snapshot, dream protocol.Team) string {
	count := 0
	for _, task := range snapshot.TeamTasks {
		if task.Team == dream.ID && task.ID != dream.Dream.Task && task.Status == protocol.TaskWorking {
			count++
		}
	}
	switch count {
	case 0:
		return i18n.T("nobody is at work yet")
	case 1:
		return i18n.T("one hire at work")
	}
	return i18n.Tf("%s hires at work", look.Count(count))
}
