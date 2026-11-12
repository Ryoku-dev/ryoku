import QtQuick
import QtQuick.Window
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/screen-capture"
import "../schema/voice-typing.js" as Schema
import "../components/shortcuts/binds.js" as Binds

// voice typing: on or off, the speech model, the online model's key, and removal
Item {
    id: root

    required property var hub
    property string pendingRow: ""

    readonly property var store: Changes.stores["voxtype"] || null
    readonly property bool loaded: root.store !== null && root.store.loaded
    readonly property bool installed: root.loaded && root.store.installed
    readonly property var current: root.store ? root.store.current : null
    readonly property bool usable: root.store !== null && root.store.usableWith(root.store.selected)
    readonly property var presetBind: ({ store: "voxtype", key: "preset" })
    readonly property string presetError: Changes.error(root.presetBind)

    // the voice typing shortcut as it is now: its shipped chord, rebound or turned off on Keyboard shortcuts
    readonly property string voiceDef: "SUPER + grave"
    readonly property bool keyKnown: Changes.desktopReady
    readonly property bool keyOff: root.keyKnown && Binds.isOff(Changes.value({ store: "desktop", key: "keybindDisabled" }), root.voiceDef)
    readonly property string keyText: root.keyKnown
        ? Binds.shown(Binds.effective(Changes.value({ store: "desktop", key: "keybindRebinds" }), root.voiceDef), { "grave": "`" })
        : ""

    function presetDetail(p) {
        var parts = [I18n.tr(p.language), I18n.tr(p.speed)];
        if (!p.cloud)
            parts.push(p.size);
        var busy = root.store.downloading === p.key;
        if (busy) {
            var words = {
                "fetch": I18n.tr("Fetching the source"), "configure": I18n.tr("Preparing the build"),
                "build": I18n.tr("Building the speech engine"), "download": I18n.tr("Downloading"),
                "install": I18n.tr("Installing")
            };
            parts.push(words[root.store.stage] || I18n.tr("Downloading"));
        } else if (root.store.removing === p.key) {
            parts.push(I18n.tr("Removing"));
        } else if (p.cloud) {
            parts.push(I18n.tr("Online"));
        } else {
            parts.push(p.present ? I18n.tr("Downloaded") : I18n.tr("Not downloaded"));
        }
        return parts.join(" · ") + "\n" + I18n.tr(p.detail);
    }

    // each model row chooses through its own binding ("preset@<key>"), so a failed choice shows
    // its error and Retry under that row, where the pointer is
    function choiceBind(key) {
        return { store: "voxtype", key: "preset@" + key };
    }

    function presetRow(p) {
        return {
            id: "model-" + p.key, section: "model", label: p.label, presetKey: p.key, bind: root.choiceBind(p.key),
            detail: root.presetDetail(p), control: "custom", activatable: true
        };
    }

    function prepared(row) {
        var out = Object.assign({}, row);
        if (row.id === "vt-enabled")
            out.detail = root.keyOff ? I18n.tr("Its shortcut is turned off. Give it keys under Voice typing shortcut to type with your voice")
                : root.keyText !== "" ? I18n.tr("Press %1 to type with your voice in the app in front; press it again to stop").arg(root.keyText)
                : row.detail;
        if (row.id === "vt-enabled" && !root.store.enabled && !root.usable)
            out.unavailable = root.current && root.current.cloud ? I18n.tr("Add your OpenAI key below first") : I18n.tr("Download a model below first");
        if (row.id === "vt-install" && !root.store.packageAvailable)
            out.detail = root.store.packageReason !== "" ? root.store.packageReason : I18n.tr("Voice typing is not available for this system.");
        if (row.id === "vt-key" && root.store.keySet)
            out.detail = I18n.tr("A key is saved. Paste a new one to replace it");
        if (row.id === "vt-remove" && !root.store.packageAvailable)
            out.detail = root.store.packageReason !== "" ? root.store.packageReason : I18n.tr("This system cannot remove Voxtype from Settings.");
        return out;
    }

    function shown(row) {
        if (!root.loaded)
            return false;
        if (row.id === "vt-install")
            return !root.installed;
        if (!root.installed)
            return false;
        if (row.id === "vt-key")
            return root.current !== null && root.current.keyKind !== "";
        return true;
    }

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: Schema.rows.filter(r => r.template !== true && root.shown(r)).map(r => root.prepared(r))
            .concat(root.installed ? root.store.presets.map(p => root.presetRow(p)) : [])
    })

    readonly property string stateLine: !root.loaded ? ""
        : !root.installed ? I18n.tr("Voxtype is not installed.")
        : root.store.enabled && root.current ? I18n.tr("On, listening with %1.").arg(I18n.tr(root.current.label))
        : I18n.tr("Off.")

    function choose(key) {
        var p = root.store.presetFor(key);
        if (!p || root.store.downloading !== "" || root.store.removing !== "")
            return;
        if (p.cloud || p.present) {
            Changes.set(root.choiceBind(key), key, "Speech model", "voice-typing");
            return;
        }
        if (Number(p.sizeBytes || 0) > 1000000000) {
            root.hub.confirm({
                title: I18n.tr("Download %1?").arg(I18n.tr(p.label)),
                text: I18n.tr("This model is a %1 download. Voice typing keeps using the model it has until it finishes.").arg(p.size),
                actionText: I18n.tr("Download"),
                destructive: false
            }, function () { root.store.download(key); });
            return;
        }
        root.store.download(key);
    }

    function removeModel(key) {
        var p = root.store.presetFor(key);
        if (!p)
            return;
        var inUse = root.store.selected === key;
        root.hub.confirm({
            title: I18n.tr("Remove %1?").arg(I18n.tr(p.label)),
            text: inUse ? I18n.tr("The model is deleted from this computer. Voice typing stops working until you download it again or choose another model.")
                        : I18n.tr("The model is deleted from this computer. You can download it again."),
            actionText: I18n.tr("Remove")
        }, function () { root.store.removeModel(key); });
    }

    function removeEngine() {
        root.hub.confirm({
            title: I18n.tr("Remove Voxtype?"),
            text: I18n.tr("Voice typing stops and the Voxtype package is uninstalled in a terminal, which asks for your password. Your model choice and downloaded models stay."),
            actionText: I18n.tr("Remove")
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
        var had = !!Changes.stores["voxtype"];
        Changes.ensureStore("voxtype", Qt.resolvedUrl("../components/voice-typing/VoxtypeStore.qml"));
        if (had)
            Changes.stores["voxtype"].reload();
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
            model: root.store && (root.store.notice !== "" || root.store.modelError !== "" || root.presetError !== "") ? modelFooter : null,
            key: root.store && root.store.keyError !== "" ? keyFooter : null
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

    // each model row the store reports takes the model control
    readonly property var controls: {
        var c = { "vt-install": installControl, "vt-key": keyControl, "vt-remove": removeControl };
        if (root.store)
            root.store.presets.forEach(p => c["model-" + p.key] = modelControl);
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
                root.store.installEngine();
            }
            visible: root.store !== null && root.store.packageAvailable
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
            readonly property var preset: row && root.store ? root.store.presetFor(row.presetKey) : null
            readonly property bool busy: preset !== null && root.store.downloading === preset.key
            readonly property bool removing: preset !== null && root.store.removing === preset.key
            readonly property bool inUse: preset !== null && root.store.selected === preset.key
            readonly property bool ready: preset !== null && (preset.cloud || preset.present)
            function activate() {
                if (ctl.preset && !ctl.busy && !ctl.inUse)
                    root.choose(ctl.preset.key);
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
                visible: !ctl.busy && ctl.preset !== null && !ctl.preset.cloud && ctl.preset.present
                enabled: root.store.downloading === "" && root.store.removing === ""
                kind: "quiet"
                small: true
                glyph: "delete"
                accessibleName: ctl.preset ? I18n.tr("Remove %1").arg(I18n.tr(ctl.preset.label)) : ""
                onClicked: root.removeModel(ctl.preset.key)
            }
        }
    }

    Component {
        id: keyControl
        Row {
            property var row: null
            spacing: LithicTokens.space.s2
            Working {
                anchors.verticalCenter: parent.verticalCenter
                visible: root.store.keySaving
                active: visible
            }
            TextField {
                id: keyField
                width: LithicTokens.size.settingsControlMax - (root.store.keySaving ? LithicTokens.space.s6 : 0)
                secret: true
                mono: true
                enabled: !root.store.keySaving
                placeholderText: root.store.keySet ? I18n.tr("Paste a new key") : I18n.tr("Paste your OpenAI key")
                Accessible.name: I18n.tr("OpenAI key")
                onEditingFinished: {
                    if (keyField.text.trim().length > 0) {
                        root.store.saveKey(keyField.text);
                        keyField.text = "";
                    }
                }
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
            visible: root.store !== null && root.store.packageAvailable
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
            readonly property string error: root.store.modelError !== "" ? root.store.modelError : root.presetError
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
                    // the work shows on the model's own row, so that row comes into view
                    var key = root.store.failedDownload !== "" ? root.store.failedDownload : root.store.failedRemove;
                    if (root.store.modelError !== "" && root.store.failedDownload !== "")
                        root.choose(root.store.failedDownload);
                    else if (root.store.modelError !== "" && root.store.failedRemove !== "")
                        root.store.removeModel(root.store.failedRemove);
                    else if (root.store.modelError !== "")
                        root.store.reload();
                    else
                        Changes.retry(root.presetBind);
                    if (key !== "" && page.rowItems["model-" + key])
                        page.reveal(page.rowItems["model-" + key]);
                }
            }
        }
    }

    Component {
        id: keyFooter
        Label {
            text: I18n.tr("The key was not saved: %1. Paste it again to try once more.").arg(root.store.keyError.replace(/\.+$/, ""))
            typeRole: "detail"
            ink: Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }

    BusyClock {
        id: downloadClock
        running: root.store !== null && root.store.downloading !== ""
    }
}
