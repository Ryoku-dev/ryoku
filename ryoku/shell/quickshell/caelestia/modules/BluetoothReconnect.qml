import QtQuick
import Quickshell
import Quickshell.Bluetooth
import Caelestia.Config
import caelestia.utils

Scope {
    id: root

    property var attemptedDevices: ({})

    readonly property bool adapterEnabled: Bluetooth.defaultAdapter ? Bluetooth.defaultAdapter.enabled : false
    readonly property var devicesList: Bluetooth.devices.values
    readonly property var autoReconnectList: GlobalConfig.services.bluetoothAutoReconnectDevices

    function checkAndReconnect() {
        if (!adapterEnabled)
            return;

        const list = autoReconnectList;
        if (!list || list.length === 0)
            return;

        for (let i = 0; i < devicesList.length; i++) {
            const device = devicesList[i];
            if (!device || !device.address)
                continue;

            if (list.indexOf(device.address) !== -1) {
                if (!attemptedDevices[device.address]) {
                    attemptedDevices[device.address] = true;

                    if (device.state === BluetoothDeviceState.Disconnected) { // qmllint disable unresolved-type
                        Logger.log("[BluetoothReconnect] Reconnecting device on startup:", device.name, device.address);
                        device.connected = true;
                    }
                }
            }
        }
    }

    onDevicesListChanged: checkAndReconnect()
    onAdapterEnabledChanged: checkAndReconnect()
    onAutoReconnectListChanged: checkAndReconnect()

    Component.onCompleted: checkAndReconnect()
}
