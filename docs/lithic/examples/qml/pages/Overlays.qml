import QtQuick
import Lithic

Item {
    id: root

    implicitHeight: content.implicitHeight + LithicTokens.space.s5 * 2

    component SpecimenFrame: CutRect {
        id: frame

        required property string title
        property string stateLabel: "Open"
        property real stageHeight: LithicTokens.space.s6 * 4
        property alias stageItem: stage
        default property alias specimen: stage.data

        implicitHeight: titleLabel.implicitHeight + LithicTokens.space.s2
                        + stageHeight + LithicTokens.space.s4 * 2
        height: implicitHeight
        fill: Lithic.slab
        strokeColor: Lithic.vein
        strokeWidth: LithicTokens.size.hairline
        cut: LithicTokens.cut.large

        Label {
            id: titleLabel
            x: LithicTokens.space.s4
            y: LithicTokens.space.s4
            text: frame.title
            typeRole: "section"
        }

        Label {
            anchors.right: parent.right
            anchors.rightMargin: LithicTokens.space.s4
            anchors.verticalCenter: titleLabel.verticalCenter
            text: frame.stateLabel
            typeRole: "caption"
            inkRole: "fg3"
        }

        Item {
            id: stage
            x: LithicTokens.space.s4
            y: titleLabel.y + titleLabel.height + LithicTokens.space.s2
            width: parent.width - LithicTokens.space.s4 * 2
            height: frame.stageHeight
        }
    }

    Column {
        id: content
        x: LithicTokens.space.s5
        y: LithicTokens.space.s5
        width: root.width - LithicTokens.space.s5 * 2
        spacing: LithicTokens.space.s5

        Column {
            width: parent.width
            spacing: LithicTokens.space.s2
            Label { text: "Overlays, navigation, data and frame"; typeRole: "name" }
            Label {
                width: parent.width
                text: "Open overlay states, fleet navigation, engraved data and the adaptive application frame."
                typeRole: "body"
                inkRole: "fg2"
            }
        }

        Grid {
            id: specimens
            width: parent.width
            columns: 2
            columnSpacing: LithicTokens.space.s4
            rowSpacing: LithicTokens.space.s4

            SpecimenFrame {
                id: popoverFrame
                width: (specimens.width - specimens.columnSpacing) / specimens.columns
                title: "Popover"

                Plaque {
                    id: popoverAnchor
                    x: (parent.width - width) / 2
                    y: LithicTokens.space.s5
                    kind: "quiet"
                    text: "Open ally actions"
                }
                Popover {
                    id: openPopover
                    parent: popoverFrame.stageItem
                    anchorItem: popoverAnchor
                    opener: popoverAnchor
                    previewEmbedded: true
                    Menu {
                        menuWidth: LithicTokens.size.menuWidthMin + LithicTokens.space.s5
                        items: [
                            { label: "Open ally", glyph: "dns", shortcut: "Enter" },
                            { label: "Copy 192.0.2.18", glyph: "content_copy" },
                            { divider: true },
                            { label: "Forget server", glyph: "delete", danger: true }
                        ]
                    }
                }
            }

            SpecimenFrame {
                width: (specimens.width - specimens.columnSpacing) / specimens.columns
                title: "Menu"
                Menu {
                    anchors.centerIn: parent
                    menuWidth: LithicTokens.size.menuWidthMin + LithicTokens.space.s6
                    previewState: "hover"
                    previewIndex: 1
                    items: [
                        { label: "Connect to prowl", glyph: "link", shortcut: "Enter" },
                        { label: "Open console", glyph: "terminal", shortcut: "Ctrl+O" },
                        { divider: true },
                        { label: "Remove localhost", glyph: "delete", danger: true },
                        { label: "Unavailable action", glyph: "block", disabled: true }
                    ]
                }
            }

            SpecimenFrame {
                id: pickerFrame
                width: (specimens.width - specimens.columnSpacing) / specimens.columns
                title: "Picker"
                stageHeight: LithicTokens.space.s6 * 6
                Picker {
                    x: LithicTokens.space.s5
                    y: LithicTokens.space.s5
                    width: LithicTokens.size.pickerWidthMin + LithicTokens.space.s6 * 2
                    label: "Machine"
                    options: ["ubuntu-24.04", "fedora-43", "windows-11", "alpine-instant"]
                    currentIndex: 1
                    overlayParent: pickerFrame.stageItem
                    previewOpen: true
                }
            }

            SpecimenFrame {
                id: colorFrame
                width: (specimens.width - specimens.columnSpacing) / specimens.columns
                title: "ColorField"
                stageHeight: LithicTokens.space.s6 * 9
                ColorField {
                    x: LithicTokens.space.s5
                    y: LithicTokens.space.s5
                    width: LithicTokens.size.settingsControlMax
                    label: "Accent colour"
                    color: Lithic.gold
                    allowAlpha: true
                    swatches: [
                        { label: "Gold", color: Lithic.gold },
                        { label: "Bone", color: Lithic.sel },
                        { label: "Jade", color: Lithic.jade },
                        { label: "Lapis", color: Lithic.lapis },
                        { label: "Amber", color: Lithic.amber }
                    ]
                    overlayParent: colorFrame.stageItem
                    previewOpen: true
                }
            }

            SpecimenFrame {
                id: dialogFrame
                width: (specimens.width - specimens.columnSpacing) / specimens.columns
                title: "Dialog"
                stageHeight: LithicTokens.space.s6 * 7

                Dialog {
                    parent: dialogFrame.stageItem
                    title: "Connect to ally"
                    previewEmbedded: true
                    actions: [
                        { text: "Cancel", kind: "quiet" },
                        { text: "Connect", kind: "gold" }
                    ]
                    Label {
                        width: parent.width
                        text: "Open ops@192.0.2.18 and use the saved fleet identity."
                        typeRole: "body"
                        inkRole: "fg2"
                        wrapMode: Text.Wrap
                    }
                }
            }

            SpecimenFrame {
                id: confirmFrame
                width: (specimens.width - specimens.columnSpacing) / specimens.columns
                title: "Confirm"
                stageHeight: LithicTokens.space.s6 * 7

                Confirm {
                    overlayParent: confirmFrame.stageItem
                    title: "Delete scratch-arch"
                    text: "This removes the machine record and its local snapshots."
                    actionText: "Delete machine"
                    requiredName: "scratch-arch"
                    destructive: true
                    previewEmbedded: true
                    previewOpen: true
                }
            }

            SpecimenFrame {
                id: tooltipFrame
                width: (specimens.width - specimens.columnSpacing) / specimens.columns
                title: "Tooltip"
                stageHeight: LithicTokens.space.s6 * 3

                Plaque {
                    id: tooltipAnchor
                    anchors.centerIn: parent
                    kind: "quiet"
                    glyph: "terminal"
                    text: "Console router"
                }
                Tooltip {
                    parent: tooltipFrame.stageItem
                    anchorItem: tooltipAnchor
                    text: "/dev/ttyUSB0"
                    previewOpen: true
                }
            }

            SpecimenFrame {
                id: placardFrame
                width: (specimens.width - specimens.columnSpacing) / specimens.columns
                title: "Placard"
                stageHeight: LithicTokens.space.s6 * 3

                Placard {
                    parent: placardFrame.stageItem
                    message: "fedora-43 moved to ally"
                    actionText: "Undo"
                    autoDismiss: false
                    remaining: 0.62
                    previewOpen: true
                }
            }

            SpecimenFrame {
                id: railFrame
                width: (specimens.width - specimens.columnSpacing) / specimens.columns
                title: "Rail"
                stateLabel: "Expanded, New menu open"
                stageHeight: LithicTokens.size.windowMinHeight

                Rail {
                    x: LithicTokens.space.s4
                    width: LithicTokens.size.railExpanded
                    height: parent.height
                    sizeClass: "expanded"
                    currentIndex: 0
                    overlayParent: railFrame.stageItem
                    previewNewOpen: true
                }
                Column {
                    x: LithicTokens.size.railExpanded + LithicTokens.space.s6
                    y: LithicTokens.space.s6
                    spacing: LithicTokens.space.s2
                    Label { text: "Fleet"; typeRole: "section" }
                    Label { text: "ally"; typeRole: "row" }
                    Label { text: "ops@192.0.2.18"; typeRole: "monoSm"; inkRole: "fg2" }
                }
            }

            SpecimenFrame {
                id: longRailFrame
                width: (specimens.width - specimens.columnSpacing) / specimens.columns
                title: "Rail"
                stateLabel: "Long, foot destination with an update notch, no New; expanded and medium"
                stageHeight: LithicTokens.size.windowDefaultHeight

                readonly property var settingsPages: [
                    { label: "Appearance", glyph: "palette" },
                    { label: "Bar", glyph: "view_day" },
                    { label: "Desktop", glyph: "desktop_windows" },
                    { label: "Displays", glyph: "monitor" },
                    { label: "Keyboard", glyph: "keyboard" },
                    { label: "Mouse", glyph: "mouse" },
                    { label: "Sound", glyph: "volume_up" },
                    { label: "Notices", glyph: "notifications" },
                    { label: "Updates", glyph: "system_update" }
                ]

                Rail {
                    x: LithicTokens.space.s4
                    width: LithicTokens.size.railExpanded
                    height: parent.height
                    sizeClass: "expanded"
                    destinations: longRailFrame.settingsPages
                    currentIndex: 2
                    showNew: false
                    footIndex: 8
                    notchIndex: 8
                }
                Rail {
                    x: LithicTokens.space.s4 + LithicTokens.size.railExpanded + LithicTokens.space.s6
                    width: LithicTokens.size.railMedium
                    height: parent.height
                    sizeClass: "medium"
                    destinations: longRailFrame.settingsPages
                    currentIndex: 8
                    showNew: false
                    footIndex: 8
                    notchIndex: 8
                }
            }

            SpecimenFrame {
                width: (specimens.width - specimens.columnSpacing) / specimens.columns
                title: "BrandMark"
                stateLabel: "Home"
                stageHeight: LithicTokens.space.s6 * 3

                BrandMark {
                    anchors.centerIn: parent
                    markSize: LithicTokens.size.brandMark
                    previewState: "focus"
                }
            }

            SpecimenFrame {
                width: (specimens.width - specimens.columnSpacing) / specimens.columns
                title: "NewPlaque"
                stateLabel: "Menu open"
                stageHeight: LithicTokens.space.s6 * 3

                Row {
                    anchors.centerIn: parent
                    spacing: LithicTokens.space.s5
                    NewPlaque { menuOpen: false }
                    NewPlaque { menuOpen: true }
                    NewPlaque { compact: true; menuOpen: false }
                }
            }

            SpecimenFrame {
                width: (specimens.width - specimens.columnSpacing) / specimens.columns
                title: "SectionIndex"
                stateLabel: "Wide list and medium picker"
                stageHeight: LithicTokens.space.s6 * 5

                Row {
                    anchors.fill: parent
                    spacing: LithicTokens.size.sectionIndexGap
                    SectionIndex {
                        sections: ["Connection", "Identity", "Storage", "Updates"]
                        currentIndex: 1
                        sizeClass: "expanded"
                    }
                    Column {
                        width: parent.width - LithicTokens.size.sectionIndexWidth
                               - parent.spacing
                        spacing: LithicTokens.space.s3
                        Label { text: "Medium"; typeRole: "caption"; inkRole: "fg3" }
                        SectionIndex {
                            width: parent.width
                            sections: ["Connection", "Identity", "Storage"]
                            currentIndex: 1
                            sizeClass: "medium"
                        }
                    }
                }
            }

            SpecimenFrame {
                width: (specimens.width - specimens.columnSpacing) / specimens.columns
                title: "EngravedChart"
                stateLabel: "Fleet load"
                stageHeight: LithicTokens.space.s6 * 5

                EngravedChart {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    height: LithicTokens.space.s6 * 3
                    values: [18, 24, 21, 38, 34, 49, 45]
                    ticks: ["09:00", "12:00", "15:00", "18:00"]
                }
            }

            SpecimenFrame {
                width: (specimens.width - specimens.columnSpacing) / specimens.columns
                title: "AppFrame"
                stateLabel: "Compact"
                stageHeight: LithicTokens.size.windowMinHeight

                AppFrame {
                    anchors.fill: parent
                    showList: true
                    compactView: "work"
                    currentDestination: 0

                    Column {
                        width: parent.width
                        spacing: LithicTokens.space.s3
                        Label { text: "Fleet"; typeRole: "sentence" }
                        Label { text: "ubuntu-24.04"; typeRole: "row" }
                        Label { text: "ally at ops@192.0.2.18"; typeRole: "monoSm"; inkRole: "fg2" }
                        Blade {
                            width: parent.width
                            fraction: 0.68
                            fill: Lithic.fg2
                        }
                    }
                }
            }

            SpecimenFrame {
                width: (specimens.width - specimens.columnSpacing) / specimens.columns
                title: "AppFrame"
                stateLabel: "Compact, no bottom bar; the host brings its own navigation"
                stageHeight: LithicTokens.size.windowMinHeight

                AppFrame {
                    anchors.fill: parent
                    showList: false
                    compactBar: false

                    Column {
                        width: parent.width
                        spacing: LithicTokens.space.s3
                        Label { text: "Settings"; typeRole: "sentence" }
                        Picker { width: parent.width; options: ["Appearance", "Bar", "Desktop"]; currentIndex: 0 }
                    }
                }
            }
        }
    }
}
