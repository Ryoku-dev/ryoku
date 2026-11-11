pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "."
import "TypingFingers.js" as Fingers

Item {
    id: preview

    required property var store
    property string nextChar: ""
    property string pressedChar: ""
    signal requestInputFocus()

    TypingKeyboardLayouts { id: layouts }

    readonly property var board: Fingers.classicBoard(layouts.rowsFor(store.keyboardLayout), store.keyboardLayout)
    readonly property var keys: board.keys
    readonly property var entries: board.entries
    readonly property var automaticFingers: board.fingers
    readonly property string boardId: "classic:" + store.keyboardLayout
    readonly property var assignedFingers: Fingers.assignments(automaticFingers, keys, boardId, store.fingerAssignments)
    readonly property var nextKeys: store.highlightNextKey ? Fingers.targets(entries, nextChar) : []
    readonly property var pressedKeys: Fingers.targets(entries, pressedChar)
    readonly property real gap: Tokens.s1
    readonly property real unit: Math.max(18, Math.min(38,
        (width - Tokens.s4 * 2 - gap * (board.width - 1)) / board.width))
    readonly property real boardWidth: board.width * unit + (board.width - 1) * gap
    readonly property real boardHeight: board.height * unit + (board.height - 1) * gap
    readonly property real legendHeight: store.fingerGuide ? 54 : 0
    implicitHeight: boardHeight + legendHeight + (store.fingerGuide ? Tokens.s3 : 0)
    property int selectedKey: -1

    function fingerName(value) {
        if (value === 6) return I18n.tr("either thumb");
        const side = value < 0 ? I18n.tr("left") : I18n.tr("right");
        const names = ["", I18n.tr("thumb"), I18n.tr("index"), I18n.tr("middle"), I18n.tr("ring"), I18n.tr("little")];
        return side + " " + names[Math.abs(value)];
    }

    function fingerFill(value) {
        const strength = [0.05, 0.12, 0.20, 0.28, 0.36, 0.44][Math.min(5, Math.abs(value))];
        return Qt.rgba(Tokens.ink.r, Tokens.ink.g, Tokens.ink.b, strength);
    }

    function setFinger(value) {
        if (selectedKey < 0)
            return;
        store.fingerAssignments = Fingers.saveAssignment(store.fingerAssignments,
            boardId, keys[selectedKey], value);
        selectedKey = -1;
        requestInputFocus();
    }

    Item {
        id: boardFace
        width: preview.boardWidth
        height: preview.boardHeight
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.top: parent.top

        Repeater {
            model: preview.keys
            delegate: Rectangle {
                id: key
                required property var modelData
                required property int index
                readonly property bool expected: preview.nextKeys.indexOf(index) >= 0
                readonly property bool pressed: preview.pressedKeys.indexOf(index) >= 0
                readonly property int finger: preview.assignedFingers[index] || 0
                x: modelData.x * (preview.unit + preview.gap)
                y: modelData.y * (preview.unit + preview.gap)
                width: modelData.w * preview.unit + Math.max(0, modelData.w - 1) * preview.gap
                height: modelData.h * preview.unit + Math.max(0, modelData.h - 1) * preview.gap
                radius: Tokens.radius
                color: pressed ? Tokens.bone : (expected ? Tokens.tint16
                    : (preview.store.fingerGuide ? preview.fingerFill(finger) : Tokens.tint5))
                border.width: expected || preview.selectedKey === index ? 2 : Tokens.border
                border.color: expected || preview.selectedKey === index ? Tokens.ink : Tokens.line
                scale: pressed ? 0.94 : (keyHover.hovered ? 1.025 : 1)
                Behavior on scale {
                    enabled: !Tokens.reduceMotion
                    NumberAnimation { duration: Tokens.snap; easing.type: Easing.OutCubic }
                }
                Behavior on color {
                    enabled: !Tokens.reduceMotion
                    ColorAnimation { duration: Tokens.snap }
                }
                Behavior on border.color {
                    enabled: !Tokens.reduceMotion
                    ColorAnimation { duration: Tokens.snap }
                }

                Text {
                    anchors.centerIn: parent
                    text: preview.entries[parent.index].label
                    color: parent.pressed ? Tokens.inkOnBone : Tokens.ink
                    font.family: Tokens.mono
                    font.pixelSize: Math.max(Tokens.fTiny, preview.unit * 0.31)
                    font.capitalization: Font.AllUppercase
                }
                HoverHandler { id: keyHover; enabled: preview.store.fingerGuide; cursorShape: Qt.PointingHandCursor }
                TapHandler {
                    enabled: preview.store.fingerGuide
                    onTapped: {
                        preview.selectedKey = key.index;
                    }
                }
            }
        }
    }

    Row {
        id: hands
        opacity: preview.store.fingerGuide ? 1 : 0
        visible: opacity > 0
        Behavior on opacity {
            enabled: !Tokens.reduceMotion
            NumberAnimation { duration: Tokens.snap; easing.type: Easing.OutCubic }
        }
        anchors.top: boardFace.bottom
        anchors.topMargin: Tokens.s3
        anchors.horizontalCenter: parent.horizontalCenter
        spacing: Tokens.s4
        Repeater {
            model: [
                { label: I18n.tr("LEFT"), values: [-5, -4, -3, -2, -1] },
                { label: I18n.tr("RIGHT"), values: [1, 2, 3, 4, 5] }
            ]
            delegate: Row {
                required property var modelData
                spacing: Tokens.s1
                Text {
                    text: parent.modelData.label
                    color: Tokens.inkMuted
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fTiny
                    font.letterSpacing: Tokens.trackLabel
                    anchors.verticalCenter: parent.verticalCenter
                }
                Repeater {
                    model: parent.modelData.values
                    delegate: Rectangle {
                        required property int modelData
                        width: 25
                        height: 25 + (5 - Math.abs(modelData)) * 3
                        radius: Tokens.radius
                        color: preview.fingerFill(modelData)
                        border.width: Tokens.border
                        border.color: Tokens.line
                        anchors.bottom: parent.bottom
                        Text {
                            anchors.centerIn: parent
                            text: String(Math.abs(parent.modelData))
                            color: Tokens.ink
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fTiny
                        }
                    }
                }
            }
        }
    }

    Rectangle {
        id: editor
        readonly property bool opened: preview.selectedKey >= 0
        visible: opacity > 0
        opacity: opened ? 1 : 0
        scale: opened ? 1 : 0.97
        z: 10
        width: Math.min(parent.width - Tokens.s4 * 2, 620)
        implicitHeight: editColumn.implicitHeight + Tokens.s4 * 2
        height: implicitHeight
        anchors.centerIn: parent
        radius: Tokens.radius
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        Behavior on opacity {
            enabled: !Tokens.reduceMotion
            NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
        }
        Behavior on scale {
            enabled: !Tokens.reduceMotion
            NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
        }

        ColumnLayout {
            id: editColumn
            anchors.fill: parent
            anchors.margins: Tokens.s4
            spacing: Tokens.s3
            Text {
                Layout.fillWidth: true
                text: preview.selectedKey >= 0
                    ? I18n.tr("Finger for %1").arg(preview.entries[preview.selectedKey].label) : ""
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fRow
                font.weight: Font.Medium
            }
            Flow {
                Layout.fillWidth: true
                spacing: Tokens.s2
                Repeater {
                    model: [
                        { label: I18n.tr("L5"), value: -5 }, { label: I18n.tr("L4"), value: -4 },
                        { label: I18n.tr("L3"), value: -3 }, { label: I18n.tr("L2"), value: -2 },
                        { label: I18n.tr("L1"), value: -1 }, { label: I18n.tr("Thumb"), value: 6 },
                        { label: I18n.tr("R1"), value: 1 }, { label: I18n.tr("R2"), value: 2 },
                        { label: I18n.tr("R3"), value: 3 }, { label: I18n.tr("R4"), value: 4 },
                        { label: I18n.tr("R5"), value: 5 }
                    ]
                    delegate: Btn {
                        required property var modelData
                        text: modelData.label
                        compact: true
                        onAct: preview.setFinger(modelData.value)
                    }
                }
            }
            RowLayout {
                Layout.fillWidth: true
                Item { Layout.fillWidth: true }
                Btn {
                    text: I18n.tr("Use automatic")
                    compact: true
                    onAct: {
                        if (preview.selectedKey >= 0) {
                            const keyPrefix = Fingers.keyId(preview.boardId, preview.keys[preview.selectedKey]) + "=";
                            preview.store.fingerAssignments = Array.from(preview.store.fingerAssignments)
                                .filter(item => !item.startsWith(keyPrefix));
                        }
                        preview.selectedKey = -1;
                        preview.requestInputFocus();
                    }
                }
                Btn { text: I18n.tr("Cancel"); compact: true; onAct: { preview.selectedKey = -1; preview.requestInputFocus(); } }
            }
        }
    }
}
