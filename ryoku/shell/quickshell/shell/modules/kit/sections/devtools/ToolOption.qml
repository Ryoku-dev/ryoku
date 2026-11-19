pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../../ask/look"

FocusScope {
    id: root
    required property real s
    required property var option
    property var value
    readonly property bool textEntry: option.type === "text"
    readonly property bool ownsText: textEntry && entry.activeFocus
    width: textEntry ? (option.id === "flags" ? 92 : 300) * s : contents.width + 20 * s
    height: 30 * s
    activeFocusOnTab: !textEntry
    signal changed(var value)
    function advance(delta) {
        if (option.type === "toggle") {
            changed(!value);
            return;
        }
        const choices = option.choices ?? [];
        if (!choices.length)
            return;
        const index = choices.findIndex(choice => choice.value === value);
        changed(choices[(Math.max(0, index) + delta + choices.length) % choices.length].value);
    }
    Keys.onSpacePressed: advance(1)
    Keys.onReturnPressed: advance(1)
    Keys.onLeftPressed: advance(-1)
    Keys.onRightPressed: advance(1)
    Shape {
        anchors.fill: parent
        fill: root.option.type === "toggle" && root.value ? Look.plate : hover.hovered ? Look.hover : "transparent"
        strokeColor: root.activeFocus ? Lithic.fg2 : Look.cardStroke
        strokeWidth: root.s
        cut: Look.cutToken * root.s
        Behavior on fill { ColorAnimation { duration: Look.dur(Look.stateMs) } }
    }
    Row {
        id: contents
        visible: !root.textEntry
        anchors.centerIn: parent
        spacing: 7 * root.s
        Text {
            text: root.option.label
            color: Lithic.fg2
            font { family: Look.sans; pixelSize: 11 * root.s }
        }
        Text {
            text: root.option.type === "toggle" ? root.value ? I18n.tr("On") : I18n.tr("Off") : (root.option.choices?.find(choice => choice.value === root.value)?.label ?? String(root.value))
            color: Lithic.fg
            font { family: Look.sans; pixelSize: 11 * root.s; weight: Font.Medium }
        }
    }
    Row {
        anchors { fill: parent; leftMargin: 10 * root.s; rightMargin: 10 * root.s }
        visible: root.textEntry
        spacing: 8 * root.s
        Text {
            id: label
            anchors.verticalCenter: parent.verticalCenter
            text: root.option.label
            color: Lithic.fg2
            font { family: Look.sans; pixelSize: 11 * root.s }
        }
        TextInput {
            id: entry
            width: parent.width - label.width - parent.spacing
            anchors.verticalCenter: parent.verticalCenter
            text: String(root.value ?? "")
            color: Lithic.fg
            selectionColor: Lithic.sel
            selectedTextColor: Lithic.onSel
            font { family: Look.mono; pixelSize: 10 * root.s }
            clip: true
            selectByMouse: true
            activeFocusOnTab: true
            onTextEdited: root.changed(text)
        }
    }
    HoverHandler { id: hover }
    TapHandler {
        enabled: !root.textEntry
        onTapped: { root.forceActiveFocus(); root.advance(1); }
    }
}
