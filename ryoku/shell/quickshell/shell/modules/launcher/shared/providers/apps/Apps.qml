import QtQuick
import Quickshell
import shell.services
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../../../../../utils/fuzzy.js" as Fuzzy
import "appactions.js" as AppActions
import ".."

// Desktop-application provider: ranks XDG entries by fuzzy match + launch
// frequency (the same ranker the pill launcher used) and launches the picked one.
// Registers as a default provider (no prefix): apps are the root search.
Provider {
    id: apps

    providerId: "apps"

    property int applicationRevision: 0

    readonly property var entries: {
        void apps.applicationRevision;
        var src = DesktopEntries.applications.values;
        var out = [];
        for (var i = 0; i < src.length; i++)
            if (src[i] && !src[i].noDisplay) out.push(src[i]);
        return out;
    }

    function mapCategory(raw) {
        const order = [
            ["TerminalEmulator", I18n.tr("Terminal")], ["WebBrowser", I18n.tr("Browser")],
            ["InstantMessaging", I18n.tr("Chat")], ["Audio", I18n.tr("Media")], ["AudioVideo", I18n.tr("Media")],
            ["Video", I18n.tr("Media")], ["Game", I18n.tr("Game")], ["Development", I18n.tr("Dev")],
            ["Graphics", I18n.tr("Graphics")], ["Office", I18n.tr("Office")], ["Settings", I18n.tr("System")],
            ["System", I18n.tr("System")], ["Utility", I18n.tr("Tool")], ["Network", I18n.tr("Net")]
        ];
        const cats = String(raw).split(/[;,]/);
        for (let i = 0; i < order.length; i++)
            if (cats.includes(order[i][0]))
                return order[i][1];
        return I18n.tr("App");
    }

    function executeEntry(entryId) {
        var entry = DesktopEntries.byId(entryId);
        if (!entry)
            return;
        Frecency.bump(entryId);
        AppLaunch.run(entry, null);
    }

    function executeDesktopAction(entryId, sourceIndex, desktopId) {
        var entry = DesktopEntries.byId(entryId);
        if (!entry)
            return;
        var source = entry.actions;
        if (!source || sourceIndex < 0 || sourceIndex >= source.length)
            return;
        var action = source[sourceIndex];
        if (!action || String(action.id).trim() !== desktopId)
            return;
        Frecency.bump(entryId);
        AppLaunch.run(entry, action);
    }

    function rowFor(entry) {
        var sub = "";
        if (entry.genericName && entry.genericName.length > 0)
            sub = entry.genericName;
        else if (entry.categories && entry.categories.length > 0)
            sub = mapCategory(entry.categories);
        var entryId = entry.id;
        var actions = [{
            id: "launch",
            name: I18n.tr("Launch"),
            icon: "",
            execute: function () { apps.executeEntry(entryId); }
        }];
        var descriptors = AppActions.describeDesktopActions(entry.actions);
        for (var i = 0; i < descriptors.length; i++) {
            var descriptor = descriptors[i];
            actions.push({
                id: descriptor.id,
                name: descriptor.name,
                icon: descriptor.icon,
                execute: (function (appId, sourceIndex, desktopId) {
                    return function () {
                        apps.executeDesktopAction(appId, sourceIndex, desktopId);
                    };
                })(entryId, descriptor.sourceIndex, descriptor.desktopId)
            });
        }
        return {
            id: entryId,
            appId: entryId,
            title: entry.name,
            subtitle: sub,
            icon: entry.icon ? Icons.path(entry.icon, "application-x-executable") : Icons.path("application-x-executable", true),
            type: "App",
            score: 0,
            actions: actions
        };
    }

    function query(text) {
        var ranked = Fuzzy.rank(apps.entries, text, Frecency.usage);
        var rows = [];
        for (var i = 0; i < ranked.length; i++)
            rows.push(rowFor(ranked[i]));
        return rows;
    }

    // Every app as a flat row sorted by name, for the ALL app ledger.
    function allRows() {
        var src = apps.entries.slice().sort(function (a, b) {
            return (a.name || "").toLowerCase().localeCompare((b.name || "").toLowerCase());
        });
        var rows = [];
        for (var i = 0; i < src.length; i++)
            rows.push(rowFor(src[i]));
        return rows;
    }

    Connections {
        target: DesktopEntries
        function onApplicationsChanged() {
            apps.applicationRevision++;
            Dispatcher.notifyAsync();
        }
    }

    Component.onCompleted: Dispatcher.register(apps)
}
