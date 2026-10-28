import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import stage.services
import stage.modules.common
import stage.modules.common.widgets
import stage.modules.ii.background.shortcuts

StyledFlickable {
    id: root
    required property string screenName
    signal addAppsToFolder(string folderId)

    readonly property var options: Config.options.background.desktopIcons
    readonly property var sources: DesktopShortcuts.otherScreens(root.screenName)

    contentWidth: width
    contentHeight: content.implicitHeight + Tokens.s4
    clip: true

    Column {
        id: content
        x: Tokens.s1
        width: Math.max(0, root.width - Tokens.s2)
        spacing: Tokens.s4

        Section {
            width: parent.width
            title: Translation.tr("DESKTOP")

            SettingCard {
                width: parent.width
                title: Translation.tr("BASICS")
                collapsible: false

                SettingRow {
                    width: parent.width
                    label: Translation.tr("Show icons")
                    desc: Translation.tr("Keep shortcuts visible on this display")
                    controlWidth: showIcons.implicitWidth
                    Sw {
                        id: showIcons
                        anchors.centerIn: parent
                        on: !DesktopShortcuts.hidden
                        onToggled: DesktopShortcuts.setHidden(!v)
                    }
                }
                SettingRow {
                    width: parent.width
                    divider: true
                    label: Translation.tr("Lock positions")
                    desc: Translation.tr("Clicks still work while icons stay in place")
                    controlWidth: lockIcons.implicitWidth
                    Sw {
                        id: lockIcons
                        anchors.centerIn: parent
                        on: Config.options.background.desktopIconsLocked ?? false
                        onToggled: Config.options.background.desktopIconsLocked = v
                    }
                }
                SettingRow {
                    width: parent.width
                    divider: true
                    label: Translation.tr("Open desktop icons")
                    desc: Translation.tr("Choose whether one click selects or opens an icon")
                    block: true
                    Seg {
                        width: parent.width
                        options: ["single", "double"]
                        labels: ({
                            "single": Translation.tr("Single click"),
                            "double": Translation.tr("Double click")
                        })
                        current: root.options.openOn
                        onChose: key => root.options.openOn = key
                    }
                }
                SettingRow {
                    width: parent.width
                    divider: true
                    label: Translation.tr("Icon size")
                    desc: Translation.tr("Scale icons and their spacing together")
                    value: `${DesktopShortcuts.iconScale}×`
                    block: true
                    Chips {
                        width: parent.width
                        options: ["0.75", "1", "1.25", "1.5", "1.75", "2"]
                        labels: ({
                            "0.75": "0.75×", "1": "1×", "1.25": "1.25×",
                            "1.5": "1.5×", "1.75": "1.75×", "2": "2×"
                        })
                        current: String(DesktopShortcuts.iconScale)
                        onChose: key => Config.options.background.desktopIconScale = Number(key)
                    }
                }
                SettingRow {
                    width: parent.width
                    divider: true
                    label: Translation.tr("Undo icon change")
                    desc: DesktopShortcuts.canUndo
                        ? Translation.tr("Restore the previous icon arrangement")
                        : Translation.tr("No icon changes to undo")
                    controlWidth: undoButton.implicitWidth
                    Btn {
                        id: undoButton
                        anchors.centerIn: parent
                        compact: true
                        armed: DesktopShortcuts.canUndo
                        text: Translation.tr("UNDO")
                        onAct: DesktopShortcuts.undo()
                    }
                }
            }
        }

        Section {
            width: parent.width
            title: Translation.tr("ARRANGEMENT")

            SettingCard {
                width: parent.width
                title: Translation.tr("LAYOUT")
                collapsible: false

                SettingRow {
                    width: parent.width
                    label: Translation.tr("Sort by")
                    desc: root.options.sortDescending
                        ? Translation.tr("Current direction: descending")
                        : Translation.tr("Current direction: ascending")
                    block: true
                    Seg {
                        width: parent.width
                        options: ["name", "type", "added", "used"]
                        labels: ({
                            "name": Translation.tr("Name"),
                            "type": Translation.tr("Type"),
                            "added": Translation.tr("Added"),
                            "used": Translation.tr("Used")
                        })
                        current: root.options.sortBy
                        onChose: key => DesktopShortcuts.sortBy(root.screenName, key)
                    }
                }
                SettingRow {
                    width: parent.width
                    divider: true
                    label: Translation.tr("Keep sorted")
                    desc: Translation.tr("Re-sort the desktop and folder contents after changes")
                    controlWidth: keepSorted.implicitWidth
                    Sw {
                        id: keepSorted
                        anchors.centerIn: parent
                        on: root.options.keepSorted
                        onToggled: DesktopShortcuts.setKeepSorted(v)
                    }
                }
                SettingRow {
                    width: parent.width
                    divider: true
                    label: Translation.tr("Auto-arrange")
                    desc: Translation.tr("Pack icons from the chosen corner and snap future drops")
                    controlWidth: autoArrange.implicitWidth
                    Sw {
                        id: autoArrange
                        anchors.centerIn: parent
                        on: root.options.autoArrange
                        onToggled: DesktopShortcuts.setAutoArrange(v)
                    }
                }
                SettingRow {
                    width: parent.width
                    divider: true
                    label: Translation.tr("Align now")
                    desc: Translation.tr("Move every icon to its nearest clear grid cell")
                    controlWidth: alignButton.implicitWidth
                    Btn {
                        id: alignButton
                        anchors.centerIn: parent
                        compact: true
                        text: Translation.tr("ALIGN")
                        onAct: DesktopShortcuts.alignToGrid(root.screenName)
                    }
                }
                SettingRow {
                    width: parent.width
                    divider: true
                    label: Translation.tr("Start from")
                    desc: Translation.tr("Move the current layout to this corner")
                    block: true
                    Seg {
                        width: parent.width
                        options: ["topLeft", "topRight", "bottomLeft", "bottomRight"]
                        labels: ({
                            "topLeft": Translation.tr("Top left"),
                            "topRight": Translation.tr("Top right"),
                            "bottomLeft": Translation.tr("Bottom left"),
                            "bottomRight": Translation.tr("Bottom right")
                        })
                        current: root.options.origin
                        onChose: key => DesktopShortcuts.setOrigin(key)
                    }
                }
                SettingRow {
                    width: parent.width
                    divider: true
                    label: Translation.tr("Fill direction")
                    desc: Translation.tr("Reflow the current layout by columns or rows")
                    block: true
                    Seg {
                        width: parent.width
                        options: ["columns", "rows"]
                        labels: ({
                            "columns": Translation.tr("Columns"),
                            "rows": Translation.tr("Rows")
                        })
                        current: root.options.flow
                        onChose: key => DesktopShortcuts.setFlow(key)
                    }
                }
                SettingRow {
                    width: parent.width
                    divider: true
                    label: Translation.tr("Spacing")
                    desc: Translation.tr("Reflow every display with this grid spacing")
                    block: true
                    Seg {
                        width: parent.width
                        options: ["compact", "normal", "wide"]
                        labels: ({
                            "compact": Translation.tr("Compact"),
                            "normal": Translation.tr("Normal"),
                            "wide": Translation.tr("Wide")
                        })
                        current: root.options.spacing
                        onChose: key => DesktopShortcuts.setSpacing(key)
                    }
                }
                SettingRow {
                    width: parent.width
                    divider: true
                    label: Translation.tr("Edge margin")
                    desc: Translation.tr("Settle icons inside this distance from every edge")
                    value: String(root.options.margin)
                    unit: "px"
                    controlWidth: marginStep.implicitWidth
                    Step {
                        id: marginStep
                        anchors.centerIn: parent
                        value: root.options.margin
                        from: 0
                        to: 120
                        stepBy: 8
                        onModified: v => DesktopShortcuts.setMargin(v)
                    }
                }
                SettingRow {
                    width: parent.width
                    divider: true
                    label: Translation.tr("Keep clear of panels")
                    desc: Translation.tr("Settle icons away from the bar and dock")
                    controlWidth: avoidPanels.implicitWidth
                    Sw {
                        id: avoidPanels
                        anchors.centerIn: parent
                        on: root.options.avoidPanels
                        onToggled: DesktopShortcuts.setAvoidPanels(v)
                    }
                }
            }
        }

        Section {
            width: parent.width
            title: Translation.tr("FOLDERS")

            SettingCard {
                width: parent.width
                title: Translation.tr("ORGANISE")
                collapsible: false

                SettingRow {
                    width: parent.width
                    label: Translation.tr("New folder")
                    desc: Translation.tr("Create an empty folder, then choose its apps")
                    controlWidth: newFolderButton.implicitWidth
                    Btn {
                        id: newFolderButton
                        anchors.centerIn: parent
                        compact: true
                        text: Translation.tr("NEW")
                        onAct: {
                            const folderId = DesktopShortcuts.newFolder(root.screenName, "");
                            if (folderId)
                                root.addAppsToFolder(folderId);
                        }
                    }
                }
                SettingRow {
                    width: parent.width
                    divider: true
                    label: Translation.tr("Stacks")
                    desc: Translation.tr("Gather loose apps, folders and files into one stack each; your folders stay as they are")
                    controlWidth: stacks.implicitWidth
                    Sw {
                        id: stacks
                        anchors.centerIn: parent
                        on: root.options.stacks
                        onToggled: DesktopShortcuts.setStacks(v)
                    }
                }
                SettingRow {
                    width: parent.width
                    visible: root.options.stacks
                    divider: true
                    label: Translation.tr("Sorting also orders what is inside each stack")
                }
            }
        }

        Section {
            width: parent.width
            title: Translation.tr("LABELS & MARKS")

            SettingCard {
                width: parent.width
                title: Translation.tr("APPEARANCE")
                collapsible: false

                SettingRow {
                    width: parent.width
                    label: Translation.tr("Labels")
                    desc: Translation.tr("Choose when icon names appear")
                    block: true
                    Seg {
                        width: parent.width
                        options: ["always", "hover", "never"]
                        labels: ({
                            "always": Translation.tr("Always"),
                            "hover": Translation.tr("On hover"),
                            "never": Translation.tr("Never")
                        })
                        current: root.options.labels
                        onChose: key => root.options.labels = key
                    }
                }
                SettingRow {
                    width: parent.width
                    visible: root.options.labels !== "never"
                    divider: true
                    label: Translation.tr("Label lines")
                    desc: Translation.tr("Allow one or two lines for long names")
                    block: true
                    Seg {
                        width: parent.width
                        options: ["1", "2"]
                        labels: ({ "1": Translation.tr("One"), "2": Translation.tr("Two") })
                        current: String(root.options.labelLines)
                        onChose: key => root.options.labelLines = Number(key)
                    }
                }
                SettingRow {
                    width: parent.width
                    visible: root.options.labels !== "never"
                    divider: true
                    label: Translation.tr("Label style")
                    desc: Translation.tr("Auto adapts the label treatment to the wallpaper")
                    block: true
                    Seg {
                        width: parent.width
                        options: ["auto", "shadow", "pill"]
                        labels: ({
                            "auto": Translation.tr("Auto"),
                            "shadow": Translation.tr("Shadow"),
                            "pill": Translation.tr("Pill")
                        })
                        current: root.options.labelStyle
                        onChose: key => root.options.labelStyle = key
                    }
                }
                SettingRow {
                    width: parent.width
                    divider: true
                    label: Translation.tr("Icon background")
                    desc: Translation.tr("Choose the plate behind each icon")
                    block: true
                    Seg {
                        width: parent.width
                        options: ["none", "translucent", "circle", "squircle"]
                        labels: ({
                            "none": Translation.tr("None"),
                            "translucent": Translation.tr("Soft"),
                            "circle": Translation.tr("Circle"),
                            "squircle": Translation.tr("Square")
                        })
                        current: root.options.iconBackground
                        onChose: key => root.options.iconBackground = key
                    }
                }
                SettingRow {
                    width: parent.width
                    divider: true
                    label: Translation.tr("Running indicator")
                    desc: Translation.tr("Mark apps that are currently open")
                    controlWidth: runningBadge.implicitWidth
                    Sw {
                        id: runningBadge
                        anchors.centerIn: parent
                        on: root.options.runningBadges
                        onToggled: root.options.runningBadges = v
                    }
                }
                SettingRow {
                    width: parent.width
                    divider: true
                    label: Translation.tr("Notification count")
                    desc: Translation.tr("Show unread counts on app icons")
                    controlWidth: notificationBadge.implicitWidth
                    Sw {
                        id: notificationBadge
                        anchors.centerIn: parent
                        on: root.options.notificationBadges
                        onToggled: root.options.notificationBadges = v
                    }
                }
            }
        }

        Section {
            width: parent.width
            visible: root.sources.length > 0
            title: Translation.tr("OTHER DISPLAYS")

            SettingCard {
                width: parent.width
                title: Translation.tr("BRING ICONS HERE")
                collapsible: false

                Repeater {
                    model: root.sources
                    delegate: SettingRow {
                        required property string modelData
                        required property int index
                        readonly property int iconCount: DesktopShortcuts.itemsFor(modelData).length
                        width: parent.width
                        divider: index > 0
                        label: modelData
                        desc: DesktopShortcuts.isConnected(modelData)
                            ? Translation.tr("%1 icons").arg(String(iconCount))
                            : Translation.tr("%1 icons, display disconnected").arg(String(iconCount))
                        controlWidth: bringButton.implicitWidth
                        Btn {
                            id: bringButton
                            anchors.centerIn: parent
                            compact: true
                            text: Translation.tr("BRING")
                            onAct: DesktopShortcuts.moveToScreen(modelData, root.screenName, null)
                        }
                    }
                }
            }
        }
    }
}
