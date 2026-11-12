import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui.Singletons

// the greeting as a new terminal draws it: the picture in its character
// cells beside the lines, in the greeting's own colours on a terminal plate
Column {
    id: preview

    required property var store

    readonly property var model: preview.store ? preview.store.ffModel : null
    readonly property var logo: preview.model ? preview.model.logo : ({ kind: "none" })
    readonly property color plate: Lithic.dark ? Lithic.field : Lithic.fg
    readonly property color ink: Lithic.dark ? Lithic.fg : Lithic.ground
    readonly property real cellW: metrics.averageCharacterWidth
    readonly property real cellH: metrics.height
    readonly property int padRight: preview.logo.paddingRight === undefined ? 5 : preview.logo.paddingRight
    readonly property int padTop: preview.logo.paddingTop === undefined ? 5 : preview.logo.paddingTop
    readonly property bool drawsLogo: preview.logo.kind !== "none"

    spacing: LithicTokens.space.s2

    function logoUrl(src) {
        var s = String(src || "");
        if (s.length === 0)
            return "";
        if (s.indexOf("file:") === 0)
            return s;
        if (s.charAt(0) === "~")
            s = Paths.home + s.slice(1);
        return "file://" + s;
    }

    FontMetrics {
        id: metrics
        font.family: Lithic.font.mono
        font.pixelSize: 12
    }

    CutRect {
        width: parent.width
        height: frame.height + LithicTokens.space.s4 * 2
        fill: preview.plate
        cut: LithicTokens.cut.small

        Item {
            id: frame
            x: LithicTokens.space.s4
            y: LithicTokens.space.s4
            width: parent.width - LithicTokens.space.s4 * 2
            height: Math.max(LithicTokens.size.factsRowHeight, (lines.implicitHeight) * frame.factor)
            clip: true
            readonly property real factor: lines.implicitWidth > 0 ? Math.min(1, frame.width / lines.implicitWidth) : 1

            Row {
                id: lines
                transformOrigin: Item.TopLeft
                scale: frame.factor
                spacing: preview.drawsLogo ? Math.round(preview.padRight * preview.cellW) : 0

                Item {
                    id: emblem
                    readonly property real boxW: Math.max(1, preview.logo.width || 1) * preview.cellW
                    readonly property real boxH: Math.max(1, preview.logo.height || 1) * preview.cellH
                    readonly property real padL: (preview.logo.padding || 0) * preview.cellW
                    readonly property real padT: preview.padTop * preview.cellH
                    width: preview.drawsLogo ? emblem.padL + emblem.boxW : 0
                    height: Math.max(readout.implicitHeight, preview.drawsLogo ? emblem.padT + emblem.boxH : 0)

                    Image {
                        x: emblem.padL
                        y: emblem.padT
                        width: emblem.boxW
                        height: emblem.boxH
                        visible: preview.logo.kind === "image" && status === Image.Ready
                        source: preview.logo.kind === "image" ? preview.logoUrl(preview.logo.source) : ""
                        fillMode: Image.PreserveAspectFit
                        sourceSize.width: Math.round(emblem.boxW * 2) || 168
                        sourceSize.height: Math.round(emblem.boxH * 2) || 168
                        smooth: true
                        asynchronous: true
                    }

                    // text art and the system logo draw in the terminal itself
                    CutRect {
                        x: emblem.padL
                        y: emblem.padT
                        width: emblem.boxW
                        height: emblem.boxH
                        visible: preview.logo.kind === "ascii" || preview.logo.kind === "builtin"
                        fill: Lithic.alpha(preview.ink, 0.06)
                        strokeColor: Lithic.alpha(preview.ink, 0.24)
                        strokeWidth: LithicTokens.size.hairline
                        cut: LithicTokens.cut.token

                        Label {
                            anchors.centerIn: parent
                            width: parent.width - LithicTokens.space.s2 * 2
                            horizontalAlignment: Text.AlignHCenter
                            text: preview.logo.kind === "ascii" ? I18n.tr("Text art") : I18n.tr("System logo")
                            typeRole: "caption"
                            ink: Lithic.alpha(preview.ink, 0.6)
                            wrapMode: Text.Wrap
                            elide: Text.ElideNone
                        }
                    }
                }

                Column {
                    id: readout
                    spacing: 0

                    Repeater {
                        model: preview.store ? preview.store.previewLines : []
                        delegate: Row {
                            id: line
                            required property var modelData
                            spacing: 0
                            height: preview.cellH

                            Repeater {
                                model: line.modelData
                                delegate: Text {
                                    required property var modelData
                                    text: modelData.text
                                    color: modelData.color.length > 0 ? modelData.color : preview.ink
                                    font.family: Lithic.font.mono
                                    font.pixelSize: 12
                                    font.weight: modelData.bold ? Font.Bold : Font.Normal
                                    textFormat: Text.PlainText
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    Row {
        width: parent.width
        spacing: LithicTokens.space.s3

        Plaque {
            id: inTerminal
            kind: "tonal"
            small: true
            glyph: "terminal"
            text: I18n.tr("Show in a terminal")
            onClicked: Spawn.run(["ryoku-app", "terminal", "--", "sh", "-c", "ryoku-fastfetch; printf '\\n'; read -r _"])
        }
        Label {
            anchors.verticalCenter: parent.verticalCenter
            width: parent.width - inTerminal.width - LithicTokens.space.s3
            visible: preview.logo.kind === "image"
            text: I18n.tr("Pictures draw sharpest in Kitty; other terminals draw them with characters.")
            typeRole: "detail"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }
}
