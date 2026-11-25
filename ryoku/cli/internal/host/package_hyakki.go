package host

import (
	"fmt"
	"sort"
	"strings"
)

// PacmanHyakkiReplacementFlags enables libalpm's replacement pass only for
// the requested transaction. Every other installed or served package remains
// ignored, including unsolicited replacers. Unrelated conflicts stay default-no.
func (a *App) PacmanHyakkiReplacementFlags(targets []string) ([]string, error) {
	requested := false
	for _, target := range targets {
		if target == "hyakki" || strings.HasSuffix(target, "/hyakki") {
			requested = true
			break
		}
	}
	if !requested {
		return nil, nil
	}
	legacy := a.query("pacman", a.pacmanTransactionArgs("-Qq", "ryoku-rashin")...)
	if legacy.Code == 1 || legacy.Code == 0 && strings.TrimSpace(legacy.Output) != "ryoku-rashin" {
		return nil, nil
	}
	if legacy.Code != 0 {
		return nil, fmt.Errorf("cannot inspect the installed legacy Hyakki package (pacman exited %d)", legacy.Code)
	}
	query := func(action string, args ...string) ([]string, error) {
		result := a.query("pacman", a.pacmanTransactionArgs(action, args...)...)
		if result.Code != 0 {
			return nil, fmt.Errorf("cannot prepare the Hyakki package replacement (%s exited %d)", action, result.Code)
		}
		return strings.Fields(result.Output), nil
	}
	local, err := query("-Qq")
	if err != nil {
		return nil, err
	}
	args := append([]string{"--noconfirm", "--print-format", "%n"}, targets...)
	resolved, err := query("-Sp", args...)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]bool, len(resolved)+1)
	for _, name := range resolved {
		allowed[name] = true
	}
	if !allowed["hyakki"] {
		return nil, fmt.Errorf("the Hyakki replacement is absent from pacman's resolved transaction")
	}
	allowed["ryoku-rashin"] = true
	served, err := query("-Slq")
	if err != nil {
		return nil, err
	}
	ignored := make(map[string]bool, len(local)+len(served))
	for _, names := range [][]string{local, served} {
		for _, name := range names {
			if !allowed[name] {
				ignored[name] = true
			}
		}
	}
	names := make([]string, 0, len(ignored))
	for name := range ignored {
		names = append(names, name)
	}
	sort.Strings(names)
	flags := []string{"-u"}
	// Linux limits each argv string independently of the total argument size.
	start, size := 0, 0
	for i, name := range names {
		if size+len(name)+1 > 60*1024 {
			flags = append(flags, "--ignore", strings.Join(names[start:i], ","))
			start, size = i, 0
		}
		size += len(name) + 1
	}
	if start < len(names) {
		flags = append(flags, "--ignore", strings.Join(names[start:], ","))
	}
	return flags, nil
}
