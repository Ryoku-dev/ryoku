pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons

Item {
    id: root
    property real s: 1
    property bool hasPrevious: false
    property bool busy: false
    property string errorText: ""
    readonly property var planes: Lithic.planes(Lithic.design)
    implicitWidth: row.implicitWidth + 2 * 13 * s
    implicitHeight: 40 * s
    width: implicitWidth
    height: implicitHeight
    CutRect {
        anchors.fill: parent
        cut: 8
        fill: Lithic.design === "lithic" ? root.planes.pane : Lithic.stone4
        strokeColor: Lithic.design === "material" ? "transparent" : Lithic.vein
        strokeWidth: Lithic.design === "material" ? 0 : 1
    }
    MouseArea { anchors.fill: parent; acceptedButtons: Qt.AllButtons; cursorShape: Qt.ArrowCursor }
    Row {
        id: row
        anchors.centerIn: parent
        spacing: 21 * root.s
        Label {
            id: message
            anchors.verticalCenter: parent.verticalCenter
            typeRole: "label"
            text: root.errorText || (root.busy ? I18n.tr("Opening") : I18n.tr("Drag over anything"))
            ink: root.errorText ? Lithic.jasper : Lithic.fg
        }
        SightKeycap { anchors.verticalCenter: parent.verticalCenter; s: root.s; key: I18n.tr("Enter"); label: I18n.tr("Last box"); visible: root.hasPrevious }
        SightKeycap { anchors.verticalCenter: parent.verticalCenter; s: root.s; key: I18n.tr("Esc"); label: I18n.tr("Cancel") }
    }
}
