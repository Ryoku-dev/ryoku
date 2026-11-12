import QtQuick
import "../kit"

Item {
    id: item

    property string headline: ""
    property string supporting: ""
    property string icon: ""
    property string position: "only"
    property real from: 0
    property real to: 100
    property real value: 0
    property real stepSize: 1
    property string suffix: ""
    signal committed(real value)

    // the save round-trips through the daemon; hold the pick so the handle doesn't snap back meanwhile
    property real held: NaN
    onValueChanged: item.held = NaN

    readonly property real span: Math.max(0.0001, item.to - item.from)
    readonly property real current: isNaN(item.held) ? item.value : item.held
    readonly property real shownValue: slider.dragging ? item.snap(slider.live) : item.current
    function snap(fraction) {
        var v = item.from + Math.max(0, Math.min(1, fraction)) * item.span;
        return Math.round(v / item.stepSize) * item.stepSize;
    }

    readonly property bool topOuter: shape.topOuter
    readonly property bool bottomOuter: shape.bottomOuter

    implicitWidth: 320
    implicitHeight: texts.implicitHeight + slider.implicitHeight + 46

    Segment {
        id: shape
        anchors.fill: parent
        position: item.position
    }

    Column {
        id: texts
        opacity: item.enabled ? 1 : 0.38
        anchors { top: parent.top; left: parent.left; right: parent.right; topMargin: 16; leftMargin: 18; rightMargin: 18 }
        spacing: 2

        Item {
            width: parent.width
            height: title.implicitHeight

            Label {
                id: title
                anchors { left: parent.left; right: amount.left; rightMargin: 12 }
                role: "title"
                font.pixelSize: 15
                text: item.headline
            }
            Label {
                id: amount
                anchors { right: parent.right; verticalCenter: title.verticalCenter }
                role: "title"
                font.family: Md.mono
                font.pixelSize: 15
                color: Md.accent
                text: Math.round(item.shownValue) + item.suffix
            }
        }
        Label {
            width: parent.width
            visible: text.length > 0
            role: "caption"
            color: Md.inkMuted
            text: item.supporting
        }
    }

    Slider {
        id: slider
        anchors { top: texts.bottom; left: parent.left; right: parent.right; topMargin: 14; leftMargin: 18; rightMargin: 18 }
        icon: item.icon
        value: (item.current - item.from) / item.span
        step: Math.max(item.stepSize, item.span / 20) / item.span
        onCommitted: function (fraction) {
            var v = item.snap(fraction);
            item.held = v;
            item.committed(v);
        }
    }
}
