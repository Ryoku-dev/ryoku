import QtQuick
import QtQuick.Effects
import Quickshell
import "../look"
import "../lib/beautify.js" as B

// The one renderer of a styled shot, at its natural size: the background, the
// shot rounded by a mask with its colour adjustments, its border, its
// directional shadow and the watermark. The stage shows it scaled, the export
// grabs it, the looks draw it small. Scaling it is the caller's business.
Item {
    id: comp

    property url source: ""
    property var style: ({})
    // a thumbnail: decode small and in the background
    property bool preview: false
    // draw the shot at this size; px-valued style values follow it, measured
    // against basis (the shot's own width) when that is given
    property size natural: Qt.size(0, 0)
    property real basis: 0
    property string mark: "力"
    // the stage: a new background crossfades over the old one
    property bool animated: false
    // the lift: how far the background has bloomed out
    property real bloom: 1

    readonly property var s: B.complete(comp.style)
    readonly property bool sized: comp.natural.width > 0 && comp.natural.height > 0
    readonly property real k: comp.sized && comp.basis > 0 ? comp.natural.width / comp.basis : 1
    readonly property real srcW: comp.sized ? comp.natural.width : (img.implicitWidth > 0 ? img.implicitWidth : 800)
    readonly property real srcH: comp.sized ? comp.natural.height : (img.implicitHeight > 0 ? img.implicitHeight : 500)
    readonly property real pad: Math.max(0, comp.s.padding) * comp.k
    readonly property var size: B.frame(comp.srcW, comp.srcH, comp.pad, comp.s.ratioKey)
    readonly property real fullW: comp.size.w
    readonly property real fullH: comp.size.h
    readonly property real radius: Math.max(0, comp.s.roundness) * comp.k
    readonly property var bg: B.background(comp.s)
    // layers at the shot's own px, not the window's scale of them
    readonly property size texture: Qt.size(Math.ceil(comp.srcW * (comp.preview ? 2 : 1)), Math.ceil(comp.srcH * (comp.preview ? 2 : 1)))
    readonly property bool ready: img.status === Image.Ready
        && (comp.bg.kind !== "image" || comp.bg.image === "" || front.imageReady)

    implicitWidth: comp.fullW
    implicitHeight: comp.fullH
    width: comp.fullW
    height: comp.fullH

    readonly property string userName: Quickshell.env("USER") || "user"

    // two background plates: the front shows the current background; on the
    // stage a new one fades in over the old, which then hides (to nothing, the
    // old one fades out instead)
    property int frontSlot: 0
    readonly property Item front: comp.frontSlot === 0 ? slotA : slotB
    readonly property Item spare: comp.frontSlot === 0 ? slotB : slotA
    readonly property string bgKey: JSON.stringify(comp.bg)
    onBgKeyChanged: {
        if (fade.running) {
            fade.stop();
            fade.settle();
        }
        if (!comp.animated || Look.reduceMotion) {
            comp.front.assign(comp.bg);
            comp.front.opacity = 1;
            comp.spare.opacity = 0;
            return;
        }
        var old = comp.front, next = comp.spare;
        next.assign(comp.bg);
        if (comp.bg.kind === "none") {
            next.z = 0;
            old.z = 1;
            next.opacity = 1;
            fade.target = old;
            fade.from = old.opacity;
            fade.to = 0;
            fade.after = null;
        } else {
            next.z = 1;
            old.z = 0;
            next.opacity = 0;
            fade.target = next;
            fade.from = 0;
            fade.to = 1;
            fade.after = old;
        }
        comp.frontSlot = comp.frontSlot === 0 ? 1 : 0;
        fade.restart();
    }
    Component.onCompleted: comp.front.assign(comp.bg)
    NumberAnimation {
        id: fade
        property Item after: null
        property: "opacity"
        duration: 200
        easing.type: Easing.OutCubic
        function settle() {
            if (fade.target) fade.target.opacity = fade.to;
            if (fade.after) fade.after.opacity = 0;
            fade.after = null;
        }
        onFinished: fade.settle()
    }

    Item {
        anchors.fill: parent
        clip: true
        opacity: Math.max(0, Math.min(1, comp.bloom))
        Plate { id: slotA }
        Plate { id: slotB; opacity: 0 }
    }

    Item {
        id: card
        anchors.centerIn: parent
        width: comp.srcW
        height: comp.srcH

        // cast from the rounded card, offset along the light's direction
        RectangularShadow {
            visible: comp.s.shadowStrength > 0
            anchors.fill: parent
            offset.x: Math.cos(comp.s.shadowAngle * Math.PI / 180) * comp.s.shadowDist * comp.k
            offset.y: Math.sin(comp.s.shadowAngle * Math.PI / 180) * comp.s.shadowDist * comp.k
            radius: comp.radius
            blur: Math.max(0, comp.s.shadowBlur) * 0.8 * comp.k
            color: Qt.rgba(0, 0, 0, Math.max(0, Math.min(1, comp.s.shadowStrength / 100)))
        }

        Image {
            id: img
            anchors.fill: parent
            source: comp.source
            cache: false
            asynchronous: comp.preview
            fillMode: Image.Stretch
            smooth: true
            mipmap: !comp.preview
            sourceSize: comp.sized ? Qt.size(Math.ceil(comp.natural.width * 2), Math.ceil(comp.natural.height * 2)) : undefined
            readonly property bool adjusted: comp.s.adjBright !== 0 || comp.s.adjContrast !== 0 || comp.s.adjSat !== 0
            layer.enabled: comp.radius > 0.01 || img.adjusted
            layer.smooth: true
            layer.textureSize: comp.texture
            layer.effect: MultiEffect {
                maskEnabled: comp.radius > 0.01
                maskSource: maskRect
                maskThresholdMin: 0.5
                maskSpreadAtMin: 1.0
                brightness: comp.s.adjBright / 100
                contrast: comp.s.adjContrast / 100
                saturation: comp.s.adjSat / 100
            }
        }
        Rectangle {
            id: maskRect
            anchors.fill: parent
            radius: comp.radius
            visible: false
            layer.enabled: true
            layer.smooth: true
            layer.textureSize: comp.texture
            antialiasing: true
        }
        Rectangle {
            anchors.fill: parent
            visible: comp.s.borderW > 0
            color: "transparent"
            radius: comp.radius
            border.width: comp.s.borderW * comp.k
            border.color: comp.s.borderColor
            antialiasing: true
        }
    }

    Row {
        id: wmark
        visible: comp.s.watermark
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.margins: Math.max(16 * comp.k, comp.pad * 0.32)
        spacing: wmark.fs * 0.55
        readonly property real fs: Math.max(15 * comp.k, comp.fullW * 0.016)

        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: comp.mark
            color: Look.sel
            font.family: Look.jp
            font.pixelSize: Math.max(1, wmark.fs * 1.4)
            font.weight: Font.Bold
            style: Text.Raised
            styleColor: Qt.rgba(0, 0, 0, 0.5)
        }
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: "<b>" + comp.userName + "</b><font color=\"#f5b53f\">@RyokuArch</font>"
            textFormat: Text.StyledText
            color: "#ffffff"
            font.family: Look.sans
            font.pixelSize: Math.max(1, wmark.fs)
            font.weight: Font.DemiBold
            style: Text.Raised
            styleColor: Qt.rgba(0, 0, 0, 0.45)
        }
    }

    component Plate: Item {
        id: plate
        anchors.fill: parent
        property string kind: "none"
        property color a: "black"
        property color b: "black"
        property real angle: 135
        property color solid: "black"
        property string image: ""
        readonly property bool imageReady: pic.status === Image.Ready
        function assign(r) {
            plate.kind = r.kind;
            if (r.a) plate.a = r.a;
            if (r.b) plate.b = r.b;
            plate.angle = r.angle;
            if (r.solid) plate.solid = r.solid;
            plate.image = r.image;
        }
        Rectangle {
            visible: plate.kind === "gradient"
            anchors.centerIn: parent
            width: Math.max(parent.width, parent.height) * 1.5
            height: width
            rotation: plate.angle - 90
            gradient: Gradient {
                GradientStop { position: 0.0; color: plate.a }
                GradientStop { position: 1.0; color: plate.b }
            }
        }
        Rectangle {
            visible: plate.kind === "solid"
            anchors.fill: parent
            color: plate.solid
        }
        Image {
            id: pic
            visible: plate.kind === "image"
            anchors.fill: parent
            source: plate.kind === "image" && plate.image !== "" ? "file://" + plate.image : ""
            fillMode: Image.PreserveAspectCrop
            asynchronous: comp.preview
            cache: false
            sourceSize: comp.preview ? Qt.size(Math.ceil(comp.fullW * 2), Math.ceil(comp.fullH * 2)) : undefined
        }
    }
}
