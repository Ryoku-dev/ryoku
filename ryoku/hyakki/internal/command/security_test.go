package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellClassificationBypassMatrix(t *testing.T) {
	t.Setenv("HOME", "/home/rashin-test")
	for _, tc := range []struct {
		command string
		tier    dangerTier
	}{
		{"echo hi # don't\nrm -rf ~", tierDanger},
		{"ls # it's\ntouch /tmp/x", tierWrite},
		{"git status # it's\ngit push --force", tierWrite},
		{"echo 'unterminated", tierDanger},
		{"echo hi # comment\ncat 'unterminated", tierDanger},
		{"echo <<EOF", tierDanger},
		{"echo { broken", tierDanger},
		{`bash -c "echo \"; rm -rf /"`, tierDanger},
		{"echo x#literal", tierRead},
		{"echo '# literal quote' # comment\nrm -rf /", tierDanger},
		{"bash -c 'rm -rf ~'", tierDanger},
		{"sh -c 'rm -rf /'", tierDanger},
		{"eval 'rm -rf ~'", tierDanger},
		{"(rm -rf /)", tierDanger},
		{"{ rm -rf ~; }", tierDanger},
		{"if true; then rm -rf /; fi", tierDanger},
		{"! rm -rf /", tierDanger},
		{"setsid rm -rf ~", tierDanger},
		{"X=~; rm -rf $X", tierDanger},
		{"echo 'rm -rf ~' | sh", tierDanger},
		{"bash -c 'mkfs.ext4 /dev/sda1'", tierDanger},
		{"find ~ -delete", tierDanger},
		{"find . -delete", tierDanger},
		{"find /tmp -exec rm {} +", tierDanger},
		{"find / -exec rm -rf {} +", tierDanger},
		{"find ~ -exec rm {} +", tierDanger},
		{"chmod -R 777 /", tierDanger},
		{"chown -R nobody /usr", tierDanger},
		{"echo $'\\''; rm -rf /; echo ''", tierDanger},
		{"resolvectl query data.attacker.example", tierWrite},
		{"docker -H ssh://evil ps", tierWrite},
		{"systemctl -H user@evil status", tierSystem},
		{"dmesg -w", tierDanger},
		{"dmesg --follow", tierDanger},
		{"cat /dev/zero", tierDanger},
		{"cat /dev/urandom", tierDanger},
		{"pactl -n info set-sink-volume @DEFAULT_SINK@ 0%", tierWrite},
		{"gsettings --schemadir get set org.x y z", tierWrite},
		{"pw-cli -r info destroy 1", tierWrite},
	} {
		t.Run(tc.command, func(t *testing.T) {
			got, _ := classify(tc.command)
			if got != tc.tier {
				t.Fatalf("classify(%q) = %s, want %s", tc.command, got, tc.tier)
			}
			if tc.tier != tierRead {
				if ok, _ := readOnlyCommand(tc.command); ok {
					t.Fatal("mutating command classified read-only")
				}
			}
		})
	}
}

func TestElevationBypassMatrix(t *testing.T) {
	for _, command := range []string{
		"eval 'sudo id'", "(sudo id)", "{ sudo id; }", "if true; then sudo id; fi",
		`find . -exec sudo id \;`, "fd . -x sudo id", "setsid sudo id", "S=sudo; $S id",
		"echo 'sudo id' | sh", "bash <<< 'sudo id'", "$'sudo' id",
		"script -qc 'sudo id' /dev/null", "echo hi # don't\nsudo id",
		"xargs sudo id", "sh -c 'eval sudo id'",
	} {
		t.Run(command, func(t *testing.T) {
			if !Elevates(command) {
				t.Fatal("elevated command escaped terminal gate")
			}
		})
	}
}

func TestCredentialBypassMatrix(t *testing.T) {
	t.Setenv("HOME", "/home/rashin-test")
	for _, command := range []string{
		"cat .env.local", "cat .env.production", "cat prod.env", "cat .envrc",
		"cd ~; cat .config/gh/hosts.yml", "cat .claude/.credentials.json",
		"grep -r . ~/.config/gh", "grep -rn . .", "grep -d recurse . ~",
		"cat /proc/self/task/123/environ", "git show HEAD:.env",
		"env", "printenv OPENAI_API_KEY", "set", "export -p", "declare -p",
		"eval 'cat .env.local'", "bash -c 'cat .env.local'", "echo hi # don't\ncat .env",
		"echo 'cat .env.local' | sh", "cd ~; cat .config/gh/hosts.yml",
		"cat ~/.npmrc", "cat ~/.pypirc", "cat ~/.pgpass", "cat ~/.cargo/credentials.toml",
		"cat harness-homes/team/.credentials.json",
		"cat ~/.config/gcloud/credentials.db", "cat state.tfstate", "cat ~/.config/BraveSoftware/Default/Login Data",
		"cat cert.p12", "cat cert.pfx", "cat store.jks", "cat backup.gpg", "cat vpn.ovpn",
	} {
		t.Run(command, func(t *testing.T) {
			if marker, secret := ReadsSecretAt(command, "/home/rashin-test"); !secret {
				t.Fatalf("ReadsSecretAt(%q) = false (%s)", command, marker)
			}
		})
	}
	for _, command := range []string{"cat .env.example", "cat .env.sample", "cat .env.template", "git commit -m 'fix token refresh'"} {
		if marker, secret := ReadsSecretAt(command, "/home/rashin-test/project"); secret {
			t.Errorf("ReadsSecretAt(%q) false positive %q", command, marker)
		}
	}
}

func TestReadOnlySensitiveOptionsAndStreams(t *testing.T) {
	for _, command := range []string{
		"jq -L dir 'import \"x\" as $c; $c'", "jq --library-path=dir . f",
		"resolvectl query evil.test", "systemctl --host=user@evil status", "loginctl -M remote list-sessions",
		"docker --context=remote ps", "pactl -n info set-sink-volume @DEFAULT_SINK@ 0%",
		"gsettings --schemadir get set org.x y z", "pw-cli -r info destroy 1",
		"dmesg -w", "dmesg --follow", "cat /dev/zero", "cat /dev/urandom",
	} {
		t.Run(command, func(t *testing.T) {
			if ok, reason := readOnlyCommand(command); ok {
				t.Fatalf("accepted unbounded, remote, or mutating command: %s", reason)
			}
		})
	}
}

func TestSecretPathResolvesAbsentChildrenAndSessionCWD(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	work := filepath.Join(home, "work")
	if err := os.Mkdir(work, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(home, ".ssh"), filepath.Join(work, "link")); err != nil {
		t.Fatal(err)
	}
	if _, secret := SecretPathAt("link/authorized_keys", work); !secret {
		t.Fatal("new file under symlinked secret tree was allowed")
	}
	if err := os.Symlink("/etc/shadow", filepath.Join(work, "shadow-link")); err != nil {
		t.Fatal(err)
	}
	if _, secret := ReadsSecretAt("cat shadow-link", work); !secret {
		t.Fatal("relative symlink target was not resolved against session cwd")
	}
	if _, secret := SecretPathAt(".env.local", work); !secret {
		t.Fatal("dotenv variant was not blocked")
	}
	if _, secret := SecretPathAt(filepath.Join(work, ".env.example"), work); secret {
		t.Fatal("example dotenv template was blocked")
	}
	if _, secret := SecretPathAt(strings.Join([]string{"/proc", "self", "task", "123", "environ"}, "/"), work); !secret {
		t.Fatal("thread environment was not blocked")
	}
}
