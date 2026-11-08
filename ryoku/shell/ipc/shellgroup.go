package main

import (
	"syscall"
	"time"
)

var shellGroupGrace = 2 * time.Second

// quickshell exits on SIGTERM without stopping what its Process objects started
// apps opened from the launcher run in their own session so they're never touched
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
