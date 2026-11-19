pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import "../look"

// One row of a result list. The row object says what it is:
//   {id, size: "one"|"two"|"task"|"calc"|"answer"|"note", glyph, glyphTone, image,
//    imageWide, title, subtitle, body, trail, trailMono, facts:[{glyph,text,mono}],
//    hint, transfer:{fraction,pieces,active,tone}, tone, disabled}
// The selection plate is the list's; a row only changes its ink on it.
Item {
    id: root

    required property real s
    property var row: ({})
    property bool selected: false

    readonly property string size: String(root.row.size || "one")
    readonly property bool stacked: root.size === "two" || root.size === "task" || root.size === "answer" || root.size === "calc"
    readonly property bool failed: root.row.tone === "error"
    readonly property color titleInk: root.failed ? Lithic.jasper
        : root.row.disabled === true ? Lithic.fg3
        : root.selected ? Look.plateInk : Lithic.fg
    readonly property color subInk: root.selected && Look.material ? Lithic.alpha(Look.plateInk, 0.78) : Lithic.fg2
    readonly property color faintInk: root.selected && Look.material ? Lithic.alpha(Look.plateInk, 0.7) : Lithic.fg3
    readonly property color markInk: root.row.glyphTone === "error" ? Lithic.jasper
        : root.row.glyphTone === "ok" ? Lithic.jade
        : root.row.glyphTone === "wait" ? Lithic.amber
        : root.row.glyphTone === "busy" ? Lithic.lapis
        : root.selected || root.row.glyphTone === "accent" ? Look.plateMark
        : Lithic.fg2
    readonly property real pad: Math.round(12 * root.s)
    readonly property string image: String(root.row.image || "")
    readonly property bool wide: root.image.length > 0 && root.row.imageWide === true
    readonly property real leadWidth: Math.round((root.wide ? 57 : 20) * root.s)
    readonly property real textX: root.pad + root.leadWidth + root.pad
    readonly property real textWidth: Math.max(0, root.width - root.textX - root.pad - (trailing.visible ? trailing.width + root.pad : 0))

    Symbol {
        x: root.pad
        y: root.stacked ? Math.round(11 * root.s) : Math.round((root.height - height) / 2)
        width: Math.round(20 * root.s)
        height: width
        visible: root.image.length === 0 && String(root.row.glyph || "").length > 0
        s: root.s
        name: String(root.row.glyph || "")
        size: 18
        ink: root.markInk
        chosen: root.selected
        Behavior on color {
            ColorAnimation { duration: Look.dur(Look.stateMs) }
        }
    }

    Loader {
        x: root.pad
        y: root.wide ? Math.round((root.height - height) / 2) : (root.stacked ? Math.round(11 * root.s) : Math.round((root.height - height) / 2))
        width: root.leadWidth
        height: Math.round((root.wide ? 32 : 20) * root.s)
        active: root.image.length > 0
        sourceComponent: Shape {
            s: root.s
            cutSize: Look.cutToken
            fill: Look.well
            clip: true
            Image {
                anchors.fill: parent
                source: "file://" + root.image
                sourceSize: Qt.size(Math.ceil(width * 2), Math.ceil(height * 2))
                fillMode: root.wide ? Image.PreserveAspectCrop : Image.PreserveAspectFit
                asynchronous: true
                cache: true
            }
        }
    }

    // one line: the title, then the quieter subtitle beside it
    Row {
        visible: !root.stacked
        x: root.textX
        width: root.textWidth
        anchors.verticalCenter: parent.verticalCenter
        spacing: Math.round(10 * root.s)

        Text {
            id: oneTitle
            width: Math.min(implicitWidth, parent.width)
            text: String(root.row.title || "")
            color: root.titleInk
            font.family: Look.sans
            font.pixelSize: Math.round(14 * root.s)
            elide: Text.ElideRight
            renderType: Text.NativeRendering
        }
        Text {
            anchors.baseline: oneTitle.baseline
            width: Math.max(0, parent.width - oneTitle.width - parent.spacing)
            visible: width > 24 * root.s && text.length > 0
            text: String(root.row.subtitle || "")
            color: root.faintInk
            font.family: Look.sans
            font.pixelSize: Math.round(13 * root.s)
            elide: Text.ElideRight
            renderType: Text.NativeRendering
        }
    }

    // two lines: title over subtitle; a task hangs its transfer line under them
    Column {
        visible: root.stacked && root.size !== "calc"
        x: root.textX
        y: Math.round((root.size === "task" ? 8 : root.size === "answer" ? 10 : 8) * root.s)
        width: root.textWidth
        spacing: Math.round(1 * root.s)

        Text {
            width: parent.width
            text: String(root.row.title || "")
            color: root.titleInk
            font.family: Look.sans
            font.pixelSize: Math.round(14 * root.s)
            font.weight: root.size === "answer" ? Font.Medium : Font.Normal
            elide: Text.ElideRight
            renderType: Text.NativeRendering
        }
        Text {
            width: parent.width
            visible: root.size !== "answer"
            text: String(root.row.subtitle || "")
            color: root.failed ? Lithic.jasper : root.subInk
            font.family: Look.sans
            font.pixelSize: Math.round(12 * root.s)
            font.features: ({ "tnum": 1 })
            elide: Text.ElideRight
            renderType: Text.NativeRendering
        }
        Text {
            width: parent.width
            visible: root.size === "answer"
            text: String(root.row.body || "")
            color: root.subInk
            font.family: Look.sans
            font.pixelSize: Math.round(13 * root.s)
            lineHeight: Math.round(18 * root.s)
            lineHeightMode: Text.FixedHeight
            wrapMode: Text.WordWrap
            maximumLineCount: 3
            elide: Text.ElideRight
            renderType: Text.NativeRendering
        }
    }

    Loader {
        id: transferLoader
        readonly property var transfer: root.row.transfer || null
        x: root.textX
        y: root.height - Math.round(11 * root.s)
        width: root.width - root.textX - root.pad
        active: root.size === "task" && transferLoader.transfer !== null
        sourceComponent: TransferLine {
            s: root.s
            fraction: transferLoader.transfer ? Number(transferLoader.transfer.fraction) : -1
            pieces: transferLoader.transfer ? (transferLoader.transfer.pieces || []) : []
            active: transferLoader.transfer ? transferLoader.transfer.active === true : false
            tone: transferLoader.transfer ? String(transferLoader.transfer.tone || "") : ""
        }
    }

    // a sum: what was typed, and its result as the figure
    Column {
        visible: root.size === "calc"
        x: root.textX
        width: root.textWidth
        anchors.verticalCenter: parent.verticalCenter
        spacing: Math.round(1 * root.s)

        Text {
            width: parent.width
            text: String(root.row.subtitle || "")
            color: root.faintInk
            font.family: Look.figures
            font.pixelSize: Math.round(12 * Look.figureScale * root.s)
            elide: Text.ElideRight
            renderType: Text.NativeRendering
        }
        Text {
            width: parent.width
            text: String(root.row.title || "")
            color: root.titleInk
            font.family: Look.figures
            font.pixelSize: Math.round(21 * Look.figureScale * root.s)
            font.weight: Font.Medium
            font.features: ({ "tnum": 1 })
            elide: Text.ElideRight
            renderType: Text.NativeRendering
        }
    }

    Row {
        id: trailing
        anchors.right: parent.right
        anchors.rightMargin: root.pad
        y: root.size === "task" || root.size === "two" || root.size === "answer" ? Math.round(9 * root.s) : Math.round((root.height - height) / 2)
        visible: width > 0
        spacing: Math.round(10 * root.s)

        Repeater {
            model: root.row.facts || []

            delegate: Row {
                id: fact
                required property var modelData
                anchors.verticalCenter: parent ? parent.verticalCenter : undefined
                spacing: Math.round(3 * root.s)

                Symbol {
                    anchors.verticalCenter: parent.verticalCenter
                    visible: String(fact.modelData.glyph || "").length > 0
                    s: root.s
                    name: String(fact.modelData.glyph || "")
                    size: 13
                    ink: root.faintInk
                }
                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    visible: text.length > 0
                    text: String(fact.modelData.text || "")
                    color: root.faintInk
                    font.family: fact.modelData.mono === true ? Look.mono : Look.figures
                    font.pixelSize: Math.round((fact.modelData.mono === true ? 10 : 12 * Look.figureScale) * root.s)
                    font.features: ({ "tnum": 1 })
                    renderType: Text.NativeRendering
                }
            }
        }
        Text {
            anchors.verticalCenter: parent.verticalCenter
            visible: text.length > 0
            text: String(root.row.trail || "")
            color: root.faintInk
            font.family: root.row.trailMono === true ? Look.figures : Look.sans
            font.pixelSize: Math.round((root.row.trailMono === true ? 12 * Look.figureScale : 12) * root.s)
            font.features: ({ "tnum": 1 })
            renderType: Text.NativeRendering
        }
        KeyHint {
            anchors.verticalCenter: parent.verticalCenter
            visible: hint.length > 0
            s: root.s
            hint: String(root.row.hint || "")
            ink: root.faintInk
        }
    }
}
