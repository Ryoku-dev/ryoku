package doctor

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

func nomarchySelected() bool {
	body, err := os.ReadFile(filepath.Join(sys.ConfigHome(), "ryoku", "shell.json"))
	if err != nil {
		return false
	}
	var store struct {
		BarStyle string `json:"barStyle"`
	}
	return json.Unmarshal(body, &store) == nil && store.BarStyle == "nomarchy"
}

func nomarchySourceRoot() string {
	for _, root := range []string{
		os.Getenv("RYOKU_NOMARCHY_SOURCE"),
		filepath.Join(sys.Xdg("XDG_DATA_HOME", ".local/share"), "ryoku", "nomarchy-package"),
		"/usr/share/ryoku/nomarchy",
	} {
		if root == "" {
			continue
		}
		if info, err := os.Stat(filepath.Join(root, "nomarchy-activate")); err == nil && !info.IsDir() {
			return root
		}
	}
	return ""
}

func runNomarchyLifecycle(root, name string, check bool) error {
	args := []string{}
	if check {
		args = append(args, "--check")
	}
	cmd := exec.Command(filepath.Join(root, name), args...)
	env := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "OMARCHY_PATH=") {
			continue
		}
		env = append(env, entry)
	}
	cmd.Env = append(env, "OMARCHY_PATH="+filepath.Join(sys.Xdg("XDG_DATA_HOME", ".local/share"), "ryoku", "nomarchy"))
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if check {
		return err
	}
	if text := strings.TrimSpace(string(out)); text != "" {
		return errors.New(text)
	}
	return err
}

func reconcileNomarchyCommands(checkOnly bool) recResult {
	root := nomarchySourceRoot()
	if root == "" {
		return warnRes(i18n.T("Nomarchy compatibility assets are not installed"))
	}
	name := "nomarchy-deactivate"
	state := i18n.T("inactive")
	if nomarchySelected() {
		name = "nomarchy-activate"
		state = i18n.T("active")
	}
	if runNomarchyLifecycle(root, name, true) == nil {
		return okRes(i18n.T("Nomarchy command surface is %s"), state)
	}
	if checkOnly {
		return wouldRes(i18n.T("Nomarchy command surface does not match the selected bar style")).
			withFix(i18n.T("ryoku doctor matches the Nomarchy command surface to the selected bar style"))
	}
	if err := runNomarchyLifecycle(root, name, false); err != nil {
		return failRes(i18n.T("could not reconcile Nomarchy commands: %v"), err)
	}
	return fixedRes(i18n.T("matched Nomarchy commands to the selected bar style"))
}
