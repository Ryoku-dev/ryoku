// Package audio is the voice loop's process I/O: microphone capture as
// 16 kHz signed 16-bit mono PCM read from a recorder child's stdout, the
// PulseAudio-protocol source list, and the Settings level meter. Every
// command it may start is in the argv allowlist below; it never writes a
// file, never names a device path, and never plays sound.
package audio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"
	"syscall"
)

// SampleRateHz is the capture rate the speech engines read
// (voice.vad.sampleRateHz in the spec, which the loader pins).
const SampleRateHz = 16000

// Runner runs one short allowlisted command and returns its stdout. The
// real one is Exec; tests pass a fake, so no sound server is ever asked.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// Stream starts one allowlisted recorder and returns its stdout. Closing
// the reader kills the child and waits for it, so it is reaped.
type Stream func(ctx context.Context, name string, args ...string) (io.ReadCloser, error)

// ErrNotAllowed is returned for any argv outside the allowlist.
var ErrNotAllowed = errors.New("audio: command not in the allowlist")

// ErrNoRecorder is returned by Start when neither pw-cat nor parec is
// installed.
var ErrNoRecorder = errors.New("audio: neither pw-cat nor parec is installed")

// ErrBadNode is returned for a capture target that is not a plain node
// name: empty after trimming is the default source, and anything with a
// path separator, a leading dash or characters outside a node name's set is
// refused.
var ErrBadNode = errors.New("audio: not a source node name")

// Recorder argv: raw 16 kHz s16 mono to stdout. pw-cat's file argument is
// "-" (stdout); parec writes stdout when it is given no file.
var (
	pwCatArgs = []string{"--record", "--raw", "--format", "s16", "--rate", "16000", "--channels", "1"}
	parecArgs = []string{"--raw", "--format=s16le", "--rate=16000", "--channels=1", "--client-name=hyakki"}
)

// pactl reads: the source list and the default source name.
var pactlArgs = [][]string{
	{"-f", "json", "list", "sources"},
	{"get-default-source"},
}

// Allowed reports whether name and args are one of the commands this
// package may start: a pw-cat or parec record to stdout, optionally aimed
// at one source node, or a pactl source read.
func Allowed(name string, args ...string) bool {
	switch name {
	case "pw-cat":
		rest, ok := cutPrefix(args, pwCatArgs)
		if !ok {
			return false
		}
		switch len(rest) {
		case 1:
			return rest[0] == "-"
		case 3:
			return rest[0] == "--target" && validNode(rest[1]) && rest[2] == "-"
		}
		return false
	case "parec":
		rest, ok := cutPrefix(args, parecArgs)
		if !ok {
			return false
		}
		switch len(rest) {
		case 0:
			return true
		case 1:
			node, ok := strings.CutPrefix(rest[0], "--device=")
			return ok && validNode(node)
		}
		return false
	case "pactl":
		return slices.ContainsFunc(pactlArgs, func(a []string) bool { return slices.Equal(a, args) })
	}
	return false
}

func cutPrefix(args, prefix []string) ([]string, bool) {
	if len(args) < len(prefix) || !slices.Equal(args[:len(prefix)], prefix) {
		return nil, false
	}
	return args[len(prefix):], true
}

// validNode accepts a PipeWire or PulseAudio node name or serial: letters,
// digits and . _ : @ -, not starting with a dash, at most 255 bytes. A path
// separator can never pass, so no device file is ever named.
func validNode(n string) bool {
	if n == "" || len(n) > 255 || n[0] == '-' {
		return false
	}
	for _, r := range n {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.', r == '_', r == ':', r == '@', r == '-':
		default:
			return false
		}
	}
	return true
}

func refuse(name string, args []string) error {
	return fmt.Errorf("%w: %s %q", ErrNotAllowed, name, args)
}

// Exec is the real Runner: it runs an allowed command with env (nil
// inherits) and returns its stdout.
func Exec(env []string) Runner {
	return func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if !Allowed(name, args...) {
			return nil, refuse(name, args)
		}
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Env = env
		return cmd.Output()
	}
}

// ExecStream is the real Stream: it starts an allowed recorder with env
// (nil inherits), stdin and stderr closed, and returns its stdout.
func ExecStream(env []string) Stream {
	return func(ctx context.Context, name string, args ...string) (io.ReadCloser, error) {
		if !Allowed(name, args...) {
			return nil, refuse(name, args)
		}
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Env = env
		out, err := cmd.StdoutPipe()
		if err != nil {
			return nil, err
		}
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		return &execReader{ReadCloser: out, cmd: cmd}, nil
	}
}

type execReader struct {
	io.ReadCloser
	cmd *exec.Cmd
}

// Close kills the recorder and reaps it. A recorder that had already exited
// reports its own exit status; one ended by the kill (this Close's or the
// context's) reports nil.
func (r *execReader) Close() error {
	_ = r.ReadCloser.Close()
	_ = r.cmd.Process.Kill()
	werr := r.cmd.Wait()
	if ws, ok := r.cmd.ProcessState.Sys().(syscall.WaitStatus); ok && ws.Signaled() && ws.Signal() == syscall.SIGKILL {
		return nil
	}
	return werr
}
