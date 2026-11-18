pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "look"
import "."

Flyout {
    id: pop

    /** Externally driven current stroke width in the 1..20 range. */
    property int selected: 4

    /** Emitted on a preset tap or a slider move. */
    signal picked(int w)

    readonly property var presets: [2, 4, 7, 12]

    /** Clamps to 1..20 and emits only when the value actually moves. */
    function apply(w) {
        var c = Math.max(1, Math.min(20, Math.round(w)));
        if (c !== pop.selected) pop.picked(c);
    }

    Column {
        width: Math.round(184 * pop.u)
        spacing: Math.round(10 * pop.u)

        Slider {
            width: parent.width
            u: pop.u
            label: I18n.tr("Width")
            from: 1
            to: 20
            value: pop.selected
            onMoved: (v) => pop.apply(v)
        }

        Row {
            id: presetRow
            width: parent.width
            readonly property real cell: Math.round(36 * pop.u)
            spacing: (width - pop.presets.length * cell) / (pop.presets.length - 1)

            Repeater {
                model: pop.presets
                Item {
                    id: cell
                    required property int modelData
                    width: presetRow.cell
                    height: Math.round(30 * pop.u)
                    readonly property bool chosen: pop.selected === modelData
                    readonly property color fillNow: cell.chosen ? (Look.adwaita ? Look.checkedFill : Look.lithic ? Look.well : Look.container)
                        : cellHover.hovered ? Look.hover : Lithic.alpha(Look.hover, 0)
                    readonly property color inkNow: cell.chosen ? (Look.lithic ? Look.sel : Look.material ? Look.containerInk : Look.fg) : Look.fg2

                    CutRect {
                        visible: Look.lithic
                        anchors.fill: parent
                        design: "lithic"
                        cut: 3 * pop.u
                        fill: cell.fillNow
                        strokeColor: cell.chosen ? Look.sel : "transparent"
                        strokeWidth: Math.max(1, Math.round(pop.u))
                    }
                    Rectangle {
                        visible: !Look.lithic
                        anchors.fill: parent
                        radius: Look.material ? height / 2 : (Look.adwaita ? 6 : 4) * pop.u
                        color: cell.fillNow
                        Behavior on color {
                            ColorAnimation { duration: Look.dur(Look.stateMs) }
                        }
                    }
                    // the stroke itself, at its weight
                    Rectangle {
                        anchors.centerIn: parent
                        width: Math.round(18 * pop.u)
                        height: Math.max(1, Math.round(Math.min(10, cell.modelData * 0.75) * pop.u))
                        radius: Look.lithic ? 0 : height / 2
                        color: cell.inkNow
                    }

                    HoverHandler { id: cellHover; cursorShape: Qt.PointingHandCursor }
                    TapHandler { onTapped: pop.apply(cell.modelData) }
                }
            }
        }
    }
}
