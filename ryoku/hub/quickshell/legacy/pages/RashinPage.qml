pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons

Item {
    id: pg

    property var hub
    readonly property bool fullBleed: true
    readonly property real bodyW: Math.min(width - Tokens.s5 * 2, 1080)

    property bool installed: false
    property bool rashinEnabled: false
    property bool running: false
    property bool busy: false
    property string errorText: ""
    property string askRoute: "auto"
    property int providerCount: 0
    property string defaultHarness: ""
    property int harnessCount: 0
    property int indexCount: 0

    Component.onCompleted: refresh()

    function refresh() {
        statusProc.running = true;
    }

    function setEnabled(value) {
        if (busy)
            return;
        busy = true;
        errorText = "";
        rashinEnabled = value;
        switchProc.command = ["ryoku-hub", "rashin", value ? "on" : "off"];
        switchProc.running = true;
    }

    function applyStatus(status) {
        installed = true;
        rashinEnabled = status.enabled === true;
        running = status.running === true;
        providerCount = status.gateway && typeof status.gateway.providers === "number" ? status.gateway.providers : 0;
        harnessCount = typeof status.harnesses === "number" ? status.harnesses : 0;
        indexCount = typeof status.indexes === "number" ? status.indexes : 0;
        const settings = status.settings || ({});
        askRoute = settings.ask && settings.ask.model ? String(settings.ask.model) : "auto";
        defaultHarness = settings.agent && settings.agent.harness ? String(settings.agent.harness) : "";
    }

    function routeName() {
        if (askRoute === "auto") return I18n.tr("Auto");
        if (askRoute === "auto:fast") return I18n.tr("Fast");
        if (askRoute === "auto:smart") return I18n.tr("Smart");
        return askRoute;
    }

    Process {
        id: statusProc
        command: ["/bin/sh", "-c", "exec rashin status --json"]
        environment: Spawn.env
        stderr: StdioCollector {}
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    pg.applyStatus(JSON.parse(this.text));
                } catch (error) {
                    pg.installed = false;
                    pg.rashinEnabled = false;
                    pg.running = false;
                    pg.providerCount = 0;
                    pg.harnessCount = 0;
                    pg.indexCount = 0;
                }
            }
        }
    }

    Process {
        id: switchProc
        environment: Spawn.env
        stderr: StdioCollector { id: switchError }
        onExited: code => {
            pg.busy = false;
            if (code !== 0)
                pg.errorText = switchError.text.trim().split("\n").pop() || I18n.tr("Rashin could not change its state.");
            pg.refresh();
        }
    }


    component SummaryCard: Rectangle {
        id: card
        property string title: ""
        property string value: ""
        property string detail: ""
        height: 150
        color: Tokens.paperLift
        border.width: 1
        border.color: Tokens.line
        Rectangle { anchors.left: parent.left; anchors.top: parent.top; width: 38; height: 3; color: Tokens.ink }
        Column {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.margins: Tokens.s4
            spacing: Tokens.s2
            Text { text: card.title; color: Tokens.ink; font.family: Tokens.display; font.pixelSize: 22 }
            Text { width: parent.width; text: card.value; color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: 12; elide: Text.ElideMiddle }
            Text { width: parent.width; text: card.detail; color: Tokens.inkDim; font.family: Tokens.mono; font.pixelSize: 11; wrapMode: Text.WordWrap }
        }
    }

    Rectangle { anchors.fill: parent; color: Tokens.paper }

    Flickable {
        anchors.fill: parent
        contentWidth: width
        contentHeight: body.implicitHeight + Tokens.s6 * 2
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll {}

        Column {
            id: body
            anchors.horizontalCenter: parent.horizontalCenter
            y: Tokens.s6
            width: pg.bodyW
            spacing: Tokens.s6

            Row {
                width: parent.width
                spacing: Tokens.s4
                Column {
                    width: parent.width - master.width - parent.spacing
                    spacing: Tokens.s2
                    Text { text: I18n.tr("Rashin"); color: Tokens.ink; font.family: Tokens.display; font.pixelSize: 38 }
                    Text {
                        width: parent.width
                        text: I18n.tr("Ask with free models, hand work to your coding harness, and search your system and projects.")
                        color: Tokens.inkDim
                        font.family: Tokens.mono
                        font.pixelSize: 13
                        wrapMode: Text.WordWrap
                    }
                }
                Rectangle {
                    id: master
                    width: 310
                    height: 88
                    color: Tokens.paperLift
                    border.width: 1
                    border.color: pg.rashinEnabled ? Tokens.ink : Tokens.line
                    Column {
                        anchors.left: parent.left
                        anchors.right: masterSwitch.left
                        anchors.leftMargin: Tokens.s4
                        anchors.rightMargin: Tokens.s3
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: Tokens.s1
                        Text { text: I18n.tr("Rashin"); color: Tokens.ink; font.family: Tokens.display; font.pixelSize: 17 }
                        Text {
                            width: parent.width
                            text: pg.busy ? I18n.tr("Changing state")
                                : !pg.installed ? I18n.tr("Not installed")
                                : pg.running ? I18n.tr("Running")
                                : pg.rashinEnabled ? I18n.tr("On, idle") : I18n.tr("Off")
                            color: Tokens.inkDim
                            font.family: Tokens.mono
                            font.pixelSize: 11
                            elide: Text.ElideRight
                        }
                    }
                    Sw {
                        id: masterSwitch
                        anchors.right: parent.right
                        anchors.rightMargin: Tokens.s4
                        anchors.verticalCenter: parent.verticalCenter
                        enabled: !pg.busy
                        on: pg.rashinEnabled
                        onToggled: value => pg.setEnabled(value)
                    }
                }
            }

            Rectangle {
                width: parent.width
                implicitHeight: zeroText.implicitHeight + Tokens.s4 * 2
                color: Tokens.tint5
                border.width: 1
                border.color: Tokens.lineSoft
                Text {
                    id: zeroText
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.top: parent.top
                    anchors.margins: Tokens.s4
                    text: I18n.tr("Rashin is optional. While it is off, no daemon, socket, timer, watcher, model, or index process runs.")
                    color: Tokens.inkDim
                    font.family: Tokens.mono
                    font.pixelSize: 12
                    wrapMode: Text.WordWrap
                }
            }

            Text {
                width: parent.width
                visible: pg.errorText.length > 0
                text: pg.errorText
                color: Tokens.alert
                font.family: Tokens.mono
                font.pixelSize: 12
                wrapMode: Text.WordWrap
            }

            Grid {
                width: parent.width
                columns: width >= 840 ? 3 : 1
                columnSpacing: Tokens.s4
                rowSpacing: Tokens.s4
                readonly property real cardWidth: (width - columnSpacing * (columns - 1)) / columns
                SummaryCard {
                    width: parent.cardWidth
                    title: I18n.tr("Ask")
                    value: I18n.tr("Route: %1").arg(pg.routeName())
                    detail: I18n.tr("%1 free provider(s) available").arg(pg.providerCount)
                }
                SummaryCard {
                    width: parent.cardWidth
                    title: I18n.tr("Agent")
                    value: pg.defaultHarness.length > 0 ? pg.defaultHarness : I18n.tr("First installed harness")
                    detail: I18n.tr("%1 installed harness(es)").arg(pg.harnessCount)
                }
                SummaryCard {
                    width: parent.cardWidth
                    title: I18n.tr("Index")
                    value: I18n.tr("System and project search")
                    detail: I18n.tr("%1 index(es)").arg(pg.indexCount)
                }
            }

            Row {
                spacing: Tokens.s3
                Btn {
                    text: I18n.tr("Open Rashin")
                    primary: true
                    armed: pg.installed
                    onAct: Spawn.runApp(["rashin", "open"], { label: "rashin" })
                }
                Btn {
                    text: I18n.tr("Open settings")
                    armed: pg.installed
                    onAct: Spawn.runApp(["rashin", "open", "--tab", "settings"], { label: "rashin" })
                }
            }
        }
    }
}
