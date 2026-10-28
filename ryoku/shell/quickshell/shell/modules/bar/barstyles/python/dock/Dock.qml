import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import QtQuick.Shapes
import Quickshell
import Quickshell.Wayland
import Ryoku.Ui.Singletons
import Quickshell.Io
import "../"
import "../reusables"
import "../../../../../components"
import shell.services as Ryoku
import stage
import shell.modules.stage.frame

Variants {
    id: universalDock
    property var targetScreen: null
    property bool surfaceVisible: true
    // The Stage Editor frames the host's monitor: the dock holds revealed and
    // raised there, exactly like the other designs (UniversalDockHost).
    property bool stageEditing: false
    model: targetScreen ? [targetScreen] : Quickshell.screens
    delegate: Component {
        id: dockDelegate
        Scope {
            id: dockScope
            required property var modelData

            PanelWindow {
                id: dockExclusionWindow
                screen: dockScope.modelData

                WlrLayershell.namespace: "qs-dock-exclusion"
                WlrLayershell.layer: dockWindow.dockOnTop ? WlrLayer.Top : WlrLayer.Bottom
                color: "transparent"
                visible: universalDock.surfaceVisible && dockWindow.isEffectivelyExclusive

                exclusionMode: ExclusionMode.Normal
                exclusiveZone: dockWindow.isEffectivelyExclusive ? dockWindow.dockReservedSpace : 0

                anchors {
                    top: dockWindow.dockPosition === "top" || dockWindow.isVertical
                    bottom: dockWindow.dockPosition === "bottom" || dockWindow.isVertical
                    left: dockWindow.dockPosition === "left" || !dockWindow.isVertical
                    right: dockWindow.dockPosition === "right" || !dockWindow.isVertical
                }

                implicitHeight: dockWindow.isVertical ? 0 : dockWindow.dockReservedSpace
                implicitWidth: dockWindow.isVertical ? dockWindow.dockReservedSpace : 0

                mask: Region {}
            }

            // Autohide does not need the full-output visual surface to stay
            // mapped. This edge-sized layer keeps the original reveal target
            // alive after the animated dock window has been released.
            PanelWindow {
                id: dockRevealWindow
                screen: dockScope.modelData
                visible: universalDock.surfaceVisible && dockWindow.initialized
                    && dockWindow.dockEnabled && dockWindow.effectiveAutohide
                    && dockAppsModel.count > 0
                    && !dockWindow.isFullscreenActive && !dockWindow.editMode
                    && !universalDock.stageEditing
                color: "transparent"
                exclusionMode: ExclusionMode.Ignore
                WlrLayershell.namespace: "qs-dock-reveal"
                WlrLayershell.layer: dockWindow.dockOnTop ? WlrLayer.Top : WlrLayer.Bottom
                WlrLayershell.keyboardFocus: WlrKeyboardFocus.None

                anchors {
                    top: dockWindow.dockPosition === "top"
                    bottom: dockWindow.dockPosition === "bottom"
                    left: dockWindow.dockPosition === "left"
                    right: dockWindow.dockPosition === "right"
                }
                margins {
                    top: dockWindow.dockPosition === "top" ? dockWindow.barOffset : 0
                    bottom: dockWindow.dockPosition === "bottom" ? dockWindow.barOffset : 0
                    left: dockWindow.dockPosition === "left" ? dockWindow.barOffset : 0
                    right: dockWindow.dockPosition === "right" ? dockWindow.barOffset : 0
                }

                implicitWidth: dockWindow.isVertical ? dockWindow.autohideHitSize
                    : Math.min(dockWindow.screenWidth, dockWindow.dockTotalLength)
                implicitHeight: dockWindow.isVertical
                    ? Math.min(dockWindow.screenHeight, dockWindow.dockTotalLength)
                    : dockWindow.autohideHitSize

                HoverHandler {
                    id: edgeHover
                    acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
                    onHoveredChanged: dockWindow.checkHideTimer()
                }
            }

            PanelWindow {
                id: dockWindow
                property var modelData: dockScope.modelData
                screen: dockScope.modelData

                WlrLayershell.namespace: "qs-dock"
                // Shima's full-output chassis occupies Top and owns its frame.
                // Its transparent edge mask otherwise receives clicks before
                // this dock when the chassis loads later.
                WlrLayershell.layer: universalDock.stageEditing || Ryoku.Config.barStyle === "iris"
                    ? WlrLayer.Overlay
                    : (dockWindow.dockOnTop || dockWindow.editMode) ? WlrLayer.Top : WlrLayer.Bottom
                focusable: dockWindow.editMode
                color: "transparent"
                exclusionMode: ExclusionMode.Ignore

                property int configRevision: 0
                property bool initialized: false

                readonly property real screenWidth: (dockWindow.screen && dockWindow.screen.width > 0) ? dockWindow.screen.width : (dockScope.modelData && dockScope.modelData.width > 0 ? dockScope.modelData.width : 0)
                readonly property real screenHeight: (dockWindow.screen && dockWindow.screen.height > 0) ? dockWindow.screen.height : (dockScope.modelData && dockScope.modelData.height > 0 ? dockScope.modelData.height : 0)
                readonly property real effectiveWindowWidth: dockWindow.width > 0 ? dockWindow.width : screenWidth
                readonly property real effectiveWindowHeight: dockWindow.height > 0 ? dockWindow.height : screenHeight

                Timer {
                    id: initTimer
                    interval: 50
                    repeat: false
                    onTriggered: dockWindow.initialized = true
                }

                Connections {
                    target: (typeof Config !== "undefined") ? Config : null
                    function onSettingsLoaded() { dockWindow.configRevision++; }
                    function onDataReadyChanged() { dockWindow.configRevision++; }
                    function onRawSettingsChanged() { dockWindow.configRevision++; }
                }

                property var defaultDockSettings: ({
                    "enabled": true,
                    "position": "bottom",
                    "onTop": true,
                    "elementSize": 44,
                    "floating": false,
                    "background": true,
                    "exclusive": false,
                    "autohide": false,
                    "smartAutohide": true,
                    "autohideTimeout": 1000,
                    "editing": false,
                    "apps": [],
                    "overrideBoundsCorrection": false,
                    "enableScrolling": false,
                    "visibleElements": 7,
                    "scrollingElements": 7,
                    "maxVisibleElements": 7,
                    "maxElements": 7,
                    "hoverScale": 120,
                    "cascadeScale": false,
                    "focusRunning": true,
                    "showRunningIndicators": true,
                    "showRunningApps": true,
                    "runningScope": "auto"
                })

                property var rawDockSettings: {
                    const design = Ryoku.Dock.cfg("python", {})
                    const apps = Ryoku.Dock.pinnedOrStarter().map(className => {
                        const entry = Ryoku.Dock.entryFor(className)
                        return {
                            name: entry?.name ?? className,
                            comment: entry?.comment ?? "",
                            desktop_id: className,
                            icon: entry?.icon ?? ""
                        }
                    })
                    return Object.assign({}, defaultDockSettings, design, {
                        enabled: Ryoku.Dock.cfg("enabled", false),
                        position: Ryoku.Dock.resolvedEdge(),
                        elementSize: Ryoku.Dock.cfg("size", 44),
                        autohide: Ryoku.Dock.cfg("autohide", false),
                        magnify: Ryoku.Dock.cfg("magnify", true),
                        apps: apps
                    })
                }


                property bool dockEnabled: rawDockSettings.enabled !== undefined ? rawDockSettings.enabled : true
                property string dockPosition: rawDockSettings.position !== undefined ? rawDockSettings.position : "bottom"
                property bool dockOnTop: {
                    let val = undefined;
                    if (rawDockSettings && rawDockSettings.onTop !== undefined) {
                        val = rawDockSettings.onTop;
                    } else if (typeof Config !== "undefined" && Config.rawSettings && Config.rawSettings["dock.onTop"] !== undefined) {
                        val = Config.rawSettings["dock.onTop"];
                    }
                    if (val === undefined || val === null) return true;
                    if (typeof val === "boolean") return val;
                    if (typeof val === "string") return val.toLowerCase() === "true" || val === "1";
                    return Boolean(val);
                }
                property int rawElementSize: rawDockSettings.elementSize !== undefined ? rawDockSettings.elementSize : 44
                property bool overrideBoundsCorrection: rawDockSettings.overrideBoundsCorrection !== undefined ? Boolean(rawDockSettings.overrideBoundsCorrection) : false

                property bool dockExclusive: {
                    let val = undefined;
                    if (rawDockSettings) {
                        val = rawDockSettings.exclusive !== undefined ? rawDockSettings.exclusive : rawDockSettings.exclusiveMode;
                    }
                    if (val === undefined && typeof Config !== "undefined" && Config.rawSettings) {
                        val = Config.rawSettings["dock.exclusive"] !== undefined ? Config.rawSettings["dock.exclusive"] : Config.rawSettings["dock.exclusiveMode"];
                    }
                    if (val === undefined || val === null) return false;
                    if (typeof val === "boolean") return val;
                    if (typeof val === "string") return val.toLowerCase() === "true" || val === "1";
                    return Boolean(val);
                }

                property bool smartAutohide: {
                    let val = undefined;
                    if (rawDockSettings && rawDockSettings.smartAutohide !== undefined) {
                        val = rawDockSettings.smartAutohide;
                    } else if (typeof Config !== "undefined" && Config.rawSettings && Config.rawSettings["dock.smartAutohide"] !== undefined) {
                        val = Config.rawSettings["dock.smartAutohide"];
                    }
                    if (val === undefined || val === null) return true;
                    if (typeof val === "boolean") return val;
                    if (typeof val === "string") return val.toLowerCase() === "true" || val === "1";
                    return Boolean(val);
                }

                // Workspace and output state comes from Ryoku's neutral WM seam.
                readonly property string myOutput: (dockWindow.screen && dockWindow.screen.name) ? dockWindow.screen.name : ""
                readonly property string activeWorkspaceKey: {
                    const workspace = Wm.focusedWorkspace;
                    if (!workspace) return "";
                    return String(workspace.name !== undefined && workspace.name !== "" ? workspace.name : workspace.id);
                }
                property int activeIndex: {
                    const fw = Wm.focusedWorkspace;
                    if (!fw) return -1;
                    let n = parseInt(fw.name !== undefined && fw.name !== "" ? fw.name : fw.id);
                    return (!isNaN(n) && n > 0) ? n - 1 : -1;
                }

                readonly property bool isWorkspaceBusy: {
                    const fw = Wm.focusedWorkspace;
                    return fw ? (fw.occupied === true) : false;
                }

                property bool autohide: rawDockSettings.autohide !== undefined ? rawDockSettings.autohide : false
                readonly property bool effectiveAutohide: autohide && (!smartAutohide || isWorkspaceBusy)

                onEffectiveAutohideChanged: hideTimer.stop()
                onActiveIndexChanged: {
                    hideTimer.stop();
                    if (typeof DockAppMenuController !== "undefined" && DockAppMenuController.isVisible) {
                        DockAppMenuController.hide();
                    }
                    dockWindow._recomputeRunningInfo();
                }

                onActiveWorkspaceKeyChanged: {
                    hideTimer.stop();
                    if (typeof DockAppMenuController !== "undefined" && DockAppMenuController.isVisible) {
                        DockAppMenuController.hide();
                    }
                    dockWindow._recomputeRunningInfo();
                }

                readonly property bool isEffectivelyExclusive: dockWindow.initialized && dockEnabled && dockExclusive && !effectiveAutohide && !isFullscreenActive && !editMode && (dockAppsModel.count > 0)
                readonly property int dockReservedSpace: Math.round(dockContainer.fullThickness + effectiveMargin)

                property real dockHoverScaleMultiplier: {
                    if (!rawDockSettings.magnify)
                        return 1;
                    const parsed = parseFloat(rawDockSettings.hoverScale);
                    if (isNaN(parsed) || parsed < 100)
                        return 1.20;
                    return parsed / 100.0;
                }

                property bool dockCascadeScale: {
                    let val = undefined;
                    if (rawDockSettings && rawDockSettings.cascadeScale !== undefined) {
                        val = rawDockSettings.cascadeScale;
                    } else if (typeof Config !== "undefined" && Config.rawSettings && Config.rawSettings["dock.cascadeScale"] !== undefined) {
                        val = Config.rawSettings["dock.cascadeScale"];
                    }
                    if (val === undefined || val === null) return false;
                    if (typeof val === "boolean") return val;
                    if (typeof val === "string") return val.toLowerCase() === "true" || val === "1";
                    return Boolean(val);
                }

                property bool focusRunning: {
                    let val = undefined;
                    if (rawDockSettings && rawDockSettings.focusRunning !== undefined) val = rawDockSettings.focusRunning;
                    else if (typeof Config !== "undefined" && Config.rawSettings && Config.rawSettings["dock.focusRunning"] !== undefined) val = Config.rawSettings["dock.focusRunning"];
                    if (val === undefined || val === null) return true;
                    if (typeof val === "boolean") return val;
                    if (typeof val === "string") return val.toLowerCase() === "true" || val === "1";
                    return Boolean(val);
                }

                property bool showRunningIndicators: {
                    let val = undefined;
                    if (rawDockSettings && rawDockSettings.showRunningIndicators !== undefined) val = rawDockSettings.showRunningIndicators;
                    else if (typeof Config !== "undefined" && Config.rawSettings && Config.rawSettings["dock.showRunningIndicators"] !== undefined) val = Config.rawSettings["dock.showRunningIndicators"];
                    if (val === undefined || val === null) return true;
                    if (typeof val === "boolean") return val;
                    if (typeof val === "string") return val.toLowerCase() === "true" || val === "1";
                    return Boolean(val);
                }

                property bool showRunningApps: {
                    let val = undefined;
                    if (rawDockSettings && rawDockSettings.showRunningApps !== undefined) val = rawDockSettings.showRunningApps;
                    else if (typeof Config !== "undefined" && Config.rawSettings && Config.rawSettings["dock.showRunningApps"] !== undefined) val = Config.rawSettings["dock.showRunningApps"];
                    if (val === undefined || val === null) return true;
                    if (typeof val === "boolean") return val;
                    if (typeof val === "string") return val.toLowerCase() === "true" || val === "1";
                    return Boolean(val);
                }

                property string runningScopeSetting: {
                    let val = undefined;
                    if (rawDockSettings && rawDockSettings.runningScope !== undefined) val = rawDockSettings.runningScope;
                    else if (typeof Config !== "undefined" && Config.rawSettings && Config.rawSettings["dock.runningScope"] !== undefined) val = Config.rawSettings["dock.runningScope"];
                    return (typeof val === "string" && val !== "") ? val : "auto";
                }

                readonly property string effectiveRunningScope: {
                    if (runningScopeSetting === "auto") {
                        return "all";
                    }
                    return runningScopeSetting;
                }

                property var runningInfoMap: ({})
                property int runningInfoRevision: 0
                property string runningWindowSignature: ""

                readonly property var wmWindows: Wm.windows || []
                readonly property var wmFocusedWindow: Wm.focusedWindow

                onWmWindowsChanged: dockWindow._syncRunningInfo(false)
                onWmFocusedWindowChanged: dockWindow._syncRunningInfo(false)

                function _windowGroups() {
                    let groups = {};
                    const windows = dockWindow.wmWindows;
                    const focusedId = dockWindow.wmFocusedWindow ? String(dockWindow.wmFocusedWindow.id) : "";
                    for (let i = 0; i < windows.length; i++) {
                        const source = windows[i];
                        if (!source) continue;
                        const rawApp = String(source.appId || "");
                        const desktopId = Ryoku.Dock.canonicalId(rawApp);
                        if (desktopId === "") continue;
                        let group = groups[desktopId];
                        if (!group) {
                            group = {
                                desktopId: desktopId,
                                appId: rawApp,
                                count: 0,
                                windows: []
                            };
                            groups[desktopId] = group;
                        }
                        const workspace = source.workspace === undefined || source.workspace === null ? "" : String(source.workspace);
                        group.windows.push({
                            id: source.id,
                            title: source.title || "",
                            appId: rawApp,
                            desktopId: desktopId,
                            output: source.output ? String(source.output) : "",
                            workspace: workspace,
                            workspaceLabel: workspace,
                            focused: String(source.id) === focusedId
                        });
                        group.count++;
                    }
                    return groups;
                }

                function _syncRunningInfo(force) {
                    const focusedId = dockWindow.wmFocusedWindow
                        ? String(dockWindow.wmFocusedWindow.id ?? "") : "";
                    const relevant = [];
                    for (const window of dockWindow.wmWindows) {
                        if (!window)
                            continue;
                        relevant.push(String(window.id ?? "") + "\u0000"
                            + String(window.appId ?? "") + "\u0000"
                            + String(window.workspace ?? "") + "\u0000"
                            + String(window.output ?? ""));
                    }
                    relevant.sort();
                    const signature = focusedId + "\u0001" + relevant.join("\u0001");
                    if (force !== true && signature === dockWindow.runningWindowSignature)
                        return;
                    dockWindow.runningWindowSignature = signature;
                    dockWindow._recomputeRunningInfo();
                }

                function _recomputeRunningInfo() {
                    let groups = dockWindow._windowGroups();
                    let newMap = {};
                    let currentOutput = dockWindow.myOutput;
                    let currentWs = dockWindow.activeWorkspaceKey;
                    let scope = dockWindow.effectiveRunningScope;

                    for (let gKey in groups) {
                        let grp = groups[gKey];
                        if (!grp || !grp.windows) continue;

                        let here = [];
                        let elsewhere = [];

                        for (let i = 0; i < grp.windows.length; i++) {
                            let w = grp.windows[i];
                            let match = false;

                            if (scope === "all") {
                                match = true;
                            } else if (scope === "output") {
                                match = (w.output === "" || w.output === currentOutput);
                            } else if (scope === "workspace") {
                                let outMatch = (w.output === "" || w.output === currentOutput);
                                let wsMatch = (w.workspace === "" || w.workspace === currentWs);
                                match = outMatch && wsMatch;
                            }

                            if (match) here.push(w);
                            else elsewhere.push(w);
                        }

                        let isFoc = here.some(function(w) { return w.focused; });
                        let isFocElse = elsewhere.some(function(w) { return w.focused; });
                        let activeWin = here.length > 0 ? here[0] : (elsewhere.length > 0 ? elsewhere[0] : null);

                        let grpInfo = {
                            groupKey: gKey,
                            desktopId: grp.desktopId,
                            appId: grp.appId,
                            count: here.length,
                            totalCount: grp.count,
                            hasWindows: (here.length > 0 || elsewhere.length > 0),
                            here: here,
                            elsewhere: elsewhere,
                            isFocused: isFoc,
                            isFocusedElsewhere: isFocElse,
                            activeWindow: activeWin
                        };

                        newMap[gKey] = grpInfo;
                        if (grp.desktopId) newMap[grp.desktopId] = grpInfo;
                        if (grp.appId) {
                            newMap[grp.appId] = grpInfo;
                            newMap["app:" + String(grp.appId).toLowerCase()] = grpInfo;
                        }
                    }

                    dockWindow.runningInfoMap = newMap;
                    dockWindow.runningInfoRevision++;
                }

                function getRunningInfoForApp(desktopId) {
                    if (!desktopId || !runningInfoMap) return null;
                    let raw = String(desktopId).trim();
                    let lower = raw.toLowerCase();
                    let cid = Ryoku.Dock.canonicalId(raw);
                    return runningInfoMap[raw] || runningInfoMap[lower]
                        || runningInfoMap[cid] || runningInfoMap["app:" + lower] || null;
                }

                function menuWindows(info) {
                    if (!info)
                        return [];
                    const stored = [].concat(info.here ?? [], info.elsewhere ?? []);
                    const currentById = {};
                    for (const window of (Wm.windows ?? []))
                        currentById[String(window?.id ?? "")] = window;
                    return stored.map(window => {
                        const current = currentById[String(window.id ?? "")];
                        if (!current)
                            return window;
                        return Object.assign({}, window, {
                            title: current.title || "",
                            appId: current.appId || window.appId
                        });
                    });
                }

                Connections {
                    target: (typeof DockAppMenuController !== "undefined") ? DockAppMenuController : null
                    function onIsVisibleChanged() {
                        dockWindow.checkHideTimer();
                    }
                    function onRequestPin(dId, name, icon) {
                        const cid = Ryoku.Dock.canonicalId(dId);
                        for (let r = runningAppsModel.count - 1; r >= 0; r--) {
                            const item = runningAppsModel.get(r);
                            if (item && Ryoku.Dock.sameApp(item.desktop_id, cid)) runningAppsModel.remove(r);
                        }
                        if (!dockWindow.isAppInDock(cid)) {
                            const entry = Ryoku.Dock.entryFor(cid);
                            dockAppsModel.append({
                                name: name || (entry ? entry.name : cid),
                                comment: entry ? (entry.comment || "") : "",
                                desktop_id: cid,
                                icon: icon || (entry ? entry.icon : cid)
                            });
                            dockWindow.saveApps();
                        }
                        dockWindow._syncRunningApps();
                    }
                    function onRequestUnpin(dId) {
                        dockWindow.removeAppByDesktopId(dId);
                    }
                }

                onEffectiveRunningScopeChanged: dockWindow._recomputeRunningInfo()

                readonly property real hoverExpansion: Math.round(Math.max(s(24), s(dockElementSize) * (dockHoverScaleMultiplier - 1.0) + s(12)))

                property bool enableScrolling: {
                    let val = undefined;
                    if (rawDockSettings) {
                        val = rawDockSettings.enableScrolling !== undefined ? rawDockSettings.enableScrolling :
                              (rawDockSettings.scrollingEnabled !== undefined ? rawDockSettings.scrollingEnabled :
                              (rawDockSettings.scrollEnabled !== undefined ? rawDockSettings.scrollEnabled :
                              (rawDockSettings.scrolling !== undefined ? rawDockSettings.scrolling : undefined)));
                    }
                    if (val === undefined && typeof Config !== "undefined" && Config.rawSettings) {
                        val = Config.rawSettings["dock.enableScrolling"] !== undefined ? Config.rawSettings["dock.enableScrolling"] :
                              (Config.rawSettings["dock.scrollingEnabled"] !== undefined ? Config.rawSettings["dock.scrollingEnabled"] :
                              (Config.rawSettings["enableScrolling"] !== undefined ? Config.rawSettings["enableScrolling"] : undefined));
                    }
                    if (val === undefined || val === null) return false;
                    if (typeof val === "boolean") return val;
                    if (typeof val === "string") return val.toLowerCase() === "true" || val === "1";
                    if (typeof val === "number") return val !== 0;
                    return Boolean(val);
                }

                property int dockVisibleElements: {
                    let val = undefined;
                    if (rawDockSettings) {
                        val = rawDockSettings.visibleElements !== undefined ? rawDockSettings.visibleElements :
                              (rawDockSettings.scrollingElements !== undefined ? rawDockSettings.scrollingElements :
                              (rawDockSettings.maxVisibleElements !== undefined ? rawDockSettings.maxVisibleElements :
                              (rawDockSettings.maxElements !== undefined ? rawDockSettings.maxElements :
                              (rawDockSettings.visibleItems !== undefined ? rawDockSettings.visibleItems :
                              (rawDockSettings.scrollElements !== undefined ? rawDockSettings.scrollElements :
                              (rawDockSettings.scrollingLimit !== undefined ? rawDockSettings.scrollingLimit :
                              (rawDockSettings.scrollingItemCount !== undefined ? rawDockSettings.scrollingItemCount :
                              (rawDockSettings.itemCount !== undefined ? rawDockSettings.itemCount : undefined))))))));
                    }
                    if (val === undefined && typeof Config !== "undefined" && Config.rawSettings) {
                        val = Config.rawSettings["dock.visibleElements"] !== undefined ? Config.rawSettings["dock.visibleElements"] :
                              (Config.rawSettings["dock.scrollingElements"] !== undefined ? Config.rawSettings["dock.scrollingElements"] :
                              (Config.rawSettings["visibleElements"] !== undefined ? Config.rawSettings["visibleElements"] : undefined));
                    }
                    let parsed = parseInt(val);
                    return (!isNaN(parsed) && parsed > 0) ? parsed : 7;
                }

                property int dockElementSize: {
                    if (overrideBoundsCorrection) return rawElementSize;
                    if (enableScrolling) return rawElementSize;
                    let countToCheck = editMode ? (dockContainer.effectiveItemCount + 1) : dockContainer.effectiveItemCount;
                    if (countToCheck <= 0) return rawElementSize;
                    let avail = (isVertical ? effectiveWindowHeight : effectiveWindowWidth) - s(40) - (sameSideAsBar ? barHeight : 0);
                    let spacing = editMode ? s(10) : s(8);
                    let pad = s(20);
                    let needed = countToCheck * s(rawElementSize) + Math.max(0, countToCheck - 1) * spacing + pad;
                    if (needed > avail) {
                        let maxElemPx = (avail - pad - Math.max(0, countToCheck - 1) * spacing) / countToCheck;
                        let scaleRatio = s(100) / 100.0;
                        if (scaleRatio <= 0) scaleRatio = 1.0;
                        let adjusted = Math.floor(maxElemPx / scaleRatio);
                        return Math.max(20, Math.min(rawElementSize, adjusted));
                    }
                    return rawElementSize;
                }

                property bool rawDockFloating: rawDockSettings.floating !== undefined ? rawDockSettings.floating : false
                property bool dockFloating: rawDockFloating || (sameSideAsBar && !isBarSolid)
                readonly property bool dockBackground: rawDockSettings.background !== undefined ? Boolean(rawDockSettings.background) : true
                readonly property real dockOpacity: dockBackground ? ThemeBackend.elementOpacity : 0.0
                property real floatingMargin: dockFloating ? s(8) : 0
                Behavior on floatingMargin {
                    enabled: dockWindow.visualContentActive && dockWindow.initialized && !dockWindow.positionChanging
                    NumberAnimation { duration: 280; easing.type: Easing.OutCubic }
                }

                property int autohideTimeout: rawDockSettings.autohideTimeout !== undefined ? rawDockSettings.autohideTimeout : 1000
                property real autohideHitSize: s(18)
                property bool editMode: false

                property real editMargin: (editMode && !dockFloating) ? s(20) : 0
                Behavior on editMargin {
                    enabled: dockWindow.visualContentActive && dockWindow.initialized && !dockWindow.positionChanging
                    NumberAnimation { duration: 320; easing.type: Easing.OutBack; easing.overshoot: 1.15 }
                }
                readonly property real effectiveMargin: floatingMargin + editMargin

                property int dragSourceIndex: -1
                property int dropTargetIndex: -1

                property bool positionChanging: false

                Timer {
                    id: positionChangeTimer
                    interval: 300
                    onTriggered: dockWindow.positionChanging = false
                }

                onDockPositionChanged: {
                    dockWindow.positionChanging = true;
                    dockContainer.scrollIndex = 0;
                    dockContainer.hoveredItemIndex = -1;
                    positionChangeTimer.restart();
                    dockWindow.publishInset();
                }

                readonly property bool surfaceEligible: universalDock.surfaceVisible
                    && dockWindow.initialized && dockEnabled
                    && (!isFullscreenActive || universalDock.stageEditing)
                    && (dockAppsModel.count > 0 || editMode || universalDock.stageEditing)
                property bool hideAnimationActive: false
                readonly property bool visualContentActive: dockWindow.visible
                    && (dockWindow.isRevealed || dockWindow.hideAnimationActive)
                visible: surfaceEligible
                    && (!effectiveAutohide || isRevealed || hideAnimationActive
                        || dockContainer.revealProgress > 0.001)

                property var rawBarSettings: {
                    let trackedRevision = configRevision;
                    return (typeof Config !== "undefined" && Config.rawSettings && Config.rawSettings.bar) ? Config.rawSettings.bar : ({});
                }

                property string barPosition: (rawBarSettings && rawBarSettings.position !== undefined) ? rawBarSettings.position : "top"
                property bool barAutohide: (rawBarSettings && rawBarSettings.autohide !== undefined) ? Boolean(rawBarSettings.autohide) : false
                property real barHeight: (rawBarSettings && rawBarSettings.height !== undefined) ? s(rawBarSettings.height) : s(40)

                property string barStyle: {
                    let trackedRevision = configRevision;
                    if (typeof Config === "undefined" || !Config.rawSettings || !Config.rawSettings.bar) return "modular";
                    let s = Config.rawSettings.bar.style;
                    if (typeof s === "string") return s;
                    if (s && typeof s === "object") {
                        if (s.fill || s.mode === "fill") return "fill";
                        if (s.solid || s.mode === "solid") return "solid";
                    }
                    return "modular";
                }
                readonly property bool isBarSolid: barStyle === "solid" || barStyle === "fill"

                readonly property bool isOsdFullscreen: (typeof OsdController !== "undefined") ? Boolean(OsdController.isFullscreen) : false
                readonly property bool isToplevelFullscreen: Wm.outputHasFullscreen(
                    dockWindow.screen ? dockWindow.screen.name : "")
                readonly property bool isFullscreenActive: isOsdFullscreen || isToplevelFullscreen

                readonly property bool isBarEffectivelyHidden: barAutohide || isFullscreenActive
                readonly property bool sameSideAsBar: (dockPosition === barPosition) && !isBarEffectivelyHidden
                readonly property real barMarginOffset: (!isBarSolid && !barAutohide) ? s(4) : 0
                readonly property real barOffset: sameSideAsBar ? (barHeight + barMarginOffset) : 0

                readonly property bool isVertical: dockPosition === "left" || dockPosition === "right"

                function s(val) {
                    return (typeof Scaler !== "undefined" && Scaler.s) ? Scaler.s(val) : val;
                }

                property real cornerRadius: ThemeBackend.borderRadius <= 16 ? ThemeBackend.borderRadius * 2 : Math.min(32, 32 - 16 * Math.exp(-(ThemeBackend.borderRadius - 16) / 12))
                property real outerCornerRadius: cornerRadius

                property real dockTotalLength: {
                    let len = (isVertical ? dockContainer.baseHeight : dockContainer.baseWidth) + (dockContainer.isAttached ? outerCornerRadius * 2 : 0) + s(12);
                    return len * 1.2;
                }

                anchors {
                    top: true
                    bottom: true
                    left: true
                    right: true
                }

                Timer {
                    id: hideTimer
                    interval: dockWindow.autohideTimeout
                }

                function checkHideTimer() {
                    let isMenuOpen = (typeof DockAppMenuController !== "undefined" && DockAppMenuController.isVisible && (!DockAppMenuController.screen || !dockWindow.screen || DockAppMenuController.screen.name === dockWindow.screen.name));
                    if (!dockHover.hovered && !edgeHover.hovered && dockWindow.effectiveAutohide
                            && !dockWindow.editMode && !universalDock.stageEditing && !isMenuOpen) {
                        hideTimer.restart();
                    } else {
                        hideTimer.stop();
                    }
                }

                // Keep the reported edit edge at its resting thickness; reveal
                // animation must not move edit guides under the pointer.
                function publishInset() {
                    GlobalStates.setDockInset(dockWindow.modelData ? dockWindow.modelData.name : "",
                        universalDock.stageEditing && dockWindow.visible ? dockWindow.dockPosition : "",
                        dockWindow.dockReservedSpace);
                }
                onVisibleChanged: dockWindow.publishInset()
                onDockReservedSpaceChanged: dockWindow.publishInset()
                Connections {
                    target: universalDock
                    function onStageEditingChanged() { dockWindow.publishInset(); dockWindow.checkHideTimer(); }
                }
                Component.onDestruction: {
                    GlobalStates.setDockInset(
                        dockWindow.modelData ? dockWindow.modelData.name : "", "", 0);
                    if (typeof DockAppMenuController !== "undefined"
                            && DockAppMenuController.isVisible
                            && (!DockAppMenuController.screen || DockAppMenuController.screen === dockWindow.screen)) {
                        DockAppMenuController.hide();
                    }
                }

                Connections {
                    target: dockHover
                    function onHoveredChanged() { dockWindow.checkHideTimer(); }
                }

                property bool isRevealed: {
                    if (isFullscreenActive && !universalDock.stageEditing) return false;
                    if (editMode || universalDock.stageEditing) return true;
                    if (!effectiveAutohide) return true;
                    if (typeof DockAppMenuController !== "undefined" && DockAppMenuController.isVisible && (!DockAppMenuController.screen || !dockWindow.screen || DockAppMenuController.screen.name === dockWindow.screen.name)) return true;
                    if (dockHover.hovered) return true;
                    if (edgeHover.hovered) return true;
                    if (hideTimer.running) return true;
                    return false;
                }

                Timer {
                    id: surfaceReleaseTimer
                    interval: 300
                    onTriggered: dockWindow.hideAnimationActive = false
                }
                onIsRevealedChanged: {
                    if (dockWindow.isRevealed) {
                        surfaceReleaseTimer.stop();
                        dockWindow.hideAnimationActive = false;
                    } else if (dockWindow.visible) {
                        dockWindow.hideAnimationActive = true;
                        surfaceReleaseTimer.restart();
                    }
                }

                ListModel {
                    id: dockAppsModel
                }

                ListModel {
                    id: runningAppsModel
                }

                function _syncRunningApps() {
                    if (!dockWindow.showRunningApps || dockWindow.editMode) {
                        if (runningAppsModel.count > 0) runningAppsModel.clear();
                        return;
                    }

                    const groups = dockWindow._windowGroups();
                    let pinnedSet = {};
                    for (let p = 0; p < dockAppsModel.count; p++) {
                        const item = dockAppsModel.get(p);
                        if (!item || !item.desktop_id) continue;
                        const cid = Ryoku.Dock.canonicalId(item.desktop_id);
                        if (cid) pinnedSet[cid] = true;
                    }

                    let candidates = [];
                    for (let key in groups) {
                        const group = groups[key];
                        if (!group || !group.windows || group.windows.length === 0) continue;
                        const cid = Ryoku.Dock.canonicalId(group.desktopId || group.appId);
                        if (!cid || pinnedSet[cid]) continue;
                        const entry = Ryoku.Dock.entryFor(cid);
                        candidates.push({
                            name: entry ? (entry.name || cid) : (group.appId || cid),
                            comment: entry ? (entry.comment || "") : "",
                            desktop_id: cid,
                            icon: entry ? (entry.icon || cid) : cid,
                            canonical_id: cid,
                            is_pinned: false
                        });
                    }

                    for (let r = runningAppsModel.count - 1; r >= 0; r--) {
                        const item = runningAppsModel.get(r);
                        let exists = false;
                        for (let c = 0; c < candidates.length; c++) {
                            if (item && item.canonical_id === candidates[c].canonical_id) {
                                exists = true;
                                break;
                            }
                        }
                        if (!exists) runningAppsModel.remove(r);
                    }

                    for (let c = 0; c < candidates.length; c++) {
                        const candidate = candidates[c];
                        let index = -1;
                        for (let r = c; r < runningAppsModel.count; r++) {
                            const item = runningAppsModel.get(r);
                            if (item && item.canonical_id === candidate.canonical_id) {
                                index = r;
                                break;
                            }
                        }
                        if (index === -1) runningAppsModel.insert(c, candidate);
                        else if (index !== c) runningAppsModel.move(index, c, 1);
                    }
                }

                onRunningInfoRevisionChanged: dockWindow._syncRunningApps()
                onShowRunningAppsChanged: dockWindow._syncRunningApps()

                function loadApps() {
                    let customApps = rawDockSettings.apps;
                    if (!customApps || !Array.isArray(customApps)) customApps = [];
                    if (dockAppsModel.count === customApps.length) {
                        let matches = true;
                        for (let i = 0; i < customApps.length; i++) {
                            let cur = dockAppsModel.get(i);
                            let app = customApps[i];
                            let id = app.desktop_id || app.id || "";
                            if (!cur || cur.desktop_id !== id || cur.name !== (app.name || "") || cur.icon !== (app.icon || "")) {
                                matches = false;
                                break;
                            }
                        }
                        if (matches) {
                            dockWindow._syncRunningApps();
                            return;
                        }
                    }
                    dockAppsModel.clear();
                    for (let k = 0; k < customApps.length; k++) {
                        let app = customApps[k];
                        dockAppsModel.append({
                            name: app.name || "",
                            comment: app.comment || "",
                            desktop_id: app.desktop_id || app.id || "",
                            icon: app.icon || ""
                        });
                    }
                    dockWindow._syncRunningApps();
                }

                function saveApps() {
                    let pins = [];
                    for (let i = 0; i < dockAppsModel.count; i++) {
                        const item = dockAppsModel.get(i);
                        if (item?.desktop_id)
                            pins.push(item.desktop_id);
                    }
                    Ryoku.Dock.setPinned(pins);
                }

                function isAppInDock(desktopId) {
                    if (!desktopId) return false;
                    for (let i = 0; i < dockAppsModel.count; i++) {
                        const item = dockAppsModel.get(i);
                        if (item && item.desktop_id && Ryoku.Dock.sameApp(item.desktop_id, desktopId)) return true;
                    }
                    return false;
                }

                function addApp(entry) {
                    if (!entry) return;
                    const id = Ryoku.Dock.canonicalId(entry.id || entry.desktop_id || "");
                    if (!id) return;
                    for (let r = runningAppsModel.count - 1; r >= 0; r--) {
                        const item = runningAppsModel.get(r);
                        if (item && Ryoku.Dock.sameApp(item.desktop_id, id)) runningAppsModel.remove(r);
                    }
                    if (dockWindow.isAppInDock(id)) {
                        dockWindow._syncRunningApps();
                        return;
                    }
                    dockAppsModel.append({
                        name: entry.name || "",
                        comment: entry.comment || "",
                        desktop_id: id,
                        icon: entry.icon || ""
                    });
                    saveApps();
                    dockWindow._syncRunningApps();
                }

                function removeAppByDesktopId(desktopId) {
                    for (let i = 0; i < dockAppsModel.count; i++) {
                        const item = dockAppsModel.get(i);
                        if (item && Ryoku.Dock.sameApp(item.desktop_id, desktopId)) {
                            dockAppsModel.remove(i, 1);
                            saveApps();
                            dockWindow._syncRunningApps();
                            return;
                        }
                    }
                }

                function setEditMode(val) {
                    dockWindow.editMode = val;
                    if (typeof Sounds !== "undefined")
                        Sounds.playSfx(val ? "guide/barconfig/out.wav" : "guide/barconfig/in.wav");
                }

                function grabPickerFocus() {
                    if (pickerSearchInput) {
                        pickerSearchInput.forceActiveFocus();
                        if (typeof pickerSearchInput.forceInputFocus === "function") {
                            pickerSearchInput.forceInputFocus();
                        }
                    }
                }

                Timer {
                    id: pickerFocusTimer
                    interval: 50
                    repeat: false
                    onTriggered: dockWindow.grabPickerFocus()
                }

                Timer {
                    id: pickerFocusRetryTimer
                    interval: 150
                    repeat: false
                    onTriggered: dockWindow.grabPickerFocus()
                }

                Timer {
                    id: pickerFocusFinalTimer
                    interval: 300
                    repeat: false
                    onTriggered: dockWindow.grabPickerFocus()
                }

                property var allDesktopApps: []
                ListModel { id: pickerFilteredModel }

                function loadAllDesktopApps() {
                    let list = [];
                    if (typeof DesktopEntries !== "undefined" && DesktopEntries.applications && DesktopEntries.applications.values) {
                        let entries = DesktopEntries.applications.values;
                        for (let i = 0; i < entries.length; i++) {
                            let e = entries[i];
                            if (e.noDisplay) continue;
                            list.push({
                                id: e.id || "",
                                name: e.name || "",
                                comment: e.comment || "",
                                icon: e.icon || ""
                            });
                        }
                    }
                    list.sort(function(a, b) {
                        return (a.name || "").localeCompare(b.name || "");
                    });
                    dockWindow.allDesktopApps = list;
                    let currentQuery = (pickerSearchInput && pickerSearchInput.text !== undefined) ? pickerSearchInput.text : "";
                    filterPickerApps(currentQuery);
                }

                function filterPickerApps(query) {
                    pickerFilteredModel.clear();
                    let q = (query || "").trim().toLowerCase();
                    for (let i = 0; i < allDesktopApps.length; i++) {
                        let app = allDesktopApps[i];
                        if (!q || app.name.toLowerCase().includes(q) || app.comment.toLowerCase().includes(q) || app.id.toLowerCase().includes(q)) {
                            pickerFilteredModel.append({
                                id: app.id || "",
                                name: app.name || "",
                                comment: app.comment || "",
                                icon: app.icon || "",
                                modelData: app
                            });
                        }
                    }
                }

                onEditModeChanged: {
                    dockWindow._syncRunningApps();
                    dockContainer.hoveredItemIndex = -1;
                    if (editMode) {
                        loadAllDesktopApps();
                        pickerFocusTimer.restart();
                        pickerFocusRetryTimer.restart();
                        pickerFocusFinalTimer.restart();
                    } else {
                        if (pickerSearchInput && typeof pickerSearchInput.clear === "function") {
                            pickerSearchInput.clear();
                        } else if (pickerSearchInput && pickerSearchInput.text !== undefined) {
                            pickerSearchInput.text = "";
                        }
                        pickerFocusTimer.stop();
                        pickerFocusRetryTimer.stop();
                        pickerFocusFinalTimer.stop();
                    }
                }

                function launchApp(desktopId) {
                    const entry = Ryoku.Dock.entryFor(desktopId);
                    if (entry) Ryoku.AppLaunch.run(entry, null);
                    else Ryoku.Dock.activate(desktopId);
                }

                function activateOrLaunch(desktopId, modifiers) {
                    if (!desktopId) return;

                    let forceNew = modifiers && (modifiers & Qt.ShiftModifier || modifiers & Qt.ControlModifier);
                    if (forceNew || !dockWindow.focusRunning) {
                        dockWindow.launchApp(desktopId);
                        return;
                    }

                    let info = dockWindow.getRunningInfoForApp(desktopId);
                    if (!info || !info.hasWindows) {
                        dockWindow.launchApp(desktopId);
                        return;
                    }

                    let targetWin = null;
                    if (info.here && info.here.length > 0) {
                        if (info.here.length === 1) {
                            targetWin = info.here[0];
                        } else {
                            let curIdx = -1;
                            for (let i = 0; i < info.here.length; i++) {
                                if (info.here[i].focused) {
                                    curIdx = i;
                                    break;
                                }
                            }
                            let targetIdx = 0;
                            if (curIdx >= 0) {
                                targetIdx = (curIdx + 1) % info.here.length;
                            }
                            targetWin = info.here[targetIdx];
                        }
                    } else if (info.elsewhere && info.elsewhere.length > 0) {
                        targetWin = info.elsewhere[0];
                    }

                    if (targetWin && targetWin.id !== undefined) {
                        Wm.focusWindow(targetWin.id);
                    } else {
                        dockWindow.launchApp(desktopId);
                    }
                }

                Component.onCompleted: {
                    dockWindow._syncRunningInfo(true);
                    loadApps();
                    loadAllDesktopApps();
                    initTimer.start();
                }

                onConfigRevisionChanged: loadApps()
                onRawDockSettingsChanged: loadApps()

                Connections {
                    target: (typeof DesktopEntries !== "undefined" && DesktopEntries.applications) ? DesktopEntries.applications : null
                    function onValuesChanged() {
                        dockWindow.loadAllDesktopApps();
                    }
                }

                readonly property rect dockInputRect: frameLayer.maskRect(dockMaskArea.x, dockMaskArea.y,
                    dockMaskArea.width, dockMaskArea.height)

                mask: Region {
                    Region {
                        item: dockWindow.editMode ? dismissArea : null
                    }
                    Region {
                        x: Math.round(dockWindow.dockInputRect.x)
                        y: Math.round(dockWindow.dockInputRect.y)
                        width: (dockWindow.isRevealed || dockContainer.revealProgress > 0.01)
                            ? Math.round(dockWindow.dockInputRect.width) : 0
                        height: (dockWindow.isRevealed || dockContainer.revealProgress > 0.01)
                            ? Math.round(dockWindow.dockInputRect.height) : 0
                    }
                    Region {
                        item: dockWindow.editMode ? appPicker : null
                    }
                }

                Item {
                    id: dismissArea
                    width: dockWindow.effectiveWindowWidth
                    height: dockWindow.effectiveWindowHeight
                    visible: dockWindow.editMode
                    z: -1

                    MouseArea {
                        anchors.fill: parent
                        onClicked: dockWindow.setEditMode(false)
                    }

                    Keys.onEscapePressed: function(event) {
                        dockWindow.setEditMode(false);
                        event.accepted = true;
                    }
                }

                // Only the dock follows the desktop into Stage's card; its
                // picker and edit dismissal layer remain full-size.
                StageFramedLayer {
                    id: frameLayer
                    screenName: dockWindow.modelData?.name ?? ""
                    edge: dockWindow.dockPosition
                    restThickness: dockWindow.isVertical ? dockWindow.screenWidth : dockWindow.screenHeight
                    screenWidth: dockWindow.screenWidth > 0 ? dockWindow.screenWidth : 1920
                    screenHeight: dockWindow.screenHeight > 0 ? dockWindow.screenHeight : 1080


                Item {
                    id: dockMaskArea
                    readonly property real rawX: dockContainer.x - (dockContainer.isAttached && !dockWindow.isVertical ? dockWindow.outerCornerRadius : 0) + dockTransform.x - dockWindow.hoverExpansion
                    readonly property real rawY: dockContainer.y - (dockContainer.isAttached && dockWindow.isVertical ? dockWindow.outerCornerRadius : 0) + dockTransform.y - dockWindow.hoverExpansion
                    readonly property real rawW: dockContainer.width + (dockContainer.isAttached && !dockWindow.isVertical ? dockWindow.outerCornerRadius * 2 : 0) + dockWindow.hoverExpansion * 2
                    readonly property real rawH: dockContainer.height + (dockContainer.isAttached && dockWindow.isVertical ? dockWindow.outerCornerRadius * 2 : 0) + dockWindow.hoverExpansion * 2
                    readonly property real minX: (dockWindow.sameSideAsBar && dockWindow.dockPosition === "left") ? dockWindow.barOffset : 0
                    readonly property real maxX: dockWindow.effectiveWindowWidth - ((dockWindow.sameSideAsBar && dockWindow.dockPosition === "right") ? dockWindow.barOffset : 0)
                    readonly property real minY: (dockWindow.sameSideAsBar && dockWindow.dockPosition === "top") ? dockWindow.barOffset : 0
                    readonly property real maxY: dockWindow.effectiveWindowHeight - ((dockWindow.sameSideAsBar && dockWindow.dockPosition === "bottom") ? dockWindow.barOffset : 0)
                    x: Math.max(rawX, minX)
                    y: Math.max(rawY, minY)
                    width: Math.max(0, Math.min(rawX + rawW, maxX) - x)
                    height: Math.max(0, Math.min(rawY + rawH, maxY) - y)
                }

                Item {
                    id: dragOverlay
                    anchors.fill: parent
                    z: 999999
                }

                Item {
                    id: dockContainer
                    visible: dockWindow.visualContentActive

                    readonly property bool isAttached: !dockWindow.dockFloating && !dockWindow.editMode && !(dockWindow.sameSideAsBar && !dockWindow.isBarSolid)
                    property real outerCornerProgress: isAttached ? 1.0 : 0.0

                    Behavior on outerCornerProgress {
                        enabled: dockWindow.visualContentActive && dockWindow.initialized && !dockWindow.positionChanging
                        NumberAnimation {
                            duration: dockContainer.isAttached ? 240 : 320
                            easing.type: dockContainer.isAttached ? Easing.OutQuad : Easing.InQuad
                        }
                    }

                    property real dynamicCornerRadius: (dockWindow.dockFloating || dockWindow.editMode)
                        ? ThemeBackend.borderRadius
                        : Math.max(0, Math.min(dockWindow.outerCornerRadius, (dockWindow.isVertical ? width : height) * 0.5))

                    readonly property real dockSideRadius: dynamicCornerRadius * (1.0 - outerCornerProgress)
                    readonly property real topLeftCorner: (dockWindow.dockPosition === "top" || dockWindow.dockPosition === "left") ? dockSideRadius : dynamicCornerRadius
                    readonly property real topRightCorner: (dockWindow.dockPosition === "top" || dockWindow.dockPosition === "right") ? dockSideRadius : dynamicCornerRadius
                    readonly property real bottomLeftCorner: (dockWindow.dockPosition === "bottom" || dockWindow.dockPosition === "left") ? dockSideRadius : dynamicCornerRadius
                    readonly property real bottomRightCorner: (dockWindow.dockPosition === "bottom" || dockWindow.dockPosition === "right") ? dockSideRadius : dynamicCornerRadius
                    readonly property bool fanFillets: dockWindow.outerCornerRadius > 0.5 && (!dockWindow.sameSideAsBar || (dockWindow.isVertical ? width : height) >= dockWindow.outerCornerRadius)

                    Behavior on dynamicCornerRadius {
                        enabled: dockWindow.visualContentActive && dockWindow.initialized && !dockWindow.positionChanging
                        NumberAnimation { duration: 300; easing.type: Easing.OutCubic }
                    }

                    readonly property int placeholderCount: 6
                    property int totalItemCount: {
                        if (dockWindow.editMode) {
                            return dockAppsModel.count > 0 ? dockAppsModel.count : placeholderCount;
                        }
                        let pinned = dockAppsModel.count;
                        let running = dockWindow.showRunningApps ? runningAppsModel.count : 0;
                        let hasSep = (pinned > 0 && running > 0);
                        return pinned + (hasSep ? 1 : 0) + running;
                    }
                    property int effectiveItemCount: {
                        if (dockWindow.enableScrolling && totalItemCount > 0) {
                            return Math.min(totalItemCount, Math.max(1, dockWindow.dockVisibleElements));
                        }
                        return totalItemCount;
                    }

                    property int scrollIndex: 0
                    readonly property int maxScrollIndex: Math.max(0, totalItemCount - effectiveItemCount)

                    property int hoveredItemIndex: -1

                    Timer {
                        id: hoverResetTimer
                        interval: 80
                        repeat: false
                        onTriggered: dockContainer.hoveredItemIndex = -1
                    }

                    function checkHoverReset() {
                        hoverResetTimer.restart();
                    }

                    function cancelHoverReset() {
                        hoverResetTimer.stop();
                    }

                    onTotalItemCountChanged: {
                        if (scrollIndex > maxScrollIndex) scrollIndex = maxScrollIndex;
                    }
                    onEffectiveItemCountChanged: {
                        if (scrollIndex > maxScrollIndex) scrollIndex = maxScrollIndex;
                    }

                    function scrollNext() {
                        if (scrollIndex < maxScrollIndex) scrollIndex++;
                    }

                    function scrollPrev() {
                        if (scrollIndex > 0) scrollIndex--;
                    }

                    WheelHandler {
                        id: dockWheelHandler
                        target: null
                        orientation: Qt.Vertical | Qt.Horizontal
                        acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
                        onWheel: function(event) {
                            if (!dockWindow.enableScrolling) return;
                            let dy = (event.angleDelta && event.angleDelta.y !== 0) ? event.angleDelta.y : (event.pixelDelta ? event.pixelDelta.y : 0);
                            let dx = (event.angleDelta && event.angleDelta.x !== 0) ? event.angleDelta.x : (event.pixelDelta ? event.pixelDelta.x : 0);
                            let delta = Math.abs(dy) >= Math.abs(dx) ? dy : dx;
                            if (delta < 0) {
                                dockContainer.scrollNext();
                            } else if (delta > 0) {
                                dockContainer.scrollPrev();
                            }
                            event.accepted = true;
                        }
                    }

                    property real itemSpacing: dockWindow.editMode ? dockWindow.s(10) : dockWindow.s(8)
                    property real itemSize: dockWindow.s(dockWindow.dockElementSize)
                    property real itemStep: itemSize + itemSpacing

                    property real dockContentLength: {
                        if (dockWindow.editMode) {
                            let cnt = dockAppsModel.count > 0 ? dockAppsModel.count : placeholderCount;
                            return cnt * itemSize + Math.max(0, cnt - 1) * itemSpacing;
                        }
                        let pinned = dockAppsModel.count;
                        let running = (dockWindow.showRunningApps && !dockWindow.editMode) ? runningAppsModel.count : 0;
                        let hasSep = (pinned > 0 && running > 0);
                        let totalCount = pinned + running + (hasSep ? 1 : 0);
                        if (dockWindow.enableScrolling && totalCount > 0) {
                            let vis = Math.min(totalCount, Math.max(1, dockWindow.dockVisibleElements));
                            return vis * itemSize + Math.max(0, vis - 1) * itemSpacing;
                        }
                        let sepThickness = dockWindow.s(1);
                        let appCount = pinned + running;
                        if (appCount === 0) return 0;
                        let appsLen = appCount * itemSize;
                        let sepLen = hasSep ? sepThickness : 0;
                        let spacingsLen = Math.max(0, totalCount - 1) * itemSpacing;
                        return appsLen + sepLen + spacingsLen;
                    }
                    property real dockThickness: itemSize

                    property real fullThickness: dockThickness + dockWindow.s(16)
                    property real fullContentLength: dockContentLength + dockWindow.s(20)

                    property real baseWidth: Math.round(dockWindow.isVertical ? fullThickness : fullContentLength)
                    property real baseHeight: Math.round(dockWindow.isVertical ? fullContentLength : fullThickness)

                    Behavior on baseWidth {
                        enabled: dockWindow.visualContentActive && dockWindow.initialized && !dockWindow.positionChanging
                        NumberAnimation { duration: 250; easing.type: Easing.OutCubic }
                    }
                    Behavior on baseHeight {
                        enabled: dockWindow.visualContentActive && dockWindow.initialized && !dockWindow.positionChanging
                        NumberAnimation { duration: 250; easing.type: Easing.OutCubic }
                    }

                    property real revealProgress: dockWindow.isRevealed ? 1.0 : 0.0
                    Behavior on revealProgress {
                        enabled: dockWindow.initialized && !dockWindow.positionChanging
                        NumberAnimation {
                            duration: dockWindow.isRevealed ? 240 : 180
                            easing.type: dockWindow.isRevealed ? Easing.OutCubic : Easing.InCubic
                        }
                    }

                    width: (dockWindow.sameSideAsBar && dockWindow.isVertical)
                        ? Math.round(baseWidth * revealProgress)
                        : baseWidth

                    height: (dockWindow.sameSideAsBar && !dockWindow.isVertical)
                        ? Math.round(baseHeight * revealProgress)
                        : baseHeight

                    x: {
                        if (!dockWindow.isVertical) return Math.round((dockWindow.effectiveWindowWidth - width) / 2);
                        if (dockWindow.dockPosition === "left") {
                            return Math.round(dockWindow.barOffset + dockWindow.effectiveMargin);
                        }
                        if (dockWindow.dockPosition === "right") {
                            return Math.round(dockWindow.effectiveWindowWidth - width - dockWindow.barOffset - dockWindow.effectiveMargin);
                        }
                        return Math.round((dockWindow.effectiveWindowWidth - width) / 2);
                    }

                    y: {
                        if (dockWindow.isVertical) return Math.round((dockWindow.effectiveWindowHeight - height) / 2);
                        if (dockWindow.dockPosition === "top") {
                            return Math.round(dockWindow.barOffset + dockWindow.effectiveMargin);
                        }
                        if (dockWindow.dockPosition === "bottom") {
                            return Math.round(dockWindow.effectiveWindowHeight - height - dockWindow.barOffset - dockWindow.effectiveMargin);
                        }
                        return Math.round((dockWindow.effectiveWindowHeight - height) / 2);
                    }

                    property real hideOffset: {
                        if (dockWindow.isVertical) {
                            let dist = width + dockWindow.barOffset + dockWindow.effectiveMargin + dockWindow.s(20);
                            return dockWindow.dockPosition === "left" ? -dist : dist;
                        } else {
                            let dist = height + dockWindow.barOffset + dockWindow.effectiveMargin + dockWindow.s(20);
                            return dockWindow.dockPosition === "top" ? -dist : dist;
                        }
                    }

                    transform: Translate {
                        id: dockTransform
                        x: (!dockWindow.sameSideAsBar && dockWindow.isVertical) ? (dockWindow.isRevealed ? 0 : dockContainer.hideOffset) : 0
                        y: (!dockWindow.sameSideAsBar && !dockWindow.isVertical) ? (dockWindow.isRevealed ? 0 : dockContainer.hideOffset) : 0

                        Behavior on x {
                            enabled: dockWindow.visualContentActive && dockWindow.initialized && !dockWindow.positionChanging && !dockWindow.sameSideAsBar
                            NumberAnimation { duration: 280; easing.type: Easing.OutQuint }
                        }
                        Behavior on y {
                            enabled: dockWindow.visualContentActive && dockWindow.initialized && !dockWindow.positionChanging && !dockWindow.sameSideAsBar
                            NumberAnimation { duration: 280; easing.type: Easing.OutQuint }
                        }
                    }

                    HoverHandler {
                        id: dockHover
                        acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
                        margin: dockWindow.hoverExpansion
                        onHoveredChanged: {
                            if (!hovered) {
                                dockContainer.hoveredItemIndex = -1;
                                dockContainer.cancelHoverReset();
                            }
                        }
                    }

                    function calculateDropIndex(mx, my) {
                        if (dockAppsModel.count <= 1) return 0;
                        let spacing = itemSpacing;
                        let size = itemSize;
                        let step = size + spacing;
                        if (dockWindow.isVertical) {
                            let layoutY = dockViewport.y + dockLayout.y;
                            let relY = my - layoutY;
                            let idx = Math.floor((relY + spacing / 2) / step);
                            return Math.max(0, Math.min(dockAppsModel.count - 1, idx));
                        } else {
                            let layoutX = dockViewport.x + dockLayout.x;
                            let relX = mx - layoutX;
                            let idx = Math.floor((relX + spacing / 2) / step);
                            return Math.max(0, Math.min(dockAppsModel.count - 1, idx));
                        }
                    }

                    GlassFillets {
                        id: dockFillets
                        card: dockBg
                        surfaceEnabled: dockWindow.dockBackground && filletRadius > 0.5
                        opacity: (dockWindow.sameSideAsBar ? dockContainer.revealProgress : 1.0) * (ThemeBackend.liquidGlass ? 1.0 : dockContainer.outerCornerProgress)
                        filletRadius: !dockContainer.fanFillets ? 0 : (ThemeBackend.liquidGlass ? dockWindow.outerCornerRadius * dockContainer.outerCornerProgress : (dockContainer.outerCornerProgress > 0.001 ? dockWindow.outerCornerRadius : 0))
                        filletDir: dockWindow.dockPosition === "bottom" ? Qt.vector2d(0, 1)
                                 : dockWindow.dockPosition === "top" ? Qt.vector2d(0, -1)
                                 : dockWindow.dockPosition === "left" ? Qt.vector2d(-1, 0)
                                 : Qt.vector2d(1, 0)
                    }

                    Item {
                        anchors.fill: parent
                        visible: dockWindow.dockBackground

                        WallpaperBlur {
                            anchors.fill: parent
                            topLeftRadius: dockContainer.topLeftCorner
                            topRightRadius: dockContainer.topRightCorner
                            bottomLeftRadius: dockContainer.bottomLeftCorner
                            bottomRightRadius: dockContainer.bottomRightCorner
                            surfaceEnabled: dockWindow.dockBackground
                            filletRadius: dockFillets.filletRadius
                            filletDir: dockFillets.filletDir
                        }
                    }

                    Item {
                        id: dockBgWrapper
                        anchors.fill: parent
                        opacity: dockWindow.dockOpacity
                        layer.enabled: dockWindow.dockOpacity < 1.0

                        MouseArea {
                            anchors.fill: parent
                            z: -1
                            acceptedButtons: Qt.LeftButton | Qt.RightButton
                            onClicked: {
                                if (typeof DockAppMenuController !== "undefined" && DockAppMenuController.isVisible) {
                                    DockAppMenuController.hide();
                                }
                            }
                        }

                        Rectangle {
                            id: dockBg
                            anchors.fill: parent
                            radius: dockContainer.dynamicCornerRadius
                            topLeftRadius: dockContainer.topLeftCorner
                            topRightRadius: dockContainer.topRightCorner
                            bottomLeftRadius: dockContainer.bottomLeftCorner
                            bottomRightRadius: dockContainer.bottomRightCorner
                            color: ThemeBackend.liquidGlass ? "transparent" : ThemeBackend.dockLauncherBase
                            border.width: (!ThemeBackend.liquidGlass && (dockWindow.dockFloating || dockWindow.editMode || dockContainer.outerCornerProgress < 0.99)) ? 1 : 0
                            border.color: (dockWindow.dockFloating || dockWindow.editMode || dockContainer.outerCornerProgress < 0.99) ? Qt.alpha(ThemeBackend.dockLauncherSurface1, 0.4 * (1.0 - dockContainer.outerCornerProgress * 0.5)) : "transparent"
                            clip: true

                        }
                    }

                    ShaderEffect {
                        visible: !ThemeBackend.liquidGlass && dockContainer.outerCornerProgress > 0.001 && dockWindow.outerCornerRadius > 0.5 && (!dockWindow.sameSideAsBar || (dockWindow.isVertical ? dockContainer.width : dockContainer.height) >= dockWindow.outerCornerRadius)
                        opacity: dockContainer.outerCornerProgress * (dockWindow.sameSideAsBar ? dockContainer.revealProgress : 1.0)
                        x: {
                            if (dockWindow.dockPosition === "left") return 0;
                            if (dockWindow.dockPosition === "right") return dockContainer.width - dockWindow.outerCornerRadius;
                            return -dockWindow.outerCornerRadius;
                        }
                        y: {
                            if (dockWindow.dockPosition === "bottom") return dockContainer.height - dockWindow.outerCornerRadius;
                            if (dockWindow.dockPosition === "left" || dockWindow.dockPosition === "right") return -dockWindow.outerCornerRadius;
                            return 0;
                        }
                        width: dockWindow.outerCornerRadius
                        height: dockWindow.outerCornerRadius
                        property vector2d itemSize: Qt.vector2d(width, height)
                        property real cornerIndex: {
                            if (dockWindow.dockPosition === "bottom") return 3.0;
                            if (dockWindow.dockPosition === "left") return 2.0;
                            if (dockWindow.dockPosition === "right") return 3.0;
                            return 1.0; // top
                        }
                        property color color: Qt.alpha(ThemeBackend.dockLauncherBase, dockWindow.dockOpacity)
                        fragmentShader: "file://" + Caching.payloadDir + "/assets/shaders/ui/corner_cutout.frag.qsb"
                    }

                    ShaderEffect {
                        visible: !ThemeBackend.liquidGlass && dockContainer.outerCornerProgress > 0.001 && dockWindow.outerCornerRadius > 0.5 && (!dockWindow.sameSideAsBar || (dockWindow.isVertical ? dockContainer.width : dockContainer.height) >= dockWindow.outerCornerRadius)
                        opacity: dockContainer.outerCornerProgress * (dockWindow.sameSideAsBar ? dockContainer.revealProgress : 1.0)
                        x: {
                            if (dockWindow.dockPosition === "left") return 0;
                            if (dockWindow.dockPosition === "right") return dockContainer.width - dockWindow.outerCornerRadius;
                            return dockContainer.width;
                        }
                        y: {
                            if (dockWindow.dockPosition === "bottom") return dockContainer.height - dockWindow.outerCornerRadius;
                            if (dockWindow.dockPosition === "left" || dockWindow.dockPosition === "right") return dockContainer.height;
                            return 0;
                        }
                        width: dockWindow.outerCornerRadius
                        height: dockWindow.outerCornerRadius
                        property vector2d itemSize: Qt.vector2d(width, height)
                        property real cornerIndex: {
                            if (dockWindow.dockPosition === "bottom") return 2.0;
                            if (dockWindow.dockPosition === "left") return 0.0;
                            if (dockWindow.dockPosition === "right") return 1.0;
                            return 0.0; // top
                        }
                        property color color: Qt.alpha(ThemeBackend.dockLauncherBase, dockWindow.dockOpacity)
                        fragmentShader: "file://" + Caching.payloadDir + "/assets/shaders/ui/corner_cutout.frag.qsb"
                    }

                    Item {
                        id: dockContentClipper
                        anchors.fill: parent
                        clip: dockWindow.sameSideAsBar && dockContainer.revealProgress < 0.999
                        opacity: dockWindow.sameSideAsBar ? dockContainer.revealProgress : 1.0
                        visible: !dockWindow.sameSideAsBar || dockContainer.revealProgress > 0.001

                        Item {
                            id: dockViewport
                            x: Math.round((dockContainer.width - width) / 2)
                            y: Math.round((dockContainer.height - height) / 2)

                            width: dockWindow.isVertical ? Math.round(dockContainer.dockThickness + dockWindow.s(16) + dockWindow.hoverExpansion * 2) : dockContainer.dockContentLength
                            height: dockWindow.isVertical ? dockContainer.dockContentLength : Math.round(dockContainer.dockThickness + dockWindow.s(16) + dockWindow.hoverExpansion * 2)
                            clip: dockWindow.enableScrolling && (dockContainer.totalItemCount > dockContainer.effectiveItemCount)
                            visible: width > 0 && height > 0

                            GridLayout {
                                id: dockLayout
                                columns: dockWindow.isVertical ? 1 : Math.max(1, dockContainer.totalItemCount)
                                rows: dockWindow.isVertical ? Math.max(1, dockContainer.totalItemCount) : 1
                                columnSpacing: dockContainer.itemSpacing
                                rowSpacing: dockContainer.itemSpacing
                                width: implicitWidth
                                height: implicitHeight

                                Behavior on columnSpacing {
                                    enabled: dockWindow.visualContentActive && dockWindow.initialized && !dockWindow.positionChanging
                                    NumberAnimation { duration: 250; easing.type: Easing.OutCubic }
                                }
                                Behavior on rowSpacing {
                                    enabled: dockWindow.visualContentActive && dockWindow.initialized && !dockWindow.positionChanging
                                    NumberAnimation { duration: 250; easing.type: Easing.OutCubic }
                                }

                                x: {
                                    if (dockWindow.isVertical) {
                                        return Math.round((dockViewport.width - width) / 2);
                                    }
                                    if (!dockWindow.enableScrolling || dockContainer.totalItemCount <= dockContainer.effectiveItemCount) {
                                        return Math.round((dockViewport.width - width) / 2);
                                    }
                                    return Math.round(-dockContainer.scrollIndex * dockContainer.itemStep);
                                }

                                y: {
                                    if (!dockWindow.isVertical) {
                                        return Math.round((dockViewport.height - height) / 2);
                                    }
                                    if (!dockWindow.enableScrolling || dockContainer.totalItemCount <= dockContainer.effectiveItemCount) {
                                        return Math.round((dockViewport.height - height) / 2);
                                    }
                                    return Math.round(-dockContainer.scrollIndex * dockContainer.itemStep);
                                }

                                Behavior on x {
                                    enabled: dockWindow.visualContentActive && dockWindow.initialized && dockWindow.enableScrolling && dockContainer.revealProgress >= 0.99 && !dockWindow.positionChanging
                                    NumberAnimation { duration: 240; easing.type: Easing.OutCubic }
                                }
                                Behavior on y {
                                    enabled: dockWindow.visualContentActive && dockWindow.initialized && dockWindow.enableScrolling && dockContainer.revealProgress >= 0.99 && !dockWindow.positionChanging
                                    NumberAnimation { duration: 240; easing.type: Easing.OutCubic }
                                }

                                Repeater {
                                    model: dockAppsModel

                                    delegate: Item {
                                        id: dockButton
                                        required property int index
                                        required property string name
                                        required property string desktop_id
                                        required property string icon
                                        implicitWidth: dockWindow.s(dockWindow.dockElementSize)
                                        implicitHeight: dockWindow.s(dockWindow.dockElementSize)
                                        Layout.preferredWidth: dockWindow.s(dockWindow.dockElementSize)
                                        Layout.preferredHeight: dockWindow.s(dockWindow.dockElementSize)
                                        Layout.alignment: Qt.AlignCenter
                                        z: (dockButton.isBeingDragged || dockWindow.dragSourceIndex === index) ? 99999 : Math.round(btnShape.scale * 100)

                                        property int itemIndex: index
                                        property real popScale: 1.0
                                        property real flashOpacity: 0.0
                                        property color btnColor: ThemeBackend.dockLauncherSurface0
                                        property int cornerRadius: Math.round(dockWindow.s(dockWindow.dockElementSize) * 0.28)
                                        property bool isDropTarget: dockWindow.dropTargetIndex === index && dockWindow.dragSourceIndex !== index
                                        property bool isBeingDragged: btnMa.drag.active
                                        property int btnSize: dockWindow.s(dockWindow.dockElementSize)

                                        readonly property var runningInfo: dockWindow.getRunningInfoForApp(dockButton.desktop_id)
                                        readonly property bool isAppRunning: runningInfo !== null && runningInfo.hasWindows
                                        readonly property bool isAppHere: runningInfo !== null && runningInfo.count > 0
                                        readonly property bool isAppFocused: runningInfo !== null && runningInfo.isFocused
                                        readonly property int appWindowCount: runningInfo !== null ? runningInfo.count : 0
                                        readonly property bool isOnlyElsewhere: runningInfo !== null && runningInfo.count === 0 && runningInfo.elsewhere.length > 0

                                        readonly property bool canHoverScale: !dockWindow.editMode && dockWindow.dragSourceIndex === -1 && !btnMa.drag.active
                                        readonly property real maxHoverScale: dockWindow.dockHoverScaleMultiplier
                                        readonly property real hoverScaleDelta: Math.max(0.0, maxHoverScale - 1.0)

                                        property real baseHoverScale: {
                                            if (!canHoverScale || dockContainer.hoveredItemIndex < 0) return 1.0;
                                            let diff = Math.abs(dockButton.itemIndex - dockContainer.hoveredItemIndex);
                                            if (diff === 0) return maxHoverScale;
                                            if (dockWindow.dockCascadeScale) {
                                                if (diff === 1) return 1.0 + hoverScaleDelta * 0.45;
                                                if (diff === 2) return 1.0 + hoverScaleDelta * 0.15;
                                            }
                                            return 1.0;
                                        }

                                        property real targetScale: {
                                            let s = baseHoverScale;
                                            if (btnMa.pressed && canHoverScale) {
                                                return s > 1.0 ? (s * 1.04) : 1.06;
                                            }
                                            return s;
                                        }

                                        property real animSpread: {
                                            if (!canHoverScale || dockContainer.hoveredItemIndex < 0) return 0.0;
                                            let diff = itemIndex - dockContainer.hoveredItemIndex;
                                            if (diff === 0) return 0.0;
                                            let sign = diff > 0 ? 1.0 : -1.0;
                                            let d = Math.abs(diff);
                                            let maxAllowedShift = Math.min(dockWindow.s(10), dockButton.btnSize * hoverScaleDelta * 0.65);
                                            if (dockWindow.dockCascadeScale) {
                                                let shift = (d === 1) ? (maxAllowedShift * 0.70) : maxAllowedShift;
                                                return sign * shift;
                                            } else {
                                                return (d === 1) ? (sign * maxAllowedShift * 0.45) : 0.0;
                                            }
                                        }

                                        property real animLift: {
                                            if (!canHoverScale || targetScale <= 1.0) return 0.0;
                                            let liftProgress = (targetScale - 1.0) / Math.max(0.01, maxHoverScale - 1.0);
                                            return dockWindow.s(5) * Math.min(1.0, Math.max(0.0, liftProgress));
                                        }

                                        property real targetOffsetX: {
                                            if (dockWindow.isVertical) {
                                                if (dockWindow.dockPosition === "left") return animLift;
                                                if (dockWindow.dockPosition === "right") return -animLift;
                                                return 0.0;
                                            } else {
                                                return animSpread;
                                            }
                                        }

                                        property real targetOffsetY: {
                                            if (dockWindow.isVertical) {
                                                if (dockWindow.dockPosition === "top") return animLift;
                                                if (dockWindow.dockPosition === "bottom") return -animLift;
                                                return 0.0;
                                            } else {
                                                if (dockWindow.dockPosition === "bottom") return -animLift;
                                                if (dockWindow.dockPosition === "top") return animLift;
                                                return 0.0;
                                            }
                                        }

                                        property real currentOffsetX: btnMa.drag.active ? 0 : targetOffsetX
                                        property real currentOffsetY: btnMa.drag.active ? 0 : targetOffsetY

                                        Behavior on currentOffsetX {
                                            enabled: dockWindow.visualContentActive && dockWindow.initialized && !dockWindow.positionChanging
                                            NumberAnimation { duration: 220; easing.type: Easing.OutCubic }
                                        }
                                        Behavior on currentOffsetY {
                                            enabled: dockWindow.visualContentActive && dockWindow.initialized && !dockWindow.positionChanging
                                            NumberAnimation { duration: 220; easing.type: Easing.OutCubic }
                                        }

                                        Rectangle {
                                            anchors.fill: parent
                                            visible: dockButton.isBeingDragged
                                            radius: dockButton.cornerRadius
                                            color: Qt.alpha(ThemeBackend.dockLauncherSurface0, 0.25)
                                            border.color: Qt.alpha(ThemeBackend.dockLauncherAccent, 0.5)
                                            border.width: 1
                                        }

                                        Rectangle {
                                            id: dropIndicator
                                            anchors.fill: parent
                                            anchors.margins: -dockWindow.s(3)
                                            radius: dockButton.cornerRadius + dockWindow.s(3)
                                            color: "transparent"
                                            border.color: ThemeBackend.dockLauncherAccent
                                            border.width: 2
                                            visible: dockButton.isDropTarget
                                            opacity: dockButton.isDropTarget ? 1.0 : 0.0
                                            Behavior on opacity {
                                                enabled: dockWindow.visualContentActive
                                                NumberAnimation { duration: 150 }
                                            }
                                        }

                                        Item {
                                            id: floatWrapper
                                            anchors.fill: !btnMa.drag.active ? parent : undefined
                                            width: dockButton.btnSize
                                            height: dockButton.btnSize
                                            z: btnMa.drag.active ? 999999 : 1

                                            transform: Translate {
                                                id: buttonTransform
                                                x: dockButton.currentOffsetX
                                                y: dockButton.currentOffsetY
                                            }

                                            Drag.active: btnMa.drag.active
                                            Drag.source: dockButton
                                            Drag.hotSpot.x: width / 2
                                            Drag.hotSpot.y: height / 2

                                            states: [
                                                State {
                                                    when: btnMa.drag.active
                                                    ParentChange { target: floatWrapper; parent: dragOverlay }
                                                    PropertyChanges {
                                                        target: floatWrapper
                                                        width: dockButton.btnSize
                                                        height: dockButton.btnSize
                                                        scale: 1.15
                                                        opacity: 0.92
                                                        z: 999999
                                                    }
                                                }
                                            ]

                                            Rectangle {
                                                id: btnShape
                                                anchors.fill: parent
                                                radius: dockButton.cornerRadius
                                                clip: true
                                                color: dockBtnGlass.active ? "transparent" : Qt.alpha(btnMa.pressed ? Qt.darker(dockButton.btnColor, 1.12) : (btnMa.containsMouse ? Qt.lighter(dockButton.btnColor, 1.12) : dockButton.btnColor), ThemeBackend.elementOpacity)

                                                Behavior on color {
                                                    enabled: dockWindow.visualContentActive
                                                    ColorAnimation { duration: 180 }
                                                }

                                                GlassFill {
                                                    id: dockBtnGlass
                                                    anchors.fill: parent
                                                    radius: parent.radius
                                                    tintColor: dockButton.btnColor
                                                    tintAmount: 0.3
                                                    press: btnMa.pressed ? 1 : Math.max(btnMa.containsMouse ? 0.3 : 0, 0.3 * Math.max(0, Math.min(1, (dockButton.baseHoverScale - 1.0) / Math.max(0.01, dockButton.hoverScaleDelta))))
                                                    Behavior on press {
                                                        enabled: dockWindow.visualContentActive
                                                        NumberAnimation { duration: 200; easing.type: Easing.OutCubic }
                                                    }
                                                }

                                                transformOrigin: Item.Center

                                                scale: dockButton.targetScale * dockButton.popScale
                                                Behavior on scale {
                                                    enabled: dockWindow.visualContentActive
                                                    NumberAnimation { duration: 220; easing.type: Easing.OutCubic }
                                                }

                                                SequentialAnimation {
                                                    id: btnPopAnim
                                                    NumberAnimation { target: dockButton; property: "popScale"; to: 1.1; duration: 110; easing.type: Easing.OutQuad }
                                                    NumberAnimation { target: dockButton; property: "popScale"; to: 1.0; duration: 420; easing.type: Easing.OutQuint }
                                                }

                                                Image {
                                                    id: appIcon
                                                    anchors.fill: parent
                                                    anchors.margins: dockWindow.s(Math.round(dockWindow.dockElementSize * 0.14))
                                                    sourceSize: Qt.size(
                                                        Math.ceil(width * (dockWindow.screen?.devicePixelRatio ?? 1)),
                                                        Math.ceil(height * (dockWindow.screen?.devicePixelRatio ?? 1)))
                                                    fillMode: Image.PreserveAspectFit
                                                    asynchronous: true
                                                    smooth: true
                                                    mipmap: true
                                                    property bool failedLoad: false

                                                    visible: source !== "" && status === Image.Ready && !failedLoad

                                                    source: {
                                                        let ic = dockButton.icon;
                                                        if (!ic) return "";
                                                        if (ic.startsWith("file://") || ic.startsWith("image://") || ic.startsWith("http://") || ic.startsWith("https://")) return ic;
                                                        return ic.startsWith("/") ? "file://" + ic : "image://icon/" + ic;
                                                    }

                                                    onStatusChanged: {
                                                        if (status === Image.Error) {
                                                            failedLoad = true;
                                                        }
                                                    }
                                                }

                                                SymbolIcon {
                                                    anchors.centerIn: parent
                                                    visible: !appIcon.visible
                                                    name: "view-app-grid"
                                                    size: dockWindow.s(Math.round(dockWindow.dockElementSize * 0.52))
                                                    color: ThemeBackend.dockLauncherText
                                                }

                                                Rectangle {
                                                    anchors.fill: parent
                                                    radius: dockButton.cornerRadius
                                                    color: "#ffffff"
                                                    opacity: dockButton.flashOpacity
                                                    PropertyAnimation on opacity {
                                                        id: btnFlashAnim
                                                        to: 0
                                                        duration: 400
                                                        easing.type: Easing.OutExpo
                                                    }
                                                }
                                            }

                                            Item {
                                                id: indicatorContainer
                                                visible: dockWindow.showRunningIndicators && dockButton.isAppRunning && !dockWindow.editMode
                                                opacity: (visible && dockContainer.revealProgress > 0.1) ? 1.0 : 0.0
                                                Behavior on opacity {
                                                    enabled: dockWindow.visualContentActive
                                                    NumberAnimation { duration: 180; easing.type: Easing.OutCubic }
                                                }

                                                anchors.horizontalCenter: !dockWindow.isVertical ? parent.horizontalCenter : undefined
                                                anchors.verticalCenter: dockWindow.isVertical ? parent.verticalCenter : undefined

                                                anchors.bottom: dockWindow.dockPosition === "bottom" ? parent.bottom : undefined
                                                anchors.top: dockWindow.dockPosition === "top" ? parent.top : undefined
                                                anchors.left: dockWindow.dockPosition === "left" ? parent.left : undefined
                                                anchors.right: dockWindow.dockPosition === "right" ? parent.right : undefined

                                                anchors.bottomMargin: dockWindow.dockPosition === "bottom" ? dockWindow.s(2) : 0
                                                anchors.topMargin: dockWindow.dockPosition === "top" ? dockWindow.s(2) : 0
                                                anchors.leftMargin: dockWindow.dockPosition === "left" ? dockWindow.s(2) : 0
                                                anchors.rightMargin: dockWindow.dockPosition === "right" ? dockWindow.s(2) : 0

                                                width: dockWindow.isVertical ? dockWindow.s(4) : contentRow.implicitWidth
                                                height: dockWindow.isVertical ? contentCol.implicitHeight : dockWindow.s(4)

                                                Rectangle {
                                                    id: elsewhereRing
                                                    visible: dockButton.isOnlyElsewhere
                                                    anchors.centerIn: parent
                                                    width: dockWindow.s(4)
                                                    height: dockWindow.s(4)
                                                    radius: width / 2
                                                    color: "transparent"
                                                    border.width: 1
                                                    border.color: ThemeBackend.dockLauncherSubtext0
                                                }

                                                Rectangle {
                                                    id: singlePill
                                                    visible: !dockButton.isOnlyElsewhere && dockButton.appWindowCount === 1 && dockButton.isAppFocused
                                                    anchors.centerIn: parent
                                                    width: dockWindow.isVertical ? dockWindow.s(3.5) : dockWindow.s(14)
                                                    height: dockWindow.isVertical ? dockWindow.s(14) : dockWindow.s(3.5)
                                                    radius: dockWindow.s(2)
                                                    color: ThemeBackend.dockLauncherAccent

                                                    Behavior on width {
                                                        enabled: dockWindow.visualContentActive
                                                        NumberAnimation { duration: 180; easing.type: Easing.OutCubic }
                                                    }
                                                    Behavior on height {
                                                        enabled: dockWindow.visualContentActive
                                                        NumberAnimation { duration: 180; easing.type: Easing.OutCubic }
                                                    }
                                                }

                                                Row {
                                                    id: contentRow
                                                    visible: !dockButton.isOnlyElsewhere && !singlePill.visible && !dockWindow.isVertical
                                                    anchors.centerIn: parent
                                                    spacing: dockWindow.s(2.5)

                                                    Repeater {
                                                        model: Math.min(3, Math.max(1, dockButton.appWindowCount))
                                                        delegate: Rectangle {
                                                            width: dockWindow.s(3.5)
                                                            height: dockWindow.s(3.5)
                                                            radius: width / 2
                                                            color: (index === 0 && dockButton.isAppFocused) ? ThemeBackend.dockLauncherAccent : (index === 0 ? ThemeBackend.dockLauncherSubtext0 : ThemeBackend.dockLauncherSurface2)
                                                        }
                                                    }
                                                }

                                                Column {
                                                    id: contentCol
                                                    visible: !dockButton.isOnlyElsewhere && !singlePill.visible && dockWindow.isVertical
                                                    anchors.centerIn: parent
                                                    spacing: dockWindow.s(2.5)

                                                    Repeater {
                                                        model: Math.min(3, Math.max(1, dockButton.appWindowCount))
                                                        delegate: Rectangle {
                                                            width: dockWindow.s(3.5)
                                                            height: dockWindow.s(3.5)
                                                            radius: width / 2
                                                            color: (index === 0 && dockButton.isAppFocused) ? ThemeBackend.dockLauncherAccent : (index === 0 ? ThemeBackend.dockLauncherSubtext0 : ThemeBackend.dockLauncherSurface2)
                                                        }
                                                    }
                                                }
                                            }

                                            MouseArea {
                                                id: btnMa
                                                anchors.fill: parent
                                                anchors.margins: -Math.max(dockContainer.itemSpacing / 2, (btnShape.scale - 1.0) * dockButton.btnSize / 2)
                                                hoverEnabled: true
                                                cursorShape: (dockWindow.editMode || inDragHold)
                                                    ? (drag.active ? Qt.ClosedHandCursor : Qt.OpenHandCursor)
                                                    : Qt.PointingHandCursor
                                                acceptedButtons: Qt.LeftButton | Qt.MiddleButton | Qt.RightButton

                                                property bool inDragHold: false

                                                pressAndHoldInterval: 350

                                                drag.target: floatWrapper
                                                drag.axis: Drag.XAndYAxis
                                                drag.threshold: (dockWindow.editMode || inDragHold) ? 5 : 99999

                                                onEntered: {
                                                    dockContainer.cancelHoverReset();
                                                    if (dockButton.canHoverScale) {
                                                        dockContainer.hoveredItemIndex = dockButton.itemIndex;
                                                    }
                                                }

                                                onExited: {
                                                    if (dockContainer.hoveredItemIndex === dockButton.itemIndex) {
                                                        dockContainer.checkHoverReset();
                                                    }
                                                }

                                                onPressed: function(mouse) {
                                                    if (mouse.button !== Qt.LeftButton) return;
                                                    inDragHold = false;
                                                    if (dockWindow.editMode) {
                                                        dockWindow.dragSourceIndex = dockButton.itemIndex;
                                                        dockWindow.dropTargetIndex = dockButton.itemIndex;
                                                        if (typeof Sounds !== "undefined") {
                                                            Sounds.playSfx("guide/barconfig/out.wav");
                                                        }
                                                    }
                                                }

                                                onPressAndHold: {
                                                    inDragHold = true;
                                                    if (!dockWindow.editMode) {
                                                        dockWindow.setEditMode(true);
                                                    }
                                                    dockWindow.dragSourceIndex = dockButton.itemIndex;
                                                    dockWindow.dropTargetIndex = dockButton.itemIndex;
                                                    if (typeof Sounds !== "undefined") {
                                                        Sounds.playSfx("guide/barconfig/out.wav");
                                                    }
                                                }

                                                onPositionChanged: function(mouse) {
                                                    if ((dockWindow.editMode || inDragHold) && drag.active) {
                                                        let pt = mapToItem(dockContainer, mouse.x, mouse.y);
                                                        dockWindow.dropTargetIndex = dockContainer.calculateDropIndex(pt.x, pt.y);
                                                    }
                                                }

                                                onReleased: {
                                                    let wasHold = inDragHold;
                                                    inDragHold = false;
                                                    if (dockWindow.editMode || wasHold) {
                                                        let wasDragging = drag.active;
                                                        floatWrapper.Drag.drop();
                                                        if (wasDragging) {
                                                            let pt = btnMa.mapToItem(dockContainer, btnMa.mouseX, btnMa.mouseY);
                                                            let fwPos = floatWrapper.mapToItem(dockContainer, 0, 0);
                                                            let completelyOutside = (fwPos.x + floatWrapper.width <= 0 || fwPos.x >= dockContainer.width ||
                                                                                     fwPos.y + floatWrapper.height <= 0 || fwPos.y >= dockContainer.height) ||
                                                                                    (pt.x < -dockWindow.s(16) || pt.x > dockContainer.width + dockWindow.s(16) ||
                                                                                     pt.y < -dockWindow.s(16) || pt.y > dockContainer.height + dockWindow.s(16));
                                                            if (completelyOutside) {
                                                                if (typeof Sounds !== "undefined") {
                                                                    Sounds.playSfx("guide/barconfig/in.wav");
                                                                }
                                                                if (dockButton.desktop_id !== "") {
                                                                    dockWindow.removeAppByDesktopId(dockButton.desktop_id);
                                                                } else if (dockWindow.dragSourceIndex >= 0 && dockWindow.dragSourceIndex < dockAppsModel.count) {
                                                                    dockAppsModel.remove(dockWindow.dragSourceIndex, 1);
                                                                    dockWindow.saveApps();
                                                                    dockWindow._syncRunningApps();
                                                                }
                                                            } else if (dockWindow.dragSourceIndex !== -1 && dockWindow.dropTargetIndex !== -1 && dockWindow.dragSourceIndex !== dockWindow.dropTargetIndex) {
                                                                if (typeof Sounds !== "undefined") {
                                                                    Sounds.playSfx("guide/barconfig/in.wav");
                                                                }
                                                                dockAppsModel.move(dockWindow.dragSourceIndex, dockWindow.dropTargetIndex, 1);
                                                                dockWindow.saveApps();
                                                            } else if (typeof Sounds !== "undefined") {
                                                                Sounds.playSfx("guide/barconfig/in.wav");
                                                            }
                                                        }
                                                        dockWindow.dragSourceIndex = -1;
                                                        dockWindow.dropTargetIndex = -1;
                                                        floatWrapper.x = 0;
                                                        floatWrapper.y = 0;
                                                    }
                                                }

                                                onClicked: function(mouse) {
                                                    if (dockWindow.editMode || inDragHold || drag.active) return;

                                                    if (mouse.button === Qt.MiddleButton) {
                                                        btnPopAnim.start();
                                                        dockButton.flashOpacity = 0.4;
                                                        btnFlashAnim.start();
                                                        if (typeof Sounds !== "undefined") Sounds.playSfx("reusables/iconbutton/click.wav");
                                                        dockWindow.launchApp(dockButton.desktop_id);
                                                        return;
                                                    }

                                                    if (mouse.button === Qt.RightButton) {
                                                        let pt = dockButton.mapToItem(null, 0, 0);
                                                        let globX = pt.x + (dockButton.width / 2);
                                                        let globY = pt.y + (dockButton.height / 2);
                                                        let info = dockWindow.getRunningInfoForApp(dockButton.desktop_id);
                                                        let winList = dockWindow.menuWindows(info);
                                                        DockAppMenuController.toggle(
                                                            dockWindow.screen,
                                                            globX,
                                                            globY,
                                                            dockWindow.dockPosition,
                                                            dockButton.desktop_id,
                                                            dockButton.name || (info ? info.appId : ""),
                                                            dockButton.icon || "",
                                                            true,
                                                            winList,
                                                            dockButton.width
                                                        );
                                                        return;
                                                    }

                                                    if (mouse.button === Qt.LeftButton) {
                                                        if (typeof DockAppMenuController !== "undefined" && DockAppMenuController.isVisible && !DockAppMenuController.isSameApp(DockAppMenuController.desktopId, dockButton.desktop_id)) {
                                                            let pt = dockButton.mapToItem(null, 0, 0);
                                                            let globX = pt.x + (dockButton.width / 2);
                                                            let globY = pt.y + (dockButton.height / 2);
                                                            let info = dockWindow.getRunningInfoForApp(dockButton.desktop_id);
                                                            let winList = dockWindow.menuWindows(info);
                                                            DockAppMenuController.show(
                                                                dockWindow.screen,
                                                                globX,
                                                                globY,
                                                                dockWindow.dockPosition,
                                                                dockButton.desktop_id,
                                                                dockButton.name || (info ? info.appId : ""),
                                                                dockButton.icon || "",
                                                                true,
                                                                winList,
                                                                dockButton.width
                                                            );
                                                            return;
                                                        }

                                                        if (typeof DockAppMenuController !== "undefined" && DockAppMenuController.isVisible && DockAppMenuController.isSameApp(DockAppMenuController.desktopId, dockButton.desktop_id)) {
                                                            DockAppMenuController.hide();
                                                        }

                                                        btnPopAnim.start();
                                                        dockButton.flashOpacity = 0.4;
                                                        btnFlashAnim.start();
                                                        if (typeof Sounds !== "undefined") Sounds.playSfx("reusables/iconbutton/click.wav");
                                                        dockWindow.activateOrLaunch(dockButton.desktop_id, mouse.modifiers);
                                                    }
                                                }
                                            }
                                        }
                                    }
                                }

                                Item {
                                    id: dockSeparator
                                    visible: dockWindow.showRunningApps && runningAppsModel.count > 0 && dockAppsModel.count > 0 && !dockWindow.editMode
                                    Layout.preferredWidth: visible ? (dockWindow.isVertical ? dockWindow.s(dockWindow.dockElementSize) : dockWindow.s(1)) : 0
                                    Layout.preferredHeight: visible ? (dockWindow.isVertical ? dockWindow.s(1) : dockWindow.s(dockWindow.dockElementSize)) : 0
                                    Layout.maximumWidth: visible ? -1 : 0
                                    Layout.maximumHeight: visible ? -1 : 0
                                    Layout.alignment: Qt.AlignCenter

                                    Rectangle {
                                        anchors.centerIn: parent
                                        width: dockWindow.isVertical ? Math.round(dockWindow.s(dockWindow.dockElementSize) * 0.5) : 1
                                        height: dockWindow.isVertical ? 1 : Math.round(dockWindow.s(dockWindow.dockElementSize) * 0.5)
                                        radius: 1
                                        color: Qt.alpha(ThemeBackend.dockLauncherSurface2, 0.7)
                                    }
                                }

                                Repeater {
                                    id: runningAppsRepeater
                                    model: (dockWindow.showRunningApps && !dockWindow.editMode) ? runningAppsModel : null

                                    delegate: Item {
                                        id: runningDockButton
                                        implicitWidth: dockWindow.s(dockWindow.dockElementSize)
                                        implicitHeight: dockWindow.s(dockWindow.dockElementSize)
                                        Layout.preferredWidth: dockWindow.s(dockWindow.dockElementSize)
                                        Layout.preferredHeight: dockWindow.s(dockWindow.dockElementSize)
                                        Layout.alignment: Qt.AlignCenter

                                        property int itemIndex: dockAppsModel.count + 1 + index
                                        property real popScale: 1.0
                                        property real flashOpacity: 0.0
                                        property color btnColor: ThemeBackend.dockLauncherSurface0
                                        property int cornerRadius: Math.round(dockWindow.s(dockWindow.dockElementSize) * 0.28)
                                        property int btnSize: dockWindow.s(dockWindow.dockElementSize)

                                        readonly property var runningInfo: dockWindow.getRunningInfoForApp(model.desktop_id)
                                        readonly property bool isAppRunning: runningInfo !== null && runningInfo.hasWindows
                                        readonly property bool isAppHere: runningInfo !== null && runningInfo.count > 0
                                        readonly property bool isAppFocused: runningInfo !== null && runningInfo.isFocused
                                        readonly property int appWindowCount: runningInfo !== null ? runningInfo.count : 0
                                        readonly property bool isOnlyElsewhere: runningInfo !== null && runningInfo.count === 0 && runningInfo.elsewhere.length > 0

                                        z: Math.round(runningBtnShape.scale * 100)

                                        readonly property bool canHoverScale: !dockWindow.editMode && dockWindow.dragSourceIndex === -1
                                        readonly property real maxHoverScale: dockWindow.dockHoverScaleMultiplier
                                        readonly property real hoverScaleDelta: Math.max(0.0, maxHoverScale - 1.0)

                                        property real baseHoverScale: {
                                            if (!canHoverScale || dockContainer.hoveredItemIndex < 0) return 1.0;
                                            let diff = Math.abs(runningDockButton.itemIndex - dockContainer.hoveredItemIndex);
                                            if (diff === 0) return maxHoverScale;
                                            if (dockWindow.dockCascadeScale) {
                                                if (diff === 1) return 1.0 + hoverScaleDelta * 0.45;
                                                if (diff === 2) return 1.0 + hoverScaleDelta * 0.15;
                                            }
                                            return 1.0;
                                        }

                                        property real targetScale: {
                                            let s = baseHoverScale;
                                            if (runningBtnMa.pressed && canHoverScale) {
                                                return s > 1.0 ? (s * 1.04) : 1.06;
                                            }
                                            return s;
                                        }

                                        property real animSpread: {
                                            if (!canHoverScale || dockContainer.hoveredItemIndex < 0) return 0.0;
                                            let diff = itemIndex - dockContainer.hoveredItemIndex;
                                            if (diff === 0) return 0.0;
                                            let sign = diff > 0 ? 1.0 : -1.0;
                                            let d = Math.abs(diff);
                                            let maxAllowedShift = Math.min(dockWindow.s(10), runningDockButton.btnSize * hoverScaleDelta * 0.65);
                                            if (dockWindow.dockCascadeScale) {
                                                let shift = (d === 1) ? (maxAllowedShift * 0.70) : maxAllowedShift;
                                                return sign * shift;
                                            } else {
                                                return (d === 1) ? (sign * maxAllowedShift * 0.45) : 0.0;
                                            }
                                        }

                                        property real animLift: {
                                            if (!canHoverScale || targetScale <= 1.0) return 0.0;
                                            let liftProgress = (targetScale - 1.0) / Math.max(0.01, maxHoverScale - 1.0);
                                            return dockWindow.s(5) * Math.min(1.0, Math.max(0.0, liftProgress));
                                        }

                                        property real targetOffsetX: {
                                            if (dockWindow.isVertical) {
                                                if (dockWindow.dockPosition === "left") return animLift;
                                                if (dockWindow.dockPosition === "right") return -animLift;
                                                return 0.0;
                                            }
                                            return animSpread;
                                        }

                                        property real targetOffsetY: {
                                            if (dockWindow.isVertical) return animSpread;
                                            if (dockWindow.dockPosition === "bottom") return -animLift;
                                            if (dockWindow.dockPosition === "top") return animLift;
                                            return 0.0;
                                        }

                                        property real currentOffsetX: targetOffsetX
                                        property real currentOffsetY: targetOffsetY

                                        Behavior on currentOffsetX {
                                            enabled: dockWindow.visualContentActive && dockWindow.initialized && !dockWindow.positionChanging
                                            NumberAnimation { duration: 220; easing.type: Easing.OutCubic }
                                        }
                                        Behavior on currentOffsetY {
                                            enabled: dockWindow.visualContentActive && dockWindow.initialized && !dockWindow.positionChanging
                                            NumberAnimation { duration: 220; easing.type: Easing.OutCubic }
                                        }

                                        Item {
                                            id: runningFloatWrapper
                                            anchors.fill: parent

                                            transform: Translate {
                                                x: runningDockButton.currentOffsetX
                                                y: runningDockButton.currentOffsetY
                                            }

                                            Rectangle {
                                                id: runningBtnShape
                                                anchors.fill: parent
                                                radius: runningDockButton.cornerRadius
                                                clip: true
                                                color: runningBtnGlass.active ? "transparent" : Qt.alpha(runningBtnMa.pressed ? Qt.darker(runningDockButton.btnColor, 1.12) : (runningBtnMa.containsMouse ? Qt.lighter(runningDockButton.btnColor, 1.12) : runningDockButton.btnColor), ThemeBackend.elementOpacity)

                                                GlassFill {
                                                    id: runningBtnGlass
                                                    anchors.fill: parent
                                                    radius: parent.radius
                                                    tintColor: runningDockButton.btnColor
                                                    tintAmount: 0.3
                                                    press: runningBtnMa.pressed ? 1 : Math.max(runningBtnMa.containsMouse ? 0.3 : 0, 0.3 * Math.max(0, Math.min(1, (runningDockButton.baseHoverScale - 1.0) / Math.max(0.01, runningDockButton.hoverScaleDelta))))
                                                    Behavior on press {
                                                        enabled: dockWindow.visualContentActive
                                                        NumberAnimation { duration: 200; easing.type: Easing.OutCubic }
                                                    }
                                                }
                                                transformOrigin: Item.Center
                                                scale: runningDockButton.targetScale * runningDockButton.popScale

                                                Behavior on color {
                                                    enabled: dockWindow.visualContentActive
                                                    ColorAnimation { duration: 180 }
                                                }
                                                Behavior on scale {
                                                    enabled: dockWindow.visualContentActive
                                                    NumberAnimation { duration: 220; easing.type: Easing.OutCubic }
                                                }

                                                Image {
                                                    id: runningAppIcon
                                                    anchors.fill: parent
                                                    anchors.margins: dockWindow.s(Math.round(dockWindow.dockElementSize * 0.14))
                                                    sourceSize: Qt.size(
                                                        Math.ceil(width * (dockWindow.screen?.devicePixelRatio ?? 1)),
                                                        Math.ceil(height * (dockWindow.screen?.devicePixelRatio ?? 1)))
                                                    fillMode: Image.PreserveAspectFit
                                                    asynchronous: true
                                                    smooth: true
                                                    mipmap: true
                                                    property bool failedLoad: false
                                                    visible: source !== "" && status === Image.Ready && !failedLoad

                                                    source: {
                                                        let ic = model.icon || "";
                                                        if (!ic) return "";
                                                        if (ic.startsWith("file://") || ic.startsWith("image://") || ic.startsWith("http://") || ic.startsWith("https://")) return ic;
                                                        return ic.startsWith("/") ? "file://" + ic : "image://icon/" + ic;
                                                    }
                                                    onStatusChanged: {
                                                        if (status === Image.Error) failedLoad = true;
                                                    }
                                                }

                                                Text {
                                                    anchors.centerIn: parent
                                                    visible: runningAppIcon.source === "" || runningAppIcon.failedLoad || runningAppIcon.status === Image.Error
                                                    text: model.name ? model.name.charAt(0).toUpperCase() : "?"
                                                    font.family: ThemeBackend.fontFamily
                                                    font.pixelSize: dockWindow.s(Math.round(dockWindow.dockElementSize * 0.42))
                                                    font.weight: Font.Bold
                                                    color: ThemeBackend.dockLauncherText
                                                }

                                                Rectangle {
                                                    anchors.fill: parent
                                                    radius: runningDockButton.cornerRadius
                                                    color: "#ffffff"
                                                    opacity: runningDockButton.flashOpacity
                                                    PropertyAnimation on opacity {
                                                        id: runningBtnFlashAnim
                                                        to: 0
                                                        duration: 400
                                                        easing.type: Easing.OutExpo
                                                    }
                                                }
                                            }

                                            Item {
                                                id: runningIndicatorContainer
                                                visible: dockWindow.showRunningIndicators && runningDockButton.isAppRunning
                                                opacity: (visible && dockContainer.revealProgress > 0.1) ? 1.0 : 0.0

                                                anchors.horizontalCenter: !dockWindow.isVertical ? parent.horizontalCenter : undefined
                                                anchors.verticalCenter: dockWindow.isVertical ? parent.verticalCenter : undefined

                                                anchors.bottom: dockWindow.dockPosition === "bottom" ? parent.bottom : undefined
                                                anchors.top: dockWindow.dockPosition === "top" ? parent.top : undefined
                                                anchors.left: dockWindow.dockPosition === "left" ? parent.left : undefined
                                                anchors.right: dockWindow.dockPosition === "right" ? parent.right : undefined

                                                anchors.bottomMargin: dockWindow.dockPosition === "bottom" ? dockWindow.s(2) : 0
                                                anchors.topMargin: dockWindow.dockPosition === "top" ? dockWindow.s(2) : 0
                                                anchors.leftMargin: dockWindow.dockPosition === "left" ? dockWindow.s(2) : 0
                                                anchors.rightMargin: dockWindow.dockPosition === "right" ? dockWindow.s(2) : 0

                                                width: dockWindow.isVertical ? dockWindow.s(4) : runningRow.implicitWidth
                                                height: dockWindow.isVertical ? runningCol.implicitHeight : dockWindow.s(4)

                                                Rectangle {
                                                    visible: runningDockButton.isOnlyElsewhere
                                                    anchors.centerIn: parent
                                                    width: dockWindow.s(4)
                                                    height: dockWindow.s(4)
                                                    radius: width / 2
                                                    color: "transparent"
                                                    border.width: 1
                                                    border.color: ThemeBackend.dockLauncherSubtext0
                                                }

                                                Rectangle {
                                                    id: runningSinglePill
                                                    visible: !runningDockButton.isOnlyElsewhere && runningDockButton.appWindowCount === 1 && runningDockButton.isAppFocused
                                                    anchors.centerIn: parent
                                                    width: dockWindow.isVertical ? dockWindow.s(3.5) : dockWindow.s(14)
                                                    height: dockWindow.isVertical ? dockWindow.s(14) : dockWindow.s(3.5)
                                                    radius: dockWindow.s(2)
                                                    color: ThemeBackend.dockLauncherAccent
                                                }

                                                Row {
                                                    id: runningRow
                                                    visible: !runningDockButton.isOnlyElsewhere && !runningSinglePill.visible && !dockWindow.isVertical
                                                    anchors.centerIn: parent
                                                    spacing: dockWindow.s(2.5)

                                                    Repeater {
                                                        model: Math.min(3, Math.max(1, runningDockButton.appWindowCount))
                                                        delegate: Rectangle {
                                                            width: dockWindow.s(3.5)
                                                            height: dockWindow.s(3.5)
                                                            radius: width / 2
                                                            color: (index === 0 && runningDockButton.isAppFocused) ? ThemeBackend.dockLauncherAccent : (index === 0 ? ThemeBackend.dockLauncherSubtext0 : ThemeBackend.dockLauncherSurface2)
                                                        }
                                                    }
                                                }

                                                Column {
                                                    id: runningCol
                                                    visible: !runningDockButton.isOnlyElsewhere && !runningSinglePill.visible && dockWindow.isVertical
                                                    anchors.centerIn: parent
                                                    spacing: dockWindow.s(2.5)

                                                    Repeater {
                                                        model: Math.min(3, Math.max(1, runningDockButton.appWindowCount))
                                                        delegate: Rectangle {
                                                            width: dockWindow.s(3.5)
                                                            height: dockWindow.s(3.5)
                                                            radius: width / 2
                                                            color: (index === 0 && runningDockButton.isAppFocused) ? ThemeBackend.dockLauncherAccent : (index === 0 ? ThemeBackend.dockLauncherSubtext0 : ThemeBackend.dockLauncherSurface2)
                                                        }
                                                    }
                                                }
                                            }

                                            MouseArea {
                                                id: runningBtnMa
                                                anchors.fill: parent
                                                anchors.margins: -Math.max(dockContainer.itemSpacing / 2, (runningBtnShape.scale - 1.0) * runningDockButton.btnSize / 2)
                                                hoverEnabled: true
                                                acceptedButtons: Qt.LeftButton | Qt.MiddleButton | Qt.RightButton
                                                cursorShape: Qt.PointingHandCursor

                                                onEntered: {
                                                    dockContainer.cancelHoverReset();
                                                    if (runningDockButton.canHoverScale) {
                                                        dockContainer.hoveredItemIndex = runningDockButton.itemIndex;
                                                    }
                                                }

                                                onExited: {
                                                    if (dockContainer.hoveredItemIndex === runningDockButton.itemIndex) {
                                                        dockContainer.checkHoverReset();
                                                    }
                                                }

                                                onClicked: function(mouse) {
                                                    if (mouse.button === Qt.MiddleButton) {
                                                        dockWindow.launchApp(model.desktop_id);
                                                        return;
                                                    }
                                                    if (mouse.button === Qt.RightButton) {
                                                        let pt = runningDockButton.mapToItem(null, 0, 0);
                                                        let globX = pt.x + (runningDockButton.width / 2);
                                                        let globY = pt.y + (runningDockButton.height / 2);
                                                        let info = dockWindow.getRunningInfoForApp(model.desktop_id);
                                                        let winList = dockWindow.menuWindows(info);
                                                        DockAppMenuController.toggle(
                                                            dockWindow.screen,
                                                            globX,
                                                            globY,
                                                            dockWindow.dockPosition,
                                                            model.desktop_id,
                                                            model.name || (info ? info.appId : ""),
                                                            model.icon || "",
                                                            false,
                                                            winList,
                                                            runningDockButton.width
                                                        );
                                                        return;
                                                    }
                                                    if (mouse.button === Qt.LeftButton) {
                                                        if (typeof DockAppMenuController !== "undefined" && DockAppMenuController.isVisible && !DockAppMenuController.isSameApp(DockAppMenuController.desktopId, model.desktop_id)) {
                                                            let pt = runningDockButton.mapToItem(null, 0, 0);
                                                            let globX = pt.x + (runningDockButton.width / 2);
                                                            let globY = pt.y + (runningDockButton.height / 2);
                                                            let info = dockWindow.getRunningInfoForApp(model.desktop_id);
                                                            let winList = dockWindow.menuWindows(info);
                                                            DockAppMenuController.show(
                                                                dockWindow.screen,
                                                                globX,
                                                                globY,
                                                                dockWindow.dockPosition,
                                                                model.desktop_id,
                                                                model.name || (info ? info.appId : ""),
                                                                model.icon || "",
                                                                false,
                                                                winList,
                                                                runningDockButton.width
                                                            );
                                                            return;
                                                        }

                                                        if (typeof DockAppMenuController !== "undefined" && DockAppMenuController.isVisible && DockAppMenuController.isSameApp(DockAppMenuController.desktopId, model.desktop_id)) {
                                                            DockAppMenuController.hide();
                                                        }

                                                        runningDockButton.flashOpacity = 0.4;
                                                        runningBtnFlashAnim.start();
                                                        if (typeof Sounds !== "undefined") Sounds.playSfx("reusables/iconbutton/click.wav");
                                                        dockWindow.activateOrLaunch(model.desktop_id, mouse.modifiers);
                                                    }
                                                }
                                            }
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
                }



                Item {
                    id: appPicker
                    visible: dockWindow.editMode || opacity > 0.001
                    opacity: dockWindow.editMode ? 1.0 : 0.0
                    scale: dockWindow.editMode ? 1.0 : 0.94
                    z: 100

                    Behavior on opacity {
                        enabled: dockWindow.visualContentActive && dockWindow.initialized && !dockWindow.positionChanging
                        NumberAnimation { duration: 260; easing.type: Easing.OutCubic }
                    }
                    Behavior on scale {
                        enabled: dockWindow.visualContentActive && dockWindow.initialized && !dockWindow.positionChanging
                        NumberAnimation { duration: 280; easing.type: Easing.OutBack; easing.overshoot: 1.1 }
                    }

                    width: Math.min(dockWindow.effectiveWindowWidth - dockWindow.s(16), dockWindow.isVertical ? dockWindow.s(320) : dockWindow.s(480))

                    readonly property int maxPickerItems: dockWindow.isVertical ? 9 : 6
                    property int targetItemCount: {
                        if (pickerFilteredModel.count > 0) {
                            return Math.min(pickerFilteredModel.count, maxPickerItems);
                        }
                        return maxPickerItems;
                    }
                    property real targetPickerHeight: dockWindow.s(70) + (targetItemCount * dockWindow.s(48))
                    property real animatedPickerHeight: targetPickerHeight
                    Behavior on animatedPickerHeight {
                        enabled: dockWindow.visualContentActive && dockWindow.initialized && !dockWindow.positionChanging
                        NumberAnimation { duration: 250; easing.type: Easing.OutCubic }
                    }

                    height: animatedPickerHeight

                    x: {
                        if (!dockWindow.isVertical) return Math.round((dockWindow.effectiveWindowWidth - width) / 2);
                        if (dockWindow.dockPosition === "left") {
                            return Math.round(dockContainer.x + dockContainer.width + dockWindow.s(12));
                        }
                        if (dockWindow.dockPosition === "right") {
                            return Math.round(dockContainer.x - width - dockWindow.s(12));
                        }
                        return Math.round((dockWindow.effectiveWindowWidth - width) / 2);
                    }

                    y: {
                        if (dockWindow.isVertical) return Math.round((dockWindow.effectiveWindowHeight - height) / 2);
                        if (dockWindow.dockPosition === "top") {
                            return Math.round(dockContainer.y + dockContainer.height + dockWindow.s(12));
                        }
                        if (dockWindow.dockPosition === "bottom") {
                            return Math.round(dockContainer.y - height - dockWindow.s(12));
                        }
                        return Math.round((dockWindow.effectiveWindowHeight - height) / 2);
                    }

                    Rectangle {
                        anchors.fill: parent
                        radius: ThemeBackend.borderRadius <= 16 ? ThemeBackend.borderRadius * 1.5 : 24
                        color: Qt.alpha(ThemeBackend.dockLauncherBase, ThemeBackend.elementOpacity)
                        border.width: ThemeBackend.liquidGlass ? 0 : 1
                        border.color: Qt.alpha(ThemeBackend.dockLauncherSurface1, 0.45)
                        clip: true

                        WallpaperBlur {
                            z: -1
                            anchors.fill: parent
                            radius: parent.radius
                            standalone: true
                        }

                        Item {
                            id: pickerContent
                            anchors.fill: parent
                            anchors.margins: dockWindow.s(12)

                            readonly property bool isSearchAtBottom: dockWindow.dockPosition === "top"

                            RowLayout {
                                id: pickerSearchRow
                                anchors.left: parent.left
                                anchors.right: parent.right
                                y: pickerContent.isSearchAtBottom ? (parent.height - height) : 0
                                height: dockWindow.s(34)
                                spacing: dockWindow.s(8)

                                Input {
                                    id: pickerSearchInput
                                    Layout.fillWidth: true
                                    Layout.preferredHeight: dockWindow.s(34)
                                    placeholderText: I18n.t("guide.dock.picker.search", "Search apps...")
                                    baseColor: ThemeBackend.dockLauncherSurface0
                                    bgOpacity: ThemeBackend.glassActive ? 0.0 : 1.0
                                    accentColor: ThemeBackend.dockLauncherAccent
                                    textColor: ThemeBackend.dockLauncherText
                                    subTextColor: ThemeBackend.dockLauncherSubtext0
                                    borderColor: Qt.alpha(ThemeBackend.dockLauncherSurface2, 0.6)
                                    cornerRadius: ThemeBackend.borderRadius
                                    fontPixelSize: dockWindow.s(11)
                                    showClearButton: true
                                    focus: dockWindow.editMode
                                    onTextEdited: function(newText) {
                                        dockWindow.filterPickerApps(newText);
                                    }
                                    onCleared: {
                                        dockWindow.filterPickerApps("");
                                    }
                                    Keys.onEscapePressed: function(event) {
                                        dockWindow.setEditMode(false);
                                        event.accepted = true;
                                    }
                                }

                                ClickButton {
                                    implicitHeight: dockWindow.s(34)
                                    maxWidth: dockWindow.s(70)
                                    cornerRadius: ThemeBackend.borderRadius
                                    buttonText: I18n.t("guide.dock.picker.done", "Done")
                                    accentColor: ThemeBackend.dockLauncherAccent
                                    textColor: ThemeBackend.liquidGlass ? ThemeBackend.dockLauncherText : ThemeBackend.dockLauncherOnAccent
                                    textFontSize: dockWindow.s(11)
                                    onClicked: {
                                        dockWindow.setEditMode(false);
                                    }
                                }
                            }

                            Rectangle {
                                id: pickerDivider
                                anchors.left: parent.left
                                anchors.right: parent.right
                                y: pickerContent.isSearchAtBottom
                                   ? (pickerSearchRow.y - dockWindow.s(10) - height)
                                   : (pickerSearchRow.y + pickerSearchRow.height + dockWindow.s(10))
                                height: 1
                                color: Qt.alpha(ThemeBackend.dockLauncherSurface1, 0.3)
                            }

                            Item {
                                id: pickerListWrapper
                                anchors.left: parent.left
                                anchors.right: parent.right
                                y: pickerContent.isSearchAtBottom ? 0 : (pickerDivider.y + pickerDivider.height + dockWindow.s(10))
                                height: pickerContent.isSearchAtBottom
                                        ? Math.max(0, pickerDivider.y - dockWindow.s(10))
                                        : Math.max(0, parent.height - y)
                                clip: true

                                ListView {
                                    id: pickerList
                                    anchors.fill: parent
                                    clip: true
                                    model: pickerFilteredModel
                                    spacing: dockWindow.s(4)
                                    boundsBehavior: Flickable.StopAtBounds
                                    property Item hoverItem: null

                                    GlassLens {
                                        id: pickerLens
                                        z: -1
                                        targetX: pickerList.hoverItem ? pickerList.hoverItem.x : 0
                                        targetY: pickerList.hoverItem ? pickerList.hoverItem.y : 0
                                        targetWidth: pickerList.hoverItem ? pickerList.hoverItem.width : 0
                                        targetHeight: pickerList.hoverItem ? pickerList.hoverItem.height : 0
                                        radius: ThemeBackend.borderRadius
                                        tintColor: ThemeBackend.dockLauncherSurface0
                                        tintAmount: 0.3
                                        shadow: 0
                                        restPress: 0.2
                                        magnify: dockWindow.s(4)
                                        press: pickerList.hoverItem && pickerList.hoverItem.pressed ? 0.8 : 0
                                    }

                                    ScrollBar.vertical: ScrollBar {
                                        active: parent.moving || parent.movingVertically
                                        width: dockWindow.s(4)
                                        policy: ScrollBar.AsNeeded
                                        contentItem: Rectangle {
                                            implicitWidth: dockWindow.s(4)
                                            radius: dockWindow.s(2)
                                            color: ThemeBackend.dockLauncherSurface2
                                        }
                                    }

                                    delegate: Rectangle {
                                        id: pickerDelegate
                                        required property var modelData
                                        required property int index

                                        width: pickerList.width - dockWindow.s(6)
                                        height: dockWindow.s(44)
                                        radius: ThemeBackend.borderRadius
                                        readonly property var resolvedItem: (modelData !== undefined && modelData) ? modelData : model
                                        readonly property bool isAdded: (resolvedItem && resolvedItem.id) ? dockWindow.isAppInDock(resolvedItem.id) : false

                                        readonly property bool pressed: pickerMa.pressed
                                        readonly property bool lensWanted: pickerMa.containsMouse

                                        onLensWantedChanged: {
                                            if (lensWanted) pickerList.hoverItem = pickerDelegate;
                                            else if (pickerList.hoverItem === pickerDelegate) pickerList.hoverItem = null;
                                        }
                                        Component.onDestruction: {
                                            if (pickerList.hoverItem === pickerDelegate) pickerList.hoverItem = null;
                                        }

                                        readonly property color contentOnAdded: pickerRowGlass.active ? ThemeBackend.dockLauncherText : ThemeBackend.dockLauncherOnAccent

                                        color: pickerRowGlass.active ? "transparent" : (isAdded ? ThemeBackend.dockLauncherAccent : ((pickerMa.containsMouse && !pickerLens.glassActive) ? Qt.alpha(ThemeBackend.dockLauncherSurface1, 0.4) : Qt.alpha(ThemeBackend.dockLauncherSurface0, 0.25)))
                                        border.width: 0

                                        Behavior on color {
                                            enabled: dockWindow.visualContentActive
                                            ColorAnimation { duration: 150 }
                                        }

                                        GlassFill {
                                            id: pickerRowGlass
                                            anchors.fill: parent
                                            radius: parent.radius
                                            allowed: pickerDelegate.isAdded || (pickerMa.containsMouse && !pickerLens.glassActive)
                                            forceClear: pickerDelegate.isAdded
                                            tintColor: pickerDelegate.isAdded ? ThemeBackend.dockLauncherText : ThemeBackend.dockLauncherSurface0
                                            tintAmount: pickerDelegate.isAdded ? 0.1 : 0.3
                                            tintStrength: pickerDelegate.isAdded ? 1.0 : 0.5
                                            press: pickerMa.pressed ? 0.8 : (pickerDelegate.isAdded ? 0.6 : 0.2)
                                        }

                                        RowLayout {
                                            anchors.fill: parent
                                            anchors.leftMargin: dockWindow.s(8)
                                            anchors.rightMargin: dockWindow.s(8)
                                            spacing: dockWindow.s(8)

                                            Rectangle {
                                                implicitWidth: dockWindow.s(28)
                                                implicitHeight: dockWindow.s(28)
                                                Layout.alignment: Qt.AlignVCenter
                                                radius: Math.round(dockWindow.s(28) * 0.28)
                                                color: pickerIconGlass.active ? "transparent" : (pickerDelegate.isAdded ? Qt.tint(ThemeBackend.dockLauncherSurface0, Qt.rgba(ThemeBackend.dockLauncherAccent.r, ThemeBackend.dockLauncherAccent.g, ThemeBackend.dockLauncherAccent.b, 0.2)) : ThemeBackend.dockLauncherSurface0)
                                                clip: true

                                                GlassFill {
                                                    id: pickerIconGlass
                                                    anchors.fill: parent
                                                    radius: parent.radius
                                                    forceClear: pickerDelegate.isAdded
                                                    tintColor: pickerDelegate.isAdded ? ThemeBackend.dockLauncherText : ThemeBackend.dockLauncherSurface2
                                                    tintAmount: pickerDelegate.isAdded ? 0.1 : 0.3
                                                }

                                                Image {
                                                    id: pIcon
                                                    anchors.fill: parent
                                                    anchors.margins: dockWindow.s(3)
                                                    sourceSize: Qt.size(
                                                        Math.ceil(width * (dockWindow.screen?.devicePixelRatio ?? 1)),
                                                        Math.ceil(height * (dockWindow.screen?.devicePixelRatio ?? 1)))
                                                    fillMode: Image.PreserveAspectFit
                                                    asynchronous: true
                                                    smooth: true
                                                    mipmap: true
                                                    property bool failedLoad: false

                                                    visible: source !== "" && status === Image.Ready && !failedLoad

                                                    source: {
                                                        let ic = pickerDelegate.resolvedItem.icon || "";
                                                        if (!ic) return "";
                                                        if (ic.startsWith("file://") || ic.startsWith("image://") || ic.startsWith("http://") || ic.startsWith("https://")) return ic;
                                                        return ic.startsWith("/") ? "file://" + ic : "image://icon/" + ic;
                                                    }

                                                    onStatusChanged: {
                                                        if (status === Image.Error) failedLoad = true;
                                                    }
                                                }

                                                Text {
                                                    anchors.centerIn: parent
                                                    visible: pIcon.source === "" || pIcon.failedLoad || pIcon.status === Image.Error
                                                    text: pickerDelegate.resolvedItem.name ? pickerDelegate.resolvedItem.name.charAt(0).toUpperCase() : "?"
                                                    font.family: ThemeBackend.fontFamily
                                                    font.pixelSize: dockWindow.s(12)
                                                    font.bold: true
                                                    color: pickerDelegate.isAdded ? (pickerRowGlass.active ? ThemeBackend.dockLauncherText : ThemeBackend.dockLauncherAccent) : ThemeBackend.dockLauncherText
                                                }
                                            }

                                            ColumnLayout {
                                                Layout.fillWidth: true
                                                Layout.alignment: Qt.AlignVCenter
                                                spacing: dockWindow.s(1)

                                                Text {
                                                    text: pickerDelegate.resolvedItem.name || pickerDelegate.resolvedItem.id || ""
                                                    font.family: ThemeBackend.fontFamily
                                                    font.pixelSize: dockWindow.s(11)
                                                    font.weight: pickerDelegate.isAdded ? Font.Bold : Font.Medium
                                                    color: pickerDelegate.isAdded ? pickerDelegate.contentOnAdded : ThemeBackend.dockLauncherText
                                                    elide: Text.ElideRight
                                                    Layout.fillWidth: true
                                                    Behavior on color {
                                                        enabled: dockWindow.visualContentActive
                                                        ColorAnimation { duration: 150 }
                                                    }
                                                }

                                                Text {
                                                    text: pickerDelegate.resolvedItem.comment || pickerDelegate.resolvedItem.id || ""
                                                    font.family: ThemeBackend.fontFamily
                                                    font.pixelSize: dockWindow.s(9)
                                                    color: pickerDelegate.isAdded ? pickerDelegate.contentOnAdded : ThemeBackend.dockLauncherSubtext0
                                                    opacity: pickerDelegate.isAdded ? 0.9 : 0.85
                                                    elide: Text.ElideRight
                                                    Layout.fillWidth: true
                                                    Behavior on color {
                                                        enabled: dockWindow.visualContentActive
                                                        ColorAnimation { duration: 150 }
                                                    }
                                                }
                                            }
                                        }

                                        MouseArea {
                                            id: pickerMa
                                            anchors.fill: parent
                                            hoverEnabled: true
                                            cursorShape: Qt.PointingHandCursor
                                            onClicked: {
                                                if (typeof Sounds !== "undefined") {
                                                    Sounds.playSfx("reusables/iconbutton/click.wav");
                                                }
                                                if (pickerDelegate.isAdded) {
                                                    dockWindow.removeAppByDesktopId(pickerDelegate.resolvedItem.id);
                                                } else {
                                                    dockWindow.addApp(pickerDelegate.resolvedItem);
                                                }
                                            }
                                        }
                                    }

                                    Item {
                                        anchors.fill: parent
                                        visible: pickerFilteredModel.count === 0

                                        Text {
                                            anchors.centerIn: parent
                                            text: I18n.t("guide.dock.picker.empty", "No matching applications")
                                            font.family: ThemeBackend.fontFamily
                                            font.pixelSize: dockWindow.s(11)
                                            color: ThemeBackend.dockLauncherSubtext0
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}
