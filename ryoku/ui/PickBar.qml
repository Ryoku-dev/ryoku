import QtQuick
import Ryoku.Ui.Singletons

// the foot bar of a catalogue cell. the catalogue itself is a Picker overlay:
// nine or more options is never inline, whatever the layout.
Rectangle {
    id: bar
    property string value: ""
    property int count: 0
    // optional key -> display label, the same map the Picker overlay takes, so
    // the closed bar reads "Polski" where the stored value is "pl".
    property var labels: ({})
    signal opened()

    implicitHeight: 26
    radius: Tokens.radius
    color: tap.pressed ? Tokens.tint16 : (bh.hovered ? Tokens.tint10 : "transparent")
    border.width: Tokens.border
    border.color: bh.hovered ? Tokens.lineStrong : Tokens.line
    Behavior on color { ColorAnimation { duration: Tokens.snap } }

    Text {
        anchors { left: parent.left; leftMargin: 9; verticalCenter: parent.verticalCenter }
        text: (bar.labels && bar.labels[bar.value] !== undefined) ? bar.labels[bar.value] : I18n.tr(bar.value)
        color: Tokens.ink
        font.family: Tokens.ui
        font.pixelSize: 11
        elide: Text.ElideRight
        width: parent.width - 70
    }
    Text {
        anchors { right: parent.right; rightMargin: 9; verticalCenter: parent.verticalCenter }
        // the chevron is the affordance; how many options the catalogue holds is
        // not something a reader needs on the row.
        text: "▾"
        color: Tokens.inkFaint
        font.family: Tokens.mono
        font.pixelSize: 9
    }
    HoverHandler { id: bh; cursorShape: Qt.PointingHandCursor }
    TapHandler { id: tap; onTapped: bar.opened() }
}
