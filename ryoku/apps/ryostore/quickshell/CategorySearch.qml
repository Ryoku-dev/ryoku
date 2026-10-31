import QtQuick
import Ryoku.Ui.Singletons

// A local category filter. Typing narrows the loaded category in place; the
// header search still reaches the whole store or Flathub.
Item {
    id: field

    property string value: ""
    property string placeholder: I18n.tr("Search this category")
    property string fieldObjectName: "ryostore-category-search-field"

    signal edited(string value)
    signal cleared()
    signal accepted()

    implicitHeight: 36

    onValueChanged: if (input.text !== value) input.text = value

    Rectangle {
        anchors.fill: parent
        radius: Tokens.radius
        color: input.activeFocus ? Tokens.tint5 : Tokens.paper
        border.width: Tokens.border
        border.color: input.activeFocus ? Tokens.lineStrong : Tokens.line

        Text {
            id: glyph
            anchors { left: parent.left; leftMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
            text: "⌕"
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fRow
        }

        TextInput {
            id: input
            objectName: field.fieldObjectName
            anchors {
                left: glyph.right; leftMargin: Tokens.s2
                right: parent.right; rightMargin: Tokens.s3
                verticalCenter: parent.verticalCenter
            }
            color: Tokens.ink
            selectionColor: Tokens.tint16
            selectedTextColor: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: Tokens.fBody
            clip: true
            activeFocusOnTab: true
            Accessible.role: Accessible.EditableText
            Accessible.name: field.placeholder
            onTextEdited: field.edited(text)
            Keys.onEscapePressed: event => {
                input.text = "";
                field.cleared();
                event.accepted = true;
            }
            Keys.onReturnPressed: event => {
                field.accepted();
                event.accepted = true;
            }

            Text {
                anchors.fill: parent
                visible: input.text === ""
                text: field.placeholder
                color: Tokens.inkMuted
                font: input.font
                verticalAlignment: Text.AlignVCenter
                elide: Text.ElideRight
            }
        }
    }

    HoverHandler { cursorShape: Qt.IBeamCursor }
    TapHandler { onTapped: input.forceActiveFocus() }
}
