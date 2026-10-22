pragma Singleton

import caelestia.services.api
import QtQuick
import QtCore
import Quickshell
import Quickshell.Io
import Caelestia
import Caelestia.Services

Item {
    id: storeRoot

    property bool loading: false
    property bool error: false
    property string errorMessage: ""

    property bool installing: false
    property string installProgress: ""
    property string installError: ""

    // id of the plugin currently being installed, for per-row progress feedback
    property string installingId: ""
    property var updatesAvailable: []

    property bool restartRequired: false

    property var installedPluginIds: []
    property bool baselineLoaded: false

    property var indexData: null
    property ListModel storePlugins: ListModel {}

    signal indexFetched()
    signal installedStateChanged()

    function compareVersions(a, b) {
        const pa = String(a || "").split(".");
        const pb = String(b || "").split(".");
        const len = Math.max(pa.length, pb.length);
        for (let i = 0; i < len; i++) {
            const na = parseInt(pa[i], 10);
            const va = isNaN(na) ? 0 : na;
            const nb = parseInt(pb[i], 10);
            const vb = isNaN(nb) ? 0 : nb;
            if (va !== vb)
                return va < vb ? -1 : 1;
        }
        return 0;
    }

    function checkForUpdates() {
        const installed = {};
        for (let i = 0; i < CaelestiaApi.plugins.available.count; i++) {
            const p = CaelestiaApi.plugins.available.get(i);
            installed[p.id || p.name] = p.version || "";
        }
        const updates = [];
        for (let i = 0; i < storePlugins.count; i++) {
            const sp = storePlugins.get(i);
            const id = sp.pluginId || sp.id;
            if (id && installed[id] !== undefined && compareVersions(installed[id], sp.version) < 0)
                updates.push(id);
        }
        updatesAvailable = updates;
        if (updates.length === 1) {
            Toaster.toast(qsTr("Plugin update available"), qsTr("1 plugin can be updated"), "update");
        } else if (updates.length > 1) {
            Toaster.toast(qsTr("Plugin updates available"), qsTr("%1 plugins can be updated").arg(updates.length), "update");
        }
    }

    function fetchIndex(branch) {
        let fetchBranch = branch || "main";
        loading = true;
        error = false;
        errorMessage = "";

        // Seed installed IDs from disk baseline if not done yet
        // (handles the race condition where fetchIndex resolves before PluginLoader's pluginsReloaded)
        if (!baselineLoaded && CaelestiaApi.plugins.available.count > 0) {
            baselineLoaded = true;
            let ids = [];
            let av = CaelestiaApi.plugins.available;
            for (let i = 0; i < av.count; i++)
                ids.push(av.get(i).id);
            installedPluginIds = ids;
            console.log("PluginStore: baseline seeded in fetchIndex:", JSON.stringify(ids));
        }

        fetchProc.command = ["curl", "-sfL", "https://raw.githubusercontent.com/ladybug-me/caelestia-kde-plugins/" + fetchBranch + "/index.json"];
        fetchProc.running = true;
    }

    // Store ids come from a remote index and become filesystem paths, so only
    // plain directory names are accepted.
    function isValidPluginId(id) {
        return typeof id === "string" && /^[A-Za-z0-9][A-Za-z0-9._-]*$/.test(id) && !id.includes("..");
    }

    function installPlugin(id, repoPath, branch, restart) {
        if (!isValidPluginId(id)) {
            console.warn("PluginStore: refusing to install plugin with unsafe id:", id);
            return;
        }

        let installBranch = branch || "main";

        // repoPath must be a relative path inside the store repo. Installed plugins
        // report an absolute *install* dir as their path, which must never be used
        // here - it would make the clone step target itself and wipe the plugin.
        let actualRepoPath = (typeof repoPath === "string" && repoPath !== "" && !repoPath.startsWith("/") && !repoPath.includes("..")) ? repoPath : ("plugins/" + id);

        installing = true;
        installingId = id;
        installProgress = "Cloning plugin '" + id + "'...";

        let targetDir = (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/caelestia/plugins/" + id;

        let script = `set -e
TMP_DIR=$(mktemp -d)
cd "$TMP_DIR"
git init -q
git remote add origin https://github.com/ladybug-me/caelestia-kde-plugins.git
git config core.sparseCheckout true
echo "$2/*" >> .git/info/sparse-checkout
git fetch -q --depth 1 --filter=blob:none origin "$3"
git reset --hard -q "origin/$3"
test -d "$2"
mkdir -p "$(dirname "$4")"
rm -rf "$4"
mv "$2" "$4"
rm -rf "$TMP_DIR"
echo "DONE"`;

        installProc.pendingId = id;
        installProc.pendingTargetDir = targetDir;
        installProc.pendingRestart = (restart === "true" || restart === true);
        installProc.command = ["bash", "-c", script, "--", id, actualRepoPath, installBranch, targetDir];
        installProc.running = true;
    }

    function removePlugin(id) {
        if (!isValidPluginId(id)) {
            console.warn("PluginStore: refusing to remove plugin with unsafe id:", id);
            return;
        }
        let targetDir = (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/caelestia/plugins/" + id;
        removeProc.pendingId = id;
        let requiresRestart = false;
        for (let i = 0; i < CaelestiaApi.plugins.available.count; i++) {
            let p = CaelestiaApi.plugins.available.get(i);
            if ((p.id || p.name) === id) {
                requiresRestart = (p.restart === "true" || p.restart === true);
                break;
            }
        }
        removeProc.pendingRestart = requiresRestart;
        removeProc.command = ["rm", "-rf", targetDir];
        removeProc.running = true;
    }

    Connections {
        target: PluginLoader

        function onPluginsReloaded() {
            if (storeRoot.baselineLoaded) {
                if (storeRoot.indexData)
                    storeRoot.checkForUpdates();
                return;
            }
            storeRoot.baselineLoaded = true;
            let ids = [];
            let av = CaelestiaApi.plugins.available;
            for (let i = 0; i < av.count; i++)
                ids.push(av.get(i).id);
            storeRoot.installedPluginIds = ids;
            console.log("PluginStore: baseline loaded, installed:", JSON.stringify(ids));
        }
    }

    Process {
        id: fetchProc

        stdout: StdioCollector { id: fetchOut }
        stderr: StdioCollector { id: fetchErr }

        onExited: (code) => {
            storeRoot.loading = false;
            if (code !== 0) {
                storeRoot.error = true;
                storeRoot.errorMessage = "Failed to fetch plugins index (curl exit " + code + "): " + fetchErr.text;
            } else {
                try {
                    storeRoot.indexData = JSON.parse(fetchOut.text);
                    storeRoot.storePlugins.clear();
                    let plugins = storeRoot.indexData.plugins || [];
                    for (let i = 0; i < plugins.length; i++) {
                        let p = plugins[i];
                        p.pluginId = p.id;
                        p.path = p.path || ("plugins/" + p.id);
                        p.mediaurl = p.mediaurl || "";
                        p.authorName = p.author ? (p.author.name || "") : "";
                        let aUrl = p.author ? (p.author.url || "") : "";
                        p.icon = p.icon || "extension";
                        p.restart = (p.restart === "true" || p.restart === true);
                        storeRoot.storePlugins.append(p);
                    }
                    storeRoot.indexFetched();
                    storeRoot.checkForUpdates();
                } catch (e) {
                    storeRoot.error = true;
                    storeRoot.errorMessage = "Failed to parse index JSON: " + e;
                }
            }
        }
    }


    function installFromUrl(url) {
        if (typeof url !== "string" || (url.indexOf("https://") !== 0 && url.indexOf("git@") !== 0)) {
            installError = qsTr("Enter a valid git URL");
            return;
        }

        installing = true;
        installError = "";
        installProgress = qsTr("Cloning plugin...");

        const targetBase = (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/caelestia/plugins";
        const script = `set -e
TMP_DIR=$(mktemp -d)
git clone -q --depth 1 "$1" "$TMP_DIR"
ID=$(jq -r .id "$TMP_DIR/metadata.json")
if ! [[ "$ID" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]] || [[ "$ID" == *..* ]]; then echo "invalid plugin id: $ID" >&2; rm -rf "$TMP_DIR"; exit 1; fi
mkdir -p "$2"
rm -rf "$2/$ID"
mv "$TMP_DIR" "$2/$ID"
echo "INSTALLED:$ID:$2/$ID"`;

        sourceProc.command = ["bash", "-c", script, "--", url, targetBase];
        sourceProc.running = true;
    }

    Process {
        id: sourceProc

        stdout: StdioCollector { id: sourceOut }
        stderr: StdioCollector { id: sourceErr }

        onExited: (code) => {
            storeRoot.installing = false;
            if (code !== 0) {
                const err = (sourceErr.text || sourceOut.text || "").trim().split("\n");
                storeRoot.installError = err[err.length - 1] || qsTr("Install failed");
                console.log("PluginStore: source install error:", sourceErr.text, sourceOut.text);
            } else {
                const match = /INSTALLED:([^:]+):(.+)/.exec(sourceOut.text || "");
                if (!match) {
                    storeRoot.installError = qsTr("Install failed");
                    return;
                }
                console.log("PluginStore: source install success for", match[1]);
                storeRoot.restartRequired = true;
                let ids = storeRoot.installedPluginIds.slice();
                if (ids.indexOf(match[1]) === -1)
                    ids.push(match[1]);
                storeRoot.installedPluginIds = ids;
                PluginLoader.addPluginToAvailable(match[1], match[2], "user");
            }
        }
    }

    Process {
        id: installProc

        property string pendingId: ""
        property string pendingTargetDir: ""
        property bool pendingRestart: false

        stdout: StdioCollector { id: installOut }
        stderr: StdioCollector { id: installErr }

        onExited: (code) => {
            storeRoot.installing = false;
            storeRoot.installingId = "";
            if (code !== 0) {
                console.log("PluginStore: install error for", installProc.pendingId, ":", installErr.text, installOut.text);
            } else {
                console.log("PluginStore: install success for", installProc.pendingId);
                if (installProc.pendingRestart)
                    storeRoot.restartRequired = true;
                let ids = storeRoot.installedPluginIds.slice();
                if (ids.indexOf(installProc.pendingId) === -1)
                    ids.push(installProc.pendingId);
                storeRoot.installedPluginIds = ids;
                console.log("PluginStore: installedPluginIds now:", JSON.stringify(ids));
                PluginLoader.addPluginToAvailable(installProc.pendingId, installProc.pendingTargetDir, "user");
            }
        }
    }

    Process {
        id: removeProc

        property string pendingId: ""
        property bool pendingRestart: false

        onExited: (code) => {
            if (removeProc.pendingRestart)
                storeRoot.restartRequired = true;
            let ids = storeRoot.installedPluginIds.slice();
            let idx = ids.indexOf(removeProc.pendingId);
            if (idx !== -1) ids.splice(idx, 1);
            storeRoot.installedPluginIds = ids;
            console.log("PluginStore: removed", removeProc.pendingId, "installedPluginIds now:", JSON.stringify(ids));
            PluginLoader.removePluginFromAvailable(removeProc.pendingId);
        }
    }
}
