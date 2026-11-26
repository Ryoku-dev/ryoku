import QtQuick
import Lithic

Rectangle {
    id: scrim

    property real alpha: 0.68
    property real reveal: 1
    property bool dismissable: true

    signal dismissed()

    color: Lithic.scrim
    opacity: scrim.alpha * scrim.reveal

    MouseArea {
        anchors.fill: parent
        enabled: scrim.dismissable
        onClicked: scrim.dismissed()
    }

    Keys.onEscapePressed: if (scrim.dismissable) scrim.dismissed()
}
