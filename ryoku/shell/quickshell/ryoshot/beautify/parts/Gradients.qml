pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../look"
import "../.."

// The twelve preset gradients and a thirteenth tile for a custom one, in the
// design's swatch: Lithic's cut tiles framed in sel when chosen, Material's and
// Adwaita's circles taking a check, Fluent's rounded squares with an accent
// border. The custom tile shows the custom gradient under a plus.
Grid {
    id: gr

    required property var studio
    property real u: 1

    readonly property real tw: Math.round(34 * gr.u)
    readonly property real th: Math.round((Look.lithic ? 26 : 34) * gr.u)
    readonly property string form: Look.lithic ? "cut" : Look.fluent ? "round" : "circle"

    columns: 7
    columnSpacing: Math.max(0, (gr.width - 7 * gr.tw) / 6)
    rowSpacing: Math.round((Look.lithic ? 6 : 10) * gr.u)

    component Tile: Item {
        id: tile
        property color a
        property color b
        property real angle: 135
        property bool chosen: false
        property bool custom: false
        signal tapped()

        width: gr.tw
        height: gr.th

        readonly property real ring: Math.max(1, Math.round((Look.material ? 2 : Look.fluent ? 2 : 1.5) * gr.u))
        readonly property real inset: tile.chosen ? tile.ring * 2 : 0

        Item {
            anchors.fill: parent
            scale: hover.hovered && !tile.chosen ? 1.08 : 1
            Behavior on scale {
                NumberAnimation { duration: Look.dur(120); easing.type: Easing.OutCubic }
            }

            GradientTile {
                anchors.fill: parent
                anchors.margins: tile.inset
                a: tile.a
                b: tile.b
                angle: tile.angle
                form: gr.form
                corner: (Look.lithic ? 3 : 4) * gr.u
                Behavior on anchors.margins {
                    NumberAnimation { duration: Look.dur(140); easing.type: Easing.OutCubic }
                }
            }
            // the chosen tile's ring, with a gap of the plane inside it
            CutRect {
                visible: Look.lithic && tile.chosen
                anchors.fill: parent
                design: "lithic"
                cut: 4 * gr.u
                strokeColor: Look.sel
                strokeWidth: tile.ring
            }
            Rectangle {
                visible: !Look.lithic && tile.chosen
                anchors.fill: parent
                radius: gr.form === "circle" ? width / 2 : 6 * gr.u
                color: "transparent"
                border.width: tile.ring
                border.color: Look.fluent ? Look.sel : Look.fg
                antialiasing: true
            }
            Icon {
                visible: (tile.chosen && (Look.material || Look.adwaita)) || (tile.custom && !tile.chosen)
                anchors.centerIn: parent
                name: tile.custom && !tile.chosen ? "plus" : "check"
                size: Math.round(16 * gr.u)
                tint: Lithic.inkOn(Lithic.mix(tile.a, tile.b, 0.5))
            }
        }

        HoverHandler {
            id: hover
            cursorShape: Qt.PointingHandCursor
        }
        TapHandler {
            onTapped: tile.tapped()
        }

        Tip {
            u: gr.u
            text: tile.custom ? I18n.tr("Custom") : ""
            shown: tile.custom && hover.hovered
            anchors.horizontalCenter: parent.horizontalCenter
            y: -height - 6 * gr.u
            z: 10
        }
    }

    Repeater {
        model: gr.studio ? gr.studio.presets : []
        delegate: Tile {
            required property int index
            required property var modelData
            a: modelData.a
            b: modelData.b
            angle: modelData.ang
            chosen: gr.studio.bgKind === "preset" && gr.studio.bgPreset === index
            onTapped: {
                gr.studio.bgKind = "preset";
                gr.studio.bgPreset = index;
            }
        }
    }
    Tile {
        custom: true
        a: gr.studio ? gr.studio.bgGradA : "white"
        b: gr.studio ? gr.studio.bgGradB : "black"
        angle: gr.studio ? gr.studio.bgGradAngle : 135
        chosen: gr.studio && gr.studio.bgKind === "gradient"
        onTapped: gr.studio.bgKind = "gradient"
    }
}
