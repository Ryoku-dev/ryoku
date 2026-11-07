import QtQuick
import Ryoku.Ui.Singletons

// A named group of settings. The section owns packing: cells declare a span
// and Flow places them. Nothing in this file knows where a cell goes, which is
// the point -- meaning decides grouping, the control decides width.
Column {
    id: sect
    property string title: ""
    property color titleColor: Tokens.ink   // opt-in accent for a featured section head
    property int gutter: Tokens.s2
    default property alias cells: flow.data

    readonly property real colWidth: (width - (Spans.cols - 1) * gutter) / Spans.cols
    // a span is a request, not a sentence: below ~300px a cell truncates its
    // own label, so narrow sheets re-wrap at a usable minimum instead.
    function span(n) { return Math.min(width, Math.max(n * colWidth + (n - 1) * gutter, 290)) }

    spacing: Tokens.s3

    Row {
        spacing: Tokens.s2
        // the reference sheet's label vocabulary: a mono // lead, the tracked
        // title, the rule, and an end tick registering where the rule stops.
        Text {
            text: "//"
            color: Tokens.inkFaint
            font.family: Tokens.mono
            font.pixelSize: Tokens.fMicro
            anchors.verticalCenter: parent.verticalCenter
        }
        Text {
            text: sect.title + "_"
            color: sect.titleColor
            font.family: Tokens.ui
            font.pixelSize: Tokens.fMicro
            font.weight: Font.Medium
            font.letterSpacing: Tokens.trackMark
            anchors.verticalCenter: parent.verticalCenter
        }
        Rectangle {
            width: Math.max(0, sect.width - 200)
            height: 1
            color: Tokens.lineSoft
            anchors.verticalCenter: parent.verticalCenter
        }
        Rectangle {
            width: 1
            height: 5
            color: Tokens.line
            anchors.verticalCenter: parent.verticalCenter
            anchors.verticalCenterOffset: -2
        }
    }
    Flow {
        id: flow
        width: parent.width
        spacing: sect.gutter
    }
}
