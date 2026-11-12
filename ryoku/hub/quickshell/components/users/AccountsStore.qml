import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"

// the people on this computer and their account profiles, read from ryoku-host;
// a Changes store so role and permission changes get undo from any page.
// keys: "role/<name>" and "perm/<name>/<permission id>"
Scope {
    id: store

    property var users: []
    property string self: ""
    property bool listLoaded: false
    property string loadError: ""
    property var permissionDefs: []
    property var storageCaps: ({ subvolumes: false, quota: false, reason: "" })
    property bool capsLoaded: false
    property var profileDefs: []
    property var profileAccounts: []
    property bool profilesLoaded: false
    // search and the schema gates read whether this is a profile account once both reads are in
    onProfilesLoadedChanged: Changes.touch()
    // values on screen while their write runs
    property var optimistic: ({})
    // writes waiting their turn: { key, command, secret, done }
    property var queue: []
    property var current: null
    property int pictureRevision: 0
    property bool pictureBusy: pictureJob.running
    property string pictureError: ""

    readonly property bool supported: Host.supports("users")
    readonly property bool working: job.running
    readonly property bool slow: job.slow
    readonly property bool lengthy: job.lengthy
    readonly property string runningKey: store.current ? store.current.key : ""
    readonly property string dataRoot: Quickshell.env("XDG_DATA_HOME") || ((Quickshell.env("HOME") || "/") + "/.local/share")
    readonly property string stateRoot: Quickshell.env("XDG_STATE_HOME") || ((Quickshell.env("HOME") || "/") + "/.local/state")

    readonly property var selfAccount: store.users.find(u => String(u.name || "") === store.self) || null
    readonly property var selfProfile: store.profileOf(store.self)
    readonly property bool inProfileAccount: store.selfProfile !== null
    readonly property bool canManage: !store.inProfileAccount && store.selfAccount !== null && store.selfAccount.role === "admin"
    readonly property int adminCount: store.users.filter(u => u.role === "admin").length
    readonly property var installedProfiles: store.profileDefs.filter(d => d.installed === true)

    function user(name) {
        return store.users.find(u => String(u.name || "") === String(name || "")) || null;
    }

    function profileOf(name) {
        return store.profileAccounts.find(p => String(p.account || "") === String(name || "")) || null;
    }

    function profileDef(id) {
        return store.profileDefs.find(d => String(d.id || "") === String(id || "")) || null;
    }

    function profileName(id) {
        var d = store.profileDef(id);
        if (d)
            return String(d.name || d.id);
        var text = String(id || "").replace(/-/g, " ");
        return text.length > 0 ? text.charAt(0).toUpperCase() + text.slice(1) : I18n.tr("Profile");
    }

    // Changes store

    function read(key) {
        // a profile account sees only its own panel; unknown until the list and profiles are read
        if (key === "inProfileAccount")
            return store.listLoaded && store.profilesLoaded ? store.inProfileAccount : undefined;
        if (store.optimistic[key] !== undefined)
            return store.optimistic[key];
        var parts = String(key).split("/");
        var u = store.user(parts[1]);
        if (!u)
            return undefined;
        if (parts[0] === "role")
            return String(u.role || "standard");
        if (parts[0] === "perm")
            return (u.permissions || []).indexOf(parts[2]) >= 0;
        return undefined;
    }

    function write(key, v, done) {
        var parts = String(key).split("/");
        var name = parts[1];
        var command = null;
        if (parts[0] === "role") {
            command = ["pkexec", "ryoku-host", "users", "set-role", name, String(v)];
        } else if (parts[0] === "perm") {
            // the full list, built when its turn comes so queued toggles add up
            command = function () {
                var u = store.user(name);
                var have = (u ? u.permissions || [] : []).filter(p => p !== "admin" && p !== parts[2]);
                for (var k in store.optimistic) {
                    var kp = k.split("/");
                    if (kp[0] === "perm" && kp[1] === name && kp[2] !== parts[2] && store.optimistic[k] !== undefined) {
                        have = have.filter(p => p !== kp[2]);
                        if (store.optimistic[k] === true)
                            have.push(kp[2]);
                    }
                }
                if (v === true)
                    have.push(parts[2]);
                if (u && u.role === "admin")
                    have.unshift("admin");
                return ["pkexec", "ryoku-host", "users", "set-perms", name, have.join(",")];
            };
        }
        if (!command) {
            done(false, I18n.tr("This change is not supported."));
            return;
        }
        var o = Object.assign({}, store.optimistic);
        o[key] = v;
        store.optimistic = o;
        Changes.touch();
        store.enqueue({ key: key, command: command, done: done });
    }

    // a change that is not a setting: add, password, limit, remove
    function run(command, secret, done) {
        store.enqueue({ key: "", command: command, secret: secret, done: done });
    }

    function enqueue(entry) {
        store.queue = store.queue.concat([entry]);
        store.next();
    }

    function next() {
        if (store.current || store.queue.length === 0)
            return;
        var entry = store.queue[0];
        store.queue = store.queue.slice(1);
        store.current = entry;
        var command = typeof entry.command === "function" ? entry.command() : entry.command;
        job.start(command, entry.secret);
    }

    function cancel() {
        job.cancel();
    }

    function settle(ok, message) {
        var entry = store.current;
        store.current = null;
        if (entry && entry.key !== "") {
            // an applied value stays on screen until the list read after it confirms it
            if (ok)
                store.awaiting = store.awaiting.concat([entry.key]);
            else if (!store.queued(entry.key))
                store.dropOptimistic([entry.key]);
        }
        if (entry && entry.done)
            entry.done(ok, message);
        Changes.touch();
        store.refresh();
        store.next();
    }

    property var awaiting: []

    function queued(key) {
        return store.queue.some(e => e.key === key) || (store.current !== null && store.current.key === key);
    }

    function dropOptimistic(keys) {
        var o = Object.assign({}, store.optimistic);
        keys.forEach(k => {
            if (!store.queued(k))
                delete o[k];
        });
        store.optimistic = o;
    }

    Job {
        id: job
        onFinished: (ok, message) => store.settle(ok, message)
    }

    // reading

    function refresh() {
        if (!store.supported)
            return;
        if (listProc.running) {
            store.refreshAgain = true;
            return;
        }
        store.watchReads();
        listProc.running = true;
    }
    property bool refreshAgain: false

    function refreshProfiles() {
        store.watchReads();
        if (!discoverProc.running)
            discoverProc.running = true;
        if (!statusProc.running)
            statusProc.running = true;
    }

    function start() {
        store.refreshProfiles();
        if (!store.supported)
            return;
        store.refresh();
        if (!store.capsLoaded && !capsProc.running)
            capsProc.running = true;
    }

    // a read that hangs is stopped after 10 s and ends in an error with Retry, never a spinner forever
    property bool readTimedOut: false
    function watchReads() {
        store.readTimedOut = false;
        readWatch.restart();
    }
    function readFailed(fallback) {
        return store.readTimedOut ? I18n.tr("Reading the accounts took too long. Retry, and if it keeps failing, run ryoku doctor.") : fallback;
    }
    Timer {
        id: readWatch
        interval: 10000
        onTriggered: {
            var hung = [listProc, capsProc, statusProc, discoverProc].filter(p => p.running);
            if (hung.length === 0)
                return;
            store.readTimedOut = true;
            hung.forEach(p => p.running = false);
        }
    }

    function parsed(text) {
        var v = JSON.parse(String(text || "{}"));
        if (v && v.error)
            throw new Error(String(v.error));
        return v;
    }

    Process {
        id: listProc
        command: ["ryoku-host", "users", "list"]
        environment: Spawn.env
        stdout: StdioCollector { id: listOut }
        onExited: code => {
            try {
                if (code !== 0)
                    throw new Error(store.readFailed(I18n.tr("Could not read the accounts on this computer. Retry, and if it keeps failing, run ryoku doctor.")));
                var v = store.parsed(listOut.text);
                store.self = String(v.self || "");
                store.users = Array.isArray(v.users) ? v.users : [];
                store.listLoaded = true;
                store.loadError = "";
            } catch (e) {
                store.loadError = String(e.message || e);
            }
            if (store.refreshAgain) {
                store.refreshAgain = false;
                store.refresh();
                return;
            }
            store.dropOptimistic(store.awaiting);
            store.awaiting = [];
            Changes.touch();
        }
    }

    Process {
        id: capsProc
        command: ["ryoku-host", "users", "caps"]
        environment: Spawn.env
        stdout: StdioCollector { id: capsOut }
        onExited: code => {
            try {
                if (code !== 0)
                    throw new Error(store.readFailed(I18n.tr("Could not read what accounts may be given here. Retry, and if it keeps failing, run ryoku doctor.")));
                var v = store.parsed(capsOut.text);
                store.permissionDefs = Array.isArray(v.permissions) ? v.permissions.filter(d => String(d.id || "") !== "admin") : [];
                store.storageCaps = v.storage || ({ subvolumes: false, quota: false, reason: "" });
                store.capsLoaded = true;
            } catch (e) {
                store.loadError = String(e.message || e);
            }
        }
    }

    // staged definitions under /var/lib, then the ones installed from RyoStore
    Process {
        id: discoverProc
        command: ["sh", "-c", [
            "emit() { kind=$1; manifest=$2; source=${manifest%/profile.json};",
            "  printf '%s\\t%s\\t' \"$kind\" \"$source\"",
            "  tr -d '\\n\\r' < \"$manifest\"",
            "  printf '\\n'",
            "}",
            "for manifest in /var/lib/ryoku/profiles/*/payload/*/profile.json; do",
            "  [ -f \"$manifest\" ] && emit staged \"$manifest\"",
            "done",
            "for manifest in \"$1\"/ryoku/profiles/*/profile.json; do",
            "  [ -f \"$manifest\" ] || continue",
            "  id=${manifest%/profile.json}; id=${id##*/}",
            "  [ -f \"$2/ryoku/store/profiles/$id.json\" ] && emit installed \"$manifest\"",
            "done",
            "exit 0"
        ].join("\n"), "sh", store.dataRoot, store.stateRoot]
        environment: Spawn.env
        property var found: []
        stdout: SplitParser {
            splitMarker: "\n"
            onRead: data => discoverProc.found = discoverProc.found.concat([String(data)])
        }
        onStarted: discoverProc.found = []
        onExited: code => {
            var defs = [];
            discoverProc.found.forEach(line => {
                var a = line.indexOf("\t"), b = a >= 0 ? line.indexOf("\t", a + 1) : -1;
                if (a < 0 || b < 0)
                    return;
                var manifest;
                try {
                    manifest = JSON.parse(line.slice(b + 1));
                } catch (e) {
                    return;
                }
                if (!manifest || !manifest.id)
                    return;
                var row = { id: String(manifest.id), name: String(manifest.name || manifest.id), version: String(manifest.version || ""),
                            source: line.slice(a + 1, b), installed: line.slice(0, a) === "installed", manifest: manifest };
                var at = defs.findIndex(d => d.id === row.id);
                if (at < 0)
                    defs.push(row);
                else if (!(defs[at].installed && !row.installed))
                    defs[at] = row;
            });
            store.profileDefs = defs;
            store.profilesLoaded = !statusProc.running;
            store.profilesFound();
        }
    }

    signal profilesFound()

    Process {
        id: statusProc
        command: ["ryoku-host", "profile", "status"]
        environment: Spawn.env
        stdout: StdioCollector { id: statusOut }
        onExited: code => {
            try {
                if (code !== 0)
                    throw new Error(store.readFailed(I18n.tr("Could not read the profile accounts. Retry, and if it keeps failing, run ryoku doctor.")));
                var v = store.parsed(statusOut.text);
                store.profileAccounts = Array.isArray(v.accounts) ? v.accounts : [];
            } catch (e) {
                store.loadError = String(e.message || e);
            }
            store.profilesLoaded = !discoverProc.running;
        }
    }

    // your picture: ~/.face, which any account may write, then AccountsService when it runs
    Job {
        id: pictureJob
        onFinished: (ok, message) => {
            store.pictureError = ok ? "" : message;
            store.pictureRevision++;
        }
    }

    function setPicture(path) {
        store.pictureError = "";
        pictureJob.start(["sh", "-c", [
            'src=$1; face="$HOME/.face"; tmp="$face.new"',
            'if command -v magick >/dev/null 2>&1; then',
            '  magick "$src[0]" -auto-orient -thumbnail 512x512^ -gravity center -extent 512x512 -strip "PNG:$tmp" || { echo "$2" >&2; exit 1; }',
            'else cp -- "$src" "$tmp" || { echo "$2" >&2; exit 1; }; fi',
            '[ "$(stat -c %s "$tmp")" -gt 1000000 ] && command -v magick >/dev/null 2>&1 && magick "$tmp" -quality 90 "JPEG:$tmp"',
            'mv -f -- "$tmp" "$face" || exit 1',
            'busctl --system call org.freedesktop.Accounts "/org/freedesktop/Accounts/User$(id -u)" org.freedesktop.Accounts.User SetIconFile s "$face" >/dev/null 2>&1 || true'
        ].join("\n"), "sh", path, I18n.tr("That file is not a picture this computer can read. Choose a PNG, JPEG or WebP image.")]);
    }

    function removePicture() {
        store.pictureError = "";
        pictureJob.start(["sh", "-c", [
            'rm -f -- "$HOME/.face" "$HOME/.face.icon"',
            'busctl --system call org.freedesktop.Accounts "/org/freedesktop/Accounts/User$(id -u)" org.freedesktop.Accounts.User SetIconFile s "" >/dev/null 2>&1 || true'
        ].join("\n")]);
    }

    Component.onCompleted: {
        store.start();
        Changes.registerStore("users", store);
    }

    Connections {
        target: Host
        function onReadyChanged() { store.start(); }
        function onUsersChanged() { store.start(); }
    }
}
