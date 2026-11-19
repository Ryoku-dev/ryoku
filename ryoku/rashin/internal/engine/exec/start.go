package exec

import (
	"context"
	"io"
	"os"
	osexec "os/exec"
	"syscall"
)

type commandPipe struct {
	io.Reader
	io.Writer
	stdin  *os.File
	stdout *os.File
}

func (p *commandPipe) Close() error {
	err := p.stdin.Close()
	if other := p.stdout.Close(); err == nil {
		err = other
	}
	return err
}

// StartCommand starts argv as a child in its own process group, with stderr
// left on the daemon's stderr. The returned pipe talks to stdin and stdout;
// its caller owns Process.Wait, group termination and pipe.Close.
func StartCommand(ctx context.Context, argv []string) (*os.Process, io.ReadWriteCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if len(argv) == 0 || argv[0] == "" {
		return nil, nil, &Error{Code: CodeNotFound, Message: "no tool host command"}
	}
	stdinRead, stdinWrite, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		_ = stdinRead.Close()
		_ = stdinWrite.Close()
		return nil, nil, err
	}
	defer stdinRead.Close()
	defer stdoutWrite.Close()
	cmd := osexec.Command(argv[0], argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdinRead, stdoutWrite, os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err = cmd.Start(); err != nil {
		_ = stdinWrite.Close()
		_ = stdoutRead.Close()
		return nil, nil, startError(argv[0], err)
	}
	if err = ctx.Err(); err != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_, _ = cmd.Process.Wait()
		_ = stdinWrite.Close()
		_ = stdoutRead.Close()
		return nil, nil, err
	}
	return cmd.Process, &commandPipe{Reader: stdoutRead, Writer: stdinWrite, stdin: stdinWrite, stdout: stdoutRead}, nil
}
