pragma ComponentBehavior: Bound
import QtQuick
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"

// Machines: the local yard. Two lanes -- LIBRARY (the yard list left, the machine
// stage right) and NEW (a CATALOG / INSTANT / ISO channel, its grid or explainer
// left, its create sheet right). Paper and ink; the departure board keeps the
// yard's headline; one red hanko on the stage is the only colour the chrome owns.
// The masthead and window chrome live in the rail; this page is the yard itself.
Item {
    id: mach

    property bool active: false           // App gates keys/focus to the shown page
    signal installEngineRequested()

    property string mode: "library"       // library | new
    property string channel: "catalog"    // catalog | instant | iso  (NEW lane)
    property string query: ""

    readonly property bool gated: Vm.capsLoaded && Vm.kvmOff
    readonly property bool engineMissing: Vm.capsLoaded && !Vm.kvmOff && !Vm.caps.quickemu
    readonly property int runningCount: {
        var n = 0;
        for (var i = 0; i < Vm.vms.length; i++)
            if (Vm.vms[i].running === true)
                n++;
        return n;
    }
    readonly property bool pending: Vm.downloading || Vm.busy

    Keys.onEscapePressed: (e) => {
        if (search.focused) { mach.forceActiveFocus(); e.accepted = true; }
        else if (mach.query.length > 0) { mach.query = ""; e.accepted = true; }
        else if (mach.mode === "new") { mach.mode = "library"; e.accepted = true; }
        else e.accepted = false;
    }
    Keys.onUpPressed: mach.moveSelection(-1)
    Keys.onDownPressed: mach.moveSelection(1)
    Keys.onReturnPressed: mach.launchSelected()
    Keys.onEnterPressed: mach.launchSelected()
    Shortcut {
        sequences: ["/", "Ctrl+K"]
        enabled: mach.active
        onActivated: search.grabFocus()
    }

    function moveSelection(dir) {
        if (mach.mode !== "library" || lib.shown.length === 0)
            return;
        var idx = -1;
        for (var i = 0; i < lib.shown.length; i++)
            if (lib.shown[i].name === Vm.selectedName) { idx = i; break; }
        idx = Math.max(0, Math.min(lib.shown.length - 1, idx + dir));
        Vm.select(lib.shown[idx].name);
    }
    function launchSelected() {
        if (mach.mode !== "library" || Vm.busy || !Vm.selected)
            return;
        if (Vm.selected.running)
            Vm.stop(Vm.selected.name);
        else if (Vm.caps.quickemu === true)
            Vm.launch(Vm.selected.name, ({ "gtk": "window", "spice": "spice", "none": "headless" })[Vm.selected.display] || "window");
    }

    onModeChanged: if (mode === "new") mach._loadChannel()
    onChannelChanged: if (mode === "new") mach._loadChannel()
    function _loadChannel() {
        if (channel === "catalog") Vm.loadCatalog(false);
        else if (channel === "instant") Vm.loadCloud();
    }

    // deep-link start mode, and an optional OS to preselect once it lands.
    Component.onCompleted: {
        var m = Quickshell.env("RYOVM_START_MODE");
        if (m === "catalog" || m === "instant") { mach.mode = "new"; mach.channel = m; }
    }
    readonly property string startOs: Quickshell.env("RYOVM_START_OS") || ""
    Connections {
        target: Vm
        function onCreated(name) { mach.mode = "library"; }
        function onOsListChanged() {
            if (mach.startOs.length === 0 || Vm.selectedOs !== null)
                return;
            for (var i = 0; i < Vm.osList.length; i++)
                if (Vm.osList[i].os === mach.startOs) { Vm.selectOs(Vm.osList[i]); break; }
        }
    }

    // ---- head --------------------------------------------------------------
    PageHead {
        id: header
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.leftMargin: Tokens.s6
        anchors.rightMargin: Tokens.s6
        anchors.topMargin: Tokens.s5
        eyebrow: I18n.tr("FLEET")
        title: I18n.tr("Machines")
    }

    // ---- toolbar -----------------------------------------------------------
    Item {
        id: toolbar
        anchors.top: header.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.leftMargin: Tokens.s6
        anchors.rightMargin: Tokens.s6
        anchors.topMargin: Tokens.s3
        height: 40
        visible: !mach.gated

        Seg {
            id: laneSeg
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            options: ["LIBRARY", "NEW"]
            current: mach.mode.toUpperCase()
            onChose: (k) => { mach.mode = k.toLowerCase(); mach.query = ""; }
        }

        Field {
            id: search
            anchors.left: laneSeg.right
            anchors.leftMargin: Tokens.s3
            anchors.verticalCenter: parent.verticalCenter
            width: 200
            toolbar: true
            text: mach.query
            placeholder: mach.mode === "library" ? I18n.tr("Filter your machines")
                : mach.channel === "instant" ? I18n.tr("Search instant images")
                : mach.channel === "iso" ? I18n.tr("Load ISO")
                : I18n.tr("Search 90+ operating systems")
            onEdited: (v) => mach.query = v
        }

        Row {
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s2
            Seg {
                anchors.verticalCenter: parent.verticalCenter
                visible: mach.mode === "new"
                options: ["CATALOG", "INSTANT", "ISO"]
                current: mach.channel.toUpperCase()
                onChose: (k) => { mach.channel = k.toLowerCase(); mach.query = ""; }
            }
            IconBtn {
                anchors.verticalCenter: parent.verticalCenter
                visible: mach.mode === "new" && mach.channel === "catalog"
                glyph: "\u21bb"
                armed: !Vm.catalogLoading
                onAct: Vm.loadCatalog(true)
            }
        }
    }

    // ---- gate: virtualization off (the only whole-page fault) --------------
    Item {
        anchors.top: toolbar.bottom
        anchors.topMargin: Tokens.s4
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: bottomBar.top
        anchors.leftMargin: Tokens.s6
        anchors.rightMargin: Tokens.s6
        visible: mach.gated

        Column {
            anchors.centerIn: parent
            width: Math.min(parent.width, 520)
            spacing: Tokens.s4
            Mark { anchors.horizontalCenter: parent.horizontalCenter; size: 96 }
            Text {
                anchors.horizontalCenter: parent.horizontalCenter
                horizontalAlignment: Text.AlignHCenter
                width: parent.width
                wrapMode: Text.WordWrap
                text: I18n.tr("Virtualization is off")
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: 18
                font.weight: Font.DemiBold
            }
            Text {
                anchors.horizontalCenter: parent.horizontalCenter
                horizontalAlignment: Text.AlignHCenter
                width: parent.width
                wrapMode: Text.WordWrap
                text: I18n.tr("A virtual machine needs hardware virtualization. Turn on SVM / AMD-V (AMD) or VT-x (Intel) in your BIOS/firmware, then reboot.")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: 13
            }
        }
    }

    // ---- engine banner: quickemu missing = a lit fault row, not a locked app --
    Rectangle {
        id: engineBanner
        anchors.top: toolbar.bottom
        anchors.topMargin: mach.engineMissing ? Tokens.s3 : 0
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.leftMargin: Tokens.s6
        anchors.rightMargin: Tokens.s6
        height: mach.engineMissing ? 44 : 0
        visible: mach.engineMissing && !mach.gated
        color: "transparent"
        radius: Tokens.radius
        border.width: Tokens.border
        border.color: Tokens.line
        antialiasing: false

        Row {
            anchors.left: parent.left
            anchors.leftMargin: Tokens.s3
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s3
            Annunciator { anchors.verticalCenter: parent.verticalCenter; label: I18n.tr("ENGINE"); lit: true; tileW: 60 }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("quickemu is not installed: machines can be imported and configured, not launched.")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: 12
            }
        }
        Btn {
            anchors.right: parent.right
            anchors.rightMargin: Tokens.s2
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("INSTALL ENGINE")
            primary: true
            onAct: mach.installEngineRequested()
        }
    }

    // ---- main: the split ---------------------------------------------------
    Item {
        id: main
        anchors.top: engineBanner.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: faultStrip.top
        anchors.leftMargin: Tokens.s6
        anchors.rightMargin: Tokens.s6
        anchors.topMargin: Tokens.s4
        anchors.bottomMargin: Tokens.s2
        visible: !mach.gated

        readonly property real gCol: (width - (Spans.cols - 1) * Tokens.s2) / Spans.cols
        readonly property real leftW: 5 * gCol + 4 * Tokens.s2
        readonly property int seamW: Tokens.s5
        readonly property bool emptyLibrary: mach.mode === "library" && Vm.vms.length === 0 && !Vm.vmsLoading && mach.query.length === 0

        Item {
            id: leftCol
            anchors.left: parent.left
            anchors.top: parent.top
            anchors.bottom: parent.bottom
            width: main.leftW
            visible: !main.emptyLibrary

            VmGrid {
                id: lib
                anchors.fill: parent
                filter: mach.query
                onBuildRequested: { mach.mode = "new"; mach.channel = "catalog"; }
                opacity: mach.mode === "library" ? 1 : 0
                visible: opacity > 0
                Behavior on opacity { NumberAnimation { duration: Tokens.swap } }
            }
            OsGrid {
                anchors.fill: parent
                filter: mach.query
                onInstallRequested: mach.installEngineRequested()
                opacity: mach.mode === "new" && mach.channel === "catalog" ? 1 : 0
                visible: opacity > 0
                Behavior on opacity { NumberAnimation { duration: Tokens.swap } }
            }
            CloudGrid {
                anchors.fill: parent
                filter: mach.query
                selected: cloudPanel.os
                onPicked: (e) => cloudPanel.os = e
                opacity: mach.mode === "new" && mach.channel === "instant" ? 1 : 0
                visible: opacity > 0
                Behavior on opacity { NumberAnimation { duration: Tokens.swap } }
            }
            Column {
                anchors.centerIn: parent
                width: parent.width - 40
                spacing: Tokens.s4
                opacity: mach.mode === "new" && mach.channel === "iso" ? 1 : 0
                visible: opacity > 0
                Behavior on opacity { NumberAnimation { duration: Tokens.swap } }
                Mark { anchors.horizontalCenter: parent.horizontalCenter; size: 96 }
                Text {
                    anchors.horizontalCenter: parent.horizontalCenter
                    horizontalAlignment: Text.AlignHCenter
                    width: parent.width
                    wrapMode: Text.WordWrap
                    text: I18n.tr("Build a machine from any ISO on disk: off-catalogue, the full QEMU reach. Fill in the sheet on the right.")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: 12
                }
            }
        }

        Rectangle {
            anchors.left: leftCol.right
            anchors.leftMargin: main.seamW / 2
            anchors.top: parent.top
            anchors.bottom: parent.bottom
            anchors.topMargin: (mach.engineMissing || mach.mode !== "library" ? 0 : -(main.y - toolbar.y)) + Tokens.s2
            anchors.bottomMargin: Tokens.s2
            width: 1
            color: Tokens.line
            visible: !main.emptyLibrary
        }

        Item {
            id: rightCol
            anchors.left: leftCol.right
            anchors.leftMargin: main.seamW
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.topMargin: mach.engineMissing || mach.mode !== "library" ? 0 : -(main.y - toolbar.y)
            anchors.bottom: parent.bottom
            visible: !main.emptyLibrary

            VmDetail {
                anchors.fill: parent
                opacity: mach.mode === "library" ? 1 : 0
                visible: opacity > 0
                Behavior on opacity { NumberAnimation { duration: Tokens.swap } }
            }
            CreatePanel {
                anchors.fill: parent
                opacity: mach.mode === "new" && mach.channel === "catalog" ? 1 : 0
                visible: opacity > 0
                Behavior on opacity { NumberAnimation { duration: Tokens.swap } }
            }
            CloudPanel {
                id: cloudPanel
                anchors.fill: parent
                opacity: mach.mode === "new" && mach.channel === "instant" ? 1 : 0
                visible: opacity > 0
                Behavior on opacity { NumberAnimation { duration: Tokens.swap } }
            }
            IsoPanel {
                anchors.fill: parent
                opacity: mach.mode === "new" && mach.channel === "iso" ? 1 : 0
                visible: opacity > 0
                Behavior on opacity { NumberAnimation { duration: Tokens.swap } }
            }
        }

        Column {
            anchors.centerIn: parent
            width: Math.min(parent.width - Tokens.s6 * 2, 560)
            spacing: Tokens.s3
            visible: main.emptyLibrary

            Mark { anchors.horizontalCenter: parent.horizontalCenter; size: 80 }
            Text {
                width: parent.width
                horizontalAlignment: Text.AlignHCenter
                text: I18n.tr("Build a local machine")
                color: Tokens.ink
                font.family: Tokens.display
                font.pixelSize: 24
            }
            Text {
                width: parent.width
                horizontalAlignment: Text.AlignHCenter
                wrapMode: Text.WordWrap
                text: I18n.tr("Create from the catalogue or import an installer ISO, then launch and operate the VM from this plate.")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: 12
            }
            Row {
                anchors.horizontalCenter: parent.horizontalCenter
                spacing: Tokens.s2
                Btn {
                    text: I18n.tr("CREATE VM")
                    primary: true
                    onAct: { mach.channel = "catalog"; mach.mode = "new"; }
                }
                Btn {
                    text: I18n.tr("IMPORT ISO")
                    onAct: { mach.channel = "iso"; mach.mode = "new"; }
                }
            }
        }
    }

    // ---- fault strip: sticky until dismissed or superseded -----------------
    Rectangle {
        id: faultStrip
        anchors.bottom: bottomBar.top
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.leftMargin: Tokens.s6
        anchors.rightMargin: Tokens.s6
        anchors.bottomMargin: Vm.fault.length > 0 ? Tokens.s2 : 0
        height: Vm.fault.length > 0 ? faultCol.implicitHeight + 2 * Tokens.s2 : 0
        visible: Vm.fault.length > 0
        color: "transparent"
        radius: Tokens.radius
        border.width: Tokens.border
        border.color: Tokens.line
        antialiasing: false
        property bool expanded: false

        Column {
            id: faultCol
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.margins: Tokens.s2
            spacing: Tokens.s2

            Item {
                width: parent.width
                height: 22
                Row {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s3
                    Annunciator { anchors.verticalCenter: parent.verticalCenter; label: I18n.tr("FAULT"); lit: true; warn: true; tileW: 52 }
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        width: faultStrip.width - 240
                        elide: Text.ElideRight
                        text: Vm.fault
                        color: Tokens.ink
                        font.family: Tokens.ui
                        font.pixelSize: 12
                    }
                }
                Row {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s3
                    Text {
                        visible: Vm.faultDetail.indexOf("\n") >= 0
                        anchors.verticalCenter: parent.verticalCenter
                        text: faultStrip.expanded ? I18n.tr("LESS") : I18n.tr("DETAIL")
                        color: fdh.hovered ? Tokens.ink : Tokens.inkMuted
                        font.family: Tokens.mono
                        font.pixelSize: 9
                        font.letterSpacing: 1.5
                        HoverHandler { id: fdh; cursorShape: Qt.PointingHandCursor }
                        TapHandler { onTapped: faultStrip.expanded = !faultStrip.expanded }
                    }
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("DISMISS")
                        color: fxh.hovered ? Tokens.ink : Tokens.inkMuted
                        font.family: Tokens.mono
                        font.pixelSize: 9
                        font.letterSpacing: 1.5
                        HoverHandler { id: fxh; cursorShape: Qt.PointingHandCursor }
                        TapHandler { onTapped: { faultStrip.expanded = false; Vm.clearFault(); } }
                    }
                }
            }

            Flickable {
                visible: faultStrip.expanded
                width: parent.width
                height: Math.min(faultDetailText.implicitHeight, 140)
                contentHeight: faultDetailText.implicitHeight
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                Text {
                    id: faultDetailText
                    width: parent.width
                    wrapMode: Text.WrapAnywhere
                    text: Vm.faultDetail
                    color: Tokens.inkMuted
                    font.family: Tokens.mono
                    font.pixelSize: 11
                }
            }
        }
    }

    // ---- bottom bar --------------------------------------------------------
    Item {
        id: bottomBar
        anchors.bottom: parent.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        height: 60

        Rectangle { anchors.top: parent.top; width: parent.width; height: 1; color: Tokens.line }

        Row {
            anchors.verticalCenter: parent.verticalCenter
            anchors.left: parent.left
            anchors.leftMargin: Tokens.s6
            spacing: Tokens.s3
            Rectangle {
                width: 6; height: 6
                anchors.verticalCenter: parent.verticalCenter
                color: Tokens.ink
                SequentialAnimation on opacity {
                    running: mach.pending
                    loops: Animation.Infinite
                    NumberAnimation { to: 0.3; duration: 600 }
                    NumberAnimation { to: 1.0; duration: 600 }
                }
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: Vm.downloading ? (Vm.dlCount > 1 ? I18n.tr("BUILDING ") + Vm.dlCount + I18n.tr(" MACHINES") : I18n.tr("BUILDING · ") + Vm.dlName)
                    : Vm.busy ? I18n.tr("WORKING")
                    : Vm.status.length > 0 ? Vm.status
                    : (mach.runningCount > 0 ? mach.runningCount + I18n.tr(" RUNNING") : I18n.tr("READY"))
                color: Vm.status.length > 0 || mach.pending ? Tokens.ink : Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: 11
                font.weight: Font.Medium
                font.letterSpacing: 1.4
                font.capitalization: Font.AllUppercase
            }
        }

        Row {
            anchors.verticalCenter: parent.verticalCenter
            anchors.right: parent.right
            anchors.rightMargin: Tokens.s6
            spacing: Tokens.s4
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                visible: Vm.downloading
                text: Vm.dlCount > 1 ? I18n.tr("CANCEL ALL") : I18n.tr("CANCEL")
                onAct: Vm.cancelCreate()
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: Vm.caps.version ? "quickemu " + Vm.caps.version : "quickemu"
                color: Tokens.inkFaint
                font.family: Tokens.mono
                font.pixelSize: 9
            }
        }
    }
}
