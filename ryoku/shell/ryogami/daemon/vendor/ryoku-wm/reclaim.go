package wm

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Reclaim answers the one question a compositor switch raises that the neutral
// store cannot: leaving the outgoing compositor for the incoming one, what does
// removing the outgoing compositor's packages actually free? The package lists
// are the providers' own (caps.Packages), because only a provider knows what it
// is made of, so neither the CLI nor the Hub ever spells a compositor package
// name; this seam is the one place both lists meet.
//
// Two exclusions keep the removal from breaking the box:
//   - a package the incoming compositor also declares is shared, so it stays;
//   - a package another installed package still needs is kept.
//
// The set is pacman's own answer to `pacman -Rs` for the outgoing-only,
// installed packages: the compositor package, its satellites, and the private
// dependencies that orphan with them. pacman is the authority because it, not a
// re-derived graph, knows how a provides-satisfied dependency resolves; this
// seam layers the two exclusions on top and pins the orchestration with tests.

// ReclaimSet is what leaving Outgoing for Incoming would remove. Targets are the
// declared compositor packages the removal names; Packages is the full
// transaction, those plus the private dependencies that orphan with them.
// Removable is false when nothing is installed to remove, so the switch UI can
// say so honestly rather than hide the keep-or-remove choice.
type ReclaimSet struct {
	Outgoing  string   `json:"outgoing"`
	Incoming  string   `json:"incoming"`
	Targets   []string `json:"targets"`
	Packages  []string `json:"packages"`
	Count     int      `json:"count"`
	Size      int64    `json:"size"` // total installed size, bytes
	Removable bool     `json:"removable"`
}

// pacman seams, replaced in tests so the orchestration runs without a live
// pacman.
var (
	pkgInstalled     = pacmanInstalled
	removalOnce      = pacmanRemovalOnce
	installedSizes   = pacmanInstalledSizes
	providerPackages = capsPackages
)

// Reclaim computes the removal set for leaving outgoing to run incoming. Both
// are provider names; their package lists come from caps.
func Reclaim(outgoing, incoming string) (ReclaimSet, error) {
	rs := ReclaimSet{Outgoing: outgoing, Incoming: incoming, Targets: []string{}, Packages: []string{}}
	out, err := providerPackages(outgoing)
	if err != nil {
		return rs, err
	}
	// The incoming provider may not be installed yet, so its binary can be
	// absent; its declared list is still read best-effort to spare a shared
	// satellite from removal.
	in, _ := providerPackages(incoming)
	targets := reclaimTargets(out, in, pkgInstalled)
	rs.Targets = targets
	if len(targets) == 0 {
		return rs, nil // nothing installed to remove: honest empty set
	}
	set, kept, err := removalSet(targets, removalOnce)
	if err != nil {
		return rs, err
	}
	// A target a surviving package still needs is dropped from the removal and
	// stays installed, so the names the switch removes are only the ones that
	// survived the plan.
	rs.Targets = kept
	if len(set) == 0 {
		return rs, nil
	}
	size, err := installedSizes(set)
	if err != nil {
		return rs, err
	}
	rs.Packages = set
	rs.Count = len(set)
	rs.Size = size
	rs.Removable = true
	return rs, nil
}

// reclaimTargets is the pure target selection: the outgoing provider's declared
// packages that are installed, minus any the incoming provider also declares (a
// shared satellite the switch must keep). Order follows the outgoing
// declaration so a caller sees a stable, most-significant-first list.
func reclaimTargets(outgoing, incoming []string, installed func(string) bool) []string {
	shared := make(map[string]bool, len(incoming))
	for _, p := range incoming {
		shared[p] = true
	}
	out := []string{}
	seen := map[string]bool{}
	for _, p := range outgoing {
		if p == "" || seen[p] || shared[p] || !installed(p) {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// removalSet drives pacman to a stable removal transaction. pacman refuses the
// whole transaction when a named target is still required by a surviving
// package; that target is then dropped (kept installed) and the plan asked
// again, so the final set removes everything safe and keeps everything needed.
// It returns the packages removed and the named targets that survived the drops.
func removalSet(targets []string, plan func([]string) (set, blocked []string, err error)) (removed, kept []string, err error) {
	cur := append([]string(nil), targets...)
	for len(cur) > 0 {
		set, blocked, err := plan(cur)
		if err != nil {
			return nil, nil, err
		}
		if len(blocked) == 0 {
			sort.Strings(set)
			return set, cur, nil
		}
		cur = drop(cur, blocked)
	}
	return nil, []string{}, nil
}

// VerifyRemoval refuses a removal whose real pacman transaction would differ
// from the reviewed set. Run immediately before the destructive removal, it asks
// pacman what `-Rs` would take for the surviving targets and compares that to the
// reviewed set: if pacman would pull in a package the review never showed, or a
// target has since become one a surviving package needs, the switch's cleanup is
// refused rather than run, so a removal can never quietly cascade past what the
// user saw.
func VerifyRemoval(rs ReclaimSet) error {
	if !rs.Removable {
		return nil
	}
	set, blocked, err := removalOnce(rs.Targets)
	if err != nil {
		return fmt.Errorf("refusing to remove %s: pacman could not plan the removal: %w", rs.Outgoing, err)
	}
	if len(blocked) > 0 {
		return fmt.Errorf("refusing to remove %s: %s is now required by another package", rs.Outgoing, strings.Join(blocked, " "))
	}
	if extra := difference(set, rs.Packages); len(extra) > 0 {
		return fmt.Errorf("refusing to remove %s: pacman would also remove %s, outside the reviewed set", rs.Outgoing, strings.Join(extra, " "))
	}
	if missing := difference(rs.Packages, set); len(missing) > 0 {
		return fmt.Errorf("refusing to remove %s: pacman would keep %s from the reviewed set", rs.Outgoing, strings.Join(missing, " "))
	}
	return nil
}

// drop returns cur without any member of rm.
func drop(cur, rm []string) []string {
	gone := make(map[string]bool, len(rm))
	for _, p := range rm {
		gone[p] = true
	}
	var out []string
	for _, p := range cur {
		if !gone[p] {
			out = append(out, p)
		}
	}
	return out
}

// difference returns the members of a that are not in b, sorted.
func difference(a, b []string) []string {
	have := make(map[string]bool, len(b))
	for _, x := range b {
		have[x] = true
	}
	var out []string
	for _, x := range a {
		if !have[x] {
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}

// capsPackages reads a provider's declared package list from its caps.
func capsPackages(name string) ([]string, error) {
	caps, err := OpenNamed(name).Caps()
	if err != nil {
		return nil, err
	}
	return caps.Packages, nil
}

// pacmanInstalled reports whether a package is installed.
func pacmanInstalled(name string) bool {
	return exec.Command("pacman", "-Q", name).Run() == nil
}

// compositorVirtual is the package every compositor variant provides and the
// ryoku-desktop umbrella depends on. A switch installs the incoming variant
// before removing the outgoing one, so the plan is measured with this virtual
// assumed present: without that, pacman refuses to drop the outgoing variant
// (the umbrella still needs the virtual) and every satellite it owns with it.
const compositorVirtual = "ryoku-desktop-compositor"

// breaksDep matches pacman's "cannot remove, a survivor needs it" line, whose
// first name is the target that has to stay: "removing X breaks dependency ...".
var breaksDep = regexp.MustCompile(`removing (\S+) breaks dependency`)

// parseRemovalPlan reads a `pacman -Rs --print` transcript. pacman writes the
// refusal lines to STDOUT, not stderr, so the scan covers both streams: the
// targets it refuses to remove come back as blocked (the caller drops them and
// re-plans), and a clean plan's package list is returned as the set.
func parseRemovalPlan(stdout, stderr string, targets []string) (set, blocked []string) {
	want := make(map[string]bool, len(targets))
	for _, t := range targets {
		want[t] = true
	}
	for _, m := range breaksDep.FindAllStringSubmatch(stdout+stderr, -1) {
		if want[m[1]] {
			blocked = append(blocked, m[1])
		}
	}
	if len(blocked) > 0 {
		return nil, blocked
	}
	return nonEmptyLines([]byte(stdout)), nil
}

// pacmanRemovalOnce asks pacman what `-Rs` would remove for the targets, with
// the compositor virtual assumed present so the plan matches the transaction
// the switch will actually run after the incoming variant is installed. On a
// clean plan it returns the packages removed; when pacman still refuses a
// target a survivor needs, that target comes back blocked (nil error) so the
// caller can drop it and ask again. --print touches nothing, and -n (nosave) is
// illegal with --print and removes the same packages, so it is left to the real
// removal.
func pacmanRemovalOnce(targets []string) (set, blocked []string, err error) {
	if len(targets) == 0 {
		return nil, nil, nil
	}
	args := append([]string{
		"-Rs", "--print", "--print-format", "%n",
		"--assume-installed", compositorVirtual + ",1",
	}, targets...)
	cmd := exec.Command("pacman", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, runErr := cmd.Output()
	set, blocked = parseRemovalPlan(string(out), stderr.String(), targets)
	if len(blocked) > 0 {
		return nil, blocked, nil
	}
	if runErr != nil {
		return nil, nil, fmt.Errorf("pacman -Rs --print: %w: %s", runErr, strings.TrimSpace(string(out)+stderr.String()))
	}
	return set, nil, nil
}

// pacmanInstalledSizes sums the installed size in bytes of the named packages,
// read from one `pacman -Qi`.
func pacmanInstalledSizes(names []string) (int64, error) {
	if len(names) == 0 {
		return 0, nil
	}
	args := append([]string{"-Qi"}, names...)
	out, err := exec.Command("pacman", args...).Output()
	if err != nil {
		return 0, fmt.Errorf("pacman -Qi: %w", err)
	}
	var total int64
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "Installed Size") {
			if i := strings.Index(line, ":"); i >= 0 {
				total += parseSize(strings.TrimSpace(line[i+1:]))
			}
		}
	}
	return total, sc.Err()
}

// nonEmptyLines splits pacman's --print output, one package name per line.
func nonEmptyLines(out []byte) []string {
	var lines []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if p := strings.TrimSpace(sc.Text()); p != "" {
			lines = append(lines, p)
		}
	}
	return lines
}

// parseSize converts a pacman "Installed Size" value ("1234.56 KiB") to bytes.
func parseSize(val string) int64 {
	fields := strings.Fields(val)
	if len(fields) < 2 {
		return 0
	}
	n, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	switch fields[1] {
	case "GiB":
		n *= 1024 * 1024 * 1024
	case "MiB":
		n *= 1024 * 1024
	case "KiB":
		n *= 1024
	}
	return int64(n)
}
