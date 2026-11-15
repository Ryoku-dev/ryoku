package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// audio.go owns the default-device volume readout: it watches PipeWire through
// `pactl subscribe`, reads the default sink and source levels, streams them to
// QML on the "audio" topic, and exposes the setters as calls. It is the volume
// half of the audio plane; the mixer's full node graph stays where it is. The
// media-key verbs in control.go write through the same server, so their
// changes arrive here as events without extra wiring, and a change made by any
// other client (another mixer, a script) is observable too.
//
// Frame values are linear floats (1.0 = 100%); audio boost legitimately
// exceeds unity and is carried as-is so the OSD can show the over-range state.

type audioFrame struct {
	Volume    float64 `json:"volume"`
	Muted     bool    `json:"muted"`
	MicVolume float64 `json:"mic_volume"`
	MicMuted  bool    `json:"mic_muted"`
	Sink      string  `json:"sink"`
	Source    string  `json:"source"`
}

// audioSubscribe is the one watcher contract: a line stream that closes when
// the process dies, and a kill for daemon shutdown so the child never outlives
// the unit (KillMode=process would orphan it, and every restart would add one).
type audioSubscribe interface {
	Lines() <-chan string
	Wait() error
	Kill()
}

// runPactl executes a pactl subcommand and returns its combined output.
// Tests replace it to pin the parsing without a sound server.
var runPactl = func(args ...string) (string, error) {
	out, err := exec.Command("pactl", args...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("pactl %s: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// startAudioSubscribe spawns the long-lived event stream. Tests replace it
// with a canned line source. The reader drains the pipe to EOF before
// waiting, so Wait never closes the descriptor under an in-flight read.
var startAudioSubscribe = func() (audioSubscribe, error) {
	cmd := exec.Command("pactl", "subscribe")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	lines := make(chan string, 64)
	done := make(chan error, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
		done <- cmd.Wait()
	}()
	return &pactlSubscribe{lines: lines, done: done, cmd: cmd}, nil
}

type pactlSubscribe struct {
	lines chan string
	done  chan error
	cmd   *exec.Cmd
}

func (p *pactlSubscribe) Lines() <-chan string { return p.lines }

func (p *pactlSubscribe) Wait() error { return <-p.done }

func (p *pactlSubscribe) Kill() {
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
}

type audioState struct {
	topic *stateTopic

	mu      sync.Mutex
	pending *time.Timer
}

// startAudio registers the topic and calls and starts the watcher. A box with
// no PipeWire pulse socket leaves the topic empty; the QML facade keeps the
// last frame and the OSD arms nothing.
func (d *daemon) startAudio() {
	a := &audioState{topic: d.registerTopic("audio")}
	d.audioLevels = a
	d.registerCall("audio.setVolume", func(raw json.RawMessage) (any, error) {
		var args struct {
			Volume float64 `json:"volume"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
		if args.Volume < 0 || args.Volume > 1.5 {
			return nil, fmt.Errorf("volume out of range: %f", args.Volume)
		}
		if _, err := runPactl("set-sink-volume", "@DEFAULT_SINK@", volumePercent(args.Volume)); err != nil {
			return nil, err
		}
		a.kick()
		return nil, nil
	})
	d.registerCall("audio.setMute", func(raw json.RawMessage) (any, error) {
		var args struct {
			Muted bool `json:"muted"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
		flag := "0"
		if args.Muted {
			flag = "1"
		}
		if _, err := runPactl("set-sink-mute", "@DEFAULT_SINK@", flag); err != nil {
			return nil, err
		}
		a.kick()
		return nil, nil
	})
	d.registerCall("audio.state", func(json.RawMessage) (any, error) {
		return a.state(), nil
	})

	a.publish()
	go a.watch(d.quit)
}

// watch keeps a `pactl subscribe` stream alive for the life of the daemon.
// Volume, mute, and default-device changes all arrive as lines; anything on a
// sink, source, or server object kicks a rescan. A dead stream (no server yet
// at login, a pulse-protocol hiccup) reopens with the backlight's backoff.
// Daemon shutdown kills the child and stops the loop, so the long-lived
// subscribe never outlives the unit.
func (a *audioState) watch(quit <-chan struct{}) {
	delay := backlightRetryMin
	for {
		select {
		case <-quit:
			return
		default:
		}
		sub, err := startAudioSubscribe()
		if err != nil {
			log.Printf("ryoku-shell: audio subscribe failed, retrying: %v", err)
			select {
			case <-quit:
				return
			case <-time.After(delay):
			}
			delay = min(delay*2, backlightRetryMax)
			continue
		}
		delay = backlightRetryMin
		// The stream answering means the server is reachable now, even if it
		// was not at daemon start; republish so the first frame is real.
		a.kick()
		for {
			select {
			case <-quit:
				sub.Kill()
				return
			case line, open := <-sub.Lines():
				if !open {
					goto reopen
				}
				if audioEventLine(line) {
					a.kick()
				}
			}
		}
	reopen:
		if err := sub.Wait(); err != nil {
			log.Printf("ryoku-shell: audio stream ended, reopening: %v", err)
		}
		select {
		case <-quit:
			return
		case <-time.After(delay):
		}
	}
}

// audioEventLine reports whether a subscribe line concerns the devices the
// frame reads. Client and card events are other people's business.
func audioEventLine(line string) bool {
	if !strings.HasPrefix(line, "Event ") {
		return false
	}
	return strings.Contains(line, " on sink") ||
		strings.Contains(line, " on source") ||
		strings.Contains(line, " on server")
}

// kick coalesces the burst of events one device switch produces into one
// rescan, the same 100ms window the battery readout uses.
func (a *audioState) kick() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pending == nil {
		a.pending = time.AfterFunc(100*time.Millisecond, a.publish)
		return
	}
	a.pending.Reset(100 * time.Millisecond)
}

var (
	audioVolumeRe = regexp.MustCompile(`\d+(?:\.\d+)?%`)
	audioMuteRe   = regexp.MustCompile(`Mute:\s*(yes|no)`)
)

// state reads the live frame from the server. Each read is a short-lived
// pactl call; failures degrade to the last-known shape rather than blanking
// the topic, because a half-live pulse socket (device gone mid-read) must not
// make the bar forget the volume it is showing.
func (a *audioState) state() audioFrame {
	frame := audioFrame{Volume: -1, MicVolume: -1}
	if out, err := runPactl("get-sink-volume", "@DEFAULT_SINK@"); err == nil {
		frame.Volume = parseAudioPercent(out)
	}
	if out, err := runPactl("get-sink-mute", "@DEFAULT_SINK@"); err == nil {
		frame.Muted = parseAudioMute(out)
	}
	if out, err := runPactl("get-source-volume", "@DEFAULT_SOURCE@"); err == nil {
		frame.MicVolume = parseAudioPercent(out)
	}
	if out, err := runPactl("get-source-mute", "@DEFAULT_SOURCE@"); err == nil {
		frame.MicMuted = parseAudioMute(out)
	}
	if out, err := runPactl("info"); err == nil {
		frame.Sink, frame.Source = parseAudioDefaults(out)
	}
	return frame
}

func (a *audioState) publish() {
	if a.topic == nil {
		return
	}
	frame, err := json.Marshal(a.state())
	if err != nil {
		return
	}
	a.topic.publish(frame)
}

// parseAudioPercent takes the first channel's percentage out of a
// get-sink-volume line and returns the linear fraction.
func parseAudioPercent(out string) float64 {
	m := audioVolumeRe.FindString(out)
	if m == "" {
		return -1
	}
	v, err := strconv.ParseFloat(strings.TrimSuffix(m, "%"), 64)
	if err != nil {
		return -1
	}
	return v / 100
}

func parseAudioMute(out string) bool {
	m := audioMuteRe.FindStringSubmatch(out)
	return len(m) == 2 && m[1] == "yes"
}

// parseAudioDefaults pulls the "Default Sink:" and "Default Source:" node
// names out of `pactl info`.
func parseAudioDefaults(out string) (string, string) {
	sink, source := "", ""
	for _, line := range strings.Split(out, "\n") {
		if rest, ok := strings.CutPrefix(line, "Default Sink: "); ok {
			sink = strings.TrimSpace(rest)
		}
		if rest, ok := strings.CutPrefix(line, "Default Source: "); ok {
			source = strings.TrimSpace(rest)
		}
	}
	return sink, source
}

// volumePercent formats a linear fraction for `pactl set-sink-volume`. One
// decimal keeps 1%-granular values exact and a boosted 1.5 at "150.0%".
func volumePercent(v float64) string {
	return strconv.FormatFloat(v*100, 'f', 1, 64) + "%"
}
