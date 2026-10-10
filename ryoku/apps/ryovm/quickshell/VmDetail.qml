pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"

Item {
    id: pane

    readonly property var vm: Vm.selected
    readonly property var det: Vm.detail
    readonly property bool running: vm ? vm.running === true : false
    readonly property string name: vm ? vm.name : ""
    property string tab: "overview"
    property string launchMode: "window"
    property bool disposableRun: false
    property int diskTarget: 64
    property real balloonMB: 0
    property bool balloonDragging: false
    readonly property int coresNum: vm && vm.cores !== "auto" ? (parseInt(vm.cores) || Vm.settings.defaultCores) : Vm.settings.defaultCores
    readonly property int ramNum: {
        if (!vm || !vm.ram || vm.ram === "auto") return Vm.settings.defaultRam;
        var value = parseFloat(vm.ram) || Vm.settings.defaultRam;
        return vm.ram.indexOf("M") >= 0 ? Math.max(1, Math.round(value / 1024)) : value;
    }
    readonly property int diskCap: {
        if (!vm || !vm.disk) return 0;
        var value = parseInt(vm.disk) || 0;
        return vm.disk.indexOf("M") >= 0 ? Math.max(1, Math.round(value / 1024)) : value;
    }
    readonly property var sealSnapshot: {
        var snapshots = det ? (det.snapshots || []) : [];
        for (var i = 0; i < snapshots.length; i++)
            if (snapshots[i].name === "sealed") return snapshots[i];
        return null;
    }
    readonly property var events: {
        var rows = [];
        for (var i = Vm.events.length - 1; i >= 0; i--)
            if (Vm.events[i].vm === name) rows.push(Vm.events[i]);
        return rows;
    }

    onVmChanged: {
        launchMode = ({ "gtk": "window", "spice": "spice", "none": "headless" })[vm ? vm.display : "gtk"] || "window";
        diskTarget = diskCap > 0 ? diskCap : 64;
    }
    onNameChanged: renameField.text = name

    Connections {
        target: Vm
        function onMonStatsChanged() {
            if (!pane.balloonDragging && Vm.monStats.balloonMB > 0)
                pane.balloonMB = Vm.monStats.balloonMB;
        }
    }
    Timer {
        id: balloonCommit
        interval: 300
        onTriggered: {
            Vm.balloon(pane.name, pane.balloonMB);
            pane.balloonDragging = false;
        }
    }

    component LabelValue: Column {
        id: fact
        property string label: ""
        property string value: "-"
        spacing: 3
        Text { text: fact.label; color: Tokens.inkFaint; font.family: Tokens.ui; font.pixelSize: 9 }
        Text { width: fact.width; text: fact.value || "-"; elide: Text.ElideRight; color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: 11 }
    }

    component ConfirmAction: Btn {
        id: confirm
        property string idleText: ""
        property string confirmText: ""
        property bool ready: true
        property bool asking: false
        signal confirmed()
        text: asking ? confirmText : idleText
        armed: ready
        primary: asking
        onAct: {
            if (!asking) {
                asking = true;
                disarm.restart();
            } else {
                asking = false;
                confirmed();
            }
        }
        Timer { id: disarm; interval: 3000; onTriggered: confirm.asking = false }
    }

    Column {
        anchors.centerIn: parent
        spacing: Tokens.s3
        visible: pane.vm === null
        Text { text: I18n.tr("Select a machine to inspect and operate it"); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 12 }
    }

    Item {
        anchors.fill: parent
        visible: pane.vm !== null

        Item {
            id: heading
            anchors { top: parent.top; left: parent.left; right: parent.right }
            height: 72
            OsIcon {
                id: osIcon
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                size: 42
                slug: pane.vm ? pane.vm.os : ""
                label: pane.name
            }
            Column {
                anchors.left: osIcon.right
                anchors.leftMargin: Tokens.s3
                anchors.right: quickActions.left
                anchors.rightMargin: Tokens.s3
                anchors.verticalCenter: parent.verticalCenter
                spacing: 3
                Text { width: parent.width; text: pane.name; elide: Text.ElideRight; color: Tokens.ink; font.family: Tokens.display; font.pixelSize: 28 }
                Text {
                    text: (pane.running ? I18n.tr("Running") : I18n.tr("Stopped"))
                        + "  /  " + (pane.vm ? (pane.vm.os || pane.vm.guest || "-") : "-")
                        + (Vm.monStats.uptimeS > 0 ? "  /  " + Remotes.uptimeShort(Vm.monStats.uptimeS) : "")
                    color: Tokens.inkMuted
                    font.family: Tokens.mono
                    font.pixelSize: 10
                }
            }
            Row {
                id: quickActions
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: Tokens.s2
                Btn {
                    visible: !pane.running
                    text: pane.disposableRun ? I18n.tr("LAUNCH BURN") : I18n.tr("LAUNCH")
                    primary: true
                    armed: !Vm.busy
                    onAct: Vm.launch(pane.name, pane.launchMode, pane.disposableRun)
                }
                Btn { visible: pane.running; text: I18n.tr("CONSOLE"); primary: true; armed: pane.vm && (pane.vm.spice || "").length > 0; onAct: Vm.openConsole(pane.name) }
                Btn { visible: pane.running; text: I18n.tr("STOP"); armed: !Vm.busy; onAct: Vm.stop(pane.name) }
            }
        }

        Flow {
            id: tabScroll
            anchors { top: heading.bottom; left: parent.left; right: parent.right }
            spacing: Tokens.s1
            Repeater {
                model: [
                    { key: "overview", label: I18n.tr("OVERVIEW") },
                    { key: "access", label: I18n.tr("ACCESS") },
                    { key: "networking", label: I18n.tr("NETWORKING") },
                    { key: "snapshots", label: I18n.tr("SNAPSHOTS") },
                    { key: "settings", label: I18n.tr("SETTINGS") },
                    { key: "activity", label: I18n.tr("ACTIVITY") }
                ]
                Btn {
                    required property var modelData
                    text: modelData.label
                    primary: pane.tab === modelData.key
                    compact: true
                    onAct: pane.tab = modelData.key
                }
            }
        }

        Flickable {
            anchors { top: tabScroll.bottom; topMargin: Tokens.s3; left: parent.left; right: parent.right; bottom: parent.bottom }
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
                    visible: pane.tab === "overview"

                    MetricsPanel {
                        width: parent.width
                        height: 300
                        cpuValues: Vm.series(pane.name, "cpu")
                        ramValues: Vm.series(pane.name, "ram")
                        diskValues: Vm.series(pane.name, "disk")
                        netValues: Vm.series(pane.name, "net")
                    }

                    Rectangle {
                        width: parent.width
                        implicitHeight: factsGrid.implicitHeight + Tokens.s4 * 2
                        color: "transparent"
                        border.width: Tokens.border
                        border.color: Tokens.line
                        antialiasing: false
                        Grid {
                            id: factsGrid
                            anchors.fill: parent
                            anchors.margins: Tokens.s4
                            columns: 3
                            rowSpacing: Tokens.s4
                            columnSpacing: Tokens.s4
                            Repeater {
                                model: [
                                    { label: I18n.tr("Operating system"), value: Vm.monStats.guestOs || (pane.vm ? pane.vm.os || pane.vm.guest : "-") },
                                    { label: I18n.tr("CPU configuration"), value: (pane.vm ? pane.vm.cores : "-") + I18n.tr(" cores") },
                                    { label: I18n.tr("Memory configuration"), value: pane.vm ? pane.vm.ram : "-" },
                                    { label: I18n.tr("Disk usage"), value: pane.vm ? Vm.human(pane.vm.diskUsed) + " / " + (pane.vm.disk || "-") : "-" },
                                    { label: I18n.tr("Guest address"), value: Vm.monStats.guestIp || "-" },
                                    { label: I18n.tr("Display"), value: pane.vm ? (pane.vm.display || "-") : "-" },
                                    { label: I18n.tr("SSH port"), value: pane.vm && pane.vm.ssh ? pane.vm.ssh : "-" },
                                    { label: I18n.tr("SPICE endpoint"), value: pane.vm && pane.vm.spice ? pane.vm.spice : "-" },
                                    { label: I18n.tr("Snapshots"), value: String(pane.det && pane.det.snapshots ? pane.det.snapshots.length : 0) },
                                    { label: I18n.tr("USB assignments"), value: String(Vm.usb.filter(function(device) { return device.assigned === true; }).length) },
                                    { label: I18n.tr("Port forwards"), value: String(Vm.portfwds.length) },
                                    { label: I18n.tr("Boot security"), value: (pane.vm && pane.vm.uefi ? I18n.tr("UEFI") : I18n.tr("BIOS")) + (pane.vm && pane.vm.secureboot ? " / " + I18n.tr("Secure Boot") : "") + (pane.vm && pane.vm.tpm ? " / " + I18n.tr("TPM") : "") },
                                    { label: I18n.tr("Install state"), value: pane.det && pane.det.installed ? I18n.tr("Installed") : I18n.tr("Installer media") },
                                    { label: I18n.tr("Run mode"), value: pane.vm && pane.vm.disposable ? I18n.tr("Disposable") : (pane.vm && pane.vm.sealed ? I18n.tr("Sealed") : I18n.tr("Persistent")) },
                                    { label: I18n.tr("SSH readiness"), value: pane.det && pane.det.sshReady ? I18n.tr("Ready") : I18n.tr("Not ready") },
                                    { label: I18n.tr("Config file"), value: pane.det ? pane.det.conf : "-" },
                                    { label: I18n.tr("Storage path"), value: pane.det ? pane.det.dir : "-" }
                                ]
                                LabelValue { required property var modelData; width: (factsGrid.width - Tokens.s4 * 2) / 3; label: modelData.label; value: modelData.value }
                            }
                        }
                    }
                }

                Column {
                    width: parent.width
                    spacing: Tokens.s4
                    visible: pane.tab === "access"

                    Head { text: I18n.tr("Display and lifecycle") }
                    Seg {
                        options: [I18n.tr("WINDOW"), I18n.tr("SPICE"), I18n.tr("HEADLESS")]
                        current: pane.launchMode === "spice" ? I18n.tr("SPICE") : (pane.launchMode === "headless" ? I18n.tr("HEADLESS") : I18n.tr("WINDOW"))
                        onChose: (mode) => {
                            pane.launchMode = mode === I18n.tr("SPICE") ? "spice" : (mode === I18n.tr("HEADLESS") ? "headless" : "window");
                            Vm.setConfig(pane.name, "display", ({ "window": "gtk", "spice": "spice", "headless": "none" })[pane.launchMode]);
                        }
                    }
                    Row {
                        spacing: Tokens.s2
                        visible: !pane.running
                        Sw { on: pane.disposableRun; onToggled: (value) => pane.disposableRun = value }
                        Text { anchors.verticalCenter: parent.verticalCenter; text: I18n.tr("Disposable launch"); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: 11 }
                    }
                    Row {
                        spacing: Tokens.s2
                        visible: pane.running
                        Btn { text: I18n.tr("CONSOLE"); primary: true; armed: pane.vm && (pane.vm.spice || "").length > 0; onAct: Vm.openConsole(pane.name) }
                        Btn { text: I18n.tr("SSH"); armed: pane.vm && (pane.vm.ssh || "").length > 0; onAct: Vm.openSsh(pane.name) }
                        Btn { text: Vm.monStats.status === "paused" ? I18n.tr("RESUME") : I18n.tr("PAUSE"); onAct: Vm.power(pane.name, Vm.monStats.status === "paused" ? "resume" : "pause") }
                        ConfirmAction { idleText: I18n.tr("RESET"); confirmText: I18n.tr("RESET NOW?"); onConfirmed: Vm.power(pane.name, "reset") }
                    }

                    Rectangle { width: parent.width; height: 1; color: Tokens.line }
                    Head { text: I18n.tr("SSH access") }
                    Row {
                        width: parent.width
                        spacing: Tokens.s2
                        Field {
                            id: sshUser
                            width: parent.width - saveSsh.width - Tokens.s2
                            text: pane.det ? (pane.det.sshUser || "") : ""
                            placeholder: I18n.tr("Guest user")
                        }
                        Btn { id: saveSsh; text: I18n.tr("SAVE USER"); armed: sshUser.text.trim().length > 0; onAct: Vm.setConfig(pane.name, "ryovm_ssh_user", sshUser.text.trim()) }
                    }
                    Row {
                        spacing: Tokens.s2
                        Btn { text: I18n.tr("OPEN SSH"); armed: pane.running && pane.vm && (pane.vm.ssh || "").length > 0; onAct: Vm.openSsh(pane.name) }
                        Btn { text: I18n.tr("COPY COMMAND"); armed: pane.running && pane.vm && (pane.vm.ssh || "").length > 0; onAct: Vm.copySsh(pane.name) }
                    }

                    Column {
                        width: parent.width
                        spacing: Tokens.s2
                        visible: pane.running && Vm.monStats.balloonMB > 0
                        Head { text: I18n.tr("Live memory") }
                        Text { text: Vm.human(pane.balloonMB * 1024 * 1024); color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: 11 }
                        Slid {
                            width: parent.width
                            from: 256
                            to: Math.max(512, pane.ramNum * 1024)
                            value: pane.balloonMB
                            onModified: (value) => {
                                pane.balloonDragging = true;
                                pane.balloonMB = value;
                                balloonCommit.restart();
                            }
                        }
                        Row {
                            spacing: Tokens.s2
                            Sw { on: Vm.monStats.pinned === true; onToggled: (value) => Vm.pin(pane.name, value ? "auto" : "off") }
                            Text { anchors.verticalCenter: parent.verticalCenter; text: I18n.tr("Pin virtual CPUs to host cores"); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: 11 }
                        }
                    }
                }

                Column {
                    width: parent.width
                    spacing: Tokens.s4
                    visible: pane.tab === "networking"

                    Head { text: I18n.tr("Endpoints") }
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
                            LabelValue { width: (parent.width - Tokens.s5 * 2) / 3; label: I18n.tr("Guest IP"); value: Vm.monStats.guestIp || "-" }
                            LabelValue { width: (parent.width - Tokens.s5 * 2) / 3; label: I18n.tr("SSH"); value: pane.vm && pane.vm.ssh ? "127.0.0.1:" + pane.vm.ssh : "-" }
                            LabelValue { width: (parent.width - Tokens.s5 * 2) / 3; label: I18n.tr("SPICE"); value: pane.vm && pane.vm.spice ? pane.vm.spice : "-" }
                        }
                    }

                    Head { text: I18n.tr("Port forwards") }
                    Text { visible: Vm.portfwds.length === 0; text: I18n.tr("No port forwards yet."); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 11 }
                    Repeater {
                        model: Vm.portfwds
                        Rectangle {
                            id: forwardRow
                            required property var modelData
                            width: parent ? parent.width : 0
                            height: 42
                            color: "transparent"
                            border.width: Tokens.border
                            border.color: Tokens.lineSoft
                            Text { anchors.left: parent.left; anchors.leftMargin: Tokens.s3; anchors.verticalCenter: parent.verticalCenter; text: "127.0.0.1:" + forwardRow.modelData.host + "  →  :" + forwardRow.modelData.guest; color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: 11 }
                            Btn { anchors.right: parent.right; anchors.rightMargin: Tokens.s2; anchors.verticalCenter: parent.verticalCenter; compact: true; visible: !pane.running; text: I18n.tr("REMOVE"); onAct: Vm.removePortfwd(pane.name, forwardRow.modelData.host + ":" + forwardRow.modelData.guest) }
                        }
                    }
                    Row {
                        width: parent.width
                        spacing: Tokens.s2
                        visible: !pane.running
                        Field { id: hostPort; width: (parent.width - addForward.width - Tokens.s2 * 2) / 2; placeholder: I18n.tr("Host port"); tabular: true }
                        Field { id: guestPort; width: hostPort.width; placeholder: I18n.tr("Guest port"); tabular: true }
                        Btn {
                            id: addForward
                            text: I18n.tr("ADD")
                            primary: true
                            armed: hostPort.text.trim().length > 0 && guestPort.text.trim().length > 0
                            onAct: {
                                Vm.addPortfwd(pane.name, hostPort.text.trim() + ":" + guestPort.text.trim());
                                hostPort.clear(); guestPort.clear();
                            }
                        }
                    }
                    Text { visible: pane.running; text: I18n.tr("Stop the machine to change port forwards."); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 11 }
                }

                Column {
                    width: parent.width
                    spacing: Tokens.s4
                    visible: pane.tab === "snapshots"

                    Head { text: I18n.tr("Snapshots") }
                    Text { visible: pane.running; text: I18n.tr("Stop the machine to manage snapshots."); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 11 }
                    Row {
                        width: parent.width
                        spacing: Tokens.s2
                        visible: pane.det && pane.det.installed && !pane.running
                        Field { id: snapshotName; width: parent.width - saveSnapshot.width - Tokens.s2; placeholder: I18n.tr("Snapshot name") }
                        Btn { id: saveSnapshot; text: I18n.tr("SAVE"); primary: true; armed: snapshotName.text.trim().length > 0; onAct: { Vm.snapshot(pane.name, "create", snapshotName.text.trim()); snapshotName.clear(); } }
                    }
                    Text { visible: pane.det && (!pane.det.snapshots || pane.det.snapshots.length === 0); text: I18n.tr("No snapshots yet."); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 11 }
                    Repeater {
                        model: pane.det ? pane.det.snapshots : []
                        Rectangle {
                            id: snapshotRow
                            required property var modelData
                            width: parent ? parent.width : 0
                            height: 50
                            color: "transparent"
                            border.width: Tokens.border
                            border.color: Tokens.lineSoft
                            Column {
                                anchors.left: parent.left
                                anchors.leftMargin: Tokens.s3
                                anchors.verticalCenter: parent.verticalCenter
                                spacing: 2
                                Text { text: snapshotRow.modelData.name; color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: 12; font.weight: Font.Medium }
                                Text { text: snapshotRow.modelData.date || ""; color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: 9 }
                            }
                            Row {
                                anchors.right: parent.right
                                anchors.rightMargin: Tokens.s2
                                anchors.verticalCenter: parent.verticalCenter
                                spacing: Tokens.s2
                                Btn { compact: true; text: I18n.tr("RESTORE"); armed: !pane.running; onAct: Vm.snapshot(pane.name, "restore", snapshotRow.modelData.name) }
                                ConfirmAction { compact: true; idleText: I18n.tr("DELETE"); confirmText: I18n.tr("DELETE?"); ready: !pane.running; onConfirmed: Vm.snapshot(pane.name, "delete", snapshotRow.modelData.name) }
                            }
                        }
                    }

                    Rectangle { width: parent.width; height: 1; color: Tokens.line }
                    Head { text: I18n.tr("Golden state") }
                    Text {
                        width: parent.width
                        wrapMode: Text.WordWrap
                        text: pane.sealSnapshot ? I18n.tr("This machine has a sealed golden state for disposable sessions.") : I18n.tr("Seal the configured machine as a clean state for disposable sessions and rollback.")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: 11
                    }
                    Row {
                        spacing: Tokens.s2
                        Btn { text: pane.sealSnapshot ? I18n.tr("RE-SEAL") : I18n.tr("SEAL MACHINE"); primary: !pane.sealSnapshot; armed: !pane.running && !Vm.busy; onAct: Vm.seal(pane.name) }
                        Btn { visible: pane.sealSnapshot !== null; text: I18n.tr("RESTORE SEAL"); armed: !pane.running && !Vm.busy; onAct: Vm.restoreSeal(pane.name) }
                        Btn { text: I18n.tr("SAVE AS TEMPLATE"); armed: !pane.running && !Vm.busy; onAct: Vm.template(pane.name) }
                    }
                }

                Column {
                    width: parent.width
                    spacing: Tokens.s4
                    visible: pane.tab === "settings"

                    Head { text: I18n.tr("Identity") }
                    Row {
                        width: parent.width
                        spacing: Tokens.s2
                        visible: !pane.running
                        Field {
                            id: renameField
                            width: parent.width - renameButton.width - Tokens.s2
                            onEdited: (value) => {
                                var clean = value.replace(/[\/\s]+/g, "-");
                                if (clean !== value) text = clean;
                            }
                        }
                        Btn { id: renameButton; text: I18n.tr("RENAME"); primary: true; armed: renameField.text.trim().length > 0 && renameField.text.trim() !== pane.name; onAct: Vm.renameVm(pane.name, renameField.text.trim()) }
                    }
                    Text { visible: pane.running; text: I18n.tr("Stop the machine to change hardware or identity."); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 11 }

                    Head { text: I18n.tr("Hardware") }
                    Row {
                        width: parent.width
                        spacing: Tokens.s5
                        opacity: pane.running ? 0.4 : 1
                        Column {
                            width: (parent.width - Tokens.s5) / 2
                            spacing: Tokens.s2
                            Text { text: I18n.tr("CPU cores"); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 10 }
                            Step { value: pane.coresNum; from: 1; to: 32; onModified: (value) => { if (!pane.running) Vm.setConfig(pane.name, "cpu_cores", Math.round(value)); } }
                            Btn { compact: true; text: I18n.tr("AUTO"); armed: !pane.running; onAct: Vm.setConfig(pane.name, "cpu_cores", "auto") }
                        }
                        Column {
                            width: (parent.width - Tokens.s5) / 2
                            spacing: Tokens.s2
                            Text { text: I18n.tr("Memory (GB)"); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 10 }
                            Step { value: pane.ramNum; from: 1; to: 128; onModified: (value) => { if (!pane.running) Vm.setConfig(pane.name, "ram", Math.round(value) + "G"); } }
                            Btn { compact: true; text: I18n.tr("AUTO"); armed: !pane.running; onAct: Vm.setConfig(pane.name, "ram", "auto") }
                        }
                    }
                    Row {
                        width: parent.width
                        spacing: Tokens.s2
                        opacity: pane.running ? 0.4 : 1
                        Step { value: pane.diskTarget; from: 8; to: 2048; stepBy: 8; onModified: (value) => { if (!pane.running) pane.diskTarget = Math.round(value); } }
                        Btn { text: I18n.tr("GROW DISK TO %1 GB").arg(pane.diskTarget); armed: !pane.running && pane.diskTarget > pane.diskCap; onAct: Vm.resizeDisk(pane.name, pane.diskTarget + "G") }
                    }

                    Head { text: I18n.tr("USB devices") }
                    Text { visible: Vm.usb.length === 0; text: I18n.tr("No USB devices detected."); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 11 }
                    Repeater {
                        model: Vm.usb
                        Rectangle {
                            id: usbRow
                            required property var modelData
                            width: parent ? parent.width : 0
                            height: 42
                            color: usbRow.modelData.assigned ? Tokens.tint5 : "transparent"
                            border.width: Tokens.border
                            border.color: Tokens.lineSoft
                            Sw { anchors.left: parent.left; anchors.leftMargin: Tokens.s3; anchors.verticalCenter: parent.verticalCenter; on: usbRow.modelData.assigned === true; onToggled: (value) => { if (!pane.running) Vm.setUsb(pane.name, usbRow.modelData.id, value); } }
                            Text { anchors.left: parent.left; anchors.leftMargin: 62; anchors.right: usbId.left; anchors.verticalCenter: parent.verticalCenter; elide: Text.ElideRight; text: usbRow.modelData.name; color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: 11 }
                            Text { id: usbId; anchors.right: parent.right; anchors.rightMargin: Tokens.s3; anchors.verticalCenter: parent.verticalCenter; text: usbRow.modelData.id; color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: 9 }
                        }
                    }

                    Rectangle { width: parent.width; height: 1; color: Tokens.line }
                    Head { text: I18n.tr("Machine storage") }
                    Row {
                        spacing: Tokens.s2
                        Btn { text: I18n.tr("OPEN FOLDER"); onAct: Vm.openFolder(pane.name) }
                        ConfirmAction { idleText: I18n.tr("RECLAIM DISK"); confirmText: I18n.tr("WIPE DISK?"); ready: !pane.running && !Vm.busy; onConfirmed: Vm.reclaimDisk(pane.name) }
                        ConfirmAction { idleText: I18n.tr("DELETE MACHINE"); confirmText: I18n.tr("DESTROY?"); ready: !pane.running && !Vm.busy; onConfirmed: Vm.deleteVm(pane.name) }
                    }
                }

                Column {
                    width: parent.width
                    spacing: Tokens.s2
                    visible: pane.tab === "activity"
                    Head { text: I18n.tr("Activity") }
                    Text {
                        width: parent.width
                        wrapMode: Text.WordWrap
                        text: I18n.tr("Starts, stops, snapshots, configuration changes, and other machine actions recorded in this session.")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                    }
                    Text { visible: pane.events.length === 0; text: I18n.tr("Nothing has been recorded for this machine yet."); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 11 }
                    Repeater {
                        model: pane.events
                        Rectangle {
                            id: eventRow
                            required property var modelData
                            width: parent ? parent.width : 0
                            height: 40
                            color: "transparent"
                            Rectangle { anchors.bottom: parent.bottom; width: parent.width; height: 1; color: Tokens.lineSoft }
                            Text { anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter; width: 62; text: eventRow.modelData.time; color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: 9 }
                            Text { anchors.left: parent.left; anchors.leftMargin: 70; anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter; elide: Text.ElideRight; text: eventRow.modelData.text; color: eventRow.modelData.kind === "fault" ? Tokens.ink : Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 11 }
                        }
                    }
                }
            }
        }
    }

    component Head: Text {
        width: parent ? parent.width : 0
        color: Tokens.ink
        font.family: Tokens.ui
        font.pixelSize: 11
        font.weight: Font.DemiBold
    }
}
