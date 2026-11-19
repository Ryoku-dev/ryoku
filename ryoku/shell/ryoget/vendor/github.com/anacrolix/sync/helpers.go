package sync

func withBlocked(f func()) {
	if !contentionOn {
		f()
		return
	}
	v := new(int)
	lockBlockers.Add(v, 0)
	f()
	lockBlockers.Remove(v)
}
