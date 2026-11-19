import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/screen-capture"
import "../components/rashin"
import "../schema/rashin.js" as Schema
import "../components/rashin/sight.js" as Sight

Item {
    id: root

    required property var hub
    property string pendingRow: ""

    readonly property var store: Changes.stores["rashin"] || null
    readonly property var sightStore: Changes.stores["sight"] || null
    readonly property var featuresStore: Changes.stores["rashinFeatures"] || null
    readonly property bool featuresReady: root.featuresStore !== null && root.featuresStore.loaded && root.featuresStore.error === ""
    readonly property bool theiaOn: Changes.value({ store: "sight", key: "on", fallback: true }) === true
    readonly property bool loaded: root.store !== null && root.store.loaded
    readonly property string state: root.loaded ? root.store.state : ""
    readonly property bool switching: root.store !== null && root.store.switching !== ""

    function routeText() {
        var r = root.store.askRoute;
        if (r === "auto")
            return I18n.tr("Picks a local model first for each question");
        if (r === "auto:fast")
            return I18n.tr("Picks the fastest available model, local first");
        if (r === "auto:smart")
            return I18n.tr("Picks the strongest available model, local first");
        return I18n.tr("Uses %1").arg(r);
    }

    function enabledDetail() {
        var s = root.store;
        if (s.switching === "on")
            return s.installed ? I18n.tr("Turning Rashin on")
                               : I18n.tr("Installing Rashin. Enter your password when asked; this can take a minute");
        if (s.switching === "off")
            return I18n.tr("Turning Rashin off and removing it");
        switch (root.state) {
        case "absent": return I18n.tr("Not installed. Turning it on installs Rashin and asks for your password");
        case "off": return I18n.tr("Off. Rashin runs nothing at all until you turn it on");
        case "idle": return I18n.tr("On. It starts when you ask something and stops again when idle");
        case "running": return I18n.tr("On and running now; it stops by itself when idle");
        }
        return "";
    }

    function gestureOptions(key) {
        var gestures = [
            { key: "tapKey", fallback: "Alt_R" },
            { key: "highlightKey", fallback: "Alt_L" },
            { key: "inplace.key", fallback: "Control_R" }
        ];
        var held = gestures.filter(g => g.key !== key).map(g => Changes.value({ store: "sight", key: g.key, fallback: g.fallback }));
        return root.sightStore.gestureKeys.filter(k => held.indexOf(k.Name) < 0)
            .map(k => ({ label: k.Label, value: k.Name }));
    }

    function prepared(row) {
        var out = Object.assign({}, row);
        var s = root.store;
        if (row.bind && row.bind.store === "sight" && row.control === "picker"
                && ["tapKey", "highlightKey", "inplace.key"].indexOf(row.bind.key) >= 0)
            out.options = root.gestureOptions(row.bind.key);
        if (row.feature === "save" && !root.featuresReady)
            out.unavailable = I18n.tr("Rashin's feature settings are not loaded");
        switch (row.id) {
        case "sight-enabled":
            out.label = I18n.tr("Tap %1 three times").arg(root.sightStore.tapLabel);
            break;
        case "sight-highlight":
            out.label = I18n.tr("Hold %1 while highlighting").arg(root.sightStore.highlightLabel);
            break;
        case "sight-target":
            out.options = Sight.targetOptions(I18n.langs, text => I18n.tr(text));
            break;
        case "sight-shortcut":
            out.detail = I18n.tr("Theia Look: %1 three times").arg(root.sightStore.tapLabel) + " · "
                + I18n.tr("Highlight: hold %1").arg(root.sightStore.highlightLabel) + " · "
                + I18n.tr("In place: %1 three times").arg(root.sightStore.keyLabel("inplace.key", "Control_R", "Right Ctrl"));
            break;
        case "sight-status":
            out.detail = root.sightStore.statusLine;
            break;
        case "rashin-enabled":
            out.detail = root.enabledDetail();
            break;
        case "rashin-app-features":
            out.detail = root.featuresReady
                ? I18n.tr("%1 of %2 on").arg(root.featuresStore.enabledCount).arg(Sight.rashinFeatureIDs.length)
                : I18n.tr("Rashin's feature settings are not loaded");
            break;
        case "status-ask":
            out.detail = s.running ? I18n.tr("%1. %2 providers ready").arg(root.routeText()).arg(s.providers) : root.routeText();
            break;
        case "status-agent":
            out.detail = I18n.tr("New Agent sessions use %1").arg(s.agentModel);
            break;
        case "status-index":
            out.detail = s.running ? I18n.tr("%1 indexes of your system and projects").arg(s.indexes)
                                   : I18n.tr("Your system and projects; the counts show while Rashin runs");
            break;
        }
        return out;
    }

    function shown(row) {
        if (!root.loaded)
            return false;
        if (row.id === "rashin-app-features")
            return root.store.enabled && !root.switching;
        if (row.section.indexOf("theia") === 0) {
            if (!root.sightStore || !root.store.enabled || root.switching)
                return false;
            if (row.id.indexOf("theia-o-") === 0 || row.id.indexOf("sight-") === 0)
                return root.theiaOn;
            return true;
        }
        if (row.section === "status")
            return root.store.installed;
        if (row.section === "open")
            return root.store.installed && root.store.enabled && !root.switching;
        return true;
    }

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: Schema.rows.filter(r => root.shown(r)).map(r => root.prepared(r))
    })

    readonly property string stateLine: root.loaded && root.store.installed && root.store.version !== ""
        ? I18n.tr("Rashin %1").arg(root.store.version) : ""

    function request(on) {
        if (!root.visible || !root.store || root.switching)
            return;
        if (on) {
            root.store.turn(true);
            return;
        }
        root.hub.confirm({
            title: I18n.tr("Turn Rashin off?"),
            text: I18n.tr("Rashin stops, its app is removed and nothing of it runs until you turn it on again. Your chats and settings stay."),
            actionText: I18n.tr("Turn off")
        }, function () {
            if (root.visible && root.store && !root.switching)
                root.store.turn(false);
        });
    }

    function sectionIndex(id) {
        for (var i = 0; i < page.layout.length; i++)
            if (page.layout[i].id === id)
                return i;
        return -1;
    }

    function focusRow(rowId) {
        if (!rowId || !root.visible)
            return;
        if (rowId === "sight")
            rowId = "theia";
        if (page.layout.length === 0 || !root.loaded) {
            root.pendingRow = rowId;
            return;
        }
        // a drawn row or section is the frame's to land on
        if (page.landingRow(rowId) !== "" || page.layout.some(s => s.id === rowId)) {
            page.focusRow(rowId);
            return;
        }
        var theiaRow = rowId.indexOf("theia") === 0 || rowId.indexOf("sight-") === 0;
        page.focusRow(root.loaded && root.store.enabled && theiaRow ? "theia-on" : "rashin-enabled");
    }

    function editorOpen(item) {
        if (item.editing === true || item.busy === true)
            return true;
        var children = item.children || [];
        for (var i = 0; i < children.length; i++)
            if (root.editorOpen(children[i]))
                return true;
        return false;
    }

    function refreshEditors(item) {
        if (item.store === root.sightStore && typeof item.refresh === "function")
            item.refresh();
        var children = item.children || [];
        for (var i = 0; i < children.length; i++)
            root.refreshEditors(children[i]);
    }

    function revisited() {
        if (!root.visible || root.switching)
            return;
        if (root.store)
            root.store.refresh();
        if (!root.editorOpen(page) && !(root.sightStore && root.sightStore.job.running)
                && !(root.featuresStore && root.featuresStore.writing > 0)) {
            if (root.sightStore)
                root.sightStore.refresh();
            if (root.featuresStore)
                root.featuresStore.refresh();
            root.refreshEditors(page);
        }
        if (root.pendingRow !== "")
            Qt.callLater(root.focusRow, root.pendingRow);
    }

    function customControls() {
        var controls = {
            "rashin-enabled": switchControl,
            "theia-on": theiaHeroControl,
            "sight-languages": sightLanguagesControl,
            "sight-model": sightModelControl,
            "sight-shortcut": sightShortcutControl,
            "theia-o-bubble": theiaBubbleControl,
            "theia-o-pinned": theiaPinnedControl,
            "theia-o-deepread.lenses": theiaLensesControl,
            "theia-o-tone.favorites": theiaTonesControl,
            "theia-o-proofread.off": theiaCategoriesControl,
            "theia-o-voice": theiaVoiceControl,
            "theia-o-quiet.apps": theiaQuietControl,
            "theia-o-readaloud.voice": theiaVoicesControl,
            "theia-o-proofread.words": theiaWordsControl
        };
        Schema.rows.forEach(function (row) {
            if (row.id.indexOf("theia-f-") === 0)
                controls[row.id] = theiaFeatureControl;
        });
        return controls;
    }

    Component.onCompleted: {
        Changes.ensureStore("sight", Qt.resolvedUrl("../components/rashin/SightStore.qml"));
        Changes.ensureStore("rashinFeatures", Qt.resolvedUrl("../components/rashin/RashinFeaturesStore.qml"));
        var had = !!Changes.stores["rashin"];
        Changes.ensureStore("rashin", Qt.resolvedUrl("../components/rashin/RashinStore.qml"));
        if (had)
            root.revisited();
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "rashin"
        asynchronousSections: true
        schema: root.schema
        stateLine: root.stateLine
        bodies: ({ rashin: root.loaded ? null : loadingBody })
        // a switch that failed says so first; a status re-read that failed says the page may be out of date
        footers: ({ rashin: root.store && root.store.error !== "" ? errorFooter
                          : root.loaded && root.store.readError !== "" ? readFooter
                          : root.featuresStore && root.featuresStore.active && root.featuresStore.error !== "" ? featuresStatusBody : null })
        controls: root.customControls()
        onLayoutChanged: {
            if (root.visible && root.pendingRow !== "") {
                var id = root.pendingRow;
                root.pendingRow = "";
                Qt.callLater(root.focusRow, id);
            }
        }
    }

    Binding {
        target: root.featuresStore
        property: "active"
        value: root.visible && root.loaded && root.store.enabled && !root.switching
        when: root.featuresStore !== null
    }

    Binding {
        target: root.sightStore
        property: "active"
        value: root.visible && root.loaded && root.store.enabled && !root.switching
        when: root.sightStore !== null
    }

    Component {
        id: sightLanguagesControl
        SightLanguages { store: root.sightStore; enabled: root.visible }
    }
    Component {
        id: sightModelControl
        SightModel { store: root.sightStore; enabled: root.visible }
    }
    Component {
        id: sightShortcutControl
        SightShortcut { hub: root.hub; enabled: root.visible }
    }

    Component {
        id: featuresStatusBody
        Row {
            width: parent ? parent.width : implicitWidth
            spacing: LithicTokens.space.s3
            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.max(0, parent.width - featuresRetry.width - parent.spacing)
                text: I18n.tr("Rashin's feature settings could not be read. Try again.")
                typeRole: "detail"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                id: featuresRetry
                anchors.verticalCenter: parent.verticalCenter
                enabled: root.visible && root.featuresStore !== null
                text: I18n.tr("Retry")
                glyph: "refresh"
                kind: "quiet"
                small: true
                onClicked: {
                    if (root.visible && root.featuresStore)
                        root.featuresStore.retry();
                }
            }
        }
    }

    Component {
        id: theiaHeroControl
        TheiaHero { store: root.sightStore; featureRows: Schema.rows.filter(r => r.id.indexOf("theia-f-") === 0); enabled: root.visible }
    }
    Component {
        id: theiaFeatureControl
        TheiaFeatureSwitch {
            available: !row || row.feature !== "save" || root.featuresReady
            enabled: root.visible && available
        }
    }
    Component {
        id: theiaBubbleControl
        TheiaBubblePicker { store: root.sightStore; enabled: root.visible }
    }
    Component {
        id: theiaPinnedControl
        TheiaPinnedPicker { store: root.sightStore; enabled: root.visible }
    }
    Component {
        id: theiaLensesControl
        TheiaBubblePicker {
            store: root.sightStore
            enabled: root.visible
            limit: 7
            ordered: false
            choices: [
                { id: "factcheck", label: I18n.tr("Fact check") },
                { id: "assumptions", label: I18n.tr("Assumptions") },
                { id: "concepts", label: I18n.tr("Key ideas") },
                { id: "contrasts", label: I18n.tr("Contrasts") },
                { id: "related", label: I18n.tr("Related reading") },
                { id: "passages", label: I18n.tr("Passages") },
                { id: "section", label: I18n.tr("Section") }
            ]
        }
    }
    Component {
        id: theiaTonesControl
        TheiaTones { store: root.sightStore; enabled: root.visible }
    }
    Component {
        id: theiaCategoriesControl
        TheiaCategories { store: root.sightStore; enabled: root.visible }
    }
    Component {
        id: theiaVoiceControl
        TheiaVoice { store: root.sightStore; enabled: root.visible }
    }
    Component {
        id: theiaQuietControl
        TheiaQuietApps { store: root.sightStore; enabled: root.visible }
    }
    Component {
        id: theiaVoicesControl
        TheiaVoices { store: root.sightStore; enabled: root.visible }
    }
    Component {
        id: theiaWordsControl
        TheiaWords { store: root.sightStore; enabled: root.visible }
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
                    text: I18n.tr("Reading Rashin's status.")
                    typeRole: "body"
                    inkRole: "fg2"
                }
            }
            Label {
                visible: root.store !== null && root.store.readError !== ""
                width: parent.width
                text: root.store ? root.store.readError + " " + I18n.tr("Check that the rashin command works in a terminal, then try again.") : ""
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
                onClicked: root.store.refresh()
            }
        }
    }

    Component {
        id: switchControl
        Row {
            id: ctl
            property var row: null
            function activate() {
                if (root.store && !root.switching)
                    root.request(!root.store.enabled);
            }
            spacing: LithicTokens.space.s2
            Working {
                anchors.verticalCenter: parent.verticalCenter
                visible: root.switching && switchClock.working
                active: visible
            }
            BladeProgress {
                anchors.verticalCenter: parent.verticalCenter
                visible: root.switching && switchClock.slow
                width: 96
                active: visible
                indeterminate: true
            }
            Plaque {
                anchors.verticalCenter: parent.verticalCenter
                visible: root.switching && switchClock.slow
                enabled: !root.store.cancelRequested
                kind: "quiet"
                small: true
                text: root.store && root.store.cancelRequested ? I18n.tr("Stopping") : I18n.tr("Cancel")
                onClicked: root.store.cancel()
            }
            Toggle {
                id: toggle
                anchors.verticalCenter: parent.verticalCenter
                enabled: !root.switching
                checked: root.switching ? root.store.switching === "on" : (root.store !== null && root.store.enabled)
                Accessible.name: I18n.tr("Use Rashin")
                onToggled: {
                    var want = toggle.checked;
                    toggle.checked = Qt.binding(() => root.switching ? root.store.switching === "on" : (root.store !== null && root.store.enabled));
                    root.request(want);
                }
            }
        }
    }

    Component {
        id: errorFooter
        Row {
            spacing: LithicTokens.space.s3
            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, parent.width - retrySwitch.width - LithicTokens.space.s3)
                text: root.store.message
                typeRole: "detail"
                ink: Lithic.jasper
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                id: retrySwitch
                kind: "quiet"
                small: true
                glyph: "refresh"
                text: I18n.tr("Retry")
                onClicked: root.store.turn(root.store.lastAsk === "on")
            }
        }
    }

    Component {
        id: readFooter
        Row {
            spacing: LithicTokens.space.s3
            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, parent.width - retryRead.width - LithicTokens.space.s3)
                text: root.store.readError + " " + I18n.tr("What this page shows may be out of date. Check that the rashin command works in a terminal, then try again.")
                typeRole: "detail"
                ink: Lithic.jasper
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                id: retryRead
                kind: "quiet"
                small: true
                glyph: "refresh"
                text: I18n.tr("Retry")
                onClicked: root.store.refresh()
            }
        }
    }

    BusyClock {
        id: switchClock
        running: root.visible && root.switching
    }
}
