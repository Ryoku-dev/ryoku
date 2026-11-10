package vault

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	i18n "ryoku-i18n"
)

type layoutRow struct {
	path    string
	files   int
	purpose string
}

func RepoIndex(root, output string) error {
	body, err := RepoIndexDoc(root)
	if err != nil {
		return err
	}
	return atomicWrite(output, []byte(body), 0o644)
}

func RepoIndexDoc(root string) (string, error) {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf(i18n.T("not a directory: %s"), root)
	}
	var body strings.Builder
	if commit, branch, date := gitFacts(root); commit != "" {
		body.WriteString("## Checkout\n\n")
		fmt.Fprintf(&body, "- Commit: %s (%s)\n", commit, branch)
		if date != "" {
			fmt.Fprintf(&body, "- Committed: %s\n", date)
		}
		fmt.Fprintf(&body, "- Indexed: %s\n\n", time.Now().Format("2006-01-02 15:04"))
	}
	body.WriteString("## Layout\n\n| Path | Files | Purpose |\n|---|---|---|\n")
	for _, row := range repoLayout(root) {
		fmt.Fprintf(&body, "| `%s` | %d | %s |\n", row.path, row.files, row.purpose)
	}
	body.WriteString("\n## Key entry points\n\n| Concern | Path |\n|---|---|\n")
	for _, entry := range [][2]string{
		{"Desktop config", "ryoku/hyprland/ and ryoku/niri/"},
		{"Window-manager seam", "ryoku/wm/"},
		{"Shell daemon", "ryoku/shell/ipc/"},
		{"Shell UI", "ryoku/shell/quickshell/"},
		{"Ryoku Settings", "ryoku/hub/"},
		{"User CLI and vault", "ryoku/cli/"},
		{"Rashin", "ryoku/rashin/"},
		{"Installer", "installation/"},
		{"Packages", "release/packages/"},
	} {
		path := strings.TrimSuffix(strings.Split(entry[1], " and ")[0], "/")
		if _, err := os.Stat(filepath.Join(root, path)); err == nil {
			fmt.Fprintf(&body, "| %s | `%s` |\n", entry[0], entry[1])
		}
	}
	return body.String(), nil
}

func gitFacts(root string) (commit, branch, date string) {
	run := func(args ...string) string {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		output, err := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...).Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(output))
	}
	return run("rev-parse", "--short", "HEAD"), run("rev-parse", "--abbrev-ref", "HEAD"), run("log", "-1", "--format=%cs")
}

func repoLayout(root string) []layoutRow {
	purposes := map[string]string{
		"ryoku":                 "desktop, apps, shell, settings, Rashin and the vault",
		"system":                "boot, hardware policy, package sets and services",
		"installation":          "installer and ISO",
		"release":               "packages, repositories and signing",
		"docs":                  "architecture and operating guides",
		"bin":                   "repository tooling",
		"tests":                 "standalone integration checks",
		".githooks":             "commit and push gates",
		"ryoku-shell-installer": "installer for an existing system",
	}
	skip := map[string]bool{".git": true, "node_modules": true, ".worktrees": true, "local": true, "vendor": true}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	rows := make([]layoutRow, 0, len(purposes))
	for _, entry := range entries {
		purpose, known := purposes[entry.Name()]
		if !entry.IsDir() || !known {
			continue
		}
		count := 0
		_ = filepath.WalkDir(filepath.Join(root, entry.Name()), func(path string, item os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if item.IsDir() && skip[item.Name()] && path != filepath.Join(root, entry.Name()) {
				return filepath.SkipDir
			}
			if !item.IsDir() {
				count++
			}
			return nil
		})
		rows = append(rows, layoutRow{entry.Name() + "/", count, purpose})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].files > rows[j].files })
	return rows
}
