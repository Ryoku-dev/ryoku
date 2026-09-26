package wm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
)

// Client resolves the provider binary once and speaks the verbs to it. Shared so
// the contract lives in one place: five hand-rolled JSON callers would be five
// copies, and the first schema change would leave four wrong.

// ErrNoProvider is not fatal. The Hub must still open on a box whose provider
// package was removed, so callers treat it as "no compositor features".
var ErrNoProvider = errors.New("no window manager provider available")

// ErrUnsupported is a different message to a user than ErrNoProvider: the
// provider is there, the compositor just cannot do that.
var ErrUnsupported = errors.New("window manager does not support this action")

// Safe for concurrent use. A successful Caps probe is cached for the process
// lifetime: a compositor does not gain features while running, and the keybind
// path cannot afford a fork to re-ask. A failed probe is NOT cached, so a daemon
// that asked before the provider was answering re-probes on the next call rather
// than running its whole life believing the compositor has no features.
type Client struct {
	detection Detection
	bin       string

	capsMu sync.Mutex
	capsOK bool
	caps   Caps
}

// Open resolves the active provider without running it, so constructing one in
// a daemon's init never blocks on a compositor that is still coming up.
func Open() *Client {
	d := Detect()
	c := &Client{detection: d}
	if d.Name != "" {
		c.bin = "ryoku-wm-" + d.Name
	}
	return c
}

// OpenNamed targets a compositor the caller is not in: the installer preparing a
// target, and `ryoku wm use <name>` previewing a switch.
func OpenNamed(name string) *Client {
	return &Client{
		detection: Detection{Name: name, Live: false, Source: "explicit"},
		bin:       "ryoku-wm-" + name,
	}
}

func (c *Client) Detection() Detection { return c.detection }

func (c *Client) Available() bool {
	if c.bin == "" {
		return false
	}
	_, err := exec.LookPath(c.bin)
	return err == nil
}

// Caps probes until it succeeds once, then serves the cached answer. A failed
// probe returns the error and leaves the cache cold, so the next call probes
// again: the provider may still be coming up. A zero Caps means every Has reads
// false rather than assuming Hyprland's feature set.
func (c *Client) Caps() (Caps, error) {
	return c.CapsContext(context.Background())
}

// CapsContext is Caps with cancellation for callers on latency-sensitive
// paths such as display recovery after resume.
func (c *Client) CapsContext(ctx context.Context) (Caps, error) {
	c.capsMu.Lock()
	defer c.capsMu.Unlock()
	if c.capsOK {
		return c.caps, nil
	}
	if c.bin == "" {
		return c.caps, ErrNoProvider
	}
	out, err := c.runContext(ctx, "caps")
	if err != nil {
		return c.caps, err
	}
	var caps Caps
	if err := json.Unmarshal(out, &caps); err != nil {
		return c.caps, err
	}
	c.caps, c.capsOK = caps, true
	return c.caps, nil
}

// Can is the gate before offering an affordance. False on a missing provider
// too, which is what a caller wants.
func (c *Client) Can(want Capability) bool {
	return c.CanContext(context.Background(), want)
}

// CanContext is Can with cancellation propagated to an uncached capability
// probe.
func (c *Client) CanContext(ctx context.Context, want Capability) bool {
	caps, err := c.CapsContext(ctx)
	if err != nil {
		return false
	}
	return caps.Has(want)
}

// Act checks the capability first, so an unsupported action is a typed error
// rather than a fork that exits zero having done nothing.
func (c *Client) Act(a Action, args ...string) error {
	return c.ActContext(context.Background(), a, args...)
}

// ActContext bounds both the capability probe and provider action.
func (c *Client) ActContext(ctx context.Context, a Action, args ...string) error {
	if c.bin == "" {
		return ErrNoProvider
	}
	if need := a.Capability(); need != "" {
		caps, err := c.CapsContext(ctx)
		if err != nil {
			return err
		}
		if !caps.Has(need) {
			return fmt.Errorf("%w: %s", ErrUnsupported, a)
		}
	}
	argv := append([]string{"act", string(a)}, args...)
	_, err := c.runContext(ctx, argv...)
	return err
}

// ActOutput is Act for actions that answer with a value.
func (c *Client) ActOutput(a Action, args ...string) (string, error) {
	return c.ActOutputContext(context.Background(), a, args...)
}

// ActOutputContext bounds both the capability probe and provider action.
func (c *Client) ActOutputContext(ctx context.Context, a Action, args ...string) (string, error) {
	if c.bin == "" {
		return "", ErrNoProvider
	}
	if need := a.Capability(); need != "" {
		caps, err := c.CapsContext(ctx)
		if err != nil {
			return "", err
		}
		if !caps.Has(need) {
			return "", fmt.Errorf("%w: %s", ErrUnsupported, a)
		}
	}
	out, err := c.runContext(ctx, append([]string{"act", string(a)}, args...)...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Apply hands over the neutral store by path, not stdin, so the provider can
// re-read it during a long apply and the call works from a hook.
func (c *Client) Apply(storePath string) (ApplyReport, error) {
	var rep ApplyReport
	if c.bin == "" {
		return rep, ErrNoProvider
	}
	out, err := c.run("apply", storePath)
	if err != nil {
		return rep, err
	}
	if err := json.Unmarshal(out, &rep); err != nil {
		return rep, fmt.Errorf("%s apply: %w", c.bin, err)
	}
	return rep, nil
}

// DryRun asks a provider what it could not honour if the store were applied to
// it, and writes nothing. Ungated, unlike Preview: the question is answerable
// by any provider, and it is how `ryoku wm use` and the Hub tell a user what a
// switch would cost BEFORE the target owns the config. Calling Apply here would
// author the target compositor's config as a side effect of asking.
func (c *Client) DryRun(storePath string) (ApplyReport, error) {
	var rep ApplyReport
	if c.bin == "" {
		return rep, ErrNoProvider
	}
	out, err := c.run("apply", storePath, "--preview")
	if err != nil {
		return rep, err
	}
	if err := json.Unmarshal(out, &rep); err != nil {
		return rep, fmt.Errorf("%s dry run: %w", c.bin, err)
	}
	return rep, nil
}

// Preview pushes the store to the live session without writing any config, for
// the Hub's appearance sliders. Gated on CapLiveConfigEval: a compositor whose
// config is file-only cannot preview, and the Hub then applies on save.
func (c *Client) Preview(storePath string) (ApplyReport, error) {
	var rep ApplyReport
	if c.bin == "" {
		return rep, ErrNoProvider
	}
	if !c.Can(CapLiveConfigEval) {
		return rep, fmt.Errorf("%w: preview", ErrUnsupported)
	}
	out, err := c.run("apply", storePath, "--preview")
	if err != nil {
		return rep, err
	}
	if err := json.Unmarshal(out, &rep); err != nil {
		return rep, fmt.Errorf("%s preview: %w", c.bin, err)
	}
	return rep, nil
}

// State is one full read, for callers that ask a single question and exit.
func (c *Client) State() (Snapshot, error) {
	return c.StateContext(context.Background())
}

// StateContext is State with a caller-owned deadline.
func (c *Client) StateContext(ctx context.Context) (Snapshot, error) {
	var snap Snapshot
	if c.bin == "" {
		return snap, ErrNoProvider
	}
	out, err := c.runContext(ctx, "state")
	if err != nil {
		return snap, err
	}
	if err := json.Unmarshal(out, &snap); err != nil {
		return snap, fmt.Errorf("%s state: %w", c.bin, err)
	}
	return snap, nil
}

// Defaults is the provider's default subtree of the neutral store. Raw JSON
// because defaults are the provider's shape, and the Hub merges rather than
// reasons about them.
func (c *Client) Defaults() ([]byte, error) {
	if c.bin == "" {
		return nil, ErrNoProvider
	}
	return c.run("defaults")
}

// Schema is the provider's exclusive settings rows, in the shape the Hub's
// settings renderer consumes. Each row stays raw: the field set is the
// provider's to define (a control the Hub knows how to draw), and the Hub emits
// it back untouched. Empty with no error when a provider declares none, so a
// compositor with no exclusive settings simply contributes no window-manager
// page rows.
func (c *Client) Schema() ([]json.RawMessage, error) {
	if c.bin == "" {
		return nil, ErrNoProvider
	}
	out, err := c.run("schema")
	if err != nil {
		return nil, err
	}
	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		return []json.RawMessage{}, nil
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	return rows, nil
}

// Binds is the effective bind legend the active provider reports: the shared
// Ryoku catalogue resolved against the user's rebinds, each row struck or
// annotated where the running compositor cannot honour it, followed by that
// compositor's own binds as custom rows. One list, so the cheatsheet and the Hub
// read the legend from the seam instead of parsing a compositor's own config.
// The provider reads the neutral store itself, so the chords reflect what the
// session actually emits. Empty with no error when a provider answers none.
func (c *Client) Binds() ([]BindRow, error) {
	if c.bin == "" {
		return nil, ErrNoProvider
	}
	out, err := c.run("binds")
	if err != nil {
		return nil, err
	}
	return decodeBinds(out)
}

// Session is the wayland-session desktop-entry body for this provider, for the
// installer and the greeter setup to write when the compositor package ships
// none. Raw bytes: the entry is the provider's to author.
func (c *Client) Session() ([]byte, error) {
	if c.bin == "" {
		return nil, ErrNoProvider
	}
	return c.run("session")
}

// Plugins returns raw JSON: the inventory shape is the provider's to define and
// the only consumer reports what it is told.
func (c *Client) Plugins(args ...string) ([]byte, error) {
	if c.bin == "" {
		return nil, ErrNoProvider
	}
	if !c.Can(CapPlugins) {
		return nil, fmt.Errorf("%w: plugins", ErrUnsupported)
	}
	return c.run(append([]string{"plugins"}, args...)...)
}

// Watch streams frames until ctx is done or the provider exits. onFrame runs on
// the reader goroutine, so a consumer doing real work should hand off.
//
// The caller owns restart policy: a compositor restart must not leave a
// permanently cold cache.
func (c *Client) Watch(ctx context.Context, onFrame func(Frame)) error {
	return c.WatchKinds(ctx, nil, onFrame)
}

// WatchKinds is Watch limited to the frame kinds the caller uses. Narrowing
// matters: the provider only reads what it has to emit, so a consumer that
// wants outputs does not make the compositor answer for windows on every
// window event.
func (c *Client) WatchKinds(ctx context.Context, kinds []FrameKind, onFrame func(Frame)) error {
	if c.bin == "" {
		return ErrNoProvider
	}
	argv := []string{"watch"}
	for _, k := range kinds {
		argv = append(argv, string(k))
	}
	cmd := providerCommandContext(ctx, c.bin, argv...)
	// The command helper also kills the complete provider/IPC process group when
	// this watcher is cancelled, so a compositor query cannot outlive it.
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	scan := bufio.NewScanner(stdout)
	// Frames carry full lists; a busy session overruns the 64 KiB default and a
	// truncated frame would read as "windows closed".
	scan.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scan.Scan() {
		line := bytes.TrimSpace(scan.Bytes())
		if len(line) == 0 {
			continue
		}
		var f Frame
		if err := json.Unmarshal(line, &f); err != nil {
			// One bad frame goes stale for that update only; the next repairs it.
			continue
		}
		onFrame(f)
	}
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return nil
	}
	if err := scan.Err(); err != nil {
		return err
	}
	return waitErr
}

func providerCommandContext(ctx context.Context, bin string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	return cmd
}

// run folds stderr into the error so a provider diagnostic reaches the caller's
// log instead of being swallowed.
func (c *Client) run(args ...string) ([]byte, error) {
	return c.runContext(context.Background(), args...)
}

func (c *Client) runContext(ctx context.Context, args ...string) ([]byte, error) {
	cmd := providerCommandContext(ctx, c.bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = ctxErr
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return nil, fmt.Errorf("%s %s: %w", c.bin, strings.Join(args, " "), err)
		}
		return nil, fmt.Errorf("%s %s: %w: %s", c.bin, strings.Join(args, " "), err, msg)
	}
	return stdout.Bytes(), nil
}

// ApplyOutputs hands the provider an output layout the display editor built. It
// persists to the compositor's config and applies live where the compositor
// allows, and reports anything it could not express the same way Apply does. The
// layout is passed by path, like Apply, so a long apply can re-read it. Gated on
// CapMonitorConfig: a provider that cannot configure outputs has no display page.
func (c *Client) ApplyOutputs(layoutPath string) (ApplyReport, error) {
	var rep ApplyReport
	if c.bin == "" {
		return rep, ErrNoProvider
	}
	if !c.Can(CapMonitorConfig) {
		return rep, fmt.Errorf("%w: output layout", ErrUnsupported)
	}
	out, err := c.run("outputs", layoutPath)
	if err != nil {
		return rep, err
	}
	if err := json.Unmarshal(out, &rep); err != nil {
		return rep, fmt.Errorf("%s outputs: %w", c.bin, err)
	}
	return rep, nil
}
