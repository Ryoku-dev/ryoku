package host

const snapshotsUnavailableReason = "Snapshots are not available on Void Linux, so updates cannot be rolled back."

// Snapshots reports whether this host provides Ryoku's snapper-based safety net.
func (a *App) Snapshots() (supported bool, reason string) {
	manager, err := a.PackageManager()
	if err == nil && manager == Pacman {
		return true, ""
	}
	return false, snapshotsUnavailableReason
}
