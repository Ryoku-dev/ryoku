pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons as RyokuUi
import QtQuick.Layouts
import Quickshell
import inir.modules.common
import inir.services
import inir.modules.iris.style
import inir.modules.iris.components

IrisWidgetFace {
    id: root

    readonly property bool analog: String(root.widget.irisOption("face", "analog")) === "analog"
    readonly property bool seconds: Boolean(root.widget.irisOption("seconds", true))
    readonly property var now: clock.date
    readonly property string timeText: Translation.locale.toString(root.now, RyokuUi.TimeFormat.pattern())
    readonly property string weekday: {
        const name = Translation.locale.toString(root.now, "dddd")
        return name.charAt(0).toUpperCase() + name.slice(1)
    }
    readonly property string longDate: Translation.locale.toString(root.now, "d MMMM")

    padding: root.analog && root.small ? root.dp(8) : root.dp(16)

    SystemClock {
        id: clock
        precision: root.analog && root.seconds && root.moving ? SystemClock.Seconds : SystemClock.Minutes
    }

    FaceDial {
        id: dial
        visible: root.analog
        face: root
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        width: root.small ? parent.width : parent.height
        height: width
        time: root.now
        seconds: root.seconds
        caption: root.small ? Translation.locale.toString(root.now, "ddd d").replace(/\./g, "") : ""
    }

    ColumnLayout {
        visible: !root.analog || !root.small
        anchors.left: root.analog ? dial.right : parent.left
        anchors.leftMargin: root.analog ? root.dp(18) : 0
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        spacing: root.dp(2)

        FaceText {
            face: root
            Layout.fillWidth: true
            text: root.weekday
            color: root.highlight
            size: root.small ? 13 : 14
            weight: Font.DemiBold
        }
        IrisClock {
            Layout.fillWidth: true
            Layout.maximumWidth: implicitWidth
            text: root.timeText
            pixelSize: root.px(root.small ? 42 : root.analog ? 46 : 58)
            weight: root.figureWeight
            color: root.ink
            separatorColor: root.highlight
            family: root.fontNumbers
        }
        FaceText {
            face: root
            Layout.fillWidth: true
            text: root.longDate
            color: root.inkSecondary
            size: root.small ? 12.5 : 13.5
        }
    }
}
