pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import "../../look"

// The selected result, read closer: its picture when it has one, where it is
// from, the fuller text and the source's facts. It shows what the list already
// knows at once and takes the daemon's detail when that arrives. A definition
// brings its examples and the credit its licence asks for, as a link.
Item {
    id: root

    required property real s
    // the result item and, once fetched, its detail ({title, url, text, site, thumb, facts, error})
    property var item: null
    property var detail: null
    // facts the view knows itself, after the source's: [{label, value}]
    property var extra: []
    property var examples: []
    property string credit: ""
    property string creditUrl: ""

    signal linkActivated(string url)

    readonly property bool matched: root.detail !== null && root.item !== null && root.detail.key === root.item.key
    readonly property var facts: (root.matched && root.detail.facts ? root.detail.facts
        : root.item && root.item.meta && root.item.meta.facts ? root.item.meta.facts : []).concat(root.extra)
    readonly property string picture: {
        if (root.matched && String(root.detail.thumb || "").length > 0)
            return String(root.detail.thumb);
        return root.item && root.item.kind === "video" ? String(root.item.thumb || "") : "";
    }
    readonly property string body: root.matched && String(root.detail.text || "").length > 0
        ? String(root.detail.text) : String(root.item ? root.item.snippet || "" : "")
    readonly property real pad: Math.round(16 * root.s)

    Shape {
        anchors.fill: parent
        anchors.margins: Math.round(8 * root.s)
        anchors.leftMargin: 0
        s: root.s
        cutSize: Look.cutCard
        fill: Look.card
        strokeColor: Look.cardStroke
        strokeWidth: Qt.colorEqual(Look.cardStroke, "transparent") ? 0 : 1
        clip: true

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
                spacing: Math.round(10 * root.s)

                // a picture is never blown up past its own pixels
                Shape {
                    id: frame
                    readonly property bool roomy: picture.implicitWidth >= column.width * 0.9
                    width: frame.roomy ? parent.width : Math.min(parent.width, Math.max(1, picture.implicitWidth))
                    height: frame.roomy ? Math.round(width * 9 / 16) : Math.min(Math.round(parent.width * 9 / 16), Math.max(1, picture.implicitHeight))
                    visible: root.picture.length > 0 && picture.status === Image.Ready
                    s: root.s
                    cutSize: Look.cutRow
                    fill: Look.well
                    clip: true

                    Image {
                        id: picture
                        anchors.fill: parent
                        source: root.picture.length > 0 ? "file://" + root.picture : ""
                        fillMode: Image.PreserveAspectCrop
                        asynchronous: true
                    }
                }

                Micro {
                    width: parent.width
                    s: root.s
                    text: String(root.item ? root.item.site || "" : "")
                    visible: text.length > 0
                }

                Text {
                    width: parent.width
                    text: String(root.matched && root.detail.title ? root.detail.title : root.item ? root.item.title || "" : "")
                    color: Lithic.fg
                    font.family: Look.sans
                    font.pixelSize: Math.round(16 * root.s)
                    font.weight: Font.Medium
                    wrapMode: Text.WordWrap
                    maximumLineCount: 4
                    elide: Text.ElideRight
                    renderType: Text.NativeRendering
                }

                Text {
                    width: parent.width
                    visible: text.length > 0
                    text: root.body
                    textFormat: Text.PlainText
                    color: Lithic.fg2
                    font.family: Look.sans
                    font.pixelSize: Math.round(13 * root.s)
                    lineHeight: Math.round(19 * root.s)
                    lineHeightMode: Text.FixedHeight
                    wrapMode: Text.WordWrap
                    renderType: Text.NativeRendering
                }

                Column {
                    width: parent.width
                    visible: root.examples.length > 0
                    spacing: Math.round(4 * root.s)

                    Repeater {
                        // a quotation can run a page: three, three lines each
                        model: root.examples.slice(0, 3)

                        delegate: Text {
                            required property var modelData
                            width: column.width
                            text: String(modelData)
                            textFormat: Text.PlainText
                            color: Lithic.fg3
                            font.family: Look.sans
                            font.pixelSize: Math.round(12 * root.s)
                            lineHeight: Math.round(18 * root.s)
                            lineHeightMode: Text.FixedHeight
                            wrapMode: Text.WordWrap
                            maximumLineCount: 3
                            elide: Text.ElideRight
                            renderType: Text.NativeRendering
                        }
                    }
                }

                Text {
                    width: parent.width
                    visible: root.matched && String(root.detail.error || "").length > 0
                    text: root.matched ? String(root.detail.error || "") : ""
                    color: Lithic.jasper
                    font.family: Look.sans
                    font.pixelSize: Math.round(12 * root.s)
                    wrapMode: Text.WordWrap
                    renderType: Text.NativeRendering
                }

                Column {
                    width: parent.width
                    visible: root.facts.length > 0
                    spacing: Math.round(5 * root.s)

                    Rectangle {
                        width: parent.width
                        height: 1
                        color: Lithic.alpha(Lithic.fg, 0.1)
                    }
                    Repeater {
                        model: root.facts

                        delegate: Item {
                            id: fact
                            required property var modelData
                            width: column.width
                            height: Math.max(key.implicitHeight, value.implicitHeight)

                            Micro {
                                id: key
                                width: Math.round(parent.width * 0.38)
                                s: root.s
                                text: String(fact.modelData.label || "")
                            }
                            Text {
                                id: value
                                x: Math.round(parent.width * 0.4)
                                width: parent.width - x
                                text: String(fact.modelData.value || "")
                                color: Lithic.fg
                                font.family: Look.sans
                                font.pixelSize: Math.round(12 * root.s)
                                font.features: ({ "tnum": 1 })
                                wrapMode: Text.WordWrap
                                maximumLineCount: 3
                                elide: Text.ElideRight
                                renderType: Text.NativeRendering
                            }
                        }
                    }
                }

                Row {
                    visible: root.credit.length > 0
                    spacing: Math.round(3 * root.s)

                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: root.credit
                        color: Lithic.goldInk
                        font.family: Look.sans
                        font.pixelSize: Math.round(12 * root.s)
                        font.underline: true
                        renderType: Text.NativeRendering
                    }
                    Symbol {
                        anchors.verticalCenter: parent.verticalCenter
                        s: root.s
                        name: "open_in_new"
                        size: 13
                        ink: Lithic.goldInk
                    }
                    HoverHandler { cursorShape: Qt.PointingHandCursor }
                    TapHandler { onTapped: root.linkActivated(root.creditUrl) }
                }

                Text {
                    width: parent.width
                    text: String(root.item ? root.item.url || "" : "")
                    visible: text.length > 0
                    color: Lithic.fg3
                    font.family: Look.mono
                    font.pixelSize: Math.round(10 * root.s)
                    wrapMode: Text.WrapAnywhere
                    maximumLineCount: 2
                    elide: Text.ElideRight
                    renderType: Text.NativeRendering
                }
            }
        }
    }
}
