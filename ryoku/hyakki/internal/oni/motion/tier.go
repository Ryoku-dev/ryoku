package motion

// Activity is the frame-tier activity (see spec.ActivityIDs): asleep in the
// sleeping state, motion while anything time-bounded or hand-driven moves,
// idle when settled in the idle state, breathing otherwise.
func (e *Engine) Activity() string {
	switch {
	case e.state == "sleeping":
		return "asleep"
	case e.moving() || e.in.Drag != nil || e.in.Speech > 0:
		return "motion"
	case e.state == "idle":
		return "idle"
	}
	return "breathing"
}

func (e *Engine) fps() int {
	fps, _ := e.s.TargetFPS(e.Activity(), e.in.BatterySaver)
	return fps
}
