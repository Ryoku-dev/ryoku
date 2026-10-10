import stage
import stage.modules.common
import stage.modules.common.functions
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
pragma Singleton
pragma ComponentBehavior: Bound

/**
 * Per-screen framing for the wallpaper each mounted desktop is painting.
 *
 * Ryogami owns wallpaper selection and its output and workspace assignments.
 * This service keeps only framing records keyed by monitor and wallpaper path;
 * workspaces that use the same picture therefore share one Stage scene and the
 * mounted Backdrop always supplies the path currently painted on that output.
 *
 * Colours still come from one picture because matugen reads the shared
 * `background.wallpaperPath`. Screens painting that path identify the colour
 * source, while choosing another colour source updates Ryogami without moving
 * any picture between outputs.
 *
 * A screen keeps one framing per picture it has shown (a few of them, most
 * recent first), so returning to a wallpaper restores the way it was left.
 * While the catalogue is open, gestures preview from the live values below;
 * only release writes one history entry.
 *
 * Framing is unavailable while another process owns the wallpaper layer
 * because the shell has no plane to transform.
 */
Singleton {
    id: root

    // How many pictures a screen remembers a framing for.
    readonly property int framingMemory: 12

    // Mounted painters publish both ownership and provider path. The latter is
    // authoritative even while outputs.json is between daemon revisions.
    property var painters: ({})
    readonly property bool hasMountedPainter: Object.keys(root.painters).length > 0
    readonly property bool externalPainterActive: Object.keys(root.painters)
        .some(name => root.painters[name]?.external === true)
    readonly property bool available: Config.ready && (root.hasMountedPainter
        ? !root.externalPainterActive : !Wallpapers.videoWallpaperActive)
    readonly property var screenNames: Array.from(Quickshell.screens).map(screen => screen.name)
    readonly property bool multiScreen: root.screenNames.length > 1

    readonly property var entries: Config.ready
        ? Array.from(Config.options.background.monitorWallpapers ?? []).filter(entry => entry && entry.monitor)
        : []

    // Which physical monitor a screen is, beyond its connector: the same
    // monitor on another port (a dock, a different cable) keeps its wallpaper
    // and framings, and another monitor on the same port does not inherit
    // them. "" when the screen reports no model or serial, or when two
    // connected screens report the same one (twin monitors without serials):
    // those fall back to the connector name.
    readonly property var screenKeys: {
        const screens = Array.from(Quickshell.screens);
        const raw = screens.map(screen => {
            const model = String(screen.model ?? "").trim();
            const serial = String(screen.serialNumber ?? "").trim();
            return model === "" && serial === "" ? "" : model + "/" + serial;
        });
        const keys = {};
        screens.forEach((screen, i) => {
            const key = raw[i];
            keys[screen.name] = key !== "" && raw.filter(other => other === key).length === 1 ? key : "";
        });
        return keys;
    }

    function keyFor(name) {
        return root.screenKeys[name] ?? "";
    }

    function registerPainter(name, external, path) {
        if (!name)
            return;
        const next = Object.assign({}, root.painters);
        next[name] = {
            "external": external === true,
            "path": root.cleanPath(path)
        };
        root.painters = next;
    }

    function unregisterPainter(name) {
        if (!name || !(name in root.painters))
            return;
        const next = Object.assign({}, root.painters);
        delete next[name];
        root.painters = next;
    }

    // The entry a screen reads and writes: its monitor's own first, then one
    // stored under its connector - an entry from before monitors were told
    // apart, or one for a screen that cannot be told apart now - unless that
    // entry belongs to another monitor.
    function matchEntry(list, name) {
        const key = root.keyFor(name);
        if (key !== "") {
            const own = list.find(entry => String(entry.key ?? "") === key);
            if (own)
                return own;
        }
        return list.find(entry => entry.monitor === name
            && (String(entry.key ?? "") === "" || key === "")) ?? null;
    }

    function cleanPath(path) {
        return FileUtils.trimFileProtocol(String(path ?? ""));
    }

    function isImagePath(path) {
        const clean = root.cleanPath(path).toLowerCase();
        return clean !== "" && Images.isValidImageByName(clean) && !Wallpapers.isVideoFile(clean);
    }

    // The picture the colours come from, as the screens that show the shared
    // wallpaper display it: the light-mode one in light mode when there is a
    // separate one, else the desktop's, else the shipped default.
    readonly property string sharedSourcePath: {
        if (Config.widgetProvider) {
            const mounted = root.cleanPath(Config.wallpaperPath);
            if (mounted !== "")
                return mounted;
        }
        const background = Config.options?.background;
        if (!background || background.useWallpaperEngine)
            return "";
        const light = root.cleanPath(background.lightModeWallpaperPath);
        if (!Appearance.m3colors.darkmode && background.useSeparateLightModeWallpaper && light !== "")
            return light;
        const path = root.cleanPath(background.wallpaperPath);
        return path !== "" ? path : root.cleanPath(Directories.defaultWallpaperImagePath);
    }
    readonly property bool sharedIsVideo: Wallpapers.isVideoFile(root.sharedSourcePath)

    function entryFor(name) {
        return root.matchEntry(root.entries, name);
    }

    // The mounted provider frame is the path actually painted on this output.
    // outputs.json is the startup fallback before Backdrop registers.
    function sourcePathFor(name) {
        const mounted = root.cleanPath(root.painters[name]?.path);
        return mounted !== "" ? mounted : root.cleanPath(Wallpapers.currentWallpaperPath(name));
    }

    // "Own" is presentation vocabulary: this output differs from the picture
    // that feeds the shared palette. Ryogami remains the only owner of paths.
    function ownPathFor(name) {
        if (!root.available || !name)
            return "";
        const path = root.sourcePathFor(name);
        return path !== root.sharedSourcePath ? path : "";
    }

    function hasOwn(name) {
        return root.ownPathFor(name) !== "";
    }

    // Empty when no output currently paints the palette's source picture.
    readonly property var colourScreens: root.screenNames
        .filter(name => root.sourcePathFor(name) === root.sharedSourcePath)
    readonly property string colourScreen: root.colourScreens.length > 0 ? root.colourScreens[0] : ""
    // Whether more than one picture is on screen, which is when "which one do
    // the colours come from" is a question at all.
    readonly property bool distinctWallpapers: {
        const shown = new Set(root.screenNames.map(name => root.sourcePathFor(name)));
        return shown.size > 1;
    }

    // A screen may only go its own way while another screen keeps showing the
    // colour wallpaper: otherwise the palette would come from a picture that is
    // on no screen at all.
    function canDetach(name) {
        if (!root.available || !root.multiScreen || root.hasOwn(name))
            return false;
        return root.screenNames.some(other => other !== name && root.ownPathFor(other) === "");
    }

    // ── Framing ──────────────────────────────────────────────────────────────

    function savedFramingFor(name, path) {
        const clean = root.cleanPath(path);
        if (!root.available || !name || clean === "")
            return WallpaperFraming.defaults();
        const hit = (root.entryFor(name)?.framings ?? []).find(f => f && root.cleanPath(f.path) === clean);
        return hit ? WallpaperFraming.normalize(hit) : WallpaperFraming.defaults();
    }

    function isLive(name, path) {
        return root.liveScreen !== "" && root.liveScreen === name && root.livePath === root.cleanPath(path);
    }

    // What the screen draws right now: the live preview while the overlay is
    // on it, the stored framing otherwise.
    function framingFor(name, path) {
        if (root.isLive(name, path))
            return root.liveFraming;
        return root.savedFramingFor(name, path);
    }

    // The angle to draw at: continuous while a quarter turn animates.
    function angleFor(name, path) {
        if (root.isLive(name, path))
            return root.liveAngle;
        return root.savedFramingFor(name, path).rotation;
    }

    // The wallpaper plane's geometry, published by each screen's wallpaper
    // surface: the plane a framed picture is laid out in (the screen at the
    // workspace zoom) and the picture's own pixel size. The overlay and the
    // panel's buttons need it to turn a gesture into a position.
    property var geometry: ({})

    function publishGeometry(name, planeWidth, planeHeight, imageWidth, imageHeight) {
        if (!name)
            return;
        const current = root.geometry[name];
        if (current && current.planeWidth === planeWidth && current.planeHeight === planeHeight
                && current.imageWidth === imageWidth && current.imageHeight === imageHeight)
            return;
        const next = Object.assign({}, root.geometry);
        next[name] = {
            "planeWidth": planeWidth,
            "planeHeight": planeHeight,
            "imageWidth": imageWidth,
            "imageHeight": imageHeight
        };
        root.geometry = next;
    }

    function geometryFor(name) {
        const g = root.geometry[name];
        return g && g.planeWidth > 0 && g.planeHeight > 0 && g.imageWidth > 0 && g.imageHeight > 0 ? g : null;
    }

    // ── The live preview ─────────────────────────────────────────────────────
    // On while Edit Mode's Wallpaper catalogue is open on the Desktop tab, for
    // the screen the mode is on.
    readonly property string framingScreen: (root.available && GlobalStates.editMode && GlobalStates.editDrawerOpen
            && GlobalStates.editDrawerSection === "wallpaper" && !GlobalStates.editLockPreview)
        ? GlobalStates.editModeMonitor : ""
    readonly property string framingPath: root.framingScreen !== "" ? root.sourcePathFor(root.framingScreen) : ""

    property string liveScreen: ""
    property string livePath: ""
    property real liveZoom: 1
    property real liveX: 0
    property real liveY: 0
    property real liveAngle: 0
    property bool liveFlipH: false
    property bool liveFlipV: false
    // A gesture owns the live values: a config change arriving meanwhile (the
    // release's own write) must not animate them out from under the pointer.
    property bool interacting: false
    // A local write may pass through an empty adapter snapshot before the
    // replacement list is readable. Keep painting the live record until the
    // store publishes the exact record that was committed.
    property var pendingWrite: null
    Timer {
        id: pendingWriteTimer
        interval: 900
        onTriggered: root.pendingWrite = null
    }

    readonly property var liveFraming: ({
        "zoom": root.liveZoom,
        "x": root.liveX,
        "y": root.liveY,
        "rotation": WallpaperFraming.snapRotation(root.liveAngle),
        "flipH": root.liveFlipH,
        "flipV": root.liveFlipV
    })

    onFramingScreenChanged: root.restartLive()
    // Ctrl+Z inside the wheel's commit window: the steps are written first,
    // so the undo takes them back rather than being written over by them.
    Connections {
        target: GlobalStates
        function onEditHistoryWillReplay() {
            root.flushGesture();
            pendingWriteTimer.stop();
            root.pendingWrite = null;
        }
    }

    onFramingPathChanged: {
        root.restartLive();
        root.requestPointOfInterest(root.framingPath);
    }

    // ── Point of interest ────────────────────────────────────────────────────
    // Where the eye goes in the picture being framed - a face, else its most
    // salient detail (scripts/images/point_of_interest.py) - so a drag can
    // snap that point, not just the picture's centre, onto the screen's
    // centre and thirds. Asked once per picture per session, only while the
    // overlay is up; { x, y, kind } in the picture's 0..1 coordinates, null
    // while unknown or when there is no answer.
    property var pointsOfInterest: ({})

    function pointOfInterestFor(path) {
        return root.pointsOfInterest[root.cleanPath(path)] ?? null;
    }

    property string _poiPending: ""
    function requestPointOfInterest(path) {
        const clean = root.cleanPath(path);
        if (clean === "" || !root.isImagePath(clean) || clean in root.pointsOfInterest)
            return;
        if (poiProcess.running) {
            root._poiPending = clean;
            return;
        }
        poiProcess.path = clean;
        poiProcess.running = true;
    }

    Process {
        id: poiProcess
        property string path: ""
        command: [FileUtils.trimFileProtocol(Quickshell.shellPath("scripts/images/point-of-interest-venv.sh")), poiProcess.path]
        stdout: StdioCollector {
            onStreamFinished: {
                let found = null;
                try {
                    const parsed = JSON.parse(this.text.trim() || "{}");
                    if (isFinite(parsed.x) && isFinite(parsed.y))
                        found = {
                            "x": Math.max(0, Math.min(1, Number(parsed.x))),
                            "y": Math.max(0, Math.min(1, Number(parsed.y))),
                            "kind": parsed.kind === "face" ? "face" : "detail"
                        };
                } catch (e) {
                    found = null;
                }
                const next = Object.assign({}, root.pointsOfInterest);
                next[poiProcess.path] = found;
                root.pointsOfInterest = next;
            }
        }
        onExited: {
            const pending = root._poiPending;
            root._poiPending = "";
            if (pending !== "")
                Qt.callLater(() => root.requestPointOfInterest(pending));
        }
    }

    function setLive(framing, angle) {
        const f = WallpaperFraming.normalize(framing);
        root.liveZoom = f.zoom;
        root.liveX = f.x;
        root.liveY = f.y;
        root.liveAngle = angle === undefined ? f.rotation : angle;
        root.liveFlipH = f.flipH;
        root.liveFlipV = f.flipV;
    }

    function restartLive() {
        // A run of wheel steps still waiting to be written belongs to the
        // screen and picture it was made on: written before either changes.
        root.flushGesture();
        liveAnimation.stop();
        root.interacting = false;
        if (root.framingScreen === "" || root.framingPath === "") {
            root.liveScreen = "";
            root.livePath = "";
            return;
        }
        root.setLive(root.savedFramingFor(root.framingScreen, root.framingPath));
        // Path first: with the screen set and the path stale, isLive() would
        // briefly answer for the wrong picture.
        root.livePath = root.framingPath;
        root.liveScreen = root.framingScreen;
    }

    // Undo, redo and the panel's buttons write the config; the preview follows
    // with an animation rather than a jump.
    Connections {
        target: Config.ready ? Config.options.background : null
        function onMonitorWallpapersChanged() {
            // JsonAdapter can publish an empty intermediate list while replacing
            // it. A local commit already has the authoritative pixels on screen;
            // ignore store echoes until that exact record becomes readable.
            Qt.callLater(() => {
                if (root.liveScreen === "" || root.interacting)
                    return;
                const saved = root.savedFramingFor(root.liveScreen, root.livePath);
                const pending = root.pendingWrite;
                if (pending && pending.screen === root.liveScreen && pending.path === root.livePath) {
                    if (WallpaperFraming.equal(saved, pending.framing)) {
                        pendingWriteTimer.stop();
                        root.pendingWrite = null;
                    }
                    return;
                }
                root.animateLiveTo(saved, false);
            });
        }
    }

    function animateLiveTo(framing, fast) {
        const target = WallpaperFraming.normalize(framing);
        liveAnimation.stop();
        const flipped = target.flipH !== root.liveFlipH || target.flipV !== root.liveFlipV;
        // A mirror cannot be animated without the picture passing through zero
        // width, which shows the plane behind it - so it lands at once, and so
        // does the offset it mirrors, or the picture would slide after jumping.
        if (flipped) {
            root.liveFlipH = target.flipH;
            root.liveFlipV = target.flipV;
            root.liveX = target.x;
            root.liveY = target.y;
        }
        const angle = WallpaperFraming.animationTarget(root.liveAngle, target.rotation);
        if (Appearance.reducedMotion) {
            root.setLive(target);
            return;
        }
        liveAnimation.fast = fast === true;
        zoomAnim.to = target.zoom;
        xAnim.to = target.x;
        yAnim.to = target.y;
        angleAnim.to = angle;
        liveAnimation.start();
    }

    ParallelAnimation {
        id: liveAnimation
        property bool fast: false
        readonly property var tier: liveAnimation.fast ? Appearance.animation.elementMoveFast : Appearance.animation.elementMove

        NumberAnimation {
            id: zoomAnim
            target: root
            property: "liveZoom"
            duration: liveAnimation.tier.duration
            easing.type: liveAnimation.tier.type
            easing.bezierCurve: liveAnimation.tier.bezierCurve
        }
        NumberAnimation {
            id: xAnim
            target: root
            property: "liveX"
            duration: liveAnimation.tier.duration
            easing.type: liveAnimation.tier.type
            easing.bezierCurve: liveAnimation.tier.bezierCurve
        }
        NumberAnimation {
            id: yAnim
            target: root
            property: "liveY"
            duration: liveAnimation.tier.duration
            easing.type: liveAnimation.tier.type
            easing.bezierCurve: liveAnimation.tier.bezierCurve
        }
        NumberAnimation {
            id: angleAnim
            target: root
            property: "liveAngle"
            duration: liveAnimation.tier.duration
            easing.type: liveAnimation.tier.type
            easing.bezierCurve: liveAnimation.tier.bezierCurve
        }

        // Keep the angle in one turn: 270 -> 0 animates to 360, which is the
        // same picture and is folded back here.
        onFinished: root.liveAngle = WallpaperFraming.snapRotation(root.liveAngle)
    }

    // ── Gestures (the overlay) ───────────────────────────────────────────────
    // Finish at the destination, not at the in-between frame where the next
    // pointer gesture happened to interrupt the landing animation.
    function settleLive() {
        if (!liveAnimation.running)
            return;
        const target = WallpaperFraming.normalize({
            "zoom": zoomAnim.to,
            "x": xAnim.to,
            "y": yAnim.to,
            "rotation": angleAnim.to,
            "flipH": root.liveFlipH,
            "flipV": root.liveFlipV
        });
        liveAnimation.stop();
        root.setLive(target, target.rotation);
    }

    function beginGesture() {
        if (root.liveScreen === "")
            return;
        root.settleLive();
        commitTimer.stop();
        root.liveAngle = WallpaperFraming.snapRotation(root.liveAngle);
        root.interacting = true;
    }

    function updateGesture(framing) {
        if (root.liveScreen === "")
            return;
        const f = WallpaperFraming.normalize(framing);
        root.liveZoom = f.zoom;
        root.liveX = f.x;
        root.liveY = f.y;
    }

    function endGesture() {
        if (root.liveScreen === "")
            return;
        root.interacting = false;
        root.commitFraming(root.liveScreen, root.livePath, root.liveFraming);
    }

    // A wheel or a pinch step: animated to, and written once the steps stop,
    // so a spin of the wheel is one history entry rather than twenty.
    function stepGesture(framing) {
        if (root.liveScreen === "")
            return;
        root.interacting = true;
        root.animateLiveTo(framing, true);
        commitTimer.restart();
    }

    // A touchpad scroll's pan: lands at once (the fingers are already a
    // continuous motion) and is written once they stop, like the wheel.
    function nudgeGesture(framing) {
        if (root.liveScreen === "")
            return;
        root.interacting = true;
        root.settleLive();
        const f = WallpaperFraming.normalize(framing);
        root.liveZoom = f.zoom;
        root.liveX = f.x;
        root.liveY = f.y;
        commitTimer.restart();
    }

    // Where a run of steps is heading: the next step builds on it rather than
    // on a value still animating towards it.
    function gestureTarget() {
        if (!liveAnimation.running)
            return root.liveFraming;
        return WallpaperFraming.normalize({
            "zoom": zoomAnim.to,
            "x": xAnim.to,
            "y": yAnim.to,
            "rotation": angleAnim.to,
            "flipH": root.liveFlipH,
            "flipV": root.liveFlipV
        });
    }

    // A single-shot Timer no longer reports `running` inside its own
    // onTriggered, so the expiry commits directly, not through flushGesture().
    Timer {
        id: commitTimer
        interval: 450
        repeat: false
        onTriggered: root.commitSteps()
    }

    // Writes a run of steps now instead of when the timer would have: before
    // a button builds on the framing (or it would build on the stored one and
    // the timer would then write the steps over the button's change), and
    // before the preview moves to another screen or picture.
    function flushGesture() {
        if (!commitTimer.running)
            return;
        commitTimer.stop();
        root.commitSteps();
    }

    function commitSteps() {
        root.interacting = false;
        if (root.liveScreen === "")
            return;
        root.commitFraming(root.liveScreen, root.livePath, root.gestureTarget());
    }

    // ── Writes ───────────────────────────────────────────────────────────────
    // Every write replaces the whole list (a `list<var>` only notifies on
    // reassignment) and records the pair for Edit Mode's history. Outside the
    // mode the history ignores the push, which is the same contract every
    // other store the mode edits keeps.

    function listCopy() {
        return root.entries.map(entry => ({
            "monitor": String(entry.monitor),
            "key": String(entry.key ?? ""),
            "framings": Array.from(entry.framings ?? [])
                .filter(f => f && root.cleanPath(f.path) !== "")
                .map(f => Object.assign({ "path": root.cleanPath(f.path) }, WallpaperFraming.normalize(f)))
        }));
    }

    // Old builds mixed provider state into the framing list. Drop only that
    // field so upgrades retain every monitor's crop history.
    function removeLegacyWallpaperPaths() {
        if (!Config.ready)
            return;
        const stored = Array.from(Config.options.background.monitorWallpapers ?? []);
        if (stored.some(entry => entry && "path" in entry))
            Config.options.background.monitorWallpapers = root.listCopy()
                .filter(entry => entry.framings.length > 0);
    }
    Component.onCompleted: Qt.callLater(root.removeLegacyWallpaperPaths)
    Connections {
        target: Config
        function onReadyChanged() {
            if (Config.ready)
                root.removeLegacyWallpaperPaths();
        }
    }

    // The screen's entry in a copy of the list, created when it has none,
    // and stamped with the connector and monitor it is on now.
    function entryIn(list, name) {
        const key = root.keyFor(name);
        let entry = root.matchEntry(list, name);
        if (!entry) {
            entry = { "monitor": name, "key": key, "framings": [] };
            list.push(entry);
        }
        entry.monitor = name;
        if (key !== "")
            entry.key = key;
        return entry;
    }

    function writeList(next) {
        const before = root.listCopy();
        // Identity-only entries carry no state worth persisting.
        const after = next.filter(entry => entry.framings.length > 0);
        if (JSON.stringify(before) === JSON.stringify(after))
            return false;
        Config.options.background.monitorWallpapers = after;
        GlobalStates.editHistoryPush({
            "undo": () => {
                Config.options.background.monitorWallpapers = before;
            },
            "redo": () => {
                Config.options.background.monitorWallpapers = after;
            }
        });
        return true;
    }

    // A framing for one picture into an entry of a list copy: most recent
    // first, the identity not kept.
    function setFramingIn(entry, path, framing) {
        const clean = root.cleanPath(path);
        const f = WallpaperFraming.normalize(framing);
        entry.framings = entry.framings.filter(item => item.path !== clean);
        if (!WallpaperFraming.isIdentity(f))
            entry.framings.unshift(Object.assign({ "path": clean }, f));
        entry.framings = entry.framings.slice(0, root.framingMemory);
    }

    function commitFraming(name, path, framing) {
        const clean = root.cleanPath(path);
        if (!root.available || !name || clean === "")
            return;
        const next = WallpaperFraming.normalize(framing);
        const live = root.isLive(name, clean);
        // Button actions update the mounted painter synchronously. Gesture
        // releases are already at `next`; neither waits for the watched store.
        if (live && !root.interacting && !WallpaperFraming.equal(root.gestureTarget(), next))
            root.animateLiveTo(next, false);
        if (live)
            root.pendingWrite = { "screen": name, "path": clean, "framing": next };
        const list = root.listCopy();
        root.setFramingIn(root.entryIn(list, name), clean, next);
        const wrote = root.writeList(list);
        if (live) {
            if (wrote)
                pendingWriteTimer.restart();
            else
                root.pendingWrite = null;
        }
    }

    // The framing a button starts from: the gesture's destination while one
    // is landing, so two quick clicks add up instead of the second undoing the
    // first.
    function currentFraming(name) {
        const path = root.sourcePathFor(name);
        if (root.isLive(name, path))
            return root.gestureTarget();
        return root.savedFramingFor(name, path);
    }

    function setZoom(name, zoom) {
        root.flushGesture();
        const current = root.currentFraming(name);
        const g = root.geometryFor(name);
        const next = g
            ? WallpaperFraming.zoomAt(g.planeWidth, g.planeHeight, g.imageWidth, g.imageHeight, current, zoom, 0, 0)
            : Object.assign({}, current, { "zoom": zoom });
        root.commitFraming(name, root.sourcePathFor(name), next);
    }

    function rotate(name, turns) {
        root.flushGesture();
        const current = root.currentFraming(name);
        const g = root.geometryFor(name);
        const next = g
            ? WallpaperFraming.rotateBy(g.planeWidth, g.planeHeight, g.imageWidth, g.imageHeight, current, turns)
            : Object.assign({}, current, { "rotation": WallpaperFraming.snapRotation(current.rotation + turns * 90) });
        root.commitFraming(name, root.sourcePathFor(name), next);
    }

    function flip(name, axis) {
        root.flushGesture();
        root.commitFraming(name, root.sourcePathFor(name), WallpaperFraming.flip(root.currentFraming(name), axis));
    }

    function centre(name) {
        root.flushGesture();
        const next = Object.assign({}, root.currentFraming(name), { "x": 0, "y": 0 });
        root.commitFraming(name, root.sourcePathFor(name), next);
    }

    function resetFraming(name) {
        root.flushGesture();
        root.commitFraming(name, root.sourcePathFor(name), WallpaperFraming.defaults());
    }

    // Picks always enter Ryoku through the per-output seam; the daemon then
    // publishes the frame that sourcePathFor observes from the mounted painter.
    function setScreenWallpaper(name, path) {
        const clean = root.cleanPath(path);
        if (!name || clean === "")
            return false;
        return Wallpapers.selectForScreen(clean, name);
    }
    function activeWorkspaceFor(name) {
        const workspaces = Wm.workspaces ?? [];
        for (let i = 0; i < workspaces.length; ++i) {
            const workspace = workspaces[i];
            if (workspace.active && workspace.output === name)
                return workspace;
        }
        return null;
    }

    function workspaceLabelFor(name) {
        const workspace = root.activeWorkspaceFor(name);
        if (!workspace)
            return "";
        return String(workspace.name || workspace.id || "");
    }

    function assignCurrentToWorkspace(name) {
        if (!name || !root.activeWorkspaceFor(name))
            return false;
        Spawn.run(["ryogami", "wallpaper", "assign", "--screen", name]);
        return true;
    }

    function clearWorkspaceWallpaper(name) {
        if (!name || !root.activeWorkspaceFor(name))
            return false;
        Spawn.run(["ryogami", "wallpaper", "unassign", "--screen", name]);
        return true;
    }



    // Ryogami still owns the write; matching the palette source makes the
    // screen shared again without restoring an island-side path.
    function attach(name) {
        if (!root.hasOwn(name))
            return false;
        return Wallpapers.selectForScreen(root.sharedSourcePath, name);
    }

    // Changing the palette source must not move wallpapers between outputs.
    // Snapshot every painted path before the shared source changes, then put
    // those paths back through Ryogami's per-output seam.
    function canMakeColourSource(name) {
        return root.hasOwn(name) && root.isImagePath(root.sourcePathFor(name));
    }

    function makeColourSource(name) {
        if (!root.canMakeColourSource(name))
            return false;
        const picture = root.sourcePathFor(name);
        const shown = {};
        for (const screen of root.screenNames)
            shown[screen] = root.sourcePathFor(screen);
        GlobalStates.editHistoryBeginBatch();
        Wallpapers.select(picture);
        for (const screen of root.screenNames)
            Wallpapers.selectForScreen(shown[screen], screen, Appearance.m3colors.darkmode, true);
        GlobalStates.editHistoryEndBatch();
        return true;
    }

    // A random pick is still output-scoped; images and videos are both valid
    // because the provider frame carries the matching still/live fields.
    function randomForScreen(name) {
        const model = Wallpapers.folderModel;
        const current = root.sourcePathFor(name);
        const candidates = [];
        for (let i = 0; i < model.count; i++) {
            if (Boolean(model.get(i, "fileIsDir")))
                continue;
            const path = root.cleanPath(model.get(i, "filePath"));
            const lower = path.toLowerCase();
            if (path === "" || path === current
                    || !Wallpapers.extensions.some(ext => lower.endsWith("." + ext)))
                continue;
            candidates.push(path);
        }
        if (candidates.length === 0)
            return;
        root.setScreenWallpaper(name, candidates[Math.floor(Math.random() * candidates.length)]);
    }

    // ── Screens, by name and by place ────────────────────────────────────────

    // A screen as a person knows it: the panel of a laptop, a monitor's model,
    // its maker - the connector only when nothing better is known, and added
    // when two screens would otherwise read the same.
    function baseDisplayName(name) {
        if (/^(eDP|LVDS|DSI)/.test(name))
            return Translation.tr("Built-in display");
        const monitor = HyprlandData.monitors.find(m => m?.name === name);
        const model = String(monitor?.model ?? "").trim();
        if (model !== "" && !/^0x[0-9a-f]+$/i.test(model))
            return model;
        const make = String(monitor?.make ?? "").trim();
        return make !== "" ? make : name;
    }

    function displayName(name) {
        const base = root.baseDisplayName(name);
        if (base === name)
            return name;
        const twin = root.screenNames.some(other => other !== name && root.baseDisplayName(other) === base);
        return twin ? base + " · " + name : base;
    }

    // The screens where they stand (logical pixels), and the box around them.
    readonly property var screenRects: Array.from(Quickshell.screens).map(screen => ({
        "name": screen.name,
        "x": screen.x,
        "y": screen.y,
        "width": screen.width,
        "height": screen.height
    }))
    readonly property var screensBox: {
        const rects = root.screenRects;
        if (rects.length === 0)
            return { "x": 0, "y": 0, "width": 0, "height": 0 };
        const left = Math.min(...rects.map(r => r.x));
        const top = Math.min(...rects.map(r => r.y));
        const right = Math.max(...rects.map(r => r.x + r.width));
        const bottom = Math.max(...rects.map(r => r.y + r.height));
        return { "x": left, "y": top, "width": right - left, "height": bottom - top };
    }

    // ── Two screens at once ──────────────────────────────────────────────────

    function canSwap(a, b) {
        if (!root.available || !a || !b || a === b)
            return false;
        return root.sourcePathFor(a) !== root.sourcePathFor(b);
    }

    // The path exchange and both framing moves are one replayable edit.
    function swapWallpapers(a, b) {
        if (!root.canSwap(a, b))
            return false;
        root.flushGesture();
        const shownA = root.sourcePathFor(a);
        const shownB = root.sourcePathFor(b);
        const framingA = root.savedFramingFor(a, shownA);
        const framingB = root.savedFramingFor(b, shownB);
        const list = root.listCopy();
        root.setFramingIn(root.entryIn(list, a), shownB, framingB);
        root.setFramingIn(root.entryIn(list, b), shownA, framingA);
        GlobalStates.editHistoryBeginBatch();
        root.writeList(list);
        Wallpapers.selectForScreen(shownB, a);
        Wallpapers.selectForScreen(shownA, b);
        GlobalStates.editHistoryEndBatch();
        return true;
    }

    // One screen's framing on another, for whatever picture that one shows:
    // a framing is relative (zoom over the cover fit, position within the
    // room left), so it means the same on a screen of another size.
    function canCopyFraming(from, to) {
        return root.available && from !== "" && to !== "" && from !== to
            && !WallpaperFraming.equal(root.currentFraming(from), root.currentFraming(to));
    }

    function copyFraming(from, to) {
        if (!root.canCopyFraming(from, to))
            return;
        root.flushGesture();
        root.commitFraming(to, root.sourcePathFor(to), root.currentFraming(from));
    }

    // ── One picture across every screen ─────────────────────────────────────
    // The picture covers the box around all the screens, and each screen is
    // framed onto its own piece of it, so the picture runs on from one screen
    // to the next. Every screen shows the same file, so it is the shared one
    // (and the colours come from it). Monitor bezels and gaps in the layout
    // are not modelled: the layout's logical pixels are the picture's.

    // The framing that puts a screen on its piece of the spanned picture.
    // The plane is larger than the screen by the workspace zoom; the framing
    // compensates so the picture runs on at the seams with the parallax at
    // rest. null when the screen or the picture's size is unknown.
    function spanFramingFor(name, imageWidth, imageHeight) {
        const rect = root.screenRects.find(r => r.name === name);
        const box = root.screensBox;
        if (!rect || !(imageWidth > 0) || !(imageHeight > 0) || !(box.width > 0) || !(box.height > 0))
            return null;
        const g = root.geometryFor(name);
        const base = g && rect.width > 0 ? g.planeWidth / rect.width : 1;
        const planeWidth = rect.width * base;
        const planeHeight = rect.height * base;
        // Image pixels to screen pixels for the whole box.
        const boxScale = Math.max(box.width / imageWidth, box.height / imageHeight);
        const cover = Math.max(planeWidth / imageWidth, planeHeight / imageHeight);
        const zoom = boxScale / cover;
        // The picture's centre from the screen's centre.
        const cx = (box.x + box.width / 2) - (rect.x + rect.width / 2);
        const cy = (box.y + box.height / 2) - (rect.y + rect.height / 2);
        const framing = WallpaperFraming.normalize({ "zoom": zoom });
        const at = WallpaperFraming.layout(planeWidth, planeHeight, imageWidth, imageHeight, framing);
        framing.x = at.rangeX > 0.5 ? Math.max(-1, Math.min(1, -cx / at.rangeX)) : 0;
        framing.y = at.rangeY > 0.5 ? Math.max(-1, Math.min(1, -cy / at.rangeY)) : 0;
        return framing;
    }

    // The size of the picture `name` shows, from its surface's probe.
    function imageSizeFor(name) {
        const g = root.geometryFor(name);
        return g ? { "width": g.imageWidth, "height": g.imageHeight } : null;
    }

    function canSpanFrom(name) {
        return root.available && root.multiScreen && root.isImagePath(root.sourcePathFor(name))
            && root.imageSizeFor(name) !== null;
    }

    readonly property bool spanned: {
        if (!root.available || !root.multiScreen)
            return false;
        const picture = root.sourcePathFor(root.screenNames[0]);
        const size = root.imageSizeFor(root.screenNames[0]);
        if (picture === "" || size === null
                || root.screenNames.some(name => root.sourcePathFor(name) !== picture))
            return false;
        return root.screenNames.every(name => {
            const target = root.spanFramingFor(name, size.width, size.height);
            return target !== null && WallpaperFraming.equal(root.savedFramingFor(name, picture), target);
        });
    }

    function spanFrom(name) {
        if (!root.canSpanFrom(name))
            return false;
        root.flushGesture();
        const picture = root.sourcePathFor(name);
        const size = root.imageSizeFor(name);
        const list = root.listCopy();
        for (const other of root.screenNames) {
            const framing = root.spanFramingFor(other, size.width, size.height);
            if (framing !== null)
                root.setFramingIn(root.entryIn(list, other), picture, framing);
        }
        GlobalStates.editHistoryBeginBatch();
        root.writeList(list);
        Wallpapers.select(picture);
        for (const other of root.screenNames)
            Wallpapers.selectForScreen(picture, other, Appearance.m3colors.darkmode, true);
        GlobalStates.editHistoryEndBatch();
        return true;
    }

    // Each screen back to the whole picture.
    function unspan() {
        if (!root.spanned)
            return;
        root.flushGesture();
        const list = root.listCopy();
        for (const name of root.screenNames)
            root.setFramingIn(root.entryIn(list, name), root.sharedSourcePath, WallpaperFraming.defaults());
        root.writeList(list);
    }
}
