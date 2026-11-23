pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../../../ask/look"
import "../../../parts"

// The selected transfer, read closer: its map at reading size (a landing bucket
// fills from its left edge, so the write heads show), its rate over the last
// minute, the figures and its files. The view hands it words; it draws them.
Item {
    id: root

    required property real s
    property string owner: ""
    property string title: ""
    property string sub: ""
    // the figure at the head: a percent, or the state in a word
    property string badge: ""
    property string badgeUnit: ""
    property string badgeTone: ""
    property string tone: ""
    property var fills: []
    property bool swept: true
    property bool mapped: true
    // what stands beside the map's name: connections, or peers and seeds
    property string headline: ""
    property var samples: []
    property bool traced: false
    property string rate: ""
    // [{label, value}]
    property var facts: []
    // why it failed and what helps, when it did
    property string reason: ""
    property string recovery: ""
    // [{name, note, selected}]; pickable ones toggle through fileToggled
    property var files: []
    property bool pickable: false
    property bool compact: false
    // short marks under the name: [{glyph, text}]
    property var marks: []
    // the minute's average and peak, already worded
    property string average: ""
    property string peak: ""
    // this download's own speed limit, worded, and whether it can take one
    property bool limitable: false
    property string limitText: ""
    // the field is taking a new link for this download
    property bool editingLink: false
    // what else can be done with it: [{id, text, glyph}]
    property var offers: []

    signal fileToggled(int index)
    signal limitStepped(int delta)
    signal offered(string id)

    readonly property real pad: Math.round(20 * root.s)
    implicitHeight: column.implicitHeight + Math.round(38 * root.s)
    readonly property color badgeInk: root.badgeTone === "error" ? Lithic.jasper
        : root.badgeTone === "done" ? Lithic.jade
        : root.badgeTone === "paused" ? Lithic.amber
        : root.badgeTone === "idle" ? Lithic.fg2 : Lithic.fg

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
            contentHeight: column.implicitHeight + Math.round(30 * root.s)
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            interactive: contentHeight > height + 1

            Column {
                id: column
                x: root.pad
                y: Math.round(14 * root.s)
                width: flick.width - root.pad * 2
                spacing: Math.round((root.compact ? 10 : 16) * root.s)

                Item {
                    width: parent.width
                    height: Math.max(names.implicitHeight, figure.height)

                    Column {
                        id: names
                        width: parent.width - (figure.visible ? figure.width + Math.round(14 * root.s) : 0)
                        spacing: Math.round(3 * root.s)

                        Line {
                            width: parent.width
                            s: root.s
                            size: 15
                            text: root.title
                            font.weight: Font.Medium
                            wrapMode: Text.Wrap
                            maximumLineCount: 2
                        }
                        Line {
                            width: parent.width
                            visible: text.length > 0
                            s: root.s
                            size: 12.5
                            text: root.sub
                            ink: Lithic.fg2
                        }
                    }
                    Row {
                        id: figure
                        x: parent.width - width
                        visible: root.badge.length > 0
                        spacing: Math.round(1 * root.s)

                        Line {
                            id: badgeText
                            s: root.s
                            size: root.badgeUnit.length > 0 ? 30 : 13
                            figures: root.badgeUnit.length > 0
                            text: root.badge
                            ink: root.badgeInk
                            font.weight: root.badgeUnit.length > 0 ? Font.Normal : Font.Medium
                        }
                        Line {
                            anchors.baseline: badgeText.baseline
                            visible: root.badgeUnit.length > 0
                            s: root.s
                            size: 15
                            figures: true
                            text: root.badgeUnit
                            ink: Lithic.fg2
                        }
                    }
                }

                Flow {
                    width: parent.width
                    visible: root.marks.length > 0
                    spacing: Math.round(6 * root.s)

                    Repeater {
                        model: root.marks

                        delegate: Shape {
                            id: mark

                            required property var modelData

                            width: Math.ceil(markRow.implicitWidth + 14 * root.s)
                            height: Math.round(22 * root.s)
                            s: root.s
                            cutSize: Look.cutToken
                            fill: Look.cap
                            strokeColor: Look.capStroke
                            strokeWidth: Qt.colorEqual(Look.capStroke, "transparent") ? 0 : 1
                            opacity: 0
                            Component.onCompleted: opacity = 1
                            Behavior on opacity {
                                NumberAnimation { duration: Look.dur(Look.stateMs * 2) }
                            }

                            Row {
                                id: markRow
                                anchors.centerIn: parent
                                spacing: Math.round(4 * root.s)

                                Symbol {
                                    anchors.verticalCenter: parent.verticalCenter
                                    s: root.s
                                    name: String(mark.modelData.glyph || "")
                                    size: 14
                                    ink: Lithic.jade
                                }
                                Line {
                                    anchors.verticalCenter: parent.verticalCenter
                                    s: root.s
                                    size: 11.5
                                    text: String(mark.modelData.text || "")
                                    ink: Lithic.fg2
                                }
                            }
                        }
                    }
                }

                Shape {
                    width: parent.width
                    height: editNote.implicitHeight + Math.round(18 * root.s)
                    visible: root.editingLink
                    s: root.s
                    cutSize: Look.cutRow
                    fill: Look.plate
                    strokeColor: Look.plateStroke
                    strokeWidth: Qt.colorEqual(Look.plateStroke, "transparent") ? 0 : 1

                    Symbol {
                        id: editGlyph
                        x: Math.round(10 * root.s)
                        anchors.verticalCenter: parent.verticalCenter
                        s: root.s
                        name: "link"
                        size: 18
                        ink: Look.plateMark
                    }
                    Line {
                        id: editNote
                        x: editGlyph.x + editGlyph.width + Math.round(8 * root.s)
                        anchors.verticalCenter: parent.verticalCenter
                        width: parent.width - x - Math.round(10 * root.s)
                        s: root.s
                        size: 12.5
                        text: I18n.tr("Type the new link in the field above. Enter uses it, Esc keeps the old one.")
                        ink: Look.plateInk
                        wrapMode: Text.WordWrap
                        maximumLineCount: 3
                    }
                }

                Column {
                    width: parent.width
                    visible: root.reason.length > 0
                    spacing: Math.round(4 * root.s)

                    Line {
                        width: parent.width
                        s: root.s
                        size: 13.5
                        text: root.reason
                        ink: Lithic.jasper
                        wrapMode: Text.WordWrap
                        maximumLineCount: 4
                    }
                    Line {
                        width: parent.width
                        visible: text.length > 0
                        s: root.s
                        size: 12.5
                        text: root.recovery
                        ink: Lithic.fg2
                        wrapMode: Text.WordWrap
                        maximumLineCount: 4
                    }
                }

                Flow {
                    width: parent.width
                    visible: root.offers.length > 0 && !root.editingLink
                    spacing: Math.round(6 * root.s)

                    Repeater {
                        model: root.offers

                        delegate: Chip {
                            required property var modelData

                            s: root.s
                            tier: "line"
                            glyph: String(modelData.glyph || "")
                            text: String(modelData.text || "")
                            onClicked: root.offered(String(modelData.id))
                        }
                    }
                }

                Item {
                    id: mapBlock
                    width: parent.width
                    visible: root.mapped || root.facts.length > 0
                    readonly property bool beside: root.mapped && width >= Math.round(500 * root.s)
                    height: beside ? Math.max(mapColumn.implicitHeight, factTable.implicitHeight)
                        : (root.mapped ? mapColumn.implicitHeight + Math.round(16 * root.s) : 0) + factTable.implicitHeight

                    Column {
                        id: mapColumn
                        width: mapBlock.beside ? parent.width - Math.round(166 * root.s) : parent.width
                        visible: root.mapped
                        spacing: Math.round(7 * root.s)
                        Item {
                            width: parent.width
                            height: Math.round(16 * root.s)
                            Micro { anchors.verticalCenter: parent.verticalCenter; s: root.s; text: I18n.tr("Pieces") }
                            Line {
                                anchors.right: parent.right
                                anchors.verticalCenter: parent.verticalCenter
                                width: Math.min(implicitWidth, parent.width * 0.7)
                                s: root.s
                                size: 12
                                figures: true
                                text: root.headline
                                ink: Lithic.fg2
                            }
                        }
                        PieceMap {
                            s: root.s
                            owner: root.owner
                            columns: 16
                            rows: 4
                            cellHeight: root.compact ? Math.round(16 * root.s) : 0
                            sweep: root.swept
                            available: mapColumn.width
                            pieces: root.fills
                            tone: root.tone
                        }
                    }
                    Flow {
                        id: factTable
                        x: mapBlock.beside ? mapColumn.width + Math.round(16 * root.s) : 0
                        y: mapBlock.beside || !root.mapped ? 0 : mapColumn.implicitHeight + Math.round(16 * root.s)
                        width: mapBlock.beside ? Math.round(150 * root.s) : parent.width
                        visible: root.facts.length > 0
                        readonly property int across: mapBlock.beside ? 1 : Math.max(1, Math.min(3, root.facts.length))
                        Repeater {
                            model: root.facts
                            delegate: Column {
                                id: fact
                                required property var modelData
                                required property int index
                                width: factTable.across === 3 ? Math.floor(factTable.width * (fact.index % 3 === 0 ? 0.5 : 0.25)) : Math.floor(factTable.width / factTable.across)
                                bottomPadding: Math.round(10 * root.s)
                                spacing: Math.round(3 * root.s)
                                Micro { width: fact.width - Math.round(8 * root.s); s: root.s; text: String(fact.modelData.label || "") }
                                Line {
                                    width: fact.width - Math.round(8 * root.s)
                                    s: root.s
                                    size: 13
                                    figures: fact.modelData.plain !== true
                                    text: String(fact.modelData.value || "")
                                    wrapMode: Text.NoWrap
                                    maximumLineCount: 1
                                }
                            }
                        }
                    }
                }

                Column {
                    width: parent.width
                    visible: root.traced
                    spacing: Math.round(7 * root.s)

                    Item {
                        width: parent.width
                        height: Math.round(16 * root.s)

                        Micro { anchors.verticalCenter: parent.verticalCenter; s: root.s; text: I18n.tr("Speed, last 60 s") }
                        Line {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            s: root.s
                            size: 13
                            figures: true
                            text: root.rate
                        }
                    }
                    SpeedTrace {
                        width: parent.width
                        height: Math.round((root.compact ? 40 : 72) * root.s)
                        s: root.s
                        samples: root.samples
                        visible: root.traced
                    }
                    Row {
                        visible: root.average.length > 0
                        spacing: Math.round(18 * root.s)

                        Repeater {
                            model: [{ label: I18n.tr("Average"), value: root.average }, { label: I18n.tr("Peak"), value: root.peak }]

                            delegate: Row {
                                id: paceItem

                                required property var modelData

                                spacing: Math.round(6 * root.s)

                                Micro { anchors.verticalCenter: parent.verticalCenter; s: root.s; text: String(paceItem.modelData.label) }
                                Line {
                                    anchors.verticalCenter: parent.verticalCenter
                                    s: root.s
                                    size: 12.5
                                    figures: true
                                    text: String(paceItem.modelData.value)
                                }
                            }
                        }
                    }
                }

                Item {
                    width: parent.width
                    height: Math.round(28 * root.s)
                    visible: root.limitable

                    Micro { anchors.verticalCenter: parent.verticalCenter; s: root.s; text: I18n.tr("Speed limit for this download") }
                    Row {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: Math.round(4 * root.s)

                        Chip {
                            anchors.verticalCenter: parent.verticalCenter
                            s: root.s
                            tier: "line"
                            glyph: "remove"
                            onClicked: root.limitStepped(-1)
                        }
                        Line {
                            id: limitValue
                            anchors.verticalCenter: parent.verticalCenter
                            width: Math.max(implicitWidth, Math.round(78 * root.s))
                            horizontalAlignment: Text.AlignHCenter
                            s: root.s
                            size: 13
                            figures: true
                            text: root.limitText
                            Behavior on text {
                                SequentialAnimation {
                                    NumberAnimation { target: limitValue; property: "opacity"; to: 0.3; duration: Look.dur(Look.stateMs / 2) }
                                    PropertyAction {}
                                    NumberAnimation { target: limitValue; property: "opacity"; to: 1; duration: Look.dur(Look.stateMs) }
                                }
                            }
                        }
                        Chip {
                            anchors.verticalCenter: parent.verticalCenter
                            s: root.s
                            tier: "line"
                            glyph: "add"
                            onClicked: root.limitStepped(1)
                        }
                    }
                }

                Column {
                    width: parent.width
                    visible: root.files.length > 0
                    spacing: Math.round(4 * root.s)

                    Micro { s: root.s; text: I18n.tr("Files") }
                    Repeater {
                        model: root.files

                        delegate: Shape {
                            id: file

                            required property var modelData
                            required property int index
                            readonly property bool on: file.modelData.selected !== false

                            width: column.width
                            height: Math.round(26 * root.s)
                            s: root.s
                            cutSize: Look.cutToken
                            fill: root.pickable && fileTap.pressed ? Look.press : root.pickable && fileHover.hovered ? Look.hover : "transparent"

                            Symbol {
                                id: tick
                                anchors.verticalCenter: parent.verticalCenter
                                s: root.s
                                name: !root.pickable ? String(file.modelData.glyph || "draft") : file.on ? "check_box" : "check_box_outline_blank"
                                size: 16
                                ink: root.pickable && file.on ? Lithic.sel : Lithic.fg3
                                chosen: root.pickable && file.on
                            }
                            Line {
                                x: Math.round(24 * root.s)
                                anchors.verticalCenter: parent.verticalCenter
                                width: Math.max(0, note.x - x - Math.round(10 * root.s))
                                s: root.s
                                size: 12.5
                                text: String(file.modelData.name || "")
                                ink: file.on ? Lithic.fg : Lithic.fg3
                                elide: Text.ElideMiddle
                            }
                            Line {
                                id: note
                                x: parent.width - width
                                anchors.verticalCenter: parent.verticalCenter
                                s: root.s
                                size: 12
                                figures: true
                                text: String(file.modelData.note || "")
                                ink: Lithic.fg2
                            }
                            HoverHandler { id: fileHover; enabled: root.pickable; cursorShape: Qt.PointingHandCursor }
                            TapHandler { id: fileTap; enabled: root.pickable; onTapped: root.fileToggled(file.index) }
                        }
                    }
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
