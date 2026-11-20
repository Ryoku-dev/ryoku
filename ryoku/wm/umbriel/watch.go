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
	haveOutputs := false
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
				haveOutputs = true
				if err := emit(wm.Frame{Kind: wm.FrameOutputs, FocusedOutput: focusedOutput(state), Outputs: outputsOf(state)}); err != nil {
					return err
				}
			}
		case "workspaces":
			if json.Unmarshal(ev.Data, &state.Workspaces) == nil {
				if err := emit(wm.Frame{Kind: wm.FrameWorkspaces, Workspaces: workspacesOf(state)}); err != nil {
					return err
				}
				// Outputs carry their active workspace as a name resolved from
				// this list. Refresh them only once an outputs event has supplied
				// the output set, or a workspace-first stream would publish an
				// empty frame that tells consumers all outputs disappeared.
				if haveOutputs {
					if err := emit(wm.Frame{Kind: wm.FrameOutputs, FocusedOutput: focusedOutput(state), Outputs: outputsOf(state)}); err != nil {
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

// emit writes one frame. A failed write or flush means the consumer went away,
// so return the error and let runWatch stop reading compositor events instead
// of spinning on a pipe nobody reads.
func emit(f wm.Frame) error {
	if err := json.NewEncoder(stdout).Encode(f); err != nil {
		return err
	}
	return stdout.Flush()
}
