package teamsui

// a change sent to the daemon shows at once and holds until the daemon's
// state agrees, so a control never snaps back mid-call
type Held[T comparable] struct {
	value T
	set   bool
}

// the value to show: the one sent while the daemon has not caught up, else actual
func (h *Held[T]) Show(actual T) T {
	if h.set && h.value == actual {
		h.set = false
	}
	if h.set {
		return h.value
	}
	return actual
}

func (h *Held[T]) Hold(value T) { h.value, h.set = value, true }

// the daemon refused the change, so the control shows what is
func (h *Held[T]) Drop(err error) {
	if err != nil {
		h.set = false
	}
}
