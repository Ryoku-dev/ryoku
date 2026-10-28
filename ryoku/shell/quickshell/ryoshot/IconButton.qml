import QtQuick
import "Singletons"

Rectangle {
    id: btn
    property string icon: ""
    property bool active: false
    property bool dim: false
    property string tooltip: ""
    property real iconSize: 18

    signal clicked()

    readonly property bool hovered: ma.containsMouse

    width: 32
    height: 32
    radius: 7
    color: active ? Theme.accent : (hovered && !dim ? Theme.hover : "transparent")

    Icon {
        anchors.centerIn: parent
        name: btn.icon
        size: btn.iconSize
        tint: btn.active ? Theme.accentInk : (btn.dim ? Theme.inkFaint : Theme.inkDim)
    }

    MouseArea {
        id: ma
        anchors.fill: parent
        hoverEnabled: true
        onClicked: if (!btn.dim) btn.clicked()
    }

    Timer {
        id: tipDelay
        interval: 450
        onTriggered: tip.opacity = 1
    }

    onHoveredChanged: {
        if (hovered && tooltip.length > 0)
            tipDelay.restart()
        else {
            tipDelay.stop()
            tip.opacity = 0
        }
    }

    Rectangle {
        id: tip
        enabled: false
        visible: btn.tooltip.length > 0
        opacity: 0
        z: 100
        anchors.top: parent.bottom
        anchors.topMargin: 6
        anchors.horizontalCenter: parent.horizontalCenter
        width: tipText.implicitWidth + 14
        height: tipText.implicitHeight + 8
        radius: 5
        color: Theme.panelSolid
        border.color: Theme.hair
        border.width: 1

        Behavior on opacity {
            NumberAnimation { duration: Theme.snap }
        }

        Text {
            id: tipText
            anchors.centerIn: parent
            text: btn.tooltip
            font.family: Theme.ui
            font.pixelSize: 11
            color: Theme.ink
        }
    }
}
