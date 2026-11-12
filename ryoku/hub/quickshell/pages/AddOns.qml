import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/add-ons"
import "../schema/add-ons.js" as Schema

// installed add-ons, bundles and Omarchy add-ons, each with a view of its own
Item {
    id: root

    required property var hub
    property string pendingRow: ""
    // the add-on or bundle open in its own view: { kind: "plugin" | "bundle", id }
    property var detail: null
    property string pendingOpen: ""
    readonly property bool ownBack: root.hub.sizeClass === "expanded"

    readonly property var store: Changes.stores["addons"] || null
    readonly property bool omarchyShown: !!root.store && root.store.omarchyAvailable

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: root.withSectionMarks(Schema.sections, Schema.rows.filter(r => r.omarchy !== true || root.omarchyShown))
    })

    // a template row per section, so a deep link to a section lands through the frame once the column is placed
    function withSectionMarks(sections, rows) {
        return rows.concat(sections.map(s => ({ id: s.id, section: s.id, label: s.title, template: true })));
    }

    function bind(id, part) {
        return { store: "addons", key: "p/" + id + "/" + part };
    }

    function hostLabel(host) {
        switch (host) {
        case "framePopout": return "Pop-out";
        case "topbarGlyph": return "Bar";
        case "sidebarCard": return "Quick settings";
        }
        return host;
    }

    function whereText(host) {
        switch (host) {
        case "framePopout": return I18n.tr("Opens as a pop-out from the edge of the screen");
        case "topbarGlyph": return I18n.tr("Shows on the bar");
        case "sidebarCard": return I18n.tr("Shows as a card on Quick settings");
        }
        return "";
    }

    function pluginLine(p) {
        var m = p.manifest || {};
        var lines = [];
        var summary = String(m.summary || m.description || "");
        if (summary.length > 0)
            lines.push(summary);
        var have = root.store.installedVersion(p);
        var facts = root.whereText(root.store.hostOf(p));
        if (have.length > 0)
            facts = I18n.tr("%1, version %2").arg(facts).arg(have);
        var update = root.store.updateFor(p);
        if (update.length > 0)
            facts += ". " + I18n.tr("Version %1 is ready to install").arg(update);
        lines.push(facts);
        return lines.join("\n");
    }

    function bundleLine(b) {
        var parts = root.store.partsOf(b);
        var lines = [];
        var summary = String(b.summary || "");
        if (summary.length > 0)
            lines.push(summary);
        if (parts.installed.length === 0)
            lines.push(I18n.tr("Nothing from it is installed"));
        else if (parts.missing.length === 0)
            lines.push(I18n.tr("Every needed part is installed"));
        else
            lines.push(I18n.tr("%1 of %2 needed parts installed").arg(parts.neededInstalled).arg(parts.needed.length));
        return lines.join("\n");
    }

    function kindsText(o) {
        var words = (o.kinds || []).map(k => {
            switch (k) {
            case "bar-widget": return I18n.tr("Bar widget");
            case "bar": return I18n.tr("Bar style");
            case "panel": return I18n.tr("Panel");
            case "overlay": return I18n.tr("Overlay");
            case "menu": return I18n.tr("Menu");
            case "service": return I18n.tr("Service");
            }
            return k;
        });
        return words.length > 0 ? words.join(", ") : I18n.tr("Add-on");
    }

    function omarchyLine(o) {
        var lines = [];
        if (o.description)
            lines.push(String(o.description));
        var facts = root.kindsText(o);
        if (o.version)
            facts = I18n.tr("%1, version %2").arg(facts).arg(o.version);
        lines.push(facts);
        if (o.blocked && o.blockedReason)
            lines.push(I18n.tr("Cannot be turned on: %1").arg(o.blockedReason));
        return lines.join("\n");
    }

    function openPlugin(id, at) {
        root.detail = { kind: "plugin", id: id, at: at || "" };
    }

    function openBundle(id, at) {
        root.detail = { kind: "bundle", id: id, at: at || "" };
        if (!root.store.statusKnown(id))
            root.store.loadStatus(id);
    }

    function closeDetail() {
        root.detail = null;
    }

    // the frame's Back steps out of an open add-on or bundle before leaving the page
    function back() {
        if (root.detail === null)
            return false;
        root.closeDetail();
        return true;
    }

    // the page chosen again from the list returns to the list
    function reset() {
        root.pendingOpen = "";
        root.closeDetail();
    }

    function askRemovePlugin(id) {
        var name = root.store.nameOf(root.store.plugin(id));
        root.hub.confirm({
            title: I18n.tr("Remove %1?").arg(name),
            text: I18n.tr("This deletes %1 and its settings from your desktop. You can install it again from RyoStore.").arg(name),
            actionText: I18n.tr("Remove")
        }, () => root.store.remove(id));
    }

    function askRemoveBundle(id) {
        var b = root.store.bundle(id);
        var name = b ? String(b.name || id) : id;
        root.hub.confirm({
            title: I18n.tr("Remove the parts of %1?").arg(name),
            text: I18n.tr("A terminal window opens and removes the parts of %1 that are on this computer. You can install them again from RyoStore.").arg(name),
            actionText: I18n.tr("Remove")
        }, () => root.store.removeBundle(id));
    }

    function askRemoveOmarchy(id) {
        var o = root.store.omarchyItem(id);
        var name = o ? String(o.name || id) : id;
        root.hub.confirm({
            title: I18n.tr("Remove %1?").arg(name),
            text: I18n.tr("This removes %1 from the Omarchy bar. You can install it again from the Omarchy market.").arg(name),
            actionText: I18n.tr("Remove")
        }, () => root.store.removeOmarchy(id));
    }

    // "add-on:<id>[:<row or section>]" and "bundle:<id>[:...]" open that item once it is known
    function openPending() {
        var target = root.pendingOpen;
        if (target === "" || !root.store)
            return;
        var rest = target.slice(target.indexOf(":") + 1);
        var colon = rest.indexOf(":");
        var id = colon >= 0 ? rest.slice(0, colon) : rest;
        var at = colon >= 0 ? rest.slice(colon + 1) : "";
        if (target.indexOf("add-on:") === 0 && root.store.plugin(id)) {
            root.pendingOpen = "";
            root.openPlugin(id, at);
        } else if (target.indexOf("bundle:") === 0 && root.store.bundle(id)) {
            root.pendingOpen = "";
            root.openBundle(id, at);
        }
    }

    function landIn(view, at) {
        if (at)
            view.focusRow(at);
    }

    // the lists load after the page; a section landed on before then would scroll to the wrong place
    readonly property bool settled: !!root.store
        && (root.store.pluginsLoaded || root.store.readError("plugins") !== "")
        && (root.store.bundlesLoaded || root.store.readError("bundles") !== "")
        && root.store.omarchyLoaded
    onSettledChanged: {
        if (root.settled && root.pendingRow !== "") {
            var id = root.pendingRow;
            root.pendingRow = "";
            Qt.callLater(root.focusRow, id);
        }
    }

    // search lands on drawn rows; a row the page draws itself lands on its section
    function focusRow(rowId) {
        if (!rowId)
            return;
        root.detail = null;
        if (rowId.indexOf("add-on:") === 0 || rowId.indexOf("bundle:") === 0) {
            root.pendingOpen = rowId;
            root.openPending();
            return;
        }
        if (page.layout.length === 0 || !root.settled) {
            root.pendingRow = rowId;
            return;
        }
        page.focusRow(rowId);
    }

    // the store outlives the page, so undo works from anywhere
    Component.onCompleted: {
        Changes.ensureStore("addons", Qt.resolvedUrl("../components/add-ons/AddOnsStore.qml"));
        if (root.store)
            root.store.refresh();
    }

    Connections {
        target: root.store ? root.store.pluginTask : null
        function onFinished(target, action, ok) {
            // the Remove that was pressed is gone with the add-on's page; the keyboard goes to the
            // list, or to RyoStore when that was the last add-on
            if (ok && action === "remove" && root.detail && root.detail.id === target) {
                var left = root.store.plugins.some(p => String(p.id || "") !== target);
                root.closeDetail();
                Qt.callLater(root.focusRow, left ? "installed" : "browse-add-ons");
            }
        }
    }

    Connections {
        target: root.store
        function onPluginsChanged() {
            if (root.detail && root.detail.kind === "plugin" && root.store.pluginsLoaded && !root.store.plugin(root.detail.id))
                root.closeDetail();
            root.openPending();
        }
        function onBundlesChanged() {
            root.openPending();
        }
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        visible: root.detail === null
        hub: root.hub
        pageId: "add-ons"
        schema: root.schema
        bodies: ({ installed: installedBody, bundles: bundlesBody, omarchy: root.omarchyShown ? omarchyBody : null })
        onLayoutChanged: {
            if (root.pendingRow !== "") {
                var id = root.pendingRow;
                root.pendingRow = "";
                Qt.callLater(root.focusRow, id);
            }
        }
    }

    Loader {
        id: detailLoader
        anchors.fill: parent
        active: root.detail !== null && root.store !== null
        sourceComponent: root.detail && root.detail.kind === "bundle" ? bundleView : pluginView
    }

    component ErrorLine: Row {
        id: errorLine
        property string text: ""
        signal retry()
        width: parent ? parent.width : 0
        visible: errorLine.text.length > 0
        spacing: LithicTokens.space.s3

        Label {
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(implicitWidth, errorLine.width - retryPlaque.width - errorLine.spacing)
            text: errorLine.text
            typeRole: "detail"
            ink: Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Plaque {
            id: retryPlaque
            anchors.verticalCenter: parent.verticalCenter
            kind: "quiet"
            small: true
            glyph: "refresh"
            text: I18n.tr("Retry")
            onClicked: errorLine.retry()
        }
    }

    component LoadingLine: Row {
        property string text: ""
        property bool active: false
        visible: active
        spacing: LithicTokens.space.s2
        Working {
            anchors.verticalCenter: parent.verticalCenter
            active: parent.active
        }
        Label {
            anchors.verticalCenter: parent.verticalCenter
            text: parent.text
            typeRole: "detail"
        }
    }

    component QuietLine: Label {
        width: parent ? parent.width : 0
        typeRole: "body"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }

    Component {
        id: installedBody
        Column {
            spacing: LithicTokens.space.s2

            ErrorLine {
                text: root.store.readError("plugins")
                onRetry: root.store.loadPlugins()
            }
            LoadingLine {
                active: !root.store.pluginsLoaded && root.store.readError("plugins") === ""
                text: I18n.tr("Looking for installed add-ons")
            }
            QuietLine {
                visible: root.store.pluginsLoaded && root.store.plugins.length === 0
                text: I18n.tr("No add-ons yet. Find some in RyoStore, below.")
            }

            Slab {
                width: parent.width
                visible: root.store.plugins.length > 0

                Column {
                    width: parent.width

                    Repeater {
                        model: root.store.plugins.length
                        delegate: Column {
                            id: pluginItem
                            required property int index
                            readonly property var plugin: root.store.plugins[pluginItem.index] || ({})
                            readonly property string pid: String(pluginItem.plugin.id || "")
                            readonly property string name: root.store.nameOf(pluginItem.plugin)
                            readonly property var onBind: root.bind(pluginItem.pid, "on")
                            width: parent.width
                            spacing: LithicTokens.space.s1

                            SettingRow {
                                width: parent.width
                                first: pluginItem.index === 0
                                title: pluginItem.name
                                detail: root.pluginLine(pluginItem.plugin)
                                activatable: true
                                onActivated: root.openPlugin(pluginItem.pid)

                                Toggle {
                                    checked: Changes.value(pluginItem.onBind) === true
                                    enabled: !(root.store.pluginTask.running && root.store.pluginTask.target === pluginItem.pid)
                                    Accessible.name: I18n.tr("Use %1").arg(pluginItem.name)
                                    onToggled: Changes.set(pluginItem.onBind, checked, pluginItem.name, "add-ons",
                                        (checked ? I18n.tr("%1 is on") : I18n.tr("%1 is off")).arg(pluginItem.name))
                                }
                                IconPlaque {
                                    small: true
                                    kind: "quiet"
                                    glyph: "chevron_right"
                                    accessibleName: I18n.tr("Open %1").arg(pluginItem.name)
                                    onClicked: root.openPlugin(pluginItem.pid)
                                }
                            }
                            ErrorLine {
                                text: Changes.error(pluginItem.onBind)
                                onRetry: Changes.retry(pluginItem.onBind)
                            }
                            TaskLine {
                                task: root.store.pluginTask
                                target: pluginItem.pid
                                doing: root.store.pluginTask.action === "remove"
                                    ? I18n.tr("Removing %1").arg(pluginItem.name)
                                    : I18n.tr("Updating %1").arg(pluginItem.name)
                            }
                        }
                    }
                }
            }

            ErrorLine {
                text: root.store.readError("catalog")
                onRetry: root.store.loadCatalog()
            }

            Row {
                visible: root.store.widgetCount > 0
                width: parent.width
                spacing: LithicTokens.space.s3

                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.min(implicitWidth, parent.width - widgetsLink.width - parent.spacing)
                    text: (root.store.widgetCount === 1
                        ? I18n.tr("One desktop widget is installed too. Widgets are placed on the desktop.")
                        : I18n.tr("%1 desktop widgets are installed too. Widgets are placed on the desktop.").arg(root.store.widgetCount))
                    typeRole: "detail"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
                Plaque {
                    id: widgetsLink
                    anchors.verticalCenter: parent.verticalCenter
                    kind: "tonal"
                    small: true
                    glyph: "arrow_forward"
                    text: I18n.tr("Desktop & dock")
                    onClicked: root.hub.navigate("desktop-dock", "")
                }
            }
        }
    }

    Component {
        id: bundlesBody
        Column {
            spacing: LithicTokens.space.s2

            ErrorLine {
                text: root.store.readError("bundles")
                onRetry: root.store.loadBundles()
            }
            ErrorLine {
                text: root.store.readError("status")
                onRetry: root.store.loadStatus("")
            }
            LoadingLine {
                active: !root.store.bundlesLoaded && root.store.readError("bundles") === ""
                text: I18n.tr("Looking for bundles")
            }
            QuietLine {
                visible: root.store.bundlesLoaded && root.store.bundles.length === 0
                text: I18n.tr("No bundles yet. Bundles install a set of apps and tools together; find them in RyoStore, below.")
            }

            Slab {
                width: parent.width
                visible: root.store.bundles.length > 0

                Column {
                    width: parent.width

                    Repeater {
                        model: root.store.bundles.length
                        delegate: Column {
                            id: bundleItem
                            required property int index
                            readonly property var bundle: root.store.bundles[bundleItem.index] || ({})
                            readonly property string bid: String(bundleItem.bundle.id || "")
                            readonly property string name: String(bundleItem.bundle.name || bundleItem.bid)
                            readonly property var parts: root.store.partsOf(bundleItem.bundle)
                            readonly property bool busy: root.store.bundleTask.running && root.store.bundleTask.target === bundleItem.bid
                            width: parent.width
                            spacing: LithicTokens.space.s1

                            SettingRow {
                                width: parent.width
                                first: bundleItem.index === 0
                                title: bundleItem.name
                                detail: root.bundleLine(bundleItem.bundle)
                                activatable: true
                                onActivated: root.openBundle(bundleItem.bid)

                                Plaque {
                                    visible: bundleItem.parts.missing.length > 0
                                    kind: "quiet"
                                    glyph: "download"
                                    small: true
                                    text: bundleItem.parts.installed.length === 0 ? I18n.tr("Install") : I18n.tr("Install missing")
                                    busy: bundleItem.busy && root.store.bundleTask.action === "install"
                                    enabled: !root.store.bundleTask.running
                                    accessibleName: I18n.tr("Install the missing parts of %1").arg(bundleItem.name)
                                    onClicked: root.store.installBundle(bundleItem.bid)
                                }
                                IconPlaque {
                                    small: true
                                    kind: "quiet"
                                    glyph: "chevron_right"
                                    accessibleName: I18n.tr("Open %1").arg(bundleItem.name)
                                    onClicked: root.openBundle(bundleItem.bid)
                                }
                            }
                            TaskLine {
                                task: root.store.bundleTask
                                target: bundleItem.bid
                                doing: root.store.bundleTask.action === "remove"
                                    ? I18n.tr("Removing the parts of %1 in a terminal window").arg(bundleItem.name)
                                    : I18n.tr("Installing the missing parts of %1 in a terminal window").arg(bundleItem.name)
                                stopHint: I18n.tr("To stop it, close the terminal window.")
                            }
                        }
                    }
                }
            }
        }
    }

    Component {
        id: omarchyBody
        Column {
            spacing: LithicTokens.space.s2

            ErrorLine {
                text: root.store.readError("omarchy")
                onRetry: root.store.loadOmarchy()
            }
            QuietLine {
                visible: root.store.omarchy.length === 0
                text: I18n.tr("No Omarchy add-ons yet.")
            }

            Slab {
                width: parent.width
                visible: root.store.omarchy.length > 0

                Column {
                    width: parent.width

                    Repeater {
                        model: root.store.omarchy.length
                        delegate: Column {
                            id: omarchyItem
                            required property int index
                            readonly property var item: root.store.omarchy[omarchyItem.index] || ({})
                            readonly property string oid: String(omarchyItem.item.id || "")
                            readonly property string name: String(omarchyItem.item.name || omarchyItem.oid)
                            readonly property var onBind: ({ store: "addons", key: "o/" + omarchyItem.oid })
                            readonly property bool busy: root.store.omarchyTask.running && root.store.omarchyTask.target === omarchyItem.oid
                            width: parent.width
                            spacing: LithicTokens.space.s1

                            SettingRow {
                                width: parent.width
                                first: omarchyItem.index === 0
                                title: omarchyItem.name
                                detail: root.omarchyLine(omarchyItem.item)

                                Toggle {
                                    checked: Changes.value(omarchyItem.onBind) === true
                                    enabled: !omarchyItem.busy && omarchyItem.item.canDisable !== false
                                        && !(omarchyItem.item.blocked === true && !checked)
                                    Accessible.name: I18n.tr("Use %1").arg(omarchyItem.name)
                                    onToggled: Changes.set(omarchyItem.onBind, checked, omarchyItem.name, "add-ons",
                                        (checked ? I18n.tr("%1 is on") : I18n.tr("%1 is off")).arg(omarchyItem.name))
                                }
                                Plaque {
                                    kind: "dangerLine"
                                    small: true
                                    text: I18n.tr("Remove")
                                    busy: omarchyItem.busy
                                    enabled: !root.store.omarchyTask.running
                                    accessibleName: I18n.tr("Remove %1").arg(omarchyItem.name)
                                    onClicked: root.askRemoveOmarchy(omarchyItem.oid)
                                }
                            }
                            ErrorLine {
                                text: Changes.error(omarchyItem.onBind)
                                onRetry: Changes.retry(omarchyItem.onBind)
                            }
                            TaskLine {
                                task: root.store.omarchyTask
                                target: omarchyItem.oid
                                doing: I18n.tr("Removing %1").arg(omarchyItem.name)
                            }
                        }
                    }
                }
            }
        }
    }

    Component {
        id: pluginView
        FocusScope {
            id: view

            readonly property string pid: root.detail ? root.detail.id : ""
            readonly property var plugin: {
                Changes.revision;
                return root.store.plugin(view.pid);
            }
            readonly property var manifest: view.plugin ? (view.plugin.manifest || {}) : ({})
            readonly property string name: root.store.nameOf(view.plugin)
            readonly property string host: Changes.value(root.bind(view.pid, "host")) || "framePopout"
            readonly property bool on: Changes.value(root.bind(view.pid, "on")) === true
            readonly property string edge: Changes.value(root.bind(view.pid, "edge")) || "top"
            readonly property string align: Changes.value(root.bind(view.pid, "align")) || "start"
            readonly property bool popout: view.on && view.host === "framePopout"
            readonly property string have: root.store.installedVersion(view.plugin)
            readonly property string update: root.store.updateFor(view.plugin)
            readonly property var fields: ((view.manifest.metadata || {}).settings) || []
            readonly property var hosts: root.store.hostsOf(view.plugin).filter(h => root.store.hostOrder.indexOf(h) >= 0)
            readonly property var groups: {
                var out = [];
                view.fields.forEach(f => {
                    var g = String(f.group || "");
                    if (out.indexOf(g) < 0)
                        out.push(g);
                });
                return out;
            }

            function fieldRow(f) {
                var g = view.groups.indexOf(String(f.group || ""));
                var row = {
                    id: "option-" + f.key, section: "group-" + g,
                    label: String(f.label || f.key), detail: String(f.description || ""),
                    changeLabel: I18n.tr("%1: %2").arg(view.name).arg(f.label || f.key),
                    bind: { store: "addons", key: "p/" + view.pid + "/set/" + f.key, fallback: f["default"] }
                };
                if (f["default"] !== undefined)
                    row["default"] = f["default"];
                var options = (f.options || []).map(o => ({ label: String(o.label), value: o.value }));
                switch (f.type) {
                case "toggle":
                    row.control = "toggle";
                    break;
                case "choice":
                    row.control = options.length <= 4 ? "segmented" : "picker";
                    row.options = options;
                    break;
                case "slider":
                    var decimals = f.decimals !== undefined ? f.decimals : 2;
                    row.control = "slider";
                    row.min = f.min !== undefined ? f.min : 0;
                    row.max = f.max !== undefined ? f.max : 1;
                    row.step = f.step !== undefined ? f.step : (decimals === 0 ? 1 : 0.01);
                    break;
                case "image":
                    row.control = "path";
                    row.kind = "file";
                    row.asUrl = true;
                    row.placeholder = I18n.tr("Choose an image");
                    row.nameFilters = [I18n.tr("Images") + " (*.png *.jpg *.jpeg *.webp *.bmp *.gif *.svg)", I18n.tr("All files") + " (*)"];
                    break;
                default:
                    row.control = "text";
                    if (f.placeholder)
                        row.placeholder = String(f.placeholder);
                    break;
                }
                return row;
            }

            readonly property var schema: {
                var where = I18n.tr("Where %1 opens").arg(view.name);
                var sections = [{ id: "place", title: "Where it shows" }];
                view.groups.forEach((g, i) => sections.push({ id: "group-" + i, title: g.length > 0 ? g : "Options" }));
                sections.push({ id: "manage", title: "Update and remove" });

                var rows = [{
                    id: "on", section: "place", label: I18n.tr("Use %1").arg(view.name), changeLabel: view.name,
                    detail: view.whereDetail, control: "toggle", bind: root.bind(view.pid, "on")
                }];
                if (view.hosts.length > 1)
                    rows.push({
                        id: "show-as", section: "place", label: "Show as", changeLabel: I18n.tr("Where %1 shows").arg(view.name),
                        detail: "Where it appears on your desktop", control: "segmented",
                        options: view.hosts.map(h => ({ label: root.hostLabel(h), value: h })),
                        bind: root.bind(view.pid, "host"), "default": String((view.manifest.defaults || {}).host || "framePopout")
                    });
                if (view.popout) {
                    rows.push({
                        id: "edge", section: "place", label: "Side of the screen", changeLabel: where,
                        detail: view.edge === "center"
                            ? "A pop-out in the centre has no edge to reach, so it opens from its keyboard shortcut. It takes turns with Quick settings, which opens there too."
                            : "It opens when the pointer reaches that side",
                        control: "segmented",
                        options: [{ label: "Centre", value: "center" }, { label: "Top", value: "top" }, { label: "Right", value: "right" },
                                  { label: "Bottom", value: "bottom" }, { label: "Left", value: "left" }],
                        bind: root.bind(view.pid, "edge"), "default": "top"
                    });
                    if (view.edge !== "center") {
                        var upright = view.edge === "left" || view.edge === "right";
                        rows.push({
                            id: "align", section: "place", label: "Along that side", changeLabel: where,
                            detail: "Where on that side it sits", control: "segmented",
                            options: upright
                                ? [{ label: "Top", value: "start" }, { label: "Middle", value: "center" }, { label: "Bottom", value: "end" }]
                                : [{ label: "Left", value: "start" }, { label: "Middle", value: "center" }, { label: "Right", value: "end" }],
                            bind: root.bind(view.pid, "align"), "default": "start"
                        });
                    }
                }
                rows = rows.concat(view.fields.filter(f => f && f.key).map(f => view.fieldRow(f)));
                return {
                    title: view.name,
                    lead: String(view.manifest.description || view.manifest.summary || I18n.tr("An add-on installed from RyoStore.")),
                    sections: sections,
                    rows: root.withSectionMarks(sections, rows)
                };
            }

            readonly property string whereDetail: {
                var text = root.whereText(view.host);
                return (text.length > 0 ? text + ". " : "") + I18n.tr("Off keeps it installed without running it.");
            }

            readonly property string stateLine: view.manifest.official === true ? ""
                : I18n.tr("A community add-on: Ryoku does not review it, and it runs inside your desktop with your permissions, so check its code before you trust it.")

            Plaque {
                id: back
                x: detailPage.columnX
                visible: root.ownBack
                kind: "quiet"
                small: true
                glyph: "arrow_back"
                text: I18n.tr("Add-ons")
                onClicked: root.closeDetail()
            }

            SettingsPage {
                id: detailPage
                anchors.top: back.visible ? back.bottom : parent.top
                anchors.topMargin: back.visible ? LithicTokens.space.s3 : 0
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.bottom: parent.bottom
                hub: root.hub
                pageId: "add-ons"
                schema: view.schema
                stateLine: view.stateLine
                bodies: ({ manage: manageBody })
                footers: ({ place: view.popout ? stageBody : view.on && view.host === "sidebarCard" ? cardFooter : null })
                property string landAt: root.detail ? root.detail.at : ""
                onLayoutChanged: {
                    if (detailPage.landAt !== "" && detailPage.layout.length > 0) {
                        Qt.callLater(root.landIn, detailPage, detailPage.landAt);
                        detailPage.landAt = "";
                    }
                }
            }

            Component {
                id: stageBody
                PopoutStage {
                    edge: view.edge
                    align: view.align
                    onPlaced: (edge, align) => Changes.set(root.bind(view.pid, "pos"), { edge: edge, align: align },
                        I18n.tr("Where %1 opens").arg(view.name), "add-ons")
                }
            }

            Component {
                id: cardFooter
                Slab {
                    HandoffRow {
                        width: parent.width
                        text: I18n.tr("Arrange on Quick settings")
                        detail: I18n.tr("Choose where its card sits among the others")
                        glyph: "arrow_forward"
                        onTriggered: root.hub.navigate("quick-settings", "extension-cards")
                    }
                }
            }

            Component {
                id: manageBody
                Column {
                    spacing: LithicTokens.space.s2

                    Slab {
                        width: parent.width

                        Column {
                            width: parent.width

                            SettingRow {
                                width: parent.width
                                first: true
                                title: view.update.length > 0 ? I18n.tr("Update to version %1").arg(view.update)
                                    : view.have.length > 0 ? I18n.tr("Version %1").arg(view.have) : I18n.tr("Version")
                                detail: view.update.length > 0 ? I18n.tr("You have version %1.").arg(view.have)
                                    : root.store.catalogKnown ? I18n.tr("This is the newest version.")
                                    : I18n.tr("RyoStore could not say whether a newer version exists. Check again, or look in RyoStore.")

                                Plaque {
                                    visible: view.update.length > 0
                                    kind: "line"
                                    small: true
                                    glyph: "download"
                                    text: I18n.tr("Update")
                                    busy: root.store.pluginTask.running && root.store.pluginTask.target === view.pid
                                        && root.store.pluginTask.action === "update"
                                    enabled: !root.store.pluginTask.running
                                    onClicked: root.store.update(view.pid)
                                }
                                Plaque {
                                    visible: view.update.length === 0 && !root.store.catalogKnown && root.store.readError("catalog") === ""
                                    kind: "quiet"
                                    small: true
                                    glyph: "refresh"
                                    text: I18n.tr("Check again")
                                    busy: root.store.catalogChecking
                                    enabled: !root.store.catalogChecking
                                    onClicked: root.store.loadCatalog()
                                }
                            }
                            SettingRow {
                                width: parent.width
                                title: I18n.tr("Remove %1").arg(view.name)
                                detail: I18n.tr("Deletes the add-on and its settings. You can install it again from RyoStore.")

                                Plaque {
                                    kind: "dangerLine"
                                    small: true
                                    glyph: "delete"
                                    text: I18n.tr("Remove")
                                    busy: root.store.pluginTask.running && root.store.pluginTask.target === view.pid
                                        && root.store.pluginTask.action === "remove"
                                    enabled: !root.store.pluginTask.running
                                    onClicked: root.askRemovePlugin(view.pid)
                                }
                            }
                        }
                    }
                    TaskLine {
                        task: root.store.pluginTask
                        target: view.pid
                        doing: root.store.pluginTask.action === "remove"
                            ? I18n.tr("Removing %1").arg(view.name)
                            : I18n.tr("Updating %1").arg(view.name)
                    }
                    ErrorLine {
                        text: root.store.readError("catalog")
                        onRetry: root.store.loadCatalog()
                    }
                }
            }
        }
    }

    Component {
        id: bundleView
        FocusScope {
            id: bview

            readonly property string bid: root.detail ? root.detail.id : ""
            readonly property var bundle: {
                Changes.revision;
                root.store.bundles;
                return root.store.bundle(bview.bid) || ({ id: bview.bid });
            }
            readonly property string name: String(bview.bundle.name || bview.bid)
            readonly property var parts: root.store.partsOf(bview.bundle)
            readonly property bool checking: !root.store.statusKnown(bview.bid) && root.store.readError("status") === ""
            readonly property bool busy: root.store.bundleTask.running && root.store.bundleTask.target === bview.bid

            function partRows(list, section) {
                return list.map((p, i) => ({
                    id: section + "-" + i, section: section, label: String(p.name || ""),
                    detail: p.available === false
                        ? (p.unavailableReason ? String(p.unavailableReason) : I18n.tr("Not available on this edition."))
                        : String(p.summary || "")
                }));
            }

            readonly property var sections: [
                { id: "bundle", title: "This bundle" },
                { id: "installed", title: "Installed" },
                { id: "missing", title: "Missing" },
                { id: "optional", title: "Optional extras" },
                { id: "unavailable", title: "Not available here" }
            ]

            readonly property var schema: ({
                title: bview.name,
                lead: String(bview.bundle.description || bview.bundle.summary || ""),
                sections: bview.sections,
                rows: root.withSectionMarks(bview.sections, bview.partRows(bview.parts.installed, "installed")
                    .concat(bview.partRows(bview.parts.missing, "missing"))
                    .concat(bview.partRows(bview.parts.optional, "optional"))
                    .concat(bview.partRows(bview.parts.unavailable, "unavailable")))
            })

            Plaque {
                id: bback
                x: bundlePage.columnX
                visible: root.ownBack
                kind: "quiet"
                small: true
                glyph: "arrow_back"
                text: I18n.tr("Add-ons")
                onClicked: root.closeDetail()
            }

            SettingsPage {
                id: bundlePage
                anchors.top: bback.visible ? bback.bottom : parent.top
                anchors.topMargin: bback.visible ? LithicTokens.space.s3 : 0
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.bottom: parent.bottom
                hub: root.hub
                pageId: "add-ons"
                schema: bview.schema
                bodies: ({ bundle: bundleHead })
                property string landAt: root.detail ? root.detail.at : ""
                onLayoutChanged: {
                    if (bundlePage.landAt !== "" && bundlePage.layout.length > 0) {
                        Qt.callLater(root.landIn, bundlePage, bundlePage.landAt);
                        bundlePage.landAt = "";
                    }
                }
            }

            Component {
                id: bundleHead
                Column {
                    spacing: LithicTokens.space.s2

                    LoadingLine {
                        active: bview.checking
                        text: I18n.tr("Checking which parts are installed")
                    }
                    ErrorLine {
                        text: root.store.readError("status")
                        onRetry: root.store.loadStatus(bview.bid)
                    }

                    Slab {
                        width: parent.width

                        Column {
                            width: parent.width

                            SettingRow {
                                width: parent.width
                                first: true
                                title: bview.parts.missing.length === 0 ? I18n.tr("Every needed part is installed")
                                    : I18n.tr("%1 of %2 needed parts installed").arg(bview.parts.neededInstalled).arg(bview.parts.needed.length)
                                detail: bview.parts.missing.length === 0 ? ""
                                    : I18n.tr("Installing finishes in a terminal window, where you confirm it.")

                                Plaque {
                                    visible: bview.parts.missing.length > 0
                                    kind: "line"
                                    small: true
                                    glyph: "download"
                                    text: bview.parts.installed.length === 0 ? I18n.tr("Install") : I18n.tr("Install missing")
                                    busy: bview.busy && root.store.bundleTask.action === "install"
                                    enabled: !root.store.bundleTask.running && !bview.checking
                                    onClicked: root.store.installBundle(bview.bid)
                                }
                            }
                            SettingRow {
                                width: parent.width
                                visible: bview.parts.installed.length > 0
                                title: I18n.tr("Remove the parts of %1").arg(bview.name)
                                detail: I18n.tr("Removes what this bundle installed, in a terminal window.")

                                Plaque {
                                    kind: "dangerLine"
                                    small: true
                                    glyph: "delete"
                                    text: I18n.tr("Remove")
                                    busy: bview.busy && root.store.bundleTask.action === "remove"
                                    enabled: !root.store.bundleTask.running && !bview.checking
                                    onClicked: root.askRemoveBundle(bview.bid)
                                }
                            }
                        }
                    }
                    TaskLine {
                        task: root.store.bundleTask
                        target: bview.bid
                        doing: root.store.bundleTask.action === "remove"
                            ? I18n.tr("Removing the parts of %1 in a terminal window").arg(bview.name)
                            : I18n.tr("Installing the missing parts of %1 in a terminal window").arg(bview.name)
                        stopHint: I18n.tr("To stop it, close the terminal window.")
                    }
                }
            }
        }
    }
}
