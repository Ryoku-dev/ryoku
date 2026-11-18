package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var sightOutputCommand = exec.CommandContext
var sightOutputLookPath = exec.LookPath

func (s *sightService) outputContext() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}

func sightPrepareCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
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
	cmd.WaitDelay = time.Second
}

var runSightInputCommand = func(ctx context.Context, input, name string, args ...string) ([]byte, error) {
	cmd := sightOutputCommand(ctx, name, args...)
	sightPrepareCommand(cmd)
	cmd.Stdin = strings.NewReader(input)
	var output sightOutput
	cmd.Stdout = &output
	err := cmd.Run()
	if ctx.Err() != nil {
		return output.Bytes(), ctx.Err()
	}
	return output.Bytes(), err
}

func (s *sightService) cancelInputLocked() {
	if s.inputCancel != nil {
		s.inputCancel()
	}
}

func (s *sightService) beginInput() (context.Context, context.CancelFunc, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.options.feature("inplace") || !s.rashin || s.outputContext().Err() != nil {
		return nil, nil, errors.New("failed")
	}
	ctx, cancel := context.WithTimeout(s.outputContext(), 5*time.Second)
	s.inputCancel = cancel
	return ctx, func() {
		cancel()
		s.mu.Lock()
		s.inputCancel = nil
		s.mu.Unlock()
	}, nil
}

func (s *sightService) replace(text, mode string) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("no-text")
	}
	s.inputMu.Lock()
	defer s.inputMu.Unlock()
	ctx, cancel, err := s.beginInput()
	if err != nil {
		return err
	}
	defer cancel()
	if mode == "copy" {
		if _, err := runSightInputCommand(ctx, text, "wl-copy"); err != nil {
			return errors.New("failed")
		}
		return nil
	}
	if _, err := sightOutputLookPath("wtype"); err != nil {
		return errors.New("no-wtype")
	}
	if mode == "type" {
		if _, err := runSightInputCommand(ctx, "", "wtype", "-d", "3", "--", text); err != nil {
			return errors.New("failed")
		}
		return nil
	}
	if mode != "paste" {
		return errors.New("failed")
	}
	readCtx, readCancel := context.WithTimeout(ctx, time.Second)
	previous, readErr := runSightInputCommand(readCtx, "", "wl-paste", "-n", "-t", "text")
	readCancel()
	_, pasteErr := runSightInputCommand(ctx, text, "wl-copy")
	if pasteErr == nil && ctx.Err() == nil {
		_, pasteErr = runSightInputCommand(ctx, "", "wtype", "-M", "ctrl", "-k", "v", "-m", "ctrl")
	}
	restoreErr := s.restoreClipboard(ctx, text, string(previous), readErr == nil, pasteErr == nil)
	if pasteErr != nil || restoreErr != nil || ctx.Err() != nil {
		return errors.New("failed")
	}
	return nil
}

func sightTerminalApp(app, configured string) bool {
	app = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(app)), ".desktop")
	switch app {
	case "kitty", "foot", "footclient", "alacritty", "wezterm", "org.wezfurlong.wezterm",
		"ghostty", "com.mitchellh.ghostty", "konsole", "org.kde.konsole",
		"gnome-console", "org.gnome.console", "kgx", "gnome-terminal", "gnome-terminal-server",
		"org.gnome.terminal", "ptyxis", "org.gnome.ptyxis", "xterm", "uxterm",
		"rxvt", "urxvt", "st", "sakura", "tilix", "com.gexperts.tilix",
		"terminator", "terminology", "xfce4-terminal", "org.xfce.terminal",
		"qterminal", "org.qterminal.qterminal", "lxterminal", "rio", "com.rioterm.rio":
		return true
	}
	fields := strings.Fields(configured)
	if len(fields) == 0 {
		return false
	}
	command := strings.ToLower(filepath.Base(strings.Trim(fields[0], `"'`)))
	return app != "" && (app == command || strings.HasSuffix(app, "."+command))
}

func (s *sightService) focusedTerminal() bool {
	app := s.focusedApp()
	if app == "" || sightTerminalApp(app, "") {
		return app != ""
	}
	home, err := os.UserConfigDir()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(filepath.Join(home, "ryoku", "desktop.json"))
	if err != nil {
		return false
	}
	var config struct {
		Desktop struct {
			Apps struct {
				Terminal string `json:"terminal"`
			} `json:"apps"`
		} `json:"desktop"`
	}
	return json.Unmarshal(data, &config) == nil && sightTerminalApp(app, config.Desktop.Apps.Terminal)
}

func (s *sightService) restoreClipboard(inputCtx context.Context, text, previous string, hadPrevious, pasted bool) error {
	if pasted && hadPrevious {
		timer := time.NewTimer(700 * time.Millisecond)
		select {
		case <-inputCtx.Done():
		case <-timer.C:
		}
		timer.Stop()
	}
	if !hadPrevious && inputCtx.Err() == nil {
		return nil
	}
	// cancellation must still put the user's clipboard back
	ctx, cancel := context.WithTimeout(context.WithoutCancel(s.outputContext()), 5*time.Second)
	defer cancel()
	readCtx, readCancel := context.WithTimeout(ctx, time.Second)
	current, err := runSightInputCommand(readCtx, "", "wl-paste", "-n", "-t", "text")
	readCancel()
	if err != nil || string(current) != text {
		return err
	}
	if hadPrevious {
		_, err = runSightInputCommand(ctx, previous, "wl-copy")
	} else {
		_, err = runSightInputCommand(ctx, "", "wl-copy", "--clear")
	}
	return err
}

func (s *sightService) undoReplace() error {
	s.inputMu.Lock()
	defer s.inputMu.Unlock()
	ctx, cancel, err := s.beginInput()
	if err != nil {
		return err
	}
	defer cancel()
	if _, err := sightOutputLookPath("wtype"); err != nil {
		return errors.New("no-wtype")
	}
	if _, err := runSightInputCommand(ctx, "", "wtype", "-M", "ctrl", "-k", "z", "-m", "ctrl"); err != nil {
		return errors.New("failed")
	}
	return nil
}
