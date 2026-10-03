pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons

Item {
    id: root

    property real s: 1
    property bool open: false
    property string mode: "compress"
    signal cancelled()
    signal confirmed(var paths)

    readonly property string home: Quickshell.env("HOME") || ""
    property url folder: "file://" + root.home
    property var selected: ({})
    readonly property int selCount: Object.keys(root.selected).length

    readonly property var videoImage: ["*.mp4", "*.mkv", "*.webm", "*.mov", "*.avi", "*.m4v", "*.png", "*.jpg", "*.jpeg", "*.webp", "*.bmp", "*.gif", "*.tiff"]
    readonly property var appFiles: ["*.AppImage", "*.pkg.tar.zst", "*.pkg.tar.xz", "*.deb", "*.rpm", "*.flatpak", "*.tar.gz", "*.tgz", "*.tar.xz", "*.tar.bz2", "*.tar.zst", "*.tar"]

    visible: opacity > 0.01
    enabled: root.open
    opacity: root.open ? 1 : 0
    z: 40

    Behavior on opacity { NumberAnimation { duration: Tokens.dur(150); easing.type: Tokens.ease } }

    onOpenChanged: {
        if (!root.open)
            return;
        root.selected = ({});
        root.folder = "file://" + root.home;
        Qt.callLater(root.refresh);
    }
    onFolderChanged: if (root.open) root.refresh()
    onModeChanged: if (root.open) root.refresh()

    function toggle(path) {
        const next = Object.assign({}, root.selected);
        if (next[path])
            delete next[path];
        else
            next[path] = true;
        root.selected = next;
    }

    function goUp() {
        const current = String(root.folder);
        if (current === "file:///" || current === "file://" + "/")
            return;
        root.folder = current.replace(/\/[^/]+\/?$/, "") || "file:///";
    }

    function chosenPaths() { return Object.keys(root.selected); }

    function localFolder() {
        var path = String(root.folder).replace(/^file:\/\//, "");
        try { return decodeURIComponent(path); }
        catch (error) { return path; }
    }

    function accepts(name) {
        var lower = String(name).toLowerCase();
        var filters = root.mode === "install" ? root.appFiles : root.videoImage;
        for (var i = 0; i < filters.length; i++) {
            var suffix = String(filters[i]).slice(1).toLowerCase();
            if (lower.endsWith(suffix))
                return true;
        }
        return false;
    }

    function refresh() {
        files.clear();
        folderScan.running = false;
        folderScan.command = ["find", root.localFolder(), "-mindepth", "1", "-maxdepth", "1",
            "-printf", "%y\\t%f\\t%p\\n"];
        folderScan.running = true;
    }

    function fileGlyph(name) {
        const ext = String(name).toLowerCase().split(".").pop();
        if (/^(png|jpe?g|webp|gif|bmp|tiff?|avif)$/.test(ext)) return "image";
        if (/^(mp4|mkv|webm|mov|avi|m4v)$/.test(ext)) return "movie";
        return "deployed_code";
    }

    ListModel { id: files }

    Process {
        id: folderScan
        stdout: StdioCollector {
            id: scanOutput
            onStreamFinished: {
                var rows = [];
                var lines = String(scanOutput.text).split("\n");
                for (var i = 0; i < lines.length; i++) {
                    if (lines[i].length === 0)
                        continue;
                    var fields = lines[i].split("\t");
                    if (fields.length < 3)
                        continue;
                    var directory = fields[0] === "d";
                    var name = fields[1];
                    var path = fields.slice(2).join("\t");
                    if (name.charAt(0) === "." || (!directory && !root.accepts(name)))
                        continue;
                    rows.push({ fileName: name, filePath: path, fileIsDir: directory });
                }
                rows.sort((a, b) => {
                    if (a.fileIsDir !== b.fileIsDir)
                        return a.fileIsDir ? -1 : 1;
                    return a.fileName.localeCompare(b.fileName, undefined, { sensitivity: "base" });
                });
                files.clear();
                for (var j = 0; j < rows.length; j++)
                    files.append(rows[j]);
            }
        }
    }

    Rectangle {
        anchors.fill: parent
        radius: Tokens.radius * root.s
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.lineStrong

        Column {
            anchors.fill: parent
            anchors.margins: Tokens.s5 * root.s
            spacing: Tokens.s3 * root.s

            Item {
                width: parent.width
                height: 34 * root.s

                IconBtn {
                    id: closeButton
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    glyph: "←"
                    onAct: root.cancelled()
                }

                Column {
                    anchors.left: closeButton.right
                    anchors.right: parent.right
                    anchors.leftMargin: Tokens.s3 * root.s
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 0
                    Text {
                        width: parent.width
                        text: root.mode === "install" ? I18n.tr("Select apps to install") : I18n.tr("Select videos or images")
                        color: Tokens.ink
                        font.family: Tokens.display
                        font.pixelSize: Tokens.fValue * root.s
                        elide: Text.ElideRight
                    }
                    Text {
                        width: parent.width
                        text: root.selCount === 0 ? I18n.tr("Choose one or more files") : I18n.tr("%1 selected").arg(root.selCount)
                        color: Tokens.inkMuted
                        font.family: Tokens.mono
                        font.pixelSize: Tokens.fTiny * root.s
                    }
                }
            }

            Rectangle {
                width: parent.width
                height: 34 * root.s
                radius: Tokens.radius * root.s
                color: Tokens.tint5
                border.width: Tokens.border
                border.color: Tokens.lineSoft

                IconBtn {
                    id: upButton
                    anchors.left: parent.left
                    anchors.leftMargin: Tokens.s1 * root.s
                    anchors.verticalCenter: parent.verticalCenter
                    glyph: "↑"
                    onAct: root.goUp()
                }
                Text {
                    anchors.left: upButton.right
                    anchors.right: parent.right
                    anchors.leftMargin: Tokens.s2 * root.s
                    anchors.rightMargin: Tokens.s3 * root.s
                    anchors.verticalCenter: parent.verticalCenter
                    text: String(root.folder).replace("file://" + root.home, "~").replace("file://", "")
                    elide: Text.ElideLeft
                    color: Tokens.inkDim
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fTiny * root.s
                }
            }

            ListView {
                id: list
                width: parent.width
                height: parent.height - y - footer.height - parent.spacing
                clip: true
                model: files
                spacing: Tokens.s1 * root.s
                boundsBehavior: Flickable.StopAtBounds
                ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }

                delegate: Rectangle {
                    id: fileRow
                    required property string fileName
                    required property string filePath
                    required property bool fileIsDir
                    readonly property bool selected: !fileRow.fileIsDir && root.selected[fileRow.filePath] === true
                    width: ListView.view ? ListView.view.width : 0
                    height: 42 * root.s
                    radius: Tokens.radius * root.s
                    color: fileRow.selected ? Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.15)
                        : rowHover.hovered ? Tokens.tint10 : "transparent"
                    border.width: Tokens.border
                    border.color: fileRow.selected ? Tokens.sun
                        : rowHover.hovered ? Tokens.lineStrong : Tokens.lineSoft
                    scale: rowTap.pressed && !Tokens.reduceMotion ? 0.98 : 1
                    Behavior on scale { NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap } }
                    Behavior on color { ColorAnimation { duration: Tokens.snap } }

                    Text {
                        font.family: "Material Symbols Rounded"
                        id: icon
                        anchors.left: parent.left
                        anchors.leftMargin: Tokens.s3 * root.s
                        anchors.verticalCenter: parent.verticalCenter
                        text: fileRow.fileIsDir ? "folder" : root.fileGlyph(fileRow.fileName)
                        color: fileRow.fileIsDir || fileRow.selected ? Tokens.sun : Tokens.inkMuted
                        font.pixelSize: Tokens.fBody * root.s
                    }
                    Text {
                        anchors.left: icon.right
                        anchors.right: stateIcon.left
                        anchors.leftMargin: Tokens.s3 * root.s
                        anchors.rightMargin: Tokens.s2 * root.s
                        anchors.verticalCenter: parent.verticalCenter
                        text: fileRow.fileName
                        elide: Text.ElideMiddle
                        color: Tokens.ink
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall * root.s
                        font.weight: Font.Medium
                    }
                    Text {
                        font.family: "Material Symbols Rounded"
                        id: stateIcon
                        anchors.right: parent.right
                        anchors.rightMargin: Tokens.s3 * root.s
                        anchors.verticalCenter: parent.verticalCenter
                        text: fileRow.fileIsDir ? "chevron_right" : fileRow.selected ? "check_circle" : "radio_button_unchecked"
                        color: fileRow.selected ? Tokens.sun : Tokens.inkFaint
                        font.pixelSize: Tokens.fBody * root.s
                    }

                    HoverHandler { id: rowHover; cursorShape: Qt.PointingHandCursor }
                    TapHandler {
                        id: rowTap
                        onTapped: fileRow.fileIsDir
                            ? (root.folder = "file://" + fileRow.filePath)
                            : root.toggle(fileRow.filePath)
                    }
                }
            }

            Row {
                id: footer
                width: parent.width
                spacing: Tokens.s2 * root.s

                Btn {
                    id: cancelButton
                    text: I18n.tr("Cancel")
                    onAct: root.cancelled()
                }
                Item {
                    width: Math.max(0, parent.width - cancelButton.width - confirmButton.width - parent.spacing * 2)
                    height: 1
                }
                Btn {
                    id: confirmButton
                    text: root.selCount > 0
                        ? (root.mode === "install" ? I18n.tr("Install %1").arg(root.selCount) : I18n.tr("Compress %1").arg(root.selCount))
                        : (root.mode === "install" ? I18n.tr("Select apps to install") : I18n.tr("Select files to compress"))
                    primary: true
                    armed: root.selCount > 0
                    onAct: root.confirmed(root.chosenPaths())
                }
            }
        }
    }
}
