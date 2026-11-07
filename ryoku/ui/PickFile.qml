pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Qt.labs.folderlistmodel
import Quickshell
import Ryoku.Ui.Singletons

// A monochrome file/folder picker modal (DESIGN.md section 6 overlay:
// paperLift + lineStrong, no shadow). Shared by every surface that needs to
// choose an image or a folder (the shell brand mark, an image border, a rice
// wallpaper, a rice export/import target), so the browser lives once instead of
// being copied per page. `foldersOnly` turns the image grid into a folder
// chooser with a "use this folder" action. `open()` shows it at `startFolder`.
Item {
    id: fp
    property bool active: false
    property bool foldersOnly: false
    // override the image glob for non-image pickers (e.g. ["*.svg"], ["*.txt"],
    // ["*.ryoprofile"]); empty keeps the default image set (png/jpg/webp/gif/svg).
    property var fileFilters: []
    property string title: I18n.tr("Choose a file")
    property string emptyText: ""
    property string home: Quickshell.env("HOME") || ""
    property url startFolder: "file://" + fp.home + "/Pictures"
    property url currentFolder: fp.startFolder
    signal picked(string path)
    signal canceled()

    function open() {
        fp.currentFolder = fp.startFolder;
        fp.active = true;
        fp.forceActiveFocus();
    }
    function goHome(sub) { fp.currentFolder = "file://" + fp.home + (sub.length ? "/" + sub : ""); }

    anchors.fill: parent
    visible: fp.active
    z: 200
    focus: fp.active
    onActiveChanged: if (fp.active) fp.forceActiveFocus()
    Keys.onEscapePressed: (event) => {
        fp.canceled();
        event.accepted = true;
    }

    FolderListModel {
        id: fm
        folder: fp.currentFolder
        showDirs: true
        showDirsFirst: true
        showDotAndDotDot: false
        showHidden: false
        nameFilters: fp.foldersOnly ? ["ryoku-no-match"]
            : (fp.fileFilters.length ? fp.fileFilters : ["*.png", "*.jpg", "*.jpeg", "*.bmp", "*.webp", "*.gif", "*.svg"])
        sortField: FolderListModel.Name
    }

    MouseArea { anchors.fill: parent; onClicked: fp.canceled() }

    Rectangle {
        anchors.centerIn: parent
        width: Math.min(parent.width - Tokens.s7 * 2, 900)
        height: Math.min(parent.height - Tokens.s6 * 2, 620)
        radius: Tokens.radius
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        MouseArea { anchors.fill: parent; onClicked: {} }

        Text {
            id: fpTitle
            anchors { left: parent.left; top: parent.top; leftMargin: Tokens.s5; topMargin: Tokens.s4 }
            text: fp.title.toUpperCase()
            color: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: Tokens.fMicro
            font.weight: Font.Medium
            font.letterSpacing: Tokens.trackLabel
        }
        Text {
            anchors { left: fpTitle.left; top: fpTitle.bottom; topMargin: Tokens.s1; right: fpClose.left; rightMargin: Tokens.s3 }
            elide: Text.ElideLeft
            text: ("" + fp.currentFolder).replace("file://", "").replace(fp.home, "~")
            color: Tokens.inkFaint
            font.family: Tokens.mono
            font.pixelSize: Tokens.fTiny
        }
        IconBtn {
            id: fpClose
            anchors { right: parent.right; top: parent.top; rightMargin: Tokens.s4; topMargin: Tokens.s4 }
            glyph: "×"
            onAct: fp.canceled()
        }

        Flow {
            id: fpNav
            anchors { left: parent.left; right: parent.right; top: fpTitle.bottom; leftMargin: Tokens.s5; rightMargin: Tokens.s5; topMargin: Tokens.s5 }
            spacing: Tokens.s2
            Btn { text: I18n.tr("UP"); onAct: fp.currentFolder = fm.parentFolder }
            Btn { text: I18n.tr("HOME"); onAct: fp.goHome("") }
            Btn { text: I18n.tr("PICTURES"); onAct: fp.goHome("Pictures") }
            Btn { text: I18n.tr("DOWNLOADS"); onAct: fp.goHome("Downloads") }
        }

        GridView {
            id: fpGrid
            anchors {
                left: parent.left; right: parent.right
                top: fpNav.bottom; bottom: fpFoot.top
                leftMargin: Tokens.s4; rightMargin: Tokens.s3
                topMargin: Tokens.s4; bottomMargin: Tokens.s2
            }
            clip: true
            readonly property int cols: Math.max(3, Math.floor(width / 190))
            cellWidth: Math.floor(width / cols)
            cellHeight: Math.round(cellWidth * 0.72)
            cacheBuffer: 1200
            boundsBehavior: Flickable.StopAtBounds
            model: fm
            ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }

            delegate: Item {
                id: fpTile
                required property string fileName
                required property url fileUrl
                required property bool fileIsDir
                readonly property bool isImg: /\.(png|jpe?g|bmp|webp|gif|svg)$/i.test(fpTile.fileName)
                width: fpGrid.cellWidth
                height: fpGrid.cellHeight

                Rectangle {
                    anchors.fill: parent
                    anchors.margins: Tokens.s1
                    radius: Tokens.radius
                    color: fpTile.fileIsDir ? (tap.pressed ? Tokens.tint16 : (tHov.hovered ? Tokens.tint5 : "transparent")) : "transparent"
                    border.width: Tokens.border
                    border.color: tHov.hovered ? Tokens.lineStrong : Tokens.line
                    clip: true
                    Behavior on border.color { ColorAnimation { duration: Tokens.snap } }
                    Behavior on color { ColorAnimation { duration: Tokens.snap } }

                    Column {
                        visible: fpTile.fileIsDir
                        anchors.centerIn: parent
                        spacing: Tokens.s2
                        Text {
                            anchors.horizontalCenter: parent.horizontalCenter
                            text: I18n.tr("DIR")
                            color: Tokens.inkMuted
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fTiny
                        }
                        Text {
                            width: fpTile.width - Tokens.s5
                            horizontalAlignment: Text.AlignHCenter
                            elide: Text.ElideMiddle
                            text: fpTile.fileName
                            color: Tokens.inkDim
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fTiny
                        }
                    }
                    // a non-image file (an ascii art, a .ryoprofile): a doc tile
                    // rather than a broken thumbnail, so PickFile serves every
                    // pick coherently, not just images.
                    Column {
                        visible: !fpTile.fileIsDir && !fpTile.isImg
                        anchors.centerIn: parent
                        spacing: Tokens.s2
                        Text {
                            anchors.horizontalCenter: parent.horizontalCenter
                            text: "\uf15b"
                            color: Tokens.inkMuted
                            font.family: Tokens.mono
                            font.pixelSize: 20
                        }
                        Text {
                            width: fpTile.width - Tokens.s5
                            horizontalAlignment: Text.AlignHCenter
                            elide: Text.ElideMiddle
                            text: fpTile.fileName
                            color: Tokens.inkDim
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fTiny
                        }
                    }

                    Image {
                        visible: !fpTile.fileIsDir && fpTile.isImg
                        anchors.fill: parent
                        anchors.margins: 1
                        asynchronous: true
                        cache: true
                        fillMode: Image.PreserveAspectCrop
                        sourceSize: Qt.size(Math.ceil(parent.width * 1.4), Math.ceil(parent.height * 1.4))
                        source: fpTile.fileIsDir ? "" : fpTile.fileUrl
                    }
                    Rectangle {
                        visible: !fpTile.fileIsDir && tHov.hovered
                        anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
                        height: 18
                        color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.72)
                        Text {
                            anchors.fill: parent
                            anchors.leftMargin: Tokens.s2
                            anchors.rightMargin: Tokens.s2
                            verticalAlignment: Text.AlignVCenter
                            elide: Text.ElideMiddle
                            text: fpTile.fileName
                            color: Tokens.ink
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fTiny
                        }
                    }
                    HoverHandler { id: tHov; cursorShape: Qt.PointingHandCursor }
                    TapHandler {
                        id: tap
                        onTapped: {
                            if (fpTile.fileIsDir) fp.currentFolder = fpTile.fileUrl;
                            else fp.picked("" + fpTile.fileUrl);
                        }
                    }
                }
            }
        }

        Text {
            anchors.centerIn: fpGrid
            visible: fm.status === FolderListModel.Ready && fm.count === 0
            text: fp.emptyText !== ""
                ? fp.emptyText
                : (fp.foldersOnly ? I18n.tr("No folders here") : I18n.tr("No images or folders here"))
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
        }

        Item {
            id: fpFoot
            anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
            height: 56
            Rectangle { height: 1; color: Tokens.lineSoft; anchors { left: parent.left; right: parent.right; top: parent.top } }
            Btn {
                visible: fp.foldersOnly
                anchors { left: parent.left; leftMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
                text: I18n.tr("USE THIS FOLDER")
                primary: true
                onAct: fp.picked("" + fp.currentFolder)
            }
            Btn {
                anchors { right: parent.right; rightMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
                text: I18n.tr("CANCEL")
                onAct: fp.canceled()
            }
        }
    }
}
