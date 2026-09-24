
import QtQuick
import QtQuick.Controls
import QtQuick.Shapes
import QtQuick.Window
import Quickshell.Io
import ".."
import "../services"
import Ryoku.Ui.Singletons

Item {
  id: settingsPanel

  property var colors
  property var service
  property bool settingsOpen: false
  property string activeTab: "selector"
  property bool openDownward: false
  property string sourcePath: ""

  // Basic (visual) vs Advanced (backend) tab set. Off by default so the picker
  // opens on the four look tabs; the ADVANCED toggle in the strip reveals the
  // behaviour/backend tabs so the surface never feels bloated.
  property bool showAdvanced: false

  property string _lastConvertResult: ""
  property string _lastOptimizeResult: ""

  signal themeChanged(string scheme, string mode, var colorIndex)

  function _s(v) { return v * Config.uiScale }

  readonly property real _keybindsColW: _s(198)

  // Which tab keys are valid in the current mode. Keep in step with the strip
  // Repeater below; used only to clamp activeTab when the mode flips.
  function _validTab(key) {
    if (!showAdvanced)
      return ["selector", "paper", "edit", "theme"].indexOf(key) >= 0
    var adv = ["general", "playlists", "paths", "comfort", "lighting", "performance", "postprocessing"]
    if (Config.matugenEnabled) adv.push("matugen")
    if (Config.canOverviewBackdrop) adv.push("overview-backdrop")
    if (Config.steamEnabled) adv.push("wallpaper-engine")
    return adv.indexOf(key) >= 0
  }

  onShowAdvancedChanged: {
    if (!_validTab(activeTab))
      activeTab = showAdvanced ? "general" : "selector"
  }

  Connections {
    target: Config
    function onMatugenEnabledChanged() {
      if (!Config.matugenEnabled && settingsPanel.activeTab === "matugen")
        settingsPanel.activeTab = "general"
    }
    function onSteamEnabledChanged() {
      if (!Config.steamEnabled && settingsPanel.activeTab === "wallpaper-engine")
        settingsPanel.activeTab = "general"
    }
  }

  Connections {
    target: ImageOptimizeService
    function onFinished(optimized, skippedCount, failed) {
      var parts = []
      if (optimized > 0) parts.push(I18n.tr("%1 optimized").arg(optimized))
      if (skippedCount > 0) parts.push(I18n.tr("%1 skipped").arg(skippedCount))
      if (failed > 0) parts.push(I18n.tr("%1 failed").arg(failed))
      settingsPanel._lastOptimizeResult = parts.join(" · ") || I18n.tr("Nothing to optimize")
    }
  }

  z: 102
  width: Math.min(((settingsPanel.activeTab === "performance" ? 1080 : (settingsPanel.activeTab === "general" || settingsPanel.activeTab === "edit") ? 900 : 760) * Config.uiScale) + _keybindsColW + _s(24), Screen.width - _s(48))
  Behavior on width { NumberAnimation { duration: Style.animFast; easing.type: Easing.OutCubic } }
  // A tab taller than the screen scrolls inside contentLoader instead of
  // running off the bottom edge. The budget is the screen minus the panel's
  // own offset (the picker places it _s(16) from the top), the tab strip and
  // the bottom padding; the panel height then follows the capped content.
  readonly property real _contentMaxHeight: Math.max(
    _s(240),
    Screen.height - _s(16) - _s(16) - tabRow.height - 36)
  height: tabRow.height + contentLoader.height + 36

  visible: settingsOpen || opacity > 0.01
  opacity: settingsOpen ? 1 : 0
  scale: settingsOpen ? 1 : 0.95
  transformOrigin: openDownward ? Item.Top : Item.Bottom
  Behavior on opacity { NumberAnimation { duration: Style.animNormal; easing.type: Easing.OutCubic } }
  Behavior on scale { NumberAnimation { duration: Style.animNormal; easing.type: Easing.OutCubic } }

  signal closeRequested()

  Keys.onEscapePressed: closeRequested()
  focus: settingsOpen

  MouseArea {
    anchors.fill: parent
    acceptedButtons: Qt.LeftButton | Qt.RightButton
    onClicked: function(mouse) {
      if (mouse.button === Qt.RightButton) settingsPanel.closeRequested()
    }
  }

  function _cloneIntegrations() {
    return Config.integrations.map(function(e) { return JSON.parse(JSON.stringify(e)) })
  }

  function _saveField(key, value) {
    if (!Config._data.components || typeof Config._data.components.wallpaperSelector !== "object" || Config._data.components.wallpaperSelector === null)
      Config.saveKey("components.wallpaperSelector.enabled", true)
    Config.saveKey("components.wallpaperSelector." + key, value)
  }

  function _saveConfigKey(path, value) {
    Config.saveKey(path, value)
  }

  function _showWarning(title, message) {
    _warningPopup.title = title
    _warningPopup.message = message
    _warningPopup.open()
  }

  function _applyPreset(expanded, sliceH, sliceW, visible, gap, skew) {
    Config.saveKey("components.wallpaperSelector.expandedWidth", expanded)
    Config.saveKey("components.wallpaperSelector.sliceHeight", sliceH)
    Config.saveKey("components.wallpaperSelector.sliceWidth", sliceW)
    Config.saveKey("components.wallpaperSelector.visibleCount", visible)
    Config.saveKey("components.wallpaperSelector.sliceSpacing", gap)
    Config.saveKey("components.wallpaperSelector.skewOffset", skew)
  }

  function _saveCustomPreset(slot) {
    var key = slot + "_" + Config.displayMode
    var preset = {}
    if (Config.displayMode === "slices") {
      preset = {
        expandedWidth: Config.wallpaperExpandedWidth,
        sliceHeight: Config.wallpaperSliceHeight,
        sliceWidth: Config.wallpaperSliceWidth,
        visibleCount: Config.wallpaperVisibleCount,
        sliceSpacing: Config.wallpaperSliceSpacing,
        skewOffset: Config.wallpaperSkewOffset
      }
    } else if (Config.displayMode === "hex") {
      preset = {
        hexRadius: Config.hexRadius,
        hexRows: Config.hexRows,
        hexCols: Config.hexCols,
        hexScrollStep: Config.hexScrollStep,
        hexArc: Config.hexArc,
        hexArcIntensity: Config.hexArcIntensity,
        hexCurve: Config.hexCurve,
        hexShape: Config.hexShape,
        hexWaves: Config.hexWaves,
        hexGapX: Config.hexGapX,
        hexGapY: Config.hexGapY,
        hexStagger: Config.hexStagger,
        hexLens: Config.hexLens,
        hexTwist: Config.hexTwist,
        hexScatter: Config.hexScatter
      }
    } else if (Config.displayMode === "wall") {
      preset = {
        gridColumns: Config.gridColumns,
        gridRows: Config.gridRows,
        gridThumbWidth: Config.gridThumbWidth,
        gridThumbHeight: Config.gridThumbHeight
      }
    } else if (Config.displayMode === "hand") {
      preset = {
        handCardWidth: Config.handCardWidth, handCardHeight: Config.handCardHeight,
        handCount: Config.handCount, handFanAngle: Config.handFanAngle,
        handFanRoll: Config.handFanRoll, handArch: Config.handArch,
        handCornerRadius: Config.handCornerRadius, handSkew: Config.handSkew,
        handSpread: Config.handSpread, handSpeed: Config.handSpeed,
        handTilt: Config.handTilt, handPerspective: Config.handPerspective,
        handGhosts: Config.handGhosts, handBob: Config.handBob, handBackdrop: Config.handBackdrop
      }
    } else if (Config.displayMode === "sandy") {
      preset = {
        sandyCenter: Config.sandyCenter, sandySliceWidth: Config.sandySliceWidth,
        sandySliceHeight: Config.sandySliceHeight, sandySkew: Config.sandySkew,
        sandySpacing: Config.sandySpacing, sandyDuration: Config.sandyDuration,
        sandyStrands: Config.sandyStrands, sandyTwist: Config.sandyTwist,
        sandyOrbit: Config.sandyOrbit, sandyTurbulence: Config.sandyTurbulence,
        sandyWaist: Config.sandyWaist, sandyFront: Config.sandyFront,
        sandyArc: Config.sandyArc, sandyEdgeSpeed: Config.sandyEdgeSpeed,
        sandyGrain: Config.sandyGrain, sandyFan: Config.sandyFan
      }
    } else if (Config.displayMode === "grid") {
      preset = {
        gridLayout: Config.gridLayout, gridColumns: Config.gridColumns, gridRows: Config.gridRows,
        gridThumbWidth: Config.gridThumbWidth, gridThumbHeight: Config.gridThumbHeight,
        gridStagger: Config.gridStagger, gridSelectedScale: Config.gridSelectedScale,
        gridFlowWave: Config.gridFlowWave, gridFlowFrequency: Config.gridFlowFrequency,
        gridScatter: Config.gridScatter, gridScaleVariance: Config.gridScaleVariance,
        gridCylinderBend: Config.gridCylinderBend, gridCylinderRadius: Config.gridCylinderRadius
      }
    }
    Config.saveKey("components.wallpaperSelector.customPresets." + key, preset)
  }

  function _loadCustomPreset(slot) {
    var key = slot + "_" + Config.displayMode
    var p = Config.wallpaperCustomPresets[key]
    if (!p) return
    if (Config.displayMode === "slices") {
      _applyPreset(p.expandedWidth, p.sliceHeight, p.sliceWidth, p.visibleCount, p.sliceSpacing, p.skewOffset)
      return
    }
    // Every other mode persists a flat map of selector fields; replay each one.
    for (var k in p) {
      if (p[k] !== undefined) settingsPanel._saveField(k, p[k])
    }
  }

  property int _tabSkew: 14

  // Persistent slim keybind cheat-sheet down the left edge, shown for every tab.
  Item {
    id: keybindsColumn
    anchors.left: parent.left
    anchors.top: parent.top
    anchors.leftMargin: settingsPanel._s(12)
    anchors.topMargin: settingsPanel._s(12)
    width: settingsPanel._keybindsColW
    height: keybindsList.height + settingsPanel._s(28)
    z: 11

    Rectangle {
      anchors.fill: parent
      radius: Style.radiusLarge
      color: settingsPanel.colors ? Qt.rgba(settingsPanel.colors.surface.r, settingsPanel.colors.surface.g, settingsPanel.colors.surface.b, 0.95) : Qt.rgba(0.06, 0.07, 0.09, 0.95)
      border.width: 1
      border.color: settingsPanel.colors ? Qt.rgba(settingsPanel.colors.surfaceText.r, settingsPanel.colors.surfaceText.g, settingsPanel.colors.surfaceText.b, 0.18) : Qt.rgba(1, 1, 1, 0.12)
    }

    Loader {
      id: keybindsList
      anchors { left: parent.left; right: parent.right; top: parent.top }
      anchors.leftMargin: settingsPanel._s(14); anchors.rightMargin: settingsPanel._s(14); anchors.topMargin: settingsPanel._s(14)
      source: "settings/KeybindsSettings.qml"
      onLoaded: item.colors = Qt.binding(function() { return settingsPanel.colors })
    }
  }

  // backdrop behind the tab row so the tabs read over the dimmed wallpaper
  Rectangle {
    anchors.fill: tabRow
    anchors.topMargin: -6; anchors.bottomMargin: -6
    anchors.leftMargin: -10; anchors.rightMargin: -10
    radius: Style.radiusLarge
    color: settingsPanel.colors ? Qt.rgba(settingsPanel.colors.surface.r, settingsPanel.colors.surface.g, settingsPanel.colors.surface.b, 0.95) : Qt.rgba(0.06, 0.07, 0.09, 0.95)
    border.width: 1
    border.color: settingsPanel.colors ? Qt.rgba(settingsPanel.colors.surfaceText.r, settingsPanel.colors.surfaceText.g, settingsPanel.colors.surfaceText.b, 0.18) : Qt.rgba(1, 1, 1, 0.12)
    z: 10
  }

  Row {
    id: tabRow
    anchors.horizontalCenter: parent.horizontalCenter
    anchors.horizontalCenterOffset: (keybindsColumn.width + settingsPanel._s(24)) / 2
    anchors.top: parent.top
    anchors.topMargin: 12
    spacing: Style.spacingSmall
    z: 11

    add: Transition {
      NumberAnimation { property: "opacity"; from: 0; to: 1; duration: Style.animNormal; easing.type: Easing.OutCubic }
      NumberAnimation { property: "scale"; from: 0.8; to: 1; duration: Style.animNormal; easing.type: Easing.OutCubic }
    }
    move: Transition {
      NumberAnimation { properties: "x"; duration: Style.animNormal; easing.type: Easing.OutCubic }
    }

    Repeater {
      model: {
        if (!settingsPanel.showAdvanced)
          return [
            { key: "selector", label: I18n.tr("SELECTOR") },
            { key: "paper",    label: I18n.tr("PAPER") },
            { key: "edit",     label: I18n.tr("EDIT") },
            { key: "theme",    label: I18n.tr("THEME") }
          ]
        var tabs = [
          { key: "general",     label: I18n.tr("GENERAL") },
          { key: "playlists",   label: I18n.tr("PLAYLISTS") },
          { key: "paths",       label: I18n.tr("PATHS") },
          { key: "comfort",     label: I18n.tr("COMFORT") },
          { key: "lighting",    label: I18n.tr("LIGHTING") },
          { key: "performance", label: I18n.tr("PERFORMANCE") },
          { key: "postprocessing", label: I18n.tr("EXTERNAL") }
        ]
        if (Config.matugenEnabled) tabs.push({ key: "matugen", label: I18n.tr("MATUGEN") })
        if (Config.canOverviewBackdrop) tabs.push({ key: "overview-backdrop", label: I18n.tr("OVERVIEW BACKDROP") })
        if (Config.steamEnabled) tabs.push({ key: "wallpaper-engine", label: I18n.tr("WALLPAPER ENGINE") })
        return tabs
      }

      FilterButton {
        colors: settingsPanel.colors
        label: I18n.tr(modelData.label)
        skew: settingsPanel._tabSkew
        height: 28
        isActive: settingsPanel.activeTab === modelData.key
        onClicked: settingsPanel.activeTab = modelData.key
      }
    }

    // divider before the mode toggle so it reads as a control, not a tab
    Rectangle {
      width: 1; height: 18
      anchors.verticalCenter: parent.verticalCenter
      color: settingsPanel.colors ? Qt.rgba(settingsPanel.colors.surfaceText.r, settingsPanel.colors.surfaceText.g, settingsPanel.colors.surfaceText.b, 0.22) : Qt.rgba(1, 1, 1, 0.18)
    }

    FilterButton {
      colors: settingsPanel.colors
      label: I18n.tr("ADVANCED")
      register: false
      skew: settingsPanel._tabSkew
      height: 28
      isActive: settingsPanel.showAdvanced
      tooltip: settingsPanel.showAdvanced
        ? I18n.tr("Showing backend settings. Click for the visual tabs.")
        : I18n.tr("Show advanced / backend settings.")
      onClicked: settingsPanel.showAdvanced = !settingsPanel.showAdvanced
    }
  }

  // The active tab's content. A tab taller than the screen scrolls inside this
  // Flickable rather than growing the panel past the bottom edge; the explicit
  // WheelHandler is what the rest of the shell uses (Qt 6.11 Flickables do not
  // take the wheel on their own), and it also keeps the wheel over the panel
  // from falling through to the carousel behind it.
  Flickable {
    id: contentLoader
    anchors.top: tabRow.bottom
    anchors.left: keybindsColumn.right
    anchors.right: parent.right
    anchors.margins: 12
    anchors.topMargin: 8
    property real _contentHeight: {
      if (settingsPanel.activeTab === "selector") return selectorContent.implicitHeight
      if (settingsPanel.activeTab === "edit") return editContent.implicitHeight
      if (settingsPanel.activeTab === "paper") return paperContent.implicitHeight
      if (settingsPanel.activeTab === "general") return generalContent.implicitHeight
      if (settingsPanel.activeTab === "playlists") return playlistsContent.implicitHeight
      if (settingsPanel.activeTab === "paths") return pathsContent.implicitHeight
      if (settingsPanel.activeTab === "comfort") return comfortContent.implicitHeight
      if (settingsPanel.activeTab === "lighting") return lightingContent.implicitHeight
      if (settingsPanel.activeTab === "wallpaper-engine") return wallpaperEngineContent.implicitHeight
      if (settingsPanel.activeTab === "performance") return performanceContent.implicitHeight
      if (settingsPanel.activeTab === "postprocessing") return postprocessingContent.implicitHeight
      if (settingsPanel.activeTab === "theme") return themeContent.implicitHeight
      if (settingsPanel.activeTab === "matugen") return matugenContent.implicitHeight
      if (settingsPanel.activeTab === "overview-backdrop") return overviewBackdropContent.implicitHeight
      return 0
    }
    height: Math.min(_contentHeight, settingsPanel._contentMaxHeight)
    contentWidth: width
    contentHeight: _contentHeight
    clip: true
    interactive: contentHeight > height
    boundsBehavior: Flickable.StopAtBounds
    flickableDirection: Flickable.VerticalFlick
    ScrollBar.vertical: ScrollBar { policy: ScrollBar.AsNeeded }
    Behavior on height { NumberAnimation { duration: Style.animFast; easing.type: Easing.OutCubic } }

    WheelHandler {
      acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
      onWheel: function (ev) {
        var step = (ev.angleDelta.y !== 0 ? ev.angleDelta.y : ev.angleDelta.x)
        var limit = Math.max(0, contentLoader.contentHeight - contentLoader.height)
        contentLoader.contentY = Math.max(0, Math.min(limit, contentLoader.contentY - step))
      }
    }

    property real _slide: 0
    transform: Translate { y: contentLoader._slide }

    ParallelAnimation {
      id: _tabFade
      NumberAnimation { target: contentLoader; property: "opacity"; from: 0; to: 1; duration: Style.animEnter; easing.type: Easing.OutCubic }
      NumberAnimation { target: contentLoader; property: "_slide"; from: 10; to: 0; duration: Style.animEnter; easing.type: Easing.OutCubic }
    }

    Connections {
      target: settingsPanel
      function onActiveTabChanged() { _tabFade.restart() }
    }

    Loader {
      id: selectorContent
      anchors.left: parent.left
      anchors.right: parent.right
      active: settingsPanel.activeTab === "selector"
      visible: active
      source: "settings/SelectorSettings.qml"
      onLoaded: {
        item.colors = Qt.binding(function() { return settingsPanel.colors })
        item.saveField = function(k, v) { settingsPanel._saveField(k, v) }
        item.saveConfigKey = function(k, v) { settingsPanel._saveConfigKey(k, v) }
        item.showWarning = function(t, m) { settingsPanel._showWarning(t, m) }
        item.applyPreset = function(ew, sh, sw, vc, gap, sk) { settingsPanel._applyPreset(ew, sh, sw, vc, gap, sk) }
        item.saveCustomPreset = function(slot) { settingsPanel._saveCustomPreset(slot) }
        item.loadCustomPreset = function(slot) { settingsPanel._loadCustomPreset(slot) }
      }
    }

    Loader {
      id: editContent
      anchors.left: parent.left
      anchors.right: parent.right
      active: settingsPanel.activeTab === "edit"
      visible: active
      source: "settings/EditSettings.qml"
      onLoaded: {
        item.colors = Qt.binding(function() { return settingsPanel.colors })
        item.sourcePath = Qt.binding(function() { return settingsPanel.sourcePath })
      }
    }

    Loader {
      id: paperContent
      anchors.left: parent.left
      anchors.right: parent.right
      active: settingsPanel.activeTab === "paper"
      visible: active
      source: "settings/PaperSettings.qml"
      onLoaded: {
        item.colors = Qt.binding(function() { return settingsPanel.colors })
        item.saveConfigKey = function(k, v) { settingsPanel._saveConfigKey(k, v) }
      }
    }

    Loader {
      id: generalContent
      anchors.left: parent.left
      anchors.right: parent.right
      active: settingsPanel.activeTab === "general"
      visible: active
      source: "settings/GeneralSettings.qml"
      onLoaded: {
        item.colors = Qt.binding(function() { return settingsPanel.colors })
        item.saveConfigKey = function(k, v) { settingsPanel._saveConfigKey(k, v) }
      }
    }

    Loader {
      id: playlistsContent
      anchors.left: parent.left
      anchors.right: parent.right
      active: settingsPanel.activeTab === "playlists"
      visible: active
      source: "settings/PlaylistSettings.qml"
      onLoaded: {
        item.colors = Qt.binding(function() { return settingsPanel.colors })
        item.saveConfigKey = function(k, v) { settingsPanel._saveConfigKey(k, v) }
      }
    }

    Loader {
      id: pathsContent
      anchors.left: parent.left
      anchors.right: parent.right
      active: settingsPanel.activeTab === "paths"
      visible: active
      source: "settings/PathsSettings.qml"
      onLoaded: {
        item.colors = Qt.binding(function() { return settingsPanel.colors })
        item.saveConfigKey = function(k, v) { settingsPanel._saveConfigKey(k, v) }
      }
    }

    Loader {
      id: comfortContent
      anchors.left: parent.left
      anchors.right: parent.right
      active: settingsPanel.activeTab === "comfort"
      visible: active
      source: "settings/ComfortSettings.qml"
      onLoaded: item.colors = Qt.binding(function() { return settingsPanel.colors })
    }

    Loader {
      id: lightingContent
      anchors.left: parent.left
      anchors.right: parent.right
      active: settingsPanel.activeTab === "lighting"
      visible: active
      source: "settings/LightingSettings.qml"
      onLoaded: item.colors = Qt.binding(function() { return settingsPanel.colors })
    }

    Loader {
      id: overviewBackdropContent
      anchors.left: parent.left
      anchors.right: parent.right
      active: settingsPanel.activeTab === "overview-backdrop"
      visible: active
      source: "settings/OverviewBackdropSettings.qml"
      onLoaded: {
        item.colors = Qt.binding(function() { return settingsPanel.colors })
        item.saveConfigKey = function(k, v) { settingsPanel._saveConfigKey(k, v) }
      }
    }

    Loader {
      id: wallpaperEngineContent
      anchors.left: parent.left
      anchors.right: parent.right
      active: settingsPanel.activeTab === "wallpaper-engine"
      visible: active
      source: "settings/WallpaperEngineSettings.qml"
      onLoaded: {
        item.colors = Qt.binding(function() { return settingsPanel.colors })
        item.saveConfigKey = function(k, v) { settingsPanel._saveConfigKey(k, v) }
      }
    }

    Loader {
      id: performanceContent
      anchors.left: parent.left
      anchors.right: parent.right
      active: settingsPanel.activeTab === "performance"
      visible: active
      source: "settings/PerformanceSettings.qml"
      onLoaded: {
        item.colors = Qt.binding(function() { return settingsPanel.colors })
        item.saveConfigKey = function(k, v) { settingsPanel._saveConfigKey(k, v) }
        item.service = Qt.binding(function() { return settingsPanel.service })
        item.openOptimizeConfirm = function() { _optimizeConfirmPopup.open() }
      }
    }

    Loader {
      id: postprocessingContent
      anchors.left: parent.left
      anchors.right: parent.right
      active: settingsPanel.activeTab === "postprocessing"
      visible: active
      source: "settings/PostprocessingSettings.qml"
      onLoaded: {
        item.colors = Qt.binding(function() { return settingsPanel.colors })
        item.saveConfigKey = function(k, v) { settingsPanel._saveConfigKey(k, v) }
      }
    }

    Loader {
      id: themeContent
      anchors.left: parent.left
      anchors.right: parent.right
      active: settingsPanel.activeTab === "theme"
      visible: active
      source: "settings/ThemeSettings.qml"
      onLoaded: {
        item.colors = Qt.binding(function() { return settingsPanel.colors })
        item.saveConfigKey = function(k, v) { settingsPanel._saveConfigKey(k, v) }
        item.notifyThemeChanged = function(s, m, ci) { settingsPanel.themeChanged(s, m, ci) }
      }
    }

    Loader {
      id: matugenContent
      anchors.left: parent.left
      anchors.right: parent.right
      active: settingsPanel.activeTab === "matugen"
      visible: active
      source: "settings/MatugenSettings.qml"
      onLoaded: {
        item.colors = Qt.binding(function() { return settingsPanel.colors })
        item.saveConfigKey = function(k, v) { settingsPanel._saveConfigKey(k, v) }
        item.cloneIntegrations = function() { return settingsPanel._cloneIntegrations() }
        item.notify = function(message, success) {
          if (!success) settingsPanel._showWarning(I18n.tr("Palette Bridge"), message)
        }
      }
    }
  }

  Rectangle {
    id: _optimizeConfirmPopup
    visible: false
    anchors.fill: parent
    z: 201
    color: settingsPanel.colors ? Qt.rgba(settingsPanel.colors.surface.r, settingsPanel.colors.surface.g, settingsPanel.colors.surface.b, 0.97) : Qt.rgba(0.08, 0.08, 0.12, 0.97)
    radius: 8

    function open() { visible = true }
    function close() { visible = false }

    MouseArea { anchors.fill: parent; onClicked: function(mouse) { mouse.accepted = true } }

    Column {
      anchors.centerIn: parent
      spacing: 12
      width: parent.width * 0.7

      Text {
        anchors.horizontalCenter: parent.horizontalCenter
        text: "\u{f03e}"
        font.family: Style.fontFamilyNerdIcons; font.pixelSize: settingsPanel._s(28)
        color: settingsPanel.colors ? settingsPanel.colors.primary : Style.fallbackAccent
      }

      Text {
        anchors.horizontalCenter: parent.horizontalCenter
        text: I18n.tr("OPTIMIZE ALL IMAGES?")
        font.family: Style.fontFamily; font.pixelSize: settingsPanel._s(14); font.weight: Font.Bold; font.letterSpacing: 1.5
        color: settingsPanel.colors ? settingsPanel.colors.surfaceText : "#fff"
      }

      Text {
        width: parent.width
        horizontalAlignment: Text.AlignHCenter
        text: {
          var p = ImageOptimizeService.presets[Config.imageOptimizePreset]
          var r = ImageOptimizeService.resolutions[Config.imageOptimizeResolution]
          var fmts = p ? p.formats.join(", ").toUpperCase() : "?"
          return I18n.tr("This will convert %1 images to WebP using the %2 preset (quality %3, max %4). Originals are moved to trash. Already optimized files will be skipped.")
            .arg(fmts)
            .arg(Config.imageOptimizePreset.toUpperCase())
            .arg(p ? p.quality : "?")
            .arg(r ? r.maxW + "x" + r.maxH : "?")
        }
        font.family: Style.fontFamily; font.pixelSize: settingsPanel._s(11); font.letterSpacing: 0.2
        color: settingsPanel.colors ? Qt.rgba(settingsPanel.colors.surfaceText.r, settingsPanel.colors.surfaceText.g, settingsPanel.colors.surfaceText.b, 0.6) : Qt.rgba(1, 1, 1, 0.5)
        wrapMode: Text.WordWrap
        lineHeight: 1.3
      }

      Text {
        width: parent.width
        horizontalAlignment: Text.AlignHCenter
        text: I18n.tr("Only images in your wallpaper directory are processed")
        font.family: Style.fontFamily; font.pixelSize: settingsPanel._s(10); font.letterSpacing: 0.2
        color: settingsPanel.colors ? Qt.rgba(settingsPanel.colors.surfaceText.r, settingsPanel.colors.surfaceText.g, settingsPanel.colors.surfaceText.b, 0.4) : Qt.rgba(1, 1, 1, 0.35)
        wrapMode: Text.WordWrap
        lineHeight: 1.3
      }

      Item { width: 1; height: 4 }

      Row {
        anchors.horizontalCenter: parent.horizontalCenter
        spacing: 8

        FilterButton {
          colors: settingsPanel.colors
          label: I18n.tr("CANCEL")
          skew: 8 * Config.uiScale; height: 26 * Config.uiScale
          onClicked: _optimizeConfirmPopup.close()
        }

        FilterButton {
          colors: settingsPanel.colors
          label: I18n.tr("OPTIMIZE")
          skew: 8 * Config.uiScale; height: 26 * Config.uiScale
          isActive: true
          onClicked: {
            _optimizeConfirmPopup.close()
            ImageOptimizeService.optimize(Config.imageOptimizePreset, Config.imageOptimizeResolution)
          }
        }
      }
    }
  }

  Rectangle {
    id: _convertConfirmPopup
    visible: false
    anchors.fill: parent
    z: 200
    color: settingsPanel.colors ? Qt.rgba(settingsPanel.colors.surface.r, settingsPanel.colors.surface.g, settingsPanel.colors.surface.b, 0.97) : Qt.rgba(0.08, 0.08, 0.12, 0.97)
    radius: 8

    function open() { visible = true }
    function close() { visible = false }

    MouseArea { anchors.fill: parent; onClicked: function(mouse) { mouse.accepted = true } }

    Column {
      anchors.centerIn: parent
      spacing: 12
      width: parent.width * 0.7

      Text {
        anchors.horizontalCenter: parent.horizontalCenter
        text: "\u{f03d}"
        font.family: Style.fontFamilyNerdIcons; font.pixelSize: settingsPanel._s(28)
        color: settingsPanel.colors ? settingsPanel.colors.primary : Style.fallbackAccent
      }

      Text {
        anchors.horizontalCenter: parent.horizontalCenter
        text: I18n.tr("OPTIMIZE ALL VIDEOS?")
        font.family: Style.fontFamily; font.pixelSize: settingsPanel._s(14); font.weight: Font.Bold; font.letterSpacing: 1.5
        color: settingsPanel.colors ? settingsPanel.colors.surfaceText : "#fff"
      }

      Text {
        width: parent.width
        horizontalAlignment: Text.AlignHCenter
        text: {
          var p = VideoConvertService.presets[Config.videoConvertPreset]
          var r = VideoConvertService.resolutions[Config.videoConvertResolution]
          return I18n.tr("This will convert all video wallpapers to HEVC (H.265) using the %1 preset (CRF %2, max %3, %4). Originals are moved to trash. Already converted files will be skipped.")
            .arg(Config.videoConvertPreset.toUpperCase())
            .arg(p ? p.crf : "?")
            .arg(p ? p.maxrate : "?")
            .arg(r ? r.maxW + "x" + r.maxH : "?")
        }
        font.family: Style.fontFamily; font.pixelSize: settingsPanel._s(11); font.letterSpacing: 0.2
        color: settingsPanel.colors ? Qt.rgba(settingsPanel.colors.surfaceText.r, settingsPanel.colors.surfaceText.g, settingsPanel.colors.surfaceText.b, 0.6) : Qt.rgba(1, 1, 1, 0.5)
        wrapMode: Text.WordWrap
        lineHeight: 1.3
      }

      Text {
        width: parent.width
        horizontalAlignment: Text.AlignHCenter
        text: I18n.tr("This may take a while depending on the number and size of videos.")
        font.family: Style.fontFamily; font.pixelSize: settingsPanel._s(10); font.letterSpacing: 0.2
        color: settingsPanel.colors ? Qt.rgba(settingsPanel.colors.surfaceText.r, settingsPanel.colors.surfaceText.g, settingsPanel.colors.surfaceText.b, 0.4) : Qt.rgba(1, 1, 1, 0.35)
        wrapMode: Text.WordWrap
        lineHeight: 1.3
      }

      Item { width: 1; height: 4 }

      Row {
        anchors.horizontalCenter: parent.horizontalCenter
        spacing: 8

        FilterButton {
          colors: settingsPanel.colors
          label: I18n.tr("CANCEL")
          skew: 8 * Config.uiScale; height: 26 * Config.uiScale
          onClicked: _convertConfirmPopup.close()
        }

        FilterButton {
          colors: settingsPanel.colors
          label: I18n.tr("CONVERT")
          skew: 8 * Config.uiScale; height: 26 * Config.uiScale
          isActive: false
          enabled: false
          opacity: 0.35
        }
      }
    }
  }

  Rectangle {
    id: _warningPopup
    visible: false
    anchors.fill: parent
    z: 200
    color: settingsPanel.colors ? Qt.rgba(settingsPanel.colors.surface.r, settingsPanel.colors.surface.g, settingsPanel.colors.surface.b, 0.97) : Qt.rgba(0.08, 0.08, 0.12, 0.97)
    radius: 8

    property string title: I18n.tr("RESTART REQUIRED")
    property string message: I18n.tr("Directory changes will take effect after restarting the app. Don't forget that includes the daemon!")

    function open() { visible = true }
    function close() { visible = false }

    MouseArea { anchors.fill: parent; onClicked: function(mouse) { mouse.accepted = true } }

    Column {
      anchors.centerIn: parent
      spacing: 12
      width: parent.width * 0.7

      Text {
        anchors.horizontalCenter: parent.horizontalCenter
        text: "\u{f0028}"
        font.family: Style.fontFamilyNerdIcons; font.pixelSize: settingsPanel._s(28)
        color: "#ffb74d"
      }

      Text {
        anchors.horizontalCenter: parent.horizontalCenter
        text: _warningPopup.title
        font.family: Style.fontFamily; font.pixelSize: settingsPanel._s(14); font.weight: Font.Bold; font.letterSpacing: 1.5
        color: settingsPanel.colors ? settingsPanel.colors.surfaceText : "#fff"
      }

      Text {
        width: parent.width
        horizontalAlignment: Text.AlignHCenter
        text: _warningPopup.message
        font.family: Style.fontFamily; font.pixelSize: settingsPanel._s(11); font.letterSpacing: 0.2
        color: settingsPanel.colors ? Qt.rgba(settingsPanel.colors.surfaceText.r, settingsPanel.colors.surfaceText.g, settingsPanel.colors.surfaceText.b, 0.6) : Qt.rgba(1, 1, 1, 0.5)
        wrapMode: Text.WordWrap
        lineHeight: 1.3
      }

      Item { width: 1; height: 2 }

      FilterButton {
        anchors.horizontalCenter: parent.horizontalCenter
        colors: settingsPanel.colors
        label: I18n.tr("OK")
        skew: 8 * Config.uiScale; height: 26 * Config.uiScale
        isActive: true
        onClicked: _warningPopup.close()
      }
    }
  }
}
