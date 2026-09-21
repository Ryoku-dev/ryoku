import QtQuick
import Quickshell
import Quickshell.Io
import "../.."
import "../../components"
import Ryoku.Ui.Singletons

// Settings for the optional Ryoku Palette Bridge. The backend handles validation
// and service policy; this page displays its status and sends actions.
Column {
    id: root
    property var colors
    property var saveConfigKey
    property var notify
    property var status: ({ contract: 1, integrations: [] })
    property bool busy: false
    property string message: ""
    property string pendingRemove: ""
    property string _statusBuf: ""
    property string _actionBuf: ""

    width: parent ? parent.width : 0
    spacing: 8

    Component.onCompleted: refresh()

    function refresh() {
        if (!statusProc.running) {
            root._statusBuf = ""
            statusProc.command = ["ryoku-hub", "palette-bridge", "status", Config.paletteBridgeSource]
            statusProc.running = true
        }
    }

    function act(args, label) {
        if (root.busy) return
        root.busy = true
        root.message = label + "…"
        root._actionBuf = ""
        actionProc.command = ["ryoku-hub", "palette-bridge"].concat(args)
        actionProc.running = true
    }

    function integration(id) {
        var rows = root.status.integrations || []
        for (var i = 0; i < rows.length; i++) if (rows[i].id === id) return rows[i]
        return ({ id: id, installed: false })
    }

    function toggleIntegration(id) {
        if (integration(id).installed && integration(id).managed) {
            if (root.pendingRemove !== id) {
                root.pendingRemove = id
                root.message = I18n.tr("Select remove again to confirm. Only bridge-owned files will be removed.")
                confirmTimer.restart()
                return
            }
            root.pendingRemove = ""
            act(["integration", "remove", id, Config.paletteBridgeSource], I18n.tr("Removing integration"))
            return
        }
        act(["integration", "install", id, Config.paletteBridgeSource], I18n.tr("Installing integration"))
    }

    Timer { id: refreshTimer; interval: 500; onTriggered: root.refresh() }
    Timer { id: confirmTimer; interval: 6000; onTriggered: root.pendingRemove = "" }

    Process {
        id: statusProc
        stdout: SplitParser { splitMarker: ""; onRead: function(data) { root._statusBuf += data } }
        onExited: function(code) {
            if (code === 0) {
                try { root.status = JSON.parse(root._statusBuf) }
                catch (e) { root.message = I18n.tr("Bridge returned invalid status.") }
            } else {
                root.status = ({ contract: 1, available: false, integrations: [] })
            }
            root._statusBuf = ""
        }
    }

    Process {
        id: actionProc
        stdout: SplitParser { splitMarker: ""; onRead: function(data) { root._actionBuf += data } }
        stderr: SplitParser { splitMarker: ""; onRead: function(data) { root._actionBuf += data } }
        onExited: function(code) {
            root.busy = false
            var detail = root._actionBuf.trim()
            root.message = code === 0 ? (detail || I18n.tr("Palette Bridge updated."))
                                      : (detail || I18n.tr("Palette Bridge action failed."))
            if (root.notify) root.notify(root.message, code === 0)
            root._actionBuf = ""
            refreshTimer.restart()
        }
    }

    SettingsCard {
        colors: root.colors
        title: I18n.tr("Ryoku Palette Bridge")
        subtitle: I18n.tr("Live Ryoku wallpaper colours for Zen, Spotify, and Vesktop.")

        RowTextInput {
            colors: root.colors
            title: I18n.tr("Build source")
            description: I18n.tr("Use a local checkout, or Ryoku's packaged source.")
            value: Config.paletteBridgeSource
            placeholder: "/usr/share/ryoku/palette-bridge"
            onCommit: function(v) {
                if (root.saveConfigKey) root.saveConfigKey("paletteBridgeSource", v)
                root.refresh()
            }
        }

        RowAction {
            colors: root.colors
            title: root.status.installed ? I18n.tr("Build and update") : I18n.tr("Build and install")
            description: root.status.sourceReady
                ? I18n.tr("Build the selected local source and install the user service.")
                : I18n.tr("Select a source containing the Palette Bridge build recipe.")
            enabled: !root.busy && !!root.status.sourceReady
            opacity: enabled ? 1 : 0.45
            onClicked: root.act(["install", Config.paletteBridgeSource], I18n.tr("Building Palette Bridge"))
        }

        RowToggle {
            colors: root.colors
            title: root.status.active ? I18n.tr("Bridge running") : I18n.tr("Enable Palette Bridge")
            description: I18n.tr("Start at login and publish wallpaper colours on the local endpoint.")
            checked: !!root.status.enabled
            enabled: !root.busy && !!root.status.installed
            opacity: enabled ? 1 : 0.45
            onToggle: function(on) { root.act(["service", on ? "enable" : "disable"], on ? I18n.tr("Starting Palette Bridge") : I18n.tr("Stopping Palette Bridge")) }
        }

        RowAction {
            colors: root.colors
            title: I18n.tr("Restart service")
            description: I18n.tr("Restart the bridge without changing its login setting.")
            enabled: !root.busy && !!root.status.installed
            opacity: enabled ? 1 : 0.45
            onClicked: root.act(["service", "restart"], I18n.tr("Restarting Palette Bridge"))
        }

        RowAction {
            colors: root.colors
            title: root.status.healthy ? I18n.tr("Health check passed") : I18n.tr("Run health check")
            description: root.message || I18n.tr("%1 · contract v%2").arg(root.status.endpoint || "127.0.0.1:47616").arg(root.status.contract || 1)
            valueLabel: root.status.active ? I18n.tr("RUNNING") : I18n.tr("STOPPED")
            enabled: !root.busy && (!!root.status.installed || !!root.status.sourceReady)
            opacity: enabled ? 1 : 0.45
            onClicked: root.act(["doctor", Config.paletteBridgeSource], I18n.tr("Checking Palette Bridge"))
        }
    }

    SettingsCard {
        colors: root.colors
        title: I18n.tr("App integrations")
        subtitle: I18n.tr("Each app is opt-in. Removal preserves files the bridge does not own.")

        RowAction {
            colors: root.colors
            title: I18n.tr("Spotify")
            description: I18n.tr("Spicetify extension for live palette updates without stopping playback.")
            valueLabel: root.pendingRemove === "spotify" ? I18n.tr("CONFIRM REMOVE") : (root.integration("spotify").installed && root.integration("spotify").managed ? I18n.tr("REMOVE") : I18n.tr("SET UP"))
            enabled: !root.busy && !!root.status.sourceReady
            opacity: enabled ? 1 : 0.45
            onClicked: root.toggleIntegration("spotify")
        }

        RowAction {
            colors: root.colors
            title: I18n.tr("Vesktop")
            description: I18n.tr("Midnight base theme and QuickCSS palette without an app reload.")
            valueLabel: root.pendingRemove === "vesktop" ? I18n.tr("CONFIRM REMOVE") : (root.integration("vesktop").installed && root.integration("vesktop").managed ? I18n.tr("REMOVE") : I18n.tr("SET UP"))
            enabled: !root.busy && !!root.status.sourceReady
            opacity: enabled ? 1 : 0.45
            onClicked: root.toggleIntegration("vesktop")
        }

        RowAction {
            colors: root.colors
            title: I18n.tr("Zen Browser")
            description: I18n.tr("Native browser theme updates with a Matugen startup fallback.")
            valueLabel: root.pendingRemove === "zen" ? I18n.tr("CONFIRM REMOVE") : (root.integration("zen").installed && root.integration("zen").managed ? I18n.tr("REMOVE") : I18n.tr("SET UP"))
            enabled: !root.busy && !!root.status.sourceReady
            opacity: enabled ? 1 : 0.45
            onClicked: root.toggleIntegration("zen")
        }
    }
}
