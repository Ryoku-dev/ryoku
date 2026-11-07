import QtQuick
import Quickshell
import Quickshell.Io
import shell.services
import "shared/Singletons"
import "shared/providers" as SharedProviders
import "shared/lib/catalog.js" as Catalog

Scope {
    id: root

    property var screen: null
    property bool active: false

    // A genuine dismiss (click-out, Esc, or launching an app) hides the variant
    // itself; the controller resets launcherOpen so the flag never goes stale
    // (a stale flag needs a second Super+Space to reopen).
    signal requestClose()

    onActiveChanged: root.syncOpen()
    onVariantReadyChanged: root.syncOpen()

    function syncOpen() {
        if (!variantReady)
            return;
        if (active)
            show(screen && screen.name ? screen.name : "");
        else
            hide();
    }

    // The shipped catalog.json joined with the launchers RyoStore installed,
    // once both are read: a Store pick then loads first instead of after the
    // default.
    property var builtinCatalog: null
    readonly property var catalog: builtinCatalog && LauncherProducts.ready
        ? Catalog.withStore(builtinCatalog, LauncherProducts.rows, LauncherProducts.stateRoot)
        : null
    readonly property var launcherContributions: Shells.contributions("launcher")
    property string activeId: ""
    // The URL the active variant runs from: a Store update keeps the id and
    // moves the view, so a changed URL is a switch too.
    property string activeSource: ""
    property string requestedId: LauncherConfig.variant
    property string pendingId: ""
    property bool fallbackTried: false
    property bool switchQueued: false
    readonly property bool variantReady:
        variantLoader.status === Loader.Ready && variantLoader.item !== null

    function selectEntry(id) {
        var resolved = Catalog.resolve(catalog, id, launcherContributions);
        if (resolved.pending)
            Shells.requestSync();
        return resolved.entry;
    }

    function sourceOf(entry) {
        return String(entry.url || Qt.resolvedUrl(entry.entrypoint));
    }

    function activate(id) {
        var next = selectEntry(id);
        if (!next)
            return;
        activeId = next.id;
        activeSource = sourceOf(next);
        variantLoader.source = activeSource;
    }

    function requestVariant(id) {
        var next = selectEntry(id);
        if (!next)
            return;
        if (next.id === activeId && sourceOf(next) === activeSource) {
            pendingId = "";
            return;
        }
        pendingId = next.id;
        if (variantReady && variantLoader.item.shown)
            variantLoader.item.hide();
        else
            finishSwitch();
    }

    function finishSwitch() {
        if (!pendingId || switchQueued)
            return;
        switchQueued = true;
        Qt.callLater(function () {
            root.switchQueued = false;
            if (!root.pendingId)
                return;
            var next = root.pendingId;
            root.pendingId = "";
            root.fallbackTried = false;
            root.activate(next);
            // a show that arrived while the switch was pending opens the new one
            if (root.active)
                root.syncOpen();
        });
    }

    function show(mon) {
        if (!pendingId && variantReady)
            variantLoader.item.show(mon);
    }

    function hide() {
        if (variantReady)
            variantLoader.item.hide();
    }

    FileView {
        id: catalogFile
        path: Qt.resolvedUrl("catalog.json")
        blockLoading: true
        printErrors: true
        onLoaded: root.builtinCatalog = Catalog.normalize(JSON.parse(text()))
    }

    onCatalogChanged: {
        if (!catalog)
            return;
        if (activeId === "")
            activate(requestedId);
        else
            requestVariant(requestedId);
    }
    onRequestedIdChanged: if (catalog) requestVariant(requestedId)
    onLauncherContributionsChanged: if (catalog) requestVariant(requestedId)

    Loader {
        id: variantLoader
        asynchronous: false
        onStatusChanged: {
            if (status !== Loader.Error || !root.catalog)
                return;
            var fallback = Catalog.fallbackEntry(root.catalog);
            if (!root.fallbackTried && fallback && root.activeId !== fallback.id) {
                root.fallbackTried = true;
                root.activeId = fallback.id;
                root.activeSource = root.sourceOf(fallback);
                source = root.activeSource;
            }
        }
    }

    Connections {
        target: root.variantReady ? variantLoader.item : null
        function onShownChanged() {
            if (target.shown)
                return;
            // Variant hid: finish a queued style switch, else it was a genuine
            // dismiss, so sync ShellState.launcherOpen back to closed.
            if (root.pendingId)
                root.finishSwitch();
            else
                root.requestClose();
        }
    }

    // Phase 10: the launcher's legacy daemon control channels are removed so this
    // instance stays self-contained. ShellState.launcherOpen (bound to `active`)
    // owns open/close now, replacing the IpcHandler (target "launcher") and the
    // ryoku-launcher.sock SocketServer the old daemon drove for toggle/show/hide/
    // state; the daemon-facing bridge is rebuilt on ShellState in Phase 10.
}
