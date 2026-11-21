package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	wm "ryoku-wm"
)

type event struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

func runWatch() error {
	conn, err := subscribe([]string{"windows", "workspaces", "outputs", "overview", "keyboard_layout"})
	if err != nil {
		return err
	}
	defer conn.Close()

	var state uState
	haveOutputs := false
	ready := false
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("umbriel watch: %w", err)
		}
		var ev event
		if json.Unmarshal(line, &ev) != nil {
			continue
		}
		// The first decoded event completes the initial picture. The daemon
		// needs FrameReady to mark this connection live; it learns the focused
		// screen only from FrameFocus, not from the focused field on outputs.
		if !ready {
			ready = true
			if err := emit(wm.Frame{Kind: wm.FrameReady}); err != nil {
				return err
			}
			// The bar hides its layout indicator until it sees FrameKeyboard.
			if err := emitKeyboardFrame(emit); err != nil {
				return err
			}
		}
		switch ev.Event {
		case "outputs":
			if json.Unmarshal(ev.Data, &state.Outputs) == nil {
				haveOutputs = true
				if err := emit(wm.Frame{Kind: wm.FrameOutputs, Outputs: outputsOf(state)}); err != nil {
					return err
				}
				if focus := focusedOutput(state); focus != "" {
					if err := emit(wm.Frame{Kind: wm.FrameFocus, FocusedOutput: focus}); err != nil {
						return err
					}
				}
			}
		case "workspaces":
			if json.Unmarshal(ev.Data, &state.Workspaces) == nil {
				if err := emit(wm.Frame{Kind: wm.FrameWorkspaces, Workspaces: workspacesOf(state)}); err != nil {
					return err
				}
				if focus := focusedWorkspaceOutput(state); focus != "" {
					if err := emit(wm.Frame{Kind: wm.FrameFocus, FocusedOutput: focus}); err != nil {
						return err
					}
				}
				// Outputs carry their active workspace as a name resolved from
				// this list. Refresh them only once an outputs event has supplied
				// the output set, or a workspace-first stream would publish an
				// empty frame that tells consumers all outputs disappeared.
				if haveOutputs {
					if err := emit(wm.Frame{Kind: wm.FrameOutputs, Outputs: outputsOf(state)}); err != nil {
						return err
					}
				}
			}
		case "windows":
			if json.Unmarshal(ev.Data, &state.Windows) == nil {
				if err := emit(wm.Frame{Kind: wm.FrameWindows, Windows: windowsOf(state)}); err != nil {
					return err
				}
			}
		case "keyboard_layout":
			if err := emitKeyboardFrame(emit); err != nil {
				return err
			}
		case "overview":
			var o struct {
				Open bool `json:"open"`
			}
			if json.Unmarshal(ev.Data, &o) == nil {
				if err := emit(wm.Frame{Kind: wm.FrameOverview, OverviewOpen: o.Open}); err != nil {
					return err
				}
			}
		}
	}
}

// emitKeyboardFrame reports the loaded layouts and the active one so the bar
// can show its layout indicator. Umbriel's keyboard_layout IPC carries both.
func emitKeyboardFrame(emit func(wm.Frame) error) error {
	raw, err := query("keyboard-layouts")
	if err != nil {
		return nil // a missing layout query must not kill the watch stream
	}
	var info struct {
		CurrentIndex int      `json:"current_index"`
		Names        []string `json:"names"`
	}
	if json.Unmarshal(raw, &info) != nil || len(info.Names) == 0 {
		return nil
	}
	current := ""
	if info.CurrentIndex >= 0 && info.CurrentIndex < len(info.Names) {
		current = info.Names[info.CurrentIndex]
	}
	return emit(wm.Frame{Kind: wm.FrameKeyboard, KeyboardLayout: current, KeyboardLayouts: info.Names})
}

// focusedWorkspaceOutput returns the compositor's focused output. Each monitor
// has its own active workspace, so deriving focus from the first active
// workspace would point the shell at the wrong screen on a multi-monitor box.
func focusedWorkspaceOutput(s uState) string {
	return focusedOutput(s)
}

// emit writes one frame. A failed write or flush means the consumer went away,
// so return the error and let runWatch stop reading compositor events instead
// of spinning on a pipe nobody reads.
func emit(f wm.Frame) error {
	if err := json.NewEncoder(stdout).Encode(f); err != nil {
		return err
	}
	return stdout.Flush()
}
