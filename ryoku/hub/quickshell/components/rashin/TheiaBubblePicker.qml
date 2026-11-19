import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

Column {
    id: root
    readonly property bool wide: true

    property var row: null
    property var value: []
    property var commit: null
    property var store: null
    property int limit: 4
    property bool ordered: true
    property bool extrasOnly: false
    property var choices: root.actionChoices
    readonly property var actionChoices: [
        { id: "explain", feature: "", label: I18n.tr("Explain") },
        { id: "text", feature: "", label: I18n.tr("Text") },
        { id: "translate", feature: "", label: I18n.tr("Translate") },
        { id: "ask", feature: "", label: I18n.tr("Ask") },
        { id: "summarize", feature: "summarize", label: I18n.tr("Summarize") },
        { id: "deepread", feature: "deepread", label: I18n.tr("Deep read") },
        { id: "solve", feature: "math", label: I18n.tr("Solve") },
        { id: "learn", feature: "learn", label: I18n.tr("Learn") },
        { id: "proofread", feature: "proofread", label: I18n.tr("Fix writing") },
        { id: "rewrite", feature: "rewrite", label: I18n.tr("Rewrite") },
        { id: "tone", feature: "tone", label: I18n.tr("Tone") },
        { id: "fluent", feature: "fluent", label: I18n.tr("Fluent") },
        { id: "humanize", feature: "humanize", label: I18n.tr("Sound like you") },
        { id: "aicheck", feature: "aicheck", label: I18n.tr("Machine check") },
        { id: "inclusive", feature: "inclusive", label: I18n.tr("Inclusive") },
        { id: "reactions", feature: "reactions", label: I18n.tr("How it reads") },
        { id: "reply", feature: "reply", label: I18n.tr("Reply") },
        { id: "define", feature: "dictionary", label: I18n.tr("Dictionary") }
    ]
    readonly property var available: {
        ShellSettings.data;
        return Array.from(root.choices || []).filter(action => {
            if (action.feature === undefined)
                return true;
            if (!action.feature)
                return !root.extrasOnly;
            return ShellSettings.get("sight.features." + action.feature, false) === true;
        });
    }
    readonly property var selected: Array.from(root.value || []).filter((id, index, all) =>
        all.indexOf(id) === index && root.available.some(action => action.id === id)).slice(0, root.limit)

    width: LithicTokens.size.settingsControlMax
    spacing: LithicTokens.space.s2

    function labelFor(id) {
        var action = root.available.find(action => action.id === id);
        return action ? action.label : id;
    }
    function toggle(id) {
        var next = root.selected.slice();
        var index = next.indexOf(id);
        if (index >= 0)
            next.splice(index, 1);
        else if (next.length < root.limit)
            next.push(id);
        if (root.commit)
            root.commit(next);
    }
    function move(id, offset) {
        var next = root.selected.slice();
        var from = next.indexOf(id);
        var to = from + offset;
        if (from < 0 || to < 0 || to >= next.length)
            return;
        next.splice(from, 1);
        next.splice(to, 0, id);
        if (root.commit)
            root.commit(next);
    }

    Flow {
        width: parent.width
        visible: root.ordered && root.selected.length > 0
        spacing: LithicTokens.space.s2
        Repeater {
            model: root.ordered ? root.selected : []
            delegate: Row {
                id: selectedRow
                required property string modelData
                required property int index
                spacing: 0
                Chip {
                    text: I18n.tr("%1. %2").arg(selectedRow.index + 1).arg(root.labelFor(selectedRow.modelData))
                    maxWidth: root.width - 2 * LithicTokens.size.plaqueHeightSmall
                    selected: true
                    onClicked: root.toggle(selectedRow.modelData)
                    Keys.onLeftPressed: root.move(selectedRow.modelData, -1)
                    Keys.onRightPressed: root.move(selectedRow.modelData, 1)
                }
                IconPlaque {
                    small: true
                    kind: "quiet"
                    glyph: "chevron_left"
                    enabled: selectedRow.index > 0
                    accessibleName: I18n.tr("Move %1 earlier").arg(root.labelFor(selectedRow.modelData))
                    onClicked: root.move(selectedRow.modelData, -1)
                }
                IconPlaque {
                    small: true
                    kind: "quiet"
                    glyph: "chevron_right"
                    enabled: selectedRow.index < root.selected.length - 1
                    accessibleName: I18n.tr("Move %1 later").arg(root.labelFor(selectedRow.modelData))
                    onClicked: root.move(selectedRow.modelData, 1)
                }
            }
        }
    }
    Flow {
        width: parent.width
        spacing: LithicTokens.space.s2
        Repeater {
            model: root.available
            delegate: Chip {
                required property var modelData
                text: modelData.label
                selected: root.selected.indexOf(modelData.id) >= 0
                enabled: selected || root.selected.length < root.limit
                onClicked: root.toggle(modelData.id)
            }
        }
    }
    Label {
        width: parent.width
        text: I18n.tr("%1 of %2 selected").arg(root.selected.length).arg(root.limit)
        typeRole: "detail"
        inkRole: "fg3"
    }

    component Chip: T.AbstractButton {
        id: chip
        property real maxWidth: root.width
        property bool selected: false
        implicitWidth: Math.min(chip.maxWidth, chipLabel.implicitWidth + checkGlyph.width * 1.5 + 2 * LithicTokens.space.s3)
        implicitHeight: LithicTokens.size.plaqueHeightSmall
        hoverEnabled: true
        focusPolicy: Qt.StrongFocus
        opacity: enabled ? 1 : Lithic.disabledOpacity
        scale: down ? 0.98 : 1
        Accessible.role: Accessible.CheckBox
        Accessible.name: text
        Accessible.checked: selected
        Keys.onReturnPressed: chip.clicked()
        Keys.onEnterPressed: chip.clicked()
        Behavior on scale { NumberAnimation { duration: Tone.quick } }
        background: CutRect {
            id: chipFace
            cut: LithicTokens.cut.token
            fill: chip.down ? Lithic.pressRow(chip.selected ? Tone.select : Lithic.field)
                : chip.hovered ? Lithic.hoverFill(chip.selected ? Tone.select : Lithic.field)
                : chip.selected ? Tone.select : Lithic.field
            strokeColor: chip.selected ? Tone.accent : Lithic.edge
            strokeWidth: LithicTokens.size.hairline
            Behavior on fill { ColorAnimation { duration: Tone.quick } }
            FocusRing {
                anchors.fill: parent
                keyboardFocusVisible: chip.visualFocus
                cut: chipFace.cut
            }
        }
        contentItem: Item {
            Glyph {
                id: checkGlyph
                anchors.left: parent.left
                anchors.leftMargin: LithicTokens.space.s3
                anchors.verticalCenter: parent.verticalCenter
                name: "check"
                size: 16
                width: chip.selected ? 16 : 0
                opacity: chip.selected ? 1 : 0
                clip: true
                ink: Tone.selectInk
                Accessible.ignored: true
                Behavior on width { NumberAnimation { duration: Tone.quick } }
                Behavior on opacity { NumberAnimation { duration: Tone.quick } }
            }
            Label {
                id: chipLabel
                anchors.left: checkGlyph.right
                anchors.leftMargin: checkGlyph.width / 2
                anchors.right: parent.right
                anchors.rightMargin: LithicTokens.space.s3
                anchors.verticalCenter: parent.verticalCenter
                text: chip.text
                typeRole: "label"
                ink: chip.selected ? Tone.selectInk : Lithic.fg
                horizontalAlignment: Text.AlignHCenter
                verticalAlignment: Text.AlignVCenter
                elide: Text.ElideRight
            }
        }
    }
}
