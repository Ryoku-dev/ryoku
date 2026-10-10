pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import QtCore
import Qt.labs.folderlistmodel
import inir
import inir.modules.common
import inir.modules.common.functions
import inir.modules.common.models
import inir.services
import shell.services as Ryoku

// Wallpaper bridge for the vendored frame. Ryoku owns wallpaper application
// (the wallpaper daemon plus ryogami); this exposes the reference's read-side
// surface — the effective path, an image-safe still URL, the gallery scan and
// the freedesktop-style thumbnail cache — so the frame's glass, mood and
// preview consumers keep working without ever driving a second wallpaper
// backend. Apply/preview verbs are intentionally absent: the frame never
// changes the wallpaper, it only samples it.
Singleton {
    id: root

    // The active wallpaper file, straight from the daemon's state.
    readonly property string effectiveWallpaperPath: FileUtils.trimFileProtocol(Ryoku.Session.wallpaper)
    readonly property string effectiveWallpaperUrl: root.stillUrlFor(root.effectiveWallpaperPath)

    // Ryoku themes from the same file the desktop wears.
    function currentThemingWallpaperPath(monitorName = ""): string {
        return root.effectiveWallpaperPath
    }

    function currentMainWallpaperPath(monitorName = ""): string {
        return root.effectiveWallpaperPath
    }

    // What the desktop actually shows: the daemon paints the wallpaper itself, so
    // the overview backdrop never replaces it here and needs no dim to sample
    // through. The frame reads these to match glass and previews to the screen.
    readonly property bool desktopShowsBackdrop: false
    readonly property real desktopDim: 0
    readonly property real desktopSaturation: 0
    readonly property real desktopContrast: 0

    function desktopWallpaperPath(monitorName = ""): string {
        return root.effectiveWallpaperPath
    }

    function isVideoFile(path: string): bool {
        const lower = String(path ?? "").toLowerCase()
        return lower.endsWith(".mp4") || lower.endsWith(".webm") || lower.endsWith(".mkv") || lower.endsWith(".mov") || lower.endsWith(".avi")
    }

    // ── Video stills ───────────────────────────────────────────────────────
    // The daemon already extracts a live poster for every clip; reuse it as the
    // image-safe still so glass surfaces sample exactly what the desktop shows.
    readonly property string livePoster: Ryoku.Session.livePoster
    property var videoFirstFrames: ({})

    function stillUrlFor(path: string): string {
        const clean = FileUtils.trimFileProtocol(String(path ?? ""))
        if (!clean)
            return ""
        if (!root.isVideoFile(clean))
            return "file://" + clean
        if (clean === root.effectiveWallpaperPath && root.livePoster)
            return "file://" + root.livePoster
        const frame = root.videoFirstFrames[clean]
        if (frame)
            return frame.startsWith("file://") ? frame : "file://" + frame
        // Caching writes videoFirstFrames, which the calling binding just read: deferred, or it loops.
        Qt.callLater(root.ensureVideoFirstFrame, clean)
        return ""
    }

    // ── Scaled playback ────────────────────────────────────────────────────
    // A live wallpaper is decoded no larger than it is drawn: a 4K file behind
    // a 1080p output plays from a cached copy at that height. The copy script
    // symlinks the original when it is already small enough.
    readonly property string _videoPlaybackDir: (Quickshell.env("XDG_CACHE_HOME")
        || (Quickshell.env("HOME") + "/.cache")) + "/ryoku/inir/video_playback"
    readonly property var _videoPlaybackHeights: [360, 540, 720, 1080, 1440, 2160]
    readonly property string _videoPlaybackScript: `${FileUtils.trimFileProtocol(Directories.scriptPath)}/videos/video-playback-copy.sh`
    // key -> "" while checking, "building" while the copy is made, the path to play, or "original"
    property var videoPlaybackCopies: ({})
    property var _videoPlaybackQueue: []

    function videoPlaybackPath(path: string, height: int): string {
        const clean = FileUtils.trimFileProtocol(String(path ?? ""))
        if (!clean || !root.isVideoFile(clean) || height <= 0)
            return clean
        const tier = root._videoPlaybackHeights.find(h => h >= height) ?? 0
        if (!tier)
            return clean
        const key = clean + "@" + tier
        const state = root.videoPlaybackCopies[key]
        if (state === undefined) {
            // Recording the request writes the map the calling binding just read.
            Qt.callLater(root._requestVideoPlaybackCopy, clean, tier)
            return ""
        }
        if (state === "") return ""
        if (state === "building" || state === "original") return clean
        return state
    }

    function _setVideoPlaybackState(key: string, state: string): void {
        const copy = Object.assign({}, root.videoPlaybackCopies)
        copy[key] = state
        root.videoPlaybackCopies = copy
    }

    function _requestVideoPlaybackCopy(path: string, tier: int): void {
        const key = path + "@" + tier
        if (root.videoPlaybackCopies[key] !== undefined)
            return
        root._setVideoPlaybackState(key, "")
        root._videoPlaybackQueue = root._videoPlaybackQueue.concat([{ key: key, path: path, tier: tier,
            fps: tier <= 540 ? 30 : 0,
            output: root._videoPlaybackDir + "/" + Qt.md5(path) + "-" + tier + (tier <= 540 ? "-30" : "") + ".mp4",
            check: true }])
        root._runVideoPlaybackQueue()
    }

    function _runVideoPlaybackQueue(): void {
        if (_videoPlaybackProc.running || root._videoPlaybackQueue.length === 0)
            return
        // Checks jump the queue: a cached copy should never wait behind a transcode.
        const queue = root._videoPlaybackQueue
        const at = queue.findIndex(job => job.check)
        const job = queue[at >= 0 ? at : 0]
        root._videoPlaybackQueue = queue.filter((_, i) => i !== (at >= 0 ? at : 0))
        _videoPlaybackProc.job = job
        _videoPlaybackProc.command = [root._videoPlaybackScript].concat(job.check ? ["--check"] : [])
            .concat([job.path, job.output, String(job.tier), String(job.fps)])
        _videoPlaybackProc.running = true
    }

    Process {
        id: _videoPlaybackProc
        property var job: null
        onExited: exitCode => {
            const job = _videoPlaybackProc.job
            if (job?.check && exitCode === 1) {
                root._setVideoPlaybackState(job.key, "building")
                root._videoPlaybackQueue = root._videoPlaybackQueue.concat([Object.assign({}, job, { check: false })])
            } else if (job) {
                root._setVideoPlaybackState(job.key, exitCode === 0 ? job.output : "original")
            }
            root._runVideoPlaybackQueue()
        }
    }

    function internalPreviewFor(monitorName: string, fallbackPath: string): string {
        return fallbackPath
    }

    readonly property bool internalPreviewActive: false
    readonly property bool batteryPauseActive: false

    function videoMotionAllowedOn(outputName: string): bool {
        return true
    }

    // ── First-frame extraction ─────────────────────────────────────────────
    readonly property string _videoThumbDir: (Quickshell.env("XDG_CACHE_HOME")
        || (Quickshell.env("HOME") + "/.cache")) + "/ryoku/inir/video_thumbnails"

    function getVideoFirstFramePath(videoPath: string): string {
        const clean = FileUtils.trimFileProtocol(String(videoPath ?? ""))
        return root._videoThumbDir + "/" + Qt.md5(clean) + ".jpg"
    }

    function ensureVideoFirstFrame(videoPath: string): void {
        const clean = FileUtils.trimFileProtocol(String(videoPath ?? ""))
        if (!clean || !root.isVideoFile(clean))
            return
        const out = root.getVideoFirstFramePath(clean)
        if (root.videoFirstFrames[clean]) {
            root.videoFirstFrames = Object.assign({}, root.videoFirstFrames, { [clean]: out })
            return
        }
        if (root._ffPending[clean])
            return
        root._ffPending[clean] = true
        root._ffQueue = root._ffQueue.concat([{ video: clean, out: out }])
        root._processNextFF()
    }

    property var _ffPending: ({})
    property var _ffQueue: []

    function _processNextFF(): void {
        if (root._ffProc.running || root._ffQueue.length === 0)
            return
        const job = root._ffQueue[0]
        root._ffQueue = root._ffQueue.slice(1)
        root._ffProc.videoPath = job.video
        root._ffProc.outputPath = job.out
        root._ffProc.running = true
    }

    Process {
        id: _ffProc
        property string videoPath: ""
        property string outputPath: ""
        command: ["ffmpeg", "-y", "-i", videoPath, "-vf",
            "scale='min(1920,iw)':-2", "-frames:v", "1", outputPath]
        onExited: exitCode => {
            delete root._ffPending[root._ffProc.videoPath]
            if (exitCode === 0) {
                root.videoFirstFrames = Object.assign({}, root.videoFirstFrames,
                    { [root._ffProc.videoPath]: root._ffProc.outputPath })
            }
            root._processNextFF()
        }
    }

    // ── Gallery scan + folder browsing ──────────────────────────────────────
    // One navigable folder model backs both the frame's read-only preview
    // gallery and the picker's browser; there is no second wallpaper engine.
    readonly property string galleryDirectory: (Quickshell.env("XDG_PICTURES_DIR")
        || (Quickshell.env("HOME") + "/Pictures")) + "/Wallpapers"
    readonly property url defaultFolder: Qt.resolvedUrl("file://" + root.galleryDirectory)
    readonly property string effectiveDirectory: FileUtils.trimFileProtocol(folderModelImpl.folder.toString())
    readonly property bool folderModelReady: folderModelImpl.status === FolderListModel.Ready
    property string searchQuery: ""
    readonly property list<string> extensions: ["jpg", "jpeg", "png", "webp", "avif", "bmp", "svg", "gif", "mp4", "webm", "mkv", "avi", "mov"]
    property list<string> wallpapers: []

    signal directoryChanged()
    onEffectiveDirectoryChanged: root.directoryChanged()

    readonly property alias folderModel: folderModelImpl
    FolderListModelWithHistory {
        id: folderModelImpl
        folder: root.defaultFolder
        caseSensitive: false
        nameFilters: {
            const query = root.searchQuery.trim().toLowerCase()
            if (query.startsWith(".")) {
                const ext = query.slice(1)
                if (root.extensions.includes(ext)) return [`*.${ext}`]
            }
            const searchParts = query.split(" ").filter(s => s.length > 0).map(s => `*${s}*`).join("")
            return root.extensions.map(ext => `*${searchParts}*.${ext}`)
        }
        showDirs: true
        showDotAndDotDot: false
        showOnlyReadable: true
        sortField: FolderListModel.Time
        sortReversed: false
        onCountChanged: root._rescan()
        onStatusChanged: if (status === FolderListModel.Ready) root._rescan()
    }

    function _rescan(): void {
        if (folderModelImpl.status !== FolderListModel.Ready)
            return
        const out = []
        for (let i = 0; i < folderModelImpl.count; i++) {
            if (folderModelImpl.get(i, "fileIsDir"))
                continue
            const p = folderModelImpl.get(i, "filePath")
                || FileUtils.trimFileProtocol(folderModelImpl.get(i, "fileURL"))
            if (p)
                out.push(p)
        }
        root.wallpapers = out
    }

    // Directory changes are validated off the UI thread: a typed path that is a
    // file opens its parent, matching the reference's browser affordance.
    Process {
        id: _dirValidateProc
        property string _nicePath: ""
        property bool _fileFallback: false
        function check(path: string): void {
            _dirValidateProc._nicePath = FileUtils.trimFileProtocol(String(path ?? "")).replace(/\/+$/, "") || "/"
            _dirValidateProc._fileFallback = false
            _dirValidateProc.command = ["test", "-d", _dirValidateProc._nicePath]
            _dirValidateProc.running = true
        }
        onExited: exitCode => {
            if (!_dirValidateProc._fileFallback) {
                if (exitCode === 0) {
                    folderModelImpl.folder = Qt.resolvedUrl("file://" + _dirValidateProc._nicePath)
                    return
                }
                _dirValidateProc._fileFallback = true
                _dirValidateProc.command = ["test", "-f", _dirValidateProc._nicePath]
                _dirValidateProc.running = true
                return
            }
            if (exitCode === 0)
                folderModelImpl.folder = Qt.resolvedUrl("file://" + FileUtils.parentDirectory(_dirValidateProc._nicePath))
        }
    }

    function setDirectory(path: string): void {
        _dirValidateProc.check(path)
    }
    function navigateUp(): void {
        folderModelImpl.navigateUp()
    }
    function navigateBack(): void {
        folderModelImpl.navigateBack()
    }
    function navigateForward(): void {
        folderModelImpl.navigateForward()
    }
    // Batch prewarm is unnecessary: ThumbnailImage resolves each still lazily.
    function generateThumbnail(size: string): void {}

    Component.onCompleted: root._rescan()

    // ── Thumbnails (freedesktop cache) ─────────────────────────────────────
    // A tiny in-memory set so list delegates can fall back to the full-size
    // file when no cached thumbnail exists yet.
    property var _knownThumbnails: ({})
    readonly property bool thumbnailGenerationRunning: false

    function videoStillPath(filePath: string): string {
        return root.getVideoFirstFramePath(filePath)
    }

    function ensureVideoStill(filePath: string): void {
        root.ensureVideoFirstFrame(filePath)
    }

    function hasKnownThumbnail(path: string): bool {
        return root._knownThumbnails[FileUtils.trimFileProtocol(String(path ?? ""))] === true
    }

    function rememberThumbnail(path: string): void {
        const clean = FileUtils.trimFileProtocol(String(path ?? ""))
        if (!clean)
            return
        const next = Object.assign({}, root._knownThumbnails)
        next[clean] = true
        root._knownThumbnails = next
    }

    function forgetThumbnail(path: string): void {
        const clean = FileUtils.trimFileProtocol(String(path ?? ""))
        const next = Object.assign({}, root._knownThumbnails)
        delete next[clean]
        root._knownThumbnails = next
    }

    function ensureThumbnailForPath(filePath: string, size = "large"): void {
        // Thumbnails are a nicety; the frame reads full-size images otherwise.
    }

    // ── Selection + apply (ryogami) ─────────────────────────────────────────
    // The Ryoku wallpaper path only acts on the main wallpaper; the picker's
    // richer target vocabulary collapses to that single daemon call.
    function currentSelectionTarget(): string {
        return "main"
    }
    function currentWallpaperPathForTarget(target: string, monitorName: string): string {
        return root.effectiveWallpaperPath
    }
    function isCurrentWallpaperPath(path: string, target: string, monitorName: string): bool {
        const clean = FileUtils.trimFileProtocol(String(path ?? ""))
        return clean.length > 0 && clean === root.effectiveWallpaperPath
    }
    // The shuffle reads its own listing: files only (a folder is never a wallpaper)
    // and no search filter, so the picker keeps the folder it is showing even when
    // the shuffle browses a folder of its own.
    FolderListModel {
        id: shuffleModel
        folder: folderModelImpl.folder
        nameFilters: root.extensions.map(ext => `*.${ext}`)
        caseSensitive: false
        showDirs: false
        showDotAndDotDot: false
        showOnlyReadable: true
    }

    function randomFromCurrentFolder(darkMode = false, monitorName = "", target = ""): void {
        if (shuffleModel.count === 0)
            return
        const filePath = shuffleModel.get(Math.floor(Math.random() * shuffleModel.count), "filePath")
        if (filePath)
            root.applySelectionTarget(String(filePath), target, monitorName)
    }

    // The shuffle: a new wallpaper from the browsed folder every few minutes,
    // through the same daemon path as the desktop menu's Next wallpaper. It
    // rests while the screen is locked. Ryoku themes every wallpaper it
    // applies, so there is no colour switch here; the daemon owns that.
    readonly property var shuffleOptions: Config.options?.background?.autoWallpaper ?? ({})
    readonly property bool shuffleEnabled: Boolean(shuffleOptions.enable ?? false)
    readonly property int shuffleMinutes: Math.max(1, Math.min(240, Number(shuffleOptions.intervalMinutes ?? 30)))
    Timer {
        id: shuffleTimer
        interval: root.shuffleMinutes * 60000
        repeat: true
        running: root.shuffleEnabled && !GlobalStates.screenLocked
        onTriggered: root.randomFromCurrentFolder()
    }

    function applySelectionTarget(path: string, target: string, monitorName: string): void {
        const clean = FileUtils.trimFileProtocol(String(path ?? ""))
        if (!clean)
            return
        // The daemon resolves a clip's poster before it swaps, so a video path
        // needs no still handling here.
        const command = ["ryogami", "wallpaper", "set", clean]
        if (target === "workspace")
            command.push("--workspace")
        const monitor = String(monitorName ?? "")
        if (monitor.length > 0)
            command.push("--screen", monitor)
        Quickshell.execDetached(command)
    }

    // Ryoku has no in-shell preview backend; the picker keeps its live-preview
    // affordance but nothing paints until Apply reaches the daemon.
    function previewWallpaper(path: string, monitorName: string): void {}
    function cancelWallpaperPreview(): void {}
    function clearWallpaperPreview(): void {}
}
