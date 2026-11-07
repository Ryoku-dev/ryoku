import QtQuick
import Quickshell
import Ryoku.Ui.Singletons
import "../kit" as K
import "../settings" as S

Item {
    id: root

    required property var scene
    required property string lane
    property bool present: true

    readonly property date now: systemClock.date
    readonly property string timeText: TimeFormat.format(now, S.Prefs.seconds)
    readonly property string dateText: now.toLocaleDateString(
        TimeFormat.locale, "ddd d MMM")
    readonly property string hintText: now.toLocaleDateString(
        TimeFormat.locale, "dddd, MMMM d, yyyy")

    implicitWidth: content.implicitWidth + 2 * K.Style.hitPad
    implicitHeight: K.Style.barH
    height: K.Style.barH

    Column {
        id: content
        anchors.centerIn: parent
        spacing: 0
        scale: press.pressScale

        K.Label {
            anchors.horizontalCenter: parent.horizontalCenter
            text: root.timeText
            role: "bar"
            numeric: true
            strong: true
        }

        K.Label {
            visible: S.Prefs.showDate
            anchors.horizontalCenter: parent.horizontalCenter
            text: root.dateText
            role: "barSub"
            tone: "dim"
            numeric: true
        }
    }

    K.Press {
        id: press
        radius: K.Style.radius
        onClicked: root.scene.toggleCard("today", root)
        onRightClicked: S.Prefs.set("clock12h", !S.Prefs.clock12h)
        onHoveredChanged: root.scene.hoverCard("today", root, hovered)
    }

    K.Hint {
        target: root
        text: root.hintText
        shown: press.hovered
    }

    SystemClock {
        id: systemClock
        precision: S.Prefs.seconds ? SystemClock.Seconds : SystemClock.Minutes
    }
}
