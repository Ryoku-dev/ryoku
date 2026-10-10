package doctor

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// one `snapper delete` handles at most this many: a huge batch can time out
// over D-Bus partway and strand the rest.
const snapshotDrainBatch = 20

// reconcileSnapperCleanup keeps pruning working. Pruning normally runs only via
// snapper-cleanup.timer -> snapper-cleanup.service, whose success is coupled to
// limine-snapper-sync (its ExecStopPost); when that breaks nothing is pruned
// and every update leaves another unremovable pair. So keep the cleanup timer
// on, the timeline timer off (Ryoku prunes by number, and Cleanup=timeline
// snapshots leak past number cleanup), and drain leaked timeline snapshots.
// Draining runs only under a number-only config (TIMELINE_CREATE="no").
func reconcileSnapperCleanup(checkOnly bool) recResult {
	if supported, reason := doctorSnapshots(); !supported {
		return noteRes("%s", reason)
	}
	if !sys.Exists("/etc/snapper/configs/root") {
		return okRes(i18n.T("root snapshots not configured, nothing to prune"))
	}
	if !sys.Has("snapper") {
		return okRes(i18n.T("snapper is not installed, so there is nothing to prune"))
	}

	cleanupOff := !sys.UnitEnabled("snapper-cleanup.timer")
	timelineOn := sys.UnitEnabled("snapper-timeline.timer")
	numberOnly := snapperTimelineOff()
	var leaked []string
	if numberOnly {
		leaked = leakedTimelineSnapshots()
	}
	// Untagged Ryoku snapshots (GPU passthrough, config import) carry no cleanup
	// algorithm, so number cleanup never reclaims them and they pile up. Retag
	// them so the cap applies. Leaks under any config, so computed unconditionally.
	orphans := untaggedRyokuSnapshots()

	if checkOnly {
		switch {
		case cleanupOff:
			return wouldRes(i18n.T("snapper-cleanup.timer is disabled, so snapshots are never pruned on a schedule")).
				withFix(i18n.T("ryoku doctor (enables snapper-cleanup.timer)"))
		case timelineOn && numberOnly:
			return wouldRes(i18n.T("snapper-timeline.timer is enabled; Ryoku prunes by number only, so timeline snapshots leak")).
				withFix(i18n.T("ryoku doctor (disables snapper-timeline.timer)"))
		case len(leaked) > 0:
			return wouldRes(i18n.T("%d leaked timeline snapshot(s) that number cleanup never reclaims"), len(leaked)).
				withFix(i18n.T("ryoku doctor (deletes them in batches, then runs snapper cleanup number)"))
		case len(orphans) > 0:
			return wouldRes(i18n.T("%d untagged Ryoku snapshot(s) that number cleanup never reclaims"), len(orphans)).
				withFix(i18n.T("ryoku doctor (tags them for number cleanup, then prunes to NUMBER_LIMIT)"))
		}
		return okRes(i18n.T("snapshot cleanup is healthy"))
	}

	var fixes []string
	if cleanupOff {
		if err := sys.Run("sudo", "systemctl", "enable", "--now", "snapper-cleanup.timer"); err == nil {
			fixes = append(fixes, i18n.T("enabled snapper-cleanup.timer"))
		}
	}
	if timelineOn && numberOnly {
		if err := sys.Run("sudo", "systemctl", "disable", "--now", "snapper-timeline.timer"); err == nil {
			fixes = append(fixes, i18n.T("disabled snapper-timeline.timer"))
		}
	}
	if len(leaked) > 0 {
		deleted, failed := drainSnapshots(leaked, snapshotDrainBatch)
		if deleted > 0 {
			fixes = append(fixes, fmt.Sprintf(i18n.T("deleted %d leaked timeline snapshot(s)"), deleted))
		}
		if failed > 0 {
			fixes = append(fixes, fmt.Sprintf(i18n.T("%d timeline snapshot(s) could not be deleted (retry: sudo snapper -c root cleanup number)"), failed))
		}
	}
	if len(orphans) > 0 {
		if tagged := retagSnapshotsNumber(orphans); tagged > 0 {
			fixes = append(fixes, fmt.Sprintf(i18n.T("tagged %d untagged Ryoku snapshot(s) for number cleanup"), tagged))
		}
	}
	// catch the numbered pile up now, not at the next timer tick.
	if len(fixes) > 0 {
		_ = sys.Run("sudo", "snapper", "-c", "root", "cleanup", "number")
	}

	if len(fixes) == 0 {
		return okRes(i18n.T("snapshot cleanup is healthy"))
	}
	return fixedRes("%s", strings.Join(fixes, "; "))
}

// snapperTimelineOff reports number-only cleanup. The config is 0640 root:root,
// so fall back to a cached sudo read; an unreadable config reads as "not off"
// so the drain never fires on a guess.
func snapperTimelineOff() bool {
	b, err := os.ReadFile("/etc/snapper/configs/root")
	contents := string(b)
	if err != nil {
		contents, _ = sys.RunOut("sudo", "-n", "cat", "/etc/snapper/configs/root")
	}
	return configTimelineOff(contents)
}

func configTimelineOff(contents string) bool {
	for _, line := range strings.Split(contents, "\n") {
		if strings.TrimSpace(line) == `TIMELINE_CREATE="no"` {
			return true
		}
	}
	return false
}

// leakedTimelineSnapshots lists Cleanup=timeline numbers, read-only and
// non-prompting; a machine that cannot answer yields nothing to drain.
func leakedTimelineSnapshots() []string {
	out, err := sys.RunOut("sudo", "-n", "snapper", "-c", "root", "--csvout",
		"list", "--columns", "number,cleanup")
	if err != nil {
		return nil
	}
	return parseLeakedTimeline(out)
}

// parseLeakedTimeline returns timeline-cleanup numbers, skipping the header and
// the base snapshot 0.
func parseLeakedTimeline(csv string) []string {
	var out []string
	for _, line := range strings.Split(csv, "\n") {
		fields := strings.Split(strings.TrimSpace(line), ",")
		if len(fields) < 2 {
			continue
		}
		num, cleanup := strings.TrimSpace(fields[0]), strings.TrimSpace(fields[1])
		if num == "" || num == "0" || !allDigits(num) {
			continue
		}
		if cleanup == "timeline" {
			out = append(out, num)
		}
	}
	return out
}

// untaggedRyokuSnapshots lists Ryoku-made snapshots with no cleanup algorithm
// (GPU passthrough, config import), read-only and non-prompting. Only Ryoku's
// own snapshots are touched, never a user's hand-made ones.
func untaggedRyokuSnapshots() []string {
	out, err := sys.RunOut("sudo", "-n", "snapper", "-c", "root", "--csvout",
		"list", "--columns", "number,cleanup,description")
	if err != nil {
		return nil
	}
	return parseUntaggedRyoku(out)
}

// parseUntaggedRyoku returns the numbers of Ryoku snapshots with an empty
// cleanup algorithm, skipping the header and base snapshot 0. SplitN keeps a
// comma inside a description in the last field.
func parseUntaggedRyoku(csv string) []string {
	var out []string
	for _, line := range strings.Split(csv, "\n") {
		fields := strings.SplitN(strings.TrimSpace(line), ",", 3)
		if len(fields) < 3 {
			continue
		}
		num, cleanup, desc := strings.TrimSpace(fields[0]), strings.TrimSpace(fields[1]), fields[2]
		if num == "" || num == "0" || !allDigits(num) {
			continue
		}
		if cleanup == "" && strings.Contains(desc, "ryoku") {
			out = append(out, num)
		}
	}
	return out
}

// retagSnapshotsNumber assigns the number cleanup algorithm to each snapshot so
// the next `snapper cleanup number` prunes it under NUMBER_LIMIT.
func retagSnapshotsNumber(numbers []string) (tagged int) {
	for _, n := range numbers {
		if err := sys.Sudo("snapper", "-c", "root", "modify", "-c", "number", n); err == nil {
			tagged++
		}
	}
	return tagged
}

// drainSnapshots deletes numbers in batches, tolerating a failed batch so the
// rest still drain.
func drainSnapshots(numbers []string, size int) (deleted, failed int) {
	for _, batch := range chunk(numbers, size) {
		args := append([]string{"snapper", "-c", "root", "delete"}, batch...)
		if err := sys.Sudo(args...); err != nil {
			failed += len(batch)
		} else {
			deleted += len(batch)
		}
	}
	return deleted, failed
}

func chunk(s []string, size int) [][]string {
	if size <= 0 {
		size = len(s)
	}
	var out [][]string
	for i := 0; i < len(s); i += size {
		end := i + size
		if end > len(s) {
			end = len(s)
		}
		out = append(out, s[i:end])
	}
	return out
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// reconcileUpdatedbPrune stops plocate/mlocate indexing every file under
// /.snapshots, which turns updatedb into an hours-long crawl on a
// snapshot-heavy box. No updatedb.conf means locate is not installed.
func reconcileUpdatedbPrune(checkOnly bool) recResult {
	if supported, reason := doctorSnapshots(); !supported {
		return noteRes("%s", reason)
	}
	const path = "/etc/updatedb.conf"
	if !sys.Exists(path) {
		return okRes(i18n.T("no /etc/updatedb.conf (locate not installed)"))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return okRes(i18n.T("cannot read %s (%v); leaving it alone"), path, err)
	}
	next, changed := addUpdatedbPrunePath(string(b), "/.snapshots")
	if !changed {
		return okRes(i18n.T("updatedb already skips /.snapshots"))
	}
	if checkOnly {
		return wouldRes(i18n.T("updatedb indexes /.snapshots, so plocate crawls every snapshot")).
			withFix(i18n.T("ryoku doctor (adds /.snapshots to PRUNEPATHS in /etc/updatedb.conf)"))
	}
	if err := writeRootFile(path, next, "0644"); err != nil {
		return failRes(i18n.T("updating %s: %v"), path, err)
	}
	return fixedRes(i18n.T("added /.snapshots to updatedb PRUNEPATHS so plocate skips snapshots"))
}

// updatedbPrunePathsLine accepts both the package's `PRUNEPATHS = "..."` and
// Ryoku's earlier compact spelling. Keeping the original prefix and suffix
// preserves package comments and avoids rewriting unrelated configuration.
var updatedbPrunePathsLine = regexp.MustCompile(`^(\s*PRUNEPATHS\s*=\s*)"([^"]*)"(\s*(?:#.*)?)$`)

// addUpdatedbPrunePath ensures path is in the sole PRUNEPATHS assignment. A
// previous Ryoku version appended a compact second assignment to stock configs;
// merge every assignment so an update repairs that invalid configuration.
func addUpdatedbPrunePath(conf, path string) (string, bool) {
	lines := strings.Split(conf, "\n")
	first := -1
	var prefix, suffix string
	paths := make([]string, 0)
	seen := make(map[string]bool)
	duplicates := make(map[int]bool)

	for i, line := range lines {
		match := updatedbPrunePathsLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if first < 0 {
			first, prefix, suffix = i, match[1], match[3]
		} else {
			duplicates[i] = true
		}
		for _, existing := range strings.Fields(match[2]) {
			if !seen[existing] {
				seen[existing] = true
				paths = append(paths, existing)
			}
		}
	}

	if first < 0 {
		out := conf
		if out != "" && !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		return out + `PRUNEPATHS="` + path + `"` + "\n", true
	}
	if !seen[path] {
		paths = append(paths, path)
	}
	next := prefix + `"` + strings.Join(paths, " ") + `"` + suffix
	out := make([]string, 0, len(lines)-len(duplicates))
	for i, line := range lines {
		if duplicates[i] {
			continue
		}
		if i == first {
			out = append(out, next)
			continue
		}
		out = append(out, line)
	}
	merged := strings.Join(out, "\n")
	return merged, merged != conf
}
