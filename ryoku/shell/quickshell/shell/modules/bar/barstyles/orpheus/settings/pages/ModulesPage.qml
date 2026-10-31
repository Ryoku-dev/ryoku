pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import "../../kit" as K
import "../../modules/workspaces" as W
import ".." as S
import "." as P

Item {
    id: root

    property var scene: null
    property bool shown: false
    property var args: ({})

    readonly property var workspaceSample: [
        { key: "1", label: "1", index: 1, active: false, occupied: true, windowCount: 1, apps: ["org.gnome.Nautilus"] },
        { key: "2", label: "2", index: 2, active: true, occupied: true, windowCount: 2, apps: ["firefox", "kitty"] },
        { key: "3", label: "3", index: 3, active: false, occupied: false, windowCount: 0, apps: [] },
        { key: "4", label: "4", index: 4, active: false, occupied: true, windowCount: 1, apps: ["kitty"] },
        { key: "5", label: "5", index: 5, active: false, occupied: false, windowCount: 0, apps: [] }
    ]
    readonly property var workspaceStyles: [
        { value: "dots", label: "Dots" },
        { value: "numbers", label: "Numbers" },
        { value: "kanji", label: "Kanji" },
        { value: "greek", label: "Greek" },
        { value: "dashes", label: "Dashes" },
        { value: "icons", label: "Icons" }
    ]
    readonly property var statusChoices: [
        { value: "wifi", label: "Wi-Fi", icon: "wifi" },
        { value: "bluetooth", label: "Bluetooth", icon: "bluetooth" },
        { value: "volume", label: "Volume", icon: "volume_up" },
        { value: "mic", label: "Microphone", icon: "mic" }
    ]
    readonly property var aiChoices: [
        { value: "claude", label: "Claude", icon: "auto_awesome" },
        { value: "codex", label: "Codex", icon: "auto_awesome" },
        { value: "opencode", label: "OpenCode", icon: "auto_awesome" }
    ]
    readonly property var readingChoices: [
        { value: "cpu", label: "Processor", icon: "memory" },
        { value: "mem", label: "Memory", icon: "data_usage" },
        { value: "temp", label: "Temperature", icon: "device_thermostat" },
        { value: "gpu", label: "Graphics", icon: "developer_board" },
        { value: "net", label: "Network", icon: "swap_vert" }
    ]
    readonly property var actionChoices: [
        { value: "launcher", label: "Launcher", icon: "search" },
        { value: "ask", label: "Ask", icon: "auto_awesome" },
        { value: "overview", label: "Overview", icon: "view_quilt" },
        { value: "clipboard", label: "Clipboard", icon: "content_paste" },
        { value: "capture", label: "Screenshot", icon: "screenshot_region" },
        { value: "wallpaper", label: "Wallpaper", icon: "wallpaper" },
        { value: "hub", label: "Ryoku Hub", icon: "settings" }
    ]

    implicitHeight: content.implicitHeight
    enabled: shown

    function contains(list, value) {
        return Array.isArray(list) && list.indexOf(value) >= 0;
    }

    function toggleList(key, value) {
        const current = Array.isArray(S.Prefs[key]) ? S.Prefs[key] : [];
        const next = current.slice();
        const index = next.indexOf(value);
        if (index >= 0)
            next.splice(index, 1);
        else
            next.push(value);
        S.Prefs.set(key, next);
    }

    Column {
        id: content
        width: root.width
        spacing: K.Style.s5

        P.PageHead {
            width: parent.width
            title: "Modules"
            description: "Choose what each module shows and how much room it takes."
        }

        SettingCard {
            width: parent.width
            title: "WORKSPACES"
            collapsible: false

            SettingRow {
                width: parent.width
                label: "Style"
                desc: "Choose how workspaces are drawn in the strip."
                block: true

                Flow {
                    id: styleFlow
                    readonly property real tileWidth: Math.floor((width - 2 * spacing) / 3)
                    width: parent.width
                    spacing: K.Style.s3

                    Repeater {
                        model: root.workspaceStyles

                        delegate: K.Tile {
                            id: workspaceTile
                            required property var modelData
                            width: styleFlow.tileWidth
                            label: modelData.label
                            selected: S.Prefs.workspaceStyle === modelData.value
                            onClicked: S.Prefs.set("workspaceStyle", modelData.value)

                            W.Strip {
                                anchors.centerIn: parent
                                style: workspaceTile.modelData.value
                                model: root.workspaceSample
                                interactive: false
                                square: S.Prefs.form === "square"
                                unit: 28
                            }
                        }
                    }
                }
            }

            SettingRow {
                width: parent.width
                label: "Show empty"
                desc: "Keep unused workspaces visible in the strip."
                controlWidth: 54
                divider: true

                K.Switch {
                    anchors.right: parent.right
                    checked: S.Prefs.workspaceShowEmpty
                    onToggled: value => S.Prefs.set("workspaceShowEmpty", value)
                }
            }

            SettingRow {
                width: parent.width
                visible: S.Prefs.workspaceShowEmpty
                label: "Spare workspace at the end"
                desc: "Show one extra empty workspace after the last one in use. Off stops the strip at the last workspace that exists."
                controlWidth: 54

                K.Switch {
                    anchors.right: parent.right
                    checked: S.Prefs.workspaceSpare
                    onToggled: value => S.Prefs.set("workspaceSpare", value)
                }
            }
        }

        SettingCard {
            width: parent.width
            title: "VOICE"
            collapsible: false

            SettingRow {
                width: parent.width
                label: "Date"
                desc: "Show the date beside the idle time."
                controlWidth: 54
                K.Switch {
                    anchors.right: parent.right
                    checked: S.Prefs.showDate
                    onToggled: value => S.Prefs.set("showDate", value)
                }
            }

            SettingRow {
                width: parent.width
                label: "Seconds"
                desc: "Include seconds in the idle clock."
                controlWidth: 54
                divider: true
                K.Switch {
                    anchors.right: parent.right
                    checked: S.Prefs.seconds
                    onToggled: value => S.Prefs.set("seconds", value)
                }
            }

            SettingRow {
                width: parent.width
                label: "12-hour clock"
                desc: "Use a shorter clock cycle."
                controlWidth: 54
                divider: true
                K.Switch {
                    anchors.right: parent.right
                    checked: S.Prefs.clock12h
                    onToggled: value => S.Prefs.set("clock12h", value)
                }
            }

            SettingRow {
                width: parent.width
                label: "Weather in the voice"
                desc: "Show current weather while the voice is idle."
                controlWidth: 54
                divider: true
                K.Switch {
                    anchors.right: parent.right
                    checked: S.Prefs.voiceWeather
                    onToggled: value => S.Prefs.set("voiceWeather", value)
                }
            }

            SettingRow {
                width: parent.width
                label: "Spin cover"
                desc: "Turn the artwork while music is playing."
                controlWidth: 54
                divider: true
                K.Switch {
                    anchors.right: parent.right
                    checked: S.Prefs.coverSpin
                    onToggled: value => S.Prefs.set("coverSpin", value)
                }
            }

            SettingRow {
                width: parent.width
                label: "Wavy progress"
                desc: "Use a moving wave for playback progress."
                controlWidth: 54
                divider: true
                K.Switch {
                    anchors.right: parent.right
                    checked: S.Prefs.wavy
                    onToggled: value => S.Prefs.set("wavy", value)
                }
            }

            SettingRow {
                width: parent.width
                label: "Title width"
                desc: "Limit the room used by the current track title."
                value: String(Math.round(S.Prefs.mediaWidth))
                unit: "px"
                controlWidth: 58
                divider: true

                Step {
                    anchors.right: parent.right
                    value: Math.round(S.Prefs.mediaWidth)
                    from: 120
                    to: 320
                    stepBy: 10
                    onModified: value => S.Prefs.set("mediaWidth", value)
                }
            }
        }

        SettingCard {
            width: parent.width
            title: "STATUS ICONS"
            collapsible: false

            SettingRow {
                width: parent.width
                label: "Visible icons"
                desc: "Choose what appears in the status cluster."
                block: true

                Flow {
                    width: parent.width
                    spacing: K.Style.s2

                    Repeater {
                        model: root.statusChoices
                        delegate: K.Chip {
                            required property var modelData
                            text: modelData.label
                            icon: modelData.icon
                            selected: root.contains(S.Prefs.statusItems, modelData.value)
                            onClicked: root.toggleList("statusItems", modelData.value)
                        }
                    }
                }
            }
        }

        SettingCard {
            width: parent.width
            title: "TRAY"
            collapsible: false

            SettingRow {
                width: parent.width
                label: "Visible items"
                desc: "Choose how many icons remain outside overflow."
                value: String(S.Prefs.trayLimit)
                controlWidth: 58

                Step {
                    anchors.right: parent.right
                    value: S.Prefs.trayLimit
                    from: 3
                    to: 12
                    stepBy: 1
                    onModified: value => S.Prefs.set("trayLimit", value)
                }
            }
        }

        SettingCard {
            width: parent.width
            title: "WINDOW"
            collapsible: false

            SettingRow {
                width: parent.width
                label: "Title width"
                desc: "Limit the room used by the focused window."
                value: String(Math.round(S.Prefs.windowWidth))
                unit: "px"
                controlWidth: 58

                Step {
                    anchors.right: parent.right
                    value: Math.round(S.Prefs.windowWidth)
                    from: 120
                    to: 420
                    stepBy: 10
                    onModified: value => S.Prefs.set("windowWidth", value)
                }
            }
        }

        SettingCard {
            width: parent.width
            title: "SYSTEM READINGS"
            collapsible: false

            SettingRow {
                width: parent.width
                label: "Readouts"
                desc: "Choose the figures shown by the system module."
                block: true

                Flow {
                    width: parent.width
                    spacing: K.Style.s2
                    Repeater {
                        model: root.readingChoices
                        delegate: K.Chip {
                            required property var modelData
                            text: modelData.label
                            icon: modelData.icon
                            selected: root.contains(S.Prefs.sysmonItems, modelData.value)
                            onClicked: root.toggleList("sysmonItems", modelData.value)
                        }
                    }
                }
            }
        }

        SettingCard {
            width: parent.width
            title: "AI USAGE"
            collapsible: false

            SettingRow {
                width: parent.width
                label: "Tools"
                desc: "Choose which coding agents the module counts. A tool with no usage data stays hidden; none selected hides the module."
                block: true

                Flow {
                    width: parent.width
                    spacing: K.Style.s2
                    Repeater {
                        model: root.aiChoices
                        delegate: K.Chip {
                            required property var modelData
                            text: modelData.label
                            icon: modelData.icon
                            selected: root.contains(S.Prefs.aiTools, modelData.value)
                            onClicked: root.toggleList("aiTools", modelData.value)
                        }
                    }
                }
            }
        }

        SettingCard {
            width: parent.width
            title: "SHORTCUTS"
            collapsible: false

            SettingRow {
                width: parent.width
                label: "Actions"
                desc: "Choose the shortcuts shown in the actions module."
                block: true

                Flow {
                    width: parent.width
                    spacing: K.Style.s2
                    Repeater {
                        model: root.actionChoices
                        delegate: K.Chip {
                            required property var modelData
                            text: modelData.label
                            icon: modelData.icon
                            selected: root.contains(S.Prefs.actions, modelData.value)
                            onClicked: root.toggleList("actions", modelData.value)
                        }
                    }
                }
            }
        }
    }
}
