pragma ComponentBehavior: Bound

import QtQuick
import Qt.labs.folderlistmodel
import shell.services
import ".."
import "../../../components"
import Ryoku.Ui.Singletons
import Lithic
import "../look"

GameBarCard {
    id: card

    title: I18n.tr("Gallery")

    readonly property string shotsDir: Capture.shotsDir
    readonly property string clipsDir: Paths.recordingsDir

    property string mode: "shots"

    function fileUrl(path) {
        return path ? "file://" + String(path).split("/").map(encodeURIComponent).join("/") : "";
    }

    function stamp(model, row) {
        const value = model.get(row, "fileModified");
        return value ? (typeof value.getTime === "function"
            ? value.getTime() : Date.parse(value) || 0) : 0;
    }

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

    Item {
        anchors.fill: parent

        Item {
            id: header
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            height: LithicTokens.size.plaqueHeightSmall * card.s

            Label {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                text: card.items.length === 1
                    ? (card.mode === "clips" ? I18n.tr("1 recent clip") : I18n.tr("1 recent shot"))
                    : (card.mode === "clips"
                        ? I18n.tr("%1 recent clips") : I18n.tr("%1 recent shots")).arg(card.items.length)
                typeRole: "detail"
                inkRole: "fg2"
            }

            Plaque {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                small: true
                kind: "quiet"
                glyph: "folder_open"
                text: I18n.tr("Folder")
                onClicked: Spawn.run(["xdg-open", card.folder])
            }
        }

        Grid {
            id: grid
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: header.bottom
            anchors.topMargin: card.space2
            columns: 3
            spacing: card.space2

            Repeater {
                model: card.items

                delegate: GameBarPlate {
                    id: thumb
                    required property var modelData

                    width: (grid.width - (grid.columns - 1) * grid.spacing) / grid.columns
                    height: 56 * card.s
                    cut: GameBarTone.controlCut
                    source: thumb.modelData.kind === "shot" ? thumb.modelData.url : ""

                    MouseArea {
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: Spawn.run(["xdg-open", thumb.modelData.path])
                    }

                    Glyph {
                        visible: thumb.modelData.kind === "clip"
                        anchors.centerIn: parent
                        name: "play_arrow"
                        size: 22
                        ink: GameBarTone.ink2
                    }

                    Label {
                        visible: thumb.modelData.kind === "clip"
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.bottom: parent.bottom
                        anchors.margins: 4 * card.s
                        text: thumb.modelData.name
                        typeRole: "monoSm"
                        inkRole: "fg2"
                        horizontalAlignment: Text.AlignHCenter
                    }
                }
            }
        }

        Label {
            visible: card.items.length === 0
            anchors.top: grid.top
            anchors.topMargin: card.space4
            width: parent.width
            text: card.mode === "clips"
                ? I18n.tr("No recordings yet") : I18n.tr("No screenshots yet")
            typeRole: "detail"
            inkRole: "fg3"
            horizontalAlignment: Text.AlignHCenter
        }

        Segmented {
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottom: parent.bottom
            width: 220 * card.s
            model: [I18n.tr("Screenshots"), I18n.tr("Recordings")]
            currentIndex: card.mode === "clips" ? 1 : 0
            onActivated: index => card.mode = index === 1 ? "clips" : "shots"
        }
    }
}
