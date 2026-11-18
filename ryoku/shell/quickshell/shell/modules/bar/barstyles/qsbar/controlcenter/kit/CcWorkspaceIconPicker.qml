import QtQuick
import "../../modules"
import Ryoku.Ui
import Ryoku.Ui.Singletons

// The icons marker's editor: a cell per workspace, then the symbols for the
// selected one. Every write goes through root.setWorkspaceIcon, so the bar, this
// editor and the preview read the one qsbar.workspaceIcons list. `root` is the
// qsbar Theme; `tk` its geometry.
Column {
    id: picker
    property var root: null
    property var tk: null

    property int slot: 1
    readonly property int slotCount: picker.root && picker.root.workspaceMode === "5" ? 5 : 10
    readonly property string current: picker.root ? picker.root.workspaceIcon(picker.slot) : ""
    readonly property real cellH: picker.tk ? picker.tk.rowH : Tokens.px(40)
    readonly property real cellGap: picker.tk ? picker.tk.gap / 2 : Tokens.s2
    readonly property color ink: picker.root ? picker.root.seal : Tokens.ink

    spacing: picker.tk ? picker.tk.gap : Tokens.s3

    onSlotCountChanged: if (picker.slot > picker.slotCount) picker.slot = picker.slotCount

    function choose(name) {
        if (picker.root) picker.root.setWorkspaceIcon(picker.slot, name)
    }
    // A known name shapes into one em-wide glyph; an unknown one lays out as
    // letters, which the bar would draw as text, so it is not saved.
    function commitText(text) {
        if (!picker.root) return
        var name = picker.root.workspaceIconName(text)
        glyphProbe.text = name
        if (name === "" || glyphProbe.advanceWidth <= glyphProbe.font.pixelSize * 1.2)
            picker.choose(name)
        nameField.text = Qt.binding(() => picker.current)
    }
    // A click takes no focus, so the field never reports its edit; save what was
    // typed before the binding refills the field with the next workspace's symbol.
    function select(n) {
        if (nameField.focused && nameField.text !== picker.current) picker.commitText(nameField.text)
        picker.slot = n
    }

    TextMetrics {
        id: glyphProbe
        font.family: "Material Symbols Rounded"
        font.pixelSize: Tokens.fRow
    }

    // the workspaces, each wearing its current symbol (or its number)
    Grid {
        id: slotGrid
        width: parent.width
        columns: 10
        spacing: picker.cellGap
        readonly property real cellW: (width - spacing * (columns - 1)) / columns

        Repeater {
            model: picker.slotCount
            delegate: CcGlyphTile {
                required property int index
                width: slotGrid.cellW
                height: picker.cellH
                ink: picker.ink
                icon: picker.root ? picker.root.workspaceIcon(index + 1) : ""
                fallback: String(index + 1)
                selected: picker.slot === index + 1
                onPicked: picker.select(index + 1)
            }
        }
    }

    UiText {
        width: parent.width
        text: I18n.tr("Workspace %1").arg(picker.slot)
        color: Tokens.inkMuted
        font.family: Tokens.mono
        font.pixelSize: Tokens.fTiny
        font.letterSpacing: Tokens.trackLabel
    }

    // its choices: the number first, then the offered symbols
    Grid {
        id: symbolGrid
        width: parent.width
        columns: 8
        spacing: picker.cellGap
        readonly property real cellW: (width - spacing * (columns - 1)) / columns

        Repeater {
            model: [""].concat(picker.root ? picker.root.workspaceIconOptions : [])
            delegate: CcGlyphTile {
                required property string modelData
                width: symbolGrid.cellW
                height: picker.cellH
                ink: picker.ink
                icon: modelData
                fallback: String(picker.slot)
                selected: picker.current === modelData
                onPicked: picker.choose(modelData)
            }
        }
    }

    Field {
        id: nameField
        width: Math.min(parent.width, 260)
        tabular: true
        placeholder: I18n.tr("Any Material Symbols name")
        text: picker.current
        onCommitted: (v) => picker.commitText(v)
    }
}
