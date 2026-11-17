import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import "apps.js" as Apps

// an engraved bar chart of space: one hatched bar a part, its length against the largest,
// with its size and count beside it. The bars draw in from the left once as they arrive,
// grow or shrink to a new figure, and stand still after
Column {
    id: chart

    property string title: ""
    // [{ id, label, bytes, count }]
    property var entries: []
    // the count beside each size, already translated, with %1 for the number
    property string countText: ""
    readonly property real largest: chart.entries.reduce((m, e) => Math.max(m, e.bytes), 0)
    property real grown: 0
    property string hoverId: ""
    // the same for a count of one
    property string oneText: ""

    spacing: LithicTokens.space.s2
    Accessible.role: Accessible.Chart
    Accessible.name: chart.title

    // the first figures print in; later ones move their bars
    onEntriesChanged: {
        if (chart.entries.length === 0 || chart.grown > 0)
            return;
        if (Lithic.reduceMotion)
            chart.grown = 1;
        else
            printIn.restart();
    }
    NumberAnimation {
        id: printIn
        target: chart
        property: "grown"
        from: 0
        to: 1
        duration: LithicTokens.motion.countUp
        easing.type: Easing.BezierSpline
        easing.bezierCurve: Lithic.curve
    }

    Label {
        text: chart.title
        typeRole: "caption"
        inkRole: "fg2"
        font.capitalization: Font.AllUppercase
        font.letterSpacing: 1.2
    }

    Repeater {
        model: chart.entries
        delegate: Item {
            id: bar
            required property var modelData
            required property int index
            readonly property bool lit: chart.hoverId === bar.modelData.id
            readonly property real track: width
            readonly property real share: chart.largest > 0 ? bar.modelData.bytes / chart.largest : 0
            width: chart.width
            height: name.implicitHeight + LithicTokens.space.s1 + 14 + 1

            HoverHandler {
                onHoveredChanged: chart.hoverId = hovered ? bar.modelData.id : (chart.hoverId === bar.modelData.id ? "" : chart.hoverId)
            }

            Label {
                id: name
                anchors.left: parent.left
                anchors.top: parent.top
                width: Math.min(implicitWidth, parent.width - value.implicitWidth - LithicTokens.space.s3)
                text: I18n.tr(bar.modelData.label)
                typeRole: "detail"
                ink: bar.lit ? Lithic.fg : Lithic.fg2
                elide: Text.ElideRight
                Behavior on ink {
                    ColorAnimation { duration: Tone.quick }
                }
            }

            // the track's baseline, a hairline the bars stand on
            CutRect {
                anchors.left: parent.left
                anchors.bottom: parent.bottom
                width: bar.track
                height: LithicTokens.size.chartBaseline
                fill: Lithic.vein
            }

            // the bar is drawn whole once; only the window over it moves
            Item {
                id: reach
                anchors.left: parent.left
                anchors.bottom: parent.bottom
                height: 14
                width: Math.max(bar.modelData.bytes > 0 ? 2 : 0, bar.track * bar.share * chart.grown)
                clip: true
                Behavior on width {
                    enabled: chart.grown >= 1
                    NumberAnimation { duration: Tone.move; easing.type: Easing.OutCubic }
                }

                Item {
                    width: Math.max(2, bar.track * bar.share)
                    height: parent.height

                    Hatch {
                        anchors.fill: parent
                        color: Lithic.alpha(bar.lit ? Lithic.fg : Lithic.fg2, LithicTokens.alpha.hatch)
                    }
                    CutRect {
                        anchors.fill: parent
                        strokeColor: bar.lit ? Lithic.fg : Lithic.fg2
                        strokeWidth: LithicTokens.size.chartLine
                        cutTopRight: Math.min(6, parent.width / 2)
                    }
                }
            }

            Label {
                id: value
                anchors.right: parent.right
                anchors.top: parent.top
                horizontalAlignment: Text.AlignRight
                text: (bar.modelData.bytes > 0 ? Apps.humanSize(bar.modelData.bytes, I18n.tr) : I18n.tr("Size not known"))
                    + " · " + (bar.modelData.count === 1 ? chart.oneText : chart.countText.arg(bar.modelData.count))
                typeRole: "detail"
                ink: bar.lit ? Lithic.fg : Lithic.fg2
                numeric: true
                Behavior on ink {
                    ColorAnimation { duration: Tone.quick }
                }
            }
        }
    }
}
