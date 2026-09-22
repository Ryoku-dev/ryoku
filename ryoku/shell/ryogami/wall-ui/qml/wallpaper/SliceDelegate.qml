import QtQuick
import QtQuick.Shapes
import QtQuick.Effects
import QtQuick.Controls
import QtMultimedia
import ".."
import "../services"
import Ryoku.Ui.Singletons

Item {
    id: delegateItem

    required property int index
    required property var model

    property var colors
    property int expandedWidth: 768
    property int sliceWidth: 108
    property int skewOffset: 28
    property var service
    property var applyRequest: null
    property var deleteRequest: null
    property bool isDepth: false

    property int selectedIdx: -1
    property bool isCurrent: ListView.isCurrentItem
    property bool isHovered: itemMouseArea.containsMouse
    property bool flipped: false
    property var _backMeta: null
    readonly property var _listView: ListView.view

    Timer {
        id: _preheatTimer
        interval: 120
        repeat: false
        onTriggered: {
            if (delegateItem.model && delegateItem.model.path)
                DaemonClient.preheat(delegateItem.model.path)
        }
    }

    onFlippedChanged: {
        if (flipped && delegateItem.model.type !== "we") {
            var key = ImageService.thumbKey(delegateItem.model.thumb, delegateItem.model.name)
            _backMeta = FileMetadataService.getMetadata(key)
            if (!_backMeta)
                FileMetadataService.probeIfNeeded(key, delegateItem.model.path, delegateItem.model.type === "video" ? "video" : "image")
        }
    }
    Connections {
        target: FileMetadataService
        enabled: delegateItem.flipped
        function onMetadataReady(key) {
            var myKey = ImageService.thumbKey(delegateItem.model.thumb, delegateItem.model.name)
            if (key === myKey)
                delegateItem._backMeta = FileMetadataService.getMetadata(key)
        }
    }

    // A skew wider than the slice itself collapses the parallelogram's flat top to
    // nothing, so the mask degenerates and the slice vanishes. Cap the shear at the
    // narrower of the current and resting widths, leaving a hairline of flat edge.
    readonly property real _skAbs: Math.min(Math.abs(skewOffset), Math.max(0, Math.min(width, sliceWidth) - 2))
    readonly property real _topLeft: skewOffset >= 0 ? _skAbs : 0
    readonly property real _topRight: skewOffset >= 0 ? width : width - _skAbs
    readonly property real _botRight: skewOffset >= 0 ? width - _skAbs : width
    readonly property real _botLeft: skewOffset >= 0 ? 0 : _skAbs
    readonly property real _slantSx: _botLeft - _topLeft
    readonly property real _slantLen: Math.max(0.001, Math.sqrt(_slantSx * _slantSx + height * height))
    readonly property real _flatW: Math.max(0.001, _topRight - _topLeft)
    property real animatedCornerRadius: Config.wallpaperSliceCornerRadius
    Behavior on animatedCornerRadius { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }
    property real animatedRTL: Config.wallpaperSliceCornerTL
    property real animatedRTR: Config.wallpaperSliceCornerTR
    property real animatedRBR: Config.wallpaperSliceCornerBR
    property real animatedRBL: Config.wallpaperSliceCornerBL
    Behavior on animatedRTL { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }
    Behavior on animatedRTR { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }
    Behavior on animatedRBR { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }
    Behavior on animatedRBL { NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic } }

    readonly property real _rcMax: Math.min(_flatW / 2 - 1, _slantLen / 2 - 1)
    readonly property real _rTL: Math.max(0, Math.min(animatedRTL, _rcMax))
    readonly property real _rTR: Math.max(0, Math.min(animatedRTR, _rcMax))
    readonly property real _rBR: Math.max(0, Math.min(animatedRBR, _rcMax))
    readonly property real _rBL: Math.max(0, Math.min(animatedRBL, _rcMax))
    readonly property real _slf: _slantSx / _slantLen
    readonly property real _hf: height / _slantLen
    readonly property real _tlInX: _topLeft + _rTL * _slf
    readonly property real _tlInY: _rTL * _hf
    readonly property real _tlOutX: _topLeft + _rTL
    readonly property real _trInX: _topRight - _rTR
    readonly property real _trOutX: _topRight + _rTR * _slf
    readonly property real _trOutY: _rTR * _hf
    readonly property real _brInX: _botRight - _rBR * _slf
    readonly property real _brInY: height - _rBR * _hf
    readonly property real _brOutX: _botRight - _rBR
    readonly property real _blInX: _botLeft + _rBL
    readonly property real _blOutX: _botLeft - _rBL * _slf
    readonly property real _blOutY: height - _rBL * _hf

    property bool suppressWidthAnim: false
    property string videoPath: delegateItem.model.videoPrev || delegateItem.model.videoFile || ""
    property bool hasVideo: videoPath.length > 0 && Config.videoPreviewEnabled
    property bool _previewArmed: false
    readonly property bool videoActive: _previewArmed && isCurrent && hasVideo && !(_listView && _listView.contentMoving)
        && (Window.window ? Window.window.visible : false)

    width: isCurrent ? expandedWidth : sliceWidth
    height: _listView ? _listView.height : 0

    onIsCurrentChanged: {
        if (!isCurrent) flipped = false
        if (isCurrent && hasVideo) {
            videoDelayTimer.restart()
        } else {
            videoDelayTimer.stop()
            _previewArmed = false
        }
        if (isCurrent) _preheatTimer.restart()
        else _preheatTimer.stop()
    }

    Timer {
        id: videoDelayTimer
        interval: Config.videoPreviewInstant ? 100 : 300
        onTriggered: delegateItem._previewArmed = true
    }

    z: isCurrent ? 100 : (isHovered ? 90 : 50 - Math.min(Math.abs(index - (_listView ? _listView.currentIndex : 0)), 50))

    readonly property real _viewCenterX: _listView ? (_listView.contentX + _listView.width / 2) : 0
    readonly property real _itemCenterX: x + width / 2
    readonly property real _halfView: _listView ? _listView.width / 2 : 1
    readonly property real _fullZone: Math.min(0.6, (expandedWidth / 2 + 2 * (sliceWidth + (_listView ? _listView.spacing : 0))) / _halfView)
    readonly property real _normDist: Math.abs(_itemCenterX - _viewCenterX) / _halfView
    opacity: _normDist <= _fullZone ? 1.0 : Math.max(0, 1.0 - (_normDist - _fullZone) / (1.2 - _fullZone))
    readonly property bool _nearViewport: opacity > 0.01
    Behavior on width {
        enabled: !suppressWidthAnim
        NumberAnimation { duration: Style.animExpand; easing.type: Easing.OutCubic }
    }

    containmentMask: Item {
        id: hitMask
        function contains(point) {
            var w = delegateItem.width
            var h = delegateItem.height
            if (h <= 0 || w <= 0) return false
            var t = point.y / h
            var leftX = delegateItem._topLeft * (1.0 - t) + delegateItem._botLeft * t
            var rightX = delegateItem._topRight * (1.0 - t) + delegateItem._botRight * t
            return point.x >= leftX && point.x <= rightX && point.y >= 0 && point.y <= h
        }
    }

    Loader {
        id: sharedVideoLoader
        width: delegateItem.width
        height: delegateItem.height
        active: delegateItem.videoActive
        visible: false
        layer.enabled: active

        sourceComponent: Video {
            anchors.fill: parent
            source: ImageService.fileUrl(delegateItem.videoPath)
            fillMode: VideoOutput.PreserveAspectCrop
            loops: MediaPlayer.Infinite
            muted: true
            Component.onCompleted: play()
        }
    }

    Item {
        id: sharedMask
        width: delegateItem.width
        height: delegateItem.height
        visible: false
        layer.enabled: delegateItem._nearViewport
        layer.smooth: true
        Shape {
            anchors.fill: parent
            antialiasing: true
            preferredRendererType: Shape.CurveRenderer
            ShapePath {
                fillColor: "white"
                strokeColor: "transparent"
                startX: delegateItem._tlOutX
                startY: 0
                PathLine { x: delegateItem._trInX; y: 0 }
                PathQuad { x: delegateItem._trOutX; y: delegateItem._trOutY; controlX: delegateItem._topRight; controlY: 0 }
                PathLine { x: delegateItem._brInX; y: delegateItem._brInY }
                PathQuad { x: delegateItem._brOutX; y: delegateItem.height; controlX: delegateItem._botRight; controlY: delegateItem.height }
                PathLine { x: delegateItem._blInX; y: delegateItem.height }
                PathQuad { x: delegateItem._blOutX; y: delegateItem._blOutY; controlX: delegateItem._botLeft; controlY: delegateItem.height }
                PathLine { x: delegateItem._tlInX; y: delegateItem._tlInY }
                PathQuad { x: delegateItem._tlOutX; y: 0; controlX: delegateItem._topLeft; controlY: 0 }
            }
        }
    }

    Item {
        id: flipContainer
        anchors.fill: parent
        transform: Rotation {
            id: flipRotation
            origin.x: flipContainer.width / 2
            origin.y: flipContainer.height / 2
            axis { x: 0; y: 1; z: 0 }
            angle: delegateItem.flipped ? 180 : 0
            Behavior on angle {
                NumberAnimation { duration: Style.animSlow; easing.type: Easing.InOutQuad }
            }
        }

    Item {
        id: frontFace
        anchors.fill: parent
        visible: flipRotation.angle < 90

    Shape {
        id: shadowShape
        z: -1
        x: delegateItem.isCurrent ? 4 : 2
        y: delegateItem.isCurrent ? 10 : 5
        width: delegateItem.width
        height: delegateItem.height
        opacity: delegateItem.isCurrent ? 0.5 : 0.3
        Behavior on x { NumberAnimation { duration: Style.animNormal } }
        Behavior on y { NumberAnimation { duration: Style.animNormal } }
        Behavior on opacity { NumberAnimation { duration: Style.animNormal } }
        ShapePath {
            fillColor: "#000000"
            strokeColor: "transparent"
            startX: delegateItem._tlOutX
            startY: 0
            PathLine { x: delegateItem._trInX; y: 0 }
            PathQuad { x: delegateItem._trOutX; y: delegateItem._trOutY; controlX: delegateItem._topRight; controlY: 0 }
            PathLine { x: delegateItem._brInX; y: delegateItem._brInY }
            PathQuad { x: delegateItem._brOutX; y: delegateItem.height; controlX: delegateItem._botRight; controlY: delegateItem.height }
            PathLine { x: delegateItem._blInX; y: delegateItem.height }
            PathQuad { x: delegateItem._blOutX; y: delegateItem._blOutY; controlX: delegateItem._botLeft; controlY: delegateItem.height }
            PathLine { x: delegateItem._tlInX; y: delegateItem._tlInY }
            PathQuad { x: delegateItem._tlOutX; y: 0; controlX: delegateItem._topLeft; controlY: 0 }
        }
    }

    Item {
        id: imageContainer
        anchors.fill: parent
        Image {
            id: thumbImage
            anchors.fill: parent
            source: delegateItem.model.thumb ? ImageService.fileUrl(delegateItem.model.thumb) : ""
            fillMode: Image.PreserveAspectCrop
            smooth: true
            asynchronous: true
            cache: true
            sourceSize.width: 400
            sourceSize.height: 720
            opacity: status === Image.Ready ? 1 : 0
            Behavior on opacity { NumberAnimation { duration: Style.animNormal; easing.type: Easing.OutCubic } }

        }

        Column {
            id: _themeSwatches
            anchors.fill: parent
            visible: delegateItem.model.kind === "theme" && thumbImage.status !== Image.Ready
            property var _sw: delegateItem.model.sw ? ("" + delegateItem.model.sw).split(",") : []
            Repeater {
                model: _themeSwatches._sw
                Rectangle {
                    width: delegateItem.width
                    height: delegateItem.height / Math.max(1, _themeSwatches._sw.length)
                    color: modelData
                }
            }
        }

        Rectangle {
            anchors.fill: parent
            visible: opacity > 0
            opacity: (thumbImage.status === Image.Ready || delegateItem.model.kind === "theme") ? 0 : 1
            Behavior on opacity { NumberAnimation { duration: Style.animNormal; easing.type: Easing.OutCubic } }
            color: delegateItem.colors ? Qt.rgba(delegateItem.colors.surfaceVariant.r, delegateItem.colors.surfaceVariant.g, delegateItem.colors.surfaceVariant.b, 0.8) : Qt.rgba(0.18, 0.20, 0.25, 0.8)
        }

        Rectangle {
            anchors.fill: parent
            color: Qt.rgba(0, 0, 0, delegateItem.isCurrent ? 0 : (delegateItem.isHovered ? 0.15 : 0.4))
            Behavior on color { ColorAnimation { duration: Style.animNormal } }
        }
        layer.enabled: delegateItem._nearViewport
        layer.smooth: true
        layer.effect: MultiEffect {
            maskEnabled: true
            maskSource: sharedMask
            maskThresholdMin: 0.3
            maskSpreadAtMin: 0.3
        }
    }

    Item {
        id: videoOverlay
        anchors.fill: parent
        visible: sharedVideoLoader.active && sharedVideoLoader.status === Loader.Ready

        ShaderEffectSource {
            anchors.fill: parent
            sourceItem: sharedVideoLoader
            live: true
        }

        layer.enabled: delegateItem._nearViewport
        layer.smooth: true
        layer.effect: MultiEffect {
            maskEnabled: true
            maskSource: sharedMask
            maskThresholdMin: 0.3
            maskSpreadAtMin: 0.3
        }
    }

    Shape {
        id: glowBorder
        anchors.fill: parent
        antialiasing: true
        preferredRendererType: Shape.CurveRenderer
        opacity: 1.0
        ShapePath {
            fillColor: "transparent"
            strokeColor: delegateItem.isCurrent
                ? (delegateItem.colors ? delegateItem.colors.primary : "#8BC34A")
                : (delegateItem.isHovered
                    ? Qt.rgba(delegateItem.colors ? delegateItem.colors.primary.r : 0.5, delegateItem.colors ? delegateItem.colors.primary.g : 0.76, delegateItem.colors ? delegateItem.colors.primary.b : 0.29, 0.4)
                    : Qt.rgba(0, 0, 0, 0.6))
            Behavior on strokeColor { ColorAnimation { duration: Style.animNormal } }
            strokeWidth: delegateItem.isCurrent ? 3 : 1
            startX: delegateItem._tlOutX
            startY: 0
            PathLine { x: delegateItem._trInX; y: 0 }
            PathQuad { x: delegateItem._trOutX; y: delegateItem._trOutY; controlX: delegateItem._topRight; controlY: 0 }
            PathLine { x: delegateItem._brInX; y: delegateItem._brInY }
            PathQuad { x: delegateItem._brOutX; y: delegateItem.height; controlX: delegateItem._botRight; controlY: delegateItem.height }
            PathLine { x: delegateItem._blInX; y: delegateItem.height }
            PathQuad { x: delegateItem._blOutX; y: delegateItem._blOutY; controlX: delegateItem._botLeft; controlY: delegateItem.height }
            PathLine { x: delegateItem._tlInX; y: delegateItem._tlInY }
            PathQuad { x: delegateItem._tlOutX; y: 0; controlX: delegateItem._topLeft; controlY: 0 }
        }
    }

    Rectangle {
        id: videoIndicator
        anchors.top: parent.top
        anchors.topMargin: 10
        x: delegateItem.skewOffset >= 0
            ? parent.width - width - 10
            : 10
        width: 22
        height: 22
        radius: 11
        color: delegateItem.videoActive ? (delegateItem.colors ? delegateItem.colors.primary : Style.fallbackAccent) : Qt.rgba(0, 0, 0, 0.7)
        border.width: 1
        border.color: delegateItem.videoActive
            ? "transparent"
            : (delegateItem.colors ? Qt.rgba(delegateItem.colors.primary.r, delegateItem.colors.primary.g, delegateItem.colors.primary.b, 0.6) : Qt.rgba(1, 1, 1, 0.4))
        visible: delegateItem.hasVideo && delegateItem.model.kind !== "theme" && delegateItem.model.kind !== "rice"
        z: 10

        Behavior on color { ColorAnimation { duration: Style.animNormal } }

        Text {
            anchors.centerIn: parent
            anchors.horizontalCenterOffset: 1
            text: "▶"
            font.pixelSize: 9
            color: delegateItem.videoActive
                ? (delegateItem.colors ? delegateItem.colors.primaryText : "#000")
                : (delegateItem.colors ? delegateItem.colors.primary : Style.fallbackAccent)
        }
    }

    Rectangle {
        id: typeBadge
        anchors.bottom: parent.bottom
        anchors.bottomMargin: 8
        property bool onRight: delegateItem.skewOffset >= 0
        width: typeBadgeText.implicitWidth + 16
        height: 16
        radius: Math.min(height / 2, delegateItem.animatedCornerRadius * 0.5)
        z: 10
        x: onRight
            ? parent.width - width - delegateItem._skAbs - 8
            : delegateItem._skAbs + 8
        color: Qt.rgba(0, 0, 0, 0.75)
        border.width: 1
        border.color: delegateItem.colors ? Qt.rgba(delegateItem.colors.primary.r, delegateItem.colors.primary.g, delegateItem.colors.primary.b, 0.4) : Qt.rgba(1, 1, 1, 0.2)
        visible: delegateItem.model.kind !== "theme" && delegateItem.model.kind !== "rice"

        Text {
            id: typeBadgeText
            anchors.centerIn: parent
            text: delegateItem.model.type === "static" ? I18n.tr("PIC") : ((delegateItem.model.type === "video" || delegateItem.model.videoFile) ? I18n.tr("VID") : I18n.tr("WE"))
            font.family: Style.fontFamily
            font.pixelSize: 9
            font.weight: Font.Bold
            font.letterSpacing: 0.5
            color: delegateItem.colors ? delegateItem.colors.tertiary : "#8bceff"
        }
    }

    Rectangle {
        id: depthBadge
        anchors.bottom: typeBadge.bottom
        anchors.right: typeBadge.onRight ? typeBadge.left : undefined
        anchors.rightMargin: typeBadge.onRight ? 4 : 0
        anchors.left: typeBadge.onRight ? undefined : typeBadge.right
        anchors.leftMargin: typeBadge.onRight ? 0 : 4
        width: height
        height: typeBadge.height
        radius: typeBadge.radius
        z: 10
        color: Qt.rgba(0, 0, 0, 0.75)
        border.width: 1
        border.color: delegateItem.colors ? delegateItem.colors.primary : Style.fallbackAccent
        visible: delegateItem.isDepth && delegateItem.model.kind !== "theme" && delegateItem.model.kind !== "rice"

        Text {
            anchors.centerIn: parent
            text: "深"
            font.family: Style.fontFamily
            font.pixelSize: 9
            font.weight: Font.Bold
            color: delegateItem.colors ? delegateItem.colors.primary : Style.fallbackAccent
        }
    }

    Item {
        id: themeNameStrip
        visible: delegateItem.model.kind === "theme"
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.leftMargin: delegateItem._skAbs
        anchors.rightMargin: delegateItem._skAbs
        height: 30
        z: 9

        Rectangle {
            anchors.fill: parent
            gradient: Gradient {
                GradientStop { position: 0.0; color: "transparent" }
                GradientStop { position: 1.0; color: Qt.rgba(0, 0, 0, 0.72) }
            }
        }

        Text {
            anchors.left: parent.left; anchors.right: parent.right
            anchors.bottom: parent.bottom
            anchors.margins: 8
            text: "" + (delegateItem.model.name || "")
            elide: Text.ElideRight
            horizontalAlignment: Text.AlignHCenter
            font.family: Style.fontFamily
            font.pixelSize: 11
            font.weight: Font.Medium
            color: "#ffffff"
        }
    }

    }

    Item {
        id: backFace
        anchors.fill: parent
        visible: flipRotation.angle >= 90
        transform: Rotation {
            origin.x: backFace.width / 2
            origin.y: backFace.height / 2
            axis { x: 0; y: 1; z: 0 }
            angle: 180
        }

        Item {
            id: backClip
            anchors.fill: parent

            Rectangle {
                anchors.fill: parent
                color: delegateItem.colors
                    ? delegateItem.colors.surfaceContainer
                    : "#1a1a2e"
            }

            ShaderEffectSource {
                anchors.fill: parent
                sourceItem: sharedVideoLoader
                live: true
                visible: delegateItem.videoActive && delegateItem.flipped && sharedVideoLoader.status === Loader.Ready
                opacity: 0.25
            }

            Image {
                anchors.fill: parent
                source: ImageService.fileUrl(delegateItem.model.thumb)
                fillMode: Image.PreserveAspectCrop
                opacity: 0.12
                visible: !(delegateItem.videoActive && delegateItem.flipped)
                cache: false; asynchronous: true
                sourceSize.width: 120
                sourceSize.height: 216
            }

            Column {
                anchors.fill: parent
                anchors.leftMargin: delegateItem._skAbs + 14
                anchors.rightMargin: delegateItem._skAbs + 14
                anchors.topMargin: 16
                anchors.bottomMargin: 16
                spacing: 10

                Text {
                    width: parent.width
                    text: delegateItem.model.name.replace(/\.[^/.]+$/, "").toUpperCase()
                    color: delegateItem.colors ? delegateItem.colors.tertiary : "#8bceff"
                    font.family: Style.fontFamily
                    font.pixelSize: 13
                    font.weight: Font.Bold
                    font.letterSpacing: 1
                    horizontalAlignment: Text.AlignHCenter
                    wrapMode: Text.Wrap
                    elide: Text.ElideRight
                    maximumLineCount: 2
                }

                Row {
                    width: parent.width
                    spacing: 0
                    visible: delegateItem.model.type !== "we"
                    layoutDirection: Qt.LeftToRight

                    Text {
                        text: FileMetadataService.formatExt(delegateItem.model.name)
                        color: delegateItem.colors ? Qt.rgba(delegateItem.colors.tertiary.r, delegateItem.colors.tertiary.g, delegateItem.colors.tertiary.b, 0.6) : Qt.rgba(1,1,1,0.35)
                        font.family: Style.fontFamily; font.pixelSize: 10; font.weight: Font.Medium; font.letterSpacing: 0.8
                    }
                    Text {
                        text: "  \u2022  "
                        color: Qt.rgba(1, 1, 1, 0.15)
                        font.family: Style.fontFamily; font.pixelSize: 10
                    }
                    Text {
                        text: delegateItem._backMeta ? (delegateItem._backMeta.width + " \u00d7 " + delegateItem._backMeta.height) : "\u2013"
                        color: delegateItem.colors ? Qt.rgba(delegateItem.colors.tertiary.r, delegateItem.colors.tertiary.g, delegateItem.colors.tertiary.b, 0.6) : Qt.rgba(1,1,1,0.35)
                        font.family: Style.fontFamily; font.pixelSize: 10; font.weight: Font.Medium; font.letterSpacing: 0.5
                    }
                    Text {
                        text: "  \u2022  "
                        color: Qt.rgba(1, 1, 1, 0.15)
                        font.family: Style.fontFamily; font.pixelSize: 10
                    }
                    Text {
                        text: delegateItem._backMeta ? FileMetadataService.formatSize(delegateItem._backMeta.filesize) : "\u2013"
                        color: delegateItem.colors ? Qt.rgba(delegateItem.colors.tertiary.r, delegateItem.colors.tertiary.g, delegateItem.colors.tertiary.b, 0.6) : Qt.rgba(1,1,1,0.35)
                        font.family: Style.fontFamily; font.pixelSize: 10; font.weight: Font.Medium; font.letterSpacing: 0.5
                    }
                }

                Item {
                    width: parent.width; height: 28

                    Text {
                        anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("FAVOURITE")
                        color: delegateItem.colors ? delegateItem.colors.tertiary : "#8bceff"
                        font.family: Style.fontFamily; font.pixelSize: 11
                        font.weight: Font.Medium; font.letterSpacing: 0.5
                    }

                    Item {
                        id: favToggle
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        width: 48; height: 24
                        property bool checked: false
                        Component.onCompleted: {
                            var key = (delegateItem.model.weId || "") !== "" ? delegateItem.model.weId : delegateItem.model.name
                            checked = delegateItem.service ? !!delegateItem.service.favouritesDb[key] : false
                        }
                        Connections {
                            target: delegateItem
                            function onFlippedChanged() {
                                if (delegateItem.flipped) {
                                    var key = (delegateItem.model.weId || "") !== "" ? delegateItem.model.weId : delegateItem.model.name
                                    favToggle.checked = delegateItem.service ? !!delegateItem.service.favouritesDb[key] : false
                                }
                            }
                        }
                        Canvas {
                            anchors.fill: parent
                            property bool isOn: favToggle.checked
                            property color fillColor: isOn
                                ? (delegateItem.colors ? delegateItem.colors.primary : Style.fallbackAccent)
                                : Qt.rgba(1, 1, 1, 0.15)
                            onFillColorChanged: requestPaint()
                            onIsOnChanged: requestPaint()
                            onPaint: {
                                var ctx = getContext("2d"); ctx.clearRect(0, 0, width, height)
                                var sk = 8; ctx.fillStyle = fillColor; ctx.beginPath()
                                ctx.moveTo(sk, 0); ctx.lineTo(width, 0)
                                ctx.lineTo(width - sk, height); ctx.lineTo(0, height)
                                ctx.closePath(); ctx.fill()
                            }
                        }
                        Canvas {
                            width: 22; height: 18; y: 3
                            x: favToggle.checked ? parent.width - width - 4 : 4
                            Behavior on x { NumberAnimation { duration: Style.animFast; easing.type: Easing.OutCubic } }
                            property color knobColor: favToggle.checked
                                ? (delegateItem.colors ? delegateItem.colors.primaryText : "#000")
                                : (delegateItem.colors ? delegateItem.colors.surfaceText : "#fff")
                            onKnobColorChanged: requestPaint()
                            onPaint: {
                                var ctx = getContext("2d"); ctx.clearRect(0, 0, width, height)
                                var sk = 5; ctx.fillStyle = knobColor; ctx.beginPath()
                                ctx.moveTo(sk, 0); ctx.lineTo(width, 0)
                                ctx.lineTo(width - sk, height); ctx.lineTo(0, height)
                                ctx.closePath(); ctx.fill()
                            }
                        }
                        MouseArea {
                            anchors.fill: parent; cursorShape: Qt.PointingHandCursor
                            onClicked: { favToggle.checked = !favToggle.checked; delegateItem.service.toggleFavourite(delegateItem.model.name, delegateItem.model.weId || "") }
                        }
                    }
                }

                Item {
                    width: parent.width; height: 28
                    visible: Config.canOverviewBackdrop && Config.overviewBackdropEnabled && delegateItem.model.type === "static"
                    property bool _isBackdrop: Config.overviewBackdropPath === delegateItem.model.path

                    Text {
                        anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("OVERVIEW BACKDROP")
                        color: delegateItem.colors ? delegateItem.colors.tertiary : "#8bceff"
                        font.family: Style.fontFamily; font.pixelSize: 11
                        font.weight: Font.Medium; font.letterSpacing: 0.5
                    }

                    Rectangle {
                        id: bdBtn
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        height: 24; width: bdBtnLbl.implicitWidth + 20; radius: 4
                        color: bdBtn.parent._isBackdrop
                            ? (delegateItem.colors ? delegateItem.colors.primary : Style.fallbackAccent)
                            : Qt.rgba(1, 1, 1, 0.12)
                        Behavior on color { ColorAnimation { duration: 140 } }

                        Text {
                            id: bdBtnLbl
                            anchors.centerIn: parent
                            text: bdBtn.parent._isBackdrop ? I18n.tr("Current ✓") : I18n.tr("Set")
                            color: bdBtn.parent._isBackdrop
                                ? (delegateItem.colors ? delegateItem.colors.primaryText : "#000")
                                : (delegateItem.colors ? delegateItem.colors.surfaceText : "#fff")
                            font.family: Style.fontFamily; font.pixelSize: 11; font.weight: Font.Medium
                        }

                        MouseArea {
                            anchors.fill: parent; cursorShape: Qt.PointingHandCursor
                            onClicked: {
                                if (bdBtn.parent._isBackdrop) delegateItem.service.applyBackdrop("")
                                else delegateItem.service.applyBackdrop(delegateItem.model.path)
                            }
                        }
                    }
                }

                Rectangle { width: parent.width; height: 1; color: Qt.rgba(1, 1, 1, 0.08) }

                Item {
                    width: parent.width
                    height: parent.height - y - backActionRow.height - parent.spacing
                }

                Row {
                    id: backActionRow
                    width: parent.width; height: 30
                    spacing: 6

                    
                    property int _slotCount: delegateItem.model.type === "we" ? 3 : 2
                    property real _slotWidth: (width - spacing * (_slotCount - 1)) / _slotCount

                    ActionButton {
                        width: backActionRow._slotWidth
                        colors: delegateItem.colors
                        icon: "\u{f0208}"; label: I18n.tr("VIEW")
                        skew: Math.abs(delegateItem.skewOffset) * 0.4
                        onClicked: {
                            var dir = delegateItem.model.path.substring(0, delegateItem.model.path.lastIndexOf("/"))
                            Qt.openUrlExternally(ImageService.fileUrl(dir))
                            delegateItem.flipped = false
                        }
                    }

                    ActionButton {
                        width: backActionRow._slotWidth
                        colors: delegateItem.colors
                        icon: "\u{f0a79}"; label: I18n.tr("DELETE"); danger: true
                        skew: Math.abs(delegateItem.skewOffset) * 0.4
                        onClicked: {
                            var idx = index
                            delegateItem.service.deleteWallpaperItem(delegateItem.model.type, delegateItem.model.name, delegateItem.model.weId || "")
                            var newIdx = Math.min(idx, delegateItem.service.filteredModel.count - 1)
                            if (delegateItem._listView) {
                                delegateItem._listView.currentIndex = -1
                                delegateItem._listView.currentIndex = newIdx
                                delegateItem._listView.positionViewAtIndex(newIdx, ListView.Center)
                            }
                        }
                    }

                    ActionButton {
                        visible: delegateItem.model.type === "we"
                        width: visible ? backActionRow._slotWidth : 0
                        colors: delegateItem.colors
                        icon: "\u{f0bef}"; label: I18n.tr("STEAM")
                        skew: Math.abs(delegateItem.skewOffset) * 0.4
                        onClicked: { delegateItem.service.openSteamPage(delegateItem.model.weId || ""); delegateItem.flipped = false }
                    }
                }
            }

            MouseArea {
                anchors.fill: parent
                z: -1
                onClicked: delegateItem.flipped = false
            }

            layer.enabled: delegateItem._nearViewport
            layer.smooth: true
            layer.effect: MultiEffect {
                maskEnabled: true
                maskSource: sharedMask
                maskThresholdMin: 0.3
                maskSpreadAtMin: 0.3
            }
        }

        Shape {
            anchors.fill: parent
            antialiasing: true
            preferredRendererType: Shape.CurveRenderer
            ShapePath {
                fillColor: "transparent"
                strokeColor: delegateItem.colors ? delegateItem.colors.primary : "#8BC34A"
                strokeWidth: 2
                startX: delegateItem._tlOutX
                startY: 0
                PathLine { x: delegateItem._trInX; y: 0 }
                PathQuad { x: delegateItem._trOutX; y: delegateItem._trOutY; controlX: delegateItem._topRight; controlY: 0 }
                PathLine { x: delegateItem._brInX; y: delegateItem._brInY }
                PathQuad { x: delegateItem._brOutX; y: delegateItem.height; controlX: delegateItem._botRight; controlY: delegateItem.height }
                PathLine { x: delegateItem._blInX; y: delegateItem.height }
                PathQuad { x: delegateItem._blOutX; y: delegateItem._blOutY; controlX: delegateItem._botLeft; controlY: delegateItem.height }
                PathLine { x: delegateItem._tlInX; y: delegateItem._tlInY }
                PathQuad { x: delegateItem._tlOutX; y: 0; controlX: delegateItem._topLeft; controlY: 0 }
            }
        }
    }

    }

    MouseArea {
        id: itemMouseArea
        anchors.fill: parent
        hoverEnabled: !delegateItem.flipped
        acceptedButtons: delegateItem.flipped ? Qt.RightButton : (Qt.LeftButton | Qt.RightButton)
        cursorShape: delegateItem.flipped ? Qt.ArrowCursor : Qt.PointingHandCursor
        onPositionChanged: function(mouse) {
            if (delegateItem.flipped) return
            if (!delegateItem._listView) return
            if (delegateItem._listView.navLocked) return
            if (delegateItem._listView.moving) return
            var globalPos = mapToItem(delegateItem._listView, mouse.x, mouse.y)
            var dx = Math.abs(globalPos.x - delegateItem._listView.lastMouseX)
            var dy = Math.abs(globalPos.y - delegateItem._listView.lastMouseY)
            if (dx > 2 || dy > 2) {
                delegateItem._listView.lastMouseX = globalPos.x
                delegateItem._listView.lastMouseY = globalPos.y
                delegateItem._listView.keyboardNavActive = false
                delegateItem._listView.currentIndex = index
            }
        }
        onClicked: function(mouse) {
            if (mouse.button === Qt.RightButton) {
                if (delegateItem._listView) delegateItem._listView.currentIndex = index
                if (delegateItem.model && delegateItem.model.kind === "rice" && delegateItem.deleteRequest) {
                    delegateItem.deleteRequest(delegateItem.model)
                    return
                }
                delegateItem.flipped = !delegateItem.flipped
            } else if (!delegateItem.flipped) {
                if (delegateItem.isCurrent) {
                    var forcePicker = !!(mouse.modifiers & Qt.ControlModifier)
                    if (delegateItem.applyRequest) {
                        delegateItem.applyRequest(delegateItem.model, forcePicker)
                    } else if (delegateItem.model.type === "we") {
                        delegateItem.service.applyWE(delegateItem.model.weId)
                    } else if (delegateItem.model.type === "video") {
                        delegateItem.service.applyVideo(delegateItem.model.path)
                    } else {
                        delegateItem.service.applyStatic(delegateItem.model.path)
                    }
                } else {
                    if (delegateItem._listView) delegateItem._listView.currentIndex = index
                }
            }
        }
    }
}
