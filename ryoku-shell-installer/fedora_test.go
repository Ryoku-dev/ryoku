package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFedoraRepositoryChannelsAndOverride(t *testing.T) {
	t.Setenv("RYOKU_RPM_REPO", "")
	for _, test := range []struct {
		ref, channel, repository string
	}{
		{"main", "stable", fedoraStableRepo},
		{"unstable-dev", "testing", fedoraTestingRepo},
		{"v1.2.3", "v1.2.3", "https://repo.ryoku.dev/stable/fedora/$releasever/releases/v1.2.3/x86_64"},
	} {
		channel, repository := fedoraRepository(test.ref)
		if channel != test.channel || repository != test.repository {
			t.Errorf("fedoraRepository(%q) = %q, %q; want %q, %q", test.ref, channel, repository, test.channel, test.repository)
		}
		if !strings.Contains(fedoraRepoContent(repository), "baseurl="+test.repository+"\n") {
			t.Errorf("repo content for %q does not carry %q", test.ref, test.repository)
		}
	}

	t.Setenv("RYOKU_RPM_REPO", "/srv/ryoku-rpms")
	channel, repository := fedoraRepository("unstable-dev")
	if channel != "testing" || repository != "/srv/ryoku-rpms" {
		t.Fatalf("override = %q, %q; want testing, /srv/ryoku-rpms", channel, repository)
	}
	if !strings.Contains(fedoraRepoContent(repository), "baseurl=/srv/ryoku-rpms\n") {
		t.Fatal("override was not written into repo content")
	}
}

func TestFedoraRepoContent(t *testing.T) {
	want := "[ryoku]\n" +
		"name=Ryoku\n" +
		"baseurl=" + fedoraTestingRepo + "\n" +
		"enabled=1\n" +
		"gpgcheck=1\n" +
		"repo_gpgcheck=1\n" +
		"gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-ryoku\n"
	if got := fedoraRepoContent(fedoraTestingRepo); got != want {
		t.Fatalf("repo content:\n%s\nwant:\n%s", got, want)
	}

	path := filepath.Join(t.TempDir(), "ryoku.repo")
	t.Setenv("RYOKU_DNF_REPO_CONFIG", path)
	t.Setenv("RYOKU_RPM_REPO", "")
	if err := os.WriteFile(path, []byte(fedoraRepoContent(fedoraStableRepo)), 0o644); err != nil {
		t.Fatal(err)
	}
	if !fedoraRepoIsConfigured(&engine{ref: "main"}) {
		t.Fatal("exact stable repo file was not accepted")
	}
	if fedoraRepoIsConfigured(&engine{ref: "unstable-dev"}) {
		t.Fatal("stable repo file was accepted for testing")
	}
}

func TestFedoraCoprContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "coprs.tsv")
	data := "copr\tincludepkgs\twhy\n" +
		"sdegler/hyprland\thyprland hypridle\tFedora lacks the required versions.\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := readFedoraCoprs(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %v", rows)
	}
	want := "[ryoku-copr-sdegler-hyprland]\n" +
		"name=Ryoku dependency: sdegler/hyprland\n" +
		"baseurl=https://download.copr.fedorainfracloud.org/results/sdegler/hyprland/fedora-$releasever-$basearch/\n" +
		"enabled=1\n" +
		"gpgcheck=1\n" +
		"repo_gpgcheck=0\n" +
		"gpgkey=https://download.copr.fedorainfracloud.org/results/sdegler/hyprland/pubkey.gpg\n" +
		"includepkgs=hyprland hypridle\n" +
		"skip_if_unavailable=1\n"
	if got := fedoraCoprContent(rows[0]); got != want {
		t.Fatalf("COPR content:\n%s\nwant:\n%s", got, want)
	}
}

func TestRPMPrimaryFingerprintRejectsWrongOrMultipleKeys(t *testing.T) {
	listing := func(fingerprints ...string) string {
		var b strings.Builder
		for _, fingerprint := range fingerprints {
			b.WriteString("pub:::::::::\n")
			b.WriteString("fpr:::::::::")
			b.WriteString(fingerprint)
			b.WriteString(":\n")
		}
		return b.String()
	}
	if got, err := rpmPrimaryFingerprint(listing(fedoraKeyFingerprint)); err != nil || got != fedoraKeyFingerprint {
		t.Fatalf("valid release key rejected: %q, %v", got, err)
	}
	if _, err := rpmPrimaryFingerprint(listing("0000000000000000000000000000000000000000")); err == nil {
		t.Fatal("wrong fingerprint was accepted")
	}
	if _, err := rpmPrimaryFingerprint(listing(fedoraKeyFingerprint, "1111111111111111111111111111111111111111")); err == nil {
		t.Fatal("two primary keys were accepted")
	}
}

func TestFedoraRepoTrustProbeRequiresPackageOutput(t *testing.T) {
	bin := t.TempDir()
	sudo := "#!/bin/sh\n[ \"$1\" != -n ] || shift\nexec \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "sudo"), []byte(sudo), 0o755); err != nil {
		t.Fatal(err)
	}
	dnfPath := filepath.Join(bin, "dnf5")
	success := "#!/bin/sh\n[ \"$*\" = \"repoquery --repo=ryoku -q ryoku\" ] || exit 64\nprintf '%s\\n' 'ryoku-1.2.3-1.fc44.x86_64'\n"
	if err := os.WriteFile(dnfPath, []byte(success), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	d := newFedoraLinux()
	e := &engine{f: &facts{distro: d}, events: make(chan any, 8)}
	if err := verifyFedoraRepoReadable(e); err != nil {
		t.Fatalf("published package was rejected: %v", err)
	}

	empty := "#!/bin/sh\n[ \"$*\" = \"repoquery --repo=ryoku -q ryoku\" ] || exit 64\n"
	if err := os.WriteFile(dnfPath, []byte(empty), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := verifyFedoraRepoReadable(e); err == nil {
		t.Fatal("an empty successful repoquery was accepted")
	}
}

func TestFedoraPackagePlanUsesTranslationsAndAllRepos(t *testing.T) {
	payload := t.TempDir()
	table := filepath.Join(payload, "fedora/packages/translations.tsv")
	if err := os.MkdirAll(filepath.Dir(table), 0o755); err != nil {
		t.Fatal(err)
	}
	data := "arch\tfedora\tlanes\tnotes\n" +
		"chromium\tchromium\tdesktop\t\n" +
		"zsh\tzsh\tdesktop\t\n" +
		"zsh-autosuggestions\tzsh-autosuggestions\tdesktop\t\n" +
		"zsh-history-substring-search\t-\tdesktop\tNot packaged.\n" +
		"zsh-syntax-highlighting\tzsh-syntax-highlighting\tdesktop\t\n" +
		"ryoku-oh-my-zsh\t-\tdesktop\tFedora uses the packaged shell config.\n"
	if err := os.WriteFile(table, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	d := newFedoraLinux()
	e := &engine{
		f:       &facts{distro: d},
		p:       &plan{browser: "chromium", shell: "zsh", compositor: "niri"},
		dry:     true,
		payload: payload,
		events:  make(chan any, 32),
	}
	if err := stepPackages(e); err != nil {
		t.Fatal(err)
	}
	close(e.events)
	var lines []string
	for event := range e.events {
		if line, ok := event.(evLine); ok {
			lines = append(lines, line.line)
		}
	}
	output := strings.Join(lines, "\n")
	wantCommand := "DRYRUN: sudo -n " + d.installCmd[0] + " install -y ryoku ryoku-desktop ryoku-desktop-niri chromium zsh zsh-autosuggestions zsh-syntax-highlighting"
	if !strings.Contains(output, wantCommand) {
		t.Fatalf("Fedora package command missing:\n%s", output)
	}
	if strings.Contains(output, "--repo") {
		t.Fatalf("Fedora install restricted dependency resolution to one repo:\n%s", output)
	}
	if !strings.Contains(output, "zsh-history-substring-search is not available on Fedora; skipping it") {
		t.Fatalf("unavailable translation was not explained:\n%s", output)
	}
}
