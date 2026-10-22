import Quickshell.Services.UPower
import caelestia.components
import caelestia.services
import caelestia.utils

PerfStat {
    readonly property bool charging: [UPowerDeviceState.Charging, UPowerDeviceState.FullyCharged, UPowerDeviceState.PendingCharge].includes(UPower.displayDevice.state)

    icon: UPower.displayDevice.isLaptopBattery ? Icons.getBatteryIcon(UPower.displayDevice.percentage, charging) : "battery_unknown"
    accent: !UPower.onBattery || UPower.displayDevice.percentage > 0.2 ? Colours.palette.m3primary : Colours.palette.m3error
    value: UPower.displayDevice.percentage
    valueText: UPower.displayDevice.isLaptopBattery ? Math.round(UPower.displayDevice.percentage * 100) : qsTr("N/A")
}
