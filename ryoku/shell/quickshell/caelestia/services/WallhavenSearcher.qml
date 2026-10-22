pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Services.Notifications
import Caelestia
import Caelestia.Config
import caelestia.utils

Singleton {
    id: root

    readonly property string apiBase: "https://wallhaven.cc/api/v1"

    property string apiKey: GlobalConfig.services.wallhavenApiKey ?? ""

    property string lastQuery: ""
    property int currentPage: 1
    property int lastPage: 1
    property string lastSeed: ""
    property list<var> results
    property var currentWallpaper: null
    property string activeDownloadId: ""
    property real activeDownloadProgress: 0
    property bool activeDownloadRunning: false

    property var filters: {
        "categories": "111",
        "purity": "100",
        "sorting": "relevance",
        "order": "desc",
        "topRange": "1M",
        "atleast": "",
        "resolutions": "",
        "ratios": "",
        "colors": ""
    }

    signal searchComplete(var results, var meta)
    signal downloadProgress(string id, real progress)
    signal downloadComplete(string id, string path)
    signal downloadFailed(string id, string error)

    function resetDownloadState(id = "") {
        activeDownloadId = id;
        activeDownloadProgress = 0;
        activeDownloadRunning = id !== "";
    }

    function handleDownloadProgress(data) {
        if (!activeDownloadRunning || !activeDownloadId)
            return;

        const progressMatch = data.match(/^PROGRESS\s+([0-9]*\.?[0-9]+)$/);
        if (progressMatch) {
            const progress = Math.max(0, Math.min(1, parseFloat(progressMatch[1])));
            if (!isNaN(progress) && progress >= activeDownloadProgress) {
                activeDownloadProgress = progress;
                downloadProgress(activeDownloadId, progress);
            }
            return;
        }

        const match = data.match(/(\d{1,3}(?:\.\d+)?)%/);
        if (!match)
            return;

        const percent = Math.max(0, Math.min(100, parseFloat(match[1])));
        const progress = percent / 100;
        if (isNaN(progress) || progress < activeDownloadProgress)
            return;

        activeDownloadProgress = progress;
        downloadProgress(activeDownloadId, progress);
    }

    function buildUrl(path: string, params: var): string {
        let url = apiBase + path + "?";
        const paramList = [];

        for (const key in params) {
            if (params[key]) {
                paramList.push(`${key}=${encodeURIComponent(params[key])}`);
            }
        }

        if (apiKey) {
            paramList.push(`apikey=${apiKey}`);
        }

        return url + paramList.join("&");
    }

    function search(query: string, page: int): void {
        if (!query || query.trim() === "")
            return;

        if (!page || page < 1)
            page = 1;

        lastQuery = query;
        currentPage = page;

        const params = {
            "q": query,
            "categories": filters.categories,
            "purity": filters.purity,
            "sorting": filters.sorting,
            "order": filters.order,
            "page": page.toString()
        };

        if (filters.atleast)
            params.atleast = filters.atleast;
        if (filters.resolutions)
            params.resolutions = filters.resolutions;
        if (filters.ratios)
            params.ratios = filters.ratios;
        if (filters.colors)
            params.colors = filters.colors;

        const url = buildUrl("/search", params);
        Logger.log("Wallhaven search:", url.replace(/([?&])apikey=[^&]*/, "$1apikey=<redacted>"));

        runSearch(url);
    }

    function searchRandom(query: string): void {
        if (!query || query.trim() === "")
            return;

        lastQuery = query;
        currentPage = 1;

        const params = {
            "q": query,
            "categories": filters.categories,
            "purity": filters.purity,
            "sorting": "random",
            "page": "1"
        };

        const url = buildUrl("/search", params);
        Logger.log("Wallhaven random:", url.replace(/([?&])apikey=[^&]*/, "$1apikey=<redacted>"));

        runSearch(url);
    }

    function searchNextPage(): void {
        if (currentPage < lastPage && lastQuery) {
            if (filters.sorting === "random" && lastSeed) {
                currentPage++;
                const params = {
                    "q": lastQuery,
                    "categories": filters.categories,
                    "purity": filters.purity,
                    "sorting": "random",
                    "seed": lastSeed,
                    "page": currentPage.toString()
                };
                const url = buildUrl("/search", params);
                loadPage(url);
            } else {
                search(lastQuery, currentPage + 1);
            }
        }
    }

    function runSearch(url: string): void {
        Requests.get(url, text => {
            try {
                const json = JSON.parse(text);
                results = json.data || [];

                if (json.meta) {
                    lastPage = json.meta.last_page || 1;
                    lastSeed = json.meta.seed || "";
                }

                searchComplete(results, json.meta || {});
            } catch (e) {
                console.error("Wallhaven parse error:", e);
                searchComplete([], {});
            }
        }, () => searchComplete([], {}));
    }

    function loadPage(url: string): void {
        runSearch(url);
    }

    function setFilter(key: string, value: string): void {
        const newFilters = {
            "categories": filters.categories,
            "purity": filters.purity,
            "sorting": filters.sorting,
            "order": filters.order,
            "topRange": filters.topRange,
            "atleast": filters.atleast,
            "resolutions": filters.resolutions,
            "ratios": filters.ratios,
            "colors": filters.colors
        };
        newFilters[key] = value;
        filters = newFilters;
    }

    function setPurity(sfw: bool, sketchy: bool, nsfw: bool): void {
        if (!apiKey && (nsfw || sketchy)) {
            console.warn("Wallhaven: Sketchy/NSFW requires API key");
        }
        let p = (sfw ? "1" : "0") + (sketchy ? "1" : "0") + (nsfw && apiKey ? "1" : "0");
        setFilter("purity", p);
    }

    function setResolution(width: int, height: int): void {
        if (width > 0 && height > 0) {
            setFilter("atleast", `${width}x${height}`);
        } else {
            setFilter("atleast", "");
        }
    }

    function setSorting(sorting: string): void {
        setFilter("sorting", sorting);
    }

    function resetFilters(): void {
        filters = {
            "categories": "111",
            "purity": apiKey ? "110" : "100",
            "sorting": "relevance",
            "order": "desc",
            "topRange": "1M",
            "atleast": "",
            "resolutions": "",
            "ratios": "",
            "colors": ""
        };
    }

    function downloadWallpaper(wallpaper: var): void {
        if (!wallpaper || !wallpaper.path) {
            console.error("Wallhaven: Invalid wallpaper data");
            return;
        }

        const fullPath = wallpaper.path || wallpaper.url || "";
        const urlMatch = fullPath.match(/\.([a-zA-Z]{3,4})(?:\?|$)/);
        let ext = urlMatch ? urlMatch[1] : "";
        if (ext) {
            ext = ext.toLowerCase();
            if (ext === "jpeg")
                ext = "jpg";
        } else {
            ext = "jpg";
        }

        const tmpPath = `${Paths.cache}/wallhaven-${wallpaper.id}.tmp`;
        const dstPath = `${Paths.wallsdir}/wallhaven-${wallpaper.id}.${ext}`;

        currentWallpaper = {
            id: wallpaper.id,
            ext: ext
        };
        if (activeDownloadRunning || downloadProc.running) {
            console.warn("Wallhaven: Download already in progress");
            return;
        }
        resetDownloadState(wallpaper.id);
        downloadProc.wallpaperId = wallpaper.id;
        downloadProc.tmpPath = tmpPath;
        downloadProc.dstPath = dstPath;

        Logger.log("Wallhaven: Downloading", wallpaper.path, "to", tmpPath);
        Logger.log("Wallhaven: Will move to", dstPath, "(ext:", ext, ")");

        downloadProc.command = [
            "python3",
            "-c",
            'import math, os, sys, urllib.request\nurl, tmp_path, dst_path = sys.argv[1:4]\nos.makedirs(os.path.dirname(dst_path), exist_ok=True)\ntry:\n    req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36"})\n    with urllib.request.urlopen(req) as response, open(tmp_path, "wb") as handle:\n        total = response.headers.get("Content-Length")\n        total = int(total) if total else 0\n        downloaded = 0\n        while True:\n            chunk = response.read(262144)\n            if not chunk:\n                break\n            handle.write(chunk)\n            downloaded += len(chunk)\n            if total > 0:\n                print(f"PROGRESS {downloaded / total:.6f}", flush=True)\n            else:\n                pseudo = min(0.92, 1.0 - math.exp(-downloaded / 1200000.0))\n                print(f"PROGRESS {pseudo:.6f}", flush=True)\n    print("PROGRESS 1", flush=True)\n    os.replace(tmp_path, dst_path)\nexcept Exception as exc:\n    try:\n        if os.path.exists(tmp_path):\n            os.remove(tmp_path)\n    except OSError:\n        pass\n    print(str(exc), file=sys.stderr, flush=True)\n    sys.exit(1)',
            wallpaper.path,
            tmpPath,
            dstPath
        ];
        downloadProc.running = true;
    }

    IpcHandler {
        function doSearch(query: string): void {
            search(query);
        }

        function doRandom(query: string): void {
            searchRandom(query);
        }

        target: "wallhaven"
    }

    Process {
        id: downloadProc

        property string wallpaperId: ""
        property string tmpPath: ""
        property string dstPath: ""

        stdout: SplitParser {
            onRead: data => root.handleDownloadProgress(data)
        }

        stderr: StdioCollector {
            id: downloadErr
        }

        // qmllint disable signal-handler-parameters
        onExited: code => {
            if (code !== 0) {
                const errorText = downloadErr.text.trim();
                resetDownloadState();
                currentWallpaper = null;
                downloadFailed(wallpaperId, errorText ? errorText : ("Download failed: " + code));
                return;
            }
            if (currentWallpaper) {
                const dst = downloadProc.dstPath;
                activeDownloadProgress = 1;
                downloadProgress(wallpaperId, 1);
                Logger.log("Wallhaven: Download complete", dst);
                resetDownloadState();
                downloadComplete(wallpaperId, dst);
            }
            currentWallpaper = null;
        }
    }
}
