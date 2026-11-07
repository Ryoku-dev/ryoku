pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import inir.modules.common
import inir.modules.common.functions

Singleton {
    id: root

    // Discovered custom widgets: [{ id, name, icon, qmlPath, dirPath, ... }]
    property list<var> widgets: []
    // Widgets the shell ships, offered for install: [{ id, name, icon, description }]
    property list<var> shippedWidgets: []
    // User copies take precedence; bundled modules remain available after updates.
    readonly property var irisModules: {
        const seen = new Set();
        return root.widgets.concat(root.shippedWidgets.filter(widget => widget.irisBuiltin))
            .filter(widget => {
                if (seen.has(widget.id)) return false;
                seen.add(widget.id);
                return String(widget.irisQmlPath || "").length > 0;
            });
    }
    readonly property bool ready: _scanDone
    readonly property string widgetsDir: `${Directories.configPath}/inir/widgets`
    readonly property string shippedWidgetsDir: FileUtils.trimFileProtocol(Directories.payloadPath("defaults/widgets"))

    property bool _scanDone: false
    // Qt caches a component by URL for the engine lifetime. Reload through a fresh path so an updated widget compiles again.
    property int _generation: 0

    Component.onCompleted: {
        _scan();
        _scanShipped();
    }
    Connections {
        target: Config
        function onReadyChanged() {
            root._seedMissingConfig();
        }
        function onCustomWidgetDataSyncedChanged() {
            root._seedMissingConfig();
        }
    }

    function reload(): void {
        root._generation += 1;
        root._scanDone = false;
        root.widgets = [];
        _scan();
    }

    function _scanShipped(): void {
        _shippedScan.running = false;
        _shippedScan.running = true;
    }

    function _parseShipped(output: string): void {
        try {
            const entries = JSON.parse(output || "[]");
            const list = [];
            for (const entry of entries) {
                const manifest = entry.manifest || {};
                if (!manifest.name)
                    continue;
                const iris = manifest.iris && typeof manifest.iris === "object" ? manifest.iris : {};
                const irisFile = typeof iris.main === "string" ? iris.main.trim() : "";
                list.push({
                    id: entry.id,
                    name: manifest.name,
                    icon: manifest.icon || "widgets",
                    description: manifest.description || "",
                    irisBuiltin: iris.builtin === true,
                    irisQmlPath: irisFile.length > 0 ? `file://${entry.dir}/${irisFile}` : ""
                });
            }
            root.shippedWidgets = list;
        } catch (e) {
            console.warn("[CustomWidgets] Failed to parse shipped manifests:", e);
        }
    }

    function isInstalled(widgetId: string): bool {
        for (let i = 0; i < root.widgets.length; i++)
            if (root.widgets[i].id === widgetId)
                return true;
        return false;
    }

    function _scan(): void {
        _scanProcess.running = true;
    }

    // Single process that finds and reads all manifests, outputs JSON array
    Process {
        id: _scanProcess
        command: [Directories.scriptsPath + "/scan-widgets.sh", root.widgetsDir, String(root._generation)]
        running: false

        stdout: StdioCollector {
            id: _scanCollector
            onStreamFinished: {
                const output = (_scanCollector.text ?? "").trim();
                root._parseResults(output || "[]");
            }
        }

        onExited: (exitCode, exitStatus) => {
            if (exitCode !== 0 && !root._scanDone) {
                root._scanDone = true;
            }
        }
    }

    // Validate manifest fields, returns array of warning strings (empty = valid)
    function _validateManifest(id: string, m: var, dir: string): list<string> {
        const warnings = [];
        if (!m.name) warnings.push(`${id}: missing "name" field`);
        if (!m.version) warnings.push(`${id}: missing "version" field`);
        const qmlFile = m.main || (id.charAt(0).toUpperCase() + id.slice(1) + ".qml");
        // configKeys type validation
        if (m.configKeys && typeof m.configKeys === "object") {
            for (const key in m.configKeys) {
                const spec = m.configKeys[key];
                const validTypes = ["int", "real", "bool", "string"];
                if (spec.type && validTypes.indexOf(spec.type) < 0)
                    warnings.push(`${id}: configKey "${key}" has unknown type "${spec.type}"`);
            }
        }
        return warnings;
    }

    function _parseResults(jsonStr: string): void {
        try {
            const entries = JSON.parse(jsonStr);
            const result = [];
            for (const entry of entries) {
                const m = entry.manifest;
                const warnings = root._validateManifest(entry.id, m, entry.dir);
                if (warnings.length > 0)
                    console.warn("[CustomWidgets]", warnings.join("; "));
                const qmlFile = m.main || (entry.id.charAt(0).toUpperCase() + entry.id.slice(1) + ".qml");
                const iris = m.iris && typeof m.iris === "object" ? m.iris : {};
                const irisFile = typeof iris.main === "string" ? iris.main.trim() : "";
                const load = entry.load || entry.dir;
                result.push({
                    id: entry.id,
                    name: m.name || entry.id,
                    icon: m.icon || "widgets",
                    version: m.version || "1.0",
                    author: m.author || "",
                    description: m.description || "",
                    category: m.category || "",
                    qmlPath: `file://${load}/${qmlFile}`,
                    irisQmlPath: irisFile.length > 0 ? `file://${load}/${irisFile}` : "",
                    irisSlots: Array.isArray(iris.slots) ? iris.slots : [],
                    dirPath: entry.dir,
                    configKeys: m.configKeys || {},
                    resizableAxes: m.resizableAxes || {},
                    defaultSize: m.defaultSize || { width: 200, height: 100 },
                    defaultConfig: m.defaultConfig || {},
                    valid: warnings.length === 0,
                    warnings: warnings
                });
            }
            root.widgets = result;
        } catch (e) {
            console.warn("[CustomWidgets] Failed to parse manifests:", e);
        }
        root._scanDone = true;
        root._seedMissingConfig();
    }

    function _readCustomConfig(widgetId: string, key: string): var {
        return Config.getNestedValue("background.widgets.custom." + widgetId + "." + key, undefined);
    }

    function _defaultForSpec(spec: var): var {
        if (spec && spec.default !== undefined)
            return spec.default;
        const type = spec?.type ?? "bool";
        if (type === "bool") return false;
        if (type === "string") {
            if (spec?.options && spec.options.length > 0) {
                const first = spec.options[0];
                return (first && typeof first === "object") ? (first.value ?? first.label ?? first.displayName ?? "") : first;
            }
            return "";
        }
        return 0;
    }

    function _widgetDefaults(widget: var, index: int): var {
        let defaults = {
            enable: false,
            placementStrategy: "free",
            x: 240 + index * 36,
            y: 240 + index * 28,
            widgetScale: 100,
            widgetOpacity: 100,
            colorMode: "auto",
            dim: 0,
            backgroundOpacity: 0.06,
            borderWidth: 1,
            borderOpacity: 0.08,
            cornerRadius: -1
        };
        const axes = widget.resizableAxes || {};
        const size = widget.defaultSize || {};
        if (axes.width && size.width !== undefined) defaults[axes.width] = size.width;
        if (axes.height && size.height !== undefined) defaults[axes.height] = size.height;
        if (axes.uniform && axes.uniform !== "widgetScale" && size.width !== undefined)
            defaults[axes.uniform] = size.width;
        const extraDefaults = widget.defaultConfig || {};
        for (const key in extraDefaults)
            defaults[key] = extraDefaults[key];
        const configKeys = widget.configKeys || {};
        for (const key in configKeys)
            defaults[key] = root._defaultForSpec(configKeys[key]);
        return defaults;
    }

    function _seedMissingConfig(): void {
        if (!Config.ready || !Config.customWidgetDataSynced || !root._scanDone || root.widgets.length === 0)
            return;
        let updates = {};
        for (let i = 0; i < root.widgets.length; i++) {
            const widget = root.widgets[i];
            const defaults = root._widgetDefaults(widget, i);
            for (const key in defaults) {
                if (root._readCustomConfig(widget.id, key) === undefined)
                    updates["background.widgets.custom." + widget.id + "." + key] = defaults[key];
            }
        }
        if (Object.keys(updates).length > 0)
            Config.setNestedValues(updates);
    }

    // Get a custom widget's config value (freeform namespace)
    function getConfigValue(widgetId: string, key: string, defaultValue: var): var {
        return Config.getNestedValue("background.widgets.custom." + widgetId + "." + key, defaultValue);
    }

    // Set a custom widget's config value
    function setConfigValue(widgetId: string, key: string, value: var): void {
        Config.setNestedValue("background.widgets.custom." + widgetId + "." + key, value);
    }

    // Create a new widget from template
    function create(name: string): void {
        if (!name || name.length === 0) return;
        _createProcess.widgetName = name;
        _createProcess.running = true;
    }

    // Delete a widget by removing its directory
    function remove(widgetId: string): void {
        if (!widgetId || widgetId.length === 0) return;
        _removeProcess.widgetId = widgetId;
        _removeProcess.running = true;
    }

    // Copy a shipped widget from defaults into the user's widgets directory
    function installShipped(widgetId: string): void {
        if (typeof widgetId !== "string" || !/^[A-Za-z0-9][A-Za-z0-9_-]*$/.test(widgetId)
                || _installShippedProcess.running) return;
        _installShippedProcess.widgetId = widgetId;
        _installShippedProcess.running = true;
    }

    // Keep the original entry point for callers that install the example.
    function installExample(): void {
        root.installShipped("example-widget");
    }

    // Open widget directory in file manager
    function openWidgetDir(widgetId: string): void {
        const dirPath = widgetId ? `${root.widgetsDir}/${widgetId}` : root.widgetsDir;
        Spawn.runApp(["xdg-open", "file://" + dirPath], { label: "folder" });
    }

    // Widget template generator — creates scaffold with all imports, services, and patterns
    Process {
        id: _createProcess
        property string widgetName: ""
        property string _pascalName: widgetName.charAt(0).toUpperCase() + widgetName.slice(1).replace(/-([a-z])/g, (_, c) => c.toUpperCase())
        running: false
        command: ["bash", "-c", `
            dir="${root.widgetsDir}/${_createProcess.widgetName}"
            mkdir -p "$dir"
            cat > "$dir/widget.json" << 'MANIFEST'
{
    "name": "${_createProcess._pascalName}",
    "icon": "widgets",
    "version": "1.0",
    "author": "",
    "description": "Custom desktop widget",
    "category": "custom",
    "main": "${_createProcess._pascalName}.qml",
    "iris": {
        "main": "IrisCompact.qml",
        "slots": ["island.desktop"]
    },
    "defaultConfig": {
        "placementStrategy": "free",
        "widgetScale": 100,
        "widgetOpacity": 100,
        "colorMode": "auto",
        "dim": 0,
        "x": 200,
        "y": 200
    },
    "configKeys": {
        "label": { "type": "string", "default": "${_createProcess._pascalName}", "label": "Widget label" },
        "showIcon": { "type": "bool", "default": true, "label": "Show icon" }
    },
    "resizableAxes": { "uniform": "widgetScale" },
    "defaultSize": { "width": 200, "height": 80 }
}
MANIFEST
            cat > "$dir/${_createProcess._pascalName}.qml" << 'QML'
// ${_createProcess._pascalName} — custom iNiR desktop widget
// Full SDK reference: defaults/widgets/WIDGET-SDK.md
// Example widget: defaults/widgets/example-widget/

import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Io
import inir
import inir.services
import inir.modules.common        // Config, Appearance, Directories, GlobalStates
import inir.modules.common.functions // ColorUtils, StringUtils, DateUtils, FileUtils
import inir.modules.common.widgets   // 130+ components (StyledText, MaterialSymbol, RippleButton, CircularProgress, Graph, CavaVisualizer...)
import inir.modules.background.widgets // AbstractBackgroundWidget base class

AbstractBackgroundWidget {
    id: root

    configEntryName: "custom.${_createProcess.widgetName}"
    defaultConfig: ({
        placementStrategy: "free", widgetScale: 100, widgetOpacity: 100,
        colorMode: "auto", dim: 0, x: 200, y: 200
    })

    implicitWidth: content.implicitWidth + Math.round(16 * scaleFactor)
    implicitHeight: content.implicitHeight + Math.round(16 * scaleFactor)
    resizableAxes: ({ uniform: "widgetScale" })
    resizeMinWidth: 80
    resizeMinHeight: 40

    // Read your widget's config keys with null-safe access:
    //   _readConfigKey("label") ?? "fallback"
    // Write config:
    //   Config.setNestedValue("background.widgets.custom.${_createProcess.widgetName}.label", value)

    // Card background using inherited appearance controls
    Rectangle {
        anchors.fill: parent
        radius: root.cornerRadiusOverride >= 0 ? root.cornerRadiusOverride : Appearance.rounding.normal
        color: root.backgroundOpacity > 0 ? ColorUtils.applyAlpha(root.colText, root.backgroundOpacity) : "transparent"
        border { width: root.borderWidth; color: ColorUtils.applyAlpha(root.colText, root.borderOpacity) }
    }

    Column {
        id: content
        anchors.centerIn: parent
        spacing: Math.round(6 * root.scaleFactor)

        Row {
            spacing: Math.round(6 * root.scaleFactor)
            anchors.horizontalCenter: parent.horizontalCenter
            MaterialSymbol {
                text: "schedule"
                iconSize: Math.round(20 * root.scaleFactor)
                color: root.colText
                anchors.verticalCenter: parent.verticalCenter
            }
            StyledText {
                text: DateTime.time
                font {
                    pixelSize: Math.round(Appearance.font.pixelSize.large * root.scaleFactor)
                    family: Appearance.font.family.numbers
                }
                color: root.colText
                anchors.verticalCenter: parent.verticalCenter
            }
        }
        StyledText {
            text: root._readConfigKey("label") ?? "${_createProcess._pascalName}"
            font.pixelSize: Math.round(Appearance.font.pixelSize.small * root.scaleFactor)
            color: ColorUtils.applyAlpha(root.colText, 0.6)
            anchors.horizontalCenter: parent.horizontalCenter
        }
    }

    // Available services (all reactive, auto-update):
    //   DateTime.time, DateTime.date, DateTime.uptime
    //   Weather.data.temp, Weather.data.description, Weather.enabled
    //   Battery.percentage (0-1), Battery.isCharging, Battery.available
    //   Audio.value (0-2.0), Audio.sink?.audio?.muted, Audio.ready
    //   Network.wifi, Network.networkName, Network.networkStrength (0-100)
    //   ResourceUsage.cpuUsage (0-1), ResourceUsage.memoryUsedPercentage (call ensureRunning() first)
    //   MprisController.titleOf(MprisController.activePlayer), MprisController.displayPlayers
    //   Notifications.unread, Notifications.list

    // Available components (import inir.modules.common.widgets):
    //   Text:     StyledText, MaterialSymbol (icon font — "wifi", "battery_full", "volume_up", etc)
    //   Buttons:  RippleButton, FloatingActionButton, MenuButton, GroupButton
    //   Progress: CircularProgress, StyledProgressBar, Graph (line chart)
    //   Input:    StyledSlider, StyledSpinBox, StyledSwitch, MaterialTextField
    //   Layout:   FadeLoader (animated show/hide), CollapsibleSection, Revealer
    //   Shapes:   MaterialShape, Circle, GlassBackground
    //   Audio:    CavaVisualizer (spectrum), CavaProcess, WaveVisualizer
    //   Effects:  StyledDropShadow, StyledRectangularShadow, StyledBlurEffect

    // Theming — always use tokens, never hardcode:
    //   Colors:   Appearance.colors.colPrimary, .colOnLayer0, .colError, .colSecondaryContainer
    //   Fonts:    Appearance.font.pixelSize.{small,normal,large,huge}, .family.{main,numbers,monospace}
    //   Rounding: Appearance.rounding.{small,normal,large,full}
    //   root.colText adapts to wallpaper brightness automatically
}
QML
            cat > "$dir/IrisCompact.qml" << 'IRIS_QML'
import QtQuick
import QtQuick.Layouts
import inir.services
import inir.modules.iris.style
import inir.modules.iris.components

Item {
    id: root
    property string irisSlot: ""
    property var targetScreen
    implicitWidth: compactRow.implicitWidth
    implicitHeight: Math.round(28 * IrisStyle.density)

    RowLayout {
        id: compactRow
        anchors.centerIn: parent
        spacing: IrisStyle.spaceSmall

        IrisMark { implicitSize: Math.round(14 * IrisStyle.density) }
        IrisText {
            text: DateTime.timeDisplay
            role: IrisText.Meta
            font.family: IrisStyle.fontNumbers
            color: IrisStyle.text
        }
    }
}
IRIS_QML
            echo "done"
        `]
        stdout: StdioCollector {
            onStreamFinished: root.reload()
        }
    }

    // Remove a widget directory
    Process {
        id: _removeProcess
        property string widgetId: ""
        running: false
        command: ["bash", "-c", `rm -rf "${root.widgetsDir}/${_removeProcess.widgetId}" && echo "removed"`]
        stdout: StdioCollector {
            onStreamFinished: root.reload()
        }
    }

    // Shipped widget manifests, read from the same scanner as the user widget dir
    Process {
        id: _shippedScan
        running: false
        command: [Directories.scriptsPath + "/scan-widgets.sh", root.shippedWidgetsDir]
        stdout: StdioCollector {
            id: _shippedScanCollector
            onStreamFinished: root._parseShipped(_shippedScanCollector.text ?? "")
        }
    }

    // Copy a shipped widget from defaults
    Process {
        id: _installShippedProcess
        property string widgetId: ""
        running: false
        command: ["bash", "-c", `
            set -eu
            src="$1/$3"
            dest="$2/$3"
            [ -d "$src" ] || { echo "fail"; exit 1; }
            if [ -e "$dest" ] || [ -L "$dest" ]; then
                echo "exists"
                exit 0
            fi
            mkdir -p "$2"
            staging=$(mktemp -d "$2/.install-$3.XXXXXX")
            trap 'rm -rf -- "$staging"' EXIT
            cp -r "$src"/. "$staging"/
            mv -Tn -- "$staging" "$dest"
            if [ -d "$staging" ]; then echo "exists"; else echo "done"; fi
        `, "install-shipped-widget", root.shippedWidgetsDir, root.widgetsDir,
            _installShippedProcess.widgetId]
        stdout: StdioCollector {
            onStreamFinished: root.reload()
        }
    }
}
