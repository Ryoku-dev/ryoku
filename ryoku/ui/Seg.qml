pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons

// 2-4 exclusive named modes. Each segment hugs its own label with side padding,
// so a long translation is never clipped inside its button; the group wraps to a
// second line when the width it is given (a cell's reserved control slot) cannot
// hold one row. Given no width bound, it measures its natural single row and
// stays on one line, so a caller that anchors it into a wide slot is unaffected.
// Spans.controlFor() still forbids five or more options here; this only draws.
Item {
    id: seg
    property var options: []
    // optLabels maps a stored value to readable text, so a compositor's numeric
    // enum can show "Two fingers" while still writing "1".
    property var labels: ({})
    property string current: ""
    signal chose(string key)

    activeFocusOnTab: true
    function activate(i) { if (i >= 0 && i < options.length) chose(options[i]) }
    Keys.onPressed: event => {
        var i = options.indexOf(current);
        if (event.key === Qt.Key_Right || event.key === Qt.Key_Down) {
            activate(Math.min(options.length - 1, i + 1)); event.accepted = true;
        } else if (event.key === Qt.Key_Left || event.key === Qt.Key_Up) {
            activate(Math.max(0, i - 1)); event.accepted = true;
        }
    }

    // a segment never drops below minSeg (a one-glyph mode still reads as a
    // button) and pads its label by padSeg either side.
    readonly property int minSeg: 52
    readonly property int padSeg: 18
    readonly property int segH: 24

    implicitWidth: measure.implicitWidth
    implicitHeight: flow.implicitHeight
    width: parent ? Math.min(measure.implicitWidth, parent.width) : measure.implicitWidth
    height: flow.implicitHeight

    // an off-screen single row, so the natural width is known before the visible
    // Flow (which can only report the width it was already given) lays out.
    Row {
        id: measure
        visible: false
        spacing: Tokens.s1
        Repeater {
            model: seg.options
            Item {
                required property string modelData
                readonly property string shown: seg.labels && seg.labels[modelData] !== undefined ? seg.labels[modelData] : modelData
                width: Math.max(seg.minSeg, mlab.implicitWidth + seg.padSeg)
                height: seg.segH
                Text {
                    id: mlab
                    text: I18n.tr(parent.shown)
                    font.family: Tokens.ui; font.pixelSize: 9
                    font.weight: Font.Medium; font.letterSpacing: 0.6
                }
            }
        }
    }

    Flow {
        id: flow
        width: seg.width
        spacing: Tokens.s1
        Repeater {
            model: seg.options
            Rectangle {
                required property string modelData
                readonly property bool on: seg.current === modelData
                readonly property string shown: seg.labels && seg.labels[modelData] !== undefined ? seg.labels[modelData] : modelData
                width: Math.max(seg.minSeg, lab.implicitWidth + seg.padSeg)
                height: seg.segH
                radius: Tokens.radius
                color: on ? Tokens.bone : (tap.pressed ? Tokens.tint16 : (sh.hovered ? Tokens.tint10 : "transparent"))
                border.width: Tokens.border
                border.color: sh.hovered && !on ? Tokens.lineStrong : Tokens.line
                Behavior on color { ColorAnimation { duration: Tokens.snap } }

                Text {
                    id: lab
                    anchors.centerIn: parent
                    text: I18n.tr(parent.shown)
                    color: parent.on ? Tokens.inkOnBone : Tokens.inkDim
                    font.family: Tokens.ui
                    font.pixelSize: 9
                    font.weight: Font.Medium
                    font.letterSpacing: 0.6
                }
                HoverHandler { id: sh; cursorShape: Qt.PointingHandCursor }
                TapHandler { id: tap; onTapped: seg.chose(parent.modelData) }
            }
        }
    }
}
