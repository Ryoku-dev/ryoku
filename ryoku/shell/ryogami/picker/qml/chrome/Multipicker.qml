import QtQuick
import Ryoku.Ui.Singletons

Item {
    id: mp

    required property PickerState state

    readonly property bool open: mp.state.multipickerOpen
    readonly property var outputs: Library.outputs
    readonly property int row: mp.state.multipickerRow
    readonly property string cardKey: (mp.open && mp.state.view && mp.row >= 0)
        ? (mp.state.view.get(mp.row).key || "") : ""
    readonly property var entry: mp.cardKey ? Library.entry(mp.state.collection, mp.cardKey) : ({})
    readonly property bool hasAudio: mp.entry.type === "video" || mp.entry.type === "we"

    property var sel: ({})
    property var mute: ({})
    property var vol: ({})
    property string themeOutput: ""
    property string targetMode: "all"

    visible: mp.open

    onOpenChanged: if (mp.open) mp._init()

    function _init() {
        var s = ({}), m = ({}), v = ({})
        for (var i = 0; i < mp.outputs.length; ++i) {
            var o = mp.outputs[i]
            var cur = o.current || ({})
            s[o.name] = true
            m[o.name] = cur.mute === true
            v[o.name] = (cur.volume !== undefined && cur.volume !== null) ? cur.volume : 100
        }
        mp.sel = s
        mp.mute = m
        mp.vol = v
        mp.themeOutput = String(Settings.value("display.themeOutput") || "")
        mp.targetMode = Settings.value("general.applyOnPickerMonitor") === true ? "monitor" : "all"
    }
    function _toggleSel(name) {
        var s = Object.assign({}, mp.sel)
        s[name] = !s[name]
        mp.sel = s
        mp.targetMode = "custom"
    }
    function _setAll(on) {
        var s = ({})
        for (var i = 0; i < mp.outputs.length; ++i)
            s[mp.outputs[i].name] = on
        mp.sel = s
        mp.targetMode = on ? "all" : "custom"
    }
    function _toggleMute(name) { var m = Object.assign({}, mp.mute); m[name] = !m[name]; mp.mute = m }
    function _setVol(name, value) { var v = Object.assign({}, mp.vol); v[name] = Math.round(value); mp.vol = v }
    function _setTheme(name) {
        mp.themeOutput = (mp.themeOutput === name) ? "" : name
        Settings.set("display.themeOutput", mp.themeOutput)
    }
    readonly property bool _allSelected: {
        for (var i = 0; i < mp.outputs.length; ++i)
            if (!mp.sel[mp.outputs[i].name]) return false
        return mp.outputs.length > 0
    }
    function _apply() {
        var names = []
        var workspace = null
        if (mp.targetMode === "workspace") {
            workspace = mp.state.currentWorkspace()
            if (workspace && workspace.output)
                names = [workspace.output]
        } else if (mp.targetMode === "monitor") {
            if (mp.state.monitor)
                names = [mp.state.monitor]
        } else if (mp.targetMode === "custom") {
            for (var i = 0; i < mp.outputs.length; ++i) {
                var n = mp.outputs[i].name
                if (mp.sel[n])
                    names.push(n)
            }
        }
        var outs = mp.targetMode === "all" ? [] : names
        var targets = outs.length ? outs : mp.outputs.map(function(o) { return o.name })
        var audio = ({}), volume = ({})
        for (var j = 0; j < targets.length; ++j) {
            var target = targets[j]
            audio[target] = !!mp.mute[target]
            volume[target] = (mp.vol[target] !== undefined) ? mp.vol[target] : 100
        }
        var workspaceTarget = mp.targetMode === "workspace"
            ? { "kind": "workspace", "workspace": workspace } : null
        mp.state.applyEntry(mp.entry, outs, mp.hasAudio ? audio : ({}),
            mp.hasAudio ? volume : ({}), workspaceTarget)
    }

    Scrim {
        anchors.fill: parent
        alpha: 0.6
        onDismissed: mp.state.multipickerOpen = false
    }

    ChamferPanel {
        anchors.centerIn: parent
        width: Math.min(560 * Theme.scale, parent.width - 40 * Theme.scale)
        height: Math.min(column.implicitHeight + 40 * Theme.scale, parent.height - 40 * Theme.scale)

        MouseArea { anchors.fill: parent }   // keep clicks inside the panel

        Column {
            id: column
            anchors.fill: parent
            anchors.margins: 20 * Theme.scale
            spacing: 12 * Theme.scale

            Text {
                text: I18n.tr("Apply wallpaper")
                font.family: Theme.display
                font.pixelSize: Theme.fontLead
                color: Theme.surfaceText
                renderType: Text.NativeRendering
            }
            Text {
                width: parent.width
                text: I18n.tr("Choose every display, this monitor, or the active workspace. Video and Wallpaper Engine sound stays independent per display.")
                wrapMode: Text.WordWrap
                font.family: Theme.sans
                font.weight: Font.Normal
                font.pixelSize: Theme.fontBase
                color: Theme.withAlpha(Theme.surfaceText, 0.56)
                renderType: Text.NativeRendering
            }

            Row {
                spacing: 8 * Theme.scale
                FixedButton {
                    mode: "toggle"
                    active: mp.targetMode === "all"
                    label: I18n.tr("All")
                    onTriggered: mp.targetMode = "all"
                }
                FixedButton {
                    mode: "toggle"
                    active: mp.targetMode === "monitor"
                    enabled: mp.state.monitor.length > 0
                    label: I18n.tr("This monitor")
                    onTriggered: mp.targetMode = "monitor"
                }
                FixedButton {
                    mode: "toggle"
                    active: mp.targetMode === "workspace"
                    enabled: mp.state.currentWorkspaceName.length > 0
                    label: mp.state.currentWorkspaceName.length > 0
                        ? I18n.tr("This workspace · %1").arg(mp.state.currentWorkspaceName)
                        : I18n.tr("This workspace")
                    onTriggered: mp.targetMode = "workspace"
                }
            }

            Flickable {
                width: parent.width
                height: Math.min(rows.implicitHeight, mp.height * 0.5)
                contentHeight: rows.implicitHeight
                clip: true
                boundsBehavior: Flickable.StopAtBounds

                Column {
                    id: rows
                    width: parent.width
                    spacing: 10 * Theme.scale

                    Repeater {
                        model: mp.outputs
                        delegate: Column {
                            required property var modelData
                            width: rows.width
                            spacing: 6 * Theme.scale

                            Row {
                                spacing: 8 * Theme.scale
                                FixedButton {
                                    mode: "toggle"
                                    active: mp.sel[modelData.name] === true
                                    label: modelData.name
                                    minWidth: 96
                                    onTriggered: mp._toggleSel(modelData.name)
                                }
                                FixedButton {
                                    mode: "toggle"
                                    active: mp.themeOutput === modelData.name
                                    label: I18n.tr("Use for colours")
                                    onTriggered: mp._setTheme(modelData.name)
                                }
                            }

                            Row {
                                visible: mp.hasAudio
                                spacing: 8 * Theme.scale
                                FixedButton {
                                    mode: "toggle"
                                    active: mp.mute[modelData.name] !== true
                                    label: mp.mute[modelData.name] === true ? I18n.tr("Muted") : I18n.tr("Sound")
                                    minWidth: 76
                                    onTriggered: mp._toggleMute(modelData.name)
                                }
                                FolioSlider {
                                    anchors.verticalCenter: parent.verticalCenter
                                    width: 160 * Theme.scale
                                    from: 0
                                    to: 100
                                    step: 1
                                    enabled: mp.mute[modelData.name] !== true
                                    value: mp.vol[modelData.name] !== undefined ? mp.vol[modelData.name] : 100
                                    onMoved: (v) => mp._setVol(modelData.name, v)
                                }
                                Text {
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: (mp.vol[modelData.name] !== undefined ? mp.vol[modelData.name] : 100) + "%"
                                    font.family: Theme.display
                                    font.pixelSize: Theme.fontSmall
                                    color: Theme.withAlpha(Theme.surfaceText, 0.7)
                                    renderType: Text.NativeRendering
                                }
                            }
                        }
                    }
                }
            }

            Row {
                anchors.right: parent.right
                spacing: 8 * Theme.scale
                FolioAction {
                    label: I18n.tr("Cancel")
                    onTriggered: mp.state.multipickerOpen = false
                }
                FolioAction {
                    label: I18n.tr("Apply")
                    active: true
                    onTriggered: mp._apply()
                }
            }
        }
    }
}
