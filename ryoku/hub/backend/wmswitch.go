package main

import (
	"fmt"
	"strings"

	wm "ryoku-wm"
)

// wm pending remembers a window-manager switch the user confirmed, so Settings
// can say a sign-in is still needed until the chosen one is running.
//
//	wm pending              {"target","args"} while a switch waits, else null
//	wm pending set <name>   remember a confirmed switch
//	wm pending clear        forget it (staying on the current one)

type wmPendingReport struct {
	Target string   `json:"target"`
	Args   []string `json:"args"`
}

func wmPendingCmd(args []string) error {
	if len(args) == 0 {
		return printJSON(wmPending())
	}
	switch args[0] {
	case "set":
		if len(args) < 2 || !wmKnown(args[1]) {
			return fmt.Errorf("wm pending set needs a known window manager")
		}
		return setWMSwitch(args[1])
	case "clear":
		return setWMSwitch("")
	default:
		return fmt.Errorf("wm pending needs set|clear or nothing")
	}
}

func setWMSwitch(name string) error {
	c := loadConfig()
	c.UI.WMSwitch = name
	return saveConfig(c)
}

// wmPending is nil once the chosen window manager runs (the stale note is
// dropped then) and while it is not yet ready to start, such as during its install
func wmPending() *wmPendingReport {
	target := loadConfig().UI.WMSwitch
	if target == "" || !wmKnown(target) {
		return nil
	}
	if wm.Detect().Name == target {
		_ = setWMSwitch("")
		return nil
	}
	if !wmDeployed(target) && !pkgInstalled(wmPackage(target)) {
		return nil
	}
	return &wmPendingReport{Target: target, Args: []string{wmDisplayName(target)}}
}

func wmDisplayName(name string) string {
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}
