import QtQuick
import Lithic

Item {
    id: root

    implicitHeight: content.implicitHeight + LithicTokens.space.s5 * 2

    component StateSpecimen: Column {
        required property string stateName
        property real specimenHeight: LithicTokens.size.emptyFrame
        default property alias specimen: specimenHost.data

        width: LithicTokens.size.searchWidth
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
                    Label { width: parent.width; text: "ally · ops@192.0.2.18"; typeRole: "monoSm"; inkRole: "fg2" }
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
    }
}
