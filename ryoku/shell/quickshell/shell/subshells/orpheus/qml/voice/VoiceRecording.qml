pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import "../kit" as K

Item {
    id: root

    required property var scene

    implicitWidth: content.implicitWidth
    implicitHeight: K.Style.barH

    function elapsedText(seconds) {
        const whole = Math.max(0, Math.floor(Number(seconds) || 0));
        const minutes = Math.floor(whole / 60);
        const remainder = whole % 60;
        return minutes + ":" + (remainder < 10 ? "0" : "") + remainder;
    }

    Row {
        id: content
        anchors.centerIn: parent
        spacing: K.Style.s2

        Rectangle {
            id: pulseDot
            property real pulseOpacity: 1

            anchors.verticalCenter: parent.verticalCenter
            width: 8 * K.Style.scale
            height: width
            radius: width / 2
            color: K.Style.alert
            opacity: pulseOpacity

            SequentialAnimation on pulseOpacity {
                running: Recorder.active && root.visible && !K.Style.still
                loops: Animation.Infinite
                onStopped: pulseDot.pulseOpacity = 1

                NumberAnimation {
                    from: 1
                    to: 0.35
                    duration: K.Style.ms(600)
                    easing.type: Easing.InOutSine
                }
                NumberAnimation {
                    from: 0.35
                    to: 1
                    duration: K.Style.ms(600)
                    easing.type: Easing.InOutSine
                }
            }
        }

        K.Label {
            anchors.verticalCenter: parent.verticalCenter
            text: "Recording"
            role: "bar"
        }

        K.Label {
            anchors.verticalCenter: parent.verticalCenter
            text: root.elapsedText(Recorder.elapsedSec)
            role: "barSub"
            tone: "dim"
            numeric: true
        }
    }
}
