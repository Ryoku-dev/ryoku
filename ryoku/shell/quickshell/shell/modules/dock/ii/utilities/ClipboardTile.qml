import QtQuick
import QtQuick.Layouts
import Ryoku.Bus
import Ryoku.Ui.Singletons
import shell.services
import stage.modules.common.widgets
import stage.services
import "../clock"
import stage.modules.common.functions
import ".."

/**
 * Clipboard: the daemon's text, image and binary history. Square: the kind of
 * the latest entry on a gem. Wide: that badge, the latest preview and its kind,
 * with the starred count. The panel holds starred and recent entries.
 */
UtilityTile {
    id: tile

    readonly property string feed: Bus.connected ? "clipboard" : ""
    readonly property var entryRoles: ["id", "kind", "mime", "size", "preview", "thumb",
        "thumbW", "thumbH", "w", "h", "starred"]
    readonly property int entryCount: historyRows.count + starredRows.count
    readonly property int starredCount: starredRows.count
    readonly property var latest: Clipboard.entries.length > 0 ? Clipboard.entries[0] : null

    function textOf(entry) {
        return String(entry?.preview ?? "").trim();
    }
    function kindOf(entry) {
        const kind = String(entry?.kind ?? "");
        if (kind === "image" || kind === "binary")
            return kind;
        const text = tile.textOf(entry);
        if (/^(file:\/\/|\/|~\/)/.test(text))
            return "file";
        if (/^https?:\/\//.test(text))
            return "link";
        return "text";
    }
    function symbolOf(entry) {
        switch (tile.kindOf(entry)) {
        case "image": return "image";
        case "binary": return "draft";
        case "file": return UtilityFiles.symbolFor(UtilityFiles.pathFromUrl(tile.textOf(entry).split(/\r?\n/)[0]));
        case "link": return "link";
        default: return "notes";
        }
    }
    function humanSize(bytes) {
        const size = Math.max(0, Number(bytes) || 0);
        if (size < 1024)
            return size + " B";
        if (size < 1048576)
            return (size / 1024).toFixed(1) + " KB";
        return (size / 1048576).toFixed(1) + " MB";
    }
    function labelOf(entry) {
        const kind = String(entry?.kind ?? "");
        if (kind === "image") {
            const dimensions = Number(entry?.w ?? 0) > 0 && Number(entry?.h ?? 0) > 0
                ? entry.w + " × " + entry.h : Translation.tr("Image");
            const mime = String(entry?.mime ?? "");
            return mime.length > 0 ? dimensions + " · " + mime : dimensions;
        }
        if (kind === "binary") {
            const mime = String(entry?.mime ?? "") || Translation.tr("Binary");
            return mime + " · " + tile.humanSize(entry?.size ?? 0);
        }
        return tile.textOf(entry).replace(/\s+/g, " ");
    }
    function kindName(entry) {
        switch (tile.kindOf(entry)) {
        case "image": return Translation.tr("Image");
        case "binary": return Translation.tr("Binary");
        case "file": return Translation.tr("File");
        case "link": return Translation.tr("Link");
        default: return Translation.tr("Text");
        }
    }

    BusListModel {
        id: historyRows
        topic: tile.feed
        path: "entries"
        key: "id"
        roles: tile.entryRoles
        filterRole: "starred"
        filterValue: false
    }
    BusListModel {
        id: starredRows
        topic: tile.feed
        path: "entries"
        key: "id"
        roles: tile.entryRoles
        filterRole: "starred"
        filterValue: true
    }

    surfaceColor: ClockStyle.colSurfaceHigh
    contentColor: ClockStyle.colOnSurface

    tooltipText: tile.latest !== null
        ? Translation.tr("Clipboard · %1").arg(tile.labelOf(tile.latest).substring(0, 60))
        : Translation.tr("Clipboard")
    panelSubtitle: tile.starredCount > 0
        ? Translation.tr("%1 starred · %2 in history").arg(tile.starredCount).arg(tile.entryCount)
        : Translation.tr("%1 in history").arg(tile.entryCount)
    panelWidth: 380

    TileBadge {
        anchors.centerIn: parent
        visible: !tile.wide
        width: Math.round(tile.side * 0.74)
        height: width
        renderScale: tile.renderScale
        shape: MaterialShape.Shape.Gem
        color: ClockStyle.colPrimaryContainer
        colSymbol: ClockStyle.colOnPrimaryContainer
        text: tile.latest !== null ? tile.symbolOf(tile.latest) : "content_paste"
        iconScale: 0.48
    }

    RowLayout {
        anchors.fill: parent
        anchors.margins: tile.pad
        anchors.rightMargin: tile.pad * 2
        visible: tile.wide
        spacing: tile.pad

        TileBadge {
            implicitWidth: tile.badgeSize
            implicitHeight: tile.badgeSize
            renderScale: tile.renderScale
            shape: MaterialShape.Shape.Gem
            color: ClockStyle.colPrimaryContainer
            colSymbol: ClockStyle.colOnPrimaryContainer
            text: tile.latest !== null ? tile.symbolOf(tile.latest) : "content_paste"
            iconScale: 0.48
        }
        ColumnLayout {
            Layout.fillWidth: true
            Layout.alignment: Qt.AlignVCenter
            spacing: 0
            TileText {
                Layout.fillWidth: true
                text: tile.latest !== null ? tile.labelOf(tile.latest) : Translation.tr("Nothing copied yet")
                color: tile.contentColor
                font.pixelSize: Math.round(tile.height * 0.27)
                font.weight: Font.DemiBold
                elide: Text.ElideRight
                maximumLineCount: 1
            }
            TileCaption {
                Layout.fillWidth: true
                text: (tile.latest !== null ? tile.kindName(tile.latest) : Translation.tr("Clipboard"))
                    + (tile.starredCount > 0 ? " · " + Translation.tr("%1 starred").arg(tile.starredCount) : "")
                color: tile.captionColor
                font.pixelSize: tile.captionSize
            }
        }
    }
}
