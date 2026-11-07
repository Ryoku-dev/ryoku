import QtQuick
import shell.barkit as Pill
import shell.services
import "../kit" as K

Item {
    id: root

    required property var scene
    required property string lane
    property bool present: true

    readonly property real markSize: 18 * K.Style.scale
    readonly property real sealSize: 28 * K.Style.scale
    readonly property real opticalOffset: markMetrics.imageMode ? 0
        : markMetrics.boundingRect.y + markMetrics.boundingRect.height / 2
            - markMetrics.tightBoundingRect.y - markMetrics.tightBoundingRect.height / 2

    implicitWidth: sealSize + 2 * K.Style.hitPad
    implicitHeight: K.Style.barH
    height: K.Style.barH

    TextMetrics {
        id: markMetrics
        readonly property bool imageMode: Theme.markSource.length > 0
        text: Theme.mark
        font.family: Theme.fontJp
        font.pixelSize: root.markSize
        font.weight: Font.Black
    }

    Item {
        id: seal
        anchors.centerIn: parent
        width: root.sealSize
        height: width
        scale: press.pressScale

        K.Press {
            id: press
            radius: K.Style.radius
            onClicked: root.scene.openSettings("")
            onRightClicked: root.scene.toggleLauncher()
        }

        Pill.BrandMark {
            anchors.centerIn: parent
            anchors.verticalCenterOffset: root.opticalOffset
            width: root.markSize
            height: root.markSize
            size: root.markSize
            color: K.Style.accent
            weight: Font.Black
        }
    }


    K.Hint {
        target: root
        text: "Orpheus settings"
        shown: press.hovered
    }
}
