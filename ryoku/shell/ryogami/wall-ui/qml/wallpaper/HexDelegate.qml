import QtQuick
import QtQuick.Shapes
import QtQuick.Effects
import QtMultimedia
import ".."
import "../services"
import Ryoku.Ui.Singletons

Item {
    id: hexItem

    property var colors
    property var service
    property int hexRadius: 140
    property string hexShape: "hexagon"
    property int gridRow: 0
    property int gridColumn: 0
    property var itemData
    property var applyRequest: null
    property bool isSelected: false
    property bool isHovered: hexMouse.containsMouse
    property bool pulledOut: false
    property bool viewMoving: false
    property bool isDepth: false

    property real parallaxX: 0
    property real parallaxY: 0

    signal flipRequested(var data, real gx, real gy, var sourceItem)
    signal hoverSelected()

    // Preview players decode the small scan-time clip, never the full source.
    property string videoPath: itemData ? (itemData.videoPrev || itemData.videoFile || "") : ""
    property bool hasVideo: videoPath.length > 0 && Config.videoPreviewEnabled
    property bool _previewArmed: false
    readonly property bool videoActive: _previewArmed && isSelected && hasVideo && !viewMoving
        && (Window.window ? Window.window.visible : false)

    onIsSelectedChanged: {
        if (isSelected && hasVideo) {
            _videoDelayTimer.restart()
        } else {
            _videoDelayTimer.stop()
            _previewArmed = false
        }
    }

    Timer {
        id: _videoDelayTimer
        interval: Config.videoPreviewInstant ? 100 : 300
        onTriggered: hexItem._previewArmed = true
    }

    width: hexShape === "rhombus" ? hexRadius * 4 : hexRadius * 2
    height: hexShape === "diamond" ? Math.ceil(hexRadius * 3.4641) : Math.ceil(hexRadius * 1.73205)

    readonly property real _r: hexRadius
    readonly property real _cx: width / 2
    readonly property real _cy: height / 2
    readonly property real _cos30: 0.866025
    readonly property real _sin30: 0.5
    readonly property string _tilePath: {
        switch (hexShape) {
        case "triangle":
            return ((gridRow + gridColumn) % 2 === 0)
                ? "M " + _cx + " 0 L " + width + " " + height + " L 0 " + height + " Z"
                : "M 0 0 L " + width + " 0 L " + _cx + " " + height + " Z"
        case "diamond": return "M " + _cx + " 0 L " + width + " " + _cy + " L " + _cx + " " + height + " L 0 " + _cy + " Z"
        case "rhombus": return "M " + _cx + " 0 L " + width + " " + _cy + " L " + _cx + " " + height + " L 0 " + _cy + " Z"
        default: return "M " + (_cx + _r) + " " + _cy + " L " + (_cx + _r * _sin30) + " " + (_cy - _r * _cos30) + " L " + (_cx - _r * _sin30) + " " + (_cy - _r * _cos30) + " L " + (_cx - _r) + " " + _cy + " L " + (_cx - _r * _sin30) + " " + (_cy + _r * _cos30) + " L " + (_cx + _r * _sin30) + " " + (_cy + _r * _cos30) + " Z"
        }
    }

    Item {
        id: hexMask
        width: hexItem.width; height: hexItem.height
        visible: false
        layer.enabled: true
        Shape {
            anchors.fill: parent
            antialiasing: true
            preferredRendererType: Shape.CurveRenderer
            ShapePath {
                fillColor: "white"
                strokeColor: "transparent"
                PathSvg { path: hexItem._tilePath }
            }
        }
    }

    Item {
        id: imageContainer
        anchors.fill: parent
        opacity: hexItem.pulledOut ? 0 : 1
        Behavior on opacity { NumberAnimation { duration: Style.animFast } }

            Rectangle {
                id: hexPlaceholder
                anchors.centerIn: parent
                width: hexItem.width * 1.3
                height: hexItem.height * 1.3
                color: Style.fallbackAccent
                opacity: (thumbImage.status === Image.Ready && thumbImage.source != "") ? 0 : 0.08
                Behavior on opacity { NumberAnimation { duration: Style.animNormal; easing.type: Easing.OutCubic } }
                visible: opacity > 0

                Text {
                    anchors.centerIn: parent
                    text: "\u{f0553}"
                    font.family: Style.fontFamilyNerdIcons; font.pixelSize: 22
                    color: Qt.rgba(1, 1, 1, 0.1)
                    visible: thumbImage.status !== Image.Ready
                }
            }

            Image {
                id: thumbImage
                width: hexItem.width * 1.3
                height: hexItem.height * 1.3
                x: (hexItem.width - width) / 2 + hexItem.parallaxX
                y: (hexItem.height - height) / 2 + hexItem.parallaxY
                source: hexItem.itemData && hexItem.itemData.thumb ? ImageService.fileUrl(hexItem.itemData.thumb) : ""
                fillMode: Image.PreserveAspectCrop
                smooth: true
                asynchronous: true
                cache: true
                sourceSize.width: Math.ceil(hexItem.width * 1.3)
                sourceSize.height: Math.ceil(hexItem.height * 1.3)
                opacity: status === Image.Ready ? 1 : 0
                Behavior on opacity { NumberAnimation { duration: Style.animNormal; easing.type: Easing.OutCubic } }
            }

            Column {
                id: _hexSwatches
                anchors.centerIn: parent
                width: hexItem.width * 1.3
                height: hexItem.height * 1.3
                visible: hexItem.itemData && hexItem.itemData.kind === "theme" && thumbImage.status !== Image.Ready
                property var _sw: (hexItem.itemData && hexItem.itemData.sw) ? ("" + hexItem.itemData.sw).split(",") : []
                Repeater {
                    model: _hexSwatches._sw
                    Rectangle {
                        width: _hexSwatches.width
                        height: _hexSwatches.height / Math.max(1, _hexSwatches._sw.length)
                        color: modelData
                    }
                }
            }

            layer.enabled: true
            layer.smooth: true
            layer.effect: MultiEffect {
                maskEnabled: true
                maskSource: hexMask
                maskThresholdMin: 0.3
                maskSpreadAtMin: 0.3
            }
    }

    Loader {
        id: _videoLoader
        width: hexItem.width
        height: hexItem.height
        active: hexItem.videoActive
        visible: false
        layer.enabled: active

        sourceComponent: Video {
            anchors.fill: parent
            source: ImageService.fileUrl(hexItem.videoPath)
            fillMode: VideoOutput.PreserveAspectCrop
            loops: MediaPlayer.Infinite
            muted: true
            Component.onCompleted: play()
        }
    }

    Item {
        id: videoOverlay
        anchors.fill: parent
        visible: _videoLoader.active && _videoLoader.status === Loader.Ready
        opacity: hexItem.pulledOut ? 0 : 1
        Behavior on opacity { NumberAnimation { duration: Style.animFast } }

        ShaderEffectSource {
            anchors.fill: parent
            sourceItem: _videoLoader
            live: true
        }

        layer.enabled: true
        layer.smooth: true
        layer.effect: MultiEffect {
            maskEnabled: true
            maskSource: hexMask
            maskThresholdMin: 0.3
            maskSpreadAtMin: 0.3
        }
    }

    Shape {
        anchors.fill: parent
        visible: hexItem.pulledOut
        opacity: hexItem.pulledOut ? 1 : 0
        Behavior on opacity { NumberAnimation { duration: Style.animFast } }
        antialiasing: true
        preferredRendererType: Shape.CurveRenderer
        ShapePath {
            fillColor: hexItem.colors ? Qt.rgba(hexItem.colors.primary.r, hexItem.colors.primary.g, hexItem.colors.primary.b, 0.08) : Qt.rgba(1,1,1,0.05)
            strokeColor: hexItem.colors ? Qt.rgba(hexItem.colors.primary.r, hexItem.colors.primary.g, hexItem.colors.primary.b, 0.4) : Qt.rgba(1,1,1,0.2)
            strokeWidth: 2
            strokeStyle: ShapePath.DashLine
            dashPattern: [4, 4]
            PathSvg { path: hexItem._tilePath }
        }
    }

    Shape {
        id: hexBorder
        anchors.fill: parent
        antialiasing: true
        preferredRendererType: Shape.CurveRenderer
        ShapePath {
            fillColor: "transparent"
            strokeColor: hexItem.isSelected
                ? (hexItem.colors ? hexItem.colors.primary : Style.fallbackAccent)
                : Qt.rgba(0, 0, 0, 0.5)
            Behavior on strokeColor { ColorAnimation { duration: Style.animFast } }
            strokeWidth: hexItem.isSelected ? 3 : 1.5
            PathSvg { path: hexItem._tilePath }
        }
    }

    Rectangle {
        id: typeBadge
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottom: parent.bottom
        anchors.bottomMargin: hexItem._r * 0.18
        width: typeBadgeLabel.implicitWidth + 14
        height: 18
        radius: 9
        color: Qt.rgba(0, 0, 0, 0.75)
        border.width: 1
        border.color: hexItem.colors ? Qt.rgba(hexItem.colors.primary.r, hexItem.colors.primary.g, hexItem.colors.primary.b, 0.4) : Qt.rgba(1,1,1,0.2)
        z: 5
        visible: hexItem.itemData && hexItem.itemData.kind !== "theme" && hexItem.itemData.kind !== "rice"

        Text {
            id: typeBadgeLabel
            anchors.centerIn: parent
            text: hexItem.itemData ? (hexItem.itemData.type === "static" ? I18n.tr("PIC") : ((hexItem.itemData.type === "video" || hexItem.itemData.videoFile) ? I18n.tr("VID") : I18n.tr("WE"))) : ""
            font.family: Style.fontFamily; font.pixelSize: 9; font.weight: Font.Bold; font.letterSpacing: 0.5
            color: hexItem.colors ? hexItem.colors.tertiary : "#8bceff"
        }
    }

    Rectangle {
        anchors.verticalCenter: typeBadge.verticalCenter
        anchors.right: typeBadge.left
        anchors.rightMargin: 4
        width: height
        height: 18
        radius: 9
        color: Qt.rgba(0, 0, 0, 0.75)
        border.width: 1
        border.color: hexItem.colors ? hexItem.colors.primary : Style.fallbackAccent
        z: 5
        visible: hexItem.isDepth && hexItem.itemData && hexItem.itemData.kind !== "theme" && hexItem.itemData.kind !== "rice"

        Text {
            anchors.centerIn: parent
            text: "深"
            font.family: Style.fontFamily; font.pixelSize: 9; font.weight: Font.Bold
            color: hexItem.colors ? hexItem.colors.primary : Style.fallbackAccent
        }
    }

    Rectangle {
        x: hexItem._cx + hexItem._r * hexItem._sin30 - width - 4
        y: hexItem._cy - hexItem._r * hexItem._cos30 + 8
        width: 20; height: 20; radius: 10
        color: hexItem.videoActive ? (hexItem.colors ? hexItem.colors.primary : Style.fallbackAccent) : Qt.rgba(0, 0, 0, 0.7)
        border.width: 1
        border.color: hexItem.videoActive
            ? "transparent"
            : (hexItem.colors ? Qt.rgba(hexItem.colors.primary.r, hexItem.colors.primary.g, hexItem.colors.primary.b, 0.6) : Qt.rgba(1,1,1,0.4))
        visible: hexItem.hasVideo && hexItem.itemData && hexItem.itemData.kind !== "theme" && hexItem.itemData.kind !== "rice"
        z: 5

        Behavior on color { ColorAnimation { duration: Style.animFast } }

        Text {
            anchors.centerIn: parent; anchors.horizontalCenterOffset: 1
            text: "▶"; font.pixelSize: 8
            color: hexItem.videoActive
                ? (hexItem.colors ? hexItem.colors.primaryText : "#000")
                : (hexItem.colors ? hexItem.colors.primary : Style.fallbackAccent)
        }
    }

    MouseArea {
        id: hexMouse
        anchors.fill: parent
        hoverEnabled: true
        acceptedButtons: Qt.LeftButton | Qt.RightButton
        cursorShape: Qt.PointingHandCursor
        function contains(point) {
            var x = point.x
            var y = point.y
            if (x < 0 || y < 0 || x > width || y > height) return false
            if (hexItem.hexShape === "triangle") {
                var t = y / height
                if ((hexItem.gridRow + hexItem.gridColumn) % 2 === 0)
                    return x >= hexItem._cx * (1 - t) && x <= hexItem._cx * (1 + t)
                return x >= hexItem._cx * t && x <= width - hexItem._cx * t
            }
            if (hexItem.hexShape === "diamond" || hexItem.hexShape === "rhombus")
                return Math.abs(x - hexItem._cx) / hexItem._cx + Math.abs(y - hexItem._cy) / hexItem._cy <= 1
            var dx = Math.abs(x - hexItem._cx)
            var dy = Math.abs(y - hexItem._cy)
            return dy <= hexItem._cos30 * hexItem._r && dx <= hexItem._r - dy * 0.57735
        }
        onContainsMouseChanged: {
            if (containsMouse) hexItem.hoverSelected()
        }
        onClicked: function(mouse) {
            if (mouse.button === Qt.RightButton && hexItem.itemData) {
                var gp = hexItem.mapToItem(null, hexItem._cx, hexItem._cy)
                hexItem.flipRequested(hexItem.itemData, gp.x, gp.y, hexItem)
            } else if (mouse.button === Qt.LeftButton && hexItem.itemData) {
                var forcePicker = !!(mouse.modifiers & Qt.ControlModifier)
                if (hexItem.applyRequest) {
                    hexItem.applyRequest(hexItem.itemData, forcePicker)
                } else if (hexItem.itemData.type === "we") {
                    hexItem.service.applyWE(hexItem.itemData.weId)
                } else if (hexItem.itemData.type === "video") {
                    hexItem.service.applyVideo(hexItem.itemData.path)
                } else {
                    hexItem.service.applyStatic(hexItem.itemData.path)
                }
            }
        }
    }
}
