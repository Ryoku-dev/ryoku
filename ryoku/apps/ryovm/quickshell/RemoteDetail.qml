pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"

Item {
    id: detail

    property string alias: ""
    property string tab: "overview"
    signal edit(string alias)

    readonly property var host: {
        for (var i = 0; i < Remotes.hosts.length; i++)
            if (Remotes.hosts[i].alias === alias) return Remotes.hosts[i];
        return null;
    }
    readonly property var health: alias.length > 0 ? Remotes.healthOf(alias) : null
    readonly property var reach: alias.length > 0 ? Remotes.reachOf(alias) : null
    readonly property string state: alias.length > 0 ? Remotes.stateOf(alias) : "unknown"
    readonly property bool probed: health && health.ok === true
    readonly property var activity: {
        var rows = [];
        for (var i = Remotes.events.length - 1; i >= 0; i--)
            if (Remotes.events[i].alias === alias) rows.push(Remotes.events[i]);
        return rows;
    }

    component Fact: Column {
        id: fact
        property string label: ""
        property string value: "-"
        spacing: 3
        Text { text: fact.label; color: Tokens.inkFaint; font.family: Tokens.ui; font.pixelSize: 9 }
        Text { width: fact.width; text: fact.value || "-"; elide: Text.ElideRight; color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: 11 }
    }

    component Head: Text {
        width: parent ? parent.width : 0
        color: Tokens.ink
        font.family: Tokens.ui
        font.pixelSize: 11
        font.weight: Font.DemiBold
    }

    Item {
        id: header
        anchors { top: parent.top; left: parent.left; right: parent.right }
        height: 72

        Column {
            anchors.left: parent.left
            anchors.right: actions.left
            anchors.rightMargin: Tokens.s3
            anchors.verticalCenter: parent.verticalCenter
            spacing: 3
            Text { width: parent.width; text: detail.alias; elide: Text.ElideRight; color: Tokens.ink; font.family: Tokens.display; font.pixelSize: 28 }
            Text {
                width: parent.width
                elide: Text.ElideRight
                text: (detail.host && detail.host.user ? detail.host.user + "@" : "")
                    + (detail.host ? (detail.host.hostName || detail.alias) : detail.alias)
                    + (detail.host && detail.host.port && detail.host.port !== 22 ? ":" + detail.host.port : "")
                color: Tokens.inkMuted
                font.family: Tokens.mono
                font.pixelSize: 10
            }
        }
        Row {
            id: actions
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s2
            Annunciator {
                anchors.verticalCenter: parent.verticalCenter
                label: ({ up: I18n.tr("UP"), warn: I18n.tr("DEGRADED"), down: I18n.tr("DOWN"), unknown: I18n.tr("UNKNOWN") })[detail.state] || I18n.tr("UNKNOWN")
                lit: detail.state === "up" || detail.state === "warn"
                warn: detail.state === "warn"
                tileW: 74
            }
            Btn { text: I18n.tr("CONNECT"); primary: true; onAct: Remotes.connect(detail.alias) }
            Btn { text: I18n.tr("PROBE"); onAct: Remotes.probe(detail.alias) }
        }
    }

    Flow {
        id: tabs
        anchors { top: header.bottom; left: parent.left; right: parent.right }
        spacing: Tokens.s1
        Repeater {
            model: [
                { key: "overview", label: I18n.tr("OVERVIEW") },
                { key: "access", label: I18n.tr("ACCESS") },
                { key: "networking", label: I18n.tr("NETWORKING") },
                { key: "workloads", label: I18n.tr("WORKLOADS") },
                { key: "settings", label: I18n.tr("SETTINGS") },
                { key: "activity", label: I18n.tr("ACTIVITY") }
            ]
            Btn {
                required property var modelData
                text: modelData.label
                primary: detail.tab === modelData.key
                compact: true
                onAct: detail.tab = modelData.key
            }
        }
    }

    Flickable {
        anchors { top: tabs.bottom; topMargin: Tokens.s3; left: parent.left; right: parent.right; bottom: parent.bottom }
        contentWidth: width
        contentHeight: content.implicitHeight + Tokens.s4
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }

        Column {
            id: content
            width: parent.width - Tokens.s2
            spacing: Tokens.s4

            Column {
                width: parent.width
                spacing: Tokens.s4
                visible: detail.tab === "overview"

                MetricsPanel {
                    width: parent.width
                    height: 300
                    cpuValues: Remotes.series(detail.alias, "cpu")
                    ramValues: Remotes.series(detail.alias, "ram")
                    diskValues: Remotes.series(detail.alias, "disk")
                    netValues: Remotes.series(detail.alias, "net")
                    samplePeriodSeconds: 15
                }

                Rectangle {
                    width: parent.width
                    implicitHeight: facts.implicitHeight + Tokens.s4 * 2
                    color: "transparent"
                    border.width: Tokens.border
                    border.color: Tokens.line
                    antialiasing: false
                    Grid {
                        id: facts
                        anchors.fill: parent
                        anchors.margins: Tokens.s4
                        columns: 3
                        rowSpacing: Tokens.s4
                        columnSpacing: Tokens.s4
                        Repeater {
                            model: [
                                { label: I18n.tr("Operating system"), value: detail.probed ? detail.health.distro : "-" },
                                { label: I18n.tr("Kernel"), value: detail.probed ? detail.health.kernel : "-" },
                                { label: I18n.tr("Uptime"), value: detail.probed ? Remotes.uptimeShort(detail.health.uptimeS) : "-" },
                                { label: I18n.tr("CPU configuration"), value: detail.probed ? detail.health.cpus + I18n.tr(" cores") : "-" },
                                { label: I18n.tr("Memory"), value: detail.probed ? Remotes.human((detail.health.memTotalKb - detail.health.memAvailKb) * 1024) + " / " + Remotes.human(detail.health.memTotalKb * 1024) : "-" },
                                { label: I18n.tr("Disk usage"), value: detail.probed ? detail.health.diskPct + "% / " + Remotes.human(detail.health.diskTotalKb * 1024) : "-" },
                                { label: I18n.tr("Round trip"), value: detail.reach && detail.reach.up ? detail.reach.rttMs + " ms" : "-" },
                                { label: I18n.tr("Signed-in users"), value: detail.probed ? String(detail.health.logins) : "-" },
                                { label: I18n.tr("Failed units"), value: detail.probed ? String(detail.health.failedUnits) : "-" },
                                { label: I18n.tr("Host name"), value: detail.probed ? detail.health.host : "-" },
                                { label: I18n.tr("Load average"), value: detail.probed ? Number(detail.health.load1).toFixed(2) + " / " + Number(detail.health.load5).toFixed(2) + " / " + Number(detail.health.load15).toFixed(2) : "-" },
                                { label: I18n.tr("Disk used"), value: detail.probed ? Remotes.human(detail.health.diskUsedKb * 1024) : "-" }
                            ]
                            Fact { required property var modelData; width: (facts.width - Tokens.s4 * 2) / 3; label: modelData.label; value: modelData.value }
                        }
                    }
                }
            }

            Column {
                width: parent.width
                spacing: Tokens.s4
                visible: detail.tab === "access"

                Head { text: I18n.tr("Access") }
                Row {
                    spacing: Tokens.s2
                    Btn { text: I18n.tr("OPEN TERMINAL"); primary: true; onAct: Remotes.connect(detail.alias) }
                    Btn { text: I18n.tr("FILES"); onAct: Remotes.openFiles(detail.host) }
                    Btn { text: I18n.tr("DEPLOY KEY"); onAct: Remotes.copyId(detail.alias) }
                }
                Row {
                    width: parent.width
                    spacing: Tokens.s2
                    Field { id: command; width: parent.width - runCommand.width - Tokens.s2; placeholder: I18n.tr("Run one command in a terminal") }
                    Btn { id: runCommand; text: I18n.tr("RUN"); armed: command.text.trim().length > 0; onAct: Remotes.runOn(detail.alias, command.text.trim()) }
                }

                Head { text: I18n.tr("Top processes") }
                Text { visible: !detail.probed; text: I18n.tr("Probe the host to read its process table."); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 11 }
                Repeater {
                    model: detail.probed ? detail.health.topProcs : []
                    Item {
                        required property string modelData
                        width: parent ? parent.width : 0
                        height: 34
                        Rectangle { anchors.bottom: parent.bottom; width: parent.width; height: 1; color: Tokens.lineSoft }
                        Text { anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter; text: modelData; color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: 10 }
                    }
                }

                Head { text: I18n.tr("Watched services") }
                Text { visible: !detail.host || !detail.host.watch || detail.host.watch.length === 0; text: I18n.tr("No services are watched on this host."); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 11 }
                Repeater {
                    model: detail.host && detail.host.watch ? detail.host.watch : []
                    Rectangle {
                        id: serviceRow
                        required property string modelData
                        width: parent ? parent.width : 0
                        height: 38
                        color: "transparent"
                        border.width: Tokens.border
                        border.color: Tokens.lineSoft
                        Text { anchors.left: parent.left; anchors.leftMargin: Tokens.s3; anchors.verticalCenter: parent.verticalCenter; text: serviceRow.modelData; color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: 10 }
                        Text { anchors.right: parent.right; anchors.rightMargin: Tokens.s3; anchors.verticalCenter: parent.verticalCenter; text: detail.probed && detail.health.services ? (detail.health.services[serviceRow.modelData] || I18n.tr("unknown")) : I18n.tr("unknown"); color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: 9 }
                    }
                }
            }

            Column {
                width: parent.width
                spacing: Tokens.s4
                visible: detail.tab === "networking"

                Head { text: I18n.tr("SSH endpoint") }
                Rectangle {
                    width: parent.width
                    height: 72
                    color: "transparent"
                    border.width: Tokens.border
                    border.color: Tokens.line
                    Row {
                        anchors.fill: parent
                        anchors.margins: Tokens.s4
                        spacing: Tokens.s5
                        Fact { width: (parent.width - Tokens.s5 * 2) / 3; label: I18n.tr("Address"); value: detail.host ? detail.host.hostName : "-" }
                        Fact { width: (parent.width - Tokens.s5 * 2) / 3; label: I18n.tr("Port"); value: detail.host ? String(detail.host.port || 22) : "-" }
                        Fact { width: (parent.width - Tokens.s5 * 2) / 3; label: I18n.tr("Proxy jump"); value: detail.host && detail.host.proxyJump ? detail.host.proxyJump : "-" }
                    }
                }

                Head { text: I18n.tr("SSH tunnels") }
                Text { visible: Remotes.tunnelsFor(detail.alias).length === 0; text: I18n.tr("No active tunnels."); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 11 }
                Repeater {
                    model: Remotes.tunnelsFor(detail.alias)
                    Rectangle {
                        id: tunnelRow
                        required property var modelData
                        width: parent ? parent.width : 0
                        height: 42
                        color: "transparent"
                        border.width: Tokens.border
                        border.color: Tokens.lineSoft
                        Text { anchors.left: parent.left; anchors.leftMargin: Tokens.s3; anchors.verticalCenter: parent.verticalCenter; text: tunnelRow.modelData.kind + "  " + tunnelRow.modelData.spec; color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: 10 }
                        Btn { anchors.right: parent.right; anchors.rightMargin: Tokens.s2; anchors.verticalCenter: parent.verticalCenter; compact: true; text: I18n.tr("CLOSE"); onAct: Remotes.closeTunnel(tunnelRow.modelData.id) }
                    }
                }
                Row {
                    width: parent.width
                    spacing: Tokens.s2
                    Field { id: tunnelSpec; width: parent.width - openTunnel.width - Tokens.s2; placeholder: I18n.tr("L:8080:127.0.0.1:80, R:..., or D:1080") }
                    Btn { id: openTunnel; text: I18n.tr("OPEN"); primary: true; armed: tunnelSpec.text.trim().length > 0; onAct: { Remotes.openTunnel(detail.alias, tunnelSpec.text.trim()); tunnelSpec.clear(); } }
                }
            }

            Column {
                width: parent.width
                spacing: Tokens.s4
                visible: detail.tab === "workloads"

                Head { text: I18n.tr("Web applications") }
                Text { visible: !detail.host || !detail.host.apps || detail.host.apps.length === 0; text: I18n.tr("No web applications are registered."); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 11 }
                Repeater {
                    model: detail.host && detail.host.apps ? detail.host.apps : []
                    Rectangle {
                        id: appRow
                        required property var modelData
                        readonly property var status: Remotes.appStatusOf(detail.alias, appRow.modelData.name)
                        width: parent ? parent.width : 0
                        height: 46
                        color: "transparent"
                        border.width: Tokens.border
                        border.color: Tokens.lineSoft
                        Text { anchors.left: parent.left; anchors.leftMargin: Tokens.s3; anchors.verticalCenter: parent.verticalCenter; text: appRow.modelData.name; color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: 11; font.weight: Font.Medium }
                        Text { anchors.right: openApp.left; anchors.rightMargin: Tokens.s3; anchors.verticalCenter: parent.verticalCenter; text: appRow.status ? appRow.status.state.toUpperCase() + " / " + appRow.status.ms + " ms" : I18n.tr("UNKNOWN"); color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: 9 }
                        Btn { id: openApp; anchors.right: parent.right; anchors.rightMargin: Tokens.s2; anchors.verticalCenter: parent.verticalCenter; compact: true; text: I18n.tr("OPEN"); onAct: Remotes.openApp(appRow.modelData.url) }
                    }
                }

                Head { text: I18n.tr("Proxmox guests") }
                Text { visible: !Remotes.isProxmox(detail.host); text: I18n.tr("This host has no Proxmox endpoint configured."); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 11 }
                Repeater {
                    model: Remotes.guestsOf(detail.alias)
                    Rectangle {
                        id: guestRow
                        required property var modelData
                        readonly property bool running: guestRow.modelData.status === "running"
                        width: parent ? parent.width : 0
                        height: 52
                        color: "transparent"
                        border.width: Tokens.border
                        border.color: Tokens.lineSoft
                        Column {
                            anchors.left: parent.left
                            anchors.leftMargin: Tokens.s3
                            anchors.right: guestActions.left
                            anchors.rightMargin: Tokens.s2
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: 2
                            Text { width: parent.width; elide: Text.ElideRight; text: guestRow.modelData.vmid + " / " + (guestRow.modelData.name || guestRow.modelData.type); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: 11; font.weight: Font.Medium }
                            Text { text: guestRow.modelData.node + " / " + guestRow.modelData.type + " / " + guestRow.modelData.status; color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: 9 }
                        }
                        Row {
                            id: guestActions
                            anchors.right: parent.right
                            anchors.rightMargin: Tokens.s2
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: Tokens.s2
                            Btn { compact: true; text: guestRow.running ? I18n.tr("SHUTDOWN") : I18n.tr("START"); armed: !Remotes.guestBusyOf(detail.alias, guestRow.modelData.vmid); onAct: Remotes.pveAct(detail.alias, guestRow.modelData.node, guestRow.modelData.type, guestRow.modelData.vmid, guestRow.running ? "shutdown" : "start") }
                            Btn { compact: true; visible: guestRow.running; text: I18n.tr("STOP"); armed: !Remotes.guestBusyOf(detail.alias, guestRow.modelData.vmid); onAct: Remotes.pveAct(detail.alias, guestRow.modelData.node, guestRow.modelData.type, guestRow.modelData.vmid, "stop") }
                        }
                    }
                }
            }

            Column {
                width: parent.width
                spacing: Tokens.s4
                visible: detail.tab === "settings"

                Head { text: I18n.tr("Remote record") }
                Rectangle {
                    width: parent.width
                    implicitHeight: notes.implicitHeight + Tokens.s4 * 2
                    color: "transparent"
                    border.width: Tokens.border
                    border.color: Tokens.line
                    Column {
                        id: notes
                        anchors.fill: parent
                        anchors.margins: Tokens.s4
                        spacing: Tokens.s2
                        Text { text: detail.host && detail.host.group ? detail.host.group : I18n.tr("Ungrouped"); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: 12; font.weight: Font.Medium }
                        Text { visible: detail.host && detail.host.tags && detail.host.tags.length > 0; text: detail.host ? detail.host.tags.join(" / ") : ""; color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: 9 }
                        Text { width: parent.width; wrapMode: Text.WordWrap; text: detail.host && detail.host.notes ? detail.host.notes : I18n.tr("No notes for this remote."); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 11 }
                        Text { width: parent.width; wrapMode: Text.WrapAnywhere; text: detail.host && detail.host.identityFile ? detail.host.identityFile : I18n.tr("Default SSH identity"); color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: 9 }
                        Text { text: detail.host && detail.host.auth === "password" ? I18n.tr("Saved password authentication") : I18n.tr("SSH key or agent authentication"); color: Tokens.inkFaint; font.family: Tokens.ui; font.pixelSize: 10 }
                    }
                }
                Row {
                    spacing: Tokens.s2
                    Btn { text: I18n.tr("EDIT REMOTE"); primary: true; onAct: detail.edit(detail.alias) }
                    Btn { text: I18n.tr("REFRESH KEYS"); onAct: Remotes.loadKeys() }
                    GuardSwitch { label: I18n.tr("FORGET REMOTE"); armedLabel: I18n.tr("FORGET NOW"); onFired: Remotes.removeHost(detail.alias) }
                }
            }

            Column {
                width: parent.width
                spacing: Tokens.s2
                visible: detail.tab === "activity"
                Head { text: I18n.tr("Activity") }
                Text {
                    width: parent.width
                    wrapMode: Text.WordWrap
                    text: I18n.tr("Status changes, connections, tunnels, and remote actions recorded in this session.")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                }
                Text { visible: detail.activity.length === 0; text: I18n.tr("Nothing has been recorded for this remote yet."); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 11 }
                Repeater {
                    model: detail.activity
                    Item {
                        id: activityRow
                        required property var modelData
                        width: parent ? parent.width : 0
                        height: 40
                        Rectangle { anchors.bottom: parent.bottom; width: parent.width; height: 1; color: Tokens.lineSoft }
                        Text { anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter; width: 62; text: activityRow.modelData.time; color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: 9 }
                        Text { anchors.left: parent.left; anchors.leftMargin: 70; anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter; elide: Text.ElideRight; text: activityRow.modelData.text; color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 11 }
                    }
                }
            }
        }
    }
}
