import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

Grid {
    id: root
    readonly property bool wide: true
    property var row: null
    property var value: []
    property var commit: null
    property var store: null
    readonly property var categories: [
        { id: "spelling", label: I18n.tr("Spelling") },
        { id: "grammar", label: I18n.tr("Grammar") },
        { id: "punctuation", label: I18n.tr("Punctuation") },
        { id: "capitalization", label: I18n.tr("Capitalization") },
        { id: "repetition", label: I18n.tr("Repetition") },
        { id: "wordiness", label: I18n.tr("Wordiness") },
        { id: "passive", label: I18n.tr("Passive voice") },
        { id: "weasel", label: I18n.tr("Vague wording") },
        { id: "adverbs", label: I18n.tr("Adverbs") },
        { id: "cliches", label: I18n.tr("Clichés") },
        { id: "readability", label: I18n.tr("Readability") },
        { id: "consistency", label: I18n.tr("Consistency") },
        { id: "formatting", label: I18n.tr("Formatting") },
        { id: "inclusive", label: I18n.tr("Inclusive language") }
    ]
    width: LithicTokens.size.settingsControlMax
    columns: Math.max(1, Math.floor((width + spacing) / (LithicTokens.size.settingsChoiceMin + spacing)))
    spacing: LithicTokens.space.s3
    readonly property real cellWidth: Math.min(LithicTokens.size.settingsControlMax,
        Math.max(0, (width - (columns - 1) * spacing) / columns))

    function setCategory(id, on) {
        var off = Array.from(root.value || []).filter(category => category !== id);
        if (!on)
            off.push(id);
        if (root.commit)
            root.commit(off);
    }

    Repeater {
        model: root.categories
        delegate: CutRect {
            id: categoryRow
            required property var modelData
            width: root.cellWidth
            height: Math.max(36, categoryLabel.implicitHeight + 2 * LithicTokens.space.s2)
            cut: Tone.rowCut
            fill: categoryToggle.hovered ? Tone.hover : Lithic.field
            strokeColor: Lithic.edge
            strokeWidth: LithicTokens.size.hairline
            Behavior on fill { ColorAnimation { duration: Tone.quick } }
            Label {
                id: categoryLabel
                anchors.left: parent.left
                anchors.leftMargin: LithicTokens.space.s2
                anchors.right: categoryToggle.left
                anchors.rightMargin: LithicTokens.space.s2
                anchors.verticalCenter: parent.verticalCenter
                text: categoryRow.modelData.label
                typeRole: "label"
                elide: Text.ElideRight
            }
            Toggle {
                id: categoryToggle
                anchors.right: parent.right
                anchors.rightMargin: LithicTokens.space.s2
                anchors.verticalCenter: parent.verticalCenter
                readonly property bool selected: Array.from(root.value || []).indexOf(categoryRow.modelData.id) < 0
                checked: selected
                Accessible.name: categoryRow.modelData.label
                onToggled: {
                    var want = checked;
                    checked = Qt.binding(() => categoryToggle.selected);
                    root.setCategory(categoryRow.modelData.id, want);
                }
            }
        }
    }
}
