import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Io
import shell.services as Ryoku
import "../../"
import "../"
import "glassNesting.js" as Nesting

Item {
    id: root

    property real radius: ThemeBackend.borderRadius
    property real topLeftRadius: radius
    property real topRightRadius: radius
    property real bottomLeftRadius: radius
    property real bottomRightRadius: radius
    property real blurStrength: 1.0
    property real tint: 0.0
    property real rim: 0.45
    property vector2d filletDir: Qt.vector2d(0, 0)
    property real filletRadius: 0
    property real filletInset: 0
    property bool earsOnly: false
    property rect boxRect: Qt.rect(0, 0, 0, 0)
    property Item maskItem: null
    property real bezelSize: Math.max(5, Math.min(26, Math.min(width, height) * 0.36))
    property bool standalone: false
    property bool surfaceEnabled: true
    property var screenOverride: null
    property bool useSnapshot: false

    readonly property bool liquid: ThemeBackend.liquidGlass
    readonly property real blurPad: liquid ? 0 : 64
    readonly property bool isGlassBackdrop: true
    property int consumers: 0
    property int nestLevel: 0
    readonly property real nest: Math.min(1, nestLevel * 0.5)
    property int frame: 0

    readonly property var hostWindow: QsWindow.window
    readonly property var hostScreen: screenOverride ? screenOverride : (hostWindow ? hostWindow.screen : null)
    readonly property var effectiveScreenOverride: screenOverride
    readonly property bool parentVisible: parent ? parent.visible : true
    readonly property string wallpaperPath: Ryoku.Session.wallIsVideo
        ? Ryoku.Session.livePoster : Ryoku.Session.wallpaper
    readonly property string wallpaperUrl: wallpaperPath !== ""
        ? "file://" + wallpaperPath + "?rev=" + revision : ""
    readonly property bool active: surfaceEnabled && parentVisible
        && ThemeBackend.blurBackground && ThemeBackend.elementOpacity < 0.999
        && width > 0 && height > 0 && hostScreen !== null
        && wallpaperPath !== "" && (hostWindow ? hostWindow.visible : true)

    property real imageX: 0
    property real imageY: 0
    property real imageScale: 1
    property real imageRotation: 0
    property rect validRect: Qt.rect(0, 0, 1, 1)
    property int revision: 0

    readonly property real screenWidth: hostScreen ? hostScreen.width : 1
    readonly property real screenHeight: hostScreen ? hostScreen.height : 1
    readonly property size textureSize: Qt.size(
        Math.max(1, Math.round(screenWidth / (liquid ? 2 : 4))),
        Math.max(1, Math.round(screenHeight / (liquid ? 2 : 4))))

    visible: active
    clip: false

    Component.onCompleted: {
        nestLevel = standalone ? 0 : Nesting.nestLevel(root, null, true);
        refreshGeometry();
    }

    function refreshTexture() {
        root.frame++;
        if (root.consumers > 0) glassTexture.scheduleUpdate();
    }

    property var geoSig: [NaN, 0, 0, 0, 0, 0, 0, 0]

    function refreshGeometry() {
        const w = root.hostWindow;
        const scr = root.hostScreen;
        if (!scr) return;
        let wx = 0;
        let wy = 0;
        if (w && w.anchors && !root.effectiveScreenOverride) {
            const a = w.anchors;
            const m = w.margins;
            if (a.left) wx = m.left;
            else if (a.right) wx = scr.width - w.width - m.right;
            else wx = (scr.width - w.width) / 2;
            if (a.top) wy = m.top;
            else if (a.bottom) wy = scr.height - w.height - m.bottom;
            else wy = (scr.height - w.height) / 2;
        }
        const p0 = root.mapFromItem(null, -wx, -wy);
        const px = root.mapFromItem(null, 1 - wx, -wy);
        const gs = root.geoSig;
        if (p0.x === gs[0] && p0.y === gs[1] && px.x === gs[2] && px.y === gs[3] && scr.width === gs[4] && scr.height === gs[5] && root.width === gs[6] && root.height === gs[7]) return;
        gs[0] = p0.x; gs[1] = p0.y; gs[2] = px.x; gs[3] = px.y; gs[4] = scr.width; gs[5] = scr.height; gs[6] = root.width; gs[7] = root.height;
        const dx = px.x - p0.x;
        const dy = px.y - p0.y;
        const len = Math.sqrt(dx * dx + dy * dy);
        if (!isFinite(len) || len <= 0) return;
        root.imageX = p0.x;
        root.imageY = p0.y;
        root.imageScale = len;
        root.imageRotation = Math.atan2(dy, dx) * 180 / Math.PI;
        const p1 = root.mapFromItem(null, scr.width - wx, scr.height - wy);
        const iw = Math.max(1, root.width);
        const ih = Math.max(1, root.height);
        const x0 = Math.max(0, Math.min(1, Math.min(p0.x, p1.x) / iw));
        const y0 = Math.max(0, Math.min(1, Math.min(p0.y, p1.y) / ih));
        const x1 = Math.max(x0, Math.min(1, Math.max(p0.x, p1.x) / iw));
        const y1 = Math.max(y0, Math.min(1, Math.max(p0.y, p1.y) / ih));
        if (x0 !== root.validRect.x || y0 !== root.validRect.y || x1 - x0 !== root.validRect.width || y1 - y0 !== root.validRect.height)
            root.validRect = Qt.rect(x0, y0, x1 - x0, y1 - y0);
        root.refreshTexture();
    }

    onXChanged: refreshGeometry()
    onYChanged: refreshGeometry()
    onWidthChanged: refreshGeometry()
    onHeightChanged: refreshGeometry()
    onHostScreenChanged: refreshGeometry()
    onWallpaperUrlChanged: refreshTexture()
    onConsumersChanged: refreshTexture()
    onLiquidChanged: refreshTexture()
    onTintChanged: refreshTexture()
    onBlurStrengthChanged: refreshTexture()
    onActiveChanged: if (active) refreshGeometry()

    FileView {
        path: root.active ? root.wallpaperPath : ""
        watchChanges: true
        printErrors: false
        onFileChanged: root.revision++
    }

    Item {
        id: mask
        anchors.fill: parent
        visible: false
        layer.enabled: true

        Rectangle {
            anchors.fill: parent
            topLeftRadius: root.topLeftRadius
            topRightRadius: root.topRightRadius
            bottomLeftRadius: root.bottomLeftRadius
            bottomRightRadius: root.bottomRightRadius
            color: "black"
        }
    }

    Item {
        id: content
        anchors.fill: parent
        anchors.margins: -root.blurPad
        visible: false
        layer.enabled: true
        layer.mipmap: root.liquid
        layer.smooth: true

        Image {
            x: root.imageX + root.blurPad
            y: root.imageY + root.blurPad
            width: root.screenWidth
            height: root.screenHeight
            scale: root.imageScale
            transformOrigin: Item.TopLeft
            rotation: root.imageRotation
            source: root.wallpaperUrl
            fillMode: Image.PreserveAspectCrop
            asynchronous: true
            cache: true
            smooth: true
            sourceSize: root.textureSize
            onStatusChanged: root.refreshTexture()
        }

        Rectangle {
            anchors.fill: parent
            visible: root.tint > 0
            color: Qt.alpha(ThemeBackend.base, root.tint)
        }
    }

    Item {
        id: rendered
        anchors.fill: parent

        MultiEffect {
            id: blurred
            x: -root.blurPad
            y: -root.blurPad
            width: root.width + 2 * root.blurPad
            height: root.height + 2 * root.blurPad
            visible: false
            source: root.liquid ? null : content
            autoPaddingEnabled: false
            blurEnabled: !root.liquid
            blurMax: 64
            blur: Math.min(1.0, root.blurStrength)
        }

        // The blur runs over a padded copy so its edges sample real wallpaper, then is cropped back.
        ShaderEffectSource {
            id: blurCrop
            anchors.fill: parent
            visible: false
            sourceItem: root.liquid ? null : blurred
            sourceRect: Qt.rect(root.blurPad, root.blurPad, root.width, root.height)
            smooth: true
        }

        MultiEffect {
            anchors.fill: parent
            visible: !root.liquid && root.maskItem !== mask
            source: visible ? blurCrop : null
            maskEnabled: true
            maskSource: root.maskItem
        }

        ShaderEffect {
            anchors.fill: parent
            visible: !root.liquid && root.maskItem === mask
            property variant src: blurCrop
            property vector2d itemSize: Qt.vector2d(width, height)
            property vector4d radii: Qt.vector4d(root.topLeftRadius, root.topRightRadius, root.bottomRightRadius, root.bottomLeftRadius)
            property real fillAlpha: Math.min(0.98, ThemeBackend.elementOpacity)
            fragmentShader: "file://" + Caching.assetsPath + "/shaders/ui/rounded_mask.frag.qsb"
        }

        LiquidGlass {
            anchors.fill: parent
            visible: root.liquid
            src: content
            topLeftRadius: root.topLeftRadius
            topRightRadius: root.topRightRadius
            bottomLeftRadius: root.bottomLeftRadius
            bottomRightRadius: root.bottomRightRadius
            tintColor: ThemeBackend.base
            tintAmount: Math.max(root.tint, 0.12) * (1 - 0.5 * root.nest)
            frost: Math.max(0.5, root.blurStrength * 2.5) * 1.08 * (1 - 0.5 * root.nest)
            rim: root.rim * (1 - 0.35 * root.nest)
            nest: root.nest
            bezel: root.bezelSize
            filletDir: root.filletDir
            filletRadius: root.filletRadius
            filletInset: root.filletInset
            earsOnly: root.earsOnly
            boxRect: root.boxRect
            validRect: root.validRect
            unpremul: 1
            clear: root.nestLevel >= 1 ? 1 : 0
            cursorPos: Qt.point(root.cursorX, root.cursorY)
            hover: root.hoverAmount
        }
    }

    ShaderEffectSource {
        id: glassTexture
        anchors.fill: parent
        sourceItem: rendered
        live: false
        mipmap: true
        visible: false
        smooth: true
        format: ShaderEffectSource.RGBA8
    }

    HoverHandler {
        id: pointer
        enabled: root.liquid && root.active
    }

    property real hoverAmount: pointer.hovered ? 1.0 : 0.0
    Behavior on hoverAmount { NumberAnimation { duration: 250; easing.type: Easing.OutCubic } }

    property real cursorX: width / 2
    property real cursorY: height / 2
    Behavior on cursorX { NumberAnimation { duration: 120; easing.type: Easing.OutCubic } }
    Behavior on cursorY { NumberAnimation { duration: 120; easing.type: Easing.OutCubic } }

    Connections {
        target: pointer
        function onPointChanged() {
            if (pointer.hovered) {
                root.cursorX = pointer.point.position.x;
                root.cursorY = pointer.point.position.y;
            }
        }
    }
}
