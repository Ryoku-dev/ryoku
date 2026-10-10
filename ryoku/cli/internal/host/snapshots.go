package host

const (
	snapshotsUnavailableReason       = "Snapshots are not available on this host, so updates cannot be rolled back."
	fedoraSnapshotsUnavailableReason = "Snapshots are not available on Fedora: it boots through GRUB, which Ryoku's snapshot boot menu does not drive."
)

// Snapshots reports whether this host provides Ryoku's snapper-based safety net.
func (a *App) Snapshots() (supported bool, reason string) {
	manager, err := a.PackageManager()
	if err == nil && (manager == Pacman || manager == XBPS) {
		return true, ""
	}
	if err == nil && manager == DNF {
		return false, fedoraSnapshotsUnavailableReason
	}
	return false, snapshotsUnavailableReason
}
