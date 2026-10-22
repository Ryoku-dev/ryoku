pragma Singleton

import caelestia.services.api
import QtQuick
import QtCore
import Quickshell
import Quickshell.Io

Item {
    id: pluginLoader

    property var pluginInstances: ({})
    property int loadedCount: 0
    property int enabledCount: 0

    property var discovered: []
    property bool loadRequested: false
    property bool finalized: false
    property int pendingMeta: 0

    property Settings pluginSettings: Settings {
        property var disabledPlugins: []

        category: "Plugins"
    }

    signal pluginsReloaded()

    function isCountedPlugin(meta) {
        return meta.enabled && (meta.type === "quickshell" || meta.type === "kwineffect");
    }

    function updateEnabledCount() {
        let count = 0;
        for (let i = 0; i < discovered.length; i++) {
            if (isCountedPlugin(discovered[i]))
                count++;
        }
        enabledCount = count;
    }

    function loadPlugin(meta) {
        let id = meta.id || meta.name;
        if (pluginInstances[id]) return;
        // Other types (a kwineffect that ships a settings front-end) opt in
        // through the manifest's `ui` field.
        let ui = meta.ui || (meta.type === "quickshell" ? "main.qml" : "");
        if (!ui) return;

        let mainFile = meta.path + "/" + ui;
        let component = Qt.createComponent("file://" + mainFile);

        let finishLoad = () => {
            if (component.status === Component.Ready) {
                let obj = component.createObject(pluginLoader);
                if (obj !== null) {
                    pluginInstances[id] = obj;
                    pluginInstances = pluginInstances;
                    loadedCount = Object.keys(pluginInstances).length;
                } else {
                    console.log("Plugin createObject failed: " + mainFile);
                }
            } else if (component.status === Component.Error) {
                console.log("Plugin compile error: " + mainFile + "\n" + component.errorString());
            }
        };

        if (component.status === Component.Loading) {
            component.statusChanged.connect(finishLoad);
        } else {
            finishLoad();
        }
    }

    function unloadPlugin(id) {
        if (pluginInstances[id]) {
            pluginInstances[id].destroy();
            delete pluginInstances[id];
            pluginInstances = pluginInstances;
            loadedCount = Object.keys(pluginInstances).length;
            console.log("Plugin unloaded: " + id);
        }
    }

    function setPluginEnabled(name, enable) {
        // Must slice() to create a new array instance, otherwise assigning it back
        // to pluginSettings.disabledPlugins won't trigger the QML change signal or save it.
        let disabled = (pluginSettings.disabledPlugins || []).slice();
        let index = disabled.indexOf(name);
        if (enable && index > -1) {
            disabled.splice(index, 1);
            pluginSettings.disabledPlugins = disabled;
        } else if (!enable && index === -1) {
            disabled.push(name);
            pluginSettings.disabledPlugins = disabled;
        }

        let av = CaelestiaApi.plugins.available;
        for (let i = 0; i < av.count; i++) {
            let item = av.get(i);
            let itemId = item.id || item.name;
            if (itemId === name || item.name === name) {
                av.setProperty(i, "enabled", enable);
                for (let j = 0; j < discovered.length; j++) {
                    let discoveredId = discovered[j].id || discovered[j].name;
                    if (discoveredId === itemId) {
                        discovered[j].enabled = enable;
                        break;
                    }
                }
                if (item.restart === true || item.restart === "true") {
                    PluginStore.restartRequired = true;
                }
                if (enable) {
                    loadPlugin(item);
                } else {
                    unloadPlugin(itemId);
                }
                updateEnabledCount();
            }
        }
    }

    function checkAndFinalize() {
        if (pendingMeta > 0) return;
        if (finalized) return;

        finalized = true;
        CaelestiaApi.plugins.available.clear();

        for (let key in pluginInstances) {
            if (pluginInstances[key]) pluginInstances[key].destroy();
        }
        pluginInstances = {};
        loadedCount = 0;

        for (let k = 0; k < discovered.length; k++) {
            let meta = discovered[k];
            CaelestiaApi.plugins.available.append(meta);

            if (meta.enabled) {
                loadPlugin(meta);
            }
        }
        updateEnabledCount();
        pluginsReloaded();
    }


    function readMetadata(pluginInfo) {
        let metaPath = pluginInfo.path + "/metadata.json";
        pendingMeta++;

        let proc = Qt.createQmlObject(`
            import Quickshell.Io
            Process {
                command: ["cat", ${JSON.stringify(metaPath)}]
                stdout: StdioCollector { id: out }
                stderr: StdioCollector { id: err }
                onExited: (code) => {
                    if (code === 0) {
                        try {
                            let meta = JSON.parse(out.text);
                            meta.path = ${JSON.stringify(pluginInfo.path)};
                            meta.source = ${JSON.stringify(pluginInfo.source)};

                            let disabled = pluginLoader.pluginSettings.disabledPlugins || [];
                            meta.enabled = (disabled.indexOf(meta.id) === -1 && disabled.indexOf(meta.name) === -1);
                            meta.settings = meta.settings || [];
                            meta.mediaurl = meta.mediaurl || "";
                            meta.restart = (meta.restart === "true" || meta.restart === true);

                            meta.authorName = meta.author ? (meta.author.name || "") : "";
                            let aUrl = meta.author ? (meta.author.url || "") : "";
                            meta.icon = meta.icon || "extension";

                            pluginLoader.discovered.push(meta);
                        } catch(e) {
                            console.log("Failed to parse metadata.json for", "${pluginInfo.id}", e);
                        }
                    }
                    pluginLoader.pendingMeta--;
                    pluginLoader.checkAndFinalize();
                    destroy();
                }
            }
        `, pluginLoader, "metaReader_" + pluginInfo.id);
        proc.running = true;
    }

    function loadPlugins(): void {
        loadRequested = true;
        discovered = [];
        finalized = false;
        pendingMeta = 0;

        let script = Quickshell.shellPath("scripts/list-plugins.sh");
        let bundledPlugins = Quickshell.shellPath("modules/plugins");

        let proc = Qt.createQmlObject(`
            import Quickshell.Io
            Process {
                command: ["bash", ${JSON.stringify(script)}, ${JSON.stringify(bundledPlugins)}]
                stdout: StdioCollector { id: out }
                stderr: StdioCollector { id: err }
                onExited: (code) => {
                    if (code === 0) {
                        try {
                            let list = JSON.parse(out.text);
                            if (list.length === 0) {
                                pluginLoader.checkAndFinalize();
                            }
                            for (let i = 0; i < list.length; i++) {
                                pluginLoader.readMetadata(list[i]);
                            }
                        } catch(e) {
                            console.log("Error parsing plugin list:", e);
                            pluginLoader.checkAndFinalize();
                        }
                    } else {
                        console.log("listPluginsProc exited with code:", code, "stderr:", err.text);
                        pluginLoader.checkAndFinalize();
                    }
                    destroy();
                }
            }
        `, pluginLoader, "listPluginsProc");
        proc.running = true;
    }

    function removePluginFromAvailable(id) {
        unloadPlugin(id);
        for (let j = 0; j < discovered.length; j++) {
            if ((discovered[j].id || discovered[j].name) === id) {
                discovered.splice(j, 1);
                break;
            }
        }
        for (let i = 0; i < CaelestiaApi.plugins.available.count; i++) {
            if (CaelestiaApi.plugins.available.get(i).id === id) {
                CaelestiaApi.plugins.available.remove(i);
                break;
            }
        }
        updateEnabledCount();
        pluginsReloaded();
    }

    function _internalAppendPlugin(meta) {
        let disabled = pluginLoader.pluginSettings.disabledPlugins || [];
        meta.enabled = (disabled.indexOf(meta.id) === -1 && disabled.indexOf(meta.name) === -1);
        meta.settings = meta.settings || [];
        meta.mediaurl = meta.mediaurl || "";
        meta.restart = (meta.restart === "true" || meta.restart === true);

        meta.authorName = meta.author ? (meta.author.name || "") : "";
        let aUrl = meta.author ? (meta.author.url || "") : "";
        meta.icon = meta.icon || "extension";

        discovered.push(meta);
        CaelestiaApi.plugins.available.append(meta);
        updateEnabledCount();
        pluginLoader.pluginsReloaded();
    }

    function addPluginToAvailable(id, path, source) {
        let meta = { path: path, source: source };
        addMetaReader.pendingId = id;
        addMetaReader.pendingPath = path;
        addMetaReader.pendingSource = source;
        addMetaReader.command = ["cat", path + "/metadata.json"];
        addMetaReader.running = true;
    }

    Process {
        id: addMetaReader

        property string pendingId: ""
        property string pendingPath: ""
        property string pendingSource: ""

        stdout: StdioCollector {
            id: addMetaOut
        }

        onExited: (code) => {
            if (code === 0 && addMetaOut.text.length > 0) {
                try {
                    let meta = JSON.parse(addMetaOut.text);
                    meta.path = addMetaReader.pendingPath;
                    meta.source = addMetaReader.pendingSource;

                    let disabled = pluginLoader.pluginSettings.disabledPlugins || [];
                    meta.enabled = (disabled.indexOf(meta.id) === -1 && disabled.indexOf(meta.name) === -1);
                    meta.settings = meta.settings || [];
                    meta.mediaurl = meta.mediaurl || "";
                    meta.icon = meta.icon || "extension";
                    meta.restart = (meta.restart === "true" || meta.restart === true);

                    meta.authorName = meta.author ? (meta.author.name || "") : "";
                    let aUrl = meta.author ? (meta.author.url || "") : "";
                    meta.authorAvatar = "";
                    if (aUrl && aUrl.indexOf("github.com/") !== -1) {
                        let cleanUrl = aUrl.endsWith("/") ? aUrl.slice(0, -1) : aUrl;
                        meta.authorAvatar = cleanUrl + ".png";
                    }

                    console.log("addPluginToAvailable: adding", meta.id, "to available list. mediaurl:", meta.mediaurl);
                    // Updating an already listed plugin replaces the stale entry instead of
                    // duplicating it, so the row reflects the fresh version right away.
                    let replacedKey = meta.id || meta.name;
                    for (let j = 0; j < pluginLoader.discovered.length; j++) {
                        if ((pluginLoader.discovered[j].id || pluginLoader.discovered[j].name) === replacedKey) {
                            pluginLoader.discovered.splice(j, 1);
                            break;
                        }
                    }
                    for (let i = 0; i < CaelestiaApi.plugins.available.count; i++) {
                        let existing = CaelestiaApi.plugins.available.get(i);
                        if ((existing.id || existing.name) === replacedKey) {
                            CaelestiaApi.plugins.available.remove(i);
                            break;
                        }
                    }
                    pluginLoader.discovered.push(meta);
                    CaelestiaApi.plugins.available.append(meta);
                    pluginsReloaded();
                    if (meta.restart) {
                        PluginStore.restartRequired = true;
                    }
                    if (meta.enabled) {
                        pluginLoader.loadPlugin(meta);
                    }
                    pluginLoader.updateEnabledCount();
                } catch(e) {
                    console.log("addPluginToAvailable: error parsing metadata:", e);
                }
            } else {
                console.log("addPluginToAvailable: cat failed, code:", code, "text:", addMetaOut.text);
            }
        }
    }

    IpcHandler {
        function count(): string {
            return pluginLoader.enabledCount.toString()
        }

        target: "plugins"
    }
}
