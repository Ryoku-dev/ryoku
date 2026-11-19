pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import "../../ask/look"

// A setting's name and value; the list owns selection and activation.
Item {
    id: root

    property real s: 1
    property var row: ({})
    property bool selected: false

    readonly property real pad: Math.round(12 * root.s)
    readonly property string kind: String(root.row.kind || "")
    readonly property bool editing: root.row.editing === true

    Line {
        id: name
        x: root.pad
        anchors.verticalCenter: parent.verticalCenter
        width: Math.min(implicitWidth, parent.width * 0.6)
        s: root.s
        size: 14
        text: String(root.row.title || "")
        ink: root.selected ? Look.plateInk : Lithic.fg
    }
    Line {
        x: name.x + name.width + Math.round(10 * root.s)
        anchors.baseline: name.baseline
        width: Math.max(0, control.x - x - Math.round(12 * root.s))
        visible: width > 40 * root.s
        s: root.s
        size: 12
        text: String(root.row.detail || "")
        ink: root.selected && Look.material ? Lithic.alpha(Look.plateInk, 0.75) : Lithic.fg3
    }

    Row {
        id: control
        x: parent.width - root.pad - width
        anchors.verticalCenter: parent.verticalCenter
        spacing: Math.round(6 * root.s)

        Symbol {
            anchors.verticalCenter: parent.verticalCenter
            visible: root.kind === "step"
            opacity: root.selected && root.row.first !== true ? 1 : 0
            s: root.s
            name: "chevron_left"
            size: 18
            ink: root.selected ? Look.plateInk : Lithic.fg2
        }
        Line {
            anchors.verticalCenter: parent.verticalCenter
            visible: root.kind !== "toggle"
            width: Math.min(implicitWidth, root.width * 0.5)
            s: root.s
            size: 13
            figures: /^[0-9]/.test(String(root.row.value || ""))
            text: String(root.row.value || "")
            ink: root.editing ? Lithic.fg3 : root.selected ? Look.plateInk : Lithic.fg
            elide: Text.ElideMiddle
        }
        Symbol {
            anchors.verticalCenter: parent.verticalCenter
            visible: root.kind === "step"
            opacity: root.selected && root.row.last !== true ? 1 : 0
            s: root.s
            name: "chevron_right"
            size: 18
            ink: root.selected ? Look.plateInk : Lithic.fg2
        }
        Symbol {
            anchors.verticalCenter: parent.verticalCenter
            visible: root.kind === "text"
            s: root.s
            name: root.editing ? "keyboard" : "edit"
            size: 15
            ink: root.editing ? Lithic.sel : Lithic.fg3
        }
        OnOff {
            anchors.verticalCenter: parent.verticalCenter
            visible: root.kind === "toggle"
            s: root.s
            on: root.row.on === true
        }
    }
}
