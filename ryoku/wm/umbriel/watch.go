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
	conn, err := subscribe([]string{"windows", "workspaces", "outputs", "overview"})
	if err != nil {
		return err
	}
	defer conn.Close()

	var state uState
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
		switch ev.Event {
		case "outputs":
			if json.Unmarshal(ev.Data, &state.Outputs) == nil {
				emit(wm.Frame{Kind: wm.FrameOutputs, FocusedOutput: focusedOutput(state), Outputs: outputsOf(state)})
			}
		case "workspaces":
			if json.Unmarshal(ev.Data, &state.Workspaces) == nil {
				emit(wm.Frame{Kind: wm.FrameWorkspaces, Workspaces: workspacesOf(state)})
				// Refresh outputs too: their active-workspace field is published as
				// a display name derived from this workspace list.
				emit(wm.Frame{Kind: wm.FrameOutputs, FocusedOutput: focusedOutput(state), Outputs: outputsOf(state)})
			}
		case "windows":
			if json.Unmarshal(ev.Data, &state.Windows) == nil {
				emit(wm.Frame{Kind: wm.FrameWindows, Windows: windowsOf(state)})
			}
		case "overview":
			var o struct {
				Open bool `json:"open"`
			}
			if json.Unmarshal(ev.Data, &o) == nil {
				emit(wm.Frame{Kind: wm.FrameOverview, OverviewOpen: o.Open})
			}
		}
	}
}

func emit(f wm.Frame) {
	_ = json.NewEncoder(stdout).Encode(f)
	_ = stdout.Flush()
}
