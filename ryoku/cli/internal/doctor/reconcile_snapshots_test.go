package doctor

import (
	"reflect"
	"ryoku-cli/internal/host"
	"strconv"
	"testing"
)

func TestConfigTimelineOff(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"number only", "SUBVOLUME=\"/\"\nTIMELINE_CREATE=\"no\"\n", true},
		{"timeline on", "TIMELINE_CREATE=\"yes\"\n", false},
		{"missing key", "SUBVOLUME=\"/\"\nNUMBER_LIMIT=\"10\"\n", false},
		{"empty", "", false},
	}
	for _, c := range cases {
		if got := configTimelineOff(c.body); got != c.want {
			t.Errorf("%s: configTimelineOff = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestConfigTimelineLeak(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"ryoku root", "TIMELINE_CREATE=\"no\"\nTIMELINE_CLEANUP=\"yes\"\n", false},
		{"leaks", "TIMELINE_CREATE=\"yes\"\nTIMELINE_CLEANUP=\"no\"\n", true},
		{"creates and cleans", "TIMELINE_CREATE=\"yes\"\nTIMELINE_CLEANUP=\"yes\"\n", false},
		{"cleanup absent", "TIMELINE_CREATE=\"yes\"\n", true},
		{"number-only shipped root", "SUBVOLUME=\"/\"\nTIMELINE_CREATE=\"no\"\nTIMELINE_CLEANUP=\"yes\"\n", false},
	}
	for _, c := range cases {
		if got := configTimelineLeak(c.body); got != c.want {
			t.Errorf("%s: configTimelineLeak = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestDisableTimelineInConfigTouchesOnlyTheKey(t *testing.T) {
	in := "# comment\nTIMELINE_CREATE=\"yes\"\nTIMELINE_CLEANUP=\"no\"\nTIMELINE_LIMIT_HOURLY=\"10\"\n"
	want := "# comment\nTIMELINE_CREATE=\"no\"\nTIMELINE_CLEANUP=\"no\"\nTIMELINE_LIMIT_HOURLY=\"10\"\n"
	if got := disableTimelineInConfig(in); got != want {
		t.Fatalf("disableTimelineInConfig = %q, want %q", got, want)
	}
	numberOnly := "TIMELINE_CREATE=\"no\"\nTIMELINE_CLEANUP=\"yes\"\n"
	if got := disableTimelineInConfig(numberOnly); got != numberOnly {
		t.Fatal("a number-only config must be left alone")
	}
}

func TestParseLeakedTimeline(t *testing.T) {
	csv := "number,cleanup\n" +
		"0,\n" + // base snapshot, never deletable
		"1,number\n" +
		"2,timeline\n" +
		"3,timeline\n" +
		"4,\n" +
		"5,number\n"
	got := parseLeakedTimeline(csv)
	if want := []string{"2", "3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLeakedTimeline = %v, want %v", got, want)
	}
}

func TestParseLeakedTimelineSkipsNonNumbered(t *testing.T) {
	csv := "number,cleanup\nfoo,timeline\n,timeline\n7,timeline\n"
	if got := parseLeakedTimeline(csv); !reflect.DeepEqual(got, []string{"7"}) {
		t.Fatalf("got %v, want [7]", got)
	}
}

func TestParseUntaggedRyoku(t *testing.T) {
	csv := "number,cleanup,description\n" +
		"0,,current\n" + // base snapshot, skipped
		"25,,ryoku gpu passthrough enable\n" + // untagged ryoku -> retag
		"44,,ryoku config import 2026\n" + // untagged ryoku -> retag
		"90,,my manual keep\n" + // untagged but not ryoku -> leave alone
		"91,number,ryoku-update\n" + // already tagged -> leave alone
		"92,timeline,ryoku-update\n" // timeline, handled elsewhere
	got := parseUntaggedRyoku(csv)
	if want := []string{"25", "44"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("parseUntaggedRyoku = %v, want %v", got, want)
	}
}

func TestChunk(t *testing.T) {
	nums := make([]string, 45)
	for i := range nums {
		nums[i] = strconv.Itoa(i + 1)
	}
	got := chunk(nums, 20)
	if len(got) != 3 {
		t.Fatalf("want 3 batches, got %d", len(got))
	}
	if len(got[0]) != 20 || len(got[1]) != 20 || len(got[2]) != 5 {
		t.Errorf("batch sizes = %d,%d,%d; want 20,20,5", len(got[0]), len(got[1]), len(got[2]))
	}
	if c := chunk(nil, 20); c != nil {
		t.Errorf("chunk(nil) = %v, want nil", c)
	}
}

func TestVoidSnapshotCleanupUsesRunitServiceOnly(t *testing.T) {
	t.Setenv("RYOKU_HOST_PKGMGR", "xbps")
	oldManager := doctorPackageManager
	oldService := doctorService
	doctorPackageManager = func() (host.PackageManager, error) { return host.XBPS, nil }
	var calls [][]string
	doctorService = func(args ...string) int {
		calls = append(calls, append([]string(nil), args...))
		return host.ExitFalse
	}
	t.Cleanup(func() {
		doctorPackageManager = oldManager
		doctorService = oldService
	})

	cleanupOff, timelineOn := snapperCleanupServiceState()
	if !cleanupOff || timelineOn {
		t.Fatalf("cleanupOff=%t timelineOn=%t, want true,false", cleanupOff, timelineOn)
	}
	want := [][]string{{"--system", "is-enabled", "snapper-cleanup.timer"}}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("service calls = %q, want %q", calls, want)
	}
}

func TestAddUpdatedbPrunePath(t *testing.T) {
	cases := []struct {
		name        string
		in          string
		want        string
		wantChanged bool
	}{
		{
			name:        "append when absent",
			in:          "PRUNENAMES=\".git\"\n",
			want:        "PRUNENAMES=\".git\"\nPRUNEPATHS=\"/.snapshots\"\n",
			wantChanged: true,
		},
		{
			name:        "extend compact existing assignment",
			in:          "PRUNEPATHS=\"/tmp /var/tmp\"\n",
			want:        "PRUNEPATHS=\"/tmp /var/tmp /.snapshots\"\n",
			wantChanged: true,
		},
		{
			name:        "extend stock spaced assignment in place",
			in:          "# plocate defaults\nPRUNEPATHS = \"/tmp /var/tmp\"\n",
			want:        "# plocate defaults\nPRUNEPATHS = \"/tmp /var/tmp /.snapshots\"\n",
			wantChanged: true,
		},
		{
			name:        "repair duplicate assignment without losing configured paths",
			in:          "PRUNEPATHS = \"/tmp\"\nPRUNEPATHS=\"/.snapshots /var/cache\"\n",
			want:        "PRUNEPATHS = \"/tmp /.snapshots /var/cache\"\n",
			wantChanged: true,
		},
		{
			name:        "already present",
			in:          "PRUNEPATHS=\"/tmp /.snapshots\"\n",
			want:        "PRUNEPATHS=\"/tmp /.snapshots\"\n",
			wantChanged: false,
		},
	}
	for _, c := range cases {
		out, changed := addUpdatedbPrunePath(c.in, "/.snapshots")
		if changed != c.wantChanged {
			t.Errorf("%s: changed = %v, want %v", c.name, changed, c.wantChanged)
		}
		if out != c.want {
			t.Errorf("%s: output = %q, want %q", c.name, out, c.want)
		}
	}
}

func TestSnapshotCleanupReconcilersAreNeutralWhenUnsupported(t *testing.T) {
	old := doctorSnapshots
	doctorSnapshots = func() (bool, string) {
		return false, "Snapshots are not available on this system."
	}
	t.Cleanup(func() { doctorSnapshots = old })

	for name, run := range map[string]func(bool) recResult{
		"cleanup":        reconcileSnapperCleanup,
		"updatedb prune": reconcileUpdatedbPrune,
	} {
		t.Run(name, func(t *testing.T) {
			got := run(false)
			if got.status != recNote || got.detail != "Snapshots are not available on this system." {
				t.Fatalf("result = %#v, want unsupported note", got)
			}
		})
	}
}
