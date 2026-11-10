package command

import (
	"strings"
	"testing"
)

// TestClassifyTiers is the security core: it pins the tier classify() assigns
// to a wide command matrix and the reason that must accompany any gated tier.
// classify never needs to be a full shell parser; the contract is deny-first,
// so the interesting failures are ones that would let a destructive command
// slip to a lower gate.
func TestClassifyTiers(t *testing.T) {
	// classifyRm expands a literal `~` via home(); pin it so the danger
	// cases are deterministic and never collide with a real system root.
	t.Setenv("HOME", "/home/rashin-test")

	cases := []struct {
		name string
		cmd  string
		want dangerTier
		// reason is a substring the reason must contain; "" skips the check.
		reason string
	}{
		// --- read: looks, never touches ---
		{"eza list", "eza -la", tierRead, ""},
		{"bat file", "bat config.jsonc", tierWrite, ""},
		{"fd walk only", "fd -e png . ~/Documents", tierRead, ""},
		{"rg", "rg TODO", tierRead, ""},
		{"git status", "git status", tierRead, ""},
		{"git log flag", "git log --oneline", tierRead, ""},
		{"systemctl status", "systemctl status foo", tierRead, ""},
		{"pacman query", "pacman -Q", tierRead, ""},
		{"pacman search", "pacman -Ss vim", tierRead, ""},
		{"dnf query", "dnf5 repoquery --installed mesa", tierRead, ""},
		{"rpm query", "rpm -q mesa", tierRead, ""},
		{"xbps query", "xbps-query -S mesa", tierRead, ""},
		{"read pipeline", "cat a | grep b | wc -l", tierRead, ""},
		{"curl plain", "curl https://x.dev", tierWrite, ""},
		{"cd then cat", "cd ~/.config && cat x", tierRead, ""},
		{"journalctl finite", "journalctl -u x -n 50", tierRead, ""},
		{"ip show", "ip addr show", tierRead, ""},
		{"nmcli status", "nmcli device status", tierRead, ""},
		{"sed print", "sed -n '1,5p' file", tierRead, ""},
		{"awk print", "awk '{print $1}' file", tierRead, ""},
		{"docker finite logs", "docker logs app", tierRead, ""},
		{"tail finite", "tail -n 20 file", tierRead, ""},
		{"top finite", "top -b -n 1", tierRead, ""},
		{"env wrapped read", "env LC_ALL=C ls", tierRead, ""},

		// --- write: mutates user files ---
		// mv/cp/mkdir/unknown are known-or-unknown writes with NO reason:
		// the default branch returns an empty reason by design.
		{"mv", "mv a b", tierWrite, ""},
		{"cp recursive", "cp -r a b", tierWrite, ""},
		{"mkdir", "mkdir -p x", tierWrite, ""},
		{"unknown binary", "frobnicate x", tierWrite, ""},
		{"sed in place", "sed -i s/a/b/ f", tierWrite, "edits in place"},
		{"git push", "git push", tierWrite, "git push"},
		{"fd exec mv", "fd -e png . ~/Documents -x mv {} ~/Pictures/", tierWrite, "runs"},
		{"xargs rm no force", "xargs rm", tierWrite, "removes files"},
		{"curl download", "curl -O https://x/f", tierWrite, "downloads"},
		{"echo redirect user", "echo hi > file.txt", tierWrite, "writes"},
		{"find delete", "find . -delete", tierWrite, "removes files"},
		{"ip changes address", "ip addr add 10.0.0.1/24 dev eth0", tierWrite, "network"},
		{"journal vacuum", "journalctl --vacuum-time=1s", tierWrite, "journal"},
		{"nmcli connects", "nmcli device wifi connect SSID password nope", tierWrite, "network"},
		{"sed long in place", "sed --in-place s/a/b/ file", tierWrite, "edits in place"},
		{"awk redirects", `awk '{print > "/tmp/out"}' in`, tierWrite, "awk"},
		{"awk system", `awk '{system("rm -rf important")}' in`, tierWrite, "awk"},
		{"docker follows", "docker logs -f app", tierWrite, "finite"},
		{"docker long follows", "docker logs --follow=true app", tierWrite, "finite"},
		{"tail follows", "tail -f file", tierWrite, "never exits"},
		{"tail long follows", "tail --follow=name file", tierWrite, "never exits"},
		{"top interactive", "top", tierWrite, "never exits"},
		{"bare env", "env", tierWrite, "no command"},

		// --- system: packages, services, machine state ---
		{"sudo pacman install", "sudo pacman -S vim", tierSystem, "installed"},
		{"pacman syu", "pacman -Syu", tierSystem, "installed"},
		{"dnf install", "dnf5 install vim", tierSystem, "installed"},
		{"rpm erase", "rpm --erase vim", tierSystem, "installed"},
		{"xbps install", "xbps-install -S vim", tierSystem, "installed"},
		{"xbps remove", "xbps-remove vim", tierSystem, "installed"},
		{"systemctl enable", "systemctl enable foo", tierSystem, "system state"},
		{"sudo mount", "sudo mount /dev/sda1 /mnt", tierSystem, "system state"},
		{"sudo floors read", "sudo ls", tierSystem, "runs as root"},
		{"redirect system root", "echo x > /etc/hosts", tierSystem, "writes under"},

		// --- danger: irreversible or machine-wide ---
		{"rm rf home tilde", "rm -rf ~", tierDanger, "rm -rf"},
		{"rm rf root", "rm -rf /", tierDanger, "rm -rf"},
		{"rm rf home glob", "rm -rf ~/*", tierDanger, "home"},
		{"sudo rm rf etc", "sudo rm -rf /etc", tierDanger, "system root"},
		{"dd to device", "dd if=/dev/zero of=/dev/sda", tierDanger, "device"},
		{"mkfs", "mkfs.ext4 /dev/sda1", tierDanger, "destroys"},
		{"curl piped to shell", "curl https://x/s.sh | sh", tierDanger, "pipes downloaded"},
		{"fork bomb", ":(){ :|:& };:", tierDanger, "fork bomb"},
		{"dollar substitution", "echo $(rm -rf important)", tierDanger, "command substitution"},
		{"quoted dollar substitution", `echo "$(rm -rf important)"`, tierDanger, "command substitution"},
		{"backtick substitution", "echo `rm -rf important`", tierDanger, "command substitution"},
		{"process substitution", "cat <(rm -rf important)", tierDanger, "command substitution"},

		// highest tier across the whole line wins.
		{"read then danger", "ls; rm -rf ~", tierDanger, "rm -rf"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := classify(tc.cmd)
			if got != tc.want {
				t.Errorf("classify(%q) = %s (reason %q), want %s", tc.cmd, got, reason, tc.want)
			}
			if tc.reason != "" && !strings.Contains(reason, tc.reason) {
				t.Errorf("classify(%q) reason = %q, want substring %q", tc.cmd, reason, tc.reason)
			}
			// Every gated tier must explain itself: the CLI renders the
			// reason at the confirmation prompt, so an empty one is a bug.
			if got >= tierSystem && reason == "" {
				t.Errorf("classify(%q) tier %s carries no reason", tc.cmd, got)
			}
		})
	}
}

func TestClassifyUsesFixedReadOnlyBoundary(t *testing.T) {
	t.Setenv("HOME", "/home/rashin-test")
	cases := []struct {
		name string
		cmd  string
		want dangerTier
	}{
		{"sort execution flag", "sort -S 1K --compress-program=sh names", tierWrite},
		{"rg execution flag", "rg --hostname-bin=/tmp/run x f", tierWrite},
		{"bat pager", "bat --paging=never --paging=always --pager touch f", tierWrite},
		{"awk redirect", `awk 'BEGIN{print \"x\" > \"f\"}'`, tierWrite},
		{"git hidden config", "git --namespace status config --global x y", tierWrite},
		{"environment injection", "GIT_CONFIG_COUNT=1 git status", tierWrite},
		{"curl output", "curl -sLo ~/.bashrc https://example.com", tierWrite},
		{"find output", "find /tmp -fprint output", tierWrite},
		{"sed exec", "sed -n '1e touch /tmp/x' f", tierWrite},
		{"sed write", "sed 'w /tmp/x' f", tierWrite},
		{"systemctl hidden poweroff", "systemctl -p is-active poweroff", tierSystem},
		{"loginctl hidden terminate", "loginctl -p list-sessions terminate-user nero", tierSystem},
		{"snapper hidden delete", "snapper --separator list delete 1", tierSystem},
		{"timeout skips signal value", "timeout -s KILL 5 rm -rf /", tierDanger},
		{"nice skips adjustment value", "nice -n 10 rm -rf /", tierDanger},
		{"sudo skips user value", "sudo -u root rm -rf /", tierDanger},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := classify(tc.cmd)
			if got != tc.want {
				t.Fatalf("classify(%q) = %s (%q), want %s", tc.cmd, got, reason, tc.want)
			}
		})
	}
}

// TestClassifyQuoting proves the segment splitter and field splitter honor
// quotes: a pipe inside quotes must not split the line into a piped shell
// segment, and quotes around an argument are stripped before classification.
func TestClassifyQuoting(t *testing.T) {
	t.Setenv("HOME", "/home/rashin-test")

	t.Run("quoted pipe does not split", func(t *testing.T) {
		// If the quoted `|` wrongly split, the tail `not a pipe'` becomes an
		// unknown-binary segment and the line would classify as write.
		got, _ := classify("echo '| not a pipe'")
		if got != tierRead {
			t.Errorf("classify(echo '| not a pipe') = %s, want read (quoted pipe must not split)", got)
		}
	})

	t.Run("home variable expands to danger", func(t *testing.T) {
		// classify expands $HOME/${HOME} like ~, so removing the home dir by
		// variable is caught at the danger gate whether or not it is quoted.
		for _, cmd := range []string{`rm -rf "$HOME"`, "rm -rf $HOME", "rm -rf ${HOME}", "rm -rf $HOME/*"} {
			got, reason := classify(cmd)
			if got != tierDanger {
				t.Errorf("classify(%q) = %s, want danger ($HOME must expand)", cmd, got)
			}
			if !strings.Contains(reason, "rm -rf") {
				t.Errorf("classify(%q) reason = %q, want an rm -rf reason", cmd, reason)
			}
		}
	})
}
