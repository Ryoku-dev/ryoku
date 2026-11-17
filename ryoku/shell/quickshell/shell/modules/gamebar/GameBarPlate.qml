import QtQuick
import Lithic

Item {
    id: root

    property url source
    property string glyph: ""
    property string placeholder: ""
    property int cut: LithicTokens.cut.small
    property color plane: Lithic.field
    property int decodeSize: 128
    default property alias content: overlay.data

    readonly property bool hasImage: image.status === Image.Ready
    readonly property real _scale: Lithic.scaleOf(root)

    CutRect {
        anchors.fill: parent
        fill: root.plane
        cut: root.cut
    }

    Item {
        anchors.fill: parent
        clip: true

        readonly property real side: Math.max(0, (width + height - root.cut * 2 * root._scale) / Math.SQRT2)

        Item {
            width: parent.side
            height: width
            x: (parent.width - width) / 2
            y: (parent.height - height) / 2
            rotation: 45
            clip: true

            Image {
                id: image
                x: (parent.width - root.width) / 2
                y: (parent.height - root.height) / 2
                width: root.width
                height: root.height
                rotation: -45
                source: root.source
                sourceSize.width: Math.round(root.decodeSize * root._scale)
                sourceSize.height: Math.round(root.decodeSize * root._scale)
                asynchronous: true
                cache: true
                fillMode: Image.PreserveAspectCrop
            }
        }
    }

    Glyph {
        anchors.centerIn: parent
        visible: !root.hasImage && root.glyph.length > 0
        name: root.glyph
        size: Math.round(Math.min(root.width, root.height) * 0.42 / root._scale)
        ink: Lithic.fg3
    }

    Label {
        anchors.centerIn: parent
        visible: !root.hasImage && root.glyph.length === 0 && root.placeholder.length > 0
        text: root.placeholder
        typeRole: "title"
        inkRole: "fg2"
    }

    CutRect {
        anchors.fill: parent
        fill: "transparent"
        strokeColor: Lithic.vein
        strokeWidth: LithicTokens.size.hairline
        cut: root.cut
    }

    Item {
        id: overlay
        anchors.fill: parent
    }
}
