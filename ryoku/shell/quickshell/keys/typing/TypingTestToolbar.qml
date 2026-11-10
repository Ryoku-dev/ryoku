pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons

Flow {
    id: toolbar

    required property var store
    property bool locked: false
    signal refocus()
    spacing: Tokens.s2

    component ToggleChip: Rectangle {
        id: chip
        required property string label
        required property bool selected
        signal chosen()
        width: chipLabel.implicitWidth + 20
        height: 26
        radius: Tokens.radius
        color: selected ? Tokens.bone : (tap.pressed ? Tokens.tint16 : (hover.hovered ? Tokens.tint10 : "transparent"))
        border.width: Tokens.border
        border.color: selected ? Tokens.bone : (hover.hovered ? Tokens.lineStrong : Tokens.line)
        opacity: toolbar.locked ? 0.45 : 1
        scale: tap.pressed ? 0.96 : (hover.hovered ? 1.025 : 1)
        Behavior on color {
            enabled: !Tokens.reduceMotion
            ColorAnimation { duration: Tokens.snap }
        }
        Behavior on border.color {
            enabled: !Tokens.reduceMotion
            ColorAnimation { duration: Tokens.snap }
        }
        Behavior on scale {
            enabled: !Tokens.reduceMotion
            NumberAnimation { duration: Tokens.snap; easing.type: Easing.OutCubic }
        }
        Behavior on opacity {
            enabled: !Tokens.reduceMotion
            NumberAnimation { duration: Tokens.snap; easing.type: Easing.OutCubic }
        }
        Text {
            id: chipLabel
            anchors.centerIn: parent
            text: chip.label
            color: chip.selected ? Tokens.inkOnBone : Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fMicro
            font.weight: Font.Medium
        }
        HoverHandler { id: hover; enabled: !toolbar.locked; cursorShape: Qt.PointingHandCursor }
        TapHandler { id: tap; enabled: !toolbar.locked; onTapped: chip.chosen() }
    }

    ToggleChip {
        label: I18n.tr("punctuation")
        selected: toolbar.store.punctuation
        onChosen: { toolbar.store.punctuation = !toolbar.store.punctuation; toolbar.refocus(); }
    }
    ToggleChip {
        label: I18n.tr("numbers")
        selected: toolbar.store.numbers
        onChosen: { toolbar.store.numbers = !toolbar.store.numbers; toolbar.refocus(); }
    }

    Repeater {
        model: [
            { mode: "time", label: I18n.tr("time") },
            { mode: "words", label: I18n.tr("words") },
            { mode: "zen", label: I18n.tr("zen") }
        ]
        delegate: ToggleChip {
            required property var modelData
            label: modelData.label
            selected: toolbar.store.mode === modelData.mode
            onChosen: { toolbar.store.mode = modelData.mode; toolbar.refocus(); }
        }
    }

    Repeater {
        model: toolbar.store.mode === "time" ? [15, 30, 60, 120]
            : (toolbar.store.mode === "words" ? [10, 25, 50, 100] : [])
        delegate: ToggleChip {
            required property int modelData
            label: toolbar.store.mode === "time" ? modelData + "s" : String(modelData)
            selected: toolbar.store.mode === "time"
                ? toolbar.store.timeLimit === modelData : toolbar.store.wordLimit === modelData
            onChosen: {
                if (toolbar.store.mode === "time") toolbar.store.timeLimit = modelData;
                else toolbar.store.wordLimit = modelData;
                toolbar.refocus();
            }
        }
    }

    ToggleChip {
        visible: toolbar.store.mode === "zen"
        label: I18n.tr("guided")
        selected: toolbar.store.zenGuided
        onChosen: { toolbar.store.zenGuided = !toolbar.store.zenGuided; toolbar.refocus(); }
    }
}
