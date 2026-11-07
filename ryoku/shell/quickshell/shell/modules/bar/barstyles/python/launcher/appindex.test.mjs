/**
 * Regression tests for the launcher app-index race condition.
 *
 * These tests model the exact flag logic extracted from Launcher.qml:
 *   - loadApps()          (lines 636-745)
 *   - Component.onCompleted (lines 142-153)
 *   - onIsVisibleChanged  (lines 517-565)
 *   - DesktopEntries Connections (lines 181-216)
 *
 * Bug: launcher appeared empty when opened before DesktopEntries finished
 * scanning /usr/share/applications. The fix makes loadApps() the sole
 * authority over `appsLoaded`, and onIsVisibleChanged retries when empty.
 *
 * Run with: node --test appindex.test.mjs
 */

import assert from "node:assert/strict";
import test from "node:test";

// ---------------------------------------------------------------------------
// Pure-JS model of the QML state machine
// ---------------------------------------------------------------------------

function makeState(desktopEntries = []) {
    return {
        appsLoaded: false,
        allApps: [],
        isVisible: false,
        // Simulates DesktopEntries.applications.values
        _desktopEntries: desktopEntries,
    };
}

/** Mirrors loadApps() in Launcher.qml (lines 636-745). */
function loadApps(state) {
    const finalArr = state._desktopEntries.filter(e => !e.noDisplay);

    state = { ...state, allApps: finalArr };

    // loadApps() is the sole authority over appsLoaded.
    if (finalArr.length > 0) {
        state = { ...state, appsLoaded: true };
    } else {
        state = { ...state, appsLoaded: false };
    }

    return state;
}

/** Mirrors Component.onCompleted (lines 142-153). */
function onCompleted(state) {
    state = loadApps(state);
    // Note: appsLoaded is managed inside loadApps() — do NOT set it here
    return state;
}

/**
 * Mirrors onIsVisibleChanged (lines 517-565).
 * Returns the new state after show/hide.
 */
function onIsVisibleChanged(state, nowVisible) {
    if (nowVisible) {
        state = { ...state, isVisible: true };

        // Attempt to load if not loaded yet, or if previous load came back empty
        if (!state.appsLoaded || state.allApps.length === 0) {
            state = loadApps(state);
        }
    } else {
        // Always reset on hide so the next show retries if needed
        state = { ...state, isVisible: false, appsLoaded: false };
    }
    return state;
}

/**
 * Mirrors the DesktopEntries Connections (onValuesChanged / onCountChanged,
 * lines 195-216).
 */
function onDesktopEntriesChanged(state, newEntries) {
    state = { ...state, _desktopEntries: newEntries };

    if (state.isVisible) {
        state = loadApps(state);
    } else {
        // Reset so the next show picks up fresh data
        state = { ...state, appsLoaded: false };
    }
    return state;
}

// ---------------------------------------------------------------------------
// Happy-path tests
// ---------------------------------------------------------------------------

test("normal boot: entries ready before onCompleted → appsLoaded=true", () => {
    const apps = [{ name: "Firefox", noDisplay: false }];
    let state = makeState(apps);
    state = onCompleted(state);

    assert.equal(state.appsLoaded, true);
    assert.equal(state.allApps.length, 1);
});

test("normal open: appsLoaded=true, opens fine", () => {
    const apps = [{ name: "Firefox", noDisplay: false }];
    let state = makeState(apps);
    state = onCompleted(state);
    state = onIsVisibleChanged(state, true);

    assert.equal(state.appsLoaded, true);
    assert.equal(state.allApps.length, 1);
});

// ---------------------------------------------------------------------------
// Race condition regression tests
// ---------------------------------------------------------------------------

test("[REGRESSION] race: DesktopEntries empty at onCompleted must NOT set appsLoaded=true", () => {
    // DesktopEntries not yet scanned at component creation
    let state = makeState([]); // empty
    state = onCompleted(state);

    // appsLoaded must stay false — the original bug set it to true here
    assert.equal(state.appsLoaded, false, "appsLoaded must be false when entries are empty");
    assert.equal(state.allApps.length, 0);
});

test("[REGRESSION] race: opening launcher while DesktopEntries still empty retries on next open", () => {
    let state = makeState([]); // DesktopEntries not ready
    state = onCompleted(state);                     // appsLoaded=false (empty)

    // User opens launcher before entries are available
    state = onIsVisibleChanged(state, true);
    assert.equal(state.allApps.length, 0, "still empty: entries not loaded yet");

    // DesktopEntries finishes scanning while launcher is closed
    state = onIsVisibleChanged(state, false);       // hide resets appsLoaded=false
    state = onDesktopEntriesChanged(state, [       // entries now available
        { name: "Firefox", noDisplay: false },
        { name: "Terminal", noDisplay: false },
    ]);

    // User opens launcher again
    state = onIsVisibleChanged(state, true);
    assert.equal(state.appsLoaded, true, "must be loaded on second open");
    assert.equal(state.allApps.length, 2, "apps must be populated on second open");
});

test("[REGRESSION] race: opening while empty retries immediately on same open if entries arrive mid-session", () => {
    let state = makeState([]);
    state = onCompleted(state);
    state = onIsVisibleChanged(state, true); // opens empty

    assert.equal(state.allApps.length, 0);

    // DesktopEntries emits signal while launcher is OPEN
    state = onDesktopEntriesChanged(state, [
        { name: "Firefox", noDisplay: false },
    ]);

    assert.equal(state.appsLoaded, true, "reactive refresh while open must set appsLoaded=true");
    assert.equal(state.allApps.length, 1, "reactive refresh while open must populate allApps");
});

test("[REGRESSION] noDisplay entries are excluded and must not mark as loaded", () => {
    // All entries are noDisplay=true — effectively empty from the launcher's POV
    let state = makeState([
        { name: "Hidden App", noDisplay: true },
        { name: "Another Hidden", noDisplay: true },
    ]);
    state = onCompleted(state);

    assert.equal(state.appsLoaded, false, "noDisplay-only result must not mark as loaded");
    assert.equal(state.allApps.length, 0);
});

test("[REGRESSION] onIsVisibleChanged retries if allApps empty even when appsLoaded=true", () => {
    // Simulate the state left by the original bug: appsLoaded=true but allApps=[]
    // This can happen if external code (not loadApps) sets appsLoaded=true.
    let state = makeState([{ name: "Firefox", noDisplay: false }]);
    state = { ...state, appsLoaded: true, allApps: [] }; // corrupted state from old bug

    state = onIsVisibleChanged(state, true);

    assert.equal(state.appsLoaded, true, "after retry with real entries, must be loaded");
    assert.equal(state.allApps.length, 1, "must have recovered apps from corrupted state");
});

// ---------------------------------------------------------------------------
// Edge-case tests
// ---------------------------------------------------------------------------

test("hide always resets appsLoaded so next show gets fresh data", () => {
    const apps = [{ name: "Firefox", noDisplay: false }];
    let state = makeState(apps);
    state = onCompleted(state);
    state = onIsVisibleChanged(state, true);
    assert.equal(state.appsLoaded, true);

    state = onIsVisibleChanged(state, false);
    assert.equal(state.appsLoaded, false, "hide must reset appsLoaded");
});

test("rapid open-close-open cycle always shows apps if entries are available", () => {
    const apps = [{ name: "Firefox", noDisplay: false }, { name: "Chromium", noDisplay: false }];
    let state = makeState(apps);
    state = onCompleted(state);

    for (let i = 0; i < 10; i++) {
        state = onIsVisibleChanged(state, true);
        assert.equal(state.allApps.length, 2, `open #${i + 1} must show apps`);
        state = onIsVisibleChanged(state, false);
    }
});

test("DesktopEntries changed while visible reloads without requiring hide/show", () => {
    let state = makeState([{ name: "Firefox", noDisplay: false }]);
    state = onCompleted(state);
    state = onIsVisibleChanged(state, true);
    assert.equal(state.allApps.length, 1);

    // A new app is installed while launcher is open
    state = onDesktopEntriesChanged(state, [
        { name: "Firefox", noDisplay: false },
        { name: "NewApp", noDisplay: false },
    ]);

    assert.equal(state.allApps.length, 2, "must include newly installed app without hide/show");
});
