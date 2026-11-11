package command

import (
	"os"
	"path/filepath"
	"strings"

	"ryoku-rashin/internal/paths"
)

// SecretPath reports paths Ask must never expose to a model. It checks both
// the name supplied by the caller and the resolved target because either side
// of a symlink can identify credential-bearing state.
func SecretPath(path string) (string, bool) {
	if marker, blocked := secretPathName(path); blocked {
		return marker, true
	}
	if resolved, err := filepath.EvalSymlinks(filepath.Clean(paths.Expand(path))); err == nil {
		if marker, blocked := secretPathName(resolved); blocked {
			return marker, true
		}
	}
	return "", false
}

func secretPathName(path string) (string, bool) {
	clean := filepath.Clean(paths.Expand(strings.TrimSpace(path)))
	low := strings.ToLower(filepath.ToSlash(clean))
	if low == "." || low == "" {
		return "", false
	}

	if low == "/etc/shadow" || low == "/etc/gshadow" || low == "/etc/sudoers" {
		return low, true
	}
	padded := "/" + strings.Trim(low, "/") + "/"
	for _, item := range []struct {
		marker string
		name   string
	}{
		{"/.ssh/", "SSH data"},
		{"/.gnupg/", "GnuPG data"},
		{"/.password-store/", "password manager data"},
		{"/.mozilla/", "browser profile"},
		{"/.zen/", "browser profile"},
		{"/.librewolf/", "browser profile"},
		{"/.floorp/", "browser profile"},
		{"/.var/app/", "sandboxed application data"},
		{"/.hermes/", "harness credentials"},
		{"/.aws/", "AWS credentials"},
		{"/.kube/", "Kubernetes credentials"},
		{"/.docker/config.json/", "Docker credentials"},
	} {
		if strings.Contains(padded, item.marker) {
			return item.name, true
		}
	}
	if low == "/dev/null" || low == "/dev/zero" || low == "/dev/urandom" {
		return "", false
	}
	if low == "/dev" || strings.HasPrefix(low, "/dev/") {
		return "/dev", true
	}
	if low == "/sys/kernel/debug" || strings.HasPrefix(low, "/sys/kernel/debug/") {
		return "/sys/kernel/debug", true
	}
	parts := strings.Split(strings.TrimPrefix(low, "/"), "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] == "proc" && (parts[i+2] == "environ" || parts[i+2] == "mem" || parts[i+2] == "cmdline") {
			return "/proc/*/" + parts[i+2], true
		}
	}

	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" && os.Getenv("HOME") != "" {
		stateHome = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	if stateHome != "" {
		privateRoot := strings.ToLower(filepath.ToSlash(filepath.Join(stateHome, "ryoku", "rashin")))
		if low == privateRoot || strings.HasPrefix(low, privateRoot+"/") {
			return "Rashin private state", true
		}
	}

	home := filepath.ToSlash(filepath.Clean(os.Getenv("HOME")))
	if home != "." && home != "" {
		homeLow := strings.ToLower(home)
		for _, item := range []struct {
			rel  string
			name string
		}{
			{".ssh", "SSH data"},
			{".gnupg", "GnuPG data"},
			{".password-store", "password manager data"},
			{".local/share/keyrings", "password manager data"},
			{".config/bitwarden", "password manager data"},
			{".config/1password", "password manager data"},
			{".config/keepassxc", "password manager data"},
			{".mozilla", "browser profile"},
			{".zen", "browser profile"},
			{".librewolf", "browser profile"},
			{".floorp", "browser profile"},
			{".var/app", "sandboxed application data"},
			{".docker/config.json", "Docker credentials"},
			{".aws", "AWS credentials"},
			{".git-credentials", "Git credentials"},
			{".kube", "Kubernetes credentials"},
			{".netrc", "network credentials"},
			{".config/gh/hosts.yml", "GitHub credentials"},
			{".config/rclone", "rclone credentials"},
			{".claude/.credentials.json", "Claude credentials"},
			{".claude.json", "Claude credentials"},
			{".codex/auth.json", "Codex credentials"},
			{".gemini/oauth_creds.json", "Gemini credentials"},
			{".config/opencode/auth.json", "OpenCode credentials"},
			{".config/omp/auth.json", "harness credentials"},
		} {
			root := homeLow + "/" + item.rel
			if low == root || strings.HasPrefix(low, root+"/") {
				return item.name, true
			}
		}
	}

	for _, marker := range []string{
		"/keyrings/", "/chromium/", "/google-chrome/", "/brave/", "/vivaldi/",
		"/discord/", "/vesktop/", "/slack/", "/element/", "/signal/", "/telegramdesktop/",
	} {
		if strings.Contains(low, marker) {
			return strings.Trim(marker, "/"), true
		}
	}
	base := filepath.Base(low)
	if strings.HasSuffix(base, ".pem") || strings.HasSuffix(base, ".key") ||
		strings.HasSuffix(base, ".kdbx") || strings.HasPrefix(base, "id_") ||
		base == ".env" || base == ".netrc" || base == ".git-credentials" ||
		base == "credentials" || base == "credentials.json" || base == "auth.json" ||
		base == "logins.json" || base == "key4.db" || base == "cookies.sqlite" {
		return base, true
	}
	for _, marker := range []string{"token", "secret", "api-key", "apikey", "password"} {
		if strings.Contains(base, marker) {
			return marker, true
		}
	}
	return "", false
}
