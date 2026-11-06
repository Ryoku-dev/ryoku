package profile

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	i18n "ryoku-i18n"
)

func UserMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	_ = stdin
	if len(args) == 0 {
		return userError(stderr, i18n.T("Choose a profile action."))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return userError(stderr, err.Error())
	}
	switch args[0] {
	case "status":
		return userStatus(home, args[1:], stdout, stderr)
	case "scope":
		return userScope(home, args[1:], stdout, stderr)
	case "reset":
		return userReset(home, args[1:], stdout, stderr)
	case "check":
		return userCheck(home, args[1:], stdout, stderr)
	case "first-run":
		return userFirstRun(home, args[1:], stderr)
	case "session-stop":
		return userSessionStop(args[1:], stderr)
	case "lay":
		return userLay(home, args[1:], stdout, stderr)
	default:
		return userError(stderr, fmt.Sprintf(i18n.T("Unknown profile action: %s"), args[0]))
	}
}

func userStatus(home string, args []string, stdout, stderr io.Writer) int {
	jsonOut, ok := onlyJSON(args)
	if !ok {
		return userError(stderr, i18n.T("profile status accepts only --json."))
	}
	r, err := ReadAccountReceipt(home)
	if err != nil {
		if os.IsNotExist(err) {
			if jsonOut {
				writeJSON(stdout, map[string]any{"active": false})
				return 0
			}
			fmt.Fprintln(stdout, i18n.T("No profile is active."))
			return 0
		}
		return userError(stderr, err.Error())
	}
	if jsonOut {
		writeJSON(stdout, map[string]any{"active": true, "receipt": r})
		return 0
	}
	fmt.Fprintf(stdout, i18n.T("Profile %s %s is active.\n"), r.Profile.ID, r.Profile.Version)
	return 0
}

func userScope(home string, args []string, stdout, stderr io.Writer) int {
	dry, jsonOut, ok := dryJSON(args)
	if !ok {
		return userError(stderr, i18n.T("profile scope accepts only --dry-run and --json."))
	}
	r, m, err := accountContext(home)
	if err != nil {
		return userError(stderr, err.Error())
	}
	entries, err := ScanDesktopEntries(SystemApplicationDirs())
	if err != nil {
		return userError(stderr, err.Error())
	}
	installed := installedFromSystemReceipt(m.ID, filepath.Base(home))
	allow := Allowed(m, installed)
	var added, removed []string
	if dry {
		added, removed, err = scopePreview(home, entries, allow, r)
	} else {
		added, removed, err = ApplyScope(home, entries, allow, r)
		if err == nil {
			err = WriteAccountReceipt(home, r)
		}
	}
	if err != nil {
		return userError(stderr, err.Error())
	}
	result := map[string]any{"hidden": hiddenIDs(r.Scope.Hidden), "added": added, "removed": removed}
	if jsonOut {
		writeJSON(stdout, result)
	} else {
		fmt.Fprintf(stdout, i18n.T("Launcher scope: %d hidden, %d added, %d removed.\n"), len(r.Scope.Hidden), len(added), len(removed))
	}
	return 0
}

func userReset(home string, args []string, stdout, stderr io.Writer) int {
	dry, jsonOut, ok := dryJSON(args)
	if !ok {
		return userError(stderr, i18n.T("profile reset accepts only --dry-run and --json."))
	}
	r, m, err := accountContext(home)
	if err != nil {
		return userError(stderr, err.Error())
	}
	changes, err := ResetHome(home, filepath.Join(r.Payload, "home"), m, Vars{Home: home, User: filepath.Base(home), ID: m.ID}, r, dry)
	if err != nil {
		return userError(stderr, err.Error())
	}
	if !dry {
		if err := WriteAccountReceipt(home, r); err != nil {
			return userError(stderr, err.Error())
		}
	}
	restored := []string{}
	for _, change := range changes {
		if change.Action == "restore" {
			restored = append(restored, change.Path)
		}
	}
	result := map[string]any{"restored": restored, "unchanged": len(r.Files) - len(restored)}
	if jsonOut {
		writeJSON(stdout, result)
	} else {
		fmt.Fprintf(stdout, i18n.T("Restored %d profile files.\n"), len(restored))
	}
	return 0
}

func userCheck(home string, args []string, stdout, stderr io.Writer) int {
	session, jsonOut, ok := sessionJSON(args)
	if !ok {
		return userError(stderr, i18n.T("profile check accepts only --session and --json."))
	}
	r, m, err := accountContext(home)
	if err != nil {
		if session {
			exec.Command("logger", "-t", "ryoku-profile", err.Error()).Run()
			return 0
		}
		return userError(stderr, err.Error())
	}
	result := Check(home, m, r, session)
	if err := WriteCheck(home, result); err != nil && !session {
		return userError(stderr, err.Error())
	}
	if session {
		for _, finding := range result.Failures {
			exec.Command("logger", "-t", "ryoku-profile", finding.Kind+": "+finding.Detail).Run()
		}
	}
	if jsonOut {
		writeJSON(stdout, result)
	} else if result.OK {
		fmt.Fprintln(stdout, i18n.T("Profile check passed."))
	} else {
		fmt.Fprintf(stdout, i18n.T("Profile check found %d problems.\n"), len(result.Failures))
	}
	if session {
		return 0
	}
	if !result.OK {
		return 1
	}
	return 0
}

func userLay(home string, args []string, stdout, stderr io.Writer) int {
	payload, systemPath, jsonOut, ok := layArgs(args)
	if !ok {
		return userError(stderr, i18n.T("profile lay needs --payload and --system-receipt."))
	}
	m, err := LoadManifest(payload)
	if err != nil {
		return userError(stderr, err.Error())
	}
	body, err := os.ReadFile(systemPath)
	if err != nil {
		return userError(stderr, err.Error())
	}
	var system SystemReceipt
	if err := json.Unmarshal(body, &system); err != nil {
		return userError(stderr, err.Error())
	}
	if system.Profile.ID != m.ID || system.Profile.Version != m.Version {
		return userError(stderr, i18n.T("The staged profile does not match its system receipt."))
	}
	prev, err := ReadAccountReceipt(home)
	if os.IsNotExist(err) {
		prev = nil
	} else if err != nil {
		return userError(stderr, err.Error())
	}
	self := system.Account.Name
	v := Vars{Home: home, User: self, ID: m.ID}
	r, changes, err := LayHome(home, filepath.Join(payload, "home"), m, v, prev)
	if err != nil {
		return userError(stderr, err.Error())
	}
	desktopPath := filepath.Join(home, ".config", "ryoku", "desktop.json")
	desktopBody, readErr := os.ReadFile(desktopPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return userError(stderr, readErr.Error())
	}
	var oldOwned []string
	if prev != nil {
		oldOwned = prev.EnvKeys
	}
	merged, owned, err := MergeDesktopEnv(desktopBody, oldOwned, DesktopEnv(m, v))
	if err != nil {
		return userError(stderr, err.Error())
	}
	if err := writeHomeFile(home, ".config/ryoku/desktop.json", merged, 0o644); err != nil {
		return userError(stderr, err.Error())
	}
	envRel := filepath.ToSlash(filepath.Join(".config", "environment.d", "zz-profile-"+m.ID+".conf"))
	if err := writeHomeFile(home, envRel, EnvironmentD(m, v), 0o644); err != nil {
		return userError(stderr, err.Error())
	}
	r.EnvKeys, r.EnvFile = owned, envRel
	if err := writeHomeFile(home, ".local/state/ryoku/default-rice-pending", []byte(m.Rice+"\n"), 0o600); err != nil {
		return userError(stderr, err.Error())
	}
	entries, err := ScanDesktopEntries(SystemApplicationDirs())
	if err != nil {
		return userError(stderr, err.Error())
	}
	installed := append(append([]string{}, system.Packages.Introduced...), system.Packages.PreExisting...)
	added, removed, err := ApplyScope(home, entries, Allowed(m, installed), r)
	if err != nil {
		return userError(stderr, err.Error())
	}
	check := Check(home, m, r, false)
	if err := WriteCheck(home, check); err != nil {
		return userError(stderr, err.Error())
	}
	if err := WriteAccountReceipt(home, r); err != nil {
		return userError(stderr, err.Error())
	}
	result := map[string]any{"ok": true, "changes": changes, "scope": map[string]any{"hidden": hiddenIDs(r.Scope.Hidden), "added": added, "removed": removed}, "check": check}
	if jsonOut {
		writeJSON(stdout, result)
	} else {
		fmt.Fprintln(stdout, i18n.T("Profile account is ready."))
	}
	return 0
}

func userFirstRun(home string, args []string, stderr io.Writer) int {
	if len(args) != 0 {
		return userError(stderr, i18n.T("profile first-run does not accept arguments."))
	}
	r, m, err := accountContext(home)
	if err != nil {
		return userError(stderr, err.Error())
	}
	_ = r
	marker := filepath.Join(home, ".local", "state", "ryoku", "profile", "first-run-done")
	if _, err := os.Stat(marker); err == nil {
		return 0
	}
	if err := writeHomeFile(home, ".local/state/ryoku/profile/first-run-done", []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o600); err != nil {
		return userError(stderr, err.Error())
	}
	if len(m.FirstRun.Command) > 0 {
		cmd := exec.Command(m.FirstRun.Command[0], m.FirstRun.Command[1:]...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := cmd.Start(); err == nil {
			cmd.Process.Release()
		}
	}
	if m.FirstRun.Message != "" {
		cmd := exec.Command("timeout", "5s", "notify-send", m.FirstRun.Message)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := cmd.Start(); err == nil {
			cmd.Process.Release()
		}
	}
	return 0
}

func userSessionStop(args []string, stderr io.Writer) int {
	if len(args) != 0 {
		return userError(stderr, i18n.T("profile session-stop does not accept arguments."))
	}
	current, err := user.Current()
	if err != nil {
		return userError(stderr, err.Error())
	}
	body, err := exec.Command("nmcli", "-t", "-f", "UUID,TYPE", "connection", "show", "--active").Output()
	if err != nil {
		return userError(stderr, err.Error())
	}
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		uuid, kind, ok := strings.Cut(line, ":")
		if !ok || kind != "vpn" {
			continue
		}
		permissions, err := exec.Command("nmcli", "-g", "connection.permissions", "connection", "show", "uuid", uuid).Output()
		if err != nil || !strings.Contains(string(permissions), "user:"+current.Username) {
			continue
		}
		_ = exec.Command("nmcli", "connection", "down", "uuid", uuid).Run()
	}
	return 0
}

func accountContext(home string) (*AccountReceipt, *Manifest, error) {
	r, err := ReadAccountReceipt(home)
	if err != nil {
		return nil, nil, err
	}
	m, err := LoadManifest(r.Payload)
	return r, m, err
}
func installedFromSystemReceipt(id, account string) []string {
	r, err := ReadSystemReceipt(SystemRoot, id, account)
	if err != nil {
		return nil
	}
	return append(append([]string{}, r.Packages.Introduced...), r.Packages.PreExisting...)
}
func hiddenIDs(entries []HiddenEntry) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.ID)
	}
	return out
}
func onlyJSON(args []string) (bool, bool) {
	jsonOut := false
	for _, arg := range args {
		if arg != "--json" || jsonOut {
			return false, false
		}
		jsonOut = true
	}
	return jsonOut, true
}
func dryJSON(args []string) (bool, bool, bool) {
	dry, jsonOut := false, false
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			if dry {
				return false, false, false
			}
			dry = true
		case "--json":
			if jsonOut {
				return false, false, false
			}
			jsonOut = true
		default:
			return false, false, false
		}
	}
	return dry, jsonOut, true
}
func sessionJSON(args []string) (bool, bool, bool) {
	session, jsonOut := false, false
	for _, arg := range args {
		switch arg {
		case "--session":
			if session {
				return false, false, false
			}
			session = true
		case "--json":
			if jsonOut {
				return false, false, false
			}
			jsonOut = true
		default:
			return false, false, false
		}
	}
	return session, jsonOut, true
}
func layArgs(args []string) (string, string, bool, bool) {
	payload, receipt, jsonOut := "", "", false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--payload":
			i++
			if i >= len(args) {
				return "", "", false, false
			}
			payload = args[i]
		case "--system-receipt":
			i++
			if i >= len(args) {
				return "", "", false, false
			}
			receipt = args[i]
		case "--json":
			jsonOut = true
		default:
			return "", "", false, false
		}
	}
	return payload, receipt, jsonOut, payload != "" && receipt != ""
}
func userError(stderr io.Writer, message string) int { fmt.Fprintln(stderr, message); return 1 }
func writeJSON(w io.Writer, value any) {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(value)
}

func scopePreview(home string, entries []DesktopEntry, allow map[string]bool, r *AccountReceipt) ([]string, []string, error) {
	previous := map[string]HiddenEntry{}
	for _, entry := range r.Scope.Hidden {
		previous[entry.ID] = entry
	}
	present := map[string]bool{}
	added, removed := []string{}, []string{}
	next := []HiddenEntry{}
	for _, entry := range entries {
		present[entry.ID] = true
		if allow[entry.ID] || entry.NoDisplay || entry.Hidden {
			if _, ok := previous[entry.ID]; ok {
				removed = append(removed, entry.ID)
			}
			continue
		}
		digest, err := hashFile(entry.Path)
		if err != nil {
			return nil, nil, err
		}
		old, exists := previous[entry.ID]
		if !exists {
			rel := filepath.ToSlash(filepath.Join(".local", "share", "applications", entry.ID))
			dst, pathErr := safeDestination(home, rel)
			if pathErr != nil {
				return nil, nil, pathErr
			}
			if _, statErr := os.Lstat(dst); statErr == nil {
				continue
			} else if !os.IsNotExist(statErr) {
				return nil, nil, statErr
			}
		}
		if !exists || old.SourceSHA256 != digest {
			added = append(added, entry.ID)
		}
		next = append(next, HiddenEntry{ID: entry.ID, SourceSHA256: digest})
	}
	for id := range previous {
		if !present[id] {
			removed = append(removed, id)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	r.Scope.Hidden = next
	return added, removed, nil
}
