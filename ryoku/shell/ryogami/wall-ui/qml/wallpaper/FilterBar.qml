import QtQuick
import QtQuick.Controls
import QtQuick.Shapes
import QtQuick.Effects
import ".."
import "../services"
import Ryoku.Ui.Singletons

Item {
    id: filterBar

    property var colors
    property var service
    property bool settingsOpen: false
    property bool browseOpen: false
    property bool themesOpen: false
    property bool ricesOpen: false
    property bool followActive: false
    property bool cacheLoading: false
    property int cacheProgress: 0
    property int cacheTotal: 0
    property bool videoConvertRunning: false
    property int videoConvertProgress: 0
    property int videoConvertTotal: 0
    property string videoConvertFile: ""
    property bool imageOptimizeRunning: false
    property int imageOptimizeProgress: 0
    property int imageOptimizeTotal: 0
    property string imageOptimizeFile: ""

    signal settingsToggled()
    signal browseToggled()
    signal themesToggled()
    signal ricesToggled()
    signal saveLookRequested()
    signal modeToggled(string mode)
    signal followToggled()

    readonly property int _skew: 10 * Config.uiScale
    property real maxWidth: 99999

    width: Math.min(filterRow.width, maxWidth)
    height: filterRow.height + (filterFlick.contentWidth > filterFlick.width ? 10 : 0)

    // A unifying paper pill behind the whole bar so it reads over any wallpaper
    // (the buttons are transparent until active). Flat print + hairline + a soft
    // shadow: Ryoku presence without a heavy chrome slab.
    Rectangle {
        anchors.fill: filterFlick
        anchors.topMargin: -7; anchors.bottomMargin: -7
        anchors.leftMargin: -12; anchors.rightMargin: -12
        radius: Style.radiusRound
        color: filterBar.colors ? Qt.rgba(filterBar.colors.surface.r, filterBar.colors.surface.g, filterBar.colors.surface.b, 0.9) : Qt.rgba(0.06, 0.07, 0.09, 0.9)
        border.width: 1
        border.color: filterBar.colors ? Qt.rgba(filterBar.colors.surfaceText.r, filterBar.colors.surfaceText.g, filterBar.colors.surfaceText.b, 0.22) : Qt.rgba(1, 1, 1, 0.15)
        z: -1
        layer.enabled: true
        layer.effect: MultiEffect { shadowEnabled: true; shadowBlur: 0.6; shadowVerticalOffset: 3; shadowColor: Qt.rgba(0, 0, 0, 0.5) }
    }

    Flickable {
        id: filterFlick
        width: filterBar.width
        height: filterRow.height
        contentWidth: filterRow.width
        contentHeight: filterRow.height
        clip: contentWidth > width
        flickableDirection: Flickable.HorizontalFlick
        boundsBehavior: Flickable.StopAtBounds

        // A Flickable ignores the wheel by default, so on a narrow card (or a
        // localized label set that widens the row past it) the chips after the
        // colour picker were clipped and unreachable: the natural scroll gesture
        // did nothing. Drive contentX from the wheel -- vertical notch and a
        // horizontal trackpad swipe alike -- clamped to the overflow. The audio
        // button keeps its own WheelHandler (volume), which wins under it since
        // handlers deliver to the deepest item first.
        WheelHandler {
            acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
            onWheel: function (ev) {
                var dx = ev.angleDelta.x
                if (dx === 0)
                    dx = ev.angleDelta.y
                if (dx === 0)
                    return
                var maxX = Math.max(0, filterFlick.contentWidth - filterFlick.width)
                filterFlick.contentX = Math.max(0, Math.min(maxX, filterFlick.contentX - dx))
                ev.accepted = true
            }
        }

        ScrollBar.horizontal: ScrollBar {
            id: filterScrollBar
            y: filterFlick.height + 4
            height: 4
            visible: filterFlick.contentWidth > filterFlick.width
            policy: ScrollBar.AlwaysOn
            contentItem: Rectangle {
                implicitHeight: 4
                radius: 2
                color: filterBar.colors ? filterBar.colors.primary : Qt.rgba(1, 1, 1, 1)
            }
            background: Rectangle {
                implicitHeight: 4
                radius: 2
                color: filterBar.colors ? filterBar.colors.surfaceContainer : Qt.rgba(0, 0, 0, 0.5)
            }
        }

    Row {
        id: filterRow
        spacing: 4 * Config.uiScale

        Repeater {
            model: [
                { type: "", label: I18n.tr("ALL") },
                { type: "static", label: I18n.tr("PIC") },
                { type: "video", label: I18n.tr("VID") },
                { type: "we", label: I18n.tr("WE") }
            ]

            FilterButton {
                colors: filterBar.colors
                label: I18n.tr(modelData.label)
                register: false
                isActive: filterBar.service ? filterBar.service.selectedTypeFilter === modelData.type : false
                onClicked: {
                    if (isActive) filterBar.service.selectedTypeFilter = ""
                    else filterBar.service.selectedTypeFilter = modelData.type
                }
            }
        }

        FolderFilter {
            anchors.verticalCenter: parent.verticalCenter
            colors: filterBar.colors
            service: filterBar.service
        }

        Repeater {
            model: [
                { mode: "date",       icon: "\u{f00f0}", label: I18n.tr("Newest") },
                { mode: "color",      icon: "\u{f03d8}", label: I18n.tr("Default (by color)") },
                { mode: "pop",        icon: "\u{f0238}", label: I18n.tr("Color pop") },
                { mode: "richness",   icon: "\u{f0b74}", label: I18n.tr("Colourful") },
                { mode: "minimalist", icon: "\u{f0764}", label: I18n.tr("Minimalist") },
                { mode: "applied",    icon: "\u{f04c5}", label: I18n.tr("Most applied") }
            ]

            FilterButton {
                colors: filterBar.colors
                icon: modelData.icon
                tooltip: I18n.tr(modelData.label)
                isActive: filterBar.service ? filterBar.service.sortMode === modelData.mode : false
                onClicked: {
                    filterBar.service.sortMode = modelData.mode
                    filterBar.service.updateFilteredModel()
                }
            }
        }

        FilterButton {
            colors: filterBar.colors
            icon: "\u{f02d1}"
            tooltip: I18n.tr("Favourites")
            isActive: filterBar.service ? filterBar.service.favouriteFilterActive : false
            onClicked: filterBar.service.favouriteFilterActive = !filterBar.service.favouriteFilterActive
        }

        FilterButton {
            id: _randomBtn
            colors: filterBar.colors
            icon: "\u{f049d}"
            tooltip: DaemonClient.rotationActive
                ? I18n.tr("Stop auto-rotate (random + playlists)")
                : I18n.tr("Auto-rotate: continuous random wallpapers. Configure interval in settings.")
            isActive: DaemonClient.rotationActive
            onClicked: {
                if (DaemonClient.rotationActive) { DaemonClient.rotationStop(); return }
                var types = []
                if (Config.randomIncludeStatic) types.push("static")
                if (Config.randomIncludeVideo) types.push("video")
                if (Config.randomIncludeWE) types.push("we")
                if (types.length === 0) {
                    _randomWarnPopup.open()
                    return
                }
                DaemonClient.randomStart(Config.randomInterval, {
                    types: types,
                    favouritesOnly: Config.randomIncludeFavourites
                })
            }

            Popup {
                id: _randomWarnPopup
                parent: _randomBtn
                x: (_randomBtn.width - width) / 2
                y: _randomBtn.height + 6
                padding: 10
                modal: false
                focus: false
                closePolicy: Popup.CloseOnPressOutside | Popup.CloseOnEscape
                background: Rectangle {
                    color: filterBar.colors ? Qt.rgba(filterBar.colors.surface.r, filterBar.colors.surface.g, filterBar.colors.surface.b, 0.97) : Qt.rgba(0.08, 0.08, 0.12, 0.97)
                    radius: 8
                    border.width: 1
                    border.color: "#ffb74d"
                }
                contentItem: Row {
                    spacing: 8
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: "\u{f0028}"
                        font.family: Style.fontFamilyNerdIcons
                        font.pixelSize: 16 * Config.uiScale
                        color: "#ffb74d"
                    }
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("Select at least one random source category in settings")
                        font.family: Style.fontFamily
                        font.pixelSize: 11 * Config.uiScale
                        font.letterSpacing: 0.2
                        color: filterBar.colors ? filterBar.colors.surfaceText : "#fff"
                    }
                }
                Timer {
                    interval: 4000
                    running: _randomWarnPopup.visible
                    onTriggered: _randomWarnPopup.close()
                }
            }
        }

        FilterButton {
            id: _audioBtn
            visible: DaemonClient.audioCapableCount > 0 || _audioPopup.visible
            colors: filterBar.colors
            icon: !Config.wallpaperMute ? "\u{f057e}" : "\u{f075f}"
            tooltip: !Config.wallpaperMute
                ? I18n.tr("Audio on (%1%) - click to mute, right-click for slider").arg(Config.wallpaperVolume)
                : I18n.tr("Audio muted - click to unmute, right-click for slider")
            isActive: !Config.wallpaperMute
            onClicked: {
                var nextMute = !Config.wallpaperMute
                Config.saveKey("wallpaperMute", nextMute)
                DaemonClient.setAudio(nextMute, null, null, function() {
                    DaemonClient.refreshAudioState()
                })
            }

            WheelHandler {
                acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
                onWheel: function(ev) {
                    var step = ev.angleDelta.y > 0 ? 5 : -5
                    var v = Math.max(0, Math.min(100, Config.wallpaperVolume + step))
                    if (v === Config.wallpaperVolume) return
                    Config.saveKey("wallpaperVolume", v)
                    DaemonClient.setAudio(null, v, null)
                }
            }

            TapHandler {
                acceptedButtons: Qt.RightButton
                gesturePolicy: TapHandler.WithinBounds
                onTapped: _audioPopup.open()
            }

            Popup {
                id: _audioPopup
                parent: _audioBtn
                x: -((width - _audioBtn.width) / 2)
                y: _audioBtn.height + 6
                width: 320
                padding: 12
                modal: false
                focus: false
                closePolicy: Popup.CloseOnPressOutside | Popup.CloseOnEscape

                onAboutToShow: DaemonClient.refreshAudioState()

                property var _volumes: ({})

                function _volFor(group) {
                    var v = _volumes[group.key]
                    if (typeof v === "number") return v
                    if (typeof group.volume === "number") return group.volume
                    return Config.wallpaperVolume
                }
                function _setVolFor(group, v) {
                    v = Math.round(Math.max(0, Math.min(100, v)))
                    var m = _volumes
                    m[group.key] = v
                    _volumes = m
                    DaemonClient.setAudio(null, v, group.outputs)
                }
                function _toggleMuteFor(group) {
                    var nextMute = !group.muted
                    if (nextMute) {
                        DaemonClient.setAudio(true, null, group.outputs, function() {
                            DaemonClient.refreshAudioState()
                        })
                    } else {
                        var primary = group.primary || (group.outputs[0] || "")
                        var others = group.outputs.filter(function(o) { return o !== primary })
                        if (others.length > 0) {
                            DaemonClient.setAudio(true, null, others)
                        }
                        if (primary) {
                            DaemonClient.setAudio(false, null, [primary], function() {
                                DaemonClient.refreshAudioState()
                            })
                        }
                    }
                }

                background: Rectangle {
                    color: filterBar.colors
                        ? Qt.rgba(filterBar.colors.surface.r, filterBar.colors.surface.g, filterBar.colors.surface.b, 0.97)
                        : Qt.rgba(0.08, 0.08, 0.12, 0.97)
                    radius: 8
                    border.width: 1
                    border.color: filterBar.colors
                        ? Qt.rgba(filterBar.colors.primary.r, filterBar.colors.primary.g, filterBar.colors.primary.b, 0.4)
                        : Qt.rgba(1, 1, 1, 0.2)
                }

                contentItem: Column {
                    spacing: 10

                    Row {
                        spacing: 8
                        width: parent.width

                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: "\u{f057e}"
                            font.family: Style.fontFamilyNerdIcons
                            font.pixelSize: 16 * Config.uiScale
                            color: filterBar.colors ? filterBar.colors.primary : Style.fallbackAccent
                        }
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr("Wallpaper audio")
                            font.family: Style.fontFamily
                            font.pixelSize: 11 * Config.uiScale
                            font.weight: Font.Bold
                            font.letterSpacing: 0.5
                            color: filterBar.colors ? filterBar.colors.surfaceText : "#fff"
                        }
                    }

                    Text {
                        visible: (DaemonClient.audioGroups || []).length === 0
                        width: parent.width
                        text: I18n.tr("No audio sources are active right now.")
                        wrapMode: Text.WordWrap
                        font.family: Style.fontFamily
                        font.pixelSize: 10 * Config.uiScale
                        font.italic: true
                        color: filterBar.colors
                            ? Qt.rgba(filterBar.colors.surfaceText.r, filterBar.colors.surfaceText.g, filterBar.colors.surfaceText.b, 0.55)
                            : Qt.rgba(1, 1, 1, 0.45)
                    }

                    Repeater {
                        model: DaemonClient.audioGroups || []

                        Item {
                            id: _audioRow
                            width: 296
                            height: 56

                            property var group: modelData
                            property int volValue: _audioPopup._volFor(modelData)
                            readonly property real _skew: 6

                            Connections {
                                target: DaemonClient
                                function onAudioGroupsChanged() {
                                    _audioRow.volValue = _audioPopup._volFor(_audioRow.group)
                                }
                            }

                            Shape {
                                anchors.fill: parent
                                layer.enabled: true
                                layer.smooth: true
                                layer.samples: 4
                                preferredRendererType: Shape.CurveRenderer

                                ShapePath {
                                    fillColor: filterBar.colors
                                        ? Qt.rgba(filterBar.colors.surfaceContainer.r, filterBar.colors.surfaceContainer.g, filterBar.colors.surfaceContainer.b, 0.8)
                                        : Qt.rgba(0.1, 0.12, 0.18, 0.8)
                                    strokeColor: filterBar.colors
                                        ? Qt.rgba(filterBar.colors.primary.r, filterBar.colors.primary.g, filterBar.colors.primary.b, 0.18)
                                        : Qt.rgba(1, 1, 1, 0.08)
                                    strokeWidth: 1
                                    startX: _audioRow._skew; startY: 0
                                    PathLine { x: _audioRow.width;                       y: 0 }
                                    PathLine { x: _audioRow.width - _audioRow._skew;     y: _audioRow.height }
                                    PathLine { x: 0;                                     y: _audioRow.height }
                                    PathLine { x: _audioRow._skew;                       y: 0 }
                                }
                            }

                            Row {
                                anchors.fill: parent
                                anchors.leftMargin: 12
                                anchors.rightMargin: 12
                                anchors.topMargin: 6
                                anchors.bottomMargin: 6
                                spacing: 8

                                Item {
                                    id: _muteIconHolder
                                    width: 24
                                    height: 24
                                    anchors.verticalCenter: parent.verticalCenter

                                    Text {
                                        anchors.centerIn: parent
                                        text: _audioRow.group.muted ? "\u{f075f}" : "\u{f057e}"
                                        font.family: Style.fontFamilyNerdIcons
                                        font.pixelSize: 16 * Config.uiScale
                                        color: _audioRow.group.muted
                                            ? (filterBar.colors
                                                ? Qt.rgba(filterBar.colors.surfaceText.r, filterBar.colors.surfaceText.g, filterBar.colors.surfaceText.b, 0.5)
                                                : Qt.rgba(1, 1, 1, 0.4))
                                            : (filterBar.colors ? filterBar.colors.primary : Style.fallbackAccent)
                                    }

                                    MouseArea {
                                        anchors.fill: parent
                                        cursorShape: Qt.PointingHandCursor
                                        onClicked: _audioPopup._toggleMuteFor(_audioRow.group)
                                    }
                                }

                                Column {
                                    anchors.verticalCenter: parent.verticalCenter
                                    spacing: 4
                                    width: parent.width - _muteIconHolder.width - _volValTxt.width - parent.spacing * 2

                                    Row {
                                        width: parent.width
                                        spacing: 6

                                        Text {
                                            text: _audioRow.group.name
                                            elide: Text.ElideMiddle
                                            width: parent.width - _outsTxt.width - parent.spacing
                                            font.family: Style.fontFamily
                                            font.pixelSize: 10 * Config.uiScale
                                            font.weight: Font.Bold
                                            font.letterSpacing: 0.3
                                            color: filterBar.colors ? filterBar.colors.surfaceText : "#fff"
                                        }

                                        Text {
                                            id: _outsTxt
                                            text: (_audioRow.group.outputs || []).length > 1
                                                ? ("× " + _audioRow.group.outputs.length)
                                                : ""
                                            font.family: Style.fontFamilyCode
                                            font.pixelSize: 9 * Config.uiScale
                                            color: filterBar.colors
                                                ? Qt.rgba(filterBar.colors.surfaceText.r, filterBar.colors.surfaceText.g, filterBar.colors.surfaceText.b, 0.5)
                                                : Qt.rgba(1, 1, 1, 0.4)
                                        }
                                    }

                                    Item {
                                        id: _rowVol
                                        width: parent.width
                                        height: 12

                                        readonly property real _ratio: Math.max(0, Math.min(1, _audioRow.volValue / 100))
                                        readonly property real _vskew: 3
                                        readonly property real _fillW: Math.round(_rowTrack.width * _ratio)

                                        Item {
                                            id: _rowTrack
                                            anchors.fill: parent

                                            Shape {
                                                anchors.fill: parent
                                                layer.enabled: true
                                                layer.smooth: true
                                                layer.samples: 4
                                                preferredRendererType: Shape.CurveRenderer
                                                ShapePath {
                                                    fillColor: filterBar.colors
                                                        ? Qt.rgba(filterBar.colors.surfaceVariant.r, filterBar.colors.surfaceVariant.g, filterBar.colors.surfaceVariant.b, 0.55)
                                                        : Qt.rgba(0.3, 0.3, 0.35, 0.55)
                                                    strokeColor: filterBar.colors
                                                        ? Qt.rgba(filterBar.colors.outline.r, filterBar.colors.outline.g, filterBar.colors.outline.b, 0.3)
                                                        : Qt.rgba(1, 1, 1, 0.1)
                                                    strokeWidth: 1
                                                    startX: _rowVol._vskew; startY: 0
                                                    PathLine { x: _rowTrack.width;                  y: 0 }
                                                    PathLine { x: _rowTrack.width - _rowVol._vskew; y: _rowTrack.height }
                                                    PathLine { x: 0;                                y: _rowTrack.height }
                                                    PathLine { x: _rowVol._vskew;                   y: 0 }
                                                }
                                            }

                                            Shape {
                                                anchors.left: parent.left
                                                anchors.top: parent.top
                                                anchors.bottom: parent.bottom
                                                width: Math.max(_rowVol._vskew * 2 + 1, _rowVol._fillW)
                                                visible: _rowVol._ratio > 0
                                                layer.enabled: true
                                                layer.smooth: true
                                                layer.samples: 4
                                                preferredRendererType: Shape.CurveRenderer
                                                ShapePath {
                                                    fillColor: _audioRow.group.muted
                                                        ? (filterBar.colors
                                                            ? Qt.rgba(filterBar.colors.primary.r, filterBar.colors.primary.g, filterBar.colors.primary.b, 0.35)
                                                            : Qt.rgba(0.5, 0.6, 0.8, 0.35))
                                                        : (filterBar.colors ? filterBar.colors.primary : "#7986cb")
                                                    strokeWidth: 0
                                                    startX: _rowVol._vskew; startY: 0
                                                    PathLine { x: parent.width;                       y: 0 }
                                                    PathLine { x: parent.width - _rowVol._vskew;      y: _rowTrack.height }
                                                    PathLine { x: 0;                                  y: _rowTrack.height }
                                                    PathLine { x: _rowVol._vskew;                     y: 0 }
                                                }
                                            }

                                            MouseArea {
                                                anchors.fill: parent
                                                cursorShape: Qt.PointingHandCursor
                                                preventStealing: true
                                                function _setFromX(x) {
                                                    var w = _rowTrack.width
                                                    if (w <= 0) return
                                                    var clamped = Math.max(0, Math.min(w, x))
                                                    var v = Math.round((clamped / w) * 100)
                                                    _audioRow.volValue = v
                                                    _audioPopup._setVolFor(_audioRow.group, v)
                                                }
                                                onPressed: function(mouse) { mouse.accepted = true; _setFromX(mouse.x) }
                                                onPositionChanged: function(mouse) { if (pressed) _setFromX(mouse.x) }
                                                onWheel: function(ev) {
                                                    ev.accepted = true
                                                    var step = ev.angleDelta.y > 0 ? 5 : -5
                                                    var v = Math.max(0, Math.min(100, _audioRow.volValue + step))
                                                    _audioRow.volValue = v
                                                    _audioPopup._setVolFor(_audioRow.group, v)
                                                }
                                            }
                                        }
                                    }
                                }

                                Text {
                                    id: _volValTxt
                                    anchors.verticalCenter: parent.verticalCenter
                                    width: 30
                                    horizontalAlignment: Text.AlignRight
                                    text: _audioRow.volValue + "%"
                                    font.family: Style.fontFamilyCode
                                    font.pixelSize: 9 * Config.uiScale
                                    color: filterBar.colors
                                        ? Qt.rgba(filterBar.colors.surfaceText.r, filterBar.colors.surfaceText.g, filterBar.colors.surfaceText.b, 0.6)
                                        : Qt.rgba(1, 1, 1, 0.5)
                                }
                            }
                        }
                    }
                }
            }
        }

        ColorFilterStrip {
            colors: filterBar.colors
            selectedValue: filterBar.service ? filterBar.service.selectedColorFilter : -1
            onValueSelected: function(v) {
                if (filterBar.service) filterBar.service.selectedColorFilter = v
            }
        }

        FilterButton {
            colors: filterBar.colors
            icon: "\u{f020a}"
            tooltip: I18n.tr("Follow wallpaper colours")
            isActive: filterBar.followActive
            onClicked: filterBar.followToggled()
        }

        FilterButton {
            colors: filterBar.colors
            icon: "\u{f0599}"
            tooltip: I18n.tr("Light mode")
            isActive: Config.matugenMode === "light"
            onClicked: filterBar.modeToggled("light")
        }

        FilterButton {
            colors: filterBar.colors
            icon: "\u{f0594}"
            tooltip: I18n.tr("Dark mode")
            isActive: Config.matugenMode === "dark"
            onClicked: filterBar.modeToggled("dark")
        }

        FilterButton {
            visible: Config.wallhavenEnabled || Config.steamEnabled
            colors: filterBar.colors
            icon: "\u{f01da}"
            tooltip: I18n.tr("Browse online sources")
            isActive: filterBar.browseOpen
            onClicked: filterBar.browseToggled()
        }

        FilterButton {
            colors: filterBar.colors
            icon: "\u{f0334}"
            tooltip: I18n.tr("Themes")
            onClicked: filterBar.themesToggled()
        }

        FilterButton {
            colors: filterBar.colors
            icon: "\u{f0b84}"
            tooltip: I18n.tr("Rices")
            onClicked: filterBar.ricesToggled()
        }

        FilterButton {
            visible: filterBar.ricesOpen
            colors: filterBar.colors
            icon: "\u{f0193}"
            tooltip: I18n.tr("Save current look as a rice")
            onClicked: filterBar.saveLookRequested()
        }

        FilterButton {
            colors: filterBar.colors
            icon: "\u{f0493}"
            tooltip: I18n.tr("Settings")
            isActive: filterBar.settingsOpen
            onClicked: filterBar.settingsToggled()
        }

        FilterButton {
            colors: filterBar.colors
            icon: "\u{f0450}"
            tooltip: DaemonClient.cacheRunning ? "Refreshing the library\u2026" : "Refresh: clear the cache and rescan the folders"
            isActive: DaemonClient.cacheRunning
            enabled: !DaemonClient.cacheRunning
            onClicked: DaemonClient.resetCache(function() {})
        }

        Item {
            width: _countLabel.implicitWidth + 24 + filterBar._skew
            height: 24 * Config.uiScale

            Canvas {
                anchors.fill: parent
                property color fillColor: filterBar.colors ? Qt.rgba(filterBar.colors.surfaceContainer.r, filterBar.colors.surfaceContainer.g, filterBar.colors.surfaceContainer.b, 0.85) : Qt.rgba(0.1, 0.12, 0.18, 0.85)
                property color strokeColor: filterBar.colors ? Qt.rgba(filterBar.colors.primary.r, filterBar.colors.primary.g, filterBar.colors.primary.b, 0.15) : Qt.rgba(1, 1, 1, 0.08)
                onFillColorChanged: requestPaint()
                onStrokeColorChanged: requestPaint()
                onWidthChanged: requestPaint()
                onPaint: {
                    var ctx = getContext("2d")
                    ctx.clearRect(0, 0, width, height)
                    var sk = filterBar._skew
                    ctx.fillStyle = fillColor
                    ctx.strokeStyle = strokeColor
                    ctx.lineWidth = 1
                    ctx.beginPath()
                    ctx.moveTo(sk, 0)
                    ctx.lineTo(width, 0)
                    ctx.lineTo(width - sk, height)
                    ctx.lineTo(0, height)
                    ctx.closePath()
                    ctx.fill()
                    ctx.stroke()
                }
            }

            Text {
                id: _countLabel
                anchors.centerIn: parent
                text: {
                    if (!filterBar.service) return "0"
                    var fc = filterBar.service.filteredModel.count
                    var tc = filterBar.service._wallpaperData.length
                    return fc + (fc !== tc ? "/" + tc : "")
                }
                font.family: Style.fontFamily
                font.pixelSize: 10 * Config.uiScale
                font.weight: Font.Bold
                font.letterSpacing: 0.5
                color: filterBar.colors ? Qt.rgba(filterBar.colors.surfaceText.r, filterBar.colors.surfaceText.g, filterBar.colors.surfaceText.b, 0.5) : Qt.rgba(1, 1, 1, 0.4)
            }
        }

        Item {
            visible: filterBar.cacheLoading || filterBar.videoConvertRunning || filterBar.imageOptimizeRunning
            width: visible ? (_statusRow.width + 24 + filterBar._skew) : 0
            height: 24 * Config.uiScale

            Canvas {
                anchors.fill: parent
                visible: parent.visible
                property color fillColor: filterBar.colors ? Qt.rgba(filterBar.colors.surfaceContainer.r, filterBar.colors.surfaceContainer.g, filterBar.colors.surfaceContainer.b, 0.85) : Qt.rgba(0.1, 0.12, 0.18, 0.85)
                property color strokeColor: filterBar.colors ? Qt.rgba(filterBar.colors.primary.r, filterBar.colors.primary.g, filterBar.colors.primary.b, 0.15) : Qt.rgba(1, 1, 1, 0.08)
                onFillColorChanged: requestPaint()
                onStrokeColorChanged: requestPaint()
                onWidthChanged: requestPaint()
                onPaint: {
                    var ctx = getContext("2d")
                    ctx.clearRect(0, 0, width, height)
                    var sk = filterBar._skew
                    ctx.fillStyle = fillColor
                    ctx.strokeStyle = strokeColor
                    ctx.lineWidth = 1
                    ctx.beginPath()
                    ctx.moveTo(sk, 0)
                    ctx.lineTo(width, 0)
                    ctx.lineTo(width - sk, height)
                    ctx.lineTo(0, height)
                    ctx.closePath()
                    ctx.fill()
                    ctx.stroke()
                }
            }

            Row {
                id: _statusRow
                anchors.centerIn: parent
                spacing: 4

                Text {
                    text: "󰔟"
                    font.pixelSize: 11 * Config.uiScale
                    font.family: Style.fontFamilyNerdIcons
                    color: filterBar.colors ? filterBar.colors.primary : Style.fallbackAccent
                    anchors.verticalCenter: parent.verticalCenter
                    RotationAnimation on rotation {
                        from: 0; to: 360; duration: 1200
                        loops: Animation.Infinite
                        running: filterBar.cacheLoading || filterBar.videoConvertRunning || filterBar.imageOptimizeRunning
                    }
                }

                Text {
                    text: {
                        var parts = []
                        if (filterBar.cacheLoading) {
                            if (filterBar.cacheTotal > 0)
                                parts.push(I18n.tr("CACHE %1/%2").arg(filterBar.cacheProgress).arg(filterBar.cacheTotal))
                            else
                                parts.push(I18n.tr("PROCESSING"))
                        }
                        if (filterBar.videoConvertRunning) {
                            if (filterBar.videoConvertTotal > 0)
                                parts.push(I18n.tr("CONVERT %1/%2").arg(filterBar.videoConvertProgress).arg(filterBar.videoConvertTotal))
                            else
                                parts.push(I18n.tr("CONVERT"))
                        }
                        if (filterBar.imageOptimizeRunning) {
                            if (filterBar.imageOptimizeTotal > 0)
                                parts.push(I18n.tr("OPTIMIZE %1/%2").arg(filterBar.imageOptimizeProgress).arg(filterBar.imageOptimizeTotal))
                            else
                                parts.push(I18n.tr("OPTIMIZE"))
                        }
                        return parts.join(" · ")
                    }
                    font.family: Style.fontFamily
                    font.pixelSize: 9 * Config.uiScale
                    font.weight: Font.Bold
                    font.letterSpacing: 0.5
                    color: filterBar.colors ? Qt.rgba(filterBar.colors.primary.r, filterBar.colors.primary.g, filterBar.colors.primary.b, 0.8) : Qt.rgba(0.5, 0.76, 0.97, 0.8)
                    anchors.verticalCenter: parent.verticalCenter
                }
            }
        }

        Item {
            visible: (filterBar.videoConvertRunning && filterBar.videoConvertFile !== "") || (filterBar.imageOptimizeRunning && filterBar.imageOptimizeFile !== "")
            width: visible ? (180 + 24 + filterBar._skew) : 0
            height: 24 * Config.uiScale
            Behavior on width { NumberAnimation { duration: Style.animFast } }

            Canvas {
                anchors.fill: parent
                visible: parent.visible
                property color fillColor: filterBar.colors ? Qt.rgba(filterBar.colors.surfaceContainer.r, filterBar.colors.surfaceContainer.g, filterBar.colors.surfaceContainer.b, 0.85) : Qt.rgba(0.1, 0.12, 0.18, 0.85)
                property color strokeColor: filterBar.colors ? Qt.rgba(filterBar.colors.primary.r, filterBar.colors.primary.g, filterBar.colors.primary.b, 0.15) : Qt.rgba(1, 1, 1, 0.08)
                onFillColorChanged: requestPaint()
                onStrokeColorChanged: requestPaint()
                onWidthChanged: requestPaint()
                onPaint: {
                    var ctx = getContext("2d")
                    ctx.clearRect(0, 0, width, height)
                    var sk = filterBar._skew
                    ctx.fillStyle = fillColor
                    ctx.strokeStyle = strokeColor
                    ctx.lineWidth = 1
                    ctx.beginPath()
                    ctx.moveTo(sk, 0)
                    ctx.lineTo(width, 0)
                    ctx.lineTo(width - sk, height)
                    ctx.lineTo(0, height)
                    ctx.closePath()
                    ctx.fill()
                    ctx.stroke()
                }
            }

            Text {
                id: _convertLogText
                anchors.centerIn: parent
                width: Math.min(implicitWidth, 180)
                text: filterBar.imageOptimizeRunning ? filterBar.imageOptimizeFile : filterBar.videoConvertFile
                font.family: Style.fontFamilyCode
                font.pixelSize: 8 * Config.uiScale
                font.letterSpacing: 0.3
                elide: Text.ElideMiddle
                maximumLineCount: 1
                color: filterBar.colors ? Qt.rgba(filterBar.colors.surfaceText.r, filterBar.colors.surfaceText.g, filterBar.colors.surfaceText.b, 0.5) : Qt.rgba(1, 1, 1, 0.4)
            }
        }
    }
    }
}
