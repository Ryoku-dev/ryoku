package doctor

import (
	"os"
	"path/filepath"
	"strings"

	"ryoku-cli/internal/sys"
)

// Ledger of packages Ryoku installed itself (the shipped apps, the ASUS daemon).
// A recorded name that is now missing means the user removed it, so nothing puts
// it back; deleting the name from the file re-arms provisioning.
var provisionedFile = func() string {
	return filepath.Join(sys.StateDir(), "provisioned")
}

func provisioned() map[string]bool {
	out := map[string]bool{}
	b, err := os.ReadFile(provisionedFile())
	if err != nil {
		return out
	}
	for _, ln := range strings.Split(string(b), "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			out[ln] = true
		}
	}
	return out
}

func recordProvisioned(pkg string) {
	if provisioned()[pkg] {
		return
	}
	path := provisionedFile()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(pkg + "\n")
}

// removedByUser reports whether pkg was provisioned by the doctor before and
// is gone now, which only a deliberate removal explains.
func removedByUser(pkg string) bool {
	return provisioned()[pkg] && !doctorPackageInstalled(pkg)
}
