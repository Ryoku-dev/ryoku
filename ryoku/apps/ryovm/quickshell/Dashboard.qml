pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"

Item {
    id: dash

    property bool active: false
    property string selectedKind: ""
    property string selectedKey: ""
    property string detailTab: "overview"

    signal navigate(string key)
    signal newMachine()
    signal newRemote()
    signal openMachine(string name)
    signal openRemote(string alias)
    signal openConsole(string id)
    signal openLog()

    readonly property int runningCount: {
        var count = 0;
        for (var i = 0; i < Vm.vms.length; i++)
            if (Vm.vms[i].running === true)
                count++;
        return count;
    }
    readonly property int alertCount: Remotes.alertCount + (Vm.fault && Vm.fault.length > 0 ? 1 : 0)
    readonly property bool authBlocked: {
        void Remotes.healthRev;
        var h = selectedKind === "remote" ? Remotes.healthOf(selectedKey) : null;
        return !!(h && h.needAuth === true);
    }
    readonly property int needAuthCount: {
        void Remotes.healthRev;
        var n = 0;
        for (var i = 0; i < Remotes.alerts.length; i++)
            if (Remotes.alerts[i].kind === "auth") n++;
        return n;
    }
    readonly property var resources: {
        void Remotes.healthRev;
        void Remotes.reachRev;
        void Remotes.consolesRev;
        var rows = [];
        var i;
        for (i = 0; i < Vm.vms.length; i++) {
            var vm = Vm.vms[i];
            rows.push({ kind: "vm", key: vm.name, title: vm.name,
                subtitle: (vm.os || vm.guest || I18n.tr("Virtual machine")),
                state: vm.running === true ? "running" : "stopped",
                reason: "", auth: false,
                sort: vm.running === true ? 1 : 3 });
        }
        for (i = 0; i < Remotes.hosts.length; i++) {
            var host = Remotes.hosts[i];
            var state = Remotes.stateOf(host.alias);
            var h = Remotes.healthOf(host.alias);
            var auth = !!(h && h.needAuth === true);
            rows.push({ kind: "remote", key: host.alias, title: host.alias,
                subtitle: (host.user ? host.user + "@" : "") + (host.hostName || host.alias),
                state: state,
                auth: auth,
                reason: auth ? I18n.tr("password needed for probes")
                       : (state === "down" && h && h.error) ? h.error
                       : state === "warn" ? Remotes.stateReason(host.alias, "warn")
                       : "",
                sort: state === "down" ? 0 : state === "warn" ? 1 : state === "up" ? 2 : 3 });
        }
        for (i = 0; i < Remotes.consoles.length; i++) {
            var c = Remotes.consoles[i];
            var cstate = Remotes.consoleStateOf(c);
            rows.push({ kind: "console", key: c.id, title: c.name,
                subtitle: c.kind + " · " + Remotes.consoleTarget(c),
                state: cstate, auth: false,
                reason: cstate === "up" ? I18n.tr("ready")
                        : cstate === "down" ? I18n.tr("not reachable")
                        : I18n.tr("not probed"),
                sort: cstate === "down" ? 0 : cstate === "up" ? 2 : 3 });
        }
        rows.sort(function(a, b) { return a.sort !== b.sort ? a.sort - b.sort : a.title.localeCompare(b.title); });
        return rows;
    }
    readonly property var selectedRow: {
        for (var i = 0; i < resources.length; i++)
            if (resources[i].kind === selectedKind && resources[i].key === selectedKey)
                return resources[i];
        return null;
    }
    readonly property var selectedVm: {
        if (selectedKind !== "vm") return null;
        for (var i = 0; i < Vm.vms.length; i++)
            if (Vm.vms[i].name === selectedKey) return Vm.vms[i];
        return null;
    }
    readonly property var selectedRemote: selectedKind === "remote" ? Remotes.healthOf(selectedKey) : null
    readonly property var activity: {
        var rows = [], i, event;
        if (selectedKind === "vm") {
            for (i = 0; i < Vm.events.length; i++) {
                event = Vm.events[i];
                if (event.vm === selectedKey)
                    rows.push({ at: event.at || 0, time: event.time, text: event.text, kind: event.kind });
            }
        } else if (selectedKind === "remote" || selectedKind === "console") {
            // console events carry the berth id in the alias slot.
            for (i = 0; i < Remotes.events.length; i++) {
                event = Remotes.events[i];
                if (event.alias === selectedKey)
                    rows.push({ at: event.at || 0, time: event.time, text: event.text, kind: event.kind });
            }
        }
        rows.sort(function(a, b) { return b.at - a.at; });
        return rows.slice(0, 20);
    }

    onResourcesChanged: {
        if (!selectedRow && resources.length > 0) {
            selectedKind = resources[0].kind;
            selectedKey = resources[0].key;
        }
    }

    Keys.onUpPressed: selectOffset(-1)
    Keys.onDownPressed: selectOffset(1)
    Keys.onReturnPressed: openSelected()
    Keys.onEnterPressed: openSelected()

    function selectOffset(offset) {
        if (resources.length === 0) return;
        var index = 0;
        for (var i = 0; i < resources.length; i++)
            if (resources[i].kind === selectedKind && resources[i].key === selectedKey) { index = i; break; }
        index = Math.max(0, Math.min(resources.length - 1, index + offset));
        selectedKind = resources[index].kind;
        selectedKey = resources[index].key;
    }
    function openSelected() {
        if (selectedKind === "vm") openMachine(selectedKey);
        else if (selectedKind === "remote") openRemote(selectedKey);
        else if (selectedKind === "console") openConsole(selectedKey);
    }

    PageHead {
        id: header
        anchors { top: parent.top; left: parent.left; right: parent.right }
        anchors.leftMargin: Tokens.s6
        anchors.rightMargin: Tokens.s6
        anchors.topMargin: Tokens.s5
        eyebrow: I18n.tr("FLEET")
        title: I18n.tr("Ryoport")
        blurb: I18n.tr("Local machines and remote systems, read as one operating fleet.")
    }

    Rectangle {
        id: summary
        anchors { top: header.bottom; left: parent.left; right: parent.right }
        anchors.leftMargin: Tokens.s6
        anchors.rightMargin: Tokens.s6
        anchors.topMargin: Tokens.s3
        height: 84
        color: "transparent"
        border.width: Tokens.border
        border.color: Tokens.line
        antialiasing: false

        readonly property var cells: [
            { label: I18n.tr("Fleet"), value: String(Vm.vms.length + Remotes.hostCount).padStart(2, "0"),
              sub: I18n.tr("%1 machines · %2 remotes").arg(Vm.vms.length).arg(Remotes.hostCount), clickable: false },
            { label: I18n.tr("Running VMs"), value: String(dash.runningCount).padStart(2, "0"),
              sub: Vm.vms.length === 0 ? I18n.tr("the yard is empty") : I18n.tr("of %1 in the yard").arg(Vm.vms.length),
              clickable: false },
            { label: I18n.tr("Reachable"), value: String(Remotes.upCount).padStart(2, "0") + I18n.tr("/") + String(Remotes.hostCount),
              sub: Remotes.probing ? I18n.tr("probing the fleet…") : I18n.tr("ssh and console berths"), clickable: false },
            { label: I18n.tr("Alerts"), value: String(dash.alertCount).padStart(2, "0"),
              sub: dash.alertCount === 0 ? I18n.tr("all quiet")
                   : dash.needAuthCount > 0 ? I18n.tr("%1 need a password").arg(dash.needAuthCount)
                   : I18n.tr("open the harbour log"), clickable: true }
        ]

        Row {
            anchors.fill: parent
            Repeater {
                model: summary.cells
                Item {
                    id: cell
                    required property var modelData
                    required property int index
                    width: summary.width / 4
                    height: summary.height
                    readonly property bool hot: cell.modelData.clickable === true
                    // the hover wash sits at the bottom of the stack; the alert
                    // cell is the door to the harbour log.
                    Rectangle {
                        anchors.fill: parent
                        color: Tokens.tint10
                        opacity: cell.hot && cellHover.hovered ? 1 : 0
                        Behavior on opacity { NumberAnimation { duration: Tokens.snap } }
                    }
                    Rectangle {
                        visible: index > 0
                        anchors.left: parent.left
                        width: 1
                        height: parent.height
                        color: Tokens.line
                    }
                    Rectangle {
                        anchors { top: parent.top; right: parent.right; margins: Tokens.s3 }
                        width: 6; height: 6
                        visible: cell.hot && dash.alertCount > 0
                        color: Tokens.bone
                        border.width: 1
                        border.color: Tokens.ink
                    }
                    Column {
                        anchors.left: parent.left
                        anchors.leftMargin: Tokens.s4
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: 2
                        Text { text: cell.modelData.label; color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 10 }
                        Text { text: cell.modelData.value; color: Tokens.ink; font.family: Tokens.display; font.pixelSize: 26 }
                        Text {
                            width: cell.width - Tokens.s4
                            text: cell.modelData.sub
                            elide: Text.ElideRight
                            color: Tokens.inkFaint
                            font.family: Tokens.mono; font.pixelSize: 8
                        }
                    }
                    HoverHandler { id: cellHover; cursorShape: cell.hot ? Qt.PointingHandCursor : Qt.ArrowCursor }
                    TapHandler {
                        enabled: cell.hot
                        onTapped: dash.openLog()
                    }
                }
            }
        }
    }

    Item {
        id: body
        anchors { top: summary.bottom; left: parent.left; right: parent.right; bottom: parent.bottom }
        anchors.leftMargin: Tokens.s6
        anchors.rightMargin: Tokens.s6
        anchors.topMargin: Tokens.s4
        anchors.bottomMargin: Tokens.s5

        Rectangle {
            id: resourcePane
            anchors { left: parent.left; top: parent.top; bottom: parent.bottom }
            width: Math.max(260, Math.min(330, parent.width * 0.31))
            color: "transparent"
            border.width: Tokens.border
            border.color: Tokens.line
            antialiasing: false

            Item {
                id: listHead
                anchors { top: parent.top; left: parent.left; right: parent.right }
                height: 48
                Text {
                    anchors.left: parent.left
                    anchors.leftMargin: Tokens.s3
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Resources")
                    color: Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: 13
                    font.weight: Font.DemiBold
                }
                Text {
                    anchors.right: parent.right
                    anchors.rightMargin: Tokens.s3
                    anchors.verticalCenter: parent.verticalCenter
                    text: String(dash.resources.length).padStart(2, "0")
                    color: Tokens.inkMuted
                    font.family: Tokens.mono
                    font.pixelSize: 10
                }
                Rectangle { anchors.bottom: parent.bottom; width: parent.width; height: 1; color: Tokens.line }
            }

            ListView {
                id: resourceList
                anchors { top: listHead.bottom; left: parent.left; right: parent.right; bottom: listActions.top }
                clip: true
                model: dash.resources
                currentIndex: {
                    for (var i = 0; i < dash.resources.length; i++)
                        if (dash.resources[i].kind === dash.selectedKind && dash.resources[i].key === dash.selectedKey) return i;
                    return -1;
                }
                ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
                delegate: Rectangle {
                    id: resourceRow
                    required property var modelData
                    required property int index
                    width: resourceList.width
                    height: 68
                    readonly property bool sel: resourceList.currentIndex === index
                    readonly property var series: resourceRow.modelData.kind === "vm"
                        ? Vm.series(resourceRow.modelData.key, "cpu")
                        : (resourceRow.modelData.kind === "console"
                            ? [] : Remotes.series(resourceRow.modelData.key, "cpu"))
                    color: sel ? Tokens.bone : "transparent"
                    antialiasing: false
                    Rectangle { anchors.bottom: parent.bottom; width: parent.width; height: 1; color: sel ? Tokens.inkOnBone : Tokens.lineSoft; opacity: 0.35 }
                    // a one-px ink lead marks a row that needs attention, so
                    // the eye finds it without relying on colour words alone.
                    Rectangle {
                        anchors { left: parent.left; top: parent.top; bottom: parent.bottom }
                        width: 2
                        visible: resourceRow.modelData.state === "down" || resourceRow.modelData.state === "warn"
                                 || resourceRow.modelData.auth === true
                        color: sel ? Tokens.inkOnBone : Tokens.ink
                    }
                    Column {
                        anchors.left: parent.left
                        anchors.leftMargin: Tokens.s3
                        anchors.right: resourceSpark.left
                        anchors.rightMargin: Tokens.s2
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: 3
                        Text {
                            width: parent.width
                            text: resourceRow.modelData.title
                                  + (resourceRow.modelData.kind === "console" ? "  ⌁" : "")
                                  + (resourceRow.modelData.auth === true ? "  鍵" : "")
                            elide: Text.ElideRight
                            color: sel ? Tokens.inkOnBone : Tokens.ink
                            font.family: Tokens.ui
                            font.pixelSize: 12
                            font.weight: Font.Medium
                        }
                        Text {
                            width: parent.width
                            text: resourceRow.modelData.subtitle
                            elide: Text.ElideRight
                            color: sel ? Tokens.inkOnBone : Tokens.inkMuted
                            opacity: sel ? 0.72 : 1
                            font.family: Tokens.mono
                            font.pixelSize: 9
                        }
                        Text {
                            width: parent.width
                            text: resourceRow.modelData.reason || ""
                            visible: text.length > 0
                            elide: Text.ElideRight
                            color: sel ? Tokens.inkOnBone : Tokens.inkFaint
                            font.family: Tokens.ui
                            font.pixelSize: 9
                        }
                    }
                    MetricSparkline {
                        id: resourceSpark
                        anchors.right: stateLabel.left
                        anchors.rightMargin: Tokens.s2
                        anchors.verticalCenter: parent.verticalCenter
                        width: 48
                        height: 24
                        visible: resourceRow.series.length > 0
                        values: resourceRow.series
                        fixedMax: 100
                        stroke: sel ? Tokens.inkOnBone : Tokens.inkMuted
                    }
                    Text {
                        id: stateLabel
                        anchors.right: parent.right
                        anchors.rightMargin: Tokens.s3
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr(resourceRow.modelData.state.toUpperCase())
                        color: sel ? Tokens.inkOnBone : Tokens.inkFaint
                        font.family: Tokens.mono
                        font.pixelSize: 9
                    }
                    MouseArea {
                        anchors.fill: parent
                        cursorShape: Qt.PointingHandCursor
                        onClicked: {
                            dash.selectedKind = resourceRow.modelData.kind;
                            dash.selectedKey = resourceRow.modelData.key;
                        }
                        onDoubleClicked: dash.openSelected()
                    }
                }
            }

            Item {
                id: listActions
                anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
                height: 52
                Rectangle { anchors.top: parent.top; width: parent.width; height: 1; color: Tokens.line }
                Row {
                    anchors.centerIn: parent
                    spacing: Tokens.s2
                    Btn { text: I18n.tr("NEW VM"); onAct: dash.newMachine() }
                    Btn { text: I18n.tr("NEW REMOTE"); onAct: dash.newRemote() }
                }
            }
        }

        Item {
            id: overview
            anchors { left: resourcePane.right; leftMargin: Tokens.s5; top: parent.top; right: parent.right; bottom: parent.bottom }

            Item {
                id: resourceHead
                anchors { top: parent.top; left: parent.left; right: parent.right }
                height: 58
                Column {
                    anchors.left: parent.left
                    anchors.right: commands.left
                    anchors.rightMargin: Tokens.s3
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 3
                    Text {
                        width: parent.width
                        text: dash.selectedRow ? dash.selectedRow.title : I18n.tr("Fleet overview")
                        elide: Text.ElideRight
                        color: Tokens.ink
                        font.family: Tokens.display
                        font.pixelSize: 28
                    }
                    Text {
                        text: dash.selectedRow ? dash.selectedRow.subtitle : I18n.tr("Select a resource to inspect its history")
                        color: Tokens.inkMuted
                        font.family: Tokens.mono
                        font.pixelSize: 10
                    }
                }
                Row {
                    id: commands
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s2
                    Btn {
                        visible: dash.selectedKind === "remote"
                        text: I18n.tr("CONNECT")
                        primary: true
                        onAct: Remotes.connect(dash.selectedKey)
                    }
                    Btn {
                        visible: dash.selectedKind === "console"
                        text: I18n.tr("OPEN CONSOLE")
                        primary: true
                        onAct: dash.openConsole(dash.selectedKey)
                    }
                    Btn { text: I18n.tr("OPEN"); onAct: dash.openSelected() }
                }
            }

            Seg {
                id: tabs
                anchors.top: resourceHead.bottom
                anchors.left: parent.left
                options: [I18n.tr("OVERVIEW"), I18n.tr("ACTIVITY")]
                current: dash.detailTab === "activity" ? I18n.tr("ACTIVITY") : I18n.tr("OVERVIEW")
                onChose: (tab) => dash.detailTab = tab === I18n.tr("ACTIVITY") ? "activity" : "overview"
            }

            Text {
                id: overviewBlurb
                anchors { top: tabs.bottom; topMargin: Tokens.s3; left: parent.left; right: parent.right }
                visible: dash.detailTab === "overview" && dash.selectedRow !== null
                text: dash.selectedKind === "console"
                    ? I18n.tr("An experimental berth opened through its own client; the session lives in a terminal or viewer window.")
                    : dash.selectedKind === "remote"
                    ? I18n.tr("Live reachability, health, and resource use for this remote.")
                    : I18n.tr("Live resource use and configuration for this machine.")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
            }

            // charts and the auth note share one slot: a Column skips the
            // invisible child, so neither can ever paint over the other.
            Column {
                id: chartArea
                anchors { top: overviewBlurb.bottom; topMargin: Tokens.s2; left: parent.left; right: parent.right }
                spacing: Tokens.s2

                MetricsPanel {
                    id: charts
                    width: chartArea.width
                    height: Math.min(306, Math.max(230, overview.height * 0.54))
                    visible: dash.detailTab === "overview" && dash.selectedRow !== null && dash.selectedKind !== "console" && !dash.authBlocked
                    cpuValues: dash.selectedKind === "vm" ? Vm.series(dash.selectedKey, "cpu") : Remotes.series(dash.selectedKey, "cpu")
                    ramValues: dash.selectedKind === "vm" ? Vm.series(dash.selectedKey, "ram") : Remotes.series(dash.selectedKey, "ram")
                    diskValues: dash.selectedKind === "vm" ? Vm.series(dash.selectedKey, "disk") : Remotes.series(dash.selectedKey, "disk")
                    netValues: dash.selectedKind === "vm" ? Vm.series(dash.selectedKey, "net") : Remotes.series(dash.selectedKey, "net")
                    samplePeriodSeconds: dash.selectedKind === "remote" ? 15 : 5
                }

                // an auth-blocked remote explains itself where the graphs would be
                // empty, with the fix beside it.
                Rectangle {
                    id: authNote
                    width: chartArea.width
                    height: authNoteCol.implicitHeight + Tokens.s4 * 2
                    visible: dash.detailTab === "overview" && dash.selectedKind === "remote"
                             && dash.selectedRemote !== null && dash.selectedRemote.needAuth === true
                    color: "transparent"
                    border.width: Tokens.border
                    border.color: Tokens.line
                    antialiasing: false
                    Column {
                        id: authNoteCol
                        anchors { fill: parent; margins: Tokens.s4 }
                        spacing: Tokens.s2
                        Text {
                            width: parent.width
                            wrapMode: Text.WordWrap
                            text: I18n.tr("This host answers, but ryoport cannot prove it is healthy without a credential. Save its password (kept in the login keyring, never in a file) and the graphs will fill on the next probe.")
                            color: Tokens.ink
                            font.family: Tokens.ui; font.pixelSize: 11
                        }
                        Row {
                            spacing: Tokens.s2
                            Field {
                                id: dashPwF
                                width: authNote.width - dashSaveBtn.width - dashProbeBtn.width - Tokens.s2 * 2 - Tokens.s4 * 2
                                secret: true
                                tabular: true
                                placeholder: I18n.tr("password for %1").arg(dash.selectedKey)
                            }
                            Btn {
                                id: dashSaveBtn
                                text: I18n.tr("SAVE")
                                primary: true
                                armed: dashPwF.text.length > 0
                                onAct: { Remotes.setPass(dash.selectedKey, dashPwF.text); dashPwF.clear(); }
                            }
                            Btn { id: dashProbeBtn; text: I18n.tr("PROBE"); onAct: Remotes.probe(dash.selectedKey) }
                        }
                    }
                }
            }

            Row {
                id: facts
                anchors { top: chartArea.bottom; topMargin: Tokens.s3; left: parent.left; right: parent.right }
                visible: dash.detailTab === "overview" && dash.selectedRow !== null
                spacing: Tokens.s5
                Repeater {
                    model: dash.selectedKind === "vm" ? [
                        { label: I18n.tr("OS"), value: dash.selectedVm ? (dash.selectedVm.os || dash.selectedVm.guest) : "-" },
                        { label: I18n.tr("CPU"), value: dash.selectedVm ? dash.selectedVm.cores : "-" },
                        { label: I18n.tr("RAM"), value: dash.selectedVm ? dash.selectedVm.ram : "-" },
                        { label: I18n.tr("DISK"), value: dash.selectedVm ? Vm.human(dash.selectedVm.diskUsed) : "-" }
                    ] : dash.selectedKind === "console" ? [
                        { label: I18n.tr("PROTOCOL"), value: Remotes.consoleById(dash.selectedKey) ? Remotes.consoleById(dash.selectedKey).kind : "-" },
                        { label: I18n.tr("TARGET"), value: Remotes.consoleById(dash.selectedKey) ? Remotes.consoleTarget(Remotes.consoleById(dash.selectedKey)) : "-" },
                        { label: I18n.tr("CLIENT"), value: Remotes.consoleById(dash.selectedKey) ? Remotes.consoleClientOf(Remotes.consoleById(dash.selectedKey).kind) : "-" },
                        { label: I18n.tr("CREDENTIAL"), value: Remotes.consoleById(dash.selectedKey) && Remotes.consoleById(dash.selectedKey).auth === "password" ? I18n.tr("keyring") : I18n.tr("none") }
                    ] : [
                        { label: I18n.tr("OS"), value: dash.selectedRemote ? (dash.selectedRemote.distro || "-") : "-" },
                        { label: I18n.tr("CPU"), value: dash.selectedRemote ? dash.selectedRemote.cpus + "c" : "-" },
                        { label: I18n.tr("UPTIME"), value: dash.selectedRemote ? Remotes.uptimeShort(dash.selectedRemote.uptimeS) : "-" },
                        { label: I18n.tr("DISK"), value: dash.selectedRemote ? dash.selectedRemote.diskPct + "%" : "-" }
                    ]
                    Column {
                        required property var modelData
                        width: (facts.width - Tokens.s5 * 3) / 4
                        spacing: 3
                        Text { text: modelData.label; color: Tokens.inkFaint; font.family: Tokens.ui; font.pixelSize: 9 }
                        Text { width: parent.width; text: modelData.value || "-"; elide: Text.ElideRight; color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: 11 }
                    }
                }
            }

            Flickable {
                anchors { top: tabs.bottom; topMargin: Tokens.s3; left: parent.left; right: parent.right; bottom: parent.bottom }
                visible: dash.detailTab === "activity" && dash.selectedRow !== null
                clip: true
                contentHeight: activityTable.implicitHeight
                boundsBehavior: Flickable.StopAtBounds
                ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
                Column {
                    id: activityTable
                    width: parent.width
                    spacing: Tokens.s2
                    Text {
                        width: parent.width
                        wrapMode: Text.WordWrap
                        text: dash.selectedKind === "remote"
                            ? I18n.tr("Status changes, connections, tunnels, and remote actions recorded in this session.")
                            : I18n.tr("Starts, stops, snapshots, configuration changes, and other machine actions recorded in this session.")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                    }
                    Rectangle { width: parent.width; height: 1; color: Tokens.line }
                    Empty {
                        width: parent.width
                        visible: dash.activity.length === 0
                        caption: I18n.tr("Nothing has been recorded for this resource yet.")
                    }
                    Repeater {
                        model: dash.activity
                        Item {
                            id: activityRow
                            required property var modelData
                            width: activityTable.width
                            height: 38
                            Rectangle { anchors.bottom: parent.bottom; width: parent.width; height: 1; color: Tokens.lineSoft }
                            Text { anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter; width: 60; text: activityRow.modelData.time; color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: 9 }
                            Text { anchors.left: parent.left; anchors.leftMargin: 68; anchors.verticalCenter: parent.verticalCenter; width: 112; elide: Text.ElideRight; text: (activityRow.modelData.kind || I18n.tr("event")).toUpperCase(); color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: 9 }
                            Text { anchors.left: parent.left; anchors.leftMargin: 188; anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter; elide: Text.ElideRight; text: activityRow.modelData.text; color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: 11 }
                        }
                    }
                }
            }

            Empty {
                anchors.centerIn: parent
                visible: dash.resources.length === 0
                caption: I18n.tr("No resources yet. Create a machine or add a remote to begin.")
            }
        }
    }
}
