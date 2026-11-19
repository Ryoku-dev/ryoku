pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../ask/look"

// Kit's foot, in the bar's grammar. Left: the section's context line (or its own
// item), or for a moment what just happened. Right: what Enter does, the second
// action, and the door to every other one. Sizes: kit-layouts SPEC 3.
Item {
    id: root

    required property real s
    property bool active: false
    property string context: ""
    property Component contextItem: null
    // {label, hint}
    property var primary: null
    property var secondary: null
    property bool hasActions: false
    property bool panelOpen: false
    property string notice: ""
    property bool noticeBad: false
    readonly property real side: Math.round(16 * root.s)

    signal primaryClicked()
    signal secondaryClicked()
    signal actionsClicked()

    implicitHeight: Math.round(40 * root.s)

    function flash(text, bad) {
        if (!root.active)
            return;
        root.noticeBad = bad === true;
        root.notice = String(text || "");
        if (root.notice.length > 0)
            hold.restart();
    }

    function reset() {
        hold.stop();
        root.notice = "";
    }

    Timer {
        id: hold
        interval: 1700
        onTriggered: root.notice = ""
    }

    Shape {
        x: 1
        width: parent.width - 2
        height: parent.height - 1
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
        property bool strong: false
        readonly property bool filled: door.strong && Look.material
        signal clicked()

        width: Math.ceil(doorRow.implicitWidth + 16 * door.s)
        height: Math.round(28 * door.s)
        cutSize: Look.cutRow
        fill: door.filled ? (doorTap.pressed ? Lithic.pressRow(Lithic.container) : doorHover.hovered ? Lithic.hoverFill(Lithic.container) : Lithic.container)
            : doorTap.pressed ? Look.press : doorHover.hovered || door.lit ? Look.plate : "transparent"
        Behavior on fill {
            enabled: root.active
            ColorAnimation { duration: Look.dur(Look.stateMs) }
        }

        Row {
            id: doorRow
            anchors.centerIn: parent
            spacing: Math.round(8 * door.s)

            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: door.label
                color: door.filled ? Lithic.containerInk : door.strong ? Lithic.fg : Lithic.fg2
                font.family: Look.sans
                font.pixelSize: Math.round(12.5 * door.s)
                font.weight: door.strong ? Font.Medium : Font.Normal
                renderType: Text.NativeRendering
            }
            KeyHint {
                anchors.verticalCenter: parent.verticalCenter
                visible: door.hint.length > 0
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
            enabled: root.active
            NumberAnimation { duration: Look.dur(leftSide.shown > 0.5 ? Look.viewMs : 140); easing.type: Easing.BezierSpline; easing.bezierCurve: leftSide.shown > 0.5 ? Look.arrive : Look.leave }
        }

        Item {
            width: parent.width
            height: parent.height
            y: -Math.round(8 * root.s) * leftSide.lift
            opacity: 1 - Math.min(1, leftSide.lift * 1.6)

            Text {
                anchors.verticalCenter: parent.verticalCenter
                width: parent.width
                visible: root.contextItem === null
                text: root.context
                color: Lithic.fg2
                font.family: Look.sans
                font.pixelSize: Math.round(12.5 * root.s)
                font.features: ({ "tnum": 1 })
                elide: Text.ElideRight
                renderType: Text.NativeRendering
            }
            Loader {
                anchors.verticalCenter: parent.verticalCenter
                width: parent.width
                height: parent.height
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
                font.pixelSize: Math.round(12.5 * root.s)
                font.weight: Font.Medium
                elide: Text.ElideRight
                renderType: Text.NativeRendering
                // the words stay while the line folds away
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
        anchors.rightMargin: Math.round(8 * root.s)
        anchors.verticalCenter: parent.verticalCenter
        spacing: Math.round(2 * root.s)

        Door {
            strong: true
            anchors.verticalCenter: parent.verticalCenter
            s: root.s
            visible: root.primary !== null && String(root.primary.label || "").length > 0
            label: root.primary ? String(root.primary.label || "") : ""
            hint: root.primary ? String(root.primary.hint || "Enter") : ""
            onClicked: root.primaryClicked()
        }
        Door {
            anchors.verticalCenter: parent.verticalCenter
            s: root.s
            visible: root.secondary !== null && String(root.secondary.label || "").length > 0
            label: root.secondary ? String(root.secondary.label || "") : ""
            hint: root.secondary ? String(root.secondary.hint || "Ctrl+Enter") : ""
            onClicked: root.secondaryClicked()
        }
        Item {
            anchors.verticalCenter: parent.verticalCenter
            width: Math.round(9 * root.s)
            height: Math.round(18 * root.s)
            visible: root.hasActions && (root.primary !== null || root.secondary !== null)

            Rectangle {
                anchors.centerIn: parent
                width: 1
                height: parent.height
                color: Lithic.alpha(Lithic.fg, Look.dark ? 0.16 : 0.2)
            }
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
