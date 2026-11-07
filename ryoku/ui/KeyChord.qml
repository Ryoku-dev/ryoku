pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons

Item {
    id: chord

    property var keys: []
    property bool dark: true
    property bool motionEnabled: true
    property int count: 1
    property int pulse: 0
    property bool held: false
    // Per-monitor UI scale, threaded from KeypressStack (default 1 = no-op).
    property real us: 1

    implicitWidth: content.implicitWidth
    implicitHeight: content.implicitHeight

    Row {
        id: content
        spacing: Tokens.s1 * chord.us

        Repeater {
            model: chord.keys
            delegate: Row {
                id: keyWrap
                required property int index
                required property string modelData
                spacing: Tokens.s1 * chord.us

                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    visible: parent.index > 0
                    text: "+"
                    color: chord.dark ? Tokens.keycapDark : Tokens.keycapLight
                    opacity: 0.72
                    font.family: Tokens.ui
                    font.pixelSize: 16 * chord.us
                    font.weight: Font.DemiBold
                }

                Keycap {
                    us: chord.us
                    text: parent.modelData
                    dark: chord.dark
                    motionEnabled: chord.motionEnabled
                    held: chord.held
                    pulse: chord.pulse
                }
            }
        }

        Rectangle {
            anchors.verticalCenter: parent.verticalCenter
            visible: chord.count > 1
            implicitWidth: countLabel.implicitWidth + 14 * chord.us
            implicitHeight: 28 * chord.us
            radius: height / 2
            color: chord.dark ? Tokens.keycapLight : Tokens.keycapDark
            border.width: Tokens.border * chord.us
            border.color: Qt.rgba(countLabel.color.r, countLabel.color.g, countLabel.color.b, 0.18)

            Text {
                id: countLabel
                anchors.centerIn: parent
                text: "×" + chord.count
                color: chord.dark ? Tokens.keycapOnLight : Tokens.keycapOnDark
                font.family: Tokens.mono
                font.pixelSize: 11 * chord.us
                font.weight: Font.DemiBold
            }
        }
    }
}
