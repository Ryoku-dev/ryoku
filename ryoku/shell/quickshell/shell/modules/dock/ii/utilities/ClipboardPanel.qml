import QtQuick
import QtQuick.Layouts
import Ryoku.Bus
import Ryoku.Ui.Singletons
import shell.services
import stage.modules.common.widgets
import stage.services
import "../clock"
import ".."

/**
 * Pins on top, then the latest history. A click copies the entry again; on
 * hover a row offers pin/unpin and delete. Images show as thumbnails.
 */
ColumnLayout {
    id: panel

    property var host: null
    readonly property var tile: panel.host?.tile ?? null
    readonly property string feed: Bus.connected ? "clipboard" : ""
    readonly property var entryRoles: ["id", "kind", "mime", "size", "preview", "thumb",
        "thumbW", "thumbH", "w", "h", "starred"]
    property var copiedEntryId: null

    spacing: 3

    BusListModel {
        id: pinnedRows
        topic: panel.feed
        path: "entries"
        key: "id"
        roles: panel.entryRoles
        filterRole: "starred"
        filterValue: true
    }
    BusListModel {
        id: historyRows
        topic: panel.feed
        path: "entries"
        key: "id"
        roles: panel.entryRoles
        filterRole: "starred"
        filterValue: false
    }

    Timer {
        id: copiedTimer
        interval: 1400
        onTriggered: panel.copiedEntryId = null
    }

    component EntryRow: RippleButton {
        id: row
        property var entry: null
        property bool first: false
        readonly property bool isPinned: row.entry?.starred === true
        readonly property bool image: row.entry?.kind === "image"
        readonly property bool copied: panel.copiedEntryId === Number(row.entry?.id ?? -1)
        Layout.fillWidth: true
        implicitHeight: 52
        buttonRadius: row.first ? ClockStyle.radiusNormal : ClockStyle.radiusSmall
        colBackground: row.copied ? ClockStyle.colPrimaryContainer
            : row.isPinned ? ClockStyle.colTertiaryContainer : ClockStyle.colField
        colBackgroundHover: row.isPinned ? ClockStyle.colTertiaryContainerHover : ClockStyle.colFieldHover
        colRipple: ClockStyle.colSurfaceActive
        onClicked: {
            Clipboard.copy(Number(row.entry.id));
            panel.copiedEntryId = Number(row.entry.id);
            copiedTimer.restart();
            panel.host?.closePanel();
        }
        HoverHandler {
            id: rowHover
        }
        readonly property color colContent: row.copied ? ClockStyle.colOnPrimaryContainer
            : row.isPinned ? ClockStyle.colOnTertiaryContainer : ClockStyle.colOnSurface

        contentItem: Item {
            RowLayout {
                anchors.fill: parent
                anchors.leftMargin: 12
                anchors.rightMargin: 8
                spacing: 10

                MaterialSymbol {
                    visible: !row.image || row.copied
                    text: row.copied ? "check" : (panel.tile?.symbolOf(row.entry) ?? "notes")
                    iconSize: 18
                    fill: 1
                    color: row.colContent
                }
                Item {
                    visible: row.image && !row.copied
                    implicitWidth: 56
                    implicitHeight: 36
                    StyledImage {
                        anchors.fill: parent
                        source: row.image && row.entry?.thumb ? "file://" + row.entry.thumb : ""
                        fillMode: Image.PreserveAspectFit
                        asynchronous: true
                        cache: false
                    }
                }
                StyledText {
                    Layout.fillWidth: true
                    text: panel.tile?.labelOf(row.entry) ?? ""
                    color: row.colContent
                    font.pixelSize: ClockStyle.textNormal
                    elide: Text.ElideRight
                    maximumLineCount: 1
                }
                ClockCardAction {
                    visible: rowHover.hovered || row.isPinned
                    implicitWidth: 30
                    implicitHeight: 30
                    symbol: "keep"
                    tip: row.isPinned ? Translation.tr("Unpin") : Translation.tr("Pin")
                    colContent: row.colContent
                    onClicked: Clipboard.star(Number(row.entry.id), !row.isPinned)
                }
                ClockCardAction {
                    visible: rowHover.hovered
                    implicitWidth: 30
                    implicitHeight: 30
                    symbol: "delete"
                    tip: Translation.tr("Delete")
                    danger: true
                    onClicked: Clipboard.del(Number(row.entry.id))
                }
            }
        }
    }

    StyledText {
        Layout.leftMargin: 4
        visible: pinnedRows.count > 0
        text: Translation.tr("Pinned")
        color: ClockStyle.colOnSurfaceVariant
        font.pixelSize: Appearance.font.pixelSize.smallest
        font.weight: Font.Bold
    }
    Repeater {
        model: pinnedRows
        delegate: EntryRow {
            required property var modelData
            required property int index
            entry: modelData
            first: index === 0
        }
    }

    StyledText {
        Layout.leftMargin: 4
        Layout.topMargin: pinnedRows.count > 0 ? 6 : 0
        text: Translation.tr("Recent")
        color: ClockStyle.colOnSurfaceVariant
        font.pixelSize: Appearance.font.pixelSize.smallest
        font.weight: Font.Bold
    }
    ListView {
        id: historyList
        Layout.fillWidth: true
        readonly property int shownCount: Math.min(8, historyRows.count)
        implicitHeight: shownCount > 0 ? shownCount * 52 + (shownCount - 1) * spacing : 0
        model: historyRows
        spacing: panel.spacing
        interactive: false
        clip: true
        reuseItems: true
        delegate: EntryRow {
            required property var modelData
            required property int index
            width: ListView.view.width
            entry: modelData
            first: index === 0
        }
    }
    Rectangle {
        Layout.fillWidth: true
        visible: pinnedRows.count === 0 && historyRows.count === 0
        implicitHeight: 64
        radius: ClockStyle.radiusLarge
        color: ClockStyle.colField
        StyledText {
            anchors.centerIn: parent
            text: Translation.tr("Nothing copied yet")
            color: ClockStyle.colOnSurfaceVariant
            font.pixelSize: ClockStyle.textNormal
        }
    }

    ClockButton {
        Layout.fillWidth: true
        Layout.topMargin: 6
        symbol: "content_paste_search"
        label: Translation.tr("Open full history")
        onClicked: {
            panel.host?.closePanel();
            Spawn.run(["ryoku-shell", "clipboard"]);
        }
    }
}
