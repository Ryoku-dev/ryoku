pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../ask/look"

Item {
    id: root

    required property real s
    property var item: null
    // false while the palette is closed but still loaded, so nothing animates unseen
    property bool live: true

    signal copyRequested()
    signal copyTextRequested()
    signal openRequested()
    signal starRequested()
    signal deleteRequested()

    readonly property real pad: Math.round(16 * root.s)
    readonly property string kind: root.item ? String(root.item.kind || "") : ""
    readonly property var words: root.item && root.item.words ? root.item.words : ({ text: "", marked: "" })
    readonly property bool hasWords: String(root.words.text || "").length > 0

    // a new item fades in, unless the hand is running down the list
    property string shownId: ""
    property real swappedAt: 0
    onItemChanged: {
        const id = root.item ? String(root.item.id) : "";
        if (id === root.shownId)
            return;
        const now = Date.now();
        const quick = now - root.swappedAt < 160;
        root.swappedAt = now;
        root.shownId = id;
        arrive.stop();
        if (quick || Look.still) {
            body.opacity = 1;
            body.lift = 0;
        } else {
            arrive.start();
        }
    }

    ParallelAnimation {
        id: arrive
        NumberAnimation { target: body; property: "opacity"; from: 0; to: 1; duration: Look.dur(Math.round(Look.stateMs * 1.6)); easing.type: Easing.OutCubic }
        NumberAnimation { target: body; property: "lift"; from: Look.material || Look.fluent ? 6 * root.s : 0; to: 0; duration: Look.dur(Math.round(Look.stateMs * 2.2)); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.arrive }
    }

    Shape {
        id: card
        anchors.fill: parent
        anchors.margins: Math.round(8 * root.s)
        anchors.leftMargin: 0
        s: root.s
        cutSize: Look.cutCard
        fill: Look.card
        strokeColor: Look.cardStroke
        strokeWidth: Qt.colorEqual(Look.cardStroke, "transparent") ? 0 : 1
        clip: true

        Item {
            id: body

            property real lift: 0

            x: root.pad
            y: root.pad + body.lift
            width: card.width - root.pad * 2
            height: card.height - root.pad * 2
            visible: root.item !== null

            Item {
                id: stage
                width: parent.width
                height: Math.max(0, facts.y - Math.round(14 * root.s))
                clip: true

                // never larger than its own pixels, and above the text read in it
                Item {
                    id: picture
                    width: parent.width
                    height: Math.max(0, stage.height - (picText.visible ? picText.height + Math.round(12 * root.s) : 0))
                    visible: root.kind === "image"

                    Shape {
                        readonly property real natW: Math.max(1, Number(root.item && root.item.thumbW) || 1)
                        readonly property real natH: Math.max(1, Number(root.item && root.item.thumbH) || 1)
                        readonly property real fit: Math.min(1, picture.width / natW, picture.height / natH)
                        anchors.centerIn: parent
                        width: Math.round(natW * fit)
                        height: Math.round(natH * fit)
                        s: root.s
                        cutSize: Look.cutRow
                        fill: Look.well
                        strokeColor: Look.lithic ? Lithic.vein : "transparent"
                        strokeWidth: Look.lithic ? 1 : 0
                        clip: true

                        Image {
                            anchors.fill: parent
                            source: root.kind === "image" && root.item && String(root.item.thumb || "").length > 0 ? "file://" + root.item.thumb : ""
                            sourceSize: Qt.size(parent.natW, parent.natH)
                            fillMode: Image.PreserveAspectFit
                            asynchronous: true
                            smooth: true
                            mipmap: true
                        }
                    }
                }

                Column {
                    id: picText
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.bottom: parent.bottom
                    visible: root.kind === "image" && (root.hasWords || (root.item !== null && root.item.reading === true))
                    spacing: Math.round(6 * root.s)

                    Micro {
                        id: picLabel
                        s: root.s
                        text: root.hasWords ? I18n.tr("Text in it") : I18n.tr("Reading the text in it")
                        SequentialAnimation on opacity {
                            running: picText.visible && !root.hasWords && root.live && !Look.still
                            loops: Animation.Infinite
                            alwaysRunToEnd: true
                            NumberAnimation { to: 0.4; duration: 700; easing.type: Easing.InOutSine }
                            NumberAnimation { to: 1; duration: 700; easing.type: Easing.InOutSine }
                        }
                    }
                    Text {
                        width: parent.width
                        visible: root.hasWords
                        text: String(root.words.marked || "").length > 0 ? root.words.marked : String(root.words.text || "")
                        textFormat: String(root.words.marked || "").length > 0 ? Text.StyledText : Text.PlainText
                        color: Lithic.fg
                        font.family: Look.sans
                        font.pixelSize: Math.round(12.5 * root.s)
                        lineHeight: Math.round(18 * root.s)
                        lineHeightMode: Text.FixedHeight
                        wrapMode: Text.Wrap
                        maximumLineCount: 4
                        elide: Text.ElideRight
                        renderType: Text.NativeRendering
                    }
                }

                Column {
                    width: parent.width
                    visible: root.kind === "colour"
                    spacing: Math.round(12 * root.s)

                    Shape {
                        width: parent.width
                        height: Math.min(Math.round(120 * root.s), Math.max(0, stage.height - colourValue.height - parent.spacing))
                        s: root.s
                        cutSize: Look.cutCard
                        fill: root.kind === "colour" ? String(root.item.swatch) : "transparent"
                        strokeColor: Lithic.alpha(Lithic.fg, 0.14)
                        strokeWidth: 1
                    }
                    Text {
                        id: colourValue
                        width: parent.width
                        text: root.kind === "colour" ? String(root.item.title || "") : ""
                        color: Lithic.fg
                        font.family: Look.mono
                        font.pixelSize: Math.round(20 * root.s)
                        elide: Text.ElideRight
                        renderType: Text.NativeRendering
                    }
                }

                Column {
                    width: parent.width
                    visible: root.kind === "link"
                    spacing: Math.round(8 * root.s)

                    Row {
                        spacing: Math.round(8 * root.s)
                        Symbol {
                            anchors.verticalCenter: parent.verticalCenter
                            s: root.s
                            name: "link"
                            size: 20
                            ink: Lithic.fg2
                        }
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            width: Math.min(implicitWidth, stage.width - 30 * root.s)
                            text: root.kind === "link" ? String(root.item.link.host || "") : ""
                            color: Lithic.fg
                            font.family: Look.sans
                            font.pixelSize: Math.round(18 * root.s)
                            font.weight: Font.Medium
                            elide: Text.ElideRight
                            renderType: Text.NativeRendering
                        }
                    }
                    Text {
                        width: parent.width
                        text: root.kind === "link" ? String(root.item.link.url || "") : ""
                        color: Lithic.fg2
                        font.family: Look.mono
                        font.pixelSize: Math.round(11.5 * root.s)
                        lineHeight: Math.round(17 * root.s)
                        lineHeightMode: Text.FixedHeight
                        wrapMode: Text.WrapAnywhere
                        maximumLineCount: 6
                        elide: Text.ElideRight
                        renderType: Text.NativeRendering
                    }
                }

                Column {
                    width: parent.width
                    visible: root.kind === "files"
                    spacing: Math.round(6 * root.s)

                    Repeater {
                        model: root.kind === "files" ? root.item.files : []

                        delegate: Row {
                            id: fileRow
                            required property var modelData
                            spacing: Math.round(8 * root.s)

                            Symbol {
                                anchors.verticalCenter: parent.verticalCenter
                                s: root.s
                                name: String(fileRow.modelData).indexOf(".") > 0 ? "draft" : "folder"
                                size: 17
                                ink: Lithic.fg2
                            }
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                width: Math.min(implicitWidth, stage.width - 28 * root.s)
                                text: String(fileRow.modelData)
                                color: Lithic.fg
                                font.family: Look.sans
                                font.pixelSize: Math.round(14 * root.s)
                                elide: Text.ElideMiddle
                                renderType: Text.NativeRendering
                            }
                        }
                    }
                    Text {
                        visible: text.length > 0
                        text: root.kind === "files" ? String(root.item.more || "") : ""
                        color: Lithic.fg3
                        font.family: Look.sans
                        font.pixelSize: Math.round(12 * root.s)
                        renderType: Text.NativeRendering
                    }
                }

                Column {
                    anchors.centerIn: parent
                    visible: root.kind === "binary"
                    spacing: Math.round(10 * root.s)

                    Symbol {
                        anchors.horizontalCenter: parent.horizontalCenter
                        s: root.s
                        name: "draft"
                        size: 44
                        ink: Lithic.fg3
                    }
                    Text {
                        anchors.horizontalCenter: parent.horizontalCenter
                        text: root.kind === "binary" ? String(root.item.mime || "") : ""
                        color: Lithic.fg2
                        font.family: Look.mono
                        font.pixelSize: Math.round(12 * root.s)
                        renderType: Text.NativeRendering
                    }
                }

                Text {
                    id: reading
                    readonly property bool shown: root.kind === "text" || root.kind === "command" || root.kind === "code"
                    readonly property int length: root.item ? String(root.item.text || "").length : 0
                    readonly property bool mono: root.item !== null && root.item.mono === true
                    // a few words read as a heading, a paragraph at reading size
                    readonly property real px: reading.length <= 48 ? (reading.mono ? 18 : 22)
                        : reading.length <= 120 ? (reading.mono ? 14.5 : 17) : (reading.mono ? 12.5 : 14)
                    readonly property real line: Math.round(reading.px * (reading.mono ? 1.45 : 1.42) * root.s)
                    width: parent.width
                    height: Math.max(0, stage.height - (cutNote.visible ? cutNote.height + Math.round(8 * root.s) : 0))
                    visible: reading.shown
                    text: reading.shown ? String(root.item.text || "") : ""
                    textFormat: Text.PlainText
                    color: Lithic.fg
                    font.family: root.item && root.item.mono ? Look.mono : Look.sans
                    font.pixelSize: Math.round(reading.px * root.s)
                    lineHeight: reading.line
                    lineHeightMode: Text.FixedHeight
                    wrapMode: Text.WrapAtWordBoundaryOrAnywhere
                    maximumLineCount: Math.max(1, Math.floor(reading.height / reading.line))
                    elide: Text.ElideRight
                    renderType: Text.NativeRendering
                }
                Text {
                    id: cutNote
                    anchors.top: reading.top
                    anchors.topMargin: Math.min(reading.contentHeight, reading.height) + Math.round(8 * root.s)
                    width: parent.width
                    visible: reading.shown && text.length > 0
                    text: reading.shown ? String(root.item.note || "") : ""
                    color: Lithic.fg3
                    font.family: Look.sans
                    font.pixelSize: Math.round(12 * root.s)
                    elide: Text.ElideRight
                    renderType: Text.NativeRendering
                }
            }

            Column {
                id: facts
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.bottom: actions.top
                anchors.bottomMargin: Math.round(14 * root.s)
                spacing: Math.round(5 * root.s)

                Rectangle {
                    width: parent.width
                    height: 1
                    color: Look.lithic ? Lithic.vein : Lithic.alpha(Lithic.fg, 0.1)
                }
                Repeater {
                    model: root.item ? root.item.facts || [] : []

                    delegate: Item {
                        id: fact
                        required property var modelData
                        width: facts.width
                        height: Math.max(key.implicitHeight, value.implicitHeight)

                        Micro {
                            id: key
                            anchors.verticalCenter: parent.verticalCenter
                            width: Math.round(parent.width * 0.3)
                            s: root.s
                            text: String(fact.modelData.label || "")
                        }
                        Text {
                            id: value
                            x: Math.round(parent.width * 0.32)
                            width: parent.width - x
                            text: String(fact.modelData.value || "")
                            color: Lithic.fg
                            font.family: fact.modelData.mono === true ? Look.mono : Look.sans
                            font.pixelSize: Math.round((fact.modelData.mono === true ? 11.5 : 12) * root.s)
                            font.features: ({ "tnum": 1 })
                            elide: Text.ElideRight
                            renderType: Text.NativeRendering
                        }
                    }
                }
            }

            Row {
                id: actions
                anchors.left: parent.left
                anchors.bottom: parent.bottom
                spacing: Math.round(6 * root.s)

                Chip {
                    s: root.s
                    tier: "accent"
                    glyph: "content_copy"
                    text: I18n.tr("Copy")
                    onClicked: root.copyRequested()
                }
                Chip {
                    visible: root.hasWords
                    s: root.s
                    tier: "line"
                    glyph: "text_fields"
                    text: I18n.tr("Copy text")
                    onClicked: root.copyTextRequested()
                }
                Chip {
                    visible: root.kind === "link"
                    s: root.s
                    tier: "line"
                    glyph: "open_in_new"
                    text: I18n.tr("Open")
                    onClicked: root.openRequested()
                }
                Chip {
                    s: root.s
                    tier: "line"
                    glyph: "star"
                    current: root.item !== null && root.item.starred === true
                    text: root.item && root.item.starred === true ? I18n.tr("Starred") : I18n.tr("Star")
                    onClicked: root.starRequested()
                }
                Chip {
                    s: root.s
                    glyph: "delete"
                    text: I18n.tr("Delete")
                    onClicked: root.deleteRequested()
                }
            }
        }
    }
}
