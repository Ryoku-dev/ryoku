import stage
import stage.modules.common
import stage.modules.common.models
import stage.modules.common.functions
import QtQuick
import Qt.labs.folderlistmodel
import Quickshell
import Quickshell.Io
pragma Singleton
pragma ComponentBehavior: Bound

/**
 * Provides a list of wallpapers and an "apply" action that calls the existing
 * switchwall.sh script. Pretty much a limited file browsing service.
 */
Singleton {
    id: root

    // Strictly increasing per apply() call, since QML dispatch is single-threaded.
    // Passed to switchwall*.sh as --request-seq so a slower, superseded backend
    // run can tell it lost the race and skip writing preview colors. Seeded from
    // Date.now() (not 0): the on-disk token file survives Quickshell restarts, but
    // this counter doesn't, so starting at 0 again let old high-water marks (from a
    // prior session, or from a PID that beat a low seq) permanently outrank every
    // future request and freeze the swatches. A wall-clock seed is always greater
    // than whatever was written before, so it self-heals on the very next switch.
    property real _wallpaperRequestSeq: Date.now()

    property string extractColorsScriptPath: FileUtils.trimFileProtocol(Directories.extractColorsScriptPath)
    property alias directory: folderModel.folder
    readonly property string effectiveDirectory: FileUtils.trimFileProtocol(folderModel.folder.toString())
    property string ryogamiWallpaperDirectory: FileUtils.trimFileProtocol(`${Directories.pictures}/Wallpapers`)
    property string ryogamiCacheDirectory: Directories.ryogamiCachePath
    property var ryogamiEntriesByPath: ({})
    property int ryogamiIndexRevision: 0
    property var ryogamiOutputs: ({})
    property int ryogamiOutputsRevision: 0
    property url defaultFolder: Qt.resolvedUrl("file://" + root.ryogamiWallpaperDirectory)
    property alias folderModel: folderModel
    property string searchQuery: ""
    readonly property list<string> extensions: [
        "jpg", "jpeg", "png", "webp", "avif", "bmp", "gif", "tiff", "tif",
        "mp4", "mkv", "webm", "avi", "mov", "m4v", "ogv"
    ]
    property list<string> wallpapers: []
    readonly property bool thumbnailGenerationRunning: false
    property var colorCache: ({})
    property string sortField: "modified"
    property bool sortReversed: false
    property var creationTimes: ({})
    property list<string> pendingCreationPaths: []
    property alias sortedFolderModel: sortedFolderModel
    property alias ryogamiLibraryModel: ryogamiLibraryModel
    property string directoryError: ""
    readonly property bool directoryLoading: folderModel.status === FolderListModel.Loading
    property var pendingVideoPosters: []
    property var requestedVideoPosters: ({})
    property int videoPosterRevision: 0

    signal changed()
    signal sortChanged()
    signal videoPosterReady(filePath: string)

    function load () {}

    function expandUserPath(path) {
        const value = String(path ?? "").trim();
        if (value.startsWith("~/"))
            return FileUtils.trimFileProtocol(Directories.home) + "/" + value.slice(2);
        return FileUtils.trimFileProtocol(value);
    }

    function loadRyogamiConfig(text) {
        let data = null;
        try {
            data = JSON.parse(String(text ?? ""));
        } catch (error) {
            return;
        }
        const paths = data?.paths ?? {};
        const wallpaper = root.expandUserPath(paths.wallpaper);
        const cache = root.expandUserPath(paths.cache);
        root.ryogamiWallpaperDirectory = wallpaper !== ""
            ? wallpaper : FileUtils.trimFileProtocol(`${Directories.pictures}/Wallpapers`);
        root.ryogamiCacheDirectory = cache !== "" ? cache : Directories.ryogamiCachePath;
    }

    function loadRyogamiIndex(text) {
        let data = null;
        try {
            data = JSON.parse(String(text ?? ""));
        } catch (error) {
            data = null;
        }
        const byPath = {};
        if (data) {
            for (const key in data) {
                const entry = data[key];
                const path = FileUtils.trimFileProtocol(String(entry?.path ?? ""));
                if (path !== "")
                    byPath[path] = entry;
            }
        }
        root.ryogamiEntriesByPath = byPath;
        root.ryogamiIndexRevision++;
        root.rebuildRyogamiLibraryModel();
    }
    function loadRyogamiOutputs(text) {
        let data = null;
        try {
            data = JSON.parse(String(text ?? ""));
        } catch (error) {
            data = null;
        }
        root.ryogamiOutputs = data ?? {};
        root.ryogamiOutputsRevision++;
    }

    function currentWallpaperPath(screenName = "") {
        void root.ryogamiOutputsRevision;
        const monitor = String(screenName ?? "");
        let entry = monitor !== "" ? root.ryogamiOutputs[monitor] : null;
        if (!entry)
            entry = root.ryogamiOutputs["*"];
        if (!entry && monitor === "") {
            const names = Object.keys(root.ryogamiOutputs).sort();
            entry = names.length > 0 ? root.ryogamiOutputs[names[0]] : null;
        }
        const path = FileUtils.trimFileProtocol(String(entry?.path ?? ""));
        if (path !== "")
            return path;
        const configured = FileUtils.trimFileProtocol(String(Config.wallpaperPath ?? ""));
        return configured !== "" ? configured : FileUtils.trimFileProtocol(Directories.defaultWallpaperImagePath);
    }


    function ryogamiThumbnailPath(filePath) {
        const entry = root.ryogamiEntriesByPath[FileUtils.trimFileProtocol(String(filePath ?? ""))];
        return FileUtils.trimFileProtocol(String(entry?.thumb ?? entry?.thumb_sm ?? ""));
    }
    function localFileUrl(filePath) {
        const clean = FileUtils.trimFileProtocol(String(filePath ?? ""));
        if (clean === "")
            return "";
        const encoded = clean.split("/").map(part => encodeURIComponent(part).replace(/[!'()*]/g, character =>
            "%" + character.charCodeAt(0).toString(16).toUpperCase())).join("/");
        return "file://" + encoded;
    }


    function videoPosterPath(filePath) {
        const clean = FileUtils.trimFileProtocol(String(filePath ?? ""));
        return clean === "" ? "" : `${Directories.stageVideoPosterCachePath}/${Qt.md5(clean)}.jpg`;
    }

    function ensureVideoPoster(filePath) {
        const clean = FileUtils.trimFileProtocol(String(filePath ?? ""));
        if (!root.isVideoFile(clean) || root.requestedVideoPosters[clean])
            return;
        const requested = Object.assign({}, root.requestedVideoPosters);
        requested[clean] = true;
        root.requestedVideoPosters = requested;
        root.pendingVideoPosters = root.pendingVideoPosters.concat([clean]);
        root.runNextVideoPoster();
    }

    function runNextVideoPoster() {
        if (videoPosterProc.running || root.pendingVideoPosters.length === 0)
            return;
        videoPosterProc.filePath = root.pendingVideoPosters[0];
        root.pendingVideoPosters = root.pendingVideoPosters.slice(1);
        videoPosterProc.outputPath = root.videoPosterPath(videoPosterProc.filePath);
        videoPosterProc.running = true;
    }

    FileView {
        id: ryogamiConfigFile
        path: Directories.ryogamiConfigPath
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: root.loadRyogamiConfig(text())
    }

    FileView {
        id: ryogamiIndexFile
        path: `${root.ryogamiCacheDirectory}/wallpaper/index.json`
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: root.loadRyogamiIndex(text())
        onLoadFailed: root.loadRyogamiIndex("")
    }
    FileView {
        id: ryogamiOutputsFile
        path: `${root.ryogamiCacheDirectory}/outputs.json`
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: root.loadRyogamiOutputs(text())
        onLoadFailed: root.loadRyogamiOutputs("")
    }


    Process {
        id: videoPosterProc
        property string filePath: ""
        property string outputPath: ""
        command: [
            "bash", "-c",
            "mkdir -p -- \"$1\" && { test -s \"$3\" || ffmpeg -v error -y -i \"$2\" -frames:v 1 -vf 'scale=640:360:force_original_aspect_ratio=decrease' \"$3\"; }",
            "stage-video-poster", Directories.stageVideoPosterCachePath, filePath, outputPath
        ]
        onExited: exitCode => {
            if (exitCode === 0) {
                root.videoPosterRevision++;
                root.videoPosterReady(videoPosterProc.filePath);
            }
            root.runNextVideoPoster();
        }
    }

    function normalizeSortField(value) {
        const field = String(value || "modified");
        return ["name", "modified", "created", "size"].includes(field) ? field : "modified";
    }

    function normalizeDateValue(value) {
        if (typeof value === "number" && isFinite(value)) {
            return (value > 0 && value < 10000000000) ? value * 1000 : value;
        }
        if (value && typeof value.toMSecsSinceEpoch === "function") {
            const milliseconds = Number(value.toMSecsSinceEpoch());
            if (isFinite(milliseconds)) return milliseconds;
        }
        if (value && typeof value.getTime === "function") {
            const milliseconds = Number(value.getTime());
            if (isFinite(milliseconds)) return milliseconds;
        }
        const parsed = Date.parse(String(value || ""));
        return isFinite(parsed) ? parsed : 0;
    }

    function sortValue(entry) {
        switch (root.sortField) {
        case "name":
            return entry.fileName.toLocaleLowerCase();
        case "created":
            return entry.fileCreated > 0 ? entry.fileCreated : entry.fileLastModified;
        case "size":
            return entry.fileSize;
        case "modified":
        default:
            return entry.fileLastModified;
        }
    }
    function compareEntries(left, right) {
        if (left.fileIsDir !== right.fileIsDir)
            return left.fileIsDir ? -1 : 1;
        const leftValue = root.sortValue(left);
        const rightValue = root.sortValue(right);
        let comparison = 0;
        if (typeof leftValue === "string") {
            comparison = leftValue.localeCompare(rightValue);
        } else if (leftValue > rightValue) {
            comparison = -1;
        } else if (leftValue < rightValue) {
            comparison = 1;
        }
        if (root.sortReversed)
            comparison = -comparison;
        return comparison !== 0 ? comparison
            : left.fileName.toLocaleLowerCase().localeCompare(right.fileName.toLocaleLowerCase());
    }


    function rebuildSortedFolderModel() {
        const entries = [];
        for (let i = 0; i < folderModel.count; i++) {
            const filePath = String(folderModel.get(i, "filePath") || FileUtils.trimFileProtocol(folderModel.get(i, "fileUrl") || folderModel.get(i, "fileURL") || ""));
            if (!filePath) continue;

            const normalizedPath = FileUtils.trimFileProtocol(filePath);
            const fileModifiedRaw = folderModel.get(i, "fileModified") ?? folderModel.get(i, "fileLastModified");
            entries.push({
                filePath: filePath,
                fileUrl: String(folderModel.get(i, "fileUrl") || folderModel.get(i, "fileURL") || filePath),
                fileName: String(folderModel.get(i, "fileName") || ""),
                fileBaseName: String(folderModel.get(i, "fileBaseName") || ""),
                fileSuffix: String(folderModel.get(i, "fileSuffix") || ""),
                fileSize: Number(folderModel.get(i, "fileSize") || 0),
                fileLastModified: root.normalizeDateValue(fileModifiedRaw),
                fileCreated: Number(root.creationTimes[normalizedPath] || 0),
                fileIsDir: Boolean(folderModel.get(i, "fileIsDir"))
            });
        }

        entries.sort(root.compareEntries);

        sortedFolderModel.clear();
        for (let i = 0; i < entries.length; i++) {
            sortedFolderModel.append(entries[i]);
        }
    }
    function rebuildRyogamiLibraryModel() {
        const query = root.searchQuery.trim().toLocaleLowerCase();
        const entries = [];
        for (const path in root.ryogamiEntriesByPath) {
            const entry = root.ryogamiEntriesByPath[path];
            const name = String(entry?.name ?? path.substring(path.lastIndexOf("/") + 1));
            const lowerName = name.toLocaleLowerCase();
            if (query !== "" && !lowerName.includes(query))
                continue;
            if (!root.extensions.some(extension => lowerName.endsWith("." + extension)))
                continue;
            const slash = name.lastIndexOf("/");
            const fileName = slash >= 0 ? name.slice(slash + 1) : name;
            const suffix = fileName.includes(".") ? fileName.slice(fileName.lastIndexOf(".") + 1) : "";
            entries.push({
                filePath: path,
                fileUrl: root.localFileUrl(path),
                fileName: fileName,
                fileBaseName: suffix === "" ? fileName : fileName.slice(0, -(suffix.length + 1)),
                fileSuffix: suffix,
                fileSize: Number(entry?.filesize ?? 0),
                fileLastModified: Number(entry?.mtime ?? 0) * 1000,
                fileCreated: Number(entry?.mtime ?? 0) * 1000,
                fileIsDir: false
            });
        }
        entries.sort(root.compareEntries);
        ryogamiLibraryModel.clear();
        for (let i = 0; i < entries.length; i++)
            ryogamiLibraryModel.append(entries[i]);
    }

    onSearchQueryChanged: root.rebuildRyogamiLibraryModel()

    function refreshCreationTimes() {
        const paths = [];
        for (let i = 0; i < folderModel.count; i++) {
            const filePath = String(folderModel.get(i, "filePath") || "");
            if (filePath) paths.push(FileUtils.trimFileProtocol(filePath));
        }

        root.pendingCreationPaths = paths;
        if (paths.length === 0) {
            root.creationTimes = ({});
            root.rebuildSortedFolderModel();
            return;
        }

        creationTimesProc.command = [
            "bash", "-c",
            "for path do stat -c '%W' -- \"$path\" 2>/dev/null || printf '0\\n'; done",
            "wallpaper-birth-times"
        ].concat(paths);
        creationTimesProc.running = true;
        root.rebuildSortedFolderModel();
    }

    function queueFolderModelRefresh() {
        folderModelRefreshTimer.restart();
    }

    function applyNativeSort() {
        if (root.sortField === "name") {
            folderModel.sortField = FolderListModel.Name;
        } else if (root.sortField === "size") {
            folderModel.sortField = FolderListModel.Size;
        } else {
            folderModel.sortField = FolderListModel.Time;
        }
        folderModel.sortReversed = root.sortReversed;
    }

    function loadSortOptions() {
        const options = Config.options?.wallpaperSelector;
        root.sortField = root.normalizeSortField(options?.sortField);
        root.sortReversed = options?.sortReversed === true;
        root.applyNativeSort();
        root.queueFolderModelRefresh();
        root.rebuildRyogamiLibraryModel();
    }

    function selectSortField(field) {
        const nextField = root.normalizeSortField(field);
        if (root.sortField === nextField) {
            root.sortReversed = !root.sortReversed;
        } else {
            root.sortField = nextField;
            root.sortReversed = false;
        }

        Config.options.wallpaperSelector.sortField = root.sortField;
        Config.options.wallpaperSelector.sortReversed = root.sortReversed;
        Config.saveOptionsNow();
        root.applyNativeSort();
        root.rebuildSortedFolderModel();
        root.rebuildRyogamiLibraryModel();
        root.sortChanged();
    }

    property list<string> videoExtensions: [
        "mp4", "mkv", "webm", "avi", "mov", "m4v", "ogv"
    ]

    /**
     * The image the shell is actually showing.
     *
     * `Config.options.background.wallpaperPath` stays empty until the user picks
     * a wallpaper, and the shipped default is what fills that gap: BackgroundRoot,
     * ConfigWallpaperSelector, ConfigBannerSelector and WallpaperDirectoryItem all
     * resolve it to `assets/images/default_wallpaper.png`, and switchwall.sh does
     * the same when it is handed no image. The colour previews were the one path
     * that did not, which is why a first install had nothing to derive them from.
     */
    // Under the Ryoku mount the live picture is the provider's (ryogami owns
    // the plane); the island's own option answers for everything else.
    readonly property string effectiveWallpaperPath: {
        const bridged = Config.wallpaperPath;
        if (bridged !== "")
            return bridged;
        const background = Config.options && Config.options.background ? Config.options.background : null;
        if (!background)
            return Directories.defaultWallpaperImagePath;
        if (background.useWallpaperEngine)
            return "/tmp/wpe_screenshot.png";
        const path = String(background.wallpaperPath || "");
        return path !== "" ? path : Directories.defaultWallpaperImagePath;
    }

    // The shell plays video wallpapers itself (VideoWallpaper.qml) instead of
    // handing them to mpvpaper. Wallpaper Engine always stays external.
    readonly property bool videoRenderedByShell: (Config.options?.background?.videoBackend ?? "mpvpaper") === "shell"
        && Config.options?.background?.useWallpaperEngine !== true

    // True while the desktop is painted by another process (mpvpaper or
    // Wallpaper Engine): image effects have nothing to sample and are locked.
    readonly property bool videoWallpaperActive: {
        const background = Config.options && Config.options.background ? Config.options.background : null;
        if (!background) return false;
        return background.useWallpaperEngine === true
            || (root.isVideoFile(background.wallpaperPath || "") && !root.videoRenderedByShell);
    }
    property bool enforcingVideoWallpaperConstraints: false

    // Seconds into `path` that switchwall.sh takes the color/poster frame from.
    function videoFrameTime(path) {
        const entry = (Config.options?.background?.videoFrameTimes ?? []).find(e => e?.path === path);
        return Number(entry?.seconds ?? 0) || 0;
    }

    // Stores the frame time for the current desktop video and regenerates its
    // poster frame and colors, without restarting the running video.
    function applyVideoFrameTime(path, seconds) {
        const value = Math.max(0, Math.round(Number(seconds) * 10) / 10);
        const others = (Config.options.background.videoFrameTimes ?? []).filter(e => e?.path !== path);
        Config.options.background.videoFrameTimes = value > 0 ? [...others, { path: path, seconds: value }] : others;
        Config.saveOptionsNow(); // switchwall.sh reads it from disk
        const envBinPath = `${FileUtils.trimFileProtocol(Directories.home)}/.local/bin:${FileUtils.trimFileProtocol(Directories.home)}/.cargo/bin:/usr/local/bin:/usr/bin:/bin`;
        Quickshell.execDetached([
            "env", "-u", "LD_LIBRARY_PATH", "-u", "PYTHONHOME", "-u", "PYTHONPATH",
            `PATH=${envBinPath}`, "bash", Directories.wallpaperSwitchScriptPath,
            "--noswitch", "--refresh-frame", "--mode", Appearance.m3colors.darkmode ? "dark" : "light"
        ]);
    }

    function isVideoFile(name) {
        const value = String(name || "").toLowerCase();
        return videoExtensions.some(ext => value.endsWith("." + ext));
    }

    function enforceVideoWallpaperConstraints() {
        if (!Config.ready || !root.videoWallpaperActive || root.enforcingVideoWallpaperConstraints)
            return;

        const background = Config.options.background;
        const parallax = background.parallax;
        if (!parallax)
            return;

        root.enforcingVideoWallpaperConstraints = true;

        background.blurWhenWindowsOpen = false;
        background.zoomOutEnabled = false;
        background.zoomOutStyle = 1;
        background.windowZoomOnOverview = false;
        background.windowZoomLiveCapture = false;
        background.cheatsheetZoomOut = false;
        background.overviewZoomOut = false;
        background.workspaceBlur = false;

        parallax.vertical = false;
        parallax.autoVertical = false;
        parallax.enableWorkspace = false;
        parallax.enableSidebar = false;
        parallax.loop = false;
        parallax.invertHorizontal = false;
        parallax.invertVertical = false;
        parallax.workspaceZoom = 1.0;

        root.enforcingVideoWallpaperConstraints = false;
        Config.saveOptionsNow();
    }

    // Executions
    Process {
        id: applyProc
    }

    Connections {
        target: Config
        function onReadyChanged() {
            if (!Config.ready) return;
            root.loadSortOptions();
            // No restore here: ryogami brings back the wallpaper it last applied
            // when it starts. This editor's own record goes stale whenever the
            // wallpaper changes elsewhere (Super+W, random, a rice), so replaying
            // it at every shell start put back a clip the user had replaced.
            root.enforceVideoWallpaperConstraints();
            root.recordRecent(Config.options.background.wallpaperPath);
            // Pre-generate lockscreen colors if configured but missing
            if (Config.options.background.useSeparateLockscreenWallpaper) {
                const lockPath = Config.options.background.lockscreenWallpaperPath;
                const deskPath = Config.options.background.wallpaperPath;
                if (lockPath && lockPath !== "" && lockPath !== deskPath) {
                    lockscreenColorsCheckProc.exec(["test", "-f", Directories.lockscreenColorsPath]);
                }
            }
        }
    }

    Connections {
        target: Config.options ? Config.options.background : null
        enabled: Config.ready
        function onWallpaperPathChanged() {
            root.enforceVideoWallpaperConstraints();
            root.recordRecent(Config.options.background.wallpaperPath);
        }
        function onUseWallpaperEngineChanged() {
            root.enforceVideoWallpaperConstraints();
        }
        // Ryogami owns playback; the backend only changes what the editor locks.
        function onVideoBackendChanged() {
            root.enforceVideoWallpaperConstraints();
        }
    }
    
    // ── Recent wallpapers ────────────────────────────────────────────────────
    // Fed by the config key rather than by apply(), so every way a wallpaper
    // lands (the pickers, switchwall.sh, a mode, a preset) counts. Wallpaper
    // Engine scenes are ids, not files, and stay out.
    readonly property int recentLimit: 7
    readonly property list<string> recentWallpapers: Persistent.ready
        ? Persistent.states.background.recentWallpapers : []

    function recordRecent(path) {
        if (!Persistent.ready || !Config.ready)
            return;
        const clean = FileUtils.trimFileProtocol(String(path || ""));
        if (clean === "" || Config.options.background.useWallpaperEngine)
            return;
        const current = Array.from(Persistent.states.background.recentWallpapers);
        if (current[0] === clean)
            return;
        Persistent.states.background.recentWallpapers =
            [clean].concat(current.filter(p => p !== clean)).slice(0, root.recentLimit);
    }

    // The wallpaper that was already up before this history existed is its
    // first entry; afterwards the change handler above keeps it.
    Connections {
        target: Persistent
        function onReadyChanged() {
            if (Persistent.ready && Config.ready)
                root.recordRecent(Config.options.background.wallpaperPath);
        }
    }

    function openFallbackPicker(darkMode = Appearance.m3colors.darkmode, lockscreen = false) {
        const envBinPath = `${FileUtils.trimFileProtocol(Directories.home)}/.local/bin:${FileUtils.trimFileProtocol(Directories.home)}/.cargo/bin:/usr/local/bin:/usr/bin:/bin`;
        let args = [
            "env", "-u", "LD_LIBRARY_PATH", "-u", "PYTHONHOME", "-u", "PYTHONPATH",
            `PATH=${envBinPath}`, "bash", Directories.wallpaperSwitchScriptPath,
            "--mode", darkMode ? "dark" : "light"
        ];
        if (lockscreen) args.push("--lockscreen");
        args.push("--request-seq", String(++root._wallpaperRequestSeq));
        Quickshell.execDetached(args);
    }

    function apply(path, darkMode = Appearance.m3colors.darkmode) {
        if (!path || path.length === 0) return;
        const isNumericWpeId = /^\d+$/.test(path.trim());
        let optionsChanged = false;
        if (Config.options && Config.options.background) {
            if (isNumericWpeId) {
                if (Config.options.background.useWallpaperEngine !== true) {
                    Config.options.background.useWallpaperEngine = true;
                    optionsChanged = true;
                }
                if (String(Config.options.background.wallpaperEngineId || "") !== path) {
                    Config.options.background.wallpaperEngineId = path;
                    optionsChanged = true;
                }
            } else {
                if (Config.options.background.useWallpaperEngine !== false) {
                    Config.options.background.useWallpaperEngine = false;
                    optionsChanged = true;
                }
                if (String(Config.options.background.wallpaperPath || "") !== path) {
                    Config.options.background.wallpaperPath = path;
                    optionsChanged = true;
                }
            }
        }
        if (optionsChanged) Config.saveOptionsNow();
        const requestSeq = ++root._wallpaperRequestSeq;
        const envBinPath = `${FileUtils.trimFileProtocol(Directories.home)}/.local/bin:${FileUtils.trimFileProtocol(Directories.home)}/.cargo/bin:/usr/local/bin:/usr/bin:/bin`;
        Quickshell.execDetached([
            "env", "-u", "LD_LIBRARY_PATH", "-u", "PYTHONHOME", "-u", "PYTHONPATH",
            `PATH=${envBinPath}`, "bash", Directories.wallpaperSwitchScriptPath,
            "--mode", darkMode ? "dark" : "light", "--image", path,
            "--request-seq", String(requestSeq)
        ]);
        root.changed();
    }

    function applyLockscreen(path, darkMode = Appearance.m3colors.darkmode) {
        if (!path || path.length === 0) return;
        let optionsChanged = false;
        if (Config.options && Config.options.background) {
            if (String(Config.options.background.lockscreenWallpaperPath || "") !== path) {
                Config.options.background.lockscreenWallpaperPath = path;
                optionsChanged = true;
            }
        }
        if (optionsChanged) Config.saveOptionsNow();
        const requestSeq = ++root._wallpaperRequestSeq;
        const envBinPath = `${FileUtils.trimFileProtocol(Directories.home)}/.local/bin:${FileUtils.trimFileProtocol(Directories.home)}/.cargo/bin:/usr/local/bin:/usr/bin:/bin`;
        Quickshell.execDetached([
            "env", "-u", "LD_LIBRARY_PATH", "-u", "PYTHONHOME", "-u", "PYTHONPATH",
            `PATH=${envBinPath}`, "bash", Directories.wallpaperSwitchScriptPath,
            "--mode", darkMode ? "dark" : "light", "--image", path, "--lockscreen", "--noswitch",
            "--request-seq", String(requestSeq)
        ]);
        Quickshell.execDetached([
            "env", "-u", "LD_LIBRARY_PATH", "-u", "PYTHONHOME", "-u", "PYTHONPATH",
            `PATH=${envBinPath}`, "bash", Directories.generateLockscreenColorsScriptPath,
            "--image", path, "--mode", darkMode ? "dark" : "light"
        ]);
        root.changed();
    }

    function applyLightModeWallpaper(path) {
        if (!path || path.length === 0) return;
        let optionsChanged = false;
        if (Config.options && Config.options.background) {
            if (String(Config.options.background.lightModeWallpaperPath || "") !== path) {
                Config.options.background.lightModeWallpaperPath = path;
                optionsChanged = true;
            }
        }
        if (optionsChanged) Config.saveOptionsNow();
        const requestSeq = ++root._wallpaperRequestSeq;
        const envBinPath = `${FileUtils.trimFileProtocol(Directories.home)}/.local/bin:${FileUtils.trimFileProtocol(Directories.home)}/.cargo/bin:/usr/local/bin:/usr/bin:/bin`;
        Quickshell.execDetached([
            "env", "-u", "LD_LIBRARY_PATH", "-u", "PYTHONHOME", "-u", "PYTHONPATH",
            `PATH=${envBinPath}`, "bash", Directories.wallpaperSwitchScriptPath,
            "--mode", "light", "--image", path, "--lightmode",
            "--request-seq", String(requestSeq)
        ]);
        root.changed();
    }

    Connections {
        target: Appearance.m3colors
        function onDarkmodeChanged() {
            if (!Config.options || !Config.options.background) return;
            if (!Config.options.background.useSeparateLightModeWallpaper) return;
            const lightPath = Config.options.background.lightModeWallpaperPath;
            const darkPath = Config.options.background.wallpaperPath;
            
            if (Appearance.m3colors.darkmode) {
                // Switched to dark mode — apply dark wallpaper
                if (darkPath && darkPath !== "") {
                    root.apply(darkPath, true);
                }
            } else {
                // Switched to light mode — apply light wallpaper
                if (lightPath && lightPath !== "") {
                    root.applyLightModeWallpaper(lightPath);
                }
            }
        }
    }

    function select(filePath, darkMode = Appearance.m3colors.darkmode) {
        if (!filePath || filePath.length === 0) return;
        const cleanPath = FileUtils.trimFileProtocol(filePath);
        if (Config.options?.background?.useSeparateLightModeWallpaper && !Appearance.m3colors.darkmode) {
            root.applyLightModeWallpaper(cleanPath);
        } else {
            root.apply(cleanPath, darkMode);
        }
    }
    function applyForScreen(filePath, screenName, darkMode) {
        const envBinPath = `${FileUtils.trimFileProtocol(Directories.home)}/.local/bin:${FileUtils.trimFileProtocol(Directories.home)}/.cargo/bin:/usr/local/bin:/usr/bin:/bin`;
        Quickshell.execDetached([
            "env", "-u", "LD_LIBRARY_PATH", "-u", "PYTHONHOME", "-u", "PYTHONPATH",
            `PATH=${envBinPath}`, "bash", Directories.wallpaperSwitchScriptPath,
            "--mode", darkMode ? "dark" : "light", "--image", filePath,
            "--screen", screenName, "--request-seq", String(++root._wallpaperRequestSeq)
        ]);
        root.changed();
    }

    // A forced same-path write pins a wildcard-backed output before the
    // wildcard changes; it has no visual delta and therefore no history entry.
    function selectForScreen(filePath, screenName, darkMode = Appearance.m3colors.darkmode, force = false) {
        const cleanPath = FileUtils.trimFileProtocol(String(filePath ?? ""));
        const monitor = String(screenName ?? "");
        if (cleanPath === "" || monitor === "")
            return false;
        const previous = root.currentWallpaperPath(monitor);
        if (previous === cleanPath && !force) {
            root.changed();
            return false;
        }
        root.applyForScreen(cleanPath, monitor, darkMode);
        if (previous !== cleanPath) {
            GlobalStates.editHistoryPush({
                "undo": () => root.applyForScreen(previous, monitor, darkMode),
                "redo": () => root.applyForScreen(cleanPath, monitor, darkMode)
            });
        }
        return true;
    }


    function selectLockscreen(filePath, darkMode = Appearance.m3colors.darkmode) {
        if (!filePath || filePath.length === 0) return;
        const cleanPath = FileUtils.trimFileProtocol(filePath);
        root.applyLockscreen(cleanPath, darkMode);
    }

    function selectLightmode(filePath, darkMode = Appearance.m3colors.darkmode) {
        if (!filePath || filePath.length === 0) return;
        const cleanPath = FileUtils.trimFileProtocol(filePath);
        if (Config.options?.background?.useSeparateLightModeWallpaper && !Appearance.m3colors.darkmode) {
            root.applyLightModeWallpaper(cleanPath);
        } else {
            Quickshell.execDetached([Directories.wallpaperSwitchScriptPath, "--mode", darkMode ? "dark" : "light", "--image", cleanPath, "--lightmode", "--noswitch",
                "--request-seq", String(++root._wallpaperRequestSeq)]);
            root.changed()
        }
    }

    function randomFromCurrentFolder(darkMode = Appearance.m3colors.darkmode) {
        const candidates = [];
        for (let i = 0; i < folderModel.count; i++) {
            if (Boolean(folderModel.get(i, "fileIsDir"))) continue;

            const filePath = String(folderModel.get(i, "filePath") || FileUtils.trimFileProtocol(folderModel.get(i, "fileUrl") || folderModel.get(i, "fileURL") || ""));
            const fileName = String(folderModel.get(i, "fileName") || filePath).toLowerCase();
            if (!filePath || !root.extensions.some(ext => fileName.endsWith("." + ext))) continue;
            candidates.push(filePath);
        }

        if (candidates.length === 0) return;
        const filePath = candidates[Math.floor(Math.random() * candidates.length)];
        print("Randomly selected wallpaper:", filePath);
        root.select(filePath, darkMode);
    }

    Process {
        id: validateDirProc
        property string nicePath: ""
        function setDirectoryIfValid(path) {
            validateDirProc.nicePath = FileUtils.trimFileProtocol(path).replace(/\/+$/, "")
            if (/^\/*$/.test(validateDirProc.nicePath)) validateDirProc.nicePath = "/";
            root.directoryError = "";
            validateDirProc.exec([
                "stat", "-c", "%f", "--", validateDirProc.nicePath
            ])
        }
        stdout: StdioCollector {
            onStreamFinished: {
                const result = text.trim().toLowerCase()
                if (result.startsWith("4")) {
                    root.directory = Qt.resolvedUrl(validateDirProc.nicePath)
                } else if (result.startsWith("8")) {
                    root.directory = Qt.resolvedUrl(FileUtils.parentDirectory(validateDirProc.nicePath))
                } else {
                    root.directoryError = Translation.tr("The selected path is not a readable folder or file.");
                }
            }
        }
    }
    function setDirectory(path) {
        validateDirProc.setDirectoryIfValid(path)
    }
    function reloadCurrentDirectory() {
    const current = folderModel.folder
    const currentPath = FileUtils.trimFileProtocol(current.toString())
    const parent = FileUtils.parentDirectory(currentPath) || "/"
    folderModel.lockNextNavigation()
    folderModel.folder = Qt.resolvedUrl(parent)
    folderModel.lockNextNavigation()
    folderModel.folder = current
    }

    function navigateUp() {
        folderModel.navigateUp()
    }
    function navigateBack() {
        folderModel.navigateBack()
    }
    function navigateForward() {
        folderModel.navigateForward()
    }

    // Folder model
    FolderListModelWithHistory {
        id: folderModel
        folder: Qt.resolvedUrl(root.defaultFolder)
        caseSensitive: false
        nameFilters: {
            const queryParts = searchQuery.split(" ").map(s => s.trim()).filter(s => s.length > 0);
            const filterPattern = queryParts.length > 0 ? queryParts.map(s => `*${s}*`).join("") : "*";
            return root.extensions.map(ext => `${filterPattern}.${ext}`);
        }
        showDirs: true
        showDotAndDotDot: false
        showOnlyReadable: true
        sortField: FolderListModel.Time
        sortReversed: false
        onCountChanged: {
            root.wallpapers = []
            for (let i = 0; i < folderModel.count; i++) {
                const path = folderModel.get(i, "filePath") || FileUtils.trimFileProtocol(folderModel.get(i, "fileUrl") || folderModel.get(i, "fileURL"))
                if (path && path.length) root.wallpapers.push(path)
            }
            root.queueFolderModelRefresh();
        }
        onFolderChanged: {
            root.directoryError = "";
            root.queueFolderModelRefresh();
        }
        onStatusChanged: {
            if (folderModel.status === FolderListModel.Ready) {
                root.queueFolderModelRefresh();
            }
        }
    }

    Timer {
        id: folderModelRefreshTimer
        interval: 100
        repeat: false
        onTriggered: root.refreshCreationTimes()
    }

    Process {
        id: creationTimesProc
        stdout: StdioCollector {
            onStreamFinished: {
                const values = text.trim().length > 0 ? text.trim().split(/\r?\n/) : [];
                const nextCreationTimes = ({});
                for (let i = 0; i < root.pendingCreationPaths.length; i++) {
                    const value = Number(values[i] || 0);
                    const ms = (isFinite(value) && value > 0) ? (value < 10000000000 ? value * 1000 : value) : 0;
                    nextCreationTimes[root.pendingCreationPaths[i]] = ms;
                }
                root.creationTimes = nextCreationTimes;
                root.rebuildSortedFolderModel();
            }
        }
    }

    ListModel {
        id: sortedFolderModel
    }
    ListModel {
        id: ryogamiLibraryModel
    }



    Process {
        id: readColorCacheProc
        stdout: StdioCollector {
            onStreamFinished: {
                if (text && text.trim().length > 0) {
                    try {
                        root.colorCache = JSON.parse(text);
                    } catch (e) {
                        console.error("[Wallpapers] Failed to parse color cache:", e);
                    }
                }
            }
        }
    }

    function loadColorCache() {
        const path = Directories.colorCachePath;
        readColorCacheProc.exec(["cat", path]);
    }

    Component.onCompleted: {
        root.loadColorCache();
    }

    // Checks if lockscreen_colors.json exists; if not, generates it in background
    Process {
        id: lockscreenColorsCheckProc
        onExited: (exitCode, exitStatus) => {
            if (exitCode !== 0) {
                // File doesn't exist: generate lockscreen colors in background
                const lockPath = Config.options.background.lockscreenWallpaperPath;
                const mode = Appearance.m3colors.darkmode ? "dark" : "light";
                Quickshell.execDetached(["bash", Directories.generateLockscreenColorsScriptPath, "--image", lockPath, "--mode", mode]);
            }
        }
    }

    IpcHandler {
        target: "wallpapers"

        function apply(path: string): void {
            root.apply(path);
        }

        function applyLockscreen(path: string): void {
            root.applyLockscreen(path);
        }
    }
}
