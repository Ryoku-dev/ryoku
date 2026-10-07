pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons

// Rashin (DESIGN.md section 11, ADVANCED). The optional, fully local agent OS
// (羅針, the system needle). This page is the Hub's welcome to Rashin, in the
// Hub's own paper and ink, the same language the web dashboard at
// 127.0.0.1:3600 now speaks. It
// leads with the samurai hero banner and the live model Hermes runs on, lays
// out what Rashin does (the vault, memory, skills, agents, chat, code), shows
// how to use it, and keeps the master switch, one-click Hermes setup and the
// dashboard link. Live state comes from `ryoku-rashin status --json`; setup
// progress is mirrored from the daemon's setup.json. Everything stays on the
// machine; Rashin is off until you switch it on.
Item {
    id: pg

    property var hub
    readonly property bool fullBleed: true

    // centered body column; the hero and every section share this measure.
    readonly property real bodyW: Math.min(pg.width - Tokens.s5 * 2, 1080)

    // ── status snapshot (page readouts, not stored config) ───────────────────
    property bool installed: true
    property bool daemonEnabled: false
    property bool running: false
    // the port the daemon reports, not a literal: a user who changed it in
    // ~/.config/ryoku/rashin.json still gets a link that resolves.
    property int port: 3600
    property bool prowlInstalled: false
    property bool prowlRunning: false
    property int prowlPort: 8788
    property string prowlUrl: ""
    property string prowlError: ""
    property bool vaultExists: false
    property int vaultFiles: 0
    property bool hermesInstalled: false
    property bool hermesConfigured: false
    property int agentsPresent: 0
    property int agentsWired: 0
    // manifest: the full agent surface (paths + agents + chat backends), from
    // `ryoku-rashin paths --json`. Drives the Agents and Connect sections.
    property var agentList: []
    property var chatBackends: []
    property string skillPath: ""
    property string prowlPath: ""
    property var vaultItems: []

    readonly property string wiredSummary: pg.agentsPresent > 0
        ? I18n.tr("%1 / %2 wired").arg(pg.agentsWired).arg(pg.agentsPresent)
        : I18n.tr("none yet")

    // live Hermes setup progress, mirrored from the daemon's setup.json.
    property string setupPhase: ""
    property string setupDetail: ""
    property bool setupOk: true

    Component.onCompleted: pg.refresh()
    function refresh() { statusProc.running = true; manifestProc.running = true; }

    Process {
        id: statusProc
        // sh-wrapped so a missing ryoku-rashin still closes stdout and fires
        // onStreamFinished; empty/unparseable output reads as "not installed".
        command: ["sh", "-c", "ryoku-rashin status --json"]
        stderr: StdioCollector {}
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var o = JSON.parse(this.text);
                    pg.installed = true;
                    pg.daemonEnabled = o.enabled === true;
                    pg.running = o.running === true;
                    if (typeof o.port === "number" && o.port > 0)
                        pg.port = o.port;
                    var p = o.prowl || ({});
                    pg.prowlInstalled = p.installed === true;
                    pg.prowlRunning = p.running === true;
                    if (typeof p.port === "number" && p.port > 0)
                        pg.prowlPort = p.port;
                    pg.prowlUrl = p.url || "";
                    pg.prowlError = p.error || "";
                    var v = o.vault || ({});
                    pg.vaultExists = v.exists === true;
                    pg.vaultFiles = (typeof v.files === "number") ? v.files : 0;
                    var h = o.hermes || ({});
                    pg.hermesInstalled = h.installed === true;
                    pg.hermesConfigured = h.configured === true;
                    var ags = o.agents || [];
                    var present = 0, wired = 0;
                    for (var i = 0; i < ags.length; i++) {
                        if (ags[i].present) {
                            present++;
                            if (ags[i].wired)
                                wired++;
                        }
                    }
                    pg.agentsPresent = present;
                    pg.agentsWired = wired;
                    return;
                } catch (e) {
                    // empty output (binary missing) or malformed JSON.
                }
                pg.installed = false;
                pg.daemonEnabled = false;
                pg.running = false;
                pg.prowlInstalled = false;
                pg.prowlRunning = false;
                pg.prowlUrl = "";
                pg.prowlError = "";
            }
        }
    }

    Process { id: enableProc; onExited: pg.refresh() }
    Process { id: disableProc; onExited: pg.refresh() }
    function setEnabled(on) {
        var p = on ? enableProc : disableProc;
        p.command = ["ryoku-rashin", on ? "enable" : "disable"];
        p.running = true;
    }

    FileView {
        id: setupFile
        path: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-rashin/setup.json"
        watchChanges: true
        printErrors: false
        onLoaded: pg.applySetup(setupFile.text())
        onFileChanged: setupFile.reload()
        onLoadFailed: {}
    }
    function applySetup(t) {
        try {
            var o = JSON.parse(t);
            pg.setupPhase = o.phase || "";
            pg.setupDetail = o.detail || "";
            pg.setupOk = o.ok !== false;
            if (pg.setupPhase === "done")
                pg.refresh();
        } catch (e) {}
    }
    function runSetup() {
        Spawn.run(["kitty", "--class", "ryoku-rashin-setup", "-e", "ryoku-rashin", "setup"]);
    }
    // Open the dashboard the daemon actually serves. Two things used to send a
    // user to a browser error they read as a 404: the port was hardcoded here
    // while the daemon reads it from its own config, and the button opened the
    // URL even with nothing listening. Take the port from `status --json` and
    // start the daemon first when it is down; `ryoku-rashin serve` returns once
    // the socket is up, so the browser never races it.
    function openDashboard() {
        var url = "http://127.0.0.1:" + pg.port;
        if (pg.running) {
            Spawn.run(["xdg-open", url]);
            return;
        }
        openProc.command = ["sh", "-c",
            "ryoku-rashin serve --if-enabled >/dev/null 2>&1 & "
            + "for i in 1 2 3 4 5 6 7 8 9 10; do "
            + "ryoku-rashin status --json 2>/dev/null | grep -q '\"running\":true' && break; sleep 0.3; done; "
            + "xdg-open " + url];
        openProc.running = true;
    }
    Process { id: openProc; onExited: pg.refresh() }

    // the agent surface, refreshed with status. Best effort: a missing binary
    // leaves the sections empty rather than erroring.
    Process {
        id: manifestProc
        command: ["sh", "-c", "ryoku-rashin paths --json"]
        stderr: StdioCollector {}
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var m = JSON.parse(this.text);
                    pg.skillPath = (m.skill && m.skill.path) || "";
                    pg.prowlPath = (m.prowl && m.prowl.path) || "";
                    pg.vaultItems = m.vault || [];
                    pg.agentList = m.agents || [];
                    pg.chatBackends = m.chatBackends || [];
                } catch (e) {}
            }
        }
    }
    Process { id: wireProc; onExited: pg.refresh() }
    Process { id: chatProc; onExited: pg.refresh() }
    // Wiring drops the pointer + the ryoku skill + prowl's skills into an
    // agent (or every present one); useChatAgent picks who drives the chat.
    function wireAgent(id) { wireProc.command = ["ryoku-rashin", "wire", id]; wireProc.running = true; }
    function wireAll() { wireProc.command = ["ryoku-rashin", "wire"]; wireProc.running = true; }
    function useChatAgent(id) { chatProc.command = ["ryoku-rashin", "agent", "use", id]; chatProc.running = true; }
    // Copy the paste snippet for an agent Rashin does not wire directly.
    function copySnippet() { Quickshell.execDetached(["sh", "-c", "ryoku-rashin paths --snippet | wl-copy"]); }
    // whether an agent can drive the Super+S chat (its ACP adapter is present).
    function agentCanChat(id) {
        for (var i = 0; i < pg.chatBackends.length; i++)
            if (pg.chatBackends[i].id === id)
                return pg.chatBackends[i].available === true;
        return false;
    }

    // ── reusable poster parts ────────────────────────────────────────────────

    // a stamped-ink status chip: hairline box, tracked mono caps, tinted.
    component Stamp: Rectangle {
        id: stamp
        property string label: ""
        property color tint: Tokens.inkDim
        implicitWidth: stampT.implicitWidth + 18
        implicitHeight: 22
        color: "transparent"
        border.width: 1
        border.color: Qt.rgba(stamp.tint.r, stamp.tint.g, stamp.tint.b, 0.55)
        Text {
            id: stampT
            anchors.centerIn: parent
            text: I18n.tr(stamp.label)
            color: stamp.tint
            font.family: Tokens.mono
            font.pixelSize: 9
            font.letterSpacing: 2
        }
    }

    // a section header: kanji + tracked caps title + hairline leader.
    component Head: Item {
        id: hd
        property string kanji: ""
        property string title: ""
        width: parent ? parent.width : 0
        height: 20
        Row {
            id: hdRow
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s2
            Text {
                text: hd.kanji; color: Tokens.alert; font.family: Tokens.jp
                font.pixelSize: 13; anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: hd.title; color: Tokens.ink; font.family: Tokens.mono
                font.pixelSize: 11; font.letterSpacing: 3; font.weight: Font.Medium
                anchors.verticalCenter: parent.verticalCenter
            }
        }
        Rectangle {
            anchors.left: hdRow.right; anchors.leftMargin: Tokens.s3
            anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
            height: 1; color: Tokens.line
        }
    }

    // a function card: an ink tab, index + kanji, name, one-liner, and an
    // optional live stat.
    component FnCard: Rectangle {
        id: fc
        property string index: ""
        property string kanji: ""
        property string name: ""
        property string desc: ""
        property string stat: ""
        height: 128
        color: Tokens.paperLift
        border.width: 1
        border.color: Tokens.line

        Rectangle { x: 0; y: 0; width: 34; height: 3; color: Tokens.ink }

        Column {
            anchors { left: parent.left; right: parent.right; top: parent.top; margins: 16 }
            spacing: 7
            Row {
                spacing: Tokens.s2
                Text {
                    text: fc.index; color: Tokens.ink; font.family: Tokens.display
                    font.pixelSize: 12; anchors.verticalCenter: parent.verticalCenter
                }
                Text {
                    text: fc.kanji; color: Tokens.inkDim; font.family: Tokens.jp
                    font.pixelSize: 12; anchors.verticalCenter: parent.verticalCenter
                }
            }
            Text {
                text: fc.name; color: Tokens.ink; font.family: Tokens.display; font.pixelSize: 21
            }
            Text {
                width: parent.width
                text: I18n.tr(fc.desc); color: Tokens.inkDim; font.family: Tokens.mono; font.pixelSize: 12
                wrapMode: Text.WordWrap; lineHeight: 1.25
            }
        }
        Text {
            visible: fc.stat !== ""
            anchors { right: parent.right; top: parent.top; margins: 16 }
            text: fc.stat; color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: 11
        }
    }

    // an example row: a mono command in red, then what it does.
    component TryRow: Row {
        id: tr
        property string cmd: ""
        property string note: ""
        width: parent ? parent.width : 0
        spacing: Tokens.s3
        Text {
            width: Math.round(tr.width * 0.34)
            text: tr.cmd; color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: 13
            elide: Text.ElideRight
        }
        Text {
            width: tr.width - Math.round(tr.width * 0.34) - tr.spacing
            text: tr.note; color: Tokens.inkDim; font.family: Tokens.mono; font.pixelSize: 13
            wrapMode: Text.WordWrap
        }
    }

    // a poster button: filled ink primary, hairline ghost otherwise.
    component PosterBtn: Item {
        id: bt
        property string label: ""
        property bool primary: false
        property bool on: true
        signal act()
        implicitWidth: btT.implicitWidth + Tokens.s5 * 2
        implicitHeight: 42
        opacity: bt.on ? 1 : 0.4
        Rectangle {
            anchors.fill: parent
            color: bt.primary ? Tokens.ink : "transparent"
            border.width: 1
            border.color: bt.primary ? Tokens.ink : Tokens.line
        }
        Text {
            id: btT
            anchors.centerIn: parent
            text: I18n.tr(bt.label); color: bt.primary ? Tokens.paper : Tokens.ink
            font.family: Tokens.mono; font.pixelSize: 11; font.letterSpacing: 2; font.weight: Font.Medium
        }
        MouseArea {
            anchors.fill: parent; enabled: bt.on
            cursorShape: Qt.PointingHandCursor
            onClicked: bt.act()
        }
    }

    // one detected coding agent: name, wiring state, skill/chat stamps, and a
    // one-click WIRE (inject the pointer + skill + prowl).
    component AgentRow: Rectangle {
        id: ar
        property var agent: ({})
        property bool canChat: false
        width: parent ? parent.width : 0
        height: 56
        color: Tokens.paperLift
        border.width: 1
        border.color: (ar.agent.present && ar.agent.wired) ? Qt.rgba(Tokens.ink.r, Tokens.ink.g, Tokens.ink.b, 0.3) : Tokens.line
        opacity: ar.agent.present ? 1 : 0.5

        Column {
            anchors { left: parent.left; leftMargin: 16; right: arActions.left; rightMargin: 12; verticalCenter: parent.verticalCenter }
            spacing: 3
            Text {
                width: parent.width
                text: ar.agent.name || ar.agent.id || ""
                color: Tokens.ink; font.family: Tokens.display; font.pixelSize: 16; elide: Text.ElideRight
            }
            Text {
                width: parent.width
                text: !ar.agent.present ? I18n.tr("not installed")
                    : (ar.agent.wired ? I18n.tr("wired to the vault") : I18n.tr("present \u00b7 not wired yet"))
                color: Tokens.inkDim; font.family: Tokens.mono; font.pixelSize: 11; elide: Text.ElideRight
            }
        }
        Row {
            id: arActions
            anchors { right: parent.right; rightMargin: 12; verticalCenter: parent.verticalCenter }
            spacing: Tokens.s2
            Stamp { visible: ar.agent.skillWired === true; label: "SKILL"; tint: Tokens.ink; anchors.verticalCenter: parent.verticalCenter }
            Stamp { visible: ar.canChat; label: "CHAT"; tint: Tokens.inkDim; anchors.verticalCenter: parent.verticalCenter }
            Rectangle {
                visible: ar.agent.present === true
                anchors.verticalCenter: parent.verticalCenter
                width: wbT.implicitWidth + Tokens.s4 * 2
                height: 30
                color: ar.agent.wired ? "transparent" : Tokens.ink
                border.width: 1
                border.color: ar.agent.wired ? Tokens.line : Tokens.ink
                Text {
                    id: wbT
                    anchors.centerIn: parent
                    text: ar.agent.wired ? I18n.tr("RE-WIRE") : I18n.tr("WIRE")
                    color: ar.agent.wired ? Tokens.ink : Tokens.paper
                    font.family: Tokens.mono; font.pixelSize: 10; font.letterSpacing: 2; font.weight: Font.Medium
                }
                MouseArea { anchors.fill: parent; cursorShape: Qt.PointingHandCursor; onClicked: pg.wireAgent(ar.agent.id) }
            }
        }
    }

    // one manifest path: label + who owns it (generated/yours/read-only) + the
    // resolved location (dim when absent).
    component PathRow: Row {
        id: pr
        property string label: ""
        property string path: ""
        property string owner: ""
        property bool ok: true
        readonly property int labelW: Math.round(pr.width * 0.22)
        readonly property int ownerW: 76
        width: parent ? parent.width : 0
        spacing: Tokens.s3
        Text {
            width: pr.labelW
            text: pr.label; color: pr.ok ? Tokens.ink : Tokens.inkDim
            font.family: Tokens.mono; font.pixelSize: 12; elide: Text.ElideRight
        }
        Text {
            width: pr.ownerW
            text: pr.owner
            color: pr.owner === "yours" ? Tokens.ink : Tokens.inkDim
            font.family: Tokens.mono; font.pixelSize: 10; font.letterSpacing: 1
            anchors.verticalCenter: parent.verticalCenter
        }
        Text {
            width: pr.width - pr.labelW - pr.ownerW - pr.spacing * 2
            text: pr.path === "" ? I18n.tr("not installed") : pr.path
            color: pr.ok ? Tokens.ink : Tokens.inkDim
            font.family: Tokens.mono; font.pixelSize: 12; elide: Text.ElideLeft
        }
    }

    // ── the page: paper and ink, one scrolling column ────────────────────
    Rectangle { anchors.fill: parent; color: Tokens.paper }

    Flickable {
        id: flick
        anchors.fill: parent
        contentWidth: width
        contentHeight: body.height + Tokens.s6 * 2
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        Column {
            id: body
            anchors.horizontalCenter: parent.horizontalCenter
            y: Tokens.s6
            width: pg.bodyW
            spacing: Tokens.s6

            // ── HERO: the samurai banner, title + live state overlaid ────────
            Item {
                width: parent.width
                height: Math.round(Math.min(width / 2.357, 320))
                clip: true

                Image {
                    id: heroImg
                    anchors.fill: parent
                    source: Ryodecors.dir + "rashin-hero.png"
                    fillMode: Image.PreserveAspectCrop
                    verticalAlignment: Image.AlignVCenter
                    asynchronous: true
                    cache: true
                }
                // procedural fallback if the banner is missing.
                Rectangle {
                    anchors.fill: parent
                    visible: heroImg.status !== Image.Ready
                    color: Tokens.paperLift
                }
                Rectangle { anchors.fill: parent; color: "transparent"; border.width: 1; border.color: Tokens.line }

                // bottom scrim so the cap stays legible over the art.
                Rectangle {
                    anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
                    height: parent.height * 0.7
                    gradient: Gradient {
                        GradientStop { position: 0.0; color: "transparent" }
                        GradientStop { position: 1.0; color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.92) }
                    }
                }

                // title cap, bottom-left.
                Column {
                    anchors { left: parent.left; bottom: parent.bottom; leftMargin: Tokens.s5; bottomMargin: Tokens.s4 }
                    spacing: 3
                    Text {
                        text: I18n.tr("RYOKU RASHIN"); color: Tokens.ink; font.family: Tokens.display
                        font.pixelSize: Math.round(Math.min(pg.bodyW * 0.05, 40))
                    }
                    Row {
                        spacing: Tokens.s2
                        Text {
                            text: "羅針"; color: Tokens.alert; font.family: Tokens.jp; font.pixelSize: 13
                            anchors.verticalCenter: parent.verticalCenter
                        }
                        Text {
                            text: I18n.tr("THE SYSTEM NEEDLE"); color: Tokens.inkDim; font.family: Tokens.mono
                            font.pixelSize: 10; font.letterSpacing: 3
                            anchors.verticalCenter: parent.verticalCenter
                        }
                    }
                }

                // live state stamp, top-right.
                Stamp {
                    anchors { right: parent.right; top: parent.top; rightMargin: Tokens.s4; topMargin: Tokens.s4 }
                    label: !pg.installed ? I18n.tr("NOT INSTALLED")
                        : (pg.running ? I18n.tr("RUNNING") : (pg.daemonEnabled ? I18n.tr("STARTING") : I18n.tr("OFF")))
                    tint: pg.running ? Tokens.ink : (pg.installed ? Tokens.inkDim : Tokens.alert)
                }
            }

            // ── tagline ──────────────────────────────────────────────────────
            Text {
                width: parent.width
                text: I18n.tr("The optional local agent OS. A resident Hermes agent keeps a living map of this machine - hardware, packages, every config beside the binary that owns it - so your coding agents read the terrain instead of rediscovering it. Nothing ever leaves the box.")
                color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: 14
                wrapMode: Text.WordWrap; lineHeight: 1.5
            }

            // ── MASTER SWITCH + PROWL: the two-process product state ─────────
            Row {
                width: parent.width
                spacing: Tokens.s4
                readonly property bool wide: pg.bodyW >= 640
                property real colW: wide ? (width - Tokens.s4) / 2 : width

                // service switch
                Rectangle {
                    width: parent.colW
                    height: 96
                    color: Tokens.paperLift
                    border.width: 1
                    border.color: pg.daemonEnabled ? Qt.rgba(Tokens.ink.r, Tokens.ink.g, Tokens.ink.b, 0.35) : Tokens.line
                    opacity: pg.installed ? 1 : 0.5

                    Column {
                        anchors { left: parent.left; right: sw.left; verticalCenter: parent.verticalCenter; leftMargin: 18; rightMargin: 14 }
                        spacing: 6
                        Row {
                            spacing: Tokens.s2
                            Text { text: "羅針"; color: Tokens.ink; font.family: Tokens.jp; font.pixelSize: 15; anchors.verticalCenter: parent.verticalCenter }
                            Text {
                                text: I18n.tr("RASHIN SERVICE"); color: Tokens.ink; font.family: Tokens.display
                                font.pixelSize: 13; anchors.verticalCenter: parent.verticalCenter
                            }
                        }
                        Text {
                            width: parent.width
                            text: pg.installed
                                ? (pg.running ? I18n.tr("Running \u00b7 127.0.0.1:%1").arg(pg.port)
                                   : (pg.daemonEnabled ? I18n.tr("Enabled \u00b7 starting\u2026") : I18n.tr("Off \u00b7 switch on to start it with the desktop")))
                                : I18n.tr("Not installed \u00b7 install ryoku-rashin")
                            color: Tokens.inkDim; font.family: Tokens.mono; font.pixelSize: 11; elide: Text.ElideRight
                        }
                    }
                    Sw {
                        id: sw
                        anchors { right: parent.right; rightMargin: 16; verticalCenter: parent.verticalCenter }
                        enabled: pg.installed
                        on: pg.daemonEnabled
                        onToggled: (c) => { pg.daemonEnabled = c; pg.setEnabled(c); }
                    }
                }

                // Prowl is Rashin's model gateway and code-intelligence engine.
                Rectangle {
                    width: parent.colW
                    height: 96
                    color: Tokens.paperLift
                    border.width: 1
                    border.color: pg.prowlRunning ? Qt.rgba(Tokens.ink.r, Tokens.ink.g, Tokens.ink.b, 0.35) : Tokens.line
                    Rectangle { x: 0; y: 0; width: 34; height: 3; color: pg.prowlRunning ? Tokens.ink : Tokens.alert }

                    Column {
                        anchors { left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter; leftMargin: 18; rightMargin: 18 }
                        spacing: 6
                        Row {
                            spacing: Tokens.s2
                            Text { text: "徘徊"; color: Tokens.alert; font.family: Tokens.jp; font.pixelSize: 12; anchors.verticalCenter: parent.verticalCenter }
                            Text {
                                text: I18n.tr("PROWL GATEWAY"); color: Tokens.ink; font.family: Tokens.display
                                font.pixelSize: 13; anchors.verticalCenter: parent.verticalCenter
                            }
                        }
                        Text {
                            width: parent.width
                            text: !pg.prowlInstalled ? I18n.tr("Not installed")
                                : (pg.prowlRunning ? I18n.tr("Running · %1").arg(pg.prowlUrl || ("127.0.0.1:" + pg.prowlPort))
                                   : I18n.tr("Offline · starts with Rashin"))
                            color: pg.prowlRunning ? Tokens.ink : Tokens.inkDim
                            font.family: Tokens.mono; font.pixelSize: 11; elide: Text.ElideRight
                        }
                        Text {
                            width: parent.width
                            text: pg.prowlError ? pg.prowlError
                                : I18n.tr("Models and code intelligence")
                            color: Tokens.inkDim; font.family: Tokens.mono; font.pixelSize: 10; elide: Text.ElideRight
                        }
                    }
                }
            }

            // ── FUNCTIONS: the numbered poster index of what Rashin does ──────
            Column {
                width: parent.width
                spacing: Tokens.s4
                Head { kanji: "\u6a5f\u80fd"; title: I18n.tr("FUNCTIONS") }

                Grid {
                    id: fnGrid
                    width: parent.width
                    columns: pg.bodyW >= 860 ? 3 : (pg.bodyW >= 560 ? 2 : 1)
                    columnSpacing: Tokens.s4
                    rowSpacing: Tokens.s4
                    readonly property real cellW: (width - columnSpacing * (columns - 1)) / columns

                    FnCard {
                        width: fnGrid.cellW; index: "01"; kanji: "\u66f8\u5eab"; name: I18n.tr("VAULT")
                        desc: I18n.tr("The living map your agents read, beside each binary.")
                        stat: pg.vaultExists ? I18n.tr("%1 files").arg(pg.vaultFiles) : ""
                    }
                    FnCard {
                        width: fnGrid.cellW; index: "02"; kanji: "\u8a18\u61b6"; name: I18n.tr("MEMORY")
                        desc: I18n.tr("What Hermes remembers, carried across every session.")
                    }
                    FnCard {
                        width: fnGrid.cellW; index: "03"; kanji: "\u6280"; name: I18n.tr("SKILLS")
                        desc: I18n.tr("Toolsets Hermes wields: search, files, the web.")
                    }
                    FnCard {
                        width: fnGrid.cellW; index: "04"; kanji: "\u4e94\u4eba\u8846"; name: I18n.tr("AGENTS")
                        desc: I18n.tr("Your coding agents, wired to one shared map of the machine.")
                        stat: pg.agentsPresent > 0 ? pg.wiredSummary : ""
                    }
                    FnCard {
                        width: fnGrid.cellW; index: "05"; kanji: "\u5bfe\u8a71"; name: I18n.tr("CHAT")
                        desc: I18n.tr("Talk to Hermes in the dashboard or a terminal.")
                    }
                    FnCard {
                        width: fnGrid.cellW; index: "06"; kanji: "\u7f85\u91dd"; name: I18n.tr("CODE")
                        desc: I18n.tr("Code intelligence: cited answers over your repos.")
                    }
                }
            }

            // ── YOUR AGENTS: one-click wire any coding CLI to the vault ───────
            Column {
                width: parent.width
                spacing: Tokens.s4
                Head { kanji: "\u4e94\u4eba\u8846"; title: I18n.tr("YOUR AGENTS") }
                Text {
                    width: parent.width
                    text: I18n.tr("Connect any coding agent to the same living map of this machine. One click adds the Ryoku and Prowl skills, then routes that agent's models through the Prowl gateway.")
                    color: Tokens.inkDim; font.family: Tokens.mono; font.pixelSize: 12; wrapMode: Text.WordWrap; lineHeight: 1.4
                }
                Column {
                    width: parent.width
                    spacing: Tokens.s2
                    Repeater {
                        model: pg.agentList
                        delegate: AgentRow {
                            required property var modelData
                            width: parent.width
                            agent: modelData
                            canChat: pg.agentCanChat(modelData.id)
                        }
                    }
                }
                PosterBtn { label: I18n.tr("WIRE ALL PRESENT"); on: pg.installed; onAct: pg.wireAll() }
            }

            // ── POINT ANY AGENT: the paths, and a paste snippet for the rest ──
            Column {
                width: parent.width
                spacing: Tokens.s4
                Head { kanji: "\u9023\u643a"; title: I18n.tr("POINT ANY AGENT") }
                Text {
                    width: parent.width
                    text: I18n.tr("On an agent Rashin doesn't wire for you? Point it at these yourself, or copy the ready-made instructions and paste them into its config.")
                    color: Tokens.inkDim; font.family: Tokens.mono; font.pixelSize: 12; wrapMode: Text.WordWrap; lineHeight: 1.4
                }
                Column {
                    width: parent.width
                    spacing: Tokens.s2
                    PathRow { label: I18n.tr("skill"); path: pg.skillPath; owner: I18n.tr("read-only"); ok: pg.skillPath !== "" }
                    PathRow { label: I18n.tr("prowl"); path: pg.prowlPath; owner: I18n.tr("tool"); ok: pg.prowlPath !== "" }
                    Repeater {
                        model: pg.vaultItems
                        delegate: PathRow {
                            required property var modelData
                            width: parent.width
                            label: modelData.label
                            path: modelData.path
                            owner: modelData.owner
                            ok: modelData.exists === true
                        }
                    }
                }
                PosterBtn { label: I18n.tr("COPY AGENT SNIPPET"); primary: true; on: pg.installed; onAct: pg.copySnippet() }
            }

            // ── CHAT BACKEND: who answers Super+S (Hermes recommended) ────────
            Column {
                width: parent.width
                spacing: Tokens.s3
                Head { kanji: "\u5bfe\u8a71"; title: I18n.tr("CHAT BACKEND") }
                Text {
                    width: parent.width
                    text: I18n.tr("Which agent answers the Super+S chat. Hermes is recommended; others need their own ACP adapter installed.")
                    color: Tokens.inkDim; font.family: Tokens.mono; font.pixelSize: 12; wrapMode: Text.WordWrap; lineHeight: 1.4
                }
                Flow {
                    width: parent.width
                    spacing: Tokens.s2
                    Repeater {
                        model: pg.chatBackends
                        delegate: Rectangle {
                            id: cb
                            required property var modelData
                            readonly property bool sel: cb.modelData.active === true
                            height: 34
                            width: cbT.implicitWidth + Tokens.s4 * 2
                            color: cb.sel ? Tokens.ink : "transparent"
                            opacity: cb.modelData.available ? 1 : 0.45
                            border.width: 1
                            border.color: cb.sel ? Tokens.ink : Tokens.line
                            Text {
                                id: cbT
                                anchors.centerIn: parent
                                text: (cb.modelData.name || cb.modelData.id) + (cb.modelData.recommended ? "  \u2605" : "")
                                color: cb.sel ? Tokens.paper : (cb.modelData.available ? Tokens.ink : Tokens.inkDim)
                                font.family: Tokens.mono; font.pixelSize: 11; font.letterSpacing: 1
                            }
                            MouseArea {
                                anchors.fill: parent
                                enabled: cb.modelData.available === true
                                cursorShape: Qt.PointingHandCursor
                                onClicked: pg.useChatAgent(cb.modelData.id)
                            }
                        }
                    }
                }
            }

            // ── TRY: example uses ────────────────────────────────────────────
            Column {
                width: parent.width
                spacing: Tokens.s3
                Head { kanji: "\u4f8b"; title: I18n.tr("TRY") }
                TryRow { cmd: "hermes"; note: I18n.tr("chat in the vault, from any terminal") }
                TryRow { cmd: "hermes gateway"; note: I18n.tr("connect Telegram / Discord / WhatsApp / Slack") }
                TryRow { cmd: "hermes model"; note: I18n.tr("switch the default model") }
                TryRow { cmd: "hermes tools"; note: I18n.tr("enable toolsets") }
                TryRow { cmd: "prowl overview"; note: I18n.tr("code intelligence on any repo") }
            }

            // ── SET UP + DASHBOARD: the controls ─────────────────────────────
            Column {
                width: parent.width
                spacing: Tokens.s4
                Head { kanji: "\u8d77\u52d5"; title: I18n.tr("GET STARTED") }

                Text {
                    width: parent.width
                    text: I18n.tr("Set up Hermes once: it installs the agent if you don't have it, wires it to the vault, and points your other coding agents at the same map. An existing Hermes install is left untouched.")
                    color: Tokens.inkDim; font.family: Tokens.mono; font.pixelSize: 12; wrapMode: Text.WordWrap; lineHeight: 1.4
                }

                Row {
                    width: parent.width
                    spacing: Tokens.s3
                    PosterBtn {
                        label: pg.hermesConfigured ? I18n.tr("RE-RUN HERMES SETUP") : I18n.tr("SET UP HERMES AGENT")
                        primary: !pg.hermesConfigured
                        on: pg.installed
                        onAct: pg.runSetup()
                    }
                    PosterBtn {
                        label: I18n.tr("OPEN DASHBOARD")
                        primary: pg.hermesConfigured
                        on: pg.installed
                        onAct: pg.openDashboard()
                    }
                }

                // live setup console, mirrored from setup.json. a failed phase
                // brightens to ink rather than shouting in red.
                Text {
                    width: parent.width
                    visible: pg.setupPhase !== ""
                    text: "\u2192 " + pg.setupPhase + (pg.setupDetail !== "" ? ": " + pg.setupDetail : "")
                    color: pg.setupOk ? Tokens.inkDim : Tokens.ink
                    font.family: Tokens.mono; font.pixelSize: 11; wrapMode: Text.WordWrap
                }
            }

            // ── privacy footer ───────────────────────────────────────────────
            Item { width: parent.width; height: Tokens.s2 }
            Row {
                width: parent.width
                spacing: Tokens.s3
                Rectangle { width: 8; height: 8; radius: 4; color: Tokens.alert; anchors.verticalCenter: parent.verticalCenter }
                Text {
                    width: parent.width - Tokens.s3 - 8
                    text: I18n.tr("Everything runs on this machine. The daemon binds 127.0.0.1 only - nothing you do here leaves the box.")
                    color: Tokens.inkDim; font.family: Tokens.mono; font.pixelSize: 11; wrapMode: Text.WordWrap
                    anchors.verticalCenter: parent.verticalCenter
                }
            }
        }
    }
}
