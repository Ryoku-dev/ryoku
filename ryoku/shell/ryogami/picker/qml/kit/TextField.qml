import QtQuick
import Lithic

// variant: "workbench" (default), "field" or "ghost"; committed fires on Enter or focus-out.
// A focus scope, so forceActiveFocus() on the field lands in the text input.
FocusScope {
    id: field

    property alias text: input.text
    property string placeholder: ""
    property string variant: "workbench"
    property string glyph: ""
    readonly property bool editing: input.activeFocus

    signal edited(string text)
    signal committed(string text)

    implicitHeight: input.implicitHeight + 12 * Theme.scale
    implicitWidth: 160 * Theme.scale

    readonly property bool _ghost: field.variant === "ghost"
    readonly property color _bg: field.variant === "field"
        ? Theme.withAlpha(Theme.surfaceText, 0.05)
        : field._ghost ? "transparent"
        : Theme.withAlpha(Theme.surfaceText, 0.05)
    readonly property color _border: field.variant === "field"
        ? Theme.withAlpha(Theme.outline, 0.4)
        : field._ghost ? "transparent"
        : Theme.withAlpha(Theme.outline, 0.4)
    readonly property color _placeholder: Theme.withAlpha(Theme.surfaceText, 0.42)
    readonly property color _selection: Theme.withAlpha(Theme.surfaceText, 0.24)

    CutRect {
        anchors.fill: parent
        cut: LithicTokens.cut.small * Theme.scale
        fill: field._ghost ? "transparent" : Lithic.field
        strokeWidth: input.activeFocus ? 2 : (field._ghost ? 0 : 1)
        strokeColor: input.activeFocus ? Lithic.sel
                   : field.enabled ? Lithic.alpha(Lithic.fg, 0.12)
                   : Lithic.alpha(Lithic.fg, 0.06)
        Behavior on strokeColor { ColorAnimation { duration: Theme.fast } }
    }

    Text {
        visible: field.glyph.length > 0
        id: leadGlyph
        anchors.left: parent.left
        anchors.leftMargin: 7 * Theme.scale
        anchors.verticalCenter: parent.verticalCenter
        text: field.glyph
        font.family: Theme.icon
        font.pixelSize: Theme.fontLabel
        color: Theme.withAlpha(Theme.surfaceText, input.activeFocus ? 0.9 : 0.5)
        renderType: Text.NativeRendering
        Behavior on color { ColorAnimation { duration: Theme.fast } }
    }

    TextInput {
        id: input
        focus: true
        anchors.fill: parent
        anchors.leftMargin: (field.glyph.length > 0 ? 26 : 9) * Theme.scale
        anchors.rightMargin: 9 * Theme.scale
        verticalAlignment: TextInput.AlignVCenter
        clip: true
        enabled: field.enabled
        font.family: Theme.sans
        font.weight: Font.Medium
        font.pixelSize: Theme.fontLabel
        color: field.enabled ? Theme.surfaceText : Theme.withAlpha(Theme.surfaceText, 0.4)
        selectionColor: field._selection
        selectedTextColor: Theme.surfaceText
        selectByMouse: true
        renderType: Text.NativeRendering
        onTextEdited: field.edited(text)
        onEditingFinished: field.committed(text)
        Keys.onReturnPressed: field.committed(text)
        Keys.onEnterPressed: field.committed(text)

        Text {
            anchors.fill: parent
            visible: input.text.length === 0
            verticalAlignment: Text.AlignVCenter
            text: field.placeholder
            font: input.font
            color: field._placeholder
            renderType: Text.NativeRendering
            elide: Text.ElideRight
        }
    }
}
