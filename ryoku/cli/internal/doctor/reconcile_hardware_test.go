package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBacklightLevels(t *testing.T) {
	root := t.TempDir()
	for dev, cur := range map[string]string{"intel_backlight": "2880", "nvidia_0": "0"} {
		dir := filepath.Join(root, dev)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "brightness"), []byte(cur+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "max_brightness"), []byte("65535\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := backlightSysRoot
	backlightSysRoot = root
	t.Cleanup(func() { backlightSysRoot = old })
	got := backlightLevels([]string{"intel_backlight", "nvidia_0", "absent"})
	want := "intel_backlight 2880/65535, nvidia_0 0/65535, absent ?/?"
	if got != want {
		t.Fatalf("backlightLevels = %q, want %q", got, want)
	}
}
