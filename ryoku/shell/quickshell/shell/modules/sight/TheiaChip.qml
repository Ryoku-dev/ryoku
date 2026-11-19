pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Templates as T
import Lithic

FocusScope {
    id: root
    property string text: ""
    property string icon: ""
    property bool selected: false
    property bool small: false
    property real s: 1
    readonly property real lithicScale: s
    readonly property var planes: Lithic.planes(Lithic.design)
    readonly property color plane: Lithic.design === "lithic" ? planes.pane : Lithic.design === "fluent" ? Lithic.mix(planes.card, Lithic.sel, 0.025) : Lithic.stone4
    readonly property color stroke: Lithic.design === "material" ? "transparent" : Lithic.design === "adwaita" ? Lithic.alpha(Lithic.fg, Lithic.dark ? 0.10 : 0.16) : Lithic.design === "fluent" ? planes.cardStroke : Lithic.vein
    signal clicked()

    implicitWidth: button.implicitWidth
    implicitHeight: button.implicitHeight
    opacity: enabled ? 1 : Lithic.disabledOpacity

    T.AbstractButton {
        id: button
        anchors.fill: parent
        focus: true
        text: root.text
        hoverEnabled: true
        focusPolicy: Qt.StrongFocus
        implicitWidth: content.implicitWidth + (root.small ? 18 : 24) * root.s
        implicitHeight: (root.small ? 28 : 32) * root.s
        leftPadding: (root.small ? 9 : 12) * root.s
        rightPadding: leftPadding
        scale: down ? 0.98 : 1
        Behavior on scale { NumberAnimation { duration: Lithic.duration(90); easing.type: Easing.OutCubic } }
        Accessible.role: Accessible.Button
        Accessible.name: root.text
        Accessible.checkable: true
        Accessible.checked: root.selected
        Accessible.onPressAction: if (root.enabled) root.clicked()
        Keys.onReturnPressed: if (root.enabled) root.clicked()
        Keys.onEnterPressed: if (root.enabled) root.clicked()
        onClicked: root.clicked()
        background: Item {
            CutRect {
                anchors.fill: parent
                cut: 6
                fill: root.selected ? Lithic.container : root.plane
                strokeColor: root.selected ? Lithic.sel : root.stroke
                strokeWidth: root.selected || Lithic.design !== "material" ? LithicTokens.size.hairline : 0
                Behavior on fill { ColorAnimation { duration: Lithic.duration(120) } }
                Behavior on strokeColor { ColorAnimation { duration: Lithic.duration(120) } }
            }
            CutRect {
                anchors.fill: parent
                cut: 6
                fill: Lithic.alpha(Lithic.fg, button.down ? 0.12 : 0.08)
                opacity: button.hovered || button.down ? 1 : 0
                Behavior on opacity { NumberAnimation { duration: Lithic.duration(button.hovered ? 140 : 100) } }
            }
            FocusRing { anchors.fill: parent; cut: 6; keyboardFocusVisible: button.visualFocus }
        }
        contentItem: Row {
            id: content
            spacing: 6 * root.s
            Glyph {
                name: "check"
                anchors.verticalCenter: parent.verticalCenter
                size: root.small ? 16 : 18
                width: root.selected ? (root.small ? 16 : 18) * root.s : 0
                opacity: root.selected ? 1 : 0
                visible: root.selected || width > 0
                clip: true
                ink: Lithic.containerInk
                Accessible.ignored: true
                Behavior on width { NumberAnimation { duration: Lithic.duration(120); easing.type: Easing.OutCubic } }
                Behavior on opacity { NumberAnimation { duration: Lithic.duration(100); easing.type: Easing.OutCubic } }
            }
            Glyph {
                visible: root.icon.length > 0
                anchors.verticalCenter: parent.verticalCenter
                name: root.icon
                size: root.small ? 16 : 18
                ink: root.selected ? Lithic.containerInk : Lithic.fg2
            }
            Label {
                anchors.verticalCenter: parent.verticalCenter
                text: root.text
                typeRole: "label"
                ink: root.selected ? Lithic.containerInk : button.hovered ? Lithic.fg : Lithic.fg2
            }
        }
    }
}
