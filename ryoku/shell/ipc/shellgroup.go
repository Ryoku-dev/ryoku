package main

import (
	"syscall"
	"time"
)

// shellGroupGrace is how long a shell's leftover helpers get to leave after
// SIGTERM before they are killed.
var shellGroupGrace = 2 * time.Second

// reapProcessGroup ends every process still in the group a shell run led.
// Quickshell exits on SIGTERM without stopping what its Process objects
// started, so each reload used to leave the shell's watchers and an item's
// pollers running while the next shell started its own. Launches the shell
// detaches (apps opened from the launcher) run in their own session, outside
// the group, and are not touched.
func reapProcessGroup(pgid int) {
	if pgid <= 0 || syscall.Kill(-pgid, syscall.SIGTERM) != nil {
		return
	}
	deadline := time.Now().Add(shellGroupGrace)
	for time.Now().Before(deadline) {
		if syscall.Kill(-pgid, 0) != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
}
