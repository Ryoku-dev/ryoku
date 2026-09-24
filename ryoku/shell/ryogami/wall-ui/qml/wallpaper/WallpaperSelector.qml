import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import QtQuick
import QtQuick.Shapes
import QtQuick.Effects
import QtQuick.Controls
import QtMultimedia
import Qt.labs.folderlistmodel
import ".."
import "../services"
import Ryoku.Ui.Singletons

Scope {
  id: wallpaperSelector

  property var colors
  property bool showing: false
  property alias selectedColorFilter: service.selectedColorFilter
  property alias selectorService: service
  property alias swService: swService
  property alias _whService: whService
  // Which monitor the picker opens on. Config.mainMonitor is the monitor a
  // wallpaper APPLIES to by default, which is a different question and is
  // usually unset (so it fell through to screens[0], the internal panel, no
  // matter which display you were on). The surface has to follow the focus,
  // like every other shell surface does, or Super+W opens on the wrong screen
  // whenever the external display is the one being used (#160). Read at open
  // time, not bound, so the panel does not hop mid-session when focus moves.
  property string mainMonitor: Config.mainMonitor
  property string openMonitor: ""
  function firstScreenName() {
      return Quickshell.screens.length > 0 ? String(Quickshell.screens[0].name) : "";
  }
  // The focused output comes from the daemon: no Wayland protocol reports it to
  // a client. Answers after the first paint, so the first screen is latched
  // first and corrected when the reply lands.
  function latchFocusedMonitor() {
      openMonitor = firstScreenName()
      DaemonClient.call("wm.focusedOutput", {}, function(result, err) {
          if (!err && result && result.name)
              openMonitor = String(result.name)
      })
  }
  property string _activeThemeName: ""
  property var _stageWalls: ({})
  signal wallpaperChanged()
  signal uiReady()

  FileView {
    id: shellFile
    path: Quickshell.env("HOME") + "/.config/ryoku/shell.json"
    watchChanges: true
    onFileChanged: reload()
    onLoaded: {
      try {
        var d = JSON.parse(shellFile.text())
        wallpaperSelector._activeThemeName = (d.theme && d.theme.theme) || ""
      } catch (e) {}
    }
  }

  FileView {
    id: stageFile
    property string _stateHome: {
      var x = Quickshell.env("XDG_STATE_HOME")
      return (x && x.length > 0) ? x : (Quickshell.env("HOME") + "/.local/state")
    }
    path: _stateHome + "/ryoku/stage-walls.json"
    watchChanges: true
    onFileChanged: reload()
    onLoaded: {
      try {
        var d = JSON.parse(stageFile.text())
        wallpaperSelector._stageWalls = (d && d.walls) ? d.walls : ({})
      } catch (e) {
        wallpaperSelector._stageWalls = ({})
      }
    }
    onLoadFailed: wallpaperSelector._stageWalls = ({})
  }

  // A wall carries the stage badge when its Ryostage effect is on (Depth or
  // Parallax); off or unlisted walls show nothing.
  function _isStageWall(path) {
    var w = path && wallpaperSelector._stageWalls && wallpaperSelector._stageWalls[path]
    return !!(w && w.effect && w.effect !== "off")
  }

  function _resetFilters() {
    service.selectedColorFilter = -1
    service.selectedTypeFilter = ""
  }

  function _closeAfterApply() {
    if (Config.closeOnSelection) wallpaperSelector.showing = false
  }

  function _applyItem(item, forcePicker) {
    if (item && item.kind === "theme") {
      Quickshell.execDetached(["ryoku-shell", "theme", item.id])
      wallpaperSelector.themesOpen = false
      _closeAfterApply()
      return
    }
    if (item && item.kind === "rice") {
      wallpaperSelector._workshopRice = {
        slug: "" + item.slug, name: "" + (item.name || item.slug || ""),
        author: "" + (item.author || ""), blurb: "" + (item.blurb || ""),
        tags: "" + (item.tags || ""), createdWith: "" + (item.createdWith || ""),
        compat: "" + (item.compat || ""), active: item.active === true,
        live: item.live === true, preview: "" + (item.preview || "")
      }
      wallpaperSelector._workshopOpen = true
      return
    }
    if (forcePicker || Config.wallpaperPerMonitor) {
      _monitorPicker.open(item)
      return
    }
    _doApply(item, null, null, null)
  }

  function _doApply(item, outputs, audioMap, volumeMap) {
    if (item.type === "we") service.applyWE(item.weId, outputs, audioMap, volumeMap)
    else if (item.type === "video") service.applyVideo(item.path, outputs, audioMap, volumeMap)
    else service.applyStatic(item.path, outputs)
  }

  function resetScroll() {
    sliceListView.currentIndex = 0
    if (service.filteredModel.count > 0)
      sliceListView.positionViewAtIndex(0, ListView.Center)
  }
  WallhavenService {
    id: whService
    wallpaperDir: Config.wallpaperDir
    apiKey: Config.wallhavenApiKey
  }

  SteamWorkshopService {
    id: swService
    weDir: Config.weDir
    apiKey: Config.steamApiKey
  }
  WallpaperSelectorService {
    id: service
    scriptsDir: Config.scriptsDir
    homeDir: Config.homeDir
    wallpaperDir: Config.wallpaperDir
    videoDir: Config.videoDir
    cacheBaseDir: Config.cacheDir
    weDir: Config.weDir
    weAssetsDir: Config.weAssetsDir
    showing: wallpaperSelector.showing
    onModelUpdated: {
      if (wallpaperSelector.showing && !wallpaperSelector.cardVisible) {
        wallpaperSelector.suppressWidthAnim = true
        wallpaperSelector.cardVisible = true
      }
      if (service.filteredModel.count > 0) {
        var idx = 0
        if (wallpaperSelector._restorePending) {
          wallpaperSelector._restorePending = false
        } else if (wallpaperSelector.showing && wallpaperSelector._preCommitIndex >= 0) {
          idx = Math.min(wallpaperSelector._preCommitIndex, service.filteredModel.count - 1)
        }
        wallpaperSelector._preCommitIndex = -1
        sliceListView.currentIndex = idx
        _positionTimer.posIdx = idx
        _positionTimer.restart()
      }
      if (service.filterTransitioning) {
        _snapshotFadeOut.start()
      }
    }
    onWallpaperApplied: {
      wallpaperSelector.wallpaperChanged()
      _closeAfterApply()
    }
    onWallpaperApplyFailed: function(message) {
      console.warn("WallpaperSelector: keeping selector open after apply failure:", message)
    }
  }

  ListModel { id: themeModel }
  ListModel { id: riceModel }

  // The theme and rice strips are read from the shell's library on every open
  // and again whenever their folders change while the picker is up, so a scheme
  // or rice installed from Ryostore appears without closing and reopening.
  // FolderListModel watches the directory natively (no inotifywait dependency);
  // its count flips as entries land or leave, and the debounce lets an install
  // finish its stage-and-rename before the catalog is asked.
  function _loadThemes() {
    if (_themeProc.running) return
    _themeBuf = ""
    _themeProc.running = true
  }
  function _loadRices() {
    if (_riceProc.running) return
    _riceBuf = ""
    _riceProc.running = true
  }
  function _reloadRices() {
    riceModel.clear()
    _loadRices()
  }

  readonly property string _dataHome: {
    var x = Quickshell.env("XDG_DATA_HOME")
    return (x && x.length > 0) ? x : (Quickshell.env("HOME") + "/.local/share")
  }
  readonly property string _configHome: {
    var x = Quickshell.env("XDG_CONFIG_HOME")
    return (x && x.length > 0) ? x : (Quickshell.env("HOME") + "/.config")
  }
  FolderListModel {
    id: themeLibrary
    folder: "file://" + wallpaperSelector._dataHome + "/ryoku/themes"
    showFiles: false
    showDirs: true
    showDotAndDotDot: false
    onCountChanged: if (wallpaperSelector.showing) _themeReload.restart()
  }
  FolderListModel {
    id: riceLibrary
    folder: "file://" + wallpaperSelector._configHome + "/ryoku/rices"
    showFiles: false
    showDirs: true
    showDotAndDotDot: false
    onCountChanged: if (wallpaperSelector.showing) _riceReload.restart()
  }
  Timer { id: _themeReload; interval: 600; onTriggered: wallpaperSelector._loadThemes() }
  function _captureRice(name) {
    Quickshell.execDetached(["ryoku-hub", "rice", "capture", name, "all"])
    _riceReload.restart()
  }
  function _deleteRice(slug) {
    Quickshell.execDetached(["ryoku-hub", "rice", "delete", slug])
    _riceReload.restart()
  }
  Timer { id: _riceReload; interval: 1200; onTriggered: wallpaperSelector._reloadRices() }

  property string _themeBuf: ""
  Process {
    id: _themeProc
    command: ["ryoku-shell", "theme", "catalog"]
    stdout: SplitParser {
      splitMarker: ""
      onRead: data => wallpaperSelector._themeBuf += data
    }
    onExited: {
      themeModel.clear()
      var list = []
      try { list = JSON.parse(wallpaperSelector._themeBuf) || [] } catch (e) { list = [] }
      for (var i = 0; i < list.length; i++) {
        var t = list[i]
        if (t.dynamic === true) continue
        // The catalog reports the preview art beside an installed scheme (the
        // store writes the catalogue's image there); an empty path leaves the
        // card to its palette pills.
        var preview = "" + (t.preview || "")
        themeModel.append({
          id: "" + t.id,
          name: "" + (t.label || t.id),
          provider: "" + (t.provider || ""),
          sw: (t.sw || []).join(","),
          dark: t.dark === true,
          kind: "theme",
          type: "static",
          weId: "",
          favourite: false,
          videoFile: "",
          path: preview,
          thumb: preview
        })
      }
      if (wallpaperSelector.themesOpen) _bindActiveViewModel()
    }
  }

  property string _riceBuf: ""
  Process {
    id: _riceProc
    command: ["ryoku-hub", "rice", "list"]
    stdout: SplitParser {
      splitMarker: ""
      onRead: data => wallpaperSelector._riceBuf += data
    }
    onExited: {
      riceModel.clear()
      var list = []
      try { list = JSON.parse(wallpaperSelector._riceBuf) || [] } catch (e) { list = [] }
      for (var i = 0; i < list.length; i++) {
        var r = list[i]
        var p = ("" + (r.preview || "")).replace(/^file:\/\//, "")
        riceModel.append({
          slug: "" + r.slug,
          name: "" + (r.name || r.slug || ""),
          kind: "rice",
          type: "static",
          weId: "",
          favourite: false,
          videoFile: "",
          path: p,
          thumb: p,
          author: "" + (r.author || ""),
          blurb: "" + (r.blurb || ""),
          tags: (r.tags && r.tags.length) ? r.tags.join(", ") : "",
          createdWith: "" + (r.createdWith || ""),
          compat: "" + (r.compat || ""),
          active: r.active === true,
          live: r.live === true,
          preview: "" + (r.preview || "")
        })
      }
      if (wallpaperSelector.ricesOpen) _bindActiveViewModel()
    }
  }

  onShowingChanged: {
    if (showing) {
      // latch the monitor before anything paints: the surface must land where
      // the user is looking, and must not then chase focus while open.
      wallpaperSelector.latchFocusedMonitor()
      _filterBarManuallyShown = Config.filterBarAlwaysVisible
      _restorePending = true
      _bindActiveViewModel()
      _loadThemes()
      _loadRices()
      service.startCacheCheck()
      cardShowTimer.restart()
    } else {
      cardShowTimer.stop()
      cardVisible = false
      settingsOpen = false
      if (gridBackOverlay.overlayOpen) { gridBackOverlay.overlayOpen = false; gridBackOverlay.visible = false; gridBackOverlay.overlayItemKey = "" }
      sliceListView.model = null
      thumbGridView.cacheBuffer = 0
      thumbGridView.model = null
      hexListView.model = null
      gc()
    }
  }
  Connections {
    target: service
    function onRequestFilterUpdate() {
      if (service.filterTransitioning) {
        _snapshotFadeOut.stop()
        _snapshotImage.visible = false
        _snapshotImage.source = ""
      }

      wallpaperSelector._preCommitIndex = sliceListView.currentIndex

      if (service._skipCrossfade || service.filteredModel.count === 0 || !wallpaperSelector.cardVisible || wallpaperSelector.anyBrowserOpen || !wallpaperSelector.isSliceMode) {
        service._skipCrossfade = false
        service.filterTransitioning = false
        service.commitFilteredModel()
        return
      }

      service.filterTransitioning = true
      _snapshotCommitFallback.restart()
      sliceListView.grabToImage(function(result) {
        _snapshotCommitFallback.stop()
        _snapshotImage.source = result.url
        _snapshotImage.visible = true
        _snapshotImage.opacity = 1.0
        service.commitFilteredModel()
      })
    }
  }

  NumberAnimation {
    id: _snapshotFadeOut
    target: _snapshotImage
    property: "opacity"
    from: 1; to: 0
    duration: Style.animNormal
    easing.type: Easing.OutCubic
    onFinished: {
      _snapshotImage.visible = false
      _snapshotImage.source = ""
      service.filterTransitioning = false
    }
  }

  Timer {
    id: _snapshotCommitFallback
    interval: 150
    onTriggered: {
      if (service.filterTransitioning) {
        _snapshotImage.visible = false
        _snapshotImage.source = ""
        service.commitFilteredModel()
        service.filterTransitioning = false
      }
    }
  }

  Timer {
    id: cardShowTimer
    interval: 4000
    onTriggered: wallpaperSelector.cardVisible = true
  }

  Timer {
    id: _positionTimer
    property int posIdx: 0
    interval: 0
    onTriggered: {
      console.log("[TIMER] posIdx=", posIdx, "count=", sliceListView.count, "visible=", sliceListView.visible, "contentX=", sliceListView.contentX, "width=", sliceListView.width, "height=", sliceListView.height, "contentWidth=", sliceListView.contentWidth)
      sliceListView.positionViewAtIndex(posIdx, ListView.Center)
      console.log("[TIMER] after position: contentX=", sliceListView.contentX)
      wallpaperSelector.suppressWidthAnim = false
    }
  }

  function _focusActiveList() {
    if (isHexMode) hexListView.forceActiveFocus()
    else if (isGridMode) thumbGridView.forceActiveFocus()
    else if (isHandMode) handView.forceActiveFocus()
    else if (isSandyMode) sandyView.forceActiveFocus()
    else if (isGridLayoutsMode) gridLayoutsView.forceActiveFocus()
    else sliceListView.forceActiveFocus()
  }

  Timer {
    id: focusTimer
    interval: 50
    onTriggered: wallpaperSelector._focusActiveList()
  }
  property int sliceWidth: Config.wallpaperSliceWidth
  Behavior on sliceWidth { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }
  property int expandedWidth: Config.wallpaperExpandedWidth
  Behavior on expandedWidth { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }
  property int sliceHeight: Config.wallpaperSliceHeight
  Behavior on sliceHeight { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }
  property int skewOffset: Config.wallpaperSkewOffset
  Behavior on skewOffset { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }
  property int sliceSpacing: Config.wallpaperSliceSpacing
  Behavior on sliceSpacing { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }
  // Negative spacing overlaps the slices (the stacked look); that is legitimate
  // down to a pitch of one pixel. Beyond that the delegates stack on top of each
  // other and the row collapses, so the effective spacing is floored at
  // -(sliceWidth - 1) and the pitch stays positive.
  readonly property int _sliceSpacingEff: sliceWidth > 1 ? Math.max(sliceSpacing, -(sliceWidth - 1)) : 0
  readonly property int _slicePitch: Math.max(1, sliceWidth + _sliceSpacingEff)
  property bool suppressWidthAnim: false
  property int topBarHeight: 50 * Config.uiScale
  property bool _filterBarManuallyShown: Config.filterBarAlwaysVisible
  property bool _filterBarHoverRevealed: false
  readonly property bool _filterBarShown: _filterBarManuallyShown || _filterBarHoverRevealed || themesOpen || ricesOpen
  property bool browseOpen: false
  property string browseSource: "wallhaven"
  property bool themesOpen: false
  property bool ricesOpen: false
  property bool _capturePromptOpen: false
  property string _deleteConfirmSlug: ""
  property string _deleteConfirmName: ""
  property bool _workshopOpen: false
  property var _workshopRice: null
  readonly property bool _navLocked: settingsOpen || _workshopOpen || _capturePromptOpen || _deleteConfirmSlug !== ""
  property bool anyBrowserOpen: browseOpen
  property var _activeModel: themesOpen ? themeModel : (ricesOpen ? riceModel : (service ? service.filteredModel : null))
  property bool isHexMode: Config.displayMode === "hex"
  property bool isGridMode: Config.displayMode === "wall"
  property bool isMosaicMode: Config.displayMode === "mosaic"
  property bool isHandMode: Config.displayMode === "hand"
  property bool isSandyMode: Config.displayMode === "sandy"
  property bool isGridLayoutsMode: Config.displayMode === "grid"
  property bool isSliceMode: !isHexMode && !isGridMode && !isMosaicMode && !isHandMode && !isSandyMode && !isGridLayoutsMode

  // Hand, Sandy and the Grid layouts share one selection cursor: each view owns
  // its own geometry but reports the focused catalogue row back through here.
  property int customViewIndex: 0
  property bool _isCustomView: isHandMode || isSandyMode || isGridLayoutsMode

  property var _focusedItem: {
    var m = _activeModel
    if (!m || m.count === 0) return null
    var idx = -1
    if (isHexMode && hexListView) idx = hexListView._selectedCol * hexListView._rows + hexListView._selectedRow
    else if (isGridMode && thumbGridView) idx = thumbGridView.hoveredIdx
    else if (isMosaicMode && mosaicView) idx = mosaicView.hoveredIdx
    else if (_isCustomView) idx = customViewIndex
    else idx = sliceListView.currentIndex
    if (idx < 0 || idx >= m.count) return null
    return m.get(idx)
  }
  readonly property string _focusedThemeName: (themesOpen && _focusedItem) ? ("" + (_focusedItem.name || "")) : ""
  readonly property string _focusedThemeCreator: (themesOpen && _focusedItem && _focusedItem.provider) ? ("" + _focusedItem.provider) : ""

  onIsHexModeChanged: if (showing) _bindActiveViewModel()
  onIsGridModeChanged: if (showing) _bindActiveViewModel()
  onIsMosaicModeChanged: if (showing) _bindActiveViewModel()
  onIsHandModeChanged: if (showing) _bindActiveViewModel()
  onIsSandyModeChanged: if (showing) _bindActiveViewModel()
  onIsGridLayoutsModeChanged: if (showing) _bindActiveViewModel()

  onThemesOpenChanged: { if (themesOpen) _loadThemes(); if (showing) _bindActiveViewModel() }
  onRicesOpenChanged: { if (ricesOpen) _loadRices(); if (showing) _bindActiveViewModel() }

  function _bindActiveViewModel() {
    var _isSlice = isSliceMode
    if (_isSlice) {
      sliceListView.model = Qt.binding(function() { return wallpaperSelector._activeModel })
      _positionTimer.posIdx = Math.min(Math.max(0, sliceListView.currentIndex), Math.max(0, (_activeModel ? _activeModel.count : 1) - 1))
      _positionTimer.restart()
    } else {
      sliceListView.model = null
    }
    if (isGridMode) {
      thumbGridView.model = Qt.binding(function() { return wallpaperSelector._activeModel })
      thumbGridView.cacheBuffer = 300
    } else {
      thumbGridView.model = null
      thumbGridView.cacheBuffer = 0
    }
    if (isHexMode) {
      hexListView.model = Qt.binding(function() { return Math.ceil((wallpaperSelector._activeModel ? wallpaperSelector._activeModel.count : 0) / Math.max(1, hexListView._rows)) })
    } else {
      hexListView.model = null
    }
  }
  property int hexRadius: Config.hexRadius
  Behavior on hexRadius { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }
  property int hexRows: Config.hexRows
  Behavior on hexRows { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }
  property int hexCols: Config.hexCols
  Behavior on hexCols { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }

  property real _gridCellW: Config.gridThumbWidth + 8
  Behavior on _gridCellW { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }
  property real _gridCellH: Config.gridThumbHeight + 8
  Behavior on _gridCellH { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }
  property real _gridTotalW: _gridCellW * Config.gridColumns
  Behavior on _gridTotalW { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }
  property int _gridTotalH: _gridCellH * Config.gridRows
  Behavior on _gridTotalH { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }

  // The fan needs ~2.4x the card height of stage (the centre card is lifted and
  // hangs below its anchor). cardHeight = stage + topBarHeight + 40, the
  // container sits 48 above the bottom and the card is capped 48 short of the
  // top, so on a short or scaled display the card size scales down to fit
  // rather than the stage pushing the toolbar off the screen. The stage derives
  // from the fitted card, so it can never exceed the panel.
  readonly property real _handFit: Math.max(0.35, Math.min(1,
      (selectorPanel.height - topBarHeight - 136 - 120 - Math.abs(Config.handArch))
      / (Config.handCardHeight * 2.4)))
  // Floor so rounding can never push the derived stage one pixel past the fit.
  readonly property int _handCardHEff: Math.floor(Config.handCardHeight * _handFit)
  readonly property int _handCardWEff: Math.floor(Config.handCardWidth * _handFit)
  readonly property int _handStageH: _handCardHEff * 2.4 + Math.abs(Config.handArch) + 120
  readonly property int _handStageW: Math.min(selectorPanel.width - 60,
      _handCardWEff * 2 + (Math.min(Config.handCount, 9) / 2) * Config.handSpread * _handFit * 2)
  // The strand is centred vertically and arcs downward with `off`, so the stage
  // takes the slice height plus the arc/lane spread, capped to the panel.
  readonly property int _sandyStageH: Math.min(selectorPanel.height - 140, Config.sandySliceHeight * 2.6 + 120)
  readonly property int _sandyStageW: Math.min(selectorPanel.width - 60, 1500)
  readonly property int _gridStageH: (Config.gridThumbHeight + 14) * Config.gridRows + 40
  readonly property int _gridStageW: Math.min(selectorPanel.width - 80, (Config.gridThumbWidth + 14) * Config.gridColumns + 40)
  // The toolbar strip lives inside the card at its top, so a card taller than
  // the screen pushes that strip above the top edge and it becomes unreachable
  // (the failure #235/#236 report). Every mode's natural height is honoured up
  // to the panel height; beyond that the content clips and scrolls instead of
  // taking the toolbar off-display.
  property int cardHeight: browseOpen ? 0 : Math.min(selectorPanel.height - 96, (isHexMode ? hexGridHeight : (isGridMode ? _gridTotalH + topBarHeight + 35 : (isMosaicMode ? Config.mosaicHeight + topBarHeight + 60 : (isHandMode ? _handStageH + topBarHeight + 40 : (isSandyMode ? _sandyStageH + topBarHeight + 40 : (isGridLayoutsMode ? _gridStageH + topBarHeight + 40 : sliceHeight + topBarHeight + 60)))))))
  property int hexCardWidth: selectorPanel.width
  // The card tracks the image content: wide enough for the visible window of
  // slices, but never wider than the screen (a card past the screen edge pushes
  // its rounded corner and the filter bar off-display, so the two stop reading
  // as one surface). The list then fills the card interior exactly, so the
  // displayed image width and the card/bar width are the same number by
  // construction rather than two formulas that can drift apart.
  property int _sliceListW: Config.wallpaperExpandedWidth + (Config.wallpaperVisibleCount - 1) * _slicePitch
  property int cardWidth: isHexMode ? hexCardWidth : (isGridMode ? _gridTotalW + 20 : (isMosaicMode ? Config.mosaicWidth + 20 : (isHandMode ? Math.max(_handStageW + 40, 600) : (isSandyMode ? _sandyStageW + 40 : (isGridLayoutsMode ? _gridStageW + 40 : Math.min(Math.max(_sliceListW + 40, 600), selectorPanel.width - 40))))))
  Behavior on cardWidth { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }
  property int hexGridHeight: {
    var rows = hexRows
    var r = hexRadius
    var spacing = 6
    var hexH = Math.ceil(r * 1.73205)
    var shape = Config.hexShape
    var tileH = shape === "diamond" ? hexH * 2 : hexH
    var stepY = tileH + spacing
    var staggerTail = shape === "triangle" ? 0 : stepY / 2
    var contentH = (rows - 1) * stepY + tileH + staggerTail
    return contentH + topBarHeight + 90
  }
  Behavior on cardHeight { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }

  property bool settingsOpen: false

  property string _currentSelectedPath: {
    if (!service || !service.filteredModel) return ""
    var idx = -1
    if (Config.displayMode === "slices")      idx = sliceListView.currentIndex
    else if (Config.displayMode === "hex" && hexListView)
                                              idx = hexListView._selectedCol * hexListView._rows + hexListView._selectedRow
    else if (Config.displayMode === "wall" && thumbGridView)
                                              idx = thumbGridView.hoveredIdx
    else if (Config.displayMode === "mosaic" && mosaicView)
                                              idx = mosaicView.hoveredIdx
    else if (_isCustomView)                   idx = customViewIndex
    if (idx < 0 || idx >= service.filteredModel.count) return ""
    var item = service.filteredModel.get(idx)
    return item ? (item.path || "") : ""
  }
  property real _settingsShift: {
    if (!settingsOpen) return 0
    var h = settingsLoader.height
    var base = h - 4
    var naturalCardY = (selectorPanel.height - cardHeight) / 2
    var settingsY = naturalCardY + base / 2 + filterBarBg.y - h - 8
    if (settingsY < 8) {
      var extra = 2 * (8 - settingsY)
      return base + extra
    }
    return base
  }
  Behavior on _settingsShift { NumberAnimation { duration: 500; easing.type: Easing.OutBack; easing.overshoot: 1.2 } }
  property bool _restorePending: false
  property int _preCommitIndex: -1
  property bool cardVisible: false
  PanelWindow {
    id: selectorPanel

    screen: Quickshell.screens.find(s => s.name === wallpaperSelector.openMonitor)
        ?? Quickshell.screens.find(s => s.name === wallpaperSelector.mainMonitor)
        ?? Quickshell.screens[0]

    anchors {
      top: true
      bottom: true
      left: true
      right: true
    }
    margins {
      top: 0
      bottom: 0
      left: 0
      right: 0
    }

    visible: wallpaperSelector.showing
    color: "transparent"

    WlrLayershell.namespace: "wallpaper-selector-parallel"
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.keyboardFocus: wallpaperSelector.showing ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None

    exclusionMode: ExclusionMode.Ignore

    // Right-to-left languages (Arabic, Hebrew, Persian) mirror the picker from
    // here: Qt flips anchors, rows, layouts and text alignment for every
    // descendant, so the one surface root knows about direction and no
    // component below has to.
    LayoutMirroring.enabled: I18n.rtl
    LayoutMirroring.childrenInherit: true

    Shortcut {
      sequence: "Ctrl+X"
      context: Qt.WindowShortcut
      onActivated: wallpaperSelector._resetFilters()
    }

    Rectangle {
      anchors.fill: parent
      color: Qt.rgba(0, 0, 0, Config.selectorBackdropOpacity / 100)
      opacity: wallpaperSelector.cardVisible ? 1 : 0
      Behavior on opacity { NumberAnimation { duration: Style.animMedium } }
      Behavior on color { ColorAnimation { duration: Style.animMedium } }
    }
    MouseArea {
      anchors.fill: parent
      acceptedButtons: Qt.LeftButton | Qt.RightButton
      onClicked: {
        if (wallpaperSelector.anyBrowserOpen) {
          wallpaperSelector.browseOpen = false
          wallpaperSelector.themesOpen = false
          wallpaperSelector.ricesOpen = false
        } else {
          wallpaperSelector.showing = false
        }
      }
    }
  Item {
    id: cardContainer
    width: wallpaperSelector.cardWidth
    height: wallpaperSelector.cardHeight
    anchors.horizontalCenter: parent.horizontalCenter
    anchors.bottom: parent.bottom
    anchors.bottomMargin: 48
    visible: wallpaperSelector.cardVisible
    opacity: 0
    property bool animateIn: wallpaperSelector.cardVisible

    onAnimateInChanged: {
      if (animateIn) {
        opacity = 1
        focusTimer.restart()
        wallpaperSelector.uiReady()
      }
    }

    MouseArea {
      anchors.fill: parent
      onClicked: {}
    }

  Item {
    id: backgroundRect
    anchors.fill: parent

    FilterBar {
      id: filterBarBg
      anchors.horizontalCenter: parent.horizontalCenter
      anchors.top: parent.top
      anchors.topMargin: 30
      // The bar is a self-contained pill centred over the wallpaper, so it is
      // capped to the panel, not the card: a narrow card (a small slice preset,
      // or a localized label set that widens the row) would otherwise hide the
      // chips after the colour picker behind an avoidable clip.
      maxWidth: selectorPanel.width - 24
      z: 10
      colors: wallpaperSelector.colors
      service: service
      settingsOpen: wallpaperSelector.settingsOpen
      cacheLoading: service.cacheLoading
      cacheProgress: service.cacheProgress
      cacheTotal: service.cacheTotal
      videoConvertRunning: VideoConvertService.running
      videoConvertProgress: VideoConvertService.progress
      videoConvertTotal: VideoConvertService.total
      videoConvertFile: VideoConvertService.currentFile
      imageOptimizeRunning: ImageOptimizeService.running
      imageOptimizeProgress: ImageOptimizeService.progress
      imageOptimizeTotal: ImageOptimizeService.total
      imageOptimizeFile: ImageOptimizeService.currentFile
      browseOpen: wallpaperSelector.browseOpen
      themesOpen: wallpaperSelector.themesOpen
      ricesOpen: wallpaperSelector.ricesOpen
      followActive: wallpaperSelector._activeThemeName === "Wallpaper"
      onSettingsToggled: { wallpaperSelector.settingsOpen = !wallpaperSelector.settingsOpen; if (!wallpaperSelector.settingsOpen) wallpaperSelector._focusActiveList() }
      onBrowseToggled: { wallpaperSelector.settingsOpen = false; wallpaperSelector.themesOpen = false; wallpaperSelector.ricesOpen = false; wallpaperSelector.browseOpen = !wallpaperSelector.browseOpen }
      onThemesToggled: { wallpaperSelector.settingsOpen = false; wallpaperSelector.browseOpen = false; wallpaperSelector.ricesOpen = false; wallpaperSelector.themesOpen = !wallpaperSelector.themesOpen }
      onRicesToggled: { wallpaperSelector.settingsOpen = false; wallpaperSelector.browseOpen = false; wallpaperSelector.themesOpen = false; wallpaperSelector.ricesOpen = !wallpaperSelector.ricesOpen }
      onSaveLookRequested: wallpaperSelector._capturePromptOpen = true
      onModeToggled: function(mode) {
        Config.saveKey("matugen.mode", mode)
        DaemonClient.retheme(Config.matugenScheme, mode, Config.matugenColorIndex)
      }
      onFollowToggled: Quickshell.execDetached(["ryoku-shell", "theme", "Wallpaper"])
      visible: !wallpaperSelector.browseOpen
      enabled: wallpaperSelector._filterBarShown
      opacity: (wallpaperSelector.browseOpen || !wallpaperSelector._filterBarShown) ? 0 : 1
      Behavior on opacity { NumberAnimation { duration: Style.animNormal } }

      HoverHandler {
        id: _filterBarHover
        onHoveredChanged: {
          if (hovered) wallpaperSelector._filterBarHoverRevealed = true
          else _filterBarHideTimer.restart()
        }
      }
    }
    
    MouseArea {
      id: filterHoverZone
      anchors.top: parent.top
      anchors.left: parent.left
      anchors.right: parent.right
      height: wallpaperSelector._filterBarShown
              ? (filterBarBg.y + filterBarBg.height + 12)
              : 24
      hoverEnabled: true
      acceptedButtons: Qt.NoButton
      propagateComposedEvents: true
      visible: !wallpaperSelector.anyBrowserOpen
      z: 9
      onContainsMouseChanged: {
        if (containsMouse) wallpaperSelector._filterBarHoverRevealed = true
        else _filterBarHideTimer.restart()
      }
    }

    Timer {
      id: _filterBarHideTimer
      interval: 250
      repeat: false
      onTriggered: {
        if (!filterHoverZone.containsMouse && !_filterBarHover.hovered)
          wallpaperSelector._filterBarHoverRevealed = false
      }
    }

    // The filter row is hidden by default now (#238 read the always-on bar as
    // clutter): this is the one discrete control that reveals it, beside the
    // top-edge hover zone and the Up-key toggle. It sits at the card's top edge
    // and steps aside the moment the bar is up, so it never crowds the strip.
    FilterButton {
      id: filterRevealHandle
      anchors.top: parent.top
      anchors.topMargin: 6
      anchors.horizontalCenter: parent.horizontalCenter
      z: 11
      visible: !wallpaperSelector.anyBrowserOpen && !wallpaperSelector._filterBarShown
      colors: wallpaperSelector.colors
      icon: "\u{f0233}"
      tooltip: I18n.tr("Show filters")
      onClicked: wallpaperSelector._filterBarManuallyShown = true
    }

    }

    CacheProgressBar {
      id: cacheProgressBar
      anchors.bottom: parent.bottom
      anchors.horizontalCenter: parent.horizontalCenter
      anchors.bottomMargin: 30
      colors: wallpaperSelector.colors
      cacheLoading: service.cacheLoading
      cacheProgress: service.cacheProgress
      cacheTotal: service.cacheTotal
    }
  }

    // Modal scrim: dims the images behind the centred settings so it reads as a
    // modal on top, and click-out closes it.
    Rectangle {
      anchors.fill: parent
      z: 998
      color: "#000000"
      opacity: wallpaperSelector.settingsOpen ? 0.5 : 0
      visible: opacity > 0.01
      Behavior on opacity { NumberAnimation { duration: Style.animMedium } }
      MouseArea { anchors.fill: parent; hoverEnabled: true; onClicked: wallpaperSelector.settingsOpen = false }
    }

    Loader {
      id: settingsLoader
      active: true
      asynchronous: true
      anchors.horizontalCenter: parent.horizontalCenter
      y: 16 * Config.uiScale
      z: 999
      sourceComponent: Component {
        SettingsPanel {
          colors: wallpaperSelector.colors
          service: wallpaperSelector.selectorService
          settingsOpen: wallpaperSelector.settingsOpen
          openDownward: true
          sourcePath: wallpaperSelector._currentSelectedPath
          onCloseRequested: { wallpaperSelector.settingsOpen = false; wallpaperSelector._focusActiveList() }
          onThemeChanged: function(scheme, mode, colorIndex) {
            console.log("WallpaperSelector: themeChanged scheme=" + scheme + " mode=" + mode + " colorIndex=" + colorIndex)
            DaemonClient.retheme(scheme, mode, (typeof colorIndex === "number") ? colorIndex : Config.matugenColorIndex)
          }
        }
      }
    }

    Loader {
      id: browseLoader
      active: wallpaperSelector.browseOpen
      anchors.horizontalCenter: parent.horizontalCenter
      anchors.top: parent.top
      anchors.topMargin: 60 * Config.uiScale
      anchors.bottom: parent.bottom
      anchors.bottomMargin: 60 * Config.uiScale
      width: Math.min(cardContainer.width - 20, Screen.width - 140 * Config.uiScale)
      z: 6
      sourceComponent: Component {
        BrowseSurface {
          width: parent ? parent.width : 0
          colors: wallpaperSelector.colors
          whService: wallpaperSelector._whService
          swService: wallpaperSelector.swService
          browserVisible: true
          source: wallpaperSelector.browseSource
          onSourceChanged: wallpaperSelector.browseSource = source
          onEscapePressed: { wallpaperSelector.browseOpen = false; wallpaperSelector._focusActiveList() }
        }
      }
    }

    Rectangle {
      id: capturePrompt
      anchors.fill: parent
      z: 1000
      visible: wallpaperSelector._capturePromptOpen || opacity > 0.01
      opacity: wallpaperSelector._capturePromptOpen ? 1 : 0
      Behavior on opacity { NumberAnimation { duration: Style.animNormal } }
      color: Qt.rgba(0, 0, 0, 0.5)
      onVisibleChanged: if (visible) _capInput.forceActiveFocus()

      readonly property color _ink: wallpaperSelector.colors ? wallpaperSelector.colors.surfaceText : "#e0e2e8"
      readonly property color _inkDim: wallpaperSelector.colors ? wallpaperSelector.colors.surfaceVariantText : "#c2c7cf"
      readonly property color _accent: wallpaperSelector.colors ? wallpaperSelector.colors.primary : Style.fallbackAccent

      MouseArea { anchors.fill: parent; hoverEnabled: true; onClicked: wallpaperSelector._capturePromptOpen = false }

      Rectangle {
        anchors.centerIn: parent
        width: Math.min(parent.width - 80, 420 * Config.uiScale)
        height: capCol.implicitHeight + 32
        radius: Style.radiusLarge
        color: wallpaperSelector.colors ? wallpaperSelector.colors.surface : "#131313"
        border.width: 1
        border.color: Qt.rgba(capturePrompt._ink.r, capturePrompt._ink.g, capturePrompt._ink.b, 0.18)
        MouseArea { anchors.fill: parent }

        Column {
          id: capCol
          anchors.left: parent.left; anchors.right: parent.right; anchors.top: parent.top
          anchors.margins: 16
          spacing: 10

          Text {
            text: I18n.tr("Save current look")
            font.family: Style.fontFamily; font.pixelSize: 14 * Config.uiScale; font.weight: Font.Medium
            color: capturePrompt._ink
          }
          Text {
            width: parent.width; wrapMode: Text.WordWrap
            text: I18n.tr("Capture your wallpaper, palette, decorations and layout as a new rice you can re-apply later.")
            font.family: Style.fontFamily; font.pixelSize: 11 * Config.uiScale
            color: capturePrompt._inkDim
          }
          Rectangle {
            width: parent.width; height: 30 * Config.uiScale
            color: wallpaperSelector.colors ? Qt.rgba(wallpaperSelector.colors.surfaceContainer.r, wallpaperSelector.colors.surfaceContainer.g, wallpaperSelector.colors.surfaceContainer.b, 0.8) : Qt.rgba(0.15, 0.17, 0.22, 0.8)
            border.width: capInput.activeFocus ? 2 : 1
            border.color: capInput.activeFocus ? capturePrompt._accent : Qt.rgba(capturePrompt._ink.r, capturePrompt._ink.g, capturePrompt._ink.b, 0.2)
            Text {
              visible: capInput.text.length === 0
              anchors.left: parent.left; anchors.leftMargin: 10; anchors.verticalCenter: parent.verticalCenter
              text: I18n.tr("Rice name")
              font.family: Style.fontFamily; font.pixelSize: 11 * Config.uiScale
              color: Qt.rgba(capturePrompt._inkDim.r, capturePrompt._inkDim.g, capturePrompt._inkDim.b, 0.7)
            }
            TextInput {
              id: capInput
              anchors.fill: parent; anchors.leftMargin: 10; anchors.rightMargin: 10
              verticalAlignment: TextInput.AlignVCenter
              font.family: Style.fontFamily; font.pixelSize: 11 * Config.uiScale
              color: capturePrompt._ink; clip: true; selectByMouse: true
              onAccepted: if (text.trim().length > 0) { wallpaperSelector._captureRice(text.trim()); text = ""; wallpaperSelector._capturePromptOpen = false }
            }
          }
          Row {
            anchors.right: parent.right
            spacing: 8
            FilterButton { colors: wallpaperSelector.colors; label: I18n.tr("CANCEL"); register: false; skew: 8; height: 28 * Config.uiScale; onClicked: { capInput.text = ""; wallpaperSelector._capturePromptOpen = false } }
            FilterButton {
              colors: wallpaperSelector.colors; label: I18n.tr("SAVE"); register: false; skew: 8; height: 28 * Config.uiScale
              hasActiveColor: true; activeColor: capturePrompt._accent; isActive: true
              onClicked: if (capInput.text.trim().length > 0) { wallpaperSelector._captureRice(capInput.text.trim()); capInput.text = ""; wallpaperSelector._capturePromptOpen = false }
            }
          }
        }
      }
    }

    Rectangle {
      id: deleteConfirm
      anchors.fill: parent
      z: 1000
      visible: wallpaperSelector._deleteConfirmSlug !== "" || opacity > 0.01
      opacity: wallpaperSelector._deleteConfirmSlug !== "" ? 1 : 0
      Behavior on opacity { NumberAnimation { duration: Style.animNormal } }
      color: Qt.rgba(0, 0, 0, 0.5)

      readonly property color _ink: wallpaperSelector.colors ? wallpaperSelector.colors.surfaceText : "#e0e2e8"
      readonly property color _inkDim: wallpaperSelector.colors ? wallpaperSelector.colors.surfaceVariantText : "#c2c7cf"

      MouseArea { anchors.fill: parent; hoverEnabled: true; onClicked: wallpaperSelector._deleteConfirmSlug = "" }

      Rectangle {
        anchors.centerIn: parent
        width: Math.min(parent.width - 80, 420 * Config.uiScale)
        height: delCol.implicitHeight + 32
        radius: Style.radiusLarge
        color: wallpaperSelector.colors ? wallpaperSelector.colors.surface : "#131313"
        border.width: 1
        border.color: Qt.rgba(deleteConfirm._ink.r, deleteConfirm._ink.g, deleteConfirm._ink.b, 0.18)
        MouseArea { anchors.fill: parent }

        Column {
          id: delCol
          anchors.left: parent.left; anchors.right: parent.right; anchors.top: parent.top
          anchors.margins: 16
          spacing: 10

          Text {
            text: I18n.tr("Delete %1?").arg(wallpaperSelector._deleteConfirmName)
            font.family: Style.fontFamily; font.pixelSize: 14 * Config.uiScale; font.weight: Font.Medium
            color: deleteConfirm._ink
          }
          Text {
            width: parent.width; wrapMode: Text.WordWrap
            text: I18n.tr("Remove this rice from your library. Your current desktop is untouched; this only deletes the saved look.")
            font.family: Style.fontFamily; font.pixelSize: 11 * Config.uiScale
            color: deleteConfirm._inkDim
          }
          Row {
            anchors.right: parent.right
            spacing: 8
            FilterButton { colors: wallpaperSelector.colors; label: I18n.tr("CANCEL"); register: false; skew: 8; height: 28 * Config.uiScale; onClicked: wallpaperSelector._deleteConfirmSlug = "" }
            FilterButton {
              colors: wallpaperSelector.colors; label: I18n.tr("DELETE"); register: false; skew: 8; height: 28 * Config.uiScale
              hasActiveColor: true; activeColor: wallpaperSelector.colors ? wallpaperSelector.colors.error : "#e2342a"; isActive: true
              onClicked: { wallpaperSelector._deleteRice(wallpaperSelector._deleteConfirmSlug); wallpaperSelector._deleteConfirmSlug = "" }
            }
          }
        }
      }
    }
    ListView {
      id: sliceListView
      property bool navLocked: wallpaperSelector._navLocked
      anchors.top: cardContainer.top
      anchors.topMargin: wallpaperSelector.topBarHeight + 15
      anchors.bottom: cardContainer.bottom
      anchors.bottomMargin: 20
      anchors.left: cardContainer.left
      anchors.right: cardContainer.right
      anchors.leftMargin: 20
      anchors.rightMargin: 20
      // The list fills the card interior, so the displayed image width equals the
      // card width (which the filter bar is centred on) by construction; the
      // card's own screen cap keeps both on-display together.
      Behavior on width { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }

      orientation: ListView.Horizontal
      model: wallpaperSelector._activeModel
      // Clip to the card interior: the cache materialises neighbours beyond the
      // visible window, and uncipped they would paint past the rounded card edge
      // and off the screen. Clipped, the strip ends exactly at the card.
      clip: true
      spacing: wallpaperSelector._sliceSpacingEff

      flickDeceleration: 1500
      maximumFlickVelocity: 3000
      boundsBehavior: Flickable.StopAtBounds
      // Materialise a bounded band of slices around the viewport, not a raw pixel
      // buffer: expandedWidth alone could be 1800px of cache, which at a small
      // slice width pulls hundreds of full-height image + shader-effect delegates
      // into memory at once and stalls the shell. Five pitches of lead and lag is
      // enough to keep flicking smooth regardless of the configured widths. Zero
      // when the picker shows another mode, so no offscreen slices materialise.
      cacheBuffer: wallpaperSelector.isSliceMode ? wallpaperSelector._slicePitch * 5 : 0

      visible: wallpaperSelector.cardVisible && !wallpaperSelector.anyBrowserOpen && wallpaperSelector.isSliceMode

      property bool keyboardNavActive: false
      property real lastMouseX: -1
      property real lastMouseY: -1

      property bool contentMoving: false
      onContentXChanged: { sliceListView.contentMoving = true; _sliceScrollStop.restart() }
      Timer { id: _sliceScrollStop; interval: 90; onTriggered: sliceListView.contentMoving = false }

      highlightFollowsCurrentItem: true
      highlightMoveDuration: Style.animExpand
      highlight: Item {}

      add: Transition {
        enabled: !service.filterTransitioning
        NumberAnimation { property: "opacity"; from: 0; to: 1; duration: Style.animEnter; easing.type: Easing.OutCubic }
        NumberAnimation { property: "scale"; from: 0.85; to: 1; duration: Style.animEnter; easing.type: Easing.OutCubic }
      }
      remove: Transition {
        enabled: !service.filterTransitioning
        NumberAnimation { property: "opacity"; to: 0; duration: Style.animNormal; easing.type: Easing.InCubic }
      }
      displaced: Transition {
        enabled: !service.filterTransitioning
        NumberAnimation { properties: "x,y"; duration: Style.animMedium; easing.type: Easing.OutCubic }
      }
      move: Transition {
        enabled: !service.filterTransitioning
        NumberAnimation { properties: "x,y"; duration: Style.animMedium; easing.type: Easing.OutCubic }
      }

      preferredHighlightBegin: (width - wallpaperSelector.expandedWidth) / 2
      preferredHighlightEnd: (width + wallpaperSelector.expandedWidth) / 2
      highlightRangeMode: ListView.StrictlyEnforceRange

      header: Item { width: (sliceListView.width - wallpaperSelector.expandedWidth) / 2; height: 1 }
      footer: Item { width: (sliceListView.width - wallpaperSelector.expandedWidth) / 2; height: 1 }

      focus: wallpaperSelector.showing
      onVisibleChanged: {
        console.log("[SLICE] onVisibleChanged visible=", visible, "count=", count, "model=", (model ? "set" : "null"), "contentX=", contentX, "width=", width, "height=", height, "contentWidth=", contentWidth)
        if (visible && !wallpaperSelector.isHexMode) forceActiveFocus()
      }

      Connections {
        target: wallpaperSelector
        function onShowingChanged() {
          if (wallpaperSelector.showing)
            wallpaperSelector._focusActiveList()
        }
      }
      onCountChanged: {
        console.log("[SLICE] onCountChanged count=", count, "visible=", visible, "contentX=", contentX, "contentWidth=", contentWidth)
        if (count > 0 && wallpaperSelector.showing && !wallpaperSelector._restorePending) {
          currentIndex = Math.min(currentIndex, count - 1)
        }
      }

      MouseArea {
        anchors.fill: parent
        propagateComposedEvents: true
        onWheel: function(wheel) {
          if (wallpaperSelector._navLocked) return
          var step = 1
          if (wheel.angleDelta.y > 0 || wheel.angleDelta.x > 0) {
            sliceListView.currentIndex = Math.max(0, sliceListView.currentIndex - step)
          } else if (wheel.angleDelta.y < 0 || wheel.angleDelta.x < 0) {
            sliceListView.currentIndex = Math.min((wallpaperSelector._activeModel ? wallpaperSelector._activeModel.count : 0) - 1, sliceListView.currentIndex + step)
          }
        }
        onPressed: function(mouse) { mouse.accepted = false }
        onReleased: function(mouse) { mouse.accepted = false }
        onClicked: function(mouse) { mouse.accepted = false }
      }

      Timer {
        id: wheelDebounce
        interval: 400
        onTriggered: {
          var centerX = sliceListView.contentX + sliceListView.width / 2
          var nearest = sliceListView.indexAt(centerX, sliceListView.height / 2)
          if (nearest >= 0) sliceListView.currentIndex = nearest
        }
      }

      Keys.onEscapePressed: wallpaperSelector.showing = false
      Keys.onReturnPressed: {
        if (currentIndex >= 0 && wallpaperSelector._activeModel && currentIndex < wallpaperSelector._activeModel.count) {
          const item = wallpaperSelector._activeModel.get(currentIndex)
          wallpaperSelector._applyItem(item)
        }
      }
      Keys.onPressed: function(event) {

        if (event.modifiers & Qt.ShiftModifier) {
          if (event.key === Qt.Key_Up) {
            wallpaperSelector._filterBarManuallyShown = !wallpaperSelector._filterBarManuallyShown
            event.accepted = true
            return
          } else if (event.key === Qt.Key_Left) {
            if (service.selectedColorFilter === -1) {
              service.selectedColorFilter = 99
            } else if (service.selectedColorFilter === 99) {
              service.selectedColorFilter = 11
            } else if (service.selectedColorFilter === 0) {
              service.selectedColorFilter = 99
            } else {
              service.selectedColorFilter--
            }
            event.accepted = true
            return
          } else if (event.key === Qt.Key_Right) {
            if (service.selectedColorFilter === -1) {
              service.selectedColorFilter = 0
            } else if (service.selectedColorFilter === 11) {
              service.selectedColorFilter = 99
            } else if (service.selectedColorFilter === 99) {
              service.selectedColorFilter = 0
            } else {
              service.selectedColorFilter++
            }
            event.accepted = true
            return
          }
        }
        if (event.key === Qt.Key_Left && !(event.modifiers & Qt.ShiftModifier)) {
          keyboardNavActive = true
          if (currentIndex > 0) {
            currentIndex--
          }
          event.accepted = true
          return
        }

        if (event.key === Qt.Key_Right && !(event.modifiers & Qt.ShiftModifier)) {
          keyboardNavActive = true
          if (currentIndex < (wallpaperSelector._activeModel ? wallpaperSelector._activeModel.count : 0) - 1) {
            currentIndex++
          }
          event.accepted = true
          return
        }
      }

      delegate: SliceDelegate {
        colors: wallpaperSelector.colors
        expandedWidth: wallpaperSelector.expandedWidth
        sliceWidth: wallpaperSelector.sliceWidth
        skewOffset: wallpaperSelector.skewOffset
        service: wallpaperSelector.selectorService
        suppressWidthAnim: wallpaperSelector.suppressWidthAnim
        isDepth: wallpaperSelector._isStageWall(model.path)
        applyRequest: function(item, forcePicker) { wallpaperSelector._applyItem(item, forcePicker) }
        deleteRequest: function(item) {
          wallpaperSelector._deleteConfirmSlug = "" + item.slug
          wallpaperSelector._deleteConfirmName = "" + (item.name || item.slug)
        }
      }
    }
    Image {
      id: _snapshotImage
      anchors.fill: sliceListView
      visible: false
      opacity: 0
      z: sliceListView.z + 1
    }

    ListView {
      id: hexListView

      anchors.top: cardContainer.top
      anchors.topMargin: wallpaperSelector.topBarHeight + 15
      anchors.bottom: cardContainer.bottom
      anchors.bottomMargin: 20
      anchors.left: cardContainer.left
      anchors.right: cardContainer.right
      visible: wallpaperSelector.cardVisible && !wallpaperSelector.anyBrowserOpen && wallpaperSelector.isHexMode

      orientation: ListView.Horizontal
      clip: true

      property bool contentMoving: false
      onContentXChanged: { hexListView.contentMoving = true; _hexScrollStop.restart() }
      Timer { id: _hexScrollStop; interval: 90; onTriggered: hexListView.contentMoving = false }
      property int _rows: wallpaperSelector.hexRows
      property real _r: wallpaperSelector.hexRadius
      property real _gridSpacingX: Config.hexGapX
      property real _gridSpacingY: Config.hexGapY
      property real _gridSpacing: Config.hexGapX
      property string _shape: Config.hexShape
      property real _hexW: _shape === "rhombus" ? _r * 4 : _r * 2
      property real _hexH: _shape === "diamond" ? Math.ceil(_r * 3.4641) : Math.ceil(_r * 1.73205)
      property real _stepX: _shape === "triangle" ? _r + _gridSpacingX : (_shape === "hexagon" ? 1.5 * _r + _gridSpacingX : _hexW / 2 + _gridSpacingX)
      property real _stepY: _hexH + _gridSpacingY
      property real _gridContentH: (_rows - 1) * _stepY + _hexH + (_shape === "triangle" ? 0 : _stepY * Config.hexStagger)
      property real _yOffset: Math.max(0, (height - _gridContentH) / 2)
      property real _visibleBand: (wallpaperSelector.hexCols - 1) * _stepX + _hexW
      property real _fadeZone: (width - _visibleBand) / 2

      boundsBehavior: Flickable.StopAtBounds
      flickDeceleration: 1500
      maximumFlickVelocity: 3000
      cacheBuffer: _stepX * 2

      focus: wallpaperSelector.showing && wallpaperSelector.isHexMode
      property bool _initialSnap: true
      onVisibleChanged: {
        if (visible) forceActiveFocus()
        if (visible) {
          _initialSnap = true
          _restored = false
          highlightMoveDuration = 0
          var startCol = Math.min(Math.floor(wallpaperSelector.hexCols / 2), count - 1)
          if (startCol >= 0) { currentIndex = startCol; _selectedCol = startCol; _selectedRow = 0 }
          positionViewAtIndex(currentIndex, ListView.Center)
          _snapRestoreTimer.restart()
        } else {
          _restored = false
        }
      }

      Timer {
        id: _snapRestoreTimer
        interval: 50
        onTriggered: {
          hexListView.highlightMoveDuration = Style.animExpand
          hexListView._initialSnap = false
        }
      }

      model: Math.ceil((wallpaperSelector._activeModel ? wallpaperSelector._activeModel.count : 0) / Math.max(1, _rows))

      property bool _restored: false
      onCountChanged: {
        if (count > 0 && visible && !_restored) {
          var startCol = Math.min(Math.floor(wallpaperSelector.hexCols / 2), count - 1)
          if (startCol >= 0) { currentIndex = startCol; _selectedCol = startCol; _selectedRow = 0 }
        }
      }

      spacing: 0

      highlightFollowsCurrentItem: true
      highlightMoveDuration: Style.animExpand
      highlight: Item {}
      preferredHighlightBegin: (width - _hexW) / 2
      preferredHighlightEnd: (width + _hexW) / 2
      highlightRangeMode: ListView.StrictlyEnforceRange

      header: Item { width: (hexListView.width - hexListView._hexW) / 2 }
      footer: Item { width: (hexListView.width - hexListView._hexW) / 2 }

      add: Transition {
        NumberAnimation { property: "opacity"; from: 0; to: 1; duration: Style.animEnter; easing.type: Easing.OutCubic }
        NumberAnimation { property: "scale"; from: 0.9; to: 1; duration: Style.animEnter; easing.type: Easing.OutCubic }
      }
      remove: Transition {
        NumberAnimation { property: "opacity"; to: 0; duration: Style.animNormal; easing.type: Easing.InCubic }
      }
      displaced: Transition {
        NumberAnimation { properties: "x,y"; duration: Style.animMedium; easing.type: Easing.OutCubic }
      }

      MouseArea {
        anchors.fill: parent
        propagateComposedEvents: true
        onWheel: function(wheel) {
          if (wallpaperSelector._navLocked) return
          var step = Config.hexScrollStep
          if (wheel.angleDelta.y > 0 || wheel.angleDelta.x > 0) {
            hexListView.currentIndex = Math.max(0, hexListView.currentIndex - step)
            hexListView._selectedCol = hexListView.currentIndex
          } else if (wheel.angleDelta.y < 0 || wheel.angleDelta.x < 0) {
            hexListView.currentIndex = Math.min(hexListView.count - 1, hexListView.currentIndex + step)
            hexListView._selectedCol = hexListView.currentIndex
          }
        }
        onPressed: function(mouse) { mouse.accepted = false }
        onReleased: function(mouse) { mouse.accepted = false }
        onClicked: function(mouse) { mouse.accepted = false }
      }

      Keys.onEscapePressed: wallpaperSelector.showing = false
      Keys.onReturnPressed: {
        var flatIdx = _selectedCol * _rows + _selectedRow
        if (flatIdx >= 0 && wallpaperSelector._activeModel && flatIdx < wallpaperSelector._activeModel.count) {
          var item = wallpaperSelector._activeModel.get(flatIdx)
          wallpaperSelector._applyItem(item)
        }
      }

      property int _selectedCol: currentIndex
      property int _selectedRow: 0

      Keys.onPressed: function(event) {
        if (event.modifiers & Qt.ShiftModifier) {
          if (event.key === Qt.Key_Up) {
            wallpaperSelector._filterBarManuallyShown = !wallpaperSelector._filterBarManuallyShown
            event.accepted = true
            return
          } else if (event.key === Qt.Key_Left) {
            if (service.selectedColorFilter === -1) service.selectedColorFilter = 99
            else if (service.selectedColorFilter === 99) service.selectedColorFilter = 11
            else if (service.selectedColorFilter === 0) service.selectedColorFilter = 99
            else service.selectedColorFilter--
            event.accepted = true
            return
          } else if (event.key === Qt.Key_Right) {
            if (service.selectedColorFilter === -1) service.selectedColorFilter = 0
            else if (service.selectedColorFilter === 11) service.selectedColorFilter = 99
            else if (service.selectedColorFilter === 99) service.selectedColorFilter = 0
            else service.selectedColorFilter++
            event.accepted = true
            return
          }
        }
        if (event.key === Qt.Key_Left && !(event.modifiers & Qt.ShiftModifier)) {
          if (currentIndex > 0) { currentIndex--; _selectedCol = currentIndex }
          event.accepted = true
          return
        }
        if (event.key === Qt.Key_Right && !(event.modifiers & Qt.ShiftModifier)) {
          if (currentIndex < count - 1) { currentIndex++; _selectedCol = currentIndex }
          event.accepted = true
          return
        }
        if (event.key === Qt.Key_Up && !(event.modifiers & Qt.ShiftModifier)) {
          if (_selectedRow > 0) _selectedRow--
          event.accepted = true
          return
        }
        if (event.key === Qt.Key_Down && !(event.modifiers & Qt.ShiftModifier)) {
          var maxRow = Math.min(_rows, (wallpaperSelector._activeModel ? wallpaperSelector._activeModel.count : 0) - _selectedCol * _rows) - 1
          if (_selectedRow < maxRow) _selectedRow++
          event.accepted = true
          return
        }
      }

      delegate: Item {
        id: hexCol
        width: hexListView._stepX
        height: hexListView.height
        clip: false
        property int colIdx: index

        readonly property real _colCenter: (x - hexListView.contentX) + width * 0.5
        readonly property bool _insideView: _colCenter > -hexListView._hexW && _colCenter < hexListView.width + hexListView._hexW
        readonly property bool _nearEdge: _colCenter < hexListView._fadeZone || _colCenter > (hexListView.width - hexListView._fadeZone)
        readonly property bool _nearLeft: _colCenter < hexListView.width / 2
        readonly property bool _visible: _insideView && !_nearEdge
        property real _colScale: _visible ? 1 : 0
        Behavior on _colScale { enabled: !hexListView._initialSnap; NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }
        property real _curveFactor: Config.hexCurve === "flat" ? 0 : Config.hexArcIntensity
        Behavior on _curveFactor { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }
        readonly property real _arcOffset: {
          if (_curveFactor === 0) return 0
          var viewCenterX = hexListView.width / 2
          var n = (_colCenter - viewCenterX) / Math.max(1, viewCenterX)
          var r = hexListView._r
          var shaped
          switch (Config.hexCurve) {
            case "wave": shaped = Math.sin(n * Math.PI * Config.hexWaves) * r; break
            case "s": shaped = n * n * n * r; break
            case "arc": shaped = -n * n * r; break
            default: shaped = 0; break
          }
          return shaped * _curveFactor
        }
        readonly property real _norm: (_colCenter - hexListView.width / 2) / Math.max(1, hexListView.width / 2)
        readonly property real _lensScale: Config.hexLens === 0 ? 1 : (1 + Config.hexLens * Math.max(0, 1 - Math.abs(_norm)))
        readonly property real _twistAngle: Config.hexTwist * _norm
        readonly property real _scatterY: Config.hexScatter === 0 ? 0 : ((((colIdx * 2654435761) % 1000) / 1000 - 0.5) * 2 * Config.hexScatter * hexListView._r)


        Repeater {
          model: Math.max(0, Math.min(hexListView._rows, (wallpaperSelector._activeModel ? wallpaperSelector._activeModel.count : 0) - hexCol.colIdx * hexListView._rows))

          HexDelegate {
            property int rowIdx: index
            property int flatIdx: hexCol.colIdx * hexListView._rows + rowIdx

            hexRadius: hexListView._r
            hexShape: Config.hexShape
            gridRow: rowIdx
            gridColumn: hexCol.colIdx
            colors: wallpaperSelector.colors
            service: wallpaperSelector.selectorService
            itemData: wallpaperSelector._activeModel ? wallpaperSelector._activeModel.get(flatIdx) : null
            isSelected: hexCol.colIdx === hexListView._selectedCol && rowIdx === hexListView._selectedRow
            viewMoving: hexListView.contentMoving
            isDepth: wallpaperSelector._isStageWall(itemData ? itemData.path : "")
            applyRequest: function(item, forcePicker) { wallpaperSelector._applyItem(item, forcePicker) }

            x: 0
            y: hexListView._yOffset + rowIdx * hexListView._stepY + (hexListView._shape === "triangle" || hexCol.colIdx % 2 === 0 ? 0 : hexListView._stepY * Config.hexStagger) + hexCol._arcOffset + hexCol._scatterY

            parallaxX: 0
            parallaxY: 0

            scale: hexCol._colScale * hexCol._lensScale
            rotation: hexCol._twistAngle
            transformOrigin: hexCol._nearLeft ? Item.Left : Item.Right
            opacity: hexCol._colScale < 0.01 ? 0 : 1
            pulledOut: hexBackOverlay.overlayItemKey !== "" && hexBackOverlay.overlayItemKey === ((itemData && ((itemData.weId || "") !== "")) ? itemData.weId : (itemData ? itemData.name : ""))

            onFlipRequested: function(data, gx, gy, sourceItem) {
              hexBackOverlay.show(data, gx, gy, sourceItem)
            }
            onHoverSelected: {
              hexListView._selectedCol = hexCol.colIdx
              hexListView._selectedRow = rowIdx
            }
          }
        }
      }
    }

    GridView {
      id: thumbGridView

      anchors.top: cardContainer.top
      anchors.topMargin: wallpaperSelector.topBarHeight + 15
      anchors.bottom: cardContainer.bottom
      anchors.bottomMargin: 20
      anchors.horizontalCenter: parent.horizontalCenter
      width: wallpaperSelector._gridTotalW
      Behavior on width { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }
      clip: true

      cellWidth: wallpaperSelector._gridCellW
      Behavior on cellWidth { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }
      cellHeight: wallpaperSelector._gridCellH
      Behavior on cellHeight { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }

      model: wallpaperSelector._activeModel
      cacheBuffer: 300
      boundsBehavior: Flickable.StopAtBounds
      interactive: false

      property bool contentMoving: false
      Timer { id: _gridScrollStop; interval: 90; onTriggered: thumbGridView.contentMoving = false }

      property real _scrollTarget: 0
      onContentYChanged: {
        thumbGridView.contentMoving = true
        _gridScrollStop.restart()
        if (!_gridScrollAnim.running) _scrollTarget = contentY
      }

      NumberAnimation {
        id: _gridScrollAnim
        target: thumbGridView
        property: "contentY"
        duration: 400
        easing.type: Easing.OutCubic
      }

      function _snapScroll(delta) {
        if (!_gridScrollAnim.running) _scrollTarget = contentY
        var step = cellHeight
        _scrollTarget += (delta > 0 ? -step : step)
        var maxY = contentHeight - height
        _scrollTarget = Math.max(0, Math.min(_scrollTarget, maxY))
        _gridScrollAnim.stop()
        _gridScrollAnim.from = contentY
        _gridScrollAnim.to = _scrollTarget
        _gridScrollAnim.start()
      }

      MouseArea {
        anchors.fill: parent
        propagateComposedEvents: true
        onWheel: function(wheel) {
          if (wallpaperSelector._navLocked) return
          thumbGridView._snapScroll(wheel.angleDelta.y)
          thumbGridView.forceActiveFocus()
        }
        onPressed: function(mouse) { mouse.accepted = false }
        onReleased: function(mouse) { mouse.accepted = false }
        onClicked: function(mouse) { mouse.accepted = false }
      }

      visible: wallpaperSelector.cardVisible && !wallpaperSelector.anyBrowserOpen && wallpaperSelector.isGridMode

      focus: wallpaperSelector.showing && wallpaperSelector.isGridMode
      onVisibleChanged: {
        if (visible) forceActiveFocus()
      }

      Keys.onEscapePressed: {
        if (gridBackOverlay.overlayOpen) gridBackOverlay.hide()
        else wallpaperSelector.showing = false
      }
      Keys.onReturnPressed: {
        if (hoveredIdx >= 0 && wallpaperSelector._activeModel && hoveredIdx < wallpaperSelector._activeModel.count) {
          var item = wallpaperSelector._activeModel.get(hoveredIdx)
          wallpaperSelector._applyItem(item)
        }
      }
      property int hoveredIdx: currentIndex

      function _ensureVisible(idx) {
        var row = Math.floor(idx / Config.gridColumns)
        var rowTop = row * cellHeight
        var rowBottom = rowTop + cellHeight
        if (rowTop < contentY) {
          _snapScrollTo(rowTop)
        } else if (rowBottom > contentY + height) {
          _snapScrollTo(rowBottom - height)
        }
      }

      function _snapScrollTo(target) {
        var maxY = contentHeight - height
        _scrollTarget = Math.max(0, Math.min(target, maxY))
        _gridScrollAnim.stop()
        _gridScrollAnim.from = contentY
        _gridScrollAnim.to = _scrollTarget
        _gridScrollAnim.start()
      }

      Keys.onUpPressed: function(event) {
        if (event.modifiers & Qt.ShiftModifier) {
          wallpaperSelector._filterBarManuallyShown = !wallpaperSelector._filterBarManuallyShown
          event.accepted = true
          return
        }
        var newIdx = currentIndex - Config.gridColumns
        if (newIdx >= 0) {
          currentIndex = newIdx
          hoveredIdx = newIdx
          _ensureVisible(newIdx)
        }
      }
      Keys.onDownPressed: function(event) {
        var newIdx = currentIndex + Config.gridColumns
        if (newIdx < count) {
          currentIndex = newIdx
          hoveredIdx = newIdx
          _ensureVisible(newIdx)
        }
      }
      Keys.onLeftPressed: function(event) {
        if (event.modifiers & Qt.ShiftModifier) {
          if (service.selectedColorFilter === -1) service.selectedColorFilter = 99
          else if (service.selectedColorFilter === 99) service.selectedColorFilter = 11
          else if (service.selectedColorFilter === 0) service.selectedColorFilter = 99
          else service.selectedColorFilter--
          event.accepted = true
          return
        }
        if (currentIndex > 0) {
          currentIndex--
          hoveredIdx = currentIndex
          _ensureVisible(currentIndex)
        }
      }
      Keys.onRightPressed: function(event) {
        if (event.modifiers & Qt.ShiftModifier) {
          if (service.selectedColorFilter === -1) service.selectedColorFilter = 0
          else if (service.selectedColorFilter === 11) service.selectedColorFilter = 99
          else if (service.selectedColorFilter === 99) service.selectedColorFilter = 0
          else service.selectedColorFilter++
          event.accepted = true
          return
        }
        if (currentIndex < count - 1) {
          currentIndex++
          hoveredIdx = currentIndex
          _ensureVisible(currentIndex)
        }
      }

      highlightMoveDuration: Style.animNormal
      highlight: Item {}

      ScrollBar.vertical: ScrollBar {
        policy: ScrollBar.AsNeeded
        width: 4
        contentItem: Rectangle {
          radius: 2
          color: wallpaperSelector.colors ? Qt.rgba(wallpaperSelector.colors.primary.r, wallpaperSelector.colors.primary.g, wallpaperSelector.colors.primary.b, 0.4)
                                          : Qt.rgba(1, 1, 1, 0.3)
        }
      }

      add: Transition {
        NumberAnimation { property: "opacity"; from: 0; to: 1; duration: Style.animEnter; easing.type: Easing.OutCubic }
        NumberAnimation { property: "scale"; from: 0.85; to: 1; duration: Style.animEnter; easing.type: Easing.OutBack; easing.overshoot: 1.2 }
      }
      remove: Transition {
        NumberAnimation { property: "opacity"; to: 0; duration: Style.animVeryFast; easing.type: Easing.InCubic }
      }
      displaced: Transition {
        NumberAnimation { properties: "x,y"; duration: Style.animFast; easing.type: Easing.OutCubic }
      }

      delegate: Item {
        id: gridThumbDelegate
        width: thumbGridView.cellWidth
        height: thumbGridView.cellHeight

        required property int index
        required property var model

        property string videoPath: model.videoPrev || model.videoFile || ""
        property bool hasVideo: videoPath.length > 0 && Config.videoPreviewEnabled
        property bool _previewArmed: false
        readonly property bool videoActive: _previewArmed && hasVideo && thumbGridView.hoveredIdx === index && !thumbGridView.contentMoving && wallpaperSelector.showing

        onVisibleChanged: {
            if (!visible) { _gridVideoDelay.stop(); _previewArmed = false }
        }

        Connections {
            target: thumbGridView
            function onHoveredIdxChanged() {
                if (thumbGridView.hoveredIdx === gridThumbDelegate.index && gridThumbDelegate.hasVideo) {
                    _gridVideoDelay.restart()
                } else {
                    _gridVideoDelay.stop()
                    gridThumbDelegate._previewArmed = false
                }
            }
        }

        Timer {
            id: _gridVideoDelay
            interval: Config.videoPreviewInstant ? 100 : 600
            onTriggered: gridThumbDelegate._previewArmed = true
        }

        property real _entryOpacity: 0.8

        Behavior on _entryOpacity { NumberAnimation { duration: 300; easing.type: Easing.OutQuad } }

        opacity: _entryOpacity

        readonly property real entryViewY: y - thumbGridView.contentY
        readonly property bool entryInView: entryViewY + height > 0 && entryViewY < thumbGridView.height

        onEntryInViewChanged: {
          if (entryInView) _entryOpacity = 1.0
          else _entryOpacity = 0.8
        }

        Component.onCompleted: {
          if (entryInView) _entryOpacity = 1.0
        }

        Rectangle {
          id: gridCardRect
          anchors.fill: parent; anchors.margins: 4; radius: 6
          color: "transparent"

          border.width: thumbGridView.hoveredIdx === gridThumbDelegate.index ? 2 : 0
          border.color: wallpaperSelector.colors ? wallpaperSelector.colors.primary : "#ff8800"
          Behavior on border.width { NumberAnimation { duration: Style.animFast; easing.type: Easing.OutQuad } }

          property bool _pulledOut: gridBackOverlay.overlayItemKey !== "" && gridBackOverlay.overlayItemKey === ((gridThumbDelegate.model.weId || "") !== "" ? gridThumbDelegate.model.weId : gridThumbDelegate.model.name)
          visible: !_pulledOut

          Rectangle {
            anchors.fill: parent; anchors.margins: gridCardRect.border.width; radius: 5
            color: wallpaperSelector.colors ? Qt.rgba(wallpaperSelector.colors.surface.r, wallpaperSelector.colors.surface.g, wallpaperSelector.colors.surface.b, 0.6) : Qt.rgba(0.12, 0.14, 0.18, 0.6)
            clip: true

          Image {
            id: gridThumbImg
            anchors.fill: parent
            source: gridThumbDelegate.model.thumb ? ImageService.fileUrl(gridThumbDelegate.model.thumb) : ""
            fillMode: Image.PreserveAspectCrop
            asynchronous: true
            smooth: true
            cache: true
            sourceSize.width: Config.gridThumbWidth
            sourceSize.height: Config.gridThumbHeight
            opacity: status === Image.Ready ? 1 : 0
            Behavior on opacity { NumberAnimation { duration: Style.animNormal; easing.type: Easing.OutCubic } }
          }

          Column {
            id: _gridSwatches
            anchors.fill: parent
            visible: gridThumbDelegate.model.kind === "theme" && gridThumbImg.status !== Image.Ready
            property var _sw: gridThumbDelegate.model.sw ? ("" + gridThumbDelegate.model.sw).split(",") : []
            Repeater {
              model: _gridSwatches._sw
              Rectangle {
                width: _gridSwatches.width
                height: _gridSwatches.height / Math.max(1, _gridSwatches._sw.length)
                color: modelData
              }
            }
          }

          Loader {
              id: _gridVideoLoader
              anchors.fill: parent
              active: gridThumbDelegate.videoActive
              visible: false
              layer.enabled: active

              sourceComponent: Video {
                  anchors.fill: parent
                  source: ImageService.fileUrl(gridThumbDelegate.videoPath)
                  fillMode: VideoOutput.PreserveAspectCrop
                  loops: MediaPlayer.Infinite
                  muted: true
                  Component.onCompleted: play()
              }
          }

          Item {
              anchors.fill: parent
              visible: _gridVideoLoader.active && _gridVideoLoader.status === Loader.Ready

              ShaderEffectSource {
                  anchors.fill: parent
                  sourceItem: _gridVideoLoader
                  live: true
              }
          }

          Rectangle {
            id: gridSkeleton
            anchors.fill: parent; radius: 6
            visible: opacity > 0
            opacity: gridThumbImg.status === Image.Ready ? 0 : 1
            Behavior on opacity { NumberAnimation { duration: Style.animNormal; easing.type: Easing.OutCubic } }
            color: wallpaperSelector.colors ? Qt.rgba(wallpaperSelector.colors.surfaceVariant.r, wallpaperSelector.colors.surfaceVariant.g, wallpaperSelector.colors.surfaceVariant.b, 0.8) : Qt.rgba(0.18, 0.20, 0.25, 0.8)

            Rectangle {
              id: gridShimmer
              width: parent.width * 0.5; height: parent.height; radius: 6
              opacity: 0.35
              gradient: Gradient {
                orientation: Gradient.Horizontal
                GradientStop { position: 0.0; color: "transparent" }
                GradientStop { position: 0.5; color: wallpaperSelector.colors ? Qt.rgba(wallpaperSelector.colors.surfaceText.r, wallpaperSelector.colors.surfaceText.g, wallpaperSelector.colors.surfaceText.b, 0.08) : Qt.rgba(1, 1, 1, 0.08) }
                GradientStop { position: 1.0; color: "transparent" }
              }
              NumberAnimation on x {
                from: -gridShimmer.width; to: gridSkeleton.width
                duration: 1200; loops: Animation.Infinite
                running: gridSkeleton.visible
              }
            }

            Text {
              anchors.centerIn: parent
              text: "\u{f0553}"
              font.family: Style.fontFamilyNerdIcons; font.pixelSize: 22
              color: wallpaperSelector.colors ? Qt.rgba(wallpaperSelector.colors.surfaceText.r, wallpaperSelector.colors.surfaceText.g, wallpaperSelector.colors.surfaceText.b, 0.15) : Qt.rgba(1,1,1,0.1)
            }
          }

          MouseArea {
            id: gridThumbMouse
            anchors.fill: parent
            cursorShape: Qt.PointingHandCursor
            hoverEnabled: true
            acceptedButtons: Qt.LeftButton | Qt.RightButton
            onContainsMouseChanged: {
              if (containsMouse) {
                thumbGridView.hoveredIdx = gridThumbDelegate.index
                thumbGridView.forceActiveFocus()
              }
            }
            onClicked: function(mouse) {
              thumbGridView.forceActiveFocus()
              if (mouse.button === Qt.RightButton) {
                var gpos = gridThumbDelegate.mapToItem(null, gridThumbDelegate.width / 2, gridThumbDelegate.height / 2)
                var d = gridThumbDelegate.model
                gridBackOverlay.show({
                  name: d.name, path: d.path, thumb: d.thumb, type: d.type,
                  weId: d.weId || "", favourite: d.favourite, videoFile: d.videoFile || ""
                }, gpos.x, gpos.y, gridThumbDelegate)
              } else {
                var d = gridThumbDelegate.model
                var forcePicker = !!(mouse.modifiers & Qt.ControlModifier)
                wallpaperSelector._applyItem(d, forcePicker)
              }
            }
          }

          Rectangle {
            anchors.bottom: parent.bottom; anchors.left: parent.left
            anchors.margins: 4
            width: gridTypeBadge.implicitWidth + 6; height: 14; radius: 3
            color: Qt.rgba(0, 0, 0, 0.6)
            visible: gridThumbDelegate.model.kind !== "theme" && gridThumbDelegate.model.kind !== "rice"
            Text {
              id: gridTypeBadge
              anchors.centerIn: parent
              text: (gridThumbDelegate.model.type === "video" || gridThumbDelegate.model.videoFile) ? I18n.tr("VID") : (gridThumbDelegate.model.type === "static" ? I18n.tr("PIC") : I18n.tr("WE"))
              font.family: Style.fontFamily; font.pixelSize: 8; font.weight: Font.Bold
              color: wallpaperSelector.colors ? wallpaperSelector.colors.primary : "#ff8800"
            }
          }

          Rectangle {
            anchors.top: parent.top; anchors.left: parent.left
            anchors.margins: 4
            width: 18; height: 18; radius: 9
            color: gridThumbDelegate.videoActive ? (wallpaperSelector.colors ? wallpaperSelector.colors.primary : Style.fallbackAccent) : Qt.rgba(0, 0, 0, 0.7)
            border.width: 1
            border.color: gridThumbDelegate.videoActive
                ? "transparent"
                : (wallpaperSelector.colors ? Qt.rgba(wallpaperSelector.colors.primary.r, wallpaperSelector.colors.primary.g, wallpaperSelector.colors.primary.b, 0.6) : Qt.rgba(1,1,1,0.4))
            visible: gridThumbDelegate.hasVideo && gridThumbDelegate.model.kind !== "theme" && gridThumbDelegate.model.kind !== "rice"
            z: 5

            Behavior on color { ColorAnimation { duration: Style.animFast } }

            Text {
              anchors.centerIn: parent; anchors.horizontalCenterOffset: 1
              text: "\u25b6"; font.pixelSize: 7
              color: gridThumbDelegate.videoActive
                  ? (wallpaperSelector.colors ? wallpaperSelector.colors.primaryText : "#000")
                  : (wallpaperSelector.colors ? wallpaperSelector.colors.primary : Style.fallbackAccent)
            }
          }

          Text {
            anchors.top: parent.top; anchors.right: parent.right
            anchors.margins: 4
            text: "\u{f0134}"
            font.family: Style.fontFamilyNerdIcons; font.pixelSize: 14
            color: wallpaperSelector.colors ? wallpaperSelector.colors.primary : "#ff8800"
            visible: gridThumbDelegate.model.favourite === true && gridThumbDelegate.model.kind !== "theme" && gridThumbDelegate.model.kind !== "rice"
          }
          }
        }
      }
    }

    MosaicView {
      id: mosaicView

      anchors.top: cardContainer.top
      anchors.topMargin: wallpaperSelector.topBarHeight + 35
      anchors.horizontalCenter: parent.horizontalCenter
      width: Config.mosaicWidth
      height: Config.mosaicHeight

      service: service
      model: wallpaperSelector._activeModel
      colors: wallpaperSelector.colors
      depthCheck: function(p) { return wallpaperSelector._isStageWall(p) }
      active: wallpaperSelector.cardVisible && !wallpaperSelector.anyBrowserOpen && wallpaperSelector.isMosaicMode
      visible: active

      onItemActivated: function(item) {
        if (item) wallpaperSelector._applyItem(item)
      }
    }

    HandView {
      id: handView
      anchors.top: cardContainer.top
      anchors.topMargin: wallpaperSelector.topBarHeight + 20
      anchors.bottom: cardContainer.bottom
      anchors.bottomMargin: 20
      anchors.horizontalCenter: parent.horizontalCenter
      width: wallpaperSelector._handStageW
      model: wallpaperSelector._activeModel
      colors: wallpaperSelector.colors
      active: wallpaperSelector.cardVisible && !wallpaperSelector.anyBrowserOpen && wallpaperSelector.isHandMode
      visible: active
      focus: wallpaperSelector.showing && wallpaperSelector.isHandMode
      selectedIndex: wallpaperSelector.customViewIndex
      cardWidth: wallpaperSelector._handCardWEff
      cardHeight: wallpaperSelector._handCardHEff
      cardCount: Config.handCount
      fanAngle: Config.handFanAngle
      fanRoll: Config.handFanRoll
      arch: Config.handArch
      spread: Config.handSpread
      skew: Config.handSkew
      cornerRadius: Config.handCornerRadius
      tilt: Config.handTilt
      perspective: Config.handPerspective
      speed: Config.handSpeed
      ghosts: Config.handGhosts ? 2 : 0
      bob: Config.handBob ? 4 : 0
      backdrop: Config.handBackdrop
      onSelectionChanged: function(index) { wallpaperSelector.customViewIndex = index }
      onItemActivated: function(item) { if (item) wallpaperSelector._applyItem(item) }
      Keys.onEscapePressed: wallpaperSelector.showing = false
    }

    SandyView {
      id: sandyView
      anchors.top: cardContainer.top
      anchors.topMargin: wallpaperSelector.topBarHeight + 20
      anchors.bottom: cardContainer.bottom
      anchors.bottomMargin: 20
      anchors.horizontalCenter: parent.horizontalCenter
      width: wallpaperSelector._sandyStageW
      model: wallpaperSelector._activeModel
      colors: wallpaperSelector.colors
      active: wallpaperSelector.cardVisible && !wallpaperSelector.anyBrowserOpen && wallpaperSelector.isSandyMode
      visible: active
      focus: wallpaperSelector.showing && wallpaperSelector.isSandyMode
      selectedIndex: wallpaperSelector.customViewIndex
      center: Config.sandyCenter
      sliceWidth: Config.sandySliceWidth
      sliceHeight: Config.sandySliceHeight
      skew: Config.sandySkew
      spacing: Config.sandySpacing / 100
      duration: Config.sandyDuration * 4
      strands: Config.sandyStrands
      twist: Config.sandyTwist * 8
      orbit: Config.sandyOrbit * 0.6
      turbulence: Config.sandyTurbulence * 0.4
      waist: Config.sandyWaist * 0.35
      front: Config.sandyFront * 0.6
      arc: Config.sandyArc * 0.6
      edgeSpeed: Config.sandyEdgeSpeed / 28
      ringSpin: Config.sandyRingSpin
      ringSize: Config.sandyRingSize * 0.6
      ringWave: Config.sandyRingWave * 0.15
      ringSoft: Config.sandyRingSoft * 0.5
      ringBlend: Config.sandyRingBlend
      ringHold: Config.sandyRingHold
      grain: Config.sandyGrain / 10
      fan: Config.sandyFan * 10
      onSelectionChanged: function(index) { wallpaperSelector.customViewIndex = index }
      onItemActivated: function(item) { if (item) wallpaperSelector._applyItem(item) }
      Keys.onEscapePressed: wallpaperSelector.showing = false
    }

    GridLayoutsView {
      id: gridLayoutsView
      anchors.top: cardContainer.top
      anchors.topMargin: wallpaperSelector.topBarHeight + 20
      anchors.bottom: cardContainer.bottom
      anchors.bottomMargin: 20
      anchors.horizontalCenter: parent.horizontalCenter
      width: wallpaperSelector._gridStageW
      model: wallpaperSelector._activeModel
      colors: wallpaperSelector.colors
      active: wallpaperSelector.cardVisible && !wallpaperSelector.anyBrowserOpen && wallpaperSelector.isGridLayoutsMode
      visible: active
      focus: wallpaperSelector.showing && wallpaperSelector.isGridLayoutsMode
      selectedIndex: wallpaperSelector.customViewIndex
      layoutKind: Config.gridLayout
      columns: Config.gridColumns
      rows: Config.gridRows
      thumbWidth: Config.gridThumbWidth
      thumbHeight: Config.gridThumbHeight
      stagger: Config.gridStagger
      selectedScale: Config.gridSelectedScale
      flowWave: Config.gridFlowWave
      flowFrequency: Config.gridFlowFrequency
      scatter: Config.gridScatter
      scaleVariance: Config.gridScaleVariance
      cylinderBend: Config.gridCylinderBend
      cylinderRadius: Config.gridCylinderRadius
      onSelectionChanged: function(index) { wallpaperSelector.customViewIndex = index }
      onItemActivated: function(item) { if (item) wallpaperSelector._applyItem(item) }
      Keys.onEscapePressed: wallpaperSelector.showing = false
    }

    // Momentary-detour header: entering themes/rices is not a sticky mode, so a
    // temporary banner announces the detour and one click walks it back to the
    // wallpaper carousel. In themes mode the focused theme's name (and creator,
    // when the scheme came from RyoStore) rides just beneath it. Anchored over
    // the top of the carousel so it reads above the tiles.
    Column {
      id: detourHeader
      anchors.top: cardContainer.top
      anchors.topMargin: wallpaperSelector.topBarHeight + 12
      anchors.horizontalCenter: parent.horizontalCenter
      spacing: 10
      z: 210
      visible: (wallpaperSelector.themesOpen || wallpaperSelector.ricesOpen) && wallpaperSelector.cardVisible

      readonly property color _ink: wallpaperSelector.colors ? wallpaperSelector.colors.surfaceText : "#e0e2e8"
      readonly property color _inkDim: wallpaperSelector.colors ? wallpaperSelector.colors.surfaceVariantText : "#c2c7cf"
      readonly property color _surface: wallpaperSelector.colors ? wallpaperSelector.colors.surface : Qt.rgba(0.06, 0.07, 0.09, 1)

      Rectangle {
        anchors.horizontalCenter: parent.horizontalCenter
        height: 26 * Config.uiScale
        width: _bannerRow.implicitWidth + 24 * Config.uiScale
        radius: Style.radiusRound
        color: Qt.rgba(detourHeader._surface.r, detourHeader._surface.g, detourHeader._surface.b, 0.92)
        border.width: 1
        border.color: Qt.rgba(detourHeader._ink.r, detourHeader._ink.g, detourHeader._ink.b, 0.2)

        Row {
          id: _bannerRow
          anchors.centerIn: parent
          spacing: 8
          Text {
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("\u2190 WALLPAPERS")
            font.family: Style.fontFamily; font.pixelSize: 10 * Config.uiScale
            font.weight: Font.Medium; font.letterSpacing: 1.2
            color: detourHeader._ink
          }
          Text {
            anchors.verticalCenter: parent.verticalCenter
            text: "\u00b7"
            font.family: Style.fontFamily; font.pixelSize: 10 * Config.uiScale
            color: detourHeader._inkDim
          }
          Text {
            anchors.verticalCenter: parent.verticalCenter
            text: wallpaperSelector.themesOpen ? I18n.tr("THEMES") : I18n.tr("RICES")
            font.family: Style.fontFamily; font.pixelSize: 10 * Config.uiScale
            font.weight: Font.Medium; font.letterSpacing: 1.2
            color: detourHeader._inkDim
          }
        }

        MouseArea {
          anchors.fill: parent
          cursorShape: Qt.PointingHandCursor
          onClicked: {
            wallpaperSelector.themesOpen = false
            wallpaperSelector.ricesOpen = false
            wallpaperSelector._focusActiveList()
          }
        }
      }

      Column {
        anchors.horizontalCenter: parent.horizontalCenter
        spacing: 2
        visible: wallpaperSelector.themesOpen && wallpaperSelector._focusedThemeName !== ""
        Text {
          anchors.horizontalCenter: parent.horizontalCenter
          visible: wallpaperSelector._focusedThemeCreator !== ""
          text: wallpaperSelector._focusedThemeCreator
          font.family: Style.fontFamily; font.pixelSize: 9 * Config.uiScale
          font.weight: Font.Medium; font.letterSpacing: 2
          color: detourHeader._inkDim
        }
        Text {
          anchors.horizontalCenter: parent.horizontalCenter
          text: wallpaperSelector._focusedThemeName
          font.family: Style.fontFamilyHeading; font.pixelSize: 22 * Config.uiScale
          color: detourHeader._ink
          style: Text.Outline
          styleColor: Qt.rgba(0, 0, 0, 0.55)
        }
      }
    }

    Item {
      id: gridBackOverlay
      anchors.fill: parent
      visible: false
      z: 200

      property var overlayData: null
      property string overlayItemKey: ""
      property var _sourceItem: null
      property real sourceX: 0
      property real sourceY: 0
      property real _openContentY: 0
      property bool overlayOpen: false
      property var _gridMeta: null

      readonly property real bigW: Math.min(Config.gridThumbWidth * 2.5, 600)
      readonly property real bigH: Math.min(Config.gridThumbHeight * 2.5, 500)

      onOverlayOpenChanged: {
        if (overlayOpen && overlayData && overlayData.type !== "we") {
          var key = ImageService.thumbKey(overlayData.thumb, overlayData.name)
          _gridMeta = FileMetadataService.getMetadata(key)
          if (!_gridMeta)
            FileMetadataService.probeIfNeeded(key, overlayData.path, overlayData.type === "video" ? "video" : "image")
        }
      }
      Connections {
        target: FileMetadataService
        enabled: gridBackOverlay.overlayOpen
        function onMetadataReady(key) {
          if (!gridBackOverlay.overlayData) return
          var myKey = ImageService.thumbKey(gridBackOverlay.overlayData.thumb, gridBackOverlay.overlayData.name)
          if (key === myKey)
            gridBackOverlay._gridMeta = FileMetadataService.getMetadata(key)
        }
      }

      function show(data, gx, gy, sourceItem) {
        overlayData = data
        overlayItemKey = (data.weId || "") !== "" ? data.weId : data.name
        _sourceItem = sourceItem || null
        _openContentY = thumbGridView.contentY
        var local = gridBackOverlay.mapFromItem(null, gx, gy)
        sourceX = local.x
        sourceY = local.y
        visible = true
        overlayOpen = true
      }

      function hide() {
        var scrollDelta = thumbGridView.contentY - _openContentY
        sourceY -= scrollDelta
        _openContentY = thumbGridView.contentY
        overlayOpen = false
      }

      Rectangle {
        anchors.fill: parent
        color: Qt.rgba(0, 0, 0, gridBackOverlay.overlayOpen ? 0.55 : 0)
        Behavior on color { ColorAnimation { duration: Style.animNormal } }
        MouseArea {
          anchors.fill: parent
          acceptedButtons: Qt.LeftButton | Qt.RightButton
          onClicked: gridBackOverlay.hide()
        }
      }

      states: [
        State {
          name: "hidden"
          when: !gridBackOverlay.overlayOpen
          PropertyChanges {
            target: gridCard
            x: gridBackOverlay.sourceX - gridCard.width / 2
            y: gridBackOverlay.sourceY - gridCard.height / 2
            scale: Config.gridThumbWidth / gridBackOverlay.bigW
            opacity: 0
          }
          PropertyChanges { target: gridCardRotation; angle: 0 }
        },
        State {
          name: "visible"
          when: gridBackOverlay.overlayOpen
          PropertyChanges {
            target: gridCard
            x: (gridBackOverlay.width - gridCard.width) / 2
            y: (gridBackOverlay.height - gridCard.height) / 2
            scale: 1
            opacity: 1
          }
          PropertyChanges { target: gridCardRotation; angle: 180 }
        }
      ]

      transitions: [
        Transition {
          from: "hidden"; to: "visible"
          SequentialAnimation {
            PropertyAction { target: gridBackOverlay; property: "visible"; value: true }
            ParallelAnimation {
              NumberAnimation { target: gridCard; properties: "x,y,scale,opacity"; duration: Style.animSlow; easing.type: Easing.OutCubic }
              NumberAnimation { target: gridCardRotation; property: "angle"; duration: Style.animSlow; easing.type: Easing.InOutQuad }
            }
          }
        },
        Transition {
          from: "visible"; to: "hidden"
          SequentialAnimation {
            ParallelAnimation {
              NumberAnimation { target: gridCard; properties: "x,y,scale"; duration: Style.animSlow; easing.type: Easing.InOutCubic }
              NumberAnimation { target: gridCardRotation; property: "angle"; duration: Style.animSlow; easing.type: Easing.InOutQuad }
              SequentialAnimation {
                PauseAnimation { duration: Style.animSlow * 0.7 }
                NumberAnimation { target: gridCard; property: "opacity"; duration: Style.animSlow * 0.3; easing.type: Easing.InQuad }
              }
            }
            PropertyAction { target: gridBackOverlay; property: "visible"; value: false }
            PropertyAction { target: gridBackOverlay; property: "overlayItemKey"; value: "" }
            PropertyAction { target: gridBackOverlay; property: "_sourceItem"; value: null }
          }
        }
      ]

      Item {
        id: gridCard
        width: gridBackOverlay.bigW
        height: gridBackOverlay.bigH
        transformOrigin: Item.Center

        transform: Rotation {
          id: gridCardRotation
          origin.x: gridCard.width / 2
          origin.y: gridCard.height / 2
          axis { x: 0; y: 1; z: 0 }
          angle: 0
        }

        Item {
          id: gridFrontFace
          anchors.fill: parent
          visible: gridCardRotation.angle < 90

          Rectangle {
            anchors.fill: parent; radius: 12
            color: wallpaperSelector.colors ? wallpaperSelector.colors.surfaceContainer : "#1a1a2e"
            clip: true

            Image {
              anchors.fill: parent
              source: gridBackOverlay.overlayData && gridBackOverlay.overlayData.thumb
                ? ImageService.fileUrl(gridBackOverlay.overlayData.thumb) : ""
              fillMode: Image.PreserveAspectCrop
              smooth: true; asynchronous: true; cache: false
              sourceSize.width: gridBackOverlay.bigW
              sourceSize.height: gridBackOverlay.bigH
            }
          }

          Rectangle {
            anchors.fill: parent; radius: 12
            color: "transparent"
            border.width: 2
            border.color: wallpaperSelector.colors ? wallpaperSelector.colors.primary : Style.fallbackAccent
          }
        }

        Item {
          id: gridBackFace
          anchors.fill: parent
          visible: gridCardRotation.angle >= 90
          transform: Rotation {
            origin.x: gridBackFace.width / 2; origin.y: gridBackFace.height / 2
            axis { x: 0; y: 1; z: 0 }
            angle: 180
          }

          Rectangle {
            anchors.fill: parent; radius: 12
            color: wallpaperSelector.colors ? wallpaperSelector.colors.surfaceContainer : "#1a1a2e"
            clip: true

            MouseArea {
              anchors.fill: parent
              acceptedButtons: Qt.RightButton
              z: -1
              onClicked: gridBackOverlay.hide()
            }

            Image {
              anchors.fill: parent
              source: gridBackOverlay.overlayData && gridBackOverlay.overlayData.thumb
                ? ImageService.fileUrl(gridBackOverlay.overlayData.thumb) : ""
              fillMode: Image.PreserveAspectCrop; opacity: 0.08
              sourceSize.width: 120
              sourceSize.height: 68
              asynchronous: true; cache: false
            }

            Column {
              id: gridBackContent
              anchors.centerIn: parent
              width: parent.width * 0.8
              spacing: 6

              Text {
                width: parent.width
                text: gridBackOverlay.overlayData ? gridBackOverlay.overlayData.name.replace(/\.[^/.]+$/, "").toUpperCase() : ""
                color: wallpaperSelector.colors ? wallpaperSelector.colors.tertiary : "#8bceff"
                font.family: Style.fontFamily; font.pixelSize: 15; font.weight: Font.Bold; font.letterSpacing: 1.2
                horizontalAlignment: Text.AlignHCenter; wrapMode: Text.Wrap; elide: Text.ElideRight; maximumLineCount: 2
              }

              Row {
                anchors.horizontalCenter: parent.horizontalCenter
                spacing: 0
                visible: gridBackOverlay.overlayData && gridBackOverlay.overlayData.type !== "we"
                Text {
                  text: gridBackOverlay.overlayData ? FileMetadataService.formatExt(gridBackOverlay.overlayData.name) : ""
                  color: wallpaperSelector.colors ? Qt.rgba(wallpaperSelector.colors.tertiary.r, wallpaperSelector.colors.tertiary.g, wallpaperSelector.colors.tertiary.b, 0.6) : Qt.rgba(1,1,1,0.35)
                  font.family: Style.fontFamily; font.pixelSize: 11; font.weight: Font.Medium; font.letterSpacing: 0.8
                }
                Text {
                  text: "  \u2022  "; color: Qt.rgba(1,1,1,0.15); font.family: Style.fontFamily; font.pixelSize: 11
                }
                Text {
                  text: gridBackOverlay._gridMeta ? (gridBackOverlay._gridMeta.width + " \u00d7 " + gridBackOverlay._gridMeta.height) : "\u2013"
                  color: wallpaperSelector.colors ? Qt.rgba(wallpaperSelector.colors.tertiary.r, wallpaperSelector.colors.tertiary.g, wallpaperSelector.colors.tertiary.b, 0.6) : Qt.rgba(1,1,1,0.35)
                  font.family: Style.fontFamily; font.pixelSize: 11; font.weight: Font.Medium; font.letterSpacing: 0.5
                }
                Text {
                  text: "  \u2022  "; color: Qt.rgba(1,1,1,0.15); font.family: Style.fontFamily; font.pixelSize: 11
                }
                Text {
                  text: gridBackOverlay._gridMeta ? FileMetadataService.formatSize(gridBackOverlay._gridMeta.filesize) : "\u2013"
                  color: wallpaperSelector.colors ? Qt.rgba(wallpaperSelector.colors.tertiary.r, wallpaperSelector.colors.tertiary.g, wallpaperSelector.colors.tertiary.b, 0.6) : Qt.rgba(1,1,1,0.35)
                  font.family: Style.fontFamily; font.pixelSize: 11; font.weight: Font.Medium; font.letterSpacing: 0.5
                }
              }

              Rectangle { width: parent.width; height: 1; color: Qt.rgba(1, 1, 1, 0.08) }

              Item {
                width: parent.width; height: 26
                Text {
                  anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                  text: I18n.tr("FAVOURITE")
                  color: wallpaperSelector.colors ? wallpaperSelector.colors.tertiary : "#8bceff"
                  font.family: Style.fontFamily; font.pixelSize: 12; font.weight: Font.Medium; font.letterSpacing: 0.5
                }
                Item {
                  id: gridFavToggle
                  anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                  width: 44; height: 22
                  property bool checked: false
                  Connections {
                    target: gridBackOverlay
                    function onOverlayOpenChanged() {
                      if (gridBackOverlay.overlayOpen && gridBackOverlay.overlayData) {
                        var key = (gridBackOverlay.overlayData.weId || "") !== "" ? gridBackOverlay.overlayData.weId : gridBackOverlay.overlayData.name
                        gridFavToggle.checked = wallpaperSelector.selectorService ? !!wallpaperSelector.selectorService.favouritesDb[key] : false
                      }
                    }
                  }
                  Canvas {
                    anchors.fill: parent
                    property bool isOn: gridFavToggle.checked
                    property color fillColor: isOn
                      ? (wallpaperSelector.colors ? wallpaperSelector.colors.primary : Style.fallbackAccent)
                      : Qt.rgba(1, 1, 1, 0.15)
                    onFillColorChanged: requestPaint(); onIsOnChanged: requestPaint()
                    onPaint: {
                      var ctx = getContext("2d"); ctx.clearRect(0, 0, width, height)
                      var sk = 6; ctx.fillStyle = fillColor; ctx.beginPath()
                      ctx.moveTo(sk, 0); ctx.lineTo(width, 0); ctx.lineTo(width - sk, height); ctx.lineTo(0, height)
                      ctx.closePath(); ctx.fill()
                    }
                  }
                  Canvas {
                    width: 20; height: 16; y: 3
                    x: gridFavToggle.checked ? parent.width - width - 3 : 3
                    Behavior on x { NumberAnimation { duration: Style.animFast; easing.type: Easing.OutCubic } }
                    property color knobColor: gridFavToggle.checked
                      ? (wallpaperSelector.colors ? wallpaperSelector.colors.primaryText : "#000")
                      : (wallpaperSelector.colors ? wallpaperSelector.colors.surfaceText : "#fff")
                    onKnobColorChanged: requestPaint()
                    onPaint: {
                      var ctx = getContext("2d"); ctx.clearRect(0, 0, width, height)
                      var sk = 4; ctx.fillStyle = knobColor; ctx.beginPath()
                      ctx.moveTo(sk, 0); ctx.lineTo(width, 0); ctx.lineTo(width - sk, height); ctx.lineTo(0, height)
                      ctx.closePath(); ctx.fill()
                    }
                  }
                  MouseArea {
                    anchors.fill: parent; cursorShape: Qt.PointingHandCursor
                    onClicked: {
                      if (!gridBackOverlay.overlayData) return
                      gridFavToggle.checked = !gridFavToggle.checked
                      wallpaperSelector.selectorService.toggleFavourite(gridBackOverlay.overlayData.name, gridBackOverlay.overlayData.weId || "")
                    }
                  }
                }
              }

              Rectangle { width: parent.width; height: 1; color: Qt.rgba(1, 1, 1, 0.08) }

              Item {
                width: parent.width; height: 26
                visible: Config.canOverviewBackdrop && Config.overviewBackdropEnabled && gridBackOverlay.overlayData && gridBackOverlay.overlayData.type === "static"
                property bool _isBackdrop: !!(gridBackOverlay.overlayData && Config.overviewBackdropPath === gridBackOverlay.overlayData.path)
                Text {
                  anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                  text: I18n.tr("OVERVIEW BACKDROP")
                  color: wallpaperSelector.colors ? wallpaperSelector.colors.tertiary : "#8bceff"
                  font.family: Style.fontFamily; font.pixelSize: 12; font.weight: Font.Medium; font.letterSpacing: 0.5
                }
                Rectangle {
                  id: gridBdBtn
                  anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                  height: 22; width: gridBdLbl.implicitWidth + 18; radius: 4
                  color: gridBdBtn.parent._isBackdrop
                    ? (wallpaperSelector.colors ? wallpaperSelector.colors.primary : Style.fallbackAccent)
                    : Qt.rgba(1, 1, 1, 0.12)
                  Behavior on color { ColorAnimation { duration: 140 } }
                  Text {
                    id: gridBdLbl
                    anchors.centerIn: parent
                    text: gridBdBtn.parent._isBackdrop ? I18n.tr("Current ✓") : I18n.tr("Set")
                    color: gridBdBtn.parent._isBackdrop
                      ? (wallpaperSelector.colors ? wallpaperSelector.colors.primaryText : "#000")
                      : (wallpaperSelector.colors ? wallpaperSelector.colors.surfaceText : "#fff")
                    font.family: Style.fontFamily; font.pixelSize: 11; font.weight: Font.Medium
                  }
                  MouseArea {
                    anchors.fill: parent; cursorShape: Qt.PointingHandCursor
                    onClicked: {
                      if (!gridBackOverlay.overlayData) return
                      if (gridBdBtn.parent._isBackdrop) wallpaperSelector.selectorService.applyBackdrop("")
                      else wallpaperSelector.selectorService.applyBackdrop(gridBackOverlay.overlayData.path)
                    }
                  }
                }
              }

              Rectangle { width: parent.width; height: 1; color: Qt.rgba(1, 1, 1, 0.08) }

              Row {
                id: gridActionRow
                width: parent.width; height: 32; spacing: 8

                property int _slotCount: gridBackOverlay.overlayData && gridBackOverlay.overlayData.type === "we" ? 3 : 2
                property real _slotWidth: (width - spacing * (_slotCount - 1)) / _slotCount

                ActionButton {
                  width: gridActionRow._slotWidth
                  colors: wallpaperSelector.colors
                  icon: "\u{f0208}"; label: I18n.tr("VIEW")
                  onClicked: { if (!gridBackOverlay.overlayData) return; var p = gridBackOverlay.overlayData.path; Qt.openUrlExternally(ImageService.fileUrl(p.substring(0, p.lastIndexOf("/")))); gridBackOverlay.hide() }
                }

                ActionButton {
                  width: gridActionRow._slotWidth
                  colors: wallpaperSelector.colors
                  icon: "\u{f0a79}"; label: I18n.tr("DELETE"); danger: true
                  onClicked: { if (!gridBackOverlay.overlayData) return; wallpaperSelector.selectorService.deleteWallpaperItem(gridBackOverlay.overlayData.type, gridBackOverlay.overlayData.name, gridBackOverlay.overlayData.weId || ""); gridBackOverlay.hide() }
                }

                ActionButton {
                  visible: gridBackOverlay.overlayData && gridBackOverlay.overlayData.type === "we"
                  width: visible ? gridActionRow._slotWidth : 0
                  colors: wallpaperSelector.colors
                  icon: "\u{f0bef}"; label: I18n.tr("STEAM")
                  onClicked: { wallpaperSelector.selectorService.openSteamPage(gridBackOverlay.overlayData.weId || ""); gridBackOverlay.hide() }
                }
              }
            }
          }

          Rectangle {
            anchors.fill: parent; radius: 12
            color: "transparent"
            border.width: 2.5
            border.color: wallpaperSelector.colors ? wallpaperSelector.colors.primary : Style.fallbackAccent
          }
        }

      }
    }

    Item {
      id: hexBackOverlay
      anchors.fill: parent
      visible: false
      z: 200

      property var overlayData: null
      property string overlayItemKey: ""
      property var _sourceItem: null
      property real sourceX: 0
      property real sourceY: 0
      property real _openContentX: 0
      property bool overlayOpen: false
      property var _hexMeta: null

      readonly property real bigR: wallpaperSelector.hexRadius * 3

      onOverlayOpenChanged: {
        if (overlayOpen && overlayData && overlayData.type !== "we") {
          var key = ImageService.thumbKey(overlayData.thumb, overlayData.name)
          _hexMeta = FileMetadataService.getMetadata(key)
          if (!_hexMeta)
            FileMetadataService.probeIfNeeded(key, overlayData.path, overlayData.type === "video" ? "video" : "image")
        }
      }
      Connections {
        target: FileMetadataService
        enabled: hexBackOverlay.overlayOpen
        function onMetadataReady(key) {
          if (!hexBackOverlay.overlayData) return
          var myKey = ImageService.thumbKey(hexBackOverlay.overlayData.thumb, hexBackOverlay.overlayData.name)
          if (key === myKey)
            hexBackOverlay._hexMeta = FileMetadataService.getMetadata(key)
        }
      }
      readonly property real bigW: bigR * 2
      readonly property real bigH: Math.ceil(bigR * 1.73205)
      readonly property real _cos30: 0.866025
      readonly property real _sin30: 0.5

      function show(data, gx, gy, sourceItem) {
        overlayData = data
        overlayItemKey = (data.weId || "") !== "" ? data.weId : data.name
        _sourceItem = sourceItem || null
        _openContentX = hexListView.contentX
        var local = hexBackOverlay.mapFromItem(null, gx, gy)
        sourceX = local.x
        sourceY = local.y
        visible = true
        overlayOpen = true
      }

      function hide() {
        var scrollDelta = hexListView.contentX - _openContentX
        sourceX -= scrollDelta
        _openContentX = hexListView.contentX
        overlayOpen = false
      }

      Rectangle {
        anchors.fill: parent
        color: Qt.rgba(0, 0, 0, hexBackOverlay.overlayOpen ? 0.55 : 0)
        Behavior on color { ColorAnimation { duration: Style.animNormal } }

        MouseArea {
          anchors.fill: parent
          acceptedButtons: Qt.LeftButton | Qt.RightButton
          onClicked: hexBackOverlay.hide()
        }
      }

      states: [
        State {
          name: "hidden"
          when: !hexBackOverlay.overlayOpen
          PropertyChanges {
            target: hexCard
            x: hexBackOverlay.sourceX - hexCard.width / 2
            y: hexBackOverlay.sourceY - hexCard.height / 2
            scale: wallpaperSelector.hexRadius / hexBackOverlay.bigR
            opacity: 0
          }
          PropertyChanges {
            target: cardRotation
            angle: 0
          }
        },
        State {
          name: "visible"
          when: hexBackOverlay.overlayOpen
          PropertyChanges {
            target: hexCard
            x: (hexBackOverlay.width - hexCard.width) / 2
            y: (hexBackOverlay.height - hexCard.height) / 2
            scale: 1
            opacity: 1
          }
          PropertyChanges {
            target: cardRotation
            angle: 180
          }
        }
      ]

      transitions: [
        Transition {
          from: "hidden"; to: "visible"
          SequentialAnimation {
            PropertyAction { target: hexBackOverlay; property: "visible"; value: true }
            ParallelAnimation {
              NumberAnimation { target: hexCard; properties: "x,y,scale,opacity"; duration: Style.animSlow; easing.type: Easing.OutCubic }
              NumberAnimation { target: cardRotation; property: "angle"; duration: Style.animSlow; easing.type: Easing.InOutQuad }
            }
          }
        },
        Transition {
          from: "visible"; to: "hidden"
          SequentialAnimation {
            ParallelAnimation {
              NumberAnimation { target: hexCard; properties: "x,y,scale"; duration: Style.animSlow; easing.type: Easing.InOutCubic }
              NumberAnimation { target: cardRotation; property: "angle"; duration: Style.animSlow; easing.type: Easing.InOutQuad }
              SequentialAnimation {
                PauseAnimation { duration: Style.animSlow * 0.7 }
                NumberAnimation { target: hexCard; property: "opacity"; duration: Style.animSlow * 0.3; easing.type: Easing.InQuad }
              }
            }
            PropertyAction { target: hexBackOverlay; property: "visible"; value: false }
            PropertyAction { target: hexBackOverlay; property: "overlayItemKey"; value: "" }
            PropertyAction { target: hexBackOverlay; property: "_sourceItem"; value: null }
          }
        }
      ]

      Item {
        id: hexCard
        width: hexBackOverlay.bigW
        height: hexBackOverlay.bigH
        transformOrigin: Item.Center

        transform: Rotation {
          id: cardRotation
          origin.x: hexCard.width / 2
          origin.y: hexCard.height / 2
          axis { x: 0; y: 1; z: 0 }
          angle: 0
        }

        Item {
          id: bigHexMask
          width: hexCard.width; height: hexCard.height
          visible: false
          layer.enabled: true
          Shape {
            anchors.fill: parent; antialiasing: true; preferredRendererType: Shape.CurveRenderer
            ShapePath {
              fillColor: "white"; strokeColor: "transparent"
              startX: hexBackOverlay.bigR * 2;  startY: hexCard.height / 2
              PathLine { x: hexBackOverlay.bigR + hexBackOverlay.bigR * hexBackOverlay._sin30; y: hexCard.height / 2 - hexBackOverlay.bigR * hexBackOverlay._cos30 }
              PathLine { x: hexBackOverlay.bigR - hexBackOverlay.bigR * hexBackOverlay._sin30; y: hexCard.height / 2 - hexBackOverlay.bigR * hexBackOverlay._cos30 }
              PathLine { x: 0;                                                                  y: hexCard.height / 2 }
              PathLine { x: hexBackOverlay.bigR - hexBackOverlay.bigR * hexBackOverlay._sin30; y: hexCard.height / 2 + hexBackOverlay.bigR * hexBackOverlay._cos30 }
              PathLine { x: hexBackOverlay.bigR + hexBackOverlay.bigR * hexBackOverlay._sin30; y: hexCard.height / 2 + hexBackOverlay.bigR * hexBackOverlay._cos30 }
              PathLine { x: hexBackOverlay.bigR * 2;                                            y: hexCard.height / 2 }
            }
          }
        }

        Item {
          id: frontFace
          anchors.fill: parent
          visible: cardRotation.angle < 90

          Item {
            anchors.fill: parent
            Image {
              anchors.fill: parent
              source: hexBackOverlay.overlayData && hexBackOverlay.overlayData.thumb
                ? ImageService.fileUrl(hexBackOverlay.overlayData.thumb) : ""
              fillMode: Image.PreserveAspectCrop
              smooth: true
              asynchronous: true; cache: false
              sourceSize.width: hexBackOverlay.bigW
              sourceSize.height: hexBackOverlay.bigH
            }
            layer.enabled: true; layer.smooth: true
            layer.effect: MultiEffect { maskEnabled: true; maskSource: bigHexMask; maskThresholdMin: 0.3; maskSpreadAtMin: 0.3 }
          }

          Shape {
            anchors.fill: parent; antialiasing: true; preferredRendererType: Shape.CurveRenderer
            ShapePath {
              fillColor: "transparent"
              strokeColor: wallpaperSelector.colors ? wallpaperSelector.colors.primary : Style.fallbackAccent
              strokeWidth: 2
              startX: hexBackOverlay.bigR * 2;  startY: hexCard.height / 2
              PathLine { x: hexBackOverlay.bigR + hexBackOverlay.bigR * hexBackOverlay._sin30; y: hexCard.height / 2 - hexBackOverlay.bigR * hexBackOverlay._cos30 }
              PathLine { x: hexBackOverlay.bigR - hexBackOverlay.bigR * hexBackOverlay._sin30; y: hexCard.height / 2 - hexBackOverlay.bigR * hexBackOverlay._cos30 }
              PathLine { x: 0;                                                                  y: hexCard.height / 2 }
              PathLine { x: hexBackOverlay.bigR - hexBackOverlay.bigR * hexBackOverlay._sin30; y: hexCard.height / 2 + hexBackOverlay.bigR * hexBackOverlay._cos30 }
              PathLine { x: hexBackOverlay.bigR + hexBackOverlay.bigR * hexBackOverlay._sin30; y: hexCard.height / 2 + hexBackOverlay.bigR * hexBackOverlay._cos30 }
              PathLine { x: hexBackOverlay.bigR * 2;                                            y: hexCard.height / 2 }
            }
          }

        }

        Item {
          id: backFace
          anchors.fill: parent
          visible: cardRotation.angle >= 90
          transform: Rotation {
            origin.x: backFace.width / 2; origin.y: backFace.height / 2
            axis { x: 0; y: 1; z: 0 }
            angle: 180
          }

          Item {
            id: backClip
            anchors.fill: parent

            MouseArea {
              anchors.fill: parent
              acceptedButtons: Qt.RightButton
              z: -1
              onClicked: hexBackOverlay.hide()
            }

            Rectangle { anchors.fill: parent; color: wallpaperSelector.colors ? wallpaperSelector.colors.surfaceContainer : "#1a1a2e" }

            Image {
              anchors.fill: parent
              source: hexBackOverlay.overlayData && hexBackOverlay.overlayData.thumb
                ? ImageService.fileUrl(hexBackOverlay.overlayData.thumb) : ""
              fillMode: Image.PreserveAspectCrop; opacity: 0.08
              sourceSize.width: 120
              sourceSize.height: 104
              asynchronous: true; cache: false
            }

            Column {
              id: backContent
              anchors.centerIn: parent
              width: hexBackOverlay.bigR * 1.6
              spacing: 4

              Text {
                width: parent.width
                text: hexBackOverlay.overlayData ? hexBackOverlay.overlayData.name.replace(/\.[^/.]+$/, "").toUpperCase() : ""
                color: wallpaperSelector.colors ? wallpaperSelector.colors.tertiary : "#8bceff"
                font.family: Style.fontFamily; font.pixelSize: 15; font.weight: Font.Bold; font.letterSpacing: 1.2
                horizontalAlignment: Text.AlignHCenter; wrapMode: Text.Wrap; elide: Text.ElideRight; maximumLineCount: 2
              }

              Row {
                anchors.horizontalCenter: parent.horizontalCenter
                spacing: 0
                visible: hexBackOverlay.overlayData && hexBackOverlay.overlayData.type !== "we"
                Text {
                  text: hexBackOverlay.overlayData ? FileMetadataService.formatExt(hexBackOverlay.overlayData.name) : ""
                  color: wallpaperSelector.colors ? Qt.rgba(wallpaperSelector.colors.tertiary.r, wallpaperSelector.colors.tertiary.g, wallpaperSelector.colors.tertiary.b, 0.6) : Qt.rgba(1,1,1,0.35)
                  font.family: Style.fontFamily; font.pixelSize: 11; font.weight: Font.Medium; font.letterSpacing: 0.8
                }
                Text {
                  text: "  \u2022  "; color: Qt.rgba(1,1,1,0.15); font.family: Style.fontFamily; font.pixelSize: 11
                }
                Text {
                  text: hexBackOverlay._hexMeta ? (hexBackOverlay._hexMeta.width + " \u00d7 " + hexBackOverlay._hexMeta.height) : "\u2013"
                  color: wallpaperSelector.colors ? Qt.rgba(wallpaperSelector.colors.tertiary.r, wallpaperSelector.colors.tertiary.g, wallpaperSelector.colors.tertiary.b, 0.6) : Qt.rgba(1,1,1,0.35)
                  font.family: Style.fontFamily; font.pixelSize: 11; font.weight: Font.Medium; font.letterSpacing: 0.5
                }
                Text {
                  text: "  \u2022  "; color: Qt.rgba(1,1,1,0.15); font.family: Style.fontFamily; font.pixelSize: 11
                }
                Text {
                  text: hexBackOverlay._hexMeta ? FileMetadataService.formatSize(hexBackOverlay._hexMeta.filesize) : "\u2013"
                  color: wallpaperSelector.colors ? Qt.rgba(wallpaperSelector.colors.tertiary.r, wallpaperSelector.colors.tertiary.g, wallpaperSelector.colors.tertiary.b, 0.6) : Qt.rgba(1,1,1,0.35)
                  font.family: Style.fontFamily; font.pixelSize: 11; font.weight: Font.Medium; font.letterSpacing: 0.5
                }
              }

              Rectangle { width: parent.width; height: 1; color: Qt.rgba(1, 1, 1, 0.08) }

              Item {
                width: parent.width; height: 26
                Text {
                  anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                  text: I18n.tr("FAVOURITE")
                  color: wallpaperSelector.colors ? wallpaperSelector.colors.tertiary : "#8bceff"
                  font.family: Style.fontFamily; font.pixelSize: 12; font.weight: Font.Medium; font.letterSpacing: 0.5
                }
                Item {
                  id: overlayFavToggle
                  anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                  width: 44; height: 22
                  property bool checked: false
                  Connections {
                    target: hexBackOverlay
                    function onOverlayOpenChanged() {
                      if (hexBackOverlay.overlayOpen && hexBackOverlay.overlayData) {
                        var key = (hexBackOverlay.overlayData.weId || "") !== "" ? hexBackOverlay.overlayData.weId : hexBackOverlay.overlayData.name
                        overlayFavToggle.checked = wallpaperSelector.selectorService ? !!wallpaperSelector.selectorService.favouritesDb[key] : false
                      }
                    }
                  }
                  Canvas {
                    anchors.fill: parent
                    property bool isOn: overlayFavToggle.checked
                    property color fillColor: isOn
                      ? (wallpaperSelector.colors ? wallpaperSelector.colors.primary : Style.fallbackAccent)
                      : Qt.rgba(1, 1, 1, 0.15)
                    onFillColorChanged: requestPaint(); onIsOnChanged: requestPaint()
                    onPaint: {
                      var ctx = getContext("2d"); ctx.clearRect(0, 0, width, height)
                      var sk = 6; ctx.fillStyle = fillColor; ctx.beginPath()
                      ctx.moveTo(sk, 0); ctx.lineTo(width, 0); ctx.lineTo(width - sk, height); ctx.lineTo(0, height)
                      ctx.closePath(); ctx.fill()
                    }
                  }
                  Canvas {
                    width: 20; height: 16; y: 3
                    x: overlayFavToggle.checked ? parent.width - width - 3 : 3
                    Behavior on x { NumberAnimation { duration: Style.animFast; easing.type: Easing.OutCubic } }
                    property color knobColor: overlayFavToggle.checked
                      ? (wallpaperSelector.colors ? wallpaperSelector.colors.primaryText : "#000")
                      : (wallpaperSelector.colors ? wallpaperSelector.colors.surfaceText : "#fff")
                    onKnobColorChanged: requestPaint()
                    onPaint: {
                      var ctx = getContext("2d"); ctx.clearRect(0, 0, width, height)
                      var sk = 4; ctx.fillStyle = knobColor; ctx.beginPath()
                      ctx.moveTo(sk, 0); ctx.lineTo(width, 0); ctx.lineTo(width - sk, height); ctx.lineTo(0, height)
                      ctx.closePath(); ctx.fill()
                    }
                  }
                  MouseArea {
                    anchors.fill: parent; cursorShape: Qt.PointingHandCursor
                    onClicked: {
                      if (!hexBackOverlay.overlayData) return
                      overlayFavToggle.checked = !overlayFavToggle.checked
                      wallpaperSelector.selectorService.toggleFavourite(hexBackOverlay.overlayData.name, hexBackOverlay.overlayData.weId || "")
                    }
                  }
                }
              }

              Rectangle { width: parent.width; height: 1; color: Qt.rgba(1, 1, 1, 0.08) }

              Item {
                width: parent.width; height: 26
                visible: Config.canOverviewBackdrop && Config.overviewBackdropEnabled && hexBackOverlay.overlayData && hexBackOverlay.overlayData.type === "static"
                property bool _isBackdrop: !!(hexBackOverlay.overlayData && Config.overviewBackdropPath === hexBackOverlay.overlayData.path)
                Text {
                  anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                  text: I18n.tr("OVERVIEW BACKDROP")
                  color: wallpaperSelector.colors ? wallpaperSelector.colors.tertiary : "#8bceff"
                  font.family: Style.fontFamily; font.pixelSize: 12; font.weight: Font.Medium; font.letterSpacing: 0.5
                }
                Rectangle {
                  id: hexBdBtn
                  anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                  height: 22; width: hexBdLbl.implicitWidth + 18; radius: 4
                  color: hexBdBtn.parent._isBackdrop
                    ? (wallpaperSelector.colors ? wallpaperSelector.colors.primary : Style.fallbackAccent)
                    : Qt.rgba(1, 1, 1, 0.12)
                  Behavior on color { ColorAnimation { duration: 140 } }
                  Text {
                    id: hexBdLbl
                    anchors.centerIn: parent
                    text: hexBdBtn.parent._isBackdrop ? I18n.tr("Current ✓") : I18n.tr("Set")
                    color: hexBdBtn.parent._isBackdrop
                      ? (wallpaperSelector.colors ? wallpaperSelector.colors.primaryText : "#000")
                      : (wallpaperSelector.colors ? wallpaperSelector.colors.surfaceText : "#fff")
                    font.family: Style.fontFamily; font.pixelSize: 11; font.weight: Font.Medium
                  }
                  MouseArea {
                    anchors.fill: parent; cursorShape: Qt.PointingHandCursor
                    onClicked: {
                      if (!hexBackOverlay.overlayData) return
                      if (hexBdBtn.parent._isBackdrop) wallpaperSelector.selectorService.applyBackdrop("")
                      else wallpaperSelector.selectorService.applyBackdrop(hexBackOverlay.overlayData.path)
                    }
                  }
                }
              }

              Rectangle { width: parent.width; height: 1; color: Qt.rgba(1, 1, 1, 0.08) }

              Row {
                id: overlayActionRow
                width: parent.width; height: 32; spacing: 8

                property int _slotCount: hexBackOverlay.overlayData && hexBackOverlay.overlayData.type === "we" ? 3 : 2
                property real _slotWidth: (width - spacing * (_slotCount - 1)) / _slotCount

                ActionButton {
                  width: overlayActionRow._slotWidth
                  colors: wallpaperSelector.colors
                  icon: "\u{f0208}"; label: I18n.tr("VIEW")
                  onClicked: { if (!hexBackOverlay.overlayData) return; var p = hexBackOverlay.overlayData.path; Qt.openUrlExternally(ImageService.fileUrl(p.substring(0, p.lastIndexOf("/")))); hexBackOverlay.hide() }
                }

                ActionButton {
                  width: overlayActionRow._slotWidth
                  colors: wallpaperSelector.colors
                  icon: "\u{f0a79}"; label: I18n.tr("DELETE"); danger: true
                  onClicked: { if (!hexBackOverlay.overlayData) return; wallpaperSelector.selectorService.deleteWallpaperItem(hexBackOverlay.overlayData.type, hexBackOverlay.overlayData.name, hexBackOverlay.overlayData.weId || ""); hexBackOverlay.hide() }
                }

                ActionButton {
                  visible: hexBackOverlay.overlayData && hexBackOverlay.overlayData.type === "we"
                  width: visible ? overlayActionRow._slotWidth : 0
                  colors: wallpaperSelector.colors
                  icon: "\u{f0bef}"; label: I18n.tr("STEAM")
                  onClicked: { wallpaperSelector.selectorService.openSteamPage(hexBackOverlay.overlayData.weId || ""); hexBackOverlay.hide() }
                }
              }
            }

            layer.enabled: true; layer.smooth: true
            layer.effect: MultiEffect { maskEnabled: true; maskSource: bigHexMask; maskThresholdMin: 0.3; maskSpreadAtMin: 0.3 }
          }

          Shape {
            anchors.fill: parent; antialiasing: true; preferredRendererType: Shape.CurveRenderer
            ShapePath {
              fillColor: "transparent"
              strokeColor: wallpaperSelector.colors ? wallpaperSelector.colors.primary : Style.fallbackAccent
              strokeWidth: 2.5
              startX: hexBackOverlay.bigR * 2;  startY: hexCard.height / 2
              PathLine { x: hexBackOverlay.bigR + hexBackOverlay.bigR * hexBackOverlay._sin30; y: hexCard.height / 2 - hexBackOverlay.bigR * hexBackOverlay._cos30 }
              PathLine { x: hexBackOverlay.bigR - hexBackOverlay.bigR * hexBackOverlay._sin30; y: hexCard.height / 2 - hexBackOverlay.bigR * hexBackOverlay._cos30 }
              PathLine { x: 0;                                                                  y: hexCard.height / 2 }
              PathLine { x: hexBackOverlay.bigR - hexBackOverlay.bigR * hexBackOverlay._sin30; y: hexCard.height / 2 + hexBackOverlay.bigR * hexBackOverlay._cos30 }
              PathLine { x: hexBackOverlay.bigR + hexBackOverlay.bigR * hexBackOverlay._sin30; y: hexCard.height / 2 + hexBackOverlay.bigR * hexBackOverlay._cos30 }
              PathLine { x: hexBackOverlay.bigR * 2;                                            y: hexCard.height / 2 }
            }
          }

        }

      }
    }

  MonitorPickerPopup {
    id: _monitorPicker
    anchors.fill: parent
    z: 1100
    colors: wallpaperSelector.colors
    wallpaperService: service
    onAccepted: function(item, outputs, audioMap, volumeMap) {
      wallpaperSelector._doApply(item, outputs, audioMap, volumeMap)
    }
    onThemeApplied: function(scheme, mode, colorIndex) {
      Config.saveKey("matugen.schemeType", scheme)
      Config.saveKey("matugen.mode", mode)
      Config.saveKey("matugen.colorIndex", colorIndex)
      DaemonClient.retheme(scheme, mode, colorIndex)
    }
  }

  Loader {
    id: riceWorkshopLoader
    active: wallpaperSelector._workshopOpen
    anchors.fill: parent
    z: 1090
    sourceComponent: Component {
      RiceWorkshop {
        colors: wallpaperSelector.colors
        open: true
        rice: wallpaperSelector._workshopRice
        onApplyRequested: function(slug) {
          Quickshell.execDetached(["ryoku-hub", "rice", "apply", slug, "all"])
          wallpaperSelector._workshopOpen = false
          wallpaperSelector.ricesOpen = false
          _closeAfterApply()
        }
        onForkRequested: function(slug) {
          Quickshell.execDetached(["ryoku-hub", "rice", "fork", slug])
          _riceReload.restart()
        }
        onRestoreRequested: {
          Quickshell.execDetached(["ryoku-hub", "rice", "restore"])
          wallpaperSelector._workshopOpen = false
          wallpaperSelector.ricesOpen = false
        }
        onDeleteRequested: function(slug, name) {
          wallpaperSelector._deleteConfirmSlug = slug
          wallpaperSelector._deleteConfirmName = name
          wallpaperSelector._workshopOpen = false
        }
        onSaveLookRequested: {
          wallpaperSelector._workshopOpen = false
          wallpaperSelector._capturePromptOpen = true
        }
        onExportRequested: function(slug, folder) {
          Quickshell.execDetached(["ryoku-hub", "rice", "export", slug, folder])
        }
        onImportRequested: function(folder) {
          Quickshell.execDetached(["ryoku-hub", "rice", "import", folder])
          _riceReload.restart()
        }
        onCloseRequested: {
          wallpaperSelector._workshopOpen = false
          wallpaperSelector._focusActiveList()
        }
      }
    }
  }

  }
}
