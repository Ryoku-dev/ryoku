import QtQuick
import Lithic

Item {
    id: root

    implicitHeight: content.implicitHeight + LithicTokens.space.s5 * 2

    component StateSpecimen: Column {
        required property string stateName
        property real specimenHeight: LithicTokens.size.emptyFrame
        property real specimenWidth: LithicTokens.size.searchWidth
        default property alias specimen: specimenHost.data

        width: specimenWidth
        spacing: LithicTokens.space.s2

        Item {
            id: specimenHost
            width: parent.width
            height: parent.specimenHeight
        }

        Label {
            width: parent.width
            text: parent.stateName
            typeRole: "caption"
            inkRole: "fg2"
            horizontalAlignment: Text.AlignHCenter
        }
    }

    component ComponentSection: Column {
        required property string sectionName
        default property alias specimens: specimenFlow.data

        width: parent.width
        spacing: LithicTokens.space.s4

        Label {
            text: parent.sectionName
            typeRole: "section"
        }

        Flow {
            id: specimenFlow
            width: parent.width
            height: childrenRect.height
            spacing: LithicTokens.space.s3
        }
    }

    Column {
        id: content
        x: LithicTokens.space.s5
        y: LithicTokens.space.s5
        width: root.width - LithicTokens.space.s5 * 2
        spacing: LithicTokens.space.s6

        Column {
            width: parent.width
            spacing: LithicTokens.space.s2

            Label {
                text: "Controls"
                typeRole: "name"
            }
            Label {
                width: parent.width
                text: "Buttons, fields, choices and disclosure controls for the Ryoport fleet."
                typeRole: "body"
                inkRole: "fg2"
            }
        }

        ComponentSection {
            sectionName: "Plaque"

            StateSpecimen {
                stateName: "Gold"
                Plaque { anchors.centerIn: parent; kind: "gold"; glyph: "link"; text: "Connect" }
            }
            StateSpecimen {
                stateName: "Line"
                Plaque { anchors.centerIn: parent; kind: "line"; glyph: "terminal"; text: "Open console" }
            }
            StateSpecimen {
                stateName: "Tonal"
                Plaque { anchors.centerIn: parent; kind: "tonal"; glyph: "content_copy"; text: "Copy address" }
            }
            StateSpecimen {
                stateName: "Quiet"
                Plaque { anchors.centerIn: parent; kind: "quiet"; text: "Cancel" }
            }
            StateSpecimen {
                stateName: "On"
                Plaque { anchors.centerIn: parent; kind: "on"; glyph: "check"; text: "Running" }
            }
            StateSpecimen {
                stateName: "Danger"
                Plaque { anchors.centerIn: parent; kind: "danger"; glyph: "stop"; text: "Stop" }
            }
            StateSpecimen {
                stateName: "Danger line"
                Plaque { anchors.centerIn: parent; kind: "dangerLine"; glyph: "delete"; text: "Remove" }
            }
            StateSpecimen {
                stateName: "Small"
                Plaque { anchors.centerIn: parent; kind: "quiet"; small: true; glyph: "content_copy"; text: "Copy address" }
            }
            StateSpecimen {
                stateName: "Busy"
                Plaque { anchors.centerIn: parent; kind: "gold"; text: "Connecting"; busy: true }
            }
            StateSpecimen {
                stateName: "Disabled"
                Plaque { anchors.centerIn: parent; kind: "line"; text: "Reconnect"; enabled: false }
            }
            StateSpecimen {
                stateName: "Hover preview"
                Plaque { anchors.centerIn: parent; kind: "quiet"; glyph: "refresh"; text: "Refresh"; previewState: "hover" }
            }
            StateSpecimen {
                stateName: "Tonal hover preview"
                Plaque { anchors.centerIn: parent; kind: "tonal"; glyph: "content_copy"; text: "Copy address"; previewState: "hover" }
            }
            StateSpecimen {
                stateName: "Pressed preview"
                Plaque { anchors.centerIn: parent; kind: "quiet"; glyph: "refresh"; text: "Refresh"; previewState: "pressed" }
            }
            StateSpecimen {
                stateName: "Keyboard focus preview"
                Plaque { anchors.centerIn: parent; kind: "line"; glyph: "terminal"; text: "Console"; previewState: "focus" }
            }
        }

        ComponentSection {
            sectionName: "SplitPlaque"

            StateSpecimen {
                stateName: "Closed"
                SplitPlaque { anchors.centerIn: parent; glyph: "link"; text: "Connect" }
            }
            StateSpecimen {
                stateName: "Menu open"
                SplitPlaque { anchors.centerIn: parent; glyph: "link"; text: "Connect"; menuOpen: true }
            }
            StateSpecimen {
                stateName: "Tonal"
                SplitPlaque { anchors.centerIn: parent; kind: "tonal"; glyph: "swap_horiz"; text: "Transfer" }
            }
            StateSpecimen {
                stateName: "Main hover preview"
                SplitPlaque { anchors.centerIn: parent; kind: "line"; glyph: "terminal"; text: "Open console"; previewState: "hover" }
            }
            StateSpecimen {
                stateName: "More focus preview"
                SplitPlaque { anchors.centerIn: parent; kind: "line"; glyph: "terminal"; text: "Open console"; morePreviewState: "focus" }
            }
            StateSpecimen {
                stateName: "Busy"
                SplitPlaque { anchors.centerIn: parent; text: "Connecting"; busy: true }
            }
            StateSpecimen {
                stateName: "Disabled"
                SplitPlaque { anchors.centerIn: parent; kind: "quiet"; text: "Connect"; enabled: false }
            }
        }

        ComponentSection {
            sectionName: "IconPlaque"

            StateSpecimen {
                stateName: "Quiet"
                IconPlaque { anchors.centerIn: parent; kind: "quiet"; glyph: "refresh"; accessibleName: "Refresh" }
            }
            StateSpecimen {
                stateName: "Tonal"
                IconPlaque { anchors.centerIn: parent; kind: "tonal"; glyph: "content_copy"; accessibleName: "Copy" }
            }
            StateSpecimen {
                stateName: "Gold"
                IconPlaque { anchors.centerIn: parent; kind: "gold"; glyph: "play_arrow"; accessibleName: "Start" }
            }
            StateSpecimen {
                stateName: "Attention notch"
                IconPlaque { anchors.centerIn: parent; kind: "quiet"; glyph: "system_update"; notchTone: "attention"; accessibleName: "Update available" }
            }
            StateSpecimen {
                stateName: "Small"
                IconPlaque { anchors.centerIn: parent; kind: "quiet"; small: true; glyph: "content_copy"; accessibleName: "Copy" }
            }
            StateSpecimen {
                stateName: "Hover preview"
                IconPlaque { anchors.centerIn: parent; kind: "quiet"; glyph: "more_horiz"; accessibleName: "More"; previewState: "hover" }
            }
            StateSpecimen {
                stateName: "Pressed preview"
                IconPlaque { anchors.centerIn: parent; kind: "quiet"; glyph: "more_horiz"; accessibleName: "More"; previewState: "pressed" }
            }
            StateSpecimen {
                stateName: "Keyboard focus preview"
                IconPlaque { anchors.centerIn: parent; kind: "line"; glyph: "terminal"; accessibleName: "Console"; previewState: "focus" }
            }
            StateSpecimen {
                stateName: "Disabled"
                IconPlaque { anchors.centerIn: parent; kind: "quiet"; glyph: "refresh"; accessibleName: "Refresh"; enabled: false }
            }
        }

        ComponentSection {
            sectionName: "TextField"

            StateSpecimen {
                stateName: "Default"
                specimenHeight: LithicTokens.space.s6 + LithicTokens.space.s5
                TextField { anchors.centerIn: parent; placeholderText: "Machine name" }
            }
            StateSpecimen {
                stateName: "Label"
                specimenHeight: LithicTokens.space.s6 + LithicTokens.space.s5
                TextField { anchors.centerIn: parent; label: "Machine"; text: "ubuntu-24.04" }
            }
            StateSpecimen {
                stateName: "Help"
                specimenHeight: LithicTokens.space.s6 + LithicTokens.space.s5
                TextField { anchors.centerIn: parent; label: "Server address"; text: "192.0.2.18"; helpText: "Use a hostname or address" }
            }
            StateSpecimen {
                stateName: "Error"
                specimenHeight: LithicTokens.space.s6 + LithicTokens.space.s5
                TextField { anchors.centerIn: parent; label: "Server address"; text: "192.0.2"; errorText: "Enter a complete address" }
            }
            StateSpecimen {
                stateName: "Secret"
                specimenHeight: LithicTokens.space.s6 + LithicTokens.space.s5
                TextField { anchors.centerIn: parent; label: "Access key"; text: "fleet-key"; secret: true }
            }
            StateSpecimen {
                stateName: "Revealed secret"
                specimenHeight: LithicTokens.space.s6 + LithicTokens.space.s5
                TextField { anchors.centerIn: parent; label: "Access key"; text: "fleet-key"; secret: true; revealed: true }
            }
            StateSpecimen {
                stateName: "Hover preview"
                specimenHeight: LithicTokens.space.s6 + LithicTokens.space.s5
                TextField { anchors.centerIn: parent; text: "fedora-43"; previewState: "hover" }
            }
            StateSpecimen {
                stateName: "Pressed preview"
                specimenHeight: LithicTokens.space.s6 + LithicTokens.space.s5
                TextField { anchors.centerIn: parent; text: "fedora-43"; previewState: "pressed" }
            }
            StateSpecimen {
                stateName: "Keyboard focus preview"
                specimenHeight: LithicTokens.space.s6 + LithicTokens.space.s5
                TextField { anchors.centerIn: parent; text: "fedora-43"; previewState: "focus" }
            }
            StateSpecimen {
                stateName: "Disabled"
                specimenHeight: LithicTokens.space.s6 + LithicTokens.space.s5
                TextField { anchors.centerIn: parent; text: "windows-11"; enabled: false }
            }
        }

        ComponentSection {
            sectionName: "Search"

            StateSpecimen {
                stateName: "Empty with hint"
                Search { anchors.centerIn: parent; placeholderText: "Search machines"; shortcutHint: "Ctrl K" }
            }
            StateSpecimen {
                stateName: "Query with clear"
                Search { anchors.centerIn: parent; text: "alpine"; accessibleName: "Search machines" }
            }
            StateSpecimen {
                stateName: "Hover preview"
                Search { anchors.centerIn: parent; placeholderText: "Search servers"; previewState: "hover" }
            }
            StateSpecimen {
                stateName: "Pressed preview"
                Search { anchors.centerIn: parent; placeholderText: "Search servers"; previewState: "pressed" }
            }
            StateSpecimen {
                stateName: "Keyboard focus preview"
                Search { anchors.centerIn: parent; text: "ally"; previewState: "focus" }
            }
            StateSpecimen {
                stateName: "Disabled"
                Search { anchors.centerIn: parent; text: "prowl"; enabled: false }
            }
        }

        ComponentSection {
            sectionName: "Toggle"

            StateSpecimen {
                stateName: "Off"
                Toggle { anchors.centerIn: parent; text: "Autostart" }
            }
            StateSpecimen {
                stateName: "On"
                Toggle { anchors.centerIn: parent; text: "Autostart"; checked: true }
            }
            StateSpecimen {
                stateName: "Hover preview"
                Toggle { anchors.centerIn: parent; text: "Keep connected"; previewState: "hover" }
            }
            StateSpecimen {
                stateName: "Pressed preview"
                Toggle { anchors.centerIn: parent; text: "Keep connected"; checked: true; previewState: "pressed" }
            }
            StateSpecimen {
                stateName: "Keyboard focus preview"
                Toggle { anchors.centerIn: parent; text: "Keep connected"; checked: true; previewState: "focus" }
            }
            StateSpecimen {
                stateName: "Disabled"
                Toggle { anchors.centerIn: parent; text: "Keep connected"; enabled: false }
            }
        }

        ComponentSection {
            sectionName: "Slider"

            StateSpecimen {
                stateName: "Minimum"
                Slider { anchors.centerIn: parent; value: 0; accessibleName: "Console scale" }
            }
            StateSpecimen {
                stateName: "Value 42"
                Slider { anchors.centerIn: parent; value: 42; accessibleName: "Console scale" }
            }
            StateSpecimen {
                stateName: "Maximum"
                Slider { anchors.centerIn: parent; value: 100; accessibleName: "Console scale" }
            }
            StateSpecimen {
                stateName: "Hover preview"
                Slider { anchors.centerIn: parent; value: 64; previewState: "hover" }
            }
            StateSpecimen {
                stateName: "Pressed preview"
                Slider { anchors.centerIn: parent; value: 64; previewState: "pressed" }
            }
            StateSpecimen {
                stateName: "Keyboard focus preview"
                Slider { anchors.centerIn: parent; value: 64; previewState: "focus" }
            }
            StateSpecimen {
                stateName: "Disabled"
                Slider { anchors.centerIn: parent; value: 64; enabled: false }
            }
        }

        ComponentSection {
            sectionName: "Stepper"

            StateSpecimen {
                stateName: "Minimum bound"
                Stepper { anchors.centerIn: parent; from: 1; to: 8; value: 1; unit: "cores"; accessibleName: "CPU cores" }
            }
            StateSpecimen {
                stateName: "Value"
                Stepper { anchors.centerIn: parent; from: 1; to: 8; value: 4; unit: "cores"; accessibleName: "CPU cores" }
            }
            StateSpecimen {
                stateName: "Maximum bound"
                Stepper { anchors.centerIn: parent; from: 1; to: 8; value: 8; unit: "cores"; accessibleName: "CPU cores" }
            }
            StateSpecimen {
                stateName: "Hover preview"
                Stepper { anchors.centerIn: parent; from: 1; to: 8; value: 4; unit: "cores"; previewState: "hover" }
            }
            StateSpecimen {
                stateName: "Pressed preview"
                Stepper { anchors.centerIn: parent; from: 1; to: 8; value: 4; unit: "cores"; previewState: "pressed" }
            }
            StateSpecimen {
                stateName: "Keyboard focus preview"
                Stepper { anchors.centerIn: parent; from: 1; to: 8; value: 4; unit: "cores"; previewState: "focus" }
            }
            StateSpecimen {
                stateName: "Disabled"
                Stepper { anchors.centerIn: parent; from: 1; to: 8; value: 4; unit: "cores"; enabled: false }
            }
        }

        ComponentSection {
            sectionName: "Filters"

            StateSpecimen {
                stateName: "Selected"
                Filters {
                    anchors.centerIn: parent
                    model: [{ label: "All", count: 6 }, { label: "Running", count: 2 }]
                }
            }
            StateSpecimen {
                stateName: "Second selected"
                Filters {
                    anchors.centerIn: parent
                    model: [{ label: "All", count: 6 }, { label: "Running", count: 2 }]
                    currentIndex: 1
                }
            }
            StateSpecimen {
                stateName: "Hover preview"
                Filters {
                    anchors.centerIn: parent
                    model: [{ label: "Needs you", count: 1 }, { label: "Stopped", count: 3 }]
                    currentIndex: 1
                    previewState: "hover"
                }
            }
            StateSpecimen {
                stateName: "Pressed preview"
                Filters {
                    anchors.centerIn: parent
                    model: [{ label: "Needs you", count: 1 }, { label: "Stopped", count: 3 }]
                    currentIndex: 1
                    previewState: "pressed"
                }
            }
            StateSpecimen {
                stateName: "Keyboard focus preview"
                Filters {
                    anchors.centerIn: parent
                    model: [{ label: "Local", count: 2 }, { label: "Remote", count: 4 }]
                    previewState: "focus"
                }
            }
            StateSpecimen {
                stateName: "Disabled"
                Filters {
                    anchors.centerIn: parent
                    model: [{ label: "Local", count: 2 }, { label: "Remote", count: 4 }]
                    enabled: false
                }
            }
        }

        ComponentSection {
            sectionName: "Segmented"

            StateSpecimen {
                stateName: "Three options"
                Segmented { anchors.centerIn: parent; model: ["Follow", "Dark", "Light"]; accessibleName: "Mode" }
            }
            StateSpecimen {
                stateName: "Second selected"
                Segmented { anchors.centerIn: parent; model: ["Standard", "High"]; currentIndex: 1; accessibleName: "Contrast" }
            }
            StateSpecimen {
                stateName: "Five options"
                specimenWidth: LithicTokens.size.listMedium + LithicTokens.space.s5
                Segmented { anchors.centerIn: parent; model: ["1 h", "6 h", "1 d", "7 d", "30 d"]; currentIndex: 2; accessibleName: "Range" }
            }
            StateSpecimen {
                stateName: "Hover preview"
                Segmented { anchors.centerIn: parent; model: ["Off", "Low", "Medium"]; currentIndex: 1; previewState: "hover" }
            }
            StateSpecimen {
                stateName: "Pressed preview"
                Segmented { anchors.centerIn: parent; model: ["Off", "Low", "Medium"]; currentIndex: 1; previewState: "pressed" }
            }
            StateSpecimen {
                stateName: "Keyboard focus preview"
                Segmented { anchors.centerIn: parent; model: ["Off", "Low", "Medium"]; currentIndex: 1; previewState: "focus" }
            }
            StateSpecimen {
                stateName: "Disabled"
                Segmented { anchors.centerIn: parent; model: ["Off", "Low", "Medium"]; enabled: false }
            }
        }

        ComponentSection {
            sectionName: "Tabs"

            StateSpecimen {
                stateName: "First selected"
                Tabs { anchors.centerIn: parent; model: ["Overview", "Console", "Files"] }
            }
            StateSpecimen {
                stateName: "Console selected"
                Tabs { anchors.centerIn: parent; model: ["Overview", "Console", "Files"]; currentIndex: 1 }
            }
            StateSpecimen {
                stateName: "Hover preview"
                Tabs { anchors.centerIn: parent; model: ["ubuntu-24.04", "fedora-43"]; currentIndex: 1; previewState: "hover" }
            }
            StateSpecimen {
                stateName: "Pressed preview"
                Tabs { anchors.centerIn: parent; model: ["ubuntu-24.04", "fedora-43"]; currentIndex: 1; previewState: "pressed" }
            }
            StateSpecimen {
                stateName: "Keyboard focus preview"
                Tabs { anchors.centerIn: parent; model: ["Overview", "Console", "Files"]; previewState: "focus" }
            }
            StateSpecimen {
                stateName: "Disabled"
                Tabs { anchors.centerIn: parent; model: ["Overview", "Console", "Files"]; enabled: false }
            }
        }

        ComponentSection {
            sectionName: "Disclosure"

            StateSpecimen {
                stateName: "Closed"
                Disclosure { anchors.centerIn: parent; width: LithicTokens.size.menuWidthMin; text: "Server details" }
            }
            StateSpecimen {
                stateName: "Open"
                specimenHeight: LithicTokens.space.s6 + LithicTokens.space.s5
                Disclosure {
                    anchors.centerIn: parent
                    width: LithicTokens.size.menuWidthMin
                    text: "Server details"
                    open: true
                    Label { width: parent.width; text: "ally at ops@192.0.2.18"; typeRole: "monoSm"; inkRole: "fg2" }
                }
            }
            StateSpecimen {
                stateName: "Hover preview"
                Disclosure { anchors.centerIn: parent; width: LithicTokens.size.menuWidthMin; text: "Console router"; previewState: "hover" }
            }
            StateSpecimen {
                stateName: "Keyboard focus preview"
                Disclosure { anchors.centerIn: parent; width: LithicTokens.size.menuWidthMin; text: "Console router"; previewState: "focus" }
            }
            StateSpecimen {
                stateName: "Disabled"
                Disclosure { anchors.centerIn: parent; width: LithicTokens.size.menuWidthMin; text: "Console router"; enabled: false }
            }
        }

        ComponentSection {
            sectionName: "Fields on planes"

            StateSpecimen {
                stateName: "On the work pane"
                specimenWidth: LithicTokens.size.draftFormMax
                specimenHeight: paneFields.implicitHeight
                Pane {
                    id: paneFields
                    anchors.fill: parent
                    kind: "work"
                    inset: LithicTokens.space.s4
                    Column {
                        width: parent.width
                        spacing: LithicTokens.space.s3
                        TextField { width: parent.width; label: "Server address"; text: "192.0.2.18"; helpText: "Use a hostname or address" }
                        Picker { width: parent.width; label: "Machine"; options: ["ubuntu-24.04", "fedora-43", "windows-11"]; currentIndex: 0 }
                        Search { width: parent.width; placeholderText: "Search machines"; shortcutHint: "Ctrl K" }
                        Stepper { from: 1; to: 8; value: 4; unit: "cores"; accessibleName: "CPU cores" }
                    }
                }
            }
            StateSpecimen {
                stateName: "Inside a slab"
                specimenWidth: LithicTokens.size.draftFormMax
                specimenHeight: slabFields.implicitHeight
                Slab {
                    id: slabFields
                    anchors.fill: parent
                    Column {
                        width: parent.width
                        spacing: LithicTokens.space.s3
                        TextField { width: parent.width; label: "Session user"; text: "ops"; previewState: "focus" }
                        Picker { width: parent.width; label: "Console"; options: ["/dev/ttyUSB0", "/dev/ttyS0"]; currentIndex: 0 }
                        TextField { width: parent.width; label: "Port"; text: "22a"; errorText: "Enter a number" }
                        Stepper { from: 1; to: 8; value: 2; unit: "cores"; accessibleName: "CPU cores" }
                    }
                }
            }
        }

        ComponentSection {
            sectionName: "Slab"

            StateSpecimen {
                stateName: "Rows with details"
                specimenWidth: LithicTokens.size.draftFormMax
                specimenHeight: rowsSlab.implicitHeight
                Slab {
                    id: rowsSlab
                    anchors.fill: parent
                    Column {
                        width: parent.width
                        SettingRow {
                            width: parent.width
                            first: true
                            title: "Autostart"
                            detail: "Open ubuntu-24.04 when Ryoport starts."
                            Toggle { checked: true }
                        }
                        SettingRow {
                            width: parent.width
                            title: "Mode"
                            detail: "Follow the palette or keep one mode."
                            Segmented { model: ["Follow", "Dark", "Light"]; accessibleName: "Mode" }
                        }
                        SettingRow {
                            width: parent.width
                            title: "CPU cores"
                            detail: "Cores given to fedora-43 on prowl."
                            Stepper { from: 1; to: 8; value: 4; unit: "cores"; accessibleName: "CPU cores" }
                        }
                    }
                }
            }
        }

        ComponentSection {
            sectionName: "SettingRow"

            StateSpecimen {
                stateName: "Rest"
                specimenWidth: LithicTokens.size.draftFormMax
                specimenHeight: restRow.implicitHeight
                Slab {
                    id: restRow
                    anchors.fill: parent
                    SettingRow {
                        width: parent.width
                        first: true
                        title: "Show seconds"
                        detail: "The bar clock counts every second."
                        Toggle { text: ""; Accessible.name: "Show seconds" }
                    }
                }
            }
            StateSpecimen {
                stateName: "Changed"
                specimenWidth: LithicTokens.size.draftFormMax
                specimenHeight: changedRow.implicitHeight
                Slab {
                    id: changedRow
                    anchors.fill: parent
                    SettingRow {
                        width: parent.width
                        first: true
                        changed: true
                        title: "Clock format"
                        detail: "How the bar writes the time."
                        Picker { options: ["24 hour", "12 hour"]; currentIndex: 1 }
                    }
                }
            }
            StateSpecimen {
                stateName: "Changed, hover shows reset"
                specimenWidth: LithicTokens.size.draftFormMax
                specimenHeight: resetRow.implicitHeight
                Slab {
                    id: resetRow
                    anchors.fill: parent
                    SettingRow {
                        width: parent.width
                        first: true
                        changed: true
                        previewState: "hover"
                        title: "Clock format"
                        detail: "How the bar writes the time."
                        Picker { options: ["24 hour", "12 hour"]; currentIndex: 1 }
                    }
                }
            }
            StateSpecimen {
                stateName: "Activatable hover preview"
                specimenWidth: LithicTokens.size.draftFormMax
                specimenHeight: hoverRow.implicitHeight
                Slab {
                    id: hoverRow
                    anchors.fill: parent
                    SettingRow {
                        width: parent.width
                        first: true
                        activatable: true
                        previewState: "hover"
                        title: "Dim inactive windows"
                        detail: "Windows without focus sit a step darker."
                        Toggle { checked: true; Accessible.name: "Dim inactive windows" }
                    }
                }
            }
            StateSpecimen {
                stateName: "Activatable pressed preview"
                specimenWidth: LithicTokens.size.draftFormMax
                specimenHeight: pressedRow.implicitHeight
                Slab {
                    id: pressedRow
                    anchors.fill: parent
                    SettingRow {
                        width: parent.width
                        first: true
                        activatable: true
                        previewState: "pressed"
                        title: "Wallpaper"
                        detail: "Choose the picture behind the desktop."
                        Glyph { name: "chevron_right"; ink: Lithic.fg2; size: LithicTokens.size.iconPlaqueIcon }
                    }
                }
            }
            StateSpecimen {
                stateName: "Keyboard focus preview"
                specimenWidth: LithicTokens.size.draftFormMax
                specimenHeight: focusRow.implicitHeight
                Slab {
                    id: focusRow
                    anchors.fill: parent
                    SettingRow {
                        width: parent.width
                        first: true
                        activatable: true
                        previewState: "focus"
                        title: "Dim inactive windows"
                        detail: "Windows without focus sit a step darker."
                        Toggle { Accessible.name: "Dim inactive windows" }
                    }
                }
            }
            StateSpecimen {
                stateName: "Unavailable"
                specimenWidth: LithicTokens.size.draftFormMax
                specimenHeight: unavailableRow.implicitHeight
                Slab {
                    id: unavailableRow
                    anchors.fill: parent
                    SettingRow {
                        width: parent.width
                        first: true
                        title: "Second display"
                        detail: "Where the second bar sits."
                        unavailable: "Connect a second display to choose this."
                        Picker { options: ["Left", "Right"]; currentIndex: 0 }
                    }
                }
            }
            StateSpecimen {
                stateName: "Highlighted by search, start of the fade"
                specimenWidth: LithicTokens.size.draftFormMax
                specimenHeight: highlightRow.implicitHeight
                Slab {
                    id: highlightRow
                    anchors.fill: parent
                    SettingRow {
                        width: parent.width
                        first: true
                        previewState: "highlight"
                        title: "Night light"
                        detail: "Warm the screen after sunset."
                        Toggle { checked: true; Accessible.name: "Night light" }
                    }
                }
            }
            StateSpecimen {
                stateName: "Long title and detail wrap"
                specimenWidth: LithicTokens.size.draftFormMax
                specimenHeight: wrapRow.implicitHeight
                Slab {
                    id: wrapRow
                    anchors.fill: parent
                    SettingRow {
                        width: parent.width
                        first: true
                        changed: true
                        title: "Keep the bar visible over full screen windows on every display"
                        detail: "Games and videos still cover it when they ask for exclusive full screen, so this only applies to ordinary windows."
                        Toggle { checked: true; Accessible.name: "Keep the bar visible" }
                    }
                }
            }
            StateSpecimen {
                stateName: "Rows stacked with veins"
                specimenWidth: LithicTokens.size.draftFormMax
                specimenHeight: stackRows.implicitHeight
                Slab {
                    id: stackRows
                    anchors.fill: parent
                    Column {
                        width: parent.width
                        SettingRow {
                            width: parent.width
                            first: true
                            title: "Show seconds"
                            detail: "The bar clock counts every second."
                            Toggle { Accessible.name: "Show seconds" }
                        }
                        SettingRow {
                            width: parent.width
                            title: "Clock format"
                            detail: "How the bar writes the time."
                            Segmented { model: ["24 hour", "12 hour"]; accessibleName: "Clock format" }
                        }
                        SettingRow {
                            width: parent.width
                            title: "Workspaces"
                            Stepper { from: 1; to: 10; value: 5; accessibleName: "Workspaces" }
                        }
                    }
                }
            }
        }

        ComponentSection {
            sectionName: "ChordField"

            StateSpecimen {
                stateName: "Not set"
                ChordField { anchors.centerIn: parent; accessibleName: "Open launcher" }
            }
            StateSpecimen {
                stateName: "Chord"
                ChordField { anchors.centerIn: parent; chord: "Super+Shift+Return"; accessibleName: "Open terminal" }
            }
            StateSpecimen {
                stateName: "Store spelling, shown canonical"
                ChordField { anchors.centerIn: parent; chord: "SHIFT + SUPER + Q"; accessibleName: "Close window" }
            }
            StateSpecimen {
                stateName: "Number pad and Page Up"
                ChordField { anchors.centerIn: parent; chord: "Ctrl+Alt+KP_3+Prior"; accessibleName: "Move window" }
            }
            StateSpecimen {
                stateName: "Media key alone"
                ChordField { anchors.centerIn: parent; chord: "XF86AudioRaiseVolume"; accessibleName: "Raise volume" }
            }
            StateSpecimen {
                stateName: "Recording"
                ChordField { anchors.centerIn: parent; previewState: "recording"; chord: "Super+D"; accessibleName: "Show desktop" }
            }
            StateSpecimen {
                stateName: "Conflict"
                specimenHeight: LithicTokens.space.s6 + LithicTokens.space.s5
                ChordField { anchors.centerIn: parent; chord: "Super+Q"; conflict: "Already used by Close window"; accessibleName: "Lock screen" }
            }
            StateSpecimen {
                stateName: "Hover preview"
                ChordField { anchors.centerIn: parent; chord: "Super+Space"; previewState: "hover"; accessibleName: "Switch layout" }
            }
            StateSpecimen {
                stateName: "Pressed preview"
                ChordField { anchors.centerIn: parent; chord: "Super+Space"; previewState: "pressed"; accessibleName: "Switch layout" }
            }
            StateSpecimen {
                stateName: "Keyboard focus preview"
                ChordField { anchors.centerIn: parent; chord: "Super+grave"; previewState: "focus"; accessibleName: "Overview" }
            }
            StateSpecimen {
                stateName: "Disabled"
                ChordField { anchors.centerIn: parent; chord: "Super+L"; enabled: false; accessibleName: "Lock screen" }
            }
        }

        ComponentSection {
            sectionName: "ColorField"

            StateSpecimen {
                stateName: "Colour"
                specimenHeight: LithicTokens.space.s6 + LithicTokens.space.s5
                ColorField { anchors.centerIn: parent; width: LithicTokens.size.searchWidth; label: "Accent colour"; color: "#ce5e32" }
            }
            StateSpecimen {
                stateName: "With alpha"
                specimenHeight: LithicTokens.space.s6 + LithicTokens.space.s5
                ColorField { anchors.centerIn: parent; width: LithicTokens.size.searchWidth; label: "Bar tint"; allowAlpha: true; color: Qt.rgba(0.29, 0.44, 0.65, 0.6) }
            }
            StateSpecimen {
                stateName: "Invalid hex"
                specimenHeight: LithicTokens.space.s6 + LithicTokens.space.s5
                ColorField { anchors.centerIn: parent; width: LithicTokens.size.searchWidth; label: "Border colour"; color: "#7ab8a0"; previewState: "error" }
            }
            StateSpecimen {
                stateName: "Hover preview"
                specimenHeight: LithicTokens.space.s6 + LithicTokens.space.s5
                ColorField { anchors.centerIn: parent; width: LithicTokens.size.searchWidth; label: "Accent colour"; color: "#ce5e32"; previewState: "hover" }
            }
            StateSpecimen {
                stateName: "Keyboard focus preview"
                specimenHeight: LithicTokens.space.s6 + LithicTokens.space.s5
                ColorField { anchors.centerIn: parent; width: LithicTokens.size.searchWidth; label: "Accent colour"; color: "#ce5e32"; previewState: "focus" }
            }
            StateSpecimen {
                stateName: "Disabled"
                specimenHeight: LithicTokens.space.s6 + LithicTokens.space.s5
                ColorField { anchors.centerIn: parent; width: LithicTokens.size.searchWidth; label: "Accent colour"; color: "#ce5e32"; enabled: false }
            }
        }

        ComponentSection {
            sectionName: "Selection"

            StateSpecimen {
                stateName: "Selected tablet"
                specimenWidth: LithicTokens.size.listMedium
                specimenHeight: LithicTokens.size.tabletHeight
                Tablet { anchors.fill: parent; title: "fedora-43"; facts: "prowl at 192.0.2.43"; tone: "alive"; claim: "Alive"; proof: "24 ms"; selected: true }
            }
            StateSpecimen {
                stateName: "Rest tablet"
                specimenWidth: LithicTokens.size.listMedium
                specimenHeight: LithicTokens.size.tabletHeight
                Tablet { anchors.fill: parent; title: "ubuntu-24.04"; facts: "ally at 192.0.2.18"; tone: "alive"; claim: "Alive"; proof: "18 ms" }
            }
            StateSpecimen {
                stateName: "Section rows"
                specimenWidth: LithicTokens.size.sectionIndexWidth
                specimenHeight: sectionRows.implicitHeight
                SectionIndex { id: sectionRows; anchors.fill: parent; sections: ["Connection", "Identity", "Storage"]; currentIndex: 1 }
            }
        }

        ComponentSection {
            sectionName: "Menu"

            StateSpecimen {
                stateName: "Checked item"
                specimenWidth: LithicTokens.size.menuWidthMin + LithicTokens.space.s6
                specimenHeight: checkedMenu.implicitHeight
                Menu {
                    id: checkedMenu
                    anchors.fill: parent
                    menuWidth: LithicTokens.size.menuWidthMin + LithicTokens.space.s6
                    items: [
                        { label: "Sort by name", glyph: "sort_by_alpha", checked: true },
                        { label: "Sort by state", glyph: "filter_list" },
                        { label: "Sort by last seen", glyph: "schedule", shortcut: "Ctrl+L" }
                    ]
                }
            }
        }
    }
}
