pragma ComponentBehavior: Bound

import QtCore
import QtQuick
import QtQuick.Controls as C
import Qt.labs.folderlistmodel
import Lithic
import Ryoku.Ui.Singletons

// takes the folder it shows, unlike Qt's folder dialog; the folder API of Ryoku.Ui's FileDialogHost
Item {
    id: host

    visible: false
    width: 0
    height: 0

    property string title: I18n.tr("Choose a folder")
    property url currentFolder: StandardPaths.writableLocation(StandardPaths.HomeLocation)
    property url selectedFolder: ""
    // where the dialog centres; the window's overlay when unset
    property Item overlayParent: null
    readonly property bool showing: dialog.visible
    readonly property bool opened: dialog.opened
    // the folder on screen, a local path
    readonly property string shownPath: host.path

    signal accepted()
    signal rejected()

    readonly property string home: host.localPath(StandardPaths.writableLocation(StandardPaths.HomeLocation))
    property string path: ""
    // the folder just left by Up, selected once its parent lists
    property string cameFrom: ""
    property bool chose: false
    readonly property Item windowOverlay: C.Overlay.overlay

    // every open starts at the host's folder, or the nearest folder above it that exists
    function open(): void {
        if (host.showing)
            return;
        host.chose = false;
        host.cameFrom = "";
        host.path = host.startPath();
        list.currentIndex = -1;
        // the list holds the keys on every open, not a control a past open left focused
        list.focus = true;
        dialog.open();
        Qt.callLater(host.settle);
    }

    function close(): void {
        dialog.close();
    }

    function localPath(url): string {
        var s = String(url || "");
        if (s.indexOf("file://") === 0)
            s = decodeURIComponent(s.slice(7));
        else if (s.indexOf("~") === 0)
            s = host.home + s.slice(1);
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

    function join(p, name): string {
        return (p === "/" ? "" : p) + "/" + name;
    }

    function shortPath(p): string {
        if (host.home.length > 1 && (p === host.home || p.indexOf(host.home + "/") === 0))
            return "~" + p.slice(host.home.length);
        return p;
    }

    // a folder model reports a parent only when that parent is a folder that exists
    function isFolder(p): bool {
        if (p === "/")
            return true;
        probe.folder = host.urlOf(host.join(p, ".ryoku-folder-probe"));
        var found = String(probe.parentFolder) !== "";
        probe.folder = "";
        return found;
    }

    function startPath(): string {
        var p = host.localPath(host.currentFolder);
        if (p === "")
            p = host.home.length > 0 ? host.home : "/";
        while (!host.isFolder(p))
            p = host.parentOf(p);
        return p;
    }

    function enter(index): void {
        if (index < 0 || index >= folders.count)
            return;
        host.cameFrom = "";
        host.path = host.join(host.path, folders.get(index, "fileName"));
        list.currentIndex = -1;
    }

    function up(): void {
        if (host.path === "/")
            return;
        host.cameFrom = host.nameOf(host.path);
        host.path = host.parentOf(host.path);
        list.currentIndex = -1;
    }

    // once a folder lists, select the folder Up came from, else the first
    function settle(): void {
        if (folders.status !== FolderListModel.Ready || folders.count === 0)
            return;
        var at = 0;
        for (var i = 0; host.cameFrom !== "" && i < folders.count; ++i) {
            if (folders.get(i, "fileName") === host.cameFrom) {
                at = i;
                break;
            }
        }
        list.select(at);
    }

    FolderListModel {
        id: folders
        folder: host.path === "" ? "" : host.urlOf(host.path)
        showFiles: false
        showHidden: false
        showDotAndDotDot: false
        sortField: FolderListModel.Name
        sortCaseSensitive: false
        onStatusChanged: host.settle()
        onCountChanged: host.settle()
    }

    FolderListModel {
        id: probe
        showFiles: false
    }

    Dialog {
        id: dialog

        parent: host.overlayParent ? host.overlayParent : host.windowOverlay
        title: host.title
        actions: [
            { text: I18n.tr("Cancel"), kind: "quiet" },
            { text: I18n.tr("Use this folder"), kind: "gold", enabled: host.path !== "" }
        ]
        onActionTriggered: index => {
            if (index !== 1)
                return;
            host.chose = true;
            host.selectedFolder = host.urlOf(host.path);
            host.accepted();
        }
        onClosed: {
            if (!host.chose)
                host.rejected();
        }

        Column {
            id: body
            width: parent ? parent.width : 0
            spacing: LithicTokens.space.s4

            readonly property real rowStep: LithicTokens.size.factsRowHeight + LithicTokens.size.rowListGap
            // the rest of the dialog: padding, a two-line title, this header, the actions
            readonly property real chrome: LithicTokens.size.dialogPadding * 2 + LithicTokens.type.sentence.line * 2
                + LithicTokens.size.dialogTitleGap + header.height + body.spacing
                + LithicTokens.size.dialogActionsGap + LithicTokens.size.plaqueHeight
            readonly property real room: (dialog.parent ? dialog.parent.height : 700)
                - LithicTokens.size.dialogMargin * 2 - body.chrome
            readonly property int rows: Math.max(3, Math.min(7, Math.floor((body.room + LithicTokens.size.rowListGap) / body.rowStep)))

            Item {
                id: header
                width: parent.width
                height: Math.max(upButton.height, where.implicitHeight)

                IconPlaque {
                    id: upButton
                    objectName: "folderChooserUp"
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    kind: "tonal"
                    glyph: "arrow_upward"
                    enabled: host.path !== "" && host.path !== "/"
                    accessibleName: I18n.tr("Up to %1").arg(host.nameOf(host.parentOf(host.path)))
                    onClicked: host.up()
                }

                Column {
                    id: where
                    anchors.left: upButton.right
                    anchors.leftMargin: LithicTokens.space.s3
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 2

                    Label {
                        width: parent.width
                        text: host.nameOf(host.path)
                        typeRole: "title"
                        elide: Text.ElideRight
                    }
                    Label {
                        width: parent.width
                        text: host.shortPath(host.path)
                        typeRole: "detail"
                        inkRole: "fg2"
                        elide: Text.ElideMiddle
                    }
                }
            }

            RowList {
                id: list
                objectName: "folderChooserList"
                width: parent.width
                height: body.rows * body.rowStep - LithicTokens.size.rowListGap
                focus: true
                model: folders
                Accessible.name: host.shortPath(host.path)
                Keys.onPressed: event => {
                    switch (event.key) {
                    case Qt.Key_Up:
                        list.select(Math.max(0, list.currentIndex - 1));
                        break;
                    case Qt.Key_Down:
                        list.select(Math.min(folders.count - 1, list.currentIndex + 1));
                        break;
                    case Qt.Key_Home:
                        list.select(0);
                        break;
                    case Qt.Key_End:
                        list.select(folders.count - 1);
                        break;
                    case Qt.Key_Return:
                    case Qt.Key_Enter:
                    case Qt.Key_Space:
                        host.enter(list.currentIndex);
                        break;
                    default:
                        return;
                    }
                    event.accepted = true;
                }

                delegate: Item {
                    id: entry
                    required property string fileName
                    required property int index
                    width: ListView.view.width - LithicTokens.size.scrollThumb - LithicTokens.space.s1
                    height: LithicTokens.size.factsRowHeight
                    Accessible.role: Accessible.ListItem
                    Accessible.name: entry.fileName

                    Glyph {
                        id: mark
                        anchors.left: parent.left
                        anchors.leftMargin: LithicTokens.space.s3
                        anchors.verticalCenter: parent.verticalCenter
                        name: "folder"
                        size: LithicTokens.size.iconPlaqueIconSmall
                        ink: entry.ListView.isCurrentItem ? Lithic.onSel2 : Lithic.fg3
                    }
                    Label {
                        anchors.left: mark.right
                        anchors.leftMargin: LithicTokens.space.s3
                        anchors.right: parent.right
                        anchors.rightMargin: LithicTokens.space.s3
                        anchors.verticalCenter: parent.verticalCenter
                        text: entry.fileName
                        typeRole: "row"
                        ink: entry.ListView.isCurrentItem ? Lithic.onSel : Lithic.fg
                        elide: Text.ElideRight
                    }
                    MouseArea {
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onEntered: list.currentIndex = entry.index
                        onClicked: host.enter(entry.index)
                    }
                }

                Label {
                    anchors.centerIn: parent
                    visible: folders.status === FolderListModel.Ready && folders.count === 0
                    text: I18n.tr("No folders inside")
                    typeRole: "detail"
                    inkRole: "fg2"
                }
            }
        }
    }
}
