package main

import (
	"bufio"
	"encoding/json"
	"os/exec"
	"strings"
	"syscall"
)

// voicewatch closes the dictation surface when Voxtype finishes on its own.
//
// The Super+` pill is tap-to-toggle: the first tap records and shows the wave,
// the next stops and hides it. But dictation also ends without a second tap --
// Voxtype stops on its own after a silence or when transcription completes --
// and nothing observed that, so the surface stayed open with the mic indicator
// long gone (#244). This watches Voxtype's own state stream and tears the
// surface down the moment it returns to idle, so the card is honest about
// whether it is actually capturing.

// voxtypeActive reports whether a status class means dictation is live. Only
// "recording" and "transcribing" do; "idle" and "stopped" mean the surface
// should not be showing.
func voxtypeActive(class string) bool {
	switch strings.ToLower(strings.TrimSpace(class)) {
	case "recording", "transcribing":
		return true
	default:
		return false
	}
}

// parseVoxtypeClass pulls the state out of one `voxtype status --format json`
// line. Voxtype emits a bare JSON object per state change; a line that is not
// that shape (a stray log, a partial read) yields ok=false and is skipped.
func parseVoxtypeClass(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "{") {
		return "", false
	}
	var s struct {
		Class string `json:"class"`
		Alt   string `json:"alt"`
	}
	if json.Unmarshal([]byte(line), &s) != nil {
		return "", false
	}
	if s.Class != "" {
		return s.Class, true
	}
	if s.Alt != "" {
		return s.Alt, true
	}
	return "", false
}

// voiceStopOnIdle is the close decision, kept pure so the race that caused #244
// is unit-testable: right after `record start` Voxtype's first streamed frame can
// still read "idle" (the transition is pending). Closing on that first frame
// would hide the card the instant it appeared, so the watcher must see an active
// state before an idle one earns a close. It returns the new sawActive flag and
// whether this frame should close the surface.
func voiceStopOnIdle(sawActive bool, class string) (closeNow, nextSawActive bool) {
	if voxtypeActive(class) {
		return false, true
	}
	return sawActive, sawActive
}

// watchVoice streams Voxtype's state for one dictation session, started from
// voice() on the ON edge. It exits on its own once dictation ends (Voxtype
// returns to idle) or the session is stopped, so an idle desktop spawns nothing.
// A stop channel lets a manual OFF tap reap the stream; the watcher only
// performs the surface close itself when Voxtype ended without that tap.
func (d *daemon) watchVoice(stop <-chan struct{}) {
	if _, err := exec.LookPath("voxtype"); err != nil {
		return
	}
	cmd := exec.Command("voxtype", "status", "--follow", "--format", "json")
	// A follow stream that outlives a crash would be reparented to init and hold
	// a daemon client slot forever; Pdeathsig ties it to this process.
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}

	kill := make(chan struct{})
	go func() {
		select {
		case <-d.quit:
			_ = cmd.Process.Kill()
		case <-kill:
		}
	}()

	classes := make(chan string, 8)
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			if class, ok := parseVoxtypeClass(sc.Text()); ok {
				select {
				case classes <- class:
				default:
				}
			}
		}
		close(classes)
	}()

	sawActive := false
	for {
		var closeNow bool
		select {
		case <-d.quit:
			close(kill)
			_ = cmd.Wait()
			return
		case <-stop:
			close(kill)
			_ = cmd.Wait()
			return
		case class, ok := <-classes:
			if !ok {
				// Stream died (Voxtype quit): treat it as dictation having ended.
				closeNow = true
			} else {
				closeNow, sawActive = voiceStopOnIdle(sawActive, class)
			}
		}
		if closeNow {
			close(kill)
			_ = cmd.Wait()
			d.voiceEnded()
			return
		}
	}
}

// voiceEnded tears the surface down after Voxtype finished without a second tap:
// it closes the card and clears the toggle so the next tap starts fresh. It is a
// no-op if a manual tap already closed the surface (voiceOn is false), so the two
// close paths never fight over the state.
func (d *daemon) voiceEnded() {
	d.voiceMu.Lock()
	if !d.voiceOn {
		d.voiceMu.Unlock()
		return
	}
	d.voiceOn = false
	d.voiceStop = nil
	d.voiceMu.Unlock()
	d.ensure("shell")
	shellIpc("closeSurface", d.activeMonitor(), "voice")
}
