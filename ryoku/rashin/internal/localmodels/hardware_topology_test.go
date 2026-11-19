package localmodels

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHardwareReadsPhysicalCoresFromSysfsWithoutCPUInfoIDs(t *testing.T) {
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
	write("proc/cpuinfo", "processor : 0\n\nprocessor : 1\n\nprocessor : 2\n")
	for _, entry := range [][2]string{{"cpu0", "0"}, {"cpu1", "0"}, {"cpu2", "1"}} {
		base := "sys/devices/system/cpu/" + entry[0] + "/topology/"
		write(base+"physical_package_id", "0\n")
		write(base+"core_id", entry[1]+"\n")
	}
	if got := scanHardware(os.DirFS(root), nil).Cores; got != 2 {
		t.Fatalf("physical cores=%d, want 2", got)
	}
}
