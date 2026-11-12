package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

type appAliasStatus struct {
	Cmd       string `json:"cmd"`
	Installed bool   `json:"installed"`
}

type appCandidateCatalog struct {
	Cmd       string           `json:"cmd"`
	Installed bool             `json:"installed"`
	Aliases   []appAliasStatus `json:"aliases"`
}

type appRoleCatalog struct {
	Role       string                `json:"role"`
	Candidates []appCandidateCatalog `json:"candidates"`
}

var readDefaultAppCatalog = func() ([]appRoleCatalog, error) {
	out, err := exec.Command("ryoku-hub", "apps").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ryoku-hub apps: %w: %s", err, strings.TrimSpace(string(out)))
	}
	var roles []appRoleCatalog
	if err := json.Unmarshal(out, &roles); err != nil {
		return nil, err
	}
	return roles, nil
}

var saveDefaultAppStore = func(data []byte) error {
	out, err := exec.Command("ryoku-hub", "desktop", "save", string(data)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ryoku-hub desktop save: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func appAliasRepairPlan(raw []byte, catalog []appRoleCatalog) ([]byte, []string, error) {
	root := map[string]any{}
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, nil, err
	}
	desktop, _ := root["desktop"].(map[string]any)
	apps, _ := desktop["apps"].(map[string]any)
	if len(apps) == 0 {
		return raw, nil, nil
	}

	roles := make(map[string]appRoleCatalog, len(catalog))
	for _, role := range catalog {
		roles[role.Role] = role
	}
	changes := make([]string, 0)
	for role, value := range apps {
		current, ok := value.(string)
		if !ok || current == "" {
			continue
		}
		definition, ok := roles[role]
		if !ok {
			continue
		}
		for _, candidate := range definition.Candidates {
			for _, alias := range candidate.Aliases {
				if alias.Cmd != current || alias.Installed || !candidate.Installed || candidate.Cmd == current {
					continue
				}
				apps[role] = candidate.Cmd
				changes = append(changes, fmt.Sprintf("%s: %s -> %s", role, current, candidate.Cmd))
			}
		}
	}
	if len(changes) == 0 {
		return raw, nil, nil
	}
	sort.Strings(changes)
	updated, err := json.Marshal(root)
	return updated, changes, err
}

func reconcileDefaultAppAliases(checkOnly bool) recResult {
	store := filepath.Join(sys.ConfigHome(), "ryoku", "desktop.json")
	raw, err := os.ReadFile(store)
	if os.IsNotExist(err) {
		return okRes(i18n.T("no saved default app roles need repair"))
	}
	if err != nil {
		return warnRes(i18n.T("could not read saved default app roles: %v"), err)
	}
	catalog, err := readDefaultAppCatalog()
	if err != nil {
		return warnRes(i18n.T("could not inspect default app aliases: %v"), err).withFix("ryoku update")
	}
	updated, changes, err := appAliasRepairPlan(raw, catalog)
	if err != nil {
		return warnRes(i18n.T("could not inspect saved default app roles: %v"), err).withFix("Ryoku Settings > Apps > Default apps")
	}
	if len(changes) == 0 {
		return okRes(i18n.T("saved default app aliases are current"))
	}
	if checkOnly {
		return wouldRes(i18n.T("saved default app commands use missing aliases: %s"), strings.Join(changes, ", ")).
			withFix(i18n.T("ryoku doctor repairs them through Ryoku Settings"))
	}
	if err := saveDefaultAppStore(updated); err != nil {
		return failRes(i18n.T("could not repair default app aliases: %v"), err).withFix("ryoku doctor")
	}
	return fixedRes(i18n.T("repaired default app aliases and regenerated desktop settings: %s"), strings.Join(changes, ", "))
}
