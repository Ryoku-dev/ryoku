package main

import (
	"strings"
	"testing"
)

// the additive probe lines parse into their own fields; a leftover carries its
// own size + label so the reclaim list is self-contained.
func TestProbeAlongsideParsesNewLines(t *testing.T) {
	stubBackend(t, strings.Join([]string{
		"sectorsize 512",
		"esp_kind ryoku",
		"existing_boot /EFI/BOOT/BOOTX64.EFI",
		"esp /dev/loop0p1",
		"esp_count 2",
		"esp_free_kib 6144",
		"leftover /dev/loop0p3 ryoku 8192",
		"leftover /dev/loop0p4 ryokuboot 1024",
		"verdict ok",
	}, "\n"))
	r := probeAlongside("/dev/loop0")
	if r.espKind != "ryoku" {
		t.Fatalf("espKind = %q, want ryoku", r.espKind)
	}
	if r.existingBoot != "/EFI/BOOT/BOOTX64.EFI" {
		t.Fatalf("existingBoot = %q", r.existingBoot)
	}
	if len(r.leftovers) != 2 {
		t.Fatalf("got %d leftovers, want 2: %+v", len(r.leftovers), r.leftovers)
	}
	if l := r.leftovers[0]; l.size != 8 || l.fs != "ryoku" || !l.reclaim || l.status != "reclaim" {
		t.Fatalf("first leftover wrong: %+v", l)
	}
	if r.leftovers[1].size != 1 {
		t.Fatalf("second leftover size = %d, want 1 GiB", r.leftovers[1].size)
	}
	if r.verdict != "ok" {
		t.Fatalf("verdict = %q, want ok", r.verdict)
	}
	if r.espCount != 2 {
		t.Fatalf("espCount = %d, want 2", r.espCount)
	}
	if r.espFreeKiB != 6144 {
		t.Fatalf("espFreeKiB = %d, want 6144", r.espFreeKiB)
	}
}

func TestAlongsideESPModeFallsBackToDedicated(t *testing.T) {
	m := alongsideModel(100, 16)
	m.espFreeKiB = 6144
	if env := strings.Join(m.installEnv(), "\n"); !strings.Contains(env, "RYOKU_ESP_MODE=dedicated") {
		t.Fatalf("small Windows ESP must select dedicated mode:\n%s", env)
	}
	m.espFreeKiB = 8192
	if env := strings.Join(m.installEnv(), "\n"); !strings.Contains(env, "RYOKU_ESP_MODE=shared") {
		t.Fatalf("ESP with 8 MiB free must select shared mode:\n%s", env)
	}
}

// an older backend that never emits the new lines leaves the new fields empty
// and still parses the lines it does send: absence is tolerated, not fatal.
func TestProbeAlongsideToleratesOldBackend(t *testing.T) {
	stubBackend(t, "sectorsize 512\nesp /dev/loop0p1\nregion 2048 90000000 43900\nverdict ok")
	r := probeAlongside("/dev/loop0")
	if r.espKind != "" || r.existingBoot != "" || len(r.leftovers) != 0 {
		t.Fatalf("old backend must leave new fields empty: %+v", r)
	}
	if r.verdict != "ok" || r.regionStart != 2048 {
		t.Fatalf("existing lines must still parse: %+v", r)
	}
}

// the strategy step names the option honestly per disk kind and dims it, with the
// cause inline, when a hard blocker (no ESP, no GPT) makes alongside impossible.
func TestDiskStrategiesGating(t *testing.T) {
	parts := []part{{dev: "EFI System", size: 1}, {dev: "ryoku", size: 931}}

	dl := diskLayout{parts: parts, gpt: true, espKind: "ryoku", probeVerdict: "ok"}
	items := diskStrategiesFor(dl)
	if items[0].key != "alongside" || items[0].label != "Install alongside (keep existing OS)" {
		t.Fatalf("non-Windows alongside label wrong: %+v", items[0])
	}
	if items[0].hint != "shrink a partition · use free space" {
		t.Fatalf("non-Windows alongside hint wrong: %q", items[0].hint)
	}
	if alongsideBlockReason(dl) != "" {
		t.Fatal("a usable ryoku disk must not block alongside")
	}

	win := diskStrategiesFor(diskLayout{parts: parts, windows: true, gpt: true, espKind: "windows", probeVerdict: "ok"})
	if win[0].label != "Install alongside Windows" {
		t.Fatalf("Windows disk must keep its copy: %+v", win[0])
	}

	noesp := diskLayout{parts: parts, gpt: true, probeVerdict: "no-esp"}
	if alongsideBlockReason(noesp) != "no EFI system partition" {
		t.Fatalf("no-esp reason = %q", alongsideBlockReason(noesp))
	}
	if it := diskStrategiesFor(noesp)[0]; !strings.Contains(it.hint, "unavailable") || !strings.Contains(it.hint, "no EFI system partition") {
		t.Fatalf("blocked alongside must show its reason inline: %q", it.hint)
	}

	if got := alongsideBlockReason(diskLayout{parts: parts, probeVerdict: "ok"}); got != "needs a GPT disk" {
		t.Fatalf("non-GPT reason = %q", got)
	}
}

// a disabled strategy renders in the list but refuses to commit; the enabled
// sibling still commits normally.
func TestDisabledStrategyRefusesCommit(t *testing.T) {
	items := diskStrategiesFor(diskLayout{parts: []part{{dev: "x", size: 1}}, gpt: true, probeVerdict: "no-esp"})
	p := newPicker(items, true)
	p.disabled = map[string]bool{"alongside": true}
	if done, _ := p.update("enter"); done {
		t.Fatal("disabled alongside must not commit on enter")
	}
	if done, _ := p.update("1"); done {
		t.Fatal("disabled alongside must not commit on the number key")
	}
	p.update("down") // move to whole
	done, sel := p.update("enter")
	if !done || items[sel].key != "whole" {
		t.Fatalf("the enabled option must still commit: done=%v sel=%d", done, sel)
	}
}

// the carve row honours both step sizes: ←/→ nudges 1 GiB, Shift+←/→ jumps the
// carve wave's 10 GiB, and the jump is reversible.
func TestCarveShiftKeyDispatch(t *testing.T) {
	m := carveModel(0) // auto-selects the NTFS carve target; lsel 0 is the carve row
	base := m.carveTakeMiB
	m.partKey("shift+right")
	if m.carveTakeMiB != base+carveBigStepMiB {
		t.Fatalf("shift+right: take %d, want %d", m.carveTakeMiB, base+carveBigStepMiB)
	}
	m.partKey("shift+left")
	if m.carveTakeMiB != base {
		t.Fatalf("shift+left must undo the jump: take %d, want %d", m.carveTakeMiB, base)
	}
	m.partKey("right")
	if m.carveTakeMiB != base+carveStepMiB {
		t.Fatalf("right: take %d, want %d", m.carveTakeMiB, base+carveStepMiB)
	}
}

// the ±big hint appears exactly when the handler will fire: the carve row is the
// active target. An idle carve row (nothing selected) advertises only ←/→.
func TestCarveFooterHintTracksState(t *testing.T) {
	active := carveModel(0) // full disk -> the target auto-selects
	active.flow, active.idx, active.w = []step{{kind: kPartition}}, 0, 112
	if f := active.footer(); !strings.Contains(f, "Ryoku's space") || !strings.Contains(f, "±big") {
		t.Fatalf("active carve row must advertise Ryoku's space + ±big: %q", f)
	}
	idle := carveModel(200)                                                        // roomy gap -> nothing auto-selected
	idle.flow, idle.idx, idle.w, idle.lsel = []step{{kind: kPartition}}, 0, 112, 1 // row 1 is the carve row
	if f := idle.footer(); strings.Contains(f, "±big") {
		t.Fatalf("an idle carve row must not advertise ±big: %q", f)
	}
}

func reviewModel() model {
	return model{
		picks: map[string]string{"disk": "alongside", "keyboard": "us", "locale": "en_US.UTF-8",
			"timezone": "Europe/Madrid", "profile": "amd", "browser": "firefox", "login-shell": "fish",
			"hostname": "ryoku", "username": "me", "password": "x", "encryption": "none"},
		diskDev: "/dev/loop0", gpt: true, freeG: 200, espG: 1, espFreeKiB: 8192, swapG: 8,
		kept:      []part{{dev: "EFI System", size: 1}, {dev: "ryoku", size: 931}},
		netOnline: true,
	}
}

// a ryoku/linux shared-ESP alongside review: green strategy, the shared-ESP boot
// line, the existing_boot=none caveat, and never the whole-disk ERASING line.
func TestReviewCopyRyokuEsp(t *testing.T) {
	m := reviewModel()
	m.espKind, m.existingBoot = "ryoku", "none"
	body := m.reviewBody(100)
	if !strings.Contains(body, "alongside (keep existing OS)") {
		t.Fatal("strategy cell must stay green for alongside")
	}
	if strings.Contains(body, "ERASING") {
		t.Fatal("alongside must never print the whole-branch ERASING line")
	}
	if !strings.Contains(body, "shared existing ESP") {
		t.Fatalf("missing shared-ESP boot line: %q", body)
	}
	if strings.Contains(body, "entry in the boot menu") {
		t.Fatal("no chainload entry exists, so the boot line must not promise one")
	}
	if !strings.Contains(body, "firmware menu only") {
		t.Fatal("existing_boot none must surface the honest caveat")
	}
	if strings.Contains(body, "reclaim") {
		t.Fatal("no leftovers means no reclaim wording")
	}
}

// a linux ESP with a real chainload path shows the entry and drops the caveat.
func TestReviewCopyLinuxEspWithBoot(t *testing.T) {
	m := reviewModel()
	m.espKind, m.existingBoot = "linux", "/EFI/systemd/systemd-bootx64.efi"
	body := m.reviewBody(100)
	if !strings.Contains(body, "Linux (existing)") {
		t.Fatalf("linux ESP must name a Linux entry: %q", body)
	}
	if strings.Contains(body, "firmware menu only") {
		t.Fatal("a chainloadable binary must not trigger the firmware-only caveat")
	}
}

func TestReviewCopyDedicatedESP(t *testing.T) {
	m := reviewModel()
	m.espKind, m.existingBoot, m.espFreeKiB = "windows", "/EFI/Microsoft/Boot/bootmgfw.efi", 6144
	body := m.reviewBody(100)
	if !strings.Contains(body, "dedicated Ryoku ESP") || !strings.Contains(body, "existing Windows ESP is untouched") {
		t.Fatalf("dedicated fallback is not explicit in review: %q", body)
	}
	if strings.Contains(body, "backed up first") {
		t.Fatal("dedicated mode must not claim it mounts or backs up the Windows ESP")
	}
}

// the reclaim ack only appears with verified leftovers, in honest wording, and
// without the old misleading "your other OS is untouched" line.
func TestReviewReclaimWording(t *testing.T) {
	m := reviewModel()
	m.espKind, m.existingBoot = "windows", "/EFI/Microsoft/Boot/bootmgfw.efi"
	m.reclaim, m.reclaimG = []part{{dev: "previous Ryoku", size: 8}}, 8
	body := m.reviewBody(100)
	if strings.Contains(body, "ERASING") {
		t.Fatal("alongside reclaim must not print the whole-branch ERASING line")
	}
	if !strings.Contains(body, "reclaiming 1 leftover partition(s) (8G) from a failed prior install") {
		t.Fatalf("reclaim ack wording wrong: %q", body)
	}
	if strings.Contains(body, "your other OS is untouched") {
		t.Fatal("the misleading untouched-OS line must be gone")
	}
}

// acceptance gate: no whole-branch ERASING is reachable with strategy=alongside,
// even on a populated disk with leftovers; whole disk still shows it.
func TestReviewNoEraseReachableForAlongside(t *testing.T) {
	m := reviewModel()
	m.existing = []part{{dev: "p1"}, {dev: "p2"}}
	m.reclaim, m.reclaimG = []part{{dev: "previous Ryoku", size: 8}}, 8
	if strings.Contains(m.reviewBody(100), "ERASING") {
		t.Fatal("alongside review must never reach the whole-branch ERASING line")
	}
	m.picks["disk"] = "whole"
	if !strings.Contains(m.reviewBody(100), "ERASING") {
		t.Fatal("whole-disk review must still show ERASING (control case)")
	}
}

// a disk with more than one ESP surfaces the multi-ESP review note naming the
// shared ESP kind; a single-ESP disk shows no such note.
func TestReviewCopyMultiEsp(t *testing.T) {
	m := reviewModel()
	m.espKind, m.existingBoot, m.espCount = "windows", "/EFI/Microsoft/Boot/bootmgfw.efi", 2
	body := m.reviewBody(100)
	if !strings.Contains(body, "2 EFI partitions found") {
		t.Fatalf("multi-ESP review must name the count: %q", body)
	}
	if !strings.Contains(body, "shares the windows ESP") {
		t.Fatalf("multi-ESP note must name the shared ESP kind: %q", body)
	}
	m.espCount = 1
	if strings.Contains(m.reviewBody(100), "EFI partitions found") {
		t.Fatal("a single-ESP disk must not show the multi-ESP note")
	}
}
