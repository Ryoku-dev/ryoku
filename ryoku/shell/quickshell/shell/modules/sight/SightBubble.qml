pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons

Item {
    id: root
    property real s: 1
    property rect bounds: Qt.rect(0, 0, 0, 0)
    property var context: null
    property point pointer: Qt.point(0, 0)
    property bool shown: false
    property real reveal: 0
    readonly property bool presented: shown || reveal > 0
    readonly property bool above: pointer.y - 10 * s - height >= bounds.y + 8 * s
    readonly property var planes: Lithic.planes(Lithic.design)
    readonly property color plane: Lithic.design === "lithic" ? planes.pane : Lithic.design === "fluent" ? Lithic.mix(planes.card, Lithic.sel, 0.025) : Lithic.stone4
    readonly property color stroke: Lithic.design === "material" ? "transparent" : Lithic.design === "adwaita" ? Lithic.alpha(Lithic.fg, Lithic.dark ? 0.10 : 0.16) : Lithic.design === "fluent" ? planes.cardStroke : Lithic.vein
    readonly property real lithicScale: s
    readonly property var modes: context && context.language && String(context.language) === String(I18n.lang)
        ? ["explain", "translate", "ask"] : ["translate", "explain", "ask"]
    signal actionRequested(string mode)
    signal dismissRequested()

    implicitWidth: actions.implicitWidth
    implicitHeight: 32 * s
    width: implicitWidth
    height: implicitHeight
    x: Math.max(bounds.x + 8 * s, Math.min(bounds.x + bounds.width - width - 8 * s, pointer.x - width / 2))
    y: Math.max(bounds.y + 8 * s, Math.min(bounds.y + bounds.height - height - 8 * s, above ? pointer.y - height - 10 * s : pointer.y + 18 * s))
    visible: presented
    opacity: reveal
    scale: 0.94 + 0.06 * reveal
    transformOrigin: above ? Item.Bottom : Item.Top

    function showAt(data, point) {
        enterMotion.stop(); exitMotion.stop(); life.stop();
        context = data;
        pointer = point;
        reveal = 0;
        shown = true;
        life.interval = 6000;
        life.start();
        Qt.callLater(function() { if (root.shown) enterMotion.restart(); });
    }
    function dismissBubble(immediate) {
        life.stop(); enterMotion.stop(); exitMotion.stop();
        shown = false;
        if (immediate || Lithic.reduceMotion) reveal = 0;
        else exitMotion.restart();
    }
    function choose(mode) {
        if (shown) actionRequested(mode);
    }
    NumberAnimation {
        id: enterMotion
        target: root; property: "reveal"; to: 1
        duration: Lithic.duration(140)
        easing.type: Easing.BezierSpline
        easing.bezierCurve: [0,0,0.2,1,1,1]
    }
    NumberAnimation {
        id: exitMotion
        target: root; property: "reveal"; to: 0
        duration: Lithic.duration(100)
        easing.type: Easing.BezierSpline
        easing.bezierCurve: [0.3,0,0.8,0.15,1,1]
    }
    Connections {
        target: Lithic
        function onReduceMotionChanged() {
            if (!Lithic.reduceMotion) return;
            enterMotion.stop(); exitMotion.stop();
            root.reveal = root.shown ? 1 : 0;
        }
    }
    Timer { id: life; onTriggered: root.dismissRequested() }
    HoverHandler {
        id: bubbleHover
        enabled: root.shown
        onHoveredChanged: {
            if (!root.shown) return;
            if (hovered) life.stop();
            else { life.interval = 2000; life.restart(); }
        }
    }
    CutRect { anchors.fill: parent; cut: 8; fill: root.plane; strokeColor: root.stroke; strokeWidth: Lithic.design === "material" ? 0 : 1 }
    Item {
        id: grain
        anchors.fill: parent
        visible: Lithic.design === "fluent"
        readonly property real radius: Lithic.cornerRadius(8 * root.s, width, height, "fluent")
        opacity: 0.55
        Image { x: grain.radius; y: 1; width: Math.max(0, grain.width - grain.radius * 2); height: Math.max(0, grain.radius - 1); source: "../notifications/designs/fluent-noise.png"; fillMode: Image.Tile; smooth: false }
        Image { x: 1; y: grain.radius; width: Math.max(0, grain.width - 2); height: Math.max(0, grain.height - grain.radius * 2); source: "../notifications/designs/fluent-noise.png"; fillMode: Image.Tile; smooth: false }
        Image { x: grain.radius; y: grain.height - grain.radius; width: Math.max(0, grain.width - grain.radius * 2); height: Math.max(0, grain.radius - 1); source: "../notifications/designs/fluent-noise.png"; fillMode: Image.Tile; smooth: false }
        CutRect { anchors.fill: parent; anchors.margins: 1; cut: 7; fill: "transparent"; strokeColor: Lithic.alpha(Lithic.fg, 0.04); strokeWidth: 1 }
    }
    Row {
        id: actions
        Repeater {
            model: root.modes
            delegate: Item {
                id: action
                required property string modelData
                required property int index
                readonly property string label: I18n.tr(modelData === "translate" ? "Translate" : modelData === "explain" ? "Explain" : "Ask")
                width: title.implicitWidth + 20 * root.s + (index > 0 ? 1 : 0)
                height: root.height
                Accessible.role: Accessible.Button
                Accessible.name: label
                Accessible.onPressAction: root.choose(modelData)
                Item {
                    anchors.fill: parent
                    anchors.leftMargin: action.index > 0 ? 1 : 0
                    scale: tap.pressed ? 0.97 : 1
                    Behavior on scale { NumberAnimation { duration: Lithic.duration(90) } }
                    CutRect {
                        anchors.fill: parent; cut: 8; fill: Lithic.alpha(Lithic.fg, 0.08)
                        opacity: hover.hovered ? 1 : 0
                        Behavior on opacity { NumberAnimation { duration: Lithic.duration(hover.hovered ? 160 : 100) } }
                    }
                    Label { id: title; anchors.centerIn: parent; text: action.label; typeRole: "label"; ink: action.index === 0 ? Lithic.fg : Lithic.fg2 }
                    HoverHandler { id: hover; enabled: root.shown; cursorShape: Qt.PointingHandCursor }
                    TapHandler { id: tap; enabled: root.shown; onTapped: root.choose(action.modelData) }
                }
                CutRect { visible: action.index > 0; x: 0; y: (action.height - 16 * root.s) / 2; width: 1; height: 16 * root.s; cut: 0; fill: Lithic.vein }
            }
        }
    }
}
