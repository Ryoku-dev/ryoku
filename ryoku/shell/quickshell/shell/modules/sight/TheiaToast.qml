pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons

Item {
    id: root
    property real s: 1
    property rect bounds: Qt.rect(0, 0, 0, 0)
    property point pointer: Qt.point(0, 0)
    property string status: "done"
    property string text: ""
    property bool undoable: false
    property bool shown: false
    property real reveal: 0
    readonly property bool presented: shown || reveal > 0
    readonly property bool working: shown && status === "working"
    readonly property real lithicScale: s
    readonly property var planes: Lithic.planes(Lithic.design)
    readonly property color plane: Lithic.design === "lithic" ? planes.pane : Lithic.design === "fluent" ? Lithic.mix(planes.card, Lithic.sel, 0.025) : Lithic.stone4
    readonly property color stroke: Lithic.design === "material" ? "transparent" : Lithic.design === "adwaita" ? Lithic.alpha(Lithic.fg, Lithic.dark ? 0.10 : 0.16) : Lithic.design === "fluent" ? planes.cardStroke : Lithic.vein
    signal cancelRequested()
    signal undoRequested()

    implicitWidth: Math.min(Math.max(32 * s, bounds.width - 16 * s), LithicTokens.size.menuWidthMax * s,
        title.implicitWidth + 43 * s + (undoable ? undoLabel.implicitWidth + 21 * s : 0))
    implicitHeight: Math.max(32 * s, title.implicitHeight + LithicTokens.space.s2 * 2 * s)
    width: Math.max(32 * s, implicitWidth)
    height: implicitHeight
    x: Math.max(bounds.x + 8 * s, Math.min(bounds.x + bounds.width - width - 8 * s, pointer.x - width / 2))
    y: Math.max(bounds.y + 8 * s, Math.min(bounds.y + bounds.height - height - 8 * s, pointer.y - height - 10 * s))
    visible: presented
    opacity: reveal
    scale: (0.96 + 0.04 * reveal) * (toastTap.pressed ? 0.98 : 1)
    Behavior on reveal { NumberAnimation { duration: Lithic.duration(root.shown ? 160 : 120); easing.type: Easing.OutCubic } }
    Behavior on scale { NumberAnimation { duration: Lithic.duration(90); easing.type: Easing.OutCubic } }
    Accessible.role: working ? Accessible.Button : Accessible.AlertMessage
    Accessible.name: text
    Accessible.description: working ? I18n.tr("Click or press Escape to cancel") : ""
    Accessible.onPressAction: activate()

    function show(label, kind, undo, point) {
        life.stop();
        if (point) pointer = point;
        text = label;
        status = kind;
        undoable = undo;
        shown = true;
        reveal = 1;
        if (kind !== "working") life.restart();
    }
    function dismiss() { life.stop(); shown = false; reveal = 0; }
    function activate() { if (working) cancelRequested(); else dismiss(); }
    Timer { id: life; interval: 4000; onTriggered: root.dismiss() }
    CutRect { anchors.fill: parent; cut: 8; fill: root.plane; strokeColor: root.stroke; strokeWidth: Lithic.design === "material" ? 0 : 1 }
    CutRect {
        anchors.fill: parent; cut: 8; fill: Lithic.alpha(Lithic.fg, 0.06)
        opacity: toastTap.containsMouse ? 1 : 0
        Behavior on opacity { NumberAnimation { duration: Lithic.duration(toastTap.containsMouse ? 160 : 100) } }
    }
    MouseArea {
        id: toastTap
        anchors.fill: parent
        anchors.rightMargin: root.undoable ? undo.width + 10 * root.s : 0
        enabled: root.shown
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        onClicked: root.activate()
    }
    Row {
        id: contents
        x: 10 * root.s
        anchors.verticalCenter: parent.verticalCenter
        spacing: 7 * root.s
        Item {
            width: 16 * root.s; height: 16 * root.s
            anchors.verticalCenter: parent.verticalCenter
            Loader {
                anchors.centerIn: parent
                active: root.working
                sourceComponent: Component { Working { active: root.working; size: 14 * root.s } }
            }
            Glyph {
                anchors.centerIn: parent
                visible: !root.working
                name: root.status === "error" ? "error_outline" : "check"
                size: 16
                ink: root.status === "error" ? Lithic.jasper : Lithic.fg2
                scale: root.status === "done" ? 1.05 : 1
                Behavior on scale { NumberAnimation { duration: Lithic.duration(120) } }
            }
        }
        Label {
            id: title
            width: Math.min(implicitWidth, Math.max(1, root.width - 43 * root.s - (root.undoable ? undo.width + contents.spacing : 0)))
            text: root.text; typeRole: "label"; ink: Lithic.fg
            wrapMode: Text.Wrap
            elide: Text.ElideNone
            anchors.verticalCenter: parent.verticalCenter
        }
        T.AbstractButton {
            id: undo
            visible: root.undoable && root.shown
            text: I18n.tr("Undo")
            implicitWidth: undoLabel.implicitWidth + 14 * root.s
            width: visible ? implicitWidth : 0
            height: 26 * root.s
            anchors.verticalCenter: parent.verticalCenter
            hoverEnabled: true
            focusPolicy: Qt.StrongFocus
            Accessible.role: Accessible.Button
            Accessible.name: text
            Keys.onReturnPressed: if (enabled) clicked()
            Keys.onEnterPressed: if (enabled) clicked()
            onClicked: root.undoRequested()
            scale: down ? 0.97 : 1
            Behavior on scale { NumberAnimation { duration: Lithic.duration(90) } }
            background: Item {
                CutRect {
                    anchors.fill: parent; cut: 6; fill: Lithic.alpha(Lithic.gold, 0.12)
                    opacity: undo.hovered || undo.down ? 1 : 0
                    Behavior on opacity { NumberAnimation { duration: Lithic.duration(undo.hovered ? 160 : 100) } }
                }
                FocusRing { anchors.fill: parent; cut: 6; keyboardFocusVisible: undo.visualFocus }
            }
            contentItem: Label {
                id: undoLabel
                text: undo.text; typeRole: "label"; ink: Lithic.gold
                horizontalAlignment: Text.AlignHCenter
                verticalAlignment: Text.AlignVCenter
            }
            HoverHandler { cursorShape: Qt.PointingHandCursor }
        }
    }
}
