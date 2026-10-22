pragma ComponentBehavior: Bound

import QtQuick
import Caelestia.Config
import caelestia.components
import caelestia.components.controls

Item {
    id: root

    property int slots: 7
    property real spacing: Tokens.spacing.small
    readonly property real slotWidth: (width - spacing * (slots - 1)) / slots
    default property alias data: row.data

    implicitWidth: slots * 56 + spacing * (slots - 1)
    implicitHeight: row.implicitHeight

    function scroll(dir: int): void {
        const step = root.slotWidth + root.spacing;
        flickable.contentX = Math.max(0, Math.min(flickable.maxContentX, flickable.contentX + dir * step));
    }

    Flickable {
        id: flickable

        readonly property real maxContentX: Math.max(0, contentWidth - width)

        anchors.fill: parent
        clip: true
        interactive: false
        contentWidth: row.implicitWidth
        contentHeight: height

        onMaxContentXChanged: {
            if (contentX > maxContentX)
                contentX = maxContentX;
        }

        Behavior on contentX {
            Anim {}
        }

        Row {
            id: row

            x: implicitWidth <= flickable.width ? (flickable.width - implicitWidth) / 2 : 0
            anchors.verticalCenter: parent.verticalCenter
            spacing: root.spacing
        }
    }

    Arrow {
        anchors.left: parent.left
        visible: flickable.contentX > 0
        icon: "chevron_left"
        onClicked: root.scroll(-1)
    }

    Arrow {
        anchors.right: parent.right
        visible: flickable.contentX < flickable.contentWidth - flickable.width - 1
        icon: "chevron_right"
        onClicked: root.scroll(1)
    }

    // a MouseArea that takes no button still takes the wheel, and the Flickable below
    // would swallow it before a WheelHandler on the root saw it
    MouseArea {
        anchors.fill: parent
        z: 2
        acceptedButtons: Qt.NoButton
        onWheel: event => {
            if (event.angleDelta.y > 0)
                root.scroll(-1);
            else if (event.angleDelta.y < 0)
                root.scroll(1);
        }
    }

    component Arrow: IconButton {
        anchors.verticalCenter: parent.verticalCenter
        z: 1
        type: IconButton.Tonal
        isRound: true
        font: Tokens.font.icon.small
        padding: Tokens.padding.extraSmall
    }
}
