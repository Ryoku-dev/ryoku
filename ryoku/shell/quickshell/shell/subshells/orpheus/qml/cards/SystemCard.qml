pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import shell.services
import Ryoku.Ui.Singletons
import "../kit" as K

K.Sheet {
    id: root

    required property var scene
    property bool shown: false
    property var args: ({})
    property bool watchingSession: false

    readonly property real diskFraction: StatsFeed.diskTotalGiB > 0
        ? StatsFeed.diskUsedGiB / StatsFeed.diskTotalGiB : 0
    readonly property real networkPeak: {
        let peak = 0;
        const down = StatsFeed.netDownHist || [];
        const up = StatsFeed.netUpHist || [];
        for (let index = 0; index < down.length; ++index)
            peak = Math.max(peak, Number(down[index]) || 0);
        for (let index = 0; index < up.length; ++index)
            peak = Math.max(peak, Number(up[index]) || 0);
        return peak;
    }
    readonly property var normalizedDown: normalize(StatsFeed.netDownHist, networkPeak)
    readonly property var normalizedUp: normalize(StatsFeed.netUpHist, networkPeak)

    pageWidth: 360

    function setLive(active) {
        Sysinfo.setActive(root, active);
        StatsFeed.setActive(root, active);
        if (active !== watchingSession) {
            Session.watchers = Math.max(0, Session.watchers + (active ? 1 : -1));
            watchingSession = active;
        }
    }

    function normalize(values, peak) {
        const source = values || [];
        const divisor = peak > 0 ? peak : 1;
        const result = [];
        for (let index = 0; index < source.length; ++index)
            result.push(Math.max(0, Math.min(1,
                (Number(source[index]) || 0) / divisor)));
        return result;
    }

    function rate(bytes) {
        const value = Math.max(0, Number(bytes) || 0);
        if (value < 1024)
            return Math.round(value) + " B/s";
        if (value < 1048576)
            return (value / 1024).toFixed(1) + " KB/s";
        if (value < 1073741824)
            return (value / 1048576).toFixed(1) + " MB/s";
        return (value / 1073741824).toFixed(1) + " GB/s";
    }

    function gib(value) {
        return Math.max(0, Number(value) || 0).toFixed(1) + " GiB";
    }

    onShownChanged: setLive(shown)
    Component.onCompleted: setLive(shown)
    Component.onDestruction: setLive(false)

    K.Scroller {
        width: parent.width
        height: Math.min(contentHeight, Math.max(180,
            root.scene.cardMaxHeight - 96))

        Column {
            width: parent.width
            spacing: K.Style.s4

            Rectangle {
                width: parent.width
                height: Math.max(88, cpuColumn.implicitHeight + K.Style.s4 * 2)
                radius: K.Style.plateRadius
                color: K.Style.plate
                border.width: K.Style.border
                border.color: K.Style.lineSoft

                Column {
                    id: cpuColumn
                    anchors.left: parent.left
                    anchors.leftMargin: K.Style.s4
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: K.Style.s1

                    K.Label {
                        text: "CPU"
                        role: "micro"
                    }

                    K.Label {
                        text: Math.round(Sysinfo.cpu * 100) + "%"
                        role: "headline"
                        numeric: true
                    }

                    K.Label {
                        visible: Sysinfo.hasTemp
                        text: Math.round(Sysinfo.tempC) + " °C"
                        role: "caption"
                        tone: "dim"
                        numeric: true
                    }
                }

                K.Sparkline {
                    anchors.left: parent.left
                    anchors.leftMargin: K.Style.s7 * 2 + K.Style.s2
                    anchors.right: parent.right
                    anchors.rightMargin: K.Style.s4
                    anchors.verticalCenter: parent.verticalCenter
                    height: 44
                    values: Sysinfo.cpuHistory
                    fill: true
                }
            }

            Rectangle {
                width: parent.width
                implicitHeight: memoryColumn.implicitHeight + K.Style.s4 * 2
                height: implicitHeight
                radius: K.Style.plateRadius
                color: K.Style.plate
                border.width: K.Style.border
                border.color: K.Style.lineSoft

                Column {
                    id: memoryColumn
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.top: parent.top
                    anchors.margins: K.Style.s4
                    spacing: K.Style.s2

                    Item {
                        width: parent.width
                        height: Math.max(memoryLabel.implicitHeight, memoryValue.implicitHeight)

                        K.Label {
                            id: memoryLabel
                            anchors.left: parent.left
                            anchors.verticalCenter: parent.verticalCenter
                            text: "Memory"
                            role: "micro"
                        }

                        K.Label {
                            id: memoryValue
                            anchors.left: memoryLabel.right
                            anchors.leftMargin: K.Style.s2
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            text: root.gib(Sysinfo.memUsedGiB) + " / "
                                + root.gib(Sysinfo.memTotalGiB)
                            role: "body"
                            tone: "dim"
                            numeric: true
                            horizontalAlignment: Text.AlignRight
                        }
                    }

                    K.Meter {
                        width: parent.width
                        value: Sysinfo.mem
                    }
                }
            }

            Rectangle {
                visible: StatsFeed.gpuAvailable
                width: parent.width
                implicitHeight: gpuColumn.implicitHeight + K.Style.s4 * 2
                radius: K.Style.plateRadius
                color: K.Style.plate
                border.width: K.Style.border
                border.color: K.Style.lineSoft

                Column {
                    id: gpuColumn
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.top: parent.top
                    anchors.margins: K.Style.s4
                    spacing: K.Style.s2

                    Item {
                        width: parent.width
                        height: Math.max(gpuLabel.implicitHeight, gpuValues.implicitHeight)

                        K.Label {
                            id: gpuLabel
                            anchors.left: parent.left
                            anchors.verticalCenter: parent.verticalCenter
                            text: "GPU"
                            role: "micro"
                        }

                        Row {
                            id: gpuValues
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: K.Style.s3

                            K.Label {
                                text: Math.round(StatsFeed.gpuPct) + "%"
                                role: "body"
                                numeric: true
                            }

                            K.Label {
                                text: Math.round(StatsFeed.gpuTempC) + " °C"
                                role: "body"
                                tone: "dim"
                                numeric: true
                            }

                            K.Label {
                                text: StatsFeed.gpuPowerW.toFixed(1) + " W"
                                role: "body"
                                tone: "dim"
                                numeric: true
                            }
                        }
                    }

                    K.Meter {
                        width: parent.width
                        value: StatsFeed.gpuPct / 100
                    }
                }
            }

            Rectangle {
                width: parent.width
                implicitHeight: networkColumn.implicitHeight + K.Style.s4 * 2
                height: implicitHeight
                radius: K.Style.plateRadius
                color: K.Style.plate
                border.width: K.Style.border
                border.color: K.Style.lineSoft

                Column {
                    id: networkColumn
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.top: parent.top
                    anchors.margins: K.Style.s4
                    spacing: K.Style.s3

                    Item {
                        width: parent.width
                        height: Math.max(networkLabel.implicitHeight, networkValues.implicitHeight)

                        K.Label {
                            id: networkLabel
                            anchors.left: parent.left
                            anchors.verticalCenter: parent.verticalCenter
                            text: "Network"
                            role: "micro"
                        }

                        Row {
                            id: networkValues
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: K.Style.s3

                            K.Label {
                                text: "↓ " + root.rate(StatsFeed.downBps)
                                role: "body"
                                numeric: true
                            }

                            K.Label {
                                text: "↑ " + root.rate(StatsFeed.upBps)
                                role: "body"
                                tone: "dim"
                                numeric: true
                            }
                        }
                    }

                    Row {
                        width: parent.width
                        spacing: K.Style.s2

                        K.Sparkline {
                            width: (networkColumn.width - K.Style.s2) / 2
                            height: 30
                            values: root.normalizedDown
                            fill: true
                        }

                        K.Sparkline {
                            width: (networkColumn.width - K.Style.s2) / 2
                            height: 30
                            values: root.normalizedUp
                            fill: true
                        }
                    }
                }
            }

            Rectangle {
                width: parent.width
                implicitHeight: diskColumn.implicitHeight + K.Style.s4 * 2
                height: implicitHeight
                radius: K.Style.plateRadius
                color: K.Style.plate
                border.width: K.Style.border
                border.color: K.Style.lineSoft

                Column {
                    id: diskColumn
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.top: parent.top
                    anchors.margins: K.Style.s4
                    spacing: K.Style.s2

                    Item {
                        width: parent.width
                        height: Math.max(diskLabel.implicitHeight, diskValue.implicitHeight)

                        K.Label {
                            id: diskLabel
                            anchors.left: parent.left
                            anchors.verticalCenter: parent.verticalCenter
                            text: "Disk"
                            role: "micro"
                        }

                        K.Label {
                            id: diskValue
                            anchors.left: diskLabel.right
                            anchors.leftMargin: K.Style.s2
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            text: root.gib(StatsFeed.diskUsedGiB) + " / "
                                + root.gib(StatsFeed.diskTotalGiB)
                            role: "body"
                            tone: "dim"
                            numeric: true
                            horizontalAlignment: Text.AlignRight
                        }
                    }

                    K.Meter {
                        width: parent.width
                        value: root.diskFraction
                    }
                }
            }

            Rectangle {
                width: parent.width
                height: 72
                radius: K.Style.plateRadius
                color: K.Style.plate
                border.width: K.Style.border
                border.color: K.Style.lineSoft

                K.Glyph {
                    id: machineGlyph
                    anchors.left: parent.left
                    anchors.leftMargin: K.Style.s4
                    anchors.verticalCenter: parent.verticalCenter
                    name: "computer"
                    size: 24
                    tone: "dim"
                }

                Column {
                    anchors.left: machineGlyph.right
                    anchors.leftMargin: K.Style.s3
                    anchors.right: btopButton.left
                    anchors.rightMargin: K.Style.s2
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: K.Style.s1

                    K.Label {
                        width: parent.width
                        text: Session.user + "@" + Session.host
                        role: "bodyStrong"
                    }

                    K.Label {
                        width: parent.width
                        text: "Up " + Session.uptimeText
                        role: "caption"
                        tone: "dim"
                        numeric: true
                    }
                }

                K.Button {
                    id: btopButton
                    anchors.right: parent.right
                    anchors.rightMargin: K.Style.s3
                    anchors.verticalCenter: parent.verticalCenter
                    text: "Open btop"
                    icon: "monitoring"
                    variant: "outline"
                    onClicked: {
                        Spawn.runApp(["ryoku-app", "terminal", "--", "btop"], { label: "btop" });
                        root.scene.closeCard();
                    }
                }
            }
        }
    }
}
