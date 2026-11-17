import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/installed-apps"
import "../components/launcher"
import "../components/installed-apps/apps.js" as Apps
import "../schema/installed-apps.js" as Schema

// every app on this computer with the space it takes: Ryoku's own pinned on top, a chart of
// the space, and the list to sort, group, filter, keep out of app search or remove
Item {
    id: root

    required property var hub
    readonly property string pageId: "installed-apps"
    property string pendingRow: ""

    readonly property var store: Changes.stores["installed-apps"] || null
    readonly property var hiddenBind: ({ store: "launcher", key: "hiddenApps" })
    readonly property var hidden: {
        var v = Changes.value(root.hiddenBind);
        return Array.isArray(v) ? v : [];
    }
    readonly property bool canHide: styles.ready && styles.supports("hiddenApps")

    // how the list reads: a source filter ("all" or a kind), words, the order, grouping
    property string kind: "all"
    property string query: ""
    property string sortBy: "size"
    property bool bySize: true
    // a removal running in a terminal window (the AUR): its app's name, until the window closes
    property string terminalApp: ""
    property string doneText: ""
    // the app whose removal plan is being read, and one that other packages need: { id, names }
    property string planningId: ""
    property var blocked: null

    readonly property var listed: root.store ? Apps.withoutRyoku(root.store.apps, root.store.ryoku) : []
    readonly property var counts: Apps.kindCounts(root.listed)
    readonly property var kindIds: ["all"].concat(Apps.kinds.map(k => k.id).filter(id => root.counts[id] > 0))
    readonly property var visibleApps: Apps.filtered(root.listed, root.kind, root.query)
    readonly property var groups: root.bySize ? Apps.grouped(root.visibleApps, root.sortBy)
        : (root.visibleApps.length > 0 ? [{ id: "all", label: "", apps: Apps.sorted(root.visibleApps, root.sortBy) }] : [])
    readonly property var space: root.store ? Apps.space(root.store.apps) : Apps.space([])

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: Schema.rows.concat(Schema.sections.map(s => ({ id: s.id, section: s.id, label: s.title, template: true })))
    })

    function size(bytes) {
        return Apps.humanSize(bytes, I18n.tr);
    }

    // "A, B and C"
    function joined(names) {
        if (names.length <= 1)
            return names.join("");
        return I18n.tr("%1 and %2").arg(names.slice(0, -1).join(", ")).arg(names[names.length - 1]);
    }

    function detailOf(app, shown) {
        var facts = [I18n.tr(Apps.kindLabel(Apps.kindOf(app)))];
        if (app.version !== "")
            facts.push(app.version);
        facts.push(app.bytes > 0 ? root.size(app.bytes) : I18n.tr("Size not known"));
        if (Apps.partOfRyoku(app))
            facts.push(I18n.tr("Part of Ryoku"));
        if (!shown)
            facts.push(I18n.tr("Out of app search"));
        var lines = app.summary !== "" ? [app.summary] : [];
        lines.push(facts.join(" · "));
        if (Apps.removeCommand(app) === null && !Apps.partOfRyoku(app))
            lines.push(I18n.tr("No package manager owns it, so Settings cannot remove it"));
        return lines.join("\n");
    }

    function ryokuDetail(app, shown) {
        var lines = app.summary !== "" ? [I18n.tr(app.summary)] : [];
        if (!app.installed) {
            lines.push(I18n.tr("Not installed"));
            return lines.join("\n");
        }
        var facts = [];
        if (app.version !== "")
            facts.push(I18n.tr("Version %1").arg(app.version));
        if (app.bytes > 0)
            facts.push(root.size(app.bytes));
        if (app.builtin)
            facts.push(I18n.tr("Part of Ryoku"));
        if (!shown && app.desktopId !== "")
            facts.push(I18n.tr("Out of app search"));
        if (facts.length > 0)
            lines.push(facts.join(" · "));
        if (app.update)
            lines.push(app.channel === "prerelease"
                ? I18n.tr("Version %1, a preview release, is ready to install").arg(app.latest)
                : I18n.tr("Version %1 is ready to install").arg(app.latest));
        return lines.join("\n");
    }

    function setShown(app, shown) {
        Changes.set(root.hiddenBind, Apps.withHidden(root.hidden, app.id, !shown), "Hidden apps", root.pageId,
            (shown ? I18n.tr("%1 shows in app search") : I18n.tr("%1 is out of app search")).arg(app.name));
    }

    function openStore(route) {
        Spawn.runApp(Schema.store(route), { label: "ryostore" });
    }

    // Remove first reads the plan as you: a package others need is never offered for removal,
    // since the package manager would refuse it; the row says which ones need it instead
    function askRemove(app) {
        var argv = Apps.planCommand(app);
        if (!argv || root.planningId !== "")
            return;
        root.blocked = null;
        root.planningId = app.id;
        Changes.run(argv, (ok, out, err, code) => {
            root.planningId = "";
            var needs = ok ? Apps.requiredBy(out) : [];
            if (needs.length > 0)
                root.blocked = { id: app.id, names: needs };
            else
                root.confirmRemove(app);
        }, { timeout: 20000 });
    }

    // the question names what goes, the space that comes back and how it runs
    function confirmRemove(app) {
        var how = Apps.removeCommand(app);
        if (!how)
            return;
        var plan = Apps.removal(app, root.store.apps);
        var text = app.bytes > 0
            ? I18n.tr("%1 is removed from this computer, and %2 is free again.").arg(app.name).arg(root.size(app.bytes))
            : I18n.tr("%1 is removed from this computer.").arg(app.name);
        if (plan.others.length > 0)
            text += " " + I18n.tr("It comes in one package with %1, which goes too.").arg(root.joined(plan.others));
        if (how.kind === "terminal")
            text += " " + I18n.tr("A terminal window opens to remove it with your password.");
        else if (how.argv[0] === "pkexec")
            text += " " + I18n.tr("Settings asks for an administrator's password the first time.");
        text += " " + I18n.tr("You can install it again from RyoStore.");
        root.hub.confirm({
            title: I18n.tr("Remove %1?").arg(app.name),
            text: text,
            actionText: I18n.tr("Remove")
        }, () => {
            if (how.kind === "terminal") {
                root.terminalApp = app.name;
                root.hub.runInTerminal(how.line, "dev.ryoku.apps", (ok, code) => {
                    root.terminalApp = "";
                    root.store.fresh = true;
                    root.store.refresh();
                }, I18n.tr("Done. Press Enter to close."));
            } else {
                root.store.remove(app.id);
            }
        });
    }

    function askRyoku(app) {
        root.hub.confirm({
            title: I18n.tr("Remove %1?").arg(app.name),
            text: (app.bytes > 0
                ? I18n.tr("%1 is removed from this computer, and %2 is free again.").arg(app.name).arg(root.size(app.bytes))
                : I18n.tr("%1 is removed from this computer.").arg(app.name))
                + " " + I18n.tr("Settings asks for an administrator's password the first time.")
                + " " + I18n.tr("You can get it again from RyoStore."),
            actionText: I18n.tr("Remove")
        }, () => root.store.changeRyoku(app.id, "remove"));
    }

    // the lists load after the page; a section landed on before then would scroll to the wrong place
    readonly property bool settled: !!root.store && root.store.appsLoaded && root.store.ryokuLoaded
    onSettledChanged: {
        if (root.settled && root.pendingRow !== "") {
            var id = root.pendingRow;
            root.pendingRow = "";
            Qt.callLater(root.focusRow, id);
        }
    }

    function focusRow(rowId) {
        if (!rowId)
            return;
        if (page.layout.length === 0 || !root.settled) {
            root.pendingRow = rowId;
            return;
        }
        page.focusRow(rowId);
    }

    Component.onCompleted: {
        Changes.ensureStore("launcher", Qt.resolvedUrl("../components/launcher/LauncherStore.qml"));
        Changes.ensureStore("installed-apps", Qt.resolvedUrl("../components/installed-apps/InstalledAppsStore.qml"));
        if (root.store)
            root.store.refresh();
    }

    Connections {
        target: root.store ? root.store.job : null
        function onFinished(target, action, ok) {
            if (!ok)
                return;
            var name = root.store.job.label;
            root.doneText = action === "update" ? I18n.tr("%1 is up to date.").arg(name) : I18n.tr("%1 was removed.").arg(name);
            doneTimer.restart();
        }
    }
    Timer {
        id: doneTimer
        interval: 8000
        onTriggered: root.doneText = ""
    }

    // what the launcher in use offers: a launcher that cannot hide apps shows every one
    LauncherStyles {
        id: styles
        visible: false
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: root.pageId
        schema: root.schema
        stateLine: root.store && root.store.appsLoaded && root.store.appsError === ""
            ? I18n.tr("%1 apps, %2 in all").arg(root.space.apps).arg(root.size(root.space.total)) : ""
        bodies: ({ ryoku: ryokuBody, space: spaceBody, apps: appsBody })
        onLayoutChanged: {
            if (root.pendingRow !== "") {
                var id = root.pendingRow;
                root.pendingRow = "";
                Qt.callLater(root.focusRow, id);
            }
        }
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

    // names the column of switches at the rows' right edge
    component ColumnHead: Item {
        width: parent ? parent.width : 0
        height: head.implicitHeight + LithicTokens.space.s2
        Label {
            id: head
            anchors.right: parent.right
            anchors.top: parent.top
            text: I18n.tr("In app search")
            typeRole: "caption"
            inkRole: "fg3"
            font.capitalization: Font.AllUppercase
            font.letterSpacing: 1.2
        }
    }

    // a figure over its name, as Home sets its readings; it counts up once as it arrives
    component Reading: Column {
        id: reading
        property string label: ""
        property real value: 0
        property var format: v => String(Math.round(v))
        property real shown: 0
        spacing: LithicTokens.space.s1
        onValueChanged: {
            if (reading.value <= 0 || Lithic.reduceMotion || count.running) {
                reading.shown = reading.value;
                return;
            }
            count.to = reading.value;
            count.restart();
        }
        NumberAnimation {
            id: count
            target: reading
            property: "shown"
            duration: LithicTokens.motion.countUp
            easing.type: Easing.BezierSpline
            easing.bezierCurve: Lithic.curve
        }
        Label {
            text: reading.format(reading.shown)
            typeRole: "section"
            numeric: true
        }
        Label {
            text: reading.label
            typeRole: "caption"
            inkRole: "fg2"
        }
    }

    Component {
        id: ryokuBody
        Column {
            spacing: LithicTokens.space.s2

            ErrorLine {
                text: root.store.ryokuError
                onRetry: root.store.refresh()
            }
            LoadingLine {
                active: !root.store.ryokuLoaded
                text: I18n.tr("Looking for Ryoku's apps")
            }
            Slab {
                width: parent.width
                visible: root.store.ryoku.length > 0

                Column {
                    width: parent.width
                    ColumnHead {}
                    Repeater {
                        model: root.store.ryoku
                        delegate: RyokuAppRow {
                            required property var modelData
                            app: modelData
                            host: root
                        }
                    }
                }
            }
        }
    }

    Component {
        id: spaceBody
        Column {
            id: spaceCol
            spacing: LithicTokens.space.s4
            readonly property bool wide: width >= 760

            LoadingLine {
                active: !root.store.appsLoaded
                text: I18n.tr("Measuring the space apps take")
            }

            Slab {
                width: parent.width
                visible: root.store.appsLoaded && root.space.packages > 0

                Column {
                    width: parent.width
                    spacing: LithicTokens.space.s5

                    Row {
                        spacing: LithicTokens.space.s6
                        Reading {
                            label: I18n.tr("Taken by apps")
                            value: root.space.total
                            format: v => root.size(v)
                        }
                        Reading {
                            label: I18n.tr("Apps")
                            value: root.space.apps
                        }
                        Reading {
                            label: I18n.tr("Packages")
                            value: root.space.packages
                        }
                    }

                    Grid {
                        width: parent.width
                        columns: spaceCol.wide ? 2 : 1
                        columnSpacing: LithicTokens.space.s6
                        rowSpacing: LithicTokens.space.s5
                        readonly property real cell: spaceCol.wide ? (width - columnSpacing) / 2 : width

                        SpaceChart {
                            width: parent.cell
                            title: I18n.tr("Where they came from")
                            entries: root.space.bySource
                            countText: I18n.tr("%1 packages")
                            oneText: I18n.tr("1 package")
                        }
                        SpaceChart {
                            width: parent.cell
                            title: I18n.tr("By size")
                            entries: root.space.byGroup
                            countText: I18n.tr("%1 packages")
                            oneText: I18n.tr("1 package")
                        }
                    }

                    Label {
                        visible: root.space.unknown > 0
                        width: parent.width
                        text: root.space.unknown === 1
                            ? I18n.tr("One package does not say its size, so the figures leave it out.")
                            : I18n.tr("%1 packages do not say their size, so the figures leave them out.").arg(root.space.unknown)
                        typeRole: "detail"
                        inkRole: "fg3"
                        wrapMode: Text.Wrap
                        elide: Text.ElideNone
                    }
                }
            }
        }
    }

    Component {
        id: appsBody
        Column {
            id: appsCol
            spacing: LithicTokens.space.s3

            ErrorLine {
                text: root.store.appsError
                onRetry: root.store.refresh()
            }
            LoadingLine {
                active: !root.store.appsLoaded
                text: I18n.tr("Looking for installed apps")
            }
            LoadingLine {
                active: root.terminalApp !== ""
                text: I18n.tr("Removing %1 in a terminal window").arg(root.terminalApp)
            }

            // the last removal or update that went well, for a few seconds
            Row {
                spacing: LithicTokens.space.s2
                opacity: root.doneText !== "" ? 1 : 0
                visible: opacity > 0
                Behavior on opacity {
                    NumberAnimation { duration: Tone.move; easing.type: Easing.OutCubic }
                }
                Glyph {
                    anchors.verticalCenter: parent.verticalCenter
                    name: "check"
                    size: 16
                    ink: Lithic.jade
                }
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    text: root.doneText
                    typeRole: "detail"
                }
            }

            Filters {
                visible: root.store.appsLoaded && root.kindIds.length > 2
                accessibleName: I18n.tr("Show apps from")
                model: root.kindIds.map(id => id === "all"
                    ? { label: I18n.tr("All"), count: root.listed.length }
                    : { label: I18n.tr(Apps.kindLabel(id)), count: root.counts[id] })
                currentIndex: Math.max(0, root.kindIds.indexOf(root.kind))
                onActivated: index => root.kind = root.kindIds[index]
            }

            Item {
                visible: root.store.appsLoaded && root.listed.length > 0
                width: parent.width
                height: Math.max(find.implicitHeight, order.implicitHeight)

                Search {
                    id: find
                    anchors.left: parent.left
                    anchors.right: order.left
                    anchors.rightMargin: LithicTokens.space.s3
                    anchors.verticalCenter: parent.verticalCenter
                    placeholderText: I18n.tr("Find an installed app")
                    accessibleName: I18n.tr("Find an installed app")
                    onTextChanged: root.query = text
                }
                Segmented {
                    id: order
                    anchors.right: groupSwitch.left
                    anchors.rightMargin: LithicTokens.space.s4
                    anchors.verticalCenter: parent.verticalCenter
                    width: 220
                    accessibleName: I18n.tr("Sort apps")
                    model: [I18n.tr("Largest first"), I18n.tr("A to Z")]
                    currentIndex: root.sortBy === "name" ? 1 : 0
                    onActivated: index => root.sortBy = index === 1 ? "name" : "size"
                }
                Row {
                    id: groupSwitch
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: LithicTokens.space.s2
                    Label {
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("Group by size")
                        typeRole: "detail"
                    }
                    Toggle {
                        anchors.verticalCenter: parent.verticalCenter
                        checked: root.bySize
                        Accessible.name: I18n.tr("Group apps by size")
                        onToggled: root.bySize = checked
                    }
                }
            }

            QuietLine {
                visible: root.store.appsLoaded && !root.canHide && styles.ready && root.listed.length > 0
                text: I18n.tr("The launcher in use cannot leave apps out, so every app shows in app search. Another launcher can, in App launcher.")
            }

            QuietLine {
                visible: root.store.appsLoaded && root.store.appsError === "" && root.listed.length > 0 && root.visibleApps.length === 0
                text: root.query.trim() !== ""
                    ? I18n.tr("No app matches \u201c%1\u201d.").arg(root.query.trim())
                    : I18n.tr("No apps from there.")
            }
            QuietLine {
                visible: root.store.appsLoaded && root.store.appsError === "" && root.listed.length === 0
                text: I18n.tr("No apps yet. Get some from RyoStore, below.")
            }

            Repeater {
                model: root.groups
                delegate: Column {
                    id: group
                    required property var modelData
                    width: appsCol.width
                    spacing: LithicTokens.space.s2

                    GroupHeader {
                        visible: group.modelData.label !== ""
                        width: parent.width
                        title: I18n.tr(group.modelData.label)
                        count: group.modelData.apps.length
                    }
                    Slab {
                        width: parent.width
                        Column {
                            width: parent.width
                            ColumnHead {}
                            Repeater {
                                model: group.modelData.apps
                                delegate: AppRow {
                                    required property var modelData
                                    app: modelData
                                    host: root
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}
