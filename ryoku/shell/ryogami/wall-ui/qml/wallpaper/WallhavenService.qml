import Quickshell
import Quickshell.Io
import QtQuick
import Ryoku.Ui.Singletons

QtObject {
  id: whService

  required property string wallpaperDir

  property string query: ""
  property string categories: "111"
  property string purity: "100"
  property string sorting: "toplist"
  property string order: "desc"
  property string topRange: "1M"
  property string atleast: ""
  property string ratios: ""
  property int selectedColor: -1
  property int currentPage: 1
  property int lastPage: 1
  property string apiKey: ""

  property var results: []
  property bool loading: false
  property string errorText: ""
  property bool hasMore: currentPage < lastPage

  readonly property string _userAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

  property var downloadStatus: ({})
  property var downloadProgress: ({})
  property var localWallhavenIds: ({})

  signal resultsUpdated()
  signal downloadFinished(string wallhavenId, string localPath)

  function scanLocalFiles() {
    _localScanOutput = ""
    _localScanProc.running = true
  }

  property string _localScanOutput: ""
  property var _localScanProc: Process {
    command: ["find", whService.wallpaperDir, "-maxdepth", "1", "-name", "wallhaven-*", "-not", "-name", "*.tmp", "-printf", "%f\n"]
    stdout: SplitParser {
      onRead: data => { whService._localScanOutput += data + "\n" }
    }
    onExited: function(exitCode, exitStatus) {
      var ids = {}
      var lines = whService._localScanOutput.split("\n")
      for (var i = 0; i < lines.length; i++) {
        var fname = lines[i].trim()
        if (!fname) continue
        var m = fname.match(/^wallhaven-([a-zA-Z0-9]+)/)
        if (m) ids[m[1]] = true
      }
      whService.localWallhavenIds = ids
    }
  }

  function search(page) {
    if (loading) { console.log("[WH] search() skipped - already loading"); return }
    currentPage = page || 1
    if (currentPage === 1) results = []
    loading = true
    errorText = ""
    console.log("[WH] search() page=" + currentPage + "  url=" + _buildUrl())
    _searchProcess.running = true
  }

  function loadMore() {
    if (loading || !hasMore) return
    search(currentPage + 1)
  }

  function clearCache() {
    results = []
    currentPage = 1
    lastPage = 1
    errorText = ""
  }

  function nextPage() {
    loadMore()
  }
  function prevPage() {
    if (loading || currentPage <= 1) return
    search(currentPage - 1)
  }

  readonly property var _allowedExts: ({"jpg": true, "jpeg": true, "png": true, "webp": true, "gif": true, "bmp": true})
  readonly property var _allowedHosts: ["w.wallhaven.cc"]

  function downloadWallpaper(wallhavenId, fullUrl) {
    if (_activeDownloads[wallhavenId]) return

    var urlObj
    try {
        urlObj = new URL(fullUrl)
    } catch (e) {
        console.warn("[WH] download refused: unparseable url id=" + wallhavenId)
        return
    }
    if (urlObj.protocol !== "https:") {
        console.warn("[WH] download refused: not https id=" + wallhavenId + " url=" + fullUrl)
        return
    }
    var hostOk = _allowedHosts.some(function(h) { return urlObj.hostname === h })
    if (!hostOk) {
        // A silent refusal here is indistinguishable from a dead network in a
        // user report; name the host so a changed wallhaven CDN is visible.
        console.warn("[WH] download refused: host not allowlisted id=" + wallhavenId + " host=" + urlObj.hostname)
        return
    }

    var ext = fullUrl.split(".").pop().split("?")[0].toLowerCase()
    if (!_allowedExts[ext]) ext = "jpg"

    var safeId = wallhavenId.replace(/[^a-zA-Z0-9]/g, "")
    if (!safeId) return

    var dest = wallpaperDir + "/wallhaven-" + safeId + "." + ext
    var status = Object.assign({}, downloadStatus)
    status[wallhavenId] = "downloading"
    downloadStatus = status
    _activeDownloads[wallhavenId] = { dest: dest }
    _downloadQueue.push({ id: wallhavenId, url: fullUrl, dest: dest })
    _drainDownloadQueue()
  }

  property var _activeDownloads: ({})
  property var _downloadQueue: []
  property int _runningDownloads: 0
  readonly property int _maxConcurrent: 3

  function _drainDownloadQueue() {
    while (_runningDownloads < _maxConcurrent && _downloadQueue.length > 0) {
      var job = _downloadQueue.shift()
      _runningDownloads++
      _spawnDownload(job.id, job.url, job.dest)
    }
  }

  function _spawnDownload(whId, url, dest) {
    var tmpDest = dest + ".tmp"
    var comp = Qt.createComponent("WallhavenDownloadProc.qml")
    var proc = comp.createObject(whService, { whId: whId, dest: dest, tmpDest: tmpDest })
    proc.command = ["curl", "-#", "-fSL", "-A", whService._userAgent, "-o", tmpDest, url]
    proc.onProgressUpdate.connect(function(id, pct) {
      var p = Object.assign({}, downloadProgress)
      p[id] = pct
      downloadProgress = p
    })
    proc.onDone.connect(function(id, success) {
      _runningDownloads--
      var s = Object.assign({}, downloadStatus)
      if (success) {
        s[id] = "done"
        downloadStatus = s
        downloadFinished(id, _activeDownloads[id] ? _activeDownloads[id].dest : "")
        scanLocalFiles()
      } else {
        s[id] = "error"
        downloadStatus = s
      }
      proc.destroy()
      _drainDownloadQueue()
    })
    proc.running = true
  }

  readonly property var _hueToWallhavenColor: ({
    0:  "cc0000",
    1:  "ff6600",
    2:  "ffcc33",
    3:  "77cc33",
    4:  "669900",
    5:  "66cccc",
    6:  "0099cc",
    7:  "0066cc",
    8:  "333399",
    9:  "663399",
    10: "993399",
    11: "ea4c88",
    99: "999999"
  })

  function _buildUrl() {
    var url = "https://wallhaven.cc/api/v1/search?"
    var params = []
    if (query) params.push("q=" + encodeURIComponent(query))
    params.push("categories=" + categories)
    params.push("purity=" + purity)
    params.push("sorting=" + sorting)
    params.push("order=" + order)
    if (sorting === "toplist" && topRange) params.push("topRange=" + topRange)
    if (atleast) params.push("atleast=" + atleast)
    if (ratios) params.push("ratios=" + ratios)
    if (selectedColor >= 0 && _hueToWallhavenColor[selectedColor])
      params.push("colors=" + _hueToWallhavenColor[selectedColor])
    params.push("page=" + currentPage)
    if (apiKey) params.push("apikey=" + apiKey)
    return url + params.join("&")
  }

  property string _searchOutput: ""

  property var _searchProcess: Process {
    command: ["curl", "-fsSL", "-A", whService._userAgent, whService._buildUrl()]
    stdout: SplitParser {
      splitMarker: ""
      onRead: data => { whService._searchOutput += data }
    }
    onRunningChanged: {
      if (running) { whService._searchOutput = ""; console.log("[WH] curl started") }
    }
    onExited: function(exitCode, exitStatus) {
      console.log("[WH] curl exited code=" + exitCode + "  bytes=" + whService._searchOutput.length)
      whService.loading = false
      if (exitCode !== 0) {
        whService.errorText = I18n.tr("Network error (curl exit %1)").arg(exitCode)
        console.log("[WH] ERROR: " + whService.errorText)
        whService.resultsUpdated()
        return
      }
      try {
        var json = JSON.parse(whService._searchOutput)
        if (json.error) {
          whService.errorText = json.error
        } else {
          var newItems = (json.data || []).map(function(item) {
            return {
              id: item.id,
              url: item.url,
              path: item.path,
              resolution: item.resolution,
              fileSize: item.file_size,
              purity: item.purity,
              category: item.category,
              thumbLarge: item.thumbs ? item.thumbs.large : "",
              thumbSmall: item.thumbs ? item.thumbs.small : "",
              colors: item.colors || []
            }
          })
          whService.results = newItems
          whService.lastPage = (json.meta && json.meta.last_page) ? json.meta.last_page : 1
          whService.currentPage = (json.meta && json.meta.current_page) ? json.meta.current_page : 1
          whService.errorText = ""
          console.log("[WH] parsed " + newItems.length + " new items, total=" + whService.results.length + "  lastPage=" + whService.lastPage)
        }
      } catch (e) {
        whService.errorText = I18n.tr("Parse error: %1").arg(e.message)
        console.log("[WH] ERROR: " + whService.errorText)
      }
      whService.resultsUpdated()
      whService.scanLocalFiles()
    }
  }
}
