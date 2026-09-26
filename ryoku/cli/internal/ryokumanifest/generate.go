package ryokumanifest

import (
	"bufio"
	"regexp"
	"sort"
	"strings"
)

// The generator turns the repo's own package-set files into a Manifest. It is
// pure over the file contents so the contract is unit-testable without a
// checkout: build-repo.sh reads the files and hands the strings here.

// PackageSet parses a system/packages/*.packages file: one name per line, '#'
// comments and blanks ignored. It returns the bare names, sorted and deduped.
func PackageSet(text string) []string {
	var out []string
	seen := map[string]bool{}
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		name := strings.TrimSpace(sc.Text())
		if name == "" || strings.HasPrefix(name, "#") || strings.HasPrefix(name, "[") {
			continue
		}
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// HardwareSets parses hardware.packages, whose [section] headers group the
// per-profile sets, into section -> names. A name before any header is ignored:
// the file's contract is that hardware packages are always profile-scoped.
func HardwareSets(text string) map[string][]string {
	out := map[string][]string{}
	section := ""
	seen := map[string]map[string]bool{}
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
			continue
		case strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
			section = strings.Trim(line, "[]")
			if seen[section] == nil {
				seen[section] = map[string]bool{}
			}
		case section != "":
			if !seen[section][line] {
				seen[section][line] = true
				out[section] = append(out[section], line)
			}
		}
	}
	for s := range out {
		sort.Strings(out[s])
	}
	return out
}

// Pkgname extracts pkgname= from a PKGBUILD, honouring the quoted and bare
// forms. It returns "" when the file carries none (a README.md in the tree).
func Pkgname(pkgbuild string) string {
	re := regexp.MustCompile(`(?m)^pkgname=(?:"([^"]+)"|'([^']+)'|([A-Za-z0-9@._+-]+))`)
	m := re.FindStringSubmatch(pkgbuild)
	if m == nil {
		return ""
	}
	for _, g := range m[1:] {
		if g != "" {
			return g
		}
	}
	return ""
}

// CompositorPackages extracts a provider's compositorPackages list from its
// caps.go: the quoted strings inside the `var compositorPackages = []string{…}`
// block, in declaration order.
func CompositorPackages(capsGo string) []string {
	re := regexp.MustCompile(`(?s)compositorPackages\s*=\s*\[\]string\{(.*?)\}`)
	m := re.FindStringSubmatch(capsGo)
	if m == nil {
		return nil
	}
	lit := regexp.MustCompile(`"([^"]+)"`)
	var out []string
	for _, s := range lit.FindAllStringSubmatch(m[1], -1) {
		out = append(out, s[1])
	}
	return out
}

// AppNames projects the deliver-once table to bare names, sorted.
func AppNames() []string {
	apps := Apps()
	out := make([]string, 0, len(apps))
	for _, a := range apps {
		out = append(out, a.Pkg)
	}
	sort.Strings(out)
	return out
}

// Inputs is everything the generator reads from the checkout, so assembly stays
// a pure function of a struct build-repo.sh fills in.
type Inputs struct {
	Release  string
	Version  string
	Commit   string
	Channel  string
	Date     string
	Base     string // system/packages/base.packages
	Dev      string // system/packages/dev.packages
	Hardware string // system/packages/hardware.packages
	AUR      string // system/packages/aur.packages
	// FirstParty maps a release/packages/<dir> name to its PKGBUILD contents.
	FirstParty map[string]string
	// Compositor maps a provider name to its caps.go contents.
	Compositor map[string]string
}

// Build assembles the manifest from the checkout's own files. The result is
// deterministic: every list is sorted, so the same tree always yields the same
// bytes and a box can trust the digest it caches.
//
// The first-party lane is the release's own delivery contract: every box on
// the release is expected to carry it. Two groups of [ryoku] PKGBUILDs are
// therefore not first-party: names a compositor provider declares (they belong
// to that compositor's variant, not to every machine -- shipping
// ryoku-desktop-niri or xwayland-satellite in the lane would offer the other
// desktop's packages to everyone), and OptIn names a hardware-gated or
// user-chosen path owns (asusctl, dualsensectl, the fingerprint module, the
// snapshot sync, the palette bridge).
func Build(in Inputs) Manifest {
	m := Manifest{
		Schema:      Schema,
		Release:     in.Release,
		Version:     in.Version,
		Commit:      in.Commit,
		Channel:     in.Channel,
		Date:        in.Date,
		Base:        PackageSet(in.Base),
		Dev:         PackageSet(in.Dev),
		Hardware:    HardwareSets(in.Hardware),
		AUR:         PackageSet(in.AUR),
		Compositor:  compositorPackages(in.Compositor),
		Provisioned: AppNames(),
	}
	declared := map[string]bool{}
	for _, pkgs := range m.Compositor {
		for _, p := range pkgs {
			declared[p] = true
		}
	}
	for _, body := range in.FirstParty {
		n := Pkgname(body)
		if n == "" || declared[n] || IsOptIn(n) {
			continue
		}
		m.FirstParty = append(m.FirstParty, n)
	}
	sort.Strings(m.FirstParty)
	return m
}

func compositorPackages(caps map[string]string) map[string][]string {
	if len(caps) == 0 {
		return nil
	}
	out := map[string][]string{}
	for name, body := range caps {
		if pkgs := CompositorPackages(body); len(pkgs) > 0 {
			out[name] = pkgs
		}
	}
	return out
}
