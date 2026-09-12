import QtQuick
import Quickshell
import Quickshell.Io
import ".."
import "../.."
import "../../components"
import Ryoku.Ui.Singletons

Column {
    id: root
    property var colors
    property var saveConfigKey
    property var cloneIntegrations
    property var notify
    property string activePage: "matugen"
    property var _templates: ({})
    readonly property var _apps: Object.keys(root._templates).sort()

    FileView {
        id: matugenFile
        path: Quickshell.env("HOME") + "/.config/ryoku/matugen.json"
        watchChanges: true
        onFileChanged: reload()
        onLoaded: {
            try {
                var d = JSON.parse(matugenFile.text())
                root._templates = (d.templates && typeof d.templates === "object") ? d.templates : ({})
            } catch (e) {}
        }
    }

    width: parent ? parent.width : 0
    spacing: 12

    Row {
        anchors.horizontalCenter: parent.horizontalCenter
        spacing: Style.spacingSmall

        FilterButton {
            colors: root.colors
            label: I18n.tr("MATUGEN")
            register: false
            height: 28
            isActive: root.activePage === "matugen"
            onClicked: root.activePage = "matugen"
        }

        FilterButton {
            colors: root.colors
            label: I18n.tr("PALETTE BRIDGE")
            register: false
            height: 28
            isActive: root.activePage === "bridge"
            onClicked: root.activePage = "bridge"
        }
    }

    SettingsCard {
        visible: root.activePage === "matugen"
        colors: root.colors
        title: I18n.tr("External Matugen")
        subtitle: I18n.tr("Run Matugen alongside Ryogami-wall's internal configuration.")

        RowTextInput {
            colors: root.colors
            title: I18n.tr("Config path")
            description: I18n.tr("Path to an external matugen config file (e.g. from your existing setup).")
            value: Config.defaultMatugenConfig
            placeholder: "/path/to/matugen.config.toml"
            onCommit: function(v) { if (root.saveConfigKey) root.saveConfigKey("defaultMatugenConfig", v) }
        }

        RowTextInput {
            colors: root.colors
            title: I18n.tr("Command")
            description: I18n.tr("Shell command to invoke. %config% = config path, %path% = wallpaper, %scheme% = scheme, %mode% = light/dark, %index% = source color index.")
            value: Config.externalMatugenCommand
            placeholder: I18n.tr("matugen -c %config% image %path% -t %scheme% -m %mode% --source-color-index %index%")
            onCommit: function(v) { if (root.saveConfigKey) root.saveConfigKey("externalMatugenCommand", v) }
        }
    }

    SettingsCard {
        visible: root.activePage === "matugen"
        colors: root.colors
        title: I18n.tr("Integrations")
        subtitle: I18n.tr("Each entry generates themed output from a template and optionally runs a reload command.")

        Repeater {
            id: integRepeater
            model: Config.integrations

            IntegrationEditorRow {
                colors: root.colors
                entryName: modelData.name || ""
                template: modelData.template || ""
                output: modelData.output || ""
                reload: modelData.reload || ""

                onSaveField: function(field, value) {
                    if (!root.cloneIntegrations || !root.saveConfigKey) return
                    var a = root.cloneIntegrations()
                    var key = field === "name" ? "name" : field
                    if (field === "reload") {
                        if (value === "" || value === undefined || value === null) delete a[index].reload
                        else a[index].reload = value
                    } else {
                        a[index][key] = value
                    }
                    root.saveConfigKey("integrations", a)
                }

                onRemoveRequested: {
                    if (!root.cloneIntegrations || !root.saveConfigKey) return
                    var a = root.cloneIntegrations()
                    a.splice(index, 1)
                    root.saveConfigKey("integrations", a)
                }
            }
        }

        RowAction {
            colors: root.colors
            title: I18n.tr("Add integration")
            description: I18n.tr("Append a new empty integration row.")
            onClicked: {
                if (!root.cloneIntegrations || !root.saveConfigKey) return
                var a = root.cloneIntegrations(); a.push({ name: "", template: "", output: "" })
                root.saveConfigKey("integrations", a)
            }
        }
    }

    SettingsCard {
        visible: root.activePage === "matugen"
        colors: root.colors
        title: I18n.tr("App templates")
        subtitle: I18n.tr("Recolour each app's config from the generated palette.")

        Grid {
            id: tplGrid
            width: parent.width
            columns: 3
            columnSpacing: 12 * Config.uiScale
            rowSpacing: 8 * Config.uiScale
            topPadding: 4 * Config.uiScale
            readonly property real cellW: (width - columnSpacing * (columns - 1)) / columns

            Repeater {
                model: root._apps

                SettingsToggle {
                    width: tplGrid.cellW
                    colors: root.colors
                    label: modelData
                    checked: root._templates[modelData] !== false
                    onToggle: function(v) {
                        var patch = {}
                        patch[modelData] = v
                        Quickshell.execDetached(["ryoku-hub", "hypr", "matugen", "set",
                            JSON.stringify({ templates: patch })])
                    }
                }
            }
        }
    }

    PaletteBridgeSettings {
        visible: root.activePage === "bridge"
        colors: root.colors
        saveConfigKey: root.saveConfigKey
        notify: root.notify
    }
}
