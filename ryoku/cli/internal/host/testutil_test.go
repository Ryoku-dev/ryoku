package host

import (
	"bytes"
	"strings"
	"syscall"
)

type fakeRunner struct {
	commands []Command
	paths    map[string]bool
	results  []Result
	answer   func(Command) Result
	alive    map[int]bool
	signals  []struct {
		pid    int
		signal syscall.Signal
	}
}

func (f *fakeRunner) Run(command Command) Result {
	f.commands = append(f.commands, command)
	if f.answer != nil {
		return f.answer(command)
	}
	if len(f.results) == 0 {
		return Result{}
	}
	result := f.results[0]
	f.results = f.results[1:]
	return result
}
func (f *fakeRunner) LookPath(name string) bool { return f.paths[name] }
func (f *fakeRunner) SignalGroup(pid int, signal syscall.Signal) error {
	f.signals = append(f.signals, struct {
		pid    int
		signal syscall.Signal
	}{pid, signal})
	return nil
}
func (f *fakeRunner) Alive(pid int) bool { return f.alive[pid] }

func testApp(runner *fakeRunner, env map[string]string) (*App, *bytes.Buffer, *bytes.Buffer) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	app := New(Config{Runner: runner, Stdout: stdout, Stderr: stderr, Getenv: func(key string) string { return env[key] }})
	return app, stdout, stderr
}

func argv(command Command) string { return command.Name + " " + strings.Join(command.Args, " ") }
