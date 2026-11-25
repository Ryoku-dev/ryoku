package localmodels

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// decisionMachine writes a simulated /proc for a computer with this much
// memory and these physical cores, and returns its root.
func decisionMachine(t *testing.T, memTotalKiB int, cores int) string {
	t.Helper()
	root := t.TempDir()
	write := func(name, value string) {
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if memTotalKiB > 0 {
		write("proc/meminfo", "MemTotal:       "+strconv.Itoa(memTotalKiB)+" kB\nMemAvailable:   1024000 kB\n")
	}
	var cpuinfo strings.Builder
	for core := range cores {
		// two threads per core: the gate counts cores, not threads
		for range 2 {
			cpuinfo.WriteString("processor : 0\nphysical id : 0\ncore id : " + strconv.Itoa(core) + "\n\n")
		}
	}
	write("proc/cpuinfo", cpuinfo.String())
	return root
}

// The gate reads the same scan local models plan with: total memory and
// physical cores, both at or above the line to pass.
func TestDecisionHardwareGateReadsTheExistingScan(t *testing.T) {
	cases := []struct {
		name        string
		memKiB      int
		cores       int
		want        bool
		reasonParts []string
	}{
		// an 8 GB laptop reports a little under 8 GiB
		{"eight gigabyte laptop", 7_600_000, 4, true, nil},
		{"exactly the line", DecisionMinRAMMiB * 1024, DecisionMinCores, true, nil},
		{"four gigabytes", 3_900_000, 8, false, []string{"too small", "6 GiB", "4 processor cores", "3.7 GiB"}},
		{"two cores", 32_000_000, 2, false, []string{"too small", "and 2."}},
		{"memory unreadable", 0, 8, false, []string{"could not be read"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := hardwareFrom(os.DirFS(decisionMachine(t, c.memKiB, c.cores)))
			if h.Cores != c.cores || h.RAMMiB != c.memKiB/1024 {
				t.Fatalf("scan read %+v from %d KiB and %d cores", h, c.memKiB, c.cores)
			}
			ok, reason := DecisionHardware(h)
			if ok != c.want || (ok && reason != "") || (!ok && reason == "") {
				t.Fatalf("gate = %v, %q", ok, reason)
			}
			for _, part := range c.reasonParts {
				if !strings.Contains(reason, part) {
					t.Fatalf("reason %q does not say %q", reason, part)
				}
			}
		})
	}
}
