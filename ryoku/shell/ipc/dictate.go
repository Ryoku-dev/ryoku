package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Dictation is Ryoku's own voice typing daemon (`ryoku-dictate`), reached over
// its unix socket. This file owns three things: the socket client, the
// `dictation` bus topic that the desktop renders, and the surface open/close
// that used to be Voxtype's.
//
// The daemon is started on demand and exits its model when idle, so nothing
// here assumes it is already up.

const (
	dictateService   = "ryoku-dictate"
	dictateIdleClose = 1500 * time.Millisecond
)

type dictationFrame struct {
	State   string  `json:"state"`
	Level   float64 `json:"level"`
	Peak    float64 `json:"peak"`
	Elapsed int64   `json:"elapsedMs"`
	Text    string  `json:"text,omitempty"`
	Error   string  `json:"error,omitempty"`
	Backend string  `json:"backend,omitempty"`
	Model   string  `json:"model,omitempty"`
	TookMs  int64   `json:"tookMs,omitempty"`
	Enabled bool    `json:"enabled"`
	// a tap that could not dictate: the card flashes this for a moment
	AttentionMs int64  `json:"attentionMs,omitempty"`
	Notice      string `json:"notice,omitempty"`
	// the engine's own automatic stop, so the card can measure against it
	MaxDurationSecs int64  `json:"maxDurationSecs,omitempty"`
	Device          string `json:"device,omitempty"`
	Mode            string `json:"mode,omitempty"`
	Language        string `json:"language,omitempty"`
	GPU             string `json:"gpu,omitempty"`
}

type dictateConn struct {
	mu   sync.Mutex
	conn net.Conn
}

func dictateSocket() string {
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if runtime == "" {
		return filepath.Join(os.TempDir(), fmt.Sprintf("ryoku-dictate-%d.sock", os.Getuid()))
	}
	return filepath.Join(runtime, "ryoku-dictate.sock")
}

// send writes one command line and returns the reply lines up to "ok" or "end".
//
// The daemon is started on demand and restarts when it is updated, so the
// cached connection is routinely a few seconds stale: the first write into it
// succeeds into the kernel buffer and only the reply then fails. One retry on a
// fresh socket is what turns that into a tap that works rather than a keybind
// that silently does nothing.
func (c *dictateConn) send(line string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	replies, err := c.exchange(line)
	if err == nil {
		return replies, nil
	}
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	return c.exchange(line)
}

// exchange needs a live connection and runs one command over it.
func (c *dictateConn) exchange(line string) ([]string, error) {
	if c.conn == nil {
		conn, err := net.DialTimeout("unix", dictateSocket(), 400*time.Millisecond)
		if err != nil {
			return nil, err
		}
		c.conn = conn
	}
	conn := c.conn
	_ = conn.SetWriteDeadline(time.Now().Add(time.Second))
	if _, err := conn.Write([]byte(line + "\n")); err != nil {
		return nil, err
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	reader := bufio.NewReader(conn)
	var replies []string
	for {
		text, err := reader.ReadString('\n')
		if err != nil {
			return replies, err
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		replies = append(replies, text)
		if text == "ok" || text == "end" {
			return replies, nil
		}
	}
}

func (c *dictateConn) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

// ensureDictate starts the on-demand unit when its socket is missing. A failed
// start is not fatal: the caller reports the reason the surface shows.
func (d *daemon) ensureDictate() {
	if _, err := os.Stat(dictateSocket()); err == nil {
		return
	}
	_ = exec.Command("ryoku-host", "svc", "--user", "start", dictateService).Run()
	for range 20 {
		if _, err := os.Stat(dictateSocket()); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// startDictation registers the topic and the calls, then keeps one connection
// open to mirror the daemon's state into the desktop.
func (d *daemon) startDictation() {
	d.dictTopic = d.registerTopic("dictation")
	d.dictConn = &dictateConn{}
	d.dictFrame = dictationFrame{State: "idle", Enabled: true, Device: "auto", Mode: "paste"}

	d.registerCall("dictation.start", func(json.RawMessage) (any, error) {
		return d.dictateCommand("start")
	})
	d.registerCall("dictation.stop", func(json.RawMessage) (any, error) {
		return d.dictateCommand("stop")
	})
	d.registerCall("dictation.cancel", func(json.RawMessage) (any, error) {
		return d.dictateCommand("cancel")
	})
	d.registerCall("dictation.toggle", func(json.RawMessage) (any, error) {
		return d.dictateCommand("toggle")
	})
	d.registerCall("dictation.configure", func(raw json.RawMessage) (any, error) {
		var args struct {
			Device   string `json:"device"`
			Mode     string `json:"mode"`
			Language string `json:"language"`
			Model    string `json:"model"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
		for key, value := range map[string]string{
			"device": args.Device, "mode": args.Mode,
			"language": args.Language, "model": args.Model,
		} {
			if value == "" {
				continue
			}
			if _, err := d.dictateCommand("set " + key + " " + value); err != nil {
				return nil, err
			}
		}
		return d.dictateStatus()
	})
	d.registerCall("dictation.status", func(json.RawMessage) (any, error) {
		return d.dictateStatus()
	})

	d.publishDictation(d.currentDictation())
	go d.watchDictation()
}

func (d *daemon) dictateCommand(command string) (any, error) {
	d.ensureDictate()
	replies, err := d.dictConn.send(command)
	if err != nil {
		return nil, err
	}
	for _, line := range replies {
		if strings.HasPrefix(line, "error ") {
			return nil, errors.New(strings.TrimPrefix(line, "error "))
		}
	}
	return "ok", nil
}

// dictateStatus asks the daemon for the values the Hub edits and folds them
// into the topic, so a reload of the Hub needs no separate reader.
func (d *daemon) dictateStatus() (any, error) {
	d.ensureDictate()
	replies, err := d.dictConn.send("status")
	if err != nil {
		return nil, err
	}
	frame := d.currentDictation()
	for _, line := range replies {
		key, value, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		switch key {
		case "state":
			frame.State = value
		case "model":
			frame.Model = value
		case "backend":
			frame.Backend = value
		case "device":
			frame.Device = value
		case "mode":
			frame.Mode = value
		case "language":
			frame.Language = value
		case "gpu":
			frame.GPU = value
		case "enabled":
			frame.Enabled = value == "true"
		case "limit":
			frame.MaxDurationSecs, _ = strconv.ParseInt(value, 10, 64)
		}
	}
	d.publishDictation(frame)
	return frame, nil
}

func (d *daemon) currentDictation() dictationFrame {
	d.dictateMu.Lock()
	defer d.dictateMu.Unlock()
	return d.dictFrame
}

// publishDictation merges a frame into the topic. Byte-identical frames are
// suppressed by the topic, so a level stream at 25 Hz stays cheap.
func (d *daemon) publishDictation(frame dictationFrame) {
	d.dictateMu.Lock()
	// Keep the fields the event stream does not carry.
	if frame.Device == "" {
		frame.Device = d.dictFrame.Device
	}
	if frame.Mode == "" {
		frame.Mode = d.dictFrame.Mode
	}
	if frame.MaxDurationSecs == 0 {
		frame.MaxDurationSecs = d.dictFrame.MaxDurationSecs
	}
	if frame.Language == "" {
		frame.Language = d.dictFrame.Language
	}
	if frame.GPU == "" {
		frame.GPU = d.dictFrame.GPU
	}
	frame.Enabled = d.dictFrame.Enabled
	d.dictFrame = frame
	d.dictateMu.Unlock()

	body, err := json.Marshal(frame)
	if err != nil {
		return
	}
	d.dictTopic.publish(body)
}

// watchDictation mirrors the engine's events. A missing daemon is normal:
// it starts on demand and exits after idling.
func (d *daemon) watchDictation() {
	for {
		select {
		case <-d.quit:
			return
		default:
		}
		conn, err := net.DialTimeout("unix", dictateSocket(), 500*time.Millisecond)
		if err != nil {
			time.Sleep(700 * time.Millisecond)
			continue
		}
		if _, err := d.dictateStatus(); err != nil {
			_ = conn.Close()
			continue
		}
		d.streamDictation(conn)
		_ = conn.Close()
	}
}

func (d *daemon) streamDictation(conn net.Conn) {
	reader := bufio.NewReader(conn)
	var recordingAt time.Time
	for {
		select {
		case <-d.quit:
			return
		default:
		}
		_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, _ := strings.Cut(line, " ")
		frame := d.currentDictation()
		switch key {
		case "state":
			frame.State = value
			if value == "recording" {
				recordingAt = time.Now()
				frame.Level, frame.Peak, frame.Text, frame.Error = 0, 0, "", ""
				frame.Notice, frame.AttentionMs = "", 0
			}
			d.publishDictation(frame)
		case "level":
			fields := strings.Fields(value)
			if len(fields) == 2 {
				frame.Level, _ = strconv.ParseFloat(fields[0], 64)
				frame.Peak, _ = strconv.ParseFloat(fields[1], 64)
				if !recordingAt.IsZero() {
					frame.Elapsed = time.Since(recordingAt).Milliseconds()
				}
				d.publishDictation(frame)
			}
		case "final":
			frame.Text = unescapeDictation(value)
			frame.Elapsed = 0
			d.publishDictation(frame)
		case "took":
			frame.TookMs, _ = strconv.ParseInt(value, 10, 64)
			d.publishDictation(frame)
		case "error":
			frame.Error = unescapeDictation(value)
			d.publishDictation(frame)
		case "backend":
			frame.Backend = value
			d.publishDictation(frame)
		case "model":
			frame.Model = unescapeDictation(value)
			d.publishDictation(frame)
		case "gpu":
			frame.GPU = value
			d.publishDictation(frame)
		}
	}
}

func unescapeDictation(text string) string {
	if !strings.Contains(text, "\\") {
		return text
	}
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		if text[i] != '\\' || i+1 >= len(text) {
			out.WriteByte(text[i])
			continue
		}
		i++
		switch text[i] {
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		case '\\':
			out.WriteByte('\\')
		default:
			// An unknown escape is the engine's own backslash: keep both bytes
			// rather than silently dropping one.
			out.WriteByte('\\')
			out.WriteByte(text[i])
		}
	}
	return out.String()
}

// dictationState is what the voice verb needs before it decides.
func (d *daemon) dictationEnabled() bool {
	if _, err := os.Stat(dictateSocket()); err != nil {
		return false
	}
	frame := d.currentDictation()
	return frame.Enabled
}

// voice is the Super+` tap. The card opens on the daemon's own "recording"
// event, so a dictation started from the Hub shows the same surface as a tap.
func (d *daemon) voice() string {
	d.voiceMu.Lock()
	defer d.voiceMu.Unlock()
	d.ensureDictate()
	if !d.dictationEnabled() {
		// the card is driven by the topic, so a refused tap says so through the
		// topic rather than opening a surface the bar may not own
		frame := d.currentDictation()
		frame.Notice = "off"
		frame.AttentionMs = time.Now().UnixMilli()
		d.publishDictation(frame)
		return "ok"
	}
	state := d.currentDictation().State
	var command string
	if state == "recording" {
		command = "stop"
	} else {
		command = "start"
	}
	if _, err := d.dictateCommand(command); err != nil {
		log.Printf("ryoku-shell: dictation %s: %v", command, err)
		frame := d.currentDictation()
		frame.State = "error"
		frame.Error = err.Error()
		d.publishDictation(frame)
		return "ok"
	}
	if command == "start" && d.currentDictation().State != "recording" {
		// The event connection had not caught the transition yet (it is a long
		// lived connection that can be mid-reconnect); pull the state so the
		// card appears with the first syllable rather than a second later.
		if _, err := d.dictateStatus(); err != nil {
			log.Printf("ryoku-shell: dictation status: %v", err)
		}
	}
	return "ok"
}
