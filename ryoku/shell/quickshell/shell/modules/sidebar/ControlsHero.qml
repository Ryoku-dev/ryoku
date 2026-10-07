pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls as QQC
import Quickshell
import Ryoku.Blobs
import Ryoku.Ui.Singletons
import shell.services

Item {
    id: root

    required property var monitor
    required property bool active
    required property real s

    QtObject {
        id: palette
        readonly property color memory: Tokens.role("secondary", Tokens.inkDim)
        readonly property color gpu: Tokens.role("tertiary", Tokens.inkMuted)
    }

    implicitHeight: (Tokens.s7 * 4 + Tokens.s5) * s
    clip: true

    function blendColor(from, to, amount): color {
        const t = Math.max(0, Math.min(1, amount));
        return Qt.rgba(from.r + (to.r - from.r) * t,
                       from.g + (to.g - from.g) * t,
                       from.b + (to.b - from.b) * t,
                       from.a + (to.a - from.a) * t);
    }

    function temperatureColor(value): color {
        if (!isFinite(value) || value < 60)
            return Tokens.ink;
        if (value >= 90)
            return Tokens.alert;
        if (value <= 85)
            return root.blendColor(Tokens.ink, Tokens.sun, (value - 60) / 25);
        return root.blendColor(Tokens.sun, Tokens.alert, (value - 85) / 5);
    }

    function formatRate(bytes): string {
        const value = Math.max(0, bytes);
        if (value >= 1024 * 1024 * 1024)
            return I18n.tr("%1 GB/s").arg((value / (1024 * 1024 * 1024)).toFixed(1));
        if (value >= 1024 * 1024)
            return I18n.tr("%1 MB/s").arg((value / (1024 * 1024)).toFixed(1));
        return I18n.tr("%1 KB/s").arg((value / 1024).toFixed(1));
    }

    function formatDiskRates(readBytes, writeBytes): string {
        const maximum = Math.max(0, readBytes, writeBytes);
        let divisor = 1024;
        let unit = I18n.tr("KB/s");
        if (maximum >= 1024 * 1024 * 1024) {
            divisor = 1024 * 1024 * 1024;
            unit = I18n.tr("GB/s");
        } else if (maximum >= 1024 * 1024) {
            divisor = 1024 * 1024;
            unit = I18n.tr("MB/s");
        }
        return I18n.tr("R %1 · W %2 %3")
            .arg((Math.max(0, readBytes) / divisor).toFixed(1))
            .arg((Math.max(0, writeBytes) / divisor).toFixed(1))
            .arg(unit);
    }

    function uptime(value): string {
        const minutes = Math.max(0, Math.floor(value / 60));
        if (minutes < 60)
            return I18n.tr("Up %1m").arg(minutes);
        const hours = Math.floor(minutes / 60);
        if (hours < 24)
            return I18n.tr("Up %1h %2m").arg(hours).arg(minutes % 60);
        return I18n.tr("Up %1d %2h").arg(Math.floor(hours / 24)).arg(hours % 24);
    }

    // Each reading glides to its next value over exactly one sample period, so
    // it is still in motion when the following sample lands and never steps.
    QtObject {
        id: displayed

        readonly property int settle: root.monitor.samplePeriodMs
        readonly property bool eased: !Tokens.reduceMotion && !Motion.reduce
        property real cpu: root.monitor.cpuAvailable && isFinite(root.monitor.cpuPercent)
                           ? root.monitor.cpuPercent
                           : 0
        property real cpuFrequency: isFinite(root.monitor.cpuFrequencyGhz)
                                    ? root.monitor.cpuFrequencyGhz
                                    : 0
        property real cpuTemperature: root.monitor.cpuTempAvailable && isFinite(root.monitor.cpuTemp)
                                      ? root.monitor.cpuTemp
                                      : 0
        property real load: isFinite(root.monitor.loadAverage) ? root.monitor.loadAverage : 0
        property real uptime: isFinite(root.monitor.uptimeSeconds) ? root.monitor.uptimeSeconds : 0

        Behavior on cpu {
            enabled: displayed.eased
            NumberAnimation { duration: displayed.settle; easing.type: Easing.Linear }
        }
        Behavior on cpuFrequency {
            enabled: displayed.eased
            NumberAnimation { duration: displayed.settle; easing.type: Easing.Linear }
        }
        Behavior on cpuTemperature {
            enabled: displayed.eased
            NumberAnimation { duration: displayed.settle; easing.type: Easing.Linear }
        }
        Behavior on load {
            enabled: displayed.eased
            NumberAnimation { duration: displayed.settle; easing.type: Easing.Linear }
        }
        Behavior on uptime {
            enabled: displayed.eased
            NumberAnimation { duration: displayed.settle; easing.type: Easing.Linear }
        }
    }

    SystemClock {
        id: clock
        precision: SystemClock.Minutes
        enabled: root.active
    }

    Item {
        id: identity
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: (Tokens.s7 + Tokens.s3) * root.s
        clip: true

        Rectangle {
            id: avatar
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            width: Tokens.s6 * root.s
            height: width
            radius: width / 2
            color: Tokens.role("primaryContainer", Tokens.tint10)

            Text {
                anchors.fill: parent
                text: root.monitor.userName.length > 0
                      ? root.monitor.userName.slice(0, 1).toUpperCase()
                      : I18n.tr("—")
                textFormat: Text.PlainText
                color: Tokens.role("onPrimaryContainer", Tokens.ink)
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall * root.s
                font.weight: Font.DemiBold
                elide: Text.ElideRight
                horizontalAlignment: Text.AlignHCenter
                verticalAlignment: Text.AlignVCenter
            }
        }

        Column {
            anchors.left: avatar.right
            anchors.leftMargin: Tokens.s2 * root.s
            anchors.right: clockBlock.left
            anchors.rightMargin: Tokens.s4 * root.s
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s1 * root.s

            Text {
                width: parent.width
                text: I18n.tr("%1@%2").arg(root.monitor.userName).arg(root.monitor.hostName)
                textFormat: Text.PlainText
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fRow * root.s
                font.weight: Font.Medium
                elide: Text.ElideRight
            }

            Text {
                width: parent.width
                text: root.monitor.sampleCount > 0
                      ? root.uptime(displayed.uptime)
                        + I18n.tr(" · Load %1").arg(displayed.load.toFixed(2))
                      : I18n.tr("—")
                textFormat: Text.PlainText
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall * root.s
                elide: Text.ElideRight
            }
        }

        Column {
            id: clockBlock
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            width: Tokens.s7 * 2 * root.s
            spacing: Tokens.s1 * root.s

            Text {
                width: parent.width
                text: TimeFormat.format(clock.date)
                textFormat: Text.PlainText
                color: Tokens.ink
                font.family: Tokens.display
                font.pixelSize: Tokens.fHero * root.s
                fontSizeMode: Text.HorizontalFit
                minimumPixelSize: Tokens.fBody * root.s
                font.weight: Font.Light
                elide: Text.ElideRight
                horizontalAlignment: Text.AlignRight
            }

            Text {
                width: parent.width
                text: Qt.formatDateTime(clock.date, I18n.tr("ddd, MMM d"))
                textFormat: Text.PlainText
                color: Tokens.inkMuted
                font.family: Tokens.mono
                font.pixelSize: Tokens.fMicro * root.s
                elide: Text.ElideRight
                horizontalAlignment: Text.AlignRight
            }
        }
    }

    Row {
        id: body
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: identity.bottom
        anchors.topMargin: Tokens.s3 * root.s
        anchors.bottom: parent.bottom
        spacing: Tokens.s4 * root.s

        Item {
            id: cpuZone
            width: (Tokens.s7 * 3 + Tokens.s2) * root.s
            height: parent.height
            clip: true

            Text {
                id: cpuHeading
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: parent.top
                text: I18n.tr("CPU")
                textFormat: Text.PlainText
                color: Tokens.inkMuted
                font.family: Tokens.mono
                font.pixelSize: Tokens.fMicro * root.s
                font.letterSpacing: Tokens.trackLabel * root.s
                elide: Text.ElideRight
            }

            Item {
                id: cpuReading
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: cpuHeading.bottom
                anchors.topMargin: Tokens.s2 * root.s
                height: cpuPercent.implicitHeight + cpuFrequency.implicitHeight

                Text {
                    id: cpuPercent
                    anchors.left: parent.left
                    anchors.right: temperaturePill.visible ? temperaturePill.left : parent.right
                    anchors.rightMargin: temperaturePill.visible ? Tokens.s2 * root.s : 0
                    anchors.top: parent.top
                    text: root.monitor.cpuAvailable
                          ? I18n.tr("%1%").arg(Math.round(displayed.cpu))
                          : I18n.tr("—")
                    textFormat: Text.PlainText
                    color: Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fHero * root.s
                fontSizeMode: Text.HorizontalFit
                minimumPixelSize: Tokens.fBody * root.s
                    font.weight: Font.Light
                    elide: Text.ElideRight

                    HoverHandler {
                        id: cpuHover
                    }

                    CornerTip { s: root.s; visible: cpuHover.hovered; text: root.monitor.cpuName.length > 0 ? root.monitor.cpuName : I18n.tr("CPU") }
                }

                Text {
                    id: cpuFrequency
                    anchors.left: parent.left
                    anchors.right: temperaturePill.visible ? temperaturePill.left : parent.right
                    anchors.rightMargin: temperaturePill.visible ? Tokens.s2 * root.s : 0
                    anchors.top: cpuPercent.bottom
                    text: displayed.cpuFrequency > 0
                          ? I18n.tr("%1 GHz").arg(displayed.cpuFrequency.toFixed(2))
                          : I18n.tr("—")
                    textFormat: Text.PlainText
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall * root.s
                    elide: Text.ElideRight
                }

                Rectangle {
                    id: temperaturePill
                    visible: root.monitor.cpuTempAvailable
                    anchors.right: parent.right
                    anchors.verticalCenter: cpuPercent.verticalCenter
                    width: temperatureRow.implicitWidth + Tokens.s1 * root.s * 2
                    height: Tokens.ctlH * root.s
                    radius: Tokens.radius * root.s
                    color: Tokens.tint5
                    border.width: Tokens.border * root.s
                    border.color: Tokens.lineSoft

                    Row {
                        id: temperatureRow
                        anchors.centerIn: parent
                        spacing: Tokens.s1 * root.s

                        Rectangle {
                            anchors.verticalCenter: parent.verticalCenter
                            width: Tokens.s1 * root.s
                            height: width
                            radius: width / 2
                            color: root.temperatureColor(displayed.cpuTemperature)
                        }

                        Text {
                            text: I18n.tr("%1 °C").arg(Math.round(displayed.cpuTemperature))
                            textFormat: Text.PlainText
                            color: Tokens.ink
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall * root.s
                            elide: Text.ElideRight
                        }
                    }

                    HoverHandler {
                        id: temperatureHover
                    }

                    CornerTip { s: root.s; visible: temperatureHover.hovered; text: root.monitor.cpuTempLabel === "zone" ? I18n.tr("Thermal zone temperature") : I18n.tr("CPU package temperature") }
                }
            }

            VitalCores {
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: cpuReading.bottom
                anchors.topMargin: Tokens.s2 * root.s
                height: Math.min(implicitHeight, Math.max(0, cpuZone.height - y))
                monitor: root.monitor
                s: root.s
            }
        }

        Item {
            id: graphZone
            width: (Tokens.s7 * 4 + Tokens.s5) * root.s
            height: parent.height
            clip: true

            readonly property bool inlineLegend: activityLabel.implicitWidth
                                                   + Tokens.s3 * root.s
                                                   + graphLegend.implicitWidth
                                                   <= width

            Text {
                id: activityLabel
                anchors.left: parent.left
                anchors.top: parent.top
                width: graphZone.inlineLegend ? implicitWidth : parent.width
                text: I18n.tr("ACTIVITY")
                textFormat: Text.PlainText
                color: Tokens.inkMuted
                font.family: Tokens.mono
                font.pixelSize: Tokens.fMicro * root.s
                font.letterSpacing: Tokens.trackLabel * root.s
                elide: Text.ElideRight
            }

            Row {
                id: graphLegend
                anchors.right: parent.right
                anchors.top: graphZone.inlineLegend ? parent.top : undefined
                anchors.bottom: graphZone.inlineLegend ? undefined : parent.bottom
                spacing: Tokens.s3 * root.s

                Repeater {
                    model: [
                        { label: I18n.tr("CPU"), color: Tokens.sun },
                        { label: I18n.tr("MEM"), color: palette.memory },
                        { label: I18n.tr("GPU"), color: palette.gpu }
                    ]

                    delegate: Row {
                        required property var modelData
                        spacing: Tokens.s1 * root.s

                        Text {
                            text: I18n.tr("●")
                            textFormat: Text.PlainText
                            color: parent.modelData.color
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fMicro * root.s
                        }

                        Text {
                            text: parent.modelData.label
                            textFormat: Text.PlainText
                            color: Tokens.inkFaint
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fMicro * root.s
                        }
                    }
                }
            }

            VitalSparkline {
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: activityLabel.bottom
                anchors.topMargin: Tokens.s2 * root.s
                anchors.bottom: graphZone.inlineLegend ? parent.bottom : graphLegend.top
                anchors.bottomMargin: graphZone.inlineLegend ? 0 : Tokens.s2 * root.s
                monitor: root.monitor
                active: root.active
                s: root.s
                channels: ["cpu", "memory", "gpu"]
                colors: [Tokens.sun, palette.memory, palette.gpu]
                fill: true
                showGrid: true
                detailPoints: false
                lineWidth: Tokens.border * root.s
            }
        }

        Column {
            id: meters
            width: Math.max(0, body.width - cpuZone.width - graphZone.width - body.spacing * 2)
            height: parent.height
            spacing: 0
            clip: true

            readonly property real rowHeight: height / 5

            VitalMeter {
                width: parent.width
                height: meters.rowHeight
                visible: root.monitor.memoryAvailable
                s: root.s
                settle: root.monitor.samplePeriodMs
                label: I18n.tr("Memory")
                valueText: {
                    const memory = I18n.tr("%1 / %2 GiB")
                        .arg(root.monitor.memoryUsedGiB.toFixed(1))
                        .arg(root.monitor.memoryTotalGiB.toFixed(1));
                    return root.monitor.swapUsedGiB > 0.1
                           ? memory + I18n.tr(" · swap %1 GiB").arg(root.monitor.swapUsedGiB.toFixed(1))
                           : memory;
                }
                showProgress: true
                progressValue: root.monitor.memoryUsedGiB
                progressMaximum: root.monitor.memoryTotalGiB
                accent: palette.memory
            }

            VitalMeter {
                width: parent.width
                height: meters.rowHeight
                visible: root.monitor.gpuAvailable
                s: root.s
                settle: root.monitor.samplePeriodMs
                label: I18n.tr("GPU")
                valueText: root.monitor.gpuTempAvailable
                           ? I18n.tr("%1% · %2 °C")
                               .arg(Math.round(root.monitor.gpuPercent))
                               .arg(Math.round(root.monitor.gpuTemp))
                           : I18n.tr("%1%").arg(Math.round(root.monitor.gpuPercent))
                tooltipText: root.monitor.gpuMemoryAvailable
                             ? I18n.tr("VRAM %1 / %2 GiB")
                                 .arg(root.monitor.gpuMemoryUsedGiB.toFixed(1))
                                 .arg(root.monitor.gpuMemoryTotalGiB.toFixed(1))
                             : ""
                showProgress: true
                progressValue: root.monitor.gpuPercent
                accent: palette.gpu
            }

            VitalMeter {
                id: networkMeter
                width: parent.width
                height: meters.rowHeight
                visible: root.monitor.networkAvailable
                s: root.s
                settle: root.monitor.samplePeriodMs
                label: I18n.tr("Network")
                valueText: I18n.tr("↓ %1  ↑ %2")
                    .arg(root.formatRate(root.monitor.networkRxBytesPerSec))
                    .arg(root.formatRate(root.monitor.networkTxBytesPerSec))
                tooltipText: root.monitor.networkInterface.length > 0
                             ? I18n.tr("Interface %1").arg(root.monitor.networkInterface)
                             : ""
                meterContent: Component {
                    VitalSparkline {
                        monitor: root.monitor
                        active: root.active && networkMeter.visible
                        s: root.s
                        channels: ["netRx", "netTx"]
                        colors: [Tokens.sun, palette.memory]
                        fill: false
                        showGrid: false
                        detailPoints: false
                    }
                }
            }

            VitalMeter {
                width: parent.width
                height: meters.rowHeight
                visible: root.monitor.diskAvailable && root.monitor.storageTotalGiB > 0
                s: root.s
                settle: root.monitor.samplePeriodMs
                label: I18n.tr("Disk")
                valueText: root.formatDiskRates(root.monitor.diskReadBytesPerSec,
                                                root.monitor.diskWriteBytesPerSec)
                tooltipText: {
                    const usage = I18n.tr("Root %1%").arg(Math.round(
                        root.monitor.storageUsedGiB / root.monitor.storageTotalGiB * 100));
                    return root.monitor.storageTempAvailable
                           ? I18n.tr("%1 · %2 °C").arg(usage).arg(Math.round(root.monitor.storageTemp))
                           : usage;
                }
                showProgress: true
                progressValue: root.monitor.storageUsedGiB
                progressMaximum: root.monitor.storageTotalGiB
                accent: Tokens.inkDim
            }

            VitalMeter {
                width: parent.width
                height: meters.rowHeight
                visible: root.monitor.batteryAvailable
                s: root.s
                settle: root.monitor.samplePeriodMs
                label: I18n.tr("Battery")
                valueText: I18n.tr("%1% · %2")
                    .arg(Math.round(root.monitor.batteryPercent))
                    .arg(root.monitor.batteryCharging ? I18n.tr("Charging") : I18n.tr("On battery"))
                showProgress: true
                progressValue: root.monitor.batteryPercent
                accent: root.monitor.batteryCharging ? Tokens.sun : Tokens.inkDim
            }
        }
    }
}
