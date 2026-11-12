import QtQuick
import Lithic

Item {
    id: root

    implicitHeight: content.implicitHeight + LithicTokens.space.s5 * 2
    // a picture of a look for the DesignTile previews; any image does
    readonly property url lookPreview: Qt.resolvedUrl("../../../media/sheet-settings.png")
    readonly property var startupApps: [
        { title: "Firefox", detail: "Opens at login on the first display", command: "firefox", on: true },
        { title: "Vesktop", detail: "Opens at login, minimised", command: "vesktop --start-minimized", on: true },
        { title: "Ryotunes", detail: "Opens at login", command: "ryotunes", on: false }
    ]

    // an editor a host would write: the item in, a value and validity out
    component StartupEditor: Column {
        property var item: ({})
        readonly property bool valid: commandField.text.trim().length > 0
        function value() { return Object.assign({}, item, { title: nameField.text, command: commandField.text }) }
        spacing: LithicTokens.space.s3
        TextField { id: nameField; width: parent.width; label: "Name"; text: parent.item.title || "" }
        TextField { id: commandField; width: parent.width; label: "Command"; mono: true; text: parent.item.command || "" }
    }

    component StartupToggle: Toggle {
        property var modelData: ({})
        property int index: -1
        checked: !!modelData.on
        Accessible.name: "Open " + (modelData.title || "") + " at login"
    }

    component HostMark: CutRect {
        fill: Lithic.stone3
        cut: LithicTokens.cut.small

        Glyph {
            anchors.centerIn: parent
            name: "dns"
            size: LithicTokens.size.iconPlaqueIcon
            ink: Lithic.fg2
        }
    }

    component Showcase: Column {
        required property string title
        default property alias specimens: specimenFlow.data
        width: parent.width
        spacing: LithicTokens.space.s4

        Label {
            text: parent.title
            typeRole: "section"
        }

        Flow {
            id: specimenFlow
            width: parent.width
            height: childrenRect.height
            spacing: LithicTokens.space.s4
        }
    }

    component State: Column {
        property string stateName: ""
        property real specimenWidth: LithicTokens.size.listMedium
        property real specimenHeight: LithicTokens.size.tabletHeight
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
                text: "Lists, containers and progress"
                typeRole: "name"
            }
            Label {
                width: parent.width
                text: "Resource rows, grouped facts, draft structure and job state for the Ryoport fleet."
                typeRole: "body"
                inkRole: "fg2"
            }
        }

        Showcase {
            title: "Tablet"

            State {
                stateName: "Idle"
                Tablet {
                    anchors.fill: parent
                    title: "ubuntu-24.04"
                    facts: "ally at ops@192.0.2.18"
                    claim: "Stopped"
                    proof: "8 vCPU"
                    mark: Component { HostMark {} }
                }
            }
            State {
                stateName: "Hovered action"
                Tablet {
                    anchors.fill: parent
                    title: "fedora-43"
                    facts: "prowl at 192.0.2.43"
                    claim: "Alive"
                    proof: "24 ms"
                    tone: "alive"
                    actionText: "Connect"
                    actionKind: "gold"
                    actionGlyph: "lan"
                    previewState: "hover"
                    mark: Component { HostMark {} }
                }
            }
            State {
                stateName: "Pressed"
                Tablet {
                    anchors.fill: parent
                    title: "windows-11"
                    facts: "mika-server at 203.0.113.70:2222"
                    claim: "Paused"
                    proof: "RDP"
                    tone: "paused"
                    previewState: "pressed"
                    mark: Component { HostMark {} }
                }
            }
            State {
                stateName: "Keyboard focus"
                Tablet {
                    anchors.fill: parent
                    title: "alpine-instant"
                    facts: "localhost at ops@127.0.0.1"
                    claim: "Instant"
                    proof: "SSH"
                    previewState: "focus"
                    mark: Component { HostMark {} }
                }
            }
            State {
                stateName: "Selected"
                Tablet {
                    anchors.fill: parent
                    title: "scratch-arch"
                    facts: "console on /dev/ttyUSB0"
                    claim: "Attention"
                    proof: "Serial"
                    tone: "attention"
                    selected: true
                    mark: Component { HostMark {} }
                }
            }
            State {
                stateName: "Becoming"
                Tablet {
                    anchors.fill: parent
                    title: "kolibrios-latest-en_US"
                    facts: "Transfer to 198.51.100.27"
                    claim: "Becoming"
                    proof: "68%"
                    tone: "becoming"
                    progress: 0.68
                    mark: Component { HostMark {} }
                }
            }
            State {
                stateName: "Compact disabled"
                specimenHeight: LithicTokens.size.tabletHeightCompact
                Tablet {
                    anchors.fill: parent
                    title: "fedora-43"
                    facts: "Connection unavailable"
                    claim: "Stopped"
                    compact: true
                    enabled: false
                    mark: Component { HostMark {} }
                }
            }
        }

        Showcase {
            title: "RowList"

            Item {
                width: LithicTokens.size.listMedium
                height: LithicTokens.size.tabletHeight * 3
                    + LithicTokens.size.rowListGap * 2

                RowList {
                    anchors.fill: parent
                    currentIndex: 1
                    model: [
                        { title: "ubuntu-24.04", facts: "ally at 192.0.2.18", tone: "alive" },
                        { title: "fedora-43", facts: "prowl at 192.0.2.43", tone: "becoming" },
                        { title: "windows-11", facts: "mika-server at 203.0.113.70", tone: "paused" }
                    ]
                    delegate: Tablet {
                        required property var modelData
                        required property int index
                        width: ListView.view.width
                        title: modelData.title
                        facts: modelData.facts
                        tone: modelData.tone
                        claim: modelData.tone === "alive" ? "Alive"
                            : modelData.tone === "becoming" ? "Becoming" : "Paused"
                        proof: index === 1 ? "42%" : "24 ms"
                        progress: index === 1 ? 0.42 : 0
                        selected: ListView.isCurrentItem
                        pane: Lithic.stone1
                        mark: Component { HostMark {} }
                        onClicked: ListView.view.currentIndex = index
                    }
                }

                Label {
                    anchors.top: parent.bottom
                    anchors.topMargin: LithicTokens.space.s2
                    width: parent.width
                    text: "Selected row with keyboard and scroll behavior"
                    typeRole: "caption"
                    inkRole: "fg2"
                    horizontalAlignment: Text.AlignHCenter
                }
            }
        }

        Showcase {
            title: "GroupHeader"

            State {
                stateName: "Needs you"
                specimenHeight: LithicTokens.size.factsRowHeight
                GroupHeader { anchors.fill: parent; title: "Needs you"; count: 2 }
            }
            State {
                stateName: "Pinned"
                specimenHeight: LithicTokens.size.factsRowHeight
                GroupHeader { anchors.fill: parent; title: "Pinned"; count: 3 }
            }
            State {
                stateName: "Running"
                specimenHeight: LithicTokens.size.factsRowHeight
                GroupHeader { anchors.fill: parent; title: "Running"; count: 4 }
            }
            State {
                stateName: "Becoming"
                specimenHeight: LithicTokens.size.factsRowHeight
                GroupHeader { anchors.fill: parent; title: "Becoming"; count: 1 }
            }
            State {
                stateName: "Stopped"
                specimenHeight: LithicTokens.size.factsRowHeight
                GroupHeader { anchors.fill: parent; title: "Stopped"; count: 6 }
            }
        }

        Showcase {
            title: "Skeleton"

            State {
                stateName: "Resting"
                Skeleton { anchors.fill: parent }
            }
            State {
                stateName: "Active"
                Skeleton { anchors.fill: parent; active: true }
            }
            State {
                stateName: "Compact"
                specimenHeight: LithicTokens.size.tabletHeightCompact
                Skeleton { anchors.fill: parent; compact: true }
            }
        }

        Showcase {
            title: "Empty"

            Item {
                width: LithicTokens.size.draftFormMax
                height: emptyState.implicitHeight

                Empty {
                    id: emptyState
                    width: parent.width
                    glyph: "link_off"
                    title: "No saved sessions"
                    body: "Connect to a fleet host, then save the session for quick access here."
                    actions: [
                        { text: "Connect", kind: "gold", glyph: "lan" },
                        { text: "Import", kind: "line", glyph: "download" }
                    ]
                }
            }
        }

        Showcase {
            title: "Pane"

            State {
                stateName: "Rail pane"
                specimenHeight: LithicTokens.size.paneDecorStrip * 3
                Pane {
                    anchors.fill: parent
                    kind: "rail"
                    inset: LithicTokens.space.s3
                    Label { text: "Fleet navigation"; typeRole: "title" }
                }
            }
            State {
                stateName: "List pane"
                specimenHeight: LithicTokens.size.paneDecorStrip * 3
                Pane {
                    anchors.fill: parent
                    kind: "list"
                    inset: LithicTokens.space.s3
                    Label { text: "Saved sessions"; typeRole: "title" }
                }
            }
            State {
                stateName: "Work pane"
                specimenHeight: LithicTokens.size.paneDecorStrip * 3
                Pane {
                    anchors.fill: parent
                    kind: "work"
                    inset: LithicTokens.space.s3
                    Label { text: "Host details"; typeRole: "title" }
                }
            }
        }

        Showcase {
            title: "MineralField"

            Repeater {
                model: ["idle", "alive", "becoming", "attention", "paused", "failed"]
                delegate: State {
                    required property string modelData
                    stateName: modelData.charAt(0).toUpperCase() + modelData.slice(1)
                    specimenWidth: LithicTokens.size.listMedium
                    specimenHeight: LithicTokens.size.paneDecorStrip * 2

                    MineralField {
                        anchors.fill: parent
                        tone: modelData
                        Label {
                            anchors.centerIn: parent
                            text: modelData.charAt(0).toUpperCase() + modelData.slice(1)
                            typeRole: "label"
                        }
                    }
                }
            }
        }

        Showcase {
            title: "Header"

            MineralField {
                width: LithicTokens.size.settingsMediumMax
                height: fleetHeader.implicitHeight
                tone: "alive"

                Header {
                    id: fleetHeader
                    width: parent.width
                    name: "ubuntu-24.04"
                    proof: "Connected to ally as ops at 192.0.2.18"
                    note: "Last checked a moment ago"
                    traits: ["Sealed", "Disposable", "Instant"]
                    mark: Component { HostMark {} }
                    actions: [
                        { text: "Open", kind: "gold", glyph: "terminal" },
                        { text: "Transfer", kind: "line", glyph: "swap_horiz" },
                        { text: "More", kind: "quiet", glyph: "more_horiz" }
                    ]
                    below: Component {
                        Sentence {
                            spans: [
                                { text: "Session route " },
                                { text: "203.0.113.24:7443", mono: true }
                            ]
                        }
                    }
                }
            }
        }

        Showcase {
            title: "Slab"

            State {
                stateName: "Quiet group"
                specimenWidth: LithicTokens.size.draftFormMax
                specimenHeight: LithicTokens.size.paneDecorStrip * 4
                Slab {
                    anchors.fill: parent
                    Label {
                        anchors.centerIn: parent
                        text: "Connection preferences"
                        typeRole: "title"
                    }
                }
            }

            State {
                stateName: "Toned, the hero plane"
                specimenWidth: LithicTokens.size.draftFormMax
                specimenHeight: LithicTokens.size.paneDecorStrip * 4
                Slab {
                    anchors.fill: parent
                    tone: "attention"
                    Claim {
                        anchors.centerIn: parent
                        tone: "attention"
                        text: "scratch-arch has a new host key"
                    }
                }
            }
        }

        Showcase {
            title: "Section"

            Section {
                width: LithicTokens.size.draftFormMax
                title: "Transfer destination"

                Slab {
                    width: parent.width
                    implicitHeight: LithicTokens.size.paneDecorStrip * 2
                    Label {
                        anchors.centerIn: parent
                        text: "prowl at 192.0.2.43:/srv/releases"
                        typeRole: "mono"
                    }
                }
            }
        }

        Showcase {
            title: "Facts"

            Facts {
                width: LithicTokens.size.draftFormMax
                facts: [
                    { label: "Host", value: "ally" },
                    { label: "Address", value: "192.0.2.18", literal: true },
                    { label: "Session", value: "ops" },
                    { label: "Console", value: "/dev/ttyUSB0", literal: true },
                    { label: "Last transfer", value: "" }
                ]
            }
        }

        Showcase {
            title: "Aside"

            Aside {
                width: LithicTokens.size.draftAsideMax
                title: "New saved session"
                facts: [
                    { label: "Host", value: "mika-server" },
                    { label: "User", value: "root" },
                    { label: "Port", value: "2222", literal: true }
                ]
                preview: "ssh -p 2222 root@203.0.113.70"
            }
        }

        Showcase {
            title: "DraftColumns"

            Column {
                spacing: LithicTokens.space.s2

                DraftColumns {
                    width: root.width - LithicTokens.space.s5 * 2
                    form: Component {
                        Slab {
                            implicitHeight: LithicTokens.size.paneDecorStrip * 4
                            Label {
                                anchors.centerIn: parent
                                text: "Wide connection form"
                                typeRole: "title"
                            }
                        }
                    }
                    aside: Component {
                        Aside {
                            title: "Saved session"
                            facts: [{ label: "Address", value: "198.51.100.28", literal: true }]
                            preview: "ssh ops@198.51.100.28"
                        }
                    }
                }

                Label {
                    text: "Wide columns"
                    typeRole: "caption"
                    inkRole: "fg2"
                }
            }

            Column {
                width: LithicTokens.size.settingsMediumMax
                spacing: LithicTokens.space.s2

                DraftColumns {
                    width: parent.width
                    form: Component {
                        Slab {
                            implicitHeight: LithicTokens.size.paneDecorStrip * 3
                            Label {
                                anchors.centerIn: parent
                                text: "Narrow transfer form"
                                typeRole: "title"
                            }
                        }
                    }
                    aside: Component {
                        Aside {
                            title: "Transfer"
                            preview: "scp image.raw prowl:/srv/images/"
                        }
                    }
                }

                Label {
                    text: "Stacked columns"
                    typeRole: "caption"
                    inkRole: "fg2"
                }
            }
        }

        Showcase {
            title: "Claim"

            Repeater {
                model: ["idle", "alive", "becoming", "attention", "paused", "failed"]
                delegate: State {
                    required property string modelData
                    stateName: modelData.charAt(0).toUpperCase() + modelData.slice(1)
                    specimenWidth: LithicTokens.size.factsLabelWidth
                    specimenHeight: LithicTokens.size.plaqueHeight
                    Claim {
                        anchors.centerIn: parent
                        tone: modelData
                        progress: modelData === "becoming" ? 0.62 : modelData === "idle" ? 0 : 1
                    }
                }
            }
        }

        Showcase {
            title: "Marks"

            State {
                stateName: "Ordered traits"
                specimenWidth: LithicTokens.size.draftFormMax
                specimenHeight: LithicTokens.size.plaqueHeight
                Marks {
                    anchors.centerIn: parent
                    traits: ["Sealed", "Disposable", "Instant"]
                }
            }
            State {
                stateName: "Trailing marks dropped"
                specimenWidth: LithicTokens.size.listMedium
                specimenHeight: LithicTokens.size.plaqueHeight
                Marks {
                    anchors.centerIn: parent
                    maximumWidth: LithicTokens.size.factsLabelWidth
                    traits: ["Disposable", "Instant", "Sealed", "Ephemeral"]
                }
            }
        }

        Showcase {
            title: "Sentence"

            State {
                stateName: "Literal data"
                specimenWidth: LithicTokens.size.draftFormMax
                specimenHeight: LithicTokens.size.plaqueHeight
                Sentence {
                    anchors.centerIn: parent
                    spans: [
                        { text: "Connected to " },
                        { text: "192.0.2.43", mono: true },
                        { text: " as ops" }
                    ]
                }
            }
        }

        Showcase {
            title: "BladeMeter"

            State {
                stateName: "Quiet 42 percent"
                specimenWidth: LithicTokens.size.listMedium
                specimenHeight: LithicTokens.size.plaqueHeight
                BladeMeter {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    value: 0.42
                }
            }
            State {
                stateName: "Attention 94 percent"
                specimenWidth: LithicTokens.size.listMedium
                specimenHeight: LithicTokens.size.plaqueHeight
                BladeMeter {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    value: 0.94
                }
            }
        }

        Showcase {
            title: "BladeProgress"

            State {
                stateName: "Determinate"
                specimenWidth: LithicTokens.size.listMedium
                specimenHeight: LithicTokens.size.plaqueHeight
                BladeProgress {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    value: 0.68
                }
            }
            State {
                stateName: "Active with edge marker"
                specimenWidth: LithicTokens.size.listMedium
                specimenHeight: LithicTokens.size.plaqueHeight
                BladeProgress {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    value: 0.42
                    active: true
                }
            }
            State {
                stateName: "Indeterminate"
                specimenWidth: LithicTokens.size.listMedium
                specimenHeight: LithicTokens.size.plaqueHeight
                BladeProgress {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    active: true
                    indeterminate: true
                }
            }
        }

        Showcase {
            title: "Working"

            State {
                stateName: "Resting"
                specimenWidth: LithicTokens.size.factsLabelWidth
                specimenHeight: LithicTokens.size.plaqueHeight
                Working { anchors.centerIn: parent }
            }
            State {
                stateName: "Active"
                specimenWidth: LithicTokens.size.factsLabelWidth
                specimenHeight: LithicTokens.size.plaqueHeight
                Working { anchors.centerIn: parent; active: true }
            }
        }

        Showcase {
            title: "BecomingName"

            BecomingName {
                width: LithicTokens.size.draftFormMax
                name: "kolibrios-latest-en_US"
                stage: "Writing image"
                progress: 0.68
                active: true
                lines: [
                    "transfer 198.51.100.27:7443",
                    "chunk 348 of 512",
                    "verified 2.7 GiB"
                ]
            }
        }

        Showcase {
            title: "ListEditor"

            State {
                stateName: "Items with a trailing toggle"
                specimenWidth: LithicTokens.size.draftFormMax
                specimenHeight: restList.implicitHeight
                Slab {
                    id: restList
                    anchors.fill: parent
                    ListEditor {
                        width: parent.width
                        model: root.startupApps
                        addText: "Add a startup app"
                        editor: Component { StartupEditor {} }
                        trailing: Component { StartupToggle {} }
                    }
                }
            }
            State {
                stateName: "Reorderable, an item being dragged"
                specimenWidth: LithicTokens.size.draftFormMax
                specimenHeight: dragList.implicitHeight
                Slab {
                    id: dragList
                    anchors.fill: parent
                    ListEditor {
                        width: parent.width
                        reorderable: true
                        previewState: "dragging"
                        model: [
                            { title: "Wallpaper", detail: "Bottom layer" },
                            { title: "Desktop widgets", detail: "Above the wallpaper" },
                            { title: "Bar", detail: "Top layer" }
                        ]
                        titleOf: item => item.title
                        addText: "Add a layer"
                        editor: Component { StartupEditor {} }
                    }
                }
            }
            State {
                stateName: "Editing an item"
                specimenWidth: LithicTokens.size.draftFormMax
                specimenHeight: editList.implicitHeight
                Slab {
                    id: editList
                    anchors.fill: parent
                    ListEditor {
                        width: parent.width
                        previewState: "editing"
                        model: root.startupApps.slice(0, 2)
                        addText: "Add a startup app"
                        editor: Component { StartupEditor {} }
                    }
                }
            }
            State {
                stateName: "Adding an item, Add waits for a command"
                specimenWidth: LithicTokens.size.draftFormMax
                specimenHeight: addList.implicitHeight
                Slab {
                    id: addList
                    anchors.fill: parent
                    ListEditor {
                        width: parent.width
                        previewState: "adding"
                        model: root.startupApps.slice(0, 1)
                        addText: "Add a startup app"
                        editor: Component { StartupEditor {} }
                    }
                }
            }
            State {
                stateName: "Empty"
                specimenWidth: LithicTokens.size.draftFormMax
                specimenHeight: emptyList.implicitHeight
                Slab {
                    id: emptyList
                    anchors.fill: parent
                    ListEditor {
                        width: parent.width
                        model: []
                        emptyText: "No apps open at login."
                        addText: "Add a startup app"
                        editor: Component { StartupEditor {} }
                    }
                }
            }
            State {
                stateName: "Rules read through titleOf and detailOf"
                specimenWidth: LithicTokens.size.draftFormMax
                specimenHeight: ruleList.implicitHeight
                Slab {
                    id: ruleList
                    anchors.fill: parent
                    ListEditor {
                        width: parent.width
                        model: [
                            { app: "Firefox", float: true, opacity: 80 },
                            { app: "Ryotunes", float: false, opacity: 100 }
                        ]
                        titleOf: item => item.app + ": " + (item.float ? "floats" : "tiles") + ", " + item.opacity + "% opacity"
                        detailOf: item => ""
                        addText: "Add a window rule"
                        editor: Component { StartupEditor {} }
                    }
                }
            }
        }

        Showcase {
            title: "DesignTile"

            State {
                stateName: "No preview, hatch placeholder"
                specimenWidth: LithicTokens.size.catalogueTileWidthMax
                specimenHeight: placeholderTile.implicitHeight
                DesignTile { id: placeholderTile; anchors.fill: parent; title: "Sumi"; status: "Ink rail on the left edge"; resource: "light" }
            }
            State {
                stateName: "Preview"
                specimenWidth: LithicTokens.size.catalogueTileWidthMax
                specimenHeight: previewTile.implicitHeight
                DesignTile { id: previewTile; anchors.fill: parent; title: "Shima"; status: "Islands for each part"; resource: "standard"; source: root.lookPreview }
            }
            State {
                stateName: "Current"
                specimenWidth: LithicTokens.size.catalogueTileWidthMax
                specimenHeight: currentTile.implicitHeight
                DesignTile { id: currentTile; anchors.fill: parent; title: "Orpheus"; status: "In use"; resource: "standard"; current: true; actionText: "Edit"; actionGlyph: "tune"; source: root.lookPreview }
            }
            State {
                stateName: "Selected"
                specimenWidth: LithicTokens.size.catalogueTileWidthMax
                specimenHeight: selectedTile.implicitHeight
                DesignTile { id: selectedTile; anchors.fill: parent; title: "Shima"; status: "Islands for each part"; resource: "standard"; selected: true; source: root.lookPreview }
            }
            State {
                stateName: "Current and selected"
                specimenWidth: LithicTokens.size.catalogueTileWidthMax
                specimenHeight: bothTile.implicitHeight
                DesignTile { id: bothTile; anchors.fill: parent; title: "Orpheus"; status: "In use"; resource: "standard"; current: true; selected: true; source: root.lookPreview }
            }
            State {
                stateName: "Hover preview with action"
                specimenWidth: LithicTokens.size.catalogueTileWidthMax
                specimenHeight: hoverTile.implicitHeight
                DesignTile { id: hoverTile; anchors.fill: parent; title: "Sumi"; status: "Ink rail on the left edge"; resource: "light"; actionText: "Preview"; actionGlyph: "visibility"; previewState: "hover" }
            }
            State {
                stateName: "Pressed preview"
                specimenWidth: LithicTokens.size.catalogueTileWidthMax
                specimenHeight: pressedTile.implicitHeight
                DesignTile { id: pressedTile; anchors.fill: parent; title: "Sumi"; status: "Ink rail on the left edge"; resource: "light"; previewState: "pressed" }
            }
            State {
                stateName: "Keyboard focus preview"
                specimenWidth: LithicTokens.size.catalogueTileWidthMax
                specimenHeight: focusTile.implicitHeight
                DesignTile { id: focusTile; anchors.fill: parent; title: "Shima"; status: "Islands for each part"; resource: "standard"; previewState: "focus"; source: root.lookPreview }
            }
            State {
                stateName: "Unavailable"
                specimenWidth: LithicTokens.size.catalogueTileWidthMax
                specimenHeight: unavailableTile.implicitHeight
                DesignTile { id: unavailableTile; anchors.fill: parent; title: "Iris"; status: "Glass bar"; unavailable: "Needs niri; this desktop runs Hyprland"; resource: "heavy"; source: root.lookPreview }
            }
            State {
                stateName: "Busy"
                specimenWidth: LithicTokens.size.catalogueTileWidthMax
                specimenHeight: busyTile.implicitHeight
                DesignTile { id: busyTile; anchors.fill: parent; title: "Iris"; status: "Installing"; busy: true; resource: "heavy" }
            }
            State {
                stateName: "Gold action, heavy mark"
                specimenWidth: LithicTokens.size.catalogueTileWidthMax
                specimenHeight: actionTile.implicitHeight
                DesignTile { id: actionTile; anchors.fill: parent; title: "Iris"; status: "Not installed"; resource: "heavy"; actionText: "Install"; actionKind: "gold"; actionGlyph: "download" }
            }
            State {
                stateName: "Disabled"
                specimenWidth: LithicTokens.size.catalogueTileWidthMax
                specimenHeight: disabledTile.implicitHeight
                DesignTile { id: disabledTile; anchors.fill: parent; title: "Sumi"; status: "Ink rail on the left edge"; resource: "light"; enabled: false }
            }
        }

        Showcase {
            title: "DesignGallery"

            State {
                stateName: "Columns fit 200 to 260 tiles 8 apart; keyboard focus on the selected tile"
                specimenWidth: parent.width
                specimenHeight: looks.implicitHeight
                DesignGallery {
                    id: looks
                    anchors.fill: parent
                    accessibleName: "Bar styles"
                    currentIndex: 2
                    previewState: "focus"
                    model: [
                        { title: "Orpheus", status: "In use", resource: "standard", current: true, source: root.lookPreview },
                        { title: "Sumi", status: "Ink rail on the left edge", resource: "light" },
                        { title: "Shima", status: "Islands for each part", resource: "standard", source: root.lookPreview },
                        { title: "Iris", status: "Installing", resource: "heavy", busy: true },
                        { title: "Kumo", status: "Floating clusters", resource: "light", actionText: "Install", actionGlyph: "download" },
                        { title: "Tsuki", status: "Glass bar", resource: "heavy", unavailable: "Needs niri; this desktop runs Hyprland" },
                        { title: "Line", status: "One hairline and the clock", resource: "light" }
                    ]
                }
            }
        }

        Showcase {
            title: "ScrollRail"

            Repeater {
                model: [
                    { name: "Overflowing, at the top", position: 0, state: "" },
                    { name: "Scrolled halfway", position: 0.5, state: "" },
                    { name: "Hover preview", position: 1, state: "hover" },
                    { name: "No overflow, hidden", position: 0, state: "", short: true }
                ]

                delegate: State {
                    id: railState
                    required property var modelData
                    stateName: modelData.name
                    specimenHeight: LithicTokens.size.listMedium / 2

                    CutRect {
                        anchors.fill: parent
                        fill: Lithic.stone2

                        Flickable {
                            id: pane
                            anchors.fill: parent
                            anchors.margins: LithicTokens.space.s2
                            clip: true
                            contentWidth: width
                            contentHeight: settingNames.implicitHeight
                            contentY: Math.max(0, contentHeight - height) * railState.modelData.position
                            boundsBehavior: Flickable.StopAtBounds

                            Column {
                                id: settingNames
                                width: pane.width
                                Repeater {
                                    model: railState.modelData.short
                                           ? ["Appearance", "Bar", "Desktop"]
                                           : ["Appearance", "Bar", "Desktop", "Displays", "Keyboard", "Mouse", "Notifications", "Sound", "Wallpaper", "Windows"]
                                    delegate: Label {
                                        required property string modelData
                                        width: settingNames.width
                                        height: LithicTokens.size.sectionIndexRow / 2
                                        verticalAlignment: Text.AlignVCenter
                                        text: modelData
                                        typeRole: "body"
                                    }
                                }
                            }
                        }

                        ScrollRail {
                            anchors.top: pane.top
                            anchors.bottom: pane.bottom
                            anchors.right: parent.right
                            anchors.rightMargin: LithicTokens.space.s1
                            flickable: pane
                            previewState: railState.modelData.state
                        }
                    }
                }
            }
        }
    }
}
