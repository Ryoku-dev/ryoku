import QtQuick
import Ryoku.Ui.Singletons
import "../../look"
import "../.."

// The custom gradient's two colours: its start and end stops either side of a
// strip of the gradient itself. The ringed stop is the one the palette under
// them recolours; tap the other to edit it.
Column {
    id: cg

    required property var studio
    property real u: 1
    property string editing: "a"

    spacing: Math.round((Look.lithic ? 10 : 12) * cg.u)

    Item {
        width: cg.width
        height: Math.round(46 * cg.u)

        component Stop: Column {
            id: stop
            property string which: "a"
            property string caption: ""
            spacing: Math.round(4 * cg.u)
            Chip {
                anchors.horizontalCenter: parent.horizontalCenter
                u: cg.u
                size: 28
                ink: cg.studio ? (stop.which === "a" ? cg.studio.bgGradA : cg.studio.bgGradB) : "white"
                selected: cg.editing === stop.which
                onTapped: cg.editing = stop.which
            }
            Text {
                anchors.horizontalCenter: parent.horizontalCenter
                text: Look.lithic ? stop.caption.toUpperCase() : stop.caption
                color: cg.editing === stop.which ? Look.fg : Look.fg3
                font.family: Look.lithic ? Look.mono : Look.sans
                font.pixelSize: Math.round((Look.lithic ? 8.5 : 11) * cg.u)
                font.weight: Look.lithic ? Font.Bold : Font.Medium
                font.letterSpacing: Look.lithic ? Look.tracking * cg.u : 0
                renderType: Text.CurveRendering
            }
        }

        Stop {
            id: startStop
            anchors.left: parent.left
            which: "a"
            caption: I18n.tr("Start")
        }
        Stop {
            id: endStop
            anchors.right: parent.right
            which: "b"
            caption: I18n.tr("End")
        }
        GradientTile {
            anchors.left: startStop.right
            anchors.right: endStop.left
            anchors.leftMargin: Math.round(10 * cg.u)
            anchors.rightMargin: Math.round(10 * cg.u)
            y: Math.round(6 * cg.u)
            height: Math.round(16 * cg.u)
            a: cg.studio ? cg.studio.bgGradA : "white"
            b: cg.studio ? cg.studio.bgGradB : "black"
            angle: 0
            form: Look.lithic ? "cut" : "round"
            corner: Look.lithic ? 3 * cg.u : Look.fluent ? 4 * cg.u : height / 2
        }
    }

    Swatches {
        width: cg.width
        u: cg.u
        colors: cg.studio ? cg.studio.palette : []
        current: cg.studio ? (cg.editing === "a" ? cg.studio.bgGradA : cg.studio.bgGradB) : "white"
        onPicked: (c) => {
            if (cg.editing === "a")
                cg.studio.bgGradA = c;
            else
                cg.studio.bgGradB = c;
        }
    }
}
