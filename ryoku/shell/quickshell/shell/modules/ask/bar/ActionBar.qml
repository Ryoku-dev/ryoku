pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import shell.services as ShellServices
import "../look"

// The bar's foot. Left: where you are, or for a moment what just happened (the
// HUD: "Copied", "Download started"). Right: what Enter does, and the door to
// every other action.
Item {
    id: root

    required property real s
    property string context: ""
    // a view's own item for the left of the foot, in place of the context line
    property Component contextItem: null
    // a surface's own mark in place of Rashin's
    property string glyph: ""
    property var primary: null
    property bool hasActions: false
    property bool panelOpen: false
    property string notice: ""
    property bool noticeBad: false
    readonly property real side: Math.round(14 * root.s)

    signal primaryClicked()
    signal actionsClicked()

    implicitHeight: Math.round(38 * root.s)

    function flash(text, bad) {
        root.noticeBad = bad === true;
        root.notice = String(text || "");
        if (root.notice.length > 0)
            hold.restart();
    }

    Timer {
        id: hold
        interval: 1700
        onTriggered: root.notice = ""
    }

    // Fluent's foot is a layer of its own, rounded with the frame
    Shape {
        anchors.fill: parent
        visible: Look.fluent
        s: root.s
        cutSize: Look.cutFrame
        cutTopLeft: 0
        cutTopRight: 0
        fill: Look.footer
    }

    component Door: Shape {
        id: door

        property string label: ""
        property string hint: ""
        property bool lit: false
        // Material fills the one action Enter runs as a tonal button
        property bool tonal: false
        readonly property bool filled: door.tonal && Look.material
        signal clicked()

        width: Math.ceil(doorRow.implicitWidth + 16 * door.s)
        height: Math.round(28 * door.s)
        cutSize: Look.cutRow
        fill: door.filled ? (doorTap.pressed ? Lithic.pressRow(Lithic.container) : doorHover.hovered ? Lithic.hoverFill(Lithic.container) : Lithic.container)
            : doorTap.pressed ? Look.press : doorHover.hovered || door.lit ? Look.plate : "transparent"
        Behavior on fill {
            ColorAnimation { duration: Look.dur(Look.stateMs) }
        }

        Row {
            id: doorRow
            anchors.centerIn: parent
            spacing: Math.round(8 * door.s)

            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: door.label
                color: door.filled ? Lithic.containerInk : Lithic.fg
                font.family: Look.sans
                font.pixelSize: Math.round(12 * door.s)
                font.weight: Font.Medium
                renderType: Text.NativeRendering
            }
            KeyHint {
                anchors.verticalCenter: parent.verticalCenter
                s: door.s
                hint: door.hint
                ink: door.filled ? Lithic.alpha(Lithic.containerInk, 0.8) : Lithic.fg3
            }
        }
        HoverHandler { id: doorHover; cursorShape: Qt.PointingHandCursor }
        TapHandler { id: doorTap; onTapped: door.clicked() }
    }

    Item {
        id: leftSide
        x: root.side
        width: Math.max(0, rightSide.x - root.side * 2)
        height: parent.height
        clip: true

        readonly property real shown: root.notice.length > 0 ? 1 : 0
        property real lift: leftSide.shown
        Behavior on lift {
            NumberAnimation { duration: Look.dur(leftSide.shown > 0.5 ? Look.viewMs : 140); easing.type: Easing.BezierSpline; easing.bezierCurve: leftSide.shown > 0.5 ? Look.arrive : Look.leave }
        }

        Row {
            anchors.verticalCenter: parent.verticalCenter
            anchors.verticalCenterOffset: -Math.round(8 * root.s) * leftSide.lift
            opacity: 1 - Math.min(1, leftSide.lift * 1.6)
            spacing: Math.round(8 * root.s)

            Image {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.round(14 * root.s)
                height: width
                visible: root.glyph.length === 0 && status === Image.Ready
                source: root.glyph.length === 0 ? ShellServices.Icons.path("dev.ryoku.rashin", true) : ""
                sourceSize: Qt.size(width * 2, height * 2)
                opacity: 0.86
            }
            Symbol {
                anchors.verticalCenter: parent.verticalCenter
                visible: root.glyph.length > 0
                s: root.s
                name: root.glyph
                size: 15
                ink: Lithic.fg3
            }
            Micro {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, leftSide.width - 24 * root.s)
                visible: root.contextItem === null
                s: root.s
                text: root.context.length > 0 ? root.context : root.glyph.length > 0 ? "" : I18n.tr("Rashin")
                font.features: ({ "tnum": 1 })
            }
            Loader {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.max(0, leftSide.width - 24 * root.s)
                height: leftSide.height
                active: root.contextItem !== null
                visible: active
                sourceComponent: root.contextItem
            }
        }

        Row {
            anchors.verticalCenter: parent.verticalCenter
            anchors.verticalCenterOffset: Math.round(8 * root.s) * (1 - leftSide.lift)
            opacity: Math.max(0, Math.min(1, leftSide.lift * 1.6 - 0.3))
            visible: opacity > 0.01
            spacing: Math.round(7 * root.s)

            Symbol {
                anchors.verticalCenter: parent.verticalCenter
                s: root.s
                name: root.noticeBad ? "error" : "check"
                size: 16
                ink: root.noticeBad ? Lithic.jasper : Lithic.jade
            }
            Text {
                id: noticeText
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, leftSide.width - 26 * root.s)
                color: Lithic.fg
                font.family: Look.sans
                font.pixelSize: Math.round(12 * root.s)
                font.weight: Font.Medium
                elide: Text.ElideRight
                renderType: Text.NativeRendering
                Connections {
                    target: root
                    function onNoticeChanged() {
                        if (root.notice.length > 0)
                            noticeText.text = root.notice;
                    }
                }
            }
        }
    }

    Row {
        id: rightSide
        anchors.right: parent.right
        anchors.rightMargin: Math.round(6 * root.s)
        anchors.verticalCenter: parent.verticalCenter
        spacing: Math.round(2 * root.s)

        Door {
            tonal: true
            anchors.verticalCenter: parent.verticalCenter
            s: root.s
            visible: root.primary !== null && String(root.primary.label || "").length > 0
            label: root.primary ? String(root.primary.label || "") : ""
            hint: root.primary ? String(root.primary.hint || "Enter") : ""
            onClicked: root.primaryClicked()
        }
        Rectangle {
            anchors.verticalCenter: parent.verticalCenter
            width: 1
            height: Math.round(14 * root.s)
            color: Lithic.alpha(Lithic.fg, 0.14)
            visible: root.hasActions && root.primary !== null
        }
        Door {
            anchors.verticalCenter: parent.verticalCenter
            s: root.s
            visible: root.hasActions
            label: I18n.tr("Actions")
            hint: "Ctrl+K"
            lit: root.panelOpen
            onClicked: root.actionsClicked()
        }
    }
}
