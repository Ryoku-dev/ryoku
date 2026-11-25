package fs

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/paths"
)

// Persistent output lives under the session directory on the host or the
// task's writable scratch HOME in a sandbox. Both paths are served by the
// same executor as the file tools; temporary sessions hold output in memory.

const (
	spillScheme   = "local://"
	spillMax      = 4 << 20
	spillInMemory = 16
)

type spillState struct {
	next   int
	memory map[int]string
	order  []int
}

// Shape fits text for the model: text over env.Limits.Told keeps its head
// and tail (tool.HeadTail) and the whole is spilled to local://N.log, named
// in a closing note so the model can page it with read. Other tools (bash)
// use it for their own long output.
func Shape(env tool.Env, text string) string {
	told := env.Limits.Told
	if told <= 0 {
		told = tool.LimitsFor(env.Profile.Window, env.Profile.Class).Told
	}
	if len(text) <= told {
		return text
	}
	head, tail := env.Limits.Head, env.Limits.Tail
	if head <= 0 || tail <= 0 {
		head, tail = told*3/8, told*3/8
	}
	shaped := tool.HeadTail(text, head, tail)
	ref, kept, err := spill(env, text)
	if err != nil {
		return shaped + "\n[The full output could not be kept, so the part left out is gone.]"
	}
	note := fmt.Sprintf("\n[Full output: %d lines, %d bytes. Page it with read %s:1-%d and so on.]",
		countLines(text), len(text), ref, max(env.Limits.ReadLines, 100))
	if kept < len(text) {
		note += fmt.Sprintf("\n[Output after %d bytes was not retained.]", kept)
	}
	return shaped + note
}

// spill keeps output through the executor so team sessions never cross the
// sandbox boundary to reach daemon-owned session files.
func spill(env tool.Env, text string) (string, int, error) {
	full := len(text)
	if len(text) > spillMax {
		text, _ = tool.ClipBytes(text, spillMax)
	}
	st := state(env.Session.ID)
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.spill == nil {
		st.spill = &spillState{memory: map[int]string{}}
		if env.Session.ID != "" && !env.Session.Temporary && env.Exec != nil {
			st.spill.next = lastSpill(env)
		}
	}
	sp := st.spill
	sp.next++
	n := sp.next
	if env.Session.ID != "" && !env.Session.Temporary {
		if env.Exec == nil {
			return "", 0, fmt.Errorf("the executor is unavailable")
		}
		path := filepath.Join(spillDir(env), strconv.Itoa(n)+".log")
		_, err := env.Exec.Write(context.Background(), exec.WriteRequest{Path: path, Content: []byte(text), MakeDirs: true, IfAbsent: true, Mode: 0o600})
		if err != nil {
			return "", 0, err
		}
	}
	sp.memory[n] = text
	sp.order = append(sp.order, n)
	if len(sp.order) > spillInMemory {
		delete(sp.memory, sp.order[0])
		sp.order = sp.order[1:]
	}
	return fmt.Sprintf("%s%d.log", spillScheme, n), min(full, spillMax), nil
}

// spilled returns a kept output by its local:// name.
func spilled(env tool.Env, ref string) (string, error) {
	name := strings.TrimPrefix(ref, spillScheme)
	if !strings.HasSuffix(name, ".log") {
		return "", fmt.Errorf("%s is not a kept output; they are named local://N.log", ref)
	}
	n, err := strconv.Atoi(strings.TrimSuffix(name, ".log"))
	if err != nil || n <= 0 {
		return "", fmt.Errorf("%s is not a kept output; they are named local://N.log", ref)
	}
	st := state(env.Session.ID)
	st.mu.Lock()
	if st.spill != nil {
		if text, ok := st.spill.memory[n]; ok {
			st.mu.Unlock()
			return text, nil
		}
	}
	st.mu.Unlock()
	if env.Session.ID != "" && !env.Session.Temporary && env.Exec != nil {
		path := filepath.Join(spillDir(env), strconv.Itoa(n)+".log")
		result, err := env.Exec.Read(context.Background(), exec.ReadRequest{Path: path, Limit: spillMax})
		if err == nil {
			return string(result.Data), nil
		}
	}
	return "", fmt.Errorf("%s is gone; run the command or search again", ref)
}

func spillDir(env tool.Env) string {
	if env.Sandbox != nil {
		if home := env.Sandbox.Env["HOME"]; home != "" {
			return filepath.Join(home, "spill")
		}
	}
	return filepath.Join(paths.SessionDir(env.Session.ID), "spill")
}

// lastSpill keeps old local:// references stable across daemon restarts.
func lastSpill(env tool.Env) int {
	result, err := env.Exec.List(context.Background(), exec.ListRequest{Path: spillDir(env), Hidden: true, Limit: 100000})
	if err != nil {
		return 0
	}
	last := 0
	for _, e := range result.Entries {
		if strings.HasSuffix(e.Name, ".log") {
			if n, err := strconv.Atoi(strings.TrimSuffix(e.Name, ".log")); err == nil && n > last {
				last = n
			}
		}
	}
	return last
}
