package host

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const usersPasswdFixture = `root:x:0:0:root:/root:/bin/bash
daemon:x:2:2:daemon:/sbin:/usr/bin/nologin
alice:x:1000:1000:Alice Example,,,:/home/alice:/bin/bash
bob:x:1001:1001:Bob Example:/home/bob:/bin/zsh
nobody:x:65534:65534:Nobody:/:/usr/bin/nologin
`

const usersGroupFixture = `root:x:0:
wheel:x:10:alice
alice:x:1000:
bob:x:1001:
input:x:97:alice,bob
kvm:x:992:bob
docker:x:991:
`

func writeUsersFixtures(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	passwdPath := filepath.Join(dir, "passwd")
	groupPath := filepath.Join(dir, "group")
	if err := os.WriteFile(passwdPath, []byte(usersPasswdFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(groupPath, []byte(usersGroupFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	return passwdPath, groupPath
}

func TestUsersListParsesLocalAccounts(t *testing.T) {
	runner := &fakeRunner{}
	passwdPath, groupPath := writeUsersFixtures(t)
	app, stdout, stderr := testApp(runner, map[string]string{
		"RYOKU_HOST_INIT": "systemd",
		"USER":            "bob",
	})
	app.cfg.PasswdPath, app.cfg.GroupPath = passwdPath, groupPath
	app.cfg.UID, app.cfg.UIDSet = 1001, true
	if code := app.Users([]string{"list"}); code != ExitOK {
		t.Fatalf("users list exit = %d, output = %s", code, stdout.String())
	}
	var result usersList
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Self != "bob" {
		t.Fatalf("self = %q", result.Self)
	}
	if len(result.Users) != 2 {
		t.Fatalf("users = %#v", result.Users)
	}
	alice, bob := result.Users[0], result.Users[1]
	if alice.Name != "alice" || alice.FullName != "Alice Example" || alice.UID != 1000 || alice.Role != "admin" {
		t.Fatalf("alice = %#v", alice)
	}
	if !reflect.DeepEqual(alice.Permissions, []string{"admin", "devices"}) {
		t.Fatalf("alice permissions = %#v", alice.Permissions)
	}
	if bob.Name != "bob" || bob.Role != "standard" {
		t.Fatalf("bob = %#v", bob)
	}
	if !reflect.DeepEqual(bob.Permissions, []string{"devices", "virtualization"}) {
		t.Fatalf("bob permissions = %#v", bob.Permissions)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestPermissionMappingSkipsMissingGroups(t *testing.T) {
	passwdPath, groupPath := writeUsersFixtures(t)
	users, groups, err := loadAccountDB(passwdPath, groupPath)
	if err != nil {
		t.Fatal(err)
	}
	var bob localUser
	for _, user := range users {
		if user.Name == "bob" {
			bob = user
		}
	}
	devices, _ := userPermission("devices")
	if got := existingPermissionGroups(devices, groups); !reflect.DeepEqual(got, []string{"input"}) {
		t.Fatalf("device groups = %#v", got)
	}
	virtualization, _ := userPermission("virtualization")
	if got := existingPermissionGroups(virtualization, groups); !reflect.DeepEqual(got, []string{"kvm"}) {
		t.Fatalf("virtualization groups = %#v", got)
	}
	if got := permissionsForUser(bob, groups); !reflect.DeepEqual(got, []string{"devices", "virtualization"}) {
		t.Fatalf("permissions = %#v", got)
	}
}

func TestQgroupUsageUsesTheHomeSubvolume(t *testing.T) {
	show := "Name: alice\nSubvolume ID: 258\nParent ID: 5\n"
	qgroups := "qgroupid rfer excl max_rfer max_excl\n" +
		"0/5 999 999 none none\n" +
		"0/258 4096 3072 1073741824 none\n"
	id := btrfsSubvolumeQgroup(show)
	if id != "0/258" {
		t.Fatalf("qgroup id = %q", id)
	}
	listed := "ID 257 gen 9 top level 5 path @home/bob\n" +
		"ID 258 gen 10 top level 5 path @home/alice\n"
	if got := listedBtrfsSubvolumeQgroup(listed, "alice"); got != id {
		t.Fatalf("listed qgroup id = %q", got)
	}
	used, limit, ok := parseQgroupUsage(qgroups, id)
	if !ok || used != 4096 || limit != 1073741824 {
		t.Fatalf("usage = %d, limit = %d, ok = %t", used, limit, ok)
	}
}

func TestUsersValidation(t *testing.T) {
	for _, name := range []string{"Upper", "two words", "-dash", strings.Repeat("a", 33), "root", "nobody"} {
		if err := validateNewUsername(name, nil); err == nil {
			t.Errorf("validateNewUsername(%q) succeeded", name)
		}
	}
	if err := validateNewUsername("valid_name-2", nil); err != nil {
		t.Fatal(err)
	}
	invalidAdds := [][]string{
		{"alice", "--full-name", "Alice"},
		{"alice", "--full-name", "Alice", "--role", "owner"},
		{"alice", "--full-name", "Alice", "--role", "admin", "--limit", "-1"},
		{"alice", "--full-name", "Alice", "--role", "admin", "--unknown", "x"},
	}
	for _, args := range invalidAdds {
		if _, _, _, _, _, err := parseUsersAdd(args); err == nil {
			t.Errorf("parseUsersAdd(%q) succeeded", args)
		}
	}
	for _, value := range []string{"-1", "+1", "1.5", "01", "18446744073709551616"} {
		if _, err := parseBytes(value); err == nil {
			t.Errorf("parseBytes(%q) succeeded", value)
		}
	}
}

func TestUsersRejectsMalformedArgv(t *testing.T) {
	tests := [][]string{
		{},
		{"list", "extra"},
		{"caps", "extra"},
		{"delete", "alice"},
		{"delete", "alice", "--all-files"},
		{"set-role", "alice", "owner"},
		{"set-perms", "alice"},
		{"set-password", "alice", "password"},
		{"set-limit", "alice", "-1"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			app, stdout, _ := testApp(&fakeRunner{}, nil)
			app.cfg.UID, app.cfg.UIDSet = 0, true
			if code := app.Users(args); code != ExitFailure {
				t.Fatalf("Users(%q) exit = %d", args, code)
			}
			var failure map[string]string
			if err := json.Unmarshal(stdout.Bytes(), &failure); err != nil || failure["error"] == "" {
				t.Fatalf("output = %q, error = %v", stdout.String(), err)
			}
		})
	}
}

func TestUsersRefusesLastAdministratorDemotion(t *testing.T) {
	passwdPath, groupPath := writeUsersFixtures(t)
	app, stdout, _ := testApp(&fakeRunner{}, nil)
	app.cfg.PasswdPath, app.cfg.GroupPath = passwdPath, groupPath
	app.cfg.UID, app.cfg.UIDSet = 0, true
	if code := app.Users([]string{"set-role", "alice", "standard"}); code != ExitFailure {
		t.Fatalf("set-role exit = %d", code)
	}
	if !strings.Contains(stdout.String(), "last Administrator") {
		t.Fatalf("output = %q", stdout.String())
	}
}

func TestUsersRefusesSelfDelete(t *testing.T) {
	passwdPath, groupPath := writeUsersFixtures(t)
	app, stdout, _ := testApp(&fakeRunner{}, map[string]string{"PKEXEC_UID": "1000"})
	app.cfg.PasswdPath, app.cfg.GroupPath = passwdPath, groupPath
	app.cfg.UID, app.cfg.UIDSet = 0, true
	if code := app.Users([]string{"delete", "alice", "--erase-files"}); code != ExitFailure {
		t.Fatalf("delete exit = %d", code)
	}
	if !strings.Contains(stdout.String(), "own account") {
		t.Fatalf("output = %q", stdout.String())
	}
}

func TestUsersRefusesLoggedInDelete(t *testing.T) {
	passwdPath, groupPath := writeUsersFixtures(t)
	runner := &fakeRunner{answer: func(command Command) Result {
		if command.Name == "loginctl" {
			return Result{Output: "1001 bob\n"}
		}
		return Result{}
	}}
	app, stdout, _ := testApp(runner, map[string]string{
		"PKEXEC_UID":      "1000",
		"RYOKU_HOST_INIT": "systemd",
	})
	app.cfg.PasswdPath, app.cfg.GroupPath = passwdPath, groupPath
	app.cfg.UID, app.cfg.UIDSet = 0, true
	if code := app.Users([]string{"delete", "bob", "--erase-files"}); code != ExitFailure {
		t.Fatalf("delete exit = %d", code)
	}
	if !strings.Contains(stdout.String(), "Sign out bob") {
		t.Fatalf("output = %q", stdout.String())
	}
	for _, command := range runner.commands {
		if command.Name == "userdel" {
			t.Fatalf("userdel ran for a signed-in user")
		}
	}
}

func TestUsersPasswordTravelsOnStdin(t *testing.T) {
	passwdPath, groupPath := writeUsersFixtures(t)
	var passwordInput string
	runner := &fakeRunner{answer: func(command Command) Result {
		if command.Name == "chpasswd" {
			body, err := io.ReadAll(command.Stdin)
			if err != nil {
				t.Fatal(err)
			}
			passwordInput = string(body)
		}
		return Result{}
	}}
	app, stdout, _ := testApp(runner, nil)
	app.cfg.PasswdPath, app.cfg.GroupPath = passwdPath, groupPath
	app.cfg.Stdin = strings.NewReader("correct horse battery staple\n")
	app.cfg.UID, app.cfg.UIDSet = 0, true
	if code := app.Users([]string{"set-password", "bob"}); code != ExitOK {
		t.Fatalf("set-password exit = %d, output = %s", code, stdout.String())
	}
	if passwordInput != "bob:correct horse battery staple\n" {
		t.Fatalf("chpasswd input = %q", passwordInput)
	}
	for _, command := range runner.commands {
		if strings.Contains(strings.Join(command.Args, " "), "correct horse") {
			t.Fatalf("password leaked into argv: %#v", command)
		}
	}
}

// An account added in the Hub logs into the administrator's shell (fish on a
// default install), not a bare bash that skips the Ryoku terminal setup.
func TestUsersAddUsesTheAdministratorsShell(t *testing.T) {
	dir := t.TempDir()
	fish := filepath.Join(dir, "fish")
	if err := os.WriteFile(fish, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ adminShell, want string }{
		{fish, fish},
		{filepath.Join(dir, "missing"), "/bin/bash"},
		{"/usr/bin/nologin", "/bin/bash"},
	} {
		passwdPath, groupPath := filepath.Join(dir, "passwd"), filepath.Join(dir, "group")
		passwd := "root:x:0:0:root:/root:/bin/bash\nalice:x:1000:1000:Alice:/home/alice:" + c.adminShell + "\n"
		if err := os.WriteFile(passwdPath, []byte(passwd), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(groupPath, []byte(usersGroupFixture), 0o600); err != nil {
			t.Fatal(err)
		}
		runner := &fakeRunner{}
		app, stdout, _ := testApp(runner, map[string]string{"PKEXEC_UID": "1000"})
		app.cfg.PasswdPath, app.cfg.GroupPath = passwdPath, groupPath
		app.cfg.HomeRoot = t.TempDir()
		app.cfg.Stdin = strings.NewReader("secret\n")
		app.cfg.UID, app.cfg.UIDSet = 0, true
		if code := app.Users([]string{"add", "carol", "--full-name", "Carol", "--role", "standard"}); code != ExitOK {
			t.Fatalf("add exit = %d, output = %s", code, stdout.String())
		}
		shell := ""
		for _, command := range runner.commands {
			if command.Name == "useradd" {
				for i, arg := range command.Args {
					if arg == "--shell" && i+1 < len(command.Args) {
						shell = command.Args[i+1]
					}
				}
			}
		}
		if shell != c.want {
			t.Errorf("admin shell %s: new account shell = %q, want %q", c.adminShell, shell, c.want)
		}
	}
}
