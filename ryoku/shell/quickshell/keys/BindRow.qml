pragma ComponentBehavior: Bound
import QtQuick
import Ryoku.Ui.Singletons

// One shortcut line: description on the left, its key caps ("+"-joined) on the
// right. A row is one line at rest; on hover, or while Kit is searching, its
// one-sentence hint drops in as a muted second line. A bind the running
// compositor cannot honour still lists, dimmed, with a small tag naming the
// reason on hover. No rules or fills: the rows are spaced, not packed.
Item {
    id: row

    property var keys: []
    property string desc: ""
    property string hint: ""
    property int taps: 0
    property bool whileHighlighting: false
    // A short reason, set only when the running compositor cannot express this
    // bind. Empty means the bind is honoured.
    property string unhonored: ""
    // Kit search reveals the full sentence for every matching row.
    property bool searching: false

    readonly property bool unavailable: row.unhonored.length > 0
    readonly property bool showHint: row.hint.length > 0 && (rowHover.hovered || row.searching)

    implicitHeight: row.showHint ? 52 : 38

    HoverHandler { id: rowHover }

    Column {
        id: labelCol
        anchors.left: parent.left
        anchors.right: caps.left
        anchors.rightMargin: Tokens.s4
        anchors.verticalCenter: parent.verticalCenter
        spacing: 2

        Item {
            id: descLine
            width: labelCol.width
            height: descText.implicitHeight

            // "not here": a bind this compositor cannot honour. Muted, so it never
            // competes with the live shortcuts; the reason rides a hover tooltip.
            Rectangle {
                id: tag
                visible: row.unavailable
                anchors.right: parent.right
                anchors.verticalCenter: descText.verticalCenter
                width: tagText.implicitWidth + Tokens.s2
                height: tagText.implicitHeight + Tokens.s1
                radius: Tokens.radius
                color: "transparent"
                border.width: Tokens.border
                border.color: Tokens.lineSoft

                Text {
                    id: tagText
                    anchors.centerIn: parent
                    text: I18n.tr("not here")
                    color: Tokens.inkFaint
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fTiny
                    font.letterSpacing: Tokens.trackLabel
                }

                HoverHandler {
                    id: tagHover
                    onHoveredChanged: {
                        if (tagHover.hovered && row.unhonored.length > 0)
                            tipDelay.restart();
                        else {
                            tipDelay.stop();
                            reasonTip.opacity = 0;
                        }
                    }
                }
                Timer {
                    id: tipDelay
                    interval: 400
                    onTriggered: reasonTip.opacity = 1
                }

                Rectangle {
                    id: reasonTip
                    enabled: false
                    visible: row.unhonored.length > 0
                    opacity: 0
                    z: 100
                    anchors.bottom: parent.top
                    anchors.bottomMargin: Tokens.s1
                    anchors.right: parent.right
                    width: reasonText.implicitWidth + Tokens.s3
                    height: reasonText.implicitHeight + Tokens.s2
                    radius: Tokens.radius
                    color: Tokens.paperLift
                    border.width: Tokens.border
                    border.color: Tokens.line
                    Behavior on opacity { NumberAnimation { duration: Tokens.snap } }

                    Text {
                        id: reasonText
                        anchors.centerIn: parent
                        text: I18n.tr(row.unhonored)
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                    }
                }
            }

            Text {
                id: descText
                anchors.left: parent.left
                anchors.right: tag.visible ? tag.left : parent.right
                anchors.rightMargin: tag.visible ? Tokens.s2 : 0
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr(row.desc)
                color: row.unavailable ? Tokens.inkFaint : Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: Tokens.fBody
                elide: Text.ElideRight
            }
        }

        Text {
            id: hintText
            visible: row.showHint
            width: labelCol.width
            text: I18n.tr(row.hint)
            color: Tokens.inkFaint
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            elide: Text.ElideRight
        }
    }

    Row {
        id: caps
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        spacing: Tokens.s1
        // A bind that cannot fire on this compositor reads as inert, not active.
        opacity: row.unavailable ? 0.5 : 1

        Text {
            visible: row.whileHighlighting
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("Hold")
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
        }
        Repeater {
            model: row.keys
            delegate: Row {
                id: capWrap
                required property string modelData
                required property int index
                spacing: Tokens.s1
                Text {
                    visible: capWrap.index > 0
                    anchors.verticalCenter: parent.verticalCenter
                    text: "+"
                    color: Tokens.inkFaint
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fMicro
                }
                KeyCap {
                    anchors.verticalCenter: parent.verticalCenter
                    text: capWrap.modelData
                }
            }
        }
        Text {
            visible: row.taps > 0 || row.whileHighlighting
            anchors.verticalCenter: parent.verticalCenter
            text: row.whileHighlighting ? I18n.tr("while highlighting") : "×" + row.taps
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
        }
    }
}
