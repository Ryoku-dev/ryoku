package discipline

import (
	"cmp"
	"slices"

	"ryoku-rashin/internal/oni/window"
)

// Desktop is the separate nested desktop a desktop task drives, as its
// outputs were when the task started. The nested desktop is Hyakki's own,
// started for the task; the person's live desktop is only read to see
// whether they turned to Hyakki's own window.
type Desktop struct{ outputs []window.Output }

// CaptureDesktop records the outputs of the nested desktop in nested. It
// refuses with ReasonMissing when the nested desktop reports no output.
func CaptureDesktop(nested window.Snapshot) (Desktop, Reason) {
	if len(nested.Outputs) == 0 {
		return Desktop{}, ReasonMissing
	}
	return Desktop{outputs: sorted(nested.Outputs)}, Accepted
}

// Scene is what the away rule reads at one moment. A nil snapshot is one
// that could not be read.
type Scene struct {
	// Presenter is true while a presenter is drawn.
	Presenter bool
	// Live is the person's own desktop, read only.
	Live *window.Snapshot
	// Nested is the nested desktop the task drives.
	Nested *window.Snapshot
}

// Check applies the away rule and returns OK only when it holds; otherwise,
// in this order: no presenter is drawn; the live desktop cannot be read, or
// one of Hyakki's own windows (self lists their app ids) has focus there;
// the nested desktop cannot be read, or its outputs changed; more than one
// nested window reports focus; a private window, or one of Hyakki's own, has
// focus in the nested desktop. The zero Desktop never passes.
func (d Desktop) Check(sc Scene, self []string) Pause {
	if len(d.outputs) == 0 {
		return PauseOutput
	}
	if !sc.Presenter {
		return PausePresenter
	}
	if sc.Live == nil {
		return PauseWindow
	}
	for _, w := range sc.Live.Windows {
		if w.Focused && Self(w, self) {
			return PauseSelf
		}
	}
	if sc.Nested == nil {
		return PauseWindow
	}
	if !slices.Equal(d.outputs, sorted(sc.Nested.Outputs)) {
		return PauseOutput
	}
	f, n := focused(*sc.Nested)
	switch {
	case n > 1:
		return PauseWindow
	case n == 1:
		return foreign(f, self, OK)
	}
	return OK
}

// sorted returns a copy of outputs ordered by name, so two reads compare
// whatever order a compositor lists them in.
func sorted(outputs []window.Output) []window.Output {
	out := slices.Clone(outputs)
	slices.SortFunc(out, func(a, b window.Output) int { return cmp.Compare(a.Name, b.Name) })
	return out
}
