package ask

import (
	"strings"
	"testing"
)

func TestMemoryTableSumsAnAppsProcessesInHumanUnits(t *testing.T) {
	ps := "zen 1048576\nzen 1048576\nniri 524288\nps 4096\nWeb Content 1572864\n"
	free := "               total        used        free      shared  buff/cache   available\nMem:     45097156608 28000000000 7000000000 2000000000 18000000000 17179869184\n"
	got := memoryTable(ps, free, 10)
	for _, want := range []string{"The app using the most memory is zen, with 2.0 GiB (2 processes).", "1. zen: 2.0 GiB (2 processes)", "2. Web Content: 1.5 GiB", "3. niri: 512 MiB", "16.0 GiB available"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in\n%s", want, got)
		}
	}
	if strings.Contains(got, "ps:") {
		t.Fatalf("the listing counted itself:\n%s", got)
	}
}

func TestCPUTableReadsTheSecondFrameAndLeavesTopOut(t *testing.T) {
	frame := func(zen, niri string) string {
		return "top - 21:00:00 up 1 day\nTasks: 3 total\n\n    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND\n" +
			"    100 nero      20   0  1000m   100m    10m S  " + zen + "   1.0   0:01.00 zen\n" +
			"    101 nero      20   0  1000m   100m    10m S  " + niri + "   1.0   0:01.00 niri\n" +
			"    102 nero      20   0  1000m   100m    10m R  99.0   1.0   0:01.00 top\n"
	}
	got := cpuTable(frame("90.0", "1.0")+"\n"+frame("5.0", "40.0"), 10)
	if !strings.HasPrefix(got, "The app using the most processor is niri") || !strings.Contains(got, "\n1. niri: 40%") || strings.Contains(got, "top:") {
		t.Fatalf("expected niri first from the second frame without top:\n%s", got)
	}
}
