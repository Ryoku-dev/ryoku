import QtQuick
import QtQuick.Window
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/screen-capture"
import "../schema/voice-typing.js" as Schema
import "../components/shortcuts/binds.js" as Binds

// voice typing: on or off, which speech model listens, which computer runs it,
// and the microphone it records from
Item {
    id: root

    required property var hub
    property string pendingRow: ""

    readonly property var store: Changes.stores["dictation"] || null
    readonly property bool loaded: root.store !== null && root.store.loaded
    readonly property bool installed: root.loaded && root.store.installed
    readonly property var current: root.store ? root.store.current : null
    readonly property bool usable: root.store !== null && root.store.usableWith(root.store.activeId())
    readonly property var deviceBind: ({ store: "dictation", key: "device" })
    readonly property string deviceError: Changes.error(root.deviceBind)

    // the voice typing shortcut as it is now: its shipped chord, rebound or turned off on Keyboard shortcuts
    readonly property string voiceDef: "SUPER + grave"
    readonly property bool keyKnown: Changes.desktopReady
    readonly property bool keyOff: root.keyKnown && Binds.isOff(Changes.value({ store: "desktop", key: "keybindDisabled" }), root.voiceDef)
    readonly property string keyText: root.keyKnown
        ? Binds.shown(Binds.effective(Changes.value({ store: "desktop", key: "keybindRebinds" }), root.voiceDef), { "grave": "`" })
        : ""

    readonly property string stateLine: !root.loaded ? ""
        : !root.installed ? I18n.tr("The Ryoku engine is not installed.")
        : root.store.enabled && root.current && root.store.activeBackend !== ""
            ? I18n.tr("On, listening with %1 on %2.").arg(I18n.tr(root.current.label)).arg(root.store.activeBackend)
        : root.store.enabled && root.current ? I18n.tr("On, listening with %1.").arg(I18n.tr(root.current.label))
        : I18n.tr("Off.")

    // model rows carry the accuracy and size that decide them, in mono because
    // the characters are the data
    function modelDetail(m) {
        var parts = [I18n.tr(m.language), m.wer];
        var busy = root.store.downloading === m.id;
        if (busy)
            parts.push(I18n.tr("Downloading"));
        else if (root.store.removing === m.id)
            parts.push(I18n.tr("Removing"));
        else if (m.bundled === true)
            parts.push(I18n.tr("Shipped"));
        else
            parts.push(m.installed === true ? I18n.tr("Downloaded") : m.size);
        return parts.join(" · ") + "\n" + I18n.tr(m.detail);
    }

    // "model@<id>" is one model row's way to choose the model: the same value as "model"
    function choiceBind(id) {
        return { store: "dictation", key: "model@" + id };
    }

    function choose(id) {
        var m = root.store.modelFor(id);
        if (!m || root.store.downloading !== "" || root.store.removing !== "")
            return;
        if (m.bundled === true || m.installed === true) {
            Changes.set(root.choiceBind(id), id, "Speech model", "voice-typing");
            return;
        }
        if (Number(m.sizeBytes || 0) > 1000000000) {
            root.hub.confirm({
                title: I18n.tr("Download %1?").arg(I18n.tr(m.label)),
                text: I18n.tr("This model is a %1 download. Voice typing keeps using the model it has until it finishes.").arg(m.size),
                actionText: I18n.tr("Download"),
                store: "dictation"
            }, function () { root.store.download(id); });
            return;
        }
        root.store.download(id);
    }

    function removeModel(id) {
        var m = root.store.modelFor(id);
        if (!m)
            return;
        var inUse = root.store.activeId() === id;
        root.hub.confirm({
            title: I18n.tr("Remove %1?").arg(I18n.tr(m.label)),
            text: inUse ? I18n.tr("The model is deleted from this computer. Voice typing falls back to the shipped model.")
                        : I18n.tr("The model is deleted from this computer. You can download it again."),
            actionText: I18n.tr("Remove"),
            store: "dictation"
        }, function () { root.store.removeModel(id); });
    }

    function installEngine() {
        root.store.installEngine();
    }

    function removeEngine() {
        root.hub.confirm({
            title: I18n.tr("Remove voice typing?"),
            text: I18n.tr("This uninstalls the Ryoku engine. Your model choice and any downloaded models stay on disk."),
            actionText: I18n.tr("Remove"),
            store: "dictation"
        }, function () { root.store.removeEngine(); });
    }

    function sectionIndex(id) {
        for (var i = 0; i < page.layout.length; i++)
            if (page.layout[i].id === id)
                return i;
        return -1;
    }

    function focusRow(rowId) {
        if (!rowId)
            return;
        if (page.layout.length === 0 || !root.loaded) {
            root.pendingRow = rowId;
            return;
        }
        if (page.landingRow(rowId) !== "") {
            page.focusRow(rowId);
            return;
        }
        var row = Schema.rows.find(r => r.id === rowId);
        // a deep link may name a section rather than a row
        var at = root.sectionIndex(row ? row.section : rowId);
        if (at >= 0)
            page.chooseSection(at);
    }

    // a terminal that installed or removed the engine hands focus back here
    readonly property bool windowActive: Window.active
    onWindowActiveChanged: {
        if (root.windowActive && root.store && root.store.downloading === "")
            root.store.reload();
    }

    Component.onCompleted: {
        var had = !!Changes.stores["dictation"];
        Changes.ensureStore("dictation", Qt.resolvedUrl("../components/voice-typing/DictationStore.qml"));
        if (had)
            Changes.stores["dictation"].reload();
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "voice-typing"
        schema: root.schema
        stateLine: root.stateLine
        bodies: ({ voice: root.loaded ? null : loadingBody })
        footers: ({
            model: root.store && (root.store.notice !== "" || root.store.modelError !== "") ? modelFooter : null,
            microphone: root.store && root.store.mode === "paste" ? pasteNote : null
        })
        controls: root.controls
        onLayoutChanged: {
            if (root.pendingRow !== "") {
                var id = root.pendingRow;
                root.pendingRow = "";
                Qt.callLater(root.focusRow, id);
            }
        }
    }

    readonly property var schema: ({ title: Schema.title, lead: Schema.lead, sections: Schema.sections, rows: Schema.rows, related: Schema.related })

    // each model row the catalogue reports takes the model control; the device
    // and behaviour rows are plain controls the frame draws itself
    readonly property var controls: {
        var c = { "vt-install": installControl, "vt-source": sourceControl, "vt-remove": removeControl };
        if (root.store)
            root.store.models.forEach(m => c["model-" + m.id] = modelControl);
        return c;
    }

    Component {
        id: loadingBody
        Column {
            spacing: LithicTokens.space.s2
            Row {
                visible: !root.store || root.store.readError === ""
                spacing: LithicTokens.space.s2
                Working {
                    anchors.verticalCenter: parent.verticalCenter
                    active: parent.visible
                }
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Reading voice typing.")
                    typeRole: "body"
                    inkRole: "fg2"
                }
            }
            Label {
                visible: root.store !== null && root.store.readError !== ""
                width: parent.width
                text: root.store ? root.store.readError : ""
                typeRole: "body"
                ink: Lithic.jasper
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                visible: root.store !== null && root.store.readError !== ""
                kind: "quiet"
                small: true
                glyph: "refresh"
                text: I18n.tr("Retry")
                onClicked: root.store.reload()
            }
        }
    }

    Component {
        id: installControl
        Plaque {
            property var row: null
            function activate() {
                root.installEngine();
            }
            visible: root.store !== null && !root.store.installed && root.store.packageAvailable
            kind: "gold"
            small: true
            glyph: "download"
            text: I18n.tr("Install")
            onClicked: activate()
        }
    }

    Component {
        id: modelControl
        Row {
            id: ctl
            property var row: null
            readonly property var model: row && root.store ? root.store.modelFor(row.modelId) : null
            readonly property bool busy: model !== null && root.store.downloading === model.id
            readonly property bool removing: model !== null && root.store.removing === model.id
            readonly property bool inUse: model !== null && root.store.activeId() === model.id
            readonly property bool ready: model !== null && (model.bundled === true || model.installed === true)
            function activate() {
                if (ctl.model && !ctl.busy && !ctl.inUse)
                    root.choose(ctl.model.id);
            }
            spacing: LithicTokens.space.s2

            Working {
                anchors.verticalCenter: parent.verticalCenter
                visible: ctl.busy || ctl.removing
                active: visible
            }
            BladeProgress {
                anchors.verticalCenter: parent.verticalCenter
                visible: ctl.busy && (root.store.percent >= 0 || downloadClock.slow)
                width: 96
                active: visible
                indeterminate: root.store.percent < 0
                value: Math.max(0, root.store.percent) / 100
            }
            Label {
                anchors.verticalCenter: parent.verticalCenter
                visible: ctl.busy && root.store.percent >= 0
                text: Math.round(root.store.percent) + "%"
                typeRole: "label"
                inkRole: "fg2"
                numeric: true
            }
            Plaque {
                visible: ctl.busy
                kind: "quiet"
                small: true
                enabled: !root.store.cancelRequested
                text: root.store.cancelRequested ? I18n.tr("Stopping") : I18n.tr("Cancel")
                onClicked: root.store.cancelDownload()
            }
            Label {
                anchors.verticalCenter: parent.verticalCenter
                visible: !ctl.busy && ctl.inUse
                text: I18n.tr("In use")
                typeRole: "label"
            }
            Plaque {
                visible: !ctl.busy && !ctl.inUse
                enabled: root.store.downloading === "" && root.store.removing === ""
                kind: "line"
                small: true
                glyph: ctl.ready ? "" : "download"
                text: ctl.ready ? I18n.tr("Use") : I18n.tr("Download")
                onClicked: ctl.activate()
            }
            IconPlaque {
                visible: !ctl.busy && ctl.model !== null && ctl.model.bundled !== true && ctl.model.installed === true
                enabled: root.store.downloading === "" && root.store.removing === ""
                kind: "quiet"
                small: true
                glyph: "delete"
                accessibleName: ctl.model ? I18n.tr("Remove %1").arg(I18n.tr(ctl.model.label)) : ""
                onClicked: root.removeModel(ctl.model.id)
            }
        }
    }

    // the microphones PipeWire offers are only known at run time, so this row is
    // the page's own control rather than a static picker
    Component {
        id: sourceControl
        Row {
            id: src
            property var row: null
            spacing: LithicTokens.space.s2
            readonly property var names: root.store ? root.store.sourceNames : []
            readonly property int wanted: root.store ? Math.max(0, root.store.sources.findIndex(s => s.name === root.store.source) + 1) : 0
            Picker {
                id: sourceBox
                width: Math.min(LithicTokens.size.settingsControlMax, parent.width)
                options: src.names
                currentIndex: src.wanted
                overlayParent: root.hub ? root.hub.overlayItem : null
                Accessible.name: I18n.tr("Microphone")
                onActivated: index => Changes.set({ store: "dictation", key: "source" },
                                                  root.store.sourceNameAt(index),
                                                  "Microphone", "voice-typing")
            }
        }
    }

    Component {
        id: removeControl
        Plaque {
            property var row: null
            function activate() {
                root.removeEngine();
            }
            visible: root.store !== null && root.store.installed && root.store.packageAvailable
            kind: "dangerLine"
            small: true
            glyph: "delete"
            text: I18n.tr("Remove")
            onClicked: activate()
        }
    }

    Component {
        id: modelFooter
        Row {
            id: modelNote
            spacing: LithicTokens.space.s3
            readonly property string error: root.store.modelError !== "" ? root.store.modelError : root.deviceError
            // the note sits below every model, so a new error is brought into view once it is placed
            onErrorChanged: if (modelNote.error !== "") revealLater.restart()
            Component.onCompleted: if (modelNote.error !== "") revealLater.restart()
            Timer {
                id: revealLater
                interval: 120
                onTriggered: page.reveal(modelNote)
            }
            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, parent.width - (retryModel.visible ? retryModel.width + LithicTokens.space.s3 : 0))
                text: parent.error !== "" ? parent.error : root.store.notice
                typeRole: "detail"
                ink: parent.error !== "" ? Lithic.jasper : Lithic.fg2
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                id: retryModel
                visible: parent.error !== ""
                kind: "quiet"
                small: true
                glyph: "refresh"
                text: I18n.tr("Retry")
                onClicked: {
                    var id = root.store.failedDownload !== "" ? root.store.failedDownload : root.store.failedRemove;
                    if (root.store.modelError !== "" && root.store.failedDownload !== "")
                        root.choose(root.store.failedDownload);
                    else if (root.store.modelError !== "" && root.store.failedRemove !== "")
                        root.store.removeModel(root.store.failedRemove);
                    else if (root.deviceError !== "")
                        Changes.retry(root.deviceBind);
                    else
                        root.store.reload();
                    if (id !== "" && page.rowItems["model-" + id])
                        page.reveal(page.rowItems["model-" + id]);
                }
            }
        }
    }

    Component {
        id: pasteNote
        Label {
            text: I18n.tr("Paste puts the words on the clipboard and pastes them, so the app cannot drop a character. Your clipboard is restored afterwards.")
            typeRole: "detail"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }

    BusyClock {
        id: downloadClock
        running: root.store !== null && root.store.downloading !== ""
    }
}
