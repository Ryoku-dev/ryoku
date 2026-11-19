import QtQuick
import QtQuick.Shapes
import Lithic
import Ryoku.Ui.Singletons
import "../look"
import ".."

// The dock's way into Beautify, its strongest secondary button: Lithic's quiet
// plaque, Material's tonal button, Adwaita's flat one, Fluent's standard one.
// Until Beautify has been opened once, the dock's arrival brings one restrained
// shine across it and a three second tip saying what it does.
Item {
    id: root

    property real u: 1
    property real cellHeight: 32
    property bool tipBelow: false
    // Beautify has never been opened
    property bool firstUse: false
    // the dock has arrived and stands still
    property bool present: false

    signal clicked()

    implicitWidth: button.implicitWidth
    implicitHeight: button.implicitHeight
    width: implicitWidth
    height: implicitHeight

    TextButton {
        id: button
        u: root.u
        cellHeight: root.cellHeight
        icon: Look.lithic ? "" : "beautify"
        text: I18n.tr("Beautify")
        keyHint: Look.keyHint("Ctrl+B")
        tipBelow: root.tipBelow
        onClicked: root.clicked()
    }

    property real shine: -0.3
    SequentialAnimation {
        id: shineRun
        PropertyAction { target: root; property: "shine"; value: -0.3 }
        PauseAnimation { duration: Look.enterMs + 120 }
        ScriptAction { script: hint.shown = true }
        NumberAnimation { target: root; property: "shine"; from: -0.3; to: 1.3; duration: 720; easing.type: Easing.OutCubic }
        PauseAnimation { duration: 3000 - 720 }
        ScriptAction { script: hint.shown = false }
    }
    readonly property bool introduce: root.firstUse && root.present
    onIntroduceChanged: {
        if (root.introduce) {
            if (Look.reduceMotion) {
                hint.shown = true;
                hintOff.restart();
            } else {
                shineRun.restart();
            }
        } else {
            shineRun.stop();
            hintOff.stop();
            root.shine = -0.3;
            hint.shown = false;
        }
    }
    Timer { id: hintOff; interval: 3000; onTriggered: hint.shown = false }

    // the band, kept inside the button's own outline
    readonly property real r: Look.lithic ? Look.cutSm * root.u
        : Look.material ? root.height / 2 : (Look.adwaita ? 6 : 4) * root.u
    Shape {
        anchors.fill: parent
        visible: root.shine > -0.25 && root.shine < 1.25
        preferredRendererType: Shape.CurveRenderer
        ShapePath {
            strokeColor: "transparent"
            fillGradient: LinearGradient {
                x1: root.shine * root.width - 22 * root.u
                y1: 0
                x2: root.shine * root.width + 22 * root.u
                y2: root.height
                GradientStop { position: 0; color: "transparent" }
                GradientStop { position: 0.5; color: Look.dark ? Lithic.alpha(Look.fg, 0.14) : Lithic.alpha(Look.stone4, 0.9) }
                GradientStop { position: 1; color: "transparent" }
            }
            PathSvg {
                readonly property real w: root.width
                readonly property real h: root.height
                readonly property real c: Math.min(root.r, w / 2, h / 2)
                path: Look.lithic
                    ? "M " + c + " 0 L " + (w - c) + " 0 L " + w + " " + c + " L " + w + " " + (h - c)
                        + " L " + (w - c) + " " + h + " L " + c + " " + h + " L 0 " + (h - c) + " L 0 " + c + " Z"
                    : "M " + c + " 0 L " + (w - c) + " 0 A " + c + " " + c + " 0 0 1 " + w + " " + c
                        + " L " + w + " " + (h - c) + " A " + c + " " + c + " 0 0 1 " + (w - c) + " " + h
                        + " L " + c + " " + h + " A " + c + " " + c + " 0 0 1 0 " + (h - c)
                        + " L 0 " + c + " A " + c + " " + c + " 0 0 1 " + c + " 0 Z"
            }
        }
    }

    Tip {
        id: hint
        z: 100
        u: root.u
        text: I18n.tr("Add a background, frame and shadow")
        anchors.horizontalCenter: parent.horizontalCenter
        y: root.tipBelow ? root.height + 6 * root.u : -height - 6 * root.u
    }
}
