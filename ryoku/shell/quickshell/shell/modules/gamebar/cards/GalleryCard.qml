pragma ComponentBehavior: Bound

import QtQuick
import Qt.labs.folderlistmodel
import shell.services
import ".."
import "../../../components"
import Ryoku.Ui.Singletons
import "../look"

// Gallery: the newest screenshots and recordings, live from the folders the
// capture path writes, plus one press to open the current folder. A shot is its
// own thumbnail; a recording is a play tile with its name, so there is no
// separate index to fall out of date.
GameBarCard {
    id: card

    title: I18n.tr("Gallery")

    readonly property string shotsDir: Capture.shotsDir
    readonly property string clipsDir: Paths.recordingsDir

    property string mode: "shots"

    // a folder name with a space or a # must still open as one file URL
    function fileUrl(path) {
        return path ? "file://" + String(path).split("/").map(encodeURIComponent).join("/") : "";
    }

    function stamp(model, row) {
        const value = model.get(row, "fileModified");
        return value ? (typeof value.getTime === "function"
            ? value.getTime() : Date.parse(value) || 0) : 0;
    }

    // Both folder models sort by modification time, so read only the rows the
    // card draws instead of scanning and sorting a whole capture folder on
    // every change. Inferred from the timestamps so a Qt release changing what
    // sortReversed means cannot flip the order; when both ends share a stamp,
    // the configured direction is the only signal left.
    function newest(model, count, kind) {
        const total = model.count;
        if (total === 0)
            return [];
        const first = stamp(model, 0);
        const last = stamp(model, total - 1);
        const fromEnd = total > 1 && (first === last ? model.sortReversed : last > first);
        const take = Math.min(count, total);
        const out = [];
        for (let i = 0; i < take; i++) {
            const row = fromEnd ? total - 1 - i : i;
            const url = model.get(row, "fileUrl");
            if (!url)
                continue;
            out.push({
                url: "" + url,
                path: "" + model.get(row, "filePath"),
                name: "" + model.get(row, "fileName"),
                kind: kind,
                t: stamp(model, row)
            });
        }
        return out;
    }

    // FolderListModel can replace a file without changing the row count, so a
    // count binding alone can stay stale. Bump a revision on every model signal
    // and let both lists read it as their dependency.
    property int revision: 0

    Connections {
        target: shotFiles
        function onDataChanged() { card.revision++ }
        function onRowsInserted() { card.revision++ }
        function onRowsRemoved() { card.revision++ }
        function onModelReset() { card.revision++ }
    }

    Connections {
        target: clipFiles
        function onDataChanged() { card.revision++ }
        function onRowsInserted() { card.revision++ }
        function onRowsRemoved() { card.revision++ }
        function onModelReset() { card.revision++ }
    }

    readonly property var shots: {
        card.revision;
        return card.newest(shotFiles, 6, "shot");
    }
    readonly property var clips: {
        card.revision;
        return card.newest(clipFiles, 6, "clip");
    }
    readonly property var items: card.mode === "clips" ? card.clips : card.shots
    readonly property string folder: card.mode === "clips" ? card.clipsDir : card.shotsDir

    FolderListModel {
        id: shotFiles
        folder: card.fileUrl(card.shotsDir)
        showDirs: false
        nameFilters: ["*.png", "*.jpg", "*.jpeg", "*.webp"]
        sortField: FolderListModel.Time
        sortReversed: false
    }

    FolderListModel {
        id: clipFiles
        folder: card.fileUrl(card.clipsDir)
        showDirs: false
        nameFilters: ["*.mp4", "*.mkv", "*.webm", "*.mov"]
        sortField: FolderListModel.Time
        sortReversed: false
    }

    Column {
        width: parent.width
        spacing: 8 * card.s

        Item {
            width: parent.width
            height: 24 * card.s

            Text {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                text: card.items.length === 1
                    ? (card.mode === "clips" ? I18n.tr("1 recent clip") : I18n.tr("1 recent shot"))
                    : (card.mode === "clips"
                        ? I18n.tr("%1 recent clips") : I18n.tr("%1 recent shots")).arg(card.items.length)
                color: GameBarLook.subtext
                font.family: GameBarLook.face
                font.pixelSize: GameBarLook.smallie * card.s
            }

            GameBarTextButton {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                s: card.s
                compact: true
                label: I18n.tr("Folder")
                onClicked: Spawn.run(["xdg-open", card.folder])
            }
        }

        Grid {
            id: grid
            width: parent.width
            columns: 3
            spacing: 6 * card.s

            Repeater {
                model: card.items

                delegate: Item {
                    id: thumb
                    required property var modelData

                    width: (grid.width - (grid.columns - 1) * grid.spacing) / grid.columns
                    height: 62 * card.s

                    Rectangle {
                        anchors.fill: parent
                        radius: GameBarLook.radiusVerySmall * card.s
                        color: GameBarLook.raised
                        clip: true

                        Image {
                            anchors.fill: parent
                            source: thumb.modelData.kind === "shot" ? thumb.modelData.url : ""
                            sourceSize.width: Math.ceil(thumb.width)
                            sourceSize.height: Math.ceil(thumb.height)
                            visible: thumb.modelData.kind === "shot"
                            asynchronous: true
                            fillMode: Image.PreserveAspectCrop
                        }

                        GlyphIcon {
                            anchors.centerIn: parent
                            anchors.verticalCenterOffset: -5 * card.s
                            width: 24 * card.s
                            height: 24 * card.s
                            visible: thumb.modelData.kind === "clip"
                            name: "play"
                            color: GameBarLook.fg
                        }

                        Text {
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.bottom: parent.bottom
                            anchors.margins: 4 * card.s
                            visible: thumb.modelData.kind === "clip"
                            textFormat: Text.PlainText
                            text: thumb.modelData.name
                            color: GameBarLook.fg
                            font.family: GameBarLook.face
                            font.pixelSize: GameBarLook.smallest * card.s
                            elide: Text.ElideMiddle
                        }

                        MouseArea {
                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: Spawn.run(["xdg-open", thumb.modelData.path])
                        }
                    }
                }
            }
        }

        Text {
            visible: card.items.length === 0
            width: parent.width
            text: card.mode === "clips"
                ? I18n.tr("No recordings yet") : I18n.tr("No screenshots yet")
            color: GameBarLook.subtext
            horizontalAlignment: Text.AlignHCenter
            font.family: GameBarLook.face
            font.pixelSize: GameBarLook.small * card.s
        }

        Row {
            width: parent.width
            spacing: 8 * card.s

            GameBarTextButton {
                s: card.s
                primary: card.mode === "shots"
                label: I18n.tr("Screenshots")
                onClicked: card.mode = "shots"
            }
            GameBarTextButton {
                s: card.s
                primary: card.mode === "clips"
                label: I18n.tr("Recordings")
                onClicked: card.mode = "clips"
            }
        }
    }
}
