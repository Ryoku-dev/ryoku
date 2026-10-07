package main

import (
	"regexp"
	"strings"
	"testing"
)

func TestParseOSRelease(t *testing.T) {
	id, like, name := parseOSRelease("NAME=\"CachyOS\"\nPRETTY_NAME=\"CachyOS Linux\"\nID=cachyos\nID_LIKE=\"arch\"\n")
	if id != "cachyos" || like != "arch" || name != "CachyOS Linux" {
		t.Fatalf("got %q %q %q", id, like, name)
	}
}

func TestShellJoin(t *testing.T) {
	if got := shellJoin("pacman", []string{"-S", "a b"}); got != "pacman -S \"a b\"" {
		t.Fatalf("got %q", got)
	}
}

func TestStripPacmanSection(t *testing.T) {
	conf := "[options]\nColor\n\n[core]\nInclude = /etc/pacman.d/mirrorlist\n\n[omarchy]\nSigLevel = Required\nServer = https://pkgs.omarchy.org/$arch\n\n[extra]\nInclude = /etc/pacman.d/mirrorlist\n"
	got := stripPacmanSection(conf, "omarchy")
	if regexp.MustCompile(`\[omarchy\]|omarchy\.org`).MatchString(got) {
		t.Fatalf("omarchy section survived:\n%s", got)
	}
	for _, keep := range []string{"[options]", "[core]", "[extra]", "Color"} {
		if !regexp.MustCompile(regexp.QuoteMeta(keep)).MatchString(got) {
			t.Fatalf("lost %q:\n%s", keep, got)
		}
	}
	// no section match leaves the file untouched.
	if stripPacmanSection(conf, "nope") != conf {
		t.Fatal("stripping an absent section changed the file")
	}
}

func TestSecureBootEnforcing(t *testing.T) {
	attr := []byte{6, 0, 0, 0} // efivarfs attribute header
	on, off := append(attr, 1), append(attr, 0)
	if !secureBootEnforcing(on, off) {
		t.Fatal("SecureBoot=1 SetupMode=0 must be enforcing")
	}
	if secureBootEnforcing(on, on) {
		t.Fatal("setup mode means nothing is enforced")
	}
	if secureBootEnforcing(off, off) || secureBootEnforcing(nil, nil) || secureBootEnforcing([]byte{1}, nil) {
		t.Fatal("short or zero variables must not read as enforcing")
	}
}

func TestDefaultPlanSecureBoot(t *testing.T) {
	f := &facts{hasNvidia: true, nouveauLive: true, secureBoot: true}
	if defaultPlan(f).nvidia {
		t.Fatal("unsigned secure boot must force the nvidia default off, even with nouveau live")
	}
	f.sbctlSigned = true
	if !defaultPlan(f).nvidia {
		t.Fatal("an sbctl-managed box keeps the nvidia default, even with nouveau live")
	}
	if !defaultPlan(&facts{hasNvidia: true, nouveauLive: true}).nvidia {
		t.Fatal("nouveau live without secure boot must default nvidia on")
	}
}

func TestMirrorlistHasOmarchy(t *testing.T) {
	if !mirrorlistHasOmarchy("# comment\nServer = https://stable-mirror.omarchy.org/$repo/os/$arch\n") {
		t.Fatal("missed the omarchy mirror")
	}
	if mirrorlistHasOmarchy("# omarchy used to live here\nServer = https://geo.mirror.pkgbuild.com/$repo/os/$arch\n") {
		t.Fatal("a comment mentioning omarchy must not count")
	}
}

func TestSudoArgvOmarchyGuard(t *testing.T) {
	pacman := []string{"pacman", "-Syu", "--noconfirm"}
	want := "env OMARCHY_ALLOW_DIRECT_PACMAN=1 pacman -Syu --noconfirm"
	if got := strings.Join(sudoArgv(pacman, true), " "); got != want {
		t.Fatalf("a guarded box must carry the escape hatch: got %q", got)
	}
	if strings.Join(sudoArgv(pacman, false), " ") != "pacman -Syu --noconfirm" {
		t.Fatal("no guard on the box, no env wrapper")
	}
	if strings.Join(sudoArgv([]string{"systemctl", "disable", "sddm"}, true), " ") != "systemctl disable sddm" {
		t.Fatal("only pacman needs the escape hatch")
	}
}

func TestDefaultPlanRetiresOmarchyGuard(t *testing.T) {
	guards := []string{"/usr/share/libalpm/hooks/00-omarchy-update-guard.hook"}
	if !defaultPlan(&facts{omarchyGuards: guards}).omarchy {
		t.Fatal("a guard hook alone must still schedule the Omarchy retirement")
	}
	if defaultPlan(&facts{}).omarchy {
		t.Fatal("nothing Omarchy on the box, nothing to retire")
	}
}

// Every hook found is retired: one left behind still blocks `ryoku update`.
func TestStepLegacyRetiresEveryOmarchyGuard(t *testing.T) {
	hooks := []string{
		"/usr/share/libalpm/hooks/00-omarchy-update-guard.hook",
		"/etc/pacman.d/hooks/00-omarchy-update-guard.hook",
	}
	e := &engine{f: &facts{omarchyGuards: hooks}, p: &plan{omarchy: true}, dry: true, events: make(chan any, 64)}
	if err := stepLegacy(e); err != nil {
		t.Fatalf("stepLegacy: %v", err)
	}
	close(e.events)
	var said []string
	for ev := range e.events {
		if l, ok := ev.(evLine); ok {
			said = append(said, l.line)
		}
	}
	log := strings.Join(said, "\n")
	for _, hook := range hooks {
		if want := "DRYRUN: sudo -n mv " + hook + " " + hook + ".pre-ryoku"; !strings.Contains(log, want) {
			t.Fatalf("%s must be moved aside:\n%s", hook, log)
		}
	}
	if len(e.pendingRestore) != len(hooks) {
		t.Fatalf("every undo must reach restore.sh: %v", e.pendingRestore)
	}
}
