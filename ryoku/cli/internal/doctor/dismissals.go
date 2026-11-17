package doctor

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

type dismissedFinding struct {
	Status string `json:"status"`
	Detail string `json:"detail"`
	Remedy string `json:"remedy,omitempty"`
}

type dismissals map[string]dismissedFinding

func dismissalPath() string {
	return filepath.Join(sys.StateDir(), "dismissed-findings.json")
}

func loadDismissals() (dismissals, error) {
	out := dismissals{}
	body, err := os.ReadFile(dismissalPath())
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = dismissals{}
	}
	return out, nil
}

func saveDismissals(saved dismissals) error {
	body, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	return sys.WriteFileAtomic(dismissalPath(), string(body)+"\n", 0o600)
}

func dismissalFor(result recResult) dismissedFinding {
	return dismissedFinding{
		Status: result.status.label(),
		Detail: strings.Join(strings.Fields(result.detail), " "),
		Remedy: strings.Join(strings.Fields(result.remedy), " "),
	}
}

func (saved dismissals) matches(name string, result recResult) bool {
	return (result.status == recWarn || result.status == recWouldFix) && saved[name] == dismissalFor(result)
}

func runWithDismissals(r reconciler, checkOnly bool, saved dismissals) recResult {
	if _, exists := saved[r.name]; exists {
		result := r.run(true)
		if saved.matches(r.name, result) {
			result.status = recDismissed
			return result
		}
		if checkOnly {
			return result
		}
	}
	return r.run(checkOnly)
}

func manageDismissals(action, name string, checks []reconciler, wantJSON bool) error {
	saved, err := loadDismissals()
	if err != nil {
		return fmt.Errorf("read doctor dismissals: %w", err)
	}
	switch action {
	case "list":
		names := make([]string, 0, len(saved))
		for name := range saved {
			names = append(names, name)
		}
		sort.Strings(names)
		findings := make([]finding, 0, len(names))
		for _, name := range names {
			findings = append(findings, finding{name: name, res: recResult{status: recDismissed, detail: saved[name].Detail}})
		}
		if wantJSON {
			return emitFindingsJSON(findings)
		}
		if len(findings) == 0 {
			fmt.Println(i18n.T("No dismissed findings"))
			return nil
		}
		printFindings(findings, true)
		return nil
	case "restore":
		if _, exists := saved[name]; !exists {
			return fmt.Errorf(i18n.T("no dismissal saved for %q"), name)
		}
		delete(saved, name)
	case "dismiss":
		found := false
		for _, r := range checks {
			if r.name != name {
				continue
			}
			result := r.run(true)
			if result.status != recWarn && result.status != recWouldFix {
				return fmt.Errorf(i18n.T("%q is %s; only warn and todo findings can be dismissed"), name, result.status.label())
			}
			saved[name] = dismissalFor(result)
			found = true
			break
		}
		if !found {
			return fmt.Errorf(i18n.T("unknown check %q; use ryoku doctor --check to list check names"), name)
		}
	}
	if err := saveDismissals(saved); err != nil {
		return fmt.Errorf("save doctor dismissals: %w", err)
	}
	fmt.Printf("%s: %s\n", action, name)
	return nil
}
