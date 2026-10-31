import QtQuick
import "Singletons"

// The text input. Both apps had one, at different heights, with different
// focus tells. Content follows the mono/grotesk boundary: a path or a key is
// mono, a search term is language and is grotesk.
Rectangle {
    id: field

    property alias text: input.text
    property string placeholder: ""
    property bool tabular: false      // mono for what a config file would hold
    property bool toolbar: false      // 36 in a toolbar, 30 in a sheet
    property bool readOnly: false     // shows a value the screen does not own
    property bool secret: false       // masked, for a password
    property bool clearOnEscape: true
    readonly property bool focused: input.activeFocus
    signal committed(string value)
    signal edited(string value)
    signal accepted()
    signal escaped()

    implicitHeight: toolbar ? 36 : 30
    radius: Tokens.radius
    color: "transparent"
    border.width: input.activeFocus ? 2 : Tokens.border
    border.color: input.activeFocus ? Tokens.ink : (hh.hovered ? Tokens.lineStrong : Tokens.line)
    Behavior on border.color { ColorAnimation { duration: Tokens.snap } }

    function clear() { input.text = "" }
    function grabFocus() { input.forceActiveFocus() }

    HoverHandler { id: hh; cursorShape: Qt.IBeamCursor }

    TextInput {
        id: input
        anchors.fill: parent
        anchors.leftMargin: 9
        anchors.rightMargin: 9
        verticalAlignment: Text.AlignVCenter
        color: Tokens.ink
        font.family: field.tabular ? Tokens.mono : Tokens.ui
        font.pixelSize: field.tabular ? 11 : 12
        selectByMouse: true
        echoMode: field.secret ? TextInput.Password : TextInput.Normal
        passwordCharacter: "\u2022"
        readOnly: field.readOnly
        clip: true
        onTextEdited: field.edited(text)
        onEditingFinished: field.committed(text)
        onAccepted: field.accepted()
        Keys.onEscapePressed: {
            if (field.clearOnEscape) {
                text = "";
                field.edited("");
            }
            field.escaped();
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
