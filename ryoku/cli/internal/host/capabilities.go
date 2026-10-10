package host

import "encoding/json"

type capability struct {
	Supported bool   `json:"supported"`
	Reason    string `json:"reason"`
}

type hostCapabilities struct {
	PackageManager PackageManager `json:"packageManager"`
	Init           InitSystem     `json:"init"`
	Snapshots      capability     `json:"snapshots"`
	AUR            capability     `json:"aur"`
}

func (a *App) Capabilities() int {
	manager, err := a.PackageManager()
	if err != nil {
		return a.failf("%v", err)
	}
	init, err := a.Init()
	if err != nil {
		return a.failf("%v", err)
	}
	snapshots, snapshotReason := a.Snapshots()
	aur := manager == Pacman
	aurReason := ""
	if !aur {
		aurReason = "The AUR is an Arch Linux service; Void installs come from XBPS."
	}
	value := hostCapabilities{
		PackageManager: manager,
		Init:           init,
		Snapshots:      capability{Supported: snapshots, Reason: snapshotReason},
		AUR:            capability{Supported: aur, Reason: aurReason},
	}
	if err := json.NewEncoder(a.cfg.Stdout).Encode(value); err != nil {
		return a.failf("encode capabilities: %v", err)
	}
	return ExitOK
}
