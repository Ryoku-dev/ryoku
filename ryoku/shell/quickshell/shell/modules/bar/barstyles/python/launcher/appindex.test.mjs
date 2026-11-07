/**
 * Regression tests for the launcher app-index race condition.
 *
 * These tests directly exercise the JavaScript handlers extracted from Launcher.qml
 * using node:vm, matching the pattern used by other QML unit tests in the repo:
 *   - loadApps()
 *   - Component.onCompleted
 *   - onIsVisibleChanged
 *   - DesktopEntries Connections (onValuesChanged)
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
// Direct evaluation of Launcher.qml handlers via vm
// ---------------------------------------------------------------------------

import fs from "node:fs";
import vm from "node:vm";

const qmlSource = fs.readFileSync(new URL("./Launcher.qml", import.meta.url), "utf8");

function extractBlock(source, marker) {
    const startIdx = source.indexOf(marker);
    if (startIdx === -1) throw new Error("Could not find " + marker + " in Launcher.qml");
    const braceStart = source.indexOf("{", startIdx);
    let depth = 1;
    let i = braceStart + 1;
    while (i < source.length && depth > 0) {
        if (source[i] === "{") depth++;
        else if (source[i] === "}") depth--;
        i++;
    }
    return source.slice(braceStart + 1, i - 1);
}

const loadAppsBody = extractBlock(qmlSource, "function loadApps()");
const onCompletedBody = extractBlock(qmlSource, "Component.onCompleted:");
const onIsVisibleChangedBody = extractBlock(qmlSource, "onIsVisibleChanged:");
const onValuesChangedBody = extractBlock(qmlSource, "function onValuesChanged()");

function makeState(desktopEntries = []) {
    const ctx = {
        launcherWindow: null,
        appsLoaded: false,
        allApps: [],
        isVisible: false,
        currentTabIndex: 0,
        smartRanking: false,
        usageRanks: {},
        DesktopEntries: {
            applications: {
                values: desktopEntries
            }
        },
        WindowRegistry: undefined,
        rankFetcher: { running: false },
        searchInput: { text: "", clear() { this.text = ""; } },
        tabSwitch: { currentIndex: 0 },
        restartItemsIntro() {},
        executeFilter(q) {},
        executeFileSearch(q) {},
        grabInputFocus() {},
        focusTimer: { restart() {}, stop() {} },
        focusRetryTimer: { restart() {}, stop() {} },
        focusFinalTimer: { restart() {}, stop() {} },
        keyboardNavTimer: { stop() {} },
        itemsIntroSequence: { stop() {} },
        appList: { resetScroll() {} },
        filterDebounceTimer: { stop() {} },
        fileDebounceTimer: { stop() {} },
        fileSearchProcess: { running: false },
        introItems: 0.0,
    };
    ctx.launcherWindow = ctx;
    vm.createContext(ctx);

    ctx.loadApps = vm.runInContext(`(function() { ${loadAppsBody} })`, ctx);
    const fnOnCompleted = vm.runInContext(`(function() { ${onCompletedBody} })`, ctx);
    const fnOnIsVisibleChanged = vm.runInContext(`(function(isVisible) {
        launcherWindow.isVisible = isVisible;
        ${onIsVisibleChangedBody}
    })`, ctx);
    const fnOnValuesChanged = vm.runInContext(`(function() { ${onValuesChangedBody} })`, ctx);

    return {
        ctx,
        loadApps: () => ctx.loadApps.call(ctx),
        onCompleted: () => fnOnCompleted.call(ctx),
        setVisible: (v) => fnOnIsVisibleChanged.call(ctx, v),
        onDesktopEntriesChanged: (entries) => {
            ctx.DesktopEntries.applications.values = entries;
            fnOnValuesChanged.call(ctx);
        }
    };
}

// ---------------------------------------------------------------------------
// Happy-path tests
// ---------------------------------------------------------------------------

test("normal boot: entries ready before onCompleted -> appsLoaded=true", () => {
    const apps = [{ name: "Firefox", id: "firefox.desktop", noDisplay: false }];
    const inst = makeState(apps);
    inst.onCompleted();

    assert.equal(inst.ctx.appsLoaded, true);
    assert.equal(inst.ctx.allApps.length, 1);
});

test("normal open: appsLoaded=true, opens fine", () => {
    const apps = [{ name: "Firefox", id: "firefox.desktop", noDisplay: false }];
    const inst = makeState(apps);
    inst.onCompleted();
    inst.setVisible(true);

    assert.equal(inst.ctx.appsLoaded, true);
    assert.equal(inst.ctx.allApps.length, 1);
});

// ---------------------------------------------------------------------------
// Race condition regression tests
// ---------------------------------------------------------------------------

test("[REGRESSION] race: DesktopEntries empty at onCompleted must NOT set appsLoaded=true", () => {
    // DesktopEntries not yet scanned at component creation
    const inst = makeState([]); // empty
    inst.onCompleted();

    // appsLoaded must stay false - the original bug set it to true here
    assert.equal(inst.ctx.appsLoaded, false, "appsLoaded must be false when entries are empty");
    assert.equal(inst.ctx.allApps.length, 0);
});

test("[REGRESSION] race: opening launcher while DesktopEntries still empty retries on next open", () => {
    const inst = makeState([]); // DesktopEntries not ready
    inst.onCompleted();                     // appsLoaded=false (empty)

    // User opens launcher before entries are available
    inst.setVisible(true);
    assert.equal(inst.ctx.allApps.length, 0, "still empty: entries not loaded yet");
    assert.equal(inst.ctx.appsLoaded, false, "appsLoaded must remain false when opening empty");

    // DesktopEntries finishes scanning while launcher is closed
    inst.setVisible(false);       // hide resets appsLoaded=false
    inst.onDesktopEntriesChanged([       // entries now available
        { name: "Firefox", id: "firefox.desktop", noDisplay: false },
        { name: "Terminal", id: "terminal.desktop", noDisplay: false },
    ]);

    // User opens launcher again
    inst.setVisible(true);
    assert.equal(inst.ctx.appsLoaded, true, "must be loaded on second open");
    assert.equal(inst.ctx.allApps.length, 2, "apps must be populated on second open");
});

test("[REGRESSION] race: opening while empty retries immediately on same open if entries arrive mid-session", () => {
    const inst = makeState([]);
    inst.onCompleted();
    inst.setVisible(true); // opens empty

    assert.equal(inst.ctx.allApps.length, 0);

    // DesktopEntries emits signal while launcher is OPEN
    inst.onDesktopEntriesChanged([
        { name: "Firefox", id: "firefox.desktop", noDisplay: false },
    ]);

    assert.equal(inst.ctx.appsLoaded, true, "reactive refresh while open must set appsLoaded=true");
    assert.equal(inst.ctx.allApps.length, 1, "reactive refresh while open must populate allApps");
});

test("[REGRESSION] noDisplay entries are excluded and must not mark as loaded", () => {
    // All entries are noDisplay=true: effectively empty from the launcher's POV
    const inst = makeState([
        { name: "Hidden App", id: "hidden1.desktop", noDisplay: true },
        { name: "Another Hidden", id: "hidden2.desktop", noDisplay: true },
    ]);
    inst.onCompleted();

    assert.equal(inst.ctx.appsLoaded, false, "noDisplay-only result must not mark as loaded");
    assert.equal(inst.ctx.allApps.length, 0);
});

test("[REGRESSION] onIsVisibleChanged retries if allApps empty even when appsLoaded=true", () => {
    // Simulate the state left by the original bug: appsLoaded=true but allApps=[]
    // This can happen if external code (not loadApps) sets appsLoaded=true.
    const inst = makeState([{ name: "Firefox", id: "firefox.desktop", noDisplay: false }]);
    inst.ctx.appsLoaded = true;
    inst.ctx.allApps = []; // corrupted state from old bug

    inst.setVisible(true);

    assert.equal(inst.ctx.appsLoaded, true, "after retry with real entries, must be loaded");
    assert.equal(inst.ctx.allApps.length, 1, "must have recovered apps from corrupted state");
});

// ---------------------------------------------------------------------------
// Edge-case tests
// ---------------------------------------------------------------------------

test("hide always resets appsLoaded so next show gets fresh data", () => {
    const apps = [{ name: "Firefox", id: "firefox.desktop", noDisplay: false }];
    const inst = makeState(apps);
    inst.onCompleted();
    inst.setVisible(true);
    assert.equal(inst.ctx.appsLoaded, true);

    inst.setVisible(false);
    assert.equal(inst.ctx.appsLoaded, false, "hide must reset appsLoaded");
});

test("rapid open-close-open cycle always shows apps if entries are available", () => {
    const apps = [
        { name: "Firefox", id: "firefox.desktop", noDisplay: false },
        { name: "Chromium", id: "chromium.desktop", noDisplay: false }
    ];
    const inst = makeState(apps);
    inst.onCompleted();

    for (let i = 0; i < 10; i++) {
        inst.setVisible(true);
        assert.equal(inst.ctx.allApps.length, 2, `open #${i + 1} must show apps`);
        inst.setVisible(false);
    }
});

test("DesktopEntries changed while visible reloads without requiring hide/show", () => {
    const inst = makeState([{ name: "Firefox", id: "firefox.desktop", noDisplay: false }]);
    inst.onCompleted();
    inst.setVisible(true);
    assert.equal(inst.ctx.allApps.length, 1);

    // A new app is installed while launcher is open
    inst.onDesktopEntriesChanged([
        { name: "Firefox", id: "firefox.desktop", noDisplay: false },
        { name: "NewApp", id: "newapp.desktop", noDisplay: false },
    ]);

    assert.equal(inst.ctx.allApps.length, 2, "must include newly installed app without hide/show");
});
