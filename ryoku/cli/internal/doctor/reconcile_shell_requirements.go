package doctor

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	i18n "ryoku-i18n"
)

type shellRequirementResult struct {
	Missing   []json.RawMessage `json:"missing"`
	Installed []json.RawMessage `json:"installed"`
	FellBack  []json.RawMessage `json:"fellBack"`
}

var shellRequirementCommand = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

func queryShellRequirements(args ...string) (shellRequirementResult, error) {
	output, err := shellRequirementCommand("ryostore", args...)
	if err != nil {
		if detail := strings.TrimSpace(string(output)); detail != "" {
			return shellRequirementResult{}, fmt.Errorf("%s", detail)
		}
		return shellRequirementResult{}, err
	}
	var result shellRequirementResult
	if err := json.Unmarshal(output, &result); err != nil {
		return shellRequirementResult{}, fmt.Errorf(i18n.T("decode ryostore response: %w"), err)
	}
	return result, nil
}

func reconcileShellRequirements(checkOnly bool) recResult {
	current, err := queryShellRequirements("internal", "shells-require", "--json")
	if err != nil {
		return warnRes(i18n.T("could not inspect selected shells and parts: %v"), err).
			withFix(i18n.T("ryostore internal shells-require --install --fallback"))
	}
	if len(current.Missing) == 0 {
		return okRes(i18n.T("selected shells and parts are installed"))
	}
	if checkOnly {
		return wouldRes(i18n.T("selected shells or parts are not installed")).
			withFix(i18n.T("ryoku doctor installs available items and applies safe fallbacks"))
	}

	fixed, err := queryShellRequirements("internal", "shells-require", "--install", "--fallback", "--json")
	if err != nil {
		return failRes(i18n.T("could not install selected shells or apply fallbacks: %v"), err).
			withFix(i18n.T("ryostore internal shells-require --install --fallback"))
	}
	changed := len(fixed.Installed)+len(fixed.FellBack) > 0
	if !changed {
		return warnRes(i18n.T("selected shells or parts remain unavailable")).
			withFix(i18n.T("ryostore internal shells-require --install --fallback"))
	}
	if output, err := shellRequirementCommand("ryoku-shell", "shells", "commit"); err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		return failRes(i18n.T("installed selected shells or applied fallbacks, but could not commit the selection: %s"), detail).
			withFix(i18n.T("ryoku-shell shells commit"))
	}
	return fixedRes(i18n.T("installed selected shells and applied safe fallbacks"))
}
