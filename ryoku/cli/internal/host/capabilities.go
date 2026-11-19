package host

import (
	"encoding/json"
	"os"
	"strings"

	i18n "ryoku-i18n"
)

type capability struct {
	Supported bool   `json:"supported"`
	Reason    string `json:"reason"`
}

type hostCapabilities struct {
	PackageManager PackageManager  `json:"packageManager"`
	Init           InitSystem      `json:"init"`
	PackageTable   string          `json:"packageTable,omitempty"`
	Edition        string          `json:"edition,omitempty"`
	Snapshots      capability      `json:"snapshots"`
	AUR            capability      `json:"aur"`
	Users          usersCapability `json:"users"`
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
		switch manager {
		case XBPS:
			aurReason = i18n.T("The AUR is an Arch Linux service; Void installs come from XBPS.")
		case DNF:
			aurReason = i18n.T("The AUR is an Arch Linux service; Fedora installs come from DNF.")
		}
	}
	value := hostCapabilities{
		PackageManager: manager,
		Init:           init,
		Edition:        a.commandEdition(),
		Snapshots:      capability{Supported: snapshots, Reason: snapshotReason},
		AUR:            capability{Supported: aur, Reason: aurReason},
		Users:          a.usersCapability(),
	}
	if manager == DNF {
		value.PackageTable = a.fedoraTablePath()
	}
	if err := json.NewEncoder(a.cfg.Stdout).Encode(value); err != nil {
		return a.failf(i18n.T("encode capabilities: %v"), err)
	}
	return ExitOK
}

func (a *App) commandEdition() string {
	path := a.getenv("RYOKU_OS_RELEASE")
	if path == "" {
		path = "/etc/os-release"
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var id, like string
	for rest := string(body); rest != ""; {
		line, remaining, _ := strings.Cut(rest, "\n")
		rest = remaining
		key, value, _ := strings.Cut(strings.TrimSpace(line), "=")
		value = strings.Trim(strings.TrimSpace(value), "\"'")
		switch key {
		case "ID":
			id = value
		case "ID_LIKE":
			like = value
		}
	}
	for _, candidates := range [2]string{id, like} {
		for candidates != "" {
			candidate, rest, _ := strings.Cut(candidates, " ")
			candidates = rest
			switch candidate {
			case "arch", "cachyos", "fedora", "void":
				return candidate
			}
		}
	}
	return ""
}
