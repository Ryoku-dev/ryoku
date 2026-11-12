package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type paperClient struct {
	d       *daemon
	spawnMu sync.Mutex
	nextID  atomic.Uint64
}

func newPaperClient(d *daemon) *paperClient {
	pc := &paperClient{d: d}
	pc.nextID.Store(1)
	return pc
}

const (
	paperStartTimeout = 3 * time.Second
	paperIOTimeout    = 20 * time.Second
	paperMaxMessage   = 1 << 20
)

type paperSource struct {
	Kind       string                 `json:"kind"`
	Path       string                 `json:"path"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// Audio is always sent so a scene follows the picker's choice, not the renderer default (muted at 80).
type paperAssignment struct {
	Outputs  []string    `json:"outputs"`
	Source   paperSource `json:"source"`
	FillMode string      `json:"fill_mode,omitempty"`
	Mute     bool        `json:"mute"`
	Volume   int         `json:"volume"`
	Layer    string      `json:"layer,omitempty"`
}

type paperApplyRequest struct {
	Assignments []paperAssignment `json:"assignments"`
	ReplaceAll  bool              `json:"replace_all,omitempty"`
	Policy      interface{}       `json:"policy,omitempty"`
}

type paperStopRequest struct {
	Outputs []string `json:"outputs,omitempty"`
}

type paperAudioRequest struct {
	Outputs []string `json:"outputs,omitempty"`
	Mute    *bool    `json:"mute,omitempty"`
	Volume  *int     `json:"volume,omitempty"`
}

type paperRequest struct {
	ID     uint64      `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

type paperError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type paperResponse struct {
	ID     uint64          `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *paperError     `json:"error,omitempty"`
}

// Resolved exactly as the renderer does so both ends agree without configuration.
func (pc *paperClient) socketPath() string {
	if p := os.Getenv("SKWD_PAPER_V2_SOCKET"); p != "" {
		return p
	}
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = filepath.Join("/tmp", fmt.Sprintf("skwd-paper-v2-%d", os.Geteuid()))
	}
	return filepath.Join(dir, "skwd-paper-v2", "paper.sock")
}

func (pc *paperClient) binary() string {
	if p := wkString(pc.d.setting("paths.paperBin")); p != "" {
		return resolvePath(p)
	}
	return "skwd-paper-v2"
}

func (pc *paperClient) apply(req paperApplyRequest) error {
	_, err := pc.exchange("paper.apply", req, true)
	return err
}

// Never spawns the renderer: with nothing running there is nothing to stop.
func (pc *paperClient) stop(outputs []string) error {
	if !pc.running() {
		return nil
	}
	_, err := pc.exchange("paper.stop", paperStopRequest{Outputs: outputs}, false)
	return err
}

func (pc *paperClient) setAudio(mute *bool, volume *int, outputs []string) error {
	if mute == nil && volume == nil {
		return nil
	}
	if !pc.running() {
		return nil
	}
	_, err := pc.exchange("paper.audio.set", paperAudioRequest{Outputs: outputs, Mute: mute, Volume: volume}, false)
	return err
}

func (pc *paperClient) running() bool {
	conn, err := net.DialTimeout("unix", pc.socketPath(), 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func (pc *paperClient) exchange(method string, params interface{}, spawn bool) (json.RawMessage, error) {
	id := pc.nextID.Add(1)
	line, err := json.Marshal(paperRequest{ID: id, Method: method, Params: params})
	if err != nil {
		return nil, err
	}
	if len(line)+1 > paperMaxMessage {
		return nil, fmt.Errorf("paper request exceeds %d bytes", paperMaxMessage)
	}
	conn, err := pc.connect(spawn)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(paperIOTimeout))
	if _, err := conn.Write(append(line, '\n')); err != nil {
		return nil, fmt.Errorf("write paper request: %w", err)
	}
	r := bufio.NewReaderSize(conn, 4096)
	respLine, err := readLineCapped(r, paperMaxMessage)
	if err != nil {
		return nil, fmt.Errorf("read paper response: %w", err)
	}
	var resp paperResponse
	if err := json.Unmarshal(respLine, &resp); err != nil {
		return nil, fmt.Errorf("decode paper response: %w", err)
	}
	if resp.ID != id {
		return nil, fmt.Errorf("paper response id %d does not match request %d", resp.ID, id)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("paper %s: %s", resp.Error.Code, resp.Error.Message)
	}
	return resp.Result, nil
}

func (pc *paperClient) connect(spawn bool) (net.Conn, error) {
	sock := pc.socketPath()
	if conn, err := net.DialTimeout("unix", sock, time.Second); err == nil {
		return conn, nil
	}
	if !spawn {
		return nil, fmt.Errorf("skwd-paper is not running")
	}
	pc.spawnMu.Lock()
	defer pc.spawnMu.Unlock()
	// Another apply may have started the renderer while this one waited.
	if conn, err := net.DialTimeout("unix", sock, time.Second); err == nil {
		return conn, nil
	}
	if err := pc.launch(sock); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(paperStartTimeout)
	for time.Now().Before(deadline) {
		if conn, err := net.DialTimeout("unix", sock, 200*time.Millisecond); err == nil {
			return conn, nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return nil, fmt.Errorf("skwd-paper did not bind %s within %s", sock, paperStartTimeout)
}

func (pc *paperClient) launch(sock string) error {
	if err := preparePaperRuntime(filepath.Dir(sock)); err != nil {
		return err
	}
	cmd := exec.Command(pc.binary(), "serve")
	cmd.Env = append(os.Environ(), "SKWD_PAPER_V2_SOCKET="+sock)
	cmd.Env = append(cmd.Env, pc.d.paperServeEnv()...)
	cmd.Stdin = nil
	if log := managedLogPath("paper"); log != "" {
		if f, err := os.OpenFile(log, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644); err == nil {
			cmd.Stdout = f
			cmd.Stderr = f
			defer f.Close()
		}
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start skwd-paper: %w", err)
	}
	// Reap the controller so a short-lived spawn does not linger as a zombie.
	go func() { _ = cmd.Wait() }()
	return nil
}

// skwd-paper refuses to bind in a directory group or other can open. Earlier
// daemons created it 0755, so an existing one is tightened as well.
func preparePaperRuntime(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("secure %s: %w", dir, err)
	}
	return nil
}

func readLineCapped(r *bufio.Reader, cap int) ([]byte, error) {
	var buf []byte
	for {
		chunk, err := r.ReadSlice('\n')
		buf = append(buf, chunk...)
		if len(buf) > cap {
			return nil, fmt.Errorf("paper response exceeds %d bytes", cap)
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil {
			return nil, err
		}
		return buf, nil
	}
}
