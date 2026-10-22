import "performance"
import QtQuick
import QtQuick.Layouts
import Quickshell.Services.UPower
import Ryoku.Blobs
import Caelestia.Config
import Caelestia.Services
import caelestia.components
import caelestia.services

Item {
    id: root

    readonly property int historyCapacity: 60
    property var cpuHistory: []

    // what the Controls page leaves shown of the vitals
    readonly property bool showCpu: Config.dashboard.performance.showCpu && HostState.element("cpu")
    readonly property bool showGpu: Config.dashboard.performance.showGpu && Gpu.type !== Gpu.None && HostState.element("gpu")
    readonly property bool showMemory: Config.dashboard.performance.showMemory && HostState.element("memory")
    readonly property bool showStorage: Config.dashboard.performance.showStorage && HostState.element("disk")
    readonly property bool showNetwork: Config.dashboard.performance.showNetwork && HostState.element("network")
    readonly property bool showBattery: UPower.displayDevice.isLaptopBattery && Config.dashboard.performance.showBattery && HostState.element("battery")

    implicitWidth: placeholder.active ? Tokens.sizes.dashboard.perfPlaceholderWidth : content.implicitWidth
    implicitHeight: placeholder.active ? placeholder.implicitHeight + Tokens.padding.extraLarge * 2 : content.implicitHeight

    SystemMonitor {
        id: sysMonitor

        active: root.visible && !placeholder.active
        onSampleCountChanged: {
            if (sampleCount <= 0) {
                root.cpuHistory = [];
                return;
            }
            const raw = isFinite(Cpu.percentage) && Cpu.percentage > 0 ? Cpu.percentage : (sysMonitor.cpuPercent / 100);
            const val = isFinite(raw) ? Math.max(0, Math.min(1, raw)) : 0;
            const next = root.cpuHistory.slice(Math.max(0, root.cpuHistory.length - root.historyCapacity + 1));
            next.push(val);
            root.cpuHistory = next;
        }
    }

    Loader {
        id: placeholder

        anchors.centerIn: parent
        active: !root.showCpu && !root.showGpu && !root.showMemory && !root.showStorage && !root.showNetwork && !root.showBattery
        asynchronous: true

        sourceComponent: ColumnLayout {
            spacing: Tokens.spacing.medium

            MaterialIcon {
                Layout.alignment: Qt.AlignHCenter
                text: "tune"
                fontStyle: Tokens.font.icon.builders.extraLarge.scale(2).build()
                color: Colours.palette.m3onSurfaceVariant
            }

            StyledText {
                Layout.topMargin: -Tokens.spacing.small
                Layout.alignment: Qt.AlignHCenter
                text: qsTr("No widgets enabled")
                font: Tokens.font.title.large
                color: Colours.palette.m3onSurface
            }

            StyledText {
                Layout.alignment: Qt.AlignHCenter
                text: qsTr("Enable widgets in the dashboard settings")
                font: Tokens.font.body.small
                color: Colours.palette.m3onSurfaceVariant
            }
        }
    }

    RowLayout {
        id: content

        anchors.left: parent.left
        anchors.right: parent.right
        spacing: Tokens.spacing.medium
        visible: !placeholder.active

        ColumnLayout {
            id: mainColumn

            Layout.fillWidth: true
            spacing: Tokens.spacing.medium

            RowLayout {
                spacing: Tokens.spacing.medium
                visible: cpuCard.active || gpuCard.active

                WrappedLoader {
                    id: cpuCard

                    active: root.showCpu

                    sourceComponent: HeroCard {
                        icon: "memory"
                        label: qsTr("CPU")
                        subLabel: Cpu.name
                        usage: !isNaN(Cpu.percentage) ? Cpu.percentage : sysMonitor.cpuPercent / 100
                        temperature: !isNaN(Cpu.temperature) && Cpu.temperature > 0 ? Cpu.temperature : sysMonitor.cpuTemp
                        accent: Colours.palette.m3primary
                        monitor: sysMonitor
                        frequency: sysMonitor.cpuFrequencyGhz
                        coreLoads: sysMonitor.coreLoads
                        coreCount: sysMonitor.coreCount
                        history: HostState.element("liveGraph") ? root.cpuHistory : []
        showTemperature: HostState.element("cpuTemperature")

                        ServiceRef {
                            service: Cpu
                        }
                    }
                }

                WrappedLoader {
                    id: gpuCard

                    active: root.showGpu

                    sourceComponent: HeroCard {
                        icon: "desktop_windows"
                        label: qsTr("GPU")
                        subLabel: Gpu.name
                        usage: Gpu.percentage
                        temperature: Gpu.temperature
                        showTemperature: HostState.element("cpuTemperature")
                        accent: Colours.palette.m3secondary
                        monitor: sysMonitor
                        vramAvailable: sysMonitor.gpuMemoryAvailable
                        vramUsedGiB: sysMonitor.gpuMemoryUsedGiB
                        vramTotalGiB: sysMonitor.gpuMemoryTotalGiB

                        ServiceRef {
                            service: Gpu
                        }
                    }
                }
            }

            RowLayout {
                spacing: Tokens.spacing.medium
                visible: storageCard.active || networkCard.active || memoryCard.active

                WrappedLoader {
                    id: storageCard

                    active: root.showStorage
                    sourceComponent: StorageCard {
                        monitor: sysMonitor
                        diskReadBytesPerSec: sysMonitor.diskReadBytesPerSec
                        diskWriteBytesPerSec: sysMonitor.diskWriteBytesPerSec
                        storageTemp: sysMonitor.storageTemp
                        storageTempAvailable: sysMonitor.storageTempAvailable
                    }
                }

                WrappedLoader {
                    id: networkCard

                    active: root.showNetwork
                    sourceComponent: NetworkCard {
                        monitor: sysMonitor
                    }
                }

                WrappedLoader {
                    id: memoryCard

                    active: root.showMemory
                    sourceComponent: MemoryCard {
                        monitor: sysMonitor
                        swapUsedGiB: sysMonitor.swapUsedGiB
                        swapTotalGiB: sysMonitor.swapTotalGiB
                    }
                }
            }
        }

        WrappedLoader {
            Layout.fillWidth: false
            active: root.showBattery
            sourceComponent: BatteryTank {}
        }
    }

    component WrappedLoader: Loader {
        Layout.fillWidth: true
        Layout.fillHeight: true
        visible: active
    }
}
