import QtQuick
import QtQuick.Controls
import Ryoku.Ui.Singletons

// the catalogue overlay. a 25-entry font list cannot be browsed, so the filter
// is the interface; Enter takes the top match.
Rectangle {
    id: pick
    property string title: ""
    property var options: []
    property string current: ""
    // optional key -> display label; a key absent from the map keeps its own
    // text. Lets a caller show a friendly option name (the anchor picker's
    // "Auto (calm spot)") while still storing the raw key.
    property var labels: ({})
    signal chose(string key)
    signal dismissed()

    width: 330
    height: 330
    radius: Tokens.radius
    color: Tokens.paperLift
    border.width: Tokens.border
    border.color: Tokens.lineStrong

    function open() { q.text = ""; q.forceActiveFocus() }
    // the display label is what the reader sees, so it is what the filter has
    // to match: in the language list the key is "pl" and the label "Polski",
    // and typing either has to find it.
    function _display(o) {
        return (pick.labels && pick.labels[o] !== undefined) ? pick.labels[o] : I18n.tr(o);
    }
    readonly property var shown: options.filter(function (o) {
        if (q.text === "")
            return true;
        var needle = q.text.toLowerCase();
        return o.toLowerCase().indexOf(needle) >= 0
            || String(pick._display(o)).toLowerCase().indexOf(needle) >= 0;
    })

    Column {
        anchors.fill: parent
        anchors.margins: Tokens.s3
        spacing: Tokens.s2

        Row {
            width: parent.width
            Text {
                text: I18n.tr(pick.title).toUpperCase()
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: 10
                font.weight: Font.Medium
                font.letterSpacing: Tokens.trackLabel
            }
            Item { width: parent.width - 190; height: 1 }
            Text {
                text: pick.shown.length + " / " + pick.options.length
                color: Tokens.inkFaint
                font.family: Tokens.mono
                font.pixelSize: 9
            }
        }
        Rectangle {
            width: parent.width
            height: 30
            color: "transparent"
            radius: Tokens.radius
            border.width: q.activeFocus ? 2 : Tokens.border
            border.color: q.activeFocus ? Tokens.ink : Tokens.line
            TextInput {
                id: q
                anchors.fill: parent
                anchors.leftMargin: 8
                anchors.rightMargin: 8
                verticalAlignment: Text.AlignVCenter
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: 12
                selectByMouse: true
                Keys.onReturnPressed: if (pick.shown.length) pick.chose(pick.shown[0])
                Keys.onEscapePressed: pick.dismissed()
                Text {
                    anchors.fill: parent
                    visible: q.text === ""
                    text: I18n.tr("Filter…")
                    color: Tokens.inkMuted
                    font: q.font
                    verticalAlignment: Text.AlignVCenter
                }
            }
        }
        Flickable {
            width: parent.width
            height: parent.height - 76
            contentHeight: col.height
            clip: true
            ScrollBar.vertical: ScrollBar { contentItem: Rectangle { implicitWidth: 3; color: Tokens.line } }
            Column {
                id: col
                width: parent.width
                Repeater {
                    model: pick.shown
                    Rectangle {
                        required property string modelData
                        width: col.width
                        height: 30
                        color: rh.hovered ? Tokens.bone : "transparent"
                        Behavior on color { ColorAnimation { duration: 70 } }
                        Text {
                            anchors { left: parent.left; leftMargin: 8; verticalCenter: parent.verticalCenter }
                            text: pick._display(parent.modelData)
                            color: rh.hovered ? Tokens.inkOnBone : (pick.current === parent.modelData ? Tokens.ink : Tokens.inkDim)
                            font.family: Tokens.ui
                            font.pixelSize: 13
                        }
                        Text {
                            anchors { right: parent.right; rightMargin: 8; verticalCenter: parent.verticalCenter }
                            visible: pick.current === parent.modelData
                            text: "●"
                            color: rh.hovered ? Tokens.inkOnBone : Tokens.ink
                            font.pixelSize: 7
                        }
                        HoverHandler { id: rh; cursorShape: Qt.PointingHandCursor }
                        TapHandler { onTapped: pick.chose(parent.modelData) }
                    }
                }
            }
        }
    }
}
