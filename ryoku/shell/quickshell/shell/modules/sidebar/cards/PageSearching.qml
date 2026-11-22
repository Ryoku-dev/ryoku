import QtQuick
import Lithic
import ".."

Item {
    id: root

    property real s: 1
    property bool running: false
    property string text: ""
    readonly property bool flakes: QsLook.lithic && root.running

    implicitHeight: line.implicitHeight + (bar.visible ? bar.height + Math.round(8 * root.s) : 0)

    Working {
        id: working
        visible: root.flakes
        active: root.flakes
        size: Math.round(14 * root.s)
        anchors.left: parent.left
        anchors.verticalCenter: line.verticalCenter
    }

    Text {
        id: line
        anchors.left: root.flakes ? working.right : parent.left
        anchors.leftMargin: root.flakes ? Math.round(10 * root.s) : 0
        anchors.right: parent.right
        text: root.text
        color: root.running ? Lithic.fg2 : Lithic.fg3
        font.family: QsLook.sans
        font.pixelSize: Math.round(12 * root.s)
        wrapMode: Text.WordWrap
        renderType: Text.NativeRendering
    }

    Item {
        id: bar
        visible: !QsLook.lithic && root.running
        anchors.top: line.bottom
        anchors.topMargin: Math.round(8 * root.s)
        width: parent.width
        height: Math.max(2, Math.round((QsLook.material ? 4 : 3) * root.s))
        clip: true

        property real slide: 0

        Rectangle {
            anchors.fill: parent
            radius: height / 2
            color: Lithic.alpha(Lithic.sel, QsLook.material ? 0.22 : 0.14)
        }
        Rectangle {
            id: segment
            width: Math.round(parent.width * 0.3)
            height: parent.height
            radius: height / 2
            color: Lithic.sel
            x: QsLook.still ? (bar.width - width) / 2 : bar.slide
        }

        NumberAnimation on slide {
            running: bar.visible && !QsLook.still
            from: -segment.width
            to: bar.width
            duration: 1400
            loops: Animation.Infinite
            easing.type: Easing.InOutCubic
        }
    }
}
