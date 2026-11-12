import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/app-rules"
import "../components/app-rules/rules.js" as Rules
import "../schema/app-rules.js" as Schema

// rules for particular apps, and for the desktop's own layers under More options
Item {
    id: root

    required property var hub
    readonly property string pageId: "app-rules"
    readonly property var storeBind: ({ store: "app-rules", key: "rules" })
    readonly property var rulesBind: ({ store: "desktop", key: "windowRules" })
    readonly property var looksBind: ({ store: "desktop", key: "appOverrides" })

    // models change only when what they show changes, so an open editor survives
    // unrelated ticks of the change service
    property var listed: Rules.vocabulary([], [])
    property var groups: []
    property var ruleModel: []
    property var spec: null
    property var layerModel: []
    property string signature: ""
    property bool reloadAfterSave: false
    property string pendingRow: ""

    property Item bodyItem: null
    property Item rulesEditor: null
    property Item moreFold: null
    property Item layersItem: null

    readonly property var layerBind: root.spec ? { store: "desktop", key: root.spec.key } : null
    readonly property bool layersShown: root.spec !== null
    // rule names come from the installed apps, which load after the page
    readonly property int knownApps: DesktopEntries.applications ? DesktopEntries.applications.values.length : 0
    onKnownAppsChanged: root.refresh()

    function appName(cls) {
        if (cls === "")
            return "";
        var e = DesktopEntries.heuristicLookup(cls);
        return e && e.name ? e.name : cls;
    }

    // mid says the name inside a sentence ("Rule for windows titled ...")
    function ruleName(item, mid) {
        var name = root.appName(String(item.cls || ""));
        var t = String(item.title || "");
        if (name !== "" && t !== "")
            return I18n.tr("%1, windows titled %2").arg(name).arg(t);
        if (t !== "")
            return mid ? I18n.tr("windows titled %1").arg(t) : I18n.tr("Windows titled %1").arg(t);
        return name !== "" ? name : mid ? I18n.tr("no app chosen") : I18n.tr("No app chosen");
    }

    function reasonsFor(index) {
        var out = [];
        var u = ProviderSchema.unhonored || [];
        for (var i = 0; i < u.length; i++)
            if (Rules.ruleOfStoreKey(root.groups, u[i].key) === index && u[i].reason && out.indexOf(u[i].reason) < 0)
                out.push(u[i].reason);
        return out;
    }

    function refresh() {
        if (!Changes.desktopReady)
            return;
        var listed = Rules.vocabulary(Settings.windowRuleActions, Settings.appOverrideFields);
        var wr = Changes.value(root.rulesBind) || [];
        var ao = Changes.value(root.looksBind) || [];
        var spec = Settings.supports("layerRules") ? Rules.layerSpec(ProviderSchema.rowsFor("layerrules")) : null;
        if (spec && Changes.value({ store: "desktop", key: spec.key }) === undefined)
            spec = null;
        var layers = spec ? (Changes.value({ store: "desktop", key: spec.key }) || []) : [];
        var sig = JSON.stringify([listed, wr, ao, spec, layers, ProviderSchema.unhonored, I18n.tr("Rules"), root.knownApps]);
        if (sig === root.signature)
            return;
        root.signature = sig;
        var groups = Rules.grouped(wr, ao);
        var unused = I18n.tr("%1, not used by this window manager");
        root.listed = listed;
        root.groups = groups;
        root.ruleModel = groups.map((g, i) => {
            var item = Rules.itemOf(g, wr, ao, listed);
            var reasons = root.reasonsFor(i);
            var line = Rules.joined(Rules.phrases(item, listed, s => I18n.tr(s), reasons.length > 0 ? "%1" : unused));
            item.name = root.ruleName(item);
            item.line = reasons.length > 0 ? (line.length > 0 ? line + ". " : "") + reasons.join(" ") : line;
            return item;
        });
        root.spec = spec;
        root.layerModel = spec ? Rules.layerItems(spec, layers).map(item => {
            item.line = Rules.joined(Rules.layerPhrases(spec, item, s => I18n.tr(s), I18n.tr("%1: %2"), unused));
            return item;
        }) : [];
    }

    Component.onCompleted: {
        Changes.ensureStore("app-rules", Qt.resolvedUrl("../components/app-rules/RuleStore.qml"));
        root.refresh();
    }
    Connections {
        target: Changes
        function onRevisionChanged() { root.refresh(); }
        function onUnsettledChanged() {
            if (!Changes.unsettled && root.reloadAfterSave) {
                root.reloadAfterSave = false;
                ProviderSchema.reload();
            }
        }
    }
    Connections {
        target: Settings
        function onRevisionChanged() { root.refresh(); }
    }
    Connections {
        target: ProviderSchema
        function onRevisionChanged() { root.refresh(); }
    }

    function stores() {
        return { windowRules: Changes.value(root.rulesBind) || [], appOverrides: Changes.value(root.looksBind) || [] };
    }

    function commitRules(next, name, notice) {
        root.reloadAfterSave = true;
        Changes.set(root.storeBind, Rules.tagged(next.windowRules, next.appOverrides), I18n.tr("Rule for %1").arg(name),
                    root.pageId, notice);
    }

    function addRule(item) {
        var s = root.stores();
        var name = root.ruleName(item, true);
        root.commitRules(Rules.withRule(s.windowRules, s.appOverrides, -1, item, root.listed), name,
                         I18n.tr("Rule for %1 added").arg(name));
    }

    function editRule(index, item) {
        var s = root.stores();
        var name = root.ruleName(item, true);
        root.commitRules(Rules.withRule(s.windowRules, s.appOverrides, index, item, root.listed), name,
                         I18n.tr("Rule for %1 changed").arg(name));
    }

    function removeRule(index) {
        var s = root.stores();
        var item = root.ruleModel[index];
        if (!item)
            return;
        var name = root.ruleName(item, true);
        root.commitRules(Rules.withoutRule(s.windowRules, s.appOverrides, index), name,
                         I18n.tr("Rule for %1 removed").arg(name));
    }

    function setLayer(index, item, notice) {
        if (!root.spec)
            return;
        var records = Changes.value(root.layerBind) || [];
        var ns = item ? item.namespace : (root.layerModel[index] ? root.layerModel[index].namespace : "");
        root.reloadAfterSave = true;
        Changes.set(root.layerBind, Rules.withLayer(root.spec, records, index, item),
                    I18n.tr("Layer rule for %1").arg(ns), root.pageId, notice.arg(ns));
    }

    // every rule at once; one undo brings them all back
    function removeAllRules() {
        var n = root.ruleModel.length;
        root.hub.confirm({
            title: I18n.tr("Remove all app rules?"),
            text: I18n.tr("All %1 rules go. Undo brings them back.").arg(n),
            actionText: I18n.tr("Remove all")
        }, () => {
            root.reloadAfterSave = true;
            Changes.set(root.storeBind, [], I18n.tr("All app rules removed"), root.pageId, I18n.tr("All app rules removed"));
        });
    }

    function removeAllLayers() {
        var n = root.layerModel.length;
        root.hub.confirm({
            title: I18n.tr("Remove all layer rules?"),
            text: I18n.tr("All %1 layer rules go. Undo brings them back.").arg(n),
            actionText: I18n.tr("Remove all")
        }, () => {
            root.reloadAfterSave = true;
            Changes.set(root.layerBind, [], I18n.tr("All layer rules removed"), root.pageId, I18n.tr("All layer rules removed"));
        });
    }

    // search lands on the rule list, or opens More options for shell layers; the layers
    // section is drawn inside More options, so a link to it lands like its rows
    function focusRow(rowId) {
        if (!rowId)
            return;
        if (!root.bodyItem) {
            root.pendingRow = rowId;
            return;
        }
        var row = Schema.rows.find(r => r.id === rowId)
            || (rowId === "layers" ? Schema.rows.find(r => r.section === "layers") : undefined);
        if (!row) {
            page.focusRow(rowId);
            return;
        }
        if (row.more === true && root.moreFold && root.layersShown) {
            if (!root.moreFold.open) {
                root.moreFold.open = true;
                unfold.restart();
            } else {
                page.reveal(root.layersItem);
            }
            return;
        }
        page.reveal(root.rulesEditor);
    }

    Timer {
        id: unfold
        interval: LithicTokens.motion.unfold + 20
        onTriggered: page.reveal(root.layersItem)
    }

    readonly property bool someUnhonored: root.ruleModel.some((item, i) => root.reasonsFor(i).length > 0)

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: root.pageId
        schema: Schema
        stateLine: root.someUnhonored ? I18n.tr("Some rules ask for things this window manager cannot do. Each rule says which.") : ""
        bodies: ({ rules: rulesBody })
    }

    Component {
        id: rulesBody

        Column {
            id: body
            spacing: LithicTokens.space.s2
            Component.onCompleted: {
                root.bodyItem = body;
                if (root.pendingRow !== "") {
                    var id = root.pendingRow;
                    root.pendingRow = "";
                    Qt.callLater(root.focusRow, id);
                }
            }
            Component.onDestruction: {
                if (root.bodyItem === body)
                    root.bodyItem = null;
            }

            Slab {
                width: parent.width

                Column {
                    width: parent.width

                    ListEditor {
                        id: rulesList
                        width: parent.width
                        model: root.ruleModel
                        titleOf: item => item.name
                        detailOf: item => item.line
                        addText: I18n.tr("Add a rule")
                        emptyText: I18n.tr("No rules yet. Add one to make an app open, look or behave its own way.")
                        accessibleName: I18n.tr("App rules")
                        editor: ruleEditor
                        onAdded: item => root.addRule(item)
                        onEdited: (index, item) => root.editRule(index, item)
                        onRemoved: index => root.removeRule(index)
                        Component.onCompleted: root.rulesEditor = rulesList
                    }

                    SaveError {
                        id: rulesError
                        bind: root.storeBind
                        onShown: page.reveal(rulesError)
                    }

                    Item {
                        visible: root.ruleModel.length >= 2
                        width: parent.width
                        height: clearRules.height
                        Plaque {
                            id: clearRules
                            anchors.right: parent.right
                            kind: "quiet"
                            small: true
                            glyph: "delete_sweep"
                            text: I18n.tr("Remove all rules")
                            onClicked: root.removeAllRules()
                        }
                    }

                    Item {
                        visible: root.layersShown
                        width: 1
                        height: LithicTokens.size.settingsRowGap
                    }

                    Disclosure {
                        id: fold
                        visible: root.layersShown
                        width: parent.width
                        text: I18n.tr("More options")
                        Component.onCompleted: root.moreFold = fold

                        Column {
                            id: layers
                            width: parent ? parent.width : 0
                            spacing: LithicTokens.space.s1
                            Component.onCompleted: root.layersItem = layers

                            Label {
                                width: parent.width
                                topPadding: LithicTokens.space.s2
                                text: I18n.tr("Shell layers")
                                typeRole: "row"
                            }
                            Label {
                                width: parent.width
                                bottomPadding: LithicTokens.space.s2
                                text: I18n.tr("Rules for the bar, launcher, notifications and other parts of the desktop, by the name of their layer")
                                typeRole: "detail"
                                wrapMode: Text.Wrap
                                elide: Text.ElideNone
                            }
                            ListEditor {
                                width: parent.width
                                model: root.layerModel
                                titleOf: item => item.namespace !== "" ? item.namespace : I18n.tr("No layer named")
                                detailOf: item => item.line
                                addText: I18n.tr("Add a layer rule")
                                emptyText: I18n.tr("No layer rules yet.")
                                accessibleName: I18n.tr("Shell layers")
                                editor: layerEditor
                                onAdded: item => root.setLayer(-1, item, I18n.tr("Layer rule for %1 added"))
                                onEdited: (index, item) => root.setLayer(index, item, I18n.tr("Layer rule for %1 changed"))
                                onRemoved: index => root.setLayer(index, null, I18n.tr("Layer rule for %1 removed"))
                            }
                            SaveError {
                                id: layersError
                                bind: root.layerBind
                                onShown: page.reveal(layersError)
                            }
                            Item {
                                visible: root.layerModel.length >= 2
                                width: parent.width
                                height: clearLayers.height
                                Plaque {
                                    id: clearLayers
                                    anchors.right: parent.right
                                    kind: "quiet"
                                    small: true
                                    glyph: "delete_sweep"
                                    text: I18n.tr("Remove all layer rules")
                                    onClicked: root.removeAllLayers()
                                }
                            }
                        }
                    }
                }
            }

            Label {
                width: parent.width
                visible: !root.layersShown
                text: I18n.tr(Schema.sections[1].missing)
                typeRole: "detail"
                inkRole: "fg3"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
        }
    }

    Component {
        id: ruleEditor
        RuleEditor {
            listed: root.listed
            taken: root.groups.filter((g, i) => i !== rulesEditorIndex()).map(g => g.key)
            overlayParent: root.hub.overlayItem
            function rulesEditorIndex() {
                return root.rulesEditor ? root.rulesEditor.editingIndex : -1;
            }
        }
    }

    Component {
        id: layerEditor
        LayerEditor {
            spec: root.spec
            overlayParent: root.hub.overlayItem
        }
    }

    // a save the window manager refused, with Retry, under the list it belongs to; shown
    // says it just appeared, so the page brings it into view (the list's editor has closed)
    component SaveError: Row {
        property var bind: null
        readonly property string error: bind ? Changes.error(bind) : ""
        signal shown()
        onErrorChanged: {
            if (error.length > 0)
                Qt.callLater(() => shown());
        }
        visible: error.length > 0
        width: parent ? parent.width : 0
        topPadding: LithicTokens.space.s2
        bottomPadding: LithicTokens.space.s2
        spacing: LithicTokens.space.s3

        Label {
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(implicitWidth, parent.width - retry.width - LithicTokens.space.s3)
            text: parent.error
            typeRole: "detail"
            ink: Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Plaque {
            id: retry
            anchors.verticalCenter: parent.verticalCenter
            kind: "quiet"
            small: true
            glyph: "refresh"
            text: I18n.tr("Retry")
            onClicked: Changes.retry(parent.bind)
        }
    }
}
