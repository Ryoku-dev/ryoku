package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// Hyprland's output capture copies the last composed frame, and a software
// cursor (no_hardware_cursors, or Hyprland's own choice on NVIDIA with more
// than one GPU) is painted into it. cursor:invisible is not applied when it is
// set: the renderer reads it from a 500 ms ticker (or on real key and pointer
// input), then damages the cursor and repaints. A capture taken in that window
// still shows the pointer, so the old fixed 80 ms sleep lost the race most of
// the time.
//
// So hiding waits out one full tick unless the pointer is provably off the
// frame. A hardware cursor is: a plain capture under the pointer differs from
// an include-cursor one while two plain captures agree. Anything else (a
// software cursor, moving content such as a video or live wallpaper, a failed
// capture) waits, because watching the frame for change cannot tell the cursor
// leaving from the content underneath moving.

const (
	cursorProbeSize = 32
	cursorTickWait  = 550 * time.Millisecond
)

// captureBox grabs one small region of the composed frame as raw PPM, with or
// without the cursor overlay. A package variable so tests run without a
// compositor.
var captureBox = func(x, y, w, h int, withCursor bool) ([]byte, error) {
	args := []string{"-t", "ppm", "-g", fmt.Sprintf("%d,%d %dx%d", x, y, w, h)}
	if withCursor {
		args = append([]string{"-c"}, args...)
	}
	return exec.Command("grim", append(args, "-")...).Output()
}

var cursorFrameSleep = time.Sleep

func hideCursorInFrame() error {
	offFrame := hardwareCursor()
	if err := evalLua(`hl.config({ cursor = { invisible = true } })`); err != nil {
		return err
	}
	if !offFrame {
		cursorFrameSleep(cursorTickWait)
	}
	return nil
}

func hardwareCursor() bool {
	out, err := query("cursorpos")
	if err != nil {
		return false
	}
	var pos struct {
		X int `json:"x"`
		Y int `json:"y"`
	}
	if json.Unmarshal(out, &pos) != nil {
		return false
	}
	x, y := pos.X-cursorProbeSize/4, pos.Y-cursorProbeSize/4
	shots := make([][]byte, 3)
	for i, withCursor := range []bool{false, true, false} {
		shot, err := captureBox(x, y, cursorProbeSize, cursorProbeSize, withCursor)
		if err != nil || len(shot) == 0 {
			return false
		}
		shots[i] = shot
	}
	return bytes.Equal(shots[0], shots[2]) && !bytes.Equal(shots[0], shots[1])
}
