package vault

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"ryoku-cli/internal/sys"
	wm "ryoku-wm"
)

type Status struct {
	Path          string
	Exists        bool
	Current       bool
	Wired         bool
	OptedOut      bool
	LegacyPresent bool
}

func Inspect() Status {
	info, err := os.Stat(Path())
	return Status{
		Path:          Path(),
		Exists:        err == nil && info.IsDir(),
		Current:       generatedCurrent(),
		Wired:         Wired(),
		OptedOut:      IsOptedOut(),
		LegacyPresent: LegacyPresent(),
	}
}

func Refresh() error {
	if err := Migrate(); err != nil {
		return err
	}
	if err := Ensure(); err != nil {
		return err
	}
	for _, doc := range []struct{ name, header, body string }{
		{"system.md", "# System\n\nHardware, kernel, GPU, displays and disks. Generated data inside the fence is replaced by `ryoku vault refresh`.", systemBody()},
		{"desktop.md", "# Ryoku desktop map\n\nSubsystem owners, paths, reload commands and GUI entry points. Read this before searching the filesystem.", desktopBody()},
		{"packages.md", "# Packages\n\nInstalled package inventory and pending updates on this machine.", packagesBody()},
		{"ownership.md", "# Ownership\n\nWho writes each config path and where a lasting change belongs.", ownershipBody()},
		{"logs.md", "# Logs\n\nExact commands for reading each desktop subsystem's logs.", logsBody()},
		{"user.md", "# User-owned changes\n\nWhere this machine differs from the shipped Ryoku config baseline.", userBody()},
		{"ryoku-repo.md", "# Ryoku source map\n\nThe source tree that produced this system.", repoBody()},
	} {
		if err := writeGenerated(doc.name, doc.header, doc.body); err != nil {
			return err
		}
	}
	if err := atomicWrite(versionPath(), []byte(versionID()+"\n"), 0o644); err != nil {
		return err
	}
	if !IsOptedOut() {
		return Wire()
	}
	return nil
}

func generatedCurrent() bool {
	if strings.TrimSpace(readFile(versionPath())) != versionID() {
		return false
	}
	for _, name := range generatedFiles {
		if !fileExists(filepath.Join(Path(), name)) {
			return false
		}
	}
	return fileExists(filepath.Join(Path(), "AGENTS.md"))
}

func versionID() string {
	if value := strings.TrimSpace(os.Getenv("RYOKU_VAULT_VERSION")); value != "" {
		return value
	}
	if repo := os.Getenv("RYOKU_REPO"); repo != "" {
		if data, err := os.ReadFile(filepath.Join(repo, "VERSION")); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	if repo := sys.ResolveRepo(); repo != "" {
		if data, err := os.ReadFile(filepath.Join(repo, "VERSION")); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	if release := readFile("/etc/ryoku-release"); release != "" {
		for _, line := range strings.Split(release, "\n") {
			if value, ok := strings.CutPrefix(line, "VERSION="); ok {
				return strings.TrimSpace(value)
			}
		}
	}
	if version := sys.InstalledVersion(); version != "" {
		return version
	}
	return "dev"
}

func systemBody() string {
	model, cores := cpuInfo()
	var body strings.Builder
	body.WriteString("## CPU\n\n")
	fmt.Fprintf(&body, "- Model: %s\n- Logical cores: %d\n\n", valueOrUnknown(model), cores)
	body.WriteString("## Memory\n\n")
	fmt.Fprintf(&body, "- Total: %s\n\n", memoryTotal())
	body.WriteString("## Kernel\n\n")
	fmt.Fprintf(&body, "- Release: %s\n\n", valueOrUnknown(probeLine(3, "uname", "-r")))
	body.WriteString("## GPU\n\n")
	gpus := gpuRows()
	if len(gpus) == 0 {
		body.WriteString("- No GPU reported\n\n")
	} else {
		for _, row := range gpus {
			fmt.Fprintf(&body, "- %s\n", row)
		}
		body.WriteString("\n")
	}
	body.WriteString("## Displays\n\n")
	outputs := monitorOutputs()
	if len(outputs) == 0 {
		body.WriteString("- No active output reported\n\n")
	} else {
		for _, output := range outputs {
			fmt.Fprintf(&body, "- %s: %dx%d, scale %g\n", output.Name, output.Width, output.Height, output.Scale)
		}
		body.WriteString("\n")
	}
	body.WriteString("## Disks\n\n")
	body.WriteString(diskTable())
	return body.String()
}

func cpuInfo() (string, int) {
	data := readFile("/proc/cpuinfo")
	model := ""
	cores := 0
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "processor") {
			cores++
		}
		if model == "" && strings.HasPrefix(line, "model name") {
			_, model, _ = strings.Cut(line, ":")
			model = strings.TrimSpace(model)
		}
	}
	return model, cores
}

func memoryTotal() string {
	for _, line := range strings.Split(readFile("/proc/meminfo"), "\n") {
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			value, _ := strconv.ParseInt(fields[1], 10, 64)
			return humanBytes(value * 1024)
		}
	}
	return "unknown"
}

func gpuRows() []string {
	output := probeLine(5, "lspci")
	var rows []string
	for _, line := range strings.Split(output, "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "vga compatible controller") || strings.Contains(lower, "3d controller") || strings.Contains(lower, "display controller") {
			rows = append(rows, strings.TrimSpace(line))
		}
	}
	return rows
}

func monitorOutputs() []wm.Output {
	client := wm.Open()
	if !client.Available() {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var outputs []wm.Output
	_ = client.Watch(ctx, func(frame wm.Frame) {
		if frame.Kind == wm.FrameOutputs {
			outputs = frame.Outputs
			cancel()
		}
	})
	return outputs
}

func diskTable() string {
	output := probeLine(5, "lsblk", "-J", "-o", "NAME,MOUNTPOINT,SIZE,FSUSED")
	var document struct {
		Devices []struct {
			Name       string  `json:"name"`
			Mountpoint *string `json:"mountpoint"`
			Size       *string `json:"size"`
			Used       *string `json:"fsused"`
			Children   []struct {
				Name       string  `json:"name"`
				Mountpoint *string `json:"mountpoint"`
				Size       *string `json:"size"`
				Used       *string `json:"fsused"`
			} `json:"children"`
		} `json:"blockdevices"`
	}
	if json.Unmarshal([]byte(output), &document) != nil {
		return "No block-device map available\n"
	}
	var body strings.Builder
	body.WriteString("| Device | Mount | Size | Used |\n|---|---|---|---|\n")
	count := 0
	write := func(name string, mountpoint, size, used *string) {
		if mountpoint == nil || *mountpoint == "" {
			return
		}
		fmt.Fprintf(&body, "| %s | %s | %s | %s |\n", name, *mountpoint, pointerValue(size), pointerValue(used))
		count++
	}
	for _, device := range document.Devices {
		write(device.Name, device.Mountpoint, device.Size, device.Used)
		for _, child := range device.Children {
			write(child.Name, child.Mountpoint, child.Size, child.Used)
		}
	}
	if count == 0 {
		return "No mounted block device reported\n"
	}
	return body.String()
}

func desktopBody() string {
	var body strings.Builder
	body.WriteString("## Acting on this desktop\n\nUse the linked `ryoku` skill and name the GUI path before its command fallback. Run `ryoku owner <path>` before editing config.\n\n")
	body.WriteString("## GUI map\n\n| Want to change | Open | Command |\n|---|---|---|\n")
	for _, row := range [][3]string{
		{"Wallpaper and palette", "Wallpaper picker; Ryoku Settings > Appearance > Wallpaper & colours", "`ryogami wallpaper ui`"},
		{"Bar style", "Ryoku Settings > Desktop > Bar", "`ryoku-shell barstyle <id>`"},
		{"Dock layout", "Stage Editor > Dock", "`ryoku-shell dock list`"},
		{"Desktop settings", "Ryoku Settings (Super + ,)", "`ryoku-shell hub open <page>[/<setting>]`"},
	} {
		fmt.Fprintf(&body, "| %s | %s | %s |\n", row[0], row[1], row[2])
	}
	body.WriteString("\n## Subsystem map\n\n| Subsystem | Config path | Owner | Reload |\n|---|---|---|---|\n")
	if detected := wm.Detect(); detected.Name != "" {
		fmt.Fprintf(&body, "| Window manager (%s) | `~/.config/%s/` | provider config | `ryoku wm act config.reload` |\n", detected.Name, wm.ConfigDir(detected.Name))
	}
	for _, row := range [][4]string{
		{"Shell, widgets and Ryoku Settings", "`~/.config/quickshell/`", "ryoku-desktop and ryoku-shell", "`ryoku reload`"},
		{"Ryoku state", "`~/.config/ryoku/`", "owning Ryoku command", "command-specific"},
		{"Terminal", "`~/.config/kitty/`", "kitty", "reopen terminal"},
		{"Shell and prompt", "`~/.config/{fish,bash,zsh}/`", "the selected shell", "open a new shell"},
		{"Vault", "`~/.local/share/ryoku/vault/`", "ryoku core", "`ryoku vault refresh`"},
	} {
		fmt.Fprintf(&body, "| %s | %s | %s | %s |\n", row[0], row[1], row[2], row[3])
	}
	body.WriteString(barDockBody())
	return body.String()
}

func barDockBody() string {
	var body strings.Builder
	body.WriteString("\n## Bar and dock\n\nThe active bar style and installed shell items are recorded in `~/.local/state/ryoku/store/shells.json`. Switch styles in Ryoku Settings > Desktop > Bar or with `ryoku-shell barstyle <id>`. The dock is shared by every style and is managed with `ryoku-shell dock show|hide|edge|autohide|pin|unpin|list`.\n")
	path := filepath.Join(sys.Xdg("XDG_STATE_HOME", ".local/state"), "ryoku", "store", "shells.json")
	var index struct {
		Active string `json:"active"`
		Items  []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Category string `json:"category"`
		} `json:"items"`
	}
	if json.Unmarshal([]byte(readFile(path)), &index) == nil && len(index.Items) > 0 {
		body.WriteString("\n| id | name | category | active |\n|---|---|---|---|\n")
		for _, item := range index.Items {
			active := "no"
			if item.ID == index.Active {
				active = "yes"
			}
			fmt.Fprintf(&body, "| %s | %s | %s | %s |\n", item.ID, item.Name, item.Category, active)
		}
	}
	return body.String()
}

func packagesBody() string {
	var body strings.Builder
	count := probeLine(5, "ryoku-host", "pkg", "count")
	body.WriteString("## Installed\n\n")
	if count == "" {
		body.WriteString("Native package database unavailable\n")
	} else {
		fmt.Fprintf(&body, "%s packages\n", strings.TrimSpace(count))
	}
	body.WriteString("\n## Explicit packages\n\n")
	explicit := explicitPackages()
	if len(explicit) == 0 {
		body.WriteString("Explicit package list unavailable\n")
	} else {
		for _, name := range explicit {
			fmt.Fprintf(&body, "- `%s`\n", name)
		}
	}
	body.WriteString("\n## Pending updates\n\n")
	var status struct {
		SystemUpdates int `json:"systemUpdates"`
	}
	if json.Unmarshal([]byte(probeLine(8, "ryoku", "status", "--json")), &status) == nil {
		fmt.Fprintf(&body, "%d packages\n", status.SystemUpdates)
	} else {
		body.WriteString("Update count unavailable\n")
	}
	return body.String()
}

func explicitPackages() []string {
	type probe struct {
		name string
		args []string
	}
	probes := []probe{
		{name: "pacman", args: []string{"-Qqe"}},
		{name: "xbps-query", args: []string{"-m"}},
		{name: "dnf", args: []string{"repoquery", "--userinstalled", "--qf", "%{name}"}},
	}
	for _, candidate := range probes {
		if _, err := exec.LookPath(candidate.name); err != nil {
			continue
		}
		output := probeLine(12, candidate.name, candidate.args...)
		if output == "" {
			continue
		}
		seen := make(map[string]bool)
		var packages []string
		for _, line := range strings.Split(output, "\n") {
			name := strings.TrimSpace(line)
			if name != "" && !seen[name] {
				seen[name] = true
				packages = append(packages, name)
			}
		}
		sort.Strings(packages)
		return packages
	}
	return nil
}

func ownershipBody() string {
	output := probeLine(10, "ryoku", "owner", "--map")
	if strings.HasPrefix(strings.TrimSpace(output), "##") {
		return output
	}
	return "## The rule\n\nRun `ryoku owner <path>` before editing config. Files under `~/.config/quickshell/` and package-owned paths are replaced by updates. Use a dedicated user override, the `~/.config/ryoku/user_edits/` overlay, or the owning GUI or command instead.\n"
}

func logsBody() string {
	var body strings.Builder
	body.WriteString("## Desktop services\n\n")
	for _, unit := range []string{"ryoku-shell.service", "ryogami.service"} {
		fmt.Fprintf(&body, "- `%s`: `journalctl --user -u %s -e`\n", unit, unit)
	}
	body.WriteString("\n## Window manager\n\n")
	if detected := wm.Detect(); detected.Name != "" {
		fmt.Fprintf(&body, "- `%s`: `journalctl --user _COMM=%s -e`\n", detected.Name, detected.Name)
	} else {
		body.WriteString("- No provider detected\n")
	}
	body.WriteString("\n## Shell UI\n\n- Running instances: `qs list --all`\n- Current logs: `qs log -p ~/.config/quickshell/shell -t 200`\n")
	state := filepath.Join(sys.Xdg("XDG_STATE_HOME", ".local/state"), "ryoku")
	body.WriteString("\n## Update and health\n\n")
	fmt.Fprintf(&body, "- Update: `%s`\n- Doctor: `%s`\n- Health: `ryoku doctor --check`\n", filepath.Join(state, "update-log.txt"), filepath.Join(state, "doctor-report.txt"))
	body.WriteString("- Crashes: `coredumpctl list` and `coredumpctl info <binary>`\n")
	return body.String()
}

func userBody() string {
	base := sys.BaseConfigDir()
	config := sys.ConfigHome()
	info, err := os.Stat(base)
	if err != nil || !info.IsDir() {
		return "No shipped config baseline is available. Treat files under `~/.config` as user-owned until `ryoku owner <path>` says otherwise.\n"
	}
	var modified, missing []string
	_ = filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(base, path)
		if err != nil {
			return nil
		}
		live := filepath.Join(config, relative)
		if !fileExists(live) {
			missing = append(missing, filepath.ToSlash(relative))
		} else if !sameFile(path, live) {
			modified = append(modified, filepath.ToSlash(relative))
		}
		return nil
	})
	sort.Strings(modified)
	sort.Strings(missing)
	if len(modified) == 0 && len(missing) == 0 {
		return "The live config matches the shipped baseline.\n"
	}
	var body strings.Builder
	body.WriteString("Respect these differences as the user's choices.\n")
	if len(modified) > 0 {
		body.WriteString("\n## Modified shipped files\n\n")
		for _, path := range modified {
			fmt.Fprintf(&body, "- `~/.config/%s`\n", path)
		}
	}
	if len(missing) > 0 {
		body.WriteString("\n## Removed shipped files\n\n")
		for _, path := range missing {
			fmt.Fprintf(&body, "- `~/.config/%s`\n", path)
		}
	}
	return body.String()
}

func repoBody() string {
	for _, root := range []string{os.Getenv("RYOKU_REPO"), sys.ResolveRepo()} {
		if root == "" {
			continue
		}
		if body, err := RepoIndexDoc(root); err == nil {
			return body
		}
	}
	for _, root := range sourceRoots() {
		if body := readFile(filepath.Join(root, "ryoku-repo.md")); body != "" {
			return body
		}
	}
	return "No shipped source map is available. Run `ryoku vault repo-index <checkout> <output>` on a checkout.\n"
}

func sameFile(left, right string) bool {
	leftDigest := fileDigest(left)
	return leftDigest != "" && leftDigest == fileDigest(right)
}

func fileDigest(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return ""
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func probeLine(seconds int, name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func humanBytes(bytes int64) string {
	const gib = 1024 * 1024 * 1024
	if bytes >= gib {
		return fmt.Sprintf("%.1f GiB", float64(bytes)/gib)
	}
	return fmt.Sprintf("%d MiB", bytes/(1024*1024))
}

func valueOrUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return strings.TrimSpace(value)
}

func pointerValue(value *string) string {
	if value == nil || *value == "" {
		return "-"
	}
	return *value
}
