import QtQuick
import Quickshell
import Quickshell.Bluetooth
import inir.services
import inir.modules.common
import inir.modules.common.functions
import inir.modules.common.widgets

QuickToggleModel {
    name: Translation.tr("Bluetooth")
    statusText: BluetoothStatus.blocked ? Translation.tr("Blocked")
        : BluetoothStatus.firstActiveDevice?.name ?? Translation.tr("Not connected")
    tooltipText: Translation.tr("%1 | Right-click to configure").arg(
        BluetoothStatus.blocked ? Translation.tr("Bluetooth is blocked")
            : BluetoothStatus.activeDeviceSummary(true) || Translation.tr("Bluetooth")
    )
    icon: BluetoothStatus.activeIcon

    available: BluetoothStatus.available
    toggled: BluetoothStatus.enabled
    mainAction: () => BluetoothStatus.toggle()
    hasMenu: true
    altAction: () => {
        AppLauncher.launch("bluetooth")
    }
}
