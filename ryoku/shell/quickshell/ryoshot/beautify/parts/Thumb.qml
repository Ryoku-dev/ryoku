import QtQuick
import QtQuick.Effects
import Lithic
import "../../look"
import ".."

// A look drawn on the user's own shot, filling its tile and cropped to the
// tile's shape. The composition renders once into its own layer at the size it
// will be seen (frame x frame), so resizing or re-masking the tile reuses it.
Item {
    id: th

    required property var studio
    property var cfg: ({})
    // the composition's box: the tile's largest size, so a shrinking tile crops
    property real frameW: th.width
    property real frameH: th.height
    property real corner: 8
    property bool cut: false

    readonly property real natW: th.studio && th.studio.natW > 0 ? th.studio.natW : 800
    readonly property real natH: th.studio && th.studio.natH > 0 ? th.studio.natH : 500
    readonly property real pad: th.cfg && th.cfg.padding !== undefined ? th.cfg.padding : 90
    // the shot's width in the tile, so the composition covers the frame
    readonly property real shotW: th.natW * Math.max(th.frameW / (th.natW + 2 * th.pad), th.frameH / (th.natH + 2 * th.pad))
    readonly property var look: Object.assign({}, th.cfg, { "ratioKey": "auto", "watermark": false })

    Item {
        id: shaped
        anchors.fill: parent
        layer.enabled: true
        layer.smooth: true
        layer.effect: MultiEffect {
            maskEnabled: true
            maskSource: mask
            maskThresholdMin: 0.5
            maskSpreadAtMin: 1.0
        }

        Item {
            id: holder
            anchors.centerIn: parent
            width: comp.width
            height: comp.height
            layer.enabled: true
            layer.smooth: true

            Composition {
                id: comp
                preview: true
                source: th.studio ? th.studio.source : ""
                mark: th.studio && th.studio.mark ? th.studio.mark : "\u529b"
                natural: Qt.size(Math.max(1, th.shotW), Math.max(1, th.shotW * th.natH / th.natW))
                basis: th.natW
                style: th.look
            }
        }
    }

    Item {
        id: mask
        anchors.fill: parent
        visible: false
        layer.enabled: true
        CutRect {
            visible: th.cut
            anchors.fill: parent
            design: "lithic"
            cut: th.corner
            fill: "white"
        }
        Rectangle {
            visible: !th.cut
            anchors.fill: parent
            radius: Math.min(th.corner, width / 2, height / 2)
            color: "white"
            antialiasing: true
        }
    }
}
