package localmodels

import (
	"io/fs"
	"os"

	i18n "ryoku-i18n"
)

// The smallest computer a local decision server is asked on. A native
// decision classifier is small, but it shares the computer with the desktop,
// the daemon and the speech engine, and has to answer one short typed
// question inside the default 1500 ms decision budget on the processor
// alone. 6 GiB of memory as the kernel reports it (MemTotal) lets an 8 GB
// computer through after firmware and graphics take their share, and turns a
// 4 GB one away; four physical cores keep that pass from queueing behind
// speech recognition and the compositor. Total memory, not free memory, is
// read: the server is one the user already runs, so what it holds is
// already spent, and free memory moves from one moment to the next.
const (
	DecisionMinRAMMiB = 6 << 10
	DecisionMinCores  = 4
)

// Hardware is this computer as the local model scan reads it: total memory
// and physical processor cores. Zero memory means it could not be read.
type Hardware struct {
	RAMMiB int
	Cores  int
}

// ScanHardware reads this computer with the scan local models plan with.
// It reads a few files under /proc and /sys and starts nothing.
func ScanHardware() Hardware {
	return hardwareFrom(os.DirFS("/"))
}

func hardwareFrom(root fs.FS) Hardware {
	h := scanHardware(root, nil)
	return Hardware{RAMMiB: h.RAMMiB, Cores: h.Cores}
}

// DecisionHardware says whether a local decision server may be asked on
// this computer and, when not, why in plain words.
func DecisionHardware(h Hardware) (bool, string) {
	switch {
	case h.RAMMiB <= 0:
		return false, i18n.T("This computer's memory could not be read, so a local decision model is not asked.")
	case h.RAMMiB < DecisionMinRAMMiB || h.Cores < DecisionMinCores:
		return false, i18n.Tf("This computer is too small for a local decision model: it needs %d GiB of memory and %d processor cores, and has %.1f GiB and %d.", DecisionMinRAMMiB>>10, DecisionMinCores, float64(h.RAMMiB)/1024, h.Cores)
	}
	return true, ""
}
