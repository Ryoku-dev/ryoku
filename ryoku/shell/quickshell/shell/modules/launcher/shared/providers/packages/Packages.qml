import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"
import "gpk.js" as Gpk
import "../requeststate.js" as RequestState
import ".."

// Package provider backed by GPK (`gpk search --json`), which spans every package
// manager gpk wraps. Routed by "/" is the actions panel; packages use explicit
// "install "/"remove "/"search " queries (matching inir) so a plain search never
// forks gpk. Search is async + cached; installs/removes spawn a terminal because
// gpk needs a tty for the privilege prompt (see gpk-launcher-backend-notes).
Provider {
    id: packages

    providerId: "packages"
    prefix: ">"
    defaultProvider: false

    property bool available: false
    onAvailableChanged: {
        packages.cachedQuery = "";
        packages.cachedRows = [];
        packages.fullFor = "";
        packages.clearRequest();
        Dispatcher.notifyAsync();
    }
    property string cachedQuery: ""
    property var cachedRows: []
    property string pendingQuery: ""
    property int pendingGeneration: 0
    property bool debounceReady: false
    property var requestState: RequestState.initial()
    property bool fastFinished: false
    property bool fullFinished: false
    property bool fullSucceeded: false

    readonly property string terminal: "kitty"

    function setRequestState(next) {
        if (next === packages.requestState)
            return false;
        var wasBusy = RequestState.isBusy(packages.requestState);
        var nowBusy = RequestState.isBusy(next);
        packages.requestState = next;
        if (wasBusy !== nowBusy)
            Dispatcher.setBusy("packages", nowBusy);
        return true;
    }

    function cancelProcesses() {
        if (fastProc.inFlight)
            fastProc.running = false;
        if (searchProc.inFlight)
            searchProc.running = false;
    }

    function clearRequest() {
        var next = RequestState.clear(packages.requestState);
        if (next === packages.requestState)
            return;
        packages.setRequestState(next);
        debounce.stop();
        packages.debounceReady = false;
        packages.cancelProcesses();
    }

    function invalidateSearch() {
        packages.cachedQuery = "";
        packages.cachedRows = [];
        packages.fullFor = "";
        packages.clearRequest();
    }

    function adoptCached(key) {
        var next = RequestState.adoptSettled(
            packages.requestState, key);
        if (next === packages.requestState)
            return;
        packages.setRequestState(next);
        debounce.stop();
        packages.debounceReady = false;
        packages.cancelProcesses();
    }

    function schedule(key) {
        var next = RequestState.begin(packages.requestState, key);
        if (next === packages.requestState)
            return;
        packages.setRequestState(next);
        packages.pendingQuery = key;
        packages.pendingGeneration = next.generation;
        packages.debounceReady = false;
        debounce.restart();
        packages.cancelProcesses();
    }

    function startPending() {
        if (!packages.debounceReady
                || fastProc.inFlight || searchProc.inFlight)
            return;
        var next = RequestState.markRunning(
            packages.requestState, packages.pendingQuery,
            packages.pendingGeneration);
        if (next === packages.requestState)
            return;
        packages.debounceReady = false;
        packages.setRequestState(next);
        packages.fullFor = "";
        packages.fastFinished = false;
        packages.fullFinished = false;
        packages.fullSucceeded = false;

        fastProc.term = packages.pendingQuery;
        fastProc.requestGeneration = packages.pendingGeneration;
        fastProc.out = "";
        fastProc.inFlight = true;
        fastProc.didStart = false;
        searchProc.term = packages.pendingQuery;
        searchProc.requestGeneration = packages.pendingGeneration;
        searchProc.out = "";
        searchProc.inFlight = true;
        searchProc.didStart = false;
        fastProc.running = true;
        searchProc.running = true;
    }

    function finishRequest(key, generation) {
        if (!packages.fastFinished || !packages.fullFinished)
            return;
        packages.setRequestState(RequestState.settle(
            packages.requestState, key, generation));
    }

    // text after ">": "search x" / "install x" / "remove x", or a bare query that
    // defaults to search. So ">yay" searches, ">install yay" installs.
    function parseOp(text) {
        var m = String(text || "").match(/^(install|remove|search)\s+(.+)$/i);
        if (m)
            return { op: m[1].toLowerCase(), term: m[2].trim() };
        var t = String(text || "").trim();
        // a bare verb mid-typing (">install ") is an incomplete command, not
        // a search for the literal word; searching it flashed unrelated rows.
        if (/^(install|remove|search)$/i.test(t))
            return null;
        return t.length ? { op: "search", term: t } : null;
    }

    function rowFor(pkg, op) {
        var verb = op === "remove" ? I18n.tr("Remove") : (pkg.installed ? I18n.tr("Reinstall") : I18n.tr("Install"));
        return {
            id: "pkg:" + pkg.source + ":" + pkg.name,
            title: pkg.name + "  " + pkg.version,
            subtitle: pkg.source + (pkg.installed ? "  " + I18n.tr("(installed)") : "") + "  " + pkg.description,
            icon: "",
            type: "Package",
            score: 0,
            actions: [{
                id: op === "remove" ? "remove" : "install",
                name: verb,
                icon: "",
                execute: function () {
                    var gpkOp = op === "remove" ? "remove" : "install";
                    Spawn.run([packages.terminal, "-e", "gpk", gpkOp, pkg.name]);
                    // the install/remove will change what a re-search should
                    // show; drop the cached rows so the next query refetches.
                    packages.invalidateSearch();
                }
            }]
        };
    }

    function query(text) {
        if (!packages.available) {
            packages.clearRequest();
            return [];
        }
        var p = parseOp(text);
        if (!p || p.term.length < 2) {
            packages.clearRequest();
            return [];
        }
        if (p.term === packages.cachedQuery) {
            packages.adoptCached(p.term);
            return packages.cachedRows.map(function (pkg) { return packages.rowFor(pkg, p.op); });
        }
        packages.schedule(p.term);
        return [];
    }

    Timer {
        id: debounce
        interval: 120
        repeat: false
        onTriggered: {
            packages.debounceReady = true;
            packages.startPending();
        }
    }

    // Fast lane: the host's native package manager answers from gpk's scan
    // cache in well under a second, while the full sweep hits live registries
    // (npm, cargo, pip...) and can take tens of seconds. Local rows show
    // immediately; the full set replaces them when it lands. fullFor marks a
    // term the full sweep has already answered so a slow fast-lane result never
    // regresses it.
    property string fullFor: ""

    Process {
        id: fastProc
        property string term: ""
        property int requestGeneration: 0
        property bool inFlight: false
        property bool didStart: false
        property string out: ""
        command: ["/usr/bin/bash", "-c",
            "manager=$(ryoku-host pkgmgr 2>/dev/null); " +
            "if [ \"$manager\" = dnf ]; then manager=dnf; else manager=pacman,aur; fi; " +
            "exec gpk search \"$1\" --json --limit 30 --manager \"$manager\"",
            "gpk-fast-search", term]
        stdout: SplitParser {
            onRead: data => fastProc.out += data + "\n"
        }
        onStarted: fastProc.didStart = true
        onRunningChanged: {
            if (!running && fastProc.inFlight && !fastProc.didStart) {
                var key = fastProc.term;
                var generation = fastProc.requestGeneration;
                fastProc.inFlight = false;
                if (packages.requestState.phase === "running"
                        && RequestState.isCurrent(
                            packages.requestState, key, generation)) {
                    packages.fastFinished = true;
                    packages.finishRequest(key, generation);
                }
                packages.startPending();
            }
        }
        onExited: (code, status) => {
            var key = fastProc.term;
            var generation = fastProc.requestGeneration;
            fastProc.inFlight = false;
            var current = packages.requestState.phase === "running"
                && RequestState.isCurrent(
                    packages.requestState, key, generation);
            if (current) {
                packages.fastFinished = true;
                var succeeded = status === 0 && code !== 1;
                if (succeeded && !packages.fullSucceeded
                        && packages.fullFor !== key) {
                    packages.cachedQuery = key;
                    packages.cachedRows = Gpk.parse(fastProc.out);
                    Dispatcher.notifyAsync();
                }
                packages.finishRequest(key, generation);
            }
            packages.startPending();
        }
    }

    Process {
        id: availProc
        command: ["gpk", "search", "--help"]
        onExited: (code) => { packages.available = (code === 0); }
    }

    Process {
        id: searchProc
        property string term: ""
        property int requestGeneration: 0
        property bool inFlight: false
        property bool didStart: false
        property string out: ""
        command: ["gpk", "search", term, "--json", "--limit", "30"]
        stdout: SplitParser {
            onRead: data => searchProc.out += data + "\n"
        }
        onStarted: searchProc.didStart = true
        onRunningChanged: {
            if (!running && searchProc.inFlight
                    && !searchProc.didStart) {
                var key = searchProc.term;
                var generation = searchProc.requestGeneration;
                searchProc.inFlight = false;
                if (packages.requestState.phase === "running"
                        && RequestState.isCurrent(
                            packages.requestState, key, generation)) {
                    packages.fullFinished = true;
                    packages.fullSucceeded = false;
                    packages.finishRequest(key, generation);
                }
                packages.startPending();
            }
        }
        onExited: (code, status) => {
            var key = searchProc.term;
            var generation = searchProc.requestGeneration;
            searchProc.inFlight = false;
            var current = packages.requestState.phase === "running"
                && RequestState.isCurrent(
                    packages.requestState, key, generation);
            if (current) {
                packages.fullFinished = true;
                packages.fullSucceeded = status === 0 && code !== 1;
                if (packages.fullSucceeded) {
                    packages.fullFor = key;
                    packages.cachedQuery = key;
                    packages.cachedRows = Gpk.parse(searchProc.out);
                    Dispatcher.notifyAsync();
                }
                packages.finishRequest(key, generation);
            }
            packages.startPending();
        }
    }

    Component.onCompleted: {
        availProc.running = true;
        Dispatcher.register(packages);
    }
}
