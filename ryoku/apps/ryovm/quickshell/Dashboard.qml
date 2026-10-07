pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"

// The harbour: ryoport's first plate. A command dossier for the whole fleet --
// the machines berthed here and the remotes reached from here -- read at a glance
// and acted on in one tap. The Profile plate's register (a dithered decor hero,
// monumental Fraunces, an instrument readout, the watermark and marginalia) put
// to work: eye candy that also happens to be the console.
Item {
    id: dash

    property bool active: false
    signal navigate(string key)
    signal newMachine()
    signal newRemote()
    signal openMachine(string name)
    signal openRemote(string alias)

    readonly property int runningCount: {
        var n = 0;
        for (var i = 0; i < Vm.vms.length; i++)
            if (Vm.vms[i].running === true) n++;
        return n;
    }

    // the harbour reads at a glance: cap the fleet plates to a preview, active and
    // ailing first, and hand the rest to the full page. A yard of fifty never
    // buries the dashboard.
    readonly property int previewCap: 6
    readonly property var machPreview: {
        var v = Vm.vms.slice();
        v.sort(function (a, b) {
            var d = (b.running === true ? 1 : 0) - (a.running === true ? 1 : 0);
            return d !== 0 ? d : (a.name < b.name ? -1 : a.name > b.name ? 1 : 0);
        });
        return v.slice(0, dash.previewCap);
    }
    readonly property var remPreview: {
        var rank = { "down": 0, "warn": 1, "unknown": 2, "up": 3 };
        var h = Remotes.hosts.slice();
        h.sort(function (a, b) {
            var ra = rank[Remotes.stateOf(a.alias)]; if (ra === undefined) ra = 2;
            var rb = rank[Remotes.stateOf(b.alias)]; if (rb === undefined) rb = 2;
            return ra !== rb ? ra - rb : (a.alias < b.alias ? -1 : a.alias > b.alias ? 1 : 0);
        });
        return h.slice(0, dash.previewCap);
    }

    property string hostName: Quickshell.env("HOSTNAME") || ""
    property string userName: Quickshell.env("USER") || I18n.tr("operator")
    Process {
        running: true
        command: ["sh", "-c", "hostname 2>/dev/null || cat /etc/hostname 2>/dev/null"]
        stdout: StdioCollector { onStreamFinished: { var s = this.text.trim(); if (s.length) dash.hostName = s; } }
    }
    property var now: new Date()
    Timer { interval: 30000; running: dash.active; repeat: true; triggeredOnStart: true; onTriggered: dash.now = new Date() }

    // ── the 顔-equivalent watermark: the harbour seal behind the plate ──
    Watermark {
        anchors.fill: parent
        text: "港"
    }

    Flickable {
        id: flick
        anchors.fill: parent
        anchors.leftMargin: Tokens.s7
        anchors.rightMargin: Tokens.s6
        contentHeight: body.implicitHeight + Tokens.s7
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }

        Column {
            id: body
            width: flick.width - Tokens.s5
            spacing: Tokens.s5
            topPadding: Tokens.s5

            // ── head: eyebrow + live clock ──
            Item {
                width: parent.width
                height: 24
                Row {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s2
                    Rectangle { width: 16; height: 1; color: Tokens.ink; anchors.verticalCenter: parent.verticalCenter }
                    Text { text: "力"; color: Tokens.ink; font.family: Tokens.jp; font.pixelSize: 11; anchors.verticalCenter: parent.verticalCenter }
                    Text {
                        text: I18n.tr("HARBOUR · COMMAND")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui; font.pixelSize: 9
                        font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                        anchors.verticalCenter: parent.verticalCenter
                    }
                }
                Marginalia {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    kana: "みなと"
                    index: TimeFormat.format(dash.now)
                    glyph: "column"; glyph2: "torii"
                    chevrons: true
                }
            }

            // ── hero: monumental identity + a dithered decor plate ──
            Item {
                width: parent.width
                height: 210

                Column {
                    id: heroText
                    anchors.left: parent.left
                    anchors.top: parent.top
                    anchors.right: heroArt.left
                    anchors.rightMargin: Tokens.s6
                    spacing: Tokens.s2

                    Text {
                        width: parent.width
                        text: dash.hostName.length > 0 ? dash.hostName : I18n.tr("harbour")
                        color: Tokens.ink
                        font.family: Tokens.display
                        font.pixelSize: 56
                        elide: Text.ElideRight
                    }
                    Text {
                        text: "@" + dash.userName + "  ·  " + TimeFormat.format(dash.now)
                        color: Tokens.inkMuted
                        font.family: Tokens.mono
                        font.pixelSize: 12
                    }
                    Text {
                        width: parent.width
                        wrapMode: Text.WordWrap
                        text: I18n.tr("The fleet at a glance: machines berthed here, remotes reached from here.")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: 13
                        font.styleName: "Italic"
                    }

                    Item { width: 1; height: Tokens.s2 }

                    // the instrument readout: four fleet vitals in a row.
                    Row {
                        spacing: Tokens.s6
                        Repeater {
                            model: [
                                { k: I18n.tr("MACHINES"), v: String(Vm.vms.length).padStart(2, "0") },
                                { k: I18n.tr("RUNNING"), v: String(dash.runningCount).padStart(2, "0") },
                                { k: I18n.tr("REMOTES"), v: String(Remotes.hostCount).padStart(2, "0") },
                                { k: I18n.tr("REACHABLE"), v: String(Remotes.upCount).padStart(2, "0") }
                            ]
                            Column {
                                id: vital
                                required property var modelData
                                spacing: 2
                                Text {
                                    text: vital.modelData.k
                                    color: Tokens.inkDim
                                    font.family: Tokens.mono; font.pixelSize: 9; font.letterSpacing: 1.4
                                }
                                Text {
                                    text: vital.modelData.v
                                    color: Tokens.ink
                                    font.family: Tokens.ui; font.pixelSize: 30; font.weight: Font.Light
                                }
                            }
                        }
                    }
                }
            }

            // ── MACHINES ──
            Section {
                width: parent.width
                title: I18n.tr("MACHINES")

                Item {
                    width: parent.width
                    implicitHeight: machHead.height + machBody.implicitHeight + Tokens.s3

                    Row {
                        id: machHead
                        width: parent.width
                        height: 26
                        spacing: Tokens.s3
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: String(dash.runningCount).padStart(2, "0") + I18n.tr(" RUNNING · ") + String(Vm.vms.length).padStart(2, "0") + I18n.tr(" TOTAL")
                            color: Tokens.inkMuted
                            font.family: Tokens.mono; font.pixelSize: 10; font.letterSpacing: 1.2
                        }
                        Item { width: parent.width - 340; height: 1 }
                    }
                    Btn {
                        anchors.right: parent.right
                        anchors.top: parent.top
                        text: I18n.tr("NEW MACHINE")
                        onAct: dash.newMachine()
                    }

                    Flow {
                        id: machBody
                        width: parent.width
                        anchors.top: machHead.bottom
                        anchors.topMargin: Tokens.s3
                        spacing: Tokens.s2

                        Repeater {
                            model: dash.machPreview
                            FleetTile {
                                required property var modelData
                                width: (machBody.width - Tokens.s2 * 2) / 3
                                title: modelData.name
                                sub: (modelData.running ? I18n.tr("RUNNING") : I18n.tr("STOPPED"))
                                    + "  ·  " + (modelData.cores === "auto" || !modelData.cores ? I18n.tr("auto") : modelData.cores + "c")
                                    + " · " + (modelData.ram || I18n.tr("auto"))
                                on: modelData.running === true
                                slug: modelData.os || ""
                                onTapped: dash.openMachine(modelData.name)
                                primaryLabel: modelData.running ? I18n.tr("STOP") : I18n.tr("LAUNCH")
                                onPrimary: {
                                    if (modelData.running) Vm.stop(modelData.name);
                                    else if (Vm.caps.quickemu === true)
                                        Vm.launch(modelData.name, ({ "gtk": "window", "spice": "spice", "none": "headless" })[modelData.display] || "window");
                                }
                                secondaryLabel: modelData.running ? I18n.tr("CONSOLE") : ""
                                onSecondary: Vm.openConsole(modelData.name)
                            }
                        }
                        MoreTile {
                            visible: Vm.vms.length > dash.previewCap
                            width: (machBody.width - Tokens.s2 * 2) / 3
                            count: Vm.vms.length - dash.previewCap
                            onTapped: dash.navigate("machines")
                        }

                        Text {
                            visible: Vm.vms.length === 0 && !Vm.vmsLoading
                            width: machBody.width
                            text: I18n.tr("No machines yet. Build one from the catalogue.")
                            color: Tokens.inkFaint
                            font.family: Tokens.ui; font.pixelSize: 12
                        }
                    }
                }
            }

            // ── REMOTES ──
            Section {
                width: parent.width
                title: I18n.tr("REMOTES")

                Item {
                    width: parent.width
                    implicitHeight: remHead.height + remBody.implicitHeight + Tokens.s3

                    Row {
                        id: remHead
                        width: parent.width
                        height: 26
                        spacing: Tokens.s3
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: String(Remotes.upCount).padStart(2, "0") + I18n.tr(" UP · ") + String(Remotes.hostCount).padStart(2, "0") + I18n.tr(" HOSTS")
                            color: Tokens.inkMuted
                            font.family: Tokens.mono; font.pixelSize: 10; font.letterSpacing: 1.2
                        }
                        Item { width: parent.width - 340; height: 1 }
                    }
                    Btn {
                        anchors.right: parent.right
                        anchors.top: parent.top
                        text: I18n.tr("NEW CONNECTION")
                        onAct: dash.newRemote()
                    }

                    Flow {
                        id: remBody
                        width: parent.width
                        anchors.top: remHead.bottom
                        anchors.topMargin: Tokens.s3
                        spacing: Tokens.s2

                        Repeater {
                            model: dash.remPreview
                            RemoteTile {
                                required property var modelData
                                width: (remBody.width - Tokens.s2 * 2) / 3
                                host: modelData
                                onTapped: dash.openRemote(modelData.alias)
                                onConnect: Remotes.connect(modelData.alias)
                            }
                        }
                        MoreTile {
                            visible: Remotes.hosts.length > dash.previewCap
                            width: (remBody.width - Tokens.s2 * 2) / 3
                            count: Remotes.hosts.length - dash.previewCap
                            onTapped: dash.navigate("remotes")
                        }

                        Text {
                            visible: Remotes.hosts.length === 0 && !Remotes.loading
                            width: remBody.width
                            text: Remotes.engineOk
                                ? I18n.tr("No remotes yet. Add a VPS, or drop hosts in ~/.ssh/config.")
                                : I18n.tr("The remote engine (ryossh) is not installed.")
                            color: Tokens.inkFaint
                            font.family: Tokens.ui; font.pixelSize: 12
                        }
                    }
                }
            }

            // ── ACTIVITY ──
            Section {
                width: parent.width
                title: I18n.tr("ACTIVITY")

                Item {
                    id: actItem
                    width: parent.width
                    // the fleet's recent doings, machines and remotes merged newest-first.
                    readonly property var feed: {
                        var out = [];
                        var i, e;
                        for (i = 0; i < Vm.events.length; i++) {
                            e = Vm.events[i];
                            out.push({ at: e.at || 0, time: e.time, tag: e.vm || "", kind: e.kind, text: e.text });
                        }
                        for (i = 0; i < Remotes.events.length; i++) {
                            e = Remotes.events[i];
                            out.push({ at: e.at || 0, time: e.time, tag: e.alias || "", kind: e.kind, text: e.text });
                        }
                        out.sort(function (a, b) { return (b.at || 0) - (a.at || 0); });
                        return out.slice(0, 6);
                    }
                    implicitHeight: actItem.feed.length > 0 ? actCol.implicitHeight : 22

                    Text {
                        visible: actItem.feed.length === 0
                        text: I18n.tr("Nothing yet. Launch a machine or open a connection.")
                        color: Tokens.inkFaint
                        font.family: Tokens.ui; font.pixelSize: 12
                    }
                    Column {
                        id: actCol
                        width: parent.width
                        visible: actItem.feed.length > 0
                        spacing: Tokens.s1
                        Repeater {
                            model: actItem.feed
                            Item {
                                required property var modelData
                                width: actCol.width
                                height: 22
                                Text {
                                    anchors.left: parent.left
                                    anchors.verticalCenter: parent.verticalCenter
                                    width: 62
                                    text: modelData.time
                                    color: Tokens.inkFaint
                                    font.family: Tokens.mono; font.pixelSize: 10
                                }
                                Text {
                                    anchors.left: parent.left; anchors.leftMargin: 70
                                    anchors.right: actTag.left; anchors.rightMargin: Tokens.s3
                                    anchors.verticalCenter: parent.verticalCenter
                                    elide: Text.ElideRight
                                    text: modelData.text
                                    color: modelData.kind === "fault" ? Tokens.ink : Tokens.inkMuted
                                    font.family: Tokens.ui; font.pixelSize: 12
                                }
                                Text {
                                    id: actTag
                                    anchors.right: parent.right
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: modelData.tag
                                    color: Tokens.inkFaint
                                    font.family: Tokens.mono; font.pixelSize: 10
                                }
                            }
                        }
                    }
                }
            }

            // ── foot marginalia ──
            Item {
                width: parent.width
                height: 40
                Barcode {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("RYOPORT-HARBOUR-") + Qt.formatDateTime(dash.now, "yyyyMMdd")
                    unit: 1.0
                    barHeight: 16
                }
                Text {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("// ONE CONSOLE FOR EVERY MACHINE")
                    color: Tokens.inkFaint
                    font.family: Tokens.mono; font.pixelSize: 9; font.letterSpacing: 1.4
                }
            }
        }
    }
}
