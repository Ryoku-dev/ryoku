import QtQuick
import Lithic
import ".."

// One row of a Menu: an action, a check or radio row, a heading or a separator.
// Lithic's rows are tight with a mono key hint, Material's are 48 tall with a
// leading icon, Adwaita's popover rows round by 6, Fluent's MenuFlyoutItem by 4.
// A triggered row asks its menu to close unless keepOpen.
Item {
    id: root

    property real u: 1
    // "item", "heading" or "separator"
    property string kind: "item"
    property string icon: ""
    property string text: ""
    property string keyHint: ""
    // "", "check" or "radio"
    property string checkable: ""
    property bool checked: false
    property bool keepOpen: false

    signal triggered()

    readonly property bool hovered: area.containsMouse
    readonly property real rowHeight: Look.material ? 44 : Look.lithic ? 28 : 32
    readonly property real padX: (Look.material ? 12 : 10) * root.u
    readonly property bool hasLead: root.icon.length > 0 || root.checkable.length > 0

    implicitHeight: root.kind === "separator" ? Math.round(9 * root.u)
        : root.kind === "heading" ? Math.round((Look.lithic ? 24 : 30) * root.u)
        : Math.round(root.rowHeight * root.u)
    implicitWidth: root.kind === "separator" ? 0
        : Math.ceil(root.padX * 2 + (root.hasLead ? lead.width + 10 * root.u : 0)
            + label.implicitWidth + (hint.visible ? hint.implicitWidth + 20 * root.u : 0))
    width: parent ? parent.width : implicitWidth
    opacity: root.enabled ? 1 : Lithic.disabledOpacity

    function _menu() {
        for (let p = root.parent; p; p = p.parent) {
            if (p.lookMenu === true)
                return p;
        }
        return null;
    }

    Rectangle {
        visible: root.kind === "separator"
        anchors.verticalCenter: parent.verticalCenter
        x: Look.lithic ? 0 : 4 * root.u
        width: parent.width - 2 * x
        height: Math.max(1, Math.round(root.u))
        color: Look.divider
    }

    // the hover plate
    CutRect {
        visible: Look.lithic && root.kind === "item"
        anchors.fill: parent
        design: "lithic"
        cut: Look.cutSm * root.u
        fill: area.pressed ? Lithic.pressRow(Look.plane) : root.hovered ? Lithic.hoverPlane(Look.plane) : Lithic.alpha(Look.plane, 0)
    }
    Rectangle {
        visible: !Look.lithic && root.kind === "item"
        anchors.fill: parent
        radius: Look.material ? 0 : (Look.adwaita ? 6 : 4) * root.u
        color: area.pressed ? Look.press : root.hovered ? Look.hover : "transparent"
        Behavior on color {
            ColorAnimation { duration: Look.dur(Look.stateMs) }
        }
    }

    Item {
        id: lead
        visible: root.kind === "item" && root.hasLead
        x: root.padX
        anchors.verticalCenter: parent.verticalCenter
        width: Math.round((Look.material ? 20 : 16) * root.u)
        height: width

        Icon {
            visible: root.checkable === "" && root.icon.length > 0
            anchors.fill: parent
            name: root.icon
            size: parent.width
            tint: Look.fg2
        }
        Icon {
            visible: root.checkable === "check" && root.checked
            anchors.fill: parent
            name: "check"
            size: parent.width
            tint: Look.lithic || Look.material ? Look.sel : Look.fg
        }
        // a radio: Lithic a cut diamond, the round designs a ring and dot
        CutRect {
            visible: Look.lithic && root.checkable === "radio"
            anchors.centerIn: parent
            width: Math.round(9 * root.u)
            height: width
            design: "lithic"
            cut: width / 2
            fill: root.checked ? Look.sel : "transparent"
            strokeColor: root.checked ? Look.sel : Look.fg3
            strokeWidth: Math.max(1, Math.round(root.u))
        }
        Rectangle {
            visible: !Look.lithic && root.checkable === "radio"
            anchors.centerIn: parent
            width: Math.round((Look.material ? 18 : 14) * root.u)
            height: width
            radius: width / 2
            color: "transparent"
            border.width: Math.max(1, Math.round((Look.material ? 2 : 1) * root.u))
            border.color: root.checked ? Look.sel : Look.fg2
            Rectangle {
                anchors.centerIn: parent
                width: parent.width * (root.checked ? 0.5 : 0)
                height: width
                radius: width / 2
                color: Look.sel
                Behavior on width {
                    NumberAnimation { duration: Look.dur(140); easing.type: Easing.OutCubic }
                }
            }
        }
    }

    Text {
        id: label
        visible: root.kind !== "separator"
        x: root.kind === "item" && root.hasLead ? lead.x + lead.width + 10 * root.u : root.padX
        anchors.verticalCenter: parent.verticalCenter
        anchors.verticalCenterOffset: root.kind === "heading" ? 2 * root.u : 0
        width: Math.min(implicitWidth, root.width - x - root.padX - (hint.visible ? hint.implicitWidth + 16 * root.u : 0))
        elide: Text.ElideRight
        text: root.kind === "heading" && Look.lithic ? root.text.toUpperCase() : root.text
        color: root.kind === "heading" ? (Look.lithic ? Look.fg3 : Look.fg2) : Look.fg
        font.family: root.kind === "heading" && Look.lithic ? Look.mono : Look.sans
        font.pixelSize: Math.round((root.kind === "heading" ? (Look.lithic ? 9 : 12) : Look.material ? 14 : 13) * root.u)
        font.weight: root.kind === "heading" ? (Look.lithic ? Font.Bold : Font.DemiBold) : Font.Normal
        font.letterSpacing: root.kind === "heading" && Look.lithic ? Look.tracking * root.u : 0
        renderType: Text.CurveRendering
    }

    Text {
        id: hint
        visible: root.kind === "item" && root.keyHint.length > 0
        anchors.right: parent.right
        anchors.rightMargin: root.padX
        anchors.verticalCenter: parent.verticalCenter
        text: root.keyHint
        color: Look.fg3
        font.family: Look.lithic ? Look.mono : Look.sans
        font.pixelSize: Math.round((Look.lithic ? 9.5 : 12) * root.u)
        renderType: Text.CurveRendering
    }

    MouseArea {
        id: area
        visible: root.kind === "item"
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        onClicked: {
            root.triggered();
            if (!root.keepOpen) {
                const m = root._menu();
                if (m)
                    m.dismiss();
            }
        }
    }
}
