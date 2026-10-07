package main

import (
	"encoding/json"
	"testing"
)

func TestReadOnlyCommand(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		// Real agent commands that must run without asking.
		{"journalctl -b -3 -o short-iso --no-pager -n 60 2>&1 | tail -n 25; echo; journalctl -b -2 --no-pager -n 40", true},
		{"sed -n '/2026-09-24T1[5-9]/,/2026-09-25T03/p' /var/log/pacman.log", true},
		{"systemctl --user status ryogami.service --no-pager", true},
		{"pacman -Qi mesa", true},
		{"find ~/.cache -name '*.log' -mmin -60", true},
		{"du -xh --max-depth=2 / 2>/dev/null | sort -rh | head", true},
		{"cat /etc/hostname", true},
		{"grep -rn TODO src", true},
		{"grep 'a|b; c' f", true},
		{`grep "a|b" f`, true},
		{"rg --json 'func ' .", true},
		{"ls -la ~/.config/ryoku", true},
		{"stat /var/log/pacman.log", true},
		{"head -n 5 /etc/os-release", true},
		{"tail -n 100 /var/log/Xorg.0.log", true},
		{"wc -l < go.mod", true},
		{"awk '/error/{print $2}' /var/log/pacman.log", true},
		{"awk '{if ($3 > 100) print $1}' data", true},
		{"sed -n '1,20p' file", true},
		{"sed -n '$=' file", true},
		{"sed -n -e '/a/p' -e '/b/p' f", true},
		{"systemctl list-units --type=service --state=running --no-pager", true},
		{"systemctl", true},
		{"pacman -Ss firefox", true},
		{"pacman -Sl core", true},
		{"pacman -Ql mesa", true},
		{"checkupdates", true},
		{"git status", true},
		{"git log --oneline -20", true},
		{"git diff HEAD~1", true},
		{"git branch -a", true},
		{"journalctl -u NetworkManager --since '1 hour ago' --no-pager", true},
		{"ip -br addr show", true},
		{"ip a", true},
		{"nmcli device status", true},
		{"loginctl list-sessions", true},
		{"coredumpctl list", true},
		{"btrfs filesystem usage /", true},
		{"snapper -c root list", true},
		{"docker ps -a", true},
		{"docker logs mycontainer", true},
		{"docker stats --no-stream", true},
		{"curl -s https://example.com/api", true},
		{"curl -X GET https://example.com", true},
		{"top -b -n 1", true},
		{"free -h && df -h", true},
		{"FOO=1 BAR=2 env PATH=/usr/bin cat file", true},
		{"env LC_ALL=C sort names", true},
		{"echo hi > /dev/null", true},
		{"printf '%s\\n' done 2>&1", true},
		{"cat f <<< 'here string'", true},
		{"grep foo < input.txt", true},
		{"ryoku status", true},
		{"ryoku-rashin agent --json", true},
		{"ryoku-shell bar list", true},
		{"qs list", true},
		{"prowl search 'where is the chat panel'", true},
		{"ryoku doctor --check", true},
		{"nvidia-smi -q -d MEMORY", true},

		// Adversarial commands that must ask.
		{"cat f > /etc/x", false},
		{"ls; rm -rf ~", false},
		{"echo $(rm x)", false},
		{"echo `rm x`", false},
		{"find . -delete", false},
		{"find . -exec rm {} \\;", false},
		{"sed -i s/a/b/ f", false},
		{"sed -n 'w /tmp/x' f", false},
		{"sed 's/a/b/' f", false},
		{`awk '{print > "/tmp/x"}' f`, false},
		{"awk '{system(\"rm x\")}' f", false},
		{"journalctl --vacuum-time=1s", false},
		{"journalctl -f", false},
		{"systemctl restart ryogami.service", false},
		{"systemctl --user start foo", false},
		{"pacman -Syu", false},
		{"pacman -S foo", false},
		{"pacman -Rns foo", false},
		{"curl -o f https://x", false},
		{"curl -d name=x https://x", false},
		{"curl -X POST https://x", false},
		{"wget https://x", false},
		{"env", false},
		{"printenv", false},
		{"bash -c ls", false},
		{"sh -c 'rm x'", false},
		{"ls & rm x", false},
		{"cat <<EOF", false},
		{"tail -f log", false},
		{"uniq a b", false},
		{"sort -o f f", false},
		{"git push", false},
		{"git commit -am wip", false},
		{"ryoku update", false},
		{"ryoku doctor", false},
		{"ryoku-rashin agent use omp", false},
		{"ryoku-shell bar show", false},
		{"qs kill", false},
		{"prowl gateway start", false},
		{"sudo cat /etc/shadow", false},
		{"cat f | sh", false},
		{"dd if=/dev/zero of=/dev/sda", false},
		{"rm -rf /", false},
		{"less file", false},
		{"vim file", false},
		{"docker logs -f c", false},
		{"docker stats", false},
		{"nvidia-smi --gpu-reset", false},
		{"ip addr add 10.0.0.1/24 dev eth0", false},
		{"nmcli radio wifi off", false},
		{"mount /dev/sda1 /mnt", false},
		{"gsettings set org.x.y z", false},
		{"tee out.txt", false},
		{"cat f 2>err.log", false},
		{"( ls )", false},
		{"foo=1", false},

		// Secret files read through the shell must ask (parent-reported).
		{"cat ~/.ssh/id_ed25519", false},
		{"cat ~/.hermes/.env", false},
		{"tail -n 5 ~/.local/share/tool/gateway/token", false},
		{"head ~/.netrc", false},
		{"jq -n input --rawfile x ~/.ssh/id_rsa", false},
		{"grep -r . ~/.password-store", false},
		{"cat /etc/shadow", false},
		{"strings ~/.gnupg/private-keys-v1.d/x.key", false},
		{"sha256sum ~/.ssh/id_rsa", false},
		{"grep foo < ~/.ssh/id_rsa", false},
		{"cat ~/.aws/credentials", false},
		{"cat ~/.kube/config", false},
		// Recursive search of the home tree for a secret word must ask.
		{"grep -r token ~/.config", false},
		{"rg password ~/.local", false},
		{"grep -R secret ~", false},
		// But an ordinary recursive search or a single named file is fine.
		{"rg token", true},
		{"grep token main.go", true},
		{"grep -rn secret src", true},
		{"cat /etc/hostname", true},
		{"cat /etc/os-release", true},
		// git global flags and external-program hooks.
		{"git -c core.pager=sh log", false},
		{"git --git-dir=/x log", false},
		{"git --exec-path=/x status", false},
		{"git log --output=out.txt", false},
		{"git diff --ext-diff", false},
		{"git -C /repo log --oneline", true},
		{"git diff --no-ext-diff", true},
		// write-to-file and destructive flags on read tools.
		{"tree -o out.txt", false},
		{"tree -L 2 /etc", true},
		{"ss -K", false},
		{"ss -tulpn", true},
		{"sensors -s", false},
		{"sensors", true},
		{"dmidecode --dump-bin dump.bin", false},
		{"dmidecode -t system", true},
		{"systemd-analyze plot", true},
	}
	for _, c := range cases {
		got, reason := readOnlyCommand(c.cmd)
		if got != c.want {
			t.Errorf("readOnlyCommand(%q) = %v (%q), want %v", c.cmd, got, reason, c.want)
		}
		if !got && reason == "" {
			t.Errorf("readOnlyCommand(%q) rejected without a reason", c.cmd)
		}
	}
}

func TestReadOnlyToolCall(t *testing.T) {
	cases := []struct {
		name  string
		kind  string
		title string
		input string
		want  bool
	}{
		{"read config", "read", "", `{"path":"/home/x/.config/ryoku/rashin.json"}`, true},
		{"read ssh key", "read", "", `{"path":"~/.ssh/id_ed25519"}`, false},
		{"read ssh key via title", "read", "read ~/.ssh/id_ed25519", `{}`, false},
		{"read netrc", "read", "", `{"file_path":"/home/x/.netrc"}`, false},
		{"read pem", "read", "", `{"filePath":"certs/server.pem"}`, false},
		{"read id_rsa", "read", "", `{"path":"id_rsa"}`, false},
		{"read credentials", "read", "", `{"path":"aws/credentials"}`, false},
		{"read normal", "read", "", `{"path":"/var/log/pacman.log"}`, true},
		{"search token query", "search", "", `{"query":"token"}`, false},
		{"search plain", "search", "", `{"pattern":"TODO","path":"src"}`, true},
		{"read paths array secret", "read", "", `{"paths":["a.txt","b/.gnupg/x"]}`, false},
		{"read paths array clean", "read", "", `{"paths":["a.txt","b/c.go"]}`, true},
		{"think no input", "think", "", ``, true},
		{"fetch get", "fetch", "", `{"url":"https://x","method":"GET"}`, true},
		{"fetch default", "fetch", "", `{"url":"https://x"}`, true},
		{"fetch post", "fetch", "", `{"url":"https://x","method":"POST"}`, false},
		{"fetch body", "fetch", "", `{"url":"https://x","body":"payload"}`, false},
		{"fetch empty body", "fetch", "", `{"url":"https://x","body":""}`, true},
		{"execute command string", "execute", "", `{"command":"ls -la"}`, true},
		{"execute command array", "execute", "", `{"command":["journalctl","-b","-3","--no-pager"]}`, true},
		{"execute write", "execute", "", `{"command":"rm -rf /tmp/x"}`, false},
		{"execute cmd field", "execute", "", `{"cmd":"cat /etc/hosts"}`, true},
		{"execute title fallback", "execute", "grep foo file", `{}`, true},
		{"execute reads a file", "execute", "", `{"command":"cat /etc/fstab"}`, true},
		{"other with command", "other", "", `{"command":"df -h"}`, true},
		{"other write command", "other", "", `{"command":"mv a b"}`, false},
		{"other no command", "other", "", `{}`, false},
		{"edit", "edit", "", `{"path":"main.go"}`, false},
		{"delete", "delete", "", `{"path":"main.go"}`, false},
		{"move", "move", "", `{"path":"main.go"}`, false},
		{"switch_mode", "switch_mode", "", `{}`, false},
		{"unknown kind", "wibble", "", `{}`, false},
	}
	for _, c := range cases {
		got, reason := readOnlyToolCall(c.kind, c.title, json.RawMessage(c.input))
		if got != c.want {
			t.Errorf("%s: readOnlyToolCall(%q,%q,%s) = %v (%q), want %v",
				c.name, c.kind, c.title, c.input, got, reason, c.want)
		}
		if !got && reason == "" {
			t.Errorf("%s: rejected without a reason", c.name)
		}
	}
}

// The three approval tiers answer the same two questions differently: does a
// read run unasked, does a write. A stale or mistyped mode is the default.
func TestApprovalsModes(t *testing.T) {
	cases := []struct {
		stored string
		mode   string
		reads  bool
		all    bool
	}{
		{"", "read-only", true, false},
		{"read-only", "read-only", true, false},
		{"ask", "ask", false, false},
		{"auto", "auto", true, true},
		{"yolo", "read-only", true, false},
	}
	for _, c := range cases {
		cfg := Config{Approvals: c.stored}
		if got := cfg.ApprovalsMode(); got != c.mode {
			t.Errorf("Approvals=%q: mode %q, want %q", c.stored, got, c.mode)
		}
		if got := cfg.AutoApproveReads(); got != c.reads {
			t.Errorf("Approvals=%q: AutoApproveReads %v, want %v", c.stored, got, c.reads)
		}
		if got := cfg.AutoApproveAll(); got != c.all {
			t.Errorf("Approvals=%q: AutoApproveAll %v, want %v", c.stored, got, c.all)
		}
	}
}
