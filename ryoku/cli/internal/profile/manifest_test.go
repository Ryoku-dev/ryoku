package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validTestManifest(t *testing.T) (string, *Manifest) {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "home", "template.txt"), "home=@HOME@\n")
	writeTestFile(t, filepath.Join(dir, "home", "seed.txt"), "seed\n")
	writeTestFile(t, filepath.Join(dir, "home", ".config", "ryoku", "rices", "hacker", "rice.json"), "{}\n")
	m := &Manifest{
		Schema: 1, ID: "hacker", Version: "1.0.0", Name: "Hacker",
		Requires:     Requires{ProfileAPI: 1, PackageManagers: []string{"pacman"}},
		Account:      AccountSpec{SuggestedName: "hacker", FullName: "Hacker", Role: "standard", Permissions: []string{"audio"}, Groups: []string{"wireshark"}, Shell: "/bin/bash"},
		Repos:        []string{"blackarch"},
		Components:   []Component{{Type: "package", Name: "nmap", Detect: "nmap", Tier: "core", Group: "Recon", Summary: "Scanner", DesktopEntries: []string{"nmap.desktop"}}},
		Capabilities: []CapabilitySpec{{Tool: "nmap", Caps: AllowedCapabilities["nmap"]}},
		Wordlists:    Wordlists{Links: []WordlistLink{{Name: "seclists", Target: "/usr/share/seclists"}}, Extract: []WordlistExtract{{Archive: "/usr/share/seclists/rockyou.tar.gz", Member: "rockyou.txt", Name: "rockyou.txt"}}},
		Hosts:        HostsSpec{Suffixes: []string{"htb"}}, Env: []EnvVar{{Key: "WORDLISTS", Value: "/usr/share/wordlists"}},
		Path: []string{"@PROFILE_BIN@", "@HOME@/go/bin"}, Launcher: LauncherSpec{Allow: []string{"kitty.desktop"}},
		StoreItems: []StoreItem{{Category: "barstyles", ID: "qsbar"}},
		Expand:     []string{"template.txt"}, SeedOnce: []string{"seed.txt"}, UserData: []string{"notes"}, Rice: "hacker",
		FirstRun: FirstRun{Command: []string{"obsidian"}, Message: "Ready."},
	}
	return dir, m
}

func cloneManifest(t *testing.T, m *Manifest) *Manifest {
	t.Helper()
	body, _ := json.Marshal(m)
	var out Manifest
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	return &out
}
func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestManifestValidationRules(t *testing.T) {
	dir, base := validTestManifest(t)
	tests := []struct {
		name, field string
		mutate      func(*Manifest)
	}{
		{"schema", "schema", func(m *Manifest) { m.Schema = 2 }},
		{"id", "id", func(m *Manifest) { m.ID = "Bad" }},
		{"version", "version", func(m *Manifest) { m.Version = "one" }},
		{"name", "name", func(m *Manifest) { m.Name = "" }},
		{"profile api", "requires.profileApi", func(m *Manifest) { m.Requires.ProfileAPI = 2 }},
		{"package manager", "requires.packageManagers", func(m *Manifest) { m.Requires.PackageManagers = []string{"apt"} }},
		{"wm capability", "requires.capabilities", func(m *Manifest) { m.Requires.Capabilities = []string{"madeUp"} }},
		{"role", "account.role", func(m *Manifest) { m.Account.Role = "admin" }},
		{"permission", "account.permissions", func(m *Manifest) { m.Account.Permissions = []string{"admin"} }},
		{"group", "account.groups", func(m *Manifest) { m.Account.Groups = []string{"wheel"} }},
		{"shell", "account.shell", func(m *Manifest) { m.Account.Shell = "bash" }},
		{"repo", "repos", func(m *Manifest) { m.Repos = []string{"aur"} }},
		{"store item category", "storeItems[0].category", func(m *Manifest) { m.StoreItems[0].Category = "plugins" }},
		{"store item id", "storeItems[0].id", func(m *Manifest) { m.StoreItems[0].ID = "Bad ID" }},
		{"component type", "components[0].type", func(m *Manifest) { m.Components[0].Type = "script" }},
		{"package name", "components[0].name", func(m *Manifest) { m.Components[0].Name = "bad name" }},
		{"detect", "components[0].detect", func(m *Manifest) { m.Components[0].Detect = "bad/path" }},
		{"duplicate component", "components[1].name", func(m *Manifest) { m.Components = append(m.Components, m.Components[0]) }},
		{"tier", "components[0].tier", func(m *Manifest) { m.Components[0].Tier = "extra" }},
		{"component group", "components[0].group", func(m *Manifest) { m.Components[0].Group = "" }},
		{"component summary", "components[0].summary", func(m *Manifest) { m.Components[0].Summary = "" }},
		{"desktop id", "components[0].desktopEntries", func(m *Manifest) { m.Components[0].DesktopEntries = []string{"nmap"} }},
		{"capability tool", "capabilities[0].tool", func(m *Manifest) { m.Capabilities[0].Tool = "bash" }},
		{"capability string", "capabilities[0].caps", func(m *Manifest) { m.Capabilities[0].Caps = "cap_net_raw=ep" }},
		{"capability component", "capabilities[0].tool", func(m *Manifest) { m.Components[0].Detect = "scan" }},
		{"wordlist link name", "wordlists.links[0].name", func(m *Manifest) { m.Wordlists.Links[0].Name = "../x" }},
		{"wordlist link target", "wordlists.links[0].target", func(m *Manifest) { m.Wordlists.Links[0].Target = "/tmp/x" }},
		{"wordlist extract name", "wordlists.extract[0].name", func(m *Manifest) { m.Wordlists.Extract[0].Name = "/rockyou.txt" }},
		{"wordlist archive", "wordlists.extract[0].archive", func(m *Manifest) { m.Wordlists.Extract[0].Archive = "archive" }},
		{"wordlist member", "wordlists.extract[0].member", func(m *Manifest) { m.Wordlists.Extract[0].Member = "dir/rockyou.txt" }},
		{"host suffix", "hosts.suffixes", func(m *Manifest) { m.Hosts.Suffixes = []string{"com"} }},
		{"env key", "env[0].key", func(m *Manifest) { m.Env[0].Key = "lower" }},
		{"reserved env", "env[0].key", func(m *Manifest) { m.Env[0].Key = "PATH" }},
		{"env newline", "env[0].value", func(m *Manifest) { m.Env[0].Value = "a\nb" }},
		{"path absolute", "path[0]", func(m *Manifest) { m.Path = []string{"relative"} }},
		{"path colon", "path[0]", func(m *Manifest) { m.Path = []string{"/a:/b"} }},
		{"path dotdot", "path[0]", func(m *Manifest) { m.Path = []string{"@HOME@/../root"} }},
		{"expand existing", "expand[0]", func(m *Manifest) { m.Expand = []string{"missing"} }},
		{"seed existing", "seedOnce[0]", func(m *Manifest) { m.SeedOnce = []string{"missing"} }},
		{"user data", "userData[0]", func(m *Manifest) { m.UserData = []string{"../notes"} }},
		{"rice slug", "rice", func(m *Manifest) { m.Rice = "../hacker" }},
		{"rice exists", "rice", func(m *Manifest) { m.Rice = "missing" }},
		{"first run command", "firstRun.command", func(m *Manifest) { m.FirstRun.Command = nil }},
		{"first run message", "firstRun.message", func(m *Manifest) { m.FirstRun.Message = "a\nb" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := cloneManifest(t, base)
			test.mutate(m)
			err := m.Validate(dir)
			if err == nil || !strings.Contains(err.Error(), test.field) {
				t.Fatalf("want %s error, got %v", test.field, err)
			}
		})
	}
}

func TestManifestAcceptsAbsoluteDetectionPath(t *testing.T) {
	dir, m := validTestManifest(t)
	m.Components[0].Detect = "/usr/share/seclists"
	m.Capabilities = nil
	if err := m.Validate(dir); err != nil {
		t.Fatal(err)
	}
}
func TestManifestRejectsUserDataFile(t *testing.T) {
	dir, m := validTestManifest(t)
	writeTestFile(t, filepath.Join(dir, "home", "notes"), "not a directory\n")
	if err := m.Validate(dir); err == nil || !strings.Contains(err.Error(), "userData[0]") {
		t.Fatalf("got %v", err)
	}
}

func TestLoadManifestRejectsUnknownFieldsAtEveryLevel(t *testing.T) {
	dir, m := validTestManifest(t)
	body, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	cases := []string{
		strings.Replace(string(body), `"schema":1`, `"unknown":true,"schema":1`, 1),
		strings.Replace(string(body), `"profileApi":1`, `"unknown":true,"profileApi":1`, 1),
		strings.Replace(string(body), `"suggestedName"`, `"unknown":true,"suggestedName"`, 1),
		strings.Replace(string(body), `"type":"package"`, `"unknown":true,"type":"package"`, 1),
	}
	for i, raw := range cases {
		if err := os.WriteFile(filepath.Join(dir, "profile.json"), []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadManifest(dir); err == nil {
			t.Fatalf("case %d accepted unknown field", i)
		}
	}
}
