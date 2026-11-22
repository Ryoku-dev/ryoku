pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Ryoku.Blobs
import Lithic
import Ryoku.Ui.Singletons
import shell.services
import "../ask/look"

Item {
    id: root

    required property bool active
    required property real s
    // the shipped board turns this off because the panel's head shows who it is
    property bool identity: true

    readonly property bool showIdentity: root.identity && SidebarState.elementVisible("identity")
    readonly property bool showCpu: SidebarState.elementVisible("cpu")
    readonly property bool showTemperature: SidebarState.elementVisible("cpuTemperature")
    readonly property bool showGraph: SidebarState.elementVisible("liveGraph")
    readonly property bool showMemory: SidebarState.elementVisible("memory")
    readonly property bool showGpu: SidebarState.elementVisible("gpu")
    readonly property bool showNetwork: SidebarState.elementVisible("network")
    readonly property bool showDisk: SidebarState.elementVisible("disk")
    readonly property bool showBattery: SidebarState.elementVisible("battery")
    readonly property bool hasReadings: root.showCpu || root.showTemperature || root.showGraph || root.showMemory
        || root.showGpu || root.showNetwork || root.showDisk || root.showBattery
    readonly property var monitor: systemMonitor

    readonly property bool cpuShown: root.showCpu || root.showTemperature || root.showGraph
    readonly property bool memoryShown: root.showMemory && root.monitor.memoryAvailable
    readonly property bool gpuShown: root.showGpu && root.monitor.gpuAvailable
    readonly property bool networkShown: root.showNetwork && root.monitor.networkAvailable
    readonly property bool diskShown: root.showDisk && root.monitor.diskAvailable && root.monitor.storageTotalGiB > 0
    readonly property bool batteryShown: root.showBattery && root.monitor.batteryAvailable
    readonly property int cellCount: (root.memoryShown ? 1 : 0) + (root.gpuShown ? 1 : 0)
        + (root.networkShown ? 1 : 0) + (root.diskShown ? 1 : 0) + (root.batteryShown ? 1 : 0)
    readonly property bool readingsShown: root.cpuShown || root.cellCount > 0

    readonly property bool carded: QsLook.adwaita || QsLook.fluent
    readonly property real inset: root.carded ? Math.round(14 * root.s) : 0
    readonly property real cpuInset: QsLook.material ? Math.round(14 * root.s) : 0
    readonly property real sectionGap: Math.round((QsLook.material ? 8 : QsLook.lithic ? 14 : 12) * root.s)
    readonly property real columnGap: Math.round((QsLook.material ? 8 : 14) * root.s)
    readonly property real rowGap: Math.round((QsLook.material ? 8 : QsLook.lithic ? 10 : 12) * root.s)
    readonly property real figurePx: Math.round((QsLook.lithic ? 26 * QsLook.figureScale : 28) * root.s)
    readonly property color cpuTemperatureInk: root.quietInk(QsLook.temperatureInk(displayed.cpuTemperature))

    implicitHeight: stack.implicitHeight
    Behavior on implicitHeight {
        enabled: root.active
        NumberAnimation { duration: QsLook.dur(QsLook.unfoldMs); easing.type: Easing.BezierSpline; easing.bezierCurve: QsLook.unfold }
    }
    clip: true

    SystemMonitor {
        id: systemMonitor
        active: root.active && (root.showIdentity || root.hasReadings)
    }

    SystemClock {
        id: clock
        precision: SystemClock.Minutes
        enabled: root.active && root.showIdentity
    }

    function quietInk(ink) {
        return Qt.colorEqual(ink, Lithic.fg) ? Lithic.fg2 : ink;
    }

    function formatRate(bytes): string {
        const value = Math.max(0, bytes);
        if (value >= 1024 * 1024 * 1024)
            return I18n.tr("%1 GB/s").arg((value / (1024 * 1024 * 1024)).toFixed(1));
        if (value >= 1024 * 1024)
            return I18n.tr("%1 MB/s").arg((value / (1024 * 1024)).toFixed(1));
        return I18n.tr("%1 KB/s").arg((value / 1024).toFixed(1));
    }

    // a half-width cell only fits compact figures, the tooltip keeps the full rates
    function compactRate(bytes): string {
        const value = Math.max(0, bytes);
        if (value >= 1024 * 1024 * 1024)
            return I18n.tr("%1G").arg((value / (1024 * 1024 * 1024)).toFixed(1));
        if (value >= 1024 * 1024)
            return I18n.tr("%1M").arg((value / (1024 * 1024)).toFixed(1));
        return I18n.tr("%1K").arg((value / 1024).toFixed(value >= 100 * 1024 ? 0 : 1));
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
        readonly property bool eased: !QsLook.still
        property real cpu: root.monitor.cpuAvailable && isFinite(root.monitor.cpuPercent)
            ? root.monitor.cpuPercent : 0
        property real cpuFrequency: isFinite(root.monitor.cpuFrequencyGhz) ? root.monitor.cpuFrequencyGhz : 0
        property real cpuTemperature: root.monitor.cpuTempAvailable && isFinite(root.monitor.cpuTemp)
            ? root.monitor.cpuTemp : 0
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

    Column {
        id: stack
        width: root.width
        spacing: Math.round(14 * root.s)

        Item {
            id: identityRow
            visible: root.showIdentity
            width: stack.width
            height: Math.round(44 * root.s)

            Avatar {
                id: avatar
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                s: root.s
                size: 36
                userName: root.monitor.userName
            }

            Column {
                anchors.left: avatar.right
                anchors.leftMargin: Math.round(12 * root.s)
                anchors.right: clockColumn.left
                anchors.rightMargin: Math.round(12 * root.s)
                anchors.verticalCenter: parent.verticalCenter
                spacing: Math.round(2 * root.s)

                Text {
                    width: parent.width
                    text: I18n.tr("%1@%2").arg(root.monitor.userName).arg(root.monitor.hostName)
                    textFormat: Text.PlainText
                    color: Lithic.fg
                    font.family: QsLook.sans
                    font.pixelSize: Math.round(14 * root.s)
                    font.weight: Font.Medium
                    renderType: Text.NativeRendering
                    elide: Text.ElideRight
                }
                Text {
                    width: parent.width
                    visible: root.monitor.sampleCount > 0
                    text: root.uptime(displayed.uptime) + I18n.tr(" · Load %1").arg(displayed.load.toFixed(2))
                    textFormat: Text.PlainText
                    color: Lithic.fg2
                    font.family: QsLook.sans
                    font.pixelSize: Math.round(12 * root.s)
                    font.features: ({ "tnum": 1 })
                    renderType: Text.NativeRendering
                    elide: Text.ElideRight
                }
            }

            Column {
                id: clockColumn
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: Math.round(2 * root.s)

                Text {
                    anchors.right: parent.right
                    text: TimeFormat.format(clock.date)
                    textFormat: Text.PlainText
                    color: Lithic.fg
                    font.family: QsLook.lithic ? QsLook.figures : QsLook.sans
                    font.pixelSize: Math.round((QsLook.lithic ? 20 * QsLook.figureScale : 22) * root.s)
                    font.weight: QsLook.lithic ? Font.Normal : Font.Light
                    font.features: ({ "tnum": 1 })
                    renderType: Text.NativeRendering
                }
                Micro {
                    anchors.right: parent.right
                    s: root.s
                    text: Qt.formatDateTime(clock.date, I18n.tr("ddd, MMM d"))
                }
            }
        }

        Item {
            id: readings
            visible: root.readingsShown
            width: stack.width
            height: body.implicitHeight + root.inset * 2

            Plate {
                visible: root.carded
                anchors.fill: parent
                s: root.s
            }

            Column {
                id: body
                x: root.inset
                y: root.inset
                width: readings.width - root.inset * 2
                spacing: root.sectionGap

                Item {
                    id: cpuBlock
                    visible: root.cpuShown
                    width: body.width
                    height: root.cpuInset * 2 + cpuRow.height
                        + (cores.visible ? Math.round(8 * root.s) + cores.height : 0)

                    Plate {
                        visible: QsLook.material
                        anchors.fill: parent
                        s: root.s
                    }

                    Item {
                        id: cpuRow
                        x: root.cpuInset
                        y: root.cpuInset
                        width: cpuBlock.width - root.cpuInset * 2
                        height: Math.max(readout.visible ? readout.implicitHeight : 0,
                            root.showGraph ? Math.round(52 * root.s) : 0)

                        Column {
                            id: readout
                            visible: root.showCpu || root.showTemperature
                            anchors.left: parent.left
                            anchors.verticalCenter: parent.verticalCenter
                            width: root.showGraph ? cpuRow.width * 0.45 - Math.round(12 * root.s) : cpuRow.width
                            spacing: Math.round(2 * root.s)
                            Accessible.role: Accessible.StaticText
                            Accessible.name: I18n.tr("Processor") + ", " + figure.text + percent.text + ", " + proof.text

                            Micro {
                                width: parent.width
                                s: root.s
                                text: I18n.tr("Processor")
                            }

                            Row {
                                visible: root.showCpu

                                Text {
                                    anchors.baseline: figure.baseline
                                    visible: QsLook.lithic && text.length > 0
                                    text: root.monitor.cpuAvailable ? "000".slice(figure.text.length) : ""
                                    color: Lithic.fg3
                                    font: figure.font
                                    renderType: Text.NativeRendering
                                }
                                Text {
                                    id: figure
                                    text: root.monitor.cpuAvailable ? String(Math.round(displayed.cpu)) : I18n.tr("–")
                                    color: Lithic.fg
                                    font.family: QsLook.lithic ? QsLook.figures : QsLook.sans
                                    font.pixelSize: root.figurePx
                                    font.weight: QsLook.lithic ? Font.Normal : Font.Light
                                    font.features: ({ "tnum": 1 })
                                    renderType: Text.NativeRendering
                                }
                                Text {
                                    id: percent
                                    anchors.baseline: figure.baseline
                                    leftPadding: Math.round((QsLook.lithic ? 4 : 2) * root.s)
                                    text: I18n.tr("%")
                                    color: Lithic.fg2
                                    font.family: QsLook.lithic ? QsLook.mono : QsLook.sans
                                    font.pixelSize: Math.round((QsLook.lithic ? 11 : 14) * root.s)
                                    renderType: Text.NativeRendering
                                }
                            }

                            Row {
                                id: proof
                                readonly property bool frequency: root.showCpu && displayed.cpuFrequency > 0
                                readonly property bool temperature: root.showTemperature && root.monitor.cpuTempAvailable
                                readonly property string text: (proof.frequency ? frequencyText.text : "")
                                    + (proof.frequency && proof.temperature ? separator.text : "")
                                    + (proof.temperature ? temperatureText.text : "")
                                visible: proof.frequency || proof.temperature

                                Text {
                                    id: frequencyText
                                    visible: proof.frequency
                                    text: I18n.tr("%1 GHz").arg(displayed.cpuFrequency.toFixed(2))
                                    color: Lithic.fg2
                                    font.family: QsLook.sans
                                    font.pixelSize: Math.round(12 * root.s)
                                    font.features: ({ "tnum": 1 })
                                    renderType: Text.NativeRendering
                                }
                                Text {
                                    id: separator
                                    visible: proof.frequency && proof.temperature
                                    text: I18n.tr(" · ")
                                    color: Lithic.fg3
                                    font.family: QsLook.sans
                                    font.pixelSize: Math.round(12 * root.s)
                                    renderType: Text.NativeRendering
                                }
                                Text {
                                    id: temperatureText
                                    visible: proof.temperature
                                    text: I18n.tr("%1 °C").arg(Math.round(displayed.cpuTemperature))
                                    color: root.cpuTemperatureInk
                                    font.family: QsLook.sans
                                    font.pixelSize: Math.round(12 * root.s)
                                    font.weight: Qt.colorEqual(root.cpuTemperatureInk, Lithic.fg2) ? Font.Normal : Font.Medium
                                    font.features: ({ "tnum": 1 })
                                    renderType: Text.NativeRendering
                                    Behavior on color {
                                        ColorAnimation { duration: QsLook.dur(QsLook.stateMs * 2) }
                                    }
                                }
                            }

                            HoverHandler {
                                id: readoutHover
                            }
                            CornerTip {
                                s: root.s
                                visible: readoutHover.hovered && text.length > 0
                                text: {
                                    const name = root.showCpu && root.monitor.cpuName.length > 0 ? root.monitor.cpuName : "";
                                    const source = !proof.temperature ? ""
                                        : root.monitor.cpuTempLabel === "zone" ? I18n.tr("Thermal zone temperature")
                                        : I18n.tr("CPU package temperature");
                                    return name.length > 0 && source.length > 0 ? name + "\n" + source : name + source;
                                }
                            }
                        }

                        Item {
                            id: graph
                            visible: root.showGraph
                            anchors.right: parent.right
                            anchors.top: parent.top
                            anchors.bottom: parent.bottom
                            width: readout.visible ? Math.round(cpuRow.width * 0.55) : cpuRow.width

                            Rectangle {
                                anchors.left: parent.left
                                anchors.right: parent.right
                                anchors.bottom: parent.bottom
                                height: Math.max(1, Math.round(root.s))
                                color: QsLook.lithic ? Lithic.vein : QsLook.fluent ? Look.rule
                                    : Lithic.alpha(Lithic.fg, QsLook.dark ? 0.12 : 0.14)
                            }

                            Loader {
                                anchors.fill: parent
                                anchors.bottomMargin: Math.max(1, Math.round(root.s))
                                active: root.showGraph && root.active
                                sourceComponent: VitalSparkline {
                                    monitor: root.monitor
                                    active: root.active
                                    s: root.s
                                    channels: ["cpu"]
                                    colors: [Lithic.sel]
                                    fill: true
                                    showGrid: false
                                    lineWidth: Math.max(1, Math.round((QsLook.material ? 2 : 1.5) * root.s))
                                }
                            }
                        }
                    }

                    VitalCores {
                        id: cores
                        visible: root.showCpu
                        x: root.cpuInset
                        anchors.top: cpuRow.bottom
                        anchors.topMargin: Math.round(8 * root.s)
                        width: cpuRow.width
                        height: implicitHeight
                        monitor: root.monitor
                        s: root.s
                    }
                }

                Rectangle {
                    visible: root.carded && root.cpuShown && root.cellCount > 0
                    width: body.width
                    height: Math.max(1, Math.round(root.s))
                    color: QsLook.adwaita ? Look.divider : Look.rule
                }

                Grid {
                    id: grid
                    readonly property real cell: (grid.width - grid.columnSpacing) / 2
                    visible: root.cellCount > 0
                    width: body.width
                    columns: 2
                    columnSpacing: root.columnGap
                    rowSpacing: root.rowGap

                    VitalMeter {
                        visible: root.memoryShown
                        width: grid.cell
                        s: root.s
                        settle: root.monitor.samplePeriodMs
                        label: I18n.tr("Memory")
                        valueText: I18n.tr("%1 / %2 GiB")
                            .arg(root.monitor.memoryUsedGiB.toFixed(1))
                            .arg(root.monitor.memoryTotalGiB.toFixed(1))
                        tooltipText: root.monitor.swapUsedGiB > 0.1
                            ? I18n.tr("Swap %1 GiB").arg(root.monitor.swapUsedGiB.toFixed(1)) : ""
                        showProgress: true
                        progressValue: root.monitor.memoryUsedGiB
                        progressMaximum: root.monitor.memoryTotalGiB
                    }

                    VitalMeter {
                        visible: root.gpuShown
                        width: grid.cell
                        s: root.s
                        settle: root.monitor.samplePeriodMs
                        label: I18n.tr("Graphics")
                        valueText: I18n.tr("%1%").arg(Math.round(root.monitor.gpuPercent))
                        detailText: root.monitor.gpuTempAvailable
                            ? I18n.tr(" · %1 °C").arg(Math.round(root.monitor.gpuTemp)) : ""
                        detailInk: root.quietInk(QsLook.temperatureInk(root.monitor.gpuTemp))
                        tooltipText: root.monitor.gpuMemoryAvailable
                            ? I18n.tr("VRAM %1 / %2 GiB")
                                .arg(root.monitor.gpuMemoryUsedGiB.toFixed(1))
                                .arg(root.monitor.gpuMemoryTotalGiB.toFixed(1))
                            : ""
                        showProgress: true
                        progressValue: root.monitor.gpuPercent
                    }

                    VitalMeter {
                        id: networkMeter
                        visible: root.networkShown
                        width: grid.cell
                        s: root.s
                        settle: root.monitor.samplePeriodMs
                        label: I18n.tr("Network")
                        valueText: I18n.tr("↓ %1  ↑ %2")
                            .arg(root.compactRate(root.monitor.networkRxBytesPerSec))
                            .arg(root.compactRate(root.monitor.networkTxBytesPerSec))
                        tooltipText: {
                            const rates = I18n.tr("↓ %1 · ↑ %2")
                                .arg(root.formatRate(root.monitor.networkRxBytesPerSec))
                                .arg(root.formatRate(root.monitor.networkTxBytesPerSec));
                            return root.monitor.networkInterface.length > 0
                                ? I18n.tr("%1 · %2").arg(rates).arg(root.monitor.networkInterface) : rates;
                        }
                        meterContent: Component {
                            VitalSparkline {
                                monitor: root.monitor
                                active: root.active && networkMeter.visible
                                s: root.s
                                channels: ["netRx", "netTx"]
                                colors: [Lithic.fg2, Lithic.sel]
                                fill: false
                                showGrid: false
                            }
                        }
                    }

                    VitalMeter {
                        visible: root.diskShown
                        width: grid.cell
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
                    }

                    VitalMeter {
                        id: batteryMeter
                        readonly property color warning: QsLook.batteryInk(root.monitor.batteryPercent,
                            root.monitor.batteryCharging)
                        readonly property bool warned: !Qt.colorEqual(batteryMeter.warning, Lithic.fg)
                        visible: root.batteryShown
                        width: grid.cell
                        s: root.s
                        settle: root.monitor.samplePeriodMs
                        label: I18n.tr("Battery")
                        valueText: I18n.tr("%1%").arg(Math.round(root.monitor.batteryPercent))
                        valueInk: batteryMeter.warning
                        detailText: I18n.tr(" · %1").arg(root.monitor.batteryCharging
                            ? I18n.tr("Charging") : I18n.tr("On battery"))
                        showProgress: true
                        progressValue: root.monitor.batteryPercent
                        accent: batteryMeter.warned ? batteryMeter.warning
                            : QsLook.lithic ? Lithic.fg2 : Lithic.sel
                    }
                }
            }
        }
    }
}
