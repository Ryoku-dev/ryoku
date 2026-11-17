import assert from "node:assert/strict";
import test from "node:test";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const { begin, release, finish } = require("./recording.js");

// Mirror the bar's state transitions: startRecording hides the strip, the
// shortcut may reopen it, and endRecordingSuppression restores the pre-recording
// strip state. The surface is shown while it is active (open or pinned) and not
// suppressed by a recording.
function timeline({ open, pinned, reopenDuringRecording }) {
    let state = { open, pinned, cycle: false, hold: false, reopen: false, suppressed: false };

    const start = begin(state.open);
    state = {
        ...state,
        reopen: start.reopen,
        cycle: true,
        hold: true,
        suppressed: true,
        open: start.closeBar ? false : state.open
    };

    if (reopenDuringRecording && release(state.hold, state.cycle))
        state = { ...state, hold: false, suppressed: false, open: true };

    const stop = finish(state.cycle, state.reopen, state.open);
    state = {
        ...state,
        open: state.reopen,
        reopen: false,
        cycle: stop.cycle,
        hold: stop.hold,
        suppressed: true
    };

    state = { ...state, suppressed: false };
    return {
        start,
        stop,
        state,
        visible: (state.open || state.pinned) && !state.suppressed
    };
}

test("starting a recording remembers an open bar and hides it", () => {
    const run = timeline({ open: true, pinned: false, reopenDuringRecording: false });
    assert.equal(run.start.reopen, true);
    assert.equal(run.start.closeBar, true);
});

test("a closed bar is not reopened just because a recording started", () => {
    const run = timeline({ open: false, pinned: false, reopenDuringRecording: false });
    assert.equal(run.start.closeBar, false);
    assert.equal(run.stop.openBar, false);
    assert.equal(run.stop.closeBar, false);
    assert.equal(run.visible, false);
});

test("an unpinned open bar comes back after the recording", () => {
    const run = timeline({ open: true, pinned: false, reopenDuringRecording: false });
    assert.equal(run.stop.openBar, true);
    assert.equal(run.state.open, true);
    assert.equal(run.visible, true);
});

test("a bar reopened during a recording keeps its strip and restores it", () => {
    const run = timeline({ open: true, pinned: false, reopenDuringRecording: true });
    assert.equal(run.stop.openBar, false);
    assert.equal(run.stop.closeBar, false);
    assert.equal(run.state.open, true);
    assert.equal(run.visible, true);
});

test("reopening a bar that was closed before the recording closes again", () => {
    const run = timeline({ open: false, pinned: false, reopenDuringRecording: true });
    assert.equal(run.state.open, false);
    assert.equal(run.stop.closeBar, true);
    assert.equal(run.visible, false);
});

test("pinned cards keep the surface after the strip returns to closed", () => {
    const run = timeline({ open: false, pinned: true, reopenDuringRecording: true });
    assert.equal(run.state.open, false);
    assert.equal(run.stop.closeBar, true);
    assert.equal(run.visible, true);
});

test("a pinned open bar stays visible through the whole cycle", () => {
    const run = timeline({ open: true, pinned: true, reopenDuringRecording: false });
    assert.equal(run.stop.openBar, true);
    assert.equal(run.visible, true);
});

test("reopening releases the hold only while a recording cycle runs", () => {
    assert.equal(release(true, true), true);
    assert.equal(release(false, true), false);
    assert.equal(release(true, false), false);
});

test("finishing without a recording cycle does nothing", () => {
    assert.equal(finish(false, true, true), null);
});
