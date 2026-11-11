package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
)

// Default Apps: the swappable roles the Default Apps page offers, each with a
// short list of common candidates. `ryoku-hub apps` reports which candidates are
// installed (a binary on PATH) so the page can show them as chips; the field
// still accepts any command. The chosen command is stored in desktop.json under
// desktop.apps and launched by the ryoku-app resolver.

type appAlias struct {
	Cmd       string `json:"cmd"`
	Installed bool   `json:"installed"`
}

type appCandidate struct {
	Label     string     `json:"label"`
	Cmd       string     `json:"cmd"`
	Installed bool       `json:"installed"`
	Aliases   []appAlias `json:"aliases"`
}

type appRole struct {
	Role       string         `json:"role"`
	Label      string         `json:"label"`
	Fallback   string         `json:"fallback"`        // the shipped default when unset
	Combo      string         `json:"combo,omitempty"` // shipped combo bound to `ryoku-app <role>`
	Candidates []appCandidate `json:"candidates"`
}

// role -> {label, fallback, candidates}. Candidates are {label, cmd}; the cmd's
// meaningful binary (the app after `-e`, else the first word) is probed on PATH.
var appRoleDefs = []struct {
	Role, Label, Fallback string
	Cands                 [][2]string
}{
	{"browser", "Browser", "chromium", [][2]string{
		{"Firefox", "firefox"}, {"Chromium", "chromium"}, {"Chrome", "google-chrome-stable"},
		{"Brave", "brave"}, {"Vivaldi", "vivaldi-stable"}, {"Zen", "zen|zen-browser"},
		{"LibreWolf", "librewolf"}, {"Qutebrowser", "qutebrowser"},
	}},
	{"terminal", "Terminal", "kitty", [][2]string{
		{"Kitty", "kitty"}, {"Alacritty", "alacritty"}, {"Foot", "foot"},
		{"WezTerm", "wezterm"}, {"Ghostty", "ghostty"}, {"Konsole", "konsole"},
	}},
	{"editor", "Editor", "kitty -e nvim", [][2]string{
		{"Neovim", "kitty -e nvim"}, {"Helix", "kitty -e hx"}, {"Vim", "kitty -e vim"},
		{"VS Code", "code"}, {"VSCodium", "codium"}, {"Zed", "zed"}, {"Sublime Text", "subl"},
	}},
	{"files", "File manager", "nautilus", [][2]string{
		{"Files (Nautilus)", "nautilus"}, {"Thunar", "thunar"}, {"Dolphin", "dolphin"},
		{"Nemo", "nemo"}, {"PCManFM", "pcmanfm-qt"}, {"Yazi", "kitty -e yazi"},
	}},
	{"notes", "Notes", "", [][2]string{
		{"Obsidian", "obsidian"}, {"Logseq", "logseq"}, {"Joplin", "joplin-desktop"},
		{"Standard Notes", "standard-notes"}, {"Zettlr", "zettlr"},
	}},
}

// binOf returns the binary a launch command actually depends on: the token after
// `-e` for a terminal-wrapped app (kitty -e nvim -> nvim), else the first word.
func binOf(cmd string) string {
	f := strings.Fields(cmd)
	for i, t := range f {
		if t == "-e" && i+1 < len(f) {
			return f[i+1]
		}
	}
	if len(f) > 0 {
		return f[0]
	}
	return ""
}

func candidateAliases(spec string) []appAlias {
	alts := strings.Split(spec, "|")
	out := make([]appAlias, 0, len(alts))
	for _, alt := range alts {
		_, err := exec.LookPath(binOf(alt))
		out = append(out, appAlias{Cmd: alt, Installed: err == nil})
	}
	return out
}

func resolveAliases(aliases []appAlias) (cmd string, installed bool) {
	for _, alias := range aliases {
		if alias.Installed {
			return alias.Cmd, true
		}
	}
	if len(aliases) == 0 {
		return "", false
	}
	return aliases[0].Cmd, false
}

func resolveCandidate(spec string) (cmd string, installed bool) {
	return resolveAliases(candidateAliases(spec))
}

func appRoles() []appRole {
	out := make([]appRole, 0, len(appRoleDefs))
	combos := roleCombos()
	for _, d := range appRoleDefs {
		fallback := d.Fallback
		// Browser default follows ryoku-app: Zen when installed (under either the
		// tarball name `zen` or the AUR `zen-browser`), else the shipped Chromium.
		if d.Role == "browser" {
			if cmd, ok := resolveCandidate("zen|zen-browser"); ok {
				fallback = cmd
			}
		}
		r := appRole{Role: d.Role, Label: d.Label, Fallback: fallback, Combo: combos[d.Role]}
		for _, c := range d.Cands {
			aliases := candidateAliases(c[1])
			cmd, installed := resolveAliases(aliases)
			r.Candidates = append(r.Candidates, appCandidate{
				Label: c[0], Cmd: cmd, Installed: installed, Aliases: aliases,
			})
		}
		out = append(out, r)
	}
	return out
}

// roleCombos maps each app role to the shipped combo its `ryoku-app <role>`
// launch is bound to in binds.lua, so the Keybinds page can show and rebind the
// key beside the app. A role with no shipped bind is absent.
func roleCombos() map[string]string {
	out := map[string]string{}
	src, err := os.ReadFile(bindsPath())
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(src), "\n") {
		if !strings.Contains(line, "hl.bind(") {
			continue
		}
		m := reBind.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		em := reExec.FindStringSubmatch(m[2])
		if em == nil || !strings.HasPrefix(strings.TrimSpace(em[1]), "ryoku-app ") {
			continue
		}
		role := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(em[1]), "ryoku-app "))
		if _, combo := resolveKeys(m[1], ""); combo != "" {
			out[role] = combo
		}
	}
	return out
}

func printApps() error {
	b, err := json.Marshal(appRoles())
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(b)
	return err
}
