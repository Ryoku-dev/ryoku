import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// kind: "plain", "order" or "swatch".
Item {
    id: chip

    property string kind: "plain"
    property string label: ""
    property bool active: false
    // Swatch index (0-12); used only when kind === "swatch".
    property int swatchIndex: 0
    property color swatchInner: "gray"
    property color swatchInnerActive: "white"

    signal clicked()

    readonly property bool _swatch: chip.kind === "swatch"
    readonly property bool _order: chip.kind === "order"
    readonly property bool hovered: hoverArea.containsMouse

    implicitHeight: _swatch ? 18 * Theme.scale : 28 * Theme.scale
    implicitWidth: _swatch
        ? 18 * Theme.scale
        : labelText.implicitWidth + 22 * Theme.scale

    CutRect {
        anchors.fill: parent
        visible: !chip._swatch
        cut: 4 * Theme.scale
        fill: chip.active ? Lithic.container
            : Lithic.alpha(Lithic.fg, chip.hovered ? 0.09 : 0.04)
        Behavior on fill { ColorAnimation { duration: Theme.fast } }
    }
    Text {
        id: labelText
        visible: !chip._swatch
        anchors.verticalCenter: parent.verticalCenter
        anchors.left: parent.left
        anchors.leftMargin: 11 * Theme.scale
        text: chip.label
        font.family: Theme.sans
        font.weight: chip.active ? Font.DemiBold : Font.Medium
        font.pixelSize: Theme.fontBase
        color: chip.active
            ? Lithic.containerInk
            : chip.hovered
                ? Theme.withAlpha(Theme.surfaceText, chip._order ? 0.88 : 0.90)
                : Theme.withAlpha(Theme.surfaceText, chip._order ? 0.54 : 0.64)
        renderType: Text.NativeRendering
        Behavior on color { ColorAnimation { duration: Theme.fast } }
    }

    Item {
        anchors.fill: parent
        visible: chip._swatch
        scale: chip.active ? 1.15 : 1.0
        Behavior on scale { NumberAnimation { duration: Theme.fast; easing.type: Theme.revealEasing } }

        Rectangle {
            anchors.fill: parent
            color: Theme.withAlpha(Theme.background, 0.9)
        }
        Rectangle {
            anchors.fill: parent
            anchors.margins: 2 * Theme.scale
            color: chip.active
                ? chip.swatchInnerActive
                : chip.hovered ? Qt.lighter(chip.swatchInner, 1.2) : chip.swatchInner
            border.width: chip.active ? 1.5 : 0
            border.color: Theme.surfaceText
        }
    }

    MouseArea {
        id: hoverArea
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        onClicked: chip.clicked()
    }
}
