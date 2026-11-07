pragma ComponentBehavior: Bound

import QtQuick

Item {
    id: root

    property var model: []
    property var value

    signal picked(var value)

    readonly property int count: model ? model.length : 0
    readonly property int activeIndex: {
        for (let index = 0; index < count; ++index) {
            if (model[index].value === value)
                return index;
        }
        return -1;
    }
    readonly property real widestCell: {
        const ready = measureRow.implicitWidth;
        let widest = 52;
        for (let index = 0; index < count; ++index) {
            const item = measures.itemAt(index);
            if (item)
                widest = Math.max(widest, item.implicitWidth);
        }
        return widest + ready * 0;
    }
    readonly property real naturalWidth: Math.max(1,
        count * widestCell + Math.max(0, count - 1) * Style.s1)
    readonly property real cellWidth: count > 0
        ? Math.max(1, (width - Math.max(0, count - 1) * Style.s1) / count) : 0

    implicitWidth: naturalWidth
    implicitHeight: 28
    clip: true
    activeFocusOnTab: true

    function move(step) {
        if (count === 0)
            return;
        const current = activeIndex < 0 ? 0 : activeIndex;
        const next = Math.max(0, Math.min(count - 1, current + step));
        picked(model[next].value);
    }

    Keys.onPressed: event => {
        if (event.key === Qt.Key_Left || event.key === Qt.Key_Up) {
            move(-1);
            event.accepted = true;
        } else if (event.key === Qt.Key_Right || event.key === Qt.Key_Down) {
            move(1);
            event.accepted = true;
        } else if (event.key === Qt.Key_Space || event.key === Qt.Key_Return
                   || event.key === Qt.Key_Enter) {
            move(0);
            event.accepted = true;
        }
    }

    Row {
        id: measureRow
        visible: false
        spacing: Style.s1

        Repeater {
            id: measures
            model: root.model

            delegate: Item {
                required property var modelData
                implicitWidth: Math.max(52, measureLabel.implicitWidth + Style.s4)
                implicitHeight: 28

                Label {
                    id: measureLabel
                    text: String(parent.modelData.label ?? "")
                    role: "label"
                }
            }
        }
    }

    Repeater {
        model: root.model

        delegate: Item {
            id: segment
            required property var modelData
            required property int index

            readonly property bool selected: index === root.activeIndex

            x: index * (root.cellWidth + Style.s1)
            width: root.cellWidth
            height: root.height
            scale: segmentPress.pressScale

            Rectangle {
                anchors.fill: parent
                radius: Style.radius
                color: segment.selected ? Style.bone
                    : segmentPress.pressed ? Style.tint16
                    : segmentPress.hovered ? Style.tint10 : "transparent"
                border.width: Style.border
                border.color: segment.selected ? Style.bone
                    : root.activeFocus ? Style.lineStrong : Style.line

                Behavior on color {
                    enabled: !Style.still
                    ColorAnimation { duration: Style.snap }
                }
            }

            Label {
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.margins: Style.s2
                anchors.verticalCenter: parent.verticalCenter
                text: String(segment.modelData.label ?? "")
                role: "label"
                tone: segment.selected ? "onBone" : "ink"
                horizontalAlignment: Text.AlignHCenter
            }

            Press {
                id: segmentPress
                tint: false
                onClicked: {
                    root.forceActiveFocus();
                    root.picked(segment.modelData.value);
                }
            }
        }
    }
}
