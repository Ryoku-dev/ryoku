pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import Quickshell
import Quickshell.Io
import inir.services
import inir.modules.common
import inir.modules.iris.components
import inir.modules.iris.style

Flickable {
    id: root

    readonly property real d: IrisStyle.density
    // Two versions, two owners: the iNiR release Shima is based on (the payload's
    // VERSION, bumped with each upstream sync) and the Ryoku package that ships it.
    readonly property string upstreamVersion: upstreamVersionFile.text().trim()
    readonly property string ryokuVersion: versionProc.version
    readonly property string windowManager: {
        const name = CompositorService.provider
        return name.length > 0 ? name.charAt(0).toUpperCase() + name.slice(1) : ""
    }
    property string tab: "inir"
    boundsBehavior: Flickable.StopAtBounds
    clip: true
    contentHeight: pageColumn.implicitHeight + 40 * root.d
    ScrollBar.vertical: IrisScrollBar {}

    FileView {
        id: upstreamVersionFile
        path: Directories.payloadPath("VERSION")
        blockLoading: true
        printErrors: false
    }
    Process {
        id: versionProc
        property string version: ""
        running: true
        command: ["ryoku", "version", "--pretty"]
        stdout: StdioCollector { onStreamFinished: versionProc.version = text.trim() }
    }

    ColumnLayout {
        id: pageColumn
        width: Math.min(root.width - 32 * root.d, 640 * root.d)
        x: Math.round((root.width - width) / 2)
        y: Math.round(18 * root.d)
        spacing: 18 * root.d

        ColumnLayout {
            Layout.fillWidth: true
            // A ColumnLayout caps its own width at its implicit width; uncapped it spans the page, so its centred rows centre on it.
            Layout.maximumWidth: Number.POSITIVE_INFINITY
            Layout.bottomMargin: 6 * root.d
            spacing: 6 * root.d
            IrisMark {
                Layout.alignment: Qt.AlignHCenter
                Layout.bottomMargin: 8 * root.d
                implicitSize: Math.round(76 * root.d)
            }
            IrisText {
                Layout.alignment: Qt.AlignHCenter
                text: "Shima"
                font.family: IrisStyle.fontTitle
                font.pixelSize: 26 * IrisStyle.typeScale
                font.weight: IrisStyle.weight(Font.Bold)
            }
            IrisText {
                Layout.alignment: Qt.AlignHCenter
                text: Translation.tr("Based on iNiR by snowarch (https://github.com/snowarch/inir)")
                color: IrisStyle.muted
                font.pixelSize: IrisStyle.typeLabel
            }
            IrisText {
                Layout.alignment: Qt.AlignHCenter
                Layout.topMargin: 6 * root.d
                Layout.maximumWidth: 520 * root.d
                horizontalAlignment: Text.AlignHCenter
                wrapMode: Text.Wrap
                text: Translation.tr("Shima is a modified version of iNiR's island design, rebuilt by Ryoku. It runs through Ryoku's window-manager layer, so it works on every compositor Ryoku supports.")
                color: IrisStyle.muted
                font.pixelSize: IrisStyle.typeMeta
            }
        }

        IrisSegmented {
            Layout.fillWidth: true
            accessibleName: Translation.tr("About")
            options: [
                { value: "inir", label: Translation.tr("iNiR · snowarch") },
                { value: "ryoku", label: Translation.tr("Ryoku maintainer - Neur0map") }
            ]
            current: root.tab
            onPicked: value => root.tab = value
        }

        ColumnLayout {
            Layout.fillWidth: true
            visible: root.tab === "inir"
            spacing: 18 * root.d

            InfoCard {
                rows: [
                    { label: Translation.tr("Based on"), value: root.upstreamVersion.length > 0 ? "iNiR " + root.upstreamVersion : "iNiR" },
                    { label: Translation.tr("Author"), value: "snowarch", highlight: true },
                    { label: Translation.tr("License"), value: Translation.tr("GPL-3.0, with the terms in NOTICE") }
                ]
            }

            Heading { text: Translation.tr("iNiR") }
            IrisLinkCard {
                tinted: true
                links: [
                    { label: Translation.tr("Source code"), value: "github.com/snowarch/inir", icon: "code", tint: IrisStyle.identity.lavender, action: () => Qt.openUrlExternally("https://github.com/snowarch/inir") },
                    { label: Translation.tr("Documentation"), icon: "menu_book", tint: IrisStyle.identity.sky, action: () => Qt.openUrlExternally("https://github.com/snowarch/inir/wiki") }
                ]
            }
        }

        ColumnLayout {
            Layout.fillWidth: true
            visible: root.tab === "ryoku"
            spacing: 18 * root.d

            InfoCard {
                rows: [
                    { label: Translation.tr("Ryoku maintainer"), value: "Neur0map", highlight: true },
                    { label: Translation.tr("Ryoku"), value: root.ryokuVersion },
                    { label: Translation.tr("Window manager"), value: root.windowManager },
                    { label: Translation.tr("System"), value: SystemInfo.distroName },
                    { label: Translation.tr("Computer"), value: SystemInfo.hostname }
                ].filter(row => String(row.value ?? "").length > 0 && row.value !== "Unknown")
            }

            Heading { text: Translation.tr("Ryoku") }
            IrisLinkCard {
                tinted: true
                links: [
                    { label: Translation.tr("Maintainer"), value: "github.com/neur0map", icon: "person", tint: IrisStyle.identity.purple, action: () => Qt.openUrlExternally("https://github.com/neur0map") },
                    { label: Translation.tr("Report an issue"), icon: "bug_report", tint: IrisStyle.identity.red, action: () => Qt.openUrlExternally("https://github.com/Ryoku-dev/ryoku/issues") },
                    { label: Translation.tr("Source code"), value: "github.com/Ryoku-dev/ryoku", icon: "code", tint: IrisStyle.identity.blue, action: () => Qt.openUrlExternally("https://github.com/Ryoku-dev/ryoku") },
                    { label: Translation.tr("Documentation"), icon: "auto_stories", tint: IrisStyle.identity.teal, action: () => Qt.openUrlExternally("https://github.com/Ryoku-dev/ryoku/tree/main/docs") }
                ]
            }
        }
    }

    component Heading: IrisText {
        Layout.leftMargin: 16 * root.d
        Layout.bottomMargin: -10 * root.d
        color: IrisStyle.muted
        font.family: IrisStyle.fontTitle
        font.pixelSize: IrisStyle.typeMeta
        font.weight: IrisStyle.weight(Font.DemiBold)
    }

    component InfoCard: Rectangle {
        id: card
        property var rows: []
        Layout.fillWidth: true
        implicitHeight: infoColumn.implicitHeight
        radius: IrisStyle.radiusTile
        color: IrisStyle.readingCard
        Column {
            id: infoColumn
            width: parent.width
            Repeater {
                model: card.rows
                Item {
                    id: infoRow
                    required property var modelData
                    required property int index
                    width: parent.width
                    height: Math.round(40 * root.d)
                    IrisText {
                        id: infoLabel
                        anchors.left: parent.left
                        anchors.leftMargin: 16 * root.d
                        anchors.verticalCenter: parent.verticalCenter
                        text: infoRow.modelData.label
                        font.pixelSize: IrisStyle.typeLabel
                    }
                    IrisText {
                        anchors.left: infoLabel.right
                        anchors.leftMargin: 16 * root.d
                        anchors.right: parent.right
                        anchors.rightMargin: 16 * root.d
                        anchors.verticalCenter: parent.verticalCenter
                        horizontalAlignment: Text.AlignRight
                        text: infoRow.modelData.value
                        color: infoRow.modelData.highlight ? IrisStyle.secondaryAccent : IrisStyle.muted
                        font.pixelSize: IrisStyle.typeLabel
                        elide: Text.ElideMiddle
                    }
                    Rectangle {
                        anchors.left: parent.left
                        anchors.leftMargin: 16 * root.d
                        anchors.right: parent.right
                        anchors.bottom: parent.bottom
                        height: 1
                        visible: infoRow.index < card.rows.length - 1
                        color: IrisStyle.hairline
                    }
                }
            }
        }
    }
}
