import QtQuick
import Lithic
import Qt.labs.folderlistmodel
import Quickshell
import Ryoku.Ui.Singletons

// Browses folders for the rice share dialog. "export" walks to a destination; "import"
// marks every folder holding a rice.json as a rice tile with its preview, so a shared
// rice is recognised on sight instead of hunted for by name.
Item {
    id: root

    property string mode: "export"
    readonly property string home: Quickshell.env("HOME") || ""
    property string path: root.home
    // The rice folder chosen by a single click; the current folder counts when it is one.
    property string selected: ""
    readonly property bool pathIsRice: here.count > 0
    readonly property string riceFolder: root.selected !== "" ? root.selected
        : (root.pathIsRice ? root.path : "")

    signal riceActivated(string path)

    function open(p) {
        root.selected = ""
        root.path = p
    }
    function up() {
        var p = root.path.replace(/\/+$/, "")
        var cut = p.lastIndexOf("/")
        root.open(cut > 0 ? p.substring(0, cut) : "/")
    }
    function tidy(p) {
        return (root.home !== "" && p.indexOf(root.home) === 0) ? "~" + p.substring(root.home.length) : p
    }
    function urlToPath(u) { return decodeURIComponent(String(u).replace(/^file:\/\//, "")) }

    readonly property var places: [
        { label: I18n.tr("Home"), glyph: "\u{f02dc}", sub: "" },
        { label: I18n.tr("Rices"), glyph: "\u{f03d8}", sub: "Rices" },
        { label: I18n.tr("Downloads"), glyph: "\u{f01da}", sub: "Downloads" },
        { label: I18n.tr("Documents"), glyph: "\u{f0219}", sub: "Documents" },
        { label: I18n.tr("Desktop"), glyph: "\u{f0379}", sub: "Desktop" }
    ]

    FolderListModel {
        id: dirs
        folder: Library.fileUrl(root.path)
        showDirs: true
        showFiles: false
        showDotAndDotDot: false
        showHidden: false
        sortField: FolderListModel.Name
    }
    // Only its count matters: a rice.json in the open folder makes the folder itself a rice.
    FolderListModel {
        id: here
        folder: Library.fileUrl(root.path)
        showDirs: false
        nameFilters: root.mode === "import" ? ["rice.json"] : ["ryoku-no-match"]
    }
    // Places that do not exist on this box stay off the row.
    FolderListModel {
        id: homeDirs
        folder: Library.fileUrl(root.home)
        showFiles: false
        showHidden: false
    }
    function placeExists(sub) {
        if (sub === "") return true
        homeDirs.count
        for (var i = 0; i < homeDirs.count; ++i)
            if (homeDirs.get(i, "fileName") === sub)
                return true
        return false
    }

    Column {
        id: head
        anchors.left: parent.left
        anchors.right: parent.right
        spacing: 10 * Theme.scale

        Flow {
            width: parent.width
            spacing: 6 * Theme.scale
            Repeater {
                model: root.places
                delegate: FixedButton {
                    required property var modelData
                    readonly property string target: modelData.sub === "" ? root.home : root.home + "/" + modelData.sub
                    visible: root.placeExists(modelData.sub)
                    mode: "toggle"
                    glyph: modelData.glyph
                    label: modelData.label
                    hpad: 11
                    active: root.path === target
                    onTriggered: root.open(target)
                }
            }
        }

        CutRect {
            width: parent.width
            height: 32 * Theme.scale
            fill: Theme.withAlpha(Theme.surfaceContainer, 0.7)
            strokeWidth: 1
            strokeColor: Theme.withAlpha(Theme.outline, 0.32)

            FixedButton {
                id: upBtn
                anchors.left: parent.left
                anchors.leftMargin: 3 * Theme.scale
                anchors.verticalCenter: parent.verticalCenter
                height: 26 * Theme.scale
                mode: "action"
                glyph: "\u{f005d}"
                hpad: 8
                enabled: root.path !== "/"
                onTriggered: root.up()
            }
            Text {
                anchors.left: upBtn.right
                anchors.leftMargin: 10 * Theme.scale
                anchors.right: parent.right
                anchors.rightMargin: 10 * Theme.scale
                anchors.verticalCenter: parent.verticalCenter
                text: root.tidy(root.path)
                elide: Text.ElideLeft
                font.family: Theme.sans
                font.weight: Font.Medium
                font.pixelSize: Theme.fontBody
                color: Theme.withAlpha(Theme.surfaceText, 0.82)
                renderType: Text.NativeRendering
            }
        }
    }

    GridView {
        id: grid
        anchors.top: head.bottom
        anchors.topMargin: 10 * Theme.scale
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        readonly property int cols: Math.max(3, Math.floor(width / (150 * Theme.scale)))
        cellWidth: Math.floor(width / cols)
        cellHeight: Math.round(cellWidth * 0.74)
        model: dirs

        delegate: Item {
            id: tile
            required property string fileName
            required property url fileUrl
            readonly property string tilePath: root.urlToPath(fileUrl)
            readonly property bool isRice: root.mode === "import" && riceFiles.count > 0 && riceFiles.hasManifest
            readonly property bool chosen: root.selected !== "" && root.selected === tile.tilePath
            readonly property string thumb: {
                if (!tile.isRice) return ""
                riceFiles.count
                for (var i = 0; i < riceFiles.count; ++i) {
                    var n = riceFiles.get(i, "fileName")
                    if (n !== "rice.json") return riceFiles.get(i, "filePath")
                }
                return ""
            }
            width: grid.cellWidth
            height: grid.cellHeight

            FolderListModel {
                id: riceFiles
                folder: tile.fileUrl
                showDirs: false
                nameFilters: root.mode === "import"
                    ? ["rice.json", "preview.png", "preview.jpg", "preview.webp", "wall.jpg", "wall.jpeg", "wall.png", "wall.webp"]
                    : ["ryoku-no-match"]
                sortField: FolderListModel.Name
                readonly property bool hasManifest: {
                    riceFiles.count
                    for (var i = 0; i < riceFiles.count; ++i)
                        if (riceFiles.get(i, "fileName") === "rice.json") return true
                    return false
                }
            }

            CutRect {
                id: face
                anchors.fill: parent
                anchors.margins: 4 * Theme.scale
                clip: true
                fill: tile.chosen ? Theme.withAlpha(Theme.surfaceText, 0.1)
                     : tileMouse.containsMouse ? Theme.withAlpha(Theme.surfaceVariant, 0.7)
                     : Theme.withAlpha(Theme.surfaceContainer, 0.6)
                strokeWidth: tile.chosen ? 2 : 1
                strokeColor: tile.chosen ? Theme.surfaceText
                            : tileMouse.containsMouse ? Theme.withAlpha(Theme.outline, 0.75)
                            : Theme.withAlpha(Theme.outline, tile.isRice ? 0.5 : 0.25)
                Behavior on fill { ColorAnimation { duration: Theme.fast } }

                Image {
                    anchors.fill: parent
                    anchors.margins: face.strokeWidth
                    visible: tile.thumb !== "" && status === Image.Ready
                    source: tile.thumb !== "" ? Library.fileUrl(tile.thumb) : ""
                    fillMode: Image.PreserveAspectCrop
                    asynchronous: true
                    sourceSize.width: 320
                    sourceSize.height: 240
                }
                Rectangle {
                    visible: tile.isRice
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.bottom: parent.bottom
                    anchors.margins: face.strokeWidth
                    height: parent.height * 0.55
                    gradient: Gradient {
                        GradientStop { position: 0; color: "transparent" }
                        GradientStop { position: 1; color: Theme.withAlpha(Theme.background, 0.92) }
                    }
                }

                Text {
                    visible: !tile.isRice
                    anchors.horizontalCenter: parent.horizontalCenter
                    anchors.bottom: parent.verticalCenter
                    anchors.bottomMargin: 2 * Theme.scale
                    text: "\u{f024b}"
                    font.family: Theme.icon
                    font.pixelSize: Theme.fs(26)
                    color: Theme.withAlpha(Theme.surfaceText, tileMouse.containsMouse ? 0.7 : 0.42)
                    renderType: Text.NativeRendering
                }

                CutRect {
                    visible: tile.isRice
                    anchors.top: parent.top
                    anchors.left: parent.left
                    anchors.margins: 7 * Theme.scale
                    width: badge.implicitWidth + 10 * Theme.scale
                    height: badge.implicitHeight + 4 * Theme.scale
                    fill: Theme.withAlpha(Theme.surfaceText, 0.92)
                    Text {
                        id: badge
                        anchors.centerIn: parent
                        text: I18n.tr("Rice").toUpperCase()
                        font.family: Theme.sans
                        font.weight: Font.DemiBold
                        font.pixelSize: Theme.fontMicro
                        font.letterSpacing: 1.2
                        color: Theme.surface
                        renderType: Text.NativeRendering
                    }
                }

                Text {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.margins: 8 * Theme.scale
                    anchors.top: tile.isRice ? undefined : parent.verticalCenter
                    anchors.topMargin: 6 * Theme.scale
                    anchors.bottom: tile.isRice ? parent.bottom : undefined
                    anchors.bottomMargin: 7 * Theme.scale
                    horizontalAlignment: tile.isRice ? Text.AlignLeft : Text.AlignHCenter
                    text: tile.fileName
                    elide: Text.ElideMiddle
                    font.family: Theme.sans
                    font.weight: Font.Medium
                    font.pixelSize: tile.isRice ? Theme.fontBody : Theme.fontBase
                    color: tile.isRice ? Theme.surfaceText : Theme.withAlpha(Theme.surfaceText, 0.78)
                    renderType: Text.NativeRendering
                }

                MouseArea {
                    id: tileMouse
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: {
                        if (tile.isRice) root.selected = tile.chosen ? "" : tile.tilePath
                        else root.open(tile.tilePath)
                    }
                    onDoubleClicked: {
                        if (tile.isRice) root.riceActivated(tile.tilePath)
                    }
                }
            }
        }
    }

    Text {
        anchors.centerIn: grid
        visible: dirs.status === FolderListModel.Ready && dirs.count === 0
        width: Math.min(grid.width - 40 * Theme.scale, 420 * Theme.scale)
        horizontalAlignment: Text.AlignHCenter
        wrapMode: Text.WordWrap
        text: root.pathIsRice ? I18n.tr("This folder is a rice. Import it as it is.")
            : root.mode === "export" ? I18n.tr("Nothing in here yet. Export here, or open another folder.")
            : I18n.tr("No folders in here.")
        font.family: Theme.sans
        font.weight: Font.Medium
        font.pixelSize: Theme.fontBody
        color: Theme.withAlpha(Theme.surfaceText, 0.5)
        renderType: Text.NativeRendering
    }
}
