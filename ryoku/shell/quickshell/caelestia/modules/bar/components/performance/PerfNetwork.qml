import QtQuick
import Caelestia.Services
import caelestia.components
import caelestia.services

PerfStat {
    readonly property real totalSpeed: (NetworkUsage.downloadSpeed ?? 0) + (NetworkUsage.uploadSpeed ?? 0)

    widthFactor: 3.5
    maxText: "999.9 MB/s"

    icon: "swap_vert"
    accent: Colours.palette.m3tertiary
    value: NaN
    valueText: Units.formatBytes(totalSpeed, true)

    ServiceRef {
        service: NetworkUsage
    }
}
