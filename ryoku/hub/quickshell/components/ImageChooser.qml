pragma ComponentBehavior: Bound

import QtCore
import QtQuick
import QtQuick.Controls as C
import Qt.labs.folderlistmodel
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"

// choosing a picture by looking at it: a folder's pictures as thumbnails, its folders first,
// a few places to jump to, and a large preview of the one under the pointer or chosen.
// The file API matches FolderChooser's: open(), accepted() with selectedFile
Item {
    id: host

    visible: false
    width: 0
    height: 0

    property string title: I18n.tr("Choose a picture")
    property url currentFolder: StandardPaths.writableLocation(StandardPaths.PicturesLocation)
    property url selectedFile: ""
    property Item overlayParent: null
    // more files to list beside pictures (a card design, "*.ryoprofile"); shown with a mark
    property var extraFilters: []
    // list short videos too (a badge holder's card picture); shown with a mark and their size
    property bool videos: false
    readonly property bool showing: dialog.visible

    signal accepted()
    signal rejected()

    readonly property string home: host.localPath(StandardPaths.writableLocation(StandardPaths.HomeLocation))
    property string path: ""
    // the picture chosen in this folder, a local path, and the one the pointer rests on
    property string chosen: ""
    property string resting: ""
    property bool took: false
    readonly property string shown: host.resting !== "" ? host.resting : host.chosen

    readonly property var places: [
        { name: I18n.tr("Pictures"), path: host.localPath(StandardPaths.writableLocation(StandardPaths.PicturesLocation)) },
        { name: I18n.tr("Wallpapers"), path: host.localPath(StandardPaths.writableLocation(StandardPaths.PicturesLocation)) + "/Wallpapers" },
        { name: I18n.tr("Downloads"), path: host.localPath(StandardPaths.writableLocation(StandardPaths.DownloadLocation)) },
        { name: I18n.tr("Home"), path: host.home }
    ]

    function open(): void {
        if (host.showing)
            return;
        host.took = false;
        host.chosen = "";
        host.resting = "";
        var p = host.localPath(host.currentFolder);
        host.path = p !== "" ? p : host.home;
        dialog.open();
    }

    function localPath(url): string {
        var s = String(url || "");
        if (s.indexOf("file://") === 0)
            s = decodeURIComponent(s.slice(7));
        if (s.indexOf("/") !== 0)
            return "";
        s = s.replace(/\/+/g, "/").replace(/\/$/, "");
        return s.length > 0 ? s : "/";
    }
    function urlOf(p): string {
        return "file://" + p.split("/").map(encodeURIComponent).join("/");
    }
    function parentOf(p): string {
        var at = p.lastIndexOf("/");
        return at <= 0 ? "/" : p.slice(0, at);
    }
    function nameOf(p): string {
        return p === "/" ? "/" : p.slice(p.lastIndexOf("/") + 1);
    }
    function shortPath(p): string {
        if (host.home.length > 1 && (p === host.home || p.indexOf(host.home + "/") === 0))
            return "~" + p.slice(host.home.length);
        return p;
    }
    function go(p): void {
        host.path = p;
        host.chosen = "";
        host.resting = "";
        grid.contentY = 0;
    }
    function take(p): void {
        host.took = true;
        host.selectedFile = host.urlOf(p);
        dialog.close();
        host.accepted();
    }

    FolderListModel {
        id: files
        folder: host.path === "" ? "" : host.urlOf(host.path)
        nameFilters: ["*.png", "*.jpg", "*.jpeg", "*.webp", "*.gif", "*.bmp", "*.PNG", "*.JPG", "*.JPEG", "*.WEBP"].concat(host.extraFilters)
            .concat(host.videos ? ["*.mp4", "*.webm", "*.mkv", "*.MP4", "*.WEBM", "*.MKV"] : [])
        showDirsFirst: true
        showHidden: false
        showDotAndDotDot: false
        sortField: FolderListModel.Name
        sortCaseSensitive: false
    }

    Dialog {
        id: dialog
        parent: host.overlayParent ? host.overlayParent : C.Overlay.overlay
        // over a dialog it was opened from
        z: 100
        title: host.title
        dialogWidth: 1040
        actions: [
            { text: I18n.tr("Cancel"), kind: "quiet" },
            { text: I18n.tr("Use this picture"), kind: "gold", enabled: host.chosen !== "" }
        ]
        onActionTriggered: index => {
            if (index === 1 && host.chosen !== "")
                host.take(host.chosen);
        }
        onClosed: {
            if (!host.took)
                host.rejected();
        }

        Column {
            width: parent ? parent.width : 0
            spacing: LithicTokens.space.s3

            // where it is, and the places to jump to
            Row {
                width: parent.width
                spacing: LithicTokens.space.s2
                IconPlaque {
                    kind: "tonal"
                    glyph: "arrow_upward"
                    enabled: host.path !== "/"
                    accessibleName: I18n.tr("Up to %1").arg(host.nameOf(host.parentOf(host.path)))
                    onClicked: host.go(host.parentOf(host.path))
                }
                Repeater {
                    model: host.places
                    delegate: Plaque {
                        required property var modelData
                        kind: host.path === modelData.path ? "tonal" : "quiet"
                        small: true
                        text: modelData.name
                        onClicked: host.go(modelData.path)
                    }
                }
            }
            Label {
                width: parent.width
                text: host.shortPath(host.path)
                typeRole: "detail"
                inkRole: "fg2"
                elide: Text.ElideMiddle
            }

            Row {
                id: panes
                width: parent.width
                height: Math.min(460, (dialog.parent ? dialog.parent.height : 800) - 300)
                spacing: LithicTokens.space.s4

                // the folder's pictures and folders
                GridView {
                    id: grid
                    width: panes.width - preview.width - panes.spacing
                    height: panes.height
                    clip: true
                    model: files
                    readonly property int columns: Math.max(3, Math.floor(width / 132))
                    cellWidth: Math.floor(width / columns)
                    cellHeight: cellWidth * 0.78 + 22
                    boundsBehavior: Flickable.StopAtBounds
                    cacheBuffer: 400

                    delegate: Item {
                        id: cell
                        required property string fileName
                        required property string filePath
                        required property bool fileIsDir
                        required property real fileSize
                        readonly property bool isChosen: !fileIsDir && host.chosen === filePath
                        readonly property bool picture: !fileIsDir && /\.(png|jpe?g|webp|gif|bmp)$/i.test(fileName)
                        readonly property bool video: !fileIsDir && /\.(mp4|webm|mkv)$/i.test(fileName)
                        width: grid.cellWidth
                        height: grid.cellHeight

                        CutRect {
                            id: frame
                            x: 4
                            y: 4
                            width: parent.width - 8
                            height: parent.height - 30
                            cut: LithicTokens.cut.small
                            fill: Lithic.stone2
                            strokeWidth: cell.isChosen ? 2 : 1
                            strokeColor: cell.isChosen ? Lithic.sel : area.containsMouse ? Lithic.edge : Lithic.vein
                            clip: true

                            Glyph {
                                visible: !cell.picture
                                anchors.centerIn: parent
                                name: cell.fileIsDir ? "folder" : cell.video ? "movie" : "description"
                                size: 34
                                ink: Lithic.fg3
                            }
                            Image {
                                visible: cell.picture
                                anchors.fill: parent
                                anchors.margins: 1
                                source: cell.picture ? host.urlOf(cell.filePath) : ""
                                sourceSize.width: 240
                                sourceSize.height: 240
                                fillMode: Image.PreserveAspectCrop
                                asynchronous: true
                                cache: true
                                opacity: status === Image.Ready ? 1 : 0
                                Behavior on opacity { NumberAnimation { duration: Tone.quick } }
                            }
                        }
                        Label {
                            x: 6
                            anchors.top: frame.bottom
                            anchors.topMargin: 4
                            width: parent.width - 12
                            text: !cell.video ? cell.fileName : cell.fileName + " · " + (cell.fileSize < 1048576
                                ? Math.max(1, Math.round(cell.fileSize / 1024)) + " KB" : (cell.fileSize / 1048576).toFixed(1) + " MB")
                            typeRole: "caption"
                            ink: cell.isChosen ? Lithic.fg : Lithic.fg2
                            elide: Text.ElideMiddle
                            horizontalAlignment: Text.AlignHCenter
                        }
                        MouseArea {
                            id: area
                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onEntered: if (!cell.fileIsDir) host.resting = cell.filePath
                            onExited: if (host.resting === cell.filePath) host.resting = ""
                            onClicked: {
                                if (cell.fileIsDir)
                                    host.go(cell.filePath);
                                else
                                    host.chosen = cell.filePath;
                            }
                            onDoubleClicked: if (!cell.fileIsDir) host.take(cell.filePath)
                        }
                    }

                    Label {
                        anchors.centerIn: parent
                        visible: files.status === FolderListModel.Ready && files.count === 0
                        text: I18n.tr("No pictures here")
                        typeRole: "detail"
                        inkRole: "fg3"
                    }
                }

                // the picture under the pointer, or the one chosen, large
                CutRect {
                    id: preview
                    width: Math.min(360, panes.width * 0.38)
                    height: panes.height
                    cut: LithicTokens.cut.mid
                    fill: Lithic.ground
                    strokeWidth: 1
                    strokeColor: Lithic.vein
                    clip: true

                    Image {
                        id: big
                        anchors.fill: parent
                        anchors.margins: 12
                        anchors.bottomMargin: 44
                        source: /\.(png|jpe?g|webp|gif|bmp)$/i.test(host.shown) ? host.urlOf(host.shown) : ""
                        sourceSize.width: 720
                        sourceSize.height: 720
                        fillMode: Image.PreserveAspectFit
                        asynchronous: true
                    }
                    Glyph {
                        visible: /\.(mp4|webm|mkv)$/i.test(host.shown)
                        anchors.centerIn: big
                        name: "movie"
                        size: 64
                        ink: Lithic.fg3
                    }
                    Label {
                        anchors.centerIn: big
                        visible: host.shown === ""
                        width: parent.width - 40
                        horizontalAlignment: Text.AlignHCenter
                        text: I18n.tr("Rest on a picture to see it here. Click to choose it, or double-click to use it.")
                        typeRole: "detail"
                        inkRole: "fg3"
                        wrapMode: Text.Wrap
                        elide: Text.ElideNone
                    }
                    Label {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.bottom: parent.bottom
                        anchors.margins: 12
                        visible: host.shown !== ""
                        text: host.nameOf(host.shown)
                        typeRole: "monoSm"
                        inkRole: "fg2"
                        elide: Text.ElideMiddle
                    }
                }
            }
        }
    }
}
