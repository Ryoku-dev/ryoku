package main

import (
	"encoding/json"

	wm "ryoku-wm"
)

// runState is one snapshot for callers that ask once and exit. It queries
// rather than opening the stream, so a one-shot caller pays one round trip
// per list instead of waiting for a replay to finish.
func runState() error {
	s, err := readStage()
	if err != nil {
		return err
	}
	current, all := s.keyboard.keyboardPair()
	snap := wm.Snapshot{
		FocusedOutput:   s.focused,
		Outputs:         s.outputsFrame(),
		Workspaces:      s.workspaceFrame(),
		Windows:         s.windowFrame(),
		KeyboardLayout:  current,
		KeyboardLayouts: all,
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(snap)
}
