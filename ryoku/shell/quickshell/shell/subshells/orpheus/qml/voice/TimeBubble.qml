import QtQuick
import Quickshell
import Ryoku.Ui.Singletons
import "../kit" as K
import "../settings" as S

Item {
    id: root

    required property var scene

    implicitWidth: Math.max(K.Style.barH, timeLabel.implicitWidth + 2 * K.Style.barPad)
    implicitHeight: K.Style.barH

    K.Label {
        id: timeLabel
        anchors.centerIn: parent
        text: TimeFormat.format(clock.date, false)
        role: "bar"
        numeric: true
        strong: true
    }

    K.Press {
        id: press
        radius: K.Style.radius
        onClicked: root.scene.toggleCard("today", root)
    }

    K.Hint {
        target: root
        text: "Today"
        shown: press.hovered
    }

    SystemClock {
        id: clock
        precision: SystemClock.Minutes
    }
}
