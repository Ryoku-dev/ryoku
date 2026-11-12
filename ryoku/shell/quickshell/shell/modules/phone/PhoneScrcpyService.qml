pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import shell.services as Shell

Singleton {
    id: root

    // Capabilities
    property bool available: false
    property string version: ""
    property int versionMajor: 0
    property int versionMinor: 0
    readonly property bool appModeSupported: available && versionMajor >= 3

    // Mirror Session
    property bool mirrorRunning: false
    property bool mirrorLaunching: false
    property int mirrorElapsedMs: 0
    property string mirrorLaunchError: ""


    // Apps Catalog
    property var apps: []
    property bool appsLoading: false
    property string appsError: ""
    property string appsSearchQuery: ""
    property var filteredApps: []

    // Active Sessions (Model: list of {id, type, package, title, pid, startedAt})
    property var sessions: []
    readonly property int sessionCount: sessions ? sessions.length : 0

    // ─── Phone screen recording ──────────────────────────────
    readonly property string recordSessionId: "record"
    property bool recordingRunning: false
    property bool recordingLaunching: false
    property int recordingElapsedMs: 0
    /** The file the running (or last) recording is written to. */
    property string recordingPath: ""
    property string recordingError: ""

    Timer {
        interval: 1000
        repeat: true
        running: root.recordingRunning
        onTriggered: root.recordingElapsedMs += 1000
    }

    // Elapsed timer for active sessions
    Timer {
        id: mirrorElapsedTimer
        interval: 1000
        repeat: true
        running: root.mirrorRunning
        onTriggered: root.mirrorElapsedMs += 1000
    }

    Connections {
        target: KdeConnectService
        ignoreUnknownSignals: true
        function onActiveDeviceIdChanged() {
            root.refreshCapabilities()
            root.refreshApps()
        }
        function onAdbReachableChanged() {
            if (KdeConnectService.adbReachable) {
                root.refreshApps()
            }
        }
    }

    Component.onCompleted: {
        if (PhoneState.runtimeReady) root.refreshCapabilities()
    }

    // The session manager only acts on commands written to its stdin: between them it sits in a
    // blocking read doing nothing. So it is started when there is a command to send and shut down
    // once it has been idle with no live session — never while one is running, since it owns those
    // scrcpy child processes and reports their exit.
    property bool _managerWanted: false
    readonly property bool _managerAllowed: PhoneState.runtimeReady && KdeConnectService.available

    function ensureManagerRunning(): void {
        if (!root._managerAllowed) return
        root._managerWanted = true
        managerIdleTimer.restart()
    }

    // Process.write() is thrown away while the child is still being spawned,
    // and `running` is already true by then — only processId/started tell the
    // two apart. So the very first command after the manager has idled out
    // used to vanish, which is precisely what a dock click is: the app list
    // gets away with it only because refreshApps() starts the manager seconds
    // before it sends anything.
    property var _pendingCommands: []

    function _send(payload): void {
        if (!root._managerAllowed) return
        root.ensureManagerRunning()
        const line = JSON.stringify(payload) + "\n"
        if (!sessionManagerProc.processId) {
            root._pendingCommands = root._pendingCommands.concat([line])
            return
        }
        sessionManagerProc.write(line)
    }

    Timer {
        id: managerIdleTimer
        interval: 10000
        repeat: false
        onTriggered: {
            if (root.sessionCount === 0 && !root.mirrorLaunching && !root.recordingLaunching && !root.appsLoading)
                root._managerWanted = false
        }
    }

    onSessionCountChanged: managerIdleTimer.restart()
    onAppsLoadingChanged: managerIdleTimer.restart()
    onMirrorLaunchingChanged: managerIdleTimer.restart()

    function refreshCapabilities(): void {
        if (!PhoneState.runtimeReady) return
        scrcpyVersionProc.running = false
        scrcpyVersionProc.running = true
    }
    function refresh(): void { root.refreshCapabilities() }

    function refreshApps(): void {
        if (!PhoneState.runtimeReady) return
        if (!appModeSupported) return
        root.appsLoading = true
        root.appsError = ""
        root.ensureManagerRunning()
        // Resolve the ADB target on demand: the phone's wireless-debugging
        // port can change between two polls of the 30s prober, and a stale
        // one silently lists zero apps.
        KdeConnectService.withAdbTarget(args => root._refreshApps(args))
    }

    function _refreshApps(targetArgs): void {
        const deviceId = KdeConnectService.activeDeviceId || "default"

        root._send({
            "cmd": "list_apps",
            "target_args": targetArgs,
            "deviceId": deviceId
        })
    }

    function setSearchQuery(query: string): void {
        root.appsSearchQuery = query
        root._updateFilteredApps()
    }
    function setAppsSearchQuery(query): void { root.setSearchQuery(query) }

    function launchMirror(): void {
        if (root.mirrorRunning) {
            root.focusMirror()
            return
        }
        // This used to leave the button spinning on "launching" forever:
        // nothing was ever sent, so nothing ever answered.
        if (!PhoneState.runtimeReady) {
            root.reportFailure("mirror", I18n.tr("The phone integration is turned off"))
            return
        }
        root.mirrorLaunching = true
        root.mirrorLaunchError = ""
        root._whenManagerAllowed("mirror", () => KdeConnectService.withAdbTarget(args => root._launchMirror(args)))
    }

    // Right after a shell (re)start KDE Connect is still being probed, and a
    // launch in that window used to be dropped. It waits for it instead.
    property var _waitingForManager: ({})

    function _whenManagerAllowed(sessionId: string, run): void {
        if (root._managerAllowed) {
            run()
            return
        }
        const map = Object.assign({}, root._waitingForManager)
        map[sessionId] = run
        root._waitingForManager = map
        managerWaitTimer.restart()
    }

    on_ManagerAllowedChanged: {
        if (!root._managerAllowed) {
            root._managerWanted = false
            root._waitingForManager = ({})
            root._resumeQueue = []
            managerWaitTimer.stop()
            managerIdleTimer.stop()
            return
        }
        root.refreshCapabilities()
        const waiting = root._waitingForManager
        root._waitingForManager = ({})
        managerWaitTimer.stop()
        for (const id in waiting) waiting[id]()
    }

    Timer {
        id: managerWaitTimer
        interval: 10000
        repeat: false
        onTriggered: {
            const waiting = root._waitingForManager
            root._waitingForManager = ({})
            for (const id in waiting) {
                if (id === root.recordSessionId) root.recordingLaunching = false
                root.reportFailure(id, I18n.tr("KDE Connect is not running"))
            }
        }
    }

    /** Opens the phone mirror in its ordinary scrcpy window. */
    function openMirrorWindow(): void {
        root.launchMirror()
    }

    /** Ends the screen mirror without touching app, camera or microphone sessions. */
    function stopMirroring(): void {
        root.stopMirror()
        if (KdeConnectService.scrcpyRunning || KdeConnectService.scrcpyLaunching)
            KdeConnectService.killScrcpy()
    }
    function stopSession(sessionId): void {
        if (!sessionId) return
        root._markIntentionalStop(sessionId)
        root._send({ "cmd": "stop", "id": sessionId })
    }

    function stopAllSessions(): void {
        for (const session of Array.from(root.sessions || []))
            root._markIntentionalStop(session.id)
        root._send({ "cmd": "stop_all" })
        root.mirrorLaunching = false
        root.recordingLaunching = false
    }

    function toggleFavorite(packageName): void { root.toggleAppFavorite(packageName) }
    function isFavorite(packageName): bool { return root.isAppFavorite(packageName) }

    // ─── Failure feedback ─────────────────────────────────────
    /** A mirror, recording or app window failed; `reason` came from scrcpy. */
    signal mirrorFailed(string sessionId, string title, string reason)

    readonly property bool islandShowsFailures: false

    function reportFailure(sessionId: string, reason: string): void {
        const title = sessionId.startsWith("app:")
            ? I18n.tr("%1 could not be mirrored").arg(root._appLabel(sessionId.substring(4)))
            : sessionId === root.recordSessionId
                ? I18n.tr("Phone recording failed")
                : I18n.tr("Phone mirroring failed")
        const rawLines = String(reason || "").split("\n").map(line => line.trim()).filter(line => line.length > 0)
        const text = sessionId === "mirror" && rawLines.length > 0
            ? rawLines[rawLines.length - 1]
            : root.friendlyError(reason)
        if (sessionId === "mirror") {
            root.mirrorLaunching = false
            root.mirrorLaunchError = text
        }
        root.mirrorFailed(sessionId, title, text)
        // The Phone tab keeps its own inline line for this.
        KdeConnectService.dispatchActionFeedback(title + ": " + text, false)
        if (!root.islandShowsFailures)
            Spawn.run(["notify-send", "--app-name=Phone", "--icon=smartphone", title, text])
    }

    function _announceRecording(): void {
        const path = root.recordingPath
        const name = path.split("/").pop()
        Spawn.run(["notify-send", "--app-name=Phone", "--icon=smartphone",
            I18n.tr("Phone recording saved"),
            name + " · " + root._fmtDuration(root.recordingElapsedMs)])
    }

    function _fmtDuration(ms: int): string {
        const total = Math.floor(ms / 1000)
        return Math.floor(total / 60) + ":" + String(total % 60).padStart(2, "0")
    }

    function _appLabel(pkg: string): string {
        const app = (root.apps || []).find(a => a.package === pkg)
        if (app && app.label) return app.label
        const last = String(pkg).split(".").pop()
        return last.length > 0 ? last.charAt(0).toUpperCase() + last.slice(1) : pkg
    }

    /** scrcpy's own messages are written for a terminal; say what to do. */
    function friendlyError(raw: string): string {
        const text = String(raw || "").trim()
        const lower = text.toLowerCase()
        if (lower.length === 0)
            return I18n.tr("scrcpy closed unexpectedly")
        if (lower.includes("no such file") && lower.includes("scrcpy"))
            return I18n.tr("scrcpy is not installed")
        if (lower.includes("unauthorized"))
            return I18n.tr("USB debugging is not authorized — accept the prompt on the phone")
        if (lower.includes("could not find any adb device") || lower.includes("no devices")
                || lower.includes("not reachable over adb") || lower.includes("device offline"))
            return I18n.tr("The phone is not reachable over ADB — check the cable or wireless debugging")
        if (lower.includes("stayed locked"))
            return I18n.tr("The phone stayed locked — unlock it and try again")
        if (lower.includes("unlock window was closed"))
            return I18n.tr("The unlock window was closed before the phone was unlocked")
        if (lower.includes("device disconnected") || lower.includes("connection lost"))
            return I18n.tr("The phone disconnected")
        if (lower.includes("encoder") || lower.includes("mediacodec") || lower.includes("video stream"))
            return I18n.tr("The phone's video encoder failed — try a lower resolution or bit rate")
        if (lower.includes("could not be finalized"))
            return I18n.tr("scrcpy did not finish writing the recording, so the file could not be saved")
        if (lower.includes("audio"))
            return I18n.tr("Audio capture failed — try turning phone audio off")
        if (lower.includes("server connection failed") || lower.includes("could not push"))
            return I18n.tr("Could not start scrcpy on the phone — reconnect it and try again")
        if (lower.includes("unrecognized option") || lower.includes("invalid"))
            return I18n.tr("This scrcpy version rejected an option: %1").arg(text)
        return text
    }

    /** Options that hold for any scrcpy session, mirror or single app. App
     *  windows used to build their command from the App Mode block alone, so
     *  "Turn screen off", "Stay awake" and every quality setting silently
     *  applied to the mirror only. */
    function _commonScrcpyArgs() {
        const opts = Shell.Config.phone?.scrcpy
        if (!opts) return []

        const args = []
        if (opts.stayAwake) args.push("--stay-awake")
        if (opts.turnScreenOff) args.push("--turn-screen-off")
        if (opts.noPowerOn) args.push("--no-power-on")
        if (!(opts.audio ?? true)) args.push("--no-audio")
        if (opts.showTouches) args.push("--show-touches")
        if (opts.fullscreen) args.push("--fullscreen")
        if (opts.alwaysOnTop) args.push("--always-on-top")
        if (opts.maxFps > 0) args.push("--max-fps=" + opts.maxFps)
        if (opts.videoBitRate) args.push("--video-bit-rate=" + opts.videoBitRate)
        if (opts.maxSize > 0) args.push("--max-size=" + opts.maxSize)
        // scrcpy 3.0 renamed --display-buffer to --video-buffer; the old
        // spelling makes scrcpy exit on a usage error instead of starting.
        if (opts.videoBuffer > 0) args.push("--video-buffer=" + opts.videoBuffer)
        args.push(...root._inputArgs())
        // Locking must not flash the phone awake on the way out. Without this
        // scrcpy restores the screen power it had turned off, and only then
        // does the sleep land — so the panel lights up for a moment first.
        if (root._sessionEndMode() === "lock") args.push("--power-off-on-close")
        return args
    }

    function _launchMirror(targetArgs): void {
        const extraArgs = root._commonScrcpyArgs()

        const opts = Shell.Config.phone?.scrcpy
        if (opts) {
            const appOpts = opts.appMode || {}
            if (appOpts.flexDisplay) {
                const w = appOpts.displayWidth || 1280
                const h = appOpts.displayHeight || 960
                const density = appOpts.displayDpi || 240
                extraArgs.push("--new-display=" + w + "x" + h + "/" + density)
                extraArgs.push("--flex-display")
                if (appOpts.keepActive) {
                    extraArgs.push("--keep-active")
                }
                if (root._sessionEndMode() === "continue") {
                    extraArgs.push("--no-vd-destroy-content")
                }
            }
        }

        root._rememberSession("mirror", "mirror", extraArgs)
        root._send(root._launchPayload("mirror", "mirror", targetArgs, extraArgs))
    }

    function stopMirror(): void {
        root._markIntentionalStop("mirror")
        root._send({
            "cmd": "stop",
            "id": "mirror"
        })
    }

    function restartMirror(): void {
        root.stopMirror()
        root.mirrorLaunching = true
        restartMirrorTimer.restart()
    }

    Timer {
        id: restartMirrorTimer
        interval: 600
        repeat: false
        onTriggered: {
            root.mirrorLaunching = false
            root.launchMirror()
        }
    }

    function _focusWindowTitle(title): void {
        if (!title) return
        const windows = Wm.windows || []
        for (let i = windows.length - 1; i >= 0; i--) {
            if (windows[i].title === title) {
                Wm.focusWindow(windows[i].id)
                return
            }
        }
    }

    function focusMirror(): void {
        root._focusWindowTitle("ryoku-phone-mirror-mirror")
    }


    /** Records the phone screen to a file, with no window: scrcpy --record
     *  --no-playback. Runs beside a mirror rather than restarting it. */
    function startRecording(): void {
        if (!PhoneState.runtimeReady) return
        if (root.recordingRunning || root.recordingLaunching) return
        const opts = Shell.Config.phone?.scrcpy
        const configured = String(opts?.recording?.folder ?? "").trim()
        const cleanFolder = configured.startsWith("file://") ? configured.substring(7) : configured
        const folder = cleanFolder || (Quickshell.env("HOME") + "/Videos")
        const stamp = Qt.formatDateTime(new Date(), "yyyy-MM-dd_HH-mm-ss")
        root.recordingPath = folder + "/phone-" + stamp + ".mp4"
        root.recordingLaunching = true
        root.recordingElapsedMs = 0

        // No control on purpose: nothing reads input from a session with no
        // window, and --turn-screen-off / --stay-awake would be refused
        // without it.
        const args = ["--no-playback", "--no-control", "--record=" + root.recordingPath]
        if (!(opts?.audio ?? true)) args.push("--no-audio")
        if (opts?.maxFps > 0) args.push("--max-fps=" + opts.maxFps)
        if (opts?.videoBitRate) args.push("--video-bit-rate=" + opts.videoBitRate)
        if (opts?.maxSize > 0) args.push("--max-size=" + opts.maxSize)

        Quickshell.execDetached(["mkdir", "-p", folder])
        // Not remembered for auto-resume: reopening would write over the
        // file that was just cut short.
        root._whenManagerAllowed(root.recordSessionId,
            () => KdeConnectService.withAdbTarget(target => root._send(root._launchPayload(root.recordSessionId, "record", target, args))))
    }

    function stopRecording(): void {
        if (root._waitingForManager[root.recordSessionId]) {
            const map = Object.assign({}, root._waitingForManager)
            delete map[root.recordSessionId]
            root._waitingForManager = map
        }
        root._markIntentionalStop(root.recordSessionId)
        root.recordingLaunching = false
        root._send({
            "cmd": "stop",
            "id": root.recordSessionId
        })
    }

    function toggleRecording(): void {
        if (root.recordingRunning || root.recordingLaunching) root.stopRecording()
        else root.startRecording()
    }


    /** Keyboard and clipboard, the same for every window that takes input. */
    function _inputArgs() {
        const opts = Shell.Config.phone?.scrcpy
        const args = []
        if ((opts?.keyboardMode ?? "uhid") === "uhid") args.push("--keyboard=uhid")
        if (!(opts?.clipboardSync ?? true)) args.push("--no-clipboard-autosync")
        return args
    }

    function launchApp(packageName: string): void {
        if (!PhoneState.runtimeReady) return
        if (!packageName) return
        if (!appModeSupported) {
            KdeConnectService.dispatchActionFeedback(I18n.tr("scrcpy 3.0+ is required for App Mode"), false)
            return
        }

        if (root.isAppRunning(packageName)) {
            root.focusApp(packageName)
            return
        }

        KdeConnectService.withAdbTarget(args => root._launchApp(packageName, args))
    }

    /** What the phone should be left doing once a session ends. */
    function _sessionEndMode(): string {
        const mode = Shell.Config.phone?.scrcpy?.appMode?.onSessionEnd
        return (mode === "continue" || mode === "lock") ? mode : "home"
    }

    function _launchApp(packageName: string, targetArgs): void {
        const sessionId = "app:" + packageName
        const appOpts = Shell.Config.phone?.scrcpy?.appMode || {}
        const useFlex = appOpts.flexDisplay ?? false
        const w = appOpts.displayWidth || 1280
        const h = appOpts.displayHeight || 960
        const density = appOpts.displayDpi || 240

        // A '+' force-stops the app before starting it. On a virtual display
        // that is mandatory: Android resumes an app that is already running in
        // its existing task, i.e. back on the phone's own screen, leaving the
        // new display showing nothing but Samsung DeX's launcher.
        const extraArgs = root._commonScrcpyArgs().concat([
            "--start-app=" + (useFlex ? "+" : "") + packageName
        ])

        if (useFlex) {
            extraArgs.push("--new-display=" + w + "x" + h + "/" + density)
            extraArgs.push("--flex-display")
            if (appOpts.keepActive) {
                extraArgs.push("--keep-active")
            }
            if (appOpts.systemDecorations === false) {
                extraArgs.push("--no-vd-system-decorations")
            }
            // Without this the virtual display takes the app down with it.
            if (root._sessionEndMode() === "continue") {
                extraArgs.push("--no-vd-destroy-content")
            }
        }

        root._rememberSession(sessionId, "app", extraArgs)
        root._send(root._launchPayload(sessionId, "app", targetArgs, extraArgs))

        PhoneState.rememberPackage(packageName)

        KdeConnectService.dispatchActionFeedback(I18n.tr("Launching %1…").arg(packageName.split(".").pop()), true)
    }

    function stopApp(packageName: string): void {
        if (!packageName) return
        root._markIntentionalStop("app:" + packageName)
        root._send({
            "cmd": "stop",
            "id": "app:" + packageName
        })
    }

    function focusApp(packageName: string): void {
        if (!packageName) return
        root._focusWindowTitle("ryoku-phone-app-app_" + packageName)
    }

    function restartApp(packageName: string): void {
        root.stopApp(packageName)
        // The relaunch has to wait for the stop to actually land: starting it
        // on the next tick would re-arm auto-resume before the deliberate
        // exit arrives, and that exit would then be treated as a crash.
        restartAppTimer.pkg = packageName
        restartAppTimer.restart()
    }

    Timer {
        id: restartAppTimer
        property string pkg: ""
        interval: 600
        repeat: false
        onTriggered: if (restartAppTimer.pkg) root.launchApp(restartAppTimer.pkg)
    }

    function stopAllApps(): void {
        const live = root.sessions || []
        for (let i = 0; i < live.length; i++) {
            if (live[i].type === "app") root._markIntentionalStop(live[i].id)
        }
        root._send({
            "cmd": "stop_all"
        })
    }

    function isAppRunning(packageName: string): bool {
        if (!packageName || !sessions) return false
        const id = "app:" + packageName
        for (let i = 0; i < sessions.length; i++) {
            if (sessions[i].id === id) return true
        }
        return false
    }

    function toggleAppFavorite(packageName: string): void {
        if (!packageName) return
        const favorite = !root.isAppFavorite(packageName)
        PhoneState.setFavoritePackage(packageName, favorite)
        root._updateFilteredApps()
    }

    function isAppFavorite(packageName: string): bool {
        if (!packageName) return false
        return (PhoneState.favoritePackages || []).indexOf(packageName) >= 0
    }

    function _updateFilteredApps(): void {
        if (!apps) {
            filteredApps = []
            return
        }
        const q = appsSearchQuery.trim().toLowerCase()
        if (!q) {
            filteredApps = apps
            return
        }
        filteredApps = apps.filter(a => {
            if (!a) return false
            if (a.name && a.name.toLowerCase().includes(q)) return true
            if (a.package && a.package.toLowerCase().includes(q)) return true
            return false
        })
    }

    onAppsChanged: root._updateFilteredApps()

    // ─── Session auto-resume ──────────────────────────────────
    // adbd restarts whenever the phone is unlocked: every ADB connection
    // drops for a few seconds and takes any live scrcpy with it, and a
    // virtual display destroys its content on the way out. Relaunching the
    // same session once the phone answers again is the only way a mirror or
    // a DeX window survives an unlock.

    // id -> {type, args} for every session that could be resumed.
    property var _sessionArgs: ({})
    // ids whose exit was asked for, so a deliberate stop is never undone.
    property var _intentionalStops: ({})
    // id -> {count, first}: a phone that refuses to come back must not be
    // retried forever.
    property var _resumeAttempts: ({})
    property var _resumeQueue: []

    readonly property int _maxResumeAttempts: 5

    function _launchPayload(sessionId, typeStr, targetArgs, extraArgs) {
        return {
            "cmd": "launch",
            "id": sessionId,
            "type": typeStr,
            "target_args": targetArgs,
            "extra_args": extraArgs,
            // "continue" is already carried by --no-vd-destroy-content; only
            // "lock" needs the manager to act after the window is gone.
            "end_action": root._sessionEndMode() === "lock" ? "lock" : "",
            "auto_unlock": Shell.Config.phone?.scrcpy?.appMode?.autoUnlock ?? true
        }
    }

    function _rememberSession(sessionId, typeStr, extraArgs): void {
        root._sessionArgs[sessionId] = { "type": typeStr, "args": extraArgs }
        delete root._intentionalStops[sessionId]
    }

    function _markIntentionalStop(sessionId): void {
        root._intentionalStops[sessionId] = true
        root._resumeQueue = root._resumeQueue.filter(id => id !== sessionId)
    }

    function _forgetSession(sessionId): void {
        delete root._sessionArgs[sessionId]
        delete root._intentionalStops[sessionId]
        delete root._resumeAttempts[sessionId]
    }

    /** Queues `sessionId` for a relaunch if its exit looks like a dropped
     *  connection rather than something the user asked for. Returns whether
     *  the window is coming back. */
    function _maybeResume(sessionId, code): bool {
        if (root._intentionalStops[sessionId]) {
            root._forgetSession(sessionId)
            return false
        }
        // scrcpy keeps an exit code of its own for a lost connection (2).
        // Anything else is either the window being closed or a launch that
        // failed outright — a bad option, an app that is not there — and
        // that would fail the same way on every retry while the real error
        // stayed hidden behind "connection lost".
        if (code !== 2) {
            root._forgetSession(sessionId)
            return false
        }
        if (!(Shell.Config.phone?.scrcpy?.autoResume ?? true)) return false
        if (!root._sessionArgs[sessionId]) return false

        const now = Date.now()
        let attempt = root._resumeAttempts[sessionId]
        if (!attempt || now - attempt.first > 120000) attempt = { "count": 0, "first": now }
        if (attempt.count >= root._maxResumeAttempts) {
            root._forgetSession(sessionId)
            root.reportFailure(sessionId, I18n.tr("Phone connection lost — could not reopen the window"))
            return false
        }
        attempt.count += 1
        root._resumeAttempts[sessionId] = attempt

        if (root._resumeQueue.indexOf(sessionId) < 0)
            root._resumeQueue = root._resumeQueue.concat([sessionId])
        return true
    }

    Timer {
        // Short: the session manager does the actual waiting now, holding the
        // relaunch until the phone answers, so retrying here is cheap.
        id: resumeTimer
        interval: 1000
        repeat: true
        running: root._resumeQueue.length > 0
        onTriggered: {
            const queued = root._resumeQueue.slice()
            root._resumeQueue = []
            for (let i = 0; i < queued.length; i++) root._resumeSession(queued[i])
        }
    }

    function _resumeSession(sessionId): void {
        const rec = root._sessionArgs[sessionId]
        if (!rec) return
        if (sessionId === "mirror") root.mirrorLaunching = true
        // withAdbTarget re-resolves the target first: the port the session
        // died on is exactly the one that just changed.
        KdeConnectService.withAdbTarget(args => {
            if (!root._sessionArgs[sessionId]) return
            root._send(root._launchPayload(sessionId, rec.type, args, rec.args))
        })
    }

    // ─── scrcpy --version probe ──────────────────────────────
    Process {
        id: scrcpyVersionProc
        command: ["scrcpy", "--version"]
        running: false

        stdout: SplitParser {
            onRead: line => {
                const match = line.match(/^scrcpy\s+v?(\d+)\.(\d+)(?:\.(\d+))?/)
                if (match) {
                    root.available = true
                    root.versionMajor = parseInt(match[1])
                    root.versionMinor = parseInt(match[2])
                    root.version = match[1] + "." + match[2] + (match[3] ? "." + match[3] : "")
                }
            }
        }
    }

    // ─── Session Manager Process ──────────────────────────────
    Process {
        id: sessionManagerProc
        stdinEnabled: true
        command: PhoneState.pdeath([
            "python3",
            Quickshell.shellPath("modules/phone/scripts/phone/scrcpy_session_manager.py")
        ])
        running: root._managerWanted && root._managerAllowed

        onStarted: {
            const queued = root._pendingCommands
            root._pendingCommands = []
            for (let i = 0; i < queued.length; i++) sessionManagerProc.write(queued[i])
        }

        // Every session was this process' child, and nothing will report on
        // them again. Left as they were, a manager that died with a window
        // open kept the session count above zero forever: the idle timer
        // never let go of it, so it was never started again either.
        onExited: {
            root.sessions = []
            root.mirrorRunning = false
            root.mirrorLaunching = false
            root.recordingRunning = false
            root.recordingLaunching = false
            root.appsLoading = false
            const queued = root._pendingCommands.length > 0
            root._managerWanted = false
            // Commands that arrived while it was going down belong to the
            // next one; onStarted delivers them.
            if (queued) Qt.callLater(root.ensureManagerRunning)
        }

        stdout: SplitParser {
            onRead: data => {
                try {
                    const msg = JSON.parse(data)
                    const ev = msg.event

                    if (ev === "apps_list") {
                        root.apps = msg.apps || []
                        root.appsLoading = false
                        root.appsError = ""
                    } else if (ev === "apps_error") {
                        root.appsLoading = false
                        root.appsError = msg.message || "Failed to list apps"
                    } else if (ev === "started") {
                        const sid = msg.id
                        if (sid === "mirror") {
                            root.mirrorRunning = true
                            root.mirrorLaunching = false
                            root.mirrorElapsedMs = 0
                        } else if (sid === root.recordSessionId) {
                            root.recordingRunning = true
                            root.recordingLaunching = false
                        }
                        let curSessions = (root.sessions || []).slice()
                        const existingIdx = curSessions.findIndex(s => s.id === sid)
                        const sessionObj = {
                            id: sid,
                            type: msg.type || (sid === "mirror" ? "mirror" : sid === root.recordSessionId ? "record" : "app"),
                            package: sid.startsWith("app:") ? sid.substring(4) : "",
                            title: msg.title || "",
                            pid: msg.pid || 0,
                            startedAt: Date.now()
                        }
                        if (existingIdx >= 0) {
                            curSessions[existingIdx] = sessionObj
                        } else {
                            curSessions.push(sessionObj)
                        }
                        root.sessions = curSessions
                        if (msg.alreadyRunning && msg.title)
                            root._focusWindowTitle(msg.title)

                    } else if (ev === "waiting") {
                        // The manager holds a launch back while the phone is
                        // unreachable or still locked; say so instead of
                        // leaving a dead-looking button.
                        if (msg.id === "mirror") root.mirrorLaunching = true
                        else if (msg.id === root.recordSessionId) root.recordingLaunching = true
                        // Android draws the PIN pad on a FLAG_SECURE surface,
                        // so that window can only ever show black there. Input
                        // still reaches the phone, so the PIN can be typed
                        // blind — but only if the user is told to.
                        const waitText = msg.reason !== "locked"
                            ? I18n.tr("Waiting for the phone to reconnect…")
                            : !msg.unlockWindow
                                ? I18n.tr("Waiting for the phone to be unlocked…")
                                : msg.secure
                                    ? I18n.tr("Type your PIN in the window that opened — Android blanks the PIN screen, so it stays black")
                                    : I18n.tr("Unlock your phone in the window that just opened")
                        KdeConnectService.dispatchActionFeedback(waitText, true)

                    } else if (ev === "exited") {
                        const sid = msg.id
                        const asked = !!root._intentionalStops[sid]
                        const resuming = root._maybeResume(sid, msg.code)
                        // A drop that is about to be reopened is not worth a
                        // toast — the window comes back on its own — and
                        // neither is a stop the user asked for.
                        // A non-zero exit that printed nothing is still a
                        // failure; only closing the window exits with 0.
                        const failed = msg.code !== 0 && !resuming && !asked
                        if (sid === "mirror") {
                            root.mirrorRunning = false
                            root.mirrorLaunching = resuming
                            if (failed) root.reportFailure(sid, msg.error || "")
                        } else if (sid === root.recordSessionId) {
                            const wasRunning = root.recordingRunning
                            root.recordingRunning = false
                            root.recordingLaunching = false
                            // A stop the user asked for can still lose the file.
                            if (failed || msg.unfinalized) root.reportFailure(sid, msg.error || "")
                            else if (wasRunning) root._announceRecording()
                        } else if (failed) {
                            // App windows used to die without a word.
                            root.reportFailure(sid, msg.error || "")
                        }
                        let curSessions = (root.sessions || []).filter(s => s.id !== sid)
                        root.sessions = curSessions

                    } else if (ev === "error") {
                        if (msg.id === root.recordSessionId) root.recordingLaunching = false
                        root.reportFailure(msg.id || "mirror", msg.message || "")
                    }
                } catch (e) {
                    console.warn("[PhoneScrcpyService] JSON error:", e, "Data:", data)
                }
            }
        }

        stderr: SplitParser {
            onRead: data => {
                if (data.trim().length > 0) {
                    console.warn("[PhoneScrcpyService stderr]", data)
                }
            }
        }
    }
}
