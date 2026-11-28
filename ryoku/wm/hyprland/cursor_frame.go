package main

import "time"

// Hyprland's output capture copies the last composed frame, and a software
// cursor (no_hardware_cursors, Hyprland's own choice on NVIDIA with more than
// one GPU, or its fallback when the hardware cursor plane fails) is painted
// into it. cursor:invisible is not applied when it is set: the renderer reads
// it from a 500 ms ticker (or on real key and pointer input), then damages the
// cursor and repaints. A capture taken in that window still shows the pointer,
// so the old fixed 80 ms sleep lost the race most of the time.
//
// So every hide waits out one full tick. Nothing Hyprland exposes says whether
// the cursor on a given output is in the frame, and comparing captures cannot
// prove it is not: content under the pointer that changes in step with the
// probe looks the same as a hardware cursor. A screenshot that is half a
// second slower beats one with the pointer in it.

const cursorTickWait = 550 * time.Millisecond

// cursorFrameSleep is a package variable so tests run without the wait.
var cursorFrameSleep = time.Sleep

func hideCursorInFrame() error {
	if err := evalLua(`hl.config({ cursor = { invisible = true } })`); err != nil {
		return err
	}
	cursorFrameSleep(cursorTickWait)
	return nil
}
