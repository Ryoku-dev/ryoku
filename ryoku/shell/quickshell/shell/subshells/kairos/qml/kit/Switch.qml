import QtQuick

Item {
    id: sw

    property bool checked: false
    signal toggled(bool value)

    implicitWidth: 52
    implicitHeight: 32
    opacity: sw.enabled ? 1 : 0.38

    Rectangle {
        id: track
        anchors.fill: parent
        radius: height / 2
        color: sw.checked ? Md.accent : Md.containerHighest
        border.width: sw.checked ? 0 : 2
        border.color: Md.inkFaint
        Behavior on color {
            enabled: !Md.still
            ColorAnimation { duration: Md.fast }
        }
    }

    Rectangle {
        id: thumb
        property real size: hit.pressed ? 28 : (sw.checked ? 24 : 16)
        width: thumb.size
        height: thumb.size
        radius: thumb.size / 2
        anchors.verticalCenter: parent.verticalCenter
        x: sw.checked ? sw.width - 4 - thumb.size : (sw.height - thumb.size) / 2
        color: sw.checked ? Md.onAccent : Md.inkFaint
        Behavior on x {
            enabled: !Md.still
            Spring { speed: "fast" }
        }
        Behavior on size {
            enabled: !Md.still
            NumberAnimation { duration: Md.fast }
        }
        Behavior on color {
            enabled: !Md.still
            ColorAnimation { duration: Md.fast }
        }

        Icon {
            anchors.centerIn: parent
            name: "check"
            size: 16
            color: Md.accent
            opacity: sw.checked ? 1 : 0
            Behavior on opacity {
                enabled: !Md.still
                NumberAnimation { duration: Md.fast }
            }
        }
    }

    StateLayer {
        id: hit
        radius: height / 2
        tint: "transparent"
        onActivated: sw.toggled(!sw.checked)
    }
}
