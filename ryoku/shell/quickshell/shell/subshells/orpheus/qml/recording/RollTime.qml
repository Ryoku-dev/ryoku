pragma ComponentBehavior: Bound

import QtQuick
import "../kit" as K

// the slot is sized for "00:00" from the start so ticking never moves the bar, and the
// last digit swaps in place instead of rolling so a running timer draws once a second
Item {
    id: root

    property string text: ""
    property string tone: "ink"

    readonly property real digitW: digitProbe.implicitWidth
    readonly property real colonW: colonProbe.implicitWidth
    readonly property real textW: cellsWidth(text)
    readonly property real slotW: Math.max(cellsWidth("00:00"), textW)

    implicitWidth: slotW
    implicitHeight: digitProbe.implicitHeight
    width: implicitWidth
    height: implicitHeight

    Behavior on implicitWidth {
        enabled: !K.Style.still
        NumberAnimation { duration: K.Style.ms(200); easing.type: Easing.OutCubic }
    }

    function cellWidth(ch) {
        return ch === ":" ? root.colonW : root.digitW;
    }

    function cellsWidth(value) {
        let total = 0;
        for (let i = 0; i < value.length; ++i)
            total += cellWidth(value.charAt(i));
        return total;
    }

    K.Label {
        id: digitProbe
        visible: false
        text: "0"
        role: "bar"
        numeric: true
    }

    K.Label {
        id: colonProbe
        visible: false
        text: ":"
        role: "bar"
        numeric: true
    }

    Row {
        x: Math.round((root.width - root.textW) / 2)
        height: root.height

        Repeater {
            model: root.text.length

            Item {
                id: cell

                required property int index
                readonly property string ch: root.text.charAt(index)
                readonly property bool last: index === root.text.length - 1
                property string shown: ch
                property string previous: ch
                property real roll: 1

                width: root.cellWidth(ch)
                height: root.height
                clip: roll < 1

                onChChanged: {
                    if (K.Style.still || cell.last || cell.ch === cell.shown) {
                        cell.shown = cell.ch;
                        cell.previous = cell.ch;
                        cell.roll = 1;
                        return;
                    }
                    cell.previous = cell.shown;
                    cell.shown = cell.ch;
                    rollIn.restart();
                }

                NumberAnimation {
                    id: rollIn
                    target: cell
                    property: "roll"
                    from: 0
                    to: 1
                    duration: K.Style.ms(160)
                    easing.type: Easing.OutCubic
                }

                K.Label {
                    visible: cell.roll < 1
                    width: cell.width
                    horizontalAlignment: Text.AlignHCenter
                    elide: Text.ElideNone
                    y: -cell.height * 0.6 * cell.roll
                    opacity: 1 - cell.roll
                    text: cell.previous
                    role: "bar"
                    tone: root.tone
                    numeric: true
                }

                K.Label {
                    width: cell.width
                    horizontalAlignment: Text.AlignHCenter
                    elide: Text.ElideNone
                    y: cell.height * 0.6 * (1 - cell.roll)
                    opacity: cell.roll
                    text: cell.shown
                    role: "bar"
                    tone: root.tone
                    numeric: true
                }
            }
        }
    }
}
