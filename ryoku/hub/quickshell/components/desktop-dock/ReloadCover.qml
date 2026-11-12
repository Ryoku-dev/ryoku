import QtQuick
import Quickshell
import Quickshell.Io
import Lithic
import Ryoku.Ui as RU
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../../ReloadCoverModel.js" as ReloadCoverModel

// what shows while the desktop reloads: the Ryoku mark, or a picture or
// video of your own, copied into Ryoku's data folder on import
Slab {
    id: root

    required property Item host
    property string pageId: "desktop-dock"

    readonly property var bind: ({ store: "reloadcover", key: "cover" })
    readonly property var media: ReloadCoverModel.normalize(Changes.value(root.bind))
    readonly property bool custom: root.media.path !== ""
    readonly property var row: ({ bind: root.bind, "default": ReloadCoverModel.empty(), label: "Your own picture or video" })
    readonly property url rendererSource: ReloadCoverModel.rendererSource(
        Quickshell.env("RYOKU_SHELL_DIR"), Quickshell.env("XDG_CONFIG_HOME"), Quickshell.env("HOME"))

    property bool importing: false
    property bool slow: false
    property bool stalled: false
    property string importError: ""
    property string lastPath: ""
    property bool cancelled: false
    // the running import; it lives in Changes, so its answer is checked against the page's life
    property var importProc: null
    readonly property var life: ({ alive: true })
    Component.onDestruction: root.life.alive = false

    readonly property bool opened: fold.open

    function openMore() {
        fold.open = true;
    }

    function assetLine() {
        if (!root.custom)
            return I18n.tr("The Ryoku mark shows while the desktop reloads.");
        var size = ReloadCoverModel.formatBytes(root.media.bytes);
        var kind = root.media.kind === "video" ? I18n.tr("video, plays muted")
            : root.media.kind === "animated" ? I18n.tr("animated picture") : I18n.tr("picture");
        var line = size !== "" ? I18n.tr("%1: %2, %3").arg(root.media.name).arg(kind).arg(size)
                               : I18n.tr("%1: %2").arg(root.media.name).arg(kind);
        return root.media.enabled ? line : I18n.tr("%1. Off, so the Ryoku mark shows.").arg(line);
    }

    function localPath(value) {
        var p = String(value || "");
        if (p.indexOf("file://") === 0)
            p = decodeURIComponent(p.slice(7));
        return p;
    }

    function startImport(path) {
        path = root.localPath(path).trim();
        if (path === "" || root.importing)
            return;
        root.lastPath = path;
        root.importError = "";
        root.importing = true;
        root.slow = false;
        root.stalled = false;
        root.cancelled = false;
        slowMark.restart();
        stallMark.restart();
        var life = root.life;
        // a missing ryoku-hub ends in an error at once instead of a spinner, and a hang ends after 30 s
        root.importProc = Changes.run(["ryoku-hub", "reload-cover", "import", path], function (ok, out, err, code, late) {
            if (life.alive)
                root.finishImport(ok, out, err, code, late);
        }, { timeout: 30000 });
    }

    function cancelImport() {
        if (!root.importProc)
            return;
        root.cancelled = true;
        root.importProc.running = false;
    }

    function finishImport(ok, out, err, code, late) {
        root.importProc = null;
        slowMark.stop();
        stallMark.stop();
        // the Cancel row hides now: its plaque gives the keyboard to the control before it first
        if (cancelImportPlaque.activeFocus) {
            var back = cancelImportPlaque.nextItemInFocusChain(false);
            if (back && back !== cancelImportPlaque)
                back.forceActiveFocus(Qt.BacktabFocusReason);
        }
        root.importing = false;
        root.slow = false;
        root.stalled = false;
        if (root.cancelled) {
            root.cancelled = false;
            return;
        }
        if (ok) {
            try {
                var d = ReloadCoverModel.normalize(JSON.parse(out.trim()));
                if (d.path === "")
                    throw new Error("missing path");
                Changes.set(root.bind, d, root.row.label, root.pageId, I18n.tr("%1 now shows while the desktop reloads").arg(d.name));
                return;
            } catch (e) {}
        }
        var why = err.trim().replace(/^ryoku-hub:\s*/, "").replace(/[.\s]+$/, "");
        // a missing tool or a hang already says what to do next; the importer's own refusals are
        // written for developers, so the page names the ones a person can fix and keeps the rest general
        var choose = I18n.tr("Choose a picture or video on this computer, under 64 MB.");
        root.importError = late ? I18n.tr("Could not use this file: copying it took too long. Try again, or choose another file.")
            : code === -1 && why !== "" ? I18n.tr("Could not use this file. %1.").arg(why)
            : /file type/.test(why) ? I18n.tr("Could not use this file: it is not a picture or video. %1").arg(choose)
            : /exceeds/.test(why) ? I18n.tr("Could not use this file: it is larger than 64 MB. %1").arg(choose)
            : /no such file|not found|regular file|local file/i.test(why) ? I18n.tr("Could not use this file: it is not a file on this computer. %1").arg(choose)
            : I18n.tr("Could not use this file. %1").arg(choose);
    }

    Timer { id: slowMark; interval: 1000; onTriggered: root.slow = true }
    Timer { id: stallMark; interval: 10000; onTriggered: root.stalled = true }

    // the preview renderer ships with the shell; without it the page says so instead of failing to load
    FileView {
        id: rendererFile
        property bool present: false
        path: root.localPath(root.rendererSource)
        printErrors: false
        onLoaded: rendererFile.present = true
        onLoadFailed: error => rendererFile.present = false
    }

    RU.FileDialogHost {
        id: chooser
        title: I18n.tr("Choose a picture or video")
        nameFilters: [I18n.tr("Pictures and videos") + " (" + ReloadCoverModel.filters().join(" ") + ")"]
        currentFolder: "file://" + (Quickshell.env("HOME") || "") + "/Pictures"
        onAccepted: root.startImport(chooser.selectedFile)
    }

    Column {
        width: parent.width

        Disclosure {
            id: fold
            width: parent.width
            text: I18n.tr("More options")

            Column {
                width: fold.width
                spacing: 0

                Item {
                    width: parent.width
                    height: preview.height + LithicTokens.space.s3

                    CutRect {
                        id: preview
                        width: Math.min(parent.width, 420)
                        height: width * 9 / 16
                        cut: LithicTokens.cut.small
                        fill: Lithic.field
                        clip: true
                        Accessible.ignored: true

                        Loader {
                            id: renderer
                            anchors.fill: parent
                            active: fold.open && root.visible && rendererFile.present
                            asynchronous: true
                            source: active ? root.rendererSource : ""
                            onLoaded: {
                                if (status === Loader.Ready && item) {
                                    item.descriptor = Qt.binding(() => root.media);
                                    item.active = Qt.binding(() => renderer.active);
                                    item.forceDefault = false;
                                }
                            }
                        }

                        Label {
                            anchors.centerIn: parent
                            width: parent.width - LithicTokens.space.s5 * 2
                            visible: !rendererFile.present || renderer.status === Loader.Error
                            horizontalAlignment: Text.AlignHCenter
                            text: I18n.tr("No preview here. The desktop still shows the Ryoku mark when a file cannot play.")
                            typeRole: "detail"
                            inkRole: "fg3"
                            wrapMode: Text.Wrap
                            elide: Text.ElideNone
                        }
                    }
                }

                OwnRow {
                    host: root.host
                    rowId: "reload-cover"
                    first: true
                    title: I18n.tr("Your own picture or video")
                    detail: root.assetLine()
                    activatable: root.custom
                    changed: Changes.isChanged(root.row)
                    onActivated: Changes.set(root.bind, Object.assign({}, root.media, { enabled: !root.media.enabled }), root.row.label, root.pageId)
                    onResetRequested: Changes.set(root.bind, ReloadCoverModel.empty(), root.row.label, root.pageId, I18n.tr("The Ryoku mark shows while the desktop reloads"))

                    Toggle {
                        anchors.verticalCenter: parent.verticalCenter
                        enabled: root.custom
                        checked: root.custom && root.media.enabled
                        Accessible.name: I18n.tr("Your own picture or video")
                        onToggled: Changes.set(root.bind, Object.assign({}, root.media, { enabled: checked }), root.row.label, root.pageId)
                    }
                }

                RowNote {
                    width: parent.width
                    text: Changes.error(root.bind)
                    onRetry: Changes.retry(root.bind)
                }

                OwnRow {
                    host: root.host
                    rowId: "reload-cover-file"
                    title: I18n.tr("Picture or video")
                    detail: I18n.tr("Best results: the same shape as your display (1920 by 1080 for most), a 2 to 4 second loop, under 20 MB. A transparent PNG or SVG suits a mark in the middle; for motion use MP4 or WebM at 24 to 30 frames a second. Every display shows the same file, fitted to it.")

                    Row {
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: LithicTokens.space.s2

                        Working {
                            anchors.verticalCenter: parent.verticalCenter
                            visible: active
                            active: root.slow
                        }
                        PathField {
                            width: 280
                            kind: "file"
                            path: root.custom ? root.media.name : ""
                            placeholder: I18n.tr("Choose a picture or video")
                            accessibleName: I18n.tr("Picture or video")
                            // stays enabled while a file is copied, so the keyboard keeps its place;
                            // startImport ignores a file given meanwhile
                            errorText: root.importError !== "" ? I18n.tr("This file was not used") : ""
                            onChooseRequested: chooser.open()
                            onEdited: path => {
                                if (path !== "" && path !== root.media.name)
                                    root.startImport(path);
                            }
                        }
                    }
                }

                Row {
                    visible: root.stalled && root.importing
                    width: parent.width
                    topPadding: LithicTokens.space.s2
                    spacing: LithicTokens.space.s3

                    BladeProgress {
                        anchors.verticalCenter: parent.verticalCenter
                        width: parent.width - cancelImportPlaque.width - parent.spacing
                        active: parent.visible
                        indeterminate: true
                    }
                    Plaque {
                        id: cancelImportPlaque
                        kind: "quiet"
                        small: true
                        text: I18n.tr("Cancel")
                        onClicked: root.cancelImport()
                    }
                }

                RowNote {
                    width: parent.width
                    topPadding: LithicTokens.space.s1
                    text: root.importError
                    onRetry: root.startImport(root.lastPath)
                }
            }
        }
    }
}
