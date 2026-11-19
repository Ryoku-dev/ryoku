package command

import (
	"os"
	"path/filepath"
	"strings"

	"ryoku-rashin/internal/paths"
)

// SecretPath checks the supplied name and its resolved destination.
func SecretPath(path string) (string, bool) {
	return SecretPathAt(path, "")
}

// SecretPathAt resolves relative names against the tool session, not rashind.
func SecretPathAt(path, cwd string) (string, bool) {
	expanded := paths.Expand(strings.TrimSpace(path))
	if !filepath.IsAbs(expanded) && cwd != "" {
		expanded = filepath.Join(cwd, expanded)
	}
	if marker, blocked := secretPathName(expanded); blocked {
		return marker, true
	}
	ancestor := filepath.Clean(expanded)
	var suffix []string
	hops := 0
	for {
		info, statErr := os.Lstat(ancestor)
		if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			target, readErr := os.Readlink(ancestor)
			if readErr == nil {
				if !filepath.IsAbs(target) {
					target = filepath.Join(filepath.Dir(ancestor), target)
				}
				for i := len(suffix) - 1; i >= 0; i-- {
					target = filepath.Join(target, suffix[i])
				}
				ancestor = filepath.Clean(target)
				suffix = nil
				hops++
				if hops > 40 {
					return "unresolvable symlink chain", true
				}
				if marker, blocked := secretPathName(ancestor); blocked {
					return marker, true
				}
				continue
			}
			return "unreadable symlink target", true
		}
		if statErr != nil && !os.IsNotExist(statErr) {
			return "uninspectable file path", true
		}
		resolved, err := filepath.EvalSymlinks(ancestor)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return secretPathName(resolved)
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", false
		}
		suffix = append(suffix, filepath.Base(ancestor))
		ancestor = parent
	}
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
	for i, part := range parts {
		if part != "proc" || i+2 >= len(parts) {
			continue
		}
		name := parts[i+2]
		if name == "task" && i+4 < len(parts) {
			name = parts[i+4]
		}
		if name == "environ" || name == "mem" || name == "cmdline" {
			return "/proc/*/" + name, true
		}
	}

	privateRoot := strings.ToLower(filepath.ToSlash(paths.StateDir()))
	if low == privateRoot || strings.HasPrefix(low, privateRoot+"/") {
		return "Rashin private state", true
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
			{".npmrc", "npm credentials"},
			{".pypirc", "Python package credentials"},
			{".pgpass", "PostgreSQL credentials"},
			{".cargo/credentials.toml", "Cargo credentials"},
			{".config/gcloud", "cloud credentials"},
			{".config/bravesoftware", "browser profile"},
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
		base == ".env" || strings.HasPrefix(base, ".env.") && base != ".env.example" && base != ".env.sample" && base != ".env.template" ||
		strings.HasSuffix(base, ".env") || base == ".envrc" ||
		strings.HasSuffix(base, ".tfstate") || strings.HasSuffix(base, ".p12") || strings.HasSuffix(base, ".pfx") ||
		strings.HasSuffix(base, ".jks") || strings.HasSuffix(base, ".gpg") || strings.HasSuffix(base, ".ovpn") ||
		base == ".netrc" || base == ".git-credentials" ||
		base == "credentials" || base == "credentials.json" || base == ".credentials.json" || base == "auth.json" ||
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
