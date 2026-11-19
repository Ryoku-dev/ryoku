pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../../../ask/look"
import "../../../parts"

// A pasted link, read: what it is, the choices it offers and where it will
// land. It takes the detail pane's place until Enter adds it or Esc sends it
// away. The view hands it words and choices; it draws them and reports presses.
Item {
    id: root

    required property real s
    // resolving | ready | error
    property string phase: "resolving"
    property string link: ""
    property string title: ""
    property string sub: ""
    property string thumb: ""
    property string glyph: "link"
    property string duration: ""
    // [{id, text}]
    property var chips: []
    property int chip: 0
    // [{name, note, glyph}] with picked: the chosen indices
    property string listTitle: ""
    property string listNote: ""
    property var entries: []
    property var picked: []
    property int cursor: -1
    // [{label, value}]
    property var facts: []
    property string note: ""
    property string reason: ""
    property string recovery: ""

    signal chipPicked(int index)
    signal entryToggled(int index)

    readonly property real pad: Math.round(20 * root.s)
    readonly property bool pictured: root.thumb.length > 0
    // a list of things to tick needs the room a full picture would take
    readonly property bool compact: root.entries.length > 0
    readonly property real entryHeight: Math.round(28 * root.s)

    // keep the highlighted entry in sight
    function reveal() {
        if (root.cursor < 0 || !entryList.visible)
            return;
        const top = column.y + entryList.y + entryRows.y + root.cursor * root.entryHeight;
        if (top < flick.contentY)
            flick.contentY = Math.max(0, top - root.entryHeight);
        else if (top + root.entryHeight > flick.contentY + flick.height)
            flick.contentY = Math.min(Math.max(0, flick.contentHeight - flick.height), top + root.entryHeight * 2 - flick.height);
    }
    onCursorChanged: root.reveal()

    Shape {
        anchors.fill: parent
        anchors.margins: Math.round(8 * root.s)
        anchors.leftMargin: 0
        s: root.s
        cutSize: Look.cutCard
        fill: Look.card
        strokeColor: Look.cardStroke
        strokeWidth: Qt.colorEqual(Look.cardStroke, "transparent") ? 0 : 1

        Flickable {
            id: flick
            anchors.fill: parent
            anchors.margins: 1
            contentWidth: width
            contentHeight: column.implicitHeight + root.pad * 2
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            interactive: contentHeight > height + 1
            Column {
                id: column
                x: root.pad
                y: root.pad
                width: flick.width - root.pad * 2
                spacing: Math.round(14 * root.s)

                // being read
                Column {
                    width: parent.width
                    visible: root.phase === "resolving"
                    spacing: Math.round(10 * root.s)

                    Line { width: parent.width; s: root.s; size: 15; text: I18n.tr("Reading the link"); font.weight: Font.Medium }
                    Line { width: parent.width; s: root.s; size: 12.5; text: root.link; ink: Lithic.fg2; elide: Text.ElideMiddle }
                    Busy { s: root.s; running: root.phase === "resolving" }
                }

                // could not be read
                Column {
                    width: parent.width
                    visible: root.phase === "error"
                    spacing: Math.round(6 * root.s)

                    Row {
                        spacing: Math.round(8 * root.s)
                        Symbol { anchors.verticalCenter: parent.verticalCenter; s: root.s; name: "error"; size: 18; ink: Lithic.jasper }
                        Line { anchors.verticalCenter: parent.verticalCenter; width: column.width - Math.round(28 * root.s); s: root.s; size: 15; text: root.reason; font.weight: Font.Medium; wrapMode: Text.WordWrap; maximumLineCount: 3 }
                    }
                    Line { width: parent.width; visible: text.length > 0; s: root.s; size: 12.5; text: root.recovery; ink: Lithic.fg2; wrapMode: Text.WordWrap; maximumLineCount: 4 }
                    Line { width: parent.width; s: root.s; size: 12; text: root.link; ink: Lithic.fg3; elide: Text.ElideMiddle }
                }

                Item {
                    width: parent.width
                    height: Math.round(width * 9 / 16)
                    visible: root.phase === "ready" && root.pictured && !root.compact

                    Picture {
                        anchors.fill: parent
                        s: root.s
                        source: root.thumb
                        glyph: root.glyph
                        glyphSize: 36
                    }
                    Shape {
                        anchors.right: parent.right
                        anchors.bottom: parent.bottom
                        anchors.margins: Math.round(6 * root.s)
                        visible: root.duration.length > 0
                        width: Math.ceil(length.implicitWidth + 10 * root.s)
                        height: Math.round(18 * root.s)
                        s: root.s
                        cutSize: Look.cutToken
                        fill: Lithic.alpha(Lithic.ground, 0.82)

                        Line { id: length; anchors.centerIn: parent; s: root.s; size: 11.5; figures: true; text: root.duration }
                    }
                }

                Row {
                    width: parent.width
                    visible: root.phase === "ready"
                    spacing: Math.round(12 * root.s)

                    Picture {
                        id: small
                        visible: !root.pictured || root.compact
                        width: Math.round((root.pictured ? 72 : 40) * root.s)
                        height: Math.round(40 * root.s)
                        s: root.s
                        source: root.compact ? root.thumb : ""
                        glyph: root.glyph
                    }
                    Column {
                        width: parent.width - (small.visible ? small.width + Math.round(12 * root.s) : 0)
                        spacing: Math.round(3 * root.s)

                        Line { width: parent.width; s: root.s; size: 16; text: root.title; font.weight: Font.Medium; wrapMode: Text.Wrap; maximumLineCount: 2 }
                        Line { width: parent.width; visible: text.length > 0; s: root.s; size: 12.5; text: root.sub; ink: Lithic.fg2 }
                    }
                }

                Column {
                    width: parent.width
                    visible: root.phase === "ready" && root.chips.length > 0
                    spacing: Math.round(8 * root.s)

                    Item {
                        width: parent.width
                        height: Math.round(20 * root.s)

                        Micro { anchors.verticalCenter: parent.verticalCenter; s: root.s; text: I18n.tr("Format") }
                        KeyHint {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            visible: root.chips.length > 1
                            s: root.s
                            hint: "Ctrl+Right"
                        }
                    }
                    Flow {
                        width: parent.width
                        spacing: Math.round(6 * root.s)

                        Repeater {
                            model: root.chips

                            delegate: Chip {
                                required property var modelData
                                required property int index

                                s: root.s
                                text: String(modelData.text || "")
                                tier: index === root.chip ? "accent" : "line"
                                onClicked: root.chipPicked(index)
                            }
                        }
                    }
                }

                Column {
                    id: entryList
                    width: parent.width
                    visible: root.phase === "ready" && root.entries.length > 0
                    spacing: Math.round(4 * root.s)

                    Item {
                        width: parent.width
                        height: Math.round(20 * root.s)

                        Micro { anchors.verticalCenter: parent.verticalCenter; s: root.s; text: root.listTitle }
                        Line {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            s: root.s
                            size: 12
                            figures: true
                            text: root.listNote
                            ink: Lithic.fg2
                        }
                    }
                    Item {
                        id: entryRows
                        width: parent.width
                        height: root.entries.length * root.entryHeight

                        Shape {
                            visible: root.cursor >= 0
                            y: Math.max(0, root.cursor) * root.entryHeight
                            width: parent.width
                            height: root.entryHeight
                            s: root.s
                            cutSize: Look.cutToken
                            fill: Look.plate
                            strokeColor: Look.plateStroke
                            strokeWidth: Qt.colorEqual(Look.plateStroke, "transparent") ? 0 : 1
                            Behavior on y {
                                NumberAnimation { duration: Look.dur(Look.plateMs); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.arrive }
                            }
                        }
                        Repeater {
                            model: root.entries

                            delegate: Item {
                                id: entry

                                required property var modelData
                                required property int index
                                readonly property bool on: root.picked.indexOf(entry.index) >= 0
                                readonly property bool lit: entry.index === root.cursor

                                y: entry.index * root.entryHeight
                                width: entryRows.width
                                height: root.entryHeight

                                Symbol {
                                    x: Math.round(6 * root.s)
                                    anchors.verticalCenter: parent.verticalCenter
                                    s: root.s
                                    name: entry.on ? "check_box" : "check_box_outline_blank"
                                    size: 16
                                    ink: entry.on ? (entry.lit ? Look.plateMark : Lithic.sel) : Lithic.fg3
                                    chosen: entry.on
                                }
                                Line {
                                    x: Math.round(30 * root.s)
                                    anchors.verticalCenter: parent.verticalCenter
                                    width: Math.max(0, entryNote.x - x - Math.round(10 * root.s))
                                    s: root.s
                                    size: 12.5
                                    text: String(entry.modelData.name || "")
                                    ink: !entry.on ? Lithic.fg3 : entry.lit ? Look.plateInk : Lithic.fg
                                    elide: entry.modelData.titled === true ? Text.ElideRight : Text.ElideMiddle
                                }
                                Line {
                                    id: entryNote
                                    x: parent.width - width - Math.round(6 * root.s)
                                    anchors.verticalCenter: parent.verticalCenter
                                    s: root.s
                                    size: 12
                                    figures: true
                                    text: String(entry.modelData.note || "")
                                    ink: Lithic.fg2
                                }
                                HoverHandler { cursorShape: Qt.PointingHandCursor }
                                TapHandler { onTapped: root.entryToggled(entry.index) }
                            }
                        }
                    }
                }

                Column {
                    width: parent.width
                    visible: root.phase === "ready" && root.facts.length > 0
                    spacing: Math.round(8 * root.s)

                    Repeater {
                        model: root.facts

                        delegate: Item {
                            id: fact

                            required property var modelData

                            width: column.width
                            height: Math.round(18 * root.s)

                            Micro { anchors.verticalCenter: parent.verticalCenter; width: Math.round(60 * root.s); s: root.s; text: String(fact.modelData.label || "") }
                            Line {
                                x: Math.round(68 * root.s)
                                anchors.verticalCenter: parent.verticalCenter
                                width: parent.width - x
                                s: root.s
                                size: 12.5
                                text: String(fact.modelData.value || "")
                                elide: Text.ElideMiddle
                            }
                        }
                    }
                }

                Line {
                    width: parent.width
                    visible: root.phase === "ready" && text.length > 0
                    s: root.s
                    size: 12
                    text: root.note
                    ink: Lithic.fg2
                    wrapMode: Text.WordWrap
                    maximumLineCount: 3
                }
            }
        }

        ScrollRail {
            anchors.right: parent.right
            anchors.rightMargin: Math.round(2 * root.s)
            anchors.top: parent.top
            anchors.bottom: parent.bottom
            anchors.topMargin: Math.round(6 * root.s)
            anchors.bottomMargin: Math.round(6 * root.s)
            flickable: flick
        }
    }
}
