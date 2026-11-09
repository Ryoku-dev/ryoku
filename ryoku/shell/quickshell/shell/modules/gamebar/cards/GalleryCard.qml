pragma ComponentBehavior: Bound

import QtQuick
import Qt.labs.folderlistmodel
import shell.services
import ".."
import "../../../components"
import Ryoku.Ui.Singletons
import "../look"

// Gallery: the newest screenshots and recordings, live from the folders the
// capture path writes, plus one press to open the current folder. Thumbnails
// are the files themselves, so there is no separate index to fall out of date.
GameBarCard {
    id: card

    title: I18n.tr("Gallery")

    readonly property string shotsDir: Capture.shotsDir
    readonly property string clipsDir: Paths.recordingsDir

    property string mode: "shots"

    function newest(model, count, kind) {
        const all = [];
        for (let i = 0; i < model.count; i++) {
            const url = model.get(i, "fileUrl");
            if (!url)
                continue;
            const stamp = model.get(i, "fileModified");
            const when = stamp ? (typeof stamp.getTime === "function"
                ? stamp.getTime() : Date.parse(stamp) || 0) : 0;
            all.push({
                url: "" + url,
                path: "" + model.get(i, "filePath"),
                name: "" + model.get(i, "fileName"),
                kind: kind,
                t: when
            });
        }
        all.sort((a, b) => b.t - a.t);
        return all.slice(0, count);
    }

    readonly property var shots: card.newest(shotFiles, 6, "shot")
    readonly property var clips: card.newest(clipFiles, 6, "clip")
    readonly property var items: card.mode === "clips" ? card.clips : card.shots
    readonly property string folder: card.mode === "clips" ? card.clipsDir : card.shotsDir

    FolderListModel {
        id: shotFiles
        folder: "file://" + card.shotsDir
        showDirs: false
        nameFilters: ["*.png", "*.jpg", "*.jpeg", "*.webp"]
    }

    FolderListModel {
        id: clipFiles
        folder: "file://" + card.clipsDir
        showDirs: false
        nameFilters: ["*.mp4", "*.mkv", "*.webm", "*.mov"]
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
                    : card.items.length + " " + (card.mode === "clips"
                        ? I18n.tr("recent clips") : I18n.tr("recent shots"))
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
