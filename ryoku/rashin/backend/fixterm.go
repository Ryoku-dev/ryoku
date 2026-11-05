package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// fixterm.go opens Fix with AI in the user's own agent harness, in a real
// terminal: the harness's TUI already streams its thinking, shows every tool
// call and its output, asks its own approvals, and switches models, so a repair
// runs where all of that is native. The problem brief is the session's first
// message and the Needle persona rides as extra system prompt where the
// harness takes one.

// fixHarness is an agent whose terminal UI can open a repair session.
type fixHarness struct {
	ID   string
	Name string
	bin  func() (string, bool)
	// argv starts the harness interactively with prompt as the first message.
	argv func(bin, persona, prompt string, s FixSettings) []string
}

func onPath(name string) func() (string, bool) {
	return func() (string, bool) {
		p, err := exec.LookPath(name)
		return p, err == nil
	}
}

// fixHarnesses lists the harnesses Fix with AI can open, in the chat backend
// preference order (Hermes leads).
func fixHarnesses() []fixHarness {
	return []fixHarness{
		{
			ID: "hermes", Name: "Hermes", bin: FindHermes,
			// Hermes has no interactive start with a first message: the brief
			// runs as one query in this terminal, then the same session opens
			// for the rest of the conversation. A failed first query must not
			// resume some older session.
			argv: func(bin, persona, prompt string, _ FixSettings) []string {
				return []string{"sh", "-c", `"$0" chat -q "$1" && exec "$0" chat --continue`, bin, persona + "\n\n" + prompt}
			},
		},
		{
			ID: "omp", Name: "Oh My Pi", bin: onPath("omp"),
			argv: func(bin, persona, prompt string, _ FixSettings) []string {
				return []string{bin, "--append-system-prompt", persona, prompt}
			},
		},
		{
			ID: "claude", Name: "Claude Code", bin: onPath("claude"),
			argv: func(bin, persona, prompt string, _ FixSettings) []string {
				return []string{bin, "--append-system-prompt", persona, prompt}
			},
		},
		{
			ID: "codex", Name: "Codex", bin: onPath("codex"),
			// Codex takes no extra system prompt flag, so the persona leads
			// the first message, as with Hermes.
			argv: func(bin, persona, prompt string, s FixSettings) []string {
				argv := []string{bin}
				if s.Model != "" {
					argv = append(argv, "-m", s.Model)
				}
				if s.Effort != "" {
					argv = append(argv, "-c", fmt.Sprintf("model_reasoning_effort=%q", s.Effort))
				}
				return append(argv, persona+"\n\n"+prompt)
			},
		},
	}
}

// pickFixHarness prefers the agent the fix setting or the chat names, then
// the first installed harness.
func pickFixHarness(want string, hs []fixHarness) (fixHarness, string, bool) {
	for _, h := range hs {
		if h.ID == want {
			if bin, ok := h.bin(); ok {
				return h, bin, true
			}
		}
	}
	for _, h := range hs {
		if bin, ok := h.bin(); ok {
			return h, bin, true
		}
	}
	return fixHarness{}, "", false
}

// fixLaunch is a repair ready to run: which harness, and the command line.
type fixLaunch struct {
	Harness string
	Argv    []string
}

func planFix(display, brief string) (fixLaunch, error) {
	cfg := LoadConfig()
	want := cfg.Fix.Agent
	if want == "" {
		want = cfg.ChatAgent
	}
	h, bin, ok := pickFixHarness(want, fixHarnesses())
	if !ok {
		return fixLaunch{}, errors.New("no agent harness to open; install Hermes or Oh My Pi")
	}
	return fixLaunch{Harness: h.Name, Argv: h.argv(bin, needlePersona, display+"\n\n"+brief, cfg.Fix)}, nil
}

// holdOnFailure keeps the terminal open when the harness exits with an error,
// so a missing key or a crash reads as a message instead of a window that
// flashes and vanishes.
func holdOnFailure(name string, argv []string) []string {
	script := `n=$1; shift; "$@"; s=$?; [ "$s" -eq 0 ] && exit 0; ` +
		`printf '\n%s stopped (exit %d). Press Enter to close.' "$n" "$s"; read -r _; exit "$s"`
	return append([]string{"sh", "-c", script, "ryoku-fix", name}, argv...)
}

// openFixTerminal opens the repair in the user's terminal.
func openFixTerminal(l fixLaunch) error {
	return launchTerminal(
		VaultDir(),
		"ryoku-fix-",
		holdOnFailure(l.Harness, l.Argv),
	)
}

var (
	terminalLookPath = exec.LookPath
	terminalRun      = func(name string, args []string) ([]byte, error) {
		return exec.Command(name, args...).CombinedOutput()
	}
	terminalStart = func(name, cwd string, args []string) error {
		cmd := exec.Command(name, args...)
		cmd.Dir = cwd
		if err := cmd.Start(); err != nil {
			return err
		}
		go func() { _ = cmd.Wait() }()
		return nil
	}
)

// launchTerminal detaches harness TUIs from the daemon so a daemon restart
// cannot close the user's work. Runit systems launch the terminal directly
// and reap it here; systemd systems keep the transient unit lifecycle.
func launchTerminal(cwd, unitPrefix string, argv []string) error {
	app, err := terminalLookPath("ryoku-app")
	if err != nil {
		return errors.New("ryoku-app is missing, so the terminal cannot be opened")
	}
	run, runErr := terminalLookPath("systemd-run")
	if runErr != nil {
		args := append([]string{"terminal", "--"}, argv...)
		if err := terminalStart(app, cwd, args); err != nil {
			return fmt.Errorf("could not open the terminal: %w", err)
		}
		return nil
	}
	args := []string{
		"--user", "--collect", "--quiet",
		"--unit=" + unitPrefix + strconv.FormatInt(time.Now().UnixNano(), 36),
		"--working-directory=" + cwd,
		"--setenv=PATH=" + os.Getenv("PATH"),
		"--", app, "terminal", "--",
	}
	args = append(args, argv...)
	if out, err := terminalRun(run, args); err != nil {
		return fmt.Errorf("could not open the terminal: %v %s", err, out)
	}
	return nil
}

// runFixHere replaces this process with the harness, for a fix asked from a
// terminal: the repair continues right where the user typed.
func runFixHere(l fixLaunch) error {
	if err := os.Chdir(VaultDir()); err != nil {
		return err
	}
	bin, err := exec.LookPath(l.Argv[0])
	if err != nil {
		return err
	}
	return syscall.Exec(bin, l.Argv, os.Environ())
}
