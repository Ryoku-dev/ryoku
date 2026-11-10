package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"

	"ryoku-cli/internal/sys"
)

// rashinOn decides whether doctor offers the optional Rashin fix command.
func rashinOn() bool {
	if !sys.Has("rashin") {
		return false
	}
	b, err := os.ReadFile(filepath.Join(sys.ConfigHome(), "ryoku", "rashin.json"))
	if err != nil {
		return false
	}
	var c struct {
		Enabled bool `json:"enabled"`
	}
	return json.Unmarshal(b, &c) == nil && c.Enabled
}
